package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// AIM-08: the POST /agents/:id/authorize response serialises the Step 3
// context-check result under `contextCheck`, beside `intentCheck`. The
// handler passes FGAResult through untouched, so these tests pin the wire
// shape API consumers will key on (dashboards, SDKs) rather than re-testing
// engine behavior.

func aim08PostAuthorize(t *testing.T, result *application.FGAResult) map[string]json.RawMessage {
	t.Helper()
	agentID := uuid.New()
	orgID := uuid.New()
	fake := &fakeAuthorizer{result: result}
	h := NewAuthorizeHandlerWithInterface(fake, &permissiveAuthorizeAgentRepo{orgID: orgID})
	app := newAuthorizeTestApp(h, orgID)

	body := strings.NewReader(`{"capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var raw map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	return raw
}

func TestAuthorize_ContextCheckSerialisation(t *testing.T) {
	t.Run("AIM-08.AC2 an unavailable-path deny serialises contextCheck with status onUnavailable blocked", func(t *testing.T) {
		raw := aim08PostAuthorize(t, &application.FGAResult{
			Allowed:        false,
			Outcome:        "DENY_CONTEXT",
			DeniedBy:       "context_check",
			DeniedReason:   "ASC summary unavailable",
			StepsTriggered: []string{"capability_check", "attribute_check", "context_check"},
			ContextCheck: &application.ContextCheckResult{
				Status:        "unavailable",
				OnUnavailable: "deny",
				Blocked:       true,
			},
		})

		field, ok := raw["contextCheck"]
		require.True(t, ok, "the response must carry the context-check result under the contextCheck key")

		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(field, &got))
		assert.Equal(t, "unavailable", got["status"])
		assert.Equal(t, "deny", got["onUnavailable"])
		assert.Equal(t, true, got["blocked"])

		assert.JSONEq(t, `"DENY_CONTEXT"`, string(raw["outcome"]))
		assert.JSONEq(t, `"context_check"`, string(raw["deniedBy"]))
		assert.JSONEq(t, `"ASC summary unavailable"`, string(raw["deniedReason"]))
	})

	t.Run("AIM-08.AC2 contextCheck is omitted when the engine did not evaluate context rules", func(t *testing.T) {
		raw := aim08PostAuthorize(t, &application.FGAResult{
			Allowed:        true,
			Outcome:        "ALLOW",
			StepsTriggered: []string{"capability_check"},
		})
		_, ok := raw["contextCheck"]
		assert.False(t, ok, "omitempty: a nil ContextCheck must not appear on the wire")
	})
}
