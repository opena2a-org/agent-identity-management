package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/sdkgen"
)

type AgentHandler struct {
	agentService             AgentServicer
	mcpService               MCPServicer
	auditService             AuditServicer
	apiKeyService            APIKeyServicer
	trustScoreHandler        *TrustScoreHandler
	alertService             AlertServicer
	verificationEventService VerificationEventServicer
	capabilityService        CapabilityServicer
	tagService               TagServicer                          // ✅ For fetching agent tags in responses
	orgRepo                  domain.OrganizationRepository        // ✅ For enforcement mode lookup
	attestationService       MCPAttestationServicer               // ✅ For getting MCP connections via attestations
	mcpServiceConcrete       *application.MCPService              // Needed for DetectMCPServersFromConfig
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
	tagService *application.TagService,
	orgRepo domain.OrganizationRepository, // ✅ For enforcement mode lookup
	attestationService *application.MCPAttestationService, // ✅ For MCP connections via attestations
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
		tagService:               tagService,
		orgRepo:                  orgRepo,
		attestationService:       attestationService,
		mcpServiceConcrete:       mcpService, // Store concrete for DetectMCPServersFromConfig
	}
}

// NewAgentHandlerWithInterfaces creates an AgentHandler with interface-based dependencies for testing
func NewAgentHandlerWithInterfaces(
	agentService AgentServicer,
	mcpService MCPServicer,
	auditService AuditServicer,
	apiKeyService APIKeyServicer,
	trustScoreHandler *TrustScoreHandler,
	alertService AlertServicer,
	verificationEventService VerificationEventServicer,
	capabilityService CapabilityServicer,
	tagService TagServicer,
	orgRepo domain.OrganizationRepository,
	attestationService MCPAttestationServicer,
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
		tagService:               tagService,
		orgRepo:                  orgRepo,
		attestationService:       attestationService,
		mcpServiceConcrete:       nil, // Not available when using interfaces
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

	// ✅ Fetch tags from agent_tags table
	tags := make([]fiber.Map, 0)
	if h.tagService != nil {
		agentTags, err := h.tagService.GetAgentTags(c.Context(), agent.ID)
		if err != nil {
			// Log error but don't fail - return empty tags
			tags = []fiber.Map{}
		} else {
			tags = make([]fiber.Map, 0, len(agentTags))
			for _, tag := range agentTags {
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

	// Return flat response with all agent fields + capabilities + tags (camelCase for frontend)
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
		"tags":                     tags,            // ✅ Include tags in response
		"metadata":                 agent.Metadata,  // ✅ Include custom metadata (model, department, etc.)
		"capabilityViolationCount": agent.CapabilityViolationCount,
		"isCompromised":            agent.IsCompromised,
		"certificateUrl":           agent.CertificateURL,
		"repositoryUrl":            agent.RepositoryURL,
		"documentationUrl":         agent.DocumentationURL,
		"keyAlgorithm":             agent.KeyAlgorithm,
		"createdBy":                agent.CreatedBy,
		"createdByName":            agent.CreatedByName,       // ✅ Audit trail: who created this agent
		"createdByEmail":           agent.CreatedByEmail,      // ✅ Audit trail: creator's email
		"createdBySdkTokenId":      agent.CreatedBySDKTokenID, // ✅ SDK token tracking for revocation
		"updatedBy":                agent.UpdatedBy,           // ✅ Audit trail: who last updated
		"updatedByName":            agent.UpdatedByName,       // ✅ Audit trail: updater's name
		"updatedByEmail":           agent.UpdatedByEmail,      // ✅ Audit trail: updater's email
		"lastActive":               agent.LastActive,
		"keyCreatedAt":             agent.KeyCreatedAt,
		"keyExpiresAt":             agent.KeyExpiresAt,
		"rotationCount":            agent.RotationCount,
		"pqcPublicKey":             agent.PQCPublicKey,
		"pqcKeyAlgorithm":          agent.PQCKeyAlgorithm,
		"hybridModeEnabled":        agent.HybridModeEnabled,
		"pqcKeyCreatedAt":          agent.PQCKeyCreatedAt,
		"pqcKeyExpiresAt":          agent.PQCKeyExpiresAt,
	}
}

// ListAgents returns all agents for the organization
func (h *AgentHandler) ListAgents(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

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

// CreateAgent creates a new agent and automatically generates an API key for it
// This is a streamlined 1-step process: create agent + create API key in one operation
func (h *AgentHandler) CreateAgent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	// ✅ Extract user email from JWT claims (set by AuthMiddleware)
	// This is used as a fallback for audit trail if user lookup fails
	userEmail, _ := c.Locals("email").(string)

	// ✅ Extract SDK token ID if present (set by SDKTokenTrackingMiddleware)
	// This enables tracking which SDK token was used to create this agent
	var sdkTokenID *uuid.UUID
	if sdkTokenIDStr, ok := c.Locals("sdk_token_id").(string); ok && sdkTokenIDStr != "" {
		if parsed, err := uuid.Parse(sdkTokenIDStr); err == nil {
			sdkTokenID = &parsed
		}
	}

	// ✅ Extract API key ID if present (set by APIKeyMiddleware)
	// This enables tracking which API key was used to create this agent
	var apiKeyID *uuid.UUID
	if apiKeyIDLocal := c.Locals("api_key_id"); apiKeyIDLocal != nil {
		if apiKeyIDVal, ok := apiKeyIDLocal.(uuid.UUID); ok {
			apiKeyID = &apiKeyIDVal
		}
	}

	var req application.CreateAgentRequest
	// Use flexible JSON unmarshaling to accept both camelCase and snake_case
	if err := utils.UnmarshalFlexibleJSON(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY: No error logging to prevent information leakage
	agent, err := h.agentService.CreateAgent(c.Context(), &req, orgID, userID, sdkTokenID, apiKeyID, userEmail)
	if err != nil {
		// Return 409 Conflict for duplicate agent name instead of leaking database internals
		if err.Error() == "an agent with this name already exists" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// ✅ AUTO-CREATE API KEY: Only for non-SDK registrations
	// SDK-registered agents authenticate via SDK token, so they don't need per-agent API keys
	// For dashboard/API registrations, auto-generate a key for immediate use
	var fullAPIKey string
	var apiKeyRecord *domain.APIKey
	var apiKeyErr error
	apiKeyCreated := false

	// Check if this is an SDK registration (set by SDKTokenTrackingMiddleware)
	isSDKRegistration, _ := c.Locals("is_sdk_registration").(bool)

	if !isSDKRegistration && sdkTokenID == nil {
		// Non-SDK registration: auto-create API key for convenience
		apiKeyName := fmt.Sprintf("%s-default-key", agent.Name)
		fullAPIKey, apiKeyRecord, apiKeyErr = h.apiKeyService.GenerateAPIKey(
			c.Context(),
			agent.ID,
			orgID,
			userID,
			apiKeyName,
			365, // Default: 1 year expiration
		)
		apiKeyCreated = apiKeyErr == nil
	}

	// Log audit for agent creation
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
			"agentName":       agent.Name,
			"agentType":       agent.AgentType,
			"apiKeyCreated":   apiKeyCreated,
			"sdkRegistration": sdkTokenID != nil,
		},
	)

	// Build response with agent details and API key
	response := h.enrichAgentResponse(c, agent)

	// ✅ Include API key in response (only shown once!) - only for non-SDK registrations
	if sdkTokenID == nil {
		if apiKeyErr == nil && fullAPIKey != "" {
			response["apiKey"] = fiber.Map{
				"key":       fullAPIKey,           // ⚠️ SENSITIVE: Only shown once, never stored in plaintext
				"id":        apiKeyRecord.ID,
				"name":      apiKeyRecord.Name,
				"prefix":    apiKeyRecord.Prefix,
				"expiresAt": apiKeyRecord.ExpiresAt,
				"createdAt": apiKeyRecord.CreatedAt,
			}
			response["apiKeyWarning"] = "Store this API key securely. It will never be shown again!"
		} else if apiKeyErr != nil {
			// API key creation failed, but agent was created successfully
			// User can create API key manually from the dashboard
			response["apiKeyError"] = "API key generation failed. You can create one manually from the API Keys page."
		}
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// GetAgent returns a single agent
func (h *AgentHandler) GetAgent(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	return c.JSON(h.enrichAgentResponse(c, agent))
}

// UpdateAgent updates an agent
func (h *AgentHandler) UpdateAgent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req application.CreateAgentRequest
	// Use flexible JSON unmarshaling to accept both camelCase and snake_case
	if err := utils.UnmarshalFlexibleJSON(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	agent, err := h.agentService.UpdateAgent(c.Context(), agentID, &req, userID)
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
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
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
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
		"verified":   true,
		"trustScore": agent.TrustScore,
		"verifiedAt": agent.VerifiedAt,
	})
}

// VerifyCapability verifies if an agent can perform the requested capability
// This is the CORE endpoint that agents call before performing sensitive operations
// @Summary Verify agent capability authorization
// @Description Verify if an agent is authorized to use a specific capability based on its registered capabilities
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body VerifyCapabilityRequest true "Capability verification request"
// @Success 200 {object} VerifyCapabilityResponse
// @Failure 403 {object} ErrorResponse "Capability denied"
// @Router /agents/{id}/verify-capability [post]
func (h *AgentHandler) VerifyCapability(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		Capability string                 `json:"capability"`          // "file:read", "file:write", "code:execute", "api:call", "db:query"
		Resource   string                 `json:"resource"`            // e.g., "/data/file.csv" or "SELECT * FROM users"
		Metadata   map[string]interface{} `json:"metadata"`            // Additional context
		Protocol   *string                `json:"protocol,omitempty"`  // Optional: "mcp", "a2a", "acp", "did", "oauth", "saml" - SDK auto-detects or user declares
		RiskLevel  *string                `json:"riskLevel,omitempty"` // Optional: "low", "medium", "high", "critical" - auto-detected if not provided
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Auto-detect risk level from capability if not explicitly provided
	var riskOverride string
	if req.RiskLevel != nil {
		riskOverride = *req.RiskLevel
	}
	detectedRiskLevel := domain.DetectRiskLevel(req.Capability, riskOverride)
	riskAutoDetected := req.RiskLevel == nil || *req.RiskLevel == ""

	// SECURITY (HIGH cross-tenant IDOR): the pre-fix code derived
	// orgID from the LOADED agent (`orgID := agent.OrganizationID`),
	// meaning an attacker with any /agents-group auth could POST to
	// /agents/<victim-org-agent>/verify-capability and the handler
	// would run the verification + write audit-log + verification-event
	// + alert rows into the VICTIM org. Cross-tenant pollution and
	// capability-decision leak. The lint passed because the body
	// mentions OrganizationID — but only as a victim-side read, not
	// as a caller-side check (documented blindspot, see memory
	// feedback_adversarial_subagent_caught_lint_blindspot).
	//
	// Post-fix: derive orgID from caller context, then LoadOwned
	// verifies the agent belongs to caller's org BEFORE any service
	// call. The downstream audit/event/alert writes still use orgID
	// (now the caller's, which matches agent.OrganizationID after
	// LoadOwned's check). Both cross-tenant and not-found collapse
	// to a fixed 404.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}
	startTime := time.Now()

	// Fetch agent and verify capabilities
	decision, reason, auditID, err := h.agentService.VerifyCapability(
		c.Context(),
		agentID,
		req.Capability,
		req.Resource,
		req.Metadata,
		c.IP(), // Capture source IP for security tracking
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Verification failed",
		})
	}

	// Calculate duration
	durationMs := int(time.Since(startTime).Milliseconds())

	// 1. LOG AUDIT ENTRY (for all verification attempts)
	auditMetadata := map[string]interface{}{
		"capability":       req.Capability,
		"resource":         req.Resource,
		"allowed":          decision,
		"reason":           reason,
		"auditId":          auditID,
		"riskLevel":        detectedRiskLevel,
		"riskAutoDetected": riskAutoDetected,
	}
	if req.Metadata != nil {
		auditMetadata["requestMetadata"] = req.Metadata
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
		"agent_capability",
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
			"capability":       req.Capability,
			"resource":         req.Resource,
			"allowed":          decision,
			"reason":           reason,
			"riskLevel":        detectedRiskLevel,
			"riskAutoDetected": riskAutoDetected,
		},
	)

	// 3. CREATE SECURITY ALERT (only for capability violations)
	if !decision && (reason == "capability_not_granted" ||
		reason == "Agent has no granted capabilities - action denied (admin must grant capabilities first)" ||
		reason == "Agent has no granted capabilities - action denied") {

		// Map detected risk level to alert severity
		// Higher risk capabilities result in higher severity alerts when violated
		alertSeverity := domain.AlertSeverityWarning // Default
		switch detectedRiskLevel {
		case domain.RiskLevelLow:
			alertSeverity = domain.AlertSeverityInfo
		case domain.RiskLevelMedium:
			alertSeverity = domain.AlertSeverityWarning
		case domain.RiskLevelHigh:
			alertSeverity = domain.AlertSeverityHigh
		case domain.RiskLevelCritical:
			alertSeverity = domain.AlertSeverityCritical
		}

		alert := &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertSecurityBreach, // Using security_breach for capability violations
			Severity:       alertSeverity,              // Severity based on risk level of attempted action
			Title:          fmt.Sprintf("Capability Violation: %s attempted %s", agent.DisplayName, req.Capability),
			Description: fmt.Sprintf("Agent '%s' attempted capability '%s' (risk: %s) on resource '%s' without authorization. Reason: %s",
				agent.DisplayName, req.Capability, detectedRiskLevel, req.Resource, reason),
			ResourceType: "agent",
			ResourceID:   agentID,
			AgentName:    agent.DisplayName, // Denormalized for display in alerts
		}

		// Create alert (non-blocking - don't fail the verification if alert creation fails)
		// SECURITY: No error logging to prevent information leakage
		_ = h.alertService.CreateAlert(c.Context(), alert)
	}

	// 4. UPDATE AGENT LAST ACTIVE TIMESTAMP (for activity tracking)
	// Update last_active regardless of whether action was allowed or denied
	// SECURITY: No logging to prevent information leakage
	_ = h.agentService.UpdateLastActive(c.Context(), agentID)

	// 5. GET ENFORCEMENT MODE from organization settings
	// This tells the SDK whether to block or allow execution on denial
	enforcementMode := "monitoring" // Safe default
	if h.orgRepo != nil {
		org, err := h.orgRepo.GetByID(orgID)
		if err == nil && org != nil && org.EnforcementMode != "" {
			enforcementMode = string(org.EnforcementMode)
		}
	}

	if !decision {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"allowed":          false,
			"reason":           reason,
			"auditId":          auditID,
			"enforcementMode":  enforcementMode,
			"riskLevel":        detectedRiskLevel,
			"riskAutoDetected": riskAutoDetected,
		})
	}

	return c.JSON(fiber.Map{
		"allowed":          true,
		"reason":           reason,
		"auditId":          auditID,
		"enforcementMode":  enforcementMode,
		"riskLevel":        detectedRiskLevel,
		"riskAutoDetected": riskAutoDetected,
	})
}

