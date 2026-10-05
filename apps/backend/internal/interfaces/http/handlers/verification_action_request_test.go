package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const (
	sdkVerificationsPath = "/api/v1/sdk-api/verifications"
	jwtVerificationsPath = "/api/v1/verifications"
)

// memoryAdmitter is one admission store shared by both mounts, with the
// statement's semantics: window first, then one insert per (agent, nonce).
// The window cells against the real database clock are in the repository's
// integration tests; here the outcome can also be forced.
type memoryAdmitter struct {
	mu     sync.Mutex
	rows   map[string]bool
	calls  int
	force  domain.ActionRequestAdmission
	err    error
	window time.Duration
}

func newMemoryAdmitter() *memoryAdmitter {
	return &memoryAdmitter{rows: map[string]bool{}, window: domain.ActionRequestWindowSeconds * time.Second}
}

func (m *memoryAdmitter) Admit(_ context.Context, agentID, _ uuid.UUID, nonce []byte, signedAt time.Time) (domain.ActionRequestAdmission, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return 0, m.err
	}
	if m.force != 0 {
		return m.force, nil
	}
	now := time.Now()
	if signedAt.Before(now.Add(-m.window)) {
		return domain.ActionRequestBehindClock, nil
	}
	if signedAt.After(now.Add(m.window)) {
		return domain.ActionRequestAheadOfClock, nil
	}
	key := agentID.String() + "/" + base64.RawURLEncoding.EncodeToString(nonce)
	if m.rows[key] {
		return domain.ActionRequestNonceReused, nil
	}
	m.rows[key] = true
	return domain.ActionRequestAdmitted, nil
}

func (m *memoryAdmitter) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// statementFixture is a verification handler mounted on both routes with
// counters on every lookup and write.
type statementFixture struct {
	app      *fiber.App
	agent    *domain.Agent
	admitter *memoryAdmitter
	signer   *statementSigner

	lookups      atomic.Int32
	capabilities atomic.Int32
	audits       atomic.Int32
	alerts       atomic.Int32
	events       atomic.Int32

	mu          sync.Mutex
	lastEvent   *application.CreateVerificationEventRequest
	lastSource  domain.VerificationEventSource
	lastCapCtx  map[string]interface{}
	lastCapName string
}

func newStatementFixture(t *testing.T) *statementFixture {
	t.Helper()
	f := &statementFixture{admitter: newMemoryAdmitter(), signer: newStatementSigner(t)}
	f.agent = &domain.Agent{
		ID:             f.signer.agentID,
		OrganizationID: uuid.New(),
		Name:           "statement-agent",
		DisplayName:    "Statement Agent",
		Status:         domain.AgentStatusVerified,
		PublicKey:      f.signer.registeredKey(),
		TrustScore:     0.8,
	}

	agentSvc := &MockAgentServiceForVerificationImpl{
		CreateSecurityAlertFunc: func(context.Context, *domain.Alert) error {
			f.alerts.Add(1)
			return nil
		},
	}
	agentSvc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
		f.lookups.Add(1)
		if id != f.agent.ID {
			return nil, errors.New("agent not found")
		}
		copied := *f.agent
		return &copied, nil
	}
	agentSvc.VerifyCapabilityFunc = func(_ context.Context, _ uuid.UUID, capability, _ string, metadata map[string]interface{}, _ string) (bool, string, uuid.UUID, error) {
		f.capabilities.Add(1)
		f.mu.Lock()
		f.lastCapName, f.lastCapCtx = capability, metadata
		f.mu.Unlock()
		return true, "", uuid.New(), nil
	}
	agentSvc.HasCapabilityNoAlertFunc = func(context.Context, uuid.UUID, string, string) (bool, error) {
		return false, nil
	}

	handler := NewVerificationHandlerWithInterfaces(
		agentSvc,
		&MockAuditServiceForVerificationImpl{LogFunc: func(context.Context, *domain.AuditLog) error {
			f.audits.Add(1)
			return nil
		}},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{
			CreateVerificationEventFunc: func(_ context.Context, source domain.VerificationEventSource, req *application.CreateVerificationEventRequest) (*domain.VerificationEvent, error) {
				f.events.Add(1)
				f.mu.Lock()
				f.lastEvent = req
				f.lastSource = source
				f.mu.Unlock()
				return &domain.VerificationEvent{ID: uuid.New()}, nil
			},
		},
		&MockOrganizationRepositoryerImpl{GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			return &domain.Organization{ID: id, EnforcementMode: domain.EnforcementModeStrict}, nil
		}},
	).WithActionRequestNonces(f.admitter)

	f.app = fiber.New()
	// The bare SDK mount (rate limiter only in production) and the JWT mount.
	f.app.Post(sdkVerificationsPath, handler.CreateVerification)
	jwt := f.app.Group(jwtVerificationsPath)
	jwt.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", f.agent.OrganizationID)
		return c.Next()
	})
	jwt.Post("/", handler.CreateVerification)
	return f
}

