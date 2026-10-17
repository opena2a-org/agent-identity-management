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
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A successful sign-in, a wrong password, an unknown email and a denied FGA
// decision, sent through the server's route table against a migrated
// database, reach a loopback collector that enforces the published HTTP
// Event Collector format: the sign-in as its audit log entry, the two failed
// sign-ins and the denial as events of their own. Each password sign-in
// route is exercised on its own server.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSIEMAuthDecisionsRoundTrip ./cmd/server/
func TestSIEMAuthDecisionsRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping SIEM sign-in and FGA decision integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "siem-integration-secret-of-at-least-32-chars")

	// The reasons each route gives for the wrong password and the unknown email.
	for _, tc := range []struct{ route, wrongReason, unknownReason string }{
		{"/api/v1/public/login", "email_or_password_mismatch", "email_or_password_mismatch"},
		{"/api/v1/auth/login/local", "password_mismatch", "unknown_email"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			siemAuthDecisionsRoundTrip(t, db, tc.route, tc.wrongReason, tc.unknownReason)
		})
	}
}

func siemAuthDecisionsRoundTrip(t *testing.T, db *sql.DB, route, wrongReason, unknownReason string) {
	ctx := context.Background()
	org, user, agent := uuid.New(), uuid.New(), uuid.New()
	name := "siem-signin-" + uuid.NewString()[:8]
	email := name + "@example.invalid"
	unknownEmail := "unknown-" + name + "@example.invalid"
	password := "Pw-" + uuid.NewString()
	wrongPassword := "Wrong-" + uuid.NewString()
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`, org, name, name+".invalid")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
		VALUES ($1, $2, $3, 'siem test user', 'local', $4, 'manager', $5, 'active')`,
		user, org, email, user.String(), hash)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id, status, trust_score)
		VALUES ($1, $2, $2, 'service', $3, $4, 'verified', 0.8)`, agent, name+"-agent", user, org)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_failures WHERE email = $1 OR email = $2`, email, unknownEmail)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, org)
	})

	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)

	repos, oauthRepo := initRepositories(db)
	forwarder, err := startSIEMForwarding(repos.AuditLog, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
		"SIEM_SPLUNK_INDEX":     "aim_audit",
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)

	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	authService := application.NewAuthService(repos.User, repos.Organization, repos.APIKey, repos.AuthFailure, nil, nil)
	agentService := application.NewAgentService(repos.Agent, nil, nil, nil, repos.Alert, nil, repos.Capability, nil, nil, repos.User, repos.Organization, nil)
	fga := application.NewFGAEngine(db, agentService, nil)
	t.Cleanup(func() { _ = fga.Shutdown(ctx) })
	services := &Services{
		Auth:    authService,
		Audit:   auditService,
		Agent:   agentService,
		FGA:     fga,
		Secrets: &application.SecretsService{},
	}
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, jwtService, repos.Organization, auditService, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(oauthRepo, repos.User, repos.Organization, auditService, nil),
			authService, jwtService, auditService),
		Authorize: handlers.NewAuthorizeHandler(fga, repos.Agent),
	}
	observeSecurityDecisions(forwarder, services, h)

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, services, jwtService, nil, db)

	const userAgent = "siem-round-trip-test/1.0"
	do := func(path, bearer string, body interface{}) (int, string) {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		// A password check takes longer than app.Test's default second under -race.
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer resp.Body.Close()
		answer, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(answer)
	}

	status, body := do(route, "", map[string]string{"email": email, "password": password})
	require.Equal(t, fiber.StatusOK, status, body)
	wrongStatus, wrongBody := do(route, "", map[string]string{"email": email, "password": wrongPassword})
	require.Equal(t, fiber.StatusUnauthorized, wrongStatus, wrongBody)
	unknownStatus, unknownBody := do(route, "", map[string]string{"email": unknownEmail, "password": password})
	assert.Equal(t, wrongStatus, unknownStatus, "an unknown email is answered as a wrong password is")
	assert.Equal(t, wrongBody, unknownBody, "an unknown email is answered as a wrong password is")

	manager, err := jwtService.GenerateAccessToken(user.String(), org.String(), email, string(domain.RoleManager))
	require.NoError(t, err)
	status, body = do(fmt.Sprintf("/api/v1/agents/%s/authorize", agent), manager, map[string]string{"capability": "files:read"})
	require.Equal(t, fiber.StatusOK, status, body)
	var decision application.FGAResult
	require.NoError(t, json.Unmarshal([]byte(body), &decision))
	require.False(t, decision.Allowed, body)

	flushSIEMForwarding(forwarder)

	var loginRow string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT id FROM audit_logs WHERE organization_id = $1 AND action = 'login'`, org).Scan(&loginRow))

	assert.Empty(t, receiver.Refusals(), "the collector refused a request")
	accepted := receiver.Accepted()
	byType := map[string][]map[string]interface{}{}
	for _, e := range accepted {
		assert.Equal(t, "opena2a:aim", e.Sourcetype)
		assert.Equal(t, "opena2a-aim", e.Source)
		assert.Equal(t, "aim_audit", e.Index)
		assert.NotEmpty(t, e.Time)
		for _, secret := range []string{email, unknownEmail, password, wrongPassword} {
			assert.NotContains(t, string(e.Event), secret, "an event carries a submitted email or password")
		}
		var event map[string]interface{}
		require.NoError(t, e.Decode(&event))
		eventType, _ := event["eventType"].(string)
		byType[eventType] = append(byType[eventType], event)
	}
	counts := map[string]int{}
	for eventType, events := range byType {
		counts[eventType] = len(events)
	}
	require.Equal(t, map[string]int{
		"aim.audit.login":        1,
		"aim.auth.signin_failed": 2,
		"aim.fga.denied":         1,
	}, counts, "events the collector accepted, by type")

	login := byType["aim.audit.login"][0]
	assert.Equal(t, loginRow, login["details"].(map[string]interface{})["auditLogId"])

	var wrong, unknown map[string]interface{}
	for _, event := range byType["aim.auth.signin_failed"] {
		assert.Equal(t, "low", event["severity"])
		details := event["details"].(map[string]interface{})
		assert.Equal(t, "password", details["method"])
		assert.Equal(t, route, details["route"])
		assert.Equal(t, userAgent, details["userAgent"])
		assert.NotEmpty(t, details["ipAddress"])
		assert.NotEmpty(t, details["reason"])
		if _, matched := details["userId"]; matched {
			wrong = details
		} else {
			unknown = details
		}
	}
	require.NotNil(t, wrong, "the wrong password's event names the user")
	require.NotNil(t, unknown, "the unknown email's event names no user")
	assert.Equal(t, user.String(), wrong["userId"])
	assert.Equal(t, org.String(), wrong["organizationId"])
	assert.Equal(t, wrongReason, wrong["reason"])
	assert.NotContains(t, unknown, "organizationId")
	assert.Equal(t, unknownReason, unknown["reason"])

	denied := byType["aim.fga.denied"][0]
	assert.Equal(t, agent.String(), denied["agentId"])
	assert.Equal(t, "files:read", denied["capability"])
	assert.Equal(t, decision.Outcome, denied["outcome"])
	assert.Equal(t, "medium", denied["severity"])
	deniedDetails := denied["details"].(map[string]interface{})
	assert.Equal(t, org.String(), deniedDetails["organizationId"])
	assert.Equal(t, decision.DeniedBy, deniedDetails["deniedBy"])
	assert.Equal(t, decision.DeniedReason, deniedDetails["deniedReason"])
}
