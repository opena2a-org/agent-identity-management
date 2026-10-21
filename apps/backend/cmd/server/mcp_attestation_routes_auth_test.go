package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// GET /api/v1/mcp-servers/:id/challenge and POST /api/v1/mcp-servers/:id/attest are
// mounted on the agent-signed MCP group. PQCAgentMiddleware passes a request on
// unauthenticated when it carries an Authorization header or lacks X-Agent-ID or
// X-Timestamp; AuthMiddleware then answers it, as on the /agents, /detection and /a2a
// groups, so no such request reaches the handler without an organization.
func TestMCPAttestationAgentRoutesFallBackToAuthMiddleware(t *testing.T) {
	app, jwtService := strictCensusApp(t)
	serverID, agentID := uuid.NewString(), uuid.NewString()
	session, err := jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), "attestation@example.com", "member")
	require.NoError(t, err)

	routes := []struct{ method, path, body string }{
		{fiber.MethodGet, "/api/v1/mcp-servers/" + serverID + "/challenge?agent_id=" + agentID, ""},
		{fiber.MethodPost, "/api/v1/mcp-servers/" + serverID + "/attest", "{}"},
	}
	probes := []struct {
		name    string
		headers map[string]string
		status  int
		body    map[string]any
	}{
		{"no credentials", nil,
			fiber.StatusUnauthorized, map[string]any{"error": "No authentication token provided"}},
		{"an agent ID without a timestamp", map[string]string{"X-Agent-ID": agentID},
			fiber.StatusUnauthorized, map[string]any{"error": "No authentication token provided"}},
		{"a bearer value that is not a token", map[string]string{"Authorization": "Bearer not-a-token"},
			fiber.StatusUnauthorized, map[string]any{"error": "Invalid or expired token"}},
		{"an Authorization header of another scheme", map[string]string{"Authorization": "Basic not-a-credential"},
			fiber.StatusUnauthorized, map[string]any{"error": "Invalid authorization header format"}},
		// AuthMiddleware admits a signed-in user's session and sets its organization. The
		// handler then runs; it has no service in this test and answers 500.
		{"a signed-in user's session", map[string]string{"Authorization": "Bearer " + session},
			fiber.StatusInternalServerError, nil},
	}

	for _, route := range routes {
		for _, probe := range probes {
			t.Run(route.method+" "+strings.SplitN(route.path, "?", 2)[0][len("/api/v1/mcp-servers/")+len(serverID):]+"/"+probe.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
				for name, value := range probe.headers {
					req.Header.Set(name, value)
				}
				resp, err := app.Test(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, probe.status, resp.StatusCode)
				if probe.body != nil {
					var body map[string]any
					require.NoError(t, json.NewDecoder(resp.Body).Decode(&body), "the answer is AuthMiddleware's JSON refusal")
					assert.Equal(t, probe.body, body)
				}
			})
		}
	}
}

// AuthMiddleware on these two routes answers only a request PQCAgentMiddleware did not
// authenticate. A request it authenticated reaches the handler whichever algorithm the
// agent signed with: Ed25519, ML-DSA-65, or both, the way an agent in hybrid mode must
// sign. The MCP server ID in each path is not a UUID, so the handler answers with its own
// first check, which reads no service.
func TestMCPAttestationAgentRoutesAdmitEverySignatureAlgorithm(t *testing.T) {
	keys := newAttestationRouteKeys(t)
	agent, hybridAgent := keys.agent(false), keys.agent(true)
	app := agentSignedRoutesApp(t, agent, hybridAgent)

	routes := []struct{ method, path, body string }{
		{fiber.MethodGet, "/api/v1/mcp-servers/not-a-uuid/challenge", ""},
		{fiber.MethodPost, "/api/v1/mcp-servers/not-a-uuid/attest", "{}"},
	}
	probes := []struct {
		name  string
		agent *domain.Agent
		alg   pqc.Algorithm
	}{
		{"Ed25519", agent, pqc.AlgorithmEd25519},
		{"ML-DSA-65", agent, pqc.AlgorithmMLDSA65},
		{"Ed25519+ML-DSA-65", agent, pqc.AlgorithmHybridEd25519MLDSA65},
		{"Ed25519+ML-DSA-65 from an agent in hybrid mode", hybridAgent, pqc.AlgorithmHybridEd25519MLDSA65},
	}

	for _, route := range routes {
		for _, probe := range probes {
			t.Run(route.method+" "+route.path[strings.LastIndex(route.path, "/"):]+"/"+probe.name, func(t *testing.T) {
				path := route.path
				if route.method == fiber.MethodGet {
					path += "?agent_id=" + probe.agent.ID.String()
				}
				status, body := keys.send(t, app, probe.agent.ID, probe.alg, route.method, path, route.body)

				assert.Equal(t, fiber.StatusBadRequest, status)
				assert.Equal(t, map[string]any{"error": "Invalid MCP server ID", "message": "id must be a UUID"}, body,
					"the handler's own answer")
			})
		}

		// PQCAgentMiddleware still refuses an agent in hybrid mode that signs with one key.
		t.Run(route.method+" "+route.path[strings.LastIndex(route.path, "/"):]+"/Ed25519 alone from an agent in hybrid mode", func(t *testing.T) {
			path := route.path
			if route.method == fiber.MethodGet {
				path += "?agent_id=" + hybridAgent.ID.String()
			}
			status, body := keys.send(t, app, hybridAgent.ID, pqc.AlgorithmEd25519, route.method, path, route.body)

			assert.Equal(t, fiber.StatusUnauthorized, status)
			refusal, _ := body["error"].(string)
			assert.True(t, strings.HasPrefix(refusal, "Agent is in hybrid mode"), "PQCAgentMiddleware's refusal, got %q", refusal)
		})
	}
}

