package handlers

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/cyberark/ccptest"
)

// A namespace on the cyberark_ccp backend, through the secrets routes: the
// credential vaulted in CyberArk reaches the agent that owns the namespace
// through POST /secrets/resolve, read from the CCP stub over mutual TLS and
// encrypted to the agent's key, and a store or a rotation through AIM is
// refused. The stub serves TLS and requires a client certificate signed by its
// own CA. The agent_id and atc_claims locals stand for what ATCAuthMiddleware
// sets for a verified ATC.

const (
	ccpRouteAppID = "AIM-Server"
	ccpRouteSafe  = "AIM-Secrets"
)

// ccpNamespaceRepo is a secrets.NamespaceRepository holding the namespaces of
// these tests; the other methods panic through the nil embedded interface.
type ccpNamespaceRepo struct {
	secrets.NamespaceRepository
	namespaces []*secrets.SecretNamespace
}

func (r *ccpNamespaceRepo) GetByID(id uuid.UUID) (*secrets.SecretNamespace, error) {
	for _, ns := range r.namespaces {
		if ns.ID == id {
			return ns, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *ccpNamespaceRepo) GetByAgentAndName(agentID uuid.UUID, name string) (*secrets.SecretNamespace, error) {
	for _, ns := range r.namespaces {
		if ns.AgentID == agentID && ns.Namespace == name {
			return ns, nil
		}
	}
	return nil, nil
}

type ccpRouteFixture struct {
	stub    *ccptest.Server
	agentID uuid.UUID
	priv    ed25519.PrivateKey
	ns      *secrets.SecretNamespace
	resolve *secretsTestAuditRepo // the secrets audit log of resolve requests
	changes *familyAuditRepo      // the audit log of management requests
	app     *fiber.App
}

// newCCPRouteFixture mounts the resolve, store and rotate routes for an agent
// that owns one namespace on cyberark_ccp. With registered false the server
// runs without the CyberArk backend, as it does when CYBERARK_CCP_URL is unset
// or the backend could not be built.
func newCCPRouteFixture(t *testing.T, registered bool) *ccpRouteFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	f := &ccpRouteFixture{
		stub:    ccptest.New(t),
		agentID: uuid.New(),
		priv:    priv,
		resolve: &secretsTestAuditRepo{},
		changes: &familyAuditRepo{},
	}
	orgID, userID := uuid.New(), uuid.New()
	f.ns = &secrets.SecretNamespace{
		ID: uuid.New(), AgentID: f.agentID, Namespace: "orders-db",
		BackendType: secrets.BackendTypeCyberArkCCP, Operations: []string{"read"},
		Status: secrets.NamespaceStatusActive, CreatedBy: userID,
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	agents := &secretsTestAgentRepo{getByID: func(id uuid.UUID) (*domain.Agent, error) {
		if id != f.agentID {
			return nil, errors.New("not found")
		}
		return &domain.Agent{ID: id, OrganizationID: orgID, Name: "orders-agent", PublicKey: &pubB64}, nil
	}}
	backends := map[secrets.BackendType]secrets.SecretsBackend{}
	if registered {
		backend, err := infrasecrets.NewCyberArkCCPBackend(&infrasecrets.CyberArkCCPConfig{
			CCPURL: f.stub.URL, AppID: ccpRouteAppID, Safe: ccpRouteSafe,
			CertPath: f.stub.ClientCertPath, KeyPath: f.stub.ClientKeyPath, CACertPath: f.stub.CACertPath,
		})
		require.NoError(t, err)
		backends[secrets.BackendTypeCyberArkCCP] = backend
	}
	svc := application.NewSecretsService(&ccpNamespaceRepo{namespaces: []*secrets.SecretNamespace{f.ns}},
		f.resolve, agents, nil, nil, backends).WithNonceAdmission(newMemoryAdmitter())
	h := NewSecretsHandler(svc, agents, nil, application.NewAuditService(f.changes))

	asAgent := func(c fiber.Ctx) error {
		c.Locals("agent_id", f.agentID)
		c.Locals("atc_claims", &atcdomain.ATCClaims{
			AgentID:      f.agentID,
			Issuer:       "aim-test",
			Capabilities: []string{application.CapabilitySecretsResolve},
			ATCID:        "atc-ccp-route",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		})
		return c.Next()
	}
	asMember := func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("organization_id", orgID)
		return c.Next()
	}
	f.app = fiber.New()
	f.app.Post("/secrets/resolve", asAgent, h.ResolveCredential)
	f.app.Post("/secrets/namespaces/:id/credentials", asMember, h.StoreCredential)
	f.app.Post("/secrets/namespaces/:id/rotate", asMember, h.RotateCredential)
	return f
}

// vault puts an account in the stub under the name the backend asks for.
func (f *ccpRouteFixture) vault(nsID uuid.UUID, content string) {
	f.stub.AddAccount(ccptest.Account{
		AppID: ccpRouteAppID, Safe: ccpRouteSafe, Object: "aim-secrets-" + nsID.String(),
		Content: content, UserName: "svc_orders",
	})
}

func (f *ccpRouteFixture) send(t *testing.T, path, body string) (int, map[string]string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]string
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	return resp.StatusCode, out
}

