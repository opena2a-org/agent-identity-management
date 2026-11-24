//go:build integration

package repository

import (
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An agent's name is unique in its organization, and the database holds that
// line, not a read before the insert. Two registrations of one name that race
// both pass any such read; the UNIQUE (organization_id, name) constraint of
// migration 001 stores one and refuses the other, which Create reports as
// ErrAgentNameTaken and the registration handlers answer with 409.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAgentNameUnique ./internal/infrastructure/repository/...

func agentNameUniqueTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping agent name uniqueness test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedAgentNameOrg inserts one organization and one admin user and returns
// their ids. Every agent of that organization is removed on cleanup.
func seedAgentNameOrg(t *testing.T, db *sql.DB) (orgID, userID uuid.UUID) {
	t.Helper()

	orgID, userID = uuid.New(), uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM agents WHERE organization_id = $1`, orgID)
		_, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.Exec(
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "agentname-org-"+suffix, "agentname-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.Exec(
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "agentname-"+suffix+"@example.com", "agentname-user", "local-"+suffix)
	require.NoError(t, err)

	return orgID, userID
}

func newNamedAgent(orgID, userID uuid.UUID, name string) *domain.Agent {
	return &domain.Agent{
		OrganizationID: orgID,
		Name:           name,
		DisplayName:    name,
		AgentType:      domain.AgentTypeAI,
		CreatedBy:      userID,
	}
}

func TestAgentNameUniqueConcurrentCreatesStoreOne(t *testing.T) {
	db := agentNameUniqueTestDB(t)
	repo := NewAgentRepository(db)
	orgID, userID := seedAgentNameOrg(t, db)
	const name = "same-name-agent"

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = repo.Create(newNamedAgent(orgID, userID, name))
		}(i)
	}
	close(start)
	wg.Wait()

	stored, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			stored++
		case assert.ErrorIs(t, err, ErrAgentNameTaken):
			refused++
		}
	}
	assert.Equal(t, 1, stored, "exactly one of the two creates is stored")
	assert.Equal(t, 1, refused, "the other create is refused as a name conflict")

	var count int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM agents WHERE organization_id = $1 AND name = $2`, orgID, name).Scan(&count))
	assert.Equal(t, 1, count)
}

// The name is unique per organization only: another organization may use it.
func TestAgentNameUniqueIsPerOrganization(t *testing.T) {
	db := agentNameUniqueTestDB(t)
	repo := NewAgentRepository(db)
	orgA, userA := seedAgentNameOrg(t, db)
	orgB, userB := seedAgentNameOrg(t, db)

	require.NoError(t, repo.Create(newNamedAgent(orgA, userA, "shared-name")))
	require.NoError(t, repo.Create(newNamedAgent(orgB, userB, "shared-name")))
}
