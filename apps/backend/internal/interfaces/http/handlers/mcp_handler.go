package handlers

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type MCPHandler struct {
	mcpService                   *application.MCPService
	mcpCapabilityService         *application.MCPCapabilityService
	auditService                 *application.AuditService
	agentRepository              *repository.AgentRepository
	verificationEventRepository  domain.VerificationEventRepository
}

func NewMCPHandler(
	mcpService *application.MCPService,
	mcpCapabilityService *application.MCPCapabilityService,
	auditService *application.AuditService,
	agentRepository *repository.AgentRepository,
	verificationEventRepository domain.VerificationEventRepository,
) *MCPHandler {
	return &MCPHandler{
		mcpService:                  mcpService,
		mcpCapabilityService:        mcpCapabilityService,
		auditService:                auditService,
		agentRepository:             agentRepository,
		verificationEventRepository: verificationEventRepository,
	}
}

// CreateMCPServer creates a new MCP server
// @Summary Create MCP server
// @Description Register a new Model Context Protocol server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param request body application.CreateMCPServerRequest true "MCP server details"
// @Success 201 {object} domain.MCPServer
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers [post]
func (h *MCPHandler) CreateMCPServer(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// Support both JWT auth (user_id) and Ed25519 agent auth (agent_id)
	var userID uuid.UUID
	var agentID *uuid.UUID // Track agent ID for SDK registrations

	if userIDLocal := c.Locals("user_id"); userIDLocal != nil {
		// JWT authentication - user creating MCP server
		userID = userIDLocal.(uuid.UUID)
		agentID = nil // No agent involved
	} else if agentIDLocal := c.Locals("agent_id"); agentIDLocal != nil {
		// Ed25519 agent authentication - agent registering MCP server via SDK
		// Use agent's creator as the user_id for audit logging
		agentIDVal := agentIDLocal.(uuid.UUID)
		agentID = &agentIDVal // Store agent ID for connection tracking

		agent, err := h.agentRepository.GetByID(agentIDVal)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Agent not found",
			})
		}
		userID = agent.CreatedBy
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req application.CreateMCPServerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	server, err := h.mcpService.CreateMCPServer(c.Context(), &req, orgID, userID, agentID)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("❌ Error creating MCP server: %v\n", err)

		// Return 409 Conflict for duplicate URL errors
		if err.Error() == "mcp server with this URL already exists" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"mcp_server",
		server.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"server_name":  server.Name,
			"server_url":   server.URL,
			"auth_method":  c.Locals("auth_method"), // Ed25519 or JWT
		},
	)

	return c.Status(fiber.StatusCreated).JSON(server)
}

// ListMCPServers lists all MCP servers for the organization
// @Summary List MCP servers
// @Description Get all MCP servers for the authenticated organization
// @Tags mcp-servers
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers [get]
func (h *MCPHandler) ListMCPServers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	servers, err := h.mcpService.ListMCPServers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers",
		})
	}

	return c.JSON(fiber.Map{
		"mcp_servers": servers,
		"total":       len(servers),
	})
}

// GetMCPServer retrieves a single MCP server
// @Summary Get MCP server
// @Description Get details of a specific MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} domain.MCPServer
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id} [get]
func (h *MCPHandler) GetMCPServer(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}

	// Verify server belongs to organization
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	return c.JSON(server)
}

// UpdateMCPServer updates an MCP server
// @Summary Update MCP server
// @Description Update an existing MCP server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body application.UpdateMCPServerRequest true "Updated MCP server details"
// @Success 200 {object} domain.MCPServer
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id} [put]
func (h *MCPHandler) UpdateMCPServer(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	var req application.UpdateMCPServerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify server belongs to organization first
	existingServer, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if existingServer.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	server, err := h.mcpService.UpdateMCPServer(c.Context(), serverID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"mcp_server",
		server.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"server_name": server.Name,
		},
	)

	return c.JSON(server)
}

// DeleteMCPServer deletes an MCP server
// @Summary Delete MCP server
// @Description Delete an MCP server
// @Tags mcp-servers
// @Param id path string true "MCP Server ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id} [delete]
func (h *MCPHandler) DeleteMCPServer(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	existingServer, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if existingServer.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.mcpService.DeleteMCPServer(c.Context(), serverID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionDelete,
		"mcp_server",
		serverID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// VerifyMCPServer performs cryptographic verification of an MCP server
// @Summary Verify MCP server
// @Description Perform cryptographic verification of an MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/verify [post]
func (h *MCPHandler) VerifyMCPServer(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Generate verification challenge
	challenge, err := h.mcpService.GenerateVerificationChallenge(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate verification challenge",
		})
	}

	// Perform verification with user context
	if err := h.mcpService.VerifyMCPServer(c.Context(), serverID, userID, c.IP()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated server to return in response
	server, _ = h.mcpService.GetMCPServer(c.Context(), serverID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionVerify,
		"mcp_server",
		server.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"server_name":  server.Name,
			"trust_score":  server.TrustScore,
			"challenge":    challenge,
		},
	)

	return c.JSON(fiber.Map{
		"verified":         true,
		"trust_score":      server.TrustScore,
		"verified_at":      server.LastVerifiedAt,
		"challenge":        challenge,
		"verification_url": server.VerificationURL,
	})
}