type writeCounts struct{ capabilities, audits, alerts, events int32 }

func (f *statementFixture) writes() writeCounts {
	return writeCounts{f.capabilities.Load(), f.audits.Load(), f.alerts.Load(), f.events.Load()}
}

func (f *statementFixture) post(t *testing.T, path string, body []byte) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

func refusalBody(code, message string) string {
	b, _ := json.Marshal(fiber.Map{"error": message, "reasonCode": code})
	return string(b)
}

func TestActionRequest_AcceptedOnceAcrossBothMounts(t *testing.T) {
	f := newStatementFixture(t)
	body := f.signer.statement(f.signer.payload(t, ""))

	status, out := f.post(t, sdkVerificationsPath, body)
	require.Equal(t, 201, status, string(out))
	afterFirst := f.writes()
	assert.Equal(t, int32(1), afterFirst.capabilities)
	assert.Equal(t, int32(1), afterFirst.audits)
	assert.Equal(t, int32(1), afterFirst.events)

	// The same signed bytes on the other mount: the store is shared.
	status, out = f.post(t, jwtVerificationsPath, body)
	assert.Equal(t, 401, status)
	assert.JSONEq(t, refusalBody("nonceReused", "this nonce was already used by this agent"), string(out))
	assert.Equal(t, afterFirst, f.writes(), "a replay writes no action, event, audit or alert")

	// And again on the first mount.
	status, _ = f.post(t, sdkVerificationsPath, body)
	assert.Equal(t, 401, status)
	assert.Equal(t, afterFirst, f.writes())
}

func TestActionRequest_JWTMountAcceptsAndSDKMountRefusesTheReplay(t *testing.T) {
	f := newStatementFixture(t)
	body := f.signer.statement(f.signer.payload(t, ""))
	status, _ := f.post(t, jwtVerificationsPath, body)
	require.Equal(t, 201, status)
	status, out := f.post(t, sdkVerificationsPath, body)
	assert.Equal(t, 401, status)
	assert.JSONEq(t, refusalBody("nonceReused", "this nonce was already used by this agent"), string(out))
}

// The decision and the records are made from the signed payload, and the
// recorded context is what was signed.
func TestActionRequest_RecordsWhatWasSigned(t *testing.T) {
	f := newStatementFixture(t)
	digest := b64url(make([]byte, 32))
	payload := f.signer.payload(t, `,"context":{"big":9007199254740993,"one":1.0,"word":"café"},"risk_level":"high","delegation_digests":["`+digest+`"]`)
	frame := paeFrame(domain.ActionRequestPayloadType, []byte(payload))
	sig := ed25519.Sign(f.signer.priv, frame)

	status, out := f.post(t, sdkVerificationsPath, f.signer.bodyWith(frame, sig, f.signer.pub))
	require.Equal(t, 201, status, string(out))

	var resp VerificationResponse
	require.NoError(t, json.Unmarshal(out, &resp))
	assert.Equal(t, "high", resp.RiskLevel)
	assert.False(t, resp.RiskAutoDetected)

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "db:read", f.lastCapName)
	require.NotNil(t, f.lastEvent)
	// The server decided this outcome, so the event is recorded as observed.
	assert.Equal(t, domain.VerificationEventSourceService, f.lastSource)
	assert.Equal(t, b64url(sig), *f.lastEvent.Signature)
	assert.Equal(t, b64url(f.signer.pub), *f.lastEvent.PublicKey)
	assert.Equal(t, "orders", *f.lastEvent.ResourceType)
	assert.Equal(t, domain.ActionRequestSchemeID, f.lastEvent.Metadata["signedStatement"])
	assert.Equal(t, []string{digest}, f.lastEvent.Metadata["delegationDigests"])

	recorded, err := json.Marshal(f.lastEvent.Metadata["context"])
	require.NoError(t, err)
	assert.Equal(t, `{"big":9007199254740993,"one":1.0,"word":"café"}`, string(recorded))
	recorded, err = json.Marshal(f.lastCapCtx)
	require.NoError(t, err)
	assert.Equal(t, `{"big":9007199254740993,"one":1.0,"word":"café"}`, string(recorded))
}

