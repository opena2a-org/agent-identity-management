//go:build integration

package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/awssmtest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/vaultkvtest"
)

// A credential stored in a namespace on the HashiCorp Vault or the AWS Secrets
// Manager backend is resolved by the agent that owns the namespace and opened
// with the agent's key, the way newResolveFixture does it with a mock backend.
// Here the repositories, the single-use nonce store and the audit log are the
// real ones on TEST_DATABASE_URL, and the backend talks to vaultkvtest through
// the Vault client or to awssmtest through the AWS SDK: in-memory servers of
// the two HTTP APIs on loopback ports, so the tests need no vault binary and
// no vendor account. The names carry RoundTrip so the CI step that fails on a
// skipped integration test selects them.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestSecretsService_.*RoundTrip' ./internal/application/

func secretsBackendRoundTripDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping secrets backend round-trip test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// assertSecretsBackendRoundTrip seeds an agent with an Ed25519 key, creates a
// namespace on backend through SecretsService, stores a credential, resolves
// it as the agent, opens it with the agent's key, and deletes the namespace.
func assertSecretsBackendRoundTrip(t *testing.T, db *sql.DB, backend secrets.SecretsBackend) {
	t.Helper()
	ctx := context.Background()
	orgID, userID := seedOrgAndUser(t, db, ctx, "secrets-backend")

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	agentID := uuid.New()
	// Registered after seedOrgAndUser, so it runs first. Deleting the agent
	// removes its namespaces, their audit rows and its admitted nonces.
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	})
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status,
		                     public_key, encrypted_private_key, key_algorithm, key_created_at,
		                     rotation_count, capabilities, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Secrets Backend Agent', 'ai_agent', 'verified',
		         $4, 'not-used-by-resolve', 'Ed25519', NOW(),
		         0, '[]'::jsonb, $5, NOW(), NOW())`,
		agentID, orgID, "secrets-backend-"+agentID.String()[:8],
		base64.StdEncoding.EncodeToString(pub), userID)
	require.NoError(t, err)

	// The nonce store admits nothing until a purge has completed; the server's
	// purge loop runs one at start.
	nonces := NewActionRequestNonceService(repository.NewAgentRequestNonceRepository(db))
	_, err = nonces.Purge(ctx)
	require.NoError(t, err)

	svc := NewSecretsService(
		repository.NewSecretNamespaceRepository(db),
		repository.NewSecretAuditRepository(db),
		repository.NewAgentRepository(db),
		nil, // the ATC path checks the ATC's capabilities, not the capability table
		nil, // the ATC claims below stand for ATCAuthMiddleware's verification
		map[secrets.BackendType]secrets.SecretsBackend{backend.Type(): backend},
	).WithNonceAdmission(nonces)

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: backend.Type(),
		Operations: []string{"read"}, CreatedBy: userID,
	}
	require.NoError(t, svc.CreateNamespace(ns))

	stored := "synthetic-credential-" + uuid.NewString()
	require.NoError(t, svc.StoreCredential(ns.ID, []byte(stored), "AES-256-GCM"))

	nonce := freshNonce()
	result, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace:      "github",
		Operation:      "read",
		Nonce:          nonce,
		Signature:      signRequest(priv, "github", "read", nonce),
		AgentPublicKey: pub,
		ATCClaims: &atcdomain.ATCClaims{
			AgentID:      agentID,
			Issuer:       "aim-test",
			Capabilities: []string{CapabilitySecretsResolve},
			ATCID:        "atc-" + string(backend.Type()),
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		},
	})
	require.NoError(t, err)
	require.Equal(t, "X25519-ChaCha20-Poly1305", result.EncryptionAlg)
	require.NotContains(t, string(result.EncryptedBlob), stored, "the resolve answer carries the credential in the clear")

	plaintext, err := decryptResolvedBlob(priv, result)
	require.NoError(t, err)
	require.Equal(t, stored, string(plaintext), "the agent's key does not open the resolved credential to the stored value")

	var auditResult string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT result FROM secret_audit_log WHERE namespace_id = $1 AND agent_id = $2`,
		ns.ID, agentID).Scan(&auditResult))
	require.Equal(t, string(secrets.AuditResultGranted), auditResult)

	require.NoError(t, svc.DeleteNamespace(ns.ID))
	_, err = backend.Retrieve(ns.ID)
	require.Error(t, err, "the backend still returns the credential after the namespace was deleted")
	var remaining int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM secret_namespaces WHERE id = $1`, ns.ID).Scan(&remaining))
	require.Zero(t, remaining, "the namespace row is still there after DeleteNamespace")
}

func TestSecretsService_HashiCorpVaultRoundTrip(t *testing.T) {
	db := secretsBackendRoundTripDB(t)
	srv := vaultkvtest.NewServer(t)

	assertSecretsBackendRoundTrip(t, db, infrasecrets.NewHashiCorpVaultBackendFromClient(vaultkvtest.NewClient(t, srv), vaultkvtest.Mount))
}

func TestSecretsService_AWSSecretsManagerRoundTrip(t *testing.T) {
	db := secretsBackendRoundTripDB(t)
	srv := awssmtest.NewServer(t)

	assertSecretsBackendRoundTrip(t, db, infrasecrets.NewAWSSecretsBackendFromClient(awssmtest.NewClient(srv), "", ""))
}
