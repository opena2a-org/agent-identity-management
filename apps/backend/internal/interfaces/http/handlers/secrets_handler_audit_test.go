package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// Each secrets management request that succeeds writes one audit entry in the
// caller's organization, under the caller's user id, with the namespace as
// its resource and the backend as its only metadata; a refused or failed one
// writes none. The server's audit log repository forwards these entries to a
// configured SIEM.

// auditedNamespaceRepo is a secrets.NamespaceRepository holding one
// namespace; the other methods panic through the nil embedded interface.
type auditedNamespaceRepo struct {
	secrets.NamespaceRepository
	ns        *secrets.SecretNamespace
	createErr error
	deleteErr error
}

func (r *auditedNamespaceRepo) Create(ns *secrets.SecretNamespace) error {
	if r.createErr != nil {
		return r.createErr
	}
	ns.ID = uuid.New()
	r.ns = ns
	return nil
}

func (r *auditedNamespaceRepo) GetByID(id uuid.UUID) (*secrets.SecretNamespace, error) {
	if r.ns == nil || r.ns.ID != id {
		return nil, errors.New("not found")
	}
	return r.ns, nil
}

func (r *auditedNamespaceRepo) Delete(uuid.UUID) error { return r.deleteErr }

// auditedBackend is a secrets.SecretsBackend whose writes answer err.
type auditedBackend struct {
	secrets.SecretsBackend
	err error
}

func (b *auditedBackend) Store(uuid.UUID, []byte, string) error  { return b.err }
func (b *auditedBackend) Rotate(uuid.UUID, []byte, string) error { return b.err }
func (b *auditedBackend) Delete(uuid.UUID) error                 { return b.err }

// failingAuditRepo refuses every write.
type failingAuditRepo struct{ domain.AuditLogRepository }

func (failingAuditRepo) Create(*domain.AuditLog) error { return errors.New("connection refused") }

type secretsAuditFixture struct {
	orgID, userID, agentID uuid.UUID
	agentOrgID             uuid.UUID // the organization of the namespace's agent
	namespaces             *auditedNamespaceRepo
	backend                *auditedBackend
	audit                  *familyAuditRepo
	auditRepo              domain.AuditLogRepository
}

func newSecretsAuditFixture() *secretsAuditFixture {
	f := &secretsAuditFixture{orgID: uuid.New(), userID: uuid.New(), agentID: uuid.New(), audit: &familyAuditRepo{}}
	f.agentOrgID = f.orgID
	f.auditRepo = f.audit
	f.backend = &auditedBackend{}
	f.namespaces = &auditedNamespaceRepo{ns: &secrets.SecretNamespace{
		ID: uuid.New(), AgentID: f.agentID, Namespace: "existing", BackendType: secrets.BackendTypeAIMNative,
	}}
	return f
}

func (f *secretsAuditFixture) app() *fiber.App {
	agents := &secretsTestAgentRepo{getByID: func(id uuid.UUID) (*domain.Agent, error) {
		return &domain.Agent{ID: id, OrganizationID: f.agentOrgID, Name: "secrets-agent"}, nil
	}}
	backends := map[secrets.BackendType]secrets.SecretsBackend{secrets.BackendTypeAIMNative: f.backend}
	svc := application.NewSecretsService(f.namespaces, nil, agents, nil, nil, backends)
	h := NewSecretsHandler(svc, agents, nil, application.NewAuditService(f.auditRepo))

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", f.userID)
		c.Locals("organization_id", f.orgID)
		return c.Next()
	})
	app.Post("/secrets/namespaces", h.CreateNamespace)
	app.Delete("/secrets/namespaces/:id", h.DeleteNamespace)
	app.Post("/secrets/namespaces/:id/credentials", h.StoreCredential)
	app.Post("/secrets/namespaces/:id/rotate", h.RotateCredential)
	return app
}

func (f *secretsAuditFixture) send(t *testing.T, method, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "secrets-audit-test/1.0")
	resp, err := f.app().Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func blobBodyOf(value string) string {
	return `{"encryptedBlob":"` + base64.StdEncoding.EncodeToString([]byte(value)) + `"}`
}

func TestSecretsHandler_EachChangeWritesOneAuditRow(t *testing.T) {
	credential := "cred-" + uuid.NewString()
	for _, tc := range []struct {
		name         string
		method, path string // %s is the existing namespace's id
		body         string
		status       int
		action       domain.AuditAction
		resourceType string
		backendType  string
	}{
		{"namespace create", "POST", "/secrets/namespaces", "", fiber.StatusCreated, domain.AuditActionCreate, "secret_namespace", "aim_native"},
		{"credential store", "POST", "/secrets/namespaces/%s/credentials", blobBodyOf(credential), fiber.StatusCreated, domain.AuditActionCreate, "secret_credential", "aim_native"},
		{"credential rotate", "POST", "/secrets/namespaces/%s/rotate", blobBodyOf(credential), fiber.StatusOK, domain.AuditActionUpdate, "secret_credential", "aim_native"},
		{"namespace delete", "DELETE", "/secrets/namespaces/%s", "", fiber.StatusOK, domain.AuditActionDelete, "secret_namespace", "aim_native"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSecretsAuditFixture()
			body := tc.body
			if tc.name == "namespace create" {
				body = fmt.Sprintf(`{"agentId":%q,"namespace":"payments","operations":["read"]}`, f.agentID)
			}
			path := tc.path
			if strings.Contains(path, "%s") {
				path = fmt.Sprintf(path, f.namespaces.ns.ID)
			}

			require.Equal(t, tc.status, f.send(t, tc.method, path, body))

			require.Len(t, f.audit.rows, 1, "one audit row for the request")
			row := f.audit.rows[0]
			assert.Equal(t, tc.action, row.Action)
			assert.Equal(t, tc.resourceType, row.ResourceType)
			assert.Equal(t, f.namespaces.ns.ID, row.ResourceID, "the resource is the namespace")
			assert.Equal(t, f.orgID, row.OrganizationID)
			require.NotNil(t, row.UserID)
			assert.Equal(t, f.userID, *row.UserID)
			assert.Nil(t, row.AgentID)
			assert.Equal(t, "secrets-audit-test/1.0", row.UserAgent)
			assert.Equal(t, map[string]interface{}{"backendType": tc.backendType}, row.Metadata)
			for _, value := range []string{credential, base64.StdEncoding.EncodeToString([]byte(credential))} {
				assert.NotContains(t, fmt.Sprint(row.Metadata), value, "the row carries the credential")
			}
		})
	}
}

