package handlers

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AIPHandler handles AIP (Agent Identity Protocol) discovery and DID resolution
type AIPHandler struct {
	agentRepo domain.AgentRepository
}

// NewAIPHandler creates a new AIP handler
func NewAIPHandler(agentRepo domain.AgentRepository) *AIPHandler {
	return &AIPHandler{
		agentRepo: agentRepo,
	}
}

// WellKnownAIP handles GET /.well-known/aip
// Returns the AIP discovery document describing provider capabilities and endpoints
func (h *AIPHandler) WellKnownAIP(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"providerDid":      "did:aip:provider_opena2a",
		"version":          "1.0",
		"conformanceLevel": 2,
		"endpoints": fiber.Map{
			"agents":       "/api/v1/agents",
			"challenge":    "/api/v1/agents/{agentId}/challenge",
			"verify":       "/api/v1/agents/{agentId}/verify",
			"capabilities": "/api/v1/capabilities",
			"trustScore":   "/api/v1/agents/{agentId}/trust",
			"atc":          "/api/v1/agents/{agentId}/atc",
			"audit":        "/api/v1/agents/{agentId}/audit",
			"didResolve":   "/api/v1/did/{did}",
		},
		"supportedAgentTypes": []string{
			"claude", "gpt", "gemini", "langchain", "crewai",
			"autogen", "semantic-kernel", "mcp-server", "a2a-agent", "custom",
		},
		"supportedCapabilityNamespaces": []string{
			"file", "db", "api", "network", "system", "mcp", "data", "payment",
		},
		"supportedProtocols": []string{"mcp", "a2a"},
	})
}

// ResolveDID handles GET /api/v1/did/*
// Resolves did:aip:aim_XXXXXXXX to a W3C DID Document
//
// SYNC: this file is NOT in .sync-protect, so it is owned by
// agent-identity-management and overwritten wholesale by the next sync. The
// conditions below (COUNCIL_LEDGER 2026-09-02, CHIEF-CISO and CHIEF-DPO) are ruled
// for BOTH repositories; if they land only here they are silently reverted the next
// time the public repo touches apps/backend. The twin change in
// agent-identity-management must land first or together with this one.
func (h *AIPHandler) ResolveDID(c fiber.Ctx) error {
	// Extract the DID from the wildcard path segment
	// Fiber v3 wildcard captures everything after /api/v1/did/
	rawDID := c.Params("*1")
	if rawDID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "missing_did",
			"message": "DID parameter is required",
		})
	}

	// Validate DID format: did:aip:<identifier>
	if !strings.HasPrefix(rawDID, "did:aip:") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_did_format",
			"message": "DID must use the did:aip method (e.g., did:aip:aim_XXXXXXXX)",
		})
	}

	// Extract the identifier after did:aip:
	identifier := strings.TrimPrefix(rawDID, "did:aip:")
	if identifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_did_format",
			"message": "DID identifier is empty",
		})
	}

	// The identifier should be a UUID (the agent ID)
	// AIM agents use UUIDs as their IDs, so did:aip:aim_<uuid> maps to GetByID
	// Strip the "aim_" prefix if present
	agentIDStr := strings.TrimPrefix(identifier, "aim_")

	// Parse agent ID as UUID
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_agent_id",
			"message": fmt.Sprintf("Invalid agent identifier in DID: %s", agentIDStr),
		})
	}

	// Look up agent from repository
	agent, err := h.agentRepo.GetByID(agentID)
	if err != nil {
		return didNotFound(c, rawDID)
	}

	// A `pending` agent has registered but has not been verified: it holds a claim on
	// an identity, not a published one. Resolving it would let anyone who can complete
	// self-registration publish a DID document from this provider, and would make the
	// difference between "registered but unverified" and "no such agent" observable to
	// an unauthenticated caller — an oracle over registrations. Both are closed by
	// answering exactly as an unknown identifier does.
	// COUNCIL_LEDGER 2026-09-02, CHIEF-CISO.
	if agent.Status == domain.AgentStatusPending {
		return didNotFound(c, rawDID)
	}

	// Build W3C DID Document (https://www.w3.org/TR/did-core/)
	did := domain.BuildAgentDID(agent.ID)

	// Build verification methods from agent's public key, collecting the id of each
	// method actually emitted. authentication/assertionMethod are built from this
	// slice rather than from a hard-coded fragment: they previously named "#key-1"
	// unconditionally, so a keyless agent's document — and a PQC-only agent's —
	// pointed at a verification method the document did not contain.
	verificationMethods := []fiber.Map{}
	verificationMethodIDs := []string{}
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		id := fmt.Sprintf("%s#key-1", did)
		verificationMethods = append(verificationMethods, fiber.Map{
			"id":                 id,
			"type":               "Ed25519VerificationKey2020",
			"controller":         did,
			"publicKeyMultibase": multibaseEncode(*agent.PublicKey),
		})
		verificationMethodIDs = append(verificationMethodIDs, id)
	}

	// Add PQC key if available
	if agent.PQCPublicKey != nil && *agent.PQCPublicKey != "" {
		algorithm := "ML-DSA-65"
		if agent.PQCKeyAlgorithm != nil {
			algorithm = *agent.PQCKeyAlgorithm
		}
		id := fmt.Sprintf("%s#pqc-key-1", did)
		verificationMethods = append(verificationMethods, fiber.Map{
			"id":                 id,
			"type":               algorithm,
			"controller":         did,
			"publicKeyMultibase": multibaseEncode(*agent.PQCPublicKey),
		})
		verificationMethodIDs = append(verificationMethodIDs, id)
	}

	// Build service endpoints
	services := []fiber.Map{
		{
			"id":              fmt.Sprintf("%s#trust", did),
			"type":            "TrustScoreService",
			"serviceEndpoint": fmt.Sprintf("/api/v1/agents/%s/trust", agent.ID.String()),
		},
		{
			"id":              fmt.Sprintf("%s#audit", did),
			"type":            "AuditService",
			"serviceEndpoint": fmt.Sprintf("/api/v1/agents/%s/audit", agent.ID.String()),
		},
		{
			"id":              fmt.Sprintf("%s#verify", did),
			"type":            "VerificationService",
			"serviceEndpoint": fmt.Sprintf("/api/v1/agents/%s/verify", agent.ID.String()),
		},
	}

	updated := documentUpdated(agent).Format(time.RFC3339)

	// `deactivated` is the document's only signal that the published key material must
	// not be trusted, so it has to cover every status that stops the agent
	// authenticating — not just `revoked`. domain.AgentStatusPermitsAuth is the
	// allow-list the auth middlewares already gate on; `pending` never reaches this
	// line (404 above), so what remains is {suspended, revoked} plus any value outside
	// the four domain constants. agents.status is a plain VARCHAR(50) with no CHECK
	// constraint, so such a value is storable, and an allow-list publishes it as
	// deactivated rather than as live keys. COUNCIL_LEDGER 2026-09-02, CHIEF-CISO.
	deactivated := !domain.AgentStatusPermitsAuth(agent.Status)

	didDocument := fiber.Map{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
		},
		"id":                 did,
		"verificationMethod": verificationMethods,
		"authentication":     verificationMethodIDs,
		"assertionMethod":    verificationMethodIDs,
		"service":            services,
		"created":            agent.CreatedAt.Format(time.RFC3339),
		"updated":            updated,
	}

	return c.JSON(fiber.Map{
		"@context":    "https://w3id.org/did-resolution/v1",
		"didDocument": didDocument,
		"didDocumentMetadata": fiber.Map{
			"created":     agent.CreatedAt.Format(time.RFC3339),
			"updated":     updated,
			"deactivated": deactivated,
		},
		"didResolutionMetadata": fiber.Map{
			"contentType": "application/did+ld+json",
		},
	})
}

