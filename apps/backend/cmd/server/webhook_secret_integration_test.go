//go:build integration

package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// recordedDelivery is one request the receiver was sent.
type recordedDelivery struct {
	method string
	path   string
	header http.Header
	body   []byte
}

// signatureVerifies checks X-Webhook-Signature the way a receiver does: v1=
// and the hex HMAC-SHA256 of "<X-Webhook-Timestamp>.<request body>" under the
// webhook's secret.
func signatureVerifies(secret string, delivery recordedDelivery) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(delivery.header.Get("X-Webhook-Timestamp") + "."))
	mac.Write(delivery.body)
	want := "v1=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(delivery.header.Get("X-Webhook-Signature")))
}

// A webhook created through POST /api/v1/webhooks hands its signing secret to
// the caller once, in the 201 answer, and an agent.created delivery, raised by
// POST /api/v1/agents through the server's route table against a migrated
// database, reaches a loopback TLS receiver signed so that secret verifies it.
// A caller-chosen secret of 40 characters is stored as given; one of 31 is
// refused. An event the server does not know and a field the create and update
// bodies do not have are refused with 400 naming them. GET never returns the
// secret. After POST /api/v1/webhooks/:id/rotate-secret, the next delivery
// verifies with the new secret and not with the old one, and the rotation's
// audit row does not carry the secret.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestWebhookSecretRoundTrip ./cmd/server/
func TestWebhookSecretRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping webhook secret integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "webhook-integration-secret-of-at-least-32-chars")

	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	name := "webhook-secret-" + uuid.NewString()[:8]
	email := name + "@example.invalid"

	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
		org, name, name+".invalid")
	require.NoError(t, err)
	t.Cleanup(func() {
		agents := `SELECT id FROM agents WHERE organization_id = $1`
		_, _ = db.ExecContext(ctx, `DELETE FROM webhook_deliveries WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM webhooks WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM api_keys WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_capabilities WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id IN (`+agents+`)`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM verification_events WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, org)
	})
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
		VALUES ($1, $2, $3, 'webhook test user', 'local', $4, 'manager', 'x', 'active')`,
		user, org, email, user.String())
	require.NoError(t, err)

	repos, _ := initRepositories(db)

	// The receiver: a TLS server on loopback that records every request.
	received := make(chan recordedDelivery, 64)
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- recordedDelivery{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(receiver.Close)
	receiverAddr := receiver.Listener.Addr().String()

	// The webhook service main builds, with a delivery client that dials the
	// receiver for any host and trusts its certificate, which names
	// example.com. The egress client would refuse the loopback address.
	webhookService := application.NewWebhookService(repos.Webhook)
	webhookService.SetDeliveryClientForIntegrationTest(func(timeout time.Duration) *http.Client {
		transport := receiver.Client().Transport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, receiverAddr)
		}
		return &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})
	t.Cleanup(func() { webhookService.Stop(5 * time.Second) })

	// The services POST /api/v1/agents reaches, built from the constructors
	// initServices uses, with the webhook service wired as main wires it.
	masterKey := make([]byte, 32)
	_, err = rand.Read(masterKey)
	require.NoError(t, err)
	keyVault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)
	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	alertService := application.NewAlertService(repos.Alert, repos.Agent, db)
	policyService := application.NewSecurityPolicyService(repos.SecurityPolicy, repos.Alert, repos.AuditLog)
	policyService.SetVerificationEventRepo(repos.VerificationEvent)
	policyService.SetCapabilityRepository(repos.Capability)
	policyService.SetAgentRepository(repos.Agent)
	trustCalc := application.NewTrustCalculatorWithVerification(repos.TrustScore, repos.APIKey, repos.AuditLog, repos.Capability, repos.Agent, repos.Alert, repos.VerificationEvent)
	verificationEvents := application.NewVerificationEventService(repos.VerificationEvent, repos.Agent, application.NewDriftDetectionService(repos.Agent, repos.Alert))
	agentService := application.NewAgentService(repos.Agent, trustCalc, repos.TrustScore, keyVault, repos.Alert, policyService, repos.Capability,
		verificationEvents, repos.Tag, repos.User, repos.Organization, nil)
	apiKeyService := application.NewAPIKeyService(repos.APIKey, repos.Agent)
	tagService := application.NewTagService(repos.Tag, repos.Agent, repos.MCPServer)
	capabilityService := application.NewCapabilityService(repos.Capability, repos.Agent, repos.AuditLog, repos.Alert, trustCalc, repos.TrustScore)
	alertService.SetWebhookService(webhookService)
	agentService.SetWebhookService(webhookService)
	apiKeyService.SetWebhookService(webhookService)

	services := &Services{
		Agent:             agentService,
		APIKey:            apiKeyService,
		Audit:             auditService,
		Alert:             alertService,
		SecurityPolicy:    policyService,
		Webhook:           webhookService,
		VerificationEvent: verificationEvents,
		Tag:               tagService,
		Capability:        capabilityService,
		Secrets:           &application.SecretsService{},
	}
	h := &Handlers{
		Agent: handlers.NewAgentHandler(agentService, nil, auditService, apiKeyService, nil, alertService, verificationEvents,
			capabilityService, tagService, repos.Organization, nil, jwtService),
		Webhook: handlers.NewWebhookHandler(webhookService, auditService, jwtService),
	}
	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, services, jwtService, nil, db)

	manager, err := jwtService.GenerateAccessToken(user.String(), org.String(), email, string(domain.RoleManager))
	require.NoError(t, err)
	do := func(method, path, body string) (int, string) {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+manager)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer resp.Body.Close()
		answer, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(answer)
	}
	decode := func(body string) map[string]interface{} {
		t.Helper()
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &out), body)
		return out
	}
	const hookURL = "https://example.com/aim-webhook"
	createBody := func(hookName string, extra string) string {
		return `{"name":"` + hookName + `","url":"` + hookURL + `","events":["agent.created"]` + extra + `}`
	}

	// A webhook with a generated secret, which the 201 answer carries once.
	status, body := do(fiber.MethodPost, "/api/v1/webhooks", createBody(name+"-generated", ""))
	require.Equal(t, fiber.StatusCreated, status, body)
	created := decode(body)
	generatedID, err := uuid.Parse(created["id"].(string))
	require.NoError(t, err)
	generatedSecret, generatedHasSecret := created["secret"].(string)
	assert.True(t, generatedHasSecret, "the create answer carries the webhook's secret: %s", body)
	assert.Len(t, generatedSecret, 64, "a generated secret is 32 random bytes in hex")

	// A caller-chosen secret of 40 characters is stored as given.
	chosenSecret := "receiver-chosen-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
	require.Len(t, chosenSecret, 40)
	status, body = do(fiber.MethodPost, "/api/v1/webhooks", createBody(name+"-chosen", `,"secret":"`+chosenSecret+`"`))
	require.Equal(t, fiber.StatusCreated, status, body)
	chosen := decode(body)
	chosenID, err := uuid.Parse(chosen["id"].(string))
	require.NoError(t, err)
	assert.Equal(t, chosenSecret, chosen["secret"], "the create answer carries the secret the caller chose")

	// A caller-chosen secret of 31 characters is refused.
	status, body = do(fiber.MethodPost, "/api/v1/webhooks", createBody(name+"-short", `,"secret":"`+strings.Repeat("s", 31)+`"`))
	assert.Equal(t, fiber.StatusBadRequest, status, body)
	assert.JSONEq(t, `{"error":"secret must be at least 32 characters"}`, body)

	// An event the server does not know is refused, on create and on update.
	unknownEvent := `{"name":"` + name + `-unknown-event","url":"` + hookURL + `","events":["agent.created","auth.failed"]}`
	status, body = do(fiber.MethodPost, "/api/v1/webhooks", unknownEvent)
	assert.Equal(t, fiber.StatusBadRequest, status, body)
	assert.JSONEq(t, `{"error":"unsupported event: auth.failed"}`, body)
	status, body = do(fiber.MethodPut, "/api/v1/webhooks/"+generatedID.String(), unknownEvent)
	assert.Equal(t, fiber.StatusBadRequest, status, body)
	assert.JSONEq(t, `{"error":"unsupported event: auth.failed"}`, body)

	// A field the bodies do not have is refused, on create and on update.
	status, body = do(fiber.MethodPost, "/api/v1/webhooks", createBody(name+"-headers", `,"headers":{"X-Tenant":"a"}`))
	assert.Equal(t, fiber.StatusBadRequest, status, body)
	assert.JSONEq(t, `{"error":"unknown field: headers"}`, body)
	status, body = do(fiber.MethodPut, "/api/v1/webhooks/"+generatedID.String(), createBody(name+"-generated", `,"headers":{"X-Tenant":"a"}`))
	assert.Equal(t, fiber.StatusBadRequest, status, body)
	assert.JSONEq(t, `{"error":"unknown field: headers"}`, body)

	// GET never returns the secret.
	status, body = do(fiber.MethodGet, "/api/v1/webhooks/"+generatedID.String(), "")
	require.Equal(t, fiber.StatusOK, status, body)
	assert.NotContains(t, decode(body), "secret")

	// Rotation answers the new secret once and writes an audit row without it.
	status, body = do(fiber.MethodPost, "/api/v1/webhooks", `{"name":"`+name+`-rotated","url":"`+hookURL+`","events":["api_key.revoked"]}`)
	require.Equal(t, fiber.StatusCreated, status, body)
	rotatedID := decode(body)["id"].(string)
	status, body = do(fiber.MethodPost, "/api/v1/webhooks/"+rotatedID+"/rotate-secret", "")
	assert.Equal(t, fiber.StatusOK, status, body)
	if status == fiber.StatusOK {
		rotation := decode(body)
		assert.Equal(t, rotatedID, rotation["id"])
		assert.Len(t, rotation["secret"], 64)
		var metadata string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT metadata::text FROM audit_logs
			WHERE organization_id = $1 AND resource_id = $2 AND action = 'update' AND resource_type = 'webhook'`,
			org, rotatedID).Scan(&metadata))
		assert.Contains(t, metadata, `"rotatedSecret": true`)
		assert.NotContains(t, metadata, rotation["secret"].(string))
	}

	require.True(t, generatedHasSecret, "the create answer carries no secret, so no delivery can be verified")

	// Deliveries are read by webhook; a delivery for another webhook waits
	// until it is asked for.
	pending := map[string][]recordedDelivery{}
	next := func(webhookID uuid.UUID) recordedDelivery {
		t.Helper()
		deadline := time.After(15 * time.Second)
		for {
			if queued := pending[webhookID.String()]; len(queued) > 0 {
				pending[webhookID.String()] = queued[1:]
				return queued[0]
			}
			select {
			case delivery := <-received:
				id := delivery.header.Get("X-Webhook-ID")
				pending[id] = append(pending[id], delivery)
			case <-deadline:
				t.Fatalf("no delivery for webhook %s within 15s", webhookID)
			}
		}
	}
	registerAgent := func(suffix string) string {
		t.Helper()
		status, body := do(fiber.MethodPost, "/api/v1/agents",
			`{"name":"`+name+`-agent-`+suffix+`","displayName":"Webhook Agent `+suffix+`","agentType":"ai_agent"}`)
		require.Equal(t, fiber.StatusCreated, status, body)
		return decode(body)["id"].(string)
	}
	// checkContract checks a delivery against the documented contract: a
	// POST of JSON, named in X-Webhook-Event, whose body has exactly the keys
	// id, event, timestamp, organizationId and data.
	checkContract := func(delivery recordedDelivery, agentID string) {
		t.Helper()
		assert.Equal(t, http.MethodPost, delivery.method)
		assert.Equal(t, "/aim-webhook", delivery.path)
		assert.Equal(t, "application/json", delivery.header.Get("Content-Type"))
		assert.Equal(t, "agent.created", delivery.header.Get("X-Webhook-Event"))
		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(delivery.body, &payload), string(delivery.body))
		keys := make([]string, 0, len(payload))
		for key := range payload {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		assert.Equal(t, []string{"data", "event", "id", "organizationId", "timestamp"}, keys)
		assert.Equal(t, "agent.created", payload["event"])
		assert.Equal(t, org.String(), payload["organizationId"])
		data, _ := payload["data"].(map[string]interface{})
		assert.Equal(t, agentID, data["agentId"])
	}

	// agent.created, raised by registering an agent, reaches both webhooks,
	// each signed with its own secret.
	firstAgent := registerAgent("1")
	delivery := next(generatedID)
	checkContract(delivery, firstAgent)
	assert.True(t, signatureVerifies(generatedSecret, delivery), "the generated secret from the create answer verifies the delivery")
	delivery = next(chosenID)
	checkContract(delivery, firstAgent)
	assert.True(t, signatureVerifies(chosenSecret, delivery), "the caller-chosen secret verifies the delivery")
	assert.False(t, signatureVerifies(generatedSecret, delivery), "another webhook's secret does not verify the delivery")

	// After rotation, the next delivery verifies with the new secret only.
	status, body = do(fiber.MethodPost, "/api/v1/webhooks/"+generatedID.String()+"/rotate-secret", "")
	require.Equal(t, fiber.StatusOK, status, body)
	rotation := decode(body)
	assert.Equal(t, generatedID.String(), rotation["id"])
	newSecret, _ := rotation["secret"].(string)
	require.Len(t, newSecret, 64)
	require.NotEqual(t, generatedSecret, newSecret)

	secondAgent := registerAgent("2")
	delivery = next(generatedID)
	checkContract(delivery, secondAgent)
	assert.True(t, signatureVerifies(newSecret, delivery), "the rotated secret verifies the next delivery")
	assert.False(t, signatureVerifies(generatedSecret, delivery), "the secret from before the rotation no longer verifies")
	delivery = next(chosenID)
	assert.True(t, signatureVerifies(chosenSecret, delivery), "rotating one webhook leaves another's secret as it was")
}
