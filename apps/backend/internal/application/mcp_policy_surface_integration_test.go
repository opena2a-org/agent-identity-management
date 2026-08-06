//go:build integration

package application

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// seedingMigration is the migration that introduced the default MCP security policies.
const seedingMigration = "052_add_default_mcp_policies.sql"

// mcpPolicyTypes are the policy types no running code path evaluates. SecurityPolicyService,
// the service constructed in cmd/server/main.go, only ever asks policyRepo.GetByType for the
// six non-MCP types; MCPPolicyEvaluator is the sole implementation of these four and has no
// non-test call site in either repo.
var mcpPolicyTypes = []string{"mcp_allowlist", "mcp_blocklist", "mcp_capabilities", "mcp_unverified"}

// seededPolicyNames are the six policies migration 052 creates. Matching on name as well as type
// keeps this guard off an operator's own MCP policies, which migration 105 deliberately does not
// touch either.
var seededPolicyNames = []string{
	"Trusted MCP Server Domains",
	"Blocked MCP Domains",
	"MCP Capability Requirements",
	"Unverified MCP Server Restrictions",
	"MCP Minimum Trust Score",
	"High-Risk MCP Server Block",
}

// TestSeededMCPPoliciesAreNotEnabled guards the invariant that a security policy is never shipped
// enabled while nothing evaluates it.
//
// Migration 052 seeded six MCP policies with is_enabled = true, two of them 'block_and_alert' at
// severity 'critical'. Because the admin security-policy page is mounted in production, an
// administrator saw enabled blocking policies that had never run. Migration 105 disables them.
//
// The test applies every migration that touches security_policies, in order, against a real
// database inside a transaction that is always rolled back. It asserts the pre-105 state
// explicitly (six seeded policies, all enabled) before asserting the post-105 state (none
// enabled), so it cannot pass vacuously on a database where the seeding never happened -- which
// is exactly how this would silently rot, since migration 052 no-ops unless the admin
// organization and admin user both exist.
//
// It fails if a future migration re-enables these rows. That is the intent: re-enabling is only
// correct once MCPPolicyEvaluator is wired AND rules.minTrustScore is rescaled to [0,1] AND the
// bare '*' allowedDomains entry is fixed. See https://github.com/opena2a-org/agent-identity-management/issues/355.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSeededMCPPoliciesAreNotEnabled ./internal/application/...
func TestSeededMCPPoliciesAreNotEnabled(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping MCP policy surface integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	through, after := securityPolicyMigrations(t)
	require.Contains(t, through, seedingMigration,
		"the seeding migration must be among the files applied before the mid-state assertion")
	require.NotEmpty(t, after,
		"no migration after %s touches security_policies; migration 105 should", seedingMigration)

	tx, err := db.Begin()
	require.NoError(t, err)
	// Always roll back. This test asserts over migration SQL, not over durable state, and must
	// not leave the seeded policies behind on a shared database.
	defer func() { _ = tx.Rollback() }()

	// Migration 052 looks the admin organization and admin user up by name and email, and
	// silently no-ops when either is missing. Create them so the seeding actually runs; without
	// this the whole test would pass while asserting nothing.
	adminOrgID := ensureRow(t, tx,
		`SELECT id FROM organizations WHERE name = 'OpenA2A Admin' LIMIT 1`,
		`INSERT INTO organizations (name, domain) VALUES ('OpenA2A Admin', 'mcp-policy-guard.invalid') RETURNING id`)
	ensureRow(t, tx,
		`SELECT id FROM users WHERE email = 'admin@opena2a.org' LIMIT 1`,
		`INSERT INTO users (organization_id, email, name, provider, provider_id)
		 VALUES ($1, 'admin@opena2a.org', 'Guard Admin', 'local', 'mcp-policy-guard')
		 RETURNING id`, adminOrgID)

	// Start from a clean slate for these six rows. Migration 052 inserts ON CONFLICT DO NOTHING,
	// so on a database where it has already run -- and where 105 has therefore already disabled
	// the rows -- re-applying it would insert nothing and the mid-state assertion below would
	// read the post-fix state as if it were the pre-fix one. Deleting first makes the test
	// deterministic on a fresh database and on a fully migrated one alike. Rolled back either way.
	_, err = tx.Exec(`DELETE FROM security_policies WHERE policy_type = ANY($1) AND name = ANY($2)`,
		pq.Array(mcpPolicyTypes), pq.Array(seededPolicyNames))
	require.NoError(t, err)

	for _, name := range through {
		applyMigration(t, tx, name)
	}

	// Pre-105 state, asserted rather than assumed: this is the defect the fix closes, and
	// asserting it here is what proves the post-fix assertion below is not vacuous.
	total, enabled := countSeededMCPPolicies(t, tx)
	require.Equal(t, 6, total,
		"migration %s should seed 6 MCP policies; got %d. If the seed changed, update this guard.",
		seedingMigration, total)
	require.Equal(t, 6, enabled,
		"expected all 6 seeded MCP policies enabled before the disabling migration runs")

	for _, name := range after {
		applyMigration(t, tx, name)
	}

	total, enabled = countSeededMCPPolicies(t, tx)
	require.Equal(t, 6, total, "the disabling migration must not delete the seeded policies, only disable them")
	require.Equal(t, 0, enabled,
		"a security policy of a type nothing evaluates is enabled. Either wire MCPPolicyEvaluator "+
			"(and rescale rules.minTrustScore to [0,1] and fix the bare '*' allowedDomains entry "+
			"first), or keep these rows disabled. See https://github.com/opena2a-org/agent-identity-management/issues/355")
}