// LogCapabilityResult logs the outcome of a capability that was verified
// @Summary Log capability result
// @Description Log whether a verified capability usage succeeded or failed
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param audit_id path string true "Audit ID from verification"
// @Param request body LogCapabilityResultRequest true "Capability result"
// @Success 200 {object} SuccessResponse
// @Router /agents/{id}/log-capability/{audit_id} [post]
func (h *AgentHandler) LogCapabilityResult(c fiber.Ctx) error {
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

	// SECURITY: pre-fix handler called h.agentService.LogCapabilityResult
	// with the path-supplied agentID + auditID and no caller-org check.
	// LoadOwned now confirms the target agent belongs to caller's org;
	// the service call only runs for caller-owned agents, preventing
	// cross-tenant audit-result pollution.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	if err := h.agentService.LogCapabilityResult(c.Context(), agentID, auditID, req.Success, req.Error, req.Result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to log capability result",
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

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

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Get agent credentials (decrypts private key)
	// SECURITY: No error logging to prevent credential-related information leakage
	publicKey, privateKey, err := h.agentService.GetAgentCredentials(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve agent credentials",
		})
	}

	// Generate SDK package based on language
	sdkBytes := make([]byte, 0)
	var filename string

	// SDK version - keep in sync with sdk/python/VERSION and sdk/python/aim_sdk/__init__.py
	const sdkVersion = "1.6.0"

	switch language {
	case "python":
		sdkBytes, err = sdkgen.GeneratePythonSDK(sdkgen.PythonSDKConfig{
			AgentID:    agentID.String(),
			PublicKey:  publicKey,
			PrivateKey: privateKey,
			AIMURL:     getAIMBaseURL(c),
			AgentName:  agent.Name,
			Version:    sdkVersion,
		})
		filename = fmt.Sprintf("aim-sdk-python-v%s.zip", sdkVersion)

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

	// Log audit (userID is optional for SDK download - could be API key auth)
	userID, _ := GetUserID(c)
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
			"language":  language,
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
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/credentials [get]
func (h *AgentHandler) GetCredentials(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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

	// normalizeAIMURL forces https for any public host so embedded agent
	// credentials can never point the SDK at an http:// endpoint that 301s and
	// strips the refresh-token POST body behind a TLS-terminating ingress.
	return normalizeAIMURL(fmt.Sprintf("%s://%s", protocol, host))
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// Support both JWT auth (user_id) and API key auth (no user_id)
	// userID is optional - GetUserID returns uuid.Nil if not present
	userID, _ := GetUserID(c)

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

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
		"talksTo":       updatedAgent.TalksTo,
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
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

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

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
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
		"talksTo":     updatedAgent.TalksTo,
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Return MCP server details with connection metadata
	type MCPServerSummary struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description,omitempty"`
		Status       string   `json:"status"`
		URL          string   `json:"url,omitempty"`
		Source       string   `json:"source,omitempty"`       // "attestation" or "talks_to"
		Capabilities []string `json:"capabilities,omitempty"` // Cached capabilities
	}

	// Use a map to deduplicate by ID
	mcpServerMap := make(map[string]MCPServerSummary)

	// PRIORITY 1: Get MCP servers from agent_mcp_connections table (attestations)
	// This is the primary source of truth for agent-MCP relationships
	if h.attestationService != nil {
		attestedServers, err := h.attestationService.GetMCPServersForAgent(c.Context(), agentID)
		if err == nil {
			for _, server := range attestedServers {
				if server != nil && server.OrganizationID == orgID {
					mcpServerMap[server.ID.String()] = MCPServerSummary{
						ID:           server.ID.String(),
						Name:         server.Name,
						Description:  server.Description,
						Status:       string(server.Status),
						URL:          server.URL,
						Source:       "attestation",
						Capabilities: server.Capabilities,
					}
				}
			}
		}
	}

	// PRIORITY 2: Also include MCP servers from agent.TalksTo (legacy/manual mapping)
	// This provides backwards compatibility for agents mapped via UI
	// Build a set of names already added from attestations to avoid duplicates
	existingNames := make(map[string]bool)
	for _, summary := range mcpServerMap {
		existingNames[summary.Name] = true
	}

	for _, entry := range agent.TalksTo {
		// Skip if we already have a server with this name from attestations
		if existingNames[entry] {
			continue
		}

		// Try to parse as UUID first
		if serverID, parseErr := uuid.Parse(entry); parseErr == nil {
			// Skip if already added from attestations
			if _, exists := mcpServerMap[serverID.String()]; exists {
				continue
			}
			// It's a UUID, look up by ID
			server, lookupErr := h.mcpService.GetMCPServer(c.Context(), serverID)
			if lookupErr == nil && server != nil && server.OrganizationID == orgID {
				// Skip if name already exists
				if existingNames[server.Name] {
					continue
				}
				mcpServerMap[server.ID.String()] = MCPServerSummary{
					ID:           server.ID.String(),
					Name:         server.Name,
					Description:  server.Description,
					Status:       string(server.Status),
					URL:          server.URL,
					Source:       "talks_to",
					Capabilities: server.Capabilities,
				}
				existingNames[server.Name] = true
			}
		} else {
			// It's a name, look up all servers and find by name
			servers, listErr := h.mcpService.ListMCPServers(c.Context(), orgID)
			if listErr == nil {
				for _, server := range servers {
					if server.Name == entry {
						// Skip if already added by ID or name
						if _, exists := mcpServerMap[server.ID.String()]; exists {
							continue
						}
						if existingNames[server.Name] {
							continue
						}
						mcpServerMap[server.ID.String()] = MCPServerSummary{
							ID:           server.ID.String(),
							Name:         server.Name,
							Description:  server.Description,
							Status:       string(server.Status),
							URL:          server.URL,
							Source:       "talks_to",
							Capabilities: server.Capabilities,
						}
						existingNames[server.Name] = true
						break
					}
				}
			}
		}
	}

	// Convert map to slice
	mcpServers := make([]MCPServerSummary, 0, len(mcpServerMap))
	for _, server := range mcpServerMap {
		mcpServers = append(mcpServers, server)
	}

	return c.JSON(fiber.Map{
		"mcpServers": mcpServers,
		"total":      len(mcpServers),
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
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

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

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Call service with mcpService for auto-registration
	result, err := h.agentService.DetectMCPServersFromConfig(
		c.Context(),
		agentID,
		&req,
		h.mcpServiceConcrete, // ✅ Pass mcpService for auto-registration
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
	// Get organization ID from middleware (API key, Ed25519, or JWT)
	orgIDLocal := c.Locals("organization_id")
	if orgIDLocal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required - organization ID not found in context",
		})
	}
	orgID, ok := orgIDLocal.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID format in context",
		})
	}
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
		// It's a UUID, get by ID.
		// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via
		// LoadOwned. Returns 404 — not 403 — for cross-tenant access to
		// avoid leaking resource existence across orgs. The name branch
		// below uses GetAgentByName which already filters by orgID, so a
		// cross-org name lookup naturally returns "not found" with no
		// existence side channel either way.
		loader := func(id uuid.UUID) (*domain.Agent, error) {
			return h.agentService.GetAgent(c.Context(), id)
		}
		agent = LoadOwned(c, loader, agentID, orgID, agentOrgID)
		if agent == nil {
			return nil
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
			"keyAlgorithm":             agent.KeyAlgorithm,
			"keyCreatedAt":             agent.KeyCreatedAt,
			"keyExpiresAt":             agent.KeyExpiresAt,
			"rotationCount":            agent.RotationCount,
			"talksTo":                  agent.TalksTo,
			"capabilities":             capabilities,
			"capabilityViolationCount": agent.CapabilityViolationCount,
			"isCompromised":            agent.IsCompromised,
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

// GetAgentAlerts returns alerts for a specific agent
// @Summary Get agent alerts
// @Description Get security and operational alerts for a specific agent
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of alerts to return (default: 50)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/alerts [get]
func (h *AgentHandler) GetAgentAlerts(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Parse pagination params
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	// Get alerts for this agent
	alerts, err := h.alertService.GetAlertsByAgent(c.Context(), agentID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch alerts",
		})
	}

	return c.JSON(fiber.Map{
		"alerts":  alerts,
		"agentId": agentID,
		"limit":   limit,
		"offset":  offset,
	})
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
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

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

	// Validate score range (0-100 matching calculateInitialTrustScore and DB schema DECIMAL(5,2))
	if req.Score < 0.0 || req.Score > 100.0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Trust score must be between 0.0 and 100.0",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Update trust score in database using agent repository
	if err := h.agentService.UpdateTrustScore(c.Context(), agentID, req.Score); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update trust score",
		})
	}

	// Get updated agent
	updatedAgent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil || updatedAgent == nil {
		// Update succeeded but re-fetch failed; return the requested score
		return c.JSON(fiber.Map{
			"success":    true,
			"agentId":    agentID,
			"agentName":  agent.Name,
			"trustScore": req.Score,
			"message":    "Trust score updated successfully",
		})
	}

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
			"agentName":     agent.Name,
			"oldTrustScore": agent.TrustScore,
			"newTrustScore": req.Score,
			"reason":        req.Reason,
			"action":        "manual_override",
		},
	)

	return c.JSON(fiber.Map{
		"success":    true,
		"agentId":    agentID,
		"agentName":  updatedAgent.Name,
		"trustScore": updatedAgent.TrustScore,
		"updatedAt":  updatedAgent.UpdatedAt,
		"message":    "Trust score updated successfully",
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
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/suspend [post]
func (h *AgentHandler) SuspendAgent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
			"action":     "suspend",
			"agentName":  agent.Name,
			"status":     agent.Status,
			"trustScore": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    "Agent suspended successfully",
		"status":     agent.Status,
		"trustScore": agent.TrustScore,
		"agent":      agent,
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
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/reactivate [post]
func (h *AgentHandler) ReactivateAgent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
			"action":     "reactivate",
			"agentName":  agent.Name,
			"status":     agent.Status,
			"trustScore": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    "Agent reactivated successfully",
		"status":     agent.Status,
		"trustScore": agent.TrustScore,
		"verifiedAt": agent.VerifiedAt,
		"agent":      agent,
	})
}

