//go:build integration

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// slackReceiver answers as a Slack incoming webhook does
// (docs.slack.dev, "Sending messages using incoming webhooks"): a POST with a
// JSON body carrying text is answered 200 "ok", a body without text 400
// "no_text", and a body that is not JSON 400 "invalid_payload". It records
// each accepted body under its request path.
type slackReceiver struct {
	*httptest.Server
	mu       sync.Mutex
	accepted map[string][]map[string]interface{}
}

func newSlackReceiver(t *testing.T) *slackReceiver {
	t.Helper()
	r := &slackReceiver{accepted: map[string][]map[string]interface{}{}}
	r.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "invalid_payload")
			return
		}
		var body map[string]interface{}
		if err := json.Unmarshal(raw, &body); err != nil || body == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "invalid_payload")
			return
		}
		if _, ok := body["text"].(string); !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "no_text")
			return
		}
		r.mu.Lock()
		r.accepted[req.URL.Path] = append(r.accepted[req.URL.Path], body)
		r.mu.Unlock()
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *slackReceiver) bodies(path string) []map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]interface{}(nil), r.accepted[path]...)
}

// pagerDutyRequest is one request the PagerDuty receiver answered.
type pagerDutyRequest struct {
	status int
	body   map[string]interface{}
}

// pagerDutyReceiver answers as the PagerDuty Events API v2 does at
// /v2/enqueue (docs.pagerduty.com, "Send an Alert Event"): 202 for a valid
// event, and 400 for malformed JSON, a routing_key that is missing or not 32
// characters, an event_action other than trigger, acknowledge or resolve, a
// payload without summary, source or severity, a severity other than
// critical, error, warning or info, a summary over 1024 characters, or a
// dedup_key over 255. Its first failFirst requests are answered 500.
type pagerDutyReceiver struct {
	*httptest.Server
	mu        sync.Mutex
	failFirst int
	requests  []pagerDutyRequest
}

func newPagerDutyReceiver(t *testing.T, failFirst int) *pagerDutyReceiver {
	t.Helper()
	r := &pagerDutyReceiver{failFirst: failFirst}
	r.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v2/enqueue" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, _ := io.ReadAll(req.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		status := http.StatusAccepted
		r.mu.Lock()
		if len(r.requests) < r.failFirst {
			status = http.StatusInternalServerError
		} else if !validPagerDutyEvent(raw) {
			status = http.StatusBadRequest
		}
		r.requests = append(r.requests, pagerDutyRequest{status: status, body: body})
		r.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(r.Close)
	return r
}

