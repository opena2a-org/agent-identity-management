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
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/tags", handler.GetTags)

	req := httptest.NewRequest("GET", "/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_UpdateTag_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)
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
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Delete("/tags/:id", handler.DeleteTag)

	req := httptest.NewRequest("DELETE", "/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Post("/agents/:id/tags", handler.AddTagsToAgent)

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_AddTagsToAgent_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
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
	handler := NewTagHandler(nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
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
	handler := NewTagHandler(nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Delete("/agents/:id/tags/:tagId", handler.RemoveTagFromAgent)

	req := httptest.NewRequest("DELETE", "/agents/not-a-uuid/tags/"+tagID.String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_RemoveTagFromAgent_InvalidTagID(t *testing.T) {
	handler := NewTagHandler(nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Delete("/agents/:id/tags/:tagId", handler.RemoveTagFromAgent)

	req := httptest.NewRequest("DELETE", "/agents/"+agentID.String()+"/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetAgentTags_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/agents/:id/tags", handler.GetAgentTags)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_SuggestTagsForAgent_InvalidAgentID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/agents/:id/tags/suggestions", handler.SuggestTagsForAgent)

	req := httptest.NewRequest("GET", "/agents/not-a-uuid/tags/suggestions", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_NoUserContext(t *testing.T) {
	handler := NewTagHandler(nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", handler.AddTagsToMCPServer)

	body := `{"tagIds":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest("POST", "/mcp-servers/"+mcpServerID.String()+"/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_AddTagsToMCPServer_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
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
	handler := NewTagHandler(nil)
	tagID := uuid.New()

	app := fiber.New()
	app.Delete("/mcp-servers/:id/tags/:tagId", handler.RemoveTagFromMCPServer)

	req := httptest.NewRequest("DELETE", "/mcp-servers/not-a-uuid/tags/"+tagID.String(), nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_RemoveTagFromMCPServer_InvalidTagID(t *testing.T) {
	handler := NewTagHandler(nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Delete("/mcp-servers/:id/tags/:tagId", handler.RemoveTagFromMCPServer)

	req := httptest.NewRequest("DELETE", "/mcp-servers/"+mcpServerID.String()+"/tags/not-a-uuid", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetMCPServerTags_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/tags", handler.GetMCPServerTags)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/tags", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_SuggestTagsForMCPServer_InvalidID(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/mcp-servers/:id/tags/suggestions", handler.SuggestTagsForMCPServer)

	req := httptest.NewRequest("GET", "/mcp-servers/not-a-uuid/tags/suggestions", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestTagHandler_GetPopularTags_NoOrgContext(t *testing.T) {
	handler := NewTagHandler(nil)

	app := fiber.New()
	app.Get("/tags/popular", handler.GetPopularTags)

	req := httptest.NewRequest("GET", "/tags/popular", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestTagHandler_SearchTags_MissingQuery(t *testing.T) {
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)

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
	handler := NewTagHandler(nil)
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
	handler := NewTagHandler(nil)
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
	handler := NewTagHandler(nil)
	agentID := uuid.New()

	app := fiber.New()
	app.Post("/agents/:id/tags", func(c fiber.Ctx) error {
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
	handler := NewTagHandler(nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
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
	handler := NewTagHandler(nil)
	mcpServerID := uuid.New()

	app := fiber.New()
	app.Post("/mcp-servers/:id/tags", func(c fiber.Ctx) error {
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
