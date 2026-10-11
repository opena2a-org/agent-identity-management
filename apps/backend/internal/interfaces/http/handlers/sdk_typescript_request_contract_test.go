package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The TypeScript SDK's requests live in sdk/typescript/tests/fixtures/sdk_requests.
// AIMClient.sdk-routes.test.ts requires the client to send each fixture's method,
// path and body; the tests below post each body to the handler its route mounts. A
// member renamed, or a signed message written differently, on either side fails one
// of the two. verifyAction posted to /api/v1/verify, which the server does not
// register (#662).

func readTSRequestFixture(t *testing.T, name string) pqcRequestFixture {
	t.Helper()
	path := backendFile(t, filepath.Join("..", "..", "sdk", "typescript", "tests", "fixtures", "sdk_requests", name+".json"))
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err, "read the %s fixture", name)
	var fx pqcRequestFixture
	require.NoError(t, json.Unmarshal(raw, &fx))
	return fx
}

// decodeTSBody decodes the fixture body into target and fails on any member target
// does not read.
func decodeTSBody(t *testing.T, fx pqcRequestFixture, target interface{}) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(fx.Body))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(target), "the TypeScript SDK sends a member %s does not read", fx.RequestType)
}

func sendTSRequest(t *testing.T, app *fiber.App, method, path string, body []byte) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out), "response body: %s", raw)
	return resp.StatusCode, out
}

// tsAgentLookup answers GetByID for one agent.
type tsAgentLookup struct{ agent *domain.Agent }

func (l tsAgentLookup) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return agentsByID(l.agent)(id)
}

func TestTypeScriptVerifyActionRequestIsAcceptedByCreateVerification(t *testing.T) {
	for _, name := range []string{"verify_action", "verify_action_minimal"} {
		t.Run(name, func(t *testing.T) {
			fx := readTSRequestFixture(t, name)
			require.Equal(t, fiber.MethodPost, fx.Method)
			var want VerificationRequest
			decodeTSBody(t, fx, &want)
			if name == "verify_action" {
				// The full request sends every member the route reads.
				requireBodyIsRequestType(t, fx, &VerificationRequest{})
			}

			agentID, err := uuid.Parse(want.AgentID)
			require.NoError(t, err)
			registered := want.PublicKey
			agent := &domain.Agent{
				ID: agentID, OrganizationID: uuid.New(), Status: domain.AgentStatusVerified,
				PublicKey: &registered, TrustScore: 0.8, Name: "ts-agent",
			}

			var reached bool
			var gotCapability, gotResource string
			var gotContext map[string]interface{}
			svc := &MockAgentServiceForVerificationImpl{}
			svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return agentsByID(agent)(id) }
			svc.VerifyCapabilityFunc = func(_ context.Context, _ uuid.UUID, capability, resource string, ctx map[string]interface{}, _ string) (bool, string, uuid.UUID, error) {
				reached = true
				gotCapability, gotResource, gotContext = capability, resource, ctx
				return true, "", uuid.New(), nil
			}
			orgs := &MockOrganizationRepositoryerImpl{
				GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
					return &domain.Organization{ID: id, EnforcementMode: domain.EnforcementModeStrict}, nil
				},
			}
			h := NewVerificationHandlerWithInterfaces(svc, &MockAuditServiceForVerificationImpl{},
				&MockAlertServiceForVerificationImpl{}, &MockVerificationEventServiceForVerificationImpl{}, orgs)
			app := fiber.New()
			app.Post(fx.Path, h.CreateVerification)

			status, out := sendTSRequest(t, app, fx.Method, fx.Path, fx.Body)
			require.Equal(t, fiber.StatusCreated, status, "the signature the TypeScript SDK made must verify: %v", out)
			require.True(t, reached, "an authenticated request reaches the capability check")
			assert.Equal(t, "approved", out["status"])
			assert.Equal(t, want.Capability, gotCapability)
			assert.Equal(t, want.Resource, gotResource)
			if len(want.Context) == 0 {
				assert.Empty(t, gotContext)
			} else {
				assert.Equal(t, want.Context, gotContext)
			}

			// The same request with one signed member changed is refused, so the
			// 201 above is the signature verifying and not the check being skipped.
			var tampered map[string]interface{}
			require.NoError(t, json.Unmarshal(fx.Body, &tampered))
			tampered["capability"] = want.Capability + ":changed"
			body, err := json.Marshal(tampered)
			require.NoError(t, err)
			reached = false
			status, out = sendTSRequest(t, app, fx.Method, fx.Path, body)
			require.Equal(t, fiber.StatusUnauthorized, status, "response: %v", out)
			assert.False(t, reached)
		})
	}
}

