package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A2AHandler handles A2A (Agent-to-Agent) protocol endpoints
type A2AHandler struct {
	a2aService *application.A2AService
	// a2aReader is the narrow read-side interface used by ownership-check
	// LoadOwned loaders (RevokeConsent + UpdateTaskState — A3d-vii.b).
	// Defaults to a2aService in NewA2AHandler; tests can swap a mock.
	a2aReader A2AReader
	// agentService is typed as the interface (AgentServicer) rather than the
	// concrete *application.AgentService so tests can swap a mock. The
	// constructor still accepts the concrete service — Go satisfies the
	// interface implicitly at assignment. Used by loadOwnedAgent
	// (A3d-vii.a) for tenant scoping.
	agentService AgentServicer
	auditService *application.AuditService
}

// NewA2AHandler creates a new A2A handler. a2aService is wired into both
// the concrete-pointer slot (used for mutation methods like
// RegisterAgentCard, RevokeConsent, UpdateA2ATaskState) and the narrow
// A2AReader slot (used by ownership-check LoadOwned loaders). Tests may
// overwrite a2aReader and/or agentService independently with mocks.
func NewA2AHandler(
	a2aService *application.A2AService,
	agentService *application.AgentService,
	auditService *application.AuditService,
) *A2AHandler {
	return &A2AHandler{
		a2aService:   a2aService,
		a2aReader:    a2aService,
		agentService: agentService,
		auditService: auditService,
	}
}

// loadOwnedAgent is the A2A-specific wrapper around LoadOwned. It adapts
// agentService.GetAgent (the only agent loader A2AHandler has access to)
// to the LoadOwned loader signature. On any failure path (loader error,
// cross-tenant mismatch, nil resource) it writes a 404 + {"error":"not
// found"} body to the response and returns nil. Callers MUST `return
// nil` to let fiber finalize the response.
//
// Mirrors the inline closure first used at GetSkillAttestations (A3c #43)
// and consolidates the pattern for the A3d-vii.a sweep — wraps 11
// agent-scoped handlers with a single line each.
func (h *A2AHandler) loadOwnedAgent(c fiber.Ctx, agentID, orgID uuid.UUID) *domain.Agent {
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	return LoadOwned(c, loader, agentID, orgID, agentOrgID)
}

// ============================================================================
// Agent Card Endpoints
// ============================================================================

// RegisterAgentCard registers an A2A agent card for an agent
// POST /api/v1/a2a/agents/:id/card
func (h *A2AHandler) RegisterAgentCard(c fiber.Ctx) error {
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

	var req struct {
		CardURL  string          `json:"cardUrl"`
		CardData json.RawMessage `json:"cardData"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.CardURL == "" && len(req.CardData) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cardUrl or cardData is required",
		})
	}

	resp, err := h.a2aService.RegisterAgentCard(c.Context(), application.RegisterAgentCardRequest{
		AgentID:  agentID,
		CardURL:  req.CardURL,
		CardData: req.CardData,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"a2a_agent_card",
		resp.CardID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentId": agentID,
			"cardUrl": req.CardURL,
			"skills":  resp.Skills,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(resp)
}

// GetAgentCard returns the A2A agent card for an agent
// GET /api/v1/a2a/agents/:id/card
func (h *A2AHandler) GetAgentCard(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. Bound on both
	// `/agents/:id/card` and `/cards/:id` (SDK alt); :id is the agent
	// UUID in both. Cross-tenant access → 404 not found.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	// Get enhanced card with AIM extensions
	card, err := h.a2aService.GetEnhancedAgentCard(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent card not found",
		})
	}

	return c.JSON(card)
}

// RefreshCardAttestation refreshes the attestation for an agent's card
// POST /api/v1/a2a/agents/:id/card/refresh
func (h *A2AHandler) RefreshCardAttestation(c fiber.Ctx) error {
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

	card, err := h.a2aService.RefreshAttestation(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"a2a_attestation",
		card.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentId":            agentID,
			"attestationExpires": card.AttestationExpiresAt,
		},
	)

	return c.JSON(fiber.Map{
		"cardId":             card.ID,
		"agentId":            card.AgentID,
		"attestationExpires": card.AttestationExpiresAt,
		"message":            "Attestation refreshed successfully",
	})
}

// ============================================================================
// Request Signing Endpoints (for SDK/programmatic access)
// ============================================================================

// SignRequest signs an A2A request for an agent
// POST /api/v1/a2a/agents/:id/sign
func (h *A2AHandler) SignRequest(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Body   string `json:"body"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent AFTER body
	// validation (matches RecordMCPConnection precedent — preserves
	// existing 400-shape contracts). Signing for a cross-tenant agent
	// would expose key material via the signing service; LoadOwned
	// short-circuits to 404.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	signature, err := h.a2aService.SignA2ARequest(
		c.Context(),
		agentID,
		req.Method,
		req.Path,
		[]byte(req.Body),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"agentId":   signature.AgentID,
		"timestamp": signature.Timestamp,
		"nonce":     signature.Nonce,
		"signature": signature.Signature,
	})
}