// RevokeAgent revokes an agent with 30-day data retention
// @Summary Revoke agent
// @Description Revoke an agent. Data is retained for 30 days and can be restored via reactivate.
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/revoke [post]
func (h *AgentHandler) RevokeAgent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Revoke the agent
	if err := h.agentService.RevokeAgent(c.Context(), agentID); err != nil {
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
			"action":     "revoked",
			"agentName":  agent.Name,
			"status":     agent.Status,
			"trustScore": agent.TrustScore,
		},
	)

	retentionUntil := time.Now().AddDate(0, 0, 30)

	return c.JSON(fiber.Map{
		"success":        true,
		"message":        "Agent revoked. Data retained for 30 days. Run reactivate within 30 days to restore.",
		"retentionUntil": retentionUntil.Format(time.RFC3339),
		"status":         agent.Status,
		"trustScore":     agent.TrustScore,
		"agent":          agent,
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
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/rotate-credentials [post]
func (h *AgentHandler) RotateCredentials(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
			"action":        "rotate_credentials",
			"agentName":     agent.Name,
			"rotationCount": agent.RotationCount,
			"keyCreatedAt":  agent.KeyCreatedAt,
			"keyExpiresAt":  agent.KeyExpiresAt,
		},
	)

	return c.JSON(fiber.Map{
		"success":           true,
		"message":           "Credentials rotated successfully",
		"publicKey":         publicKey,
		"privateKey":        privateKey, // ⚠️ SENSITIVE: Only returned once during rotation
		"previousPublicKey": agent.PreviousPublicKey,
		"rotationCount":     agent.RotationCount,
		"keyCreatedAt":      agent.KeyCreatedAt,
		"keyExpiresAt":      agent.KeyExpiresAt,
		"warning":           "Store the private key securely. It will not be shown again.",
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
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/keys [put]
func (h *AgentHandler) UpdateAgentKeys(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse request body
	var req struct {
		PublicKey string `json:"publicKey"`
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

	// SECURITY (A3b-ii / defects #18-25 follow-on): tenant-scoping via
	// LoadOwned. Returns 404 — not 403 — for cross-tenant access to avoid
	// leaking resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
	}

	// Get auth method from context (set by auth middleware)
	authMethod := ""
	if am := c.Locals("auth_method"); am != nil {
		authMethod, _ = am.(string)
	}

	// Update public key (passes auth method for key replacement security check)
	if err := h.agentService.UpdateAgentPublicKey(c.Context(), agentID, req.PublicKey, authMethod); err != nil {
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
			"action":        "update_public_key",
			"agentName":     agent.Name,
			"rotationCount": agent.RotationCount,
			"keyCreatedAt":  agent.KeyCreatedAt,
		},
	)

	return c.JSON(fiber.Map{
		"success":             true,
		"message":             "Public key updated successfully",
		"publicKey":           agent.PublicKey,
		"previous_public_key": agent.PreviousPublicKey,
		"rotationCount":       agent.RotationCount,
		"keyCreatedAt":        agent.KeyCreatedAt,
		"keyExpiresAt":        agent.KeyExpiresAt,
	})
}

// ========================================
// Agent Activity & Audit Logs
// ========================================

// GetAgentActivity retrieves all actions performed BY an agent
// This includes attestations, verifications, and other agent-initiated actions
// @Summary Get agent activity
// @Description Retrieve all actions performed by an agent (attestations, verifications, etc.)
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Maximum number of results (default 50)"
// @Param offset query int false "Pagination offset (default 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found (also returned for cross-tenant access; see tenant_scope.go:41-46)"
// @Router /agents/{id}/activity [get]
func (h *AgentHandler) GetAgentActivity(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3b / defects #18-25 follow-on): tenant-scoping via LoadOwned.
	// Returns 404 — not 403 — for cross-tenant access to avoid leaking
	// resource existence across orgs. See tenant_scope.go:41-46.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePagination(c)

	// Get agent activity from audit logs (orgID-scoped — LoadOwned above
	// already verified the agent belongs to orgID; the orgID filter is
	// defense-in-depth against future refactors of the agent loader).
	activities, err := h.auditService.GetAgentActivity(c.Context(), orgID, agentID, p.Limit, p.Offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agent activity",
		})
	}

	// Transform audit logs to activity response format
	activityList := make([]fiber.Map, 0, len(activities))
	for _, log := range activities {
		activityList = append(activityList, fiber.Map{
			"id":           log.ID,
			"action":       log.Action,
			"resourceType": log.ResourceType,
			"resourceId":   log.ResourceID,
			"timestamp":    log.Timestamp,
			"ipAddress":    log.IPAddress,
			"userAgent":    log.UserAgent,
			"metadata":     log.Metadata,
			"agentName":    log.AgentName,
		})
	}

	return c.JSON(fiber.Map{
		"activities": activityList,
		"total":      len(activityList),
		"limit":      p.Limit,
		"offset":     p.Offset,
		"agentId":    agentID,
	})
}
