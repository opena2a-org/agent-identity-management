//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seededIDLister lists through the real repository query, then narrows to the
// servers this test seeded, so the test does not score other tests' rows in a
// shared database.
type seededIDLister struct {
	repo   *repository.MCPServerRepository
	seeded []uuid.UUID // rows this test inserted
	extra  []uuid.UUID // IDs with no row, to exercise a scoring failure
	t      *testing.T
}

func (l *seededIDLister) ListAllIDs(ctx context.Context) ([]uuid.UUID, error) {
	all, err := l.repo.ListAllIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range l.seeded {
		assert.Contains(l.t, all, id, "ListAllIDs returns every server")
	}
	return append(append([]uuid.UUID{}, l.seeded...), l.extra...), nil
}

// TestRescoreMCPServersOnce_ReplacesRescaledPlaceholders seeds the two values
// a server held after migration 104 when it was registered before the
// calculator was wired in (0.75 from the rescaled 75.0 literal, 0.0 from the
// column default), with no calculated score row, and checks that the pass
// leaves each server holding a calculated score.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestRescoreMCPServersOnce ./internal/application/...
func TestRescoreMCPServersOnce_ReplacesRescaledPlaceholders(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping MCP rescoring integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	// Registered first so it runs last: the cleanups below need the connection.
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID, userID := seedOrgAndUser(t, db, ctx, "mcp-rescore")

	sdkID, manualID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		for _, id := range []uuid.UUID{sdkID, manualID} {
			_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_score_history WHERE mcp_server_id = $1`, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM mcp_trust_scores WHERE mcp_server_id = $1`, id)
			_, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, id)
		}
	})
	seed := func(id uuid.UUID, name, status string, verified bool, publicKey sql.NullString, score float64) {
		_, err := db.ExecContext(ctx,
			`INSERT INTO mcp_servers
			   (id, organization_id, name, url, version, public_key, status, is_verified,
			    trust_score, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, '1.0.0', $5, $6, $7, $8, $9, NOW() - INTERVAL '40 days', NOW())`,
			id, orgID, name+"-"+id.String()[:8], "https://"+name+"-"+id.String()[:8]+".example.com",
			publicKey, status, verified, score, userID)
		require.NoError(t, err)
	}
	seed(sdkID, "mcp-rescore-sdk", "verified", true, sql.NullString{String: "pk", Valid: true}, 0.75)
	seed(manualID, "mcp-rescore-manual", "pending", false, sql.NullString{}, 0.0)

	serverRepo := repository.NewMCPServerRepository(db)
	calculator := NewMCPTrustCalculatorWithRepo(
		serverRepo,
		repository.NewMCPAttestationRepository(db),
		repository.NewMCPServerCapabilityRepository(db),
		repository.NewAlertRepository(db),
		repository.NewMCPTrustScoreRepository(db),
	)
	missing := uuid.New()
	lister := &seededIDLister{repo: serverRepo, seeded: []uuid.UUID{sdkID, manualID}, extra: []uuid.UUID{missing}, t: t}

	result, err := RescoreMCPServersOnce(ctx, newFakeMarkerStore(), lister, calculator)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Scored)
	assert.Equal(t, []uuid.UUID{missing}, result.Failed)

	for _, id := range []uuid.UUID{sdkID, manualID} {
		var cached, calculated float64
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT trust_score FROM mcp_servers WHERE id = $1`, id).Scan(&cached))
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT score FROM mcp_trust_scores WHERE mcp_server_id = $1
			 ORDER BY created_at DESC LIMIT 1`, id).Scan(&calculated),
			"server %s has a calculated score row", id)
		assert.InDelta(t, calculated, cached, 1e-4, "server %s holds its calculated score", id)
	}

	var sdkScore float64
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT trust_score FROM mcp_servers WHERE id = $1`, sdkID).Scan(&sdkScore))
	assert.NotEqual(t, 0.75, sdkScore, "the rescaled 75.0 placeholder is replaced")
}

// TestSystemConfigRepository_MarkerRoundTrip checks the marker store the
// rescoring pass uses at startup against the real system_config table.
func TestSystemConfigRepository_MarkerRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping system_config integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	// Registered first so it runs last: the cleanups below need the connection.
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	ctx := context.Background()
	key := "integration_test_marker_" + uuid.New().String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM system_config WHERE key = $1`, key)
	})

	markers := repository.NewSystemConfigRepository(db)
	set, err := markers.IsMarkerSet(ctx, key)
	require.NoError(t, err)
	assert.False(t, set)

	require.NoError(t, markers.SetMarker(ctx, key, "first"))
	require.NoError(t, markers.SetMarker(ctx, key, "second"), "setting twice is not an error")

	set, err = markers.IsMarkerSet(ctx, key)
	require.NoError(t, err)
	assert.True(t, set)
}