// VerifyRequest verifies an incoming A2A request signature
// POST /api/v1/a2a/verify
func (h *A2AHandler) VerifyRequest(c fiber.Ctx) error {
	var req application.VerifyA2ARequestRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	result, err := h.a2aService.VerifyA2ARequest(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if !result.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"valid": false,
			"error": result.Error,
		})
	}

	return c.JSON(fiber.Map{
		"valid":            true,
		"agentId":          result.AgentID,
		"agentName":        result.AgentName,
		"trustScore":       result.TrustScore,
		"capabilities":     result.Capabilities,
		"attestationValid": result.AttestationValid,
	})
}

// ============================================================================
// Trust Score Endpoints
// ============================================================================

// GetA2ATrustScore returns the A2A-specific trust score for an agent
// GET /api/v1/a2a/agents/:id/trust-score
// Auto-computes trust score if none exists or score is zero
func (h *A2AHandler) GetA2ATrustScore(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. Trust scores
	// disclose competitive/operational information; cross-tenant reads
	// are an existence + value leak. LoadOwned → 404 on mismatch.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	// First try to get existing trust score
	score, err := h.a2aService.GetA2ATrustScore(c.Context(), agentID)
	if err != nil || score == nil || score.A2ATrustScore == nil || *score.A2ATrustScore == 0 {
		// Auto-compute if no score exists or score is zero
		computedScore, computeErr := h.a2aService.ComputeA2ATrustScore(c.Context(), agentID)
		if computeErr != nil {
			// If computation fails, return zero score with message
			return c.JSON(fiber.Map{
				"agentId": agentID,
				"score":   0.0,
				"message": "Could not compute trust score: " + computeErr.Error(),
			})
		}
		return c.JSON(computedScore)
	}

	return c.JSON(score)
}

// ComputeA2ATrustScore computes and updates the A2A trust score
// POST /api/v1/a2a/agents/:id/trust-score/compute
func (h *A2AHandler) ComputeA2ATrustScore(c fiber.Ctx) error {
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

	score, err := h.a2aService.ComputeA2ATrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCalculate,
		"a2a_trust_score",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"a2aTrustScore": score.A2ATrustScore,
			"peerAverage":   score.PeerTrustAverage,
			"uniquePeers":   score.UniquePeersCount,
		},
	)

	return c.JSON(score)
}

// GetPeerTrustScore returns the trust relationship between two agents
// GET /api/v1/a2a/agents/:id/peers/:peer_id/trust
func (h *A2AHandler) GetPeerTrustScore(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	peerID, err := uuid.Parse(c.Params("peer_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid peer ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the PATH agent only. A2A peer
	// trust is fundamentally cross-org by protocol design — agents in
	// different orgs build trust relationships through interactions, so
	// the peer agent MUST NOT be scoped. The path agent however is the
	// subject of the trust query and must belong to the caller's org;
	// otherwise this endpoint becomes an enumeration oracle for which
	// org-B agents trust which org-C peers.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	trust, err := h.a2aService.GetPeerTrustScore(c.Context(), agentID, peerID)
	if err != nil {
		// SECURITY (A3d-vii.a Phase 4.5): normalize the post-LoadOwned
		// "not found" body to match `tenant_scope.go:respondResourceNotFound`.
		// Pre-fix the body text differed between cross-tenant access ({"error":"not found"})
		// and "peer relation missing for a same-org agent"
		// ({"error":"Peer trust relationship not found"}), giving a same-org
		// caller a side channel to distinguish the two states.
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "not found",
		})
	}

	return c.JSON(trust)
}

// ============================================================================
// Task Logging Endpoints
// ============================================================================

