package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// a2aAgentByIDColumns are the columns AgentRepository.GetByID selects, in order.
var a2aAgentByIDColumns = []string{
	"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version",
	"public_key", "encrypted_private_key", "key_algorithm", "certificate_url", "repository_url", "documentation_url",
	"trust_score", "verified_at", "talks_to", "capabilities", "metadata", "created_at", "updated_at", "created_by", "last_active",
	"key_created_at", "key_expires_at", "key_rotation_grace_until", "previous_public_key", "rotation_count",
	"pqc_public_key", "pqc_key_algorithm", "hybrid_mode_enabled", "pqc_key_created_at", "pqc_key_expires_at", "previous_pqc_public_key",
	"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
	"updated_by", "updated_by_name", "updated_by_email",
	"capability_violation_count", "is_compromised", "declared_purpose",
}

// postSignedA2AVerify posts to POST /a2a/verify a request for agentID whose
// request hash is signed with priv, with the first signature byte flipped when
// flip is set, against an agent row in hybrid mode holding pub.
func postSignedA2AVerify(t *testing.T, pub ed25519.PublicKey, priv ed25519.PrivateKey, flip bool) (int, string) {
	t.Helper()
	agentID := uuid.New()
	nonceDB, nonceMock, err := sqlmock.New()
	require.NoError(t, err)
	defer nonceDB.Close()
	agentDB, agentMock, err := sqlmock.New()
	require.NoError(t, err)
	defer agentDB.Close()

	now := time.Now().UTC()
	agentMock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WithArgs(agentID).WillReturnRows(
		sqlmock.NewRows(a2aAgentByIDColumns).AddRow(
			agentID.String(), uuid.New().String(), "billing-agent", "Billing Agent", nil, "ai_agent", "verified", nil,
			base64.StdEncoding.EncodeToString(pub), nil, "Ed25519", nil, nil, nil,
			0.8, nil, nil, []byte(`["api:call"]`), nil, now, now, uuid.New().String(), nil,
			nil, nil, nil, nil, nil,
			"FAKE-MLDSA-PUBLIC-KEY", "ML-DSA-65", true, now, nil, nil,
			"", "", nil, nil,
			nil, "", "",
			int64(0), false, nil,
		))
	nonceMock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM a2a_request_nonces WHERE nonce = \$1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	svc := application.NewA2AService(
		nil, nil, nil, nil, nil, nil,
		repository.NewA2ARequestNonceRepository(nonceDB),
		nil, nil, nil, nil, nil,
		repository.NewAgentRepository(agentDB),
		nil, nil, nil,
	)
	app := fiber.New()
	app.Post("/a2a/verify", NewA2AHandler(svc, nil, nil).VerifyRequest)

	hash := sha256.Sum256([]byte("POST /tasks " + agentID.String()))
	signature := ed25519.Sign(priv, hash[:])
	if flip {
		signature[0] ^= 0x01
	}
	body, err := json.Marshal(application.VerifyA2ARequestRequest{
		AgentID:     agentID,
		Timestamp:   now.Unix(),
		Nonce:       uuid.New().String(),
		Signature:   base64.StdEncoding.EncodeToString(signature),
		RequestHash: hex.EncodeToString(hash[:]),
	})
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/a2a/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NoError(t, agentMock.ExpectationsWereMet())
	assert.NoError(t, nonceMock.ExpectationsWereMet(), "only the nonce check is made, and no nonce is recorded")
	return resp.StatusCode, string(got)
}

// POST /a2a/verify answers a request from an agent in hybrid mode signed with
// Ed25519 alone 401 with valid: false and the error and reasonCode the
// agent-signature routes that verify an Ed25519 signature alone answer it with.
// A signature that does not verify gets the signature refusal, with no
// reasonCode.
func TestA2AVerifyRefusesAnAgentInHybridModeSigningWithEd25519Alone(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	status, body := postSignedA2AVerify(t, pub, priv, false)
	assert.Equal(t, fiber.StatusUnauthorized, status, body)
	assert.JSONEq(t, `{"valid":false,"error":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`, body)

	status, body = postSignedA2AVerify(t, pub, priv, true)
	assert.Equal(t, fiber.StatusUnauthorized, status, body)
	assert.JSONEq(t, `{"valid":false,"error":"signature verification failed"}`, body,
		"a signature that does not verify gets the signature refusal")
}
