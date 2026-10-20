package middleware

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The signed agent request: a statement about the request in X-Agent-Request,
// signed over its DSSE pre-authentication encoding, admitted once.
//
// Every cell checks the status and the body byte for byte, whether the next
// handler ran, and what the middleware read and wrote: agent rows read,
// admission calls made and nonce rows stored. The admission store here stands
// in for the admission statement; the statement itself is tested against
// PostgreSQL in the repository's integration tests.

// The nonce service the server runs is an admitter.
var _ AgentRequestNonceAdmitter = (*application.ActionRequestNonceService)(nil)

const (
	v2Audience = "https://aim.example.test"
	v2Path     = "/protected"
)

// v2Agents is the agent source: it knows the given agents and counts reads.
type v2Agents struct {
	rows  map[uuid.UUID]*domain.Agent
	reads int
}

func (s *v2Agents) GetAgent(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	s.reads++
	if a, ok := s.rows[id]; ok {
		return a, nil
	}
	return nil, errors.New("agent not found")
}

// v2Nonces stands in for agent_request_nonces and its admission statement: it
// places the signed time against its own clock reading with the window of
// domain.ActionRequestWindowSeconds, inclusive, and stores (agent, nonce) once.
// It also has the purge side of domain.ActionRequestNonceRepository, so
// application.ActionRequestNonceService can run over it.
type v2Nonces struct {
	now   time.Time
	rows  map[string]bool
	calls []v2Admission
	err   error
}

type v2Admission struct {
	agentID, organizationID uuid.UUID
	nonce                   []byte
	signedAt                time.Time
}

var _ domain.ActionRequestNonceRepository = (*v2Nonces)(nil)

func (n *v2Nonces) Admit(_ context.Context, agentID, organizationID uuid.UUID, nonce []byte, signedAt time.Time) (domain.ActionRequestAdmission, error) {
	n.calls = append(n.calls, v2Admission{agentID, organizationID, append([]byte(nil), nonce...), signedAt})
	if n.err != nil {
		return 0, n.err
	}
	window := time.Duration(domain.ActionRequestWindowSeconds) * time.Second
	switch {
	case signedAt.Before(n.now.Add(-window)):
		return domain.ActionRequestBehindClock, nil
	case signedAt.After(n.now.Add(window)):
		return domain.ActionRequestAheadOfClock, nil
	}
	key := agentID.String() + "/" + base64.RawURLEncoding.EncodeToString(nonce)
	if n.rows[key] {
		return domain.ActionRequestNonceReused, nil
	}
	n.rows[key] = true
	return domain.ActionRequestAdmitted, nil
}

func (n *v2Nonces) ListOrganizationIDs(context.Context) ([]uuid.UUID, error) { return nil, nil }

func (n *v2Nonces) PurgeOrganization(context.Context, uuid.UUID) (int64, error) { return 0, nil }

// v2Harness is the middleware mounted in front of one route, GET and POST.
type v2Harness struct {
	agents  *v2Agents
	nonces  *v2Nonces
	app     *fiber.App
	reached int
	locals  fiber.Map
}

// newV2Harness mounts the middleware with audience and the stand-in store.
func newV2Harness(audience string, rows ...*domain.Agent) *v2Harness {
	h := &v2Harness{
		agents: &v2Agents{rows: map[uuid.UUID]*domain.Agent{}},
		nonces: &v2Nonces{now: time.Now(), rows: map[string]bool{}},
	}
	for _, a := range rows {
		h.agents.rows[a.ID] = a
	}
	h.mount(audience, h.nonces)
	return h
}

func (h *v2Harness) mount(audience string, admitter AgentRequestNonceAdmitter) {
	// The server's own header buffer (cmd/server): an ML-DSA-65 signature does not fit
	// in fiber's 4 KB default.
	h.app = fiber.New(fiber.Config{ReadBufferSize: 16384})
	h.app.Use(AgentRequestV2Middleware(h.agents, admitter, audience))
	handler := func(c fiber.Ctx) error {
		h.reached++
		h.locals = fiber.Map{}
		for _, k := range []string{"agent_id", "organization_id", "authenticated_via", "auth_method",
			"auth_algorithm", "agent_request_scheme"} {
			h.locals[k] = c.Locals(k)
		}
		return c.JSON(fiber.Map{"reached": true})
	}
	h.app.Get(v2Path, handler)
	h.app.Post(v2Path, handler)
}

