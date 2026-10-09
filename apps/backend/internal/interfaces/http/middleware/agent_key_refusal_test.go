package middleware

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// The agent-signature middlewares answer a request whose key is not an agent's registered
// key with one body, and read the agent's status only after the signature verifies.
//
// Before, both read the status first and named it in the refusal, so a caller holding
// only an agent id learned whether the agent existed and whether it was suspended or
// revoked: the four "no such key" causes each had their own string, and a suspended
// agent answered with its status to a request signed by any key at all.
//
// Every cell below is a byte-for-byte body comparison. A 401 alone proves nothing here,
// because every one of these requests was refused before the change too; what changed is
// which refusal, and so what the refusal tells an unauthenticated caller.

// keyNotRecognizedBody is written out as a literal so this file states the contract
// independently of the declaration it checks.
const keyNotRecognizedBody = `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`

func statusDeniedBody(status string) string {
	return `{"error":"Agent is not permitted to authenticate (status: ` + status + `)","reasonCode":"agentStatusDenied"}`
}

// agentKeys is one keypair per algorithm family.
type agentKeys struct {
	edPub   ed25519.PublicKey
	edPriv  ed25519.PrivateKey
	pqcPub  []byte
	pqcPriv []byte
}

func newAgentKeys(t *testing.T) agentKeys {
	t.Helper()
	edPub, edPriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	return agentKeys{edPub: edPub, edPriv: edPriv, pqcPub: kp.PublicKey, pqcPriv: kp.PrivateKey}
}

func b64(raw []byte) *string {
	s := base64.StdEncoding.EncodeToString(raw)
	return &s
}

// registeredAgent is an agent row in `status` holding both of `keys`.
func registeredAgent(status domain.AgentStatus, keys agentKeys) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Status:         status,
		PublicKey:      b64(keys.edPub),
		PQCPublicKey:   b64(keys.pqcPub),
	}
}

// signedRequest is one request a client builds: the agent id it names, the keys it
// signs with and presents, and an optional mutation applied to the signature bytes.
type signedRequest struct {
	agentID uuid.UUID
	alg     pqc.Algorithm
	signer  agentKeys
	flip    bool
	// presentEd25519 and presentPQC override the presented-key headers; nil presents
	// the signer's own key.
	presentEd25519 *string
	presentPQC     *string
}

func flipFirst(sig []byte) []byte {
	out := append([]byte(nil), sig...)
	out[0] ^= 0x01
	return out
}

// do sends r through app and returns the status code and the raw body.
func (r signedRequest) do(t *testing.T, app *fiber.App) (int, string) {
	t.Helper()

	const path = "/protected"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	message := []byte("GET\n" + path + "\n" + ts)

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Agent-ID", r.agentID.String())
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Algorithm", string(r.alg))

	edSig := ed25519.Sign(r.signer.edPriv, message)
	var pqcSig []byte
	if pqc.IsPQCAlgorithm(r.alg) || pqc.IsHybridAlgorithm(r.alg) {
		var err error
		pqcSig, err = pqc.SignMLDSA(pqc.AlgorithmMLDSA65, r.signer.pqcPriv, message)
		require.NoError(t, err)
	}
	if r.flip {
		edSig = flipFirst(edSig)
		if pqcSig != nil {
			pqcSig = flipFirst(pqcSig)
		}
	}

	presented := base64.StdEncoding.EncodeToString(r.signer.edPub)
	if r.presentEd25519 != nil {
		presented = *r.presentEd25519
	}
	presentedPQC := base64.StdEncoding.EncodeToString(r.signer.pqcPub)
	if r.presentPQC != nil {
		presentedPQC = *r.presentPQC
	}

	switch {
	case pqc.IsHybridAlgorithm(r.alg):
		req.Header.Set("X-Signature-Ed25519", base64.StdEncoding.EncodeToString(edSig))
		req.Header.Set("X-Signature-MLDSA", base64.StdEncoding.EncodeToString(pqcSig))
		req.Header.Set("X-Public-Key", presented)
		req.Header.Set("X-PQC-Public-Key", presentedPQC)
	case pqc.IsPQCAlgorithm(r.alg):
		req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(pqcSig))
		req.Header.Set("X-PQC-Public-Key", presentedPQC)
	default:
		req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(edSig))
		req.Header.Set("X-Public-Key", presented)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// newKeyRefusalApp mounts `mount` over a real AgentService whose repository knows only
