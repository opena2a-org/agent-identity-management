package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Reuse detection with family revocation (RFC 9700 section 4.14.2): a login
// refresh token presented again after it was rotated out ends the whole
// sign-in. Every refresh token of that family is refused from then on, each
// refused presentation is recorded once (an audit row and a SECURITY log
// line, identifiers only), and a store that did not answer records nothing.

const familyRefusal = "Token has been revoked or is invalid"
const familyTestUA = "family-cell/1.0"

// familyAuditRepo records every audit row the refresh route writes.
type familyAuditRepo struct {
	domain.AuditLogRepository
	rows []*domain.AuditLog
}

func (r *familyAuditRepo) Create(row *domain.AuditLog) error {
	r.rows = append(r.rows, row)
	return nil
}

func newRefreshTestAppAudit(t *testing.T, users domain.UserRepository, sdkRepo domain.SDKTokenRepository, audit *familyAuditRepo) (*fiber.App, *auth.JWTService) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	jwtSvc := auth.NewJWTService()
	if sdkRepo == nil {
		sdkRepo = &refreshTestSDKRepo{}
	}
	h := NewAuthRefreshHandler(jwtSvc, application.NewSDKTokenService(sdkRepo), users, application.NewAuditService(audit))
	app := fiber.New()
	app.Post("/auth/refresh", h.RefreshToken)
	return app, jwtSvc
}

type familyFixture struct {
	app    *fiber.App
	svc    *auth.JWTService
	store  *rotationStore
	audit  *familyAuditRepo
	p1     string
	userID uuid.UUID
	orgID  uuid.UUID
}

func familyApp(t *testing.T, store *rotationStore, failOpen bool) *familyFixture {
	t.Helper()
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "family@example.com"), nil
	}}
	audit := &familyAuditRepo{}
	app, jwtSvc := newRefreshTestAppAudit(t, users, nil, audit)
	if store != nil {
		jwtSvc.SetRevoker(auth.NewTokenRevoker(store, failOpen))
	}
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "family@example.com", "admin")
	require.NoError(t, err)
	return &familyFixture{app: app, svc: jwtSvc, store: store, audit: audit, p1: refresh, userID: userID, orgID: orgID}
}

// captureLog routes the standard logger into a buffer for the test's life.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

func postRefreshUA(t *testing.T, app *fiber.App, refreshToken string) (string, int) {
	t.Helper()
	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, refreshToken)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", familyTestUA)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode
}

func familyKeyWrites(store *rotationStore) int {
	n := 0
	for k := range store.data {
		if strings.HasPrefix(k, "revoked:fam:") {
			n++
		}
	}
	return n
}

