package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// APIKeyMiddleware validates API keys from Authorization header or X-API-Key header
// Used for SDK authentication and direct API calls
func APIKeyMiddleware(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		println("DEBUG: APIKeyMiddleware checking path:", path)

		// Skip API key middleware for verification endpoints (use signature auth instead)
		if path == "/api/v1/sdk-api/verifications" ||
			strings.HasPrefix(path, "/api/v1/sdk-api/verifications/") {
			println("DEBUG: Skipping API key check for verification endpoint")
			return c.Next()
		}

		println("DEBUG: API key middleware running for path:", path)
		var apiKey string

		// Try Authorization header first (Bearer token format)
		authHeader := c.Get("Authorization")
		println("DEBUG: Authorization header:", authHeader)
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				apiKey = parts[1]
				println("DEBUG: Extracted API key:", apiKey[:10]+"...")
			}
		}

		// Fallback to X-API-Key header
		if apiKey == "" {
			apiKey = c.Get("X-API-Key")
		}

		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No API key provided",
			})
		}

		// Hash the API key
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Look up API key in database and get the user who created it
	var keyData struct {
		ID             uuid.UUID  `db:"id"`
		OrganizationID uuid.UUID  `db:"organization_id"`
		AgentID        uuid.UUID  `db:"agent_id"`
		UserID         uuid.UUID  `db:"user_id"`
		Name           string     `db:"name"`
		IsActive       bool       `db:"is_active"`
		ExpiresAt      *time.Time `db:"expires_at"`
	}

	query := `
		SELECT ak.id, ak.organization_id, ak.agent_id, ak.created_by as user_id, ak.name, ak.is_active, ak.expires_at
		FROM api_keys ak
		WHERE ak.key_hash = $1
		LIMIT 1
	`

	err := db.QueryRow(query, keyHash).Scan(
		&keyData.ID,
		&keyData.OrganizationID,
		&keyData.AgentID,
		&keyData.UserID,
		&keyData.Name,
		&keyData.IsActive,
		&keyData.ExpiresAt,
	)

		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key",
			})
		}

		// Check if key is active
		if !keyData.IsActive {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key is inactive",
			})
		}

		// Check if key is expired
		if keyData.ExpiresAt != nil && keyData.ExpiresAt.Before(time.Now()) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key has expired",
			})
		}

		// Update last_used_at timestamp
		updateQuery := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
		_, _ = db.Exec(updateQuery, keyData.ID)

	// Set context for downstream handlers
	c.Locals("api_key_id", keyData.ID)
	c.Locals("organization_id", keyData.OrganizationID)
	c.Locals("agent_id", keyData.AgentID)
	c.Locals("user_id", keyData.UserID) // ✅ Set user_id for capability requests
	c.Locals("auth_method", "api_key")

	return c.Next()
	}
}

// OptionalAPIKeyMiddleware is like APIKeyMiddleware but doesn't fail if no API key
// Useful for endpoints that work both authenticated and unauthenticated
func OptionalAPIKeyMiddleware(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var apiKey string

		// Try Authorization header first
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				apiKey = parts[1]
			}
		}

		// Fallback to X-API-Key header
		if apiKey == "" {
			apiKey = c.Get("X-API-Key")
		}

		// If no API key, continue without setting context
		if apiKey == "" {
			return c.Next()
		}

		// Hash the API key
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Look up API key
	var keyData struct {
		ID             uuid.UUID  `db:"id"`
		OrganizationID uuid.UUID  `db:"organization_id"`
		AgentID        uuid.UUID  `db:"agent_id"`
		UserID         uuid.UUID  `db:"user_id"`
		Name           string     `db:"name"`
		IsActive       bool       `db:"is_active"`
		ExpiresAt      *time.Time `db:"expires_at"`
	}

	query := `
		SELECT ak.id, ak.organization_id, ak.agent_id, ak.created_by as user_id, ak.name, ak.is_active, ak.expires_at
		FROM api_keys ak
		WHERE ak.key_hash = $1
		LIMIT 1
	`

	err := db.QueryRow(query, keyHash).Scan(
		&keyData.ID,
		&keyData.OrganizationID,
		&keyData.AgentID,
		&keyData.UserID,
		&keyData.Name,
		&keyData.IsActive,
		&keyData.ExpiresAt,
	)

		// If key not found or invalid, continue without auth
		if err != nil {
			return c.Next()
		}

		// Check if key is active and not expired
		if !keyData.IsActive || (keyData.ExpiresAt != nil && keyData.ExpiresAt.Before(time.Now())) {
			return c.Next()
		}

		// Update last_used_at
		updateQuery := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
		_, _ = db.Exec(updateQuery, keyData.ID)

	// Set context
	c.Locals("api_key_id", keyData.ID)
	c.Locals("organization_id", keyData.OrganizationID)
	c.Locals("agent_id", keyData.AgentID)
	c.Locals("user_id", keyData.UserID)
	c.Locals("auth_method", "api_key")

	return c.Next()
	}
}
