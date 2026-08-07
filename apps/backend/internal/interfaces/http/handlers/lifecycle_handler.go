package handlers

import (
	"crypto/sha256"
	"encoding/hex"
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

// revocationReasonRevoked is the only reason code this list emits.
//
// It must stay a closed set. The endpoint is unauthenticated and spans every
// organization, so the moment a reason carries operator-supplied free text it becomes
// a cross-organization disclosure channel for whatever a customer typed into it.
const revocationReasonRevoked = "revoked"

// revocationListPageSize bounds each repository read while the list is assembled.
// AgentRepository.List rejects a non-positive limit precisely so that no caller can ask
// it for an unbounded result set.
const revocationListPageSize = 500

// GetRevocationList returns the list of revoked agent IDs
// GET /api/v1/revocations
//
// Unauthenticated and cross-organization by design: a verifier has to be able to check
// revocation without holding a credential, which is what makes offline verification
// possible. Minimization is therefore the control that applies here, not authentication.
//
// The payload carries the agent id and a reason code, and nothing else. It previously
// also carried the agent's name and a revocation timestamp:
//   - `name` is an organization-internal namespace (`UNIQUE(organization_id, name)`)
//     with no opt-in mechanism and no consumer — neither SDK's CRL type has the field.
//   - `revokedAt` was read from `agents.updated_at`, which any later write to the row
//     silently rewrites. There is no `revoked_at` column to source it from.
//
// Both were inert only because this endpoint returned an empty list to every caller.
// Do not add fields back without a consumer that reads them.
func (h *LifecycleHandler) GetRevocationList(c fiber.Ctx) error {
	type RevokedEntry struct {
		AgentID uuid.UUID `json:"agentId"`
		Reason  string    `json:"reason"`
	}

	// Walk every page rather than asking for one unbounded result set. The list is
	// deliberately NOT truncated: a CRL that silently drops entries tells a verifier a
	// revoked agent is good, which is the same fail-open this endpoint was fixed for.
	//
	// The `status = 'revoked'` predicate runs in SQL. Filtering List's results in Go
	// instead made every request to this unauthenticated route read every agent row —
	// hundreds of milliseconds of server work at 20,000 agents, at a cost the caller
	// controls. Two independent measurements at that size landed at ~320ms and ~630ms
	// depending on row width, which is the point: it scales with the table, not with the
	// answer. The exact figure is not pinned here because nothing can check it.
	revoked := make([]RevokedEntry, 0)
	for offset := 0; ; offset += revocationListPageSize {
		ids, err := h.agentRepo.ListRevokedIDs(revocationListPageSize, offset)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch revocations",
			})
		}

		for _, id := range ids {
			revoked = append(revoked, RevokedEntry{
				AgentID: id,
				Reason:  revocationReasonRevoked,
			})
		}

		if len(ids) < revocationListPageSize {
			break
		}
	}

	// The validator is derived from the revocation set itself, so it changes when and
	// only when the set changes. It deliberately excludes `generatedAt`, which moves on
	// every request. The previous validator was built from the entry count and a
	// wall-clock bucket, which collides whenever one agent is revoked and another
	// leaves the list inside the same bucket — that is a stale CRL served as fresh.
	digest := sha256.New()
	for _, entry := range revoked {
		_, _ = digest.Write(entry.AgentID[:])
		_, _ = digest.Write([]byte(entry.Reason))
	}
	c.Set("Cache-Control", "max-age=300")
	c.Set("ETag", fmt.Sprintf(`"%s"`, hex.EncodeToString(digest.Sum(nil))))

	// The key stays `revocations`, which is what ATP-SPEC v1.0.0-rc1 §8.1 specifies. That
	// document is not in this repository — it lives at
	// https://github.com/opena2a-standards/agent-trust-protocol (ATP-SPEC.md, §8.1
	// "Trust Proof Revocation") and its response schema is published at
	// https://specs.opena2a.org/schemas/atp/revocation-list-v1.schema.json — so the
	// citation is given in full rather than as a bare section number nobody here can
	// resolve. Note this endpoint's path is NOT the one §8.1 specifies
	// (`/api/v1/trust/revocations`, served by the Registry) and AIM claims no conformance
	// to it; the key name is where the two agree. Both SDK CRL
	// types decode `entries` instead, so a fetcher wired naively against this body does
	// not get a usable list.
	//
	// Do NOT "fix" that by emitting both keys, and do not rename it in the SDKs either,
	// until the SDK side is verified to fail closed on a shape it cannot read. The
	// TypeScript cache throws on a bad shape; the Java cache accepted a decoded
	// `Crl(entries=null)` as a valid list, cached it FRESH, and the verifier read null
	// entries as "nothing is revoked" — a silent fail-open. Both are fixed now
	// (CrlCache.refreshNow rejects null entries, LocalAtxVerifier rejects a malformed
	// CRL rather than treating it as empty), but the ordering rule stands: whichever end
	// moves first must not be the one that turns a loud failure into a quiet one.
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

	// SECURITY: scope the lookup to the caller's organization. The agent ids arrive in the
	// REQUEST BODY, so without this any authenticated user could read status, trust score
	// and last-active for any agent in any organization by naming its UUID. Ids outside the
	// caller's org are absent from the result rather than rejected, so the response does not
	// reveal whether they exist.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agents, err := h.agentService.GetAgentsByIDs(c.Context(), orgID, ids)
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