func marshalled(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// F1 + F4 (Done-when 1): a rotated-out token presented again is refused, the
// live token of the same family is refused after it, the family key is
// written, and each refusal is recorded once with identifiers only.
func TestRefreshToken_ReuseRevokesTheFamilyAndRecordsIt(t *testing.T) {
	logBuf := captureLog(t)
	f := familyApp(t, &rotationStore{}, false)
	jti1 := jtiOfToken(t, f.svc, f.p1)

	out, status := postRefresh(t, f.app, f.p1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)
	p2 := out.RefreshToken
	jti2 := jtiOfToken(t, f.svc, p2)
	assert.Empty(t, f.audit.rows, "a successful refresh records nothing")

	body, status := postRefreshUA(t, f.app, f.p1)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	assert.True(t, f.store.data["revoked:fam:"+jti1], "the family key is written under the login token's jti")

	body, status = postRefreshUA(t, f.app, p2)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the token that was live is refused after the reuse")
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken")

	require.Len(t, f.audit.rows, 2, "one row per refused presentation")
	reuse, member := f.audit.rows[0], f.audit.rows[1]
	assert.Equal(t, domain.AuditActionRefreshTokenReuse, reuse.Action)
	require.NotNil(t, reuse.UserID)
	assert.Equal(t, f.userID, *reuse.UserID)
	assert.Equal(t, f.orgID, reuse.OrganizationID)
	assert.Equal(t, familyTestUA, reuse.UserAgent)
	assert.Equal(t, jti1, reuse.Metadata["familyId"])
	assert.Equal(t, jti1, reuse.Metadata["jti"])
	assert.Equal(t, true, reuse.Metadata["familyRevoked"])
	assert.Equal(t, domain.AuditActionRefreshSessionRevoked, member.Action)
	assert.Equal(t, jti1, member.Metadata["familyId"])
	assert.Equal(t, jti2, member.Metadata["jti"])

	logged := logBuf.String()
	assert.Equal(t, 1, strings.Count(logged, "SECURITY refresh_token_reuse"))
	assert.Equal(t, 1, strings.Count(logged, "SECURITY refresh_session_revoked"))
	assert.Equal(t, 2, strings.Count(logged, "family="+jti1))
	assert.Contains(t, logged, jti1, "control: the identifier is in the log")
	for _, secret := range []string{f.p1, p2} {
		assert.NotContains(t, logged, secret, "no token in the log")
		for _, row := range f.audit.rows {
			assert.NotContains(t, marshalled(t, row.Metadata), secret, "no token in a row")
		}
	}
}

// F2: the family spans the whole chain; a reuse of a middle member ends it.
func TestRefreshToken_ReuseOfAMiddleMemberEndsTheChain(t *testing.T) {
	f := familyApp(t, &rotationStore{}, false)
	out2, status := postRefresh(t, f.app, f.p1)
	require.Equal(t, fiber.StatusOK, status)
	out3, status := postRefresh(t, f.app, out2.RefreshToken)
	require.Equal(t, fiber.StatusOK, status)

	_, status = postRefresh(t, f.app, out2.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	_, status = postRefresh(t, f.app, out3.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the newest token is refused after a middle member's reuse")
}

// F3: a login refresh token minted before this change joins the mechanism on
// its first rotation: its jti names the family, and its reuse ends it.
func TestRefreshToken_PreChangeTokenJoinsTheFamilyOnRotation(t *testing.T) {
	f := familyApp(t, &rotationStore{}, false)
	now := time.Now()
	l, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.JWTClaims{
		UserID:         f.userID.String(),
		OrganizationID: f.orgID.String(),
		TokenType:      auth.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    auth.IssuerUser,
			Subject:   f.userID.String(),
			ID:        uuid.New().String(),
		},
	}).SignedString([]byte("test-secret-key-for-unit-tests-32"))
	require.NoError(t, err)
	jtiL := jtiOfToken(t, f.svc, l)

	out, status := postRefresh(t, f.app, l)
	require.Equal(t, fiber.StatusOK, status)
	claims, err := f.svc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, jtiL, claims.SessionID)

	_, status = postRefresh(t, f.app, l)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	_, status = postRefresh(t, f.app, out.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.True(t, f.store.data["revoked:fam:"+jtiL])
}

// F5: a store that does not answer records nothing and revokes no family;
// enforcement follows the fail-open setting as before. When the store answers
// again, a real reuse is recorded (the positive control).
func TestRefreshToken_UnansweredStoreRecordsNoReuse(t *testing.T) {
	logBuf := captureLog(t)
	closed := familyApp(t, &rotationStore{existsErr: errors.New("store down")}, false)
	body, status := postRefreshUA(t, closed.app, closed.p1)
	assert.Equal(t, fiber.StatusUnauthorized, status, "fail-closed refuses")
	assert.Contains(t, body, familyRefusal)
	assert.Empty(t, closed.audit.rows)
	assert.Equal(t, 0, familyKeyWrites(closed.store))
	assert.NotContains(t, logBuf.String(), "SECURITY")

	open := familyApp(t, &rotationStore{existsErr: errors.New("store down")}, true)
	_, status = postRefresh(t, open.app, open.p1)
	assert.Equal(t, fiber.StatusOK, status, "fail-open allows")
	assert.Empty(t, open.audit.rows)

	closed.store.existsErr = nil
	out, status := postRefresh(t, closed.app, closed.p1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)
	_, status = postRefreshUA(t, closed.app, closed.p1)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	require.Len(t, closed.audit.rows, 1, "control: a real reuse is recorded once the store answers")
	assert.Equal(t, domain.AuditActionRefreshTokenReuse, closed.audit.rows[0].Action)
}

// F6: two instances over one store. B read the token as not revoked (its
// refresh then failed at the principal lookup), A rotated it, and B is given
// it again: B must not serve it from its cache, and the family is over.
func TestRefreshToken_CrossInstanceReplayIsRefused(t *testing.T) {
	store := &rotationStore{}
	userID, orgID := uuid.New(), uuid.New()
	usersA := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "xi@example.com"), nil
	}}
	failOnce := true
	usersB := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		if failOnce {
			failOnce = false
			return nil, errors.New("lookup failed once")
		}
		return activeUser(userID, orgID, domain.RoleAdmin, "xi@example.com"), nil
	}}
	appA, svcA := newRefreshTestAppAudit(t, usersA, nil, &familyAuditRepo{})
	svcA.SetRevoker(auth.NewTokenRevoker(store, false))
	appB, svcB := newRefreshTestAppAudit(t, usersB, nil, &familyAuditRepo{})
	svcB.SetRevoker(auth.NewTokenRevoker(store, false))
	_, p1, err := svcA.GenerateTokenPair(userID.String(), orgID.String(), "xi@example.com", "admin")
	require.NoError(t, err)

	_, status := postRefresh(t, appB, p1)
	require.Equal(t, fiber.StatusUnauthorized, status, "B's lookup fails after its revocation read")
	out, status := postRefresh(t, appA, p1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)

	_, status = postRefresh(t, appB, p1)
	assert.Equal(t, fiber.StatusUnauthorized, status, "B does not serve the rotated-out token from its cache")
	_, status = postRefresh(t, appA, out.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the family is over on every instance")
}

