package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// AgentHandler.GetAgentKeyVault Tests
// ===========================

func TestAgentHandler_GetAgentKeyVault_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", handler.GetAgentKeyVault)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// The key vault says whether the server holds the agent's private key, and
// never returns the key itself.
func TestAgentHandler_GetAgentKeyVault_ServerHoldsPrivateKey(t *testing.T) {
	const encryptedKeySentinel = "FAKE-ENCRYPTED-KEY-1234"

	tests := []struct {
		name                string
		encryptedPrivateKey *string
		want                bool
	}{
		{name: "encrypted private key stored", encryptedPrivateKey: func() *string { s := encryptedKeySentinel; return &s }(), want: true},
		{name: "no private key stored", encryptedPrivateKey: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgID := uuid.New()
			agentID := uuid.New()
			publicKey := "FAKE-PUBLIC-KEY-1234"
			mockAgentSvc := &MockAgentServiceImpl{
				GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
					return &domain.Agent{
						ID:                  agentID,
						OrganizationID:      orgID,
						Name:                "vault-agent",
						PublicKey:           &publicKey,
						EncryptedPrivateKey: tt.encryptedPrivateKey,
						KeyAlgorithm:        "Ed25519",
					}, nil
				},
			}
			handler := NewAgentHandlerWithInterfaces(
				mockAgentSvc, nil, &MockAuditServiceImpl{}, nil, nil, nil, nil, nil, nil, nil, nil,
			)

			app := fiber.New()
			app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
				c.Locals("organization_id", orgID)
				c.Locals("user_id", uuid.New())
				return handler.GetAgentKeyVault(c)
			})

			resp, err := app.Test(httptest.NewRequest("GET", "/agents/"+agentID.String()+"/key-vault", nil))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &body))

			got, present := body["serverHoldsPrivateKey"]
			require.True(t, present, "key vault body must carry serverHoldsPrivateKey, got %s", raw)
			assert.Equal(t, tt.want, got)

			assert.NotContains(t, body, "encryptedPrivateKey")
			assert.NotContains(t, string(raw), encryptedKeySentinel)
		})
	}
}

// ===========================
// AgentHandler.GetAgentAuditLogs Tests
// ===========================

func TestAgentHandler_GetAgentAuditLogs_NoOrgContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", handler.GetAgentAuditLogs)

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_NoUserContext(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_InvalidAgentID(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_InvalidOrgIDType(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		c.Locals("user_id", uuid.New())
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentAuditLogs_InvalidUserIDType(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.GetAgentAuditLogs(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/audit-logs", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_InvalidOrgIDType(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid") // Wrong type
		c.Locals("user_id", uuid.New())
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAgentHandler_GetAgentKeyVault_InvalidUserIDType(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", "not-a-uuid") // Wrong type
		return handler.GetAgentKeyVault(c)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.New().String()+"/key-vault", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}
