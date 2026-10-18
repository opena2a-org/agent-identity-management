package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
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
	// A password change ends every session from before it: a token whose
	// sign-in (auth_time, or its own iat for an SDK-download token) precedes
	// the recorded change is answered like an expired one.
	if auth.SignedInBeforePasswordChange(claims, user.PasswordChangedAt) {
		return nil, "Invalid or expired refresh token"
	}
	return user, ""
}

// presenter is the client presenting a token on this request: the
// trusted-proxy client address (identical to c.IP() when TRUSTED_PROXIES is
// unset) and the raw User-Agent header. Both are copied: a header value is
// a view of the request buffer, which the server reuses for the next
// request, and the values are stored on an sdk_tokens row and in an audit
// row.
func presenter(c fiber.Ctx) auth.Client {
	return auth.Client{Address: strings.Clone(middleware.ClientIP(c)), UserAgent: strings.Clone(c.Get("User-Agent"))}
}

// recordRefusal records one refused presentation of a revoked token: a
// SECURITY log line (the operator's record, kept even when the audit insert
// fails) and one audit row in the tenant's log. Both carry identifiers only
// (user, organization, family, jti, client address and user agent), never a
// token or a client mark. clientMatch, when not empty, is the reuse's
// classification (sameClient, differentClient or unknown). nil-safe on the
// audit service for handlers built by struct literal.
func (h *AuthRefreshHandler) recordRefusal(c fiber.Ctx, claims *auth.JWTClaims, action domain.AuditAction, familyRevoked *bool, clientMatch string) {
	meta := map[string]interface{}{}
	extra := ""
	if familyRevoked != nil {
		meta["familyRevoked"] = *familyRevoked
		extra = fmt.Sprintf(" familyRevoked=%t", *familyRevoked)
	}
	if clientMatch != "" {
		meta["clientMatch"] = clientMatch
		extra += " clientMatch=" + clientMatch
	}
	h.recordRefusalWith(c, claims, action, meta, extra)
}

// recordRefusalWith is recordRefusal with the record's own fields: meta joins
// the audit row's identifiers, extra the log line's.
func (h *AuthRefreshHandler) recordRefusalWith(c fiber.Ctx, claims *auth.JWTClaims, action domain.AuditAction, meta map[string]interface{}, extra string) {
	family, jti := claims.FamilyID(), claims.ID
	ip, ua := recordedClientIP(c), c.Get("User-Agent")
	meta["familyId"] = family
	meta["jti"] = jti
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
		log.Printf("Refresh: audit row %s for family %s not written: %v", action, family, err)
	}
}

// sdkRowRefused records why an SDK-download token whose sdk_tokens row is no
// longer active was refused. A row the server retired by rotation, presented
// again, is a reuse and ends the family. A row revoked with its family is a
// member of an ended family and is recorded as one, as a login family member
// is. A row its owner revoked records nothing, and neither does any token of
// another issuer tracked in the table.
func (h *AuthRefreshHandler) sdkRowRefused(c fiber.Ctx, token string, row *domain.SDKToken) {
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil || claims.Issuer != auth.IssuerSDK || claims.FamilyID() == "" {
		return
	}
	switch {
	case row.RevokedFor(domain.SDKTokenRevokeReasonRotation):
		h.endFamilyForReuse(c, claims, h.classifySDKRowReuse(c, row))
	case row.RevokedFor(domain.SDKTokenRevokeReasonFamilyRevoked):
		h.recordRefusal(c, claims, domain.AuditActionRefreshSessionRevoked, nil, "")
	}
}

// classifySDKRowReuse classifies a reuse detected by an sdk_tokens row, which
// no denylist mark describes: the row rotation created from the reused row
// (its successor) stores the address and user agent of the client that
// rotated, and the presenter is compared with them under the mark's rule
// (auth.CompareClients). It is unknown when the successor row cannot be read
// or stores no client, or an address has no class.
func (h *AuthRefreshHandler) classifySDKRowReuse(c fiber.Ctx, row *domain.SDKToken) string {
	if h.sdkTokenService == nil {
		return auth.ClientMatchUnknown
	}
	next, err := h.sdkTokenService.SuccessorOf(c.Context(), row)
	if err != nil || next.IPAddress == nil || next.UserAgent == nil {
		return auth.ClientMatchUnknown
	}
	return auth.CompareClients(auth.Client{Address: *next.IPAddress, UserAgent: *next.UserAgent}, presenter(c))
}

