package middleware

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a/identity/backend/internal/application"
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
	result = strings.ReplaceAll(result, "\":", "\": ")  // Add space after colon
	result = strings.ReplaceAll(result, ",\"", ", \"")  // Add space after comma before quote
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
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid agent ID format",
			})
		}

		// Validate timestamp (prevent replay attacks)
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid timestamp format",
			})
		}

		now := time.Now().Unix()
		// Allow 5 minutes clock skew
		if timestamp < now-300 || timestamp > now+300 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Request timestamp expired or invalid",
			})
		}

		// Load agent from database
		agent, err := agentService.GetAgent(c.Context(), agentID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Agent not found",
			})
		}

		// Check if agent has a registered public key
		var verifyPublicKey string
		if agent.PublicKey != nil && *agent.PublicKey != "" {
			// Use registered key from database
			verifyPublicKey = *agent.PublicKey
			fmt.Printf("🔑 Using REGISTERED public key from database (first 20): %s...\n", verifyPublicKey[:20])
		} else {
			// Agent hasn't registered a key yet, use the one from request
			// (This allows first-time registration)
			verifyPublicKey = publicKeyB64
			fmt.Printf("🔑 Using REQUEST public key (first 20): %s...\n", publicKeyB64[:20])
		}
		fmt.Printf("🔑 Request sent public key (first 20): %s...\n", publicKeyB64[:20])

		// Decode public key
		publicKeyBytes, err := base64.StdEncoding.DecodeString(verifyPublicKey)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid public key format",
			})
		}

		if len(publicKeyBytes) != ed25519.PublicKeySize {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(publicKeyBytes)),
			})
		}

		publicKey := ed25519.PublicKey(publicKeyBytes)

		// Decode signature
		signatureBytes, err := base64.StdEncoding.DecodeString(signatureB64)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid signature format",
			})
		}

		// Reconstruct the signed message
		// Format: METHOD\nENDPOINT\nTIMESTAMP\n[BODY]
		method := strings.ToUpper(c.Method())
		path := c.Path()

		messageParts := []string{method, path, timestampStr}

		// Add body if present (for POST/PUT requests)
		if len(c.Body()) > 0 {
			// CRITICAL: SDK already sends JSON with sorted keys (Python's json.dumps(sort_keys=True))
			// Use the original body as-is to preserve exact formatting including number precision
			bodyStr := string(c.Body())
			fmt.Printf("🔍 Backend using original body: %s\n", bodyStr[:200])
			messageParts = append(messageParts, bodyStr)
		}

		message := strings.Join(messageParts, "\n")
		msgPreview := message
		if len(message) > 500 {
			msgPreview = message[:500]
		}
		fmt.Printf("🔍 Backend verifying message (first 500 chars):\n%s\n", msgPreview)

		// Verify Ed25519 signature
		if !ed25519.Verify(publicKey, []byte(message), signatureBytes) {
			// Debug logging for signature verification failure
			fmt.Printf("❌ Ed25519 signature verification FAILED\n")
			fmt.Printf("   Agent ID: %s\n", agentID)
			fmt.Printf("   Timestamp: %s\n", timestampStr)
			fmt.Printf("   Message to verify:\n%s\n", message)
			fmt.Printf("   Public key (first 20 chars): %s...\n", verifyPublicKey[:20])
			fmt.Printf("   Signature (first 20 chars): %s...\n", signatureB64[:20])

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid signature",
			})
		}

		fmt.Printf("✅ Ed25519 signature verification PASSED for agent %s\n", agentID)

		// Signature is valid! Set agent context for handlers
		c.Locals("agent_id", agentID)
		c.Locals("organization_id", agent.OrganizationID)
		c.Locals("authenticated_via", "ed25519")
		c.Locals("auth_method", "ed25519") // Set auth_method so handlers can recognize Ed25519 auth

		return c.Next()
	}
}
