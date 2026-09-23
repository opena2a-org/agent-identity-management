package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// AuthRefreshHandler handles token refresh operations
type AuthRefreshHandler struct {
	jwtService      *auth.JWTService
	sdkTokenService *application.SDKTokenService
	users           domain.UserRepository
	audit           *application.AuditService
}

// NewAuthRefreshHandler creates a new auth refresh handler. A refresh token is
// an identity handle, not an authorization grant: the new access token's role
// and email are read from the user record at refresh time, so the user
// repository is mandatory — a nil one is a boot-time refusal, never a fallback
// to the token's own claims. The audit service records every refused
// presentation of a revoked token (a reuse, or a member of a revoked
// session) in the tenant's audit log; it is mandatory too.
func NewAuthRefreshHandler(jwtService *auth.JWTService, sdkTokenService *application.SDKTokenService, users domain.UserRepository, audit *application.AuditService) *AuthRefreshHandler {
	if users == nil {
		panic("NewAuthRefreshHandler: user repository is required")
	}
	if audit == nil {
		panic("NewAuthRefreshHandler: audit service is required")
	}
	return &AuthRefreshHandler{
		jwtService:      jwtService,
		sdkTokenService: sdkTokenService,
		users:           users,
		audit:           audit,
	}
}

// refreshPrincipal resolves the account behind a refresh token from the
// database and decides whether it may still hold a session. It returns the
// user to mint from, or the 401 reason. No path falls back to the token's
// embedded role or email.
func (h *AuthRefreshHandler) refreshPrincipal(claims *auth.JWTClaims) (*domain.User, string) {
	if claims.TokenType == auth.TokenTypeAccess || claims.Issuer == auth.IssuerService {
		return nil, "Invalid or expired refresh token"
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, "Invalid or expired refresh token"
	}
	user, err := h.users.GetByID(userID)
	if err != nil || user == nil {
		return nil, "Invalid or expired refresh token"
	}
	if user.OrganizationID.String() != claims.OrganizationID {
		return nil, "Invalid or expired refresh token"
	}
	if !user.CanHoldSession() {
		return nil, "Account is not active"
	}
	return user, ""
}

// recordRefusal records one refused presentation of a revoked token: a
// SECURITY log line (the operator's record, kept even when the audit insert
// fails) and one audit row in the tenant's log. Both carry identifiers only
// (user, organization, family, jti, client address and user agent), never a
// token. nil-safe on the audit service for handlers built by struct literal.
func (h *AuthRefreshHandler) recordRefusal(c fiber.Ctx, claims *auth.JWTClaims, action domain.AuditAction, familyRevoked *bool) {
	family, jti := claims.FamilyID(), claims.ID
	ip, ua := c.IP(), c.Get("User-Agent")
	meta := map[string]interface{}{"familyId": family, "jti": jti}
	extra := ""
	if familyRevoked != nil {
		meta["familyRevoked"] = *familyRevoked
		extra = fmt.Sprintf(" familyRevoked=%t", *familyRevoked)
	}
	log.Printf("SECURITY %s user=%s org=%s family=%s jti=%s%s ip=%s ua=%q", action, claims.UserID, claims.OrganizationID, family, jti, extra, ip, ua)
	if h.audit == nil {
		return
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return
	}
	orgID, err := uuid.Parse(claims.OrganizationID)
	if err != nil {
		return
	}
	if err := h.audit.LogAction(c.Context(), orgID, userID, action, "user", userID, ip, ua, meta); err != nil {
		log.Printf("⚠️  Refresh: audit row %s for family %s not written: %v", action, family, err)
	}
}

