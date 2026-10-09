package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The handlers that verify an agent's signature themselves (the token endpoint, legacy
// verification creation and the SDK verification read) read the agent's status only
// after the signature verifies, and answer an unknown agent, a missing key, a key that
// does not decode and a different key alike.
//
// Before, each read the agent row first and answered from it: "Agent not found", the
// status, "has no registered public key", "Public key mismatch", a 500 for a corrupt
// key. A caller holding only an agent id could tell those apart. Every cell compares the
// body byte for byte, because every one of these requests was refused before the change
// as well; what changed is which refusal.

// Written out as literals so this file states the contract independently of the
// declarations it checks.
const (
	handlerKeyNotRecognizedBody = `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`
	tokenNotRecognizedBody      = `{"error":"invalid_client","error_description":"the assertion is not signed by the registered key of a known agent"}`
)

// refusalAgent is an agent row in `status` holding pub.
func refusalAgent(status domain.AgentStatus, pub ed25519.PublicKey) *domain.Agent {
	encoded := base64.StdEncoding.EncodeToString(pub)
	return &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Status: status, PublicKey: &encoded}
}

func mustKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return pub, priv
}

// agentsByID answers GetAgent / GetByID for the given agents and an error for any other id.
func agentsByID(agents ...*domain.Agent) func(uuid.UUID) (*domain.Agent, error) {
	byID := map[uuid.UUID]*domain.Agent{}
	for _, a := range agents {
		byID[a.ID] = a
	}
	return func(id uuid.UUID) (*domain.Agent, error) {
		if a, ok := byID[id]; ok {
			return a, nil
		}
		return nil, errors.New("agent not found")
	}
}

// malformedKeyAgent is a verified agent whose stored key decodes to 16 bytes.
func malformedKeyAgent() (*domain.Agent, string, []byte) {
	short := []byte("sixteen-byte-key")
	stored := base64.StdEncoding.EncodeToString(short)
	return &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Status: domain.AgentStatusVerified, PublicKey: &stored}, stored, short
}

func captureStdLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	fn()
	return buf.String()
}

func requireOneLogLineWithoutKey(t *testing.T, logged, stored string, raw []byte) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(logged, "\n"), "\n")
	require.Len(t, lines, 1, "exactly one log line, got %q", logged)
	require.NotContains(t, logged, stored)
	require.NotContains(t, logged, hex.EncodeToString(raw))
}

func flipByte(sig []byte) []byte {
	out := append([]byte(nil), sig...)
	out[0] ^= 0x01
	return out
}

// ---------------------------------------------------------------------------
// Token endpoint (jwt-bearer)
// ---------------------------------------------------------------------------

func tokenApp(t *testing.T, agents ...*domain.Agent) *fiber.App {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	t.Setenv("AIM_BASE_URL", "https://aim.test")
	h := NewOAuthTokenHandler(auth.NewJWTService(), &MockAgentRepositoryerImpl{GetByIDFunc: agentsByID(agents...)})
	app := fiber.New()
	app.Post("/oauth/token", h.HandleTokenRequest)
	return app
}

func tokenClaims(agentID uuid.UUID) map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"iss": agentID.String(), "sub": agentID.String(), "aud": "https://aim.test",
		"iat": float64(now.Unix()), "exp": float64(now.Add(2 * time.Minute).Unix()),
	}
}

