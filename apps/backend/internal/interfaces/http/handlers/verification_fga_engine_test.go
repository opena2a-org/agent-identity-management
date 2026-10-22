package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The SDK verification route decides with the FGA engine's five steps (#656).
//
// POST /api/v1/sdk-api/verifications decided with AgentService.VerifyCapability
// alone and then emitted an fga.authorize span with one fga.capability_check
// child after the fact; the agent's FGA policy for the capability was never
// read. Now VerifyCapability is step 1 of FGAEngine.Authorize: the attribute,
// context and chain checks run on the agent's stored policy, or on the default
// policy when it has none, and the intent check is dispatched for HIGH and
// MEDIUM risk. These tests drive the real handler and the real engine over
// sqlmock, with the span tree recorded.

// fgaPolicyColumns matches the SELECT in FGAEngine.loadPolicy.
var fgaPolicyColumns = []string{
	"id", "agent_id", "capability",
	"allowed_attributes", "denied_attributes", "allowed_objects", "allowed_actions",
	"row_predicate", "context_rules", "chain_rules", "intent_check", "risk_level",
}

// storedPolicy is one fga_policies row for agent and capability, LOW risk, with
// the given text[] literals for allowed_actions and denied_attributes.
func storedPolicy(agent *domain.Agent, capability, allowedActions, deniedAttributes string) *sqlmock.Rows {
	return sqlmock.NewRows(fgaPolicyColumns).AddRow(
		uuid.New().String(), agent.ID.String(), capability,
		"{}", deniedAttributes, "{}", allowedActions,
		[]byte(`{}`), []byte(`{}`), []byte(`{}`), []byte(`{}`), "LOW",
	)
}

const (
	fgaAllowReason    = "Action matches registered capabilities and passes all security policies"
	fgaNotVerified    = "Agent not verified - all actions denied"
	fgaMonitoringNote = "Action allowed by MONITORING mode (no capabilities registered yet) - logged for review. Policy: none"

	fgaPolicyUnreadable  = "Action denied: the agent's FGA policy for this capability could not be read"
	fgaVerificationError = "Action denied: the verification could not be completed"
)

// fgaVerificationCase configures one route: the organization's enforcement
// mode, the agent's policy row (nil: none) or the failure reading it, and
// step 1's answer.
type fgaVerificationCase struct {
	mode      domain.EnforcementMode
	policy    *sqlmock.Rows
	policyErr error
	allowed   bool
	reason    string
	verifyErr error
}

// fgaVerificationApp mounts CreateVerification bare, as cmd/server does on
// the SDK API, over a real FGAEngine on sqlmock, with every audit entry,
// verification event and span captured, and step 1's calls counted.
type fgaVerificationApp struct {
	app      *fiber.App
	recorder *tracetest.SpanRecorder
	audits   []*domain.AuditLog
	events   []*application.CreateVerificationEventRequest
	verified int
}

func newFGAVerificationApp(t *testing.T, agent *domain.Agent, tc fgaVerificationCase) *fgaVerificationApp {
	t.Helper()
	fa := &fgaVerificationApp{recorder: tracetest.NewSpanRecorder()}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(fa.recorder))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		_ = tp.Shutdown(context.Background())
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	switch {
	case tc.policyErr != nil:
		mock.ExpectQuery("FROM fga_policies").WillReturnError(tc.policyErr)
	case tc.policy != nil:
		mock.ExpectQuery("FROM fga_policies").WillReturnRows(tc.policy)
	default:
		mock.ExpectQuery("FROM fga_policies").WillReturnRows(sqlmock.NewRows(fgaPolicyColumns))
	}
	// Every other DB touch (the attestation and tool-call inserts, the ASC
	// read) is best-effort in the engine; sqlmock's unexpected-call errors
	// exercise those paths.

	// No AgentService on the engine: step 1 is the route's VerifyCapability.
	engine := application.NewFGAEngine(db, nil, slog.Default())
	// Nothing listens here, so a dispatched intent check fails open at once.
	engine.SetDaemonURL("http://127.0.0.1:1")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = engine.Shutdown(ctx)
	})

	lookup := agentsByID(agent)
	svc := &MockAgentServiceForVerificationImpl{}
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return lookup(id) }
	svc.VerifyCapabilityFunc = func(context.Context, uuid.UUID, string, string, map[string]interface{}, string) (bool, string, uuid.UUID, error) {
		fa.verified++
		return tc.allowed, tc.reason, uuid.New(), tc.verifyErr
	}
	audit := &MockAuditServiceForVerificationImpl{LogFunc: func(_ context.Context, entry *domain.AuditLog) error {
		fa.audits = append(fa.audits, entry)
		return nil
	}}
	events := &MockVerificationEventServiceForVerificationImpl{
		CreateVerificationEventFunc: func(_ context.Context, _ domain.VerificationEventSource, req *application.CreateVerificationEventRequest) (*domain.VerificationEvent, error) {
			fa.events = append(fa.events, req)
			return &domain.VerificationEvent{ID: uuid.New(), OrganizationID: req.OrganizationID, AgentID: &req.AgentID}, nil
		},
	}
	orgs := &MockOrganizationRepositoryerImpl{GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
		return &domain.Organization{ID: id, EnforcementMode: tc.mode}, nil
	}}
	h := &VerificationHandler{
		agentServicer:             svc,
		auditServicer:             audit,
		alertServicer:             &MockAlertServiceForVerificationImpl{},
		verificationEventServicer: events,
		orgRepoInterface:          orgs,
		fgaEngine:                 engine,
	}
	fa.app = fiber.New()
	fa.app.Post("/api/v1/sdk-api/verifications", h.CreateVerification)
	return fa
}

