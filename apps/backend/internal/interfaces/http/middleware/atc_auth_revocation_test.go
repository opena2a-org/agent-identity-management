package middleware

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	infraatc "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/atc"
)

const revocationTestIssuer = "https://aim.test.opena2a.org"

// countingRevocation answers a read with revoked or err and counts reads.
type countingRevocation struct {
	revoked bool
	err     error
	reads   int
}

func (s *countingRevocation) AgentRevoked(context.Context, uuid.UUID) (bool, error) {
	s.reads++
	return s.revoked, s.err
}

// issueServerATC mints a CBOR ATC for agentID signed with the server's ATC
// issuer key, the format the server's verifier accepts.
func issueServerATC(t *testing.T, key ed25519.PrivateKey, agentID uuid.UUID) string {
	t.Helper()
	now := time.Now()
	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         agentID.String(),
		Issuer:          revocationTestIssuer,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(5 * time.Minute).Unix()),
		Capabilities:    []string{"secrets:resolve"},
		IssuerPublicKey: []byte(key.Public().(ed25519.PublicKey)),
	}
	encMode, err := cbor.CanonicalEncOptions().EncMode()
	require.NoError(t, err)
	signed, err := encMode.Marshal(atcdomain.SignedPayload{
		ATCID:           payload.ATCID,
		AgentID:         payload.AgentID,
		Issuer:          payload.Issuer,
		IssuedAt:        payload.IssuedAt,
		ExpiresAt:       payload.ExpiresAt,
		Capabilities:    payload.Capabilities,
		IssuerPublicKey: payload.IssuerPublicKey,
	})
	require.NoError(t, err)
	payload.Ed25519Sig = ed25519.Sign(key, signed)
	raw, err := encMode.Marshal(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// presentATC sends one request carrying an ATC for agentID through the
// middleware, with the server's verifier reading source, and reports the
// response, its JSON body and whether the route handler ran.
func presentATC(t *testing.T, source atcdomain.RevocationSource, agentID uuid.UUID) (status int, retryAfter string, body map[string]string, reached bool) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier, err := infraatc.NewServerATCVerifier(revocationTestIssuer, key, source)
	require.NoError(t, err)

	app := fiber.New()
	app.Use(ATCAuthMiddleware(verifier))
	app.Get("/resolve", func(c fiber.Ctx) error {
		reached = true
		return c.JSON(fiber.Map{"agent": c.Locals("agent_id").(uuid.UUID).String()})
	})

	req := httptest.NewRequest("GET", "/resolve", nil)
	req.Header.Set("Authorization", "ATC "+issueServerATC(t, key, agentID))
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, resp.Header.Get("Retry-After"), body, reached
}

// An ATC whose agent's revocation state cannot be read is answered 503 with
// Retry-After and atc_revocation_unknown, and the route handler does not run.
func TestATCAuthMiddleware_UnknownRevocationStateIs503WithRetryAfter(t *testing.T) {
	source := &countingRevocation{err: errors.New("store unavailable")}
	status, retryAfter, body, reached := presentATC(t, source, uuid.New())

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, strconv.Itoa(int(atcdomain.RevocationUnknownRetryAfter/time.Second)), retryAfter)
	assert.Equal(t, map[string]string{
		"error":   atcdomain.ErrCodeRevocationUnknown,
		"message": "the revocation state of this ATC could not be read",
	}, body, "the answer is the code and the fixed line, without the store's error")
	assert.False(t, reached, "the route handler ran for an ATC whose revocation state is unknown")
}

// An ATC whose agent is revoked is answered 401 atc_revoked, without
// Retry-After, and the route handler does not run.
func TestATCAuthMiddleware_RevokedAgentIs401(t *testing.T) {
	source := &countingRevocation{revoked: true}
	status, retryAfter, body, reached := presentATC(t, source, uuid.New())

	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, retryAfter)
	assert.Equal(t, atcdomain.ErrCodeRevoked, body["error"])
	assert.False(t, reached, "the route handler ran for the ATC of a revoked agent")
}

// A request whose ATC is admitted reads its agent's revocation state once.
func TestATCAuthMiddleware_ReadsRevocationOncePerRequest(t *testing.T) {
	source := &countingRevocation{}
	agentID := uuid.New()
	status, _, body, reached := presentATC(t, source, agentID)

	assert.Equal(t, fiber.StatusOK, status)
	assert.True(t, reached)
	assert.Equal(t, agentID.String(), body["agent"])
	assert.Equal(t, 1, source.reads, "the revocation state was read %d times for one request", source.reads)
}