// assertion signs claims with priv; mutate, when set, rewrites the signature bytes.
func assertion(t *testing.T, priv ed25519.PrivateKey, claims map[string]interface{}, mutate func([]byte) []byte) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sig := ed25519.Sign(priv, []byte(input))
	if mutate != nil {
		sig = mutate(sig)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func postAssertion(t *testing.T, app *fiber.App, clientID uuid.UUID, assertion string) (int, string) {
	t.Helper()
	form := strings.NewReader(
		"grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" +
			"&client_id=" + clientID.String() +
			"&client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer" +
			"&client_assertion=" + assertion)
	req := httptest.NewRequest("POST", "/oauth/token", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func TestOAuthToken_AnswersEveryUnrecognisedKeyAndFailedSignatureAlike(t *testing.T) {
	pub, priv := mustKeypair(t)
	_, otherPriv := mustKeypair(t)

	verified := refusalAgent(domain.AgentStatusVerified, pub)
	suspended := refusalAgent(domain.AgentStatusSuspended, pub)
	noKey := refusalAgent(domain.AgentStatusVerified, pub)
	noKey.PublicKey = nil
	app := tokenApp(t, verified, suspended, noKey)

	unknown := uuid.New()
	for name, tc := range map[string]struct {
		id        uuid.UUID
		assertion string
	}{
		"unknown agent":                  {unknown, assertion(t, otherPriv, tokenClaims(unknown), nil)},
		"no registered key":              {noKey.ID, assertion(t, priv, tokenClaims(noKey.ID), nil)},
		"a different key":                {verified.ID, assertion(t, otherPriv, tokenClaims(verified.ID), nil)},
		"a flipped signature byte":       {verified.ID, assertion(t, priv, tokenClaims(verified.ID), flipByte)},
		"suspended, a different key":     {suspended.ID, assertion(t, otherPriv, tokenClaims(suspended.ID), nil)},
		"suspended, a flipped signature": {suspended.ID, assertion(t, priv, tokenClaims(suspended.ID), flipByte)},
	} {
		code, body := postAssertion(t, app, tc.id, tc.assertion)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, tokenNotRecognizedBody, body, name)
	}

	// The status, and only after the signature verified.
	code, body := postAssertion(t, app, suspended.ID, assertion(t, priv, tokenClaims(suspended.ID), nil))
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, `{"error":"invalid_client","error_description":"Agent is not permitted to authenticate (status: suspended)"}`, body)

	// The control.
	code, body = postAssertion(t, app, verified.ID, assertion(t, priv, tokenClaims(verified.ID), nil))
	require.Equal(t, fiber.StatusOK, code, body)
}

// A stored key that does not decode used to answer 500 server_error, which told the
// caller the agent existed. It is now the same 401, and the operator's signal is one log
// line that carries neither the stored value nor its bytes.
func TestOAuthToken_AMalformedStoredKeyGetsTheMergedRefusalAndOneLogLine(t *testing.T) {
	agent, stored, raw := malformedKeyAgent()
	_, priv := mustKeypair(t)
	app := tokenApp(t, agent)

	var code int
	var body string
	logged := captureStdLog(t, func() {
		code, body = postAssertion(t, app, agent.ID, assertion(t, priv, tokenClaims(agent.ID), nil))
	})
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, tokenNotRecognizedBody, body)
	requireOneLogLineWithoutKey(t, logged, stored, raw)
}

// The signature's encoding is checked before the agent is read: a malformed encoding gets
// the same status and body for an unknown agent as for a known agent holding a key.
func TestOAuthToken_AMalformedSignatureEncodingIsAnsweredBeforeTheLookup(t *testing.T) {
	pub, priv := mustKeypair(t)
	known := refusalAgent(domain.AgentStatusSuspended, pub)
	app := tokenApp(t, known)
	broken := func([]byte) []byte { return nil }

	malformed := func(id uuid.UUID) string {
		a := assertion(t, priv, tokenClaims(id), broken)
		return a + "!!" // not base64url
	}
	unknown := uuid.New()
	codeUnknown, bodyUnknown := postAssertion(t, app, unknown, malformed(unknown))
	codeKnown, bodyKnown := postAssertion(t, app, known.ID, malformed(known.ID))
	require.Equal(t, fiber.StatusBadRequest, codeKnown, bodyKnown)
	require.Equal(t, codeUnknown, codeKnown)
	require.Equal(t, bodyUnknown, bodyKnown)
}

// ---------------------------------------------------------------------------
// Legacy verification creation
// ---------------------------------------------------------------------------

type createVerificationCase struct {
	agentID   uuid.UUID
	signer    ed25519.PrivateKey
	presented ed25519.PublicKey
	// presentedRaw overrides presented with a literal header value.
	presentedRaw string
	flip         bool
}

