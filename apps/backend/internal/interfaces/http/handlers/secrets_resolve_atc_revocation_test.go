package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// refusingATCVerifier answers Verify with err.
type refusingATCVerifier struct{ err error }

func (v refusingATCVerifier) Verify(string) (*atcdomain.ATCClaims, error) { return nil, v.err }

// resolveWithBodyATC posts a correctly signed resolve request that carries an
// ATC in its body, to a service whose ATC verifier answers verifyErr.
func resolveWithBodyATC(t *testing.T, verifyErr error) (status int, retryAfter string, body map[string]string, audit *secretsTestAuditRepo) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	agentID := uuid.New()
	registeredB64 := base64.StdEncoding.EncodeToString(pub)
	agentRepo := &secretsTestAgentRepo{
		getByID: func(uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: uuid.New(), PublicKey: &registeredB64}, nil
		},
	}
	audit = &secretsTestAuditRepo{}
	svc := application.NewSecretsService(nil, audit, agentRepo, nil, refusingATCVerifier{err: verifyErr}, nil).
		WithNonceAdmission(newMemoryAdmitter())
	handler := NewSecretsHandler(svc, agentRepo, nil, nil)

	app := fiber.New()
	app.Post("/secrets/resolve", func(c fiber.Ctx) error {
		c.Locals("agent_id", agentID)
		return c.Next()
	}, handler.ResolveCredential)

	nonce := time.Now().UTC().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	payload, err := json.Marshal(map[string]string{
		"namespace":      "github",
		"operation":      "read",
		"nonce":          nonce,
		"signature":      base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("github|read|"+nonce))),
		"agentPublicKey": registeredB64,
		"atcId":          "an-atc-the-verifier-refuses",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/secrets/resolve", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &body))
	return resp.StatusCode, resp.Header.Get("Retry-After"), body, audit
}

// A body ATC whose revocation state cannot be read is answered 503 with
// Retry-After and atc_revocation_unknown, and the secrets audit entry is a
// denial that names the code.
func TestSecretsHandler_Resolve_ATCRevocationUnknownIs503(t *testing.T) {
	status, retryAfter, body, audit := resolveWithBodyATC(t, &atcdomain.ATCError{
		Code:    atcdomain.ErrCodeRevocationUnknown,
		Message: "the revocation state of this ATC could not be read",
	})

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, strconv.Itoa(int(atcdomain.RevocationUnknownRetryAfter/time.Second)), retryAfter)
	assert.Equal(t, atcdomain.ErrCodeRevocationUnknown, body["error"])
	assert.Equal(t, "the revocation state of this ATC could not be read", body["message"])
	require.Len(t, audit.entries, 1)
	assert.Equal(t, secrets.AuditResultDenied, audit.entries[0].Result)
	assert.Contains(t, audit.entries[0].DenyReason, atcdomain.ErrCodeRevocationUnknown)
}

// The 503 body is the stated line, not the verifier's error text: an error
// whose message carries other words is answered with the same body.
func TestSecretsHandler_Resolve_ATCRevocationUnknownAnswersItsStatedLine(t *testing.T) {
	planted := "planted-" + uuid.NewString()
	status, _, body, _ := resolveWithBodyATC(t, &atcdomain.ATCError{
		Code:    atcdomain.ErrCodeRevocationUnknown,
		Message: "pq: relation " + planted + " does not exist",
	})

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, map[string]string{
		"error":   atcdomain.ErrCodeRevocationUnknown,
		"message": atcdomain.RevocationUnknownMessage,
	}, body)
	for _, value := range body {
		assert.NotContains(t, value, planted)
	}
}

// A revoked body ATC stays a refusal without Retry-After.
func TestSecretsHandler_Resolve_RevokedBodyATCIs403(t *testing.T) {
	status, retryAfter, body, audit := resolveWithBodyATC(t, &atcdomain.ATCError{
		Code:    atcdomain.ErrCodeRevoked,
		Message: "ATC has been revoked",
	})

	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Empty(t, retryAfter)
	assert.Equal(t, "Secret resolution refused", body["error"])
	require.Len(t, audit.entries, 1)
	assert.Contains(t, audit.entries[0].DenyReason, atcdomain.ErrCodeRevoked)
}
