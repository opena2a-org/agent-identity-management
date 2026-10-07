//go:build integration

package application

import (
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// rescaleMigration puts rules.minTrustScore on the canonical [0,1] scale for mcp_* policies.
const rescaleMigration = "112_rescale_mcp_policy_min_trust_score.sql"

// TestMCPPolicyMinTrustScoreOutOfScaleIsRescaled applies the rescaling migration against a real
// database, inside a transaction that is always rolled back.
//
// MCPPolicyEvaluator compares rules.minTrustScore against MCPServer.TrustScore, which migration
// 104 made canonical [0,1]. Migration 052 seeded floors of 50 and 30, and the admin page stored
// the percentage typed into its form, so every such floor sat above every representable trust
// score and would have rejected every MCP server once the evaluator ran.
//
// The name carries "OutOfScale" so the CI step that fails on a skipped integration test covers it.
func TestMCPPolicyMinTrustScoreOutOfScaleIsRescaled(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping MCP policy minTrustScore rescale test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	tx, err := db.Begin()
	require.NoError(t, err)
	// Always roll back: this asserts over migration SQL, not over durable state.
	defer func() { _ = tx.Rollback() }()

	t.Run("operator rows", func(t *testing.T) {
		suffix := uuid.NewString()
		var orgID, userID string
		require.NoError(t, tx.QueryRow(
			`INSERT INTO organizations (name, domain) VALUES ($1, $2) RETURNING id`,
			"MCP floor guard "+suffix, "mcp-floor-"+suffix+".invalid").Scan(&orgID))
		require.NoError(t, tx.QueryRow(
			`INSERT INTO users (organization_id, email, name, provider, provider_id)
			 VALUES ($1, $2, 'Floor Guard', 'local', $3) RETURNING id`,
			orgID, "mcp-floor-"+suffix+"@example.invalid", "mcp-floor-"+suffix).Scan(&userID))

		insert := func(name, policyType, rules string, enabled bool) {
			t.Helper()
			_, err := tx.Exec(`
				INSERT INTO security_policies
					(organization_id, name, description, policy_type, enforcement_action,
					 severity_threshold, rules, is_enabled, created_by)
				VALUES ($1, $2, '', $3, 'alert_only', 'medium', $4::jsonb, $5, $6)`,
				orgID, name, policyType, rules, enabled, userID)
			require.NoError(t, err)
		}

		insert("floor 50", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 50}`, false)
		insert("floor 30", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 30}`, true)
		insert("floor 72.5", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 72.5}`, false)
		insert("floor 100", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 100}`, false)
		insert("canonical 0.7", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 0.7}`, false)
		insert("boundary 1", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 1}`, false)
		insert("no floor", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": 0}`, false)
		insert("string floor", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": "50"}`, false)
		insert("boolean floor", "mcp_allowlist", `{"allowedDomains": ["*"], "minTrustScore": true}`, false)
		insert("absent floor", "mcp_blocklist", `{"blockedDomains": ["*.malware.com"]}`, false)
		insert("non-mcp floor", "trust_score_low", `{"minTrustScore": 50}`, false)

		for run := 1; run <= 2; run++ {
			// The second run proves the migration is idempotent: a value already on [0,1] is
			// never divided again.
			applyMigration(t, tx, rescaleMigration)

			for _, tc := range []struct {
				name string
				want string // rules->'minTrustScore' as jsonb text; "" means the key is absent
			}{
				{"floor 50", "0.5"},
				{"floor 30", "0.3"},
				{"floor 72.5", "0.725"},
				{"floor 100", "1"},
				{"canonical 0.7", "0.7"},
				{"boundary 1", "1"},
				{"no floor", "0"},
				{"string floor", `"50"`},
				{"boolean floor", "true"},
				{"absent floor", ""},
				{"non-mcp floor", "50"},
			} {
				require.Equal(t, tc.want, floorOf(t, tx, orgID, tc.name),
					"run %d: rules.minTrustScore of %q", run, tc.name)
			}
		}

		var domains string
		var enabled int
		require.NoError(t, tx.QueryRow(`
			SELECT (SELECT rules->>'allowedDomains' FROM security_policies
			        WHERE organization_id = $1 AND name = 'floor 50'),
			       (SELECT COUNT(*) FROM security_policies
			        WHERE organization_id = $1 AND is_enabled)`, orgID).Scan(&domains, &enabled))
		require.Equal(t, `["*"]`, domains, "the other rule keys must be preserved")
		require.Equal(t, 1, enabled, "the migration must not enable or disable any policy")
	})

	t.Run("seeded policies", func(t *testing.T) {
		// Migration 052 looks the admin organization and user up by name and email and no-ops
		// when either is missing, so create them for the seeding to run. Delete any seeded rows
		// first: 052 inserts ON CONFLICT DO NOTHING, and on a migrated database the existing
		// rows already carry the rescaled values, which would let this pass without the fix.
		adminOrgID := ensureRow(t, tx,
			`SELECT id FROM organizations WHERE name = 'OpenA2A Admin' LIMIT 1`,
			`INSERT INTO organizations (name, domain) VALUES ('OpenA2A Admin', 'mcp-floor-guard.invalid') RETURNING id`)
		ensureRow(t, tx,
			`SELECT id FROM users WHERE email = 'admin@opena2a.org' LIMIT 1`,
			`INSERT INTO users (organization_id, email, name, provider, provider_id)
			 VALUES ($1, 'admin@opena2a.org', 'Guard Admin', 'local', 'mcp-floor-guard')
			 RETURNING id`, adminOrgID)
		_, err := tx.Exec(`DELETE FROM security_policies WHERE policy_type = ANY($1) AND name = ANY($2)`,
			pq.Array(mcpPolicyTypes), pq.Array(seededPolicyNames))
		require.NoError(t, err)

		applyMigration(t, tx, seedingMigration)
		require.Equal(t, "50", floorOf(t, tx, adminOrgID, "MCP Minimum Trust Score"),
			"migration 052 should seed a 0-100 floor of 50; if the seed changed, update this guard")
		require.Equal(t, "30", floorOf(t, tx, adminOrgID, "High-Risk MCP Server Block"),
			"migration 052 should seed a 0-100 floor of 30; if the seed changed, update this guard")

		applyMigration(t, tx, rescaleMigration)
		require.Equal(t, "0.5", floorOf(t, tx, adminOrgID, "MCP Minimum Trust Score"))
		require.Equal(t, "0.3", floorOf(t, tx, adminOrgID, "High-Risk MCP Server Block"))
		require.Equal(t, "0", floorOf(t, tx, adminOrgID, "Trusted MCP Server Domains"))
	})
}

// floorOf returns rules->'minTrustScore' of the named policy as jsonb text, or "" when the key is
// absent.
func floorOf(t *testing.T, tx *sql.Tx, orgID, name string) string {
	t.Helper()
	var floor sql.NullString
	require.NoError(t, tx.QueryRow(`
		SELECT (rules -> 'minTrustScore')::text
		FROM security_policies
		WHERE organization_id = $1 AND name = $2`, orgID, name).Scan(&floor))
	return floor.String
}