// v2Request is one request a client builds and signs. send fills in the zero
// fields.
type v2Request struct {
	method string // the method sent; GET when ""
	target string // the request target sent; v2Path when ""
	body   string

	agentID uuid.UUID
	keys    agentKeys
	algs    []pqc.Algorithm // the algorithms that sign, in header order; Ed25519 when nil
	fold    bool            // send the signatures as one comma-separated header line
	flip    bool            // flip a bit of every signature
	// flipOnly flips a bit of the signature of this algorithm alone.
	flipOnly pqc.Algorithm

	timestamp time.Time // the stand-in store's clock reading when zero
	nonce     []byte    // 16 fresh random bytes when nil

	// edit replaces members of the built payload by raw JSON, appends a member the
	// payload does not have, and removes a member whose value is "".
	edit map[string]string
	// payload, when set, is sent and signed in place of the built one.
	payload []byte
	// statement, when set, is the X-Agent-Request value, sent as is.
	statement string
	// statementTwice sends the X-Agent-Request header on two lines.
	statementTwice bool
	// signatures, when not nil, are the signature header values sent as is; an
	// empty slice sends no signature header.
	signatures []string
	// headers are set on the request as sent.
	headers map[string]string
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sha256B64url(s string) string {
	sum := sha256.Sum256([]byte(s))
	return b64url(sum[:])
}

// members are the members a client signs, in order, as raw JSON values.
func (r v2Request) members() [][2]string {
	m := [][2]string{
		{"method", jsonString(r.method)},
		{"target", jsonString(r.target)},
		{"audience", jsonString(v2Audience)},
	}
	if r.body != "" {
		m = append(m, [2]string{"body_sha256", jsonString(sha256B64url(r.body))})
	}
	return append(m,
		[2]string{"agent_id", jsonString(r.agentID.String())},
		[2]string{"timestamp", jsonString(r.timestamp.UTC().Format(time.RFC3339Nano))},
		[2]string{"nonce", jsonString(b64url(r.nonce))},
	)
}

// encodeV2Payload writes members as one compact JSON object, after edit.
func encodeV2Payload(members [][2]string, edit map[string]string) []byte {
	out := make([][2]string, 0, len(members)+len(edit))
	seen := map[string]bool{}
	for _, m := range members {
		seen[m[0]] = true
		if v, ok := edit[m[0]]; ok {
			if v == "" {
				continue
			}
			m[1] = v
		}
		out = append(out, m)
	}
	for name, v := range edit {
		if !seen[name] && v != "" {
			out = append(out, [2]string{name, v})
		}
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, m := range out {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jsonString(m[0]) + ":" + m[1])
	}
	b.WriteByte('}')
	return []byte(b.String())
}

// v2PAE is the pre-authentication encoding a client signs, written out here so
// these tests state the format independently of agentRequestPAE.
func v2PAE(payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s",
		len("application/vnd.opena2a.agent-request.v2+json"), "application/vnd.opena2a.agent-request.v2+json",
		len(payload), payload))
}

// sign returns the signature header values over payload, one per algorithm. An
// ML-DSA level other than 65 signs with a key of that level made for the call,
// which no agent here has registered.
func (r v2Request) sign(t *testing.T, payload []byte) []string {
	t.Helper()
	message := v2PAE(payload)
	values := make([]string, 0, len(r.algs))
	for _, alg := range r.algs {
		var sig []byte
		var err error
		switch alg {
		case pqc.AlgorithmEd25519:
			sig = ed25519.Sign(r.keys.edPriv, message)
		case pqc.AlgorithmMLDSA65:
			sig, err = pqc.SignMLDSA(alg, r.keys.pqcPriv, message)
			require.NoError(t, err)
		default:
			kp, genErr := pqc.GenerateMLDSAKeyPair(alg)
			require.NoError(t, genErr)
			sig, err = pqc.SignMLDSA(alg, kp.PrivateKey, message)
			require.NoError(t, err)
		}
		if r.flip || r.flipOnly == alg {
			sig = flipFirst(sig)
		}
		values = append(values, string(alg)+":"+b64url(sig))
	}
	return values
}