func (tc createVerificationCase) do(t *testing.T, app *fiber.App) (int, string) {
	t.Helper()
	req := VerificationRequest{
		AgentID:    tc.agentID.String(),
		Capability: "read_file",
		Resource:   "/tmp/x",
		Timestamp:  strconv.FormatInt(time.Now().Unix(), 10),
		PublicKey:  base64.StdEncoding.EncodeToString(tc.presented),
	}
	if tc.presentedRaw != "" {
		req.PublicKey = tc.presentedRaw
	}
	message, err := verificationSigningMessage(req)
	require.NoError(t, err)
	sig := ed25519.Sign(tc.signer, message)
	if tc.flip {
		sig = flipByte(sig)
	}
	req.Signature = base64.StdEncoding.EncodeToString(sig)

	raw, err := json.Marshal(req)
	require.NoError(t, err)
	httpReq := httptest.NewRequest("POST", "/api/v1/sdk-api/verifications", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// createVerificationApp mounts CreateVerification bare, as cmd/server does on the SDK
// API. reached is set when a request gets past authentication to the capability check.
func createVerificationApp(reached *bool, agents ...*domain.Agent) *fiber.App {
	lookup := agentsByID(agents...)
	svc := &MockAgentServiceForVerificationImpl{}
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return lookup(id) }
	svc.VerifyCapabilityFunc = func(context.Context, uuid.UUID, string, string, map[string]interface{}, string) (bool, string, uuid.UUID, error) {
		*reached = true
		return false, "stopped by the test", uuid.Nil, errors.New("stopped by the test")
	}
	h := NewVerificationHandlerWithInterfaces(svc, &MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{}, &MockVerificationEventServiceForVerificationImpl{},
		&MockOrganizationRepositoryerImpl{})
	app := fiber.New()
	app.Post("/api/v1/sdk-api/verifications", h.CreateVerification)
	return app
}

func TestCreateVerification_ReadsStatusOnlyAfterTheSignatureVerifies(t *testing.T) {
	pub, priv := mustKeypair(t)
	otherPub, otherPriv := mustKeypair(t)

	verified := refusalAgent(domain.AgentStatusVerified, pub)
	suspended := refusalAgent(domain.AgentStatusSuspended, pub)
	noKey := refusalAgent(domain.AgentStatusVerified, pub)
	noKey.PublicKey = nil
	var reached bool
	app := createVerificationApp(&reached, verified, suspended, noKey)

	// Named residual: this legacy route keeps its unknown-agent 404.
	code, body := createVerificationCase{agentID: uuid.New(), signer: otherPriv, presented: otherPub}.do(t, app)
	require.Equal(t, fiber.StatusNotFound, code, body)
	require.Equal(t, `{"error":"Agent not found"}`, body)

	for name, tc := range map[string]createVerificationCase{
		"no registered key":          {agentID: noKey.ID, signer: priv, presented: pub},
		"a different key":            {agentID: verified.ID, signer: otherPriv, presented: otherPub},
		"suspended, a different key": {agentID: suspended.ID, signer: otherPriv, presented: otherPub},
	} {
		code, body := tc.do(t, app)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, handlerKeyNotRecognizedBody, body, name)
	}

	code, suspendedBody := createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub, flip: true}.do(t, app)
	require.Equal(t, fiber.StatusUnauthorized, code, suspendedBody)
	_, verifiedBody := createVerificationCase{agentID: verified.ID, signer: priv, presented: pub, flip: true}.do(t, app)
	require.Equal(t, verifiedBody, suspendedBody)
	require.NotContains(t, suspendedBody, "suspended")

	code, body = createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub}.do(t, app)
	require.Equal(t, fiber.StatusForbidden, code, body)
	require.Equal(t, `{"error":"Agent status is suspended, cannot perform actions"}`, body)
	require.False(t, reached)

	createVerificationCase{agentID: verified.ID, signer: priv, presented: pub}.do(t, app)
	require.True(t, reached, "the control must get past authentication")
}

func TestCreateVerification_AMalformedStoredKeyGetsTheMergedBodyAndOneLogLine(t *testing.T) {
	agent, stored, raw := malformedKeyAgent()
	_, priv := mustKeypair(t)
	var reached bool
	app := createVerificationApp(&reached, agent)

	var code int
	var body string
	logged := captureStdLog(t, func() {
		code, body = createVerificationCase{agentID: agent.ID, signer: priv, presentedRaw: stored}.do(t, app)
	})
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, handlerKeyNotRecognizedBody, body)
	requireOneLogLineWithoutKey(t, logged, stored, raw)
}