func validPagerDutyEvent(raw []byte) bool {
	var event struct {
		RoutingKey  *string `json:"routing_key"`
		EventAction string  `json:"event_action"`
		DedupKey    string  `json:"dedup_key"`
		Payload     *struct {
			Summary  *string `json:"summary"`
			Source   *string `json:"source"`
			Severity *string `json:"severity"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return false
	}
	if event.RoutingKey == nil || len([]rune(*event.RoutingKey)) != 32 {
		return false
	}
	switch event.EventAction {
	case "trigger", "acknowledge", "resolve":
	default:
		return false
	}
	if len([]rune(event.DedupKey)) > 255 {
		return false
	}
	p := event.Payload
	if p == nil || p.Summary == nil || *p.Summary == "" || p.Source == nil || *p.Source == "" || p.Severity == nil {
		return false
	}
	if len([]rune(*p.Summary)) > 1024 {
		return false
	}
	switch *p.Severity {
	case "critical", "error", "warning", "info":
		return true
	}
	return false
}

func (r *pagerDutyReceiver) answered() []pagerDutyRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]pagerDutyRequest(nil), r.requests...)
}

// dialByHost returns a client builder whose connections to each host:port in
// routes go to the loopback receiver it names, and which trusts the
// receivers' certificates. Any other destination is refused.
func dialByHost(routes map[string]string, certs ...*x509.Certificate) func(time.Duration) *http.Client {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: nil,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					target, ok := routes[addr]
					if !ok {
						return nil, fmt.Errorf("no loopback receiver for %s", addr)
					}
					var d net.Dialer
					return d.DialContext(ctx, network, target)
				},
				// httptest's certificate is issued for example.com.
				TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "example.com", MinVersion: tls.VersionTLS12},
			},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
}

// alertChannelsRig is a migrated database seeded with two organizations, a
// manager and a viewer in the first and a manager in the second, and one
// agent of the first, served through the real route table with the alert
// channel service the server builds. Its cleanup removes every seeded row.
type alertChannelsRig struct {
	t   *testing.T
	db  *sql.DB
	ctx context.Context

	name                      string
	org, otherOrg             uuid.UUID
	manager, viewer           uuid.UUID
	managerToken, viewerToken string
	otherToken                string
	agentID                   uuid.UUID
	agentDisplay              string
	agentPub                  ed25519.PublicKey
	agentKey                  ed25519.PrivateKey
	channels                  *application.AlertChannelService
	app                       *fiber.App
}

// newAlertChannelsRig seeds the rig and starts the alert channel service with
// frontendURL and emailService, as startAlertChannels does for the server. It
// skips the test when TEST_DATABASE_URL is not set.
func newAlertChannelsRig(t *testing.T, frontendURL string, emailService domain.EmailService) *alertChannelsRig {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping alert channel integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "alert-channel-integration-secret-of-32-chars")

	ctx := context.Background()
	org, otherOrg := uuid.New(), uuid.New()
	manager, viewer, otherManager := uuid.New(), uuid.New(), uuid.New()
	name := "alert-channels-" + uuid.NewString()[:8]

	// Monitoring mode: an agent with no granted capability is allowed and
	// raises one warning alert.
	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain, enforcement_mode) VALUES ($1, $2, $3, 'monitoring'), ($4, $5, $6, 'monitoring')`,
		org, name, name+".invalid", otherOrg, name+"-other", name+"-other.invalid")
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, o := range []uuid.UUID{org, otherOrg} {
			agents := `SELECT id FROM agents WHERE organization_id = $1`
			_, _ = db.ExecContext(ctx, `DELETE FROM alert_channels WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM capability_violations WHERE agent_id IN (`+agents+`)`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM trust_scores WHERE agent_id IN (`+agents+`)`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM verification_events WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM alerts WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE organization_id = $1`, o)
			_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, o)
		}
	})
	seedUser := func(id, orgID uuid.UUID, role domain.UserRole) string {
		t.Helper()
		email := string(role) + "-" + id.String()[:8] + "@" + name + ".invalid"
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
			VALUES ($1, $2, $3, 'alert channel test user', 'local', $4, $5, 'x', 'active')`,
			id, orgID, email, id.String(), string(role))
		require.NoError(t, err)
		return email
	}
	managerEmail := seedUser(manager, org, domain.RoleManager)
	viewerEmail := seedUser(viewer, org, domain.RoleViewer)
	otherEmail := seedUser(otherManager, otherOrg, domain.RoleManager)

	// The agent's display name carries Slack's control characters, which the
	// Slack text escapes.
	agentID := uuid.New()
	agentDisplay := name + " <ops> & co"
	pub, agentKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, public_key, trust_score, created_by)
		VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', $5, 0.95, $6)`,
		agentID, org, name+"-agent", agentDisplay, base64.StdEncoding.EncodeToString(pub), manager)
	require.NoError(t, err)

	repos, _ := initRepositories(db)
	channels := startAlertChannels(repos, frontendURL, emailService)
	t.Cleanup(channels.Stop)
	channels.SetRetryDelaysForIntegrationTest([]time.Duration{20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond})

	// The services the verification route reaches, built as initServices
	// builds them on the repositories initRepositories returns, so the alert
	// is written through the one alert repository the channels observe.
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
	fga := application.NewFGAEngine(db, agentService, nil)
	t.Cleanup(func() { _ = fga.Shutdown(ctx) })
	secretsService := application.NewSecretsService(repos.SecretNamespace, repos.SecretAudit, repos.Agent, repos.Capability, nil,
		map[domainsecrets.BackendType]domainsecrets.SecretsBackend{
			domainsecrets.BackendTypeAIMNative: infrasecrets.NewAIMNativeBackend(repos.SecretCredential),
		})
	services := &Services{
		Audit:             auditService,
		Alert:             alertService,
		Agent:             agentService,
		SecurityPolicy:    policyService,
		VerificationEvent: verificationEvents,
		FGA:               fga,
		Secrets:           secretsService,
		AlertChannel:      channels,
	}
	h := &Handlers{
		Verification: handlers.NewVerificationHandler(agentService, auditService, alertService, trustCalc, verificationEvents, repos.Organization, nil),
		AlertChannel: handlers.NewAlertChannelHandler(channels, auditService, jwtService),
	}

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(app, h, services, jwtService, nil, db)

	token := func(user, orgID uuid.UUID, email string, role domain.UserRole) string {
		t.Helper()
		tok, err := jwtService.GenerateAccessToken(user.String(), orgID.String(), email, string(role))
		require.NoError(t, err)
		return tok
	}
	return &alertChannelsRig{
		t:            t,
		db:           db,
		ctx:          ctx,
		name:         name,
		org:          org,
		otherOrg:     otherOrg,
		manager:      manager,
		viewer:       viewer,
		managerToken: token(manager, org, managerEmail, domain.RoleManager),
		viewerToken:  token(viewer, org, viewerEmail, domain.RoleViewer),
		otherToken:   token(otherManager, otherOrg, otherEmail, domain.RoleManager),
		agentID:      agentID,
		agentDisplay: agentDisplay,
		agentPub:     pub,
		agentKey:     agentKey,
		channels:     channels,
		app:          app,
	}
}

