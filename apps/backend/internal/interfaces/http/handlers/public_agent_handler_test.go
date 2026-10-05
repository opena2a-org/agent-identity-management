package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signedInPublicRegisterApp mounts Register behind the identity that
// OptionalAuthMiddleware sets for a valid user access token.
func signedInPublicRegisterApp(handler *PublicAgentHandler) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return c.Next()
	})
	app.Post("/public/agents/register", handler.Register)
	return app
}

// insertCountingAgentRepo stands in for the agents table. It counts insert
// attempts and refuses each one the way PostgreSQL refuses a row whose
// organization does not exist. No other repository method is set, so a request
// that reaches one fails its test.
type insertCountingAgentRepo struct {
	domain.AgentRepository
	inserts atomic.Int32
}

func (r *insertCountingAgentRepo) Create(*domain.Agent) error {
	r.inserts.Add(1)
	return errors.New(`pq: insert or update on table "agents" violates foreign key constraint "agents_organization_id_fkey"`)
}

// publicRegisterRoute wires Register as the server does: the real agent
// service over insertCountingAgentRepo, behind the real OptionalAuthMiddleware.
func publicRegisterRoute(t *testing.T) (*fiber.App, *auth.JWTService, *insertCountingAgentRepo) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	jwtService := auth.NewJWTService()

	masterKey := make([]byte, 32)
	_, err := rand.Read(masterKey)
	require.NoError(t, err)
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)

	repo := &insertCountingAgentRepo{}
	agentService := application.NewAgentService(repo, nil, nil, vault, nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Use(middleware.OptionalAuthMiddleware(jwtService))
	app.Post("/api/v1/public/agents/register", NewPublicAgentHandler(agentService, nil, vault).Register)
	return app, jwtService, repo
}

const publicRegisterValidBody = `{"name":"test","displayName":"Test Agent","description":"A test agent","agentType":"claude"}`

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
	app := signedInPublicRegisterApp(&PublicAgentHandler{})

	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestPublicAgentHandler_Register_MissingRequiredFields(t *testing.T) {
	app := signedInPublicRegisterApp(&PublicAgentHandler{})

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
	app := signedInPublicRegisterApp(&PublicAgentHandler{})

	body := `{"name":"test","displayName":"Test Agent","description":"A test agent","agentType":"invalid_type"}`
	req := httptest.NewRequest("POST", "/public/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// TestPublicAgentHandler_Register_WithoutUserTokenAnswers401 pins what a caller
// with no user access token gets. An agent row needs a user and an
// organization, and on this route only a user access token supplies them, so
// the handler used to run every such request through key generation and an
// insert the database refused, and answered 400 "invalid organization or user
// for this registration". It now answers 401, names the two paths that do
// register an agent, and never reaches the agents table.
func TestPublicAgentHandler_Register_WithoutUserTokenAnswers401(t *testing.T) {
	app, jwtService, repo := publicRegisterRoute(t)

	refreshToken, err := jwtService.GenerateRefreshToken(uuid.NewString(), uuid.NewString())
	require.NoError(t, err)

	tests := []struct {
		name   string
		header string
		value  string
		body   string
	}{
		{name: "no credential", body: publicRegisterValidBody},
		{name: "API key in X-AIM-API-Key", header: "X-AIM-API-Key", value: "not-a-real-key", body: publicRegisterValidBody},
		{name: "API key in X-API-Key", header: "X-API-Key", value: "not-a-real-key", body: publicRegisterValidBody},
		{name: "API key as bearer", header: "Authorization", value: "Bearer not-a-real-key", body: publicRegisterValidBody},
		{name: "refresh token as bearer", header: "Authorization", value: "Bearer " + refreshToken, body: publicRegisterValidBody},
		{name: "no credential and a body that is not JSON", body: "not json"},
		{name: "no credential and an unknown agent type", body: `{"name":"test","displayName":"Test Agent","description":"A test agent","agentType":"invalid_type"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/public/agents/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

			var payload map[string]string
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
			assert.Equal(t, publicRegisterAuthRequiredMessage, payload["error"])
		})
	}

	assert.Equal(t, int32(0), repo.inserts.Load(), "a caller with no user access token must not reach the agents table")
}

// TestPublicRegisterAuthRequiredMessage_NamesBothWorkingPaths keeps the 401
// actionable: it has to say what to send here and where an API key is accepted.
func TestPublicRegisterAuthRequiredMessage_NamesBothWorkingPaths(t *testing.T) {
	assert.Contains(t, publicRegisterAuthRequiredMessage, "Authorization: Bearer <token>")
	assert.Contains(t, publicRegisterAuthRequiredMessage, "X-API-Key")
	assert.Contains(t, publicRegisterAuthRequiredMessage, "POST /api/v1/agents")
}

// TestPublicAgentHandler_Register_WithUserAccessTokenReachesTheService is the
// other half: a signed-in user is not turned away. The stand-in table refuses
// the insert, so the request ends in the service's 400 for an organization
// that does not exist, after exactly one insert attempt.
func TestPublicAgentHandler_Register_WithUserAccessTokenReachesTheService(t *testing.T) {
	app, jwtService, repo := publicRegisterRoute(t)

	accessToken, err := jwtService.GenerateAccessToken(uuid.NewString(), uuid.NewString(), "member@example.com", "member")
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/public/agents/register", strings.NewReader(publicRegisterValidBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var payload map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.Equal(t, application.ErrInvalidOrgOrUser.Error(), payload["error"])
	assert.Equal(t, int32(1), repo.inserts.Load())
}

func TestPublicAgentHandler_Register_MissingAgentType(t *testing.T) {
	app := signedInPublicRegisterApp(&PublicAgentHandler{})

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
