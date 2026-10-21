package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// A resolve request signs namespace|operation|nonce, and the SDKs' resolve
// refuses, before signing, a namespace that holds a line feed, a carriage
// return or "|", or that begins with "DSSEv1 ". POST /secrets/namespaces
// refuses the same names with 400 and a line naming the rule; the agent is
// not looked up, and nothing is stored or audited for the request.

// namespaceCreateAnswer posts a namespace named name for f's agent, and
// returns the status, the body and the number of agent lookups made.
func namespaceCreateAnswer(t *testing.T, f *secretsAuditFixture, name string) (int, string, int) {
	t.Helper()
	lookups := 0
	agents := &secretsTestAgentRepo{getByID: func(id uuid.UUID) (*domain.Agent, error) {
		lookups++
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

	body, err := json.Marshal(map[string]any{"agentId": f.agentID, "namespace": name, "operations": []string{"read"}})
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/secrets/namespaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(answer), lookups
}

func TestSecretsHandler_CreateNamespaceRefusesANameResolveDoesNotSign(t *testing.T) {
	for _, tc := range []struct {
		class, name, line string
	}{
		{"line feed", "team\nprod",
			"namespace must not contain a line feed: resolve signs namespace|operation|nonce"},
		{"carriage return", "team\rprod",
			"namespace must not contain a carriage return: resolve signs namespace|operation|nonce"},
		{"vertical bar", "team|prod",
			"namespace must not contain a vertical bar (|): resolve signs namespace|operation|nonce"},
		{"DSSE prefix", "DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}",
			`namespace must not begin with "DSSEv1 ", the first bytes of a DSSE statement an agent key signs`},
	} {
		t.Run(tc.class, func(t *testing.T) {
			f := newSecretsAuditFixture()

			status, answer, lookups := namespaceCreateAnswer(t, f, tc.name)

			assert.Equal(t, fiber.StatusBadRequest, status)
			want, err := json.Marshal(fiber.Map{"error": tc.line})
			require.NoError(t, err)
			assert.JSONEq(t, string(want), answer)
			assert.Zero(t, lookups, "the agent was looked up for a refused name")
			assert.Equal(t, "existing", f.namespaces.ns.Namespace, "a namespace was stored")
			assert.Empty(t, f.audit.rows, "a refused request wrote an audit row")
		})
	}
}

// Only those three characters, and the seven characters "DSSEv1 " at the
// start of the name, are refused.
func TestSecretsHandler_CreateNamespaceStoresANameResolveSigns(t *testing.T) {
	for _, name := range []string{"team-prod", "DSSEv1", "dssev1 creds", " DSSEv1 creds", "creds DSSEv1 x"} {
		t.Run(name, func(t *testing.T) {
			f := newSecretsAuditFixture()

			status, _, lookups := namespaceCreateAnswer(t, f, name)

			assert.Equal(t, fiber.StatusCreated, status)
			assert.Equal(t, 1, lookups)
			assert.Equal(t, name, f.namespaces.ns.Namespace)
		})
	}
}
