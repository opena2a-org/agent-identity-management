package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ========================================
// Agent Security Endpoints
// ========================================

// GetAgentKeyVault returns the agent's key vault information (public key, certificate, expiration, rotation status)
// @Summary Get agent key vault
// @Description Get agent's cryptographic key vault information including public key, certificate URL, key expiration, and rotation status
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/key-vault [get]
func (h *AgentHandler) GetAgentKeyVault(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Log audit - viewing key vault is a sensitive action
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"agent_key_vault",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
		},
	)

	// Return key vault information
	return c.JSON(fiber.Map{
		"agentId":                 agentID.String(),
		"agentName":               agent.Name,
		"publicKey":               agent.PublicKey,
		"keyAlgorithm":            agent.KeyAlgorithm,
		"certificateUrl":          agent.CertificateURL,
		"keyCreatedAt":           agent.KeyCreatedAt,
		"keyExpiresAt":           agent.KeyExpiresAt,
		"key_rotation_grace_until": agent.KeyRotationGraceUntil,
		"rotationCount":           agent.RotationCount,
		"has_previous_public_key":  agent.PreviousPublicKey != nil,
	})
}

// GetAgentAuditLogs returns audit logs for a specific agent with pagination
// @Summary Get agent audit logs
// @Description Get audit logs for a specific agent with pagination support
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of logs to return (default: 50, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/audit-logs [get]
func (h *AgentHandler) GetAgentAuditLogs(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse pagination parameters
	limit := 50 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && parsedLimit == 1 {
			if limit > 100 {
				limit = 100 // enforce max
			}
			if limit < 1 {
				limit = 50 // reset to default
			}
		}
	}

	offset := 0 // default
	if offsetStr := c.Query("offset"); offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get audit logs filtered by agent ID (entity_id)
	logs, total, err := h.auditService.GetAuditLogs(
		c.Context(),
		orgID,
		"",       // action filter (empty = all)
		"agent",  // entity_type
		&agentID, // entity_id filter
		nil,      // user_id filter (nil = all users)
		nil,      // start_date
		nil,      // end_date
		limit,
		offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch audit logs",
		})
	}

	// Log this audit log query
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"agent_audit_logs",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":       agent.Name,
			"results_returned": len(logs),
			"limit":            limit,
			"offset":           offset,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":   agentID.String(),
		"agentName": agent.Name,
		"logs":       logs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}
