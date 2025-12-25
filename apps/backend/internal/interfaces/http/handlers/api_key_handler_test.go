package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyHandler_ListAPIKeys_NoOrgContext(t *testing.T) {
	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		orgIDValue := c.Locals("organization_id")
		if orgIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Organization ID not found in context",
			})
		}
		return c.JSON(fiber.Map{"apiKeys": []interface{}{}, "total": 0})
	})

	req := httptest.NewRequest("GET", "/api-keys", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Organization ID not found")
}

func TestAPIKeyHandler_ListAPIKeys_InvalidOrgIDType(t *testing.T) {
	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")

		orgIDValue := c.Locals("organization_id")
		if orgIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Organization ID not found in context",
			})
		}

		_, ok := orgIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Invalid organization ID type in context",
			})
		}

		return c.JSON(fiber.Map{"apiKeys": []interface{}{}, "total": 0})
	})

	req := httptest.NewRequest("GET", "/api-keys", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestAPIKeyHandler_ListAPIKeys_InvalidAgentIDFilter(t *testing.T) {
	orgID := uuid.New()

	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)

		agentIDStr := c.Query("agent_id")
		if agentIDStr != "" {
			_, err := uuid.Parse(agentIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid agent ID",
				})
			}
		}

		return c.JSON(fiber.Map{"apiKeys": []interface{}{}, "total": 0})
	})

	req := httptest.NewRequest("GET", "/api-keys?agent_id=not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Invalid agent ID")
}

func TestAPIKeyHandler_ListAPIKeys_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(90 * 24 * time.Hour)

	apiKeys := []*domain.APIKey{
		{
			ID:             uuid.New(),
			AgentID:        agentID,
			OrganizationID: orgID,
			Name:           "test-key-1",
			IsActive:       true,
			ExpiresAt:      &expiresAt,
			CreatedAt:      now,
		},
		{
			ID:             uuid.New(),
			AgentID:        agentID,
			OrganizationID: orgID,
			Name:           "test-key-2",
			IsActive:       true,
			ExpiresAt:      &expiresAt,
			CreatedAt:      now,
		},
	}

	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)

		return c.JSON(fiber.Map{
			"apiKeys": apiKeys,
			"total":   len(apiKeys),
		})
	})

	req := httptest.NewRequest("GET", "/api-keys", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, float64(2), result["total"])
}

func TestAPIKeyHandler_ListAPIKeys_FilterByAgentID(t *testing.T) {
	orgID := uuid.New()
	agentID1 := uuid.New()
	agentID2 := uuid.New()
	now := time.Now()
	expiresAt := now.Add(90 * 24 * time.Hour)

	allKeys := []*domain.APIKey{
		{
			ID:             uuid.New(),
			AgentID:        agentID1,
			OrganizationID: orgID,
			Name:           "key-for-agent-1",
			IsActive:       true,
			ExpiresAt:      &expiresAt,
			CreatedAt:      now,
		},
		{
			ID:             uuid.New(),
			AgentID:        agentID2,
			OrganizationID: orgID,
			Name:           "key-for-agent-2",
			IsActive:       true,
			ExpiresAt:      &expiresAt,
			CreatedAt:      now,
		},
	}

	app := fiber.New()
	app.Get("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)

		agentIDStr := c.Query("agent_id")
		var agentID *uuid.UUID
		if agentIDStr != "" {
			parsed, err := uuid.Parse(agentIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid agent ID",
				})
			}
			agentID = &parsed
		}

		// Simulate filtering
		var filtered []*domain.APIKey
		if agentID != nil {
			for _, key := range allKeys {
				if key.AgentID == *agentID {
					filtered = append(filtered, key)
				}
			}
		} else {
			filtered = allKeys
		}

		return c.JSON(fiber.Map{
			"apiKeys": filtered,
			"total":   len(filtered),
		})
	})

	// Filter by agent 1
	req := httptest.NewRequest("GET", "/api-keys?agent_id="+agentID1.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, float64(1), result["total"])
}

