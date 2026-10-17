package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// An API key's creation, an agent's key read (raw or inside its SDK), rotated
// or replaced, a secret's rotation and a webhook's creation, update or secret
// rotation refuse
// an acting access token whose session (family) was revoked, through the same
// guard as the three credential-minting routes (credential_mint_family_test.go):
// the unchanged 401, one credential_mint_refused row and one SECURITY line
// under the route's own label, and no call to anything behind the route. A
// live session proceeds; a store that does not answer follows the fail-open
// setting and records nothing; a token with no session is not checked.

// sessionRoute is one guarded route, mounted on its real handler over stubs
// that append every service and repository call to calls.
type sessionRoute struct {
	label      string // the route label the refusal records
	method     string
	route      string
	body       string
	liveStatus int
	mint       string // the call that hands out or changes what the route guards
	handler    func(f *mintFixture, calls *[]string) fiber.Handler
}

func sessionRoutes() []sessionRoute {
	keyBody := fmt.Sprintf(`{"agentId":%q,"name":"mint-key"}`, uuid.NewString())
	blobBody := fmt.Sprintf(`{"encryptedBlob":%q}`, base64.StdEncoding.EncodeToString([]byte("FAKE-BLOB")))
	hookBody := `{"name":"mint-hook","url":"https://hooks.example.com/new","events":["agent.created"]}`
	return []sessionRoute{
		{label: "api_key_create", method: "POST", route: "/api-keys", body: keyBody, liveStatus: fiber.StatusCreated,
			mint: "APIKeyRepository.Create", handler: handlerOf(apiKeyHandlerFor, (*APIKeyHandler).CreateAPIKey)},
		{label: "agent_rotate_credentials", method: "POST", route: "/agents/:id/rotate-credentials", liveStatus: fiber.StatusOK,
			mint: "RotateCredentials", handler: handlerOf(agentHandlerFor, (*AgentHandler).RotateCredentials)},
		{label: "agent_credentials", method: "GET", route: "/agents/:id/credentials", liveStatus: fiber.StatusOK,
			mint: "GetAgentCredentials", handler: handlerOf(agentHandlerFor, (*AgentHandler).GetCredentials)},
		{label: "agent_sdk_download", method: "GET", route: "/agents/:id/sdk", liveStatus: fiber.StatusOK,
			mint: "GetAgentCredentials", handler: handlerOf(agentHandlerFor, (*AgentHandler).DownloadSDK)},
		{label: "agent_keys_update", method: "PUT", route: "/agents/:id/keys", body: `{"publicKey":"FAKE-PUBLIC-KEY"}`, liveStatus: fiber.StatusOK,
			mint: "UpdateAgentPublicKey", handler: handlerOf(agentHandlerFor, (*AgentHandler).UpdateAgentKeys)},
		{label: "secret_rotate", method: "POST", route: "/secrets/namespaces/:id/rotate", body: blobBody, liveStatus: fiber.StatusOK,
			mint: "SecretsBackend.Rotate", handler: handlerOf(secretsHandlerFor, (*SecretsHandler).RotateCredential)},
		{label: "webhook_create", method: "POST", route: "/webhooks", body: hookBody, liveStatus: fiber.StatusCreated,
			mint: "CreateWebhook", handler: handlerOf(webhookHandlerFor, (*WebhookHandler).CreateWebhook)},
		{label: "webhook_update", method: "PUT", route: "/webhooks/:id", body: hookBody, liveStatus: fiber.StatusOK,
			mint: "UpdateWebhook", handler: handlerOf(webhookHandlerFor, (*WebhookHandler).UpdateWebhook)},
		{label: "webhook_rotate_secret", method: "POST", route: "/webhooks/:id/rotate-secret", liveStatus: fiber.StatusOK,
			mint: "RotateWebhookSecret", handler: handlerOf(webhookHandlerFor, (*WebhookHandler).RotateWebhookSecret)},
	}
}

// handlerOf binds a handler method to a handler built over recording stubs.
func handlerOf[H any](build func(*mintFixture, *[]string) H, method func(H, fiber.Ctx) error) func(*mintFixture, *[]string) fiber.Handler {
	return func(f *mintFixture, calls *[]string) fiber.Handler {
		h := build(f, calls)
		return func(c fiber.Ctx) error { return method(h, c) }
	}
}

func sendSessionRoute(t *testing.T, f *mintFixture, family string, cell sessionRoute, calls *[]string) (int, string) {
	t.Helper()
	app := fiber.New()
	app.Add([]string{cell.method}, cell.route, f.acting(family, cell.handler(f, calls)))
	return doRequest(t, app, cell.method, strings.Replace(cell.route, ":id", uuid.NewString(), 1), cell.body)
}

func countOf(calls []string, name string) int {
	n := 0
	for _, call := range calls {
		if call == name {
			n++
		}
	}
	return n
}