func v2Nonce(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// send builds r, sends it through the harness and returns the status and the
// raw body.
func (h *v2Harness) send(t *testing.T, r v2Request) (int, string) {
	t.Helper()
	if r.method == "" {
		r.method = fiber.MethodGet
	}
	if r.target == "" {
		r.target = v2Path
	}
	if r.timestamp.IsZero() {
		r.timestamp = h.nonces.now
	}
	if r.nonce == nil {
		r.nonce = v2Nonce(t)
	}
	if r.algs == nil {
		r.algs = []pqc.Algorithm{pqc.AlgorithmEd25519}
	}
	payload := r.payload
	if payload == nil {
		payload = encodeV2Payload(r.members(), r.edit)
	}
	statement := r.statement
	if statement == "" {
		statement = b64url(payload)
	}
	signatures := r.signatures
	if signatures == nil {
		signatures = r.sign(t, payload)
	}

	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req := httptest.NewRequest(r.method, r.target, body)
	req.Header.Add("X-Agent-Request", statement)
	if r.statementTwice {
		req.Header.Add("X-Agent-Request", statement)
	}
	if r.fold && len(signatures) > 0 {
		req.Header.Set("X-Agent-Request-Signature", strings.Join(signatures, ", "))
	} else {
		for _, s := range signatures {
			req.Header.Add("X-Agent-Request-Signature", s)
		}
	}
	for name, v := range r.headers {
		req.Header.Set(name, v)
	}

	resp, err := h.app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// stored is the number of nonce rows the stand-in store holds.
func (h *v2Harness) stored() int { return len(h.nonces.rows) }

func v2Refusal(code, message string) string {
	b, err := json.Marshal(map[string]string{"error": message, "reasonCode": code})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// v2Agent is an agent row with keys' Ed25519 key, and its ML-DSA-65 key when
// pqcKey is set.
func v2Agent(keys agentKeys, status domain.AgentStatus, pqcKey, hybrid bool) *domain.Agent {
	a := registeredAgent(status, keys)
	if !pqcKey {
		a.PQCPublicKey = nil
	}
	a.HybridModeEnabled = hybrid
	return a
}

const reachedBody = `{"reached":true}`

func TestAgentRequestV2_SignsTheDSSEEncodingOfThePayload(t *testing.T) {
	payload := []byte(`{"method":"GET","target":"/api/v1/agents?limit=10","audience":"https://aim.example.test",` +
		`"agent_id":"6f1c2a54-2f0e-4c39-9d4f-3b8e2c7a9d10","timestamp":"2026-10-19T12:00:00Z","nonce":"AAECAwQFBgcICQoLDA0ODw"}`)
	require.Len(t, payload, 207)

	want := `DSSEv1 45 application/vnd.opena2a.agent-request.v2+json 207 ` + string(payload)
	assert.Equal(t, want, string(agentRequestPAE(payload)))
	assert.Equal(t, want, string(v2PAE(payload)), "the harness signs the bytes the golden line pins")

	// The leading bytes name the format, and differ from those of the action-request
	// statement at the payload type.
	assert.True(t, strings.HasPrefix(string(agentRequestPAE(payload)), "DSSEv1 45 application/vnd.opena2a.agent-request.v2+json "))
	assert.NotEqual(t, domain.ActionRequestPayloadType, domain.AgentRequestPayloadType)
}

func TestAgentRequestV2_AcceptsAStatementOnceAndRefusesItsResend(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	h := newV2Harness(v2Audience, agent)

	req := v2Request{agentID: agent.ID, keys: keys, target: v2Path + "?limit=10&offset=0", nonce: v2Nonce(t)}
	status, body := h.send(t, req)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, reachedBody, body)
	assert.Equal(t, fiber.Map{
		"agent_id":             agent.ID,
		"organization_id":      agent.OrganizationID,
		"authenticated_via":    "ed25519",
		"auth_method":          "ed25519",
		"auth_algorithm":       "Ed25519",
		"agent_request_scheme": "agent-request-v2",
	}, h.locals)

	require.Len(t, h.nonces.calls, 1)
	call := h.nonces.calls[0]
	assert.Equal(t, agent.ID, call.agentID)
	assert.Equal(t, agent.OrganizationID, call.organizationID, "the organization is the stored agent's")
	assert.Equal(t, req.nonce, call.nonce)
	assert.True(t, call.signedAt.Equal(h.nonces.now), "the signed timestamp, as signed")

	// The same request again, byte for byte.
	status, body = h.send(t, req)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, v2Refusal("nonceReused", "this nonce was already used by this agent"), body)
	assert.Equal(t, 1, h.reached)
	assert.Equal(t, 1, h.stored())
	assert.Len(t, h.nonces.calls, 2)
}

func TestAgentRequestV2_AcceptsEachSignatureForm(t *testing.T) {
	keys := newAgentKeys(t)
	edOnly := v2Agent(keys, domain.AgentStatusVerified, false, false)
	both := v2Agent(keys, domain.AgentStatusVerified, true, false)
	hybrid := v2Agent(keys, domain.AgentStatusVerified, true, true)
	ed, ml := pqc.AlgorithmEd25519, pqc.AlgorithmMLDSA65

	cases := []struct {
		name       string
		agent      *domain.Agent
		algs       []pqc.Algorithm
		fold       bool
		method     string
		authMethod string
		algorithm  string
	}{
		{"Ed25519", edOnly, []pqc.Algorithm{ed}, false, fiber.MethodGet, "ed25519", "Ed25519"},
		{"Ed25519, agent with both keys", both, []pqc.Algorithm{ed}, false, fiber.MethodGet, "ed25519", "Ed25519"},
		{"ML-DSA-65, agent with both keys", both, []pqc.Algorithm{ml}, false, fiber.MethodGet, "mldsa", "ML-DSA-65"},
		{"hybrid, two header lines", hybrid, []pqc.Algorithm{ed, ml}, false, fiber.MethodGet, "hybrid", "Ed25519+ML-DSA-65"},
		{"hybrid, ML-DSA line first", hybrid, []pqc.Algorithm{ml, ed}, false, fiber.MethodGet, "hybrid", "Ed25519+ML-DSA-65"},
		{"hybrid, one comma-separated line", hybrid, []pqc.Algorithm{ed, ml}, true, fiber.MethodGet, "hybrid", "Ed25519+ML-DSA-65"},
		{"hybrid, with a body", hybrid, []pqc.Algorithm{ed, ml}, false, fiber.MethodPost, "hybrid", "Ed25519+ML-DSA-65"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newV2Harness(v2Audience, tc.agent)
			req := v2Request{agentID: tc.agent.ID, keys: keys, algs: tc.algs, fold: tc.fold, method: tc.method}
			if tc.method == fiber.MethodPost {
				req.body = `{"name":"weather-reader","capabilities":["read"]}`
			}
			status, body := h.send(t, req)
			require.Equal(t, fiber.StatusOK, status, body)
			assert.Equal(t, 1, h.reached)
			assert.Equal(t, 1, h.stored())
			assert.Equal(t, tc.authMethod, h.locals["auth_method"])
			assert.Equal(t, tc.authMethod, h.locals["authenticated_via"])
			assert.Equal(t, tc.algorithm, h.locals["auth_algorithm"])
		})
	}
}