// post signs a verification for capability with the given context, as the
// SDK does, posts it, and returns the status and the decoded body.
func (fa *fgaVerificationApp) post(t *testing.T, agent *domain.Agent, pub ed25519.PublicKey, priv ed25519.PrivateKey, capability string, ctx map[string]interface{}) (int, map[string]interface{}) {
	t.Helper()
	req := VerificationRequest{
		AgentID:    agent.ID.String(),
		Capability: capability,
		Resource:   "customers",
		Context:    ctx,
		Timestamp:  strconv.FormatInt(time.Now().Unix(), 10),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
	}
	message, err := verificationSigningMessage(req)
	require.NoError(t, err)
	req.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message))
	raw, err := json.Marshal(req)
	require.NoError(t, err)
	httpReq := httptest.NewRequest("POST", "/api/v1/sdk-api/verifications", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := fa.app.Test(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded), string(body))
	return resp.StatusCode, decoded
}

// spanTree returns the ended fga.authorize span, its attributes, and the
// names of its ended children in start order, without the worker span of
// the detached intent check, which may still be running.
func (fa *fgaVerificationApp) spanTree(t *testing.T) (sdktrace.ReadOnlySpan, map[string]attribute.Value, []string) {
	t.Helper()
	var parent sdktrace.ReadOnlySpan
	for _, s := range fa.recorder.Ended() {
		if s.Name() == "fga.authorize" {
			parent = s
		}
	}
	require.NotNil(t, parent, "expected an fga.authorize span")
	var children []string
	for _, s := range fa.recorder.Ended() {
		if s.Parent().SpanID() == parent.SpanContext().SpanID() && !strings.HasSuffix(s.Name(), ".worker") {
			children = append(children, s.Name())
		}
	}
	return parent, fgaSpanAttributes(parent), children
}

func fgaSpanAttributes(s sdktrace.ReadOnlySpan) map[string]attribute.Value {
	attrs := make(map[string]attribute.Value, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}
	return attrs
}

// decisionAudit returns the one audit entry the decision wrote.
func (fa *fgaVerificationApp) decisionAudit(t *testing.T) *domain.AuditLog {
	t.Helper()
	require.Len(t, fa.audits, 1, "the decision writes one audit entry")
	return fa.audits[0]
}

var (
	fourFGAStepSpans = []string{"fga.capability_check", "fga.attribute_check", "fga.context_check", "fga.chain_check"}
	fiveFGAStepSpans = append(append([]string{}, fourFGAStepSpans...), "fga.intent_check_async")
	fiveFGASteps     = []string{"capability_check", "attribute_check", "context_check", "chain_check", "intent_check_async"}
)

