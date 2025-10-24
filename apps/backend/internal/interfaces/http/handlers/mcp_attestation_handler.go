package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type MCPAttestationHandler struct {
	attestationService *application.MCPAttestationService
	auditService       *application.AuditService
}

func NewMCPAttestationHandler(
	attestationService *application.MCPAttestationService,
	auditService *application.AuditService,
) *MCPAttestationHandler {
	return &MCPAttestationHandler{
		attestationService: attestationService,
		auditService:       auditService,
	}
}

// AttestMCP handles agent attestation of an MCP server
// @Summary Attest MCP server
// @Description Submit cryptographically signed attestation from a verified agent
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body application.AttestMCPRequest true "Attestation data and signature"
// @Success 200 {object} application.AttestMCPResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/attest [post]
func (h *MCPAttestationHandler) AttestMCP(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	var req application.AttestMCPRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// Verify and record attestation
	response, err := h.attestationService.VerifyAndRecordAttestation(c.Context(), mcpServerID, &req)
	if err != nil {
		// Determine status code based on error
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "only verified agents can attest MCPs" ||
			err.Error() == "invalid attestation signature" ||
			err.Error() == "attestation expired (older than 5 minutes)" {
			statusCode = fiber.StatusForbidden
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Attestation failed",
			"message": err.Error(),
		})
	}

	// Audit log
	userID := c.Locals("user_id")
	orgID := c.Locals("organization_id")
	if userID != nil && orgID != nil {
		h.auditService.LogAction(
			c.Context(),
			orgID.(uuid.UUID),  // Organization ID first
			userID.(uuid.UUID), // Then user ID
			domain.AuditActionAttest,
			"mcp_server",
			mcpServerID,
			c.IP(),              // IP address
			c.Get("User-Agent"), // User agent
			fiber.Map{
				"attestation_id":       response.AttestationID,
				"confidence_score":     response.MCPConfidenceScore,
				"attestation_count":    response.AttestationCount,
				"agent_id":             req.Attestation.AgentID,
				"capabilities_found":   req.Attestation.CapabilitiesFound,
				"connection_latency_ms": req.Attestation.ConnectionLatencyMs,
			},
		)
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetMCPAttestations retrieves all attestations for an MCP server
// @Summary Get MCP attestations
// @Description Retrieve all agent attestations for an MCP server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/attestations [get]
func (h *MCPAttestationHandler) GetMCPAttestations(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Get attestations
	attestations, confidenceScore, lastAttestedAt, err := h.attestationService.GetMCPAttestations(c.Context(), mcpServerID)
	if err != nil {
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Failed to get attestations",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"attestations":       attestations,
		"total":              len(attestations),
		"confidence_score":   confidenceScore,
		"last_attested_at":   lastAttestedAt,
	})
}

// GetConnectedAgents retrieves all agents connected to an MCP server
// @Summary Get connected agents
// @Description Retrieve all agents that have connections to this MCP server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/agents [get]
func (h *MCPAttestationHandler) GetConnectedAgents(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Get connected agents
	agents, err := h.attestationService.GetConnectedAgentsForMCP(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get connected agents",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"agents": agents,
		"total":  len(agents),
	})
}

// GetAgentMCPServers retrieves all MCP servers connected to an agent
// @Summary Get agent MCP servers
// @Description Retrieve all MCP servers that an agent is connected to
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/agents/{id}/mcp-servers [get]
func (h *MCPAttestationHandler) GetAgentMCPServers(c fiber.Ctx) error {
	// Parse agent ID from URL
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// Get MCP servers
	mcpServers, err := h.attestationService.GetMCPServersForAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get MCP servers",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"mcp_servers": mcpServers,
		"total":       len(mcpServers),
	})
}
