package handlers

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// mcpServerByIDLookup is the single-method subset of any MCP server
// repository this handler needs. Both domain.MCPServerRepository (the
// production type) and a test-side stub satisfy it via Go structural
// typing — matching the agentByIDLookup pattern from capability_handler.go.
type mcpServerByIDLookup interface {
	GetByID(id uuid.UUID) (*domain.MCPServer, error)
}

type MCPAttestationHandler struct {
	attestationService *application.MCPAttestationService
	verifier           MCPAttestationVerifier
	auditService       *application.AuditService
	agentRepo          agentByIDLookup
	mcpServerRepo      mcpServerByIDLookup
}

func NewMCPAttestationHandler(
	attestationService *application.MCPAttestationService,
	auditService *application.AuditService,
	agentRepo domain.AgentRepository,
	mcpServerRepo domain.MCPServerRepository,
) *MCPAttestationHandler {
	return &MCPAttestationHandler{
		attestationService: attestationService,
		verifier:           attestationService,
		auditService:       auditService,
		agentRepo:          agentRepo,
		mcpServerRepo:      mcpServerRepo,
	}
}

// getVerifier returns the injected verifier (set by NewMCPAttestationHandler
// to attestationService, or overwritten in tests).
func (h *MCPAttestationHandler) getVerifier() MCPAttestationVerifier {
	if h.verifier != nil {
		return h.verifier
	}
	return h.attestationService
}

// GetAttestationChallenge generates a server-side challenge for proof of private key possession
// @Summary Get attestation challenge
// @Description Request a challenge nonce that must be signed and included in the attestation
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param agent_id query string true "Agent ID requesting the challenge"
// @Success 200 {object} application.ChallengeResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/challenge [get]
func (h *MCPAttestationHandler) GetAttestationChallenge(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Parse agent ID from query param
	agentIDStr := c.Query("agent_id")
	if agentIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_id query parameter is required",
		})
	}

	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope BOTH the path MCP server and the
	// query-supplied agent before calling the service. The service has
	// distinct error strings ("agent not found" vs "mcp server not found")
	// which would otherwise function as a cross-tenant existence oracle for
	// either resource. Collapsing to 404 + "not found" via LoadOwned
	// removes the side channel. The query-agent check also closes the
	// class-#1 lint blindspot (c.Query() tenant UUID).
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Generate challenge
	response, err := h.attestationService.GenerateChallenge(c.Context(), agentID, mcpServerID)
	if err != nil {
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "agent not found" || err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		} else if err.Error() == "only verified agents can request attestation challenges" {
			statusCode = fiber.StatusForbidden
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Failed to generate challenge",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// AttestMCP handles agent attestation of an MCP server
// @Summary Attest MCP server
// @Description Submit cryptographically signed attestation from a verified agent
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body application.AttestMCPRequest true "Attestation data and signature"
// @Success 200 {object} application.AttestMCPResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/attest [post]
func (h *MCPAttestationHandler) AttestMCP(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	var req application.AttestMCPRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path MCP server before the
	// service runs the cryptographic signature check. Without this,
	// "mcp server not found" from the service is observably distinct
	// from "invalid attestation signature", giving any authenticated
	// agent a cross-tenant existence oracle on MCP UUIDs. The body's
	// AgentID is NOT scoped here — the signature check against the
	// agent's pubkey is the trust boundary, and body-IDOR via AgentID
	// is tracked as out-of-scope class-#2 (lint blindspot) follow-up.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}

	// Verify and record attestation
	// SECURITY: error.Error() is NOT echoed back to the client. The
	// service collapses existence-or-ownership failures to
	// ErrAttestationFailed and signature/payload failures to
	// ErrAttestationInvalid; both map to the SAME fixed-body 403 so an
	// attacker holding a valid Ed25519 token cannot distinguish "I'm
	// targeting a victim agent in another org" from "my signature is
	// bad." Other errors (DB failure, etc.) surface as 500 with a fixed
	// message — never the wrapped error text, which can carry victim
	// agent status / trust score / UUID.
	response, err := h.getVerifier().VerifyAndRecordAttestation(c.Context(), mcpServerID, &req)
	if err != nil {
		if errors.Is(err, application.ErrAttestationFailed) || errors.Is(err, application.ErrAttestationInvalid) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Attestation failed",
				"message": "Attestation could not be verified.",
			})
		}
		// Unexpected error path (DB outage, marshaling bug, etc.).
		// Generic 500 — no error string surfaced to the client.
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Attestation failed",
			"message": "Internal error processing attestation.",
		})
	}

	// Audit log - support both user auth (JWT) and agent auth (Ed25519)
	userIDLocal := c.Locals("user_id")
	agentIDLocal := c.Locals("agent_id")
	orgIDLocal := c.Locals("organization_id")

	// Get org ID (set by both JWT and Ed25519 middleware)
	var actorOrgID uuid.UUID
	if orgIDLocal != nil {
		actorOrgID = orgIDLocal.(uuid.UUID)
	}

	// Always log attestation events - this is a critical security action
	// Use appropriate method based on whether this is a user or agent action
	if userIDLocal != nil {
		// User-initiated action (JWT auth)
		h.auditService.LogAction(
			c.Context(),
			actorOrgID,
			userIDLocal.(uuid.UUID),
			domain.AuditActionAttest,
			"mcp_server",
			mcpServerID,
			c.IP(),
			c.Get("User-Agent"),
			fiber.Map{
				"attestation_id":        response.AttestationID,
				"confidence_score":      response.MCPConfidenceScore,
				"attestation_count":     response.AttestationCount,
				"agentId":               req.Attestation.AgentID,
				"capabilities_found":    req.Attestation.CapabilitiesFound,
				"connection_latency_ms": req.Attestation.ConnectionLatencyMs,
				"auth_method":           "jwt",
			},
		)
	} else {
		// Agent-initiated action (Ed25519 SDK auth)
		var agentID uuid.UUID
		if agentIDLocal != nil {
			agentID = agentIDLocal.(uuid.UUID)
		} else if parsedID, err := uuid.Parse(req.Attestation.AgentID); err == nil {
			agentID = parsedID
		}

		h.auditService.LogAgentAction(
			c.Context(),
			actorOrgID,
			agentID,
			domain.AuditActionAttest,
			"mcp_server",
			mcpServerID,
			c.IP(),
			c.Get("User-Agent"),
			fiber.Map{
				"attestation_id":        response.AttestationID,
				"confidence_score":      response.MCPConfidenceScore,
				"attestation_count":     response.AttestationCount,
				"agentId":               req.Attestation.AgentID,
				"capabilities_found":    req.Attestation.CapabilitiesFound,
				"connection_latency_ms": req.Attestation.ConnectionLatencyMs,
				"auth_method":           "ed25519",
			},
		)
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetMCPAttestations retrieves all attestations for an MCP server
// @Summary Get MCP attestations
// @Description Retrieve all agent attestations for an MCP server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/attestations [get]
func (h *MCPAttestationHandler) GetMCPAttestations(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path MCP server. The service
	// returns "mcp server not found" only when the row is absent — for a
	// row in another org, it would return the attestation list (an
	// IDOR + existence leak). LoadOwned collapses both to 404.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}

	// Get attestations
	attestations, confidenceScore, lastAttestedAt, err := h.attestationService.GetMCPAttestations(c.Context(), mcpServerID)
	if err != nil {
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Failed to get attestations",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"attestations":       attestations,
		"total":              len(attestations),
		"confidence_score":   confidenceScore,
		"last_attested_at":   lastAttestedAt,
	})
}