func TestCreateVerification_IsDecidedByTheFiveFGASteps(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability string
		wantSteps  []string
		wantSpans  []string
	}{
		{name: "a critical capability runs all five steps", capability: "payment:refund", wantSteps: fiveFGASteps, wantSpans: fiveFGAStepSpans},
		{name: "a low capability runs four and skips the intent check", capability: "db:read", wantSteps: fiveFGASteps[:4], wantSpans: fourFGAStepSpans},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pub, priv := mustKeypair(t)
			agent := refusalAgent(domain.AgentStatusVerified, pub)
			fa := newFGAVerificationApp(t, agent, fgaVerificationCase{mode: domain.EnforcementModeStrict, allowed: true, reason: fgaAllowReason})

			code, body := fa.post(t, agent, pub, priv, tc.capability, map[string]interface{}{})
			require.Equal(t, fiber.StatusCreated, code, body)
			assert.Equal(t, "approved", body["status"])
			assert.Equal(t, 1, fa.verified, "VerifyCapability is step 1, run once")

			_, attrs, children := fa.spanTree(t)
			assert.Equal(t, "default", attrs["fga.policy"].AsString(), "an agent without a policy row is decided under the default policy")
			assert.Equal(t, "ALLOW", attrs["fga.outcome"].AsString())
			assert.Equal(t, tc.wantSteps, attrs["fga.steps_triggered"].AsStringSlice())
			assert.Equal(t, tc.wantSpans, children, "one child span per step that ran")

			entry := fa.decisionAudit(t)
			assert.Equal(t, domain.AuditAction(tc.capability), entry.Action)
			assert.Equal(t, "ALLOW", entry.Metadata["fgaOutcome"])
			assert.Equal(t, tc.wantSteps, entry.Metadata["fgaStepsTriggered"])
			assert.NotContains(t, entry.Metadata, "fgaDeniedBy")
			assert.NotContains(t, entry.Metadata, "denialReason")

			require.Len(t, fa.events, 1)
			assert.Equal(t, domain.VerificationEventStatusSuccess, fa.events[0].Status)
			assert.Equal(t, "ALLOW", fa.events[0].Metadata["fgaOutcome"])
			assert.Equal(t, tc.wantSteps, fa.events[0].Metadata["fgaStepsTriggered"])
		})
	}
}

func TestCreateVerification_AStoredPolicyDeniesTheCallInStrictMode(t *testing.T) {
	const capability = "db:read"
	for _, tc := range []struct {
		name       string
		policy     func(agent *domain.Agent) *sqlmock.Rows
		context    map[string]interface{}
		wantReason string
	}{
		{
			name:       "an action outside the policy's allowed actions",
			policy:     func(a *domain.Agent) *sqlmock.Rows { return storedPolicy(a, capability, "{read}", "{}") },
			context:    map[string]interface{}{"action": "delete"},
			wantReason: "Action 'delete' not in allowed list",
		},
		{
			name:       "an attribute the policy denies",
			policy:     func(a *domain.Agent) *sqlmock.Rows { return storedPolicy(a, capability, "{}", "{ssn}") },
			context:    map[string]interface{}{"attributes": []string{"name", "ssn"}},
			wantReason: "Attribute 'ssn' is denied by policy",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pub, priv := mustKeypair(t)
			agent := refusalAgent(domain.AgentStatusVerified, pub)
			fa := newFGAVerificationApp(t, agent, fgaVerificationCase{mode: domain.EnforcementModeStrict, policy: tc.policy(agent), allowed: true, reason: fgaAllowReason})

			code, body := fa.post(t, agent, pub, priv, capability, tc.context)
			require.Equal(t, fiber.StatusForbidden, code, body)
			assert.Equal(t, "denied", body["status"])
			assert.Equal(t, tc.wantReason, body["denialReason"])
			assert.Equal(t, 1, fa.verified)

			_, attrs, children := fa.spanTree(t)
			assert.Equal(t, "stored", attrs["fga.policy"].AsString())
			assert.Equal(t, "DENY_ATTRIBUTE", attrs["fga.outcome"].AsString())
			assert.Equal(t, "attribute_check", attrs["fga.denied_by"].AsString())
			assert.Equal(t, []string{"fga.capability_check", "fga.attribute_check"}, children, "the decision stops at the step that denies")

			entry := fa.decisionAudit(t)
			assert.Equal(t, tc.wantReason, entry.Metadata["denialReason"])
			assert.Equal(t, "DENY_ATTRIBUTE", entry.Metadata["fgaOutcome"])
			assert.Equal(t, "attribute_check", entry.Metadata["fgaDeniedBy"])
			assert.Equal(t, tc.wantReason, entry.Metadata["fgaDeniedReason"])

			require.Len(t, fa.events, 1)
			assert.Equal(t, domain.VerificationEventStatusFailed, fa.events[0].Status)
			require.NotNil(t, fa.events[0].ErrorReason)
			assert.Equal(t, tc.wantReason, *fa.events[0].ErrorReason)
		})
	}
}

