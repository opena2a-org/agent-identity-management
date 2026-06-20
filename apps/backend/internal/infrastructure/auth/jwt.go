package auth

import (
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
)

// Token types. The `typ` claim separates access tokens (valid as a bearer) from
// refresh tokens (valid only at /auth/refresh), so a refresh token cannot be
// replayed as a session token.
// GRACE: tokens minted before this claim existed have an empty TokenType; the
// access middleware and refresh endpoint treat an empty type as legacy and
// allow it, so this rolls out without breaking live sessions. Newly issued
// tokens always carry a type and are enforced immediately.
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
	secret         []byte
	accessExpiry   time.Duration
	refreshExpiry  time.Duration
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

// RefreshAccessToken generates a new access token from a refresh token
func (s *JWTService) RefreshAccessToken(refreshToken string) (string, error) {
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}

	// An access token must not be usable to mint another access token. (Empty
	// TokenType = legacy token issued before this claim; allowed during grace.)
	if claims.TokenType == TokenTypeAccess {
		return "", fmt.Errorf("access token cannot be used to refresh")
	}

	// Generate new access token
	return s.GenerateAccessToken(claims.UserID, claims.OrganizationID, claims.Email, claims.Role)
}

// RefreshTokenPair generates new access AND refresh tokens (token rotation)
// This implements token rotation for enhanced security:
// - Old refresh token is invalidated after use
// - New refresh token issued with 90-day expiry
// Returns: newAccessToken, newRefreshToken, error
func (s *JWTService) RefreshTokenPair(refreshToken string) (string, string, error) {
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

	// Generate new access token
	newAccessToken, err = s.GenerateAccessToken(claims.UserID, claims.OrganizationID, claims.Email, claims.Role)
	if err != nil {
		return "", "", err
	}

	// Generate new refresh token (with same type as original)
	if isSDKToken {
		newRefreshToken, err = s.GenerateSDKRefreshToken(claims.UserID, claims.OrganizationID, claims.Email, claims.Role)
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