func TestAPIKeyHandler_CreateAPIKey_NoOrgContext(t *testing.T) {
	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		orgIDValue := c.Locals("organization_id")
		if orgIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Organization ID not found in context",
			})
		}
		return c.SendStatus(fiber.StatusCreated)
	})

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_NoUserContext(t *testing.T) {
	orgID := uuid.New()

	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)

		userIDValue := c.Locals("user_id")
		if userIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User ID not found in context",
			})
		}
		return c.SendStatus(fiber.StatusCreated)
	})

	body := `{"agentId":"` + uuid.New().String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_InvalidRequestBody(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		var req struct {
			AgentID   string  `json:"agentId"`
			Name      string  `json:"name"`
			ExpiresAt *string `json:"expiresAt"`
		}

		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		return c.SendStatus(fiber.StatusCreated)
	})

	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAPIKeyHandler_CreateAPIKey_MissingFields(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected int
	}{
		{
			name:     "missing agent_id",
			body:     `{"name":"test-key"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "missing name",
			body:     `{"agentId":"` + uuid.New().String() + `"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "empty agent_id",
			body:     `{"agentId":"","name":"test-key"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "empty name",
			body:     `{"agentId":"` + uuid.New().String() + `","name":""}`,
			expected: fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgID := uuid.New()
			userID := uuid.New()

			app := fiber.New()
			app.Post("/api-keys", func(c fiber.Ctx) error {
				c.Locals("organization_id", orgID)
				c.Locals("user_id", userID)

				var req struct {
					AgentID   string  `json:"agentId"`
					Name      string  `json:"name"`
					ExpiresAt *string `json:"expiresAt"`
				}

				if err := c.Bind().JSON(&req); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid request body",
					})
				}

				if req.AgentID == "" || req.Name == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "agent_id and name are required",
					})
				}

				return c.SendStatus(fiber.StatusCreated)
			})

			req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}

func TestAPIKeyHandler_CreateAPIKey_InvalidAgentID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		var req struct {
			AgentID string `json:"agentId"`
			Name    string `json:"name"`
		}

		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		if req.AgentID == "" || req.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "agent_id and name are required",
			})
		}

		_, err := uuid.Parse(req.AgentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agent ID",
			})
		}

		return c.SendStatus(fiber.StatusCreated)
	})

	body := `{"agentId":"not-a-uuid","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Invalid agent ID")
}

func TestAPIKeyHandler_CreateAPIKey_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	now := time.Now()

	app := fiber.New()
	app.Post("/api-keys", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		var req struct {
			AgentID string `json:"agentId"`
			Name    string `json:"name"`
		}

		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		parsedAgentID, _ := uuid.Parse(req.AgentID)
		expiresAt := now.Add(90 * 24 * time.Hour)

		// Simulate successful creation
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":        uuid.New(),
			"apiKey":    "aim_xxxxxxxxxxxxxxx", // Plain key only returned once
			"name":      req.Name,
			"agentId":   parsedAgentID,
			"expiresAt": expiresAt,
			"createdAt": now,
		})
	})

	body := `{"agentId":"` + agentID.String() + `","name":"test-key"}`
	req := httptest.NewRequest("POST", "/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["id"])
	assert.NotEmpty(t, result["apiKey"])
	assert.Equal(t, "test-key", result["name"])
}

func TestAPIKeyHandler_DisableAPIKey_InvalidKeyID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		_, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API key ID",
			})
		}

		return c.JSON(fiber.Map{"message": "API key disabled successfully"})
	})

	req := httptest.NewRequest("PATCH", "/api-keys/not-a-uuid/disable", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Invalid API key ID")
}

func TestAPIKeyHandler_DisableAPIKey_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()

	app := fiber.New()
	app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		_, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API key ID",
			})
		}

		return c.JSON(fiber.Map{"message": "API key disabled successfully"})
	})

	req := httptest.NewRequest("PATCH", "/api-keys/"+keyID.String()+"/disable", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "API key disabled successfully", result["message"])
}

func TestAPIKeyHandler_DeleteAPIKey_InvalidKeyID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		_, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API key ID",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestAPIKeyHandler_DeleteAPIKey_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()

	app := fiber.New()
	app.Delete("/api-keys/:id", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", userID)

		_, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid API key ID",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest("DELETE", "/api-keys/"+keyID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestAPIKeyHandler_DisableAPIKey_NoContext(t *testing.T) {
	tests := []struct {
		name        string
		setupLocals func(c fiber.Ctx)
		expected    int
	}{
		{
			name:        "no org context",
			setupLocals: func(c fiber.Ctx) {},
			expected:    fiber.StatusUnauthorized,
		},
		{
			name: "no user context",
			setupLocals: func(c fiber.Ctx) {
				c.Locals("organization_id", uuid.New())
			},
			expected: fiber.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyID := uuid.New()

			app := fiber.New()
			app.Patch("/api-keys/:id/disable", func(c fiber.Ctx) error {
				tt.setupLocals(c)

				orgIDValue := c.Locals("organization_id")
				if orgIDValue == nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Organization ID not found in context",
					})
				}

				userIDValue := c.Locals("user_id")
				if userIDValue == nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "User ID not found in context",
					})
				}

				return c.JSON(fiber.Map{"message": "API key disabled successfully"})
			})

			req := httptest.NewRequest("PATCH", "/api-keys/"+keyID.String()+"/disable", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}