// do sends one request through the route table and returns its status and
// body.
func (r *alertChannelsRig) do(method, path, bearer string, body interface{}) (int, string) {
	r.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(r.t, err)
		reader = strings.NewReader(string(raw))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "alert-channels-round-trip-test/1.0")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := r.app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(r.t, err)
	defer resp.Body.Close()
	answer, err := io.ReadAll(resp.Body)
	require.NoError(r.t, err)
	return resp.StatusCode, string(answer)
}

// channelView is an alert channel as the routes answer it.
type channelView struct {
	ID                 uuid.UUID              `json:"id"`
	Name               string                 `json:"name"`
	Kind               string                 `json:"kind"`
	MinSeverity        string                 `json:"minSeverity"`
	IsActive           bool                   `json:"isActive"`
	Config             map[string]interface{} `json:"config"`
	LastDeliveryStatus *string                `json:"lastDeliveryStatus"`
	LastDeliveryCode   *int                   `json:"lastDeliveryCode"`
	LastDeliveryAt     *time.Time             `json:"lastDeliveryAt"`
}

// create creates a channel as the manager and returns the answer, which must
// be 201.
func (r *alertChannelsRig) create(channelName, kind string, config interface{}, floor string) channelView {
	r.t.Helper()
	status, body := r.do(fiber.MethodPost, "/api/v1/alert-channels", r.managerToken, map[string]interface{}{
		"name": channelName, "kind": kind, "config": config, "minSeverity": floor,
	})
	require.Equal(r.t, fiber.StatusCreated, status, body)
	var view channelView
	require.NoError(r.t, json.Unmarshal([]byte(body), &view))
	require.NotEqual(r.t, uuid.Nil, view.ID, body)
	return view
}

// get reads a channel as the manager.
func (r *alertChannelsRig) get(id uuid.UUID) (int, string, channelView) {
	r.t.Helper()
	status, body := r.do(fiber.MethodGet, "/api/v1/alert-channels/"+id.String(), r.managerToken, nil)
	var view channelView
	if status == fiber.StatusOK {
		require.NoError(r.t, json.Unmarshal([]byte(body), &view), body)
	}
	return status, body, view
}

// raiseWarningAlert has the agent, which holds no capability, ask for one
// through the verification route, which raises one warning alert in the
// organization. It returns the alert's id and title.
func (r *alertChannelsRig) raiseWarningAlert() (uuid.UUID, string) {
	r.t.Helper()
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
	message := fmt.Sprintf(`{"action_type": "%s", "agent_id": "%s", "context": {}, "resource": null, "timestamp": "%s"}`,
		"db:read", r.agentID, timestamp)
	status, body := r.do(fiber.MethodPost, "/api/v1/sdk-api/verifications", "", map[string]interface{}{
		"agentId":    r.agentID.String(),
		"capability": "db:read",
		"resource":   nil,
		"context":    map[string]interface{}{},
		"timestamp":  timestamp,
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(r.agentKey, []byte(message))),
		"publicKey":  base64.StdEncoding.EncodeToString(r.agentPub),
	})
	require.Less(r.t, status, 300, body)

	var alertID uuid.UUID
	var alertTitle, alertSeverity string
	require.NoError(r.t, r.db.QueryRowContext(r.ctx, `SELECT id, title, severity FROM alerts WHERE organization_id = $1`, r.org).
		Scan(&alertID, &alertTitle, &alertSeverity), "the verification raised exactly one alert")
	require.Equal(r.t, "warning", alertSeverity)
	return alertID, alertTitle
}