// GetConnectedAgents retrieves all agents connected to an MCP server
// @Summary Get connected agents
// @Description Retrieve all agents that have connections to this MCP server
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/agents [get]
func (h *MCPAttestationHandler) GetConnectedAgents(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path MCP server. This handler
	// is not currently bound to any route (the live /mcp-servers/:id/agents
	// route uses MCPHandler.GetMCPServerAgents), but the handler is
	// reachable as a Go symbol and the tenant-scope lint correctly flags
	// it. LoadOwned here is defense-in-depth in case the route is
	// re-wired in a future change.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}

	// Get connected agents
	agents, err := h.attestationService.GetConnectedAgentsForMCP(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get connected agents",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"agents": agents,
		"total":  len(agents),
	})
}

// GetAgentMCPServers retrieves all MCP servers connected to an agent
// @Summary Get agent MCP servers
// @Description Retrieve all MCP servers that an agent is connected to
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/agents/{id}/mcp-servers [get]
func (h *MCPAttestationHandler) GetAgentMCPServers(c fiber.Ctx) error {
	// Parse agent ID from URL
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path agent. This handler is not
	// currently bound to any route (the live /agents/:id/mcp-servers route
	// uses AgentHandler.GetAgentMCPServers), but the symbol is still
	// reachable and the lint flags it. LoadOwned here is defense-in-depth
	// for any future rewire.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Get MCP servers
	mcpServers, err := h.attestationService.GetMCPServersForAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get MCP servers",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"mcpServers": mcpServers,
		"total":       len(mcpServers),
	})
}

