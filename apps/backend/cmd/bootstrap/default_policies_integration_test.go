//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/seed"
)

// Migrations 015 and 052 insert the OpenA2A Admin organization's default
// security policies only when the admin@opena2a.org user already exists, and
// migration 013 no longer creates that user: `aim-bootstrap --default` does,
// after the migrations have run. These tests run the bootstrap against a
// database with every migration applied and read the policies it leaves.
//
// Build-tag gated: requires TEST_DATABASE_URL, a postgres:// URL of a
// database with every migration applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestDefaultBootstrap ./cmd/bootstrap/...

func bootstrapIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping bootstrap integration test")
	}
	db, err := openDatabase(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// keepBootstrapMarker puts the bootstrap_completed row of system_config back
// as it was when the test ends: runBootstrap sets it, and the database is
// shared with other tests.
func keepBootstrapMarker(t *testing.T, db *sql.DB) {
	t.Helper()
	var value string
	err := db.QueryRow(`SELECT value FROM system_config WHERE key = 'bootstrap_completed'`).Scan(&value)
	if err == sql.ErrNoRows {
		t.Cleanup(func() {
			_, _ = db.Exec(`DELETE FROM system_config WHERE key = 'bootstrap_completed'`)
		})
		return
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`UPDATE system_config SET value = $1 WHERE key = 'bootstrap_completed'`, value)
	})
}

// migrationPolicyName matches the name a security_policies INSERT in
// migrations 015 and 052 gives its policy: the string after admin_org_id.
var migrationPolicyName = regexp.MustCompile(`admin_org_id,\s*'([^']+)'`)

// migrationPolicyNames returns the names of the policies a migration inserts.
func migrationPolicyNames(t *testing.T, file string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", file))
	require.NoError(t, err)
	names := make([]string, 0)
	for _, m := range migrationPolicyName.FindAllStringSubmatch(string(body), -1) {
		names = append(names, m[1])
	}
	require.NotEmpty(t, names, "%s inserts no policy this test can read", file)
	return names
}

type storedPolicy struct {
	Name      string
	Type      string
	Enabled   bool
	CreatedBy string
}

// policiesOf returns the policies of the organization with the given domain,
// sorted by name.
func policiesOf(t *testing.T, db *sql.DB, orgDomain string) []storedPolicy {
	t.Helper()
	rows, err := db.Query(`
		SELECT p.name, p.policy_type, p.is_enabled, p.created_by
		FROM security_policies p
		JOIN organizations o ON o.id = p.organization_id
		WHERE o.domain = $1`, orgDomain)
	require.NoError(t, err)
	defer rows.Close()
	policies := make([]storedPolicy, 0)
	for rows.Next() {
		var p storedPolicy
		require.NoError(t, rows.Scan(&p.Name, &p.Type, &p.Enabled, &p.CreatedBy))
		policies = append(policies, p)
	}
	require.NoError(t, rows.Err())
	sort.Slice(policies, func(i, j int) bool { return policies[i].Name < policies[j].Name })
	return policies
}

func policyNames(policies []storedPolicy) []string {
	names := make([]string, 0, len(policies))
	for _, p := range policies {
		names = append(names, p.Name)
	}
	return names
}

// On a database where every migration has run, the --default run that
// creates the admin adds the policies migrations 015 and 052 define to the
// admin's organization, created by that admin, with the MCP policies disabled
// as migration 105 leaves them. A second --default run adds none.
func TestDefaultBootstrapAddsTheMigrationsDefaultPoliciesOnce(t *testing.T) {
	db := bootstrapIntegrationDB(t)
	keepBootstrapMarker(t, db)
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	newConfig := func() *BootstrapConfig {
		return &BootstrapConfig{
			AdminEmail:    "bootstrap-policies-" + suffix + "@example.com",
			AdminPassword: "FAKE-pass-1234",
			AdminName:     "System Administrator",
			OrgName:       "Bootstrap Policies " + suffix,
			OrgDomain:     "bootstrap-policies-" + suffix + ".example.com",
			MaxUsers:      100,
			MaxAgents:     1000,
			Default:       true,
		}
	}
	orgDomain := newConfig().OrgDomain
	t.Cleanup(func() {
		// The admin and the policies go with the organization (ON DELETE CASCADE).
		_, _ = db.Exec(`DELETE FROM organizations WHERE domain = $1`, orgDomain)
	})

	want := append(migrationPolicyNames(t, "015_add_default_security_policies.sql"),
		migrationPolicyNames(t, "052_add_default_mcp_policies.sql")...)
	sort.Strings(want)

	require.Empty(t, policiesOf(t, db, orgDomain), "the organization has policies before the bootstrap runs")

	var out bytes.Buffer
	require.NoError(t, runBootstrap(ctx, &out, db, newConfig()))

	var adminID string
	require.NoError(t, db.QueryRow(`
		SELECT u.id FROM users u JOIN organizations o ON o.id = u.organization_id
		WHERE o.domain = $1 AND u.email = $2`, orgDomain, newConfig().AdminEmail).Scan(&adminID))

	first := policiesOf(t, db, orgDomain)
	require.Equal(t, want, policyNames(first),
		"the policies the --default run left in the admin's organization; the run printed:\n%s", out.String())
	for _, p := range first {
		require.Equal(t, adminID, p.CreatedBy, "policy %q is not attributed to the admin the run created", p.Name)
		require.Equal(t, !strings.HasPrefix(p.Type, "mcp_"), p.Enabled,
			"policy %q (%s): migration 105 leaves the MCP policies disabled and the others enabled", p.Name, p.Type)
	}

	out.Reset()
	require.NoError(t, runBootstrap(ctx, &out, db, newConfig()))
	require.Equal(t, first, policiesOf(t, db, orgDomain),
		"a second --default run changed the organization's policies; it printed:\n%s", out.String())
}

