package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
)

// PublicAgentHandler handles public agent registration (no authentication required)
type PublicAgentHandler struct {
	agentService *application.AgentService
	authService  *application.AuthService
	keyVault     *crypto.KeyVault
}

// NewPublicAgentHandler creates a new public agent handler
func NewPublicAgentHandler(
	agentService *application.AgentService,
	authService *application.AuthService,
	keyVault *crypto.KeyVault,
) *PublicAgentHandler {
	return &PublicAgentHandler{
		agentService: agentService,
		authService:  authService,
		keyVault:     keyVault,
	}
}

// PublicRegisterRequest represents a public agent registration request
type PublicRegisterRequest struct {
	Name                string           `json:"name" validate:"required"`
	DisplayName         string           `json:"display_name" validate:"required"`
	Description         string           `json:"description" validate:"required"`
	AgentType           domain.AgentType `json:"agent_type" validate:"required"`
	Version             string           `json:"version"`
	OrganizationDomain  string           `json:"organization_domain"` // e.g., "example.com"
	UserEmail           string           `json:"user_email"`          // Optional: for user association
	RepositoryURL       string           `json:"repository_url"`
	DocumentationURL    string           `json:"documentation_url"`
}

// PublicRegisterResponse includes credentials (private key only returned ONCE)
type PublicRegisterResponse struct {
	AgentID     string  `json:"agent_id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	PublicKey   string  `json:"public_key"`
	PrivateKey  string  `json:"private_key"` // ⚠️ ONLY returned on registration
	AIMURL      string  `json:"aim_url"`
	Status      string  `json:"status"`
	TrustScore  float64 `json:"trust_score"`
	Message     string  `json:"message"`
}

// Register handles public agent self-registration
// @Summary Public agent self-registration
// @Description Register an agent without authentication. Returns credentials including private key (ONLY ONCE).
// @Tags public
// @Accept json
// @Produce json
// @Param request body PublicRegisterRequest true "Registration request"
// @Success 201 {object} PublicRegisterResponse "Agent registered successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /public/agents/register [post]
func (h *PublicAgentHandler) Register(c fiber.Ctx) error {
	var req PublicRegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Name == "" || req.DisplayName == "" || req.Description == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, display_name, and description are required",
		})
	}

	if req.AgentType != domain.AgentTypeAI && req.AgentType != domain.AgentTypeMCP {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_type must be 'ai_agent' or 'mcp_server'",
		})
	}

	// Extract API key from header
	apiKey := c.Get("X-AIM-API-Key")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "X-AIM-API-Key header is required for agent registration",
		})
	}

	// Validate API key and extract user identity
	validation, err := h.authService.ValidateAPIKey(c.Context(), apiKey)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid API key: %v", err),
		})
	}

	// Use real user and organization from API key
	userID := validation.User.ID
	orgID := validation.Organization.ID

	// Create agent (keys generated automatically by AgentService)
	agent, err := h.agentService.CreateAgent(c.Context(), &application.CreateAgentRequest{
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		AgentType:        req.AgentType,
		Version:          req.Version,
		RepositoryURL:    req.RepositoryURL,
		DocumentationURL: req.DocumentationURL,
	}, orgID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create agent: %v", err),
		})
	}

	// Get the actual keys from the created agent
	publicKey, privateKey, err := h.agentService.GetAgentCredentials(c.Context(), agent.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to retrieve agent credentials: %v", err),
		})
	}

	// Calculate initial trust score
	trustScore := h.calculateInitialTrustScore(&req)

	// Build response with credentials (private key ONLY returned here!)
	response := PublicRegisterResponse{
		AgentID:     agent.ID.String(),
		Name:        agent.Name,
		DisplayName: agent.DisplayName,
		PublicKey:   publicKey,
		PrivateKey:  privateKey, // ⚠️ CRITICAL: Only returned ONCE
		AIMURL:      c.BaseURL(),
		Status:      string(agent.Status),
		TrustScore:  trustScore,
		Message:     h.buildRegistrationMessage(agent.Status),
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// calculateInitialTrustScore calculates trust score for new agent
func (h *PublicAgentHandler) calculateInitialTrustScore(req *PublicRegisterRequest) float64 {
	score := 50.0 // Base score

	// Bonus for providing repository URL
	if req.RepositoryURL != "" {
		score += 10.0
	}

	// Bonus for documentation
	if req.DocumentationURL != "" {
		score += 5.0
	}

	// Bonus for version specified
	if req.Version != "" {
		score += 5.0
	}

	// Bonus for GitHub/GitLab repos
	if strings.Contains(req.RepositoryURL, "github.com") || strings.Contains(req.RepositoryURL, "gitlab.com") {
		score += 10.0
	}

	// TODO: Add more sophisticated trust scoring
	// - Organization reputation
	// - Email domain verification
	// - Code signing verification

	if score > 100.0 {
		score = 100.0
	}

	return score
}

// buildRegistrationMessage creates helpful message based on status
func (h *PublicAgentHandler) buildRegistrationMessage(status domain.AgentStatus) string {
	switch status {
	case domain.AgentStatusVerified:
		return "✅ Agent registered and auto-verified! You can start using it immediately."
	case domain.AgentStatusPending:
		return "⏳ Agent registered. Pending manual verification by administrator."
	default:
		return "Agent registered successfully."
	}
}