// Before the signature verifies, the four key cases get one status and one
// byte-identical body, and nothing about the agent's status leaks.
func TestActionRequest_KeyCasesShareOneBody(t *testing.T) {
	want := refusalBody("agentKeyNotRecognized", "the signing key is not the registered key of a known agent")
	other := newStatementSigner(t)
	malformed := "not-a-key"
	short := base64.StdEncoding.EncodeToString(make([]byte, 31))

	cases := map[string]func(f *statementFixture) []byte{
		"unknown agent": func(f *statementFixture) []byte {
			other.agentID = uuid.New()
			return other.statement(other.payload(t, ""))
		},
		"agent with no key": func(f *statementFixture) []byte {
			f.agent.PublicKey = nil
			return f.signer.statement(f.signer.payload(t, ""))
		},
		"stored key that does not decode": func(f *statementFixture) []byte {
			f.agent.PublicKey = &malformed
			return f.signer.statement(f.signer.payload(t, ""))
		},
		"stored key of the wrong length": func(f *statementFixture) []byte {
			f.agent.PublicKey = &short
			return f.signer.statement(f.signer.payload(t, ""))
		},
		"different key": func(f *statementFixture) []byte {
			impostor := &statementSigner{agentID: f.signer.agentID, pub: other.pub, priv: other.priv}
			return impostor.statement(impostor.payload(t, ""))
		},
		"suspended agent with a different key": func(f *statementFixture) []byte {
			f.agent.Status = domain.AgentStatusSuspended
			impostor := &statementSigner{agentID: f.signer.agentID, pub: other.pub, priv: other.priv}
			return impostor.statement(impostor.payload(t, ""))
		},
	}
	var bodies []string
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			f := newStatementFixture(t)
			status, out := f.post(t, sdkVerificationsPath, build(f))
			assert.Equal(t, 401, status)
			assert.Equal(t, want, string(out))
			assert.NotContains(t, string(out), "status")
			assert.Equal(t, 0, f.admitter.callCount(), "no nonce is spent before the signature verifies")
			assert.Equal(t, writeCounts{}, f.writes())
			bodies = append(bodies, string(out))
		})
	}
	for _, b := range bodies {
		assert.Equal(t, bodies[0], b)
	}
}

func TestActionRequest_BadSignatureIsRefusedWithoutStatusText(t *testing.T) {
	for _, status := range []domain.AgentStatus{domain.AgentStatusVerified, domain.AgentStatusSuspended, domain.AgentStatusRevoked} {
		t.Run(string(status), func(t *testing.T) {
			f := newStatementFixture(t)
			f.agent.Status = status
			frame := paeFrame(domain.ActionRequestPayloadType, []byte(f.signer.payload(t, "")))
			sig := ed25519.Sign(f.signer.priv, frame)
			sig[10] ^= 0x01

			code, out := f.post(t, sdkVerificationsPath, f.signer.bodyWith(frame, sig, f.signer.pub))
			assert.Equal(t, 401, code)
			assert.Equal(t, refusalBody("signatureInvalid",
				"the signature does not verify over signedBytes with this agent's registered key"), string(out))
			assert.Equal(t, 0, f.admitter.callCount())
			assert.Equal(t, writeCounts{}, f.writes())
		})
	}
}

