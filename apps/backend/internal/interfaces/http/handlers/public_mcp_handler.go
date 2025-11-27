package handlers

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
)

// PublicMCPHandler handles public (no user auth) MCP server operations
// Uses agent authentication (cryptographic signatures) instead of JWT
type PublicMCPHandler struct {
	mcpService   *application.MCPService
	agentService *application.AgentService
	auditService *application.AuditService
}

// NewPublicMCPHandler creates a new public MCP handler
func NewPublicMCPHandler(
	mcpService *application.MCPService,
	agentService *application.AgentService,
	auditService *application.AuditService,
) *PublicMCPHandler {
	return &PublicMCPHandler{
		mcpService:   mcpService,
		agentService: agentService,
		auditService: auditService,
	}
}

// RegisterMCPServerRequest represents the request to register an MCP server
type RegisterMCPServerRequest struct {
	AgentID       string   `json:"agent_id"`       // Agent registering the MCP server
	ServerName    string   `json:"server_name"`    // Name of MCP server
	ServerURL     string   `json:"server_url"`     // URL of MCP server
	PublicKey     string   `json:"public_key"`     // Ed25519 public key of MCP server
	Capabilities  []string `json:"capabilities"`   // Server capabilities
	Description   string   `json:"description"`    // Server description
	Version       string   `json:"version"`        // Server version
	Timestamp     int64    `json:"timestamp"`      // Request timestamp
	Signature     string   `json:"signature"`      // Cryptographic signature
}

// RegisterMCPServer registers an MCP server using agent authentication
// @Summary Register MCP server (public, agent-authenticated)
// @Description Register an MCP server using cryptographic signature (no user login required)
// @Tags public-mcp
// @Accept json
// @Produce json
// @Param request body RegisterMCPServerRequest true "MCP Server Registration"
// @Success 201 {object} domain.MCPServer
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /public/mcp-servers/register [post]
func (h *PublicMCPHandler) RegisterMCPServer(c fiber.Ctx) error {
	var req RegisterMCPServerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.AgentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_id is required",
		})
	}
	if req.ServerName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "server_name is required",
		})
	}
	if req.ServerURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "server_url is required",
		})
	}
	if req.PublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "public_key is required",
		})
	}
	if len(req.Capabilities) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "capabilities list cannot be empty",
		})
	}
	if req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "signature is required for authentication",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent_id format",
		})
	}

	// Get agent to verify signature
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	// Verify agent is verified
	if agent.Status != domain.AgentStatusVerified {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent is not verified",
		})
	}

	// Verify agent has public key
	if agent.PublicKey == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent has no public key",
		})
	}

	// Verify cryptographic signature
	// Message format: "register_mcp_server:{agent_id}:{server_name}:{server_url}:{timestamp}"
	message := "register_mcp_server:" + req.AgentID + ":" + req.ServerName + ":" + req.ServerURL + ":" + strconv.FormatInt(req.Timestamp, 10)

	// Decode hex public key
	publicKeyBytes, err := crypto.DecodePublicKey(*agent.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid agent public key",
		})
	}

	// Decode base64 signature
	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature format",
		})
	}

	isValid := crypto.VerifySignature(publicKeyBytes, []byte(message), signatureBytes)
	if !isValid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature - authentication failed",
		})
	}

	// Create MCP server
	createReq := &application.CreateMCPServerRequest{
		Name:         req.ServerName,
		Description:  req.Description,
		URL:          req.ServerURL,
		Version:      req.Version,
		PublicKey:    req.PublicKey,
		Capabilities: req.Capabilities,
	}

	server, err := h.mcpService.CreateMCPServer(c.Context(), createReq, agent.OrganizationID, agentID, &agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		agent.OrganizationID,
		agentID,
		domain.AuditActionCreate,
		"mcp_server",
		server.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"serverName":  server.Name,
			"serverUrl":   server.URL,
			"registered_by_agent": agent.Name,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(server)
}

