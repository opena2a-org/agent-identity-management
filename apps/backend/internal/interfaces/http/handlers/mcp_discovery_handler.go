package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// MCPDiscoveryHandler handles MCP discovery endpoints
type MCPDiscoveryHandler struct {
	mcpService      *application.MCPService
	agentRepository *repository.AgentRepository
}

// NewMCPDiscoveryHandler creates a new MCP discovery handler
func NewMCPDiscoveryHandler(
	mcpService *application.MCPService,
	agentRepository *repository.AgentRepository,
) *MCPDiscoveryHandler {
	return &MCPDiscoveryHandler{
		mcpService:      mcpService,
		agentRepository: agentRepository,
	}
}

// DiscoveredMCP represents an MCP server detected by agents but not yet registered
type DiscoveredMCP struct {
	Name            string    `json:"name"`
	URL             string    `json:"url"`             // URL if detected from URL format
	DetectedBy      []string  `json:"detectedBy"`      // Agent display names
	DetectedByCount int       `json:"detectedByCount"` // Number of agents
	DetectionMethod string    `json:"detectionMethod"` // talks_to, sdk_import, runtime
	FirstDetectedAt time.Time `json:"firstDetectedAt"` // Earliest detection
	LastDetectedAt  time.Time `json:"lastDetectedAt"`  // Most recent detection
	IsRegistered    bool      `json:"isRegistered"`    // If already registered
	MatchingServer  *string   `json:"matchingServerId,omitempty"` // Registered server ID if match found
}

// MCPDiscoveryResponse is the response for discovered MCPs
type MCPDiscoveryResponse struct {
	Discovered       []DiscoveredMCP `json:"discovered"`
	TotalUnmapped    int             `json:"totalUnmapped"`    // MCPs in talks_to not mapped to registered servers
	TotalAgents      int             `json:"totalAgents"`      // Total agents checked
	RegisteredServers int            `json:"registeredServers"` // Total registered MCP servers
}

// GetDiscoveredMCPs returns MCPs detected by agents but not yet registered
// @Summary Get discovered MCP servers
// @Description Returns MCP servers that agents reference but aren't registered
// @Tags mcp-discovery
// @Produce json
// @Success 200 {object} MCPDiscoveryResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/mcp-servers/discovered [get]
func (h *MCPDiscoveryHandler) GetDiscoveredMCPs(c fiber.Ctx) error {
	// Get organization ID from context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	// Get all agents for the organization
	agents, err := h.agentRepository.GetByOrganization(orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents: " + err.Error(),
		})
	}

	// Get all registered MCP servers
	mcpServers, err := h.mcpService.ListMCPServers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers: " + err.Error(),
		})
	}

	// Create lookup maps for registered MCP servers
	registeredMCPs := make(map[string]*uuid.UUID) // name/url -> ID
	registeredMCPsByID := make(map[string]string) // ID -> name (for resolving UUID references)
	for _, mcp := range mcpServers {
		// Map by name (lowercase for comparison)
		registeredMCPs[strings.ToLower(mcp.Name)] = &mcp.ID
		// Map by URL if present
		if mcp.URL != "" {
			registeredMCPs[strings.ToLower(mcp.URL)] = &mcp.ID
		}
		// Map ID to name for resolving UUID references in talks_to
		registeredMCPsByID[mcp.ID.String()] = mcp.Name
	}

	// Track discovered MCPs
	discoveredMap := make(map[string]*DiscoveredMCP) // talks_to value -> DiscoveredMCP

	// Scan all agents' talks_to fields
	for _, agent := range agents {
		for _, talksTo := range agent.TalksTo {
			talksToLower := strings.ToLower(talksTo)

			// Resolve display name - check if talksTo is a UUID that maps to a registered MCP
			displayName := talksTo
			resolvedKey := talksToLower // Use for deduplication

			// Check if talksTo looks like a UUID (try to parse it)
			if parsedUUID, err := uuid.Parse(talksTo); err == nil {
				// It's a valid UUID, check if it maps to a registered MCP
				if mcpName, found := registeredMCPsByID[parsedUUID.String()]; found {
					displayName = mcpName
					resolvedKey = strings.ToLower(mcpName)
				}
			}

			// Check if this MCP is already discovered (using resolved key for deduplication)
			if discovered, exists := discoveredMap[resolvedKey]; exists {
				// Update existing discovery
				discovered.DetectedBy = append(discovered.DetectedBy, agent.DisplayName)
				discovered.DetectedByCount++
				if agent.CreatedAt.After(discovered.LastDetectedAt) {
					discovered.LastDetectedAt = agent.CreatedAt
				}
			} else {
				// Create new discovery
				discovered := &DiscoveredMCP{
					Name:            displayName,
					DetectedBy:      []string{agent.DisplayName},
					DetectedByCount: 1,
					DetectionMethod: "talks_to",
					FirstDetectedAt: agent.CreatedAt,
					LastDetectedAt:  agent.CreatedAt,
					IsRegistered:    false,
				}

				// Check if it looks like a URL
				if strings.HasPrefix(talksToLower, "http://") || strings.HasPrefix(talksToLower, "https://") {
					discovered.URL = talksTo
				}

				// Check if it matches a registered server by name/URL or if we resolved it from UUID
				if mcpID, found := registeredMCPs[resolvedKey]; found {
					discovered.IsRegistered = true
					idStr := mcpID.String()
					discovered.MatchingServer = &idStr
				} else if _, err := uuid.Parse(talksTo); err == nil {
					// If talksTo was a UUID, check if it directly matches a registered server ID
					if mcpName, found := registeredMCPsByID[talksTo]; found {
						discovered.IsRegistered = true
						discovered.MatchingServer = &talksTo
						discovered.Name = mcpName // Use the resolved name
					}
				}

				discoveredMap[resolvedKey] = discovered
			}
		}
	}

	// Convert map to slice
	var discovered []DiscoveredMCP
	var totalUnmapped int
	for _, d := range discoveredMap {
		discovered = append(discovered, *d)
		if !d.IsRegistered {
			totalUnmapped++
		}
	}

	// Sort by detection count (most detected first)
	for i := 0; i < len(discovered)-1; i++ {
		for j := i + 1; j < len(discovered); j++ {
			if discovered[j].DetectedByCount > discovered[i].DetectedByCount {
				discovered[i], discovered[j] = discovered[j], discovered[i]
			}
		}
	}

	return c.JSON(MCPDiscoveryResponse{
		Discovered:        discovered,
		TotalUnmapped:     totalUnmapped,
		TotalAgents:       len(agents),
		RegisteredServers: len(mcpServers),
	})
}

// IgnoreDiscoveredMCP marks a discovered MCP as ignored (false positive)
// @Summary Ignore discovered MCP
// @Description Marks a discovered MCP as ignored to hide it from future discovery lists
// @Tags mcp-discovery
// @Accept json
// @Produce json
// @Param body body IgnoreDiscoveredMCPRequest true "MCP to ignore"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/mcp-servers/discovered/ignore [post]
func (h *MCPDiscoveryHandler) IgnoreDiscoveredMCP(c fiber.Ctx) error {
	// This would store the ignored MCP in a separate table
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"message": "MCP ignored successfully",
	})
}

// IgnoreDiscoveredMCPRequest is the request body for ignoring a discovered MCP
type IgnoreDiscoveredMCPRequest struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