// ListTasks returns paginated A2A tasks with optional filters
// GET /api/v1/a2a/tasks
func (h *A2AHandler) ListTasks(c fiber.Ctx) error {
	limit := 100
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	var agentID *uuid.UUID
	if idStr := c.Query("agentId"); idStr != "" {
		if parsed, err := uuid.Parse(idStr); err == nil {
			agentID = &parsed
		}
	}
	state := c.Query("state")

	tasks, total, err := h.a2aService.ListA2ATasks(c.Context(), agentID, state, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"tasks":  tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// LogTask logs an A2A task for audit trail
// POST /api/v1/a2a/tasks
// Accepts both internal format and SDK format for compatibility
func (h *A2AHandler) LogTask(c fiber.Ctx) error {
	// Accept SDK format with field name variations
	var sdkReq struct {
		// SDK format
		TargetAgentID  string `json:"targetAgentId"`
		TaskID         string `json:"taskId"`
		TaskType       string `json:"taskType"`
		Status         string `json:"status"`
		SkillID        string `json:"skillId"`
		SdkClientAgent string `json:"clientAgentId"` // SDK can also send clientAgentId as string
		// Internal format
		ExternalTaskID string    `json:"externalTaskId"`
		ContextID      string    `json:"contextId"`
		ClientAgentID  uuid.UUID `json:"_clientAgentId"` // Renamed to avoid conflict
		RemoteAgentID  uuid.UUID `json:"remoteAgentId"`
	}
	if err := c.Bind().JSON(&sdkReq); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Convert SDK format to internal format
	var req application.LogA2ATaskRequest
	if sdkReq.TargetAgentID != "" {
		// SDK format - convert fields
		targetAgentID, err := uuid.Parse(sdkReq.TargetAgentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid target agent ID",
			})
		}
		req.RemoteAgentID = targetAgentID
		req.ExternalTaskID = sdkReq.TaskID
		req.ContextID = sdkReq.TaskType // Use taskType as context
		if sdkReq.SkillID != "" {
			req.SkillID = sdkReq.SkillID
		} else {
			req.SkillID = sdkReq.TaskType // Fallback to taskType
		}

		// Try to get client agent ID from request body first, then context
		if sdkReq.SdkClientAgent != "" {
			clientAgentID, err := uuid.Parse(sdkReq.SdkClientAgent)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid client agent ID",
				})
			}
			req.ClientAgentID = clientAgentID
		} else {
			// Fallback to context
			clientAgentID, err := h.getAgentIDFromContext(c)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Client agent ID required - provide in request body or context",
				})
			}
			req.ClientAgentID = clientAgentID
		}
	} else {
		// Internal format - use directly
		req.ExternalTaskID = sdkReq.ExternalTaskID
		req.ContextID = sdkReq.ContextID
		req.RemoteAgentID = sdkReq.RemoteAgentID
		req.SkillID = sdkReq.SkillID

		// Try clientAgentId as string first (SDK format), then UUID field
		if sdkReq.SdkClientAgent != "" {
			clientAgentID, err := uuid.Parse(sdkReq.SdkClientAgent)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid client agent ID",
				})
			}
			req.ClientAgentID = clientAgentID
		} else if sdkReq.ClientAgentID != uuid.Nil {
			req.ClientAgentID = sdkReq.ClientAgentID
		} else {
			// Fallback to context
			clientAgentID, err := h.getAgentIDFromContext(c)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Client agent ID required",
				})
			}
			req.ClientAgentID = clientAgentID
		}
	}

	// SECURITY (body-class #2): the ClientAgentID has been resolved
	// from one of three paths (SDK body, internal body, or Locals
	// context). All three are attacker-controlled in the body-supplied
	// cases — without this check, an attacker holding any valid
	// A2A token can plant a phantom A2ATask row claiming the VICTIM's
	// agent initiated a task against the attacker's RemoteAgentID,
	// polluting the victim org's analytics + peer-trust history.
	// RemoteAgentID is intentionally NOT scoped (A2A tasks are
	// cross-org by protocol design — the remote side may legitimately
	// belong to another organization).
	//
	// Filed as defect in
	// todo/2026-05-21-a3d-logtask-phantom-task-idor.md (P2). Fix uses
	// the existing agentService.GetAgent loader closure mirroring
	// GetSkillAttestations:1226 and UpdateTaskState (A3d-vii.b).
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	agentLoader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, agentLoader, req.ClientAgentID, orgID, agentOrgID) == nil {
		return nil
	}

	task, err := h.a2aService.LogA2ATask(c.Context(), req)
	if err != nil {
		// err.Error() is NOT echoed — could carry victim agent state
		// via wrapped service errors (mirrors A3d-vii.b PR #187 +
		// AttestMCP PR #188 no-echo discipline).
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to log task",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// UpdateTaskState updates the state of an A2A task
// PUT /api/v1/a2a/tasks/:id/state
// NOTE (A3d-vii): UpdateTaskState is intentionally NOT closed in
// A3d-vii.a. The path :id is a TASK UUID, not an agent UUID — the
// service does not take orgID and there is no current FK-via-agent
// resolution helper. Tracked in the audit doc as A3d-vii.c follow-up
// (LoadOwnedViaAgent on tasks once domain.A2ATask gets an AgentID
// accessor exposed at the handler layer).
func (h *A2AHandler) UpdateTaskState(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	taskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid task ID",
		})
	}

	var req struct {
		State        string `json:"state"`
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	state := domain.A2ATaskState(req.State)
	if !isValidTaskState(state) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid task state",
		})
	}

	// SECURITY (A3d-vii.b): tenant-scope by verifying the task's
	// ClientAgentID belongs to caller's org BEFORE mutating state.
	// Without this gate ANY authenticated A2A caller could forge a
	// completed/failed/cancelled transition on any tenant's task,
	// poisoning downstream analytics and trust scores in the victim
	// org. A2ATask has no direct OrganizationID; ownership flows
	// through the client-side agent. RemoteAgentID is intentionally
	// NOT scoped — A2A tasks are cross-org by protocol design.
	task, err := h.a2aReader.GetA2ATask(c.Context(), taskID)
	if err != nil || task == nil {
		respondResourceNotFound(c)
		return nil
	}
	agentLoader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, agentLoader, task.ClientAgentID, orgID, agentOrgID) == nil {
		return nil
	}

	if err := h.a2aService.UpdateA2ATaskState(c.Context(), taskID, state, req.ErrorCode, req.ErrorMessage); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"taskId":  taskID,
		"state":   state,
		"message": "Task state updated",
	})
}

// ============================================================================
// Skill Endpoints
// ============================================================================

// GetAgentSkills returns all A2A skills for an agent
// GET /api/v1/a2a/agents/:id/skills
func (h *A2AHandler) GetAgentSkills(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	skills, err := h.a2aService.GetAgentSkills(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"agentId": agentID,
		"skills":  skills,
		"count":   len(skills),
	})
}

// SearchSkills searches for A2A skills across all agents
// GET /api/v1/a2a/skills/search
func (h *A2AHandler) SearchSkills(c fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query parameter 'q' is required",
		})
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	skills, err := h.a2aService.SearchSkills(c.Context(), query, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"query":  query,
		"skills": skills,
		"count":  len(skills),
	})
}

// ============================================================================
// Intent-Based Routing (Discovery)
// ============================================================================