func TestActionRequest_HybridModeAgentIsRefusedWithoutSpendingTheNonce(t *testing.T) {
	f := newStatementFixture(t)
	f.agent.HybridModeEnabled = true
	body := f.signer.statement(f.signer.payload(t, ""))

	status, out := f.post(t, sdkVerificationsPath, body)
	assert.Equal(t, 401, status)
	assert.Equal(t, refusalBody("hybridSignatureRequired",
		"Agent is in hybrid mode and action-request-v1 statements cannot be hybrid-signed"), string(out))
	assert.Equal(t, 0, f.admitter.callCount())
	assert.Equal(t, writeCounts{}, f.writes())

	// Re-sent inside its window after hybrid mode is turned off, it is admitted.
	f.agent.HybridModeEnabled = false
	status, out = f.post(t, sdkVerificationsPath, body)
	assert.Equal(t, 201, status, string(out))
}

// A suspended or revoked agent's valid statement spends its nonce and writes
// nothing else; re-sent after reactivation it is a reused nonce.
func TestActionRequest_StatusGateRunsAfterAdmission(t *testing.T) {
	for _, status := range []domain.AgentStatus{domain.AgentStatusSuspended, domain.AgentStatusRevoked} {
		t.Run(string(status), func(t *testing.T) {
			f := newStatementFixture(t)
			f.agent.Status = status
			body := f.signer.statement(f.signer.payload(t, ""))

			code, out := f.post(t, sdkVerificationsPath, body)
			assert.Equal(t, 401, code)
			assert.Equal(t, refusalBody("agentStatusDenied", domain.AgentStatusDeniedMessage(status)), string(out))
			assert.Equal(t, 1, f.admitter.callCount(), "the nonce is spent")
			assert.Len(t, f.admitter.rows, 1, "exactly one admission row")
			assert.Equal(t, writeCounts{}, f.writes(), "no other write")

			f.agent.Status = domain.AgentStatusVerified
			code, out = f.post(t, sdkVerificationsPath, body)
			assert.Equal(t, 401, code)
			assert.Equal(t, refusalBody("nonceReused", "this nonce was already used by this agent"), string(out))
		})
	}
}

// A stale statement from a suspended agent gets the window refusal, never the
// status refusal.
func TestActionRequest_WindowRefusalsWriteNothing(t *testing.T) {
	behind := refusalBody("timestampOutsideWindow", "request timestamp is more than 30 seconds behind the AIM database clock")
	ahead := refusalBody("timestampOutsideWindow", "request timestamp is more than 30 seconds ahead of the AIM database clock")
	for _, tc := range []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"behind", -31 * time.Second, behind},
		{"ahead", 31 * time.Second, ahead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStatementFixture(t)
			f.agent.Status = domain.AgentStatusSuspended
			payload := fmt.Sprintf(`{"action_type":"db:read","agent_id":"%s","resource":null,"timestamp":"%s","nonce":"%s"}`,
				f.signer.agentID, time.Now().Add(tc.offset).UTC().Format(time.RFC3339Nano), newTestNonce(t))
			code, out := f.post(t, sdkVerificationsPath, f.signer.statement(payload))
			assert.Equal(t, 401, code)
			assert.Equal(t, tc.want, string(out))
			assert.Empty(t, f.admitter.rows)
			assert.Equal(t, writeCounts{}, f.writes())
		})
	}
}

// When the admission statement cannot run, the request is refused 503 with no
// record, and never as a credential refusal.
func TestActionRequest_StoreFailureRefusesWithNoRecord(t *testing.T) {
	want := refusalBody("freshnessCheckUnavailable",
		"cannot check this request's time and nonce against the AIM database; nothing was recorded")
	for name, err := range map[string]error{
		"database error":    errors.New("connection closed"),
		"statement timeout": domain.ErrActionRequestAdmissionTimedOut,
		"purge not running": domain.ErrActionRequestNoncePurgeNotRunning,
	} {
		for _, status := range []domain.AgentStatus{domain.AgentStatusVerified, domain.AgentStatusSuspended} {
			t.Run(name+"/"+string(status), func(t *testing.T) {
				f := newStatementFixture(t)
				f.agent.Status = status
				f.admitter.err = err
				code, out := f.post(t, sdkVerificationsPath, f.signer.statement(f.signer.payload(t, "")))
				assert.Equal(t, 503, code)
				assert.Equal(t, want, string(out))
				assert.Equal(t, writeCounts{}, f.writes())
			})
		}
	}
}

