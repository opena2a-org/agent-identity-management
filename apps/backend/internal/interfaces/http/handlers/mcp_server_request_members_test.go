package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two MCP server routes an SDK can confuse: PUT /agents/{id}/mcp-servers
// adds identifiers to an agent's list and binds AddMCPServersRequest, and
// POST /mcp-servers (also /sdk-api/agents/{id}/mcp-servers) creates a server
// and binds CreateMCPServerRequest. A request meant for one route and sent to
// the other carries none of the members the handler reads.

// jsonMember is the JSON name a request type gives one of its fields.
func jsonMember(t *testing.T, value interface{}, field string) string {
	t.Helper()
	f, ok := reflect.TypeOf(value).FieldByName(field)
	require.True(t, ok, "%T has no field %s", value, field)
	name := strings.Split(f.Tag.Get("json"), ",")[0]
	require.NotEmpty(t, name, "%T.%s has no JSON name", value, field)
	return name
}

func decodeError(t *testing.T, body []byte) string {
	t.Helper()
	var answer map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &answer), "answer is not JSON: %s", body)
	message, _ := answer["error"].(string)
	return message
}

func TestAgentHandler_AddMCPServersToAgent_RefusalNamesTheMemberItReads(t *testing.T) {
	handler := &AgentHandler{}
	app := fiber.New()
	app.Put("/agents/:id/mcp-servers", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddMCPServersToAgent(c)
	})

	// The snake-case spelling the old SDK clients sent is not a member the
	// request type binds, so the list it carries is never read.
	body := `{"mcp_server_ids":["x"],"detected_method":"manual"}`
	req := httptest.NewRequest("PUT", "/agents/"+uuid.New().String()+"/mcp-servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	message := decodeError(t, raw)

	member := jsonMember(t, application.AddMCPServersRequest{}, "MCPServerIDs")
	assert.Equal(t, member+" is required and must not be empty", message)
	assert.NotContains(t, message, "mcp_server_ids")
}

func TestMCPHandler_CreateMCPServer_RefusesAServerWithoutNameOrURL(t *testing.T) {
	nameMember := jsonMember(t, application.CreateMCPServerRequest{}, "Name")
	urlMember := jsonMember(t, application.CreateMCPServerRequest{}, "URL")

	cases := []struct {
		name    string
		body    string
		missing string
	}{
		{
			// The request the Java and the generated Python clients sent here to
			// add a server to the agent's list. It used to store a server with an
			// empty name and URL, and the next one was answered 409.
			name:    "the members of the attach route",
			body:    `{"mcp_server_ids":["x"],"detected_method":"manual","confidence":100,"metadata":{}}`,
			missing: nameMember + " and " + urlMember + " are required",
		},
		{name: "no url", body: `{"name":"filesystem"}`, missing: urlMember + " is required"},
		{name: "no name", body: `{"url":"stdio://npx/filesystem"}`, missing: nameMember + " is required"},
		{name: "blank name and url", body: `{"name":"  ","url":"\t"}`, missing: nameMember + " and " + urlMember + " are required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No service is wired: a request that passes validation reaches it
			// and fails, which the recover middleware answers with a 500.
			handler := &MCPHandler{}
			app := fiber.New()
			app.Use(recover.New())
			app.Post("/mcp-servers", withMCPOrgContext(handler.CreateMCPServer))

			req := httptest.NewRequest("POST", "/mcp-servers", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "answer: %s", raw)
			assert.Equal(t, tc.missing, decodeError(t, raw))
		})
	}
}