// RouteByIntent finds the best agent for a given intent using FTS
// GET /api/v1/a2a/route?intent=X
func (h *A2AHandler) RouteByIntent(c fiber.Ctx) error {
	intent := c.Query("intent")
	if intent == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query parameter 'intent' is required",
		})
	}

	// Parse optional min trust score (default 0.5)
	minTrustScore := 0.5
	if minTrustStr := c.Query("minTrustScore"); minTrustStr != "" {
		if score, err := strconv.ParseFloat(minTrustStr, 64); err == nil && score >= 0 && score <= 1 {
			minTrustScore = score
		}
	}

	resp, err := h.a2aService.RouteByIntent(c.Context(), intent, minTrustScore)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if resp.Agent == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":        "No agent found matching intent",
			"intent":       intent,
			"alternatives": 0,
		})
	}

	return c.JSON(resp)
}

// CapableOf finds agents capable of a given skill/intent (multiple results)
// GET /api/v1/a2a/capable-of?intent=X
func (h *A2AHandler) CapableOf(c fiber.Ctx) error {
	intent := c.Query("intent")
	if intent == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query parameter 'intent' is required",
		})
	}

	// Parse optional min trust score (default 0.5)
	minTrustScore := 0.5
	if minTrustStr := c.Query("minTrustScore"); minTrustStr != "" {
		if score, err := strconv.ParseFloat(minTrustStr, 64); err == nil && score >= 0 && score <= 1 {
			minTrustScore = score
		}
	}

	// Parse limit (default 10)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	agents, err := h.a2aService.CapableOf(c.Context(), intent, minTrustScore, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"intent": intent,
		"agents": agents,
		"count":  len(agents),
	})
}

// ============================================================================
// Consent Management Endpoints
// ============================================================================

// RecordConsent records user consent for cross-agent data sharing
// POST /api/v1/a2a/consent
func (h *A2AHandler) RecordConsent(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req application.RecordConsentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3c #42 pair): always stamp the caller's orgID onto the
	// consent record. Without this, the record lands with
	// organization_id = NULL and becomes invisible to the new
	// org-scoped ListUserConsents read path. Any body-supplied
	// organizationId is overridden — caller's authenticated org is the
	// only source of truth.
	req.OrganizationID = &orgID

	// Set IP and User-Agent from request
	req.IPAddress = c.IP()
	req.UserAgent = c.Get("User-Agent")

	consent, err := h.a2aService.RecordConsent(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"a2a_consent",
		consent.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"grantorAgentId":   req.GrantorAgentID,
			"recipientAgentId": req.RecipientAgentID,
			"scope":            req.Scope,
			"purpose":          req.Purpose,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(consent)
}

// CheckConsent checks if consent exists for a specific scope
// GET /api/v1/a2a/consent/check
func (h *A2AHandler) CheckConsent(c fiber.Ctx) error {
	userID := c.Query("userId")
	grantorID := c.Query("grantorAgentId")
	recipientID := c.Query("recipientAgentId")
	scope := c.Query("scope")

	if userID == "" || grantorID == "" || recipientID == "" || scope == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "userId, grantorAgentId, recipientAgentId, and scope are required",
		})
	}

	grantorUUID, err := uuid.Parse(grantorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid grantor agent ID",
		})
	}

	recipientUUID, err := uuid.Parse(recipientID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid recipient agent ID",
		})
	}

	hasConsent, err := h.a2aService.CheckConsent(c.Context(), userID, grantorUUID, recipientUUID, scope)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"hasConsent":       hasConsent,
		"userId":           userID,
		"grantorAgentId":   grantorID,
		"recipientAgentId": recipientID,
		"scope":            scope,
	})
}

// RevokeConsent revokes a consent record
// POST /api/v1/a2a/consent/:id/revoke
func (h *A2AHandler) RevokeConsent(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}

	consentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid consent ID",
		})
	}

	// SECURITY (A3d-vii.b): tenant-scope by verifying the consent record
	// belongs to caller's org BEFORE revoking. Without this gate ANY
	// authenticated A2A caller could revoke any tenant's consent by
	// guessing the UUID. A2AConsentRecord.OrganizationID is *uuid.UUID
	// (nullable); consentOrgID returns uuid.Nil for unassigned records,
	// which never matches a real caller's org → 404. The LoadOwned guard
	// also precedes the auditService.LogAction call so a forged revoke
	// attempt does NOT emit an audit row against the caller's org for a
	// victim's resource.
	consentLoader := func(id uuid.UUID) (*domain.A2AConsentRecord, error) {
		return h.a2aReader.GetConsent(c.Context(), id)
	}
	if LoadOwned(c, consentLoader, consentID, orgID, consentOrgID) == nil {
		return nil
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		req.Reason = "User revoked consent"
	}

	if err := h.a2aService.RevokeConsent(c.Context(), consentID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionDelete,
		"a2a_consent",
		consentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"reason": req.Reason,
		},
	)

	return c.JSON(fiber.Map{
		"consentId": consentID,
		"revoked":   true,
		"message":   "Consent revoked successfully",
	})
}

// ListUserConsents lists all consent records for a user
// GET /api/v1/a2a/consent/user/:userId
func (h *A2AHandler) ListUserConsents(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	userID := c.Params("userId")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	includeRevoked := c.Query("includeRevoked") == "true"

	// SECURITY (A3c #42): the service scopes the query by the caller's
	// orgID. Cross-tenant userId enumeration returns an empty list, which
	// is indistinguishable from "user has no consents in your org" — no
	// existence side channel is exposed.
	consents, err := h.a2aService.ListUserConsents(c.Context(), userID, orgID, includeRevoked)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"userId":   userID,
		"consents": consents,
		"count":    len(consents),
	})
}

