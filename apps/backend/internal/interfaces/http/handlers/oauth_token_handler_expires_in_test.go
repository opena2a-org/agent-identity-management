package handlers

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// expires_in in the token response must be the lifetime of the token it accompanies.
//
// The endpoint answered a fixed 3600 while GenerateServiceToken stamped the configured
// access-token lifetime, 7200 seconds by default. A client caching by expires_in then
// dropped a valid token at half its life, and under a JWT_ACCESS_TTL below an hour it kept
// presenting a token the server had already expired.
func TestOAuthTokenExpiresInIsTheIssuedTokensLifetime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		accessTTL string
		want      int
	}{
		{"default lifetime", "", 7200},
		{"configured lifetime", "45m", 2700},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_ACCESS_TTL", tc.accessTTL)
			agent, priv := newAgent(t, domain.AgentStatusVerified)

			status, body := postToken(t, agent, priv, validClaims(agent))
			require.Equal(t, fiber.StatusOK, status, "token request was refused: %s", body)

			var resp struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &resp))

			parts := strings.Split(resp.AccessToken, ".")
			require.Len(t, parts, 3)
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			require.NoError(t, err)
			var issued struct {
				Iat int64 `json:"iat"`
				Exp int64 `json:"exp"`
			}
			require.NoError(t, json.Unmarshal(payload, &issued))

			assert.Equal(t, tc.want, resp.ExpiresIn)
			assert.Equal(t, issued.Exp-issued.Iat, int64(resp.ExpiresIn),
				"expires_in must equal the issued token's exp - iat")
		})
	}
}