// Each ML-DSA level is verified at its own key length, alone and beside Ed25519.
func TestAgentRequestV2_AcceptsEachMLDSALevel(t *testing.T) {
	for _, alg := range []pqc.Algorithm{pqc.AlgorithmMLDSA44, pqc.AlgorithmMLDSA65, pqc.AlgorithmMLDSA87} {
		kp, err := pqc.GenerateMLDSAKeyPair(alg)
		require.NoError(t, err)
		keys := newAgentKeys(t)
		for _, hybrid := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s hybrid=%t", alg, hybrid), func(t *testing.T) {
				agent := v2Agent(keys, domain.AgentStatusVerified, false, hybrid)
				agent.PQCPublicKey = b64(kp.PublicKey)
				h := newV2Harness(v2Audience, agent)

				req := v2Request{method: fiber.MethodGet, target: v2Path, agentID: agent.ID, timestamp: h.nonces.now, nonce: v2Nonce(t)}
				req.payload = encodeV2Payload(req.members(), nil)
				message := v2PAE(req.payload)
				sig, err := pqc.SignMLDSA(alg, kp.PrivateKey, message)
				require.NoError(t, err)
				req.signatures = []string{string(alg) + ":" + b64url(sig)}
				want, method := string(alg), "mldsa"
				if hybrid {
					req.signatures = append(req.signatures, "Ed25519:"+b64url(ed25519.Sign(keys.edPriv, message)))
					want, method = "Ed25519+"+string(alg), "hybrid"
				}

				status, body := h.send(t, req)
				require.Equal(t, fiber.StatusOK, status, body)
				assert.Equal(t, want, h.locals["auth_algorithm"])
				assert.Equal(t, method, h.locals["auth_method"])
				assert.Equal(t, 1, h.stored())
			})
		}
	}
}

// The signatures cover the payload as it was sent. A payload with insignificant
// whitespace verifies over its own bytes, and a signature over the compact
// form of the same members does not verify over it.
func TestAgentRequestV2_VerifiesThePayloadAsReceived(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	h := newV2Harness(v2Audience, agent)

	base := v2Request{agentID: agent.ID, keys: keys, timestamp: h.nonces.now, nonce: v2Nonce(t),
		method: fiber.MethodGet, target: v2Path, algs: []pqc.Algorithm{pqc.AlgorithmEd25519}}
	compact := encodeV2Payload(base.members(), nil)
	spaced := []byte(strings.ReplaceAll(strings.ReplaceAll(string(compact), `":"`, `": "`), `","`, `", "`))
	require.NotEqual(t, compact, spaced)

	signedCompact := base
	signedCompact.payload = compact
	other := base
	other.payload = spaced
	other.signatures = signedCompact.sign(t, compact)
	status, body := h.send(t, other)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, v2Refusal("signatureInvalid",
		"the Ed25519 signature does not verify over the statement with this agent's registered key"), body)
	assert.Equal(t, 0, h.stored())

	asSent := base
	asSent.payload = spaced
	status, body = h.send(t, asSent)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, 1, h.stored())
}