// AddPublicKey adds a public key to an MCP server
// @Summary Add public key
// @Description Add a public key to an MCP server for cryptographic verification
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body application.AddPublicKeyRequest true "Public key details"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/keys [post]
func (h *MCPHandler) AddPublicKey(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	var req application.AddPublicKeyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.mcpService.AddPublicKey(c.Context(), serverID, &req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"mcp_server",
		serverID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":   "add_public_key",
			"key_type": req.KeyType,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":   "Public key added successfully",
		"server_id": serverID,
		"key_type":  req.KeyType,
	})
}

// GetVerificationStatus retrieves the verification status of an MCP server
// @Summary Get verification status
// @Description Get the current verification status of an MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} domain.MCPServerVerificationStatus
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/verification-status [get]
func (h *MCPHandler) GetVerificationStatus(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	status, err := h.mcpService.GetVerificationStatus(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get verification status",
		})
	}

	return c.JSON(status)
}

// GetMCPServerCapabilities retrieves all capabilities for an MCP server
// @Summary Get MCP server capabilities
// @Description Get all detected capabilities for an MCP server (tools, resources, prompts)
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/capabilities [get]
func (h *MCPHandler) GetMCPServerCapabilities(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Fetch detailed capabilities from mcp_server_capabilities table
	capabilities, err := h.mcpCapabilityService.GetCapabilities(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch capabilities",
		})
	}

	// Return detailed capabilities with full metadata
	return c.JSON(fiber.Map{
		"capabilities": capabilities,
		"total":        len(capabilities),
	})
}

// GetMCPServerAgents retrieves all agents that talk to an MCP server
// @Summary Get agents for MCP server
// @Description Get all agents that are configured to communicate with this MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/agents [get]
func (h *MCPHandler) GetMCPServerAgents(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Fetch agents that have this MCP server in their talks_to array
	// Try both by ID and by NAME (agents often use names, not IDs)
	agentsByID, err := h.agentRepository.GetByMCPServer(serverID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents by ID",
		})
	}

	agentsByName, err := h.agentRepository.GetByMCPServerName(server.Name, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents by name",
		})
	}

	// Combine and deduplicate
	agentMap := make(map[uuid.UUID]*domain.Agent)
	for _, agent := range agentsByID {
		agentMap[agent.ID] = agent
	}
	for _, agent := range agentsByName {
		agentMap[agent.ID] = agent
	}

	agents := make([]*domain.Agent, 0, len(agentMap))
	for _, agent := range agentMap {
		agents = append(agents, agent)
	}

	// Map to simple response with just the info needed for the modal
	agentSummaries := make([]fiber.Map, 0, len(agents))
	for _, agent := range agents {
		agentSummaries = append(agentSummaries, fiber.Map{
			"id":           agent.ID,
			"name":         agent.Name,
			"display_name": agent.DisplayName,
			"agent_type":   agent.AgentType,
			"status":       agent.Status,
		})
	}

	return c.JSON(fiber.Map{
		"agents": agentSummaries,
		"total":  len(agentSummaries),
	})
}

// GetMCPVerificationEvents retrieves verification events for a specific MCP server
// @Summary Get MCP server verification events
// @Description Get all verification events for a specific MCP server with pagination
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param limit query int false "Number of events to return" default(50)
// @Param offset query int false "Number of events to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/verification-events [get]
func (h *MCPHandler) GetMCPVerificationEvents(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if server.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Parse pagination parameters
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Get verification events for this MCP server
	events, total, err := h.verificationEventRepository.GetByMCPServer(serverID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch verification events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMCPAuditLogs retrieves audit logs for a specific MCP server
// @Summary Get MCP server audit logs
// @Description Get all audit logs for a specific MCP server with pagination
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param limit query int false "Number of logs to return" default(50)
// @Param offset query int false "Number of logs to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// VerifyMCPAction verifies if an MCP server can perform the requested action
// @Summary Verify MCP action authorization
// @Description Verify if an MCP server is authorized to perform a specific action
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body VerifyMCPActionRequest true "Action verification request"
// @Success 200 {object} VerifyActionResponse
// @Failure 403 {object} ErrorResponse "Action denied"
// @Router /mcp-servers/{id}/verify-action [post]
func (h *MCPHandler) VerifyMCPAction(c fiber.Ctx) error {
	mcpID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP ID",
		})
	}

	var req struct {
		ActionType    string                 `json:"action_type"`    // "database_query", "api_call", "file_access"
		Resource      string                 `json:"resource"`       // e.g., "SELECT * FROM table" or "POST /api/endpoint"
		TargetService string                 `json:"target_service"` // e.g., "postgresql://prod-db"
		Metadata      map[string]interface{} `json:"metadata"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify MCP action
	decision, reason, auditID, err := h.mcpService.VerifyMCPAction(
		c.Context(),
		mcpID,
		req.ActionType,
		req.Resource,
		req.TargetService,
		req.Metadata,
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Verification failed",
		})
	}

	if !decision {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"allowed":  false,
			"reason":   reason,
			"audit_id": auditID,
		})
	}

	return c.JSON(fiber.Map{
		"allowed":  true,
		"reason":   reason,
		"audit_id": auditID,
	})
}

// GetConnectedAgents returns all agents using an MCP server
// @Summary Get connected agents
// @Description Get list of agents that use this MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/connected-agents [get]
func (h *MCPHandler) GetConnectedAgents(c fiber.Ctx) error {
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Get connected agents
	agents, err := h.mcpService.GetConnectedAgents(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"connected_agents": agents,
		"count":            len(agents),
	})
}
