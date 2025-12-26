package handlers

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewMCPDiscoveryHandler Tests
// ===========================

func TestNewMCPDiscoveryHandler_NilDeps(t *testing.T) {
	handler := NewMCPDiscoveryHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// MCPDiscoveryHandler.GetDiscoveredMCPs Tests
// ===========================

func TestMCPDiscoveryHandler_GetDiscoveredMCPs_NoOrgContext(t *testing.T) {
	handler := &MCPDiscoveryHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/discovered", handler.GetDiscoveredMCPs)

	req := httptest.NewRequest("GET", "/mcp-servers/discovered", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// MCPDiscoveryHandler.IgnoreDiscoveredMCP Tests
// ===========================

func TestMCPDiscoveryHandler_IgnoreDiscoveredMCP_Success(t *testing.T) {
	// This endpoint just returns a placeholder response
	handler := &MCPDiscoveryHandler{}
	app := fiber.New()
	app.Post("/mcp-servers/discovered/ignore", handler.IgnoreDiscoveredMCP)

	req := httptest.NewRequest("POST", "/mcp-servers/discovered/ignore", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// Response Struct Tests
// ===========================

func TestDiscoveredMCP_Struct(t *testing.T) {
	now := time.Now()
	matchingServer := "server-123"
	discovered := DiscoveredMCP{
		Name:            "test-mcp",
		URL:             "http://localhost:8080",
		DetectedBy:      []string{"agent1", "agent2"},
		DetectedByCount: 2,
		DetectionMethod: "talks_to",
		FirstDetectedAt: now.Add(-24 * time.Hour),
		LastDetectedAt:  now,
		IsRegistered:    true,
		MatchingServer:  &matchingServer,
	}

	assert.Equal(t, "test-mcp", discovered.Name)
	assert.Equal(t, "http://localhost:8080", discovered.URL)
	assert.Len(t, discovered.DetectedBy, 2)
	assert.Equal(t, 2, discovered.DetectedByCount)
	assert.Equal(t, "talks_to", discovered.DetectionMethod)
	assert.True(t, discovered.IsRegistered)
	assert.NotNil(t, discovered.MatchingServer)
}

func TestDiscoveredMCP_NilMatchingServer(t *testing.T) {
	discovered := DiscoveredMCP{
		Name:           "unregistered-mcp",
		IsRegistered:   false,
		MatchingServer: nil,
	}

	assert.False(t, discovered.IsRegistered)
	assert.Nil(t, discovered.MatchingServer)
}

func TestMCPDiscoveryResponse_Struct(t *testing.T) {
	resp := MCPDiscoveryResponse{
		Discovered:        []DiscoveredMCP{},
		TotalUnmapped:     5,
		TotalAgents:       10,
		RegisteredServers: 3,
	}

	assert.Empty(t, resp.Discovered)
	assert.Equal(t, 5, resp.TotalUnmapped)
	assert.Equal(t, 10, resp.TotalAgents)
	assert.Equal(t, 3, resp.RegisteredServers)
}

func TestMCPDiscoveryResponse_WithDiscoveredMCPs(t *testing.T) {
	discovered := []DiscoveredMCP{
		{Name: "mcp1", DetectedByCount: 3},
		{Name: "mcp2", DetectedByCount: 1},
	}

	resp := MCPDiscoveryResponse{
		Discovered:        discovered,
		TotalUnmapped:     2,
		TotalAgents:       5,
		RegisteredServers: 8,
	}

	assert.Len(t, resp.Discovered, 2)
	assert.Equal(t, "mcp1", resp.Discovered[0].Name)
	assert.Equal(t, 3, resp.Discovered[0].DetectedByCount)
}
