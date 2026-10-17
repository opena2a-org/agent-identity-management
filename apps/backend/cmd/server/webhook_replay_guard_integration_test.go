//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

// The refusals of webhookReceiver, one for each check docs/API.md
// § Verifying a delivery names.
var (
	errReceiverSignature = errors.New("the signature does not verify")
	errReceiverStale     = errors.New("the timestamp is more than 5 minutes from the receiver's clock")
	errReceiverReplay    = errors.New("the event was already processed")
)

// webhookReceiver checks deliveries the way docs/API.md § Verifying a delivery
// tells a receiver to. It is written from that text and does not call the
// server's signing code, so a delivery it accepts is one a receiver following
// the documentation accepts.
type webhookReceiver struct {
	mu sync.Mutex
	// secrets holds each webhook's secret by its X-Webhook-ID.
	secrets map[string]string
	// now is the receiver's clock.
	now func() time.Time
	// processed holds X-Webhook-ID and payload id of each event processed.
	processed map[string]bool
}

func newWebhookReceiver(now func() time.Time) *webhookReceiver {
	return &webhookReceiver{secrets: map[string]string{}, now: now, processed: map[string]bool{}}
}

func (r *webhookReceiver) addWebhook(id, secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[id] = secret
}

// withClock returns a receiver with the same secrets, nothing processed yet,
// and the clock now.
func (r *webhookReceiver) withClock(now func() time.Time) *webhookReceiver {
	r.mu.Lock()
	defer r.mu.Unlock()
	other := newWebhookReceiver(now)
	for id, secret := range r.secrets {
		other.secrets[id] = secret
	}
	return other
}

// accept checks a delivery and processes it: it returns nil for a delivery
// the documentation says to process, and the refusal otherwise.
func (r *webhookReceiver) accept(d recordedDelivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	webhookID := d.header.Get("X-Webhook-ID")
	secret, known := r.secrets[webhookID]
	if !known {
		return fmt.Errorf("no secret for webhook %q", webhookID)
	}

	// 1. v1= and the hex HMAC-SHA256 of "<X-Webhook-Timestamp>.<raw body>",
	// compared in constant time with each entry of X-Webhook-Signature.
	timestamp := d.header.Get("X-Webhook-Timestamp")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(d.body)
	expected := []byte("v1=" + hex.EncodeToString(mac.Sum(nil)))
	verified := false
	for _, entry := range strings.Split(d.header.Get("X-Webhook-Signature"), ",") {
		if subtle.ConstantTimeCompare(expected, []byte(strings.TrimSpace(entry))) == 1 {
			verified = true
		}
	}
	if !verified {
		return errReceiverSignature
	}

	// 2. A timestamp more than 5 minutes before or after the receiver's clock.
	sent, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %v", errReceiverStale, err)
	}
	if skew := r.now().Sub(time.Unix(sent, 0)); skew > 5*time.Minute || skew < -5*time.Minute {
		return errReceiverStale
	}

	// 3. An event already processed for this webhook is not processed again.
	// A webhook.test delivery has no id and no event to process.
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(d.body, &payload); err != nil {
		return err
	}
	if payload.ID == "" {
		if d.header.Get("X-Webhook-Event") == "webhook.test" {
			return nil
		}
		return errors.New("an event delivery without an id")
	}
	key := webhookID + "/" + payload.ID
	if r.processed[key] {
		return errReceiverReplay
	}
	r.processed[key] = true
	return nil
}

// receivedAttempt is one request the receiver was sent, the status it
// answered, and its verdict.
type receivedAttempt struct {
	delivery recordedDelivery
	status   int
	verdict  error
}

// payloadID returns the id of a delivery's payload.
func payloadID(t *testing.T, d recordedDelivery) string {
	t.Helper()
	var payload struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(d.body, &payload), string(d.body))
	return payload.ID
}

// sentAt returns a delivery's X-Webhook-Timestamp.
func sentAt(t *testing.T, d recordedDelivery) int64 {
	t.Helper()
	sent, err := strconv.ParseInt(d.header.Get("X-Webhook-Timestamp"), 10, 64)
	require.NoError(t, err, "X-Webhook-Timestamp is Unix seconds: %q", d.header.Get("X-Webhook-Timestamp"))
	return sent
}