// migrationsNaming returns the migration files whose text names table, in
// the order the migration runner applies them.
func migrationsNaming(t *testing.T, table string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	require.NoError(t, err)
	names := make([]string, 0)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		if strings.Contains(string(body), table) {
			names = append(names, filepath.Base(path))
		}
	}
	require.NotEmpty(t, names, "no migration names %s", table)
	sort.Strings(names)
	return names
}

// policyRow is a security_policies row without the columns that say which
// organization it belongs to, who created it and when.
type policyRow struct {
	Description       string
	PolicyType        string
	EnforcementAction string
	SeverityThreshold string
	Rules             map[string]interface{}
	AppliesTo         string
	IsEnabled         bool
	Priority          int
}

// policyRowsOf returns the policies of an organization by name.
func policyRowsOf(t *testing.T, tx *sql.Tx, orgID string) map[string]policyRow {
	t.Helper()
	rows, err := tx.Query(`
		SELECT name, COALESCE(description, ''), policy_type, enforcement_action,
		       severity_threshold, rules::text, applies_to, is_enabled, priority
		FROM security_policies WHERE organization_id = $1`, orgID)
	require.NoError(t, err)
	defer rows.Close()
	byName := make(map[string]policyRow)
	for rows.Next() {
		var name, rules string
		var r policyRow
		require.NoError(t, rows.Scan(&name, &r.Description, &r.PolicyType, &r.EnforcementAction,
			&r.SeverityThreshold, &rules, &r.AppliesTo, &r.IsEnabled, &r.Priority))
		require.NoError(t, json.Unmarshal([]byte(rules), &r.Rules), "rules of policy %q", name)
		byName[name] = r
	}
	require.NoError(t, rows.Err())
	return byName
}

// The rows the --default run adds are the rows the migrations leave for an
// organization whose admin exists when they run. In a transaction that is
// always rolled back, the test gives the OpenA2A Admin organization the user
// admin@opena2a.org if no such user exists, removes that organization's
// policies, applies every migration that names security_policies in order,
// then adds seed.DefaultPolicies to a second organization and compares the two
// organizations' rows. A migration that changes the rows 015 and 052 insert
// fails this test until seed.DefaultPolicies carries the same change.
func TestDefaultPoliciesMatchWhatTheMigrationsLeave(t *testing.T) {
	db := bootstrapIntegrationDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// Always rolled back: the migrations are applied again here only to read
	// the rows they write.
	defer func() { _ = tx.Rollback() }()

	var adminOrgID string
	err = tx.QueryRow(`SELECT id FROM organizations WHERE name = 'OpenA2A Admin' LIMIT 1`).Scan(&adminOrgID)
	if err == sql.ErrNoRows {
		err = tx.QueryRow(`INSERT INTO organizations (name, domain)
			VALUES ('OpenA2A Admin', 'bootstrap-policy-parity.invalid') RETURNING id`).Scan(&adminOrgID)
	}
	require.NoError(t, err)
	var adminID string
	err = tx.QueryRow(`SELECT id FROM users WHERE email = 'admin@opena2a.org' LIMIT 1`).Scan(&adminID)
	if err == sql.ErrNoRows {
		err = tx.QueryRow(`INSERT INTO users (organization_id, email, name, provider, provider_id)
			VALUES ($1, 'admin@opena2a.org', 'Parity Admin', 'local', 'bootstrap-policy-parity')
			RETURNING id`, adminOrgID).Scan(&adminID)
	}
	require.NoError(t, err)

	_, err = tx.Exec(`DELETE FROM security_policies WHERE organization_id = $1`, adminOrgID)
	require.NoError(t, err)
	for _, name := range migrationsNaming(t, "security_policies") {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		_, err = tx.Exec(string(body))
		require.NoErrorf(t, err, "applying %s", name)
	}
	fromMigrations := policyRowsOf(t, tx, adminOrgID)

	suffix := uuid.NewString()[:8]
	var orgID, userID uuid.UUID
	require.NoError(t, tx.QueryRow(`INSERT INTO organizations (name, domain) VALUES ($1, $2) RETURNING id`,
		"Bootstrap Policy Parity "+suffix, "bootstrap-policy-parity-"+suffix+".example.com").Scan(&orgID))
	require.NoError(t, tx.QueryRow(`INSERT INTO users (organization_id, email, name, provider, provider_id)
		VALUES ($1, $2, 'Parity Admin', 'local', $3) RETURNING id`,
		orgID, "bootstrap-policy-parity-"+suffix+"@example.com", "bootstrap-policy-parity-"+suffix).Scan(&userID))
	added, err := seed.AddDefaultPolicies(ctx, tx, orgID, userID)
	require.NoError(t, err)
	require.EqualValues(t, len(seed.DefaultPolicies), added)

	require.Equal(t, fromMigrations, policyRowsOf(t, tx, orgID.String()),
		"the policies the migrations leave for the OpenA2A Admin organization, then the ones the bootstrap adds")
}
