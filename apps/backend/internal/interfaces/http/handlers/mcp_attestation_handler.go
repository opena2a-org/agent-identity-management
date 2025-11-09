package handlers

import (
	"fmt"

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
		// Log the actual error for debugging
		fmt.Printf("❌ Attestation failed for MCP %s: %v\n", mcpServerID, err)

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

// ManualAttestMCP handles manual attestation by a user (non-SDK, JWT-based)
// @Summary Manually attest MCP server
// @Description Submit manual attestation from a logged-in user for an MCP server they've verified
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body ManualAttestationRequest true "Manual attestation details"
// @Success 200 {object} application.AttestMCPResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id/manual-attest [post]
func (h *MCPAttestationHandler) ManualAttestMCP(c fiber.Ctx) error {
	// Get authenticated user ID from JWT middleware
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get organization ID
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type ManualAttestationRequest struct {
		Notes                string   `json:"notes"`                  // Optional notes from user
		CapabilitiesVerified []string `json:"capabilities_verified"`  // Capabilities user verified
		ConnectionTested     bool     `json:"connection_tested"`      // Did user test connection?
		HealthCheckPassed    bool     `json:"health_check_passed"`    // Did health check pass?
	}

	var req ManualAttestationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// Call service method for manual attestation
	response, err := h.attestationService.RecordManualAttestation(
		c.Context(),
		mcpServerID,
		userID,
		orgID,
		req.CapabilitiesVerified,
		req.ConnectionTested,
		req.HealthCheckPassed,
		req.Notes,
	)
	if err != nil {
		fmt.Printf("❌ Manual attestation failed for MCP %s: %v\n", mcpServerID, err)

		statusCode := fiber.StatusInternalServerError
		if err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Manual attestation failed",
			"message": err.Error(),
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionAttest,
		"mcp_server",
		mcpServerID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"attestation_id":      response.AttestationID,
			"confidence_score":    response.MCPConfidenceScore,
			"attestation_count":   response.AttestationCount,
			"attestation_type":    "manual",
			"capabilities_verified": req.CapabilitiesVerified,
			"connection_tested":   req.ConnectionTested,
		},
	)

	return c.Status(fiber.StatusOK).JSON(response)
}

// RecordMCPConnection handles agent recording MCP tool usage
// @Summary Record MCP connection
// @Description Record that an agent is using an MCP server tool (creates/updates agent-MCP connection)
// @Tags sdk-api
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body map[string]interface{} true "Connection data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/sdk-api/agents/{agent_id}/mcp-connections [post]
func (h *MCPAttestationHandler) RecordMCPConnection(c fiber.Ctx) error {
	// Get agent ID from URL path (already authenticated by SDK middleware)
	agentID, err := uuid.Parse(c.Params("agent_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type RecordConnectionRequest struct {
		MCPServerID    string `json:"mcp_server_id"`
		ToolName       string `json:"tool_name"`
		MCPURL         string `json:"mcp_url"`
		MCPName        string `json:"mcp_name"`
		ConnectionType string `json:"connection_type"`
	}

	var req RecordConnectionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// Validate required fields
	if req.MCPServerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mcp_server_id is required",
		})
	}

	if req.ToolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tool_name is required",
		})
	}

	mcpServerID, err := uuid.Parse(req.MCPServerID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Record the connection
	connection, err := h.attestationService.RecordAgentMCPConnection(
		c.Context(),
		agentID,
		mcpServerID,
		req.ToolName,
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to record MCP connection",
			"message": err.Error(),
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		uuid.Nil, // No organization context for SDK endpoints
		agentID,
		domain.AuditActionCreate,
		"agent_mcp_connection",
		connection.ID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"connection_id":      connection.ID,
			"mcp_server_id":      mcpServerID,
			"tool_name":          req.ToolName,
			"connection_type":    connection.ConnectionType,
			"attestation_count":  connection.AttestationCount,
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":           true,
		"connection_id":     connection.ID,
		"agent_id":          connection.AgentID,
		"mcp_server_id":     connection.MCPServerID,
		"connection_type":   connection.ConnectionType,
		"attestation_count": connection.AttestationCount,
		"last_attested_at":  connection.LastAttestedAt,
		"message":           "MCP connection recorded successfully",
	})
}
