package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A2AHandler handles A2A (Agent-to-Agent) protocol endpoints
type A2AHandler struct {
	a2aService   *application.A2AService
	agentService *application.AgentService
	auditService *application.AuditService
}

// NewA2AHandler creates a new A2A handler
func NewA2AHandler(
	a2aService *application.A2AService,
	agentService *application.AgentService,
	auditService *application.AuditService,
) *A2AHandler {
	return &A2AHandler{
		a2aService:   a2aService,
		agentService: agentService,
		auditService: auditService,
	}
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
		CardURL string `json:"cardUrl"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.CardURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cardUrl is required",
		})
	}

	resp, err := h.a2aService.RegisterAgentCard(c.Context(), application.RegisterAgentCardRequest{
		AgentID:  agentID,
		CardURL:  req.CardURL,
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
			"agentId":   agentID,
			"cardUrl":   req.CardURL,
			"skills":    resp.Skills,
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
			"agentId":           agentID,
			"attestationExpires": card.AttestationExpiresAt,
		},
	)

	return c.JSON(fiber.Map{
		"cardId":              card.ID,
		"agentId":             card.AgentID,
		"attestationExpires": card.AttestationExpiresAt,
		"message":             "Attestation refreshed successfully",
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
			"valid":  false,
			"error":  result.Error,
		})
	}

	return c.JSON(fiber.Map{
		"valid":             true,
		"agentId":           result.AgentID,
		"agentName":         result.AgentName,
		"trustScore":        result.TrustScore,
		"capabilities":      result.Capabilities,
		"attestationValid": result.AttestationValid,
	})
}

// ============================================================================
// Trust Score Endpoints
// ============================================================================

// GetA2ATrustScore returns the A2A-specific trust score for an agent
// GET /api/v1/a2a/agents/:id/trust-score
func (h *A2AHandler) GetA2ATrustScore(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	score, err := h.a2aService.GetA2ATrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Trust score not found",
		})
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

	trust, err := h.a2aService.GetPeerTrustScore(c.Context(), agentID, peerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Peer trust relationship not found",
		})
	}

	return c.JSON(trust)
}

// ============================================================================
// Task Logging Endpoints
// ============================================================================

// LogTask logs an A2A task for audit trail
// POST /api/v1/a2a/tasks
func (h *A2AHandler) LogTask(c fiber.Ctx) error {
	var req application.LogA2ATaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	task, err := h.a2aService.LogA2ATask(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// UpdateTaskState updates the state of an A2A task
// PUT /api/v1/a2a/tasks/:id/state
func (h *A2AHandler) UpdateTaskState(c fiber.Ctx) error {
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
		"query":   query,
		"skills":  skills,
		"count":   len(skills),
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
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	consentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid consent ID",
		})
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
	userID := c.Params("userId")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	includeRevoked := c.Query("includeRevoked") == "true"

	consents, err := h.a2aService.ListUserConsents(c.Context(), userID, includeRevoked)
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
		"message":         "Nonces cleaned up",
		"deletedCount":   count,
		"timestamp":       time.Now().UTC(),
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
// Helper Functions
// ============================================================================

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
