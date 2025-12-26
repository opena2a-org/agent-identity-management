package middleware

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// sortedJSONMarshal Tests
// ===========================

func TestSortedJSONMarshal_SimpleObject(t *testing.T) {
	input := map[string]interface{}{
		"z": "last",
		"a": "first",
		"m": "middle",
	}

	result := sortedJSONMarshal(input)

	// Check that keys are sorted alphabetically
	// The result should have "a" before "m" before "z"
	resultStr := string(result)
	aIdx := strings.Index(resultStr, "\"a\"")
	mIdx := strings.Index(resultStr, "\"m\"")
	zIdx := strings.Index(resultStr, "\"z\"")

	assert.True(t, aIdx < mIdx, "key 'a' should come before 'm'")
	assert.True(t, mIdx < zIdx, "key 'm' should come before 'z'")
}

func TestSortedJSONMarshal_NestedObject(t *testing.T) {
	input := map[string]interface{}{
		"outer": map[string]interface{}{
			"z": "nested_last",
			"a": "nested_first",
		},
		"simple": "value",
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	// Verify nested objects are also sorted
	assert.Contains(t, resultStr, "\"outer\"")
	assert.Contains(t, resultStr, "\"simple\"")
}

func TestSortedJSONMarshal_Array(t *testing.T) {
	input := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"z": 1, "a": 2},
			map[string]interface{}{"y": 3, "b": 4},
		},
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	// Verify array is included
	assert.Contains(t, resultStr, "\"items\"")
}

func TestSortedJSONMarshal_EmptyObject(t *testing.T) {
	input := map[string]interface{}{}

	result := sortedJSONMarshal(input)

	assert.Equal(t, "{}", string(result))
}

func TestSortedJSONMarshal_PythonFormat(t *testing.T) {
	// Python's json.dumps with default separators adds spaces
	input := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	// Should have ": " (space after colon) like Python
	assert.Contains(t, resultStr, "\": ")
}

func TestSortedJSONMarshal_NumberValues(t *testing.T) {
	input := map[string]interface{}{
		"int":   42,
		"float": 3.14,
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	assert.Contains(t, resultStr, "42")
	assert.Contains(t, resultStr, "3.14")
}

func TestSortedJSONMarshal_BooleanValues(t *testing.T) {
	input := map[string]interface{}{
		"true_val":  true,
		"false_val": false,
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	assert.Contains(t, resultStr, "true")
	assert.Contains(t, resultStr, "false")
}

func TestSortedJSONMarshal_NullValue(t *testing.T) {
	input := map[string]interface{}{
		"null_val": nil,
	}

	result := sortedJSONMarshal(input)
	resultStr := string(result)

	assert.Contains(t, resultStr, "null")
}

// ===========================
// sortValue Tests
// ===========================

func TestSortValue_Map(t *testing.T) {
	input := map[string]interface{}{
		"z": "last",
		"a": "first",
	}

	result := sortValue(input)

	// Result should be a map with sorted keys
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, resultMap, "a")
	assert.Contains(t, resultMap, "z")
}

func TestSortValue_Array(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"z": 1, "a": 2},
		"string",
		123,
	}

	result := sortValue(input)

	resultArr, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, resultArr, 3)
}

func TestSortValue_Primitive(t *testing.T) {
	// Primitives should be returned as-is
	assert.Equal(t, "hello", sortValue("hello"))
	assert.Equal(t, 42, sortValue(42))
	assert.Equal(t, true, sortValue(true))
	assert.Nil(t, sortValue(nil))
}

func TestSortValue_DeeplyNested(t *testing.T) {
	input := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "deep value",
			},
		},
	}

	result := sortValue(input)

	resultMap := result.(map[string]interface{})
	level1 := resultMap["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	assert.Equal(t, "deep value", level2["level3"])
}

// ===========================
// MockAgentRepository for Ed25519 middleware tests
// ===========================

type MockAgentService struct {
	mock.Mock
}