// the given agents; any other id is answered the way the repository answers a missing row.
func newKeyRefusalApp(mount func(*application.AgentService) fiber.Handler, agents ...*domain.Agent) *fiber.App {
	repo := &mocks.MockAgentRepository{}
	for _, a := range agents {
		repo.On("GetByID", a.ID).Return(a, nil)
	}
	repo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))

	svc := application.NewAgentService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// The server's own header buffer (cmd/server): an ML-DSA-65 key and signature do not
	// fit in fiber's 4 KB default.
	app := fiber.New(fiber.Config{ReadBufferSize: 16384})
	app.Use(mount(svc))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"reached": true})
	})
	return app
}

// captureLog collects everything the standard logger writes while fn runs.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}()
	fn()
	return buf.String()
}

type keyRefusalMount struct {
	name  string
	mount func(*application.AgentService) fiber.Handler
	algs  []pqc.Algorithm
}

var keyRefusalMounts = []keyRefusalMount{
	{"PQCAgentMiddleware", PQCAgentMiddleware, []pqc.Algorithm{
		pqc.AlgorithmEd25519, pqc.AlgorithmMLDSA65, pqc.AlgorithmHybridEd25519MLDSA65,
	}},
	{"Ed25519AgentMiddleware", Ed25519AgentMiddleware, []pqc.Algorithm{pqc.AlgorithmEd25519}},
}

// The four causes that mean "this is not the registered key of a known agent" get one
// body, byte for byte, on every algorithm branch.
func TestAgentSignatureMiddlewares_AnswerEveryUnrecognisedKeyAlike(t *testing.T) {
	for _, m := range keyRefusalMounts {
		for _, alg := range m.algs {
			t.Run(m.name+"/"+string(alg), func(t *testing.T) {
				registered := newAgentKeys(t)
				other := newAgentKeys(t)

				noKey := registeredAgent(domain.AgentStatusVerified, registered)
				noKey.PublicKey, noKey.PQCPublicKey = nil, nil

				known := registeredAgent(domain.AgentStatusVerified, registered)

				app := newKeyRefusalApp(m.mount, noKey, known)

				cells := map[string]signedRequest{
					"unknown agent":     {agentID: uuid.New(), alg: alg, signer: other},
					"no registered key": {agentID: noKey.ID, alg: alg, signer: registered},
					"a different key":   {agentID: known.ID, alg: alg, signer: other},
				}
				for name, req := range cells {
					code, body := req.do(t, app)
					require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
					require.Equal(t, keyNotRecognizedBody, body, "%s must get the one merged body", name)
				}
			})
		}
	}
}

// A registered key that does not decode to its algorithm's key length is a corrupt row,
// not a bad request: the caller gets the merged body, and the operator gets exactly one
// log line, which carries neither the stored value nor its bytes.
func TestAgentSignatureMiddlewares_AMalformedStoredKeyGetsTheMergedBodyAndOneLogLine(t *testing.T) {
	for _, m := range keyRefusalMounts {
		for _, alg := range m.algs {
			t.Run(m.name+"/"+string(alg), func(t *testing.T) {
				registered := newAgentKeys(t)
				agent := registeredAgent(domain.AgentStatusVerified, registered)

				// The request presents the stored value itself, so a byte compare of the
				// presented key cannot be what refuses it. On the hybrid branch the ML-DSA
				// key is the corrupt one.
				short := []byte("sixteen-byte-key")
				stored := b64(short)
				req := signedRequest{agentID: agent.ID, alg: alg, signer: registered}
				if pqc.IsPQCAlgorithm(alg) {
					agent.PQCPublicKey, req.presentPQC = stored, stored
				} else {
					agent.PublicKey, req.presentEd25519 = stored, stored
				}
				app := newKeyRefusalApp(m.mount, agent)

				var code int
				var body string
				logged := captureLog(t, func() { code, body = req.do(t, app) })

				require.Equal(t, fiber.StatusUnauthorized, code, body)
				require.Equal(t, keyNotRecognizedBody, body)
				lines := strings.Split(strings.TrimRight(logged, "\n"), "\n")
				require.Len(t, lines, 1, "exactly one log line, got %q", logged)
				require.NotContains(t, logged, *stored)
				require.NotContains(t, logged, hex.EncodeToString(short))
				require.Contains(t, logged, agent.ID.String(), "the line names the agent whose row is corrupt")
			})
		}
	}
}

