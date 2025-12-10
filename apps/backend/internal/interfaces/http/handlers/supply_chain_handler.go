package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

// SupplyChainHandler handles supply chain analytics endpoints
type SupplyChainHandler struct {
	connectionRepo *repository.AgentMCPConnectionRepository
	agentRepo      *repository.AgentRepository
	mcpRepo        *repository.MCPServerRepository
	capabilityRepo *repository.MCPServerCapabilityRepository
}

// NewSupplyChainHandler creates a new supply chain handler
func NewSupplyChainHandler(
	connectionRepo *repository.AgentMCPConnectionRepository,
	agentRepo *repository.AgentRepository,
	mcpRepo *repository.MCPServerRepository,
	capabilityRepo *repository.MCPServerCapabilityRepository,
) *SupplyChainHandler {
	return &SupplyChainHandler{
		connectionRepo: connectionRepo,
		agentRepo:      agentRepo,
		mcpRepo:        mcpRepo,
		capabilityRepo: capabilityRepo,
	}
}

// ConnectionWithDetails includes agent and MCP server names
type ConnectionWithDetails struct {
	ID               string  `json:"id"`
	AgentID          string  `json:"agentId"`
	AgentName        string  `json:"agentName"`
	MCPServerID      string  `json:"mcpServerId"`
	MCPServerName    string  `json:"mcpServerName"`
	ConnectionType   string  `json:"connectionType"`
	AttestationCount int     `json:"attestationCount"`
	FirstConnectedAt string  `json:"firstConnectedAt"`
	LastAttestedAt   *string `json:"lastAttestedAt"`
	IsActive         bool    `json:"isActive"`
}

// SupplyChainAnalyticsResponse is the full response for supply chain dashboard
type SupplyChainAnalyticsResponse struct {
	Stats            *repository.SupplyChainStats         `json:"stats"`
	AttestationTrend []repository.AttestationTrendEntry   `json:"attestationTrend"`
	Connections      []ConnectionWithDetails              `json:"connections"`
}

// GetSupplyChainAnalytics returns all data needed for the supply chain dashboard
// @Summary Get supply chain analytics
// @Description Returns comprehensive supply chain data including stats, trends, and connections
// @Tags supply-chain
// @Produce json
// @Param days query int false "Number of days for trend data" default(7)
// @Success 200 {object} SupplyChainAnalyticsResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/supply-chain/analytics [get]
func (h *SupplyChainHandler) GetSupplyChainAnalytics(c fiber.Ctx) error {
	// Get organization ID from context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		log.Printf("❌ Supply chain analytics: organization_id not found in context, locals: %v", c.Locals("organization_id"))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}
	log.Printf("✅ Supply chain analytics: orgID=%s", orgID)

	// Get days parameter (default 7)
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays >= 1 && parsedDays <= 90 {
			days = parsedDays
		}
	}

	// Get supply chain stats
	stats, err := h.connectionRepo.GetSupplyChainStats(c.Context(), orgID)
	if err != nil {
		log.Printf("❌ GetSupplyChainStats error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get supply chain stats: " + err.Error(),
		})
	}
	log.Printf("✅ Stats: connections=%d, attestations=%d", stats.TotalConnections, stats.TotalAttestations)

	// Get attestation trend
	trend, err := h.connectionRepo.GetAttestationTrend(c.Context(), orgID, days)
	if err != nil {
		log.Printf("❌ GetAttestationTrend error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get attestation trend: " + err.Error(),
		})
	}
	log.Printf("✅ Trend: %d entries", len(trend))

	// Get connections with details
	connections, err := h.connectionRepo.ListByOrganization(c.Context(), orgID)
	if err != nil {
		log.Printf("❌ ListByOrganization error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get connections: " + err.Error(),
		})
	}
	log.Printf("✅ Connections: %d found", len(connections))

	// Build agent and MCP server name maps for lookup
	agentNames := make(map[string]string)
	mcpNames := make(map[string]string)

	// Get all agents for org
	agents, err := h.agentRepo.GetByOrganization(orgID)
	if err != nil {
		log.Printf("⚠️ Failed to get agents for org %s: %v", orgID, err)
	}
	for _, agent := range agents {
		agentNames[agent.ID.String()] = agent.DisplayName
	}
	log.Printf("✅ Loaded %d agents for name lookup", len(agents))

	// Get all MCP servers for org
	mcpServers, err := h.mcpRepo.GetByOrganization(orgID)
	if err != nil {
		log.Printf("⚠️ Failed to get MCP servers for org %s: %v", orgID, err)
	}
	for _, mcp := range mcpServers {
		mcpNames[mcp.ID.String()] = mcp.Name
	}
	log.Printf("✅ Loaded %d MCP servers for name lookup", len(mcpServers))

	// Transform connections to include names
	connectionDetails := make([]ConnectionWithDetails, 0, len(connections))
	for _, conn := range connections {
		var lastAttested *string
		if conn.LastAttestedAt != nil {
			t := conn.LastAttestedAt.Format("2006-01-02T15:04:05Z")
			lastAttested = &t
		}

		connectionDetails = append(connectionDetails, ConnectionWithDetails{
			ID:               conn.ID.String(),
			AgentID:          conn.AgentID.String(),
			AgentName:        agentNames[conn.AgentID.String()],
			MCPServerID:      conn.MCPServerID.String(),
			MCPServerName:    mcpNames[conn.MCPServerID.String()],
			ConnectionType:   string(conn.ConnectionType),
			AttestationCount: conn.AttestationCount,
			FirstConnectedAt: conn.FirstConnectedAt.Format("2006-01-02T15:04:05Z"),
			LastAttestedAt:   lastAttested,
			IsActive:         conn.IsActive,
		})
	}

	return c.JSON(SupplyChainAnalyticsResponse{
		Stats:            stats,
		AttestationTrend: trend,
		Connections:      connectionDetails,
	})
}

// CapabilityDriftResponse is the response for capability drift alerts
type CapabilityDriftResponse struct {
	Stats  *repository.CapabilityDriftStats   `json:"stats"`
	Alerts []repository.CapabilityDriftAlert  `json:"alerts"`
}

// GetCapabilityDriftAlerts returns capability drift alerts for the organization
// @Summary Get capability drift alerts
// @Description Returns alerts for MCP capabilities that have been added, removed, or not verified recently
// @Tags supply-chain
// @Produce json
// @Param days query int false "Number of days to check for drift" default(7)
// @Success 200 {object} CapabilityDriftResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/supply-chain/drift-alerts [get]
func (h *SupplyChainHandler) GetCapabilityDriftAlerts(c fiber.Ctx) error {
	// Get organization ID from context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	// Get days parameter (default 7)
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays >= 1 && parsedDays <= 90 {
			days = parsedDays
		}
	}

	// Get capability drift alerts
	alerts, stats, err := h.capabilityRepo.GetCapabilityDriftAlerts(orgID, days)
	if err != nil {
		log.Printf("❌ GetCapabilityDriftAlerts error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get capability drift alerts: " + err.Error(),
		})
	}
	log.Printf("✅ Drift alerts: %d alerts found", len(alerts))

	// Ensure alerts is not nil (return empty array instead)
	if alerts == nil {
		alerts = []repository.CapabilityDriftAlert{}
	}

	return c.JSON(CapabilityDriftResponse{
		Stats:  stats,
		Alerts: alerts,
	})
}