func TestAgentRequestV2_RefusalsWriteNothing(t *testing.T) {
	keys := newAgentKeys(t)
	otherKeys := newAgentKeys(t)
	edOnly := v2Agent(keys, domain.AgentStatusVerified, false, false)
	both := v2Agent(keys, domain.AgentStatusVerified, true, false)
	hybrid := v2Agent(keys, domain.AgentStatusVerified, true, true)
	ed, ml := pqc.AlgorithmEd25519, pqc.AlgorithmMLDSA65
	window := time.Duration(domain.ActionRequestWindowSeconds) * time.Second

	const (
		formNonce       = "nonce is not in the form agent-request-v2 requires"
		edInvalid       = "the Ed25519 signature does not verify over the statement with this agent's registered key"
		mlInvalid       = "the ML-DSA signature does not verify over the statement with this agent's registered key"
		requestMismatch = "requestMismatch"
	)
	signatureLine := "each X-Agent-Request-Signature value is an algorithm, a colon and an unpadded base64url " +
		"signature; the algorithm is Ed25519, ML-DSA-44, ML-DSA-65 or ML-DSA-87"

	cells := []struct {
		name   string
		req    v2Request
		status int
		body   string
		// reads is the agent rows read; admits the admission calls.
		reads, admits int
	}{
		{"timestamp behind the window", v2Request{agentID: edOnly.ID, keys: keys, timestamp: time.Now().Add(-window - time.Second)},
			401, v2Refusal("timestampOutsideWindow", "request timestamp is behind the AIM database clock by more than the admission window"), 1, 1},
		{"timestamp ahead of the window", v2Request{agentID: edOnly.ID, keys: keys, timestamp: time.Now().Add(window + 2*time.Second)},
			401, v2Refusal("timestampOutsideWindow", "request timestamp is ahead of the AIM database clock by more than the admission window"), 1, 1},
		{"nonce of 15 bytes", v2Request{agentID: edOnly.ID, keys: keys, nonce: make([]byte, 15)},
			400, v2Refusal("invalidRequest", formNonce), 0, 0},
		{"nonce of 17 bytes", v2Request{agentID: edOnly.ID, keys: keys, nonce: make([]byte, 17)},
			400, v2Refusal("invalidRequest", formNonce), 0, 0},
		{"audience of another deployment", v2Request{agentID: edOnly.ID, keys: keys, edit: map[string]string{"audience": `"https://aim.other.test"`}},
			401, v2Refusal("audienceMismatch", "the statement's audience is not this server's public origin"), 0, 0},
		{"audience differing only in case", v2Request{agentID: edOnly.ID, keys: keys, edit: map[string]string{"audience": `"https://AIM.example.test"`}},
			401, v2Refusal("audienceMismatch", "the statement's audience is not this server's public origin"), 0, 0},
		{"audience with a trailing slash", v2Request{agentID: edOnly.ID, keys: keys, edit: map[string]string{"audience": `"https://aim.example.test/"`}},
			401, v2Refusal("audienceMismatch", "the statement's audience is not this server's public origin"), 0, 0},
		{"body_sha256 absent beside a body", v2Request{agentID: edOnly.ID, keys: keys, method: fiber.MethodPost, body: `{"a":1}`,
			edit: map[string]string{"body_sha256": ""}},
			401, v2Refusal(requestMismatch, "this request has a body and the statement has no body_sha256"), 0, 0},
		{"body_sha256 beside no body", v2Request{agentID: edOnly.ID, keys: keys, edit: map[string]string{"body_sha256": jsonString(sha256B64url(""))}},
			401, v2Refusal(requestMismatch, "the statement has body_sha256 and this request has no body"), 0, 0},
		{"body_sha256 of other bytes", v2Request{agentID: edOnly.ID, keys: keys, method: fiber.MethodPost, body: `{"a":1}`,
			edit: map[string]string{"body_sha256": jsonString(sha256B64url(`{"a":2}`))}},
			401, v2Refusal(requestMismatch, "body_sha256 is not the SHA-256 of this request's body"), 0, 0},
		{"method of another request", v2Request{agentID: edOnly.ID, keys: keys, edit: map[string]string{"method": `"POST"`}},
			401, v2Refusal(requestMismatch, "the statement's method is not this request's method"), 0, 0},
		{"target with another query", v2Request{agentID: edOnly.ID, keys: keys, target: v2Path + "?limit=11",
			edit: map[string]string{"target": `"/protected?limit=10"`}},
			401, v2Refusal(requestMismatch, "the statement's target is not this request's path and query"), 0, 0},
		{"target without the query sent", v2Request{agentID: edOnly.ID, keys: keys, target: v2Path + "?limit=10",
			edit: map[string]string{"target": `"/protected"`}},
			401, v2Refusal(requestMismatch, "the statement's target is not this request's path and query"), 0, 0},
		{"legacy signed-request bytes as the payload", v2Request{agentID: edOnly.ID, keys: keys,
			payload: []byte("GET\n" + v2Path + "\n" + strconv.FormatInt(time.Now().Unix(), 10))},
			400, v2Refusal("invalidRequest", "the payload is not one JSON object in valid UTF-8 with no trailing bytes"), 0, 0},
		{"unknown agent", v2Request{agentID: uuid.New(), keys: keys},
			401, keyNotRecognizedBody, 1, 0},
		{"signed by a key that is not the agent's", v2Request{agentID: edOnly.ID, keys: otherKeys},
			401, v2Refusal("signatureInvalid", edInvalid), 1, 0},
		{"a flipped signature", v2Request{agentID: edOnly.ID, keys: keys, flip: true},
			401, v2Refusal("signatureInvalid", edInvalid), 1, 0},
		{"a flipped ML-DSA signature", v2Request{agentID: both.ID, keys: keys, algs: []pqc.Algorithm{ml}, flipOnly: ml},
			401, v2Refusal("signatureInvalid", mlInvalid), 1, 0},
		{"hybrid agent, Ed25519 signature flipped", v2Request{agentID: hybrid.ID, keys: keys, algs: []pqc.Algorithm{ed, ml}, flipOnly: ed},
			401, v2Refusal("signatureInvalid", edInvalid), 1, 0},
		{"hybrid agent, ML-DSA signature flipped", v2Request{agentID: hybrid.ID, keys: keys, algs: []pqc.Algorithm{ed, ml}, flipOnly: ml},
			401, v2Refusal("signatureInvalid", mlInvalid), 1, 0},
		{"an algorithm the agent has no key for", v2Request{agentID: edOnly.ID, keys: keys, algs: []pqc.Algorithm{ml}},
			401, keyNotRecognizedBody, 1, 0},
		{"an ML-DSA level other than the registered key's", v2Request{agentID: both.ID, keys: keys, algs: []pqc.Algorithm{pqc.AlgorithmMLDSA44}},
			401, keyNotRecognizedBody, 1, 0},
		{"ML-DSA-87 for an agent whose key is ML-DSA-65", v2Request{agentID: both.ID, keys: keys, algs: []pqc.Algorithm{pqc.AlgorithmMLDSA87}},
			401, keyNotRecognizedBody, 1, 0},
		{"hybrid algorithms for an agent with one key", v2Request{agentID: edOnly.ID, keys: keys, algs: []pqc.Algorithm{ed, ml}},
			401, keyNotRecognizedBody, 1, 0},
		{"a third signature", v2Request{agentID: hybrid.ID, keys: keys,
			signatures: []string{"Ed25519:" + b64url(make([]byte, 64)), "ML-DSA-65:AAAA", "Ed25519:AAAA"}},
			400, v2Refusal("invalidRequest", "X-Agent-Request-Signature carries more than two signatures"), 0, 0},
		{"two Ed25519 signatures", v2Request{agentID: hybrid.ID, keys: keys, algs: []pqc.Algorithm{ed, ed}},
			400, v2Refusal("invalidRequest", "two signatures are one Ed25519 and one ML-DSA signature"), 0, 0},
		{"hybrid agent, Ed25519 alone", v2Request{agentID: hybrid.ID, keys: keys, algs: []pqc.Algorithm{ed}},
			401, `{"error":"Agent is in hybrid mode: sign the statement with both Ed25519 and ML-DSA","reasonCode":"hybridSignatureRequired"}`, 1, 0},
		{"hybrid agent, ML-DSA alone", v2Request{agentID: hybrid.ID, keys: keys, algs: []pqc.Algorithm{ml}},
			401, `{"error":"Agent is in hybrid mode: sign the statement with both Ed25519 and ML-DSA","reasonCode":"hybridSignatureRequired"}`, 1, 0},
		{"agent not in hybrid mode, two signatures", v2Request{agentID: both.ID, keys: keys, algs: []pqc.Algorithm{ed, ml}},
			401, v2Refusal("singleSignatureRequired", "Agent is not in hybrid mode: sign the statement with one of its registered keys"), 1, 0},
		{"no signature header", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{}},
			400, v2Refusal("invalidRequest", "X-Agent-Request-Signature is required"), 0, 0},
		{"an empty signature value", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{""}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"a signature value with no algorithm", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{b64url(make([]byte, 64))}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"an algorithm in lower case", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{"ed25519:" + b64url(make([]byte, 64))}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"a hybrid algorithm name", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{"Ed25519+ML-DSA-65:" + b64url(make([]byte, 64))}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"a padded signature", v2Request{agentID: edOnly.ID, keys: keys,
			signatures: []string{"Ed25519:" + base64.URLEncoding.EncodeToString(make([]byte, 64))}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"a trailing comma", v2Request{agentID: edOnly.ID, keys: keys, signatures: []string{"Ed25519:" + b64url(make([]byte, 64)) + ","}},
			400, v2Refusal("invalidRequest", signatureLine), 0, 0},
		{"the statement header sent twice", v2Request{agentID: edOnly.ID, keys: keys, statementTwice: true},
			400, v2Refusal("invalidRequest", "X-Agent-Request must be sent once"), 0, 0},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			h := newV2Harness(v2Audience, edOnly, both, hybrid)
			status, body := h.send(t, cell.req)
			assert.Equal(t, cell.status, status)
			assert.Equal(t, cell.body, body)
			assert.Equal(t, 0, h.reached, "the next handler ran")
			assert.Equal(t, cell.reads, h.agents.reads, "agent rows read")
			assert.Len(t, h.nonces.calls, cell.admits, "admission calls")
			assert.Equal(t, 0, h.stored(), "nonce rows stored")
		})
	}
}