// ManualAttestMCP handles manual attestation by a user (non-SDK, JWT-based)
// @Summary Manually attest MCP server
// @Description Submit manual attestation from a logged-in user for an MCP server they've verified
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param request body ManualAttestationRequest true "Manual attestation details"
// @Success 200 {object} application.AttestMCPResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id/manual-attest [post]
func (h *MCPAttestationHandler) ManualAttestMCP(c fiber.Ctx) error {
	// Get authenticated user ID from JWT middleware
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get organization ID
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type ManualAttestationRequest struct {
		Notes                string   `json:"notes"`                 // Optional notes from user
		CapabilitiesVerified []string `json:"capabilitiesVerified"`  // Capabilities user verified
		ConnectionTested     bool     `json:"connectionTested"`      // Did user test connection?
		HealthCheckPassed    bool     `json:"healthCheckPassed"`     // Did health check pass?
	}

	var req ManualAttestationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path MCP server AFTER body
	// validation. Placed here (not before the JSON parse) so that
	// malformed-input tests retain their 400-shape contract; the
	// security boundary is the service call below, not the JSON parse.
	// The underlying service does take orgID and scopes correctly, but
	// the service's "mcp server not found" error string is observably
	// distinct from other failure modes — this handler-layer LoadOwned
	// collapses any cross-tenant attempt to the same 404 + "not found"
	// body, matching the existence-secrecy guarantee used elsewhere.
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}

	// Call service method for manual attestation
	response, err := h.attestationService.RecordManualAttestation(
		c.Context(),
		mcpServerID,
		userID,
		orgID,
		req.CapabilitiesVerified,
		req.ConnectionTested,
		req.HealthCheckPassed,
		req.Notes,
	)
	// SECURITY: No error logging to prevent information leakage
	if err != nil {
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Manual attestation failed",
			"message": err.Error(),
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionAttest,
		"mcp_server",
		mcpServerID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"attestation_id":      response.AttestationID,
			"confidence_score":    response.MCPConfidenceScore,
			"attestation_count":   response.AttestationCount,
			"attestation_type":    "manual",
			"capabilities_verified": req.CapabilitiesVerified,
			"connection_tested":   req.ConnectionTested,
		},
	)

	return c.Status(fiber.StatusOK).JSON(response)
}

// RevokeAttestation revokes a specific attestation (supply chain security - incident response)
// @Summary Revoke attestation
// @Description Revoke a specific attestation when an agent's key is compromised or attestation is invalid
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param attestation_id path string true "Attestation ID"
// @Param request body RevokeAttestationRequest true "Revocation reason"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/attestations/{attestation_id}/revoke [post]
func (h *MCPAttestationHandler) RevokeAttestation(c fiber.Ctx) error {
	// Get authenticated user ID from JWT middleware
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get organization ID
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	// Parse attestation ID from URL
	attestationID, err := uuid.Parse(c.Params("attestation_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid attestation ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type RevokeAttestationRequest struct {
		Reason string `json:"reason"` // Reason for revocation
	}

	var req RevokeAttestationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Revocation reason is required",
		})
	}

	// Revoke the attestation. The service enforces tenant-scoping (A3c
	// #47) by collapsing "not found" and "cross-tenant" into a single
	// sentinel error; map both to 404 with the standard not-found body
	// shape to preserve existence-secrecy.
	err = h.attestationService.RevokeAttestation(c.Context(), attestationID, orgID, userID, req.Reason)
	if err != nil {
		if errors.Is(err, application.ErrAttestationNotFound) {
			respondResourceNotFound(c)
			return nil
		}
		// Server-side log preserves the underlying cause without
		// leaking it to the client.
		fmt.Printf("⚠️  RevokeAttestation failed: attestationID=%s callerOrgID=%s err=%v\n",
			attestationID, orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke attestation",
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionRevoke,
		"attestation",
		attestationID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"attestation_id": attestationID,
			"reason":         req.Reason,
		},
	)

	// SECURITY: No logging of revocation events to prevent information leakage

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":        true,
		"attestationId": attestationID,
		"message":        "Attestation revoked successfully",
		"reason":         req.Reason,
	})
}

