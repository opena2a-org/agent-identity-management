package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewMCPGraphHandler Tests
// ===========================

func TestNewMCPGraphHandler_NilDeps(t *testing.T) {
	handler := NewMCPGraphHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// MCPGraphHandler.GetConnectionGraph Tests
// ===========================

func TestMCPGraphHandler_GetConnectionGraph_NoOrgContext(t *testing.T) {
	handler := &MCPGraphHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/graph", handler.GetConnectionGraph)

	req := httptest.NewRequest("GET", "/mcp-servers/graph", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// MCPGraphHandler.GetMCPServerConnections Tests
// ===========================

func TestMCPGraphHandler_GetMCPServerConnections_InvalidID(t *testing.T) {
	handler := &MCPGraphHandler{}
	app := fiber.New()
	app.Get("/mcp-servers/:id/connections", handler.GetMCPServerConnections)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/connections", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// Helper function tests
// ===========================

func TestMCPGraph_getNodeSize(t *testing.T) {
	tests := []struct {
		name            string
		connectionCount int
		expectedSize    int
	}{
		{"No connections", 0, 15},
		{"1 connection", 1, 20},
		{"5 connections", 5, 20},  // > 0 but not > 5
		{"6 connections", 6, 25},  // > 5 but not > 10
		{"10 connections", 10, 25},
		{"11 connections", 11, 30}, // > 10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeSize(tt.connectionCount)
			assert.Equal(t, tt.expectedSize, result)
		})
	}
}

func TestMCPGraph_getColorFromScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"High score", 85, "#22c55e"},
		{"Medium-high score", 65, "#84cc16"},
		{"Medium score", 45, "#eab308"},
		{"Low-medium score", 25, "#f97316"},
		{"Low score", 10, "#ef4444"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getColorFromScore(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMCPGraph_matchesMCPServer(t *testing.T) {
	serverID := uuid.New()
	mcpServer := &domain.MCPServer{
		ID:   serverID,
		Name: "test-mcp",
		URL:  "http://localhost:8080",
	}

	tests := []struct {
		name     string
		talksTo  string
		expected bool
	}{
		{"Match by name", "test-mcp", true},
		{"Match by URL", "http://localhost:8080", true},
		{"Match by ID", serverID.String(), true},
		{"No match", "other-mcp", false},
		{"Empty talksTo", "", false},
		{"Partial URL no match", "localhost", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMCPServer(mcpServer, tt.talksTo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMCPGraph_matchesMCPServer_EmptyURL(t *testing.T) {
	serverID := uuid.New()
	// MCP server with empty URL
	mcpServer := &domain.MCPServer{
		ID:   serverID,
		Name: "test-mcp",
		URL:  "", // Empty URL
	}

	tests := []struct {
		name     string
		talksTo  string
		expected bool
	}{
		{"Match by name with empty URL", "test-mcp", true},
		{"Match by ID with empty URL", serverID.String(), true},
		{"No match with empty URL", "http://some-url.com", false},
		// Empty talksTo matches empty URL (both are empty strings)
		{"Empty talksTo with empty URL", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMCPServer(mcpServer, tt.talksTo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMCPGraph_matchesMCPServer_NonEmptyURLWithNonMatchingTalksTo(t *testing.T) {
	serverID := uuid.New()
	mcpServer := &domain.MCPServer{
		ID:   serverID,
		Name: "production-mcp",
		URL:  "https://mcp.example.com:8443/api",
	}

	tests := []struct {
		name     string
		talksTo  string
		expected bool
	}{
		{"Exact URL match", "https://mcp.example.com:8443/api", true},
		{"Exact name match", "production-mcp", true},
		{"Exact ID match", serverID.String(), true},
		{"Partial URL doesn't match", "mcp.example.com", false},
		{"Different port doesn't match", "https://mcp.example.com:9000/api", false},
		{"HTTP instead of HTTPS doesn't match", "http://mcp.example.com:8443/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMCPServer(mcpServer, tt.talksTo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// Struct Tests
// ===========================

func TestGraphNode_Struct(t *testing.T) {
	node := GraphNode{
		ID:         "mcp-123",
		Type:       "mcp",
		Label:      "Test MCP",
		Size:       20,
		Color:      "#22c55e",
		Status:     "active",
		TrustScore: 85.5,
	}

	assert.Equal(t, "mcp-123", node.ID)
	assert.Equal(t, "mcp", node.Type)
	assert.Equal(t, "Test MCP", node.Label)
	assert.Equal(t, 20, node.Size)
	assert.Equal(t, "#22c55e", node.Color)
	assert.Equal(t, "active", node.Status)
	assert.Equal(t, 85.5, node.TrustScore)
}

func TestGraphEdge_Struct(t *testing.T) {
	edge := GraphEdge{
		ID:     "agent-1-mcp-1",
		Source: "agent-1",
		Target: "mcp-1",
		Weight: 2,
		Type:   "attestation",
	}

	assert.Equal(t, "agent-1-mcp-1", edge.ID)
	assert.Equal(t, "agent-1", edge.Source)
	assert.Equal(t, "mcp-1", edge.Target)
	assert.Equal(t, 2, edge.Weight)
	assert.Equal(t, "attestation", edge.Type)
}

func TestMCPGraphResponse_Struct(t *testing.T) {
	resp := MCPGraphResponse{
		Nodes: []GraphNode{
			{ID: "mcp-1", Type: "mcp", Label: "MCP 1"},
			{ID: "agent-1", Type: "agent", Label: "Agent 1"},
		},
		Edges: []GraphEdge{
			{ID: "agent-1-mcp-1", Source: "agent-1", Target: "mcp-1"},
		},
	}

	assert.Len(t, resp.Nodes, 2)
	assert.Len(t, resp.Edges, 1)
	assert.Equal(t, "mcp-1", resp.Nodes[0].ID)
	assert.Equal(t, "agent-1", resp.Edges[0].Source)
}

func TestMCPGraphResponse_Empty(t *testing.T) {
	resp := MCPGraphResponse{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	assert.Empty(t, resp.Nodes)
	assert.Empty(t, resp.Edges)
}

func TestMCPGraph_getColorFromScore_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"Exactly 80", 80, "#22c55e"},
		{"Exactly 60", 60, "#84cc16"},
		{"Exactly 40", 40, "#eab308"},
		{"Exactly 20", 20, "#f97316"},
		{"Zero", 0, "#ef4444"},
		{"Negative", -10, "#ef4444"},
		{"100", 100, "#22c55e"},
		{"79.99", 79.99, "#84cc16"},
		{"59.99", 59.99, "#eab308"},
		{"39.99", 39.99, "#f97316"},
		{"19.99", 19.99, "#ef4444"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getColorFromScore(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}