// An organization's alerts reach its Slack and PagerDuty alert channels in
// each vendor's published format, through the server's route table against a
// migrated database. Three channels are created through the API: Slack A with
// the floor warning, Slack B with the floor high, and PagerDuty with the
// floor info. One warning alert, raised by an agent's verification, reaches
// Slack A and PagerDuty, which answers 500 once and accepts the retry, and
// does not reach Slack B. No answer carries a webhook URL or routing key;
// a test message, the role and organization checks and the request checks
// are driven through the same routes.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAlertChannelsRoundTrip ./cmd/server/
func TestAlertChannelsRoundTrip(t *testing.T) {
	const frontendURL = "https://aim.example.test"
	r := newAlertChannelsRig(t, frontendURL, nil)
	db, ctx, org, manager := r.db, r.ctx, r.org, r.manager
	managerToken, viewerToken, otherToken := r.managerToken, r.viewerToken, r.otherToken
	agentID, agentDisplay := r.agentID, r.agentDisplay
	do, get := r.do, r.get
	var err error

	slack := newSlackReceiver(t)
	pagerDuty := newPagerDutyReceiver(t, 1)
	r.channels.SetDeliveryClientForIntegrationTest(dialByHost(map[string]string{
		"hooks.slack.com:443":      slack.Listener.Addr().String(),
		"events.pagerduty.com:443": pagerDuty.Listener.Addr().String(),
	}, slack.Certificate(), pagerDuty.Certificate()))

	// Destinations built here from fragments: two Slack webhook URLs and a
	// 32-character routing key.
	slackPathA := "/services/" + "aim-test-a-" + uuid.NewString()
	slackPathB := "/services/" + "aim-test-b-" + uuid.NewString()
	slackURLA := "https://" + "hooks.slack.com" + slackPathA
	slackURLB := "https://" + "hooks.slack.com" + slackPathB
	routingKey := strings.ReplaceAll(uuid.NewString(), "-", "")
	require.Len(t, routingKey, 32)
	secrets := []string{slackURLA, slackURLB, slackPathA, slackPathB, routingKey}

	create := func(channelName, kind string, config map[string]string, floor string) channelView {
		t.Helper()
		return r.create(channelName, kind, config, floor)
	}

	// A viewer cannot create a channel.
	status, body := do(fiber.MethodPost, "/api/v1/alert-channels", viewerToken, map[string]interface{}{
		"name": "viewer", "kind": "slack", "config": map[string]string{"webhookUrl": slackURLA}, "minSeverity": "info",
	})
	assert.Equal(t, fiber.StatusForbidden, status, body)

	slackA := create("slack-a", "slack", map[string]string{"webhookUrl": slackURLA}, "warning")
	slackB := create("slack-b", "slack", map[string]string{"webhookUrl": slackURLB}, "high")
	pd := create("pagerduty", "pagerduty", map[string]string{"routingKey": routingKey}, "info")
	assert.Equal(t, "****"+slackURLA[len(slackURLA)-4:], slackA.Config["webhookUrl"])
	assert.Equal(t, "****"+routingKey[len(routingKey)-4:], pd.Config["routingKey"])
	assert.True(t, slackA.IsActive)

	// The alert: a verification by the agent, which holds no capability.
	alertID, alertTitle := r.raiseWarningAlert()

	// Wait for both deliveries to be recorded.
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, _, a := get(slackA.ID)
		_, _, p := get(pd.ID)
		if a.LastDeliveryStatus != nil && p.LastDeliveryStatus != nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "the alert's deliveries were not recorded")
		time.Sleep(50 * time.Millisecond)
	}

	// Slack A: one message in Slack's format, with the alert's strings escaped.
	escapedTitle := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(alertTitle)
	gotA := slack.bodies(slackPathA)
	require.Len(t, gotA, 1, "Slack A gets the warning alert once")
	text, _ := gotA[0]["text"].(string)
	assert.Contains(t, text, escapedTitle)
	assert.Contains(t, text, "AIM alert, severity warning: ")
	assert.Contains(t, text, "Agent: "+strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(agentDisplay))
	assert.Contains(t, text, frontendURL+"/dashboard/admin/alerts")
	assert.NotContains(t, text, "<ops>")
	assert.NotContains(t, gotA[0], "blocks")

	// Slack B's floor is high: it gets nothing.
	assert.Empty(t, slack.bodies(slackPathB), "Slack B's floor is above the alert's severity")

	// PagerDuty: a 500, then the retry accepted.
	got := pagerDuty.answered()
	require.Len(t, got, 2, "PagerDuty gets the first attempt and one retry")
	assert.Equal(t, http.StatusInternalServerError, got[0].status)
	assert.Equal(t, http.StatusAccepted, got[1].status, "the PagerDuty receiver refused the event: %v", got[1].body)
	event := got[1].body
	assert.Equal(t, routingKey, event["routing_key"])
	assert.Equal(t, "trigger", event["event_action"])
	assert.Equal(t, alertID.String(), event["dedup_key"])
	payload, _ := event["payload"].(map[string]interface{})
	require.NotNil(t, payload, "%v", event)
	assert.Equal(t, "warning", payload["severity"])
	assert.Equal(t, agentDisplay, payload["source"])
	assert.Equal(t, alertTitle, payload["summary"])
	assert.Equal(t, string(domain.AlertSecurityBreach), payload["class"])
	details, _ := payload["custom_details"].(map[string]interface{})
	require.NotNil(t, details, "%v", payload)
	assert.Equal(t, alertID.String(), details["alertId"])
	assert.Equal(t, org.String(), details["organizationId"])
	assert.Equal(t, "warning", details["severity"])
	assert.Equal(t, agentID.String(), details["resourceId"])
	stamp, _ := payload["timestamp"].(string)
	_, err = time.Parse(time.RFC3339, stamp)
	assert.NoError(t, err, "payload.timestamp %q is RFC 3339", stamp)

	// Each channel's last delivery.
	_, _, a := get(slackA.ID)
	_, _, p := get(pd.ID)
	_, _, b := get(slackB.ID)
	require.NotNil(t, a.LastDeliveryStatus)
	require.NotNil(t, p.LastDeliveryStatus)
	assert.Equal(t, "delivered", *a.LastDeliveryStatus)
	assert.Equal(t, "delivered", *p.LastDeliveryStatus)
	require.NotNil(t, a.LastDeliveryCode)
	require.NotNil(t, p.LastDeliveryCode)
	assert.Equal(t, http.StatusOK, *a.LastDeliveryCode)
	assert.Equal(t, http.StatusAccepted, *p.LastDeliveryCode)
	assert.NotNil(t, a.LastDeliveryAt)
	assert.Nil(t, b.LastDeliveryStatus)

	// No answer carries a destination.
	status, listBody := do(fiber.MethodGet, "/api/v1/alert-channels", viewerToken, nil)
	require.Equal(t, fiber.StatusOK, status, listBody)
	raws := []string{listBody}
	for _, id := range []uuid.UUID{slackA.ID, slackB.ID, pd.ID} {
		status, raw, _ := get(id)
		require.Equal(t, fiber.StatusOK, status, raw)
		raws = append(raws, raw)
	}
	for _, raw := range raws {
		for _, secret := range secrets {
			assert.NotContains(t, raw, secret, "an answer carries a destination")
		}
	}
	assert.Contains(t, listBody, `"****`+routingKey[len(routingKey)-4:]+`"`)

	// A test message through Slack A.
	status, body = do(fiber.MethodPost, "/api/v1/alert-channels/"+slackA.ID.String()+"/test", managerToken, nil)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.JSONEq(t, `{"delivered": true, "statusCode": 200}`, body)
	gotA = slack.bodies(slackPathA)
	require.Len(t, gotA, 2)
	assert.Equal(t, "AIM test message for alert channel slack-a", gotA[1]["text"])

	// An update without config keeps the stored destination.
	status, body = do(fiber.MethodPut, "/api/v1/alert-channels/"+slackB.ID.String(), managerToken, map[string]interface{}{
		"minSeverity": "critical",
	})
	require.Equal(t, fiber.StatusOK, status, body)
	var updated channelView
	require.NoError(t, json.Unmarshal([]byte(body), &updated))
	assert.Equal(t, "critical", updated.MinSeverity)
	assert.Equal(t, slackB.Config, updated.Config)

	// Another organization's manager reaches none of the channels.
	for _, req := range [][2]string{
		{fiber.MethodGet, "/api/v1/alert-channels/" + slackA.ID.String()},
		{fiber.MethodPost, "/api/v1/alert-channels/" + slackA.ID.String() + "/test"},
		{fiber.MethodDelete, "/api/v1/alert-channels/" + slackA.ID.String()},
	} {
		status, body := do(req[0], req[1], otherToken, nil)
		assert.Equal(t, fiber.StatusNotFound, status, "%s %s: %s", req[0], req[1], body)
	}
	status, body = do(fiber.MethodGet, "/api/v1/alert-channels", otherToken, nil)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.NotContains(t, body, slackA.ID.String())

	// Requests the server refuses, each with its reason.
	refusals := []struct {
		body   map[string]interface{}
		reason string
	}{
		{map[string]interface{}{"name": "sms", "kind": "sms", "config": map[string]string{}, "minSeverity": "info"}, "unsupported kind: sms"},
		{map[string]interface{}{"name": "x", "kind": "slack", "config": map[string]string{"webhookUrl": slackURLA}, "minSeverity": "info", "color": "red"}, "unknown field: color"},
		{map[string]interface{}{"name": "x", "kind": "slack", "config": map[string]string{"webhookUrl": slackURLA, "channel": "#x"}, "minSeverity": "info"}, "unknown field: config.channel"},
		{map[string]interface{}{"name": "x", "kind": "slack", "config": map[string]string{"webhookUrl": "https://example.com/x"}, "minSeverity": "info"}, "config.webhookUrl"},
		{map[string]interface{}{"name": "x", "kind": "pagerduty", "config": map[string]string{"routingKey": "short"}, "minSeverity": "info"}, "config.routingKey"},
		{map[string]interface{}{"name": "x", "kind": "slack", "config": map[string]string{"webhookUrl": slackURLA}, "minSeverity": "medium"}, "minSeverity"},
	}
	for _, r := range refusals {
		status, body := do(fiber.MethodPost, "/api/v1/alert-channels", managerToken, r.body)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		var answer map[string]string
		require.NoError(t, json.Unmarshal([]byte(body), &answer), body)
		assert.Contains(t, answer["error"], r.reason)
	}

	// Delete answers 204, and the channel is gone.
	status, body = do(fiber.MethodDelete, "/api/v1/alert-channels/"+slackB.ID.String(), managerToken, nil)
	require.Equal(t, fiber.StatusNoContent, status, body)
	status, body, _ = get(slackB.ID)
	assert.Equal(t, fiber.StatusNotFound, status, body)

	// One audit row for each create, update and delete, carrying the kind and
	// floor only.
	rows, err := db.QueryContext(ctx, `SELECT action, resource_id, user_id, metadata::text FROM audit_logs
		WHERE organization_id = $1 AND resource_type = 'alert_channel' ORDER BY timestamp`, org)
	require.NoError(t, err)
	defer rows.Close()
	var audited [][2]string
	for rows.Next() {
		var action, resourceID, metadata string
		var userID sql.NullString
		require.NoError(t, rows.Scan(&action, &resourceID, &userID, &metadata))
		audited = append(audited, [2]string{action, resourceID})
		assert.Equal(t, manager.String(), userID.String)
		var fields map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(metadata), &fields))
		assert.ElementsMatch(t, []string{"kind", "minSeverity"}, keysOf(fields), metadata)
		for _, secret := range secrets {
			assert.NotContains(t, metadata, secret)
		}
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, [][2]string{
		{"create", slackA.ID.String()},
		{"create", slackB.ID.String()},
		{"create", pd.ID.String()},
		{"update", slackB.ID.String()},
		{"delete", slackB.ID.String()},
	}, audited)
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
