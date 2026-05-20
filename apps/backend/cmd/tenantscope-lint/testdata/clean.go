// Test fixture for tenantscope-lint: handler reads a tenant-scoped
// URL-param (mcp_id) and invokes LoadOwned to scope the lookup.
// The lint must NOT flag this handler.
//
//go:build ignore_lint_testdata

package testdata

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type LintTestCleanHandler struct {
	repo interface {
		GetByID(id uuid.UUID) (*Resource, error)
	}
}

type Resource struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
}

// GetThing reads c.Params("mcp_id") and wraps the lookup in LoadOwned.
// The lint must not flag this handler.
func (h *LintTestCleanHandler) GetThing(c fiber.Ctx) error {
	mcpIDStr := c.Params("mcp_id")
	mcpID, err := uuid.Parse(mcpIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid mcp_id",
		})
	}
	callerOrgID := c.Locals("organization_id").(uuid.UUID)
	resource, err := LoadOwned(c, h.repo.GetByID, mcpID, callerOrgID, func(r *Resource) uuid.UUID {
		return r.OrganizationID
	})
	if err != nil {
		return err
	}
	return c.JSON(resource)
}

// LoadOwned is referenced here so the lint's recognizedHelpers check
// sees the call. The real implementation lives in
// internal/interfaces/http/handlers/tenant_scope.go; this stub keeps the
// fixture standalone for the lint parser.
func LoadOwned[T any](
	c fiber.Ctx,
	loader func(uuid.UUID) (T, error),
	id uuid.UUID,
	callerOrgID uuid.UUID,
	orgIDOf func(T) uuid.UUID,
) (T, error) {
	var zero T
	return zero, nil
}
