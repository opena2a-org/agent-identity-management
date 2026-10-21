package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
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
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY: replace the pre-fix existence oracle (404 "Agent not
	// found" vs 403 "Access denied") with the LoadOwned helper. Both
	// branches now collapse to a fixed 404 body, closing the side
	// channel that let an attacker enumerate agent UUIDs across
	// tenants. The returned agent is reused below for audit-metadata
	// capture (agent.Name) and the response body.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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
		"agentId":               agentID.String(),
		"agentName":             agent.Name,
		"publicKey":             agent.PublicKey,
		"keyAlgorithm":          agent.KeyAlgorithm,
		"certificateUrl":        agent.CertificateURL,
		"keyCreatedAt":          agent.KeyCreatedAt,
		"keyExpiresAt":          agent.KeyExpiresAt,
		"keyRotationGraceUntil": agent.KeyRotationGraceUntil,
		"rotationCount":         agent.RotationCount,
		"hasPreviousPublicKey":  agent.PreviousPublicKey != nil,
		// Whether the server stores this agent's private key: the fact only, never the key.
		"serverHoldsPrivateKey": agent.EncryptedPrivateKey != nil,
		"pqcPublicKey":          agent.PQCPublicKey,
		"pqcKeyAlgorithm":       agent.PQCKeyAlgorithm,
		"hybridModeEnabled":     agent.HybridModeEnabled,
		"pqcKeyCreatedAt":       agent.PQCKeyCreatedAt,
		"pqcKeyExpiresAt":       agent.PQCKeyExpiresAt,
	})
}

// agentAuditLogsRoute names the audit log route as the server registers it,
// for the outcome middleware.BindAgentID records.
const agentAuditLogsRoute = "GET /api/v1/agents/:id/audit-logs"

// GetAgentAuditLogs returns audit logs for a specific agent with pagination
// @Summary Get agent audit logs
// @Description Get audit logs for a specific agent with pagination support. Only an admin's records carry ipAddress, userAgent and metadata; every other caller gets each record without them. A request an agent signs with its Ed25519 key, or sends with its service token, may name only that agent's own ID.
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of logs to return (default: 50, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied, or another agent's ID named by an agent's own request"
// @Router /agents/{id}/audit-logs [get]
func (h *AgentHandler) GetAgentAuditLogs(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	// A user's session and an API key carry a user ID. A request an agent
	// signs, and its service token, carry the agent's ID alone.
	userID, byUser := c.Locals("user_id").(uuid.UUID)
	callerAgentID, byAgent := c.Locals("agent_id").(uuid.UUID)
	if !byUser && (!byAgent || callerAgentID == uuid.Nil) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// An agent without a user ID reads its own audit log only: another
	// agent's ID is refused as a route bound to its agent refuses it, before
	// the agent it names is read. An API key carries its creator's user ID and
	// is served as before while the route's binding is held
	// (agent_binding_routes_test.go).
	if !byUser && !middleware.BindAgentID(c, agentAuditLogsRoute, agentID) {
		return nil
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

	// SECURITY: replace the pre-fix existence oracle (404 "Agent not
	// found" vs 403 "Access denied") with the LoadOwned helper. Both
	// branches now collapse to a fixed 404 body, closing the side
	// channel that let an attacker enumerate agent UUIDs across
	// tenants. The returned agent is reused below for audit-metadata
	// capture (agent.Name) and the response body.
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	agent := LoadOwned(c, loader, agentID, orgID, agentOrgID)
	if agent == nil {
		return nil
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

	// Log this audit log query, with the agent as its actor when no user made it
	metadata := map[string]interface{}{
		"agentName":        agent.Name,
		"results_returned": len(logs),
		"limit":            limit,
		"offset":           offset,
	}
	if byUser {
		h.auditService.LogAction(c.Context(), orgID, userID, domain.AuditActionView, "agent_audit_logs", agentID, c.IP(), c.Get("User-Agent"), metadata)
	} else {
		h.auditService.LogAgentAction(c.Context(), orgID, callerAgentID, domain.AuditActionView, "agent_audit_logs", agentID, c.IP(), c.Get("User-Agent"), metadata)
	}

	return c.JSON(fiber.Map{
		"agentId":   agentID.String(),
		"agentName": agent.Name,
		"logs":      auditLogsForCaller(c, logs),
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}
