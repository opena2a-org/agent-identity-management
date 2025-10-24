package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

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
	content, err := ioutil.ReadFile(schemaPath)
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
	adminPassword := getEnvOrDefault("ADMIN_PASSWORD", "admin123456") // Change in production!
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
	if adminPassword == "admin123456" {
		log.Println("   ⚠️  WARNING: Using default admin password - CHANGE IN PRODUCTION!")
	}

	return nil
}

// seedDefaults creates default security policies and other initial data
func seedDefaults(db *sql.DB) error {
	log.Println("   🔐 Creating default security policies...")

	// Read seed data file
	seedPath := filepath.Join("seed", "default_security_policies.sql")
	content, err := ioutil.ReadFile(seedPath)
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

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