func TestActionRequest_NoAdmissionStoreRefusesEveryStatement(t *testing.T) {
	f := newStatementFixture(t)
	handler := NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{MockAgentServiceImpl: MockAgentServiceImpl{
			GetAgentFunc: func(context.Context, uuid.UUID) (*domain.Agent, error) { return f.agent, nil },
		}},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{},
	)
	app := fiber.New()
	app.Post(sdkVerificationsPath, handler.CreateVerification)
	req := httptest.NewRequest("POST", sdkVerificationsPath, bytes.NewReader(f.signer.statement(f.signer.payload(t, ""))))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)
}

func TestActionRequest_ShapeRefusalsReadNoStoredState(t *testing.T) {
	f := newStatementFixture(t)

	payload := strings.Replace(f.signer.payload(t, ""), `,"nonce":"`, `,"nonce_":"`, 1)
	payload = strings.Replace(payload, `"nonce_":`, `"risk_level":`, 1)
	status, out := f.post(t, sdkVerificationsPath, f.signer.statement(payload))
	assert.Equal(t, 400, status)
	assert.Equal(t, refusalBody("nonceRequired", "the signed statement has no nonce"), string(out))

	status, _ = f.post(t, sdkVerificationsPath, f.signer.body(f.signer.frameOfSize(t, 65537)))
	assert.Equal(t, 422, status)

	status, _ = f.post(t, jwtVerificationsPath, f.signer.statement(f.signer.payload(t, `,"context":`+nest(33))))
	assert.Equal(t, 422, status)

	assert.Equal(t, int32(0), f.lookups.Load(), "no agent lookup, so no signature check")
	assert.Equal(t, 0, f.admitter.callCount())
	assert.Equal(t, writeCounts{}, f.writes())

	// The next request is served.
	status, out = f.post(t, sdkVerificationsPath, f.signer.statement(f.signer.payload(t, "")))
	assert.Equal(t, 201, status, string(out))
}

// The original body form may not carry the members only a signed statement
// defines: unsigned, they would be claims nothing checks.
func TestActionRequest_LegacyBodyWithFreshMembersIsRefused(t *testing.T) {
	for _, member := range []string{"nonce", "delegationDigests"} {
		t.Run(member, func(t *testing.T) {
			f := newStatementFixture(t)
			body, _ := json.Marshal(map[string]any{
				"agentId": f.signer.agentID.String(), "capability": "db:read", "timestamp": "2026-10-05T12:00:00Z",
				"signature": "x", "publicKey": "y", member: "z",
			})
			status, out := f.post(t, sdkVerificationsPath, body)
			assert.Equal(t, 400, status)
			assert.Equal(t, refusalBody("invalidRequest",
				"nonce and delegationDigests are accepted only inside a signed statement (signedBytes)"), string(out))
			assert.Equal(t, int32(0), f.lookups.Load())
			assert.Equal(t, writeCounts{}, f.writes())
		})
	}
}

// The original five-key request, signed the way the Python and Java SDKs sign
// it, still verifies and touches no admission store.
func TestActionRequest_LegacyFiveKeyRequestStillVerifies(t *testing.T) {
	f := newStatementFixture(t)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z"
	message := fmt.Sprintf(`{"action_type": "db:read", "agent_id": "%s", "context": {}, "resource": null, "timestamp": "%s"}`,
		f.signer.agentID, timestamp)
	body, _ := json.Marshal(map[string]any{
		"agentId":    f.signer.agentID.String(),
		"capability": "db:read",
		"resource":   nil,
		"context":    map[string]any{},
		"timestamp":  timestamp,
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(f.signer.priv, []byte(message))),
		"publicKey":  *f.signer.registeredKey(),
	})
	for _, path := range []string{sdkVerificationsPath, jwtVerificationsPath} {
		status, out := f.post(t, path, body)
		assert.Equal(t, 201, status, string(out))
	}
	assert.Equal(t, 0, f.admitter.callCount())
}