// ListAllConsents lists the calling organization's consent records, paginated.
// Not an admin endpoint: the route carries no role middleware, so the org
// predicate below is the only tenant boundary this handler has.
func (h *A2AHandler) ListAllConsents(c fiber.Ctx) error {
	// SECURITY: this route returned every tenant's consent records — user_id,
	// purpose, data_types, both agent IDs and user_agent — to any
	// authenticated caller. It had no organization predicate and no role
	// middleware, unlike its neighbours on the same route group.
	//
	// Pagination now goes through the shared helper, which caps limit and
	// offset. The hand-rolled parsing it replaces accepted any positive limit,
	// so ?limit=100000 paged the whole table in one request.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	p := ParsePaginationWithDefaults(c, 100, 100)
	limit, offset := p.Limit, p.Offset

	consents, total, err := h.a2aService.ListAllConsents(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"consents": consents,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// ListAllTrustScores lists A2A trust scores for the calling organization's
// agents, paginated. Not an admin endpoint, for the same reason as
// ListAllConsents.
func (h *A2AHandler) ListAllTrustScores(c fiber.Ctx) error {
	// SECURITY: same defect as ListAllConsents — no organization predicate and
	// an uncapped limit. This one exposed every tenant's agent IDs and
	// behavioural metrics (task counts, response times, trust score).
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	p := ParsePaginationWithDefaults(c, 100, 100)
	limit, offset := p.Limit, p.Offset

	scores, total, err := h.a2aService.ListAllTrustScores(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"scores": scores,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ============================================================================
// Policy Evaluation Endpoint
// ============================================================================

// EvaluatePolicy evaluates A2A policies for a given action
// POST /api/v1/a2a/policies/evaluate
func (h *A2AHandler) EvaluatePolicy(c fiber.Ctx) error {
	var req application.EvaluateA2APolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	decision, err := h.a2aService.EvaluateA2APolicy(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(decision)
}

// ============================================================================
// Public Agent Card Endpoint (Well-Known)
// ============================================================================

// GetPublicAgentCard returns the public A2A agent card (for discovery)
// GET /.well-known/agent.json
func (h *A2AHandler) GetPublicAgentCard(c fiber.Ctx) error {
	// Get agent ID from query or header
	agentID := c.Query("agentId")
	if agentID == "" {
		agentID = c.Get("X-Agent-ID")
	}
	if agentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Agent ID is required (query param 'agentId' or header 'X-Agent-ID')",
		})
	}

	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	card, err := h.a2aService.GetEnhancedAgentCard(c.Context(), agentUUID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent card not found",
		})
	}

	// Set Content-Type for A2A discovery
	c.Set("Content-Type", "application/json")

	// Return the card as raw JSON for A2A protocol compliance
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to serialize agent card",
		})
	}

	return c.Send(cardJSON)
}

// ============================================================================
// Maintenance Endpoints
// ============================================================================

// CleanupExpiredNonces removes expired nonces (admin only)
// POST /api/v1/a2a/maintenance/cleanup-nonces
func (h *A2AHandler) CleanupExpiredNonces(c fiber.Ctx) error {
	count, err := h.a2aService.CleanupExpiredNonces(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":      "Nonces cleaned up",
		"deletedCount": count,
		"timestamp":    time.Now().UTC(),
	})
}

// RefreshExpiredCards refreshes cards with expired attestations (admin only)
// POST /api/v1/a2a/maintenance/refresh-cards
func (h *A2AHandler) RefreshExpiredCards(c fiber.Ctx) error {
	count, err := h.a2aService.RefreshExpiredCards(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":        "Cards refreshed",
		"refreshedCount": count,
		"timestamp":      time.Now().UTC(),
	})
}

// ============================================================================
// Skill Registration
// ============================================================================

// RegisterSkill registers a new skill for an agent
// POST /api/v1/a2a/skills
func (h *A2AHandler) RegisterSkill(c fiber.Ctx) error {
	var req struct {
		AgentID     string   `json:"agentId"`
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		InputModes  []string `json:"inputModes"`
		OutputModes []string `json:"outputModes"`
		Examples    []string `json:"examples"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		// Try to get agent ID from authenticated user
		agentID, err = h.getAgentIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid or missing agent ID",
			})
		}
	}

	skill := &domain.A2ASkill{
		AgentID:     agentID,
		SkillID:     req.ID,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		InputModes:  req.InputModes,
		OutputModes: req.OutputModes,
		Examples:    req.Examples,
	}

	if err := h.a2aService.RegisterSkill(c.Context(), skill); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(skill)
}

// ============================================================================
// Attestation Operations
// ============================================================================

// AttestSkill creates an attestation for another agent's skill
// POST /api/v1/a2a/attestations
func (h *A2AHandler) AttestSkill(c fiber.Ctx) error {
	var req struct {
		AttestingAgentID string                 `json:"attestingAgentId"` // SDK sends this
		AttestedAgentID  string                 `json:"attestedAgentId"`
		SkillID          string                 `json:"skillId"`
		AttestationType  string                 `json:"attestationType"`
		Confidence       float64                `json:"confidence"`
		Evidence         map[string]interface{} `json:"evidence"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	attestedAgentID, err := uuid.Parse(req.AttestedAgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid attested agent ID",
		})
	}

	// Get attesting agent - try request body first, then context
	var attestingAgentID uuid.UUID
	if req.AttestingAgentID != "" {
		attestingAgentID, err = uuid.Parse(req.AttestingAgentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid attesting agent ID",
			})
		}
	} else {
		// Fallback to context
		attestingAgentID, err = h.getAgentIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Could not determine attesting agent - provide attestingAgentId in request",
			})
		}
	}

	attestReq := application.AttestSkillRequest{
		AttestingAgentID: attestingAgentID,
		AttestedAgentID:  attestedAgentID,
		SkillID:          req.SkillID,
		AttestationType:  req.AttestationType,
		Confidence:       req.Confidence,
		Evidence:         req.Evidence,
	}

	attestation, err := h.a2aService.AttestSkill(c.Context(), attestReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(attestation)
}

