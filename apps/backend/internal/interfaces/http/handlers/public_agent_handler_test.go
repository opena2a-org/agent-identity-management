package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewPublicAgentHandler Tests
// ===========================

func TestNewPublicAgentHandler_NilDeps(t *testing.T) {
	handler := NewPublicAgentHandler(nil, nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// PublicAgentHandler.Register Tests
// ===========================

func TestPublicAgentHandler_Register_InvalidJSON(t *testing.T) {
	handler := &PublicAgentHandler{}
	app := fiber.New()
	app.Post("/public/agents/register", handler.Register)

	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicAgentHandler_Register_MissingRequiredFields(t *testing.T) {
	handler := &PublicAgentHandler{}
	app := fiber.New()
	app.Post("/public/agents/register", handler.Register)

	// Missing name, displayName, description
	body := `{"agentType":"claude"}`
	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicAgentHandler_Register_InvalidAgentType(t *testing.T) {
	handler := &PublicAgentHandler{}
	app := fiber.New()
	app.Post("/public/agents/register", handler.Register)

	body := `{"name":"test","displayName":"Test Agent","description":"A test agent","agentType":"invalid_type"}`
	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicAgentHandler_Register_NoAuthRequired(t *testing.T) {
	handler := &PublicAgentHandler{}
	app := fiber.New()
	// Add recover middleware so nil agentService panic returns 500 instead of crashing
	app.Use(func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "internal server error",
				})
			}
		}()
		return c.Next()
	})
	app.Post("/public/agents/register", handler.Register)

	body := `{"name":"test","displayName":"Test Agent","description":"A test agent","agentType":"claude"}`
	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-AIM-API-Key header -- endpoint is public, should NOT return 401

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Without a real agentService wired up, we expect 500 (nil pointer on service call),
	// but critically NOT 401 -- the endpoint no longer requires authentication
	assert.NotEqual(t, fiber.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestPublicAgentHandler_Register_MissingAgentType(t *testing.T) {
	handler := &PublicAgentHandler{}
	app := fiber.New()
	app.Post("/public/agents/register", handler.Register)

	// Has name, displayName, description but missing agentType
	body := `{"name":"test","displayName":"Test Agent","description":"A test agent"}`
	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ===========================
// isValidAgentTypeForPublicAPI Tests
// ===========================

func TestIsValidAgentTypeForPublicAPI(t *testing.T) {
	tests := []struct {
		name      string
		agentType domain.AgentType
		expected  bool
	}{
		// LLM Providers
		{"Claude", domain.AgentTypeClaude, true},
		{"GPT", domain.AgentTypeGPT, true},
		{"Gemini", domain.AgentTypeGemini, true},
		{"Llama", domain.AgentTypeLlama, true},
		{"Mistral", domain.AgentTypeMistral, true},
		{"Cohere", domain.AgentTypeCohere, true},
		// Frameworks
		{"LangChain", domain.AgentTypeLangChain, true},
		{"LlamaIndex", domain.AgentTypeLlamaIndex, true},
		{"AutoGen", domain.AgentTypeAutoGen, true},
		{"CrewAI", domain.AgentTypeCrewAI, true},
		{"LangGraph", domain.AgentTypeLangGraph, true},
		{"Haystack", domain.AgentTypeHaystack, true},
		{"SemanticKernel", domain.AgentTypeSemanticKernel, true},
		// Copilots & Assistants
		{"Copilot", domain.AgentTypeCopilot, true},
		{"Assistant", domain.AgentTypeAssistant, true},
		{"Chatbot", domain.AgentTypeChatbot, true},
		// Autonomous
		{"AutoGPT", domain.AgentTypeAutoGPT, true},
		{"BabyAGI", domain.AgentTypeBabyAGI, true},
		// Generic
		{"Custom", domain.AgentTypeCustom, true},
		{"Demo", domain.AgentTypeDemo, true},
		{"AI", domain.AgentTypeAI, true},
		// Invalid
		{"Invalid", domain.AgentType("invalid"), false},
		{"Empty", domain.AgentType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAgentTypeForPublicAPI(tt.agentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// calculateInitialTrustScore Tests
// ===========================

func TestPublicAgentHandler_calculateInitialTrustScore(t *testing.T) {
	handler := &PublicAgentHandler{}

	tests := []struct {
		name     string
		req      *PublicRegisterRequest
		expected float64
	}{
		{
			name:     "Base score only",
			req:      &PublicRegisterRequest{},
			expected: 50.0,
		},
		{
			name: "With repository URL",
			req: &PublicRegisterRequest{
				RepositoryURL: "https://example.com/repo",
			},
			expected: 60.0, // 50 + 10
		},
		{
			name: "With documentation URL",
			req: &PublicRegisterRequest{
				DocumentationURL: "https://docs.example.com",
			},
			expected: 55.0, // 50 + 5
		},
		{
			name: "With version",
			req: &PublicRegisterRequest{
				Version: "1.0.0",
			},
			expected: 55.0, // 50 + 5
		},
		{
			name: "With GitHub repo",
			req: &PublicRegisterRequest{
				RepositoryURL: "https://github.com/example/repo",
			},
			expected: 70.0, // 50 + 10 + 10
		},
		{
			name: "With all bonuses",
			req: &PublicRegisterRequest{
				RepositoryURL:    "https://github.com/example/repo",
				DocumentationURL: "https://docs.example.com",
				Version:          "1.0.0",
			},
			expected: 80.0, // 50 + 10 + 10 + 5 + 5
		},
		{
			name: "With GitLab repo",
			req: &PublicRegisterRequest{
				RepositoryURL: "https://gitlab.com/example/repo",
			},
			expected: 70.0, // 50 + 10 + 10 (GitLab bonus)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.calculateInitialTrustScore(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// buildRegistrationMessage Tests
// ===========================

func TestPublicAgentHandler_buildRegistrationMessage(t *testing.T) {
	handler := &PublicAgentHandler{}

	tests := []struct {
		name     string
		status   domain.AgentStatus
		contains string
	}{
		{"Verified", domain.AgentStatusVerified, "auto-verified"},
		{"Pending", domain.AgentStatusPending, "Pending"},
		{"Default", domain.AgentStatus("unknown"), "registered successfully"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.buildRegistrationMessage(tt.status)
			assert.Contains(t, result, tt.contains)
		})
	}
}

// ===========================
// Struct Tests
// ===========================

func TestPublicRegisterRequest_Struct(t *testing.T) {
	req := PublicRegisterRequest{
		Name:               "test-agent",
		DisplayName:        "Test Agent",
		Description:        "A test AI agent for unit testing",
		AgentType:          domain.AgentTypeClaude,
		Version:            "1.0.0",
		OrganizationDomain: "example.com",
		UserEmail:          "user@example.com",
		RepositoryURL:      "https://github.com/example/agent",
		DocumentationURL:   "https://docs.example.com/agent",
	}

	assert.Equal(t, "test-agent", req.Name)
	assert.Equal(t, "Test Agent", req.DisplayName)
	assert.Equal(t, "A test AI agent for unit testing", req.Description)
	assert.Equal(t, domain.AgentTypeClaude, req.AgentType)
	assert.Equal(t, "1.0.0", req.Version)
	assert.Equal(t, "example.com", req.OrganizationDomain)
	assert.Equal(t, "user@example.com", req.UserEmail)
	assert.Equal(t, "https://github.com/example/agent", req.RepositoryURL)
	assert.Equal(t, "https://docs.example.com/agent", req.DocumentationURL)
}

func TestPublicRegisterRequest_EmptyStruct(t *testing.T) {
	req := PublicRegisterRequest{}

	assert.Empty(t, req.Name)
	assert.Empty(t, req.DisplayName)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.AgentType)
	assert.Empty(t, req.Version)
	assert.Empty(t, req.OrganizationDomain)
	assert.Empty(t, req.UserEmail)
	assert.Empty(t, req.RepositoryURL)
	assert.Empty(t, req.DocumentationURL)
}

func TestPublicRegisterResponse_Struct(t *testing.T) {
	resp := PublicRegisterResponse{
		AgentID:     "agent-uuid-123",
		Name:        "test-agent",
		DisplayName: "Test Agent",
		PublicKey:   "base64-public-key",
		PrivateKey:  "base64-private-key",
		AIMURL:      "https://aim.example.com",
		Status:      "verified",
		TrustScore:  85.0,
		Message:     "Agent registered and auto-verified",
	}

	assert.Equal(t, "agent-uuid-123", resp.AgentID)
	assert.Equal(t, "test-agent", resp.Name)
	assert.Equal(t, "Test Agent", resp.DisplayName)
	assert.Equal(t, "base64-public-key", resp.PublicKey)
	assert.Equal(t, "base64-private-key", resp.PrivateKey)
	assert.Equal(t, "https://aim.example.com", resp.AIMURL)
	assert.Equal(t, "verified", resp.Status)
	assert.Equal(t, 85.0, resp.TrustScore)
	assert.Equal(t, "Agent registered and auto-verified", resp.Message)
}

func TestPublicRegisterResponse_EmptyStruct(t *testing.T) {
	resp := PublicRegisterResponse{}

	assert.Empty(t, resp.AgentID)
	assert.Empty(t, resp.Name)
	assert.Empty(t, resp.DisplayName)
	assert.Empty(t, resp.PublicKey)
	assert.Empty(t, resp.PrivateKey)
	assert.Empty(t, resp.AIMURL)
	assert.Empty(t, resp.Status)
	assert.Equal(t, 0.0, resp.TrustScore)
	assert.Empty(t, resp.Message)
}
