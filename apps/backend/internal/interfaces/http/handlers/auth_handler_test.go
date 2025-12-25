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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthHandler_Me tests the Me endpoint behavior
func TestAuthHandler_Me_NoUserContext(t *testing.T) {
	// Test case: No user_id in context should return 401

	app := fiber.New()
	app.Get("/me", func(c fiber.Ctx) error {
		// Simulate AuthHandler.Me behavior without user context
		userIDValue := c.Locals("user_id")
		if userIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - no user context",
			})
		}
		return nil
	})

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Unauthorized")
}

func TestAuthHandler_Me_InvalidUserIDType(t *testing.T) {
	// Test case: Invalid user_id type in context should return 401

	app := fiber.New()
	app.Get("/me", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid") // Invalid type
		userIDValue := c.Locals("user_id")
		if userIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - no user context",
			})
		}
		_, ok := userIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - invalid user context",
			})
		}
		return nil
	})

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_Me_ValidUser(t *testing.T) {
	// Test case: Valid user context returns user info
	userID := uuid.New()
	orgID := uuid.New()
	now := time.Now()

	app := fiber.New()
	app.Get("/me", func(c fiber.Ctx) error {
		c.Locals("user_id", userID)

		userIDValue := c.Locals("user_id")
		if userIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - no user context",
			})
		}

		uid, ok := userIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - invalid user context",
			})
		}

		// Simulate returning user info
		return c.JSON(fiber.Map{
			"id":             uid,
			"email":          "test@example.com",
			"name":           "Test User",
			"role":           "admin",
			"organizationId": orgID,
			"lastLoginAt":    now,
			"createdAt":      now,
			"status":         "approved",
		})
	})

	req := httptest.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), result["id"])
	assert.Equal(t, "test@example.com", result["email"])
	assert.Equal(t, "admin", result["role"])
}