// GetConsensusStatus returns the consensus verification status for a skill
// GET /api/v1/a2a/consensus/:agentId/:skillId
// GET /api/v1/a2a/agents/:id/skills/:skillId/consensus (SDK compatibility)
func (h *A2AHandler) GetConsensusStatus(c fiber.Ctx) error {
	// Try both parameter naming conventions
	agentIDStr := c.Params("agentId")
	if agentIDStr == "" {
		agentIDStr = c.Params("id") // SDK path uses :id
	}
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	skillID := c.Params("skillId")
	if skillID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID is required",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. Bound on both
	// `/consensus/:agentId/:skillId` and `/agents/:id/skills/:skillId/consensus`
	// — both route forms identify the SAME agent UUID via the fallback
	// parameter lookup above. SkillID is a non-UUID identifier and is
	// not scoped here; cross-tenant skill enumeration via a same-org
	// agent is bounded by what skills that agent has, which is already
	// authorized.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	result, err := h.a2aService.GetConsensusStatus(c.Context(), agentID, skillID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// GetSkillAttestations returns all attestations for an agent's skill
// GET /api/v1/a2a/attestations/:agentId/:skillId
func (h *A2AHandler) GetSkillAttestations(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	agentID, err := uuid.Parse(c.Params("agentId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	skillID := c.Params("skillId")

	// SECURITY (A3c #43): tenant-scope by verifying the agent belongs to
	// caller's org BEFORE reading attestations. Returns 404 for
	// cross-tenant access to preserve existence-secrecy (see
	// tenant_scope.go:41-46).
	loader := func(id uuid.UUID) (*domain.Agent, error) {
		return h.agentService.GetAgent(c.Context(), id)
	}
	if LoadOwned(c, loader, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	attestations, err := h.a2aService.GetAgentAttestations(c.Context(), agentID, skillID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(attestations)
}

// GetAgentAttestations returns all attestations for an agent (SDK-compatible path)
// GET /api/v1/a2a/agents/:id/attestations
func (h *A2AHandler) GetAgentAttestations(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. Sibling of
	// GetSkillAttestations (A3c #43, already merged) on the SDK route
	// `/a2a/agents/:id/attestations`.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	skillID := c.Query("skillId")

	attestations, err := h.a2aService.GetAgentAttestations(c.Context(), agentID, skillID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"attestations": attestations,
	})
}

// ============================================================================
// Security Check Operations
// ============================================================================

// CheckSecurity evaluates security policies for an A2A request
// POST /api/v1/a2a/security/check
func (h *A2AHandler) CheckSecurity(c fiber.Ctx) error {
	var req struct {
		RequestingAgentID string `json:"requestingAgentId"`
		TargetAgentID     string `json:"targetAgentId"`
		SkillID           string `json:"skillId"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	requestingID, err := uuid.Parse(req.RequestingAgentID)
	if err != nil {
		// Try to get from context
		requestingID, err = h.getAgentIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid requesting agent ID",
			})
		}
	}

	targetID, err := uuid.Parse(req.TargetAgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid target agent ID",
		})
	}

	checkReq := &domain.A2ASecurityCheckRequest{
		RequestingAgentID: requestingID,
		TargetAgentID:     targetID,
		SkillID:           req.SkillID,
	}

	result, err := h.a2aService.CheckA2ASecurity(c.Context(), checkReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// GetSecuritySettings returns the A2A security settings for the organization
// GET /api/v1/a2a/security/settings
func (h *A2AHandler) GetSecuritySettings(c fiber.Ctx) error {
	orgID, err := h.getOrgIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Could not determine organization",
		})
	}

	settings, err := h.a2aService.GetSecuritySettings(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(settings)
}

// UpdateSecuritySettings updates the A2A security settings for the organization
// PUT /api/v1/a2a/security/settings
func (h *A2AHandler) UpdateSecuritySettings(c fiber.Ctx) error {
	var settings domain.A2ASecuritySettings

	if err := c.Bind().JSON(&settings); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	orgID, err := h.getOrgIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Could not determine organization",
		})
	}

	settings.OrganizationID = orgID

	if err := h.a2aService.UpdateSecuritySettings(c.Context(), &settings); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(settings)
}

// ============================================================================
// Alternative SDK-Compatible Routes
// ============================================================================

// SignRequestAlt handles signing without agent ID in path (SDK compatibility)
// POST /api/v1/a2a/sign
func (h *A2AHandler) SignRequestAlt(c fiber.Ctx) error {
	var req struct {
		AgentID string `json:"agentId"`
		Method  string `json:"method"`
		Path    string `json:"path"`
		Body    string `json:"body"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		agentID, err = h.getAgentIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid or missing agent ID",
			})
		}
	}

	signature, err := h.a2aService.SignA2ARequest(c.Context(), agentID, req.Method, req.Path, []byte(req.Body))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(signature)
}

// GetTrustScoreAlt handles trust score with agent ID in path (SDK compatibility)
// GET /api/v1/a2a/trust/:id
// Auto-computes trust score if none exists or score is zero
func (h *A2AHandler) GetTrustScoreAlt(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. SDK alt route
	// `/a2a/trust/:id` — same semantics as GetA2ATrustScore above.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	// First try to get existing trust score
	score, err := h.a2aService.GetA2ATrustScore(c.Context(), agentID)
	if err != nil || score == nil || score.A2ATrustScore == nil || *score.A2ATrustScore == 0 {
		// Auto-compute if no score exists or score is zero
		computedScore, computeErr := h.a2aService.ComputeA2ATrustScore(c.Context(), agentID)
		if computeErr != nil {
			// If computation fails, return zero score with message
			return c.JSON(fiber.Map{
				"agentId": agentID,
				"score":   0.0,
				"message": "Could not compute trust score: " + computeErr.Error(),
			})
		}
		return c.JSON(computedScore)
	}

	return c.JSON(score)
}

// RouteByIntentPost handles intent routing via POST (SDK compatibility)
// POST /api/v1/a2a/discovery/route
func (h *A2AHandler) RouteByIntentPost(c fiber.Ctx) error {
	var req struct {
		Intent        string  `json:"intent"`
		MinTrustScore float64 `json:"minTrustScore"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Intent == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intent is required",
		})
	}

	result, err := h.a2aService.RouteByIntent(c.Context(), req.Intent, req.MinTrustScore)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// ============================================================================
// Helper Functions
// ============================================================================

func (h *A2AHandler) getAgentIDFromContext(c fiber.Ctx) (uuid.UUID, error) {
	// Try to get from locals (set by auth middleware) - try both naming conventions
	if agentID, ok := c.Locals("agent_id").(uuid.UUID); ok {
		return agentID, nil
	}
	if agentIDStr, ok := c.Locals("agent_id").(string); ok {
		return uuid.Parse(agentIDStr)
	}
	if agentID, ok := c.Locals("agentId").(uuid.UUID); ok {
		return agentID, nil
	}
	if agentIDStr, ok := c.Locals("agentId").(string); ok {
		return uuid.Parse(agentIDStr)
	}
	return uuid.UUID{}, fmt.Errorf("agent ID not found in context")
}

func (h *A2AHandler) getOrgIDFromContext(c fiber.Ctx) (uuid.UUID, error) {
	// Try to get from locals (set by auth middleware) - try both naming conventions
	if orgID, ok := c.Locals("organization_id").(uuid.UUID); ok {
		return orgID, nil
	}
	if orgIDStr, ok := c.Locals("organization_id").(string); ok {
		return uuid.Parse(orgIDStr)
	}
	if orgID, ok := c.Locals("organizationId").(uuid.UUID); ok {
		return orgID, nil
	}
	if orgIDStr, ok := c.Locals("organizationId").(string); ok {
		return uuid.Parse(orgIDStr)
	}
	return uuid.UUID{}, fmt.Errorf("organization ID not found in context")
}

func isValidTaskState(state domain.A2ATaskState) bool {
	switch state {
	case domain.A2ATaskStateSubmitted,
		domain.A2ATaskStateWorking,
		domain.A2ATaskStateInputNeeded,
		domain.A2ATaskStateCompleted,
		domain.A2ATaskStateFailed,
		domain.A2ATaskStateCancelled:
		return true
	}
	return false
}

// ============================================================================
// SDK Compatibility Handlers
// ============================================================================

// UpdateTrustScore updates the trust score for an agent
// PUT /api/v1/a2a/trust/:id
func (h *A2AHandler) UpdateTrustScore(c fiber.Ctx) error {
	targetAgentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		Score      float64 `json:"score"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent AFTER body
	// validation (preserves 400-shape contract for _InvalidJSON tests).
	// Trust-score writes have integrity impact across the trust graph;
	// LoadOwned confines the write to agents in caller's org.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, targetAgentID, orgID) == nil {
		return nil
	}

	// Compute and return the updated trust score
	score, err := h.a2aService.ComputeA2ATrustScore(c.Context(), targetAgentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update trust score: " + err.Error(),
		})
	}

	return c.JSON(score)
}

