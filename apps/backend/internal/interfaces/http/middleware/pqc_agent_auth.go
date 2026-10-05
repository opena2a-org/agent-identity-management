package middleware

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// PQCAgentMiddleware validates post-quantum cryptography signed requests from SDK agents
// Supports:
// - Pure Ed25519 (X-Algorithm: Ed25519 or absent)
// - Pure ML-DSA (X-Algorithm: ML-DSA-44/65/87)
// - Hybrid Ed25519+ML-DSA (X-Algorithm: Ed25519+ML-DSA-44/65/87)
//
// For hybrid mode, BOTH signatures must be valid (defense in depth)
//
// Headers:
// - X-Agent-ID: Agent UUID
// - X-Algorithm: Signature algorithm (Ed25519, ML-DSA-65, Ed25519+ML-DSA-65, etc.)
// - X-Signature: Base64-encoded signature (Ed25519 or ML-DSA)
// - X-Signature-Ed25519: Base64-encoded Ed25519 signature (for hybrid mode)
// - X-Signature-MLDSA: Base64-encoded ML-DSA signature (for hybrid mode)
// - X-Timestamp: Unix timestamp of request
// - X-Public-Key: Agent's Ed25519 public key (base64)
// - X-PQC-Public-Key: Agent's ML-DSA public key (base64, for PQC/hybrid modes)
func PQCAgentMiddleware(agentService *application.AgentService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If Authorization header is present (JWT), skip PQC and let JWT middleware handle it
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			return c.Next()
		}

		// Extract headers
		agentIDStr := c.Get("X-Agent-ID")
		algorithm := c.Get("X-Algorithm")
		timestampStr := c.Get("X-Timestamp")

		// Check if basic required headers are present
		if agentIDStr == "" || timestampStr == "" {
			// Let other middlewares handle it
			return c.Next()
		}

		// Default to Ed25519 if algorithm not specified
		if algorithm == "" {
			algorithm = string(pqc.AlgorithmEd25519)
		}

		// Parse algorithm
		alg := pqc.Algorithm(algorithm)
		if !pqc.IsValidAlgorithm(alg) {
			return refuseS1(c, metrics.S1ReasonUnsupportedAlgorithm, fiber.StatusBadRequest,
				fmt.Sprintf("Unsupported algorithm: %s", algorithm))
		}

		// Parse agent ID
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			return refuseS1(c, metrics.S1ReasonInvalidAgentID, fiber.StatusUnauthorized, "Invalid agent ID format")
		}

		// Validate timestamp (prevent replay attacks)
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return refuseS1(c, metrics.S1ReasonInvalidTimestamp, fiber.StatusUnauthorized, "Invalid timestamp format")
		}

		// The two directions answer the caller identically and are counted apart: a
		// server clock running fast refuses honest requests as skew_past, one running
		// slow refuses them as skew_future.
		now := time.Now().Unix()
		const maxClockSkewSeconds = 30
		if timestamp < now-maxClockSkewSeconds {
			return refuseS1(c, metrics.S1ReasonSkewPast, fiber.StatusUnauthorized, "Request timestamp expired or invalid")
		}
		if timestamp > now+maxClockSkewSeconds {
			return refuseS1(c, metrics.S1ReasonSkewFuture, fiber.StatusUnauthorized, "Request timestamp expired or invalid")
		}

		// Load agent from database
		agent, err := agentService.GetAgent(c.Context(), agentID)
		if err != nil {
			return refuseS1(c, metrics.S1ReasonAgentLookupFailed, fiber.StatusUnauthorized, "Agent not found")
		}

		// SECURITY: Revocation is enforced HERE, on the read path, not only at the write
		// that sets the status. RevokeAgent expresses denial purely
		// as `agents.status`, so an agent that keeps its key material after being revoked
		// or suspended authenticated successfully until this check existed. Checked before
		// any signature work so a denied agent costs no ML-DSA verification.
		if !agentStatusPermitsAuth(agent.Status) {
			return refuseS1(c, metrics.S1ReasonAgentStatusDenied, fiber.StatusUnauthorized, agentStatusDeniedMessage(agent.Status))
		}

		// Check if hybrid mode is required but agent doesn't support it
		if pqc.IsHybridAlgorithm(alg) && !agent.HybridModeEnabled {
			// Check if agent has PQC key registered
			if agent.PQCPublicKey == nil || *agent.PQCPublicKey == "" {
				return refuseS1(c, metrics.S1ReasonNoRegisteredKey, fiber.StatusUnauthorized,
					"Agent does not have PQC key registered for hybrid mode")
			}
		}

		// Reconstruct the signed message
		method := strings.ToUpper(c.Method())
		path := c.OriginalURL()
		messageParts := []string{method, path, timestampStr}
		if len(c.Body()) > 0 {
			messageParts = append(messageParts, string(c.Body()))
		}
		message := []byte(strings.Join(messageParts, "\n"))

		// Verify based on algorithm type
		var authMethod string
		switch {
		case pqc.IsHybridAlgorithm(alg):
			// Hybrid mode: verify BOTH Ed25519 and ML-DSA
			authMethod = "hybrid"
			if refusal := verifyHybridSignature(c, agent, alg, message); refusal != nil {
				return refuseS1(c, refusal.reason, fiber.StatusUnauthorized, refusal.message)
			}

		case pqc.IsPQCAlgorithm(alg):
			// Pure ML-DSA mode
			authMethod = "mldsa"
			if refusal := verifyMLDSASignature(c, agent, alg, message); refusal != nil {
				return refuseS1(c, refusal.reason, fiber.StatusUnauthorized, refusal.message)
			}

		default:
			// Pure Ed25519 mode (default)
			authMethod = "ed25519"
			if refusal := verifyEd25519Signature(c, agent, message); refusal != nil {
				return refuseS1(c, refusal.reason, fiber.StatusUnauthorized, refusal.message)
			}
		}

		// Signature(s) valid! Set agent context for handlers
		c.Locals("agent_id", agentID)
		c.Locals("organization_id", agent.OrganizationID)
		c.Locals("authenticated_via", authMethod)
		c.Locals("auth_method", authMethod)
		c.Locals("auth_algorithm", algorithm)

		return c.Next()
	}
}

