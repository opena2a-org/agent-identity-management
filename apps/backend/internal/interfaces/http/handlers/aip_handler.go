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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "did_not_found",
			"message": fmt.Sprintf("No agent found for DID: %s", rawDID),
		})
	}

	// Build W3C DID Document (https://www.w3.org/TR/did-core/)
	did := fmt.Sprintf("did:aip:aim_%s", agent.ID.String())

	// Build verification methods from agent's public key
	verificationMethods := []fiber.Map{}
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		verificationMethods = append(verificationMethods, fiber.Map{
			"id":                 fmt.Sprintf("%s#key-1", did),
			"type":              "Ed25519VerificationKey2020",
			"controller":        did,
			"publicKeyMultibase": multibaseEncode(*agent.PublicKey),
		})
	}

	// Add PQC key if available
	if agent.PQCPublicKey != nil && *agent.PQCPublicKey != "" {
		algorithm := "ML-DSA-65"
		if agent.PQCKeyAlgorithm != nil {
			algorithm = *agent.PQCKeyAlgorithm
		}
		verificationMethods = append(verificationMethods, fiber.Map{
			"id":                 fmt.Sprintf("%s#pqc-key-1", did),
			"type":              algorithm,
			"controller":        did,
			"publicKeyMultibase": multibaseEncode(*agent.PQCPublicKey),
		})
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

	didDocument := fiber.Map{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
		},
		"id":                   did,
		"verificationMethod":   verificationMethods,
		"authentication":       []string{fmt.Sprintf("%s#key-1", did)},
		"assertionMethod":      []string{fmt.Sprintf("%s#key-1", did)},
		"service":              services,
		"created":              agent.CreatedAt.Format(time.RFC3339),
		"updated":              agent.UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(fiber.Map{
		"@context":    "https://w3id.org/did-resolution/v1",
		"didDocument": didDocument,
		"didDocumentMetadata": fiber.Map{
			"created":     agent.CreatedAt.Format(time.RFC3339),
			"updated":     agent.UpdatedAt.Format(time.RFC3339),
			"deactivated": agent.Status == domain.AgentStatusRevoked,
		},
		"didResolutionMetadata": fiber.Map{
			"contentType": "application/did+ld+json",
		},
	})
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
