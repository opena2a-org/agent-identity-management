//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestMCPCapabilitiesCacheTrigger_DerivesFromDetailTable is the
// regression test for issue #164. Migration 096 installs an
// AFTER INSERT/UPDATE/DELETE trigger on mcp_server_capabilities
// that recomputes the mcp_servers.capabilities JSONB cache as
// `DISTINCT capability_type` filtered to is_active = TRUE.
//
// Conservative semantic: when the detail table is empty for a
// given server, the trigger PRESERVES the existing JSONB (which
// is whatever the registering agent claimed at create time).
// This avoids wiping the agent card to "no capabilities" before
// discovery has run.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMCPCapabilitiesCacheTrigger ./internal/application/...
func TestMCPCapabilitiesCacheTrigger_DerivesFromDetailTable(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_server_capabilities WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())`,
		orgID, "mcp-caps-test-org-"+serverID.String()[:8])
	require.NoError(t, err)

	// Seed MCP server with the registering agent's claim of all
	// three capability types.
	claimed := `["resources", "prompts", "tools"]`
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers (id, organization_id, name, url, version, capabilities, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', $5::jsonb, NOW(), NOW())`,
		serverID, orgID,
		"mcp-caps-test-server-"+serverID.String()[:8],
		"https://mcp-caps-test-"+serverID.String()[:8]+".example.com",
		claimed)
	require.NoError(t, err)

	// Insert detail rows of TWO types only ('tool' and 'resource').
	// 5 'tool' rows + 2 'resource' rows + 0 'prompt' rows.
	for i := 0; i < 5; i++ {
		_, err = db.ExecContext(ctx,
			`INSERT INTO mcp_server_capabilities
			   (mcp_server_id, name, capability_type, is_active, detected_at, created_at, updated_at)
			 VALUES ($1, $2, 'tool', TRUE, NOW(), NOW(), NOW())`,
			serverID, "tool-"+uuid.New().String()[:8])
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx,
			`INSERT INTO mcp_server_capabilities
			   (mcp_server_id, name, capability_type, is_active, detected_at, created_at, updated_at)
			 VALUES ($1, $2, 'resource', TRUE, NOW(), NOW(), NOW())`,
			serverID, "resource-"+uuid.New().String()[:8])
		require.NoError(t, err)
	}

	derived := readCapabilitiesArray(t, db, ctx, serverID)
	require.Equal(t, []string{"resource", "tool"}, derived,
		"trigger must derive cache as DISTINCT capability_type from detail rows (sorted, only types that actually exist)")
}

// TestMCPCapabilitiesCacheTrigger_PreservesClaimWhenDetailEmpty
// asserts the preserve-on-empty semantic. When the detail table
// has zero active rows, the JSONB cache must NOT be wiped to []
// or NULL — the agent's claim stays.
func TestMCPCapabilitiesCacheTrigger_PreservesClaimWhenDetailEmpty(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_server_capabilities WHERE mcp_server_id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())`,
		orgID, "mcp-caps-preserve-test-org-"+serverID.String()[:8])
	require.NoError(t, err)

	claimed := `["tools", "prompts"]`
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers (id, organization_id, name, url, version, capabilities, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', $5::jsonb, NOW(), NOW())`,
		serverID, orgID,
		"mcp-caps-preserve-test-server-"+serverID.String()[:8],
		"https://mcp-caps-preserve-test-"+serverID.String()[:8]+".example.com",
		claimed)
	require.NoError(t, err)

	// Insert a row then delete it. After DELETE the detail table
	// is empty again — the trigger must preserve the claim.
	rowID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_server_capabilities
		   (id, mcp_server_id, name, capability_type, is_active, detected_at, created_at, updated_at)
		 VALUES ($1, $2, 'temp-tool', 'tool', TRUE, NOW(), NOW(), NOW())`,
		rowID, serverID)
	require.NoError(t, err)

	// After INSERT, derived = ["tool"]. Verify, then delete.
	require.Equal(t, []string{"tool"},
		readCapabilitiesArray(t, db, ctx, serverID),
		"after one 'tool' INSERT the cache derives to [\"tool\"]")

	_, err = db.ExecContext(ctx,
		`DELETE FROM mcp_server_capabilities WHERE id = $1`, rowID)
	require.NoError(t, err)

	// After DELETE: detail table empty. Cache must preserve last
	// derived value (not the original claim — once discovery has
	// happened the trigger has already overwritten the claim).
	// This is intentional: the cache reflects the most recent
	// known reality, even if that reality has since been retracted.
	require.Equal(t, []string{"tool"},
		readCapabilitiesArray(t, db, ctx, serverID),
		"after DELETE empties the detail table, preserve last derived value (do not wipe to [])")
}

// TestMCPCapabilitiesCacheTrigger_PreservesClaimOnFreshServer
// asserts that a server which has NEVER had detail-table rows
// keeps its claim untouched. This is the brand-new-MCP path.
func TestMCPCapabilitiesCacheTrigger_PreservesClaimOnFreshServer(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping trigger regression test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID := uuid.New()
	serverID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err = db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())`,
		orgID, "mcp-caps-fresh-test-org-"+serverID.String()[:8])
	require.NoError(t, err)

	claimed := `["tools", "resources"]`
	_, err = db.ExecContext(ctx,
		`INSERT INTO mcp_servers (id, organization_id, name, url, version, capabilities, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '1.0.0', $5::jsonb, NOW(), NOW())`,
		serverID, orgID,
		"mcp-caps-fresh-test-server-"+serverID.String()[:8],
		"https://mcp-caps-fresh-test-"+serverID.String()[:8]+".example.com",
		claimed)
	require.NoError(t, err)

	// No detail rows ever inserted. The cache should still be the
	// claim, untouched.
	require.Equal(t, []string{"resources", "tools"},
		readCapabilitiesArray(t, db, ctx, serverID),
		"fresh MCP with no detail rows must keep the agent's claim verbatim")
}

func readCapabilitiesArray(t *testing.T, db *sql.DB, ctx context.Context, serverID uuid.UUID) []string {
	t.Helper()
	var raw []byte
	err := db.QueryRowContext(ctx,
		`SELECT capabilities::text FROM mcp_servers WHERE id = $1`, serverID,
	).Scan(&raw)
	require.NoError(t, err)
	var arr []string
	require.NoError(t, json.Unmarshal(raw, &arr))
	sort.Strings(arr)
	return arr
}