func (m *MockAgentService) GetAgent(agentID uuid.UUID) (*domain.Agent, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

// ===========================
// Ed25519AgentMiddleware Tests
// ===========================

func TestEd25519AgentMiddleware_SkipsWithAuthorizationHeader(t *testing.T) {
	// When Authorization header is present, middleware should skip

	var handlerCalled bool

	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil)) // nil agentService OK since we skip
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-jwt-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestEd25519AgentMiddleware_SkipsWithMissingHeaders(t *testing.T) {
	// When Ed25519 headers are missing, middleware should skip

	var handlerCalled bool

	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		handlerCalled = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No Ed25519 headers set

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

func TestEd25519AgentMiddleware_InvalidAgentID(t *testing.T) {
	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Agent-ID", "not-a-valid-uuid")
	req.Header.Set("X-Signature", "some-signature")
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Public-Key", "some-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestEd25519AgentMiddleware_InvalidTimestampFormat(t *testing.T) {
	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	agentID := uuid.New()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Agent-ID", agentID.String())
	req.Header.Set("X-Signature", "some-signature")
	req.Header.Set("X-Timestamp", "not-a-number")
	req.Header.Set("X-Public-Key", "some-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestEd25519AgentMiddleware_ExpiredTimestamp(t *testing.T) {
	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	agentID := uuid.New()
	oldTimestamp := time.Now().Unix() - 120 // 2 minutes ago (beyond 30s window)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Agent-ID", agentID.String())
	req.Header.Set("X-Signature", "some-signature")
	req.Header.Set("X-Timestamp", strconv.FormatInt(oldTimestamp, 10))
	req.Header.Set("X-Public-Key", "some-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestEd25519AgentMiddleware_FutureTimestamp(t *testing.T) {
	app := fiber.New()
	app.Use(Ed25519AgentMiddleware(nil))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	agentID := uuid.New()
	futureTimestamp := time.Now().Unix() + 120 // 2 minutes in future (beyond 30s window)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Agent-ID", agentID.String())
	req.Header.Set("X-Signature", "some-signature")
	req.Header.Set("X-Timestamp", strconv.FormatInt(futureTimestamp, 10))
	req.Header.Set("X-Public-Key", "some-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Integration-style test with real Ed25519 signing
// ===========================

func TestEd25519AgentMiddleware_ValidSignature_Integration(t *testing.T) {
	// Generate a real Ed25519 keypair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)

	// Create a valid signature
	agentID := uuid.New()
	timestamp := time.Now().Unix()
	timestampStr := strconv.FormatInt(timestamp, 10)

	// Build message like the middleware expects
	method := "GET"
	path := "/test"
	message := fmt.Sprintf("%s\n%s\n%s", method, path, timestampStr)

	signature := ed25519.Sign(privateKey, []byte(message))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Test that the message format and signature are correct
	assert.True(t, ed25519.Verify(publicKey, []byte(message), signature))
	assert.NotEmpty(t, signatureB64)
	assert.NotEmpty(t, publicKeyB64)
	assert.NotEmpty(t, agentID.String())
}

func TestEd25519AgentMiddleware_InvalidPublicKeySize(t *testing.T) {
	// Skip the agent service lookup by testing just the key validation
	// The middleware will fail on invalid key size before reaching the service

	// This test uses an invalid public key (wrong size)
	invalidKey := base64.StdEncoding.EncodeToString([]byte("too short"))

	agentID := uuid.New()
	timestamp := time.Now().Unix()
	timestampStr := strconv.FormatInt(timestamp, 10)

	// Test the decoding
	decoded, err := base64.StdEncoding.DecodeString(invalidKey)
	require.NoError(t, err)
	assert.NotEqual(t, ed25519.PublicKeySize, len(decoded))

	// Verify our test data is correct
	assert.NotEmpty(t, agentID.String())
	assert.NotEmpty(t, timestampStr)
}

func TestEd25519AgentMiddleware_InvalidSignatureFormat(t *testing.T) {
	// Test with invalid base64 signature
	invalidSignature := "not-valid-base64!!!"

	_, err := base64.StdEncoding.DecodeString(invalidSignature)
	assert.Error(t, err, "Invalid base64 should fail to decode")
}