// agentSignedRoutesApp mounts the server's routes over a real AgentService whose
// repository holds agents, with the server's header buffer: an ML-DSA-65 key and
// signature do not fit in fiber's 4 KB default.
func agentSignedRoutesApp(t *testing.T, agents ...*domain.Agent) *fiber.App {
	t.Helper()
	t.Setenv("JWT_SECRET", "FAKE-not-a-real-jwt-secret-for-route-tests")

	repo := &mocks.MockAgentRepository{}
	for _, a := range agents {
		repo.On("GetByID", a.ID).Return(a, nil)
	}
	repo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))
	agentService := application.NewAgentService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New(fiber.Config{ReadBufferSize: 16384})
	mountAPIRoutes(app, &Handlers{}, &Services{Secrets: &application.SecretsService{}, Agent: agentService}, auth.NewJWTService(), nil, nil)
	return app
}

// attestationRouteKeys is one agent keypair per algorithm family.
type attestationRouteKeys struct {
	edPub   ed25519.PublicKey
	edPriv  ed25519.PrivateKey
	pqcPub  []byte
	pqcPriv []byte
}

func newAttestationRouteKeys(t *testing.T) attestationRouteKeys {
	t.Helper()
	edPub, edPriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)
	return attestationRouteKeys{edPub: edPub, edPriv: edPriv, pqcPub: kp.PublicKey, pqcPriv: kp.PrivateKey}
}

// agent is a verified agent registered with both keys, in hybrid mode when hybrid is set.
func (k attestationRouteKeys) agent(hybrid bool) *domain.Agent {
	edKey := base64.StdEncoding.EncodeToString(k.edPub)
	pqcKey := base64.StdEncoding.EncodeToString(k.pqcPub)
	return &domain.Agent{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Status:            domain.AgentStatusVerified,
		PublicKey:         &edKey,
		PQCPublicKey:      &pqcKey,
		HybridModeEnabled: hybrid,
	}
}

// send signs method, path and body for agentID with alg over the message
// PQCAgentMiddleware verifies (method, path, timestamp and body, one per line), sends the
// request through app, and returns the status code and the decoded JSON body.
func (k attestationRouteKeys) send(t *testing.T, app *fiber.App, agentID uuid.UUID, alg pqc.Algorithm, method, path, body string) (int, map[string]any) {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	parts := []string{method, path, ts}
	if body != "" {
		parts = append(parts, body)
	}
	message := []byte(strings.Join(parts, "\n"))

	edSig := base64.StdEncoding.EncodeToString(ed25519.Sign(k.edPriv, message))
	edKey := base64.StdEncoding.EncodeToString(k.edPub)
	pqcKey := base64.StdEncoding.EncodeToString(k.pqcPub)
	var pqcSig string
	if pqc.IsPQCAlgorithm(alg) || pqc.IsHybridAlgorithm(alg) {
		sig, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, k.pqcPriv, message)
		require.NoError(t, err)
		pqcSig = base64.StdEncoding.EncodeToString(sig)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Agent-ID", agentID.String())
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Algorithm", string(alg))
	switch {
	case pqc.IsHybridAlgorithm(alg):
		req.Header.Set("X-Signature-Ed25519", edSig)
		req.Header.Set("X-Signature-MLDSA", pqcSig)
		req.Header.Set("X-Public-Key", edKey)
		req.Header.Set("X-PQC-Public-Key", pqcKey)
	case pqc.IsPQCAlgorithm(alg):
		req.Header.Set("X-Signature", pqcSig)
		req.Header.Set("X-PQC-Public-Key", pqcKey)
	default:
		req.Header.Set("X-Signature", edSig)
		req.Header.Set("X-Public-Key", edKey)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var answer map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&answer))
	return resp.StatusCode, answer
}
