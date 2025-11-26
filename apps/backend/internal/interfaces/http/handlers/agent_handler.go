package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/sdkgen"
)

type AgentHandler struct {
	agentService              *application.AgentService
	mcpService                *application.MCPService
	auditService              *application.AuditService
	apiKeyService             *application.APIKeyService
	trustScoreHandler         *TrustScoreHandler
	alertService              *application.AlertService
	verificationEventService  *application.VerificationEventService
	capabilityService         *application.CapabilityService
}

func NewAgentHandler(
	agentService *application.AgentService,
	mcpService *application.MCPService,
	auditService *application.AuditService,
	apiKeyService *application.APIKeyService,
	trustScoreHandler *TrustScoreHandler,
	alertService *application.AlertService,
	verificationEventService *application.VerificationEventService,
	capabilityService *application.CapabilityService,
) *AgentHandler {
	return &AgentHandler{
		agentService:             agentService,
		mcpService:               mcpService,
		auditService:             auditService,
		apiKeyService:            apiKeyService,
		trustScoreHandler:        trustScoreHandler,
		alertService:             alertService,
		verificationEventService: verificationEventService,
		capabilityService:        capabilityService,
	}
}

func (h *AgentHandler) enrichAgentResponse(c fiber.Ctx, agent *domain.Agent) fiber.Map {
    // Fetch capabilities from agent_capabilities table
    capabilities, err := h.capabilityService.GetAgentCapabilities(c.Context(), agent.ID, true)
    if err != nil {
        // Log error but don't fail - return empty capabilities
        capabilities = []*domain.AgentCapability{}
    }

    // Extract capability types as simple string array (frontend compatible)
    capabilityTypes := make([]string, 0, len(capabilities))
    for _, cap := range capabilities {
        capabilityTypes = append(capabilityTypes, cap.CapabilityType)
    }

    // Return flat response with all agent fields + capabilities (camelCase for frontend)
    return fiber.Map{
        "id":                       agent.ID,
        "organizationId":           agent.OrganizationID,
        "name":                     agent.Name,
        "displayName":              agent.DisplayName,
        "description":              agent.Description,
        "agentType":                agent.AgentType,
        "status":                   agent.Status,
        "version":                  agent.Version,
        "publicKey":                agent.PublicKey,
        "trustScore":               agent.TrustScore,
        "verifiedAt":               agent.VerifiedAt,
        "createdAt":                agent.CreatedAt,
        "updatedAt":                agent.UpdatedAt,
        "talksTo":                  agent.TalksTo,
        "capabilities":             capabilityTypes,
        "capabilityViolationCount": agent.CapabilityViolationCount,
        "isCompromised":            agent.IsCompromised,
        "certificateUrl":           agent.CertificateURL,
        "repositoryUrl":            agent.RepositoryURL,
        "documentationUrl":         agent.DocumentationURL,
        "keyAlgorithm":             agent.KeyAlgorithm,
        "createdBy":                agent.CreatedBy,
        "lastActive":               agent.LastActive,
        "keyCreatedAt":             agent.KeyCreatedAt,
        "keyExpiresAt":             agent.KeyExpiresAt,
        "rotationCount":            agent.RotationCount,
    }
}



// ListAgents returns all agents for the organization
func (h *AgentHandler) ListAgents(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}
	enriched := make([]fiber.Map, 0, len(agents))
	for _, agent := range agents {
		enriched = append(enriched, h.enrichAgentResponse(c, agent))
	}
	return c.JSON(fiber.Map{
		"agents": enriched,
		"total":  len(enriched),
	})
}

// CreateAgent creates a new agent
func (h *AgentHandler) CreateAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req application.CreateAgentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	agent, err := h.agentService.CreateAgent(c.Context(), &req, orgID, userID)
	if err != nil {
		// Log the full error for debugging
		fmt.Printf("ERROR creating agent: %v\n", err)
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
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
			"agentType": agent.AgentType,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(agent)
}

// GetAgent returns a single agent
func (h *AgentHandler) GetAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	// Verify agent belongs to organization
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	return c.JSON(h.enrichAgentResponse(c, agent))
}

