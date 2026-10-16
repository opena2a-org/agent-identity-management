package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// metadataRequestRepo keeps the capability requests the service creates.
type metadataRequestRepo struct {
	domain.CapabilityRequestRepository
	created []*domain.CapabilityRequest
}

func (r *metadataRequestRepo) List(domain.CapabilityRequestFilter) ([]*domain.CapabilityRequestWithDetails, error) {
	return nil, nil
}

func (r *metadataRequestRepo) Create(req *domain.CapabilityRequest) error {
	r.created = append(r.created, req)
	return nil
}

// metadataCapabilityRepo answers that the agent holds no capability yet.
type metadataCapabilityRepo struct {
	domain.CapabilityRepository
}

func (metadataCapabilityRepo) GetCapabilitiesByAgentID(uuid.UUID) ([]*domain.AgentCapability, error) {
	return nil, nil
}

// postCapabilityRequest posts body to POST /sdk-api/agents/:id/capability-requests
// for an agent of the caller's organization, through the real service.
func postCapabilityRequest(t *testing.T, body string) (int, []byte, *metadataRequestRepo) {
	t.Helper()
	orgID, agentID := uuid.New(), uuid.New()
	agentRepo := &capRequestTestAgentRepo{getByID: func(uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: agentID, OrganizationID: orgID, CreatedBy: uuid.New(), Name: "requesting-agent"}, nil
	}}
	requests := &metadataRequestRepo{}
	svc := application.NewCapabilityRequestService(requests, metadataCapabilityRepo{}, agentRepo, nil)
	h := NewCapabilityRequestHandlers(svc, agentRepo)

	app := fiber.New()
	app.Post("/sdk-api/agents/:id/capability-requests", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("agent_id", agentID)
		return h.CreateCapabilityRequest(c)
	})
	req := httptest.NewRequest("POST", "/sdk-api/agents/"+agentID.String()+"/capability-requests", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw, requests
}

// The metadata object an agent sends with a capability request is stored with
// the request as sent and returned with it.
func TestCreateCapabilityRequest_StoresTheMetadataItIsSent(t *testing.T) {
	status, raw, requests := postCapabilityRequest(t, `{
		"capabilityType": "file_write",
		"reason": "writes the nightly report to the shared drive",
		"metadata": {"job": "nightly-report", "paths": ["/reports"], "nested": {"retries": 3}}
	}`)
	require.Equal(t, fiber.StatusCreated, status, string(raw))

	want := domain.JSONBMap{
		"job":    "nightly-report",
		"paths":  []interface{}{"/reports"},
		"nested": map[string]interface{}{"retries": float64(3)},
	}
	require.Len(t, requests.created, 1)
	assert.Equal(t, want, requests.created[0].Metadata, "the stored request carries the metadata")

	var got domain.CapabilityRequest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, want, got.Metadata, "the answer returns it")
}

func TestCreateCapabilityRequest_WithoutMetadataStoresNone(t *testing.T) {
	status, raw, requests := postCapabilityRequest(t, `{"capabilityType": "file_write", "reason": "writes the nightly report"}`)
	require.Equal(t, fiber.StatusCreated, status, string(raw))
	require.Len(t, requests.created, 1)
	assert.Nil(t, requests.created[0].Metadata)
}

func TestCreateCapabilityRequest_MetadataThatIsNotAnObjectIs400(t *testing.T) {
	for _, metadata := range []string{`"free text"`, `["a", "b"]`, `42`} {
		status, raw, requests := postCapabilityRequest(t,
			`{"capabilityType": "file_write", "reason": "writes the nightly report", "metadata": `+metadata+`}`)
		assert.Equal(t, fiber.StatusBadRequest, status, "metadata %s", metadata)
		assert.JSONEq(t, `{"error":"invalid request body"}`, string(raw))
		assert.Empty(t, requests.created, "metadata %s: nothing is stored", metadata)
	}
}
