package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Python SDK's PQC writes sent pqcPublicKey/enableHybridMode, newPqcPublicKey
// and enabled, none of which the request types below read: the key arrived empty
// and set_hybrid_mode(True) turned hybrid mode off. The request bodies now live in
// sdk/python/tests/fixtures/pqc_requests, which a Python test also reads, so a
// member renamed on either side fails one of the two.

type pqcRequestFixture struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	RequestType string          `json:"requestType"`
	Body        json.RawMessage `json:"body"`
}

func readPQCRequestFixture(t *testing.T, name string) pqcRequestFixture {
	t.Helper()
	path := backendFile(t, filepath.Join("..", "..", "sdk", "python", "tests", "fixtures", "pqc_requests", name+".json"))
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err, "read the %s fixture", name)
	var fx pqcRequestFixture
	require.NoError(t, json.Unmarshal(raw, &fx))
	return fx
}

// requireBodyIsRequestType asserts the fixture body and the request type carry
// the same members: none the type does not read, and every one it does.
func requireBodyIsRequestType(t *testing.T, fx pqcRequestFixture, target interface{}) {
	t.Helper()
	typ := reflect.TypeOf(target).Elem()
	require.Equal(t, typ.Name(), fx.RequestType, "the fixture names the request type it pins")

	dec := json.NewDecoder(bytes.NewReader(fx.Body))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(target), "the fixture body sends a member %s does not read", typ.Name())

	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fx.Body, &members))
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		_, ok := members[name]
		assert.True(t, ok, "%s reads %q and the fixture body does not send it", typ.Name(), name)
	}
}

// postPQCFixture mounts handler on the fixture's own method and path, as an
// authenticated member of orgID, and sends body.
func postPQCFixture(t *testing.T, fx pqcRequestFixture, handler fiber.Handler, orgID, agentID uuid.UUID, body []byte) (int, map[string]interface{}) {
	t.Helper()
	app := fiber.New()
	app.Add([]string{fx.Method}, strings.Replace(fx.Path, "{id}", ":id", 1), func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleMember))
		return handler(c)
	})

	req := httptest.NewRequest(fx.Method, strings.Replace(fx.Path, "{id}", agentID.String(), 1), bytes.NewReader(body))
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

func TestRegisterPQCKeyReadsTheSDKRequestBody(t *testing.T) {
	fx := readPQCRequestFixture(t, "register_pqc_key")
	var want RegisterPQCKeyRequest
	requireBodyIsRequestType(t, fx, &want)

	orgID, agentID := uuid.New(), uuid.New()
	var gotKey, gotAlg string
	gotHybrid, called := true, false
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "pqc-agent"}, nil
			},
			UpdateAgentPQCKeyFunc: func(_ context.Context, id uuid.UUID, key, alg string, hybrid bool) error {
				called = true
				gotKey, gotAlg, gotHybrid = key, alg, hybrid
				return nil
			},
		},
		auditService: &MockAuditServiceImpl{},
	}

	status, out := postPQCFixture(t, fx, h.RegisterPQCKey, orgID, agentID, fx.Body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	require.True(t, called, "the key must be stored")
	assert.Equal(t, want.PublicKey, gotKey)
	assert.Equal(t, want.Algorithm, gotAlg)
	assert.False(t, gotHybrid)
}

func TestRotatePQCKeyReadsTheSDKRequestBody(t *testing.T) {
	fx := readPQCRequestFixture(t, "rotate_pqc_key")
	var want RotatePQCKeyRequest
	requireBodyIsRequestType(t, fx, &want)

	orgID, agentID := uuid.New(), uuid.New()
	existing := "existing-key"
	var gotKey, gotAlg string
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: id, OrganizationID: orgID, Name: "pqc-agent", PQCPublicKey: &existing}, nil
			},
			RotateAgentPQCKeyFunc: func(_ context.Context, id uuid.UUID, key, alg string) error {
				gotKey, gotAlg = key, alg
				return nil
			},
		},
		auditService: &MockAuditServiceImpl{},
	}

	status, out := postPQCFixture(t, fx, h.RotatePQCKey, orgID, agentID, fx.Body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	assert.Equal(t, want.NewPublicKey, gotKey, "the stored key must be the one sent")
	assert.Equal(t, want.Algorithm, gotAlg)
}

func TestSetHybridModeReadsTheSDKRequestBody(t *testing.T) {
	fx := readPQCRequestFixture(t, "set_hybrid_mode")
	var want EnableHybridModeRequest
	requireBodyIsRequestType(t, fx, &want)
	require.False(t, want.Enable, "the SDK only sends enable=false")

	orgID, agentID := uuid.New(), uuid.New()
	edKey, pqcKey := "ed25519-key", "pqc-key"
	var got []bool
	h := &AgentHandler{
		agentService: &MockAgentServiceImpl{
			GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{
					ID: id, OrganizationID: orgID, Name: "pqc-agent",
					PublicKey: &edKey, PQCPublicKey: &pqcKey, HybridModeEnabled: true,
				}, nil
			},
			SetAgentHybridModeFunc: func(_ context.Context, id uuid.UUID, enabled bool) error {
				got = append(got, enabled)
				return nil
			},
		},
		auditService: &MockAuditServiceImpl{},
	}

	status, out := postPQCFixture(t, fx, h.SetHybridMode, orgID, agentID, fx.Body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	assert.Equal(t, false, out["hybridModeEnabled"])

	// false is also what a member the server does not read decodes to, so send
	// the fixture's member set to true and check the handler sees it.
	var flipped map[string]interface{}
	require.NoError(t, json.Unmarshal(fx.Body, &flipped))
	for name := range flipped {
		flipped[name] = true
	}
	body, err := json.Marshal(flipped)
	require.NoError(t, err)
	status, out = postPQCFixture(t, fx, h.SetHybridMode, orgID, agentID, body)
	require.Equal(t, fiber.StatusOK, status, "response: %v", out)
	assert.Equal(t, true, out["hybridModeEnabled"])

	assert.Equal(t, []bool{false, true}, got)
}
