package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewSupplyChainHandler Tests
// ===========================

func TestNewSupplyChainHandler_NilDeps(t *testing.T) {
	handler := NewSupplyChainHandler(nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// SupplyChainHandler.GetSupplyChainAnalytics Tests
// ===========================

func TestSupplyChainHandler_GetSupplyChainAnalytics_NoOrgContext(t *testing.T) {
	handler := &SupplyChainHandler{}
	app := fiber.New()
	app.Get("/supply-chain/analytics", handler.GetSupplyChainAnalytics)

	req := httptest.NewRequest("GET", "/supply-chain/analytics", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// SupplyChainHandler.GetCapabilityDriftAlerts Tests
// ===========================

func TestSupplyChainHandler_GetCapabilityDriftAlerts_NoOrgContext(t *testing.T) {
	handler := &SupplyChainHandler{}
	app := fiber.New()
	app.Get("/supply-chain/drift-alerts", handler.GetCapabilityDriftAlerts)

	req := httptest.NewRequest("GET", "/supply-chain/drift-alerts", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Response Struct Tests
// ===========================

func TestConnectionWithDetails_Struct(t *testing.T) {
	lastAttested := "2024-01-15T10:00:00Z"
	conn := ConnectionWithDetails{
		ID:               "test-id",
		AgentID:          "agent-id",
		AgentName:        "Test Agent",
		MCPServerID:      "mcp-id",
		MCPServerName:    "Test MCP",
		ConnectionType:   "direct",
		AttestationCount: 5,
		FirstConnectedAt: "2024-01-01T10:00:00Z",
		LastAttestedAt:   &lastAttested,
		IsActive:         true,
	}

	assert.Equal(t, "test-id", conn.ID)
	assert.Equal(t, "agent-id", conn.AgentID)
	assert.Equal(t, "Test Agent", conn.AgentName)
	assert.Equal(t, "mcp-id", conn.MCPServerID)
	assert.Equal(t, "Test MCP", conn.MCPServerName)
	assert.Equal(t, "direct", conn.ConnectionType)
	assert.Equal(t, 5, conn.AttestationCount)
	assert.Equal(t, "2024-01-01T10:00:00Z", conn.FirstConnectedAt)
	assert.Equal(t, &lastAttested, conn.LastAttestedAt)
	assert.True(t, conn.IsActive)
}

func TestConnectionWithDetails_NilLastAttested(t *testing.T) {
	conn := ConnectionWithDetails{
		ID:             "test-id",
		LastAttestedAt: nil,
	}

	assert.Nil(t, conn.LastAttestedAt)
}

func TestSupplyChainAnalyticsResponse_Struct(t *testing.T) {
	resp := SupplyChainAnalyticsResponse{
		Stats:            nil,
		AttestationTrend: nil,
		Connections:      []ConnectionWithDetails{},
	}

	assert.Nil(t, resp.Stats)
	assert.Nil(t, resp.AttestationTrend)
	assert.Empty(t, resp.Connections)
}