func TestAgentRequestV2_RefusesAPayloadOutsideItsForm(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	notOneObject := "the payload is not one JSON object in valid UTF-8 with no trailing bytes"
	form := func(member string) string { return member + " is not in the form agent-request-v2 requires" }

	valid := func(h *v2Harness) []byte {
		return encodeV2Payload(v2Request{method: fiber.MethodGet, target: v2Path, agentID: agent.ID,
			timestamp: h.nonces.now, nonce: make([]byte, 16)}.members(), nil)
	}
	cells := []struct {
		name    string
		payload func(valid []byte) []byte
		edit    map[string]string
		header  string
		code    string
		message string
	}{
		{name: "a header value that is not unpadded base64url", header: "AAAA==",
			code: "invalidRequest", message: "X-Agent-Request is not unpadded base64url"},
		{name: "a JSON array", payload: func([]byte) []byte { return []byte(`["GET"]`) },
			code: "invalidRequest", message: notOneObject},
		{name: "a space before the object", payload: func(v []byte) []byte { return append([]byte(" "), v...) },
			code: "invalidRequest", message: notOneObject},
		{name: "a second object after it", payload: func(v []byte) []byte { return append(append([]byte{}, v...), []byte(" {}")...) },
			code: "invalidRequest", message: notOneObject},
		{name: "bytes that are not UTF-8", payload: func(v []byte) []byte {
			return []byte(strings.Replace(string(v), `"GET"`, "\"G\xffT\"", 1))
		}, code: "invalidRequest", message: notOneObject},
		{name: "a member name twice", payload: func(v []byte) []byte {
			return []byte(strings.Replace(string(v), `{"method":"GET",`, `{"method":"GET","method":"GET",`, 1))
		}, code: "invalidRequest", message: "the payload repeats a member name"},
		{name: "the same name twice through an escape", payload: func(v []byte) []byte {
			return []byte(strings.Replace(string(v), `{"method":"GET",`, `{"method":"GET","method":"GET",`, 1))
		}, code: "invalidRequest", message: "the payload repeats a member name"},
		{name: "a member the format does not define", edit: map[string]string{"resource": `"agents"`},
			code: "invalidRequest", message: "the payload has a member agent-request-v2 does not define"},
		{name: "a member name in another case", edit: map[string]string{"Nonce": `"AAAAAAAAAAAAAAAAAAAAAA"`},
			code: "invalidRequest", message: "the payload has a member agent-request-v2 does not define"},
		{name: "no method", edit: map[string]string{"method": ""},
			code: "invalidRequest", message: "the payload is missing method"},
		{name: "no audience", edit: map[string]string{"audience": ""},
			code: "invalidRequest", message: "the payload is missing audience"},
		{name: "no nonce", edit: map[string]string{"nonce": ""},
			code: "nonceRequired", message: "the signed statement has no nonce"},
		{name: "an empty method", edit: map[string]string{"method": `""`},
			code: "invalidRequest", message: form("method")},
		{name: "a target that is not a path", edit: map[string]string{"target": `"protected"`},
			code: "invalidRequest", message: form("target")},
		{name: "a number for the timestamp", edit: map[string]string{"timestamp": `1760875200`},
			code: "invalidRequest", message: form("timestamp")},
		{name: "a timestamp with an offset", edit: map[string]string{"timestamp": `"2026-10-19T12:00:00+00:00"`},
			code: "invalidRequest", message: form("timestamp")},
		{name: "an agent_id in braces", edit: map[string]string{"agent_id": jsonString("{" + agent.ID.String() + "}")},
			code: "invalidRequest", message: form("agent_id")},
		{name: "a padded nonce", edit: map[string]string{"nonce": jsonString(base64.URLEncoding.EncodeToString(make([]byte, 16)))},
			code: "invalidRequest", message: form("nonce")},
		{name: "an object for the nonce", edit: map[string]string{"nonce": `{"bytes":"AAAAAAAAAAAAAAAAAAAAAA"}`},
			code: "invalidRequest", message: form("nonce")},
		{name: "a body_sha256 of 31 bytes", edit: map[string]string{"body_sha256": jsonString(b64url(make([]byte, 31)))},
			code: "invalidRequest", message: form("body_sha256")},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			h := newV2Harness(v2Audience, agent)
			req := v2Request{agentID: agent.ID, keys: keys, nonce: make([]byte, 16), edit: cell.edit, statement: cell.header}
			if cell.payload != nil {
				req.payload = cell.payload(valid(h))
			}
			status, body := h.send(t, req)
			assert.Equal(t, fiber.StatusBadRequest, status)
			assert.Equal(t, v2Refusal(cell.code, cell.message), body)
			assert.Equal(t, 0, h.reached)
			assert.Equal(t, 0, h.agents.reads, "no agent row is read for a malformed statement")
			assert.Empty(t, h.nonces.calls)
		})
	}
}

