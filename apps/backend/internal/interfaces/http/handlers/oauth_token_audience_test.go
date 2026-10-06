package handlers

import (
	"crypto/ed25519"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// With AIM_BASE_URL unset, the token endpoint accepts an assertion whose aud is the origin
// the request arrived on. An origin is scheme, host AND port: the TypeScript SDK sets aud
// to the base URL it was given, so an agent talking to http://localhost:8080 signs
// aud=http://localhost:8080. The check used to build the origin from the host name alone,
// which refused the SDK's own assertion on any server listening on a port other than the
// scheme's default, and accepted http://localhost, the origin of another service.

// postTokenWithoutBaseURL drives the real handler with AIM_BASE_URL unset. The request goes
// to `target`, so its Host header carries that URL's host and port.
func postTokenWithoutBaseURL(t *testing.T, target string, agent *domain.Agent, priv ed25519.PrivateKey, aud string) (int, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	t.Setenv("AIM_BASE_URL", "")

	repo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) { return agent, nil },
	}
	h := NewOAuthTokenHandler(auth.NewJWTService(), repo)

	app := fiber.New()
	app.Post("/api/v1/oauth/token", h.HandleTokenRequest)

	claims := validClaims(agent)
	claims["aud"] = aud
	form := strings.NewReader(
		"grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" +
			"&client_id=" + agent.ID.String() +
			"&client_assertion=" + signAssertion(t, priv, claims))

	req := httptest.NewRequest("POST", target, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func TestOAuthTokenAcceptsTheRequestOriginAsAudienceIncludingItsPort(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		aud    string
	}{
		{"origin with a port", "http://localhost:8080/api/v1/oauth/token", "http://localhost:8080"},
		{"origin with a port and the API prefix", "http://localhost:8080/api/v1/oauth/token", "http://localhost:8080/api/v1"},
		{"origin with a port and a trailing slash", "http://localhost:8080/api/v1/oauth/token", "http://localhost:8080/"},
		{"origin on the scheme's default port", "http://aim.internal/api/v1/oauth/token", "http://aim.internal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, priv := newAgent(t, domain.AgentStatusVerified)

			status, body := postTokenWithoutBaseURL(t, tc.target, agent, priv, tc.aud)

			require.Equal(t, fiber.StatusOK, status,
				"an assertion addressed to the origin the request arrived on was refused (body: %s)", body)
			assert.Contains(t, body, "access_token")
		})
	}
}

func TestOAuthTokenRefusesAnAudienceOnTheSameHostButAnotherPort(t *testing.T) {
	for _, tc := range []struct {
		name string
		aud  string
	}{
		{"no port, which is port 80", "http://localhost"},
		{"another port", "http://localhost:9090"},
		{"another scheme", "https://localhost:8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, priv := newAgent(t, domain.AgentStatusVerified)

			status, body := postTokenWithoutBaseURL(t, "http://localhost:8080/api/v1/oauth/token", agent, priv, tc.aud)

			require.Equal(t, fiber.StatusUnauthorized, status,
				"an assertion addressed to another origin on the same host was accepted (body: %s)", body)
			assert.NotContains(t, body, "access_token")
		})
	}
}
