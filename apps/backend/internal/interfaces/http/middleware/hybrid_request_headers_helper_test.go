package middleware

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// The TypeScript and Java SDKs each carry a createHybridRequestHeaders helper, and both
// document it as not accepted by AIM's request verification. This file measures that
// sentence: a request built the helper's way, signed with the agent's own registered
// keys and naming the agent, is refused by PQCAgentMiddleware, and the same headers over
// the message the middleware reconstructs are accepted.
//
// What the helpers do, written out here so the measurement does not depend on either SDK:
// they sign "timestamp:METHOD:path:hex(sha256(body))" (an empty last part without a
// body) with both keys, and return X-Algorithm, X-Signature-Ed25519, X-Signature-MLDSA
// and X-Timestamp, with X-Agent-ID left for the caller. The timestamp is the caller's, or
// the clock in milliseconds when the caller passes none.

// helperMessage is the message both helpers sign.
func helperMessage(ts, method, path, body string) []byte {
	bodyHash := ""
	if body != "" {
		sum := sha256.Sum256([]byte(body))
		bodyHash = hex.EncodeToString(sum[:])
	}
	return []byte(ts + ":" + strings.ToUpper(method) + ":" + path + ":" + bodyHash)
}

// verifiedMessage is the message PQCAgentMiddleware reconstructs and verifies.
func verifiedMessage(ts, method, path, body string) []byte {
	parts := []string{strings.ToUpper(method), path, ts}
	if body != "" {
		parts = append(parts, body)
	}
	return []byte(strings.Join(parts, "\n"))
}

// sendHybridHeaders sends method, path and body through PQCAgentMiddleware for an agent
// registered with keys, carrying the helpers' five headers with both signatures made by
// those keys over message. It returns the status code and the raw body.
func sendHybridHeaders(t *testing.T, keys agentKeys, method, path, body, ts string, message []byte) (int, string) {
	t.Helper()

	agent := registeredAgent(domain.AgentStatusVerified, keys)
	repo := &mocks.MockAgentRepository{}
	repo.On("GetByID", agent.ID).Return(agent, nil)
	repo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))
	svc := application.NewAgentService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// The server's own header buffer (cmd/server): an ML-DSA-65 signature does not fit in
	// fiber's 4 KB default.
	app := fiber.New(fiber.Config{ReadBufferSize: 16384})
	app.Use(PQCAgentMiddleware(svc))
	app.All(path, func(c fiber.Ctx) error {
		via, _ := c.Locals("authenticated_via").(string)
		return c.JSON(fiber.Map{"authenticatedVia": via})
	})

	pqcSig, err := pqc.SignMLDSA(pqc.AlgorithmMLDSA65, keys.pqcPriv, message)
	require.NoError(t, err)

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-Agent-ID", agent.ID.String())
	req.Header.Set("X-Algorithm", string(pqc.AlgorithmHybridEd25519MLDSA65))
	req.Header.Set("X-Signature-Ed25519", base64.StdEncoding.EncodeToString(ed25519.Sign(keys.edPriv, message)))
	req.Header.Set("X-Signature-MLDSA", base64.StdEncoding.EncodeToString(pqcSig))
	req.Header.Set("X-Timestamp", ts)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(answer)
}

func TestPQCAgentMiddleware_RefusesARequestSignedTheWayCreateHybridRequestHeadersSigns(t *testing.T) {
	keys := newAgentKeys(t)
	const path = "/api/v1/detection/report"
	const jsonBody = `{"name":"agent"}`

	seconds := strconv.FormatInt(time.Now().Unix(), 10)
	millis := strconv.FormatInt(time.Now().UnixMilli(), 10)

	const signatureRefused = `{"error":"invalid Ed25519 signature in hybrid mode","reasonCode":"signatureInvalid"}`
	const timestampRefused = `{"error":"Request timestamp expired or invalid"}`

	cells := []struct {
		name   string
		method string
		body   string
		ts     string
		want   string
	}{
		// The timestamp the helpers stamp when the caller passes none.
		{"clock in milliseconds, no body", "GET", "", millis, timestampRefused},
		{"clock in milliseconds, with a body", "POST", jsonBody, millis, timestampRefused},
		// The most favourable call: the caller passes the Unix seconds AIM reads.
		{"caller's Unix seconds, no body", "GET", "", seconds, signatureRefused},
		{"caller's Unix seconds, with a body", "POST", jsonBody, seconds, signatureRefused},
	}
	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			status, body := sendHybridHeaders(t, keys, cell.method, path, cell.body, cell.ts,
				helperMessage(cell.ts, cell.method, path, cell.body))
			require.Equal(t, fiber.StatusUnauthorized, status, "body: %s", body)
			require.Equal(t, cell.want, body)
		})
	}

	// Non-vacuity: the same agent, keys, headers and timestamp are accepted once the
	// signatures cover the message the middleware reconstructs, so what refuses the cells
	// above is what the helpers sign and stamp, not the headers they name.
	for _, cell := range []struct{ name, method, body string }{
		{"no body", "GET", ""},
		{"with a body", "POST", jsonBody},
	} {
		t.Run("the verified message is accepted, "+cell.name, func(t *testing.T) {
			status, body := sendHybridHeaders(t, keys, cell.method, path, cell.body, seconds,
				verifiedMessage(seconds, cell.method, path, cell.body))
			require.Equal(t, fiber.StatusOK, status, "body: %s", body)
			require.Equal(t, `{"authenticatedVia":"hybrid"}`, body)
		})
	}
}
