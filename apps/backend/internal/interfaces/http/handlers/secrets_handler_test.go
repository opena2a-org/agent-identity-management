package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secretsTestAgentRepo is a domain.AgentRepository stub that only
// implements GetByID (the single method LoadOwnedViaAgent calls). The
// other 12 methods are inherited from the nil-embedded interface and
// would panic if invoked — proves we don't reach an unintended method.
type secretsTestAgentRepo struct {
	domain.AgentRepository
	getByID func(id uuid.UUID) (*domain.Agent, error)
}

func (r *secretsTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.getByID(id)
}

// secretsTestNamespaceRepo is a secrets.NamespaceRepository stub.
// Only GetByID is implemented — Delete would dispatch through and is
// safe to return nil. The other methods would panic via embedded nil.
type secretsTestNamespaceRepo struct {
	secrets.NamespaceRepository
	getByID func(id uuid.UUID) (*secrets.SecretNamespace, error)
	deleteF func(id uuid.UUID) error
}

func (r *secretsTestNamespaceRepo) GetByID(id uuid.UUID) (*secrets.SecretNamespace, error) {
	return r.getByID(id)
}

func (r *secretsTestNamespaceRepo) Delete(id uuid.UUID) error {
	if r.deleteF != nil {
		return r.deleteF(id)
	}
	return nil
}

// ===========================
// A3d-iii: cross-tenant access on namespace-path-id SecretsHandler
// routes must return 404 with existence-secrecy body. Each row
// exercises the LoadOwnedViaAgent guard wired in this PR. The mock
// agentRepo.GetByID returns an agent in a different org; every
// handler must short-circuit at LoadOwnedViaAgent and return 404
// without touching the service backends (StoreCredential and
// RotateCredential would otherwise panic on the nil backends map).
// ===========================

func TestSecretsHandler_NamespaceScoped_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	namespaceID := uuid.New()
	agentID := uuid.New()

	agentRepo := &secretsTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID, Name: "victim-agent"}, nil
		},
	}
	nsRepo := &secretsTestNamespaceRepo{
		getByID: func(id uuid.UUID) (*secrets.SecretNamespace, error) {
			return &secrets.SecretNamespace{
				ID:        namespaceID,
				AgentID:   agentID,
				Namespace: "victim-namespace",
			}, nil
		},
	}
	// SecretsService constructed with only the nsRepo wired — other
	// dependencies are nil, so any path beyond the LoadOwnedViaAgent
	// guard that touches them would panic (which the test harness
	// reports as failure). This is the regression-proofing signal.
	svc := application.NewSecretsService(nsRepo, nil, nil, nil, nil, nil)
	handler := NewSecretsHandler(svc, agentRepo)

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *SecretsHandler)
	}{
		{
			name:        "GetNamespace",
			method:      "GET",
			requestPath: "/secrets/namespaces/" + namespaceID.String(),
			body:        "",
			setup: func(app *fiber.App, h *SecretsHandler) {
				app.Get("/secrets/namespaces/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetNamespace(c)
				})
			},
		},
		{
			name:        "DeleteNamespace",
			method:      "DELETE",
			requestPath: "/secrets/namespaces/" + namespaceID.String(),
			body:        "",
			setup: func(app *fiber.App, h *SecretsHandler) {
				app.Delete("/secrets/namespaces/:id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.DeleteNamespace(c)
				})
			},
		},
		{
			name:        "StoreCredential",
			method:      "POST",
			requestPath: "/secrets/namespaces/" + namespaceID.String() + "/credentials",
			body:        `{"encryptedBlob":"` + base64.StdEncoding.EncodeToString([]byte("dummy")) + `"}`,
			setup: func(app *fiber.App, h *SecretsHandler) {
				app.Post("/secrets/namespaces/:id/credentials", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.StoreCredential(c)
				})
			},
		},
		{
			name:        "RotateCredential",
			method:      "POST",
			requestPath: "/secrets/namespaces/" + namespaceID.String() + "/rotate",
			body:        `{"encryptedBlob":"` + base64.StdEncoding.EncodeToString([]byte("dummy")) + `"}`,
			setup: func(app *fiber.App, h *SecretsHandler) {
				app.Post("/secrets/namespaces/:id/rotate", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.RotateCredential(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}

func (r *secretsTestAgentRepo) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