func TestCreateVerification_AStoredPolicyDenialRunsInMonitoringModeAndIsRecorded(t *testing.T) {
	const capability = "db:read"
	const wantReason = "Action 'delete' not in allowed list"
	pub, priv := mustKeypair(t)
	agent := refusalAgent(domain.AgentStatusVerified, pub)
	fa := newFGAVerificationApp(t, agent, fgaVerificationCase{
		mode: domain.EnforcementModeMonitoring, policy: storedPolicy(agent, capability, "{read}", "{}"), allowed: true, reason: fgaMonitoringNote,
	})

	code, body := fa.post(t, agent, pub, priv, capability, map[string]interface{}{"action": "delete"})
	require.Equal(t, fiber.StatusCreated, code, body)
	assert.Equal(t, "auto-approved", body["status"])
	assert.NotContains(t, body, "denialReason")

	_, attrs, _ := fa.spanTree(t)
	assert.Equal(t, "DENY_ATTRIBUTE", attrs["fga.outcome"].AsString(), "the trace records the engine's decision")

	entry := fa.decisionAudit(t)
	assert.Equal(t, true, entry.Metadata["autoApproved"])
	assert.NotContains(t, entry.Metadata, "denialReason")
	assert.Equal(t, "DENY_ATTRIBUTE", entry.Metadata["fgaOutcome"])
	assert.Equal(t, "attribute_check", entry.Metadata["fgaDeniedBy"])
	assert.Equal(t, wantReason, entry.Metadata["fgaDeniedReason"])

	require.Len(t, fa.events, 1)
	assert.Equal(t, domain.VerificationEventStatusSuccess, fa.events[0].Status)
	assert.Equal(t, "DENY_ATTRIBUTE", fa.events[0].Metadata["fgaOutcome"])
	assert.Equal(t, wantReason, fa.events[0].Metadata["fgaDeniedReason"])
}

func TestCreateVerification_AStep1DenialKeepsVerifyCapabilitysReason(t *testing.T) {
	pub, priv := mustKeypair(t)
	agent := refusalAgent(domain.AgentStatusVerified, pub)
	fa := newFGAVerificationApp(t, agent, fgaVerificationCase{mode: domain.EnforcementModeStrict, allowed: false, reason: fgaNotVerified})

	code, body := fa.post(t, agent, pub, priv, "payment:refund", map[string]interface{}{})
	require.Equal(t, fiber.StatusForbidden, code, body)
	assert.Equal(t, "denied", body["status"])
	assert.Equal(t, fgaNotVerified, body["denialReason"])

	_, attrs, children := fa.spanTree(t)
	assert.Equal(t, "DENY", attrs["fga.outcome"].AsString())
	assert.Equal(t, "capability_check", attrs["fga.denied_by"].AsString())
	assert.Equal(t, []string{"fga.capability_check"}, children)
	for _, s := range fa.recorder.Ended() {
		if s.Name() == "fga.capability_check" {
			child := fgaSpanAttributes(s)
			assert.False(t, child["fga.allowed"].AsBool())
			assert.Equal(t, fgaNotVerified, child["fga.denied_reason"].AsString())
		}
	}

	entry := fa.decisionAudit(t)
	assert.Equal(t, fgaNotVerified, entry.Metadata["denialReason"])
	assert.Equal(t, "capability_check", entry.Metadata["fgaDeniedBy"])
}

// driverError is a database driver's error for a read of table: the driver's
// prefix, the table and the host it was reading from.
func driverError(table string) error {
	return fmt.Errorf("pq: relation %s does not exist (host policy-store.internal)", table)
}

// assertCarriesNoDriverText fails when the response body raw carries the
// message of err, a driverError for table, or any part of it.
func assertCarriesNoDriverText(t *testing.T, raw string, err error, table string) {
	t.Helper()
	for _, fragment := range []string{err.Error(), "pq:", table, "does not exist", "policy-store.internal"} {
		assert.NotContains(t, raw, fragment, "the response body carries the driver's words")
	}
}

