package middleware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/agentauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
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

		// Reconstruct the signed message
		method := strings.ToUpper(c.Method())
		path := c.OriginalURL()
		messageParts := []string{method, path, timestampStr}
		if len(c.Body()) > 0 {
			messageParts = append(messageParts, string(c.Body()))
		}
		message := []byte(strings.Join(messageParts, "\n"))

		// The signature headers are part of the request's shape: read and decoded before
		// any agent is, so a malformed request gets the same answer whichever agent it names.
		var signed pqcSignedRequest
		var malformed *s1Refusal
		switch {
		case pqc.IsHybridAlgorithm(alg):
			signed, malformed = readHybridSignatures(c)
		case pqc.IsPQCAlgorithm(alg):
			signed, malformed = readMLDSASignature(c)
		default:
			signed, malformed = readEd25519Signature(c)
		}
		if malformed != nil {
			return refuseS1(c, malformed.reason, fiber.StatusUnauthorized, malformed.message)
		}

		// SECURITY: only the agent's registered keys are read before the signature
		// verifies. An unknown agent, an agent with no registered key for the algorithm
		// (no key may be supplied by the request itself: that would be trust on first
		// use), a registered key that does not decode, and a presented key that is not
		// the registered one all get the one refusal, so this answer tells a caller who
		// holds only an agent id nothing about that agent.
		keys := agentauth.KeySet(c.Context(), agentService, agentID)

		var verified agentauth.VerifiedAgent
		var authMethod string
		switch {
		case pqc.IsHybridAlgorithm(alg):
			// Hybrid mode: verify BOTH Ed25519 and ML-DSA
			authMethod = "hybrid"
			verified, err = keys.VerifyHybrid(alg, signed.ed25519Key, signed.mldsaKey, message, signed.ed25519Sig, signed.mldsaSig)
		case pqc.IsPQCAlgorithm(alg):
			// Pure ML-DSA mode
			authMethod = "mldsa"
			verified, err = keys.VerifyMLDSA(alg, signed.mldsaKey, message, signed.mldsaSig)
		default:
			// Pure Ed25519 mode (default)
			authMethod = "ed25519"
			verified, err = keys.VerifyEd25519(signed.ed25519Key, message, signed.ed25519Sig)
		}
		if err != nil {
			refusal := agentKeyRefusal(err, pqcSignatureFailure(alg, err))
			return refuseS1(c, refusal.reason, fiber.StatusUnauthorized, refusal.message)
		}

		// SECURITY: Revocation is enforced HERE, on the read path, not only at the write
		// that sets the status. RevokeAgent expresses denial purely as `agents.status`, so
		// an agent that keeps its key material after being revoked or suspended
		// authenticated successfully until this check existed. It runs after the signature
		// verifies, so the status it names reaches only a caller holding this agent's key.
		agent := agentauth.LoadVerifiedAgent(verified)
		if !agentStatusPermitsAuth(agent.Status) {
			return refuseS1(c, metrics.S1ReasonAgentStatusDenied, fiber.StatusUnauthorized, agentStatusDeniedMessage(agent.Status))
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

// pqcSignedRequest is what a request carries for its algorithm: its signatures, decoded,
// and the public keys it presents, base64 as sent ("" when the header is absent).
type pqcSignedRequest struct {
	ed25519Sig []byte
	mldsaSig   []byte
	ed25519Key string
	mldsaKey   string
}

// readEd25519Signature reads the signature and public key headers of a pure Ed25519
// request. It reads no agent.
func readEd25519Signature(c fiber.Ctx) (pqcSignedRequest, *s1Refusal) {
	var signed pqcSignedRequest
	signatureB64 := c.Get("X-Signature")
	signed.ed25519Key = c.Get("X-Public-Key")
	if signatureB64 == "" || signed.ed25519Key == "" {
		return signed, s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing Ed25519 signature or public key")
	}
	var err error
	if signed.ed25519Sig, err = base64.StdEncoding.DecodeString(signatureB64); err != nil {
		return signed, s1Refused(metrics.S1ReasonSignatureMalformed, "invalid signature format")
	}
	return signed, nil
}

// readMLDSASignature reads the signature header, and the public key if the request
// presents one, of a pure ML-DSA request. It reads no agent.
func readMLDSASignature(c fiber.Ctx) (pqcSignedRequest, *s1Refusal) {
	var signed pqcSignedRequest
	signatureB64 := c.Get("X-Signature")
	if signatureB64 == "" {
		signatureB64 = c.Get("X-Signature-MLDSA")
	}
	if signatureB64 == "" {
		return signed, s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing ML-DSA signature")
	}
	var err error
	if signed.mldsaSig, err = base64.StdEncoding.DecodeString(signatureB64); err != nil {
		return signed, s1Refused(metrics.S1ReasonSignatureMalformed, "invalid ML-DSA signature format")
	}
	signed.mldsaKey = c.Get("X-PQC-Public-Key")
	return signed, nil
}

// readHybridSignatures reads both signature headers of a hybrid request, and the public
// keys it presents. BOTH signatures are required. It reads no agent.
func readHybridSignatures(c fiber.Ctx) (pqcSignedRequest, *s1Refusal) {
	var signed pqcSignedRequest
	ed25519SigB64 := c.Get("X-Signature-Ed25519")
	mldsaSigB64 := c.Get("X-Signature-MLDSA")
	// Fallback: if using single X-Signature header with Ed25519
	if ed25519SigB64 == "" {
		ed25519SigB64 = c.Get("X-Signature")
	}
	if ed25519SigB64 == "" {
		return signed, s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing Ed25519 signature for hybrid mode")
	}
	if mldsaSigB64 == "" {
		return signed, s1Refused(metrics.S1ReasonMissingSignatureHeaders, "missing ML-DSA signature for hybrid mode")
	}
	var err error
	if signed.ed25519Sig, err = base64.StdEncoding.DecodeString(ed25519SigB64); err != nil {
		return signed, s1Refused(metrics.S1ReasonSignatureMalformed, "invalid Ed25519 signature format")
	}
	if signed.mldsaSig, err = base64.StdEncoding.DecodeString(mldsaSigB64); err != nil {
		return signed, s1Refused(metrics.S1ReasonSignatureMalformed, "invalid ML-DSA signature format")
	}
	signed.ed25519Key = c.Get("X-Public-Key")
	signed.mldsaKey = c.Get("X-PQC-Public-Key")
	return signed, nil
}

// agentKeyRefusal is the refusal for err from an agentauth Verify method, used by both
// signed-request middlewares; signatureInvalid is the message for a signature that does
// not verify under the registered key.
//
// The four causes behind agentauth.ErrKeyNotRecognized carry the one message, and
// refuseS1 answers each with the one body, so the caller is not told which it was. Each
// is still counted under its own reason: the counter is read by operators, not callers,
// and keeps a corrupt stored key apart from an unknown agent or a client presenting the
// wrong key.
func agentKeyRefusal(err error, signatureInvalid string) *s1Refusal {
	cause, keyNotRecognized := agentauth.KeyNotRecognizedCause(err)
	switch {
	case keyNotRecognized && cause == agentauth.KeyCauseAgentUnknown:
		return s1Refused(metrics.S1ReasonAgentLookupFailed, agentauth.KeyNotRecognizedMessage)
	case keyNotRecognized && cause == agentauth.KeyCauseNoRegisteredKey:
		return s1Refused(metrics.S1ReasonNoRegisteredKey, agentauth.KeyNotRecognizedMessage)
	case keyNotRecognized && cause == agentauth.KeyCauseRegisteredKeyMalformed:
		return s1Refused(metrics.S1ReasonRegisteredKeyMalformed, agentauth.KeyNotRecognizedMessage)
	case keyNotRecognized:
		// The one cause left: the request presented a key that is not the registered one.
		return s1Refused(metrics.S1ReasonPublicKeyMismatch, agentauth.KeyNotRecognizedMessage)
	case errors.Is(err, agentauth.ErrMLDSASignatureInvalid):
		return s1Refused(metrics.S1ReasonSignatureInvalidMLDSA, "%s", signatureInvalid)
	default:
		return s1Refused(metrics.S1ReasonSignatureInvalidEd25519, "%s", signatureInvalid)
	}
}

// pqcSignatureFailure is the refusal message for a signature that did not verify under
// the registered key.
func pqcSignatureFailure(alg pqc.Algorithm, err error) string {
	hybrid := pqc.IsHybridAlgorithm(alg)
	switch {
	case errors.Is(err, agentauth.ErrMLDSASignatureInvalid) && hybrid:
		return "invalid ML-DSA signature in hybrid mode"
	case errors.Is(err, agentauth.ErrMLDSASignatureInvalid):
		return "invalid ML-DSA signature"
	case hybrid:
		return "invalid Ed25519 signature in hybrid mode"
	default:
		return "invalid Ed25519 signature"
	}
}
