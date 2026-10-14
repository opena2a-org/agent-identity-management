package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An SDK-download token family lives at most SDK_TOKEN_FAMILY_MAX_AGE (8760h
// by default) from the download, however often it rotates. The birth is the
// created_at of the row the download wrote, carried to every successor row as
// familyCreatedAt; a row without it is read as its own created_at. A refresh
// past the bound is answered as an expired token, before the account lookup,
// the presented row is revoked for the family's age, nothing is minted, and
// the refusal is recorded once with identifiers only. Login refresh tokens
// are outside the bound.

// agedChain downloads an SDK on the fixture and moves the download row's
// creation back by age, so the chain was born that long ago.
func agedChain(t *testing.T, f *sdkFamilyFixture, age time.Duration) string {
	t.Helper()
	s1 := f.downloadSDK(t)
	f.repo.row(s1).CreatedAt = time.Now().Add(-age)
	return s1
}

// B1: a chain born 366 days ago is refused with the expiry answer, no
// successor row is written, the presented row is revoked for the family's
// age, and the refusal is recorded once with identifiers only. A chain born
// 364 days ago refreshes and its successor carries the root's birth.
func TestRefreshToken_SDKFamilyPastMaxAgeIsRefusedAndItsRowRevoked(t *testing.T) {
	logs := captureLog(t)
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := agedChain(t, f, 366*24*time.Hour)
	family := jtiOfToken(t, f.svc, s1)
	logs.Reset()

	body, status := postRefreshUA(t, f.refresh, s1)
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, sessionExpiredBody, body, "the answer is the expired token's, byte for byte")
	assert.Len(t, f.repo.rows, 1, "no successor row")
	assert.True(t, f.repo.row(s1).RevokedFor(domain.SDKTokenRevokeReasonFamilyMaxAge), "the presented row is revoked for the family's age")
	assert.Equal(t, 0, f.repo.activeRows())
	assert.Equal(t, 0, familyKeyWrites(f.store), "no family key: the row is the record")
	require.Len(t, f.audit.rows, 1, "one audit row")
	row := f.audit.rows[0]
	assert.Equal(t, domain.AuditActionRefreshFamilyMaxAge, row.Action)
	assert.Equal(t, family, row.Metadata["familyId"])
	assert.Equal(t, familyTestUA, row.UserAgent)
	assert.NotContains(t, marshalled(t, row.Metadata), "eyJ", "no token in the row")
	logged := logs.String()
	assert.Equal(t, 1, strings.Count(logged, "SECURITY refresh_family_max_age"))
	assert.Contains(t, logged, "family="+family)
	assert.Contains(t, logged, "familyCreatedAt=")
	assert.NotContains(t, logged, s1, "the token is never logged")

	// Control: a chain born 364 days ago refreshes, and the successor row
	// carries the root's birth.
	s2 := agedChain(t, f, 364*24*time.Hour)
	root := f.repo.row(s2)
	out, status := postRefresh(t, f.refresh, s2)
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Rotated)
	assert.True(t, root.CreatedAt.Equal(f.repo.row(out.RefreshToken).FamilyCreatedAt()), "the successor carries the root's created_at as its birth")
	assert.Len(t, f.audit.rows, 1, "a refresh inside the bound records nothing")
}

// B3: a row without a birth (the download's row, or a row from before the
// birth was carried) is read as its own created_at, and the first rotation
// writes the birth. An unreadable birth is read the same way: an absent
// value never lengthens the family's life.
func TestRefreshToken_SDKRowWithoutABirthIsReadAsItsCreatedAt(t *testing.T) {
	t.Setenv("SDK_TOKEN_FAMILY_MAX_AGE", "1h")
	f := newSDKFamilyFixture(t, &rotationStore{})

	s1 := agedChain(t, f, 30*time.Minute)
	root := f.repo.row(s1)
	_, has := root.Metadata[domain.SDKTokenMetadataFamilyCreatedAt]
	require.False(t, has, "the download row carries no birth")
	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status, "30 minutes old is inside a 1h bound")
	raw, ok := f.repo.row(out.RefreshToken).Metadata[domain.SDKTokenMetadataFamilyCreatedAt].(string)
	require.True(t, ok, "the first rotation writes the birth")
	born, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err)
	assert.True(t, born.Equal(root.CreatedAt), "the birth written is the root's created_at")

	s2 := agedChain(t, f, 2*time.Hour)
	body, status := postRefreshRaw(t, f.refresh, s2)
	require.Equal(t, fiber.StatusUnauthorized, status, "a row without a birth created 2h ago is past a 1h bound")
	assert.Equal(t, sessionExpiredBody, body)

	s3 := agedChain(t, f, 2*time.Hour)
	f.repo.row(s3).Metadata[domain.SDKTokenMetadataFamilyCreatedAt] = "not a time"
	body, status = postRefreshRaw(t, f.refresh, s3)
	require.Equal(t, fiber.StatusUnauthorized, status, "an unreadable birth is read as the row's created_at")
	assert.Equal(t, sessionExpiredBody, body)
}

