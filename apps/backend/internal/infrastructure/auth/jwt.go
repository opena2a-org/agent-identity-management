package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	jwt.RegisteredClaims
}

// JWTService handles JWT operations
type JWTService struct {
	secret         []byte
	accessExpiry   time.Duration
	refreshExpiry  time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService() *JWTService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "development-secret-change-in-production"
	}

	accessExpiry := 24 * time.Hour
	refreshExpiry := 7 * 24 * time.Hour

	return &JWTService{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateSDKRefreshToken generates a long-lived refresh token for SDK usage (1 year)
// This token is embedded in downloaded SDKs for auto-authentication
func (s *JWTService) GenerateSDKRefreshToken(userID, orgID, email, role string) (string, error) {
	now := time.Now()
	sdkExpiry := 365 * 24 * time.Hour // 1 year

	claims := JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(sdkExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "agent-identity-management-sdk",
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
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "agent-identity-management",
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
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "agent-identity-management",
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

	// Generate new access token
	return s.GenerateAccessToken(claims.UserID, claims.OrganizationID, claims.Email, claims.Role)
}
