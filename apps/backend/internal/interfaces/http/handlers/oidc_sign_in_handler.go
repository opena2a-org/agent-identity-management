package handlers

import (
	"context"
	"errors"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth"
)

// oidcSignInService is what the handler asks of application.OIDCSignInService.
type oidcSignInService interface {
	StartSignIn(ctx context.Context, organizationID uuid.UUID) (*application.OIDCSignInStart, error)
	CompleteSignIn(ctx context.Context, code, state, flow string) (*domain.User, error)
	Configuration(ctx context.Context, organizationID uuid.UUID) (*domain.OIDCSignInConfig, error)
	Configure(ctx context.Context, organizationID, actorID uuid.UUID, in application.OIDCSignInConfigInput) (*domain.OIDCSignInConfig, error)
	RemoveConfiguration(ctx context.Context, organizationID uuid.UUID) error
	RedirectURL() string
}

// OIDCSignInHandler serves sign-in through an organization's OpenID Connect
// provider, and the administration of that provider.
//
//	POST   /api/v1/auth/oidc/:organizationId/login  starts a sign-in
//	POST   /api/v1/auth/oidc/callback               completes it with a login pair
//	GET    /api/v1/admin/oidc-sign-in               the caller's organization's provider
//	PUT    /api/v1/admin/oidc-sign-in               stores it
//	DELETE /api/v1/admin/oidc-sign-in               removes it
type OIDCSignInHandler struct {
	service      oidcSignInService
	jwtService   *auth.JWTService
	auditService *application.AuditService
}

// NewOIDCSignInHandler builds the handler. A completed sign-in is answered
// with a pair from jwtService and recorded in the audit log as a login.
func NewOIDCSignInHandler(service *application.OIDCSignInService, jwtService *auth.JWTService, auditService *application.AuditService) *OIDCSignInHandler {
	return &OIDCSignInHandler{service: service, jwtService: jwtService, auditService: auditService}
}

// StartSignIn answers the provider's authorization URL for the organization
// in the path, and the flow value the client sends back to CompleteSignIn
// with the code and state the provider redirects to the dashboard with.
func (h *OIDCSignInHandler) StartSignIn(c fiber.Ctx) error {
	organizationID, err := uuid.Parse(c.Params("organizationId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organizationId must be a UUID"})
	}
	start, err := h.service.StartSignIn(c.Context(), organizationID)
	switch {
	case errors.Is(err, domain.ErrOIDCSignInNotConfigured):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "The organization has no OIDC sign-in"})
	case errors.Is(err, oauth.ErrOIDCProviderUnavailable):
		log.Printf("OIDC sign-in start for organization %s: %v", organizationID, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "The identity provider could not be reached"})
	case err != nil:
		log.Printf("OIDC sign-in start for organization %s: %v", organizationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start the sign-in"})
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(fiber.Map{
		"authorizationUrl": start.AuthorizationURL,
		"flow":             start.Flow,
		"expiresAt":        start.ExpiresAt.UTC(),
	})
}

// oidcCallbackRequest is the body of POST /api/v1/auth/oidc/callback.
type oidcCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
	Flow  string `json:"flow"`
}

// CompleteSignIn completes a sign-in with the code and state the provider
// redirected with and the flow StartSignIn answered, and answers a login pair
// in the form POST /api/v1/auth/login/local answers one.
func (h *OIDCSignInHandler) CompleteSignIn(c fiber.Ctx) error {
	var req oidcCallbackRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Code == "" || req.State == "" || req.Flow == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code, state and flow are required"})
	}

	user, err := h.service.CompleteSignIn(c.Context(), req.Code, req.State, req.Flow)
	var refusal *application.OIDCSignInRefusal
	switch {
	case errors.As(err, &refusal):
		log.Printf("OIDC sign-in refused: reason=%s organization=%s user=%s", refusal.Reason, refusal.OrganizationID, refusal.UserID)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Sign-in through the identity provider was refused",
			"code":  refusal.Reason,
		})
	case errors.Is(err, oauth.ErrOIDCProviderUnavailable):
		log.Printf("OIDC sign-in callback: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "The identity provider could not be reached"})
	case err != nil:
		log.Printf("OIDC sign-in callback: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to complete the sign-in"})
	}

	accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
		user.ID.String(),
		user.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate tokens"})
	}
	recordSignIn(c, h.auditService, h.jwtService, refreshToken, fiber.Map{"method": "oidc"})

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(fiber.Map{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": fiber.Map{
			"id":                  user.ID,
			"email":               user.Email,
			"name":                user.Name,
			"role":                user.Role,
			"organizationId":      user.OrganizationID,
			"forcePasswordChange": user.ForcePasswordChange,
		},
	})
}

