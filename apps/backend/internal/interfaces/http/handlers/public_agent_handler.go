package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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
	Name               string           `json:"name" validate:"required"`
	DisplayName        string           `json:"displayName" validate:"required"`
	Description        string           `json:"description" validate:"required"`
	AgentType          domain.AgentType `json:"agentType" validate:"required"`
	Version            string           `json:"version"`
	OrganizationDomain string           `json:"organizationDomain"` // e.g., "example.com"
	UserEmail          string           `json:"userEmail"`          // Optional: for user association
	RepositoryURL      string           `json:"repositoryUrl"`
	DocumentationURL   string           `json:"documentationUrl"`
}

// PublicRegisterResponse includes credentials (private key only returned ONCE)
type PublicRegisterResponse struct {
	AgentID     string  `json:"agentId"`
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	PublicKey   string  `json:"publicKey"`
	PrivateKey  string  `json:"privateKey"` // ONLY returned on registration
	AIMURL      string  `json:"aimUrl"`
	Status      string  `json:"status"`
	TrustScore  float64 `json:"trustScore"`
	Message     string  `json:"message"`
}

// isValidAgentTypeForPublicAPI checks if the given agent type is valid for public registration
func isValidAgentTypeForPublicAPI(agentType domain.AgentType) bool {
	validTypes := map[domain.AgentType]bool{
		// LLM Providers
		domain.AgentTypeClaude:  true,
		domain.AgentTypeGPT:     true,
		domain.AgentTypeGemini:  true,
		domain.AgentTypeLlama:   true,
		domain.AgentTypeMistral: true,
		domain.AgentTypeCohere:  true,
		// Frameworks
		domain.AgentTypeLangChain:      true,
		domain.AgentTypeLlamaIndex:     true,
		domain.AgentTypeAutoGen:        true,
		domain.AgentTypeCrewAI:         true,
		domain.AgentTypeLangGraph:      true,
		domain.AgentTypeHaystack:       true,
		domain.AgentTypeSemanticKernel: true,
		// Copilots & Assistants
		domain.AgentTypeCopilot:   true,
		domain.AgentTypeAssistant: true,
		domain.AgentTypeChatbot:   true,
		// Autonomous
		domain.AgentTypeAutoGPT: true,
		domain.AgentTypeBabyAGI: true,
		// Generic
		domain.AgentTypeCustom: true,
		// Legacy support
		domain.AgentTypeAI: true,
	}
	return validTypes[agentType]
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
	if req.Name == "" || req.DisplayName == "" || req.Description == "" || req.AgentType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, displayName, description, and agentType are required",
		})
	}

	if !isValidAgentTypeForPublicAPI(req.AgentType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid agentType: %s", req.AgentType),
		})
	}

	// Use authenticated user/org if available (set by OptionalAuthMiddleware),
	// otherwise use zero UUIDs for truly public (unauthenticated) registration.
	var userID, orgID uuid.UUID
	var userEmail string

	if uid, ok := c.Locals("user_id").(uuid.UUID); ok {
		userID = uid
	}
	if oid, ok := c.Locals("organization_id").(uuid.UUID); ok {
		orgID = oid
	}
	if req.UserEmail != "" {
		userEmail = req.UserEmail
	}

	// Create agent (keys generated automatically by AgentService)
	// Public registration has no API key or SDK token to track.
	// Both sdkTokenID and apiKeyID are nil for this flow.
	agent, err := h.agentService.CreateAgent(c.Context(), &application.CreateAgentRequest{
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		AgentType:        req.AgentType,
		Version:          req.Version,
		RepositoryURL:    req.RepositoryURL,
		DocumentationURL: req.DocumentationURL,
	}, orgID, userID, nil, nil, userEmail)
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
		PrivateKey:  privateKey, // CRITICAL: Only returned ONCE
		// normalizeAIMURL forces https for any public host (see sdk_handler.go):
		// behind a TLS-terminating ingress c.BaseURL() reports http, and an agent
		// that POSTs to http first gets a 301 that drops the request body.
		AIMURL:      normalizeAIMURL(c.BaseURL()),
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

	if score > 100.0 {
		score = 100.0
	}

	return score
}

// buildRegistrationMessage creates helpful message based on status
func (h *PublicAgentHandler) buildRegistrationMessage(status domain.AgentStatus) string {
	switch status {
	case domain.AgentStatusVerified:
		return "Agent registered and auto-verified. You can start using it immediately."
	case domain.AgentStatusPending:
		return "Agent registered. Pending manual verification by administrator."
	default:
		return "Agent registered successfully."
	}
}
