//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// TestMCPManifestDrift_Wiring exercises the manifest/drift service end-to-end against a real
// database (migrations 098/099 + mcp_server_capabilities). It validates the three behaviors the
// capability and attestation services depend on once the manifest service is wired in:
//
//   - a server's first /.well-known capture seeds the baseline and records no drift;
//   - a later capture that adds a high-risk tool records HIGH-severity drift and advances the baseline;
//   - an attestation whose observed tool set diverges from the baseline only records drift once
//     multi-agent consensus is reached (below consensus it is ignored to avoid single-agent poisoning).
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestMCPManifestDrift_Wiring ./internal/application/...
func TestMCPManifestDrift_Wiring(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping manifest drift integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	capabilityRepo := repository.NewMCPServerCapabilityRepository(db)
	manifestRepo := repository.NewMCPManifestRepository(db)
	svc := NewMCPManifestService(capabilityRepo, manifestRepo)

	// seedServer creates an organization + MCP server and registers cleanup. The manifest tables
	// cascade-delete with the server, so removing the server clears any baselines/drift it produced.
	seedServer := func(t *testing.T) uuid.UUID {
		orgID := uuid.New()
		userID := uuid.New()
		serverID := uuid.New()
		suffix := serverID.String()[:8]
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM mcp_server_capabilities WHERE mcp_server_id = $1`, serverID)
			_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, serverID)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
			_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
		})
		_, err := db.ExecContext(ctx,
			`INSERT INTO organizations (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
			orgID, "mcp-manifest-test-org-"+suffix, "mcp-manifest-test-"+suffix+".example.com")
		require.NoError(t, err)
		_, err = db.ExecContext(ctx,
			`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'test', $5, NOW(), NOW())`,
			userID, orgID, "owner-"+suffix+"@example.com", "manifest-test-owner-"+suffix, userID.String())
		require.NoError(t, err)
		_, err = db.ExecContext(ctx,
			`INSERT INTO mcp_servers (id, organization_id, name, url, version, capabilities, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, '1.0.0', '[]'::jsonb, $5, NOW(), NOW())`,
			serverID, orgID,
			"mcp-manifest-test-server-"+suffix,
			"https://mcp-manifest-test-"+suffix+".example.com",
			userID)
		require.NoError(t, err)
		return serverID
	}

	addTool := func(t *testing.T, serverID uuid.UUID, name string) {
		schema, _ := json.Marshal(map[string]any{"type": "object", "title": name})
		_, err := db.ExecContext(ctx,
			`INSERT INTO mcp_server_capabilities
			   (id, mcp_server_id, name, capability_type, description, capability_schema,
			    is_active, detected_at, created_at, updated_at)
			 VALUES ($1, $2, $3, 'tool', '', $4::jsonb, TRUE, NOW(), NOW(), NOW())`,
			uuid.New(), serverID, name, schema)
		require.NoError(t, err)
	}

	driftCount := func(t *testing.T, serverID uuid.UUID) int {
		var n int
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM mcp_manifest_drift WHERE mcp_server_id = $1`, serverID).Scan(&n))
		return n
	}

	t.Run("well-known first capture seeds baseline with no drift", func(t *testing.T) {
		serverID := seedServer(t)
		addTool(t, serverID, "list_files")
		addTool(t, serverID, "get_status")

		drift, err := svc.RecomputeFromWellKnown(ctx, serverID)
		require.NoError(t, err)
		require.Nil(t, drift, "first capture must not record drift")

		baseline, entries, err := manifestRepo.GetCurrentBaseline(serverID)
		require.NoError(t, err)
		require.NotNil(t, baseline, "first capture must seed a baseline")
		require.Equal(t, 2, baseline.ToolCount)
		require.Len(t, entries, 2)
		require.Equal(t, 0, driftCount(t, serverID))
	})

	t.Run("well-known adding a high-risk tool records HIGH drift", func(t *testing.T) {
		serverID := seedServer(t)
		addTool(t, serverID, "list_files")
		_, err := svc.RecomputeFromWellKnown(ctx, serverID) // seed baseline
		require.NoError(t, err)

		// Silently add a tool that touches a high-risk surface (exec/command) after the server was trusted.
		addTool(t, serverID, "exec_command")

		drift, err := svc.RecomputeFromWellKnown(ctx, serverID)
		require.NoError(t, err)
		require.NotNil(t, drift, "adding a tool must record drift")
		require.Equal(t, domain.DriftSeverityHigh, drift.Severity)
		require.Equal(t, []string{"tool:exec_command"}, drift.AddedTools)
		require.Empty(t, drift.RemovedTools)
		require.Equal(t, 1, driftCount(t, serverID))

		// Baseline advanced to the new surface, so a re-capture is now stable (no repeated drift).
		baseline, _, err := manifestRepo.GetCurrentBaseline(serverID)
		require.NoError(t, err)
		require.Equal(t, 2, baseline.ToolCount)
		again, err := svc.RecomputeFromWellKnown(ctx, serverID)
		require.NoError(t, err)
		require.Nil(t, again)
		require.Equal(t, 1, driftCount(t, serverID))
	})

	t.Run("attestation divergence ignored below consensus, recorded at consensus", func(t *testing.T) {
		serverID := seedServer(t)
		addTool(t, serverID, "list_files")
		_, err := svc.RecomputeFromWellKnown(ctx, serverID) // baseline = {list_files}
		require.NoError(t, err)
		require.Equal(t, 0, driftCount(t, serverID))

		// An agent reports a tool the server never declared. Generic category tokens are ignored.
		observed := []string{"tools", "list_files", "delete_everything"}

		// Below consensus: a single (or sub-threshold) agent's divergence must not move the baseline
		// or record drift.
		drift, err := svc.RecomputeFromAttestation(ctx, serverID, observed, false)
		require.NoError(t, err)
		require.Nil(t, drift, "sub-consensus divergence must not record drift")
		require.Equal(t, 0, driftCount(t, serverID))
		baseline, _, err := manifestRepo.GetCurrentBaseline(serverID)
		require.NoError(t, err)
		require.Equal(t, 1, baseline.ToolCount, "baseline unchanged below consensus")

		// At consensus: the divergence is confirmed and recorded as drift; the high-risk
		// "delete_everything" addition escalates severity to high.
		drift, err = svc.RecomputeFromAttestation(ctx, serverID, observed, true)
		require.NoError(t, err)
		require.NotNil(t, drift, "consensus-confirmed divergence must record drift")
		require.Equal(t, domain.ManifestSourceAttestation, drift.Source)
		require.Equal(t, domain.DriftSeverityHigh, drift.Severity)
		require.Equal(t, []string{"tool:delete_everything"}, drift.AddedTools)
		require.Empty(t, drift.RemovedTools, "list_files is in both sets, so nothing removed")
		require.Equal(t, 1, driftCount(t, serverID))

		// Baseline-poisoning guard: attestation NEVER moves the baseline. Only the server's own
		// /.well-known capture is authoritative, so the baseline stays exactly what well-known seeded.
		baseline, _, err = manifestRepo.GetCurrentBaseline(serverID)
		require.NoError(t, err)
		require.Equal(t, domain.ManifestSourceWellKnown, baseline.CapturedFrom, "attestation must not author the baseline")
		require.Equal(t, 1, baseline.ToolCount, "baseline unchanged by attestation")

		// Dedup: the same divergence reported again (still consensus-met) must NOT record a second
		// drift row, since the baseline correctly never absorbed it.
		drift, err = svc.RecomputeFromAttestation(ctx, serverID, observed, true)
		require.NoError(t, err)
		require.Nil(t, drift, "identical unacknowledged divergence must not be re-recorded")
		require.Equal(t, 1, driftCount(t, serverID))
	})

	t.Run("well-known reconcile detects removal and schema change", func(t *testing.T) {
		serverID := seedServer(t)

		// Drive capabilities through the same reconcile path DetectCapabilities uses, so this exercises
		// removal/schema-change detection rather than the addition-only behavior of a plain insert.
		tool := func(name, title string) *domain.MCPServerCapability {
			schema, _ := json.Marshal(map[string]any{"type": "object", "title": title})
			return &domain.MCPServerCapability{
				MCPServerID: serverID, Name: name, CapabilityType: domain.MCPCapabilityTypeTool,
				CapabilitySchema: schema,
			}
		}

		// Initial trusted surface: two tools. Seed baseline, no drift.
		require.NoError(t, capabilityRepo.ReplaceActiveCapabilities(serverID,
			[]*domain.MCPServerCapability{tool("list_files", "v1"), tool("read_data", "v1")}))
		_, err := svc.RecomputeFromWellKnown(ctx, serverID)
		require.NoError(t, err)
		require.Equal(t, 0, driftCount(t, serverID))

		// Server silently: removes read_data, reshapes list_files' schema, adds exec_command.
		require.NoError(t, capabilityRepo.ReplaceActiveCapabilities(serverID,
			[]*domain.MCPServerCapability{tool("list_files", "v2-reshaped"), tool("exec_command", "v1")}))

		drift, err := svc.RecomputeFromWellKnown(ctx, serverID)
		require.NoError(t, err)
		require.NotNil(t, drift, "removal + schema change + add must record drift")
		require.Equal(t, []string{"tool:exec_command"}, drift.AddedTools)
		require.Equal(t, []string{"tool:read_data"}, drift.RemovedTools, "removed tool must be detected (not left active)")
		require.Equal(t, []string{"tool:list_files"}, drift.ChangedTools, "reshaped schema must be detected")
		require.Equal(t, domain.DriftSeverityHigh, drift.Severity, "exec_command addition escalates to high")
		require.Equal(t, 1, driftCount(t, serverID))
	})
}
