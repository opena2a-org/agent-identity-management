package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// sdkTokenLogoutReason is the revoke_reason an SDK-download token's
// sdk_tokens row carries when the token was logged out.
const sdkTokenLogoutReason = "logout"

type AuthHandler struct {
	authService     *application.AuthService
	jwtService      *auth.JWTService
	orgRepo         domain.OrganizationRepository
	auditService    *application.AuditService
	sdkTokenService *application.SDKTokenService
	signInFailed    func(SignInFailure)
}

// SetSignInFailureObserver registers fn to receive each password sign-in
// LocalLogin refuses. fn runs on the request's goroutine, so it must not
// block. Set it before the handler serves requests; nil observes nothing.
func (h *AuthHandler) SetSignInFailureObserver(fn func(SignInFailure)) {
	h.signInFailed = fn
}

// NewAuthHandler builds the auth handler. The SDK token service retires an
// SDK-download token's sdk_tokens row at logout, so the dashboard's token list
// agrees with the denylist.
func NewAuthHandler(
	authService *application.AuthService,
	jwtService *auth.JWTService,
	orgRepo domain.OrganizationRepository,
	auditService *application.AuditService,
	sdkTokenService *application.SDKTokenService,
) *AuthHandler {
	return &AuthHandler{
		authService:     authService,
		jwtService:      jwtService,
		orgRepo:         orgRepo,
		auditService:    auditService,
		sdkTokenService: sdkTokenService,
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
		"lastLoginAt":      utcTime(user.LastLoginAt),
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
		reason, userID, orgID := signInFailedUnclassified, uuid.Nil, uuid.Nil
		var refusal *application.SignInRefusal
		isRefusal := errors.As(err, &refusal)
		if isRefusal {
			reason, userID, orgID = refusal.Reason, refusal.UserID, refusal.OrganizationID
		}
		reportSignInFailure(c, h.signInFailed, reason, userID, orgID)
		switch {
		case reason == application.SignInRefusedProviderSignInRequired:
			return c.Status(fiber.StatusForbidden).JSON(providerSignInRequiredAnswer(orgID))
		case !isRefusal:
			// The service answers an error that is not a refusal only after
			// the password matched: the organization's requirement of
			// provider sign-in could not be read.
			return respondServerError(c, fiber.StatusInternalServerError, err)
		}
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

	// Audit log successful login, naming the token family it issued. Metadata
	// is route facts only; the account is identified by the row's columns.
	recordSignIn(c, h.auditService, h.jwtService, refreshToken, fiber.Map{"method": "password"})

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
		if rule, ok := changePasswordRule(err, "currentPassword"); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": rule,
			})
		}
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	// The change ends every session signed in before it, this one included.
	// The session that made it is issued a new pair, minted from the user
	// read after the change so its sign-in time is not before the change's
	// boundary. When the pair cannot be issued, the change still stands and
	// the answer carries no tokens: the client signs in again. A user whose
	// organization requires sign-in through its identity provider is issued
	// no pair for a password, and signs in again through the provider.
	response := fiber.Map{"message": "Password changed successfully"}
	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		log.Printf("ChangePassword: user %s not read after the change, no new pair issued: %v", userID, err)
		return c.JSON(response)
	}
	if refused := h.authService.CheckPasswordSignIn(c.Context(), user); refused != nil {
		var refusal *application.SignInRefusal
		if errors.As(refused, &refusal) {
			response["code"] = refusal.Reason
			response["organizationId"] = user.OrganizationID
		} else {
			log.Printf("ChangePassword: no new pair issued for user %s: %v", userID, refused)
		}
		return c.JSON(response)
	}
	if h.jwtService != nil {
		accessToken, refreshToken, err := h.jwtService.GenerateTokenPairSince(
			user.ID.String(),
			user.OrganizationID.String(),
			user.Email,
			string(user.Role),
			user.PasswordChangedAt,
		)
		if err == nil {
			recordSignIn(c, h.auditService, h.jwtService, refreshToken, fiber.Map{"method": "password_change"})
			response["accessToken"] = accessToken
			response["refreshToken"] = refreshToken
		} else {
			log.Printf("ChangePassword: the new pair for user %s was not issued: %v", userID, err)
		}
	}

	return c.JSON(response)
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
	// identifiers only, never a token; its client address is recordedClientIP,
	// the same address the sign-in row and the refresh route's rows record.
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
				if err := h.auditService.LogAction(c.Context(), orgID, userID, domain.AuditActionLogout, "user", userID, recordedClientIP(c), c.Get("User-Agent"), meta); err != nil {
					log.Printf("Logout: audit row not written: %v", err)
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
		ok, _ := h.jwtService.RevokeSessionCheckedFrom(c.Context(), refreshToken, presenter(c))
		// An SDK-download token is also tracked by row in sdk_tokens, which the
		// dashboard lists, and its family ends by row: the logout retires the
		// presented row and revokes the family's still-active rows, with or
		// without a revocation store. The answer claims the revocation only
		// when those row writes succeeded and, where a store is configured,
		// the store's writes did too.
		if claims, err := h.jwtService.ValidateToken(refreshToken); err == nil && claims.Issuer == auth.IssuerSDK {
			rowsEnded := h.retireSDKTokenRow(c.Context(), refreshToken, claims)
			ok = rowsEnded && (ok || !h.jwtService.RevocationConfigured())
		}
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

// retireSDKTokenRow marks the sdk_tokens row of a logged-out SDK-download
// token revoked, then ends the token's family by row: every still-active row
// of the family is revoked with the logout reason, as a reuse revokes them,
// so a logout with any token of the family ends it. A presented row already
// retired (by rotation or an earlier logout) is the state logout wants and is
// kept as it is. Reports false when no SDK token service is wired, the token
// has no row, or a write failed.
func (h *AuthHandler) retireSDKTokenRow(ctx context.Context, token string, claims *auth.JWTClaims) bool {
	if h.sdkTokenService == nil {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	retired := true
	if revokeErr := h.sdkTokenService.RevokeByTokenHash(ctx, tokenHash, sdkTokenLogoutReason); revokeErr != nil {
		if row, err := h.sdkTokenService.GetByTokenHash(ctx, tokenHash); err != nil || row == nil || row.IsActive() {
			log.Printf("Logout: sdk_tokens row not revoked for token id %s: %v", claims.ID, revokeErr)
			retired = false
		}
	}
	return h.revokeSDKTokenFamily(ctx, claims) && retired
}

// revokeSDKTokenFamily revokes the still-active sdk_tokens rows of the
// logged-out SDK-download token's family, the family named by its claims and
// owned by their user; a token that names no family has none to end. Reports
// false when the owner cannot be read from the claims or the write failed.
func (h *AuthHandler) revokeSDKTokenFamily(ctx context.Context, claims *auth.JWTClaims) bool {
	family := claims.FamilyID()
	if family == "" {
		return true
	}
	userID, err := uuid.Parse(claims.UserID)
	if err == nil {
		err = h.sdkTokenService.RevokeFamily(ctx, userID, family, sdkTokenLogoutReason)
	}
	if err != nil {
		log.Printf("Logout: sdk_tokens family %s not revoked for token id %s: %v", family, claims.ID, err)
		return false
	}
	return true
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