// resolveSigned posts a resolve request for the namespace, signed by the agent.
func (f *ccpRouteFixture) resolveSigned(t *testing.T) (int, map[string]string) {
	t.Helper()
	nonce := time.Now().UTC().Format(time.RFC3339Nano) + ":" + uuid.NewString()
	signed := f.ns.Namespace + "|read|" + nonce
	body, err := json.Marshal(map[string]string{
		"namespace":      f.ns.Namespace,
		"operation":      "read",
		"nonce":          nonce,
		"signature":      base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, []byte(signed))),
		"agentPublicKey": base64.StdEncoding.EncodeToString(f.priv.Public().(ed25519.PublicKey)),
	})
	require.NoError(t, err)
	return f.send(t, "/secrets/resolve", string(body))
}

// openResolved opens a resolve answer with the X25519 key derived from the
// agent's Ed25519 key, the way the SDKs do.
func openResolved(t *testing.T, priv ed25519.PrivateKey, answer map[string]string) []byte {
	t.Helper()
	blob, err := base64.StdEncoding.DecodeString(answer["encryptedBlob"])
	require.NoError(t, err)
	ephemeralBytes, err := base64.StdEncoding.DecodeString(answer["ephemeralPubKey"])
	require.NoError(t, err)
	seed := sha512.Sum512(priv.Seed())
	xPriv, err := ecdh.X25519().NewPrivateKey(seed[:32])
	require.NoError(t, err)
	ephemeral, err := ecdh.X25519().NewPublicKey(ephemeralBytes)
	require.NoError(t, err)
	shared, err := xPriv.ECDH(ephemeral)
	require.NoError(t, err)
	key := sha256.Sum256(shared)
	aead, err := chacha20poly1305.New(key[:])
	require.NoError(t, err)
	require.Greater(t, len(blob), aead.NonceSize())
	plaintext, err := aead.Open(nil, blob[:aead.NonceSize()], blob[aead.NonceSize():], nil)
	require.NoError(t, err)
	return plaintext
}