func TestTypeScriptReportCapabilitiesRequestIsReadByRegisterCapability(t *testing.T) {
	fx := readTSRequestFixture(t, "register_capability")
	require.Equal(t, fiber.MethodPost, fx.Method)
	var want RegisterCapabilityRequest
	decodeTSBody(t, fx, &want)
	require.NotEmpty(t, want.CapabilityType)

	orgID, agentID := uuid.New(), uuid.New()
	var granted []string
	h := NewCapabilityHandlerWithInterfaces(&MockCapabilityServiceImpl{
		GrantCapabilityFunc: func(_ context.Context, id uuid.UUID, capabilityType string, _ map[string]interface{}, _ *uuid.UUID, mode string) (*domain.AgentCapability, error) {
			granted = append(granted, capabilityType)
			return &domain.AgentCapability{ID: uuid.New(), AgentID: id, CapabilityType: capabilityType, ExecutionMode: mode}, nil
		},
	})
	h.agentRepo = tsAgentLookup{agent: &domain.Agent{ID: agentID, OrganizationID: orgID}}
	h.orgRepo = &MockOrganizationRepository{} // monitoring mode: the capability is granted

	app := fiber.New()
	app.Post(strings.Replace(fx.Path, "{id}", ":id", 1), func(c fiber.Ctx) error {
		// As PQCAgentMiddleware leaves it for a signed agent request.
		c.Locals("organization_id", orgID)
		c.Locals("agent_id", agentID)
		return h.RegisterCapability(c)
	})

	status, out := sendTSRequest(t, app, fx.Method, strings.Replace(fx.Path, "{id}", agentID.String(), 1), fx.Body)
	require.Equal(t, fiber.StatusCreated, status, "response: %v", out)
	assert.Equal(t, "granted", out["status"])
	assert.Equal(t, []string{want.CapabilityType}, granted)
}

func TestTypeScriptUpdateAgentRequestIsAppliedByUpdateAgent(t *testing.T) {
	fx := readTSRequestFixture(t, "update_agent")
	require.Equal(t, fiber.MethodPut, fx.Method)
	var want application.CreateAgentRequest
	decodeTSBody(t, fx, &want)

	// Every member the SDK sends is one AgentService.UpdateAgent applies. name is
	// bound by the request type and never applied, so the SDK does not send it.
	applied := map[string]bool{
		"displayName": true, "description": true, "agentType": true, "version": true,
		"capabilities": true, "talksTo": true, "metadata": true,
	}
	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fx.Body, &members))
	for name := range members {
		assert.True(t, applied[name], "the TypeScript SDK sends %q, which UpdateAgent does not apply", name)
	}

	orgID, agentID := uuid.New(), uuid.New()
	var got *application.CreateAgentRequest
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "ts-agent"}, nil
			},
			UpdateAgentFunc: func(_ context.Context, id uuid.UUID, req *application.CreateAgentRequest, _ uuid.UUID) (*domain.Agent, error) {
				got = req
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "ts-agent", DisplayName: req.DisplayName}, nil
			},
		},
		auditService:      &MockAuditServiceImpl{},
		capabilityService: &MockCapabilityServiceImpl{},
	}

	app := fiber.New()
	app.Put(strings.Replace(fx.Path, "{id}", ":id", 1), func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleMember))
		return h.UpdateAgent(c)
	})

	status, out := sendTSRequest(t, app, fx.Method, strings.Replace(fx.Path, "{id}", agentID.String(), 1), fx.Body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	require.NotNil(t, got, "the update must reach the service")
	assert.Equal(t, want, *got)
	assert.Empty(t, got.Name)
}
