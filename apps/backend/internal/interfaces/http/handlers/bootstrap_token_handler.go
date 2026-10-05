package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// BootstrapTokenHeader carries a bootstrap token on the exchange call. The
// token is accepted here or in the JSON body, never in the URL.
const BootstrapTokenHeader = "X-AIM-Bootstrap-Token"

// Field limits for an exchange request. They match the agents table columns
// closely enough that a request within them is not refused by the database.
const (
	bootstrapMaxNameLen        = 255
	bootstrapMaxDescriptionLen = 2000
	bootstrapMaxVersionLen     = 50
)

// bootstrapTokenService is what the handler needs from
// application.BootstrapTokenService.
type bootstrapTokenService interface {
	Mint(ctx context.Context, orgID, userID uuid.UUID, meta application.BootstrapRequestMeta) (*application.MintedBootstrapToken, error)
	Revoke(ctx context.Context, orgID, userID uuid.UUID, meta application.BootstrapRequestMeta) (int64, error)
	RevokeExposed(ctx context.Context, plaintext string) error
	Exchange(ctx context.Context, plaintext string, req application.BootstrapExchangeRequest, meta application.BootstrapRequestMeta) (*application.BootstrapExchangeResult, error)
}

// BootstrapTokenHandler serves the onboarding bootstrap token endpoints.
type BootstrapTokenHandler struct {
	service      bootstrapTokenService
	dashboardURL string
}

// NewBootstrapTokenHandler creates a BootstrapTokenHandler. dashboardURL is
// the dashboard origin (FRONTEND_URL) an exchange reports, so a client that
// registered an agent can link to it without guessing the dashboard address
// from the API address; empty omits it.
func NewBootstrapTokenHandler(service bootstrapTokenService, dashboardURL string) *BootstrapTokenHandler {
	return &BootstrapTokenHandler{service: service, dashboardURL: strings.TrimRight(strings.TrimSpace(dashboardURL), "/")}
}

// MintBootstrapTokenResponse is returned once, by the mint call. Token is the
// only place the plaintext ever leaves the server.
type MintBootstrapTokenResponse struct {
	ID            string    `json:"id"`
	Token         string    `json:"token"`
	DisplayPrefix string    `json:"displayPrefix"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// ExchangeBootstrapTokenRequest is the exchange body. BootstrapToken is an
// alternative to the X-AIM-Bootstrap-Token header.
type ExchangeBootstrapTokenRequest struct {
	BootstrapToken string           `json:"bootstrapToken"`
	Name           string           `json:"name"`
	DisplayName    string           `json:"displayName"`
	Description    string           `json:"description"`
	AgentType      domain.AgentType `json:"agentType"`
	Version        string           `json:"version"`
	PublicKey      string           `json:"publicKey"`
}

// ExchangeBootstrapTokenResponse describes the registered agent.
type ExchangeBootstrapTokenResponse struct {
	AgentID        string `json:"agentId"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Status         string `json:"status"`
	PublicKey      string `json:"publicKey"`
	PrivateKey     string `json:"privateKey,omitempty"`
	AIMURL         string `json:"aimUrl"`
	// DashboardURL is the dashboard origin; the agent's page is
	// DashboardURL + "/dashboard/agents/" + AgentID.
	DashboardURL string `json:"dashboardUrl,omitempty"`
}

func bootstrapRequestMeta(c fiber.Ctx) application.BootstrapRequestMeta {
	return application.BootstrapRequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
}

// noStore keeps a response carrying a credential out of every cache.
func noStore(c fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
}

// Mint issues a bootstrap token for the caller's organization and revokes the
// caller's previous unused one.
// @Summary Mint an onboarding bootstrap token
// @Tags onboarding
// @Produce json
// @Success 201 {object} MintBootstrapTokenResponse
// @Router /onboarding/bootstrap-tokens [post]
func (h *BootstrapTokenHandler) Mint(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	minted, err := h.service.Mint(c.Context(), orgID, userID, bootstrapRequestMeta(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create bootstrap token",
		})
	}
	noStore(c)
	return c.Status(fiber.StatusCreated).JSON(MintBootstrapTokenResponse{
		ID:            minted.Token.ID.String(),
		Token:         minted.Plaintext,
		DisplayPrefix: minted.Token.DisplayPrefix,
		Scope:         minted.Token.Scope,
		ExpiresAt:     minted.Token.ExpiresAt,
	})
}

// Revoke revokes the caller's unused bootstrap tokens in the caller's
// organization.
// @Summary Revoke the caller's unused onboarding bootstrap tokens
// @Tags onboarding
// @Produce json
// @Router /onboarding/bootstrap-tokens/revoke [post]
func (h *BootstrapTokenHandler) Revoke(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	n, err := h.service.Revoke(c.Context(), orgID, userID, bootstrapRequestMeta(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke bootstrap token",
		})
	}
	return c.JSON(fiber.Map{"revoked": n})
}

