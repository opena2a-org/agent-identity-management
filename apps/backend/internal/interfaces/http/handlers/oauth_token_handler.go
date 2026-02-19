package handlers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// OAuthTokenHandler handles the OAuth 2.0 token endpoint (RFC 6749)
type OAuthTokenHandler struct {
	jwtService *auth.JWTService
	agentRepo  domain.AgentRepository
}

// NewOAuthTokenHandler creates a new OAuth token handler
func NewOAuthTokenHandler(jwtService *auth.JWTService, agentRepo domain.AgentRepository) *OAuthTokenHandler {
	return &OAuthTokenHandler{
		jwtService: jwtService,
		agentRepo:  agentRepo,
	}
}

// HandleTokenRequest implements POST /oauth/token per RFC 6749
// Supports grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer (RFC 7523)
func (h *OAuthTokenHandler) HandleTokenRequest(c fiber.Ctx) error {
	contentType := c.Get("Content-Type")
	if contentType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Content-Type header is required",
		})
	}

	// Parse request body - support both form-urlencoded and JSON
	grantType := c.FormValue("grant_type")
	if grantType == "" {
		// Try JSON body as fallback
		var body struct {
			GrantType           string `json:"grant_type"`
			ClientID            string `json:"client_id"`
			ClientAssertion     string `json:"client_assertion"`
			ClientAssertionType string `json:"client_assertion_type"`
		}
		if err := c.Bind().JSON(&body); err == nil {
			grantType = body.GrantType
			if grantType == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error":             "invalid_request",
					"error_description": "grant_type is required",
				})
			}
			return h.processTokenRequest(c, body.GrantType, body.ClientID, body.ClientAssertion, body.ClientAssertionType)
		}

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "grant_type is required",
		})
	}

	clientID := c.FormValue("client_id")
	clientAssertion := c.FormValue("client_assertion")
	clientAssertionType := c.FormValue("client_assertion_type")

	return h.processTokenRequest(c, grantType, clientID, clientAssertion, clientAssertionType)
}

func (h *OAuthTokenHandler) processTokenRequest(c fiber.Ctx, grantType, clientID, clientAssertion, clientAssertionType string) error {
	// Validate grant_type
	if grantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "unsupported_grant_type",
			"error_description": "Only urn:ietf:params:oauth:grant-type:jwt-bearer is supported",
		})
	}

	// Validate required fields for jwt-bearer grant
	if clientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "client_id is required",
		})
	}

	if clientAssertion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "client_assertion is required",
		})
	}

	// Validate assertion type
	if clientAssertionType != "" && clientAssertionType != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Unsupported client_assertion_type",
		})
	}

	// Validate the JWT assertion structure (must be a valid JWT with 3 parts)
	parts := strings.Split(clientAssertion, ".")
	if len(parts) != 3 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: not a valid JWT",
		})
	}

	// Decode and validate the JWT payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid base64 encoding",
		})
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid JSON payload",
		})
	}

	// Verify the assertion's subject matches the client_id
	sub, _ := claims["sub"].(string)
	if sub != clientID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion subject does not match client_id",
		})
	}

	// SECURITY: Verify the JWT signature against the agent's registered public key.
	// The client_id must be a registered agent UUID with an Ed25519 public key.
	agentID, err := uuid.Parse(clientID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "client_id must be a valid agent UUID",
		})
	}

	agent, err := h.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Agent not found",
		})
	}

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Agent has no registered public key",
		})
	}

	// Verify Ed25519 signature: sign(header.payload) must match the signature part
	signedContent := parts[0] + "." + parts[1]
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid signature encoding",
		})
	}

	publicKeyBytes, err := base64.StdEncoding.DecodeString(*agent.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Failed to decode agent public key",
		})
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Agent public key has invalid size",
		})
	}

	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(signedContent), signatureBytes) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "JWT signature verification failed",
		})
	}

	// Generate an access token for the authenticated agent
	accessToken, err := h.jwtService.GenerateAccessToken(clientID, agent.OrganizationID.String(), clientID, "service")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Failed to generate access token",
		})
	}

	return c.JSON(fiber.Map{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}