// The status is read only after the signature verifies. A suspended agent's status is
// therefore reachable only by a caller holding its key.
func TestAgentSignatureMiddlewares_ReadStatusOnlyAfterTheSignatureVerifies(t *testing.T) {
	for _, m := range keyRefusalMounts {
		for _, alg := range m.algs {
			t.Run(m.name+"/"+string(alg), func(t *testing.T) {
				registered := newAgentKeys(t)
				other := newAgentKeys(t)
				suspended := registeredAgent(domain.AgentStatusSuspended, registered)
				verified := registeredAgent(domain.AgentStatusVerified, registered)
				app := newKeyRefusalApp(m.mount, suspended, verified)

				// A different key: the merged body, exactly as for an unknown agent.
				code, body := signedRequest{agentID: suspended.ID, alg: alg, signer: other}.do(t, app)
				require.Equal(t, fiber.StatusUnauthorized, code, body)
				require.Equal(t, keyNotRecognizedBody, body,
					"a suspended agent asked with a different key must not answer with its status")

				// The registered key with a flipped signature byte: the signature refusal,
				// the same body a verified agent gets for the same mistake.
				code, suspendedBody := signedRequest{agentID: suspended.ID, alg: alg, signer: registered, flip: true}.do(t, app)
				require.Equal(t, fiber.StatusUnauthorized, code, suspendedBody)
				_, verifiedBody := signedRequest{agentID: verified.ID, alg: alg, signer: registered, flip: true}.do(t, app)
				require.Equal(t, verifiedBody, suspendedBody,
					"a flipped signature byte must get the same refusal whatever the agent's status")
				require.Contains(t, suspendedBody, `"reasonCode":"signatureInvalid"`)
				require.NotContains(t, suspendedBody, "suspended")

				// A valid request: now, and only now, the status refusal.
				code, body = signedRequest{agentID: suspended.ID, alg: alg, signer: registered}.do(t, app)
				require.Equal(t, fiber.StatusUnauthorized, code, body)
				require.Equal(t, statusDeniedBody("suspended"), body)

				// The control: the same request from a verified agent is admitted.
				code, body = signedRequest{agentID: verified.ID, alg: alg, signer: registered}.do(t, app)
				require.Equal(t, fiber.StatusOK, code, body)
			})
		}
	}
}

// H: a hybrid-mode agent asked with keys that are not its own gets the merged body, not
// a refusal about hybrid mode; the hybrid flag is an attribute, read after verification.
func TestPQCAgentMiddleware_AHybridModeAgentWithADifferentKeyGetsTheMergedBody(t *testing.T) {
	registered := newAgentKeys(t)
	other := newAgentKeys(t)

	hybrid := registeredAgent(domain.AgentStatusVerified, registered)
	hybrid.HybridModeEnabled = true

	// No PQC key at all: the old code answered with a hybrid-specific string.
	noPQC := registeredAgent(domain.AgentStatusVerified, registered)
	noPQC.PQCPublicKey = nil

	app := newKeyRefusalApp(PQCAgentMiddleware, hybrid, noPQC)

	for name, req := range map[string]signedRequest{
		"hybrid-mode agent, different keys":           {agentID: hybrid.ID, alg: pqc.AlgorithmHybridEd25519MLDSA65, signer: other},
		"hybrid-mode agent, Ed25519 with another key": {agentID: hybrid.ID, alg: pqc.AlgorithmEd25519, signer: other},
		"hybrid request, no registered PQC key":       {agentID: noPQC.ID, alg: pqc.AlgorithmHybridEd25519MLDSA65, signer: other},
	} {
		code, body := req.do(t, app)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, keyNotRecognizedBody, body, name)
	}
}

// Sanity for the helpers: a request signed by the registered keys verifies on every
// branch, so a refusal above is never the harness's own mistake.
func TestAgentKeyRefusalHarness_SignsWhatTheMiddlewareVerifies(t *testing.T) {
	registered := newAgentKeys(t)
	agent := registeredAgent(domain.AgentStatusVerified, registered)
	app := newKeyRefusalApp(PQCAgentMiddleware, agent)
	for _, alg := range keyRefusalMounts[0].algs {
		code, body := signedRequest{agentID: agent.ID, alg: alg, signer: registered}.do(t, app)
		require.Equal(t, fiber.StatusOK, code, fmt.Sprintf("%s: %s", alg, body))
	}
}
