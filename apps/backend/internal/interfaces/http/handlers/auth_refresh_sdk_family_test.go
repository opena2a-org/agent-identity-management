package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// SDK-download token families: an SDK-download token carries a sid set at
// download (its own jti) and copied on every rotation, the same mechanism a
// login refresh token uses. A rotated-out SDK token presented again ends the
// whole chain that grew from it: the family key is written (revoked:fam:),
// every sdk_tokens row of the family is revoked, the event is recorded the
// same way as a login reuse, and the recovery route refuses the family. A
// login family and an SDK family never share an id.

// sdkFamilyRepo models sdk_tokens by token hash: rows are created by the
// download and rotation paths, retired by a set-if-absent UPDATE (a row
// already revoked is not revoked again), and revoked by family: the
// download's row and every row that descends from it by parent_token. Reads
// return a copy, as a database read would.
type sdkFamilyRepo struct {
	domain.SDKTokenRepository
	rows map[string]*domain.SDKToken
	// beforeRetire, when set, runs once inside the retirement before the row
	// is checked: another presentation of the same token that retires the row
	// first.
	beforeRetire func()
}

func (r *sdkFamilyRepo) Create(token *domain.SDKToken) error {
	r.rows[token.TokenHash] = token
	return nil
}

func (r *sdkFamilyRepo) GetByTokenHash(hash string) (*domain.SDKToken, error) {
	row, ok := r.rows[hash]
	if !ok {
		return nil, fmt.Errorf("not tracked")
	}
	cp := *row
	return &cp, nil
}

func (r *sdkFamilyRepo) RecordUsage(string, string) error { return nil }

func (r *sdkFamilyRepo) RevokeByTokenHash(hash, reason string) error {
	if f := r.beforeRetire; f != nil {
		r.beforeRetire = nil
		f()
	}
	row, ok := r.rows[hash]
	if !ok || row.RevokedAt != nil {
		return fmt.Errorf("SDK token not found or already revoked")
	}
	row.Revoke(reason)
	return nil
}

// Rotate retires the row and stores its successor's together, for a refresh
// route that rotates a row in one transaction: neither write happens when the
// row is no longer active.
func (r *sdkFamilyRepo) Rotate(hash, reason string, next *domain.SDKToken) error {
	if err := r.RevokeByTokenHash(hash, reason); err != nil {
		return err
	}
	return r.Create(next)
}

func (r *sdkFamilyRepo) RevokeFamily(userID uuid.UUID, familyID, reason string) error {
	inFamily := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for _, row := range r.rows {
			id := row.ID.String()
			if row.UserID != userID || inFamily[id] {
				continue
			}
			parent, _ := row.Metadata["parent_token"].(string)
			if row.TokenID == familyID || inFamily[parent] {
				inFamily[id] = true
				grew = true
			}
		}
	}
	for _, row := range r.rows {
		if inFamily[row.ID.String()] && row.RevokedAt == nil {
			row.Revoke(reason)
		}
	}
	return nil
}

func (r *sdkFamilyRepo) row(token string) *domain.SDKToken {
	return r.rows[sha256Hex(token)]
}

func (r *sdkFamilyRepo) activeRows() int {
	n := 0
	for _, row := range r.rows {
		if row.IsActive() {
			n++
		}
	}
	return n
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type sdkFamilyFixture struct {
	refresh  *fiber.App
	download *fiber.App
	recover  *fiber.App
	svc      *auth.JWTService
	store    *rotationStore
	repo     *sdkFamilyRepo
	audit    *familyAuditRepo
	userID   uuid.UUID
	orgID    uuid.UUID
	// actingFamily is the sid of the access token that downloads and recovers.
	actingFamily string
}

// newSDKFamilyFixture wires the refresh, download and recovery routes to one
// jwt service, one sdk_tokens model and one audit log. A nil store means no
// revocation store is configured.
func newSDKFamilyFixture(t *testing.T, store *rotationStore) *sdkFamilyFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	svc := auth.NewJWTService()
	if store != nil {
		svc.SetRevoker(auth.NewTokenRevoker(store, false))
	}
	f := &sdkFamilyFixture{
		svc:    svc,
		store:  store,
		repo:   &sdkFamilyRepo{rows: map[string]*domain.SDKToken{}},
		audit:  &familyAuditRepo{},
		userID: uuid.New(),
		orgID:  uuid.New(),
	}
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(f.userID, f.orgID, domain.RoleAdmin, "sdkfam@example.com"), nil
	}}
	auditSvc := application.NewAuditService(f.audit)
	tokens := application.NewSDKTokenService(f.repo)

	rh := NewAuthRefreshHandler(svc, tokens, users, auditSvc)
	f.refresh = fiber.New()
	f.refresh.Post("/auth/refresh", rh.RefreshToken)

	// the zip is built from SDK_BASE_DIR/<sdk>; a two-file fixture stands in for the SDK tree
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "python", "aim_sdk"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "VERSION"), []byte("0.0.0-fixture\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "aim_sdk", "__init__.py"), []byte("# fixture\n"), 0o644))
	t.Setenv("SDK_BASE_DIR", dir)
	acting := func(next fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("user_id", f.userID)
			c.Locals("organization_id", f.orgID)
			c.Locals("email", "sdkfam@example.com")
			c.Locals("role", "admin")
			c.Locals("sid", f.actingFamily)
			return next(c)
		}
	}
	dh := NewSDKHandler(svc, f.repo, auditSvc)
	f.download = fiber.New()
	f.download.Get("/sdk/download", acting(dh.DownloadSDK))
	vh := NewSDKTokenRecoveryHandler(tokens, svc, users, auditSvc)
	f.recover = fiber.New()
	f.recover.Post("/auth/sdk/recover", acting(vh.RecoverRevokedToken))
	return f
}

