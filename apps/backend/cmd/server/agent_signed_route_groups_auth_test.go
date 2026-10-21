package main

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// The /sdk-api, /detection, /agents and /a2a groups run PQCAgentMiddleware and then
// AuthMiddleware. A request PQCAgentMiddleware authenticated passes AuthMiddleware
// whichever algorithm the agent signed it with: Ed25519, ML-DSA-65, or both, the way an
// agent in hybrid mode must sign. Each route here is answered after AuthMiddleware by a
// check that reads no service: the agent-path binding, for a path naming another agent,
// or the handler's own agent-ID check.
func TestAgentSignedRouteGroupsAdmitEverySignatureAlgorithm(t *testing.T) {
	keys := newAttestationRouteKeys(t)
	agent, hybridAgent := keys.agent(false), keys.agent(true)
	app := agentSignedRoutesApp(t, agent, hybridAgent)
	otherAgent := uuid.NewString()

	pathMismatch := map[string]any{
		"error":      "An agent may only act on its own agent ID on this route",
		"reasonCode": middleware.AgentPathMismatchReasonCode,
	}
	routes := []struct {
		group, method, path, body string
		status                    int
		answer                    map[string]any
	}{
		{"/sdk-api", fiber.MethodPost, "/api/v1/sdk-api/agents/" + otherAgent + "/capabilities", "{}",
			fiber.StatusForbidden, pathMismatch},
		{"/detection", fiber.MethodPost, "/api/v1/detection/agents/" + otherAgent + "/report", "{}",
			fiber.StatusForbidden, pathMismatch},
		{"/agents", fiber.MethodGet, "/api/v1/agents/not-a-uuid", "",
			fiber.StatusBadRequest, map[string]any{"error": "Invalid agent ID"}},
		{"/a2a", fiber.MethodPost, "/api/v1/a2a/agents/" + otherAgent + "/sign", "{}",
			fiber.StatusForbidden, pathMismatch},
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
			t.Run(route.group+"/"+probe.name, func(t *testing.T) {
				status, body := keys.send(t, app, probe.agent.ID, probe.alg, route.method, route.path, route.body)

				assert.Equal(t, route.status, status)
				assert.Equal(t, route.answer, body, "the answer of the check after AuthMiddleware")
			})
		}

		// PQCAgentMiddleware still refuses an agent in hybrid mode that signs with one key.
		t.Run(route.group+"/Ed25519 alone from an agent in hybrid mode", func(t *testing.T) {
			status, body := keys.send(t, app, hybridAgent.ID, pqc.AlgorithmEd25519, route.method, route.path, route.body)

			assert.Equal(t, fiber.StatusUnauthorized, status)
			refusal, _ := body["error"].(string)
			assert.True(t, strings.HasPrefix(refusal, "Agent is in hybrid mode"), "PQCAgentMiddleware's refusal, got %q", refusal)
		})
	}
}