// reuseDetected handles a refresh token that was retired and was presented
// again (RFC 9700 section 4.14.2): a login token denylisted by rotation or
// logout. The record says whether the presenter is the client that retired
// the token (a race with itself) or not, read from the denylist mark
// (auth.ClassifyReuse). An SDK-download token whose sdk_tokens row was
// retired by rotation is classified from its successor row instead
// (sdkRowRefused). A token without a family (an access or service token)
// records nothing here.
func (h *AuthRefreshHandler) reuseDetected(c fiber.Ctx, token string) {
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil || claims.FamilyID() == "" {
		return
	}
	h.endFamilyForReuse(c, claims, h.jwtService.ClassifyReuse(c.Context(), claims.ID, presenter(c)))
}

// endFamilyForReuse ends the family of a reused refresh token and records the
// reuse with its classification. Whoever holds the chain that grew from the
// token cannot be told apart from the legitimate client, so the whole family
// is revoked; enforcement is the same whatever the classification. An
// SDK-download family's sdk_tokens rows are revoked too, so the family ends
// even where no revocation store is configured; the record's familyRevoked
// is true when either write succeeded.
func (h *AuthRefreshHandler) endFamilyForReuse(c fiber.Ctx, claims *auth.JWTClaims, clientMatch string) {
	familyRevoked, revokeErr := h.jwtService.RevokeFamily(c.Context(), claims)
	if revokeErr != nil {
		log.Printf("Refresh: family %s could not be revoked after a reuse: %v", claims.FamilyID(), revokeErr)
	}
	if claims.Issuer == auth.IssuerSDK && h.sdkTokenService != nil {
		if userID, err := uuid.Parse(claims.UserID); err == nil {
			if rowsErr := h.sdkTokenService.RevokeFamily(c.Context(), userID, claims.FamilyID(), domain.SDKTokenRevokeReasonFamilyRevoked); rowsErr == nil {
				familyRevoked = true
			} else {
				log.Printf("Refresh: sdk_tokens rows of family %s could not be revoked after a reuse: %v", claims.FamilyID(), rowsErr)
			}
		}
	}
	h.recordRefusal(c, claims, domain.AuditActionRefreshTokenReuse, &familyRevoked, clientMatch)
}

// rotateSDKTokenRow revokes the presented token's sdk_tokens row (oldToken,
// found by oldTokenHash) and creates the row of its successor,
// newRefreshToken, in one transaction: both happen or neither does. The new
// row carries the old row's owner and device forward, its usage count plus
// this refresh, and its lineage in metadata.
//
// SECURITY: each SDK instance has its own token from the download flow, so
// retiring one instance's old token doesn't affect other instances.
func (h *AuthRefreshHandler) rotateSDKTokenRow(c fiber.Ctx, oldToken *domain.SDKToken, oldTokenHash, rotatedFrom, newRefreshToken string) error {
	newTokenID, err := h.jwtService.GetTokenID(newRefreshToken)
	if err != nil {
		return fmt.Errorf("new refresh token id unreadable: %w", err)
	}
	if newTokenID == "" {
		return errors.New("new refresh token carries no id")
	}

	newHasher := sha256.New()
	newHasher.Write([]byte(newRefreshToken))
	newTokenHash := hex.EncodeToString(newHasher.Sum(nil))

	// The rotating client, as the route saw it: the trusted-proxy client
	// address and the raw User-Agent header. A later reuse of oldToken is
	// classified by comparing its presenter with this row (classifySDKRowReuse),
	// so the row stores what the comparison reads. A forwarded address that
	// does not parse is not stored; the connecting address is.
	client := presenter(c)
	newIPAddress, userAgent := client.Address, client.UserAgent
	if net.ParseIP(newIPAddress) == nil {
		newIPAddress = c.IP()
	}

	// Get rotation count from old token metadata
	rotationCount := 1
	if oldToken.Metadata != nil {
		if count, ok := oldToken.Metadata["rotationCount"].(float64); ok {
			rotationCount = int(count) + 1
		}
	}

	// IMPORTANT: Carry forward the parent token's usage count so it's cumulative.
	// The family's birth travels with the chain: the successor carries the
	// presented row's familyCreatedAt, or that row's own created_at when it
	// carries none (the row is then the root, or a row from before the birth
	// was carried), so the birth survives a purge of the root row and a
	// rotation never makes a family younger.
	now := time.Now()
	next := &domain.SDKToken{
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
			"rotated_from":  rotatedFrom,
			"rotationCount": rotationCount,
			"parent_token":  oldToken.ID.String(), // Track token lineage
		},
	}
	next.SetFamilyCreatedAt(oldToken.FamilyCreatedAt())
	return h.sdkTokenService.RotateToken(c.Context(), oldTokenHash, "token_rotation", next)
}

