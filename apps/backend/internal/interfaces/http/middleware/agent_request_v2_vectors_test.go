package middleware

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The agent-request-v2 conformance vectors in docs/specs: signed requests, as
// the bytes a client sends, each with the answer this middleware gives it.
// buildAgentRequestV2Vectors makes them from keys derived from public labels.
// TestAgentRequestV2VectorsAreCurrent holds the committed file to what it makes,
// and TestAgentRequestV2Vectors sends every committed request through
// AgentRequestV2Middleware. The format is written out here, header names and
// payload type included, so the vectors state it independently of the
// middleware's declarations.

var updateAgentRequestV2Vectors = flag.Bool("update", false, "rewrite docs/specs/agent-request-v2-vectors.json")

var agentRequestV2VectorPath = filepath.Join("..", "..", "..", "..", "..", "..", "docs", "specs", "agent-request-v2-vectors.json")

const (
	vecPayloadType       = "application/vnd.opena2a.agent-request.v2+json"
	vecActionPayloadType = "application/vnd.opena2a.action-request.v1+json"
	vecServerTime        = "2026-10-20T12:00:00Z"
	vecOrganization      = "00000000-0000-4000-8000-0000000000a0"
	vecUnregisteredAgent = "00000000-0000-4000-8000-000000000009"
	vecQueryTarget       = "/protected?limit=10&offset=0"
	vecBody              = `{"name":"example agent"}`
)

type v2VectorFile struct {
	Scheme      string          `json:"scheme"`
	Description []string        `json:"description"`
	PayloadType string          `json:"payloadType"`
	Audience    string          `json:"audience"`
	ServerTime  string          `json:"serverTime"`
	Agents      []v2VectorAgent `json:"agents"`
	Vectors     []v2Vector      `json:"vectors"`
}

type v2VectorAgent struct {
	Name           string `json:"name"`
	AgentID        string `json:"agentId"`
	OrganizationID string `json:"organizationId"`
	Status         string `json:"status"`
	HybridMode     bool   `json:"hybridMode"`
	Ed25519Label   string `json:"ed25519Label"`
	Ed25519        string `json:"ed25519"`
	MLDSA65Label   string `json:"mlDsa65Label,omitempty"`
	MLDSA65        string `json:"mlDsa65,omitempty"`
}

type v2Vector struct {
	Name         string                `json:"name"`
	Signer       string                `json:"signer"`
	StoredNonces []v2VectorStoredNonce `json:"storedNonces"`
	SignedText   string                `json:"signedText"`
	Request      v2VectorRequest       `json:"request"`
	Want         v2VectorWant          `json:"want"`
}

type v2VectorStoredNonce struct {
	AgentID string `json:"agentId"`
	Nonce   string `json:"nonce"`
}

type v2VectorRequest struct {
	Method     string      `json:"method"`
	Target     string      `json:"target"`
	Body       string      `json:"body"`
	Statement  string      `json:"statement"`
	Signatures []string    `json:"signatures"`
	Headers    [][2]string `json:"headers"`
}

type v2VectorWant struct {
	Status      int    `json:"status"`
	ReasonCode  string `json:"reasonCode"`
	NonceStored bool   `json:"nonceStored"`
}

// vecAgent is a registered agent whose keys are derived from public labels.
type vecAgent struct {
	name    string
	id      uuid.UUID
	status  domain.AgentStatus
	hybrid  bool
	edLabel string
	edPriv  ed25519.PrivateKey
	mlLabel string
	mlPub   []byte // nil when the agent has no ML-DSA-65 key
	mlPriv  []byte
}

// newVecAgent derives the agent's keys: each seed is the SHA-256 of its label.
func newVecAgent(name, id string, status domain.AgentStatus, withMLDSA, hybrid bool) vecAgent {
	a := vecAgent{name: name, id: uuid.MustParse(id), status: status, hybrid: hybrid}
	a.edLabel = "agent-request-v2 vectors: " + name + " agent, Ed25519"
	edSeed := sha256.Sum256([]byte(a.edLabel))
	a.edPriv = ed25519.NewKeyFromSeed(edSeed[:])
	if withMLDSA {
		a.mlLabel = "agent-request-v2 vectors: " + name + " agent, ML-DSA-65"
		mlSeed := sha256.Sum256([]byte(a.mlLabel))
		pk, sk := mldsa65.NewKeyFromSeed(&mlSeed)
		a.mlPub, a.mlPriv = pk.Bytes(), sk.Bytes()
	}
	return a
}

