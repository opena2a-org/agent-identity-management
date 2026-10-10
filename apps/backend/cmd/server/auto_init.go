package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// adminSeedOrgDomain is the organization migration 013 creates and
// `aim-bootstrap --default` seeds its administrator into.
const adminSeedOrgDomain = "admin.opena2a.org"

// seedAdminFromEnv creates the first administrator from ADMIN_EMAIL and
// ADMIN_PASSWORD when the database has no administrator and no account with
// that email. It never changes an existing account, so a password changed in
// the dashboard survives a restart with ADMIN_PASSWORD still set. Like
// `aim-bootstrap --default`, the seeded password must be changed at first sign-in.
func seedAdminFromEnv(db *sql.DB) error {
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		return nil
	}

	adminEmail := getEnvOrDefault("ADMIN_EMAIL", "admin@opena2a.org")

	var exists bool
	if err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users WHERE role = 'admin' OR LOWER(email) = LOWER($1))`,
		adminEmail,
	).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check for an existing administrator: %w", err)
	}
	if exists {
		log.Printf("ℹ️  ADMIN_PASSWORD not applied: an administrator or an account for %s already exists. It only seeds the first administrator; change passwords from the dashboard.", adminEmail)
		return nil
	}

	passwordHash, err := auth.NewPasswordHasher().HashPassword(adminPassword)
	if err != nil {
		return fmt.Errorf("ADMIN_PASSWORD was not used to seed %s: %w", adminEmail, err)
	}

	var orgID string
	if err := db.QueryRow(`SELECT id FROM organizations WHERE domain = $1`, adminSeedOrgDomain).Scan(&orgID); err != nil {
		return fmt.Errorf("failed to find the %s organization to seed %s into: %w", adminSeedOrgDomain, adminEmail, err)
	}

	userID := uuid.New()
	result, err := db.Exec(`
		INSERT INTO users (
			id, organization_id, email, name, role, provider, provider_id,
			password_hash, status, email_verified, force_password_change, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 'admin', 'local', $5, $6, 'active', TRUE, TRUE, NOW(), NOW()
		)
		ON CONFLICT (organization_id, email) DO NOTHING
	`,
		userID, orgID, adminEmail, getEnvOrDefault("ADMIN_NAME", "System Administrator"),
		fmt.Sprintf("local-%s", userID), passwordHash,
	)
	if err != nil {
		return fmt.Errorf("failed to seed administrator %s: %w", adminEmail, err)
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("✅ Seeded administrator %s from ADMIN_PASSWORD (change the password at first sign-in)", adminEmail)
	}
	return nil
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
