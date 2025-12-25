package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ContextError represents an error extracting values from request context
type ContextError struct {
	Field   string
	Message string
}

// GetOrganizationID safely extracts organization_id from Fiber context locals.
// Returns the UUID and true if successful, or uuid.Nil and false if not found or invalid type.
func GetOrganizationID(c fiber.Ctx) (uuid.UUID, bool) {
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return uuid.Nil, false
	}
	orgID, ok := orgIDValue.(uuid.UUID)
	return orgID, ok
}

// GetUserID safely extracts user_id from Fiber context locals.
// Returns the UUID and true if successful, or uuid.Nil and false if not found or invalid type.
func GetUserID(c fiber.Ctx) (uuid.UUID, bool) {
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return uuid.Nil, false
	}
	userID, ok := userIDValue.(uuid.UUID)
	return userID, ok
}

// RequireOrganizationID extracts organization_id and returns an error response if not found.
// Returns the organization ID and nil error on success, or uuid.Nil and an error response on failure.
func RequireOrganizationID(c fiber.Ctx) (uuid.UUID, error) {
	orgID, ok := GetOrganizationID(c)
	if !ok {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}
	return orgID, nil
}

// RequireUserID extracts user_id and returns an error response if not found.
// Returns the user ID and nil error on success, or uuid.Nil and an error response on failure.
func RequireUserID(c fiber.Ctx) (uuid.UUID, error) {
	userID, ok := GetUserID(c)
	if !ok {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}
	return userID, nil
}

// RequireOrgAndUserID extracts both organization_id and user_id from context.
// This is a convenience function for handlers that need both values.
// Returns both IDs and nil error on success, or returns an error response on failure.
func RequireOrgAndUserID(c fiber.Ctx) (orgID uuid.UUID, userID uuid.UUID, err error) {
	orgID, err = RequireOrganizationID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	userID, err = RequireUserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return orgID, userID, nil
}