// UpdateAgent updates an agent
func (h *AgentHandler) UpdateAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req application.CreateAgentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify agent belongs to organization first
	existingAgent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if existingAgent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	agent, err := h.agentService.UpdateAgent(c.Context(), agentID, &req)
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
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
		},
	)

	return c.JSON(h.enrichAgentResponse(c, agent))
}

// DeleteAgent deletes an agent
func (h *AgentHandler) DeleteAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization first
	existingAgent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if existingAgent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.agentService.DeleteAgent(c.Context(), agentID); err != nil {
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
		"agent",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// VerifyAgent verifies an agent (admin/manager only)
func (h *AgentHandler) VerifyAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization first
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.agentService.VerifyAgent(c.Context(), agentID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated agent to return in response
	agent, _ = h.agentService.GetAgent(c.Context(), agentID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionVerify,
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":  agent.Name,
			"trustScore": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"verified":    true,
		"trustScore": agent.TrustScore,
		"verifiedAt": agent.VerifiedAt,
	})
}

// VerifyAction verifies if an agent can perform the requested action
// This is the CORE endpoint that agents call before every action
// @Summary Verify agent action authorization
// @Description Verify if an agent is authorized to perform a specific action based on its registered capabilities
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body VerifyActionRequest true "Action verification request"
// @Success 200 {object} VerifyActionResponse
// @Failure 403 {object} ErrorResponse "Action denied"
// @Router /agents/{id}/verify-action [post]
func (h *AgentHandler) VerifyAction(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		ActionType string                 `json:"action_type"`        // "read_file", "write_file", "execute_code", "network_request", "database_query"
		Resource   string                 `json:"resource"`           // e.g., "/data/file.csv" or "SELECT * FROM users"
		Metadata   map[string]interface{} `json:"metadata"`           // Additional context
		Protocol   *string                `json:"protocol,omitempty"` // Optional: "mcp", "a2a", "acp", "did", "oauth", "saml" - SDK auto-detects or user declares
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get agent and organization details for logging
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	orgID := agent.OrganizationID
	startTime := c.Context().Time()

	// Fetch agent and verify capabilities
	decision, reason, auditID, err := h.agentService.VerifyAction(
		c.Context(),
		agentID,
		req.ActionType,
		req.Resource,
		req.Metadata,
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Verification failed",
		})
	}

	// Calculate duration
	durationMs := int(c.Context().Time().Sub(startTime).Milliseconds())

	// 1. LOG AUDIT ENTRY (for all verification attempts)
	auditMetadata := map[string]interface{}{
		"action_type": req.ActionType,
		"resource":    req.Resource,
		"allowed":     decision,
		"reason":      reason,
		"audit_id":    auditID,
	}
	if req.Metadata != nil {
		auditMetadata["request_metadata"] = req.Metadata
	}

	userID := uuid.Nil // System action - no specific user
	if userIDLocal := c.Locals("user_id"); userIDLocal != nil {
		if uid, ok := userIDLocal.(uuid.UUID); ok {
			userID = uid
		}
	}

	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionVerify,
		"agent_action",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		auditMetadata,
	)

	// 2. RECORD VERIFICATION EVENT (for monitoring dashboard)
	verificationStatus := domain.VerificationEventStatusSuccess

	if !decision {
		verificationStatus = domain.VerificationEventStatusFailed
	}

	// Determine protocol: SDK auto-detects and sends protocol, or we default to MCP
	// Protocol is independent of talks_to (which tracks MCP server dependencies via tool calling)
	protocol := domain.VerificationProtocolMCP // Default to MCP
	if req.Protocol != nil && *req.Protocol != "" {
		// SDK provided protocol - trust the SDK's auto-detection or user's explicit declaration
		switch *req.Protocol {
		case "mcp":
			protocol = domain.VerificationProtocolMCP
		case "a2a":
			protocol = domain.VerificationProtocolA2A
		case "acp":
			protocol = domain.VerificationProtocolACP
		case "did":
			protocol = domain.VerificationProtocolDID
		case "oauth":
			protocol = domain.VerificationProtocolOAuth
		case "saml":
			protocol = domain.VerificationProtocolSAML
		}
	}

	h.verificationEventService.LogVerificationEvent(
		c.Context(),
		orgID,
		agentID,
		protocol, // SDK auto-detects protocol or user explicitly declares in secure()
		domain.VerificationTypeCapability,
		verificationStatus,
		durationMs,
		domain.InitiatorTypeAgent,
		nil, // No specific initiator ID for agent self-verification
		map[string]interface{}{
			"action_type": req.ActionType,
			"resource":    req.Resource,
			"allowed":     decision,
			"reason":      reason,
		},
	)

	// 3. CREATE SECURITY ALERT (only for capability violations)
	if !decision && (reason == "capability_not_granted" ||
		reason == "Agent has no granted capabilities - action denied (admin must grant capabilities first)" ||
		reason == "Agent has no granted capabilities - action denied") {

		alert := &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertSecurityBreach, // Using security_breach for capability violations
			Severity:       domain.AlertSeverityHigh,
			Title:          fmt.Sprintf("Capability Violation: %s attempted %s", agent.DisplayName, req.ActionType),
			Description: fmt.Sprintf("Agent '%s' attempted action '%s' on resource '%s' without required capability. Reason: %s",
				agent.DisplayName, req.ActionType, req.Resource, reason),
			ResourceType: "agent",
			ResourceID:   agentID,
		}

		// Create alert (non-blocking - don't fail the verification if alert creation fails)
		if err := h.alertService.CreateAlert(c.Context(), alert); err != nil {
			fmt.Printf("WARNING: Failed to create security alert for capability violation: %v\n", err)
		}
	}

	// 4. UPDATE AGENT LAST ACTIVE TIMESTAMP (for activity tracking)
	// Update last_active regardless of whether action was allowed or denied
	// This helps track when agents were last seen attempting actions
	fmt.Printf("🔄 Updating last_active for agent %s...\n", agentID)
	if err := h.agentService.UpdateLastActive(c.Context(), agentID); err != nil {
		// Log but don't fail the request if timestamp update fails
		fmt.Printf("❌ WARNING: Failed to update agent last_active: %v\n", err)
	} else {
		fmt.Printf("✅ Successfully updated last_active for agent %s\n", agentID)
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

// LogActionResult logs the outcome of an action that was verified
// @Summary Log action result
// @Description Log whether a verified action succeeded or failed
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param audit_id path string true "Audit ID from verification"
// @Param request body LogActionResultRequest true "Action result"
// @Success 200 {object} SuccessResponse
// @Router /agents/{id}/log-action/{audit_id} [post]
func (h *AgentHandler) LogActionResult(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	auditID, err := uuid.Parse(c.Params("audit_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid audit ID",
		})
	}

	var req struct {
		Success bool                   `json:"success"`
		Error   string                 `json:"error,omitempty"`
		Result  map[string]interface{} `json:"result,omitempty"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.agentService.LogActionResult(c.Context(), agentID, auditID, req.Success, req.Error, req.Result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to log action result",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// DownloadSDK generates and downloads SDK package with embedded credentials
// @Summary Download SDK for agent
// @Description Generate and download SDK package (Python, Node.js, or Go) with embedded credentials
// @Tags agents
// @Produce application/zip
// @Param id path string true "Agent ID"
// @Param lang query string false "SDK language (python, nodejs, go)" default(python)
// @Success 200 {file} binary "SDK package as zip file"
// @Failure 400 {object} ErrorResponse "Invalid agent ID or language"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/sdk [get]
func (h *AgentHandler) DownloadSDK(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Get SDK language (default: python)
	language := c.Query("lang", "python")
	if language != "python" && language != "nodejs" && language != "go" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid language. Supported: python, nodejs, go",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get agent credentials (decrypts private key)
	publicKey, privateKey, err := h.agentService.GetAgentCredentials(c.Context(), agentID)
	if err != nil {
		fmt.Println(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve agent credentials",
		})
	}

	// Generate SDK package based on language
	var sdkBytes []byte
	var filename string

	switch language {
	case "python":
		sdkBytes, err = sdkgen.GeneratePythonSDK(sdkgen.PythonSDKConfig{
			AgentID:    agentID.String(),
			PublicKey:  publicKey,
			PrivateKey: privateKey,
			AIMURL:     getAIMBaseURL(c),
			AgentName:  agent.Name,
			Version:    "1.0.0",
		})
		filename = fmt.Sprintf("aim-sdk-%s-python.zip", agent.Name)

	case "nodejs":
		// TODO: Implement Node.js SDK generator
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "Node.js SDK not yet implemented",
		})

	case "go":
		// TODO: Implement Go SDK generator
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "Go SDK not yet implemented",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate SDK",
		})
	}

	// Set response headers for file download
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Set("Content-Length", fmt.Sprintf("%d", len(sdkBytes)))

	// Log audit
	userID := c.Locals("user_id").(uuid.UUID)
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"agent_sdk",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"language":   language,
			"agentName": agent.Name,
		},
	)

	return c.Send(sdkBytes)
}

// GetCredentials returns the agent's cryptographic credentials (public and private keys)
// @Summary Get agent credentials
// @Description Retrieve Ed25519 public and private keys for an agent
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} CredentialsResponse
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/credentials [get]
func (h *AgentHandler) GetCredentials(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get agent credentials (decrypts private key)
	publicKey, privateKey, err := h.agentService.GetAgentCredentials(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve agent credentials",
		})
	}

	// Log audit - viewing credentials is a sensitive action
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"agent_credentials",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":    agentID.String(),
		"publicKey":  publicKey,
		"privateKey": privateKey,
	})
}

// getAIMBaseURL extracts the base URL from the request
func getAIMBaseURL(c fiber.Ctx) string {
	// Get protocol (http or https)
	protocol := "http"
	if c.Protocol() == "https" || c.Get("X-Forwarded-Proto") == "https" {
		protocol = "https"
	}

	// Get host
	host := c.Hostname()

	return fmt.Sprintf("%s://%s", protocol, host)
}

// ========================================
// MCP Server Relationship Management
// ========================================

// AddMCPServersToAgent adds MCP servers to an agent's talks_to list
// @Summary Add MCP servers to agent
// @Description Add MCP servers to an agent's allowed communication list (talks_to)
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body application.AddMCPServersRequest true "MCP servers to add"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /api/v1/agents/{id}/mcp-servers [put]
func (h *AgentHandler) AddMCPServersToAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// Support both JWT auth (user_id) and API key auth (no user_id)
	var userID uuid.UUID
	if userIDLocal := c.Locals("user_id"); userIDLocal != nil {
		userID = userIDLocal.(uuid.UUID)
	}
	// If no user_id (API key auth), we'll fetch it from the agent later

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse request body
	var req application.AddMCPServersRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if len(req.MCPServerIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mcp_server_ids is required and must not be empty",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Add MCP servers to agent's talks_to list
	updatedAgent, addedServers, err := h.agentService.AddMCPServers(
		c.Context(),
		agentID,
		req.MCPServerIDs,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// For API key auth (no user_id), use agent's creator for audit logging
	auditUserID := userID
	if auditUserID == uuid.Nil {
		auditUserID = agent.CreatedBy
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		auditUserID,
		domain.AuditActionUpdate,
		"agent",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":          "add_mcp_servers",
			"added_servers":   addedServers,
			"detected_method": req.DetectedMethod,
			"total_talks_to":  len(updatedAgent.TalksTo),
			"auth_method":     c.Locals("auth_method"), // API key or JWT
		},
	)

	return c.JSON(fiber.Map{
		"message":       fmt.Sprintf("Successfully added %d MCP server(s)", len(addedServers)),
		"talksTo":      updatedAgent.TalksTo,
		"added_servers": addedServers,
		"total_count":   len(updatedAgent.TalksTo),
	})
}

// RemoveMCPServerFromAgent removes a single MCP server from an agent's talks_to list
// @Summary Remove MCP server from agent
// @Description Remove a specific MCP server from an agent's allowed communication list
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param mcp_id path string true "MCP Server ID or name"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /api/v1/agents/{id}/mcp-servers/{mcp_id} [delete]
func (h *AgentHandler) RemoveMCPServerFromAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	mcpServerID := c.Params("mcp_id")
	if mcpServerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "MCP server ID is required",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Remove MCP server from agent's talks_to list
	updatedAgent, err := h.agentService.RemoveMCPServer(
		c.Context(),
		agentID,
		mcpServerID,
	)
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
		"agent",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":         "remove_mcp_server",
			"removed_server": mcpServerID,
			"total_talks_to": len(updatedAgent.TalksTo),
		},
	)

	return c.JSON(fiber.Map{
		"message":     "Successfully removed MCP server",
		"talksTo":    updatedAgent.TalksTo,
		"total_count": len(updatedAgent.TalksTo),
	})
}

// BulkRemoveMCPServersFromAgent removes multiple MCP servers from an agent's talks_to list
// @Summary Remove multiple MCP servers from agent
// @Description Remove multiple MCP servers from an agent's allowed communication list
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body map[string][]string true "MCP server IDs to remove"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// GetAgentMCPServers retrieves detailed information about MCP servers an agent talks to
// @Summary Get agent's MCP servers
// @Description Get detailed information about MCP servers this agent is allowed to communicate with
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /api/v1/agents/{id}/mcp-servers [get]
func (h *AgentHandler) GetAgentMCPServers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get MCP servers (need to pass MCP repository - we'll handle this in routing)
	// For now, return the talks_to array
	// TODO: Implement full server details lookup

	return c.JSON(fiber.Map{
		"agentId":   agentID.String(),
		"agentName": agent.Name,
		"talksTo":   agent.TalksTo,
		"total":      len(agent.TalksTo),
	})
}

// ========================================
// Auto-Detection of MCP Servers
// ========================================

// DetectAndMapMCPServers auto-detects MCP servers from Claude Desktop config and maps them to agent
// @Summary Auto-detect and map MCP servers
// @Description Automatically detect MCP servers from Claude Desktop config file and map them to agent's talks_to list
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body application.DetectMCPServersRequest true "Auto-detection configuration"
// @Success 200 {object} application.DetectMCPServersResult
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /api/v1/agents/{id}/mcp-servers/detect [post]
func (h *AgentHandler) DetectAndMapMCPServers(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse request body
	var req application.DetectMCPServersRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.ConfigPath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "config_path is required",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Call service with mcpService for auto-registration
	result, err := h.agentService.DetectMCPServersFromConfig(
		c.Context(),
		agentID,
		&req,
		h.mcpService, // ✅ Pass mcpService for auto-registration
		orgID,
		userID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	if !req.DryRun {
		h.auditService.LogAction(
			c.Context(),
			orgID,
			userID,
			domain.AuditActionUpdate,
			"agent",
			agentID,
			c.IP(),
			c.Get("User-Agent"),
			map[string]interface{}{
				"action":           "auto_detect_mcps",
				"detected_count":   len(result.DetectedServers),
				"registered_count": result.RegisteredCount,
				"mapped_count":     result.MappedCount,
				"config_path":      req.ConfigPath,
				"auto_register":    req.AutoRegister,
			},
		)
	}

	return c.JSON(result)
}

// GetAgentByIdentifier returns agent by ID or name (SDK API endpoint with API key auth)
// @Summary Get agent by ID or name
// @Description Get agent details by UUID or name. Works with API key authentication for SDK usage.
// @Tags sdk-api
// @Accept json
// @Produce json
// @Param identifier path string true "Agent ID (UUID) or name"
// @Success 200 {object} domain.Agent
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /api/v1/sdk-api/agents/{identifier} [get]
func (h *AgentHandler) GetAgentByIdentifier(c fiber.Ctx) error {
	// Get organization ID from API key middleware
	orgID := c.Locals("organization_id").(uuid.UUID)
	identifier := c.Params("identifier")

	if identifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Agent identifier (ID or name) is required",
		})
	}

	// Try to parse as UUID first
	agentID, err := uuid.Parse(identifier)
	var agent *domain.Agent

	if err == nil {
		// It's a UUID, get by ID
		agent, err = h.agentService.GetAgent(c.Context(), agentID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Agent not found",
			})
		}

		// Verify agent belongs to the organization
		if agent.OrganizationID != orgID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Agent does not belong to your organization",
			})
		}
	} else {
		// It's a name, get by name
		agent, err = h.agentService.GetAgentByName(c.Context(), orgID, identifier)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Agent not found",
				"message": fmt.Sprintf("No agent found with name '%s' in your organization", identifier),
			})
		}
	}
	capabilities, err := h.capabilityService.GetAgentCapabilities(c.Context(), agent.ID, true)
	if err != nil {
		capabilities = []*domain.AgentCapability{}
	}
	// Return agent details (excluding sensitive private key)
	return c.JSON(fiber.Map{
		"success": true,
		"agent": fiber.Map{
			"id":                         agent.ID,
			"organizationId":            agent.OrganizationID,
			"name":                       agent.Name,
			"displayName":               agent.DisplayName,
			"description":                agent.Description,
			"agentType":                 agent.AgentType,
			"status":                     agent.Status,
			"version":                    agent.Version,
			"publicKey":                 agent.PublicKey,
			"trustScore":                agent.TrustScore,
			"verifiedAt":                agent.VerifiedAt,
			"createdAt":                 agent.CreatedAt,
			"updatedAt":                 agent.UpdatedAt,
			"keyAlgorithm":              agent.KeyAlgorithm,
			"keyCreatedAt":             agent.KeyCreatedAt,
			"keyExpiresAt":             agent.KeyExpiresAt,
			"rotationCount":             agent.RotationCount,
			"talksTo":                   agent.TalksTo,
			"capabilities":               capabilities,
			"capabilityViolationCount": agent.CapabilityViolationCount,
			"isCompromised":             agent.IsCompromised,
		},
	})
}

// ========================================
// Trust Score Management (RESTful nesting under /agents/:id/trust-score/*)
// ========================================

// GetAgentTrustScore returns current trust score for an agent
// Wrapper that delegates to TrustScoreHandler for RESTful endpoint consistency
// @Summary Get agent trust score
// @Description Get current trust score for an agent
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/trust-score [get]
func (h *AgentHandler) GetAgentTrustScore(c fiber.Ctx) error {
	// Delegate to existing trust score handler
	return h.trustScoreHandler.GetTrustScore(c)
}

// GetAgentTrustScoreHistory returns trust score history for an agent
// Wrapper that delegates to TrustScoreHandler for RESTful endpoint consistency
// @Summary Get agent trust score history
// @Description Get trust score changes over time for an agent
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of history entries to return (default: 30)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/trust-score/history [get]
func (h *AgentHandler) GetAgentTrustScoreHistory(c fiber.Ctx) error {
	// Delegate to existing trust score handler
	return h.trustScoreHandler.GetTrustScoreHistory(c)
}

// UpdateAgentTrustScore manually updates trust score (admin override)
// @Summary Update agent trust score (admin only)
// @Description Manually override the trust score for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body UpdateTrustScoreRequest true "New trust score"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/trust-score [put]
func (h *AgentHandler) UpdateAgentTrustScore(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse request body
	var req struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate score range (0.000 to 9.999 based on database schema)
	if req.Score < 0.0 || req.Score > 9.999 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Trust score must be between 0.0 and 9.999",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Update trust score in database using agent repository
	if err := h.agentService.UpdateTrustScore(c.Context(), agentID, req.Score); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update trust score",
		})
	}

	// Get updated agent
	updatedAgent, _ := h.agentService.GetAgent(c.Context(), agentID)

	// Log audit - manual trust score override is a sensitive admin action
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"trust_score",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":      agent.Name,
			"oldTrustScore": agent.TrustScore,
			"newTrustScore": req.Score,
			"reason":          req.Reason,
			"action":          "manual_override",
		},
	)

	return c.JSON(fiber.Map{
		"success":     true,
		"agentId":    agentID,
		"agentName":  updatedAgent.Name,
		"trustScore": updatedAgent.TrustScore,
		"updatedAt":  updatedAgent.UpdatedAt,
		"message":     "Trust score updated successfully",
	})
}

// RecalculateAgentTrustScore triggers trust score recalculation
// Wrapper that delegates to TrustScoreHandler.CalculateTrustScore
// @Summary Recalculate agent trust score
// @Description Trigger recalculation of trust score based on current metrics
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/trust-score/recalculate [post]
func (h *AgentHandler) RecalculateAgentTrustScore(c fiber.Ctx) error {
	// Delegate to existing trust score handler
	return h.trustScoreHandler.CalculateTrustScore(c)
}

// ========================================
// Agent Lifecycle Management
// ========================================

// SuspendAgent suspends an agent by setting its status to suspended
// @Summary Suspend agent
// @Description Suspend an agent by setting its status to suspended. The agent will be unable to perform actions.
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/suspend [post]
func (h *AgentHandler) SuspendAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization first
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Suspend the agent
	if err := h.agentService.SuspendAgent(c.Context(), agentID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated agent to return in response
	agent, _ = h.agentService.GetAgent(c.Context(), agentID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":      "suspend",
			"agentName":  agent.Name,
			"status":      agent.Status,
			"trustScore": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"success":     true,
		"message":     "Agent suspended successfully",
		"status":      agent.Status,
		"trustScore": agent.TrustScore,
		"agent":       agent,
	})
}

// ReactivateAgent reactivates a suspended agent by setting its status to verified
// @Summary Reactivate agent
// @Description Reactivate a suspended agent by setting its status to verified. The agent will be able to perform actions again.
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/reactivate [post]
func (h *AgentHandler) ReactivateAgent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization first
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Reactivate the agent
	if err := h.agentService.ReactivateAgent(c.Context(), agentID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated agent to return in response
	agent, _ = h.agentService.GetAgent(c.Context(), agentID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":      "reactivate",
			"agentName":  agent.Name,
			"status":      agent.Status,
			"trustScore": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"success":     true,
		"message":     "Agent reactivated successfully",
		"status":      agent.Status,
		"trustScore": agent.TrustScore,
		"verifiedAt": agent.VerifiedAt,
		"agent":       agent,
	})
}

// RotateCredentials rotates an agent's cryptographic credentials by generating new Ed25519 keypair
// @Summary Rotate agent credentials
// @Description Generate new Ed25519 keypair for agent. Previous public key is stored for grace period.
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/rotate-credentials [post]
func (h *AgentHandler) RotateCredentials(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization first
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Rotate credentials (generates new keypair)
	publicKey, privateKey, err := h.agentService.RotateCredentials(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated agent to return in response
	agent, _ = h.agentService.GetAgent(c.Context(), agentID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":         "rotate_credentials",
			"agentName":     agent.Name,
			"rotationCount": agent.RotationCount,
			"keyCreatedAt": agent.KeyCreatedAt,
			"keyExpiresAt": agent.KeyExpiresAt,
		},
	)

	return c.JSON(fiber.Map{
		"success":             true,
		"message":             "Credentials rotated successfully",
		"publicKey":          publicKey,
		"private_key":         privateKey, // ⚠️ SENSITIVE: Only returned once during rotation
		"previous_public_key": agent.PreviousPublicKey,
		"rotationCount":      agent.RotationCount,
		"keyCreatedAt":      agent.KeyCreatedAt,
		"keyExpiresAt":      agent.KeyExpiresAt,
		"warning":             "Store the private key securely. It will not be shown again.",
	})
}

// UpdateAgentKeys allows SDK to register its own public key
// @Summary Update agent public key
// @Description Register or update an agent's public key. Used by SDK during initialization.
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param body body object true "Public key to register"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/keys [put]
func (h *AgentHandler) UpdateAgentKeys(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse request body
	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.PublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "public_key is required",
		})
	}

	// Verify agent belongs to organization first
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Update public key
	if err := h.agentService.UpdateAgentPublicKey(c.Context(), agentID, req.PublicKey); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated agent
	agent, _ = h.agentService.GetAgent(c.Context(), agentID)

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent",
		agent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":         "update_public_key",
			"agentName":     agent.Name,
			"rotationCount": agent.RotationCount,
			"keyCreatedAt": agent.KeyCreatedAt,
		},
	)

	return c.JSON(fiber.Map{
		"success":             true,
		"message":             "Public key updated successfully",
		"publicKey":          agent.PublicKey,
		"previous_public_key": agent.PreviousPublicKey,
		"rotationCount":      agent.RotationCount,
		"keyCreatedAt":      agent.KeyCreatedAt,
		"keyExpiresAt":      agent.KeyExpiresAt,
	})
}
