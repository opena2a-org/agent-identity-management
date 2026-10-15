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

// A2ADelegationRepository against the a2a_delegation table of a migrated
// database: the column list, the defaults read back, the one-active-
// delegation-per-pair index and the organization predicate.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestA2ADelegationRepositoryIntegration ./internal/infrastructure/repository/...

// delegationIntegrationFixture stands up organization org with agents a and b
// and a user, and organization other with agent x. Everything it creates is
// removed on cleanup.
func delegationIntegrationFixture(t *testing.T, db *sql.DB) (org, other, user, a, b, x uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	org, other = uuid.New(), uuid.New()
	user, otherUser := uuid.New(), uuid.New()
	a, b, x = uuid.New(), uuid.New(), uuid.New()
	suffix := uuid.NewString()[:8]

	for _, o := range []struct {
		id   uuid.UUID
		name string
	}{{org, "delegation-test-a-" + suffix}, {other, "delegation-test-b-" + suffix}} {
		_, err := db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
			o.id, o.name, o.name+".invalid")
		require.NoError(t, err)
	}
	for _, u := range []struct{ id, org uuid.UUID }{{user, org}, {otherUser, other}} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, organization_id, email, name, provider, provider_id)
			 VALUES ($1, $2, $3, 'delegation test user', 'local', $4)`,
			u.id, u.org, "delegation-"+u.id.String()[:8]+"@example.invalid", u.id.String())
		require.NoError(t, err)
	}
	for _, ag := range []struct{ id, org, creator uuid.UUID }{{a, org, user}, {b, org, user}, {x, other, otherUser}} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id)
			 VALUES ($1, $2, $2, 'service', $3, $4)`,
			ag.id, "delegation-agent-"+ag.id.String()[:8], ag.creator, ag.org)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_delegation WHERE organization_id = ANY($1)`, pqUUIDArray(org, other))
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = ANY($1)`, pqUUIDArray(a, b, x))
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = ANY($1)`, pqUUIDArray(user, otherUser))
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = ANY($1)`, pqUUIDArray(org, other))
	})
	return org, other, user, a, b, x
}

func TestA2ADelegationRepositoryIntegration(t *testing.T) {
	db := a2aScopeTestDB(t)
	ctx := context.Background()
	repo := NewA2ADelegationRepository(db)
	org, other, user, a, b, _ := delegationIntegrationFixture(t, db)

	validFrom := time.Now().UTC().Truncate(time.Microsecond)
	until := validFrom.Add(24 * time.Hour)
	d := &domain.A2ADelegation{
		OrganizationID:         org,
		DelegatorAgentID:       a,
		DelegateAgentID:        b,
		SkillIDs:               []string{"search", "summarize"},
		MaxChainDepth:          2,
		TrustAttenuationFactor: 0.5,
		MinDelegatedTrustScore: 0.4,
		CanDelegateFurther:     true,
		IsActive:               true,
		ValidFrom:              validFrom,
		ValidUntil:             &until,
		CreatedBy:              &user,
	}
	require.NoError(t, repo.Create(ctx, d))
	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.True(t, d.RequiresConsent, "requires_consent takes its column default")
	assert.False(t, d.CreatedAt.IsZero())

	t.Run("a second active delegation for the pair is refused", func(t *testing.T) {
		err := repo.Create(ctx, &domain.A2ADelegation{
			OrganizationID: org, DelegatorAgentID: a, DelegateAgentID: b,
			MaxChainDepth: 3, TrustAttenuationFactor: 0.8, MinDelegatedTrustScore: 0.3,
			IsActive: true, ValidFrom: validFrom,
		})
		assert.ErrorIs(t, err, domain.ErrA2ADelegationActive)
	})

	t.Run("the stored row reads back, attenuation fields included", func(t *testing.T) {
		found, err := repo.GetByIDs(ctx, org, []uuid.UUID{d.ID, uuid.New()})
		require.NoError(t, err)
		require.Len(t, found, 1)
		got := found[0]
		assert.Equal(t, d.ID, got.ID)
		assert.Equal(t, []string{"search", "summarize"}, got.SkillIDs)
		assert.Equal(t, 2, got.MaxChainDepth)
		assert.Equal(t, 0.5, got.TrustAttenuationFactor)
		assert.Equal(t, 0.4, got.MinDelegatedTrustScore)
		assert.True(t, got.CanDelegateFurther)
		assert.True(t, got.IsActive)
		assert.True(t, validFrom.Equal(got.ValidFrom))
		require.NotNil(t, got.ValidUntil)
		assert.True(t, until.Equal(*got.ValidUntil))
		require.NotNil(t, got.CreatedBy)
		assert.Equal(t, user, *got.CreatedBy)
	})

	t.Run("another organization reads nothing and revokes nothing", func(t *testing.T) {
		found, err := repo.GetByIDs(ctx, other, []uuid.UUID{d.ID})
		require.NoError(t, err)
		assert.Empty(t, found)

		listed, err := repo.ListByOrganization(ctx, other, 100, 0)
		require.NoError(t, err)
		assert.Empty(t, listed)

		_, err = repo.Revoke(ctx, other, d.ID)
		assert.ErrorIs(t, err, domain.ErrA2ADelegationNotFound)
		still, err := repo.GetByIDs(ctx, org, []uuid.UUID{d.ID})
		require.NoError(t, err)
		require.Len(t, still, 1)
		assert.True(t, still[0].IsActive)
	})

	t.Run("a revoked pair can be granted again, and the list is newest first", func(t *testing.T) {
		revoked, err := repo.Revoke(ctx, org, d.ID)
		require.NoError(t, err)
		assert.False(t, revoked.IsActive)

		again := &domain.A2ADelegation{
			OrganizationID: org, DelegatorAgentID: a, DelegateAgentID: b,
			MaxChainDepth: 3, TrustAttenuationFactor: 0.8, MinDelegatedTrustScore: 0.3,
			IsActive: true, ValidFrom: validFrom,
		}
		require.NoError(t, repo.Create(ctx, again))
		assert.Empty(t, again.SkillIDs)
		assert.Nil(t, again.CreatedBy)

		listed, err := repo.ListByOrganization(ctx, org, 100, 0)
		require.NoError(t, err)
		require.Len(t, listed, 2)
		assert.Equal(t, again.ID, listed[0].ID)
		assert.Equal(t, d.ID, listed[1].ID)

		page, err := repo.ListByOrganization(ctx, org, 1, 1)
		require.NoError(t, err)
		require.Len(t, page, 1)
		assert.Equal(t, d.ID, page[0].ID)
	})
}
