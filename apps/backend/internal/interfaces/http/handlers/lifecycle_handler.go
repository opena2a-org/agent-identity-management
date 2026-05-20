package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// LifecycleHandler handles agent lifecycle endpoints
type LifecycleHandler struct {
	agentService *application.AgentService
	agentRepo    domain.AgentRepository
}

// NewLifecycleHandler creates a new lifecycle handler
func NewLifecycleHandler(
	agentService *application.AgentService,
	agentRepo domain.AgentRepository,
) *LifecycleHandler {
	return &LifecycleHandler{
		agentService: agentService,
		agentRepo:    agentRepo,
	}
}

// Heartbeat records a heartbeat for an agent
// POST /api/v1/sdk-api/agents/:id/heartbeat
func (h *LifecycleHandler) Heartbeat(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (defect #20): verify the agent belongs to the caller's
	// organization before mutating its heartbeat or trust score. Without this
	// check, any SDK token holder can spoof liveness signals for agents in
	// other organizations.
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	agent, err := h.agentService.RecordHeartbeat(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	return c.JSON(fiber.Map{
		"agentId":       agent.ID,
		"status":        agent.Status,
		"lastHeartbeat": agent.LastHeartbeat,
		"trustScore":    agent.TrustScore,
	})
}

// GetRevocationList returns the list of revoked agent and token IDs
// GET /api/v1/revocations
func (h *LifecycleHandler) GetRevocationList(c fiber.Ctx) error {
	agents, err := h.agentRepo.List(0, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	type RevokedEntry struct {
		AgentID   uuid.UUID `json:"agentId"`
		Name      string    `json:"name"`
		RevokedAt time.Time `json:"revokedAt"`
		Reason    string    `json:"reason"`
	}

	revoked := make([]RevokedEntry, 0)
	for _, agent := range agents {
		if agent.Status == domain.AgentStatusRevoked {
			revoked = append(revoked, RevokedEntry{
				AgentID:   agent.ID,
				Name:      agent.Name,
				RevokedAt: agent.UpdatedAt,
				Reason:    "revoked",
			})
		}
	}

	if revoked == nil {
		revoked = []RevokedEntry{}
	}

	// Set cache headers
	c.Set("Cache-Control", "max-age=300")
	c.Set("ETag", fmt.Sprintf(`"%d-%d"`, len(revoked), time.Now().Unix()/300))

	return c.JSON(fiber.Map{
		"revocations": revoked,
		"total":       len(revoked),
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// BulkStatus returns status for multiple agents
// POST /api/v1/agents/bulk-status
func (h *LifecycleHandler) BulkStatus(c fiber.Ctx) error {
	var request struct {
		AgentIDs []string `json:"agentIds"`
	}

	if err := c.Bind().JSON(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(request.AgentIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agentIds is required",
		})
	}

	if len(request.AgentIDs) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Maximum 100 agent IDs per request",
		})
	}

	// Parse UUIDs
	ids := make([]uuid.UUID, 0)
	for _, idStr := range request.AgentIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid agent ID: %s", idStr),
			})
		}
		ids = append(ids, id)
	}

	agents, err := h.agentService.GetAgentsByIDs(c.Context(), ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	type AgentStatusEntry struct {
		ID         uuid.UUID  `json:"id"`
		Status     string     `json:"status"`
		TrustScore float64    `json:"trustScore"`
		LastActive *time.Time `json:"lastActive"`
	}

	entries := make([]AgentStatusEntry, 0)
	for _, agent := range agents {
		entries = append(entries, AgentStatusEntry{
			ID:         agent.ID,
			Status:     string(agent.Status),
			TrustScore: agent.TrustScore,
			LastActive: agent.LastActive,
		})
	}

	if entries == nil {
		entries = []AgentStatusEntry{}
	}

	return c.JSON(fiber.Map{
		"agents": entries,
		"total":  len(entries),
	})
}
