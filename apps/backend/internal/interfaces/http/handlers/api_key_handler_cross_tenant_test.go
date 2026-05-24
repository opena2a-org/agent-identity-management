package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiKeyTestRepo is a domain.APIKeyRepository stub. Only GetByID is
// implemented; other methods inherit from the nil-embedded interface
// and will panic if reached — regression-proofing signal for handlers
// that read fields beyond OrganizationID.
type apiKeyTestRepo struct {
	domain.APIKeyRepository
	getByID func(id uuid.UUID) (*domain.APIKey, error)
}

func (r *apiKeyTestRepo) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	return r.getByID(id)
}

// ===========================
// Cross-tenant access on the 2 APIKeyHandler routes
// (DisableAPIKey + DeleteAPIKey) must return 404 with the
// existence-secrecy body.
//
// Pre-fix shape: handler called h.apiKeyService.{RevokeAPIKey,
// DeleteAPIKey}(keyID, orgID). The service-layer check returned a
// distinct error string ("API key does not belong to organization"
// vs the generic GetByID error). Handler echoed err.Error() in the
// response — existence side channel that lets an attacker enumerate
// API key UUIDs across tenants by status + body string.
//
// Regression-proofing: apiKeyService and auditService are nil. A
// bypass on either handler would nil-deref on the service call →
// fiber 500, distinguishable from the secure 404.
// ===========================

func TestAPIKeyHandler_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	victimKeyID := uuid.New()

	apiKeyRepoForeignOrg := &apiKeyTestRepo{
		getByID: func(id uuid.UUID) (*domain.APIKey, error) {
			return &domain.APIKey{
				ID:             victimKeyID,
				OrganizationID: differentOrgID,
				Name:           "victim-key",
				IsActive:       true,
			}, nil
		},
	}

	// apiKeyService + auditService intentionally nil — a bypass on
	// either handler reaches h.apiKeyService.{RevokeAPIKey,
	// DeleteAPIKey} which nil-derefs → fiber 500 ≠ 404.
	handler := &APIKeyHandler{
		apiKeyService: nil,
		auditService:  nil,
		apiKeyRepo:    apiKeyRepoForeignOrg,
	}

	tests := []struct {
		name        string
		method      string
		requestPath string
		setup       func(*fiber.App, *APIKeyHandler)
	}{
		{
			name:        "DisableAPIKey",
			method:      "POST",
			requestPath: "/api-keys/" + victimKeyID.String() + "/disable",
			setup: func(app *fiber.App, h *APIKeyHandler) {
				app.Post("/api-keys/:id/disable", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.DisableAPIKey(c)
				})
			},
		},
		{
			name:        "DeleteAPIKey",
			method:      "DELETE",
			requestPath: "/api-keys/" + victimKeyID.String(),
			setup: func(app *fiber.App, h *APIKeyHandler) {
				app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.DeleteAPIKey(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			req := httptest.NewRequest(tt.method, tt.requestPath, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)
			assert.JSONEq(t, `{"error":"not found"}`, bodyStr,
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, bodyStr)
			// Defense-in-depth: confirm no fragment of the pre-fix
			// service error string leaks through.
			assert.NotContains(t, bodyStr, "does not belong to organization",
				"%s: response must not contain pre-fix service-layer cross-tenant message", tt.name)
			assert.NotContains(t, bodyStr, "victim-key",
				"%s: response must not leak victim key name", tt.name)
		})
	}

	// Suppress unused import warning for net/http when test changes
	// in the future (kept here for table-driven body support).
	_ = http.MethodPost
}
