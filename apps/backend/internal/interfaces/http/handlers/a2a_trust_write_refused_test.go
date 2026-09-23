package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PUT /a2a/trust/:id used to bind a body, recompute, and answer 200 while
// discarding what it was sent. It now refuses with 405 and says why, before
// any lookup: an asserted score is never applied, so nothing is loaded.
func TestRefuseTrustScoreWrite_405WithReason(t *testing.T) {
	handler := &A2AHandler{}
	app := fiber.New()
	app.Put("/a2a/trust/:id", handler.RefuseTrustScoreWrite)

	req := httptest.NewRequest("PUT", "/a2a/trust/"+uuid.New().String(),
		strings.NewReader(`{"score":0.99,"confidence":1,"reason":"trust me"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusMethodNotAllowed, resp.StatusCode)
	assert.Equal(t, "GET", resp.Header.Get("Allow"))
	var body map[string]string
	raw, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, A2ATrustScoreNotAsserted, body["error"])
	assert.Contains(t, body["record"], "/interaction")
	assert.NotContains(t, string(raw), "0.99", "the asserted score is never echoed as if applied")
}
