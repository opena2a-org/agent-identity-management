package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression coverage for defects #3 + #26 + #27: API array fields must
// serialize as `[]` (empty array), not `null`, when a list endpoint has zero
// rows. The React dashboard iterates these arrays with
// `.forEach`/`.map`/`.filter` and crashes the panel on `null`. The fix lives
// in repository row-scan loops (`xs := make([]T, 0)` instead of
// `var xs []T`) and is enforced going forward by `cmd/jsonarray-lint`.
//
// These tests exercise the END-TO-END contract for representative endpoints:
// service returns an empty (non-nil) slice, handler builds the response,
// JSON body has `"<field>": []` not `"<field>": null`.

// rawArrayField inspects a JSON response body and returns the raw bytes for a
// top-level field. Used to distinguish `[]` (empty array) from `null` (the
// bug). `json.Decode` into `map[string]interface{}` would coerce both to
// untyped nil, hiding the regression.
func rawArrayField(t *testing.T, body []byte, field string) string {
	t.Helper()
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	v, ok := raw[field]
	require.True(t, ok, "response missing field %q (body=%s)", field, string(body))
	return string(v)
}

func TestTrustScoreHistory_EmptyReturnsArrayNotNull(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, _ uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	trustMock := &MockTrustCalculatorServicerImpl{
		GetTrustScoreHistoryAuditTrailFunc: func(_ context.Context, _ uuid.UUID, _ int) ([]*domain.TrustScoreHistoryEntry, error) {
			return make([]*domain.TrustScoreHistoryEntry, 0), nil
		},
	}

	handler := NewTrustScoreHandlerWithInterfaces(trustMock, agentMock, &MockAuditServiceImpl{})
	app := createTrustScoreTestApp(handler.GetTrustScoreHistory, orgID, userID)
	app.Get("/agents/:id/trust-score/history", handler.GetTrustScoreHistory)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/trust-score/history", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body := make([]byte, resp.ContentLength)
	_, _ = resp.Body.Read(body)

	got := rawArrayField(t, body, "history")
	assert.Equal(t, "[]", got, "history field must serialize as `[]` not `null` when empty")
}

func TestAgentMCPServers_EmptyReturnsArrayNotNull(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()

	agentMock := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, _ uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             agentID,
				OrganizationID: orgID,
				Name:           "test-agent",
			}, nil
		},
	}

	attestationMock := &MockMCPAttestationServiceImpl{
		GetMCPServersForAgentFunc: func(_ context.Context, _ uuid.UUID) ([]*domain.MCPServer, error) {
			return make([]*domain.MCPServer, 0), nil
		},
	}

	handler := NewAgentHandlerWithInterfaces(
		agentMock,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		attestationMock,
	)
	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/mcp-servers", handler.GetAgentMCPServers)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/mcp-servers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body := make([]byte, resp.ContentLength)
	_, _ = resp.Body.Read(body)

	got := rawArrayField(t, body, "mcpServers")
	assert.Equal(t, "[]", got, "mcpServers field must serialize as `[]` not `null` when empty")
}

// Marshal-layer contract: protects the test suite from a hypothetical future
// stdlib change that would serialize `make([]T, 0)` as anything other than
// `[]`. If this ever fails, the entire defect class above must be re-audited.
func TestEmptySliceMarshalContract(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"make pointer slice", struct {
			Items []*domain.Agent `json:"items"`
		}{Items: make([]*domain.Agent, 0)}, `{"items":[]}`},
		{"make value slice", struct {
			Items []string `json:"items"`
		}{Items: make([]string, 0)}, `{"items":[]}`},
		{"composite literal empty slice", struct {
			Items []int `json:"items"`
		}{Items: []int{}}, `{"items":[]}`},
		{"nil slice (the bug)", struct {
			Items []string `json:"items"`
		}{Items: nil}, `{"items":null}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(b))
		})
	}
}