// RecordInteraction records an interaction (success or failure) with a peer agent
// POST /api/v1/a2a/trust/:id/interaction
func (h *A2AHandler) RecordInteraction(c fiber.Ctx) error {
	targetAgentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	agentID, err := h.getAgentIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent ID required",
		})
	}

	var req struct {
		TaskType    string `json:"taskType"`
		Success     bool   `json:"success"`
		DurationMs  int64  `json:"durationMs"`
		ErrorReason string `json:"errorReason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the PATH target agent.
	// Placed AFTER body validation per the RecordMCPConnection
	// precedent so existing `_InvalidJSON` tests retain their
	// 400-shape contract. Note this is the opposite role from
	// GetPeerTrustScore (where path :id is the caller-side agent and
	// peer_id is the cross-org peer): here path :id IS the target
	// peer, so scoping it confines the recompute side effect
	// (`ComputeA2ATrustScore(targetAgentID)` at line ~1635) to agents
	// in caller's own org. Without this gate, any authenticated A2A
	// caller could trigger trust-score recomputation across any org
	// by submitting fake interactions against a guessed target UUID.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, targetAgentID, orgID) == nil {
		return nil
	}

	// Update peer trust based on interaction
	peerTrust, err := h.a2aService.GetPeerTrustScore(c.Context(), agentID, targetAgentID)
	if err != nil {
		// Create new peer trust relationship if doesn't exist
		peerTrust = &domain.A2APeerTrust{
			AgentID:     agentID,
			PeerAgentID: targetAgentID,
		}
	}

	// Adjust trust based on success/failure
	currentTrust := 0.5
	if peerTrust.PeerTrustScore != nil {
		currentTrust = *peerTrust.PeerTrustScore
	}

	if req.Success {
		currentTrust = min(1.0, currentTrust+0.05)
	} else {
		currentTrust = max(0.0, currentTrust-0.1)
	}

	peerTrust.PeerTrustScore = &currentTrust

	// Return the computed trust score
	score, err := h.a2aService.ComputeA2ATrustScore(c.Context(), targetAgentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to compute trust score",
		})
	}

	return c.JSON(score)
}

// CapableOfPost handles POST requests for capability discovery (Java SDK compatibility)
// POST /api/v1/a2a/discovery/capable
func (h *A2AHandler) CapableOfPost(c fiber.Ctx) error {
	var req struct {
		SkillIDs      []string `json:"skillIds"`
		MinTrustScore float64  `json:"minTrustScore"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Use skill IDs as intent for search
	intent := ""
	if len(req.SkillIDs) > 0 {
		intent = req.SkillIDs[0] // Use first skill as intent
	}

	agents, err := h.a2aService.CapableOf(c.Context(), intent, req.MinTrustScore, 20)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find capable agents",
		})
	}

	return c.JSON(fiber.Map{
		"agents": agents,
	})
}

