package handlers

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

type AuthHandler struct {
	authService  *application.AuthService
	jwtService   *auth.JWTService
	orgRepo      domain.OrganizationRepository
	auditService *application.AuditService
}

func NewAuthHandler(
	authService *application.AuthService,
	jwtService *auth.JWTService,
	orgRepo domain.OrganizationRepository,
	auditService *application.AuditService,
) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		jwtService:   jwtService,
		orgRepo:      orgRepo,
		auditService: auditService,
	}
}

// Me returns current user info
func (h *AuthHandler) Me(c fiber.Ctx) error {
	// Get user_id from context (set by auth middleware)
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no user context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid user context",
		})
	}

	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Resolve the organization name so clients can show the user's org scope
	// without a second request. Best-effort: omit on lookup failure.
	var organizationName string
	if org, orgErr := h.orgRepo.GetByID(user.OrganizationID); orgErr == nil && org != nil {
		organizationName = org.Name
	}

	return c.JSON(fiber.Map{
		"id":               user.ID,
		"email":            user.Email,
		"name":             user.Name,
		"role":             user.Role,
		"organizationId":   user.OrganizationID,
		"organizationName": organizationName,
		"lastLoginAt":      user.LastLoginAt,
		"createdAt":        user.CreatedAt,
		"status":           user.Status,
	})
}

// LocalLogin handles email/password authentication
func (h *AuthHandler) LocalLogin(c fiber.Ctx) error {
	type LoginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Authenticate user (this also updates last_login_at)
	user, err := h.authService.LoginWithPassword(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Generate JWT tokens
	accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
		user.ID.String(),
		user.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	// Audit log successful login
	h.auditService.LogAction(
		c.Context(),
		user.OrganizationID,
		user.ID,
		domain.AuditActionLogin,
		"user",
		user.ID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"email":  user.Email,
			"role":   user.Role,
			"method": "password",
		},
	)

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

// ChangePassword handles password change requests
func (h *AuthHandler) ChangePassword(c fiber.Ctx) error {
	type ChangePasswordRequest struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	var req ChangePasswordRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Current password and new password are required",
		})
	}

	// Get user_id from context (set by auth middleware)
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no user context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid user context",
		})
	}

	// Change password
	err := h.authService.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
	})
}

// GetCurrentOrganization returns the current user's organization
func (h *AuthHandler) GetCurrentOrganization(c fiber.Ctx) error {
	// Get organization_id from context (set by auth middleware)
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no organization context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid organization context",
		})
	}

	// Get organization from repository
	org, err := h.orgRepo.GetByID(orgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	// Return organization info
	return c.JSON(fiber.Map{
		"id":        org.ID,
		"name":      org.Name,
		"maxAgents": org.MaxAgents,
		"isActive":  org.IsActive,
		"createdAt": org.CreatedAt,
		"updatedAt": org.UpdatedAt,
	})
}

// Logout clears authentication
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	// The audit row. This route sits in the public auth group, so no
	// middleware sets the principal; the row comes from the presented token's
	// own claims: the bearer access token, or the refresh token the client
	// sends in the body. Cookies are not credentials here. One row per logout
	// with a valid token; garbage tokens record nothing. The row carries
	// identifiers only, never a token.
	var req LogoutRequest
	_ = c.Bind().JSON(&req)
	refreshToken := req.RefreshToken
	if h.auditService != nil && h.jwtService != nil {
		if claims, source := h.logoutPrincipal(bearerToken(c), refreshToken); claims != nil {
			userID, err1 := uuid.Parse(claims.UserID)
			orgID, err2 := uuid.Parse(claims.OrganizationID)
			if err1 == nil && err2 == nil {
				meta := fiber.Map{"token": source, "jti": claims.ID}
				if family := claims.FamilyID(); family != "" {
					meta["familyId"] = family
				}
				if err := h.auditService.LogAction(c.Context(), orgID, userID, domain.AuditActionLogout, "user", userID, c.IP(), c.Get("User-Agent"), meta); err != nil {
					log.Printf("⚠️  Logout: audit row not written: %v", err)
				}
			}
		}
	}

	revoked := fiber.Map{"accessToken": false, "refreshToken": false}
	if accessToken := bearerToken(c); accessToken != "" && h.jwtService != nil {
		ok, _ := h.jwtService.RevokeTokenChecked(c.Context(), accessToken)
		revoked["accessToken"] = ok
	}
	if refreshToken != "" && h.jwtService != nil {
		ok, _ := h.jwtService.RevokeSessionChecked(c.Context(), refreshToken)
		revoked["refreshToken"] = ok
	}

	// Clear the session cookies that earlier versions set at sign-in. They are
	// not read as credentials; this only removes them from the browser.
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		HTTPOnly: true,
		MaxAge:   -1,
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HTTPOnly: true,
		MaxAge:   -1,
	})

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
		"revoked": revoked,
	})
}

// LogoutRequest is the optional JSON body of POST /api/v1/auth/logout: the
// refresh token to revoke. No validation tag: a request without a body must
// keep logging out (the bearer is still revoked).
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// bearerToken extracts the JWT from the Authorization header ("Bearer <token>").
// The access_token cookie is not a credential.
func bearerToken(c fiber.Ctx) string {
	if authHeader := c.Get("Authorization"); authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	return ""
}

// logoutPrincipal returns the claims the logout row is written from: the
// bearer access token when it validates, else the refresh token when it does.
func (h *AuthHandler) logoutPrincipal(bearer, refresh string) (*auth.JWTClaims, string) {
	if bearer != "" {
		if claims, err := h.jwtService.ValidateToken(bearer); err == nil && claims.TokenType == auth.TokenTypeAccess {
			return claims, "access"
		}
	}
	if refresh != "" {
		if claims, err := h.jwtService.ValidateToken(refresh); err == nil && claims.TokenType != auth.TokenTypeAccess {
			return claims, "refresh"
		}
	}
	return nil, ""
}
