package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"golang.org/x/crypto/bcrypt"
)

// autoInitialize checks if this is a fresh deployment and initializes everything automatically
func autoInitialize(db *sql.DB) error {
	// Check if database is already initialized
	if isInitialized(db) {
		log.Println("ℹ️  Database already initialized, skipping auto-initialization")
		return nil
	}

	log.Println("🚀 First run detected - initializing AIM...")

	// Step 1: Apply complete schema
	if err := applyCompleteSchema(db); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}

	// Step 2: Create admin user and organization
	if err := createBootstrapData(db); err != nil {
		return fmt.Errorf("failed to create bootstrap data: %w", err)
	}

	// Step 3: Seed default security policies
	if err := seedDefaults(db); err != nil {
		return fmt.Errorf("failed to seed defaults: %w", err)
	}

	// Step 4: Mark as initialized
	if err := markInitialized(db); err != nil {
		return fmt.Errorf("failed to mark initialized: %w", err)
	}

	log.Println("✅ AIM initialized successfully!")
	return nil
}

// isInitialized checks if database has been initialized (system_config table exists with bootstrap_completed=true)
func isInitialized(db *sql.DB) bool {
	// Check if system_config table exists
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'system_config'
		)
	`
	if err := db.QueryRow(query).Scan(&exists); err != nil {
		return false
	}

	if !exists {
		return false
	}

	// Check if bootstrap_completed is true
	var value string
	query = `SELECT value FROM system_config WHERE key = 'bootstrap_completed'`
	if err := db.QueryRow(query).Scan(&value); err != nil {
		return false
	}

	return value == "true"
}

// applyCompleteSchema applies the complete database schema for fresh deployments
func applyCompleteSchema(db *sql.DB) error {
	log.Println("   📊 Applying complete database schema...")

	// Read complete schema file
	schemaPath := filepath.Join("schema", "complete_schema.sql")
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read complete_schema.sql: %w", err)
	}

	// Execute schema in a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema: %w", err)
	}

	log.Println("   ✅ Database schema applied")
	return nil
}

// createBootstrapData creates the initial admin user and organization
func createBootstrapData(db *sql.DB) error {
	log.Println("   👤 Creating admin user and organization...")

	// Get configuration from environment (with sensible defaults)
	adminEmail := getEnvOrDefault("ADMIN_EMAIL", "admin@localhost")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		// SECURITY: Generate a cryptographically random password instead of using a static default
		randomBytes := make([]byte, 24)
		if _, err := rand.Read(randomBytes); err != nil {
			return fmt.Errorf("failed to generate random admin password: %w", err)
		}
		adminPassword = base64.URLEncoding.EncodeToString(randomBytes)
		log.Println("Generated random admin password. Set ADMIN_PASSWORD env var to use a known password.")
		log.Printf("  Temporary admin password: %s...%s", adminPassword[:4], adminPassword[len(adminPassword)-4:])
	}
	adminName := getEnvOrDefault("ADMIN_NAME", "System Administrator")
	orgName := getEnvOrDefault("ORG_NAME", "Default Organization")
	orgDomain := getEnvOrDefault("ORG_DOMAIN", "localhost")

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Execute bootstrap in a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Create organization
	var orgID string
	query := `
		INSERT INTO organizations (name, domain, plan_type, max_agents, max_users, is_active)
		VALUES ($1, $2, 'enterprise', 1000, 100, true)
		RETURNING id
	`
	if err := tx.QueryRow(query, orgName, orgDomain).Scan(&orgID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create organization: %w", err)
	}

	// Create admin user
	var userID string
	query = `
		INSERT INTO users (
			organization_id, email, name, role, provider, provider_id,
			password_hash, email_verified, force_password_change, status
		)
		VALUES ($1, $2, $3, 'admin', 'local', $4, $5, true, false, 'active')
		RETURNING id
	`
	if err := tx.QueryRow(query, orgID, adminEmail, adminName, adminEmail, string(passwordHash)).Scan(&userID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bootstrap data: %w", err)
	}

	log.Printf("   ✅ Created organization: %s", orgName)
	log.Printf("   ✅ Created admin user: %s", adminEmail)

	return nil
}

// seedDefaults creates default security policies and other initial data
func seedDefaults(db *sql.DB) error {
	log.Println("   🔐 Creating default security policies...")

	// Read seed data file
	seedPath := filepath.Join("seed", "default_security_policies.sql")
	content, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("failed to read seed file: %w", err)
	}

	// Execute seed data
	if _, err := db.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to execute seed data: %w", err)
	}

	log.Println("   ✅ Default security policies created")
	return nil
}

// markInitialized sets bootstrap_completed flag in system_config
func markInitialized(db *sql.DB) error {
	query := `
		INSERT INTO system_config (key, value, description)
		VALUES ('bootstrap_completed', 'true', 'Indicates successful initial setup')
		ON CONFLICT (key) DO UPDATE SET value = 'true'
	`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to mark initialized: %w", err)
	}

	return nil
}

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
