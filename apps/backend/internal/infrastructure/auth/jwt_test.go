package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupJWTTest(t *testing.T) func() {
	originalSecret := os.Getenv("JWT_SECRET")
	originalAccessTTL := os.Getenv("JWT_ACCESS_TTL")
	originalRefreshTTL := os.Getenv("JWT_REFRESH_TTL")

	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")

	return func() {
		if originalSecret != "" {
			os.Setenv("JWT_SECRET", originalSecret)
		} else {
			os.Unsetenv("JWT_SECRET")
		}
		if originalAccessTTL != "" {
			os.Setenv("JWT_ACCESS_TTL", originalAccessTTL)
		} else {
			os.Unsetenv("JWT_ACCESS_TTL")
		}
		if originalRefreshTTL != "" {
			os.Setenv("JWT_REFRESH_TTL", originalRefreshTTL)
		} else {
			os.Unsetenv("JWT_REFRESH_TTL")
		}
	}
}

func TestNewJWTService(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	assert.NotNil(t, service)
}

func TestNewJWTService_MissingSecret(t *testing.T) {
	originalSecret := os.Getenv("JWT_SECRET")
	os.Unsetenv("JWT_SECRET")
	defer func() {
		if originalSecret != "" {
			os.Setenv("JWT_SECRET", originalSecret)
		}
	}()

	assert.Panics(t, func() {
		NewJWTService()
	})
}

func TestNewJWTService_ShortSecret(t *testing.T) {
	originalSecret := os.Getenv("JWT_SECRET")
	os.Setenv("JWT_SECRET", "short")
	defer func() {
		if originalSecret != "" {
			os.Setenv("JWT_SECRET", originalSecret)
		} else {
			os.Unsetenv("JWT_SECRET")
		}
	}()

	assert.Panics(t, func() {
		NewJWTService()
	})
}

func TestJWTService_GenerateAccessToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	token, err := service.GenerateAccessToken(userID, orgID, "test@example.com", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, orgID, claims.OrganizationID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "agent-identity-management", claims.Issuer)
}

func TestJWTService_GenerateRefreshToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	token, err := service.GenerateRefreshToken(userID, orgID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, orgID, claims.OrganizationID)
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	accessToken, refreshToken, err := service.GenerateTokenPair(userID, orgID, "test@example.com", "member")
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Both should be valid
	accessClaims, err := service.ValidateToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, "member", accessClaims.Role)

	refreshClaims, err := service.ValidateToken(refreshToken)
	require.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
}

func TestJWTService_GenerateSDKRefreshToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	token, err := service.GenerateSDKRefreshToken(userID, orgID, "sdk@example.com", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, orgID, claims.OrganizationID)
	assert.Equal(t, "sdk@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "agent-identity-management-sdk", claims.Issuer)
}

func TestJWTService_ValidateToken_Invalid(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()

	// Invalid token string
	_, err := service.ValidateToken("invalid-token")
	assert.Error(t, err)

	// Empty token
	_, err = service.ValidateToken("")
	assert.Error(t, err)

	// Malformed token
	_, err = service.ValidateToken("a.b.c")
	assert.Error(t, err)
}

func TestJWTService_ValidateToken_WrongSecret(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	// Create a token with different secret
	claims := JWTClaims{
		UserID:         uuid.New().String(),
		OrganizationID: uuid.New().String(),
		Email:          "test@example.com",
		Role:           "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("different-secret-key-that-is-long"))

	service := NewJWTService()
	_, err := service.ValidateToken(tokenString)
	assert.Error(t, err)
}

func TestJWTService_ValidateToken_Expired(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	// Create an expired token
	claims := JWTClaims{
		UserID:         uuid.New().String(),
		OrganizationID: uuid.New().String(),
		Email:          "test@example.com",
		Role:           "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret-key-for-unit-tests-32"))

	service := NewJWTService()
	_, err := service.ValidateToken(tokenString)
	assert.Error(t, err)
}

