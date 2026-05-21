package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Helper Function Tests
// ===========================

func TestContains_ExactMatch(t *testing.T) {
	assert.True(t, contains("hello", "hello"))
}

func TestContains_Prefix(t *testing.T) {
	assert.True(t, contains("hello world", "hello"))
}

func TestContains_Suffix(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
}

func TestContains_Middle(t *testing.T) {
	assert.True(t, contains("hello world test", "world"))
}

func TestContains_NotFound(t *testing.T) {
	assert.False(t, contains("hello", "world"))
}

func TestContains_LongerSubstring(t *testing.T) {
	assert.False(t, contains("hi", "hello"))
}

func TestContains_Empty(t *testing.T) {
	assert.True(t, contains("hello", ""))
}

func TestContainsMiddle_Found(t *testing.T) {
	assert.True(t, containsMiddle("hello world test", "world"))
}

func TestContainsMiddle_AtStart(t *testing.T) {
	assert.True(t, containsMiddle("hello world", "hello"))
}

func TestContainsMiddle_AtEnd(t *testing.T) {
	assert.True(t, containsMiddle("hello world", "world"))
}

func TestContainsMiddle_NotFound(t *testing.T) {
	assert.False(t, containsMiddle("hello", "world"))
}

// ===========================
// NewTagHandler Tests
// ===========================

func TestNewTagHandler_NilService(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.tagService)
}

// ===========================
// Request/Response Struct Tests
// ===========================

func TestCreateTagRequest_Struct(t *testing.T) {
	req := CreateTagRequest{
		Key:         "env",
		Value:       "production",
		Category:    "environment",
		Description: "Production environment",
		Color:       "#FF5733",
	}

	assert.Equal(t, "env", req.Key)
	assert.Equal(t, "production", req.Value)
	assert.Equal(t, "environment", req.Category)
	assert.Equal(t, "Production environment", req.Description)
	assert.Equal(t, "#FF5733", req.Color)
}

func TestAddTagsRequest_Struct(t *testing.T) {
	id1 := uuid.New().String()
	id2 := uuid.New().String()

	req := AddTagsRequest{
		TagIDs: []string{id1, id2},
	}

	assert.Len(t, req.TagIDs, 2)
	assert.Equal(t, id1, req.TagIDs[0])
	assert.Equal(t, id2, req.TagIDs[1])
}

func TestUpdateTagRequest_Struct(t *testing.T) {
	req := UpdateTagRequest{
		Key:         "updated-key",
		Value:       "updated-value",
		Category:    "new-category",
		Description: "Updated description",
		Color:       "#00FF00",
	}

	assert.Equal(t, "updated-key", req.Key)
	assert.Equal(t, "updated-value", req.Value)
	assert.Equal(t, "new-category", req.Category)
	assert.Equal(t, "Updated description", req.Description)
	assert.Equal(t, "#00FF00", req.Color)
}

// ===========================
// Handler Context Tests
// ===========================

func TestTagHandler_CreateTag_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Post("/tags", handler.CreateTag)

	body := `{"key":"env","value":"prod","category":"environment"}`
	req := httptest.NewRequest("POST", "/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "Unauthorized", result["error"])
}

func TestTagHandler_CreateTag_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Post("/tags", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return handler.CreateTag(c)
	})

	body := `{"key":"env","value":"prod","category":"environment"}`
	req := httptest.NewRequest("POST", "/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "Organization ID not found", result["error"])
}

