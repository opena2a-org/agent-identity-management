package handlers

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// fleetGovernanceTokenStep returns the fleet governance guide's step about the
// token endpoint: from its "## Step 4" heading to the next second-level heading.
func fleetGovernanceTokenStep(t *testing.T, guide string) string {
	t.Helper()
	start := strings.Index(guide, "\n## Step 4")
	require.NotEqual(t, -1, start, "the fleet governance guide has no Step 4")
	rest := guide[start+1:]
	if end := strings.Index(rest, "\n## "); end != -1 {
		return rest[:end]
	}
	return rest
}

// documentedTokenResponse returns the JSON example in `step` that carries an
// access token, decoded.
func documentedTokenResponse(t *testing.T, step string) map[string]interface{} {
	t.Helper()
	for _, block := range regexp.MustCompile("(?s)```json\n(.*?)```").FindAllStringSubmatch(step, -1) {
		if !strings.Contains(block[1], "access_token") {
			continue
		}
		var example map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(block[1]), &example),
			"the documented token response must be valid JSON")
		return example
	}
	t.Fatal("the token step shows no JSON response carrying access_token")
	return nil
}

// textHas and textLacks report the needle alone on failure. assert.Contains
// prints the whole haystack, which here is a guide or a source file.
func textHas(t *testing.T, text, needle, what string) {
	t.Helper()
	assert.True(t, strings.Contains(text, needle), "%s: %q not found", what, needle)
}

func textLacks(t *testing.T, text, needle, what string) {
	t.Helper()
	assert.False(t, strings.Contains(text, needle), "%s: %q still present", what, needle)
}

// TestFleetGovernanceGuideDescribesTheTokenTheServerIssues pins what the fleet
// governance guide says about the token endpoint to a token the handler has
// just issued.
//
// The guide told readers to request a token at /api/v1/token, showed a response
// whose token header read EdDSA, and told other services to verify the token
// against a JWK Set at /.well-known/jwks.json. The server mounts the endpoint
// at /api/v1/oauth/token, signs the token with HS256 under JWT_SECRET, and
// publishes no key for it, so the documented request named a path the server
// does not mount and no published key verified the token the guide described.
// The JWK Set the server serves holds its Ed25519 card-attestation and
// ATC-issuer keys only.
//
// The guide is compared with a real response rather than with constants, so a
// change to the signing method or to the response fields fails here until the
// guide is rewritten to match.
func TestFleetGovernanceGuideDescribesTheTokenTheServerIssues(t *testing.T) {
	// Unset, so the lifetime is the server default the guide quotes.
	t.Setenv("JWT_ACCESS_TTL", "")

	agent, priv := newAgent(t, domain.AgentStatusVerified)
	status, body := postToken(t, agent, priv, validClaims(agent))
	require.Equal(t, fiber.StatusOK, status, "the handler refused a well-formed request: %s", body)

	var issued map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &issued))
	accessToken, _ := issued["access_token"].(string)
	segments := strings.Split(accessToken, ".")
	require.Len(t, segments, 3, "the access token is a compact JWT")

	rawHeader, err := base64.RawURLEncoding.DecodeString(segments[0])
	require.NoError(t, err)
	var header struct {
		Alg string `json:"alg"`
	}
	require.NoError(t, json.Unmarshal(rawHeader, &header))
	require.Equal(t, "HS256", header.Alg,
		"the guide's verification paragraph is written for a symmetric token; "+
			"if the signing method changes, rewrite that paragraph and this test together")

	// The route the guide has to name, and the absence of the one it used to.
	routes := readBackendFile(t, "cmd/server/main.go")
	textHas(t, routes, `oauth := v1.Group("/oauth")`, "the token endpoint sits under /api/v1/oauth")
	textHas(t, routes, `oauth.Post("/token", h.OAuthToken.HandleTokenRequest)`,
		"the token endpoint is POST /api/v1/oauth/token")
	// The JWK Set the server mounts publishes its Ed25519 signing keys, none of
	// which verifies the HS256 access token.
	for _, key := range newJWKSTestRing(t).JWKS().Keys {
		assert.NotEqual(t, header.Alg, key.Alg, "the JWK Set publishes no key for the access token (%s)", key.Purpose)
		assert.Equal(t, "OKP", key.Kty, "the JWK Set publishes no symmetric key (%s)", key.Purpose)
	}
	textHas(t, routes, `agents.Use(middleware.ServicePrincipalMiddleware(`,
		"the /api/v1/agents routes are where the server accepts the token")

	guide := readBackendFile(t, "../../docs/use-cases/fleet-governance.md")
	step := fleetGovernanceTokenStep(t, guide)

	// Verification: a symmetric token has no published verification key.
	textLacks(t, strings.ToLower(guide), "jwks",
		"the guide must not point readers at a JWK Set the server does not publish")
	textHas(t, step, header.Alg, "the guide names the algorithm the access token is signed with")
	textHas(t, step, "JWT_SECRET", "the guide names the key that signs and verifies the token")
	textHas(t, step, "no JWK Set", "the guide says that no published key verifies the token")
	textHas(t, step, "/api/v1/agents", "the guide names the routes on which the server verifies the token")

	// Issuance: the path, the grant and the fields the handler reads.
	textHas(t, step, "/api/v1/oauth/token", "the guide names the path the server mounts")
	textLacks(t, guide, "/api/v1/token", "the guide must not name a path the server does not mount")
	for _, field := range []string{
		"grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id=",
		"client_assertion=",
	} {
		textHas(t, step, field, "the documented request carries what the handler requires")
	}
	textLacks(t, step, `"agentId"`, "the handler reads no agentId field")
	textLacks(t, step, "OIDC", "the endpoint is an OAuth 2.0 grant and reads no identity provider setting")
	// postToken sets AIM_BASE_URL and validClaims uses the same value as aud,
	// so the request above is the audience rule the guide states.
	textHas(t, step, "AIM_BASE_URL", "the guide names the setting the assertion's aud must equal")

	// The response: the same fields, and a sample token that starts as a real one does.
	example := documentedTokenResponse(t, step)
	assert.Equal(t, sortedKeys(issued), sortedKeys(example),
		"the documented response carries exactly the fields the handler returns")
	assert.Equal(t, issued["token_type"], example["token_type"])
	assert.Equal(t, issued["expires_in"], example["expires_in"],
		"the documented lifetime is the server default")
	sample, _ := example["access_token"].(string)
	sample = strings.TrimSuffix(sample, "...")
	require.NotEmpty(t, sample, "the documented response shows the start of a token")
	assert.True(t, strings.HasPrefix(accessToken, sample),
		"the documented token must start as an issued one does, with the %s header", header.Alg)
}
