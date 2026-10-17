//go:build integration

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
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
	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A verification refused for a capability the agent was not granted, a
// verification refused for a suspended agent, and the four secrets management
// requests (namespace create, credential store, credential rotate, namespace
// delete), sent through the server's route table against a migrated
// database, reach a loopback collector that enforces the published HTTP
// Event Collector format: every alert the denial inserts as one
// aim.alert.created event, the suspended agent's refusal as its audit log
// entry, and each secrets request as the audit log entry it now writes. The
// denial is sent twice, so the alert it repeats is counted on the open alert
// and is not forwarded a second time. No event and no audit row carries the
// credential.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSIEMAlertsAndSecretsRoundTrip ./cmd/server/
func TestSIEMAlertsAndSecretsRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping SIEM alert and secrets integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "siem-integration-secret-of-at-least-32-chars")

	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	name := "siem-alerts-" + uuid.NewString()[:8]
	email := name + "@example.invalid"

	// Strict mode: a capability the agent was not granted is refused.
	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain, enforcement_mode) VALUES ($1, $2, $3, 'strict')`,
		org, name, name+".invalid")
	require.NoError(t, err)
	t.Cleanup(func() {
		agents := `SELECT id FROM agents WHERE organization_id = $1`
		_, _ = db.ExecContext(ctx, `DELETE FROM secret_credentials WHERE namespace_id IN (SELECT id FROM secret_namespaces WHERE agent_id IN (`+agents+`))`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM secret_namespaces WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_capabilities WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_violations WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM verification_events WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, org)
	})
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
		VALUES ($1, $2, $3, 'siem test user', 'local', $4, 'manager', 'x', 'active')`,
		user, org, email, user.String())
	require.NoError(t, err)

	repos, oauthRepo := initRepositories(db)

	// An agent holding db:read, and a suspended agent, each with the Ed25519
	// key it signs its verifications with. Each refused db:write takes 10
	// points off the trust score; from 0.95 two of them stay above the
	// thresholds at which the policy service alerts on the score and
	// suspends the agent, so both reach the capability check.
	seedAgent := func(status string) (uuid.UUID, ed25519.PrivateKey) {
		t.Helper()
		id := uuid.New()
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, public_key, trust_score, created_by)
			VALUES ($1, $2, $3, $3, 'ai_agent', $4, $5, 0.95, $6)`,
			id, org, name+"-"+status+"-"+id.String()[:8], status, base64.StdEncoding.EncodeToString(pub), user)
		require.NoError(t, err)
		return id, priv
	}
	enforced, enforcedKey := seedAgent("verified")
	require.NoError(t, repos.Capability.CreateCapability(&domain.AgentCapability{
		AgentID: enforced, CapabilityType: "db:read", GrantedBy: &user,
	}))
	suspended, suspendedKey := seedAgent("suspended")

	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)

	forwarder, err := startSIEMForwarding(repos.AuditLog, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
		"SIEM_SPLUNK_INDEX":     "aim_audit",
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)

	// The services the verification, secrets and sign-in routes reach, built
	// from the constructors initServices uses on the repositories
	// initRepositories returns, so every service writes alerts through the
	// one alert repository main shares.
	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	alertService := application.NewAlertService(repos.Alert, repos.Agent, db)
	policyService := application.NewSecurityPolicyService(repos.SecurityPolicy, repos.Alert, repos.AuditLog)
	policyService.SetVerificationEventRepo(repos.VerificationEvent)
	policyService.SetCapabilityRepository(repos.Capability)
	policyService.SetAgentRepository(repos.Agent)
	trustCalc := application.NewTrustCalculatorWithVerification(repos.TrustScore, repos.APIKey, repos.AuditLog, repos.Capability, repos.Agent, repos.Alert, repos.VerificationEvent)
	verificationEvents := application.NewVerificationEventService(repos.VerificationEvent, repos.Agent, application.NewDriftDetectionService(repos.Agent, repos.Alert))
	agentService := application.NewAgentService(repos.Agent, trustCalc, repos.TrustScore, nil, repos.Alert, policyService, repos.Capability,
		verificationEvents, repos.Tag, repos.User, repos.Organization, nil)
	authService := application.NewAuthService(repos.User, repos.Organization, repos.APIKey, repos.AuthFailure, nil, nil)
	fga := application.NewFGAEngine(db, agentService, nil)
	t.Cleanup(func() { _ = fga.Shutdown(ctx) })
	secretsService := application.NewSecretsService(repos.SecretNamespace, repos.SecretAudit, repos.Agent, repos.Capability, nil,
		map[domainsecrets.BackendType]domainsecrets.SecretsBackend{
			domainsecrets.BackendTypeAIMNative: infrasecrets.NewAIMNativeBackend(repos.SecretCredential),
		})
	services := &Services{
		Auth:              authService,
		Audit:             auditService,
		Alert:             alertService,
		Agent:             agentService,
		SecurityPolicy:    policyService,
		VerificationEvent: verificationEvents,
		FGA:               fga,
		Secrets:           secretsService,
	}
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, jwtService, repos.Organization, auditService, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(oauthRepo, repos.User, repos.Organization, auditService, nil),
			authService, jwtService, auditService),
		Authorize:    handlers.NewAuthorizeHandler(fga, repos.Agent),
		Verification: handlers.NewVerificationHandler(agentService, auditService, alertService, trustCalc, verificationEvents, repos.Organization, nil),
		Secrets:      handlers.NewSecretsHandler(secretsService, repos.Agent, jwtService, auditService),
	}
	observeSecurityDecisions(forwarder, services, h)

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, services, jwtService, nil, db)

	do := func(method, path, bearer string, body interface{}) (int, string) {
		t.Helper()
		var reader io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			require.NoError(t, err)
			reader = strings.NewReader(string(raw))
		}
		req := httptest.NewRequest(method, path, reader)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "siem-round-trip-test/1.0")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer resp.Body.Close()
		answer, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(answer)
	}

	// The verification the Python SDK sends for a decorated call: no
	// resource, an empty context, signed over the action_type, agent_id,
	// context, resource and timestamp fields.
	verify := func(agent uuid.UUID, key ed25519.PrivateKey, capability string) (int, string) {
		t.Helper()
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
		message := fmt.Sprintf(`{"action_type": "%s", "agent_id": "%s", "context": {}, "resource": null, "timestamp": "%s"}`,
			capability, agent, timestamp)
		return do(fiber.MethodPost, "/api/v1/sdk-api/verifications", "", map[string]interface{}{
			"agentId":    agent.String(),
			"capability": capability,
			"resource":   nil,
			"context":    map[string]interface{}{},
			"timestamp":  timestamp,
			"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(message))),
			"publicKey":  base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)),
		})
	}

	// The denial: db:write for an agent that holds db:read, sent twice.
	for i := 0; i < 2; i++ {
		status, body := verify(enforced, enforcedKey, "db:write")
		require.Equal(t, fiber.StatusForbidden, status, body)
		assert.Contains(t, body, `"denied"`)
	}

	// The refusal: the suspended agent's verification.
	status, body := verify(suspended, suspendedKey, "db:read")
	require.Equal(t, fiber.StatusForbidden, status, body)

	// The secrets requests, with a credential built here and never written
	// anywhere but the request bodies.
	manager, err := jwtService.GenerateAccessToken(user.String(), org.String(), email, string(domain.RoleManager))
	require.NoError(t, err)
	credential := "cred-" + uuid.NewString()
	rotated := "cred-" + uuid.NewString()
	status, body = do(fiber.MethodPost, "/api/v1/secrets/namespaces", manager, map[string]interface{}{
		"agentId":     enforced.String(),
		"namespace":   "siem-" + name,
		"backendType": "aim_native",
		"operations":  []string{"read"},
	})
	require.Equal(t, fiber.StatusCreated, status, body)
	var created struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &created))
	namespace := created.ID
	require.NotEqual(t, uuid.Nil, namespace, body)

	status, body = do(fiber.MethodPost, "/api/v1/secrets/namespaces/"+namespace.String()+"/credentials", manager,
		map[string]string{"encryptedBlob": base64.StdEncoding.EncodeToString([]byte(credential))})
	require.Equal(t, fiber.StatusCreated, status, body)
	status, body = do(fiber.MethodPost, "/api/v1/secrets/namespaces/"+namespace.String()+"/rotate", manager,
		map[string]string{"encryptedBlob": base64.StdEncoding.EncodeToString([]byte(rotated))})
	require.Equal(t, fiber.StatusOK, status, body)
	status, body = do(fiber.MethodDelete, "/api/v1/secrets/namespaces/"+namespace.String(), manager, nil)
	require.Equal(t, fiber.StatusOK, status, body)

	flushSIEMForwarding(forwarder)

	assert.Empty(t, receiver.Refusals(), "the collector refused a request")
	secrets := []string{
		credential, rotated,
		base64.StdEncoding.EncodeToString([]byte(credential)),
		base64.StdEncoding.EncodeToString([]byte(rotated)),
	}
	byType := map[string][]map[string]interface{}{}
	byAuditLogID := map[string]map[string]interface{}{}
	for _, e := range receiver.Accepted() {
		assert.Equal(t, "opena2a:aim", e.Sourcetype)
		assert.Equal(t, "opena2a-aim", e.Source)
		assert.Equal(t, "aim_audit", e.Index)
		for _, secret := range secrets {
			assert.NotContains(t, string(e.Event), secret, "an event carries the credential")
		}
		var event map[string]interface{}
		require.NoError(t, e.Decode(&event))
		eventType, _ := event["eventType"].(string)
		byType[eventType] = append(byType[eventType], event)
		if details, ok := event["details"].(map[string]interface{}); ok {
			if id, ok := details["auditLogId"].(string); ok {
				byAuditLogID[id] = event
			}
		}
	}

	// Alerts: one event for each row the two denials inserted, none for the
	// repeat counted on an open alert.
	alertSeverity := map[string]string{"info": "info", "warning": "medium", "high": "high", "critical": "critical"}
	rows, err := db.QueryContext(ctx,
		`SELECT id, alert_type, severity, resource_type, resource_id, occurrence_count FROM alerts WHERE organization_id = $1`, org)
	require.NoError(t, err)
	type alertRow struct {
		id, alertType, severity, resourceType, resourceID string
		occurrences                                       int
	}
	alertRows := map[string]alertRow{}
	repeats := 0
	for rows.Next() {
		var r alertRow
		require.NoError(t, rows.Scan(&r.id, &r.alertType, &r.severity, &r.resourceType, &r.resourceID, &r.occurrences))
		alertRows[r.id] = r
		repeats += r.occurrences - 1
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.NotEmpty(t, alertRows, "the denial raised no alert")
	assert.Positive(t, repeats, "the second denial counted a repeat on an open alert")

	alertEvents := byType["aim.alert.created"]
	assert.Len(t, alertEvents, len(alertRows), "one aim.alert.created event for each alert row")
	forwarded := map[string]int{}
	for _, event := range alertEvents {
		details := event["details"].(map[string]interface{})
		id, _ := details["alertId"].(string)
		forwarded[id]++
		row, ok := alertRows[id]
		if !assert.True(t, ok, "event for an alert that is not a row: %v", event) {
			continue
		}
		assert.Equal(t, alertSeverity[row.severity], event["severity"])
		assert.Equal(t, enforced.String(), event["agentId"])
		assert.Equal(t, org.String(), details["organizationId"])
		assert.Equal(t, row.alertType, details["alertType"])
		assert.Equal(t, row.resourceType, details["resourceType"])
		assert.Equal(t, row.resourceID, details["resourceId"])
	}
	for id := range alertRows {
		assert.Equal(t, 1, forwarded[id], "alert %s forwarded once", id)
	}

	// The refusal: its audit log entry, forwarded once.
	var refusalRow string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT id FROM audit_logs WHERE organization_id = $1 AND action = $2`,
		org, string(domain.AuditActionVerificationRefused)).Scan(&refusalRow))
	refusals := byType["aim.audit.verification_refused"]
	require.Len(t, refusals, 1)
	refusalDetails := refusals[0]["details"].(map[string]interface{})
	assert.Equal(t, refusalRow, refusalDetails["auditLogId"])
	assert.Equal(t, "agent", refusalDetails["actorType"])
	assert.Equal(t, "agent_action", refusalDetails["resourceType"])
	assert.Equal(t, suspended.String(), refusals[0]["agentId"])
	assert.Equal(t, "medium", refusals[0]["severity"])

	// Secrets: one audit row for each request, in the caller's organization and
	// under the caller's user, naming the namespace and its backend only.
	rows, err = db.QueryContext(ctx, `SELECT id, action, resource_type, resource_id, user_id, metadata::text FROM audit_logs
		WHERE organization_id = $1 AND resource_type IN ('secret_namespace', 'secret_credential') ORDER BY timestamp`, org)
	require.NoError(t, err)
	type secretsRow struct{ id, action, resourceType, resourceID, userID, metadata string }
	var secretsRows []secretsRow
	for rows.Next() {
		var r secretsRow
		var userID sql.NullString
		require.NoError(t, rows.Scan(&r.id, &r.action, &r.resourceType, &r.resourceID, &userID, &r.metadata))
		r.userID = userID.String
		secretsRows = append(secretsRows, r)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	var got [][2]string
	for _, r := range secretsRows {
		got = append(got, [2]string{r.action, r.resourceType})
	}
	require.Equal(t, [][2]string{
		{"create", "secret_namespace"},
		{"create", "secret_credential"},
		{"update", "secret_credential"},
		{"delete", "secret_namespace"},
	}, got, "the secrets audit rows, in request order")
	secretsEvents := 0
	for _, events := range byType {
		for _, event := range events {
			details, _ := event["details"].(map[string]interface{})
			if rt, _ := details["resourceType"].(string); rt == "secret_namespace" || rt == "secret_credential" {
				secretsEvents++
			}
		}
	}
	assert.Equal(t, len(secretsRows), secretsEvents, "one event for each secrets audit row")
	for _, r := range secretsRows {
		assert.Equal(t, namespace.String(), r.resourceID)
		assert.Equal(t, user.String(), r.userID)
		assert.JSONEq(t, `{"backendType":"aim_native"}`, r.metadata)
		event, ok := byAuditLogID[r.id]
		if !assert.True(t, ok, "audit row %s (%s %s) was not forwarded", r.id, r.action, r.resourceType) {
			continue
		}
		assert.Equal(t, "aim.audit."+r.action, event["eventType"])
		details := event["details"].(map[string]interface{})
		assert.Equal(t, r.resourceType, details["resourceType"])
		assert.Equal(t, namespace.String(), details["resourceId"])
		assert.Equal(t, org.String(), details["organizationId"])
		assert.Equal(t, user.String(), details["userId"])
		assert.Equal(t, map[string]interface{}{"backendType": "aim_native"}, details["metadata"])
	}

	// No audit row of the organization carries the credential.
	rows, err = db.QueryContext(ctx, `SELECT COALESCE(metadata::text, '') FROM audit_logs WHERE organization_id = $1`, org)
	require.NoError(t, err)
	for rows.Next() {
		var metadata string
		require.NoError(t, rows.Scan(&metadata))
		for _, secret := range secrets {
			assert.NotContains(t, metadata, secret, "an audit row carries the credential")
		}
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
}
