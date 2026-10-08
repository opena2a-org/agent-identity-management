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

// postTokenWithHost drives the real handler with AIM_BASE_URL set to `baseURL` (empty
// leaves it unset) and the request's Host header set to `host`, which a client chooses
// freely.
func postTokenWithHost(t *testing.T, baseURL, host string, agent *domain.Agent, priv ed25519.PrivateKey, aud string) (int, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	t.Setenv("AIM_BASE_URL", baseURL)

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

	req := httptest.NewRequest("POST", "/api/v1/oauth/token", form)
	req.Host = host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// The Host header names whatever host and port the client puts there. With AIM_BASE_URL
// set, the server knows its own address and does not consult Host, so an assertion
// addressed to a host and port the client names in Host, for example one an agent signed
// for another service, is refused. With AIM_BASE_URL unset, Host is the only address the
// server has, and the origin it names is accepted, port included.
func TestOAuthTokenConsultsTheHostHeaderOnlyWhenBaseURLIsUnset(t *testing.T) {
	const baseURL = "https://aim.example.com"
	for _, tc := range []struct {
		name    string
		baseURL string
		host    string
		aud     string
		accept  bool
	}{
		{"base URL set, Host names another server with a port", baseURL, "evil.example:9443", "http://evil.example:9443", false},
		{"base URL set, Host names another server with a port, API prefix", baseURL, "evil.example:9443", "http://evil.example:9443/api/v1", false},
		{"base URL set, Host names another server without a port", baseURL, "evil.example", "http://evil.example", false},
		{"base URL set, aud is the base URL", baseURL, "evil.example:9443", baseURL, true},
		{"base URL set, aud is the base URL with the API prefix", baseURL, "evil.example:9443", baseURL + "/api/v1", true},
		{"base URL unset, aud is the origin Host names, with its port", "", "evil.example:9443", "http://evil.example:9443", true},
		{"base URL unset, aud is the origin Host names, without a port", "", "evil.example", "http://evil.example", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, priv := newAgent(t, domain.AgentStatusVerified)

			status, body := postTokenWithHost(t, tc.baseURL, tc.host, agent, priv, tc.aud)

			if tc.accept {
				require.Equal(t, fiber.StatusOK, status, "the assertion was refused (body: %s)", body)
				assert.Contains(t, body, "access_token")
				return
			}
			require.Equal(t, fiber.StatusUnauthorized, status,
				"an assertion addressed to the origin a client named in Host was accepted with AIM_BASE_URL set (body: %s)", body)
			assert.NotContains(t, body, "access_token")
		})
	}
}