// ---------------------------------------------------------------------------
// SDK verification read
// ---------------------------------------------------------------------------

func sdkReadApp(agents ...*domain.Agent) (*fiber.App, uuid.UUID) {
	verificationID := uuid.New()
	lookup := agentsByID(agents...)
	svc := &MockAgentServiceForVerificationImpl{}
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return lookup(id) }
	approved := domain.VerificationResultVerified
	events := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(_ context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			owner := agents[0].ID
			return &domain.VerificationEvent{ID: verificationID, OrganizationID: agents[0].OrganizationID,
				AgentID: &owner, Result: &approved, CreatedAt: time.Now()}, nil
		},
	}
	h := NewVerificationHandlerWithInterfaces(svc, &MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{}, events, &MockOrganizationRepositoryerImpl{})
	app := fiber.New()
	app.Get("/api/v1/sdk-api/verifications/:id", h.GetVerificationSDK)
	return app, verificationID
}

func sdkRead(t *testing.T, app *fiber.App, verificationID, agentID uuid.UUID, priv ed25519.PrivateKey, flip bool) (int, string) {
	t.Helper()
	ts := time.Now().Unix()
	msg := fmt.Sprintf("GET\n/api/v1/sdk-api/verifications/%s\n%s\n%d", verificationID, agentID, ts)
	sig := ed25519.Sign(priv, []byte(msg))
	if flip {
		sig = flipByte(sig)
	}
	req := httptest.NewRequest("GET", "/api/v1/sdk-api/verifications/"+verificationID.String(), nil)
	req.Header.Set("X-AIM-Agent-ID", agentID.String())
	req.Header.Set("X-AIM-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-AIM-Signature", base64.StdEncoding.EncodeToString(sig))
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestGetVerificationSDK_ReadsStatusOnlyAfterTheSignatureVerifies(t *testing.T) {
	pub, priv := mustKeypair(t)
	_, otherPriv := mustKeypair(t)

	verified := refusalAgent(domain.AgentStatusVerified, pub)
	suspended := refusalAgent(domain.AgentStatusSuspended, pub)
	noKey := refusalAgent(domain.AgentStatusVerified, pub)
	noKey.PublicKey = nil
	app, vid := sdkReadApp(verified, suspended, noKey)

	for name, id := range map[string]uuid.UUID{"unknown agent": uuid.New(), "no registered key": noKey.ID} {
		code, body := sdkRead(t, app, vid, id, otherPriv, false)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, handlerKeyNotRecognizedBody, body, name)
	}

	// This route carries no key, so a different key is a signature that does not verify:
	// the same refusal whatever the agent's status.
	for name, signer := range map[string]struct {
		priv ed25519.PrivateKey
		flip bool
	}{"a different key": {otherPriv, false}, "a flipped signature byte": {priv, true}} {
		code, suspendedBody := sdkRead(t, app, vid, suspended.ID, signer.priv, signer.flip)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, suspendedBody)
		_, verifiedBody := sdkRead(t, app, vid, verified.ID, signer.priv, signer.flip)
		require.Equal(t, verifiedBody, suspendedBody, name)
		require.NotContains(t, suspendedBody, "suspended", name)
	}

	// A suspended agent no longer reads its verification events, and learns why only
	// with a valid signature.
	code, body := sdkRead(t, app, vid, suspended.ID, priv, false)
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, `{"error":"Agent is not permitted to authenticate (status: suspended)","reasonCode":"agentStatusDenied"}`, body)

	code, body = sdkRead(t, app, vid, verified.ID, priv, false)
	require.Equal(t, fiber.StatusOK, code, body)
}

func TestGetVerificationSDK_AMalformedStoredKeyGetsTheMergedBodyAndOneLogLine(t *testing.T) {
	agent, stored, raw := malformedKeyAgent()
	_, priv := mustKeypair(t)
	app, vid := sdkReadApp(agent)

	var code int
	var body string
	logged := captureStdLog(t, func() { code, body = sdkRead(t, app, vid, agent.ID, priv, false) })
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, handlerKeyNotRecognizedBody, body)
	requireOneLogLineWithoutKey(t, logged, stored, raw)
}
