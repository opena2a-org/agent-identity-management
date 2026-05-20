// Test fixture for tenantscope-lint: handler reads a tenant-scoped
// URL-param (mcp_id) without invoking LoadOwned/LoadOwnedViaAgent
// and without any visible OrganizationID identifier check.
//
// This file lives under testdata/ so the Go toolchain ignores it; the
// lint parses it directly via go/parser.
//
//go:build ignore_lint_testdata

package testdata

import (
	"github.com/gofiber/fiber/v3"
)

type LintTestViolatingHandler struct{}

// GetThing reads c.Params("mcp_id") without LoadOwned and without
// mentioning OrganizationID. The lint must flag this handler.
func (h *LintTestViolatingHandler) GetThing(c fiber.Ctx) error {
	mcpID := c.Params("mcp_id")
	if mcpID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mcp_id required",
		})
	}
	return c.JSON(fiber.Map{"mcp_id": mcpID})
}
