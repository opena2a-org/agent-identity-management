package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSDKTokenRepository is a mock implementation of domain.SDKTokenRepository
type MockSDKTokenRepository struct {
	mock.Mock
}

func (m *MockSDKTokenRepository) Create(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByTokenID(tokenID string) (*domain.SDKToken, error) {
	args := m.Called(tokenID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByTokenHash(tokenHash string) (*domain.SDKToken, error) {
	args := m.Called(tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	args := m.Called(userID, includeRevoked)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByOrganizationID(orgID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	args := m.Called(orgID, includeRevoked)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) Update(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) Revoke(id uuid.UUID, reason string) error {
	args := m.Called(id, reason)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) RevokeByTokenHash(tokenHash string, reason string) error {
	args := m.Called(tokenHash, reason)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) RevokeAllForUser(userID uuid.UUID, reason string) error {
	args := m.Called(userID, reason)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) RecordUsage(tokenID string, ipAddress string) error {
	args := m.Called(tokenID, ipAddress)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) DeleteExpired() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSDKTokenRepository) GetActiveCount(userID uuid.UUID) (int, error) {
	args := m.Called(userID)
	return args.Int(0), args.Error(1)
}

// ===========================
// NewSDKTokenTrackingMiddleware Tests
// ===========================

func TestNewSDKTokenTrackingMiddleware_Success(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)

	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	assert.NotNil(t, middleware)
	assert.NotNil(t, middleware.sdkTokenRepo)
}

// ===========================
// Handler Tests
// ===========================

func TestSDKTokenTrackingMiddleware_NoHeaders(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var isSDKRegistration interface{}
	var sdkTokenID interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		isSDKRegistration = c.Locals("is_sdk_registration")
		sdkTokenID = c.Locals("sdk_token_id")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Nil(t, isSDKRegistration)
	assert.Nil(t, sdkTokenID)
}

func TestSDKTokenTrackingMiddleware_WithSDKTokenHeader(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)

	userID := uuid.New()
	sdkToken := &domain.SDKToken{
		TokenID: "test-token-123",
		UserID:  userID,
	}

	mockRepo.On("GetByTokenID", "test-token-123").Return(sdkToken, nil)
	mockRepo.On("RecordUsage", "test-token-123", mock.Anything).Return(nil)

	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var isSDKRegistration interface{}
	var capturedTokenID interface{}
	var capturedUserID interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		isSDKRegistration = c.Locals("is_sdk_registration")
		capturedTokenID = c.Locals("sdk_token_id")
		capturedUserID = c.Locals("sdk_token_user_id")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-SDK-Token", "test-token-123")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, true, isSDKRegistration)
	assert.Equal(t, "test-token-123", capturedTokenID)
	assert.Equal(t, userID, capturedUserID)
}

func TestSDKTokenTrackingMiddleware_WithSDKTokenHeader_TokenNotFound(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)

	mockRepo.On("GetByTokenID", "unknown-token").Return(nil, nil)
	mockRepo.On("RecordUsage", "unknown-token", mock.Anything).Return(nil)

	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var capturedUserID interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		capturedUserID = c.Locals("sdk_token_user_id")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-SDK-Token", "unknown-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Nil(t, capturedUserID) // User ID not set when token not found
}

func TestSDKTokenTrackingMiddleware_WithPythonSDKUserAgent(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var isSDKRegistration interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		isSDKRegistration = c.Locals("is_sdk_registration")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "AIM-Python-SDK/1.0.0")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, true, isSDKRegistration)
}

func TestSDKTokenTrackingMiddleware_WithJavaSDKUserAgent(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var isSDKRegistration interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		isSDKRegistration = c.Locals("is_sdk_registration")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "AIM-Java-SDK/2.0.0")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, true, isSDKRegistration)
}

func TestSDKTokenTrackingMiddleware_WithBearerToken(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var accessTokenJTI interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		accessTokenJTI = c.Locals("access_token_jti")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Create a JWT-like token with JTI claim
	// This is a valid JWT structure (header.payload.signature) but won't verify
	// We only need it to be parseable since we use ParseUnverified
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 = {"alg":"HS256","typ":"JWT"}
	// eyJqdGkiOiJ0ZXN0LWp0aS0xMjMifQ = {"jti":"test-jti-123"}
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJqdGkiOiJ0ZXN0LWp0aS0xMjMifQ.signature"

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "test-jti-123", accessTokenJTI)
}

func TestSDKTokenTrackingMiddleware_WithInvalidBearerToken(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var accessTokenJTI interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		accessTokenJTI = c.Locals("access_token_jti")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-format")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should still succeed, just won't set the JTI
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Nil(t, accessTokenJTI)
}

func TestSDKTokenTrackingMiddleware_CombinedHeadersAndUserAgent(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)

	userID := uuid.New()
	sdkToken := &domain.SDKToken{
		TokenID: "combined-token",
		UserID:  userID,
	}

	mockRepo.On("GetByTokenID", "combined-token").Return(sdkToken, nil)
	mockRepo.On("RecordUsage", "combined-token", mock.Anything).Return(nil)

	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	var isSDKRegistration interface{}
	var capturedTokenID interface{}

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Get("/test", func(c fiber.Ctx) error {
		isSDKRegistration = c.Locals("is_sdk_registration")
		capturedTokenID = c.Locals("sdk_token_id")
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-SDK-Token", "combined-token")
	req.Header.Set("User-Agent", "AIM-Python-SDK/1.0.0")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, true, isSDKRegistration)
	assert.Equal(t, "combined-token", capturedTokenID)
}

func TestSDKTokenTrackingMiddleware_PassesRequestThrough(t *testing.T) {
	mockRepo := new(MockSDKTokenRepository)
	middleware := NewSDKTokenTrackingMiddleware(mockRepo)

	app := fiber.New()
	app.Use(middleware.Handler())
	app.Post("/api/v1/agents", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"id":   "agent-123",
			"name": "Test Agent",
		})
	})

	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, "agent-123", result["id"])
	assert.Equal(t, "Test Agent", result["name"])
}
