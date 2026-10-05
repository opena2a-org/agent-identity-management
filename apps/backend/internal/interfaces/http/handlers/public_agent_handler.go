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

// PublicAgentHandler handles agent registration on the /public route group.
// The group runs no mandatory auth middleware, but an agent belongs to a user
// and an organization, so Register serves only a signed-in user.
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

// publicRegisterAuthRequiredMessage is the 401 body for a caller this route
// cannot register an agent for. It names the two registration paths that work,
// so a caller holding the other kind of credential knows where to send it.
const publicRegisterAuthRequiredMessage = "Authentication required. Send a signed-in user's access token as " +
	"'Authorization: Bearer <token>' to this route, or register with an API key in the X-API-Key header at " +
	"POST /api/v1/agents."

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
		// Demo (registered by `aim-sdk demo`)
		domain.AgentTypeDemo: true,
		// Legacy support
		domain.AgentTypeAI: true,
	}
	return validTypes[agentType]
}

// Register registers an agent for the signed-in user
// @Summary Register an agent as a signed-in user
// @Description Registers an agent in the caller's organization and returns its credentials, including the private key (ONLY ONCE). Requires a user access token in the Authorization header; any other caller gets 401. To register with an API key, send it in the X-API-Key header to POST /agents.
// @Tags public
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body PublicRegisterRequest true "Registration request"
// @Success 201 {object} PublicRegisterResponse "Agent registered successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "No valid user access token"
// @Failure 409 {object} ErrorResponse "Agent name already exists"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /public/agents/register [post]
func (h *PublicAgentHandler) Register(c fiber.Ctx) error {
	// OptionalAuthMiddleware sets the user and organization only for a valid
	// user access token. An agent row needs both, so any other caller (no
	// credential, an API key in any header, a refresh or revoked token) cannot
	// be registered here. Answer before reading the body: the service would
	// otherwise generate a keypair and attempt an insert the database refuses.
	userID, hasUser := c.Locals("user_id").(uuid.UUID)
	orgID, hasOrg := c.Locals("organization_id").(uuid.UUID)
	if !hasUser || !hasOrg || userID == uuid.Nil || orgID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": publicRegisterAuthRequiredMessage,
		})
	}

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

	var userEmail string
	if req.UserEmail != "" {
		userEmail = req.UserEmail
	}

	// Create agent (keys generated automatically by AgentService)
	// This route has no API key or SDK token to track.
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
		// The service already maps DB constraint violations to safe messages, so
		// surface err.Error() directly — do NOT re-prefix with "Failed to create
		// agent:" (the service message is already "failed to create agent: ..."),
		// which produced a doubled prefix. registrationErrorStatus mirrors the
		// authenticated handler: 409 for a duplicate name, 400 for an invalid
		// org/user, 500 otherwise.
		return c.Status(registrationErrorStatus(err)).JSON(fiber.Map{
			"error": err.Error(),
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
