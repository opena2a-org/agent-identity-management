package handlers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The OAuth token endpoint, RFC 7523 jwt-bearer grant.
//
// Two defects lived here and both were invisible to a positive test, because a correctly
// formed request from a healthy agent succeeded before the fix and succeeds after it:
//
//  1. No agent-status check. AgentService.RevokeAgent expresses revocation purely as a
//     write to agents.status and does NOT clear agents.public_key, so a revoked agent
//     holding its private key kept minting service tokens here indefinitely while every
//     signature and API-key path denied it.
//  2. Only the `sub` claim was read. RFC 7523 §3 requires `aud` and `exp`; without them an
//     assertion never expired and was replayable forever.
//
// So every test below is a REJECTION test. A verifier is defined by what it refuses, and
// the happy-path case exists only as the control that proves the refusals are about the one
// property each names.

// signAssertion builds a real Ed25519-signed client assertion with the given claims.
func signAssertion(t *testing.T, priv ed25519.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	sig := ed25519.Sign(priv, []byte(signingInput))

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// postToken drives the real handler with a real signed assertion and returns status + body.
func postToken(t *testing.T, agent *domain.Agent, priv ed25519.PrivateKey, claims map[string]interface{}) (int, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	t.Setenv("AIM_BASE_URL", "https://aim.test")

	repo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) { return agent, nil },
	}
	h := NewOAuthTokenHandler(auth.NewJWTService(), repo)

	app := fiber.New()
	app.Post("/oauth/token", h.HandleTokenRequest)

	form := strings.NewReader(
		"grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" +
			"&client_id=" + agent.ID.String() +
			"&client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer" +
			"&client_assertion=" + signAssertion(t, priv, claims))

	req := httptest.NewRequest("POST", "/oauth/token", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// newAgent returns an agent in `status` holding a real Ed25519 keypair.
func newAgent(t *testing.T, status domain.AgentStatus) (*domain.Agent, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(pub)
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Status:         status,
		PublicKey:      &encoded,
	}, priv
}

// validClaims is a well-formed assertion for `agent`, which every rejection test then
// breaks in exactly one place.
func validClaims(agent *domain.Agent) map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"iss": agent.ID.String(),
		"sub": agent.ID.String(),
		"aud": "https://aim.test",
		"iat": float64(now.Unix()),
		"exp": float64(now.Add(2 * time.Minute).Unix()),
	}
}

// The control. Everything below breaks exactly one property of THIS request, so if this
// case ever stops succeeding the rejection tests stop proving anything about their own
// property.
func TestOAuthTokenIssuesForAHealthyAgent(t *testing.T) {
	agent, priv := newAgent(t, domain.AgentStatusVerified)

	status, body := postToken(t, agent, priv, validClaims(agent))

	require.Equal(t, fiber.StatusOK, status, "control request was refused: %s", body)
	assert.Contains(t, body, "access_token")
}

// Defect 1: a revoked or suspended agent must not obtain a token, even though its public
// key is still registered — RevokeAgent does not clear it.
func TestOAuthTokenRefusesAgentsWhoseStatusDeniesAuth(t *testing.T) {
	for _, tc := range []struct {
		status     domain.AgentStatus
		shouldMint bool
	}{
		{domain.AgentStatusVerified, true},
		{domain.AgentStatusPending, true},
		{domain.AgentStatusRevoked, false},
		{domain.AgentStatusSuspended, false},
		// agents.status carries no CHECK constraint, so a value outside the domain
		// constants is storable. An allow-list denies it; a deny-list would mint for it.
		{domain.AgentStatus("deactivated"), false},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			agent, priv := newAgent(t, tc.status)

			status, body := postToken(t, agent, priv, validClaims(agent))

			if tc.shouldMint {
				require.Equal(t, fiber.StatusOK, status, "body: %s", body)
				return
			}
			require.Equal(t, fiber.StatusUnauthorized, status,
				"a %s agent obtained a service token; revocation that does not stop token issuance "+
					"is not revocation", tc.status)
			assert.Contains(t, body, string(tc.status),
				"the denial should name the status so an operator can see why")
		})
	}
}

// Defect 2: the temporal and audience claims RFC 7523 §3 requires. Each case breaks exactly
// one claim of an otherwise valid assertion.
func TestOAuthTokenEnforcesAssertionClaims(t *testing.T) {
	now := time.Now()

	for _, tc := range []struct {
		name   string
		mutate func(map[string]interface{})
		why    string
	}{
		{"missing exp", func(c map[string]interface{}) { delete(c, "exp") },
			"an assertion with no expiry is replayable forever — the defect itself"},
		{"expired", func(c map[string]interface{}) { c["exp"] = float64(now.Add(-10 * time.Minute).Unix()) },
			"an expired assertion must not mint a token"},
		{"exp far in the future", func(c map[string]interface{}) { c["exp"] = float64(now.Add(72 * time.Hour).Unix()) },
			"an unbounded lifetime is close to no expiry at all"},
		{"missing aud", func(c map[string]interface{}) { delete(c, "aud") },
			"without aud, an assertion minted for another service is accepted here"},
		{"aud for another service", func(c map[string]interface{}) { c["aud"] = "https://somewhere.else" },
			"an assertion addressed elsewhere must not be accepted here"},
		{"not yet valid", func(c map[string]interface{}) { c["nbf"] = float64(now.Add(10 * time.Minute).Unix()) },
			"an assertion that is not yet valid must not mint a token"},
		{"iat in the future", func(c map[string]interface{}) { c["iat"] = float64(now.Add(10 * time.Minute).Unix()) },
			"a future iat is an unbounded lifetime wearing a different hat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, priv := newAgent(t, domain.AgentStatusVerified)
			claims := validClaims(agent)
			tc.mutate(claims)

			status, body := postToken(t, agent, priv, claims)

			require.Equal(t, fiber.StatusUnauthorized, status, tc.why+" (body: %s)", body)
			assert.NotContains(t, body, "access_token",
				"a token was issued for an assertion that should have been refused")
		})
	}
}

// Claims are only trusted once the signature proves them. An assertion whose claims are
// perfect but whose signature is another key's must still be refused — otherwise the claim
// checks are being applied to attacker-authored text.
func TestOAuthTokenRefusesAValidLookingAssertionSignedByTheWrongKey(t *testing.T) {
	agent, _ := newAgent(t, domain.AgentStatusVerified)
	_, attackerPriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	status, body := postToken(t, agent, attackerPriv, validClaims(agent))

	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.NotContains(t, body, "access_token")
}
