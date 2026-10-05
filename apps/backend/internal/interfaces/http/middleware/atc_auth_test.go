package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	infraatc "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/atc"
)

// A request presenting a JWT-format token as its ATC is refused by the
// verifier the server builds, and never reaches the route handler.
func TestATCAuthMiddleware_RefusesJWTFormatToken(t *testing.T) {
	_, serverKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier, err := infraatc.NewServerATCVerifier("https://aim.test.opena2a.org", serverKey, nil)
	require.NoError(t, err)

	agentID := uuid.New()
	now := time.Now()
	atcID := uuid.New().String()
	token, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss":          "aim-server",
		"sub":          agentID.String(),
		"iat":          now.Unix(),
		"exp":          now.Add(5 * time.Minute).Unix(),
		"jti":          atcID,
		"agent_id":     agentID.String(),
		"capabilities": []string{"secrets:resolve"},
		"atc_id":       atcID,
	}).SignedString(serverKey)
	require.NoError(t, err)

	reached := false
	app := fiber.New()
	app.Use(ATCAuthMiddleware(verifier))
	app.Get("/resolve", func(c fiber.Ctx) error {
		reached = true
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/resolve", nil)
	req.Header.Set("Authorization", "ATC "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	assert.False(t, reached, "the route handler ran for a JWT-format token")
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, atcdomain.ErrCodeMalformed, body["error"])
}