// familyPastMaxAge refuses an SDK-download token presented past its family's
// maximum age (SDK_TOKEN_FAMILY_MAX_AGE, counted from the row the download
// wrote): the presented row is revoked for the family's age, so nothing of the
// chain stays live, the refusal is recorded once (an audit row and a SECURITY
// line, identifiers only), and the token is answered as an expired token is.
// Reports false, with nothing written, for every other token.
func (h *AuthRefreshHandler) familyPastMaxAge(c fiber.Ctx, claims *auth.JWTClaims, row *domain.SDKToken, tokenHash string, now time.Time) bool {
	if claims.Issuer != auth.IssuerSDK || row == nil {
		return false
	}
	born := row.FamilyCreatedAt()
	if now.Sub(born) <= h.jwtService.SDKTokenFamilyMaxAge() {
		return false
	}
	if err := h.sdkTokenService.RevokeByTokenHash(c.Context(), tokenHash, domain.SDKTokenRevokeReasonFamilyMaxAge); err != nil {
		log.Printf("Refresh: sdk_tokens row of family %s past its maximum age could not be revoked: %v", claims.FamilyID(), err)
	}
	bornAt := born.UTC().Format(time.RFC3339)
	h.recordRefusalWith(c, claims, domain.AuditActionRefreshFamilyMaxAge, map[string]interface{}{"familyCreatedAt": bornAt}, " familyCreatedAt="+bornAt)
	return true
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

	// The presented token's active sdk_tokens row, when it has one (an
	// SDK-download token, or a recovered credential), and the hash it is
	// found by; the family's maximum age is read from the row below.
	var sdkRow *domain.SDKToken
	var tokenHash string
	if err == nil && tokenID != "" {
		// Hash the token to check if it's tracked and revoked
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		tokenHash = hex.EncodeToString(hasher.Sum(nil))

		// Check if token is tracked in SDK tokens table
		sdkToken, err := h.sdkTokenService.GetByTokenHash(c.Context(), tokenHash)
		if err == nil && sdkToken != nil {
			// Token is tracked in SDK table - verify it's still active
			if !sdkToken.IsActive() {
				h.sdkRowRefused(c, req.RefreshToken, sdkToken)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Token has been revoked or is invalid",
				})
			}
			sdkRow = sdkToken
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
				h.recordRefusal(c, claims, domain.AuditActionRefreshSessionRevoked, nil, "")
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
	now := time.Now()
	if h.jwtService.SessionExpired(claims, now) {
		log.Printf("Refresh: sign-in past the maximum session age refused user=%s org=%s family=%s signedIn=%s ip=%s ua=%q",
			claims.UserID, claims.OrganizationID, claims.FamilyID(), claims.SignedInAt().UTC().Format(time.RFC3339), recordedClientIP(c), c.Get("User-Agent"))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}
	// An SDK-download token family lives at most SDK_TOKEN_FAMILY_MAX_AGE
	// from the download, however often it rotates. Past it the presented
	// token is answered as above, before the account lookup, and its row is
	// revoked; the holder downloads a new SDK with a sign-in.
	if h.familyPastMaxAge(c, claims, sdkRow, tokenHash, now) {
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
	// An SDK-download family is born at its download, and each rotation gives
	// its new member a fresh iat. A family downloaded before the last
	// password change is refused by its birth as well, so a rotation in
	// flight when the password changed cannot carry the family past it.
	if claims.Issuer == auth.IssuerSDK && sdkRow != nil && user.PasswordChangedAt != nil &&
		sdkRow.FamilyCreatedAt().Before(auth.PasswordChangeBoundary(*user.PasswordChangedAt)) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}
	// While the account must change its password its session is not renewed:
	// the auth middleware admits it only to GET /auth/me and
	// POST /auth/change-password, and the change issues a new pair.
	if user.ForcePasswordChange {
		return middleware.PasswordChangeRequired(c)
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
	// SDK-download tokens are retired by row in sdk_tokens below, and the
	// same holds for them: a presentation whose row another presentation
	// retired first is a reuse. Minting before retiring means a signing
	// failure leaves the client with the token it still holds; only a response
	// lost after the write costs a re-login.
	// The retirement is atomic on a store with set-if-absent: two presentations
	// of one token within a request's duration cannot both retire it. The one
	// that loses the write is a reuse (the same token presented twice), so the
	// family is revoked, the reuse recorded, and the minted tokens are not
	// handed out.
	rotated := false
	if claims.Issuer == auth.IssuerUser {
		retired, lost, revokeErr := h.jwtService.RetireTokenCheckedFrom(c.Context(), req.RefreshToken, presenter(c))
		if lost {
			h.reuseDetected(c, req.RefreshToken)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked or is invalid",
			})
		}
		if retired {
			rotated = true
		} else {
			newRefreshToken = req.RefreshToken
			if revokeErr != nil {
				log.Printf("Refresh: denylist write failed for jti %s; the presented refresh token was returned unchanged: %v", tokenID, revokeErr)
			}
		}
	}

	// SDK-download tokens (a different issuer) are tracked by row in
	// sdk_tokens: the old token's row is revoked and the new token's row
	// created in one transaction, and the new token is handed out only when
	// that transaction committed. When it did not (the old row is no longer
	// active, the insert failed, or the new row could not be built), neither
	// write took effect and the presented SDK token goes back unchanged with a
	// fresh access token (rotated=false), so an SDK client never holds two
	// live refresh tokens, nor a new one without a row, which the next refresh
	// would refuse. A login-issued token with a row (a recovered SDK token)
	// was retired by the denylist above; its row follows the same rotation,
	// and a failure there is logged without changing the answer.
	if tokenID != "" {
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		oldTokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Get old token info for creating new token entry
		oldToken, lookupErr := h.sdkTokenService.ValidateToken(c.Context(), oldTokenHash)

		// Record usage on the old token (updates last_used_at, usage_count).
		// This is bookkeeping: a failed write is logged and the refresh proceeds.
		if err := h.sdkTokenService.RecordTokenUsage(c.Context(), tokenID, c.IP()); err != nil {
			log.Printf("Refresh: usage of token id %s not recorded: %v", tokenID, err)
		}

		if newRefreshToken != req.RefreshToken && (claims.Issuer == auth.IssuerSDK || oldToken != nil) {
			var rotateErr error
			if oldToken == nil {
				rotateErr = fmt.Errorf("no active sdk_tokens row: %v", lookupErr)
			} else {
				rotateErr = h.rotateSDKTokenRow(c, oldToken, oldTokenHash, tokenID, newRefreshToken)
			}
			if claims.Issuer != auth.IssuerSDK {
				if rotateErr != nil {
					log.Printf("Refresh: sdk_tokens rotation failed for token id %s: %v", tokenID, rotateErr)
				}
			} else if rotateErr == nil {
				rotated = true
			} else {
				newRefreshToken = req.RefreshToken
				log.Printf("Refresh: sdk_tokens rotation failed for token id %s; the presented refresh token was returned unchanged: %v", tokenID, rotateErr)
			}
		}
	}

	// An SDK-download token that did not rotate is checked once more. Its row
	// was active when it was read above; when it no longer is, another
	// presentation of the same token retired it in between, so this one lost
	// the retirement. It is answered as that check would answer it now (a row
	// retired by rotation is a reuse and ends the family, with any row this
	// presentation created), and what it minted is not handed out.
	if claims.Issuer == auth.IssuerSDK && !rotated && tokenID != "" {
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		row, err := h.sdkTokenService.GetByTokenHash(c.Context(), hex.EncodeToString(hasher.Sum(nil)))
		if err == nil && row != nil && !row.IsActive() {
			h.sdkRowRefused(c, req.RefreshToken, row)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked or is invalid",
			})
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