// ListMCPServersForAgent lists MCP servers for a specific agent
// @Summary List MCP servers for agent (public, agent-authenticated)
// @Description List all MCP servers registered by or accessible to this agent
// @Tags public-mcp
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param signature query string true "Cryptographic signature"
// @Param timestamp query int true "Request timestamp"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /public/mcp-servers/agent/{agent_id} [get]
func (h *PublicMCPHandler) ListMCPServersForAgent(c fiber.Ctx) error {
	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent_id format",
		})
	}

	// Get signature and timestamp from query params
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")

	if signature == "" || timestamp == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "signature and timestamp are required",
		})
	}

	// Get agent to verify signature
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	// Verify agent has public key
	if agent.PublicKey == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent has no public key",
		})
	}

	// Decode hex public key
	publicKeyBytes, err := crypto.DecodePublicKey(*agent.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid agent public key",
		})
	}

	// Decode base64 signature
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature format",
		})
	}

	// Verify signature
	message := "list_mcp_servers:" + agentID.String() + ":" + timestamp
	isValid := crypto.VerifySignature(publicKeyBytes, []byte(message), signatureBytes)
	if !isValid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature - authentication failed",
		})
	}

	// List MCP servers for this agent's organization
	servers, err := h.mcpService.ListMCPServers(c.Context(), agent.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers",
		})
	}

	return c.JSON(fiber.Map{
		"mcpServers": servers,
		"total":       len(servers),
	})
}

// VerifyMCPAction verifies an MCP action using agent authentication
// @Summary Verify MCP action (public, agent-authenticated)
// @Description Verify an MCP tool/resource/prompt action using agent signature
// @Tags public-mcp
// @Accept json
// @Produce json
// @Param server_id path string true "MCP Server ID"
// @Param request body VerifyMCPActionRequest true "Action Verification"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /public/mcp-servers/{server_id}/verify [post]
func (h *PublicMCPHandler) VerifyMCPAction(c fiber.Ctx) error {
	// Parse server ID
	serverID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid mcp_server_id format",
		})
	}

	var req struct {
		AgentID    string                 `json:"agent_id"`
		ActionType string                 `json:"action_type"`
		Resource   string                 `json:"resource"`
		Context    map[string]interface{} `json:"context"`
		RiskLevel  string                 `json:"risk_level"`
		Timestamp  int64                  `json:"timestamp"`
		Signature  string                 `json:"signature"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.AgentID == "" || req.ActionType == "" || req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_id, action_type, and signature are required",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent_id format",
		})
	}

	// Get agent
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	// Verify agent has public key
	if agent.PublicKey == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent has no public key",
		})
	}

	// Decode hex public key
	publicKeyBytes, err := crypto.DecodePublicKey(*agent.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid agent public key",
		})
	}

	// Decode base64 signature
	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature format",
		})
	}

	// Verify signature
	message := "verify_mcp_action:" + req.AgentID + ":" + serverID.String() + ":" + req.ActionType + ":" + strconv.FormatInt(req.Timestamp, 10)
	isValid := crypto.VerifySignature(publicKeyBytes, []byte(message), signatureBytes)
	if !isValid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid signature - authentication failed",
		})
	}

	// ✅ SIMPLE CAPABILITY CHECK (MVP)
	// Get MCP server to check if agent is allowed to talk to it
	mcpServer, err := h.mcpService.GetMCPServer(c.Context(), serverID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "MCP server not found",
		})
	}

	// Check if MCP server name/ID is in agent's talks_to list
	isAuthorized := false
	if len(agent.TalksTo) == 0 {
		// If no TalksTo configured, allow all MCP servers (backwards compatibility)
		isAuthorized = true
	} else {
		for _, allowedServer := range agent.TalksTo {
			// Match by name (case-insensitive) or ID
			if strings.EqualFold(allowedServer, mcpServer.Name) || allowedServer == serverID.String() {
				isAuthorized = true
				break
			}
		}
	}

	// If NOT authorized, create alert and reject
	if !isAuthorized {
		// Create capability violation alert
		// (In future, this would use AlertService)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":              "Agent not authorized to access this MCP server",
			"agentId":           agentID.String(),
			"mcp_server_id":      serverID.String(),
			"mcp_server_name":    mcpServer.Name,
			"violation_type":     "unauthorized_mcp_access",
			"severity":           "high",
			"trust_score_impact": -10,
		})
	}

	// ✅ AUTHORIZED - Return success
	verificationID := uuid.New()

	return c.JSON(fiber.Map{
		"verification_id":    verificationID.String(),
		"status":             "approved",
		"mcp_server_id":      serverID.String(),
		"mcp_server_name":    mcpServer.Name,
		"agentId":           agentID.String(),
		"action_type":        req.ActionType,
		"timestamp":          req.Timestamp,
		"trust_score_impact": 0.5,
	})
}

// VerifyMCPActionRequest represents an MCP action verification request
type VerifyMCPActionRequest struct {
	AgentID    string                 `json:"agent_id"`
	ActionType string                 `json:"action_type"`
	Resource   string                 `json:"resource"`
	Context    map[string]interface{} `json:"context"`
	RiskLevel  string                 `json:"risk_level"`
	Timestamp  int64                  `json:"timestamp"`
	Signature  string                 `json:"signature"`
}