// A signed agent request carries no other credential: whichever authenticator
// would read the other header, the request is refused here, before anything is
// read.
func TestAgentRequestV2_RefusesAStatementBesideAnotherCredential(t *testing.T) {
	others := []string{
		"Authorization", "X-API-Key",
		"X-Agent-ID", "X-Algorithm", "X-Timestamp", "X-Signature", "X-Signature-Ed25519", "X-Signature-MLDSA",
		"X-Public-Key", "X-PQC-Public-Key",
		"X-AIM-Agent-ID", "X-AIM-Timestamp", "X-AIM-Signature",
	}
	assert.Equal(t, others, agentRequestOtherPrincipalHeaders)

	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	for _, name := range others {
		t.Run(name, func(t *testing.T) {
			h := newV2Harness(v2Audience, agent)
			status, body := h.send(t, v2Request{agentID: agent.ID, keys: keys,
				headers: map[string]string{name: "not-a-real-credential"}})
			assert.Equal(t, fiber.StatusBadRequest, status)
			assert.Equal(t, v2Refusal("conflictingAuthentication",
				"X-Agent-Request cannot be combined with "+name+": a request is authenticated one way"), body)
			assert.Equal(t, 0, h.reached)
			assert.Equal(t, 0, h.agents.reads)
			assert.Empty(t, h.nonces.calls)
		})
	}
}

// A refused statement stops at this middleware: an authenticator after it does
// not get the request.
func TestAgentRequestV2_ARefusalDoesNotReachALaterAuthenticator(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	h := newV2Harness(v2Audience, agent)

	later := 0
	app := fiber.New(fiber.Config{ReadBufferSize: 16384})
	app.Use(AgentRequestV2Middleware(h.agents, h.nonces, v2Audience))
	app.Use(func(c fiber.Ctx) error {
		later++
		return c.Next()
	})
	app.Get(v2Path, func(c fiber.Ctx) error { return c.SendString("reached") })
	h.app = app

	status, _ := h.send(t, v2Request{agentID: agent.ID, keys: keys, flip: true})
	assert.Equal(t, fiber.StatusUnauthorized, status)
	status, _ = h.send(t, v2Request{agentID: agent.ID, keys: keys, edit: map[string]string{"audience": `"https://aim.other.test"`}})
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, 0, later)

	status, _ = h.send(t, v2Request{agentID: agent.ID, keys: keys})
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 1, later)
}

func TestAgentRequestV2_RefusesEveryStatementWhenNoAudienceIsConfigured(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	want := v2Refusal("audienceNotConfigured",
		"this server has no configured public origin and accepts no agent-request-v2 statement")

	for name, req := range map[string]v2Request{
		"a valid statement":           {agentID: agent.ID, keys: keys},
		"a statement beside a bearer": {agentID: agent.ID, keys: keys, headers: map[string]string{"Authorization": "Bearer not-a-real-token"}},
		"a malformed statement":       {agentID: agent.ID, keys: keys, statement: "not base64url"},
	} {
		t.Run(name, func(t *testing.T) {
			h := newV2Harness("", agent)
			status, body := h.send(t, req)
			assert.Equal(t, fiber.StatusServiceUnavailable, status)
			assert.Equal(t, want, body)
			assert.Equal(t, 0, h.reached)
			assert.Equal(t, 0, h.agents.reads)
			assert.Empty(t, h.nonces.calls)
		})
	}
}

func TestAgentRequestV2_LeavesARequestWithoutTheStatementToTheNextHandler(t *testing.T) {
	for _, audience := range []string{v2Audience, ""} {
		h := newV2Harness(audience)
		req := httptest.NewRequest(fiber.MethodGet, v2Path, nil)
		req.Header.Set("X-Agent-ID", uuid.NewString())
		req.Header.Set("X-Agent-Request-Signature", "Ed25519:"+b64url(make([]byte, 64)))
		resp, err := h.app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.Equal(t, 1, h.reached)
		assert.Nil(t, h.locals["agent_id"])
		assert.Equal(t, 0, h.agents.reads)
		assert.Empty(t, h.nonces.calls)
	}
}