// downloadSDK downloads an SDK and returns the refresh token embedded in it.
func (f *sdkFamilyFixture) downloadSDK(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/sdk/download?sdk=python", nil)
	resp, err := f.download.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, string(raw))
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	for _, file := range zr.File {
		if !strings.HasSuffix(file.Name, filepath.Join(".aim", "sdk_credentials.json")) {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		var creds SDKCredentials
		require.NoError(t, json.NewDecoder(rc).Decode(&creds))
		_ = rc.Close()
		require.NotEmpty(t, creds.RefreshToken)
		return creds.RefreshToken
	}
	t.Fatal("the SDK zip carries no credentials file")
	return ""
}

func (f *sdkFamilyFixture) recoverWith(t *testing.T, old string) (int, string) {
	t.Helper()
	return doRequest(t, f.recover, "POST", "/auth/sdk/recover", recoverBody(old))
}

func (f *sdkFamilyFixture) reuseRows() []*domain.AuditLog {
	var out []*domain.AuditLog
	for _, row := range f.audit.rows {
		if row.Action == domain.AuditActionRefreshTokenReuse {
			out = append(out, row)
		}
	}
	return out
}

// SF1 (Done-when 1): a rotated-out SDK token presented again ends the chain
// that grew from it. The live token is refused after it, the family key is
// written, every row of the family is revoked, and the reuse is recorded once
// with the family's id.
func TestRefreshToken_SDKReuseEndsTheFamily(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	family := jtiOfToken(t, f.svc, s1)

	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)
	s2 := out.RefreshToken

	body, status := postRefreshRaw(t, f.refresh, s1)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the rotated-out token is refused")
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken")

	body, status = postRefreshRaw(t, f.refresh, s2)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the reuse ended the chain: the live token is refused too")
	assert.NotContains(t, body, "accessToken")

	assert.True(t, f.store.data["revoked:fam:"+family], "the family key is written under the SDK family's id")
	assert.Equal(t, 1, familyKeyWrites(f.store))
	assert.Equal(t, 0, f.repo.activeRows(), "every row of the family is revoked")
	reuse := f.reuseRows()
	require.Len(t, reuse, 1, "the reuse is recorded once")
	assert.Equal(t, family, reuse[0].Metadata["familyId"])
	assert.Equal(t, true, reuse[0].Metadata["familyRevoked"])
}

// SF2 (Done-when 1): the sid is set at download (the token's own jti) and
// copied on rotation; the access token the SDK path mints carries the family.
// The download's row has the family's id as its token ID and the rotated row
// descends from it, which is how the family's rows are found.
func TestRefreshToken_SDKTokenCarriesItsFamilyFromDownloadThroughRotation(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	c1, err := f.svc.ValidateToken(s1)
	require.NoError(t, err)
	assert.Equal(t, c1.ID, c1.SessionID, "at download the family is the token's own jti")
	assert.Equal(t, c1.ID, c1.FamilyID())

	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status)
	c2, err := f.svc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, auth.TokenTypeSDK, c2.TokenType)
	assert.Equal(t, c1.ID, c2.SessionID, "rotation copies the family")
	assert.NotEqual(t, c1.ID, c2.ID, "rotation draws a fresh jti")
	ac, err := f.svc.ValidateToken(out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, c1.ID, ac.AccessFamilyID(), "the SDK path's access token carries the family")

	assert.Equal(t, c1.ID, f.repo.row(s1).TokenID, "the download row's token ID is the family's id")
	assert.Equal(t, f.repo.row(s1).ID.String(), f.repo.row(out.RefreshToken).Metadata["parent_token"], "the rotated row descends from the download row")
}