// B4: the bound comes from SDK_TOKEN_FAMILY_MAX_AGE; a value that is not a
// positive duration warns and the default (8760h) is in force.
func TestRefreshToken_SDKFamilyMaxAgeComesFromTheEnvironment(t *testing.T) {
	t.Setenv("SDK_TOKEN_FAMILY_MAX_AGE", "1h")
	f := newSDKFamilyFixture(t, &rotationStore{})
	body, status := postRefreshRaw(t, f.refresh, agedChain(t, f, 2*time.Hour))
	require.Equal(t, fiber.StatusUnauthorized, status, "a chain 2h old is past a 1h bound")
	assert.Equal(t, sessionExpiredBody, body)

	logs := captureLog(t)
	t.Setenv("SDK_TOKEN_FAMILY_MAX_AGE", "soon")
	g := newSDKFamilyFixture(t, &rotationStore{})
	assert.Contains(t, logs.String(), "WARNING: SDK_TOKEN_FAMILY_MAX_AGE must be a positive duration, using default 8760h")
	assert.Equal(t, 8760*time.Hour, g.svc.SDKTokenFamilyMaxAge())
	_, status = postRefresh(t, g.refresh, agedChain(t, g, 2*time.Hour))
	assert.Equal(t, fiber.StatusOK, status, "under the default a chain 2h old refreshes")
}

// B5: a login refresh token of the same age is outside the bound; an SDK
// chain of that age on the same app is refused.
func TestRefreshToken_SDKFamilyMaxAgeLeavesLoginTokensAlone(t *testing.T) {
	t.Setenv("SDK_TOKEN_FAMILY_MAX_AGE", "1h")
	f := newSDKFamilyFixture(t, &rotationStore{})
	signedIn := time.Now().Add(-2 * time.Hour)
	login := signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, uuid.New().String(), signedIn, signedIn))
	out, status := postRefresh(t, f.refresh, login)
	require.Equal(t, fiber.StatusOK, status, "a sign-in 2h old is inside JWT_SESSION_MAX_AGE and outside the SDK bound")
	assert.True(t, out.Rotated)

	body, status := postRefreshRaw(t, f.refresh, agedChain(t, f, 2*time.Hour))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, sessionExpiredBody, body)
}

// B6: across three rotations every row's birth is the root's created_at and
// never later than its parent's, and none is the row's own creation.
func TestRefreshToken_SDKRotationsCarryTheRootsBirthUnchanged(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	tok := agedChain(t, f, 100*24*time.Hour)
	root := f.repo.row(tok)
	parentBirth := root.FamilyCreatedAt()
	require.True(t, parentBirth.Equal(root.CreatedAt), "the root's birth is its own created_at")
	for i := 1; i <= 3; i++ {
		out, status := postRefresh(t, f.refresh, tok)
		require.Equal(t, fiber.StatusOK, status, "rotation %d", i)
		require.True(t, out.Rotated, "rotation %d", i)
		row := f.repo.row(out.RefreshToken)
		born := row.FamilyCreatedAt()
		assert.True(t, born.Equal(root.CreatedAt), "rotation %d: the birth is the root's created_at", i)
		assert.False(t, born.After(parentBirth), "rotation %d: never later than the parent's", i)
		assert.True(t, born.Before(row.CreatedAt), "rotation %d: the birth is not the row's own creation", i)
		parentBirth, tok = born, out.RefreshToken
	}
	assert.Len(t, f.repo.rows, 4)
}

// B7: the bound is checked before the account, so an aged chain whose owner
// can no longer hold a session gets the expiry answer, not the account's state.
func TestRefreshToken_SDKFamilyMaxAgeIsCheckedBeforeTheAccount(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	deactivated := activeUser(userID, orgID, domain.RoleAdmin, "gone-sdk@example.com")
	deactivated.Status = domain.UserStatusDeactivated
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) { return deactivated, nil }}
	repo := &rotationSDKRepo{token: &domain.SDKToken{
		ID: uuid.New(), UserID: userID, OrganizationID: orgID,
		CreatedAt: time.Now().Add(-366 * 24 * time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour),
	}}
	app, svc := newRefreshTestAppAudit(t, users, repo, &familyAuditRepo{})
	sdk, err := svc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "gone-sdk@example.com", "admin")
	require.NoError(t, err)

	body, status := postRefreshRaw(t, app, sdk)
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, sessionExpiredBody, body, "past the bound the answer is expiry, not the account's state")
	require.Len(t, repo.revoked, 1)
	assert.Equal(t, domain.SDKTokenRevokeReasonFamilyMaxAge, repo.revoked[0].reason)
	assert.Empty(t, repo.created)

	// Control: inside the bound the account is looked up.
	repo.token.CreatedAt = time.Now().Add(-364 * 24 * time.Hour)
	body, status = postRefreshRaw(t, app, sdk)
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, "Account is not active")
}
