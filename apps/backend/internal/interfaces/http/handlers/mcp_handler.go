package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

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
	tagService                   *application.TagService              // ✅ For fetching MCP server tags in responses
	attestationService           *application.MCPAttestationService   // ✅ For rich audit trail with attestations
}

func NewMCPHandler(
	mcpService *application.MCPService,
	mcpCapabilityService *application.MCPCapabilityService,
	auditService *application.AuditService,
	agentRepository *repository.AgentRepository,
	verificationEventRepository domain.VerificationEventRepository,
	tagService *application.TagService, // ✅ For fetching MCP server tags in responses
	attestationService *application.MCPAttestationService, // ✅ For rich audit trail with attestations
) *MCPHandler {
	return &MCPHandler{
		mcpService:                  mcpService,
		mcpCapabilityService:        mcpCapabilityService,
		auditService:                auditService,
		agentRepository:             agentRepository,
		verificationEventRepository: verificationEventRepository,
		tagService:                  tagService,
		attestationService:          attestationService,
	}
}

// enrichMCPServerResponse adds tags to MCP server response
func (h *MCPHandler) enrichMCPServerResponse(c fiber.Ctx, server *domain.MCPServer) fiber.Map {
	// ✅ Fetch tags from mcp_server_tags table
	var tags []fiber.Map
	if h.tagService != nil {
		serverTags, err := h.tagService.GetMCPServerTags(c.Context(), server.ID)
		if err != nil {
			tags = []fiber.Map{}
		} else {
			tags = make([]fiber.Map, 0, len(serverTags))
			for _, tag := range serverTags {
				tags = append(tags, fiber.Map{
					"id":          tag.ID,
					"key":         tag.Key,
					"value":       tag.Value,
					"category":    tag.Category,
					"description": tag.Description,
					"color":       tag.Color,
				})
			}
		}
	} else {
		tags = []fiber.Map{}
	}

	return fiber.Map{
		"id":                  server.ID,
		"organizationId":      server.OrganizationID,
		"name":                server.Name,
		"description":         server.Description,
		"url":                 server.URL,
		"version":             server.Version,
		"publicKey":           server.PublicKey,
		"status":              server.Status,
		"isVerified":          server.IsVerified,
		"verificationUrl":     server.VerificationURL,
		"trustScore":          server.TrustScore,
		"capabilities":        server.Capabilities,
		"tags":                tags, // ✅ Include tags in response
		"createdAt":           server.CreatedAt,
		"updatedAt":           server.UpdatedAt,
		"createdBy":           server.CreatedBy,
		"createdByName":       server.CreatedByName,       // ✅ Audit trail: creator name
		"createdByEmail":      server.CreatedByEmail,      // ✅ Audit trail: creator email
		"createdBySdkTokenId": server.CreatedBySDKTokenID, // ✅ SDK token tracking
		"updatedBy":           server.UpdatedBy,           // ✅ Audit trail: last updater
		"updatedByName":       server.UpdatedByName,       // ✅ Audit trail: updater name
		"updatedByEmail":      server.UpdatedByEmail,      // ✅ Audit trail: updater email
		"lastVerifiedAt":      server.LastVerifiedAt,
		"verificationCount":   server.VerificationCount,
		"registeredByAgent":   server.RegisteredByAgent,
		"attestationCount":    server.AttestationCount,    // ✅ MCP attestation count
		"confidenceScore":     server.ConfidenceScore,     // ✅ MCP confidence score
		"lastAttestedAt":      server.LastAttestedAt,      // ✅ Last attestation timestamp
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

	// ✅ If SDK passes RegisteredByAgent in request body, use it for connection tracking
	// This allows OAuth-authenticated requests to still create agent-MCP connections
	if req.RegisteredByAgent != "" && agentID == nil {
		if parsedAgentID, err := uuid.Parse(req.RegisteredByAgent); err == nil {
			agentID = &parsedAgentID
		}
	}

	// ✅ Extract SDK token ID from context (set by SDK token tracking middleware)
	var sdkTokenID *uuid.UUID
	if sdkTokenIDLocal := c.Locals("sdk_token_id"); sdkTokenIDLocal != nil {
		if tokenIDStr, ok := sdkTokenIDLocal.(string); ok && tokenIDStr != "" {
			if parsedID, err := uuid.Parse(tokenIDStr); err == nil {
				sdkTokenID = &parsedID
			}
		}
	}

	// ✅ Extract API key ID from context (set by API key middleware)
	var apiKeyID *uuid.UUID
	if apiKeyIDLocal := c.Locals("api_key_id"); apiKeyIDLocal != nil {
		if apiKeyIDVal, ok := apiKeyIDLocal.(uuid.UUID); ok {
			apiKeyID = &apiKeyIDVal
		}
	}

	// SECURITY: No error logging to prevent information leakage
	server, err := h.mcpService.CreateMCPServer(c.Context(), &req, orgID, userID, agentID, sdkTokenID, apiKeyID)
	if err != nil {
		// Return 409 Conflict for duplicate URL errors - include existing server ID for SDK
		if err.Error() == "mcp server with this URL already exists" {
			response := fiber.Map{
				"error": err.Error(),
			}
			// ✅ Include existing server ID so SDK can still use it for attestation
			if server != nil {
				response["existingId"] = server.ID.String()
				response["name"] = server.Name
			}
			return c.Status(fiber.StatusConflict).JSON(response)
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
			"serverName": server.Name,
			"serverUrl":  server.URL,
			"authMethod": c.Locals("auth_method"), // Ed25519 or JWT
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

	// ✅ Enrich each server with tags and tool count
	enriched := make([]fiber.Map, 0, len(servers))
	for _, server := range servers {
		serverMap := h.enrichMCPServerResponse(c, server)

		// ✅ Add actual tool count from mcp_server_capabilities table
		if h.mcpCapabilityService != nil {
			capabilities, err := h.mcpCapabilityService.GetCapabilities(c.Context(), server.ID)
			if err == nil {
				// Count only tools (not prompts/resources)
				toolCount := 0
				for _, cap := range capabilities {
					if cap.CapabilityType == "tool" {
						toolCount++
					}
				}
				serverMap["toolCount"] = toolCount
			} else {
				serverMap["toolCount"] = 0
			}
		} else {
			serverMap["toolCount"] = 0
		}

		enriched = append(enriched, serverMap)
	}

	return c.JSON(fiber.Map{
		"mcpServers": enriched,
		"total":      len(enriched),
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

	// ✅ Enrich response with tags
	return c.JSON(h.enrichMCPServerResponse(c, server))
}

// GetMCPServerByName retrieves an MCP server by name (for SDK capability caching)
// @Summary Get MCP server by name
// @Description Get details of an MCP server by name - useful for SDK capability caching
// @Tags mcp-servers
// @Produce json
// @Param name query string true "MCP Server name"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/by-name [get]
func (h *MCPHandler) GetMCPServerByName(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	name := c.Query("name")
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name query parameter is required",
		})
	}

	server, err := h.mcpService.GetMCPServerByName(c.Context(), orgID, name)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "MCP server not found",
			"name":    name,
			"message": "No cached capabilities available - SDK should run local discovery",
		})
	}

	// Return server with capabilities (for SDK caching)
	return c.JSON(fiber.Map{
		"id":           server.ID,
		"name":         server.Name,
		"url":          server.URL,
		"capabilities": server.Capabilities,
		"hasCachedCapabilities": len(server.Capabilities) > 0,
		"toolCount":    len(server.Capabilities),
	})
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
			"serverName": server.Name,
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
			"serverName":  server.Name,
			"trustScore":  server.TrustScore,
			"challenge":    challenge,
		},
	)

	return c.JSON(fiber.Map{
		"verified":        true,
		"trustScore":      server.TrustScore,
		"verifiedAt":      server.LastVerifiedAt,
		"challenge":       challenge,
		"verificationUrl": server.VerificationURL,
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
			"action":  "add_public_key",
			"keyType": req.KeyType,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Public key added successfully",
		"serverId": serverID,
		"keyType":  req.KeyType,
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

// DetectCapabilities manually triggers capability detection for an MCP server
// @Summary Detect capabilities
// @Description Manually trigger capability detection from the MCP server
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/detect-capabilities [post]
func (h *MCPHandler) DetectCapabilities(c fiber.Ctx) error {
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

	// Trigger capability detection
	if err := h.mcpCapabilityService.DetectCapabilities(c.Context(), serverID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to detect capabilities",
			"details": err.Error(),
		})
	}

	// Fetch the newly detected capabilities
	capabilities, err := h.mcpCapabilityService.GetCapabilities(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch capabilities after detection",
		})
	}

	return c.JSON(fiber.Map{
		"success":      true,
		"message":      "Capabilities detected successfully",
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
	log.Printf("⭐ GetMCPServerAgents: HANDLER CALLED")
	orgID := c.Locals("organization_id").(uuid.UUID)
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	log.Printf("🔍 GetMCPServerAgents: serverID=%s, orgID=%s", serverID, orgID)

	// Verify server belongs to organization first
	server, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		log.Printf("❌ GetMCPServerAgents: MCP server not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	log.Printf("✅ GetMCPServerAgents: Found MCP server '%s' (orgID=%s)", server.Name, server.OrganizationID)

	if server.OrganizationID != orgID {
		log.Printf("❌ GetMCPServerAgents: Org mismatch - server.OrgID=%s != request.OrgID=%s", server.OrganizationID, orgID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Fetch agents that have this MCP server in their talks_to array
	// Try both by ID and by NAME (agents often use names, not IDs)
	log.Printf("🔍 GetMCPServerAgents: Searching for agents by MCP server ID and name...")
	agentsByID, err := h.agentRepository.GetByMCPServer(serverID, orgID)
	if err != nil {
		log.Printf("❌ GetMCPServerAgents: Failed to fetch agents by ID: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents by ID",
		})
	}
	log.Printf("✅ GetMCPServerAgents: Found %d agents by ID", len(agentsByID))

	agentsByName, err := h.agentRepository.GetByMCPServerName(server.Name, orgID)
	if err != nil {
		log.Printf("❌ GetMCPServerAgents: Failed to fetch agents by name: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents by name",
		})
	}
	log.Printf("✅ GetMCPServerAgents: Found %d agents by name", len(agentsByName))

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
			"displayName": agent.DisplayName,
			"agentType":   agent.AgentType,
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
// @Summary Verify MCP capability authorization
// @Description Verify if an MCP server is authorized to use a specific capability
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body VerifyMCPCapabilityRequest true "Capability verification request"
// @Success 200 {object} VerifyCapabilityResponse
// @Failure 403 {object} ErrorResponse "Capability denied"
// @Router /mcp-servers/{id}/verify-capability [post]
func (h *MCPHandler) VerifyMCPCapability(c fiber.Ctx) error {
	mcpID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP ID",
		})
	}

	var req struct {
		Capability    string                 `json:"capability"`    // "db:query", "api:call", "file:access"
		Resource      string                 `json:"resource"`      // e.g., "SELECT * FROM table" or "POST /api/endpoint"
		TargetService string                 `json:"targetService"` // e.g., "postgresql://prod-db"
		Metadata      map[string]interface{} `json:"metadata"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify MCP capability
	decision, reason, auditID, err := h.mcpService.VerifyMCPCapability(
		c.Context(),
		mcpID,
		req.Capability,
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
			"allowed": false,
			"reason":  reason,
			"auditId": auditID,
		})
	}

	return c.JSON(fiber.Map{
		"allowed": true,
		"reason":  reason,
		"auditId": auditID,
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
		"connectedAgents": agents,
		"count":           len(agents),
	})
}

// GetMCPServerAuditLogs returns a unified audit timeline for a specific MCP server
// @Summary Get MCP server audit logs
// @Description Get a unified audit timeline including audit logs, attestation events, and capability changes
// @Tags mcp-servers
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param limit query int false "Number of logs to return (default: 50, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string "Invalid MCP server ID"
// @Failure 404 {object} map[string]string "MCP server not found"
// @Failure 403 {object} map[string]string "Access denied"
// @Router /api/v1/mcp-servers/{id}/audit-logs [get]
func (h *MCPHandler) GetMCPServerAuditLogs(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid MCP server ID",
		})
	}

	// Parse pagination parameters
	limit := 50 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			if limit > 100 {
				limit = 100 // enforce max
			}
			if limit < 1 {
				limit = 50 // reset to default
			}
		}
	}

	offset := 0 // default
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	// Verify MCP server belongs to organization
	mcpServer, err := h.mcpService.GetMCPServer(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}
	if mcpServer.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Build unified timeline from multiple sources
	type TimelineEvent struct {
		ID          string                 `json:"id"`
		EventType   string                 `json:"eventType"`   // "audit", "attestation", "capability"
		Action      string                 `json:"action"`      // create, update, verify, attest, revoke, etc.
		Timestamp   interface{}            `json:"timestamp"`
		ActorType   string                 `json:"actorType"`   // "user", "agent", "system"
		ActorID     *string                `json:"actorId,omitempty"`
		ActorName   string                 `json:"actorName,omitempty"`
		Description string                 `json:"description"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
		IPAddress   string                 `json:"ipAddress,omitempty"`
	}

	var timeline []TimelineEvent

	// 1. Get audit logs
	logs, _, err := h.auditService.GetAuditLogs(
		c.Context(),
		orgID,
		"",           // action filter (empty = all)
		"mcp_server", // entity_type
		&mcpServerID, // entity_id filter
		nil,          // user_id filter (nil = all users)
		nil,          // start_date
		nil,          // end_date
		100,          // Get more, we'll combine and re-sort
		0,
	)
	if err == nil && logs != nil {
		for _, log := range logs {
			actorType := "system"
			var actorID *string
			actorName := "System"

			if log.UserID != nil && *log.UserID != uuid.Nil {
				actorType = "user"
				idStr := log.UserID.String()
				actorID = &idStr
				if log.UserName != "" {
					actorName = log.UserName
				} else {
					actorName = "User"
				}
			} else if log.AgentID != nil && *log.AgentID != uuid.Nil {
				actorType = "agent"
				idStr := log.AgentID.String()
				actorID = &idStr
				if log.AgentName != "" {
					actorName = log.AgentName
				} else {
					actorName = "Agent"
				}
			}

			// Build description based on action
			description := h.buildAuditDescription(string(log.Action), mcpServer.Name, log.Metadata)

			timeline = append(timeline, TimelineEvent{
				ID:          log.ID.String(),
				EventType:   "audit",
				Action:      string(log.Action),
				Timestamp:   log.Timestamp,
				ActorType:   actorType,
				ActorID:     actorID,
				ActorName:   actorName,
				Description: description,
				Metadata:    log.Metadata,
				IPAddress:   log.IPAddress,
			})
		}
	}

	// 2. Get attestation events (if attestation service is available)
	if h.attestationService != nil {
		attestations, _, _, err := h.attestationService.GetMCPAttestations(c.Context(), mcpServerID)
		if err == nil && attestations != nil {
			for _, att := range attestations {
				actorType := "agent"
				actorID := att.AgentID.String()
				actorName := att.AgentName
				if actorName == "" {
					actorName = "Agent"
				}

				action := "attest"
				if !att.IsValid {
					action = "attestation_expired"
				}

				description := fmt.Sprintf("%s attested to %s", actorName, mcpServer.Name)
				if !att.IsValid {
					description = fmt.Sprintf("Attestation from %s expired", actorName)
				}

				metadata := map[string]interface{}{
					"agentTrustScore":     att.AgentTrustScore,
					"healthCheckPassed":   att.HealthCheckPassed,
					"connectionLatencyMs": att.ConnectionLatencyMs,
					"isValid":             att.IsValid,
				}
				if len(att.CapabilitiesConfirmed) > 0 {
					metadata["capabilitiesConfirmed"] = att.CapabilitiesConfirmed
				}

				timeline = append(timeline, TimelineEvent{
					ID:          att.ID.String(),
					EventType:   "attestation",
					Action:      action,
					Timestamp:   att.VerifiedAt,
					ActorType:   actorType,
					ActorID:     &actorID,
					ActorName:   actorName,
					Description: description,
					Metadata:    metadata,
				})
			}
		}
	}

	// 3. Get capability detection events (from capability service)
	if h.mcpCapabilityService != nil {
		capabilities, err := h.mcpCapabilityService.GetCapabilities(c.Context(), mcpServerID)
		if err == nil && capabilities != nil {
			for _, cap := range capabilities {
				description := fmt.Sprintf("Capability '%s' (%s) detected", cap.Name, cap.CapabilityType)

				timeline = append(timeline, TimelineEvent{
					ID:          cap.ID.String(),
					EventType:   "capability",
					Action:      "capability_detected",
					Timestamp:   cap.DetectedAt,
					ActorType:   "system",
					ActorName:   "MCP Discovery",
					Description: description,
					Metadata: map[string]interface{}{
						"capabilityName": cap.Name,
						"capabilityType": cap.CapabilityType,
						"description":    cap.Description,
						"isActive":       cap.IsActive,
					},
				})
			}
		}
	}

	// Sort by timestamp (most recent first) - using a simple bubble sort for clarity
	// In production, you might use sort.Slice with proper timestamp comparison
	for i := 0; i < len(timeline)-1; i++ {
		for j := 0; j < len(timeline)-i-1; j++ {
			// Compare timestamps - handle different timestamp types
			t1 := h.getTimestamp(timeline[j].Timestamp)
			t2 := h.getTimestamp(timeline[j+1].Timestamp)
			if t1.Before(t2) {
				timeline[j], timeline[j+1] = timeline[j+1], timeline[j]
			}
		}
	}

	// Apply pagination
	total := len(timeline)
	if offset >= total {
		timeline = []TimelineEvent{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		timeline = timeline[offset:end]
	}

	return c.JSON(fiber.Map{
		"logs":   timeline,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// buildAuditDescription creates a human-readable description for audit events
func (h *MCPHandler) buildAuditDescription(action, serverName string, metadata map[string]interface{}) string {
	switch action {
	case "create":
		return fmt.Sprintf("MCP server '%s' was registered", serverName)
	case "update":
		return fmt.Sprintf("MCP server '%s' was updated", serverName)
	case "delete":
		return fmt.Sprintf("MCP server '%s' was deleted", serverName)
	case "verify":
		return fmt.Sprintf("MCP server '%s' verification requested", serverName)
	case "attest":
		return fmt.Sprintf("Attestation submitted for '%s'", serverName)
	case "revoke":
		if reason, ok := metadata["reason"].(string); ok {
			return fmt.Sprintf("Attestation revoked: %s", reason)
		}
		return "Attestation revoked"
	default:
		return fmt.Sprintf("Action '%s' performed on '%s'", action, serverName)
	}
}

// getTimestamp extracts time.Time from various timestamp types
func (h *MCPHandler) getTimestamp(ts interface{}) time.Time {
	switch v := ts.(type) {
	case time.Time:
		return v
	case *time.Time:
		if v != nil {
			return *v
		}
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Time{}
}