// SF3: a login family and an SDK family never share an id. An SDK downloaded
// with a login session's access token starts its own family, and ending
// either family leaves the other live.
func TestRefreshToken_LoginAndSDKFamiliesNeverShareAnID(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	_, l1, err := f.svc.GenerateTokenPair(f.userID.String(), f.orgID.String(), "sdkfam@example.com", "admin")
	require.NoError(t, err)
	login := jtiOfToken(t, f.svc, l1)
	f.actingFamily = login

	s1 := f.downloadSDK(t)
	sc, err := f.svc.ValidateToken(s1)
	require.NoError(t, err)
	require.NotEmpty(t, sc.FamilyID())
	assert.NotEqual(t, login, sc.FamilyID(), "the SDK family is not the downloading session's family")
	assert.Equal(t, sc.ID, sc.FamilyID())

	// An SDK reuse ends the SDK family only.
	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status)
	_, status = postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusUnauthorized, status)
	lout, status := postRefresh(t, f.refresh, l1)
	assert.Equal(t, fiber.StatusOK, status, "the login family is untouched by the SDK reuse")
	_, status = postRefresh(t, f.refresh, out.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)

	// A login reuse ends the login family only.
	s2 := f.downloadSDK(t)
	_, status = postRefresh(t, f.refresh, l1)
	require.Equal(t, fiber.StatusUnauthorized, status, "a login reuse")
	_, status = postRefresh(t, f.refresh, lout.RefreshToken)
	require.Equal(t, fiber.StatusUnauthorized, status, "the login family is over")
	_, status = postRefresh(t, f.refresh, s2)
	assert.Equal(t, fiber.StatusOK, status, "an SDK family is untouched by the login reuse")
	assert.Equal(t, 2, familyKeyWrites(f.store), "one key per family")
}

// SF4: without a revocation store the SDK chain still ends, by row: the
// family's rows are revoked, so the live token is refused.
func TestRefreshToken_SDKReuseEndsTheFamilyByRowWithoutAStore(t *testing.T) {
	f := newSDKFamilyFixture(t, nil)
	s1 := f.downloadSDK(t)
	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)

	_, status = postRefresh(t, f.refresh, s1)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	_, status = postRefresh(t, f.refresh, out.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the live token's row was revoked with its family")
	assert.Equal(t, 0, f.repo.activeRows())
	reuse := f.reuseRows()
	require.Len(t, reuse, 1)
	assert.Equal(t, true, reuse[0].Metadata["familyRevoked"], "the rows ended the family")
}

// SF5: two presentations of one SDK token: the one that loses the row's
// set-if-absent retirement is a reuse. It gets no tokens, no row is live for
// what it minted, and the family ends, the winner's new token with it.
func TestRefreshToken_SDKRetirementRaceLoserEndsTheFamily(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	var winner *RefreshTokenResponse
	var winnerStatus int
	f.repo.beforeRetire = func() { winner, winnerStatus = postRefresh(t, f.refresh, s1) }

	body, status := postRefreshRaw(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, winnerStatus, "the first retirement wins")
	assert.Equal(t, fiber.StatusUnauthorized, status, "the presentation that lost the retirement is refused")
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken", "the loser's minted tokens are not handed out")
	assert.Equal(t, 0, f.repo.activeRows(), "no row of the family is live, none for what the loser minted")

	_, status = postRefresh(t, f.refresh, winner.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the family is over, the winner's token with it")
	assert.Equal(t, 1, familyKeyWrites(f.store))
	assert.Len(t, f.reuseRows(), 1)
}

// SF6: the recovery route refuses a family a reuse ended, with or without a
// revocation store; a row its owner revoked is still recovered.
func TestRecoverRevokedToken_RefusesARevokedSDKFamily(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			s1 := f.downloadSDK(t)
			out, status := postRefresh(t, f.refresh, s1)
			require.Equal(t, fiber.StatusOK, status)
			_, status = postRefresh(t, f.refresh, s1)
			require.Equal(t, fiber.StatusUnauthorized, status)
			rows := len(f.repo.rows)

			status, body := f.recoverWith(t, out.RefreshToken)
			assert.Equal(t, fiber.StatusUnauthorized, status, "the live token's family was ended by the reuse")
			assert.Contains(t, body, familyRefusal)
			assert.NotContains(t, body, "refreshToken")
			assert.Len(t, f.repo.rows, rows, "no recovered credential row")

			// control: a row its owner revoked (no reuse) is recovered
			s3 := f.downloadSDK(t)
			require.NoError(t, f.repo.Revoke(f.repo.row(s3).ID, "user_revoked"))
			status, body = f.recoverWith(t, s3)
			assert.Equal(t, fiber.StatusOK, status, body)
		})
	}
}

func (r *sdkFamilyRepo) Revoke(id uuid.UUID, reason string) error {
	for _, row := range r.rows {
		if row.ID == id && row.RevokedAt == nil {
			row.Revoke(reason)
			return nil
		}
	}
	return fmt.Errorf("SDK token not found or already revoked")
}