func (a vecAgent) entry() v2VectorAgent {
	e := v2VectorAgent{
		Name:           a.name,
		AgentID:        a.id.String(),
		OrganizationID: vecOrganization,
		Status:         string(a.status),
		HybridMode:     a.hybrid,
		Ed25519Label:   a.edLabel,
		Ed25519:        base64.StdEncoding.EncodeToString(a.edPriv.Public().(ed25519.PublicKey)),
	}
	if a.mlPub != nil {
		e.MLDSA65Label = a.mlLabel
		e.MLDSA65 = base64.StdEncoding.EncodeToString(a.mlPub)
	}
	return e
}

// signature is one signature header value over message.
func (a vecAgent) signature(t *testing.T, alg pqc.Algorithm, message []byte) string {
	t.Helper()
	switch alg {
	case pqc.AlgorithmEd25519:
		return "Ed25519:" + b64url(ed25519.Sign(a.edPriv, message))
	case pqc.AlgorithmMLDSA65:
		require.NotNil(t, a.mlPriv, "agent %s has no ML-DSA-65 key", a.name)
		sig, err := pqc.SignMLDSA(alg, a.mlPriv, message)
		require.NoError(t, err)
		return "ML-DSA-65:" + b64url(sig)
	}
	t.Fatalf("agent %s has no %s key", a.name, alg)
	return ""
}

// vecPAE is the DSSE v1 pre-authentication encoding of payloadType and payload.
func vecPAE(payloadType string, payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload))
}

// vecStatement is one statement's members, as values.
type vecStatement struct {
	method, target, audience string
	bodySHA256               string // "" for none
	agentID                  string
	timestamp                string
	nonce                    string
}

// vecJSONString is s as a JSON string without the HTML escaping json.Marshal
// applies, so the & of a query is written as itself and not as &.
func vecJSONString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// payload is the statement as compact JSON, after edit (see encodeV2Payload).
func (s vecStatement) payload(edit map[string]string) []byte {
	m := [][2]string{
		{"method", vecJSONString(s.method)},
		{"target", vecJSONString(s.target)},
		{"audience", vecJSONString(s.audience)},
	}
	if s.bodySHA256 != "" {
		m = append(m, [2]string{"body_sha256", vecJSONString(s.bodySHA256)})
	}
	m = append(m,
		[2]string{"agent_id", vecJSONString(s.agentID)},
		[2]string{"timestamp", vecJSONString(s.timestamp)},
		[2]string{"nonce", vecJSONString(s.nonce)},
	)
	return encodeV2Payload(m, edit)
}

// vecCase is one vector as the builder states it.
type vecCase struct {
	name       string
	signer     vecAgent
	stored     []v2VectorStoredNonce
	method     string
	target     string
	body       string
	statement  []byte
	signed     []byte
	signatures []string
	fold       bool
	headers    [][2]string
	want       v2VectorWant
}

func (c vecCase) vector() v2Vector {
	lines := c.signatures
	if c.fold {
		lines = []string{strings.Join(c.signatures, ", ")}
	}
	headers := c.headers
	if headers == nil {
		headers = [][2]string{}
	}
	if c.body != "" {
		headers = append([][2]string{{"Content-Type", "application/json"}}, headers...)
	}
	stored := c.stored
	if stored == nil {
		stored = []v2VectorStoredNonce{}
	}
	return v2Vector{
		Name:         c.name,
		Signer:       c.signer.name,
		StoredNonces: stored,
		SignedText:   string(c.signed),
		Request: v2VectorRequest{
			Method:     c.method,
			Target:     c.target,
			Body:       c.body,
			Statement:  string(c.statement),
			Signatures: lines,
			Headers:    headers,
		},
		Want: c.want,
	}
}