// Exchange registers one agent in the token's organization and consumes the
// token. The token is the only credential: the route carries no session.
// @Summary Register an agent with an onboarding bootstrap token
// @Tags onboarding
// @Accept json
// @Produce json
// @Param X-AIM-Bootstrap-Token header string false "Bootstrap token (or bootstrapToken in the body)"
// @Success 201 {object} ExchangeBootstrapTokenResponse
// @Router /onboarding/bootstrap-tokens/exchange [post]
func (h *BootstrapTokenHandler) Exchange(c fiber.Ctx) error {
	noStore(c)

	// A token in the query string has been written to every access log and
	// proxy between the caller and here. Treat it as exposed: revoke it and
	// refuse the request.
	if exposed := bootstrapTokensInQuery(c); len(exposed) > 0 {
		for _, plaintext := range exposed {
			_ = h.service.RevokeExposed(c.Context(), plaintext)
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Send the bootstrap token in the " + BootstrapTokenHeader + " header or the JSON body, never in the URL. " +
				"The token in this URL has been revoked; generate a new one from the onboarding screen.",
			"code": "bootstrap_token_in_url",
		})
	}

	var body ExchangeBootstrapTokenRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}
	}

	headerToken := strings.TrimSpace(c.Get(BootstrapTokenHeader))
	bodyToken := strings.TrimSpace(body.BootstrapToken)
	if headerToken != "" && bodyToken != "" && headerToken != bodyToken {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "The " + BootstrapTokenHeader + " header and bootstrapToken in the body carry different tokens; send one.",
		})
	}
	plaintext := headerToken
	if plaintext == "" {
		plaintext = bodyToken
	}
	if plaintext == "" {
		return bootstrapTokenRefusal(c, domain.ErrBootstrapTokenMalformed)
	}

	if msg := validateBootstrapExchangeBody(&body); msg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
	}

	result, err := h.service.Exchange(c.Context(), plaintext, application.BootstrapExchangeRequest{
		Name:        body.Name,
		DisplayName: body.DisplayName,
		Description: body.Description,
		AgentType:   body.AgentType,
		Version:     body.Version,
		PublicKey:   body.PublicKey,
	}, bootstrapRequestMeta(c))
	if err != nil {
		return bootstrapExchangeError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(ExchangeBootstrapTokenResponse{
		AgentID:        result.Agent.ID.String(),
		OrganizationID: result.Agent.OrganizationID.String(),
		Name:           result.Agent.Name,
		DisplayName:    result.Agent.DisplayName,
		Status:         string(result.Agent.Status),
		PublicKey:      result.PublicKey,
		PrivateKey:     result.PrivateKey,
		AIMURL:         normalizeAIMURL(c.BaseURL()),
		DashboardURL:   h.dashboardURL,
	})
}

// bootstrapTokensInQuery returns every query value that is, or every value of
// a token-named key, which may be a bootstrap token.
func bootstrapTokensInQuery(c fiber.Ctx) []string {
	found := make([]string, 0)
	for key, value := range c.Request().URI().QueryArgs().All() {
		k, v := string(key), string(value)
		lowered := strings.ToLower(k)
		if strings.Contains(v, domain.BootstrapTokenPrefix) ||
			strings.Contains(k, domain.BootstrapTokenPrefix) ||
			lowered == "token" || lowered == "bootstraptoken" || lowered == "bootstrap_token" {
			found = append(found, v, k)
		}
	}
	return found
}

func validateBootstrapExchangeBody(body *ExchangeBootstrapTokenRequest) string {
	body.Name = strings.TrimSpace(body.Name)
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	body.Description = strings.TrimSpace(body.Description)
	body.Version = strings.TrimSpace(body.Version)
	body.PublicKey = strings.TrimSpace(body.PublicKey)

	switch {
	case len(body.Name) > bootstrapMaxNameLen:
		return fmt.Sprintf("name must be at most %d characters", bootstrapMaxNameLen)
	case len(body.DisplayName) > bootstrapMaxNameLen:
		return fmt.Sprintf("displayName must be at most %d characters", bootstrapMaxNameLen)
	case len(body.Description) > bootstrapMaxDescriptionLen:
		return fmt.Sprintf("description must be at most %d characters", bootstrapMaxDescriptionLen)
	case len(body.Version) > bootstrapMaxVersionLen:
		return fmt.Sprintf("version must be at most %d characters", bootstrapMaxVersionLen)
	case body.AgentType != "" && !isValidAgentTypeForPublicAPI(body.AgentType):
		return fmt.Sprintf("invalid agentType: %s", body.AgentType)
	}
	return ""
}

// bootstrapTokenRefusal answers 401 for a token that cannot be exchanged,
// with a machine-readable code the SDK can act on.
func bootstrapTokenRefusal(c fiber.Ctx, err error) error {
	code, message := "bootstrap_token_invalid", "The bootstrap token is not valid. Generate a new one from the onboarding screen."
	switch {
	case errors.Is(err, domain.ErrBootstrapTokenExpired):
		code, message = "bootstrap_token_expired", "The bootstrap token has expired. Generate a new one from the onboarding screen."
	case errors.Is(err, domain.ErrBootstrapTokenUsed):
		code, message = "bootstrap_token_used", "The bootstrap token has already registered an agent. Generate a new one from the onboarding screen."
	case errors.Is(err, domain.ErrBootstrapTokenRevoked):
		code, message = "bootstrap_token_revoked", "The bootstrap token has been revoked. Generate a new one from the onboarding screen."
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": message, "code": code})
}

func bootstrapExchangeError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrBootstrapTokenMalformed),
		errors.Is(err, domain.ErrBootstrapTokenNotFound),
		errors.Is(err, domain.ErrBootstrapTokenExpired),
		errors.Is(err, domain.ErrBootstrapTokenUsed),
		errors.Is(err, domain.ErrBootstrapTokenRevoked):
		return bootstrapTokenRefusal(c, err)
	case errors.Is(err, application.ErrBootstrapTokenScope):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error(), "code": "bootstrap_token_scope"})
	case errors.Is(err, application.ErrBootstrapInvalidPublicKey):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	status := registrationErrorStatus(err)
	if status == fiber.StatusInternalServerError {
		return c.Status(status).JSON(fiber.Map{"error": "Failed to register agent"})
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}