func TestSecretsHandler_CyberArkNamespace_ResolvesTheCredentialVaultedInCyberArk(t *testing.T) {
	f := newCCPRouteFixture(t, true)
	f.vault(f.ns.ID, "FAKE-CCP-CONTENT-1")
	f.vault(uuid.New(), "FAKE-CCP-CONTENT-2")

	status, answer := f.resolveSigned(t)

	require.Equal(t, fiber.StatusOK, status, "answer: %v", answer)
	assert.Equal(t, "X25519-ChaCha20-Poly1305", answer["encryptionAlg"])
	assert.NotContains(t, answer["encryptedBlob"], base64.StdEncoding.EncodeToString([]byte("FAKE-CCP-CONTENT-1")))
	assert.Equal(t, "FAKE-CCP-CONTENT-1", string(openResolved(t, f.priv, answer)),
		"the agent's key opens the answer to the credential vaulted for its namespace")

	requests := f.stub.Requests()
	require.Len(t, requests, 1, "one CCP request per resolve")
	assert.Equal(t, ccptest.ClientCommonName, requests[0].ClientCommonName)
	assert.Equal(t, []string{"aim-secrets-" + f.ns.ID.String()}, requests[0].Params["Object"])
	assert.Equal(t, []string{ccpRouteSafe}, requests[0].Params["Safe"])
	assert.Equal(t, []string{ccpRouteAppID}, requests[0].Params["AppID"])

	require.Len(t, f.resolve.entries, 1)
	entry := f.resolve.entries[0]
	assert.Equal(t, secrets.AuditResultGranted, entry.Result)
	assert.Equal(t, f.ns.ID, entry.NamespaceID)
	assert.Equal(t, f.agentID, entry.AgentID)
	assert.Equal(t, "atc-ccp-route", entry.ATCID)
}

func TestSecretsHandler_CyberArkNamespace_RefusesStoreAndRotate(t *testing.T) {
	for _, route := range []string{"credentials", "rotate"} {
		t.Run(route, func(t *testing.T) {
			f := newCCPRouteFixture(t, true)
			f.vault(f.ns.ID, "FAKE-CCP-CONTENT-1")

			status, answer := f.send(t, "/secrets/namespaces/"+f.ns.ID.String()+"/"+route, blobBodyOf("FAKE-CCP-CONTENT-2"))

			assert.Equal(t, fiber.StatusConflict, status)
			assert.Equal(t, map[string]string{"error": "the namespace's backend is read-only: store and rotate its credential in the backend"}, answer)
			assert.Empty(t, f.changes.rows, "a refused request writes no audit entry")
			assert.Empty(t, f.stub.Requests(), "nothing is sent to CCP")
		})
	}
}

// Without the backend, as when CYBERARK_CCP_URL is unset or the backend could
// not be built, a resolve request for a cyberark_ccp namespace is refused and
// nothing is sent to CCP.
func TestSecretsHandler_CyberArkNamespace_ResolveIsRefusedWithoutTheBackend(t *testing.T) {
	f := newCCPRouteFixture(t, false)
	f.vault(f.ns.ID, "FAKE-CCP-CONTENT-1")

	status, answer := f.resolveSigned(t)

	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Equal(t, map[string]string{"error": "Secret resolution refused"}, answer)
	require.Len(t, f.resolve.entries, 1)
	assert.Equal(t, secrets.AuditResultDenied, f.resolve.entries[0].Result)
	assert.Equal(t, "backend not available: cyberark_ccp", f.resolve.entries[0].DenyReason)
	assert.Empty(t, f.stub.Requests())
}

// A resolve request is refused when CCP finds no account for the namespace in
// the safe, and when CCP answers with an error.
func TestSecretsHandler_CyberArkNamespace_ResolveIsRefusedWhenCCPServesNoAccount(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*ccpRouteFixture)
	}{
		{"no account for the namespace", func(f *ccpRouteFixture) {
			f.vault(uuid.New(), "FAKE-CCP-CONTENT-2")
		}},
		{"CCP answers 500", func(f *ccpRouteFixture) {
			f.vault(f.ns.ID, "FAKE-CCP-CONTENT-1")
			f.stub.AnswerEvery(http.StatusInternalServerError, "APPAP282E The password is being changed by the CPM.")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCCPRouteFixture(t, true)
			tc.setup(f)

			status, answer := f.resolveSigned(t)

			assert.Equal(t, fiber.StatusForbidden, status)
			assert.Equal(t, map[string]string{"error": "Secret resolution refused"}, answer)
			require.Len(t, f.resolve.entries, 1)
			assert.Equal(t, secrets.AuditResultDenied, f.resolve.entries[0].Result)
			assert.Equal(t, "credential retrieval failed", f.resolve.entries[0].DenyReason)
			assert.Len(t, f.stub.Requests(), 1)
		})
	}
}
