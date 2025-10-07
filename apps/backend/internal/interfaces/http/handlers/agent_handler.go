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
	agentService *application.AgentService
	auditService *application.AuditService
}

func NewAgentHandler(
	agentService *application.AgentService,
	auditService *application.AuditService,
) *AgentHandler {
	return &AgentHandler{
		agentService: agentService,
		auditService: auditService,
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

	return c.JSON(fiber.Map{
		"agents": agents,
		"total":  len(agents),
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
			"agent_name": agent.Name,
			"agent_type": agent.AgentType,
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

	return c.JSON(agent)
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
			"agent_name": agent.Name,
		},
	)

	return c.JSON(agent)
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
			"agent_name":  agent.Name,
			"trust_score": agent.TrustScore,
		},
	)

	return c.JSON(fiber.Map{
		"verified":    true,
		"trust_score": agent.TrustScore,
		"verified_at": agent.VerifiedAt,
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
		ActionType string                 `json:"action_type"` // "read_file", "write_file", "execute_code", "network_request", "database_query"
		Resource   string                 `json:"resource"`    // e.g., "/data/file.csv" or "SELECT * FROM users"
		Metadata   map[string]interface{} `json:"metadata"`    // Additional context
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

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
			"agent_name": agent.Name,
		},
	)

	return c.Send(sdkBytes)
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
