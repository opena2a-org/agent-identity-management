package handlers

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextTestErrorHandler answers like the server's error handler: a *fiber.Error
// with its own code and message, anything else with a 500
func contextTestErrorHandler(c fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err.Error(),
	})
}

// newContextTestApp creates a Fiber app with the context test error handler
func newContextTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: contextTestErrorHandler,
	})
}

// ===========================
// GetOrganizationID Tests
// ===========================

func TestGetOrganizationID_NotFound(t *testing.T) {
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		gotID, gotOK = GetOrganizationID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, uuid.Nil, gotID)
	assert.False(t, gotOK)
}

func TestGetOrganizationID_InvalidType(t *testing.T) {
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", "not-a-uuid")
		gotID, gotOK = GetOrganizationID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, uuid.Nil, gotID)
	assert.False(t, gotOK)
}

func TestGetOrganizationID_Valid(t *testing.T) {
	expectedID := uuid.New()
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", expectedID)
		gotID, gotOK = GetOrganizationID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, expectedID, gotID)
	assert.True(t, gotOK)
}

// ===========================
// GetUserID Tests
// ===========================

func TestGetUserID_NotFound(t *testing.T) {
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		gotID, gotOK = GetUserID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, uuid.Nil, gotID)
	assert.False(t, gotOK)
}

func TestGetUserID_InvalidType(t *testing.T) {
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", "not-a-uuid")
		gotID, gotOK = GetUserID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, uuid.Nil, gotID)
	assert.False(t, gotOK)
}

func TestGetUserID_Valid(t *testing.T) {
	expectedID := uuid.New()
	app := fiber.New()
	var gotID uuid.UUID
	var gotOK bool

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", expectedID)
		gotID, gotOK = GetUserID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, expectedID, gotID)
	assert.True(t, gotOK)
}

// ===========================
// RequireOrganizationID Tests
// ===========================

func TestRequireOrganizationID_NotFound(t *testing.T) {
	app := newContextTestApp()

	app.Get("/test", func(c fiber.Ctx) error {
		orgID, err := RequireOrganizationID(c)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"orgId": orgID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequireOrganizationID_Valid(t *testing.T) {
	expectedID := uuid.New()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", expectedID)
		orgID, err := RequireOrganizationID(c)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"orgId": orgID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// RequireUserID Tests
// ===========================

func TestRequireUserID_NotFound(t *testing.T) {
	app := newContextTestApp()

	app.Get("/test", func(c fiber.Ctx) error {
		userID, err := RequireUserID(c)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"userId": userID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequireUserID_Valid(t *testing.T) {
	expectedID := uuid.New()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", expectedID)
		userID, err := RequireUserID(c)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"userId": userID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// ===========================
// RequireOrgAndUserID Tests
// ===========================

func TestRequireOrgAndUserID_NoOrg(t *testing.T) {
	app := newContextTestApp()
	var gotOrgID, gotUserID uuid.UUID
	var gotErr error

	app.Get("/test", func(c fiber.Ctx) error {
		// No organization_id set - returns ErrOrganizationIDNotFound
		gotOrgID, gotUserID, gotErr = RequireOrgAndUserID(c)
		return gotErr
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The error handler answers ErrOrganizationIDNotFound with its 401
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, uuid.Nil, gotOrgID)
	assert.Equal(t, uuid.Nil, gotUserID)
	assert.ErrorIs(t, gotErr, ErrOrganizationIDNotFound)
}

func TestRequireOrgAndUserID_NoUser(t *testing.T) {
	app := newContextTestApp()
	var gotOrgID, gotUserID uuid.UUID
	var gotErr error

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		// No user_id set - returns ErrUserIDNotFound
		gotOrgID, gotUserID, gotErr = RequireOrgAndUserID(c)
		return gotErr
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// The error handler answers ErrUserIDNotFound with its 401
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	// Both IDs are nil when the helper fails
	assert.Equal(t, uuid.Nil, gotOrgID)
	assert.Equal(t, uuid.Nil, gotUserID)
	assert.ErrorIs(t, gotErr, ErrUserIDNotFound)
}

func TestRequireOrgAndUserID_Valid(t *testing.T) {
	expectedOrgID := uuid.New()
	expectedUserID := uuid.New()
	app := fiber.New()
	var gotOrgID, gotUserID uuid.UUID

	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("organization_id", expectedOrgID)
		c.Locals("user_id", expectedUserID)
		var err error
		gotOrgID, gotUserID, err = RequireOrgAndUserID(c)
		if err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, expectedOrgID, gotOrgID)
	assert.Equal(t, expectedUserID, gotUserID)
}

// ===========================
// ContextError Tests
// ===========================

func TestContextError_Struct(t *testing.T) {
	err := ContextError{
		Field:   "organization_id",
		Message: "not found in context",
	}

	assert.Equal(t, "organization_id", err.Field)
	assert.Equal(t, "not found in context", err.Message)
}
