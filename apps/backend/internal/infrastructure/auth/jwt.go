package auth

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token issuers. The issuer doubles as a coarse token-type marker so that a
// long-lived SDK refresh token cannot be presented as a user access token.
// SECURITY: the access-auth middleware rejects tokens minted with IssuerSDK.
const (
	// IssuerUser is the issuer for interactive user access + refresh tokens.
	IssuerUser = "agent-identity-management"
	// IssuerSDK is the issuer for the long-lived (90-day) SDK refresh token,
	// which is only valid at the /auth/refresh endpoint — never as a bearer.
	IssuerSDK = "agent-identity-management-sdk"
	// IssuerService is the issuer for tokens minted by the OAuth token endpoint
	// (RFC 7523 jwt-bearer) to an authenticated *agent*. These are machine
	// principals, not humans: they carry no user role and must never traverse a
	// human role gate. SECURITY: the access-auth middleware rejects this issuer,
	// so a service token is only usable behind ServicePrincipalMiddleware, which
	// additionally pins the token to its own agent ID.
	IssuerService = "agent-identity-management-service"
)

// Token types. The `typ` claim separates access tokens (valid as a bearer) from
// refresh tokens (valid only at /auth/refresh), so a refresh token cannot be
// replayed as a session token.
// GRACE: tokens minted before this claim existed have an empty TokenType.
// The access middleware no longer accepts an empty type (retired once all
// short-lived legacy access tokens had aged out within their TTL). The refresh
// endpoint still treats an empty type as a legacy refresh/SDK token and allows
// it, so long-lived (up to 90-day SDK) tokens keep working until they age out.
// Newly issued tokens always carry a type and are enforced immediately.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	TokenTypeSDK     = "sdk"
)

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	// TokenType is "access" | "refresh" | "sdk". Empty on legacy tokens.
	TokenType string `json:"typ,omitempty"`
	jwt.RegisteredClaims
}

// JWTService handles JWT operations
type JWTService struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	revoker       *TokenRevoker
}

// SetRevoker attaches a token-revocation store. Optional: if never set,
// revocation is disabled and IsRevoked always returns false.
func (s *JWTService) SetRevoker(r *TokenRevoker) {
	s.revoker = r
}

// IsRevoked reports whether the token identified by jti has been revoked.
func (s *JWTService) IsRevoked(ctx context.Context, jti string) bool {
	return s.revoker.IsRevoked(ctx, jti)
}

// RevokeToken denylists the given token (by its jti) for its remaining lifetime.
// Best-effort: an invalid/expired token or a missing revoker is a no-op.
func (s *JWTService) RevokeToken(ctx context.Context, tokenString string) error {
	if s.revoker == nil || tokenString == "" {
		return nil
	}
	claims, err := s.ValidateToken(tokenString)
	if err != nil || claims.ExpiresAt == nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}
	return s.revoker.Revoke(ctx, claims.ID, ttl)
}