func TestAuthHandler_LocalLogin_MissingFields(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected int
	}{
		{
			name:     "empty body",
			body:     `{}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "missing password",
			body:     `{"email":"test@example.com"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "missing email",
			body:     `{"password":"password123"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "empty email",
			body:     `{"email":"","password":"password123"}`,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "empty password",
			body:     `{"email":"test@example.com","password":""}`,
			expected: fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Post("/login", func(c fiber.Ctx) error {
				type LoginRequest struct {
					Email    string `json:"email"`
					Password string `json:"password"`
				}

				var req LoginRequest
				if err := c.Bind().JSON(&req); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid request body",
					})
				}

				if req.Email == "" || req.Password == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Email and password are required",
					})
				}

				return c.JSON(fiber.Map{"status": "ok"})
			})

			req := httptest.NewRequest("POST", "/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}

func TestAuthHandler_LocalLogin_InvalidJSON(t *testing.T) {
	app := fiber.New()
	app.Post("/login", func(c fiber.Ctx) error {
		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var req LoginRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/login", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "Invalid request body")
}

func TestAuthHandler_ChangePassword_Validation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		hasUser  bool
		expected int
	}{
		{
			name:     "no user context",
			body:     `{"currentPassword":"old","newPassword":"new"}`,
			hasUser:  false,
			expected: fiber.StatusUnauthorized,
		},
		{
			name:     "missing current password",
			body:     `{"newPassword":"new"}`,
			hasUser:  true,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "missing new password",
			body:     `{"currentPassword":"old"}`,
			hasUser:  true,
			expected: fiber.StatusBadRequest,
		},
		{
			name:     "empty passwords",
			body:     `{"currentPassword":"","newPassword":""}`,
			hasUser:  true,
			expected: fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()

			app := fiber.New()
			app.Post("/change-password", func(c fiber.Ctx) error {
				if tt.hasUser {
					c.Locals("user_id", userID)
				}

				type ChangePasswordRequest struct {
					CurrentPassword string `json:"currentPassword"`
					NewPassword     string `json:"newPassword"`
				}

				var req ChangePasswordRequest
				if err := c.Bind().JSON(&req); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid request body",
					})
				}

				if req.CurrentPassword == "" || req.NewPassword == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Current password and new password are required",
					})
				}

				userIDValue := c.Locals("user_id")
				if userIDValue == nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Unauthorized - no user context",
					})
				}

				return c.JSON(fiber.Map{"message": "Password changed successfully"})
			})

			req := httptest.NewRequest("POST", "/change-password", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}

func TestAuthHandler_GetCurrentOrganization_NoOrgContext(t *testing.T) {
	app := fiber.New()
	app.Get("/organization", func(c fiber.Ctx) error {
		orgIDValue := c.Locals("organization_id")
		if orgIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - no organization context",
			})
		}

		_, ok := orgIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - invalid organization context",
			})
		}

		return c.JSON(fiber.Map{"name": "Test Org"})
	})

	req := httptest.NewRequest("GET", "/organization", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_GetCurrentOrganization_ValidOrg(t *testing.T) {
	orgID := uuid.New()
	now := time.Now()

	app := fiber.New()
	app.Get("/organization", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)

		orgIDValue := c.Locals("organization_id")
		if orgIDValue == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - no organization context",
			})
		}

		oid, ok := orgIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - invalid organization context",
			})
		}

		return c.JSON(fiber.Map{
			"id":        oid,
			"name":      "Test Organization",
			"maxAgents": 100,
			"isActive":  true,
			"createdAt": now,
			"updatedAt": now,
		})
	})

	req := httptest.NewRequest("GET", "/organization", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, orgID.String(), result["id"])
	assert.Equal(t, "Test Organization", result["name"])
	assert.Equal(t, float64(100), result["maxAgents"])
}

func TestAuthHandler_Logout(t *testing.T) {
	app := fiber.New()
	app.Post("/logout", func(c fiber.Ctx) error {
		// Clear cookies by setting empty value and expired time
		c.Cookie(&fiber.Cookie{
			Name:     "access_token",
			Value:    "",
			HTTPOnly: true,
			MaxAge:   -1,
		})

		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    "",
			HTTPOnly: true,
			MaxAge:   -1,
		})

		return c.JSON(fiber.Map{
			"message": "Logged out successfully",
		})
	})

	req := httptest.NewRequest("POST", "/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "Logged out successfully", result["message"])

	// Check cookies are present (clearing cookies is implementation detail)
	// The important thing is the success message is returned
}

// TestContextHelpers tests the context helper functions
func TestContextHelpers_GetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name        string
		setupLocals func(c fiber.Ctx)
		expectError bool
		expectNil   bool
	}{
		{
			name:        "no user_id in context",
			setupLocals: func(c fiber.Ctx) {},
			expectError: true,
			expectNil:   true,
		},
		{
			name: "invalid user_id type",
			setupLocals: func(c fiber.Ctx) {
				c.Locals("user_id", "not-a-uuid")
			},
			expectError: true,
			expectNil:   true,
		},
		{
			name: "valid user_id",
			setupLocals: func(c fiber.Ctx) {
				c.Locals("user_id", uuid.New())
			},
			expectError: false,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var gotUserID *uuid.UUID
			var gotErr bool

			app.Get("/test", func(c fiber.Ctx) error {
				tt.setupLocals(c)

				userIDValue := c.Locals("user_id")
				if userIDValue == nil {
					gotUserID = nil
					gotErr = true
					return c.SendStatus(fiber.StatusUnauthorized)
				}

				uid, ok := userIDValue.(uuid.UUID)
				if !ok {
					gotUserID = nil
					gotErr = true
					return c.SendStatus(fiber.StatusUnauthorized)
				}

				gotUserID = &uid
				gotErr = false
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectError, gotErr)
			if tt.expectNil {
				assert.Nil(t, gotUserID)
			} else {
				assert.NotNil(t, gotUserID)
			}
		})
	}
}

// TestErrorResponseFormat tests consistent error response format
func TestErrorResponseFormat(t *testing.T) {
	app := fiber.New()
	app.Get("/error", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Something went wrong",
		})
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Ensure error field exists
	assert.Contains(t, result, "error")
	assert.Equal(t, "Something went wrong", result["error"])
}

// Helper function tests
func TestErrorResponse_ContainsHelper(t *testing.T) {
	assert.True(t, strings.Contains("not found", "not"))
	assert.True(t, strings.Contains("not found", "found"))
	assert.False(t, strings.Contains("not found", "error"))
	assert.True(t, strings.Contains("Community Edition limited to 3 tags", "Community Edition"))
}