// verifyEd25519Signature verifies a pure Ed25519 signature
func verifyEd25519Signature(c fiber.Ctx, agent *domain.Agent, message []byte) *s1Refusal {
	signatureB64 := c.Get("X-Signature")
	publicKeyB64 := c.Get("X-Public-Key")

	if signatureB64 == "" || publicKeyB64 == "" {
		return s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing Ed25519 signature or public key")
	}

	// SECURITY: Agent MUST have a registered Ed25519 public key
	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return s1Refused(metrics.S1ReasonNoRegisteredKey, "agent has no registered Ed25519 public key")
	}
	verifyPublicKey := *agent.PublicKey

	// Verify provided key matches registered key
	if publicKeyB64 != verifyPublicKey {
		return s1Refused(metrics.S1ReasonPublicKeyMismatch, "provided Ed25519 public key does not match registered key")
	}

	// Decode public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(verifyPublicKey)
	if err != nil {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid public key format")
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid Ed25519 public key size")
	}

	// Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonSignatureMalformed, "invalid signature format")
	}

	// Verify
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), message, signatureBytes) {
		return s1Refused(metrics.S1ReasonSignatureInvalidEd25519, "invalid Ed25519 signature")
	}

	return nil
}

// verifyMLDSASignature verifies a pure ML-DSA signature
func verifyMLDSASignature(c fiber.Ctx, agent *domain.Agent, alg pqc.Algorithm, message []byte) *s1Refusal {
	signatureB64 := c.Get("X-Signature")
	pqcPublicKeyB64 := c.Get("X-PQC-Public-Key")

	if signatureB64 == "" {
		signatureB64 = c.Get("X-Signature-MLDSA")
	}
	if signatureB64 == "" {
		return s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing ML-DSA signature")
	}

	// SECURITY: Agent MUST have a registered PQC public key for ML-DSA auth
	if agent.PQCPublicKey == nil || *agent.PQCPublicKey == "" {
		return s1Refused(metrics.S1ReasonNoRegisteredKey, "agent has no registered ML-DSA public key")
	}
	verifyPublicKey := *agent.PQCPublicKey

	// Verify provided key matches registered key
	if pqcPublicKeyB64 != "" && pqcPublicKeyB64 != verifyPublicKey {
		return s1Refused(metrics.S1ReasonPublicKeyMismatch, "provided ML-DSA public key does not match registered key")
	}

	// Decode public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(verifyPublicKey)
	if err != nil {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid PQC public key format")
	}

	// Validate key size
	expectedSize, err := pqc.GetExpectedPublicKeySize(alg)
	if err != nil {
		return s1Refused(metrics.S1ReasonUnsupportedAlgorithm, "unsupported ML-DSA algorithm: %s", alg)
	}
	if len(publicKeyBytes) != expectedSize {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid ML-DSA public key size: expected %d, got %d", expectedSize, len(publicKeyBytes))
	}

	// Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonSignatureMalformed, "invalid ML-DSA signature format")
	}

	// Verify ML-DSA signature
	if err := pqc.VerifyMLDSA(alg, publicKeyBytes, message, signatureBytes); err != nil {
		return s1Refused(metrics.S1ReasonSignatureInvalidMLDSA, "invalid ML-DSA signature: %v", err)
	}

	return nil
}

