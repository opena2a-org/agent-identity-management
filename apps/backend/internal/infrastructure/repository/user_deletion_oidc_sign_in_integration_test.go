//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An organization's OIDC sign-in configuration is a system_config row, which
// has no reference to organizations, so no cascade removes it. The account
// deletion purge removes it with the organization it deletes. The configuration
// of an organization the purge keeps, because a user is left or because a
// reference refused the delete, stays.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserDeletionPurge ./internal/infrastructure/repository/...
func TestUserDeletionPurgeRemovesTheOIDCSignInConfigurationOfTheOrganizationItDeletes(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	configs := NewOIDCSignInConfigRepository(db)
	at := time.Now().UTC().Add(-70 * 24 * time.Hour).Truncate(time.Microsecond)

	purged := seedTenant(t, db, ctx, "deletion-oidc-purged")
	userDeletionPutOIDCSignInConfig(t, db, ctx, configs, purged)
	_, err := repo.RequestDeletion(ctx, purged.userID, at)
	require.NoError(t, err)

	stays := seedTenant(t, db, ctx, "deletion-oidc-stays")
	userDeletionPutOIDCSignInConfig(t, db, ctx, configs, stays)
	member := userDeletionSeedUser(t, db, ctx, stays.orgID, "member", "active")
	_, err = repo.RequestDeletion(ctx, member, at)
	require.NoError(t, err)

	chained := seedTenant(t, db, ctx, "deletion-oidc-chained")
	userDeletionPutOIDCSignInConfig(t, db, ctx, configs, chained)
	organizationDeleteInsertChain(t, db, ctx, chained.orgID)
	_, err = repo.RequestDeletion(ctx, chained.userID, at)
	require.NoError(t, err)

	_ = organizationDeleteCaptureLog(t)
	due, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)

	deleted := userDeletionsOfOrg(due, purged.orgID)
	require.Len(t, deleted, 1)
	require.True(t, deleted[0].OrganizationDeleted)
	assert.Equal(t, 0, userDeletionOIDCSignInConfigRows(t, db, ctx, purged.orgID),
		"the deleted organization's configuration is removed with it")
	_, err = configs.Get(ctx, purged.orgID)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured)

	kept := userDeletionsOfOrg(due, stays.orgID)
	require.Len(t, kept, 1)
	require.False(t, kept[0].OrganizationDeleted)
	assert.Equal(t, 1, userDeletionOIDCSignInConfigRows(t, db, ctx, stays.orgID),
		"an organization with a user left keeps its configuration")

	refused := userDeletionsOfOrg(due, chained.orgID)
	require.Len(t, refused, 1)
	require.False(t, refused[0].OrganizationDeleted)
	assert.Equal(t, 1, userDeletionOIDCSignInConfigRows(t, db, ctx, chained.orgID),
		"an organization whose delete was refused keeps its configuration")
}

func userDeletionPutOIDCSignInConfig(t *testing.T, db *sql.DB, ctx context.Context, configs *OIDCSignInConfigRepository, f tenantFixture) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM system_config WHERE key = $1`, oidcSignInConfigKey(f.orgID))
	})
	require.NoError(t, configs.Put(ctx, &domain.OIDCSignInConfig{
		OrganizationID:     f.orgID,
		Issuer:             "https://idp.example.test",
		ClientID:           "aim-dashboard",
		SealedClientSecret: "FAKE-SEALED-SECRET",
		UpdatedBy:          f.userID,
	}))
	require.Equal(t, 1, userDeletionOIDCSignInConfigRows(t, db, ctx, f.orgID))
}

func userDeletionOIDCSignInConfigRows(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM system_config WHERE key = $1`, oidcSignInConfigKey(orgID)).Scan(&n))
	return n
}
