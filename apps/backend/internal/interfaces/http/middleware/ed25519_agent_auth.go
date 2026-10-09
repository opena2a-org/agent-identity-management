package middleware

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/agentauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// sortedJSONMarshal marshals JSON with sorted keys to match Python's json.dumps(sort_keys=True)
// Python's default uses separators=(', ', ': ') with spaces after colons and commas
func sortedJSONMarshal(v interface{}) []byte {
	// Recursively sort all objects in the data structure
	sorted := sortValue(v)

	// Marshal with standard Go json (compact, no spaces)
	compactBytes, err := json.Marshal(sorted)
	if err != nil {
		return []byte("{}")
	}

	// Convert compact JSON to Python's default format with spaces
	// Python uses: (', ', ': ') = space after comma, space after colon
	result := string(compactBytes)
	result = strings.ReplaceAll(result, "\":", "\": ") // Add space after colon
	result = strings.ReplaceAll(result, ",\"", ", \"") // Add space after comma before quote
	result = strings.ReplaceAll(result, ",[", ", [")   // Add space after comma before bracket
	result = strings.ReplaceAll(result, ",{", ", {")   // Add space after comma before brace

	return []byte(result)
}

// sortValue recursively sorts all maps by key
func sortValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Sort map keys
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Create ordered map representation
		sorted := make(map[string]interface{})
		for _, k := range keys {
			sorted[k] = sortValue(val[k])
		}
		return sorted

	case []interface{}:
		// Recursively sort array elements
		sorted := make([]interface{}, len(val))
		for i, item := range val {
			sorted[i] = sortValue(item)
		}
		return sorted

	default:
		return val
	}
}

// Ed25519AgentMiddleware validates Ed25519 signed requests from SDK agents
// This middleware checks for:
// - X-Agent-ID: Agent UUID
// - X-Signature: Base64-encoded Ed25519 signature
// - X-Timestamp: Unix timestamp of request
// - X-Public-Key: Agent's Ed25519 public key (base64)
func Ed25519AgentMiddleware(agentService *application.AgentService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If Authorization header is present (JWT), skip Ed25519 and let JWT middleware handle it
		// This is critical for key registration workflow where SDK needs JWT auth before Ed25519
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			return c.Next()
		}

		// Extract headers
		agentIDStr := c.Get("X-Agent-ID")
		signatureB64 := c.Get("X-Signature")
		timestampStr := c.Get("X-Timestamp")
		publicKeyB64 := c.Get("X-Public-Key")

		// Check if all required headers are present
		if agentIDStr == "" || signatureB64 == "" || timestampStr == "" || publicKeyB64 == "" {
			// If Ed25519 headers are missing, this might be a JWT or API key request
			// Let other middlewares handle it
			return c.Next()
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

		now := time.Now().Unix()
		// SECURITY: Allow only 30 seconds clock skew to minimize replay attack window
		// This is a balance between security and usability for network latency
		//
		// The two directions answer the caller identically and are counted apart: a
		// server clock running fast refuses honest requests as skew_past, one running
		// slow refuses them as skew_future.
		const maxClockSkewSeconds = 30
		if timestamp < now-maxClockSkewSeconds {
			return refuseS1(c, metrics.S1ReasonSkewPast, fiber.StatusUnauthorized, "Request timestamp expired or invalid")
		}
		if timestamp > now+maxClockSkewSeconds {
			return refuseS1(c, metrics.S1ReasonSkewFuture, fiber.StatusUnauthorized, "Request timestamp expired or invalid")
		}

		// Decode signature. Part of the request's shape: checked before any agent is read,
		// so a malformed signature gets the same answer whichever agent it names.
		signatureBytes, err := base64.StdEncoding.DecodeString(signatureB64)
		if err != nil {
			return refuseS1(c, metrics.S1ReasonSignatureMalformed, fiber.StatusUnauthorized, "Invalid signature format")
		}

		// Reconstruct the signed message
		// Format: METHOD\nENDPOINT\nTIMESTAMP\n[BODY]
		// CRITICAL: Use OriginalURL() to include query parameters in signature verification
		// Python SDK signs endpoint with query params, so we must include them here
		method := strings.ToUpper(c.Method())
		path := c.OriginalURL() // Use full URL with query params, not c.Path() which strips them

		messageParts := []string{method, path, timestampStr}

		// Add body if present (for POST/PUT requests)
		if len(c.Body()) > 0 {
			// CRITICAL: SDK already sends JSON with sorted keys (Python's json.dumps(sort_keys=True))
			// Use the original body as-is to preserve exact formatting including number precision
			bodyStr := string(c.Body())
			messageParts = append(messageParts, bodyStr)
		}

		message := strings.Join(messageParts, "\n")

		// SECURITY: only the agent's registered key is read before the signature verifies.
		// The key MUST be registered: a key the request supplies itself in X-Public-Key is
		// never trusted (trust on first use); key registration happens through
		// authenticated channels (JWT auth). An unknown agent, an agent with no registered
		// key, a registered key that does not decode, and a presented key that is not the
		// registered one all get the one refusal, so this answer tells a caller who holds
		// only an agent id nothing about that agent.
		keys := agentauth.KeySet(c.Context(), agentService, agentID)
		verified, err := keys.VerifyEd25519(publicKeyB64, []byte(message), signatureBytes)
		if err != nil {
			refusal := agentKeyRefusal(err, "Invalid signature")
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

		// Signature is valid! Set agent context for handlers
		c.Locals("agent_id", agentID)
		c.Locals("organization_id", agent.OrganizationID)
		c.Locals("authenticated_via", "ed25519")
		c.Locals("auth_method", "ed25519") // Set auth_method so handlers can recognize Ed25519 auth

		return c.Next()
	}
}