// reuseDetected handles a login refresh token that is denylisted and was
// presented again (RFC 9700 section 4.14.2): whoever holds the chain that
// grew from it cannot be told apart from the legitimate client, so the whole
// family is revoked and the event recorded. Tokens without a family (SDK
// download tokens, which are retired by row) record nothing here.
func (h *AuthRefreshHandler) reuseDetected(c fiber.Ctx, token string) {
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil || claims.FamilyID() == "" {
		return
	}
	familyRevoked, revokeErr := h.jwtService.RevokeFamily(c.Context(), claims)
	if revokeErr != nil {
		log.Printf("⚠️  Refresh: family %s could not be revoked after a reuse: %v", claims.FamilyID(), revokeErr)
	}
	h.recordRefusal(c, claims, domain.AuditActionRefreshTokenReuse, &familyRevoked)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token (with token rotation)
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthRefreshHandler) RefreshToken(c fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "refresh_token is required",
		})
	}

	// Check if this is an SDK token and verify it's not revoked BEFORE rotating
	tokenID, err := h.jwtService.GetTokenID(req.RefreshToken)

	// SECURITY: a refresh token revoked on logout or retired by rotation must
	// not be able to mint new tokens. (SDK tokens are additionally checked
	// against the DB table below.) The read is uncached: every refresh token
	// is presented once, so a stale "not revoked" entry would only ever serve
	// a replay. A presentation the store confirmed as revoked is a reuse of a
	// rotated-out token (a logged-out token is denylisted the same way): the
	// whole family is revoked and the event recorded. When the store did not
	// answer, enforcement follows the fail-open setting and nothing is
	// recorded, since there is no evidence.
	if err == nil && tokenID != "" {
		revoked, known := h.jwtService.CheckRevoked(c.Context(), tokenID)
		if revoked {
			if known {
				h.reuseDetected(c, req.RefreshToken)
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked or is invalid",
			})
		}
	}

	if err == nil && tokenID != "" {
		// Hash the token to check if it's tracked and revoked
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		tokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Check if token is tracked in SDK tokens table
		sdkToken, err := h.sdkTokenService.GetByTokenHash(c.Context(), tokenHash)
		if err == nil && sdkToken != nil {
			// Token is tracked in SDK table - verify it's still active
			if !sdkToken.IsActive() {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Token has been revoked or is invalid",
				})
			}
		} else {
			// Token NOT found in SDK table - check if it was issued as an SDK token.
			// SDK tokens have issuer "agent-identity-management-sdk". If a token claims
			// to be an SDK token but has no database record, it was deleted/purged and
			// should be rejected to prevent deleted SDK tokens from being refreshed.
			claims, validateErr := h.jwtService.ValidateToken(req.RefreshToken)
			if validateErr == nil && claims != nil && claims.Issuer == "agent-identity-management-sdk" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Token has been revoked or is invalid",
				})
			}
			// Regular login tokens (issuer "agent-identity-management") are not tracked
			// in the SDK table and are allowed to refresh normally.
		}
	}

	// Resolve the principal from the database: role and email come from the
	// user record, never from the refresh token's claims (login-issued refresh
	// tokens carry none, and an embedded role would outlive a demotion).
	claims, err := h.jwtService.ValidateToken(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// A member of a revoked family (a sign-in ended by a reuse, or by logout)
	// is refused before the principal lookup, with the same answer as a
	// revoked token, so the refusal tells the presenter nothing more.
	if family := claims.FamilyID(); family != "" {
		revoked, known := h.jwtService.CheckFamilyRevoked(c.Context(), family)
		if revoked {
			if known {
				h.recordRefusal(c, claims, domain.AuditActionRefreshSessionRevoked, nil)
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked or is invalid",
			})
		}
	}

	// A sign-in lasts at most JWT_SESSION_MAX_AGE however often it rotates.
	// Past it the presented token is answered like an expired token, before
	// the account lookup, so an ended sign-in learns nothing about the
	// account; the refusal is logged with identifiers only.
	if h.jwtService.SessionExpired(claims, time.Now()) {
		log.Printf("Refresh: sign-in past the maximum session age refused user=%s org=%s family=%s signedIn=%s ip=%s ua=%q",
			claims.UserID, claims.OrganizationID, claims.FamilyID(), claims.SignedInAt().UTC().Format(time.RFC3339), c.IP(), c.Get("User-Agent"))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}
	user, refusal := h.refreshPrincipal(claims)
	if refusal != "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": refusal,
		})
	}

	// Validate refresh token and generate new tokens (with rotation)
	newAccessToken, newRefreshToken, err := h.jwtService.RefreshTokenPair(req.RefreshToken, user.Email, string(user.Role))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// Rotation, in this order: validate, mint, retire, answer. A new refresh
	// token is handed out only when the presented one was actually retired, so
	// a login chain never holds two live refresh tokens. A login-issued token
	// is retired by writing its jti to the denylist for its remaining lifetime;
	// when that cannot happen (no revocation store, or the store refused the
	// write, under either fail-open setting) the presented token goes back
	// unchanged with a fresh access token, and the answer says so (rotated).
	// SDK-download tokens are retired by row in sdk_tokens below. Minting
	// before retiring means a signing failure leaves the client with the token
	// it still holds; only a response lost after the write costs a re-login.
	rotated := false
	if claims.Issuer == auth.IssuerUser {
		retired, revokeErr := h.jwtService.RevokeTokenChecked(c.Context(), req.RefreshToken)
		if retired {
			rotated = true
		} else {
			newRefreshToken = req.RefreshToken
			if revokeErr != nil {
				log.Printf("⚠️  Refresh: denylist write failed for jti %s; the presented refresh token was returned unchanged: %v", tokenID, revokeErr)
			}
		}
	}

	// SDK-download tokens (a different issuer) are tracked by row: the old
	// token's row is revoked by hash and a new row is created for the new
	// token. Rotation is reported only when that revocation succeeded.
	if tokenID != "" {
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		oldTokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Get old token info for creating new token entry
		oldToken, _ := h.sdkTokenService.ValidateToken(c.Context(), oldTokenHash)

		// Record usage on the old token (updates last_used_at, usage_count)
		ipAddress := c.IP()
		_ = h.sdkTokenService.RecordTokenUsage(c.Context(), tokenID, ipAddress)

		// SECURITY: Revoke the old refresh token after rotation
		// Each SDK instance has its own token from the download flow, so revoking
		// one instance's old token doesn't affect other instances.
		if revokeErr := h.sdkTokenService.RevokeByTokenHash(c.Context(), oldTokenHash, "token_rotation"); revokeErr == nil {
			if claims.Issuer == auth.IssuerSDK {
				rotated = true
			}
		} else {
			log.Printf("⚠️  Refresh: sdk_tokens revocation failed for token id %s: %v", tokenID, revokeErr)
		}

		// Save the new rotated SDK token to database
		if oldToken != nil {
			// Get new token ID from rotated refresh token
			newTokenID, err := h.jwtService.GetTokenID(newRefreshToken)
			if err == nil && newTokenID != "" {
				// Hash the new token
				newHasher := sha256.New()
				newHasher.Write([]byte(newRefreshToken))
				newTokenHash := hex.EncodeToString(newHasher.Sum(nil))

				// Get client info
				newIPAddress := c.IP()
				userAgent := c.Get("User-Agent")

				// Get rotation count from old token metadata
				rotationCount := 1
				if oldToken.Metadata != nil {
					if count, ok := oldToken.Metadata["rotationCount"].(float64); ok {
						rotationCount = int(count) + 1
					}
				}

				// Create new SDK token entry (old token remains valid until expiry)
				// IMPORTANT: Carry forward the parent token's usage count so it's cumulative
				now := time.Now()
				newSDKToken := &domain.SDKToken{
					ID:                uuid.New(),
					UserID:            oldToken.UserID,
					OrganizationID:    oldToken.OrganizationID,
					TokenHash:         newTokenHash,
					TokenID:           newTokenID,
					DeviceName:        oldToken.DeviceName,
					DeviceFingerprint: oldToken.DeviceFingerprint,
					IPAddress:         &newIPAddress,
					UserAgent:         &userAgent,
					UsageCount:        oldToken.UsageCount + 1, // Carry forward parent's usage count + 1 for this refresh
					LastUsedAt:        &now,                    // Token is being used right now
					CreatedAt:         now,
					ExpiresAt:         now.Add(90 * 24 * time.Hour), // 90 days
					Metadata: map[string]interface{}{
						"source":        "token_rotation",
						"rotated_from":  tokenID,
						"rotationCount": rotationCount,
						"parent_token":  oldToken.ID.String(), // Track token lineage
					},
				}

				// Save to database (critical for next rotation)
				_ = h.sdkTokenService.CreateToken(c.Context(), newSDKToken)
			}
		}
	}

	// Return new tokens. Report the REAL access-token lifetime (JWT_ACCESS_TTL)
	// so clients schedule their refresh on the correct cadence.
	return c.JSON(RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    h.jwtService.AccessTTLSeconds(),
		Rotated:      rotated,
	})
}

// Request/Response types
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"` // New refresh token (token rotation)
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
	// Rotated is true only when the presented refresh token was retired and a
	// new one issued; false means RefreshToken is the presented token, unchanged.
	Rotated bool `json:"rotated"`
}