func TestJWTService_RefreshAccessToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	// Generate a refresh token
	refreshToken, err := service.GenerateRefreshToken(userID, orgID)
	require.NoError(t, err)

	// Use it to get a new access token
	newAccessToken, err := service.RefreshAccessToken(refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)

	// Validate the new access token
	claims, err := service.ValidateToken(newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, orgID, claims.OrganizationID)
}

func TestJWTService_RefreshAccessToken_InvalidToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()

	_, err := service.RefreshAccessToken("invalid-token")
	assert.Error(t, err)
}

func TestJWTService_RefreshTokenPair(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	// Generate initial tokens
	_, refreshToken, err := service.GenerateTokenPair(userID, orgID, "test@example.com", "admin")
	require.NoError(t, err)

	// Rotate tokens
	newAccessToken, newRefreshToken, err := service.RefreshTokenPair(refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEmpty(t, newRefreshToken)
	assert.NotEqual(t, refreshToken, newRefreshToken)

	// Validate both new tokens
	accessClaims, err := service.ValidateToken(newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)

	refreshClaims, err := service.ValidateToken(newRefreshToken)
	require.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
}

func TestJWTService_RefreshTokenPair_SDKToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	// Generate SDK refresh token
	sdkRefreshToken, err := service.GenerateSDKRefreshToken(userID, orgID, "sdk@example.com", "admin")
	require.NoError(t, err)

	// Rotate tokens - should generate new SDK refresh token
	newAccessToken, newRefreshToken, err := service.RefreshTokenPair(sdkRefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEmpty(t, newRefreshToken)

	// Validate new refresh token is also SDK type
	refreshClaims, err := service.ValidateToken(newRefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "agent-identity-management-sdk", refreshClaims.Issuer)
}

func TestJWTService_GetTokenID(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	token, err := service.GenerateAccessToken(userID, orgID, "test@example.com", "admin")
	require.NoError(t, err)

	tokenID, err := service.GetTokenID(token)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenID)

	// Token ID should be a valid UUID
	_, err = uuid.Parse(tokenID)
	assert.NoError(t, err)
}

func TestJWTService_GetTokenID_InvalidToken(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()

	service := NewJWTService()

	_, err := service.GetTokenID("invalid-token")
	assert.Error(t, err)
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	value := getEnv("TEST_VAR", "fallback")
	assert.Equal(t, "test_value", value)

	// Test with non-existing env var
	value = getEnv("NON_EXISTING_VAR", "fallback")
	assert.Equal(t, "fallback", value)
}

func TestJWTService_CustomExpiry(t *testing.T) {
	originalSecret := os.Getenv("JWT_SECRET")
	originalAccessTTL := os.Getenv("JWT_ACCESS_TTL")
	originalRefreshTTL := os.Getenv("JWT_REFRESH_TTL")

	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	os.Setenv("JWT_ACCESS_TTL", "30m")
	os.Setenv("JWT_REFRESH_TTL", "24h")

	defer func() {
		if originalSecret != "" {
			os.Setenv("JWT_SECRET", originalSecret)
		} else {
			os.Unsetenv("JWT_SECRET")
		}
		if originalAccessTTL != "" {
			os.Setenv("JWT_ACCESS_TTL", originalAccessTTL)
		} else {
			os.Unsetenv("JWT_ACCESS_TTL")
		}
		if originalRefreshTTL != "" {
			os.Setenv("JWT_REFRESH_TTL", originalRefreshTTL)
		} else {
			os.Unsetenv("JWT_REFRESH_TTL")
		}
	}()

	service := NewJWTService()
	userID := uuid.New().String()
	orgID := uuid.New().String()

	accessToken, err := service.GenerateAccessToken(userID, orgID, "test@example.com", "admin")
	require.NoError(t, err)

	claims, err := service.ValidateToken(accessToken)
	require.NoError(t, err)

	// Check expiry is approximately 30 minutes from now
	expectedExpiry := time.Now().Add(30 * time.Minute)
	assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 5*time.Second)
}