// ListAgentCards returns a list of registered agent cards
// GET /api/v1/a2a/cards
func (h *A2AHandler) ListAgentCards(c fiber.Ctx) error {
	limit := 100
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	cards, err := h.a2aService.ListAgentCards(c.Context(), limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"cards":  cards,
		"limit":  limit,
		"offset": offset,
	})
}

// RegisterAgentCardAlt handles POST /cards for Java SDK compatibility
// POST /api/v1/a2a/cards
func (h *A2AHandler) RegisterAgentCardAlt(c fiber.Ctx) error {
	var req struct {
		AgentID string `json:"agentId"`
		CardURL string `json:"cardUrl"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Get agent ID from request body or context
	var agentID uuid.UUID
	var err error
	if req.AgentID != "" {
		agentID, err = uuid.Parse(req.AgentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agent ID",
			})
		}
	} else {
		agentID, err = h.getAgentIDFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Agent ID required",
			})
		}
	}

	// Create the agent card using the service
	card, err := h.a2aService.RegisterAgentCard(c.Context(), application.RegisterAgentCardRequest{
		AgentID: agentID,
		CardURL: req.CardURL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to register agent card: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(card)
}

// UpdateAgentCard updates an existing agent card
// PUT /api/v1/a2a/cards/:id
func (h *A2AHandler) UpdateAgentCard(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Version     string   `json:"version"`
		Endpoint    string   `json:"endpoint"`
		Skills      []string `json:"skills"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// SECURITY (A3d-vii.a): tenant-scope the path agent. SDK alt
	// route `PUT /a2a/cards/:id`. Placed AFTER body validation.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if h.loadOwnedAgent(c, agentID, orgID) == nil {
		return nil
	}

	// Get existing card
	card, err := h.a2aService.GetEnhancedAgentCard(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent card not found",
		})
	}

	// Return the existing card (updates would require service method)
	return c.JSON(card)
}

// ListPeerTrusts returns all peer trust relationships for the authenticated agent
// GET /api/v1/a2a/peers
func (h *A2AHandler) ListPeerTrusts(c fiber.Ctx) error {
	agentID, err := h.getAgentIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Agent ID required",
		})
	}

	peers, err := h.a2aService.ListPeerTrusts(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if peers == nil {
		peers = make([]*domain.A2APeerTrust, 0)
	}

	return c.JSON(fiber.Map{
		"peers":   peers,
		"agentId": agentID,
	})
}

// DeleteSkill deletes a registered skill
// DELETE /api/v1/a2a/skills/:id
func (h *A2AHandler) DeleteSkill(c fiber.Ctx) error {
	skillID := c.Params("id")
	if skillID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID required",
		})
	}

	// For now, return success (proper implementation needs DeleteSkill service method)
	return c.SendStatus(fiber.StatusNoContent)
}

// GetSecurityViolations returns security policy violations
// GET /api/v1/a2a/security/violations
func (h *A2AHandler) GetSecurityViolations(c fiber.Ctx) error {
	orgID, err := h.getOrgIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID required",
		})
	}

	limit := 50
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	violations, err := h.a2aService.GetSecurityViolations(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get security violations",
		})
	}

	return c.JSON(violations)
}
