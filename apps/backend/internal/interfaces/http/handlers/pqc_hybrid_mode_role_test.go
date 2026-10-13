package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// Hybrid mode marks an agent as requiring both an Ed25519 and an ML-DSA signature.
// Turning it on takes the manager or admin role on both routes that can do it:
// POST /agents/:id/pqc-key with enableHybrid true and POST /agents/:id/hybrid-mode
// with enable true. A member still registers a key without it and still turns it off.
//
// Every cell goes through MemberMiddleware, the gate both routes are mounted behind,
// so the answer a role gets is the one the route gives it.

// hybridRoleCall is what one request did: the status and body it got, and what it
// stored. stored is nil when nothing was written.
type hybridRoleCall struct {
	status int
	body   map[string]interface{}
	stored *bool
}

// postAsRole sends body to handler mounted behind MemberMiddleware, as a principal
// of orgID holding role. An empty role sets none, as an agent signature does.
func postAsRole(t *testing.T, handler fiber.Handler, orgID uuid.UUID, role string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	app := fiber.New()
	app.Post("/agents/:id/act", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		if role != "" {
			c.Locals("role", role)
		}
		return c.Next()
	}, middleware.MemberMiddleware(), handler)

	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/act", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(answer, &out), "response body: %s", answer)
	return resp.StatusCode, out
}

// registerWithHybrid posts a well-formed ML-DSA-65 key with the given enableHybrid.
func registerWithHybrid(t *testing.T, role string, enableHybrid bool) hybridRoleCall {
	t.Helper()
	kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
	require.NoError(t, err)

	orgID := uuid.New()
	call := hybridRoleCall{}
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "pqc-agent"}, nil
			},
			UpdateAgentPQCKeyFunc: func(_ context.Context, _ uuid.UUID, _, _ string, hybrid bool) error {
				call.stored = &hybrid
				return nil
			},
		},
		auditService: &MockAuditServiceImpl{},
	}
	call.status, call.body = postAsRole(t, h.RegisterPQCKey, orgID, role, map[string]interface{}{
		"publicKey":    base64.StdEncoding.EncodeToString(kp.PublicKey),
		"algorithm":    "ML-DSA-65",
		"enableHybrid": enableHybrid,
	})
	return call
}

// setHybrid posts enable for an agent that holds both keys hybrid mode needs.
func setHybrid(t *testing.T, role string, enable bool) hybridRoleCall {
	t.Helper()
	orgID := uuid.New()
	edKey, pqcKey := "ed25519-key", "pqc-key"
	call := hybridRoleCall{}
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{
					ID: id, OrganizationID: orgID, Name: "pqc-agent",
					PublicKey: &edKey, PQCPublicKey: &pqcKey, HybridModeEnabled: !enable,
				}, nil
			},
			SetAgentHybridModeFunc: func(_ context.Context, _ uuid.UUID, enabled bool) error {
				call.stored = &enabled
				return nil
			},
		},
		auditService: &MockAuditServiceImpl{},
	}
	call.status, call.body = postAsRole(t, h.SetHybridMode, orgID, role, map[string]interface{}{"enable": enable})
	return call
}

var hybridModeActs = []struct {
	name string
	send func(t *testing.T, role string, on bool) hybridRoleCall
}{
	{"POST pqc-key enableHybrid", registerWithHybrid},
	{"POST hybrid-mode enable", setHybrid},
}

func TestTurningOnHybridModeTakesTheManagerOrAdminRole(t *testing.T) {
	for _, act := range hybridModeActs {
		for _, role := range []domain.UserRole{domain.RoleManager, domain.RoleAdmin} {
			t.Run(act.name+" true as "+string(role), func(t *testing.T) {
				call := act.send(t, string(role), true)
				require.Equal(t, fiber.StatusOK, call.status, "response: %v", call.body)
				require.NotNil(t, call.stored, "the request must be stored")
				assert.True(t, *call.stored, "hybrid mode must be stored on")
			})
		}

		t.Run(act.name+" true as member", func(t *testing.T) {
			call := act.send(t, string(domain.RoleMember), true)
			require.Equal(t, fiber.StatusForbidden, call.status, "response: %v", call.body)
			assert.Equal(t, "Manager or admin access required to turn on hybrid mode", call.body["error"])
			assert.Nil(t, call.stored, "a refused request must store nothing")
		})
	}
}

// The member cell of the register route names the request that works for a member.
func TestRegisterPQCKeyTellsAMemberToRegisterWithoutHybridMode(t *testing.T) {
	call := registerWithHybrid(t, string(domain.RoleMember), true)
	require.Equal(t, fiber.StatusForbidden, call.status, "response: %v", call.body)
	assert.Equal(t, "Send enableHybrid false to register the key without hybrid mode", call.body["hint"])
}

func TestAMemberStillRegistersAKeyWithoutHybridModeAndTurnsHybridModeOff(t *testing.T) {
	for _, act := range hybridModeActs {
		t.Run(act.name+" false as member", func(t *testing.T) {
			call := act.send(t, string(domain.RoleMember), false)
			require.Equal(t, fiber.StatusOK, call.status, "response: %v", call.body)
			require.NotNil(t, call.stored, "the request must be stored")
			assert.False(t, *call.stored, "hybrid mode must be stored off")
		})
	}
}

// A viewer, and a principal with no role, are refused by the route's gate before the
// handler reads the body: their answer does not depend on enable.
func TestAViewerOrARolelessPrincipalIsRefusedByTheRouteGateEitherWay(t *testing.T) {
	for _, act := range hybridModeActs {
		for _, on := range []bool{true, false} {
			sent := act.name + " " + strconv.FormatBool(on)
			t.Run(sent+" as viewer", func(t *testing.T) {
				call := act.send(t, string(domain.RoleViewer), on)
				require.Equal(t, fiber.StatusForbidden, call.status, "response: %v", call.body)
				assert.Equal(t, "Member access required (viewers cannot perform this action)", call.body["error"])
				assert.Nil(t, call.stored, "a refused request must store nothing")
			})
			t.Run(sent+" with no role", func(t *testing.T) {
				call := act.send(t, "", on)
				require.Equal(t, fiber.StatusUnauthorized, call.status, "response: %v", call.body)
				assert.Nil(t, call.stored, "a refused request must store nothing")
			})
		}
	}
}
