package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// The Require* helpers return these when a principal passed authentication
// without the context value a handler needs. They are *fiber.Error values
// so the app's error handler answers 401 with this message; a handler
// returns them unchanged. The helpers write no response themselves.
var (
	ErrOrganizationIDNotFound = fiber.NewError(fiber.StatusUnauthorized, "Organization ID not found in context")
	ErrUserIDNotFound         = fiber.NewError(fiber.StatusUnauthorized, "User ID not found in context")
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

// RequireOrganizationID extracts organization_id from context.
// Returns the organization ID and nil error on success, or uuid.Nil and
// ErrOrganizationIDNotFound (a 401 *fiber.Error) on failure.
func RequireOrganizationID(c fiber.Ctx) (uuid.UUID, error) {
	orgID, ok := GetOrganizationID(c)
	if !ok {
		return uuid.Nil, ErrOrganizationIDNotFound
	}
	return orgID, nil
}

// RequireUserID extracts user_id from context.
// Returns the user ID and nil error on success, or uuid.Nil and
// ErrUserIDNotFound (a 401 *fiber.Error) on failure.
func RequireUserID(c fiber.Ctx) (uuid.UUID, error) {
	userID, ok := GetUserID(c)
	if !ok {
		return uuid.Nil, ErrUserIDNotFound
	}
	return userID, nil
}

// RequireOrgAndUserID extracts both organization_id and user_id from context.
// This is a convenience function for handlers that need both values.
// Returns both IDs and nil error on success, or uuid.Nil for both and the
// first helper's 401 *fiber.Error on failure.
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