// RevokeAllAttestationsByAgent revokes all attestations made by a specific agent
// @Summary Revoke all attestations by agent
// @Description Revoke all attestations made by a specific agent (use when agent key is compromised)
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body RevokeAllAttestationsRequest true "Revocation reason"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/agents/{agent_id}/attestations/revoke-all [post]
func (h *MCPAttestationHandler) RevokeAllAttestationsByAgent(c fiber.Ctx) error {
	// Get authenticated user ID from JWT middleware
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get organization ID
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	// Parse agent ID from URL
	agentID, err := uuid.Parse(c.Params("agent_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type RevokeAllAttestationsRequest struct {
		Reason string `json:"reason"` // Reason for revocation (e.g., "key compromised", "agent decommissioned")
	}

	var req RevokeAllAttestationsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Revocation reason is required",
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path agent. Placed AFTER body
	// validation so that malformed-input tests retain their 400-shape
	// contract; the security boundary is the service call below. The
	// underlying service does NOT take an orgID parameter today —
	// without this LoadOwned, ANY authenticated Manager in ANY org can
	// revoke ALL attestations made by ANY agent in ANY org (destructive
	// cross-tenant blast radius: invalidates the agent's attestation
	// records and recomputes MCP confidence scores for every MCP the
	// victim agent has attested). Handler-layer LoadOwned confines the
	// revoke to agents in the caller's own org.
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Revoke all attestations by this agent
	revokedCount, err := h.attestationService.RevokeAllAttestationsByAgent(c.Context(), agentID, userID, req.Reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to revoke attestations",
			"message": err.Error(),
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionRevoke,
		"agent_attestations",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"agentId":       agentID,
			"revokedCount": revokedCount,
			"reason":        req.Reason,
		},
	)

	// SECURITY: No logging of revocation events to prevent information leakage

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"agentId":      agentID,
		"revokedCount": revokedCount,
		"message":       fmt.Sprintf("Successfully revoked %d attestations", revokedCount),
		"reason":        req.Reason,
	})
}

// RecordMCPConnection handles agent recording MCP tool usage
// @Summary Record MCP connection
// @Description Record that an agent is using an MCP server tool (creates/updates agent-MCP connection)
// @Tags sdk-api
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body map[string]interface{} true "Connection data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/sdk-api/agents/{agent_id}/mcp-connections [post]
func (h *MCPAttestationHandler) RecordMCPConnection(c fiber.Ctx) error {
	// Get agent ID from URL path (already authenticated by SDK middleware)
	agentID, err := uuid.Parse(c.Params("agent_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	// Parse request body
	type RecordConnectionRequest struct {
		MCPServerID    string `json:"mcpServerId"`
		ToolName       string `json:"toolName"`
		MCPURL         string `json:"mcpUrl"`
		MCPName        string `json:"mcpName"`
		ConnectionType string `json:"connectionType"`
	}

	var req RecordConnectionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// Validate required fields
	if req.MCPServerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mcp_server_id is required",
		})
	}

	if req.ToolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tool_name is required",
		})
	}

	mcpServerID, err := uuid.Parse(req.MCPServerID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// SECURITY (defect #19): verify the agent in the URL belongs to the
	// caller's organization before recording the MCP connection. Without
	// this check, an SDK token for org A can record fake MCP connections
	// for any agent in any org by guessing UUIDs. Placed AFTER the body
	// validation so that malformed-input tests retain their 400-shape
	// contract; the security boundary is the service call below, not the
	// JSON parse.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Record the connection
	connection, err := h.attestationService.RecordAgentMCPConnection(
		c.Context(),
		agentID,
		mcpServerID,
		req.ToolName,
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to record MCP connection",
			"message": err.Error(),
		})
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		uuid.Nil, // No organization context for SDK endpoints
		agentID,
		domain.AuditActionCreate,
		"agent_mcp_connection",
		connection.ID,
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"connection_id":      connection.ID,
			"mcp_server_id":      mcpServerID,
			"tool_name":          req.ToolName,
			"connection_type":    connection.ConnectionType,
			"attestation_count":  connection.AttestationCount,
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":           true,
		"connection_id":     connection.ID,
		"agentId":          connection.AgentID,
		"mcp_server_id":     connection.MCPServerID,
		"connection_type":   connection.ConnectionType,
		"attestation_count": connection.AttestationCount,
		"last_attested_at":  connection.LastAttestedAt,
		"message":           "MCP connection recorded successfully",
	})
}