// The backend recorded is the namespace's own, whichever one it names.
func TestSecretsHandler_AuditRowNamesTheNamespaceBackend(t *testing.T) {
	f := newSecretsAuditFixture()
	body := fmt.Sprintf(`{"agentId":%q,"namespace":"payments","backendType":"vault","operations":["read"]}`, f.agentID)

	require.Equal(t, fiber.StatusCreated, f.send(t, "POST", "/secrets/namespaces", body))

	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, map[string]interface{}{"backendType": "vault"}, f.audit.rows[0].Metadata)
}

func TestSecretsHandler_RefusedOrFailedChangeWritesNoAuditRow(t *testing.T) {
	failed := errors.New("backend unavailable")
	tooLarge := base64.StdEncoding.EncodeToString(make([]byte, application.MaxCredentialBlobSize+1))
	for _, tc := range []struct {
		name         string
		method, path string
		body         string
		setup        func(*secretsAuditFixture)
		status       int
	}{
		{"create without a namespace name", "POST", "/secrets/namespaces", `{"agentId":"%a"}`, nil, fiber.StatusBadRequest},
		{"create for another organization's agent", "POST", "/secrets/namespaces", `{"agentId":"%a","namespace":"payments"}`,
			func(f *secretsAuditFixture) { f.agentOrgID = uuid.New() }, fiber.StatusNotFound},
		{"create the repository refuses", "POST", "/secrets/namespaces", `{"agentId":"%a","namespace":"payments"}`,
			func(f *secretsAuditFixture) { f.namespaces.createErr = failed }, fiber.StatusInternalServerError},
		{"store with a blob that is not base64", "POST", "/secrets/namespaces/%s/credentials", `{"encryptedBlob":"%%%"}`, nil, fiber.StatusBadRequest},
		{"store a blob over the size limit", "POST", "/secrets/namespaces/%s/credentials", `{"encryptedBlob":"` + tooLarge + `"}`, nil, fiber.StatusRequestEntityTooLarge},
		{"store in another organization's namespace", "POST", "/secrets/namespaces/%s/credentials", blobBodyOf("value"),
			func(f *secretsAuditFixture) { f.agentOrgID = uuid.New() }, fiber.StatusNotFound},
		{"store the backend refuses", "POST", "/secrets/namespaces/%s/credentials", blobBodyOf("value"),
			func(f *secretsAuditFixture) { f.backend.err = failed }, fiber.StatusInternalServerError},
		{"rotate without a blob", "POST", "/secrets/namespaces/%s/rotate", `{}`, nil, fiber.StatusBadRequest},
		{"rotate in another organization's namespace", "POST", "/secrets/namespaces/%s/rotate", blobBodyOf("value"),
			func(f *secretsAuditFixture) { f.agentOrgID = uuid.New() }, fiber.StatusNotFound},
		{"rotate the backend refuses", "POST", "/secrets/namespaces/%s/rotate", blobBodyOf("value"),
			func(f *secretsAuditFixture) { f.backend.err = failed }, fiber.StatusInternalServerError},
		{"delete another organization's namespace", "DELETE", "/secrets/namespaces/%s", "",
			func(f *secretsAuditFixture) { f.agentOrgID = uuid.New() }, fiber.StatusNotFound},
		{"delete an unknown namespace", "DELETE", "/secrets/namespaces/" + uuid.NewString(), "", nil, fiber.StatusNotFound},
		{"delete the repository refuses", "DELETE", "/secrets/namespaces/%s", "",
			func(f *secretsAuditFixture) { f.namespaces.deleteErr = failed }, fiber.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSecretsAuditFixture()
			if tc.setup != nil {
				tc.setup(f)
			}
			path := strings.Replace(tc.path, "%s", f.namespaces.ns.ID.String(), 1)
			body := strings.Replace(tc.body, "%a", f.agentID.String(), 1)

			assert.Equal(t, tc.status, f.send(t, tc.method, path, body))
			assert.Empty(t, f.audit.rows, "a refused or failed request writes no audit row")
		})
	}
}

// The change has been made when the audit write fails, so the answer stands.
func TestSecretsHandler_AuditWriteFailureLeavesTheAnswer(t *testing.T) {
	f := newSecretsAuditFixture()
	f.auditRepo = failingAuditRepo{}

	assert.Equal(t, fiber.StatusCreated,
		f.send(t, "POST", "/secrets/namespaces/"+f.namespaces.ns.ID.String()+"/credentials", blobBodyOf("value")))
}
