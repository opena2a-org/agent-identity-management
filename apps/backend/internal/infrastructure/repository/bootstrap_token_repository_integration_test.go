//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BootstrapTokenRepository against PostgreSQL (migration 115). The single-use
// and tenant-scoping rules live in the SQL WHERE clauses, which a mock
// replaces, so they are pinned here.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestBootstrapTokenRepository ./internal/infrastructure/repository/...

func bootstrapTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping bootstrap token repository test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// bootstrapSeedOrgUser inserts one organization and one user in it.
func bootstrapSeedOrgUser(t *testing.T, db *sql.DB) (orgID, userID uuid.UUID) {
	t.Helper()
	orgID, userID = uuid.New(), uuid.New()
	suffix := orgID.String()[:8]
	_, err := db.Exec(`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())`, orgID, "bootstrap-org-"+suffix, "bootstrap-"+suffix+".example.invalid")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, organization_id, email, name, password_hash, role,
			provider, provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, 'bootstrap user', 'x', 'member', 'local', $4, NOW(), NOW())`,
		userID, orgID, "bootstrap-"+suffix+"@example.invalid", "local-"+suffix)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
	})
	return orgID, userID
}

func bootstrapMint(t *testing.T, repo *BootstrapTokenRepository, orgID, userID uuid.UUID, now time.Time) (string, *domain.BootstrapToken) {
	t.Helper()
	plaintext, hash, prefix, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)
	tok := &domain.BootstrapToken{
		OrganizationID: orgID, CreatedBy: userID, TokenHash: hash, DisplayPrefix: prefix,
		Scope: domain.BootstrapTokenScopeAgentsRegister, CreatedAt: now, ExpiresAt: now.Add(domain.BootstrapTokenTTL),
	}
	require.NoError(t, repo.CreateReplacingOpen(context.Background(), tok, now))
	return plaintext, tok
}

func bootstrapNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func TestBootstrapTokenRepository_StoresNoPlaintext(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewBootstrapTokenRepository(db)
	orgID, userID := bootstrapSeedOrgUser(t, db)
	plaintext, tok := bootstrapMint(t, repo, orgID, userID, bootstrapNow())

	var row string
	require.NoError(t, db.QueryRow(`SELECT row_to_json(b)::text FROM bootstrap_tokens b WHERE id = $1`, tok.ID).Scan(&row))
	secret := strings.TrimPrefix(plaintext, domain.BootstrapTokenPrefix)
	assert.NotContains(t, row, plaintext)
	assert.NotContains(t, row, secret[:9])
	assert.Contains(t, row, domain.HashBootstrapToken(plaintext))
	assert.Contains(t, row, `"display_prefix":"`+secret[:8]+`"`)
}

func TestBootstrapTokenRepository_ClaimIsSingleUseAndRespectsExpiry(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewBootstrapTokenRepository(db)
	ctx := context.Background()
	orgID, userID := bootstrapSeedOrgUser(t, db)
	now := bootstrapNow()
	_, tok := bootstrapMint(t, repo, orgID, userID, now)

	ok, err := repo.Claim(ctx, tok.ID, now.Add(domain.BootstrapTokenTTL))
	require.NoError(t, err)
	assert.False(t, ok, "expired at exactly the TTL")

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := repo.Claim(ctx, tok.ID, now.Add(time.Minute)); err == nil && ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), wins.Load())

	got, err := repo.GetByHash(ctx, tok.TokenHash)
	require.NoError(t, err)
	assert.ErrorIs(t, got.CheckUsable(now.Add(time.Minute)), domain.ErrBootstrapTokenUsed)

	// Released claim reopens the token; a token with an agent stays used.
	require.NoError(t, repo.ReleaseClaim(ctx, tok.ID, now.Add(time.Minute)))
	ok, err = repo.Claim(ctx, tok.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBootstrapTokenRepository_MintRevokesOnlyTheCallersOpenToken(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewBootstrapTokenRepository(db)
	ctx := context.Background()
	orgA, userA := bootstrapSeedOrgUser(t, db)
	orgB, userB := bootstrapSeedOrgUser(t, db)
	now := bootstrapNow()

	_, first := bootstrapMint(t, repo, orgA, userA, now)
	_, other := bootstrapMint(t, repo, orgB, userB, now)
	_, second := bootstrapMint(t, repo, orgA, userA, now.Add(time.Second))

	got, err := repo.GetByHash(ctx, first.TokenHash)
	require.NoError(t, err)
	assert.ErrorIs(t, got.CheckUsable(now), domain.ErrBootstrapTokenRevoked)
	for _, tok := range []*domain.BootstrapToken{second, other} {
		got, err := repo.GetByHash(ctx, tok.TokenHash)
		require.NoError(t, err)
		assert.NoError(t, got.CheckUsable(now.Add(time.Second)))
	}

	// Org B's user revoking in org A, or org A's user id presented in org B,
	// touches nothing in org A.
	n, err := repo.RevokeOpenForUser(ctx, orgA, userB, now)
	require.NoError(t, err)
	assert.Zero(t, n)
	n, err = repo.RevokeOpenForUser(ctx, orgB, userA, now)
	require.NoError(t, err)
	assert.Zero(t, n)

	n, err = repo.RevokeOpenForUser(ctx, orgA, userA, now.Add(2*time.Second))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	got, err = repo.GetByHash(ctx, other.TokenHash)
	require.NoError(t, err)
	assert.Nil(t, got.RevokedAt, "org B's token is untouched")
}

func TestBootstrapTokenRepository_OneOpenTokenPerUserIsEnforcedBySchema(t *testing.T) {
	db := bootstrapTestDB(t)
	orgID, userID := bootstrapSeedOrgUser(t, db)
	now := bootstrapNow()
	insert := `INSERT INTO bootstrap_tokens (organization_id, created_by, token_hash, display_prefix, expires_at)
		VALUES ($1, $2, $3, 'abcdefgh', $4)`
	_, err := db.Exec(insert, orgID, userID, strings.Repeat("a", 64), now.Add(time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(insert, orgID, userID, strings.Repeat("b", 64), now.Add(time.Minute))
	assert.Error(t, err, "a second open token for the same user must violate the partial unique index")
}

func TestBootstrapTokenRepository_GetByHashUnknown(t *testing.T) {
	db := bootstrapTestDB(t)
	repo := NewBootstrapTokenRepository(db)
	_, err := repo.GetByHash(context.Background(), strings.Repeat("0", 64))
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenNotFound)
}