func TestTagHandler_CreateTag_InvalidJSON(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Post("/tags", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return handler.CreateTag(c)
	})

	req := httptest.NewRequest("POST", "/tags", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetTags_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/tags", handler.GetTags)

	req := httptest.NewRequest("GET", "/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_UpdateTag_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Put("/tags/:id", handler.UpdateTag)

	body := `{"key":"updated","value":"val"}`
	req := httptest.NewRequest("PUT", "/tags/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "Invalid tag ID", result["error"])
}

func TestTagHandler_UpdateTag_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Put("/tags/:id", handler.UpdateTag)

	body := `{"key":"updated","value":"val"}`
	req := httptest.NewRequest("PUT", "/tags/"+tagID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_DeleteTag_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Delete("/tags/:id", handler.DeleteTag)

	req := httptest.NewRequest("DELETE", "/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	agentID := uuid.New()

	app := fiber.New()
	// Set org but NOT user — verifies the user-context guard returns
	// 401 after the A3d-i org-scoping check passes.
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.AddTagsToAgent(c)
	})

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToAgent(c)
	})

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/agents/not-a-uuid/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_InvalidTagIDFormat(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToAgent(c)
	})

	body := `{"tagIds":["not-a-uuid"]}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "Invalid tag ID format", result["error"])
}

func TestTagHandler_RemoveTagFromAgent_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Delete("/agents/:id/tags/:tagId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.RemoveTagFromAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid/tags/"+tagID.String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_RemoveTagFromAgent_InvalidTagID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Delete("/agents/:id/tags/:tagId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.RemoveTagFromAgent(c)
	})

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetAgentTags_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/agents/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetAgentTags(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_SuggestTagsForAgent_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/agents/:id/tags/suggestions", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.SuggestTagsForAgent(c)
	})

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/tags/suggestions", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	// Set org but NOT user — verifies the user-context guard returns
	// 401 after the A3d-ii org-scoping check passes.
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.AddTagsToMCPServer(c)
	})

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+mcpServerID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToMCPServer(c)
	})

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/mcp-servers/not-a-uuid/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_RemoveTagFromMCPServer_InvalidMCPServerID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Delete("/mcp-servers/:id/tags/:tagId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.RemoveTagFromMCPServer(c)
	})

	req := httptest.NewRequest("DELETE", "/mcp-servers/not-a-uuid/tags/"+tagID.String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_RemoveTagFromMCPServer_InvalidTagID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Delete("/mcp-servers/:id/tags/:tagId", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.RemoveTagFromMCPServer(c)
	})

	req := httptest.NewRequest("DELETE", "/mcp-servers/"+mcpServerID.String()+"/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetMCPServerTags_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.GetMCPServerTags(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_SuggestTagsForMCPServer_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/tags/suggestions", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.SuggestTagsForMCPServer(c)
	})

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/tags/suggestions", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetPopularTags_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/tags/popular", handler.GetPopularTags)

	req := httptest.NewRequest("GET", "/tags/popular", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_SearchTags_MissingQuery(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/tags/search", handler.SearchTags)

	req := httptest.NewRequest("GET", "/tags/search", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "query parameter 'q' is required")
}

func TestTagHandler_SearchTags_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)

	app := fiber.New()
	app.Get("/tags/search", handler.SearchTags)

	req := httptest.NewRequest("GET", "/tags/search?q=test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// Additional Handler Context Tests
// ===========================

func TestTagHandler_UpdateTag_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Put("/tags/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		// No organization_id set
		return handler.UpdateTag(c)
	})

	body := `{"key":"updated","value":"val"}`
	req := httptest.NewRequest("PUT", "/tags/"+tagID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_UpdateTag_InvalidJSON(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Put("/tags/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return handler.UpdateTag(c)
	})

	req := httptest.NewRequest("PUT", "/tags/"+tagID.String(), strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_InvalidJSON(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToAgent(c)
	})

	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/tags", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_InvalidJSON(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToMCPServer(c)
	})

	req := httptest.NewRequest("POST", "/mcp-servers/"+mcpServerID.String()+"/tags", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_InvalidTagIDFormat(t *testing.T) {
	handler := NewTagHandler(nil, nil, nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return handler.AddTagsToMCPServer(c)
	})

	body := `{"tagIds":["not-a-uuid"]}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+mcpServerID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "Invalid tag ID format", result["error"])
}

// Note: Some tag handler methods don't have authorization checks at the handler level
// and rely on middleware for auth. Tests for these would require mocked services
// to avoid nil pointer dereferences when auth passes but service is nil.

// ===========================
// A3d-i: cross-tenant access on agent-scoped tag handlers must return
// 404 with existence-secrecy body. Each row exercises the LoadOwned
// guard wired in this PR. The mock GetAgent returns an agent in a
// different org; every handler must short-circuit at LoadOwned and
// never reach the tagService (which is nil here — a service dispatch
// would panic, proving the LoadOwned gate fired). Captured-flag
// pattern matches PR #150's panic-proof tests.
// ===========================