func encodedBody(t *testing.T, body map[string]interface{}) string {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return string(raw)
}

func TestCreateVerification_AnUnreadablePolicyDeniesBeforeStep1(t *testing.T) {
	pub, priv := mustKeypair(t)
	agent := refusalAgent(domain.AgentStatusVerified, pub)
	readErr := driverError("fga_policies")
	fa := newFGAVerificationApp(t, agent, fgaVerificationCase{
		mode: domain.EnforcementModeMonitoring, policyErr: readErr, allowed: true, reason: fgaAllowReason,
	})

	code, body := fa.post(t, agent, pub, priv, "db:read", map[string]interface{}{})
	require.Equal(t, fiber.StatusForbidden, code, body)
	assert.Equal(t, "denied", body["status"])
	assert.Equal(t, fgaPolicyUnreadable, body["denialReason"])
	assertCarriesNoDriverText(t, encodedBody(t, body), readErr, "fga_policies")
	assert.Equal(t, 0, fa.verified, "step 1 does not run when the policy cannot be read")

	parent, attrs, children := fa.spanTree(t)
	assert.Equal(t, "ERROR", attrs["fga.outcome"].AsString())
	assert.Contains(t, parent.Status().Description, readErr.Error(), "the span records the driver's error")
	assert.Empty(t, children)

	entry := fa.decisionAudit(t)
	assert.Equal(t, "ERROR", entry.Metadata["fgaOutcome"])
	assert.Equal(t, fgaPolicyUnreadable, entry.Metadata["denialReason"])
	require.Len(t, fa.events, 1)
	require.NotNil(t, fa.events[0].ErrorReason)
	assert.Equal(t, fgaPolicyUnreadable, *fa.events[0].ErrorReason)
}

// Step 1 failing with no reason of its own is denied with a fixed reason, and
// the error it returned reaches neither the response body nor the audit entry.
func TestCreateVerification_AStep1ErrorWithoutAReasonIsDeniedWithAFixedReason(t *testing.T) {
	pub, priv := mustKeypair(t)
	agent := refusalAgent(domain.AgentStatusVerified, pub)
	readErr := driverError("agent_capabilities")
	fa := newFGAVerificationApp(t, agent, fgaVerificationCase{mode: domain.EnforcementModeStrict, verifyErr: readErr})

	code, body := fa.post(t, agent, pub, priv, "db:read", map[string]interface{}{})
	require.Equal(t, fiber.StatusForbidden, code, body)
	assert.Equal(t, "denied", body["status"])
	assert.Equal(t, fgaVerificationError, body["denialReason"])
	assertCarriesNoDriverText(t, encodedBody(t, body), readErr, "agent_capabilities")
	assert.Equal(t, 1, fa.verified)

	entry := fa.decisionAudit(t)
	assert.Equal(t, "ERROR", entry.Metadata["fgaOutcome"])
	assert.Equal(t, fgaVerificationError, entry.Metadata["denialReason"])
}

func TestFGARequestForVerification_ReadsWhatTheCallTouchesFromTheContext(t *testing.T) {
	agentID := uuid.New()
	check := application.CapabilityCheck(func(context.Context) (bool, string, error) { return true, "", nil })
	req := fgaRequestForVerification(agentID, verificationInput{
		capability: "db:read",
		resource:   "customers",
		context: map[string]interface{}{
			"attributes": []interface{}{"name", 7, "ssn"},
			"action":     "read",
			"dataClass":  "pii",
			"jit_access": false,
		},
	}, "low", check)
	assert.Equal(t, agentID, req.AgentID)
	assert.Equal(t, "db:read", req.Capability)
	assert.Equal(t, "customers", req.Resource)
	assert.Equal(t, "low", req.RiskLevel)
	assert.Equal(t, []string{"name", "ssn"}, req.Attributes, "members of attributes that are not strings are left out")
	assert.Equal(t, "read", req.Action)
	assert.Equal(t, "pii", req.DataClass)
	assert.NotNil(t, req.CapabilityCheck)

	bare := fgaRequestForVerification(agentID, verificationInput{capability: "db:read", context: map[string]interface{}{"action": 3}}, "", check)
	assert.Nil(t, bare.Attributes)
	assert.Empty(t, bare.Action, "an action that is not a string names no action")
	assert.Empty(t, bare.DataClass)

	assert.Nil(t, fgaDecisionMetadata(nil))
}