// Each attempt of a webhook delivery, the retry of a failed one and a test
// delivery included, is signed when it is sent: X-Webhook-Timestamp carries the
// sending time in Unix seconds and X-Webhook-Signature carries v1= and the hex
// HMAC-SHA256 of "<timestamp>.<body>" under the secret from the create answer.
// A receiver that follows docs/API.md § Verifying a delivery accepts an
// agent.created delivery, raised by POST /api/v1/agents through the server's
// route table against a migrated database, and refuses the same request sent
// again, the request with its timestamp changed, and the request checked more
// than 5 minutes after it was sent. A first attempt the receiver answers 500 is
// retried with a newer timestamp, a signature valid for it and the same payload
// id, and the retry is accepted. The Python example of that section verifies
// the captured deliveries.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestWebhookReplayGuardRoundTrip ./cmd/server/
func TestWebhookReplayGuardRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping webhook replay guard integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "webhook-integration-secret-of-at-least-32-chars")

	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	name := "webhook-replay-" + uuid.NewString()[:8]
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

	// The receiver: a TLS server on loopback that checks every request it is
	// sent. For a webhook in failFirst it answers the first request 500
	// before checking it, as a receiver that fails before processing does.
	receiver := newWebhookReceiver(time.Now)
	attempts := make(chan receivedAttempt, 64)
	var failMu sync.Mutex
	failFirst := map[string]bool{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		attempt := receivedAttempt{delivery: recordedDelivery{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body}}
		webhookID := r.Header.Get("X-Webhook-ID")
		failMu.Lock()
		fail := failFirst[webhookID]
		delete(failFirst, webhookID)
		failMu.Unlock()
		switch {
		case fail:
			attempt.status = http.StatusInternalServerError
		default:
			attempt.verdict = receiver.accept(attempt.delivery)
			attempt.status = http.StatusNoContent
			if attempt.verdict != nil && !errors.Is(attempt.verdict, errReceiverReplay) {
				attempt.status = http.StatusUnauthorized
			}
		}
		w.WriteHeader(attempt.status)
		select {
		case attempts <- attempt:
		default:
		}
	}))
	t.Cleanup(server.Close)
	serverAddr := server.Listener.Addr().String()

	// The webhook service main builds, with retries 1.5 seconds apart, so a
	// retry is sent in a later second than the attempt before it, and a
	// delivery client that dials the receiver for any host.
	webhookService := application.NewWebhookService(repos.Webhook)
	config := application.DefaultWebhookConfig()
	config.DefaultRetryDelay = 1500 * time.Millisecond
	config.MaxRetryDelay = 1500 * time.Millisecond
	webhookService.SetConfig(config)
	webhookService.SetDeliveryClientForIntegrationTest(func(timeout time.Duration) *http.Client {
		transport := server.Client().Transport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, serverAddr)
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
	createWebhook := func(hookName string) (uuid.UUID, string) {
		t.Helper()
		status, body := do(fiber.MethodPost, "/api/v1/webhooks",
			`{"name":"`+hookName+`","url":"https://example.com/aim-webhook","events":["agent.created"]}`)
		require.Equal(t, fiber.StatusCreated, status, body)
		var created struct {
			ID     uuid.UUID `json:"id"`
			Secret string    `json:"secret"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &created), body)
		require.NotEmpty(t, created.Secret, "the create answer carries the webhook's secret: %s", body)
		receiver.addWebhook(created.ID.String(), created.Secret)
		return created.ID, created.Secret
	}

	// Attempts are read by webhook; an attempt for another webhook waits
	// until it is asked for.
	pending := map[string][]receivedAttempt{}
	next := func(webhookID uuid.UUID) receivedAttempt {
		t.Helper()
		deadline := time.After(15 * time.Second)
		for {
			if queued := pending[webhookID.String()]; len(queued) > 0 {
				pending[webhookID.String()] = queued[1:]
				return queued[0]
			}
			select {
			case attempt := <-attempts:
				id := attempt.delivery.header.Get("X-Webhook-ID")
				pending[id] = append(pending[id], attempt)
			case <-deadline:
				t.Fatalf("no delivery attempt for webhook %s within 15s", webhookID)
			}
		}
	}

	acceptedID, acceptedSecret := createWebhook(name + "-accepted")
	retriedID, retriedSecret := createWebhook(name + "-retried")
	failMu.Lock()
	failFirst[retriedID.String()] = true
	failMu.Unlock()

	status, body := do(fiber.MethodPost, "/api/v1/agents",
		`{"name":"`+name+`-agent","displayName":"Webhook Replay Agent","agentType":"ai_agent"}`)
	require.Equal(t, fiber.StatusCreated, status, body)

	// The agent.created delivery carries the sending time and a v1 signature,
	// and the receiver accepts it with the secret from the create answer.
	accepted := next(acceptedID)
	delivery := accepted.delivery
	assert.Equal(t, "agent.created", delivery.header.Get("X-Webhook-Event"))
	assert.Equal(t, acceptedID.String(), delivery.header.Get("X-Webhook-ID"))
	assert.Regexp(t, regexp.MustCompile(`^[0-9]+$`), delivery.header.Get("X-Webhook-Timestamp"))
	assert.Regexp(t, regexp.MustCompile(`^v1=[0-9a-f]{64}$`), delivery.header.Get("X-Webhook-Signature"))
	require.NoError(t, accepted.verdict, "a receiver following docs/API.md accepts the delivery (X-Webhook-Timestamp %q, X-Webhook-Signature %q)",
		delivery.header.Get("X-Webhook-Timestamp"), delivery.header.Get("X-Webhook-Signature"))
	assert.Equal(t, http.StatusNoContent, accepted.status)
	sent := sentAt(t, delivery)
	assert.InDelta(t, time.Now().Unix(), sent, 60, "X-Webhook-Timestamp is the time the attempt was sent")
	eventID := payloadID(t, delivery)
	require.NotEmpty(t, eventID)

	// The same request sent a second time is refused as a replay.
	assert.ErrorIs(t, receiver.accept(delivery), errReceiverReplay)

	// The request with its timestamp changed fails the signature check.
	changed := recordedDelivery{method: delivery.method, path: delivery.path, header: delivery.header.Clone(), body: delivery.body}
	changed.header.Set("X-Webhook-Timestamp", strconv.FormatInt(sent+1, 10))
	assert.ErrorIs(t, receiver.withClock(time.Now).accept(changed), errReceiverSignature)

	// Checked more than 5 minutes after or before its timestamp, the request
	// is refused; checked 5 minutes after it, it is accepted.
	at := func(offset time.Duration) func() time.Time {
		return func() time.Time { return time.Unix(sent, 0).Add(offset) }
	}
	assert.ErrorIs(t, receiver.withClock(at(5*time.Minute+time.Second)).accept(delivery), errReceiverStale)
	assert.ErrorIs(t, receiver.withClock(at(-5*time.Minute-time.Second)).accept(delivery), errReceiverStale)
	assert.NoError(t, receiver.withClock(at(5*time.Minute)).accept(delivery))

	// The first attempt to the second webhook is answered 500; its retry
	// carries a newer timestamp, a signature valid for it and the same
	// payload id, which is the id the first webhook received, and is accepted.
	first := next(retriedID)
	assert.Equal(t, http.StatusInternalServerError, first.status)
	assert.NoError(t, receiver.withClock(at(0)).accept(first.delivery), "the first attempt is signed for its own timestamp")
	retry := next(retriedID)
	assert.NoError(t, retry.verdict, "the retry of an event the receiver did not process is accepted")
	assert.Equal(t, http.StatusNoContent, retry.status)
	assert.Greater(t, sentAt(t, retry.delivery), sentAt(t, first.delivery), "the retry is signed when it is sent")
	assert.NotEqual(t, first.delivery.header.Get("X-Webhook-Signature"), retry.delivery.header.Get("X-Webhook-Signature"))
	assert.Equal(t, eventID, payloadID(t, first.delivery))
	assert.Equal(t, eventID, payloadID(t, retry.delivery))
	assert.Equal(t, first.delivery.body, retry.delivery.body, "every attempt of one event sends the same body")
	assert.ErrorIs(t, receiver.accept(retry.delivery), errReceiverReplay)

	// A test delivery is signed the same way.
	status, body = do(fiber.MethodPost, "/api/v1/webhooks/"+acceptedID.String()+"/test", "")
	require.Equal(t, fiber.StatusOK, status, body)
	test := next(acceptedID)
	assert.Equal(t, "webhook.test", test.delivery.header.Get("X-Webhook-Event"))
	assert.Regexp(t, regexp.MustCompile(`^v1=[0-9a-f]{64}$`), test.delivery.header.Get("X-Webhook-Signature"))
	assert.NoError(t, test.verdict, "a receiver following docs/API.md accepts the test delivery")
	assert.Contains(t, body, `"success":true`)

	// The Python example of docs/API.md verifies the captured deliveries and
	// refuses the changed and the late one.
	type exampleCase struct {
		Name      string  `json:"name"`
		Secret    string  `json:"secret"`
		Timestamp string  `json:"timestamp"`
		Signature string  `json:"signature"`
		BodyHex   string  `json:"bodyHex"`
		Now       float64 `json:"now"`
	}
	exampleOf := func(caseName, secret string, d recordedDelivery, now float64) exampleCase {
		return exampleCase{Name: caseName, Secret: secret, Timestamp: d.header.Get("X-Webhook-Timestamp"),
			Signature: d.header.Get("X-Webhook-Signature"), BodyHex: hex.EncodeToString(d.body), Now: now}
	}
	nowSeconds := float64(time.Now().Unix())
	cases := []exampleCase{
		exampleOf("agent.created", acceptedSecret, delivery, nowSeconds),
		exampleOf("changed timestamp", acceptedSecret, changed, nowSeconds),
		exampleOf("checked 301 seconds after it was sent", acceptedSecret, delivery, float64(sent+301)),
		exampleOf("another webhook's secret", retriedSecret, delivery, nowSeconds),
		exampleOf("retry", retriedSecret, retry.delivery, nowSeconds),
		exampleOf("webhook.test", acceptedSecret, test.delivery, nowSeconds),
	}
	input, err := json.Marshal(cases)
	require.NoError(t, err)
	_, err = exec.LookPath("python3")
	require.NoError(t, err, "python3 runs the docs/API.md verification example")
	example := exec.CommandContext(ctx, "python3", "-I", "cmd/server/testdata/webhook_signature_example.py")
	example.Dir = "../.."
	example.Stdin = bytes.NewReader(input)
	output, err := example.CombinedOutput()
	require.NoError(t, err, string(output))
	t.Logf("python3 -I cmd/server/testdata/webhook_signature_example.py:\n%s", output)
	assert.Equal(t, strings.Join([]string{
		"agent.created: verified",
		"changed timestamp: refused",
		"checked 301 seconds after it was sent: refused",
		"another webhook's secret: refused",
		"retry: verified",
		"webhook.test: verified",
	}, "\n")+"\n", string(output))
}