func TestTagHandler_AgentScoped_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	agentID := uuid.New()
	differentOrgID := uuid.New()

	agentRepo := &MockAgentRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: differentOrgID, Name: "victim"}, nil
		},
	}
	// tagService is intentionally nil: the LoadOwned gate must short-
	// circuit BEFORE any tagService dispatch, so reaching the nil
	// service would panic (which the harness records as a test failure
	// independent of the assertion below).
	handler := &TagHandler{tagService: nil, agentRepo: agentRepo}

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *TagHandler)
	}{
		{
			name:        "AddTagsToAgent",
			method:      "POST",
			requestPath: "/agents/" + agentID.String() + "/tags",
			body:        `{"tagIds":["` + uuid.New().String() + `"]}`,
			setup: func(app *fiber.App, h *TagHandler) {
				app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", callerUserID)
					return h.AddTagsToAgent(c)
				})
			},
		},
		{
			name:        "RemoveTagFromAgent",
			method:      "DELETE",
			requestPath: "/agents/" + agentID.String() + "/tags/" + uuid.New().String(),
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Delete("/agents/:id/tags/:tagId", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.RemoveTagFromAgent(c)
				})
			},
		},
		{
			name:        "GetAgentTags",
			method:      "GET",
			requestPath: "/agents/" + agentID.String() + "/tags",
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Get("/agents/:id/tags", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetAgentTags(c)
				})
			},
		},
		{
			name:        "SuggestTagsForAgent",
			method:      "GET",
			requestPath: "/agents/" + agentID.String() + "/tags/suggestions",
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Get("/agents/:id/tags/suggestions", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.SuggestTagsForAgent(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}

// ===========================
// A3d-ii: cross-tenant access on MCP-scoped tag handlers must return
// 404 with existence-secrecy body. Each row exercises the LoadOwned
// guard wired in this PR. The mock GetByID returns an MCP server in a
// different org; every handler must short-circuit at LoadOwned and
// never reach the tagService (which is nil here — a service dispatch
// would panic, proving the LoadOwned gate fired). Same captured-flag-
// via-nil-service pattern as the agent-scoped test above.
// ===========================

func TestTagHandler_MCPScoped_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	mcpServerID := uuid.New()
	differentOrgID := uuid.New()

	mcpServerRepo := &MockMCPServerRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: mcpServerID, OrganizationID: differentOrgID, Name: "victim-mcp"}, nil
		},
	}
	// tagService is intentionally nil: the LoadOwned gate must short-
	// circuit BEFORE any tagService dispatch, so reaching the nil
	// service would panic (which the harness records as a test failure
	// independent of the assertion below).
	handler := &TagHandler{tagService: nil, agentRepo: nil, mcpServerRepo: mcpServerRepo}

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *TagHandler)
	}{
		{
			name:        "AddTagsToMCPServer",
			method:      "POST",
			requestPath: "/mcp-servers/" + mcpServerID.String() + "/tags",
			body:        `{"tagIds":["` + uuid.New().String() + `"]}`,
			setup: func(app *fiber.App, h *TagHandler) {
				app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", callerUserID)
					return h.AddTagsToMCPServer(c)
				})
			},
		},
		{
			name:        "RemoveTagFromMCPServer",
			method:      "DELETE",
			requestPath: "/mcp-servers/" + mcpServerID.String() + "/tags/" + uuid.New().String(),
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Delete("/mcp-servers/:id/tags/:tagId", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.RemoveTagFromMCPServer(c)
				})
			},
		},
		{
			name:        "GetMCPServerTags",
			method:      "GET",
			requestPath: "/mcp-servers/" + mcpServerID.String() + "/tags",
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Get("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.GetMCPServerTags(c)
				})
			},
		},
		{
			name:        "SuggestTagsForMCPServer",
			method:      "GET",
			requestPath: "/mcp-servers/" + mcpServerID.String() + "/tags/suggestions",
			body:        "",
			setup: func(app *fiber.App, h *TagHandler) {
				app.Get("/mcp-servers/:id/tags/suggestions", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					return h.SuggestTagsForMCPServer(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}