// F7: the SDK-download path is unchanged: no sid, no family key, no row; a
// revoked SDK row is refused with no row. Control: a login-pair reuse on the
// same store writes one family key.
func TestRefreshToken_SDKPathHasNoFamily(t *testing.T) {
	store := &rotationStore{}
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "sdk@example.com"), nil
	}}
	sdkRepo := &rotationSDKRepo{token: &domain.SDKToken{
		ID: uuid.New(), UserID: userID, OrganizationID: orgID, ExpiresAt: time.Now().Add(24 * time.Hour),
	}}
	audit := &familyAuditRepo{}
	app, jwtSvc := newRefreshTestAppAudit(t, users, sdkRepo, audit)
	jwtSvc.SetRevoker(auth.NewTokenRevoker(store, false))
	old, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "sdk@example.com", "admin")
	require.NoError(t, err)

	out, status := postRefresh(t, app, old)
	require.Equal(t, fiber.StatusOK, status)
	claims, err := jwtSvc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	assert.Empty(t, claims.SessionID)
	assert.Equal(t, 0, familyKeyWrites(store))
	assert.Empty(t, audit.rows)

	revokedAt := time.Now()
	sdkRepo.token.RevokedAt = &revokedAt
	body, status := postRefreshRaw(t, app, out.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	assert.Empty(t, audit.rows, "a revoked SDK row records no reuse")

	sdkRepo.token = nil // login tokens are not tracked in sdk_tokens
	_, p1, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "sdk@example.com", "admin")
	require.NoError(t, err)
	_, status = postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusOK, status)
	_, status = postRefresh(t, app, p1)
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, 1, familyKeyWrites(store), "control: a login-pair reuse writes one family key")
}

// F8 (pin): without a revocation store nothing is retired, so a second
// presentation is not a reuse: 200 both times, the token unchanged, no rows.
func TestRefreshToken_NoStoreMeansNoReuse(t *testing.T) {
	f := familyApp(t, nil, false)
	for i := 0; i < 2; i++ {
		out, status := postRefresh(t, f.app, f.p1)
		require.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, f.p1, out.RefreshToken)
		assert.False(t, out.Rotated)
	}
	assert.Empty(t, f.audit.rows)
}

var _ = context.Background