// securityPolicyMigrations returns the migration filenames that touch security_policies, split
// at the seeding migration: everything up to and including it, then everything after. Derived
// from the directory rather than hardcoded, so a future migration that re-enables these rows is
// picked up automatically instead of slipping past a stale list.
func securityPolicyMigrations(t *testing.T) (through, after []string) {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, entries, "no migration files found")

	var matched []string
	for _, path := range entries {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		if strings.Contains(string(body), "security_policies") {
			matched = append(matched, filepath.Base(path))
		}
	}
	require.NotEmpty(t, matched, "no migration mentions security_policies")
	sort.Strings(matched) // zero-padded three-digit prefixes sort lexically in numeric order

	seen := false
	for _, name := range matched {
		if seen {
			after = append(after, name)
			continue
		}
		through = append(through, name)
		if name == seedingMigration {
			seen = true
		}
	}
	return through, after
}

func applyMigration(t *testing.T, tx *sql.Tx, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	require.NoError(t, err)
	_, err = tx.Exec(string(body))
	require.NoErrorf(t, err, "applying %s", name)
}

// ensureRow returns the id from selectSQL, inserting via insertSQL first if there is no row.
// insertArgs are bound as query parameters rather than interpolated, so no identifier read back
// out of the database is ever concatenated into SQL text.
func ensureRow(t *testing.T, tx *sql.Tx, selectSQL, insertSQL string, insertArgs ...any) string {
	t.Helper()
	var id string
	err := tx.QueryRow(selectSQL).Scan(&id)
	if err == sql.ErrNoRows {
		require.NoError(t, tx.QueryRow(insertSQL, insertArgs...).Scan(&id))
		return id
	}
	require.NoError(t, err)
	return id
}

func countSeededMCPPolicies(t *testing.T, tx *sql.Tx) (total, enabled int) {
	t.Helper()
	err := tx.QueryRow(`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_enabled)
		FROM security_policies
		WHERE policy_type = ANY($1)
		  AND name = ANY($2)`, pq.Array(mcpPolicyTypes), pq.Array(seededPolicyNames)).Scan(&total, &enabled)
	require.NoError(t, err)
	return total, enabled
}
