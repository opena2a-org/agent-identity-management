//go:build integration

package main

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// scripts/quickstart.sh and the fleet governance guide start the server with
// ADMIN_PASSWORD set and never run aim-bootstrap: the server's own seed
// creates the first administrator, after the startup migrations. Migrations
// 015 and 052 add the OpenA2A Admin organization's default security policies
// only when admin@opena2a.org already exists, so on such a database they add
// none. This test does what such a first start does, on an empty database it
// creates, and reads the policies the administrator's organization holds.
//
// Build-tag gated: requires TEST_DATABASE_URL, a postgres:// URL for a role
// that may create databases. The test creates its database on that server and
// drops it when it ends.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSeedAdmin_ ./cmd/server/...

// emptySeedTestDatabase creates an empty database on the server
// TEST_DATABASE_URL names, and drops it when the test ends.
func emptySeedTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping the first administrator seed test")
	}
	owner, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = owner.Close() })
	require.NoError(t, owner.Ping())

	name := "aim_seed_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	_, err = owner.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name))
	require.NoError(t, err, "the role TEST_DATABASE_URL names must be able to create a database")

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = owner.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(name) + ` WITH (FORCE)`)
	})
	require.NoError(t, db.Ping())
	return db
}

// seedTestMigrationPolicy matches the name a security_policies INSERT in
// migrations 015 and 052 gives its policy: the string after admin_org_id.
var seedTestMigrationPolicy = regexp.MustCompile(`admin_org_id,\s*'([^']+)'`)

// seedTestMigrationPolicyNames returns the names of the policies a migration
// inserts, read from migrations/ in the working directory.
func seedTestMigrationPolicyNames(t *testing.T, file string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("migrations", file))
	require.NoError(t, err)
	names := make([]string, 0)
	for _, m := range seedTestMigrationPolicy.FindAllStringSubmatch(string(body), -1) {
		names = append(names, m[1])
	}
	require.NotEmpty(t, names, "%s inserts no policy this test can read", file)
	return names
}

type seedTestPolicy struct {
	Name      string
	Type      string
	Enabled   bool
	OrgDomain string
	CreatedBy string
}

// seedTestPolicies returns every security policy in the database, sorted by
// name.
func seedTestPolicies(t *testing.T, db *sql.DB) []seedTestPolicy {
	t.Helper()
	rows, err := db.Query(`
		SELECT p.name, p.policy_type, p.is_enabled, o.domain, p.created_by
		FROM security_policies p
		JOIN organizations o ON o.id = p.organization_id`)
	require.NoError(t, err)
	defer rows.Close()
	policies := make([]seedTestPolicy, 0)
	for rows.Next() {
		var p seedTestPolicy
		require.NoError(t, rows.Scan(&p.Name, &p.Type, &p.Enabled, &p.OrgDomain, &p.CreatedBy))
		policies = append(policies, p)
	}
	require.NoError(t, rows.Err())
	sort.Slice(policies, func(i, j int) bool { return policies[i].Name < policies[j].Name })
	return policies
}

// On an empty database, after the migrations the server applies at startup,
// the ADMIN_PASSWORD seed that creates the first administrator adds the
// policies migrations 015 and 052 define to that administrator's
// organization, created by that administrator, with the MCP policies
// disabled as migration 105 leaves them. A second start finds the
// administrator and adds none.
func TestSeedAdmin_EmptyDatabaseGetsTheMigrationsDefaultPoliciesOnce(t *testing.T) {
	db := emptySeedTestDatabase(t)
	// runMigrations reads migrations/ from the working directory, which for
	// the server is apps/backend.
	t.Chdir(filepath.Join("..", ".."))
	require.NoError(t, runMigrations(db))

	var before int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM security_policies`).Scan(&before))
	require.Zero(t, before, "the migrations left policies on a database with no user")

	admin := config.AdminSeedConfig{Email: "admin@opena2a.org", Password: seedTestPassword, Name: "System Administrator"}
	require.NoError(t, seedAdmin(db, admin))

	var adminID string
	require.NoError(t, db.QueryRow(`
		SELECT u.id FROM users u JOIN organizations o ON o.id = u.organization_id
		WHERE o.domain = $1 AND u.email = $2 AND u.role = 'admin'`,
		adminSeedOrgDomain, admin.Email).Scan(&adminID), "the seed created no administrator")

	want := append(seedTestMigrationPolicyNames(t, "015_add_default_security_policies.sql"),
		seedTestMigrationPolicyNames(t, "052_add_default_mcp_policies.sql")...)
	sort.Strings(want)

	first := seedTestPolicies(t, db)
	names := make([]string, 0, len(first))
	for _, p := range first {
		names = append(names, p.Name)
	}
	require.Equal(t, want, names, "the policies after the first start's ADMIN_PASSWORD seed")
	for _, p := range first {
		require.Equal(t, adminSeedOrgDomain, p.OrgDomain, "policy %q is not in the administrator's organization", p.Name)
		require.Equal(t, adminID, p.CreatedBy, "policy %q is not attributed to the administrator the seed created", p.Name)
		require.Equal(t, !strings.HasPrefix(p.Type, "mcp_"), p.Enabled,
			"policy %q (%s): migration 105 leaves the MCP policies disabled and the others enabled", p.Name, p.Type)
	}

	require.NoError(t, seedAdmin(db, admin))
	require.Equal(t, first, seedTestPolicies(t, db), "a second start changed the policies")
}
