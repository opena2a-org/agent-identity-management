package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// secretsTestAuditRepo records audit entries; other methods are not reached.
type secretsTestAuditRepo struct {
	secrets.AuditRepository
	entries []*secrets.SecretAuditEntry
}

func (r *secretsTestAuditRepo) Create(e *secrets.SecretAuditEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

// resolveOverHTTP posts a resolve request signed by priv for agentID, naming
// presentedKey as agentPublicKey. The service has no nonce admission store and
// no namespace repository or backends: a request that reaches them panics.
func resolveOverHTTP(t *testing.T, agentID uuid.UUID, registered ed25519.PublicKey, priv ed25519.PrivateKey, presentedKey ed25519.PublicKey) (int, map[string]string, *secretsTestAuditRepo) {
	t.Helper()
	return resolveFieldsOverHTTP(t, agentID, registered, priv, presentedKey, "github", "read")
}

// resolveFieldsOverHTTP is resolveOverHTTP for the namespace and operation
// given, signed as namespace|operation|nonce.
func resolveFieldsOverHTTP(t *testing.T, agentID uuid.UUID, registered ed25519.PublicKey, priv ed25519.PrivateKey, presentedKey ed25519.PublicKey, namespace, operation string) (int, map[string]string, *secretsTestAuditRepo) {
	t.Helper()
	registeredB64 := base64.StdEncoding.EncodeToString(registered)
	agentRepo := &secretsTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: uuid.New(), PublicKey: &registeredB64}, nil
		},
	}
	audit := &secretsTestAuditRepo{}
	svc := application.NewSecretsService(nil, audit, agentRepo, nil, nil, nil)
	handler := NewSecretsHandler(svc, agentRepo, nil, nil)

	app := fiber.New()
	app.Post("/secrets/resolve", func(c fiber.Ctx) error {
		c.Locals("agent_id", agentID)
		return c.Next()
	}, handler.ResolveCredential)

	nonce := time.Now().UTC().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	body, err := json.Marshal(map[string]string{
		"namespace":      namespace,
		"operation":      operation,
		"nonce":          nonce,
		"signature":      base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(namespace+"|"+operation+"|"+nonce))),
		"agentPublicKey": base64.StdEncoding.EncodeToString(presentedKey),
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/secrets/resolve", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]string
	require.NoError(t, json.Unmarshal(raw, &out))
	return resp.StatusCode, out, audit
}

// Without a working single-use check, a correctly signed request is answered
// 503 and nothing is resolved.
func TestSecretsHandler_Resolve_NonceCheckUnavailableIs503(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	status, body, audit := resolveOverHTTP(t, uuid.New(), pub, priv, pub)

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, map[string]string{"error": "Nonce check unavailable; retry the request with a new nonce"}, body)
	require.Len(t, audit.entries, 1)
	assert.Equal(t, secrets.AuditResultDenied, audit.entries[0].Result)
}

// A request naming a key other than the registered one is refused with 403
// before the single-use check runs.
func TestSecretsHandler_Resolve_OtherAgentPublicKeyIs403(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	status, body, audit := resolveOverHTTP(t, uuid.New(), pub, priv, otherPub)

	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Contains(t, body["error"], "agentPublicKey is not the registered key")
	require.Len(t, audit.entries, 1)
	assert.Equal(t, "agentPublicKey is not the agent's registered key", audit.entries[0].DenyReason)
}

// A correctly signed request whose namespace or operation holds a vertical
// bar, a carriage return or a line feed is refused with 403 and the line that
// names both members and the rule, before its nonce is checked: the signed
// text joins the fields with '|', so it would read as other field values too.
func TestSecretsHandler_Resolve_FieldHoldingASeparatorIs403(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	cases := []struct{ namespace, operation, denyReason string }{
		{"git|hub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"git\rhub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"git\nhub", "read", "namespace contains a vertical bar, a carriage return or a line feed"},
		{"github", "re|ad", "operation contains a vertical bar, a carriage return or a line feed"},
		{"github", "re\rad", "operation contains a vertical bar, a carriage return or a line feed"},
		{"github", "re\nad", "operation contains a vertical bar, a carriage return or a line feed"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q %q", tc.namespace, tc.operation), func(t *testing.T) {
			status, body, audit := resolveFieldsOverHTTP(t, uuid.New(), pub, priv, pub, tc.namespace, tc.operation)

			assert.Equal(t, fiber.StatusForbidden, status)
			assert.Equal(t, map[string]string{"error": "namespace and operation must not contain a vertical bar (|), a carriage return or a line feed"}, body)
			require.Len(t, audit.entries, 1)
			assert.Equal(t, secrets.AuditResultDenied, audit.entries[0].Result)
			assert.Equal(t, tc.denyReason, audit.entries[0].DenyReason)
		})
	}
}