// The status is read after the nonce is admitted, so a statement from a
// suspended or revoked agent spends its nonce and reaches nothing; a statement
// whose signature does not verify learns nothing of the status and spends none.
func TestAgentRequestV2_ReadsTheStatusAfterAdmission(t *testing.T) {
	keys := newAgentKeys(t)
	for _, status := range []domain.AgentStatus{domain.AgentStatusSuspended, domain.AgentStatusRevoked} {
		t.Run(string(status), func(t *testing.T) {
			agent := v2Agent(keys, status, false, false)
			h := newV2Harness(v2Audience, agent)

			code, body := h.send(t, v2Request{agentID: agent.ID, keys: keys, flip: true})
			assert.Equal(t, fiber.StatusUnauthorized, code)
			assert.Equal(t, v2Refusal("signatureInvalid",
				"the Ed25519 signature does not verify over the statement with this agent's registered key"), body)
			assert.Equal(t, 0, h.stored())

			req := v2Request{agentID: agent.ID, keys: keys, nonce: v2Nonce(t)}
			code, body = h.send(t, req)
			assert.Equal(t, fiber.StatusUnauthorized, code)
			assert.Equal(t, statusDeniedBody(string(status)), body)
			assert.Equal(t, 1, h.stored(), "the nonce is spent")
			assert.Equal(t, 0, h.reached)

			code, body = h.send(t, req)
			assert.Equal(t, fiber.StatusUnauthorized, code)
			assert.Equal(t, v2Refusal("nonceReused", "this nonce was already used by this agent"), body)
		})
	}

	// pending authenticates, as it does through the other agent authenticators.
	agent := v2Agent(keys, domain.AgentStatusPending, false, false)
	h := newV2Harness(v2Audience, agent)
	code, body := h.send(t, v2Request{agentID: agent.ID, keys: keys})
	assert.Equal(t, fiber.StatusOK, code, body)
}

func TestAgentRequestV2_RefusesWhenAdmissionCannotRun(t *testing.T) {
	keys := newAgentKeys(t)
	agent := v2Agent(keys, domain.AgentStatusVerified, false, false)
	unavailable := v2Refusal("freshnessCheckUnavailable",
		"cannot check this request's time and nonce against the AIM database; nothing was recorded")

	for _, tc := range []struct {
		err   error
		class string
	}{
		{domain.ErrActionRequestNoncePurgeNotRunning, "purge_not_running"},
		{domain.ErrActionRequestAdmissionTimedOut, "statement_timeout"},
		{errors.New("connection refused"), "other"},
	} {
		t.Run(tc.class, func(t *testing.T) {
			h := newV2Harness(v2Audience, agent)
			h.nonces.err = tc.err
			var status int
			var body string
			logged := captureLog(t, func() { status, body = h.send(t, v2Request{agentID: agent.ID, keys: keys}) })
			assert.Equal(t, fiber.StatusServiceUnavailable, status)
			assert.Equal(t, unavailable, body)
			assert.Contains(t, logged, "agent-request-v2 refused: freshness check unavailable (error_class="+tc.class+")")
			assert.NotContains(t, logged, agent.ID.String(), "the log line carries no request value")
			assert.Equal(t, 0, h.reached)
		})
	}

	t.Run("no admission store", func(t *testing.T) {
		h := newV2Harness(v2Audience, agent)
		h.mount(v2Audience, nil)
		var status int
		var body string
		logged := captureLog(t, func() { status, body = h.send(t, v2Request{agentID: agent.ID, keys: keys}) })
		assert.Equal(t, fiber.StatusServiceUnavailable, status)
		assert.Equal(t, unavailable, body)
		assert.Contains(t, logged, "agent-request-v2 refused: freshness check unavailable (error_class=no_store)")
		assert.Equal(t, 0, h.reached)
	})

	// The server's nonce service admits nothing until its first purge completes.
	t.Run("the nonce service before and after its first purge", func(t *testing.T) {
		h := newV2Harness(v2Audience, agent)
		service := application.NewActionRequestNonceService(h.nonces)
		h.mount(v2Audience, service)

		var status int
		var body string
		logged := captureLog(t, func() { status, body = h.send(t, v2Request{agentID: agent.ID, keys: keys}) })
		assert.Equal(t, fiber.StatusServiceUnavailable, status)
		assert.Equal(t, unavailable, body)
		assert.Contains(t, logged, "error_class=purge_not_running")
		assert.Empty(t, h.nonces.calls, "the store is not reached")

		_, err := service.Purge(context.Background())
		require.NoError(t, err)
		status, body = h.send(t, v2Request{agentID: agent.ID, keys: keys})
		assert.Equal(t, fiber.StatusOK, status, body)
		assert.Equal(t, 1, h.stored())
	})
}

// The window is the admission call's: the middleware reads no clock, no window
// value and no configuration, and calls Admit once.
func TestAgentRequestV2_AdmissionIsTheOnlyWindowOnItsPath(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "agent_request_v2.go", nil, 0)
	require.NoError(t, err)

	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		assert.NotContains(t, []string{"os", "flag", "github.com/spf13/viper"}, path)
		assert.False(t, strings.HasSuffix(path, "/internal/config"), path)
	}

	admits := 0
	middleware := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == "AgentRequestV2Middleware" {
				middleware = true
			}
		case *ast.SelectorExpr:
			if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "time" {
				switch x.Sel.Name {
				case "Now", "Since", "Until", "After", "Tick", "NewTimer", "NewTicker":
					t.Errorf("%s: reads the process clock (time.%s)", fset.Position(x.Pos()), x.Sel.Name)
				}
			}
			switch x.Sel.Name {
			case "ActionRequestWindowSeconds", "ActionRequestNonceRetentionSeconds":
				t.Errorf("%s: reads a window value (%s)", fset.Position(x.Pos()), x.Sel.Name)
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Admit" {
				admits++
			}
		}
		return true
	})
	require.True(t, middleware, "the walk did not find AgentRequestV2Middleware")
	assert.Equal(t, 1, admits, "calls to Admit")
}