// NewJWTService creates a new JWT service.
// Access tokens default to 2h (JWT_ACCESS_TTL); refresh tokens to 7d
// (JWT_REFRESH_TTL) with rotation. The client enforces a shorter idle timeout
// on top of the absolute access-token expiry.
func NewJWTService() *JWTService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		panic("JWT_SECRET environment variable is required")
	}

	// SECURITY: Validate JWT secret minimum length (256 bits = 32 bytes recommended)
	if len(secret) < 32 {
		panic("JWT_SECRET must be at least 32 characters for security")
	}

	// Get expiry durations from env or use secure defaults
	// Access tokens last 2 hours for better UX during active sessions
	// Refresh tokens last 7 days and support rotation
	defaultAccessExpiry := 2 * time.Hour
	defaultRefreshExpiry := 168 * time.Hour

	accessExpiry, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "2h"))
	if err != nil {
		log.Printf("WARNING: invalid JWT_ACCESS_TTL, using default 2h: %v", err)
		accessExpiry = defaultAccessExpiry
	}
	refreshExpiry, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		log.Printf("WARNING: invalid JWT_REFRESH_TTL, using default 168h: %v", err)
		refreshExpiry = defaultRefreshExpiry
	}

	return &JWTService{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// AccessTTLSeconds returns the configured access-token lifetime in seconds.
// Used so the refresh endpoint reports the real expiry to clients instead of
// a hard-coded value.
func (s *JWTService) AccessTTLSeconds() int {
	return int(s.accessExpiry.Seconds())
}

// getEnv is a helper function to get env var with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GenerateSDKRefreshToken generates a refresh token for SDK usage (90 days)
// This token is embedded in downloaded SDKs for auto-authentication
// Security: Reduced from 1 year to 90 days to minimize exposure window
func (s *JWTService) GenerateSDKRefreshToken(userID, orgID, email, role string) (string, error) {
	now := time.Now()
	sdkExpiry := 90 * 24 * time.Hour // 90 days (reduced from 365 for security)

	claims := JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		TokenType:      TokenTypeSDK,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(sdkExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    IssuerSDK,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// GenerateTokenPair generates access and refresh tokens
func (s *JWTService) GenerateTokenPair(userID, orgID, email, role string) (accessToken, refreshToken string, err error) {
	// Generate access token
	accessToken, err = s.GenerateAccessToken(userID, orgID, email, role)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshToken, err = s.GenerateRefreshToken(userID, orgID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// GenerateAccessToken generates an access token
func (s *JWTService) GenerateAccessToken(userID, orgID, email, role string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		TokenType:      TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    IssuerUser,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// GenerateServiceToken mints an access token for a machine principal — an agent
// that authenticated at the OAuth token endpoint via RFC 7523 jwt-bearer.
//
// SECURITY: deliberately different from GenerateAccessToken in two ways.
//   - Issuer is IssuerService, which AuthMiddleware rejects. A service token
//     therefore cannot be presented on any human-authenticated route, even one
//     that forgets to add a gate.
//   - Role is left EMPTY. Previously this path minted role="service", a string
//     absent from the domain.UserRole enum. MemberMiddleware rejected only
//     "viewer", so "service" traversed it and reached 35 routes including
//     GET /agents/:id/credentials — letting one compromised agent read every
//     sibling agent's private key in the same organization. An empty role now
//     fails every role gate's type assertion as well as its allow-list.
//
// Subject is the agent's own ID; ServicePrincipalMiddleware pins route access to
// it so a service principal can only ever act on itself.
func (s *JWTService) GenerateServiceToken(agentID, orgID string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:         agentID,
		OrganizationID: orgID,
		TokenType:      TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    IssuerService,
			Subject:   agentID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// GenerateRefreshToken generates a refresh token
func (s *JWTService) GenerateRefreshToken(userID, orgID string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		TokenType:      TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    IssuerUser,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken validates and parses a JWT token
func (s *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshTokenPair generates new access AND refresh tokens (token rotation)
// This implements token rotation for enhanced security:
// - Old refresh token is invalidated after use
// - New refresh token issued with 90-day expiry
// Returns: newAccessToken, newRefreshToken, error
//
// A refresh token is an identity handle, not an authorization grant: the
// caller supplies the principal's CURRENT email and role (read from the user
// record), and nothing here reads the refresh token's own Email or Role
// claims. Login-issued refresh tokens carry none; an embedded role would
// outlive a demotion for the refresh lifetime.
func (s *JWTService) RefreshTokenPair(refreshToken, email, role string) (string, string, error) {
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// An access token must not be replayed at the refresh endpoint. (Empty
	// TokenType = legacy token issued before this claim; allowed during grace.)
	if claims.TokenType == TokenTypeAccess {
		return "", "", fmt.Errorf("access token cannot be used to refresh")
	}

	// Check if this is an SDK token (different issuer)
	isSDKToken := claims.Issuer == IssuerSDK

	var newAccessToken, newRefreshToken string

	// Generate new access token from the supplied principal
	newAccessToken, err = s.GenerateAccessToken(claims.UserID, claims.OrganizationID, email, role)
	if err != nil {
		return "", "", err
	}

	// Generate new refresh token (with same type as original)
	if isSDKToken {
		newRefreshToken, err = s.GenerateSDKRefreshToken(claims.UserID, claims.OrganizationID, email, role)
	} else {
		newRefreshToken, err = s.GenerateRefreshToken(claims.UserID, claims.OrganizationID)
	}
	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

// GetTokenID extracts the JTI (token ID) from a JWT without full validation
// Useful for token revocation checks before full validation
func (s *JWTService) GetTokenID(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// SECURITY: Enforce HMAC signing method to prevent algorithm confusion attacks
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok {
		return claims.ID, nil
	}

	return "", fmt.Errorf("failed to extract token ID")
}
