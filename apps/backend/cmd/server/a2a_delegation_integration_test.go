//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A delegation chain granted and verified over HTTP, through the server's
// route table, against the a2a_delegation and agents tables of a migrated
// database. The root delegator's trust score is the agents row's.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestA2ADelegationChainOverHTTPAgainstTheDatabase ./cmd/server/
func TestA2ADelegationChainOverHTTPAgainstTheDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping delegation chain integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	ctx := context.Background()

	org, user := uuid.New(), uuid.New()
	agents := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	name := "delegation-http-" + uuid.NewString()[:8]
	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`, org, name, name+".invalid")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role)
		VALUES ($1, $2, $3, 'delegation test user', 'local', $4, 'manager')`,
		user, org, name+"@example.invalid", user.String())
	require.NoError(t, err)
	for i, id := range agents {
		_, err = db.ExecContext(ctx, `INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id, status, trust_score)
			VALUES ($1, $2, $2, 'service', $3, $4, 'verified', $5)`,
			id, fmt.Sprintf("%s-agent-%d", name, i), user, org, []float64{0.9, 0.6, 0.5, 0.4}[i])
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_delegation WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, org)
	})

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "delegation-integration-secret-of-at-least-32-chars")
	jwtService := auth.NewJWTService()
	svc := application.NewA2ADelegationService(repository.NewA2ADelegationRepository(db), repository.NewAgentRepository(db))
	h := &Handlers{A2ADelegation: handlers.NewA2ADelegationHandler(svc, application.NewAuditService(repository.NewAuditLogRepository(db)))}
	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, &Services{Secrets: &application.SecretsService{}}, jwtService, nil, db)

	manager, err := jwtService.GenerateAccessToken(user.String(), org.String(), "manager@example.invalid", string(domain.RoleManager))
	require.NoError(t, err)
	do := func(path, body string) (int, map[string]interface{}) {
		t.Helper()
		req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+manager)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var decoded map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &decoded), "body: %s", raw)
		return resp.StatusCode, decoded
	}

	var ids []string
	for i := 0; i < 3; i++ {
		status, d := do("/api/v1/a2a/delegations", fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"canDelegateFurther":%t}`,
			agents[i], agents[i+1], i < 2))
		require.Equal(t, fiber.StatusCreated, status, "%v", d)
		ids = append(ids, `"`+d["id"].(string)+`"`)
	}
	var audited int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE organization_id = $1 AND resource_type = 'a2a_delegation' AND action = 'create'`, org).Scan(&audited))
	assert.Equal(t, 3, audited)

	chain := `{"delegationIds":[` + strings.Join(ids, ",") + `]}`
	status, verdict := do("/api/v1/a2a/delegations/verify", chain)
	require.Equal(t, fiber.StatusOK, status, "%v", verdict)
	assert.Equal(t, true, verdict["valid"])
	assert.Equal(t, 0.9, verdict["rootTrustScore"])
	assert.Equal(t, 0.4608, verdict["effectiveTrust"])

	_, err = db.ExecContext(ctx, `UPDATE agents SET trust_score = 0.5 WHERE id = $1`, agents[0])
	require.NoError(t, err)
	status, verdict = do("/api/v1/a2a/delegations/verify", chain)
	require.Equal(t, fiber.StatusOK, status, "%v", verdict)
	assert.Equal(t, false, verdict["valid"])
	hops := verdict["hops"].([]interface{})
	require.Len(t, hops, 3)
	assert.Equal(t, 0.256, hops[2].(map[string]interface{})["effectiveTrust"])
	assert.Equal(t, "effective trust 0.256 is below the trust floor 0.3", hops[2].(map[string]interface{})["error"])
}