// R1-R8: a revoked session is refused on each route with the unchanged 401,
// nothing behind the route is called, and the refusal is recorded once under
// the route's own label.
func TestCredentialRoutes_RefuseARevokedSession(t *testing.T) {
	for _, cell := range sessionRoutes() {
		t.Run(cell.label, func(t *testing.T) {
			logBuf := captureLog(t)
			f := newMintFixture(t, &rotationStore{}, false)
			f.revokeFamily()
			var calls []string
			status, body := sendSessionRoute(t, f, f.family, cell, &calls)
			assert.Equal(t, fiber.StatusUnauthorized, status)
			// the body is not printed: a route that was not refused answers with the credential
			assert.True(t, strings.Contains(body, mintRefusal), "the body is the refresh route's refusal")
			assert.Empty(t, calls, "nothing behind the route is called")
			assertRefusedRow(t, f, cell.label)
			assert.Equal(t, 1, strings.Count(logBuf.String(), "SECURITY credential_mint_refused route="+cell.label+" "))
		})
	}
}

// L1-L8 (control): a live session reaches each route, which hands out or
// changes what it guards once, and no refusal is recorded.
func TestCredentialRoutes_LiveSessionProceeds(t *testing.T) {
	for _, cell := range sessionRoutes() {
		t.Run(cell.label, func(t *testing.T) {
			f := newMintFixture(t, &rotationStore{}, false)
			var calls []string
			status, _ := sendSessionRoute(t, f, f.family, cell, &calls)
			assert.Equal(t, cell.liveStatus, status)
			assert.Equal(t, 1, countOf(calls, cell.mint), "calls: %v", calls)
			assert.Empty(t, refusedRows(f.audit.rows), "no refusal is recorded")
		})
	}
}

// U1-U8: a store that does not answer records nothing and follows the
// fail-open setting on each route.
func TestCredentialRoutes_UnansweredStoreFollowsFailOpen(t *testing.T) {
	for _, cell := range sessionRoutes() {
		t.Run(cell.label, func(t *testing.T) {
			logBuf := captureLog(t)
			closed := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, false)
			var calls []string
			status, _ := sendSessionRoute(t, closed, closed.family, cell, &calls)
			assert.Equal(t, fiber.StatusUnauthorized, status, "fail-closed refuses")
			assert.Empty(t, calls)
			assert.Empty(t, closed.audit.rows)
			assert.NotContains(t, logBuf.String(), "SECURITY")

			open := newMintFixture(t, &rotationStore{existsErr: errors.New("store down")}, true)
			calls = nil
			status, _ = sendSessionRoute(t, open, open.family, cell, &calls)
			assert.Equal(t, cell.liveStatus, status, "fail-open serves")
			assert.Equal(t, 1, countOf(calls, cell.mint), "calls: %v", calls)
			assert.Empty(t, refusedRows(open.audit.rows))
		})
	}
}

// T1-T8: an access token with no session (minted before sessions carried an
// sid) is not checked on each route.
func TestCredentialRoutes_NoSessionIsNotChecked(t *testing.T) {
	for _, cell := range sessionRoutes() {
		t.Run(cell.label, func(t *testing.T) {
			f := newMintFixture(t, &rotationStore{}, false)
			f.revokeFamily()
			var calls []string
			status, _ := sendSessionRoute(t, f, "", cell, &calls)
			assert.Equal(t, cell.liveStatus, status)
			assert.Equal(t, 1, countOf(calls, cell.mint), "calls: %v", calls)
			assert.Empty(t, refusedRows(f.audit.rows))
		})
	}
}

// N1: a handler built without an audit service passes a nil *AuditService;
// the guard still refuses a revoked session and records no row.
func TestRefuseIfFamilyRevoked_NilAuditServiceRefusesWithoutARow(t *testing.T) {
	logBuf := captureLog(t)
	f := newMintFixture(t, &rotationStore{}, false)
	f.revokeFamily()
	var audit *application.AuditService
	app := fiber.New()
	app.Post("/guarded", f.acting(f.family, func(c fiber.Ctx) error {
		if !refuseIfFamilyRevoked(c, f.svc, audit, "guarded") {
			return nil
		}
		return c.SendStatus(fiber.StatusNoContent)
	}))
	status, body := doRequest(t, app, "POST", "/guarded", "")
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, mintRefusal)
	assert.Empty(t, f.audit.rows)
	assert.Equal(t, 1, strings.Count(logBuf.String(), "SECURITY credential_mint_refused route=guarded "))
}

// ---- handlers over recording stubs ----------------------------------------

// mintAPIKeyRepo is a domain.APIKeyRepository that records each key it creates.
type mintAPIKeyRepo struct {
	domain.APIKeyRepository
	calls *[]string
}

