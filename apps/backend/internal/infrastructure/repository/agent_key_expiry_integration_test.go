//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AgentRepository.SuspendAgentsWithExpiredKeys (#359).
//
// Key-expiry enforcement used to be a loop over List followed by Update. List does not
// select key_expires_at, so the loop never suspended anything; selecting it would have
// made the loop reachable and Update would then have cleared every column List does not
// read, including the encrypted private key, the PQC key and the capability grant. These
// tests pin the replacement against a real database, because both defects lived in SQL
// that a mock replaces.
//
// The cutoff is a fixed instant in 1990 and every fixture is placed relative to it, so
// the UPDATE, which is table-wide by design, can only match rows this test seeded. A
// shared database with other suites' agents is left untouched.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSuspendAgentsWithExpiredKeys ./internal/infrastructure/repository/...

var keyExpiryCutoff = time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

func keyExpiryTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping key expiry enforcement test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

type keyExpiryFixture struct {
	status    string
	expiresAt *time.Time
	graceTo   *time.Time
}

func at(d time.Duration) *time.Time {
	v := keyExpiryCutoff.Add(d)
	return &v
}

// seedKeyExpiryAgents inserts one agent per fixture, each carrying full key material, and
// returns their ids in fixture order.
func seedKeyExpiryAgents(t *testing.T, db *sql.DB, fixtures []keyExpiryFixture) []uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID, orgID := uuid.New(), uuid.New()
	suffix := orgID.String()[:8]
	ids := make([]uuid.UUID, len(fixtures))
	for i := range ids {
		ids[i] = uuid.New()
	}

	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, id)
		}
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "keyexpiry-org-"+suffix, "keyexpiry-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "keyexpiry-"+suffix+"@example.com", "keyexpiry-user", "local-"+suffix)
	require.NoError(t, err)

	for i, f := range fixtures {
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status,
			                     public_key, encrypted_private_key, key_algorithm,
			                     key_created_at, key_expires_at, key_rotation_grace_until,
			                     previous_public_key, rotation_count, capabilities,
			                     pqc_public_key, pqc_key_algorithm, hybrid_mode_enabled,
			                     created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'ai_agent', $5,
			         'PUB', 'SECRET-ENCRYPTED-KEY', 'Ed25519',
			         $6, $7, $8,
			         'PREV-PUB', 7, '["cap:one","cap:two"]'::jsonb,
			         'PQC-PUBLIC-KEY', 'ML-DSA-65', true,
			         $9, NOW(), NOW())`,
			ids[i], orgID, "keyexpiry-agent-"+ids[i].String()[:8], "Key Expiry Agent",
			f.status, keyExpiryCutoff.Add(-365*24*time.Hour), f.expiresAt, f.graceTo, userID)
		require.NoError(t, err, "seeding fixture %d", i)
	}
	return ids
}

func agentStatus(t *testing.T, db *sql.DB, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM agents WHERE id = $1`, id).Scan(&status))
	return status
}

func TestSuspendAgentsWithExpiredKeys(t *testing.T) {
	db := keyExpiryTestDB(t)
	repo := NewAgentRepository(db)

	fixtures := []keyExpiryFixture{
		{status: "verified", expiresAt: at(-time.Hour)},                           // 0 expired, no grace window
		{status: "verified", expiresAt: at(-time.Hour), graceTo: at(time.Hour)},   // 1 expired, inside its grace window
		{status: "pending", expiresAt: at(-time.Hour), graceTo: at(-time.Minute)}, // 2 expired, grace window closed
		{status: "verified", expiresAt: at(time.Hour)},                            // 3 not yet expired
		{status: "verified"},                                            // 4 no expiry recorded
		{status: "revoked", expiresAt: at(-time.Hour)},                  // 5 expired but revoked
		{status: "suspended", expiresAt: at(-time.Hour)},                // 6 expired, already suspended
		{status: "verified", expiresAt: at(0)},                          // 7 expires exactly at the cutoff
		{status: "verified", expiresAt: at(-time.Hour), graceTo: at(0)}, // 8 grace ends exactly at the cutoff
	}
	ids := seedKeyExpiryAgents(t, db, fixtures)

	returned, err := repo.SuspendAgentsWithExpiredKeys(keyExpiryCutoff)
	require.NoError(t, err)

	seeded := map[uuid.UUID]int{}
	for i, id := range ids {
		seeded[id] = i
	}
	var mine []int
	for _, id := range returned {
		if i, ok := seeded[id]; ok {
			mine = append(mine, i)
		}
	}
	assert.ElementsMatch(t, []int{0, 2, 8}, mine,
		"returned the wrong fixtures (indexes into the table above)")

	want := []string{
		"suspended", "verified", "suspended", "verified", "verified",
		"revoked", "suspended", "verified", "suspended",
	}
	for i, id := range ids {
		assert.Equal(t, want[i], agentStatus(t, db, id), "fixture %d", i)
	}

	// The reason this is a narrow UPDATE: suspending must not touch the key material.
	var (
		encryptedKey, keyAlgorithm, previousKey, pqcKey, pqcAlgorithm, capabilities string
		rotationCount                                                               int
		hybrid                                                                      bool
		keyCreatedAt, keyExpiresAt                                                  sql.NullTime
	)
	require.NoError(t, db.QueryRow(
		`SELECT encrypted_private_key, key_algorithm, previous_public_key, pqc_public_key,
		        pqc_key_algorithm, capabilities::text, rotation_count, hybrid_mode_enabled,
		        key_created_at, key_expires_at
		 FROM agents WHERE id = $1`, ids[0]).Scan(
		&encryptedKey, &keyAlgorithm, &previousKey, &pqcKey, &pqcAlgorithm, &capabilities,
		&rotationCount, &hybrid, &keyCreatedAt, &keyExpiresAt))
	assert.Equal(t, "SECRET-ENCRYPTED-KEY", encryptedKey)
	assert.Equal(t, "Ed25519", keyAlgorithm)
	assert.Equal(t, "PREV-PUB", previousKey)
	assert.Equal(t, "PQC-PUBLIC-KEY", pqcKey)
	assert.Equal(t, "ML-DSA-65", pqcAlgorithm)
	assert.JSONEq(t, `["cap:one","cap:two"]`, capabilities)
	assert.Equal(t, 7, rotationCount)
	assert.True(t, hybrid)
	assert.True(t, keyCreatedAt.Valid, "key_created_at was cleared")
	assert.True(t, keyExpiresAt.Valid, "key_expires_at was cleared")

	// A second run finds nothing new: the suspended rows are excluded by status.
	again, err := repo.SuspendAgentsWithExpiredKeys(keyExpiryCutoff)
	require.NoError(t, err)
	for _, id := range again {
		_, ok := seeded[id]
		assert.False(t, ok, "fixture %d suspended twice", seeded[id])
	}
}