// didNotFound is the single resolution-failure answer. An unknown identifier and a
// `pending` agent MUST be byte-for-byte indistinguishable to the caller, so both go
// through here rather than through two bodies that could drift apart.
func didNotFound(c fiber.Ctx, rawDID string) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error":   "did_not_found",
		"message": fmt.Sprintf("No agent found for DID: %s", rawDID),
	})
}

// documentUpdated is the timestamp the DID document publishes as `updated`.
//
// It is deliberately NOT agents.updated_at. That column moves on every write to the
// row — a display-name edit, a tag, a trust-score recomputation, an operator's
// administrative change — so publishing it on an unauthenticated endpoint turns the
// resolver into an activity feed about the agent's operator, readable by anyone who
// knows a UUID. COUNCIL_LEDGER 2026-09-02, CHIEF-DPO decision 1 disallows it as a
// source outright.
//
// A DID document's `updated` means "when this DOCUMENT last changed", and the only
// part of this document that ever changes is the key material. So it is the later of
// the two key-generation timestamps when either is set, and the agent's creation
// instant when the agent carries no key at all — which is also the only timestamp
// that is already public in `created`.
func documentUpdated(agent *domain.Agent) time.Time {
	var latest *time.Time
	for _, ts := range []*time.Time{agent.KeyCreatedAt, agent.PQCKeyCreatedAt} {
		if ts == nil {
			continue
		}
		if latest == nil || ts.After(*latest) {
			latest = ts
		}
	}
	if latest != nil {
		return *latest
	}
	return agent.CreatedAt
}

// multibaseEncode converts a base64-encoded public key to multibase format (base64url with 'u' prefix)
func multibaseEncode(base64Key string) string {
	// Decode from standard base64
	raw, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		// If it fails to decode, return as-is with multibase prefix
		return "u" + base64Key
	}
	// Re-encode as base64url (no padding) with multibase 'u' prefix
	return "u" + base64.RawURLEncoding.EncodeToString(raw)
}