func buildAgentRequestV2Vectors(t *testing.T) v2VectorFile {
	t.Helper()
	serverTime, err := time.Parse(time.RFC3339, vecServerTime)
	require.NoError(t, err)

	edAgent := newVecAgent("ed25519", "00000000-0000-4000-8000-000000000001", domain.AgentStatusVerified, false, false)
	mlAgent := newVecAgent("ml-dsa-65", "00000000-0000-4000-8000-000000000002", domain.AgentStatusVerified, true, false)
	hybridAgent := newVecAgent("hybrid", "00000000-0000-4000-8000-000000000003", domain.AgentStatusVerified, true, true)
	suspended := newVecAgent("suspended", "00000000-0000-4000-8000-000000000004", domain.AgentStatusSuspended, false, false)

	at := func(offset time.Duration) string { return serverTime.Add(offset).UTC().Format(time.RFC3339Nano) }
	nonce := func(label string, size int) string {
		sum := sha256.Sum256([]byte("agent-request-v2 vectors: nonce " + label))
		return b64url(sum[:size])
	}
	statement := func(a vecAgent, method, target, body, nonceValue, timestamp string) vecStatement {
		s := vecStatement{method: method, target: target, audience: v2Audience, agentID: a.id.String(),
			timestamp: timestamp, nonce: nonceValue}
		if body != "" {
			s.bodySHA256 = sha256B64url(body)
		}
		return s
	}
	// signed is a request carrying stmt, signed by signer over the encoding of
	// stmt under the agent-request-v2 payload type, one signature per algorithm.
	signed := func(name string, signer vecAgent, method, target, body string, stmt []byte, algs ...pqc.Algorithm) vecCase {
		c := vecCase{name: name, signer: signer, method: method, target: target, body: body, statement: stmt,
			signed: vecPAE(vecPayloadType, stmt)}
		for _, alg := range algs {
			c.signatures = append(c.signatures, signer.signature(t, alg, c.signed))
		}
		return c
	}
	accepted := v2VectorWant{Status: fiber.StatusOK, NonceStored: true}
	refused := func(status int, reasonCode string) v2VectorWant {
		return v2VectorWant{Status: status, ReasonCode: reasonCode}
	}
	ed := pqc.AlgorithmEd25519
	ml := pqc.AlgorithmMLDSA65
	get, post := fiber.MethodGet, fiber.MethodPost

	var cases []vecCase
	add := func(c vecCase, want v2VectorWant) {
		c.want = want
		cases = append(cases, c)
	}

	// Accepted.
	firstNonce := nonce("ed25519 get", 16)
	first := signed("Ed25519, a GET with a query", edAgent, get, vecQueryTarget, "",
		statement(edAgent, get, vecQueryTarget, "", firstNonce, at(0)).payload(nil), ed)
	add(first, accepted)
	add(signed("Ed25519, a POST with a body", edAgent, post, v2Path, vecBody,
		statement(edAgent, post, v2Path, vecBody, nonce("ed25519 post", 16), at(0)).payload(nil), ed), accepted)
	add(signed("ML-DSA-65 alone, from an agent not in hybrid mode", mlAgent, get, v2Path, "",
		statement(mlAgent, get, v2Path, "", nonce("ml-dsa-65", 16), at(0)).payload(nil), ml), accepted)
	add(signed("hybrid mode, Ed25519 and ML-DSA-65 on two header lines", hybridAgent, get, v2Path, "",
		statement(hybridAgent, get, v2Path, "", nonce("hybrid lines", 16), at(0)).payload(nil), ed, ml), accepted)
	folded := signed("hybrid mode, Ed25519 and ML-DSA-65 on one comma-separated line", hybridAgent, post, v2Path, vecBody,
		statement(hybridAgent, post, v2Path, vecBody, nonce("hybrid folded", 16), at(0)).payload(nil), ed, ml)
	folded.fold = true
	add(folded, accepted)
	add(signed("timestamp 30 seconds behind the server clock", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("behind 30", 16), at(-30*time.Second)).payload(nil), ed), accepted)
	add(signed("timestamp 30 seconds ahead of the server clock", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("ahead 30", 16), at(30*time.Second)).payload(nil), ed), accepted)
	reordered := statement(edAgent, get, v2Path, "", nonce("reordered", 16), at(0))
	reorderedPayload := []byte("{\n" +
		`  "nonce": ` + vecJSONString(reordered.nonce) + ",\n" +
		`  "timestamp": ` + vecJSONString(reordered.timestamp) + ",\n" +
		`  "agent_id": ` + vecJSONString(reordered.agentID) + ",\n" +
		`  "audience": ` + vecJSONString(reordered.audience) + ",\n" +
		`  "target": ` + vecJSONString(reordered.target) + ",\n" +
		`  "method": ` + vecJSONString(reordered.method) + "\n" +
		"}")
	add(signed("statement with line breaks, indentation and its members in another order", edAgent, get, v2Path, "",
		reorderedPayload, ed), accepted)

	// Refused: time and nonce.
	add(signed("timestamp 31 seconds behind the server clock", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("behind 31", 16), at(-31*time.Second)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "timestampOutsideWindow"))
	add(signed("timestamp 31 seconds ahead of the server clock", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("ahead 31", 16), at(31*time.Second)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "timestampOutsideWindow"))
	replay := first
	replay.name = "the first vector's request sent again, its nonce already stored"
	replay.stored = []v2VectorStoredNonce{{AgentID: edAgent.id.String(), Nonce: firstNonce}}
	add(replay, refused(fiber.StatusUnauthorized, "nonceReused"))
	add(signed("nonce of 15 bytes", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("15 bytes", 15), at(0)).payload(nil), ed),
		refused(fiber.StatusBadRequest, "invalidRequest"))
	add(signed("nonce of 17 bytes", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("17 bytes", 17), at(0)).payload(nil), ed),
		refused(fiber.StatusBadRequest, "invalidRequest"))
	add(signed("statement with no nonce", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", "", at(0)).payload(map[string]string{"nonce": ""}), ed),
		refused(fiber.StatusBadRequest, "nonceRequired"))

	// Refused: the statement does not name this request at this server.
	otherOrigin := statement(edAgent, get, v2Path, "", nonce("other origin", 16), at(0))
	otherOrigin.audience = "https://other.example.test"
	add(signed("audience of another origin", edAgent, get, v2Path, "", otherOrigin.payload(nil), ed),
		refused(fiber.StatusUnauthorized, "audienceMismatch"))
	upperCase := statement(edAgent, get, v2Path, "", nonce("upper case", 16), at(0))
	upperCase.audience = "https://AIM.example.test"
	add(signed("audience that differs from the server's only in letter case", edAgent, get, v2Path, "", upperCase.payload(nil), ed),
		refused(fiber.StatusUnauthorized, "audienceMismatch"))
	add(signed("a body and no body_sha256", edAgent, post, v2Path, vecBody,
		statement(edAgent, post, v2Path, "", nonce("no digest", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "requestMismatch"))
	add(signed("body_sha256 and no body", edAgent, post, v2Path, "",
		statement(edAgent, post, v2Path, vecBody, nonce("no body", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "requestMismatch"))
	add(signed("body_sha256 of other bytes than the body sent", edAgent, post, v2Path, vecBody,
		statement(edAgent, post, v2Path, `{"name":"another agent"}`, nonce("other digest", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "requestMismatch"))
	add(signed("target whose query is in another order than the one sent", edAgent, get, "/protected?offset=0&limit=10", "",
		statement(edAgent, get, vecQueryTarget, "", nonce("query order", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "requestMismatch"))
	add(signed("method POST signed and GET sent", edAgent, get, v2Path, "",
		statement(edAgent, post, v2Path, "", nonce("method", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "requestMismatch"))

	// Refused: another credential beside the statement, or a statement that is
	// not an agent-request-v2 payload.
	legacyText := "GET\n" + vecQueryTarget + "\n" + strconv.FormatInt(serverTime.Unix(), 10)
	beside := signed("X-Agent-ID, X-Timestamp and X-Signature beside the statement", edAgent, get, vecQueryTarget, "",
		statement(edAgent, get, vecQueryTarget, "", nonce("legacy beside", 16), at(0)).payload(nil), ed)
	beside.headers = [][2]string{
		{"X-Agent-ID", edAgent.id.String()},
		{"X-Timestamp", strconv.FormatInt(serverTime.Unix(), 10)},
		{"X-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(edAgent.edPriv, []byte(legacyText)))},
	}
	add(beside, refused(fiber.StatusBadRequest, "conflictingAuthentication"))
	bearer := signed("Authorization beside the statement", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("authorization beside", 16), at(0)).payload(nil), ed)
	bearer.headers = [][2]string{{"Authorization", "Bearer not-a-real-token"}}
	add(bearer, refused(fiber.StatusBadRequest, "conflictingAuthentication"))
	add(vecCase{
		name:   "an agent-request-v1 signed text sent as the statement, signed as agent-request-v1 signs it",
		signer: edAgent, method: get, target: vecQueryTarget,
		statement:  []byte(legacyText),
		signed:     []byte(legacyText),
		signatures: []string{edAgent.signature(t, ed, []byte(legacyText))},
	}, refused(fiber.StatusBadRequest, "invalidRequest"))
	add(signed("statement with a member the format does not define", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("extra member", 16), at(0)).payload(map[string]string{"scope": `"read"`}), ed),
		refused(fiber.StatusBadRequest, "invalidRequest"))

	// Refused: the signatures.
	add(signed("hybrid mode, Ed25519 alone", hybridAgent, get, v2Path, "",
		statement(hybridAgent, get, v2Path, "", nonce("hybrid ed25519 alone", 16), at(0)).payload(nil), ed),
		refused(fiber.StatusUnauthorized, "hybridSignatureRequired"))
	add(signed("not in hybrid mode, Ed25519 and ML-DSA-65", mlAgent, get, v2Path, "",
		statement(mlAgent, get, v2Path, "", nonce("two signatures", 16), at(0)).payload(nil), ed, ml),
		refused(fiber.StatusUnauthorized, "singleSignatureRequired"))
	noMLDSAKey := statement(edAgent, get, v2Path, "", nonce("no ml-dsa key", 16), at(0)).payload(nil)
	add(signed("ML-DSA-65 for an agent with no ML-DSA-65 key, signed with another agent's", mlAgent, get, v2Path, "",
		noMLDSAKey, ml), refused(fiber.StatusUnauthorized, "agentKeyNotRecognized"))
	thrice := signed("three signatures", edAgent, get, v2Path, "",
		statement(edAgent, get, v2Path, "", nonce("three signatures", 16), at(0)).payload(nil), ed)
	thrice.signatures = []string{thrice.signatures[0], thrice.signatures[0], thrice.signatures[0]}
	add(thrice, refused(fiber.StatusBadRequest, "invalidRequest"))
	actionType := statement(edAgent, get, v2Path, "", nonce("action type", 16), at(0)).payload(nil)
	add(vecCase{
		name:   "signed under the action-request-v1 payload type",
		signer: edAgent, method: get, target: v2Path,
		statement:  actionType,
		signed:     vecPAE(vecActionPayloadType, actionType),
		signatures: []string{edAgent.signature(t, ed, vecPAE(vecActionPayloadType, actionType))},
	}, refused(fiber.StatusUnauthorized, "signatureInvalid"))
	bare := statement(edAgent, get, v2Path, "", nonce("bare statement", 16), at(0)).payload(nil)
	add(vecCase{
		name:   "signed over the statement alone, without its encoding",
		signer: edAgent, method: get, target: v2Path,
		statement:  bare,
		signed:     bare,
		signatures: []string{edAgent.signature(t, ed, bare)},
	}, refused(fiber.StatusUnauthorized, "signatureInvalid"))
	unregistered := statement(edAgent, get, v2Path, "", nonce("unregistered", 16), at(0))
	unregistered.agentID = vecUnregisteredAgent
	add(signed("agent_id of no registered agent", edAgent, get, v2Path, "", unregistered.payload(nil), ed),
		refused(fiber.StatusUnauthorized, "agentKeyNotRecognized"))

	// Refused after admission: the nonce stays stored.
	add(signed("suspended agent", suspended, get, v2Path, "",
		statement(suspended, get, v2Path, "", nonce("suspended", 16), at(0)).payload(nil), ed),
		v2VectorWant{Status: fiber.StatusUnauthorized, ReasonCode: "agentStatusDenied", NonceStored: true})

	file := v2VectorFile{
		Scheme: "agent-request-v2",
		Description: []string{
			"Conformance vectors for agent-request-v2, the signed agent request of OpenA2A AIM (Agent Identity " +
				"Management). Each vector is one HTTP request and the answer the AIM backend's agent-request-v2 " +
				"verifier gives it.",
			"The request sends statement, the JSON payload, in the X-Agent-Request header as unpadded base64url, " +
				"and each entry of signatures as one X-Agent-Request-Signature header line. A signature is an " +
				"algorithm name, a colon and the unpadded base64url signature; one line may carry two signatures " +
				"separated by a comma. headers are the other headers the request sends.",
			"A signature covers the DSSE v1 pre-authentication encoding of payloadType and the statement: " +
				"\"DSSEv1\", a space, the byte length of payloadType in ASCII decimal, a space, payloadType, a " +
				"space, the byte length of the statement, a space and the statement. signedText is the bytes a " +
				"vector's signatures cover, which is that encoding unless the vector's name says otherwise.",
			"Statements are compact JSON with the members in the order method, target, audience, body_sha256, " +
				"agent_id, timestamp, nonce, unless the vector's name says otherwise. body_sha256 is the unpadded " +
				"base64url SHA-256 of the body; nonce is unpadded base64url.",
			"The verifier's public origin is audience and its database clock reads serverTime. The agents are " +
				"registered as listed, signer names the agent whose keys made a vector's signatures, and " +
				"storedNonces are the nonces already stored when the request arrives.",
			"want is the answer: the HTTP status, the reasonCode of a refusal, and nonceStored, whether the " +
				"request leaves its nonce stored. A request answered 200 reaches the route handler; a refused " +
				"request does not.",
			"Each key is derived from its label: the Ed25519 seed (RFC 8032) and the ML-DSA-65 seed (xi of " +
				"FIPS 204 ML-DSA.KeyGen_internal) are the SHA-256 of the label's UTF-8 bytes. Public keys are " +
				"standard base64. ML-DSA-65 signatures use the deterministic variant of FIPS 204 ML-DSA.Sign with " +
				"an empty context. These keys are test values and protect nothing.",
		},
		PayloadType: vecPayloadType,
		Audience:    v2Audience,
		ServerTime:  vecServerTime,
	}
	for _, a := range []vecAgent{edAgent, mlAgent, hybridAgent, suspended} {
		file.Agents = append(file.Agents, a.entry())
	}
	for _, c := range cases {
		file.Vectors = append(file.Vectors, c.vector())
	}
	return file
}

func encodeAgentRequestV2Vectors(t *testing.T, file v2VectorFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(file))
	return buf.Bytes()
}

// The committed vectors are exactly what buildAgentRequestV2Vectors makes. A
// change to the signed bytes, the keys or an expected answer turns this red.
func TestAgentRequestV2VectorsAreCurrent(t *testing.T) {
	want := encodeAgentRequestV2Vectors(t, buildAgentRequestV2Vectors(t))
	if *updateAgentRequestV2Vectors {
		require.NoError(t, os.WriteFile(agentRequestV2VectorPath, want, 0o600))
	}

	got, err := os.ReadFile(agentRequestV2VectorPath)
	require.NoError(t, err)
	if bytes.Equal(want, got) {
		return
	}
	wantLines, gotLines := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			t.Fatalf("%s differs from what the test builds at line %d:\n  committed: %.200s\n  built:     %.200s\n"+
				"run the test with -update only for a deliberate change of format", agentRequestV2VectorPath, i+1, g, w)
		}
	}
}

func readAgentRequestV2Vectors(t *testing.T) v2VectorFile {
	t.Helper()
	raw, err := os.ReadFile(agentRequestV2VectorPath)
	require.NoError(t, err)
	var file v2VectorFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&file))
	return file
}

// sendAgentRequestV2Vector sends r through app as the vector states it and
// returns the status and the raw body.
func sendAgentRequestV2Vector(t *testing.T, app *fiber.App, r v2VectorRequest) (int, string) {
	t.Helper()
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req := httptest.NewRequest(r.Method, r.Target, body)
	req.Header.Add("X-Agent-Request", b64url([]byte(r.Statement)))
	for _, line := range r.Signatures {
		req.Header.Add("X-Agent-Request-Signature", line)
	}
	for _, h := range r.Headers {
		req.Header.Add(h[0], h[1])
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// Every committed vector, sent through AgentRequestV2Middleware with the file's
// agents registered, the vector's stored nonces in the admission store and that
// store's clock at serverTime, gets the vector's answer. Beside the answer, each
// cell checks what the request caused: a request answered 200 reaches the
// handler once and stores its nonce; a refused one reaches no handler and stores
// a nonce only where the vector says so. The admission store is called for the
// accepted requests, the window and nonce refusals and the status refusal, and
// for no other, so every other refusal is decided before a nonce is spent.
func TestAgentRequestV2Vectors(t *testing.T) {
	file := readAgentRequestV2Vectors(t)
	require.Equal(t, domain.AgentRequestSchemeID, file.Scheme)
	require.Equal(t, domain.AgentRequestPayloadType, file.PayloadType)
	serverTime, err := time.Parse(time.RFC3339, file.ServerTime)
	require.NoError(t, err)

	agents := map[string]*domain.Agent{}
	for _, a := range file.Agents {
		id, err := uuid.Parse(a.AgentID)
		require.NoError(t, err)
		org, err := uuid.Parse(a.OrganizationID)
		require.NoError(t, err)
		edKey := a.Ed25519
		row := &domain.Agent{ID: id, OrganizationID: org, Status: domain.AgentStatus(a.Status),
			PublicKey: &edKey, HybridModeEnabled: a.HybridMode}
		if a.MLDSA65 != "" {
			mlKey := a.MLDSA65
			row.PQCPublicKey = &mlKey
		}
		agents[a.Name] = row
	}

	admittedBy := map[string]bool{"": true, "timestampOutsideWindow": true, "nonceReused": true, "agentStatusDenied": true}
	reasonCodes := map[string]bool{}
	acceptedForms := map[string]bool{}
	for _, v := range file.Vectors {
		reasonCodes[v.Want.ReasonCode] = true
		t.Run(v.Name, func(t *testing.T) {
			require.Contains(t, agents, v.Signer)
			h := &v2Harness{
				agents: &v2Agents{rows: map[uuid.UUID]*domain.Agent{}},
				nonces: &v2Nonces{now: serverTime, rows: map[string]bool{}},
			}
			for _, a := range agents {
				h.agents.rows[a.ID] = a
			}
			for _, n := range v.StoredNonces {
				raw, err := base64.RawURLEncoding.Strict().DecodeString(n.Nonce)
				require.NoError(t, err)
				require.Len(t, raw, domain.ActionRequestNonceBytes)
				h.nonces.rows[uuid.MustParse(n.AgentID).String()+"/"+n.Nonce] = true
			}
			h.mount(file.Audience, h.nonces)
			before := h.stored()

			status, body := sendAgentRequestV2Vector(t, h.app, v.Request)
			require.Equal(t, v.Want.Status, status, body)

			if v.Want.Status == fiber.StatusOK {
				assert.Equal(t, reachedBody, body)
				assert.Equal(t, 1, h.reached)
				assert.Equal(t, agents[v.Signer].ID, h.locals["agent_id"])
				assert.Equal(t, "agent-request-v2", h.locals["agent_request_scheme"])
				var algs []string
				for _, line := range v.Request.Signatures {
					var names []string
					for _, s := range strings.Split(line, ",") {
						name, _, _ := strings.Cut(strings.TrimSpace(s), ":")
						names = append(names, name)
					}
					algs = append(algs, strings.Join(names, ","))
				}
				acceptedForms[strings.Join(algs, " | ")] = true
			} else {
				var refusal map[string]string
				require.NoError(t, json.Unmarshal([]byte(body), &refusal), body)
				assert.Equal(t, v.Want.ReasonCode, refusal["reasonCode"], body)
				assert.Len(t, refusal, 2, "a refusal body is error and reasonCode: %s", body)
				assert.NotEmpty(t, refusal["error"])
				assert.Equal(t, 0, h.reached, "a refused request reaches no handler")
			}

			stored := 0
			if v.Want.NonceStored {
				stored = 1
			}
			assert.Equal(t, before+stored, h.stored(), "nonce rows")
			calls := 0
			if admittedBy[v.Want.ReasonCode] {
				calls = 1
			}
			assert.Len(t, h.nonces.calls, calls, "admission calls")
		})
	}

	// Each class of refusal has a vector, and each signature form an accepted one.
	var codes []string
	for code := range reasonCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	assert.Equal(t, []string{"", "agentKeyNotRecognized", "agentStatusDenied", "audienceMismatch",
		"conflictingAuthentication", "hybridSignatureRequired", "invalidRequest", "nonceRequired", "nonceReused",
		"requestMismatch", "signatureInvalid", "singleSignatureRequired", "timestampOutsideWindow"}, codes)
	for _, form := range []string{"Ed25519", "ML-DSA-65", "Ed25519 | ML-DSA-65", "Ed25519,ML-DSA-65"} {
		assert.True(t, acceptedForms[form], "no accepted vector signs as %q", form)
	}
}