// RecordMCPUsageReport handles agent reporting MCP supply chain usage analytics
// @Summary Record MCP usage report
// @Description Record agent's MCP tool usage statistics for supply chain analytics
// @Tags sdk-api
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID"
// @Param request body MCPUsageReportRequest true "Usage report data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/sdk-api/agents/{agent_id}/mcp-usage-report [post]
func (h *MCPAttestationHandler) RecordMCPUsageReport(c fiber.Ctx) error {
	// Get agent ID from URL path (already authenticated by SDK middleware)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid agent ID",
			"message": err.Error(),
		})
	}

	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// SECURITY (defect #19b): same tenant-scoping pattern as
	// RecordMCPConnection. Verify the URL agent belongs to caller's org
	// before processing the usage report body.
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	// Parse request body
	type ToolUsageEntry struct {
		Count     int    `json:"count"`
		FirstUsed string `json:"firstUsed"`
		LastUsed  string `json:"lastUsed"`
	}

	type MCPServerUsage struct {
		ToolUsage            map[string]ToolUsageEntry `json:"toolUsage"`
		LastAttestedAt       *string                   `json:"lastAttestedAt"`
		CapabilitiesAttested []string                  `json:"capabilitiesAttested"`
		ConfidenceScore      *float64                  `json:"confidenceScore"`
		FirstSeenAt          *string                   `json:"firstSeenAt"`
	}

	type MCPUsageReportRequest struct {
		AgentID    string                    `json:"agentId"`
		MCPServers map[string]MCPServerUsage `json:"mcpServers"`
		ReportedAt string                    `json:"reportedAt"`
	}

	var req MCPUsageReportRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	// Calculate totals for response
	serversReported := len(req.MCPServers)
	totalInvocations := 0

	for serverID, serverUsage := range req.MCPServers {
		// Calculate total invocations for this server
		serverInvocations := 0
		for _, toolUsage := range serverUsage.ToolUsage {
			serverInvocations += toolUsage.Count
			totalInvocations += toolUsage.Count
		}

		// Try to parse server ID and record the usage data
		mcpServerID, parseErr := uuid.Parse(serverID)
		if parseErr != nil {
			// Log but continue - invalid UUIDs are skipped
			continue
		}

		// Convert local ToolUsageEntry to application.ToolUsageEntry
		serviceToolUsage := make(map[string]application.ToolUsageEntry)
		for toolName, usage := range serverUsage.ToolUsage {
			serviceToolUsage[toolName] = application.ToolUsageEntry{
				Count:     usage.Count,
				FirstUsed: usage.FirstUsed,
				LastUsed:  usage.LastUsed,
			}
		}

		// Record the usage report via service
		err := h.attestationService.RecordMCPUsageReport(
			c.Context(),
			agentID,
			mcpServerID,
			serviceToolUsage,
			serverUsage.CapabilitiesAttested,
			serverUsage.ConfidenceScore,
			serverUsage.LastAttestedAt,
			serverUsage.FirstSeenAt,
			serverInvocations,
		)
		if err != nil {
			// Log error but continue processing other servers
			// Don't fail the whole request for one server's failure
			continue
		}
	}

	// Audit log
	h.auditService.LogAction(
		c.Context(),
		uuid.Nil, // No organization context for SDK endpoints
		agentID,
		domain.AuditActionCreate,
		"mcp_usage_report",
		agentID, // Use agent ID as entity ID for reports
		c.IP(),
		c.Get("User-Agent"),
		fiber.Map{
			"servers_reported":   serversReported,
			"total_invocations":  totalInvocations,
			"reported_at":        req.ReportedAt,
		},
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":          true,
		"serversReported":  serversReported,
		"totalInvocations": totalInvocations,
		"message":          fmt.Sprintf("Recorded usage report for %d MCP servers with %d total invocations", serversReported, totalInvocations),
	})
}

// GetConsensusStatus returns the multi-agent consensus status for an MCP server
// @Summary Get MCP consensus status
// @Description Get the current multi-agent consensus verification status and progress
// @Tags mcp-servers
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {object} application.ConsensusStatus
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/mcp-servers/{id}/consensus-status [get]
func (h *MCPAttestationHandler) GetConsensusStatus(c fiber.Ctx) error {
	// Parse MCP server ID from URL
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid MCP server ID",
			"message": err.Error(),
		})
	}

	// SECURITY (A3d-vi): tenant-scope the path MCP server. Consensus
	// status for an MCP in another org would otherwise be readable by
	// anyone who can guess the UUID — a cross-tenant existence + state
	// leak. LoadOwned collapses both to 404.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.mcpServerRepo.GetByID, mcpServerID, orgID, mcpServerOrgID) == nil {
		return nil
	}

	// Get consensus status
	status, err := h.attestationService.GetConsensusStatus(c.Context(), mcpServerID)
	if err != nil {
		statusCode := fiber.StatusInternalServerError
		if err.Error() == "mcp server not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   "Failed to get consensus status",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(status)
}
