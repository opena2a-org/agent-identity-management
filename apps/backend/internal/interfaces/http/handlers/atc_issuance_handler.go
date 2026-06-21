package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// ATCIssuanceHandler exposes POST /agents/:id/atc. It issues a portable Agent
// Trust Credential carrying AIM's behavioral trust score, by delegating signing +
// transparency-log recording to the Registry (the Certificate Authority).
type ATCIssuanceHandler struct {
	service      *application.ATCIssuanceService
	agentService *application.AgentService
	auditService *application.AuditService
}

// NewATCIssuanceHandler wires the handler.
func NewATCIssuanceHandler(
	service *application.ATCIssuanceService,
	agentService *application.AgentService,
	auditService *application.AuditService,
) *ATCIssuanceHandler {
	return &ATCIssuanceHandler{
		service:      service,
		agentService: agentService,
		auditService: auditService,
	}
}

// IssueATC handles POST /agents/:id/atc.
func (h *ATCIssuanceHandler) IssueATC(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	// Reject a missing or zero organization. A Nil orgID would otherwise compare
	// equal to an agent whose OrganizationID is also Nil, collapsing the tenant
	// check below — fail closed instead of issuing a credential cross-tenant.
	if !ok || orgID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid agent ID"})
	}

	// Authorize: the agent must belong to the caller's organization. This mirrors
	// the trust-score endpoints' tenant check so a credential can only be issued
	// for an agent the caller owns.
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Agent not found"})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	result, err := h.service.IssueForAgent(c.Context(), agentID)
	if err != nil {
		// A missing service-account token is a server configuration problem, not a
		// client error — surface it distinctly so operators can spot an unprovisioned
		// REGISTRY_ATC_TOKEN (Phase C) rather than misreading it as a bad request.
		if errors.Is(err, registry.ErrTokenNotConfigured) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "ATC issuance is not configured (registry service-account token missing)",
			})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to issue Agent Trust Credential",
		})
	}

	cred := result.Credential

	// Audit the issuance. Best-effort: a logging failure must not fail an
	// otherwise-successful issuance.
	_ = h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionGenerate,
		"atc",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":            agent.Name,
			"agentDid":             cred.AgentDID,
			"trustScore":           cred.TrustScore,
			"trustLevel":           cred.TrustLevel,
			"transparencyLogIndex": cred.TransparencyLogIndex,
		},
	)

	// Return the Registry's VERBATIM signed credential bytes (cred.Raw), not the
	// parsed mirror struct. The hybrid signature covers JCS(TBS) over the full field
	// set (scanSummary, issuerChain, publisherDid, buildAttestation, declaredPurpose);
	// AIM's AgentTrustCredential models only the subset it needs for audit/UX, so
	// re-marshalling it would strip signed fields and yield a credential whose
	// signature no longer verifies. AIM is a pass-through for the signed artifact;
	// the Registry is the CA. The parsed struct above is used only for the audit log.
	//
	// The unsigned provenance context (confidence, isolation caveat) keeps the badge
	// honest about factor 9 until the verified isolation writer ships
	// (aim-isolation-verification).
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"credential": cred.Raw,
		"provenance": fiber.Map{
			"confidence":            result.Confidence,
			"isolationSelfReported": result.IsolationSelfReported,
			"note":                  "trustScore is AIM's 9-factor behavioral score; factor 9 (execution isolation) is self-reported until verified-isolation attestation ships.",
		},
	})
}