// oidcConfigurationAnswer is the answer of the administration routes. It
// never carries the client secret, sealed or not.
func (h *OIDCSignInHandler) oidcConfigurationAnswer(cfg *domain.OIDCSignInConfig) fiber.Map {
	if cfg == nil {
		return fiber.Map{"configured": false, "redirectUrl": h.service.RedirectURL()}
	}
	return fiber.Map{
		"configured":      true,
		"issuer":          cfg.Issuer,
		"clientId":        cfg.ClientID,
		"clientSecretSet": cfg.SealedClientSecret != "",
		"redirectUrl":     h.service.RedirectURL(),
		"updatedBy":       cfg.UpdatedBy,
		"updatedAt":       cfg.UpdatedAt.UTC(),
	}
}

// GetConfiguration answers the caller's organization's provider, or
// "configured": false. Either answer carries the redirect URL to register
// with the provider.
func (h *OIDCSignInHandler) GetConfiguration(c fiber.Ctx) error {
	organizationID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	cfg, err := h.service.Configuration(c.Context(), organizationID)
	if errors.Is(err, domain.ErrOIDCSignInNotConfigured) {
		return c.JSON(h.oidcConfigurationAnswer(nil))
	}
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	return c.JSON(h.oidcConfigurationAnswer(cfg))
}

// oidcConfigurationRequest is the body of PUT /api/v1/admin/oidc-sign-in.
type oidcConfigurationRequest struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// PutConfiguration stores the caller's organization's provider, in place of
// any earlier one, and records the change in the audit log.
func (h *OIDCSignInHandler) PutConfiguration(c fiber.Ctx) error {
	organizationID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	var req oidcConfigurationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	cfg, err := h.service.Configure(c.Context(), organizationID, userID, application.OIDCSignInConfigInput{
		Issuer:       req.Issuer,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
	})
	if err != nil && !errors.Is(err, application.ErrOIDCSignInConfigInvalid) {
		log.Printf("OIDC sign-in configuration of organization %s refused: %v", organizationID, err)
	}
	switch {
	case errors.Is(err, application.ErrOIDCIssuerInvalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "issuer must be an https URL with a host and no user information, query or fragment, of at most 2048 characters",
		})
	case errors.Is(err, application.ErrOIDCClientIDInvalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientId is required, at most 512 characters"})
	case errors.Is(err, application.ErrOIDCClientSecretInvalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientSecret is required, at most 2048 characters"})
	case errors.Is(err, oauth.ErrOIDCIssuerMismatch):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "The issuer's configuration document names another issuer; issuer must be the issuer the document names, exactly",
		})
	case errors.Is(err, oauth.ErrOIDCProviderUnavailable):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "The issuer's configuration document could not be read, or names an endpoint that is not an https URL",
		})
	case err != nil:
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	h.audit(c, organizationID, userID, domain.AuditActionUpdate, fiber.Map{"issuer": cfg.Issuer, "clientId": cfg.ClientID})
	return c.JSON(h.oidcConfigurationAnswer(cfg))
}

// DeleteConfiguration removes the caller's organization's provider and
// records the removal in the audit log.
func (h *OIDCSignInHandler) DeleteConfiguration(c fiber.Ctx) error {
	organizationID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	if err := h.service.RemoveConfiguration(c.Context(), organizationID); err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	h.audit(c, organizationID, userID, domain.AuditActionDelete, fiber.Map{})
	return c.SendStatus(fiber.StatusNoContent)
}

// audit writes the row of a configuration change. A row that cannot be
// written is logged and does not fail the change.
func (h *OIDCSignInHandler) audit(c fiber.Ctx, organizationID, userID uuid.UUID, action domain.AuditAction, meta fiber.Map) {
	if h.auditService == nil {
		return
	}
	if err := h.auditService.LogAction(c.Context(), organizationID, userID, action, "oidc_sign_in_config", organizationID,
		recordedClientIP(c), c.Get("User-Agent"), meta); err != nil {
		log.Printf("OIDC sign-in configuration: audit row not written: %v", err)
	}
}
