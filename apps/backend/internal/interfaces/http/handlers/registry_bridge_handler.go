package handlers

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// RegistryBridgeHandler exposes an admin endpoint to manually trigger a
// registry bridge push. The actual aggregation logic lives in
// RegistryBridgeService.
type RegistryBridgeHandler struct {
	bridgeService *application.RegistryBridgeService
}

// NewRegistryBridgeHandler creates a new handler for registry bridge operations.
func NewRegistryBridgeHandler(bridgeService *application.RegistryBridgeService) *RegistryBridgeHandler {
	return &RegistryBridgeHandler{
		bridgeService: bridgeService,
	}
}

// TriggerPush manually triggers the registry bridge aggregation and push.
// POST /api/v1/admin/registry-bridge/push
func (h *RegistryBridgeHandler) TriggerPush(c fiber.Ctx) error {
	if h.bridgeService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "registry bridge is not enabled",
			"message": "Set REGISTRY_BRIDGE_ENABLED=true to enable the registry bridge",
		})
	}

	ctx := context.Background()
	if err := h.bridgeService.AggregateAndPush(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "registry bridge push failed",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "Registry bridge push completed",
	})
}