func (r *mintAPIKeyRepo) Create(key *domain.APIKey) error {
	*r.calls = append(*r.calls, "APIKeyRepository.Create")
	key.ID = uuid.New()
	return nil
}

// mintSecretsBackend is a secrets.SecretsBackend that records each rotation.
type mintSecretsBackend struct {
	secrets.SecretsBackend
	calls *[]string
}

func (b *mintSecretsBackend) Rotate(uuid.UUID, []byte, string) error {
	*b.calls = append(*b.calls, "SecretsBackend.Rotate")
	return nil
}

// mintAgentRepo answers every agent lookup with an agent of the acting organization.
func mintAgentRepo(f *mintFixture, calls *[]string) *secretsTestAgentRepo {
	return &secretsTestAgentRepo{getByID: func(id uuid.UUID) (*domain.Agent, error) {
		*calls = append(*calls, "AgentRepository.GetByID")
		return &domain.Agent{ID: id, OrganizationID: f.orgID, Name: "mint-agent"}, nil
	}}
}

func apiKeyHandlerFor(f *mintFixture, calls *[]string) *APIKeyHandler {
	keys := &mintAPIKeyRepo{calls: calls}
	return NewAPIKeyHandler(application.NewAPIKeyService(keys, mintAgentRepo(f, calls)), application.NewAuditService(f.audit), keys, f.svc)
}

func agentHandlerFor(f *mintFixture, calls *[]string) *AgentHandler {
	agents := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			*calls = append(*calls, "GetAgent")
			return &domain.Agent{ID: id, OrganizationID: f.orgID, Name: "mint-agent"}, nil
		},
		RotateCredentialsFunc: func(context.Context, uuid.UUID) (string, string, error) {
			*calls = append(*calls, "RotateCredentials")
			return "FAKE-PUBLIC-KEY", "FAKE-PRIVATE-KEY", nil
		},
		GetAgentCredentialsFunc: func(context.Context, uuid.UUID) (string, string, error) {
			*calls = append(*calls, "GetAgentCredentials")
			return "FAKE-PUBLIC-KEY", "FAKE-PRIVATE-KEY", nil
		},
		UpdateAgentPublicKeyFunc: func(context.Context, uuid.UUID, string) error {
			*calls = append(*calls, "UpdateAgentPublicKey")
			return nil
		},
	}
	h := NewAgentHandlerWithInterfaces(agents, nil, application.NewAuditService(f.audit), nil, nil, nil, nil, nil, nil, nil, nil)
	h.jwtService = f.svc
	return h
}

func secretsHandlerFor(f *mintFixture, calls *[]string) *SecretsHandler {
	agentID := uuid.New()
	namespaces := &secretsTestNamespaceRepo{getByID: func(id uuid.UUID) (*secrets.SecretNamespace, error) {
		*calls = append(*calls, "NamespaceRepository.GetByID")
		return &secrets.SecretNamespace{ID: id, AgentID: agentID, Namespace: "mint-namespace", BackendType: secrets.BackendTypeAIMNative}, nil
	}}
	agents := mintAgentRepo(f, calls)
	backends := map[secrets.BackendType]secrets.SecretsBackend{secrets.BackendTypeAIMNative: &mintSecretsBackend{calls: calls}}
	svc := application.NewSecretsService(namespaces, nil, agents, nil, nil, backends)
	return NewSecretsHandler(svc, agents, f.svc, application.NewAuditService(f.audit))
}

func webhookHandlerFor(f *mintFixture, calls *[]string) *WebhookHandler {
	webhooks := &MockWebhookServicerImpl{
		CreateWebhookFunc: func(_ context.Context, req *application.CreateWebhookRequest, orgID, _ uuid.UUID) (*domain.Webhook, error) {
			*calls = append(*calls, "CreateWebhook")
			return &domain.Webhook{ID: uuid.New(), OrganizationID: orgID, Name: req.Name, URL: req.URL}, nil
		},
		GetWebhookFunc: func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) {
			*calls = append(*calls, "GetWebhook")
			return &domain.Webhook{ID: id, OrganizationID: f.orgID, Name: "mint-hook", URL: "https://hooks.example.com/old"}, nil
		},
		UpdateWebhookFunc: func(_ context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error) {
			*calls = append(*calls, "UpdateWebhook")
			return &domain.Webhook{ID: id, OrganizationID: f.orgID, Name: req.Name, URL: req.URL}, nil
		},
		RotateWebhookSecretFunc: func(_ context.Context, _, _ uuid.UUID) (string, error) {
			*calls = append(*calls, "RotateWebhookSecret")
			return "rotated-" + uuid.NewString(), nil
		},
	}
	h := NewWebhookHandlerWithInterfaces(webhooks, application.NewAuditService(f.audit))
	h.jwtService = f.svc
	return h
}