// verifyHybridSignature verifies both Ed25519 and ML-DSA signatures
// BOTH must be valid for the request to be authenticated
func verifyHybridSignature(c fiber.Ctx, agent *domain.Agent, alg pqc.Algorithm, message []byte) *s1Refusal {
	// Get both signatures
	ed25519SigB64 := c.Get("X-Signature-Ed25519")
	mldsaSigB64 := c.Get("X-Signature-MLDSA")

	// Fallback: if using single X-Signature header with Ed25519
	if ed25519SigB64 == "" {
		ed25519SigB64 = c.Get("X-Signature")
	}

	if ed25519SigB64 == "" {
		return s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing Ed25519 signature for hybrid mode")
	}
	if mldsaSigB64 == "" {
		return s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing ML-DSA signature for hybrid mode")
	}

	// SECURITY: Both keys MUST be registered for hybrid mode
	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return s1Refused(metrics.S1ReasonNoRegisteredKey, "agent has no registered Ed25519 public key for hybrid mode")
	}
	if agent.PQCPublicKey == nil || *agent.PQCPublicKey == "" {
		return s1Refused(metrics.S1ReasonNoRegisteredKey, "agent has no registered ML-DSA public key for hybrid mode")
	}

	ed25519PubKeyB64 := *agent.PublicKey
	pqcPubKeyB64 := *agent.PQCPublicKey

	// Verify provided keys match registered keys
	providedEd25519 := c.Get("X-Public-Key")
	providedPQC := c.Get("X-PQC-Public-Key")
	if providedEd25519 != "" && providedEd25519 != ed25519PubKeyB64 {
		return s1Refused(metrics.S1ReasonPublicKeyMismatch, "provided Ed25519 public key does not match registered key")
	}
	if providedPQC != "" && providedPQC != pqcPubKeyB64 {
		return s1Refused(metrics.S1ReasonPublicKeyMismatch, "provided ML-DSA public key does not match registered key")
	}

	// 1. Verify Ed25519 first (faster)
	ed25519PubKeyBytes, err := base64.StdEncoding.DecodeString(ed25519PubKeyB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid Ed25519 public key format")
	}
	if len(ed25519PubKeyBytes) != ed25519.PublicKeySize {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid Ed25519 public key size")
	}

	ed25519SigBytes, err := base64.StdEncoding.DecodeString(ed25519SigB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonSignatureMalformed, "invalid Ed25519 signature format")
	}

	if !ed25519.Verify(ed25519.PublicKey(ed25519PubKeyBytes), message, ed25519SigBytes) {
		return s1Refused(metrics.S1ReasonSignatureInvalidEd25519, "invalid Ed25519 signature in hybrid mode")
	}

	// 2. Verify ML-DSA (slower but quantum-resistant)
	pqcPubKeyBytes, err := base64.StdEncoding.DecodeString(pqcPubKeyB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid ML-DSA public key format")
	}

	expectedSize, err := pqc.GetExpectedPublicKeySize(alg)
	if err != nil {
		return s1Refused(metrics.S1ReasonUnsupportedAlgorithm, "unsupported ML-DSA algorithm in hybrid mode: %s", alg)
	}
	if len(pqcPubKeyBytes) != expectedSize {
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, "invalid ML-DSA public key size: expected %d, got %d", expectedSize, len(pqcPubKeyBytes))
	}

	mldsaSigBytes, err := base64.StdEncoding.DecodeString(mldsaSigB64)
	if err != nil {
		return s1Refused(metrics.S1ReasonSignatureMalformed, "invalid ML-DSA signature format")
	}

	if err := pqc.VerifyMLDSA(alg, pqcPubKeyBytes, message, mldsaSigBytes); err != nil {
		return s1Refused(metrics.S1ReasonSignatureInvalidMLDSA, "invalid ML-DSA signature in hybrid mode: %v", err)
	}

	// BOTH signatures are valid!
	return nil
}
