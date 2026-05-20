package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// MCPGraphHandler handles MCP-to-Agent connection graph endpoints
type MCPGraphHandler struct {
	mcpService              *application.MCPService
	agentRepository         *repository.AgentRepository
	mcpAttestationRepository domain.MCPAttestationRepository
}

// NewMCPGraphHandler creates a new MCP graph handler
func NewMCPGraphHandler(
	mcpService *application.MCPService,
	agentRepository *repository.AgentRepository,
	mcpAttestationRepository domain.MCPAttestationRepository,
) *MCPGraphHandler {
	return &MCPGraphHandler{
		mcpService:              mcpService,
		agentRepository:         agentRepository,
		mcpAttestationRepository: mcpAttestationRepository,
	}
}

// GraphNode represents a node in the MCP-Agent connection graph
type GraphNode struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`       // "mcp" or "agent"
	Label      string  `json:"label"`
	Size       int     `json:"size"`       // Based on connection count
	Color      string  `json:"color"`      // Based on trust/confidence score
	Status     string  `json:"status"`     // active, verified, etc.
	TrustScore float64 `json:"trustScore"` // Trust or confidence score
}

// GraphEdge represents an edge between nodes
type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Weight int    `json:"weight"` // Attestation count or connection strength
	Type   string `json:"type"`   // "attestation", "connection"
}

// MCPGraphResponse is the response structure for the graph endpoint
type MCPGraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetConnectionGraph returns the MCP-Agent connection graph
// @Summary Get MCP-Agent connection graph
// @Description Returns network graph data for visualization of MCP-Agent relationships
// @Tags mcp-graph
// @Produce json
// @Success 200 {object} MCPGraphResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/mcp-servers/graph [get]
func (h *MCPGraphHandler) GetConnectionGraph(c fiber.Ctx) error {
	// Get organization ID from context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	// Get all MCP servers for the organization
	mcpServers, err := h.mcpService.ListMCPServers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers: " + err.Error(),
		})
	}

	// Get all agents for the organization
	agents, err := h.agentRepository.GetByOrganization(orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents: " + err.Error(),
		})
	}

	// Build graph nodes and edges
	nodes := make([]GraphNode, 0)
	edges := make([]GraphEdge, 0)
	nodeIndex := make(map[string]bool)
	edgeIndex := make(map[string]bool)

	// Add MCP server nodes
	for _, mcp := range mcpServers {
		nodeID := "mcp-" + mcp.ID.String()
		if !nodeIndex[nodeID] {
			nodeIndex[nodeID] = true
			nodes = append(nodes, GraphNode{
				ID:         nodeID,
				Type:       "mcp",
				Label:      mcp.Name,
				Size:       getNodeSize(mcp.ConnectedAgentsCount),
				Color:      getColorFromScore(mcp.ConfidenceScore),
				Status:     string(mcp.Status),
				TrustScore: mcp.ConfidenceScore,
			})
		}
	}

	// Add agent nodes
	for _, agent := range agents {
		nodeID := "agent-" + agent.ID.String()
		if !nodeIndex[nodeID] {
			nodeIndex[nodeID] = true
			nodes = append(nodes, GraphNode{
				ID:         nodeID,
				Type:       "agent",
				Label:      agent.DisplayName,
				Size:       getNodeSize(len(agent.TalksTo)),
				Color:      getColorFromScore(agent.TrustScore * 100), // TrustScore is 0-1
				Status:     string(agent.Status),
				TrustScore: agent.TrustScore * 100,
			})
		}
	}

	// Build edges from agent talks_to relationships and attestations
	for _, agent := range agents {
		agentNodeID := "agent-" + agent.ID.String()

		// Add edges based on talks_to (MCP connections)
		for _, talksTo := range agent.TalksTo {
			// Try to find the MCP server by name or URL
			for _, mcp := range mcpServers {
				if matchesMCPServer(mcp, talksTo) {
					mcpNodeID := "mcp-" + mcp.ID.String()
					edgeID := agentNodeID + "-" + mcpNodeID

					if !edgeIndex[edgeID] {
						edgeIndex[edgeID] = true
						edges = append(edges, GraphEdge{
							ID:     edgeID,
							Source: agentNodeID,
							Target: mcpNodeID,
							Weight: 1,
							Type:   "connection",
						})
					}
				}
			}
		}
	}

	// Add attestation edges
	for _, mcp := range mcpServers {
		if h.mcpAttestationRepository != nil {
			attestations, err := h.mcpAttestationRepository.GetValidAttestationsByMCP(mcp.ID)
			if err == nil {
				for _, attestation := range attestations {
					if !attestation.IsValid || attestation.AgentID == nil {
						continue
					}
					agentNodeID := "agent-" + attestation.AgentID.String()
					mcpNodeID := "mcp-" + mcp.ID.String()
					edgeID := agentNodeID + "-" + mcpNodeID + "-attestation"

					if !edgeIndex[edgeID] {
						edgeIndex[edgeID] = true
						edges = append(edges, GraphEdge{
							ID:     edgeID,
							Source: agentNodeID,
							Target: mcpNodeID,
							Weight: 2, // Attestations have higher weight
							Type:   "attestation",
						})
					}
				}
			}
		}
	}

	return c.JSON(MCPGraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}

// GetMCPServerConnections returns connections for a specific MCP server
// @Summary Get MCP server connections
// @Description Returns agents connected to a specific MCP server
// @Tags mcp-graph
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} MCPGraphResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/mcp-servers/{id}/connections [get]
func (h *MCPGraphHandler) GetMCPServerConnections(c fiber.Ctx) error {
	// Get MCP server ID from params
	serverIDStr := c.Params("id")
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Get the MCP server
	mcpServer, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}

	// Get all agents for the organization
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	agents, err := h.agentRepository.GetByOrganization(orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents: " + err.Error(),
		})
	}

	// Build mini graph centered on the MCP server
	nodes := make([]GraphNode, 0)
	edges := make([]GraphEdge, 0)
	nodeIndex := make(map[string]bool)

	// Add MCP server node
	mcpNodeID := "mcp-" + mcpServer.ID.String()
	nodes = append(nodes, GraphNode{
		ID:         mcpNodeID,
		Type:       "mcp",
		Label:      mcpServer.Name,
		Size:       30, // Central node is larger
		Color:      getColorFromScore(mcpServer.ConfidenceScore),
		Status:     string(mcpServer.Status),
		TrustScore: mcpServer.ConfidenceScore,
	})
	nodeIndex[mcpNodeID] = true

	// Find connected agents
	for _, agent := range agents {
		for _, talksTo := range agent.TalksTo {
			if matchesMCPServer(mcpServer, talksTo) {
				agentNodeID := "agent-" + agent.ID.String()
				if !nodeIndex[agentNodeID] {
					nodeIndex[agentNodeID] = true
					nodes = append(nodes, GraphNode{
						ID:         agentNodeID,
						Type:       "agent",
						Label:      agent.DisplayName,
						Size:       20,
						Color:      getColorFromScore(agent.TrustScore * 100),
						Status:     string(agent.Status),
						TrustScore: agent.TrustScore * 100,
					})
				}

				edges = append(edges, GraphEdge{
					ID:     agentNodeID + "-" + mcpNodeID,
					Source: agentNodeID,
					Target: mcpNodeID,
					Weight: 1,
					Type:   "connection",
				})
				break
			}
		}
	}

	// Add attestation connections
	if h.mcpAttestationRepository != nil {
		attestations, err := h.mcpAttestationRepository.GetValidAttestationsByMCP(serverID)
		if err == nil {
			for _, attestation := range attestations {
				if !attestation.IsValid {
					continue
				}
				if attestation.AgentID == nil {
					continue
				}
				agentNodeID := "agent-" + attestation.AgentID.String()
				if !nodeIndex[agentNodeID] {
					// Find agent details
					for _, agent := range agents {
						if agent.ID == *attestation.AgentID {
							nodeIndex[agentNodeID] = true
							nodes = append(nodes, GraphNode{
								ID:         agentNodeID,
								Type:       "agent",
								Label:      agent.DisplayName,
								Size:       20,
								Color:      getColorFromScore(agent.TrustScore * 100),
								Status:     string(agent.Status),
								TrustScore: agent.TrustScore * 100,
							})
							break
						}
					}
				}

				edges = append(edges, GraphEdge{
					ID:     agentNodeID + "-" + mcpNodeID + "-attestation",
					Source: agentNodeID,
					Target: mcpNodeID,
					Weight: 2,
					Type:   "attestation",
				})
			}
		}
	}

	return c.JSON(MCPGraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}

// Helper functions

func getNodeSize(connectionCount int) int {
	// Base size + connections
	baseSize := 15
	if connectionCount > 10 {
		return baseSize + 15
	} else if connectionCount > 5 {
		return baseSize + 10
	} else if connectionCount > 0 {
		return baseSize + 5
	}
	return baseSize
}

func getColorFromScore(score float64) string {
	// Return color based on trust/confidence score (0-100)
	if score >= 80 {
		return "#22c55e" // green-500
	} else if score >= 60 {
		return "#84cc16" // lime-500
	} else if score >= 40 {
		return "#eab308" // yellow-500
	} else if score >= 20 {
		return "#f97316" // orange-500
	}
	return "#ef4444" // red-500
}

func matchesMCPServer(mcp *domain.MCPServer, talksTo string) bool {
	// Check if talksTo matches the MCP server by name, URL, or ID
	if talksTo == mcp.Name || talksTo == mcp.URL || talksTo == mcp.ID.String() {
		return true
	}
	// Also check for partial URL matches
	if len(talksTo) > 0 && len(mcp.URL) > 0 {
		if talksTo == mcp.URL || mcp.URL == talksTo {
			return true
		}
	}
	return false
}
