package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

const (
	banner = `
 █████╗ ██╗███╗   ███╗    ██████╗  ██████╗  ██████╗ ████████╗███████╗████████╗██████╗  █████╗ ██████╗
██╔══██╗██║████╗ ████║    ██╔══██╗██╔═══██╗██╔═══██╗╚══██╔══╝██╔════╝╚══██╔══╝██╔══██╗██╔══██╗██╔══██╗
███████║██║██╔████╔██║    ██████╔╝██║   ██║██║   ██║   ██║   ███████╗   ██║   ██████╔╝███████║██████╔╝
██╔══██║██║██║╚██╔╝██║    ██╔══██╗██║   ██║██║   ██║   ██║   ╚════██║   ██║   ██╔══██╗██╔══██║██╔═══╝
██║  ██║██║██║ ╚═╝ ██║    ██████╔╝╚██████╔╝╚██████╔╝   ██║   ███████║   ██║   ██║  ██║██║  ██║██║
╚═╝  ╚═╝╚═╝╚═╝     ╚═╝    ╚═════╝  ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝

Agent Identity Management - Initial Setup
`
)

type BootstrapConfig struct {
	AdminEmail    string
	AdminPassword string
	AdminName     string
	OrgName       string
	OrgDomain     string
	MaxUsers      int
	MaxAgents     int
	DatabaseURL   string
	SkipPrompts   bool
}

func main() {
	// Load environment variables
	_ = godotenv.Load()

	// Parse command line flags
	config := &BootstrapConfig{}
	flag.StringVar(&config.AdminEmail, "admin-email", "", "Admin user email address")
	flag.StringVar(&config.AdminPassword, "admin-password", "", "Admin user password")
	flag.StringVar(&config.AdminName, "admin-name", "System Administrator", "Admin user display name")
	flag.StringVar(&config.OrgName, "org-name", "", "Organization name")
	flag.StringVar(&config.OrgDomain, "org-domain", "localhost", "Organization domain")
	flag.IntVar(&config.MaxUsers, "max-users", 100, "Maximum users allowed")
	flag.IntVar(&config.MaxAgents, "max-agents", 1000, "Maximum agents allowed")
	flag.StringVar(&config.DatabaseURL, "database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	flag.BoolVar(&config.SkipPrompts, "yes", false, "Skip confirmation prompts")
	flag.Parse()

	// Print banner
	fmt.Print(banner)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ Configuration error: %v", err)
	}

	// Connect to database
	fmt.Println("📊 Connecting to database...")
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
	}

	// Check if bootstrap already completed
	if isBootstrapped(db) {
		fmt.Println("⚠️  System already bootstrapped!")
		if !config.SkipPrompts {
			fmt.Print("Do you want to create another admin user? (yes/no): ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "yes" && strings.ToLower(response) != "y" {
				fmt.Println("❌ Bootstrap cancelled")
				return
			}
		}
	}

	// Show configuration summary
	fmt.Println("\n📋 Bootstrap Configuration:")
	fmt.Printf("   • Admin Email:    %s\n", config.AdminEmail)
	fmt.Printf("   • Admin Name:     %s\n", config.AdminName)
	fmt.Printf("   • Organization:   %s\n", config.OrgName)
	fmt.Printf("   • Domain:         %s\n", config.OrgDomain)
	fmt.Printf("   • Max Users:      %d\n", config.MaxUsers)
	fmt.Printf("   • Max Agents:     %d\n", config.MaxAgents)

	// Confirm
	if !config.SkipPrompts {
		fmt.Print("\n⚠️  This will create the initial admin user and organization. Continue? (yes/no): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "yes" && strings.ToLower(response) != "y" {
			fmt.Println("❌ Bootstrap cancelled")
			return
		}
	}

	// Run bootstrap
	fmt.Println("\n🚀 Starting bootstrap process...")

	if err := runBootstrap(context.Background(), db, config); err != nil {
		log.Fatalf("❌ Bootstrap failed: %v", err)
	}

	fmt.Println("\n✅ Bootstrap completed successfully!")
	fmt.Printf("\n🔐 Admin Credentials:\n")
	fmt.Printf("   Email:    %s\n", config.AdminEmail)
	fmt.Printf("   Password: %s\n", config.AdminPassword)
	fmt.Printf("\n🌐 You can now log in at: http://localhost:3000/login\n")
	fmt.Println("\n⚠️  IMPORTANT: Please change the admin password after first login!")
}

func validateConfig(config *BootstrapConfig) error {
	if config.AdminEmail == "" {
		return fmt.Errorf("admin email is required (use --admin-email)")
	}

	if config.AdminPassword == "" {
		return fmt.Errorf("admin password is required (use --admin-password)")
	}

	if config.OrgName == "" {
		return fmt.Errorf("organization name is required (use --org-name)")
	}

	if config.DatabaseURL == "" {
		return fmt.Errorf("database URL is required (use --database-url or set DATABASE_URL env var)")
	}

	// Validate password strength
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.ValidatePassword(config.AdminPassword); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	return nil
}

func isBootstrapped(db *sql.DB) bool {
	var value string
	query := `SELECT value FROM system_config WHERE key = 'bootstrap_completed'`
	err := db.QueryRow(query).Scan(&value)
	if err != nil {
		return false
	}
	return value == "true"
}

func runBootstrap(ctx context.Context, db *sql.DB, config *BootstrapConfig) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Check if organization exists
	fmt.Println("1️⃣  Checking organization...")
	var orgID uuid.UUID
	query := `SELECT id FROM organizations WHERE domain = $1`
	err = tx.QueryRow(query, config.OrgDomain).Scan(&orgID)

	if err != nil {
		// Organization doesn't exist, create it
		fmt.Printf("   Creating organization '%s'...\n", config.OrgName)
		orgID = uuid.New()
		query = `
			INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err = tx.Exec(query, orgID, config.OrgName, config.OrgDomain, "enterprise", config.MaxAgents, config.MaxUsers, true)
		if err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}
		fmt.Println("   ✓ Organization created")
	} else {
		fmt.Printf("   ✓ Organization exists (ID: %s)\n", orgID)
	}

	// 2. Hash password
	fmt.Println("2️⃣  Hashing password...")
	passwordHasher := auth.NewPasswordHasher()
	passwordHash, err := passwordHasher.HashPassword(config.AdminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	fmt.Println("   ✓ Password hashed")

	// 3. Create admin user
	fmt.Println("3️⃣  Creating admin user...")
	userID := uuid.New()
	providerID := fmt.Sprintf("local-%s", userID.String())

	query = `
		INSERT INTO users (
			id, organization_id, email, name, role, provider, provider_id,
			password_hash, email_verified, force_password_change, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW()
		)
		ON CONFLICT (organization_id, email) DO UPDATE
		SET role = $5, password_hash = $8, email_verified = $9, force_password_change = $10, updated_at = NOW()
		RETURNING id
	`

	err = tx.QueryRow(query,
		userID,
		orgID,
		config.AdminEmail,
		config.AdminName,
		domain.RoleAdmin,
		"local",
		providerID,
		passwordHash,
		true,  // email_verified
		true,  // force_password_change - user must change default password
	).Scan(&userID)

	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	fmt.Printf("   ✓ Admin user created (ID: %s)\n", userID)

	// 4. Mark bootstrap as completed
	fmt.Println("4️⃣  Updating system configuration...")
	query = `
		INSERT INTO system_config (key, value, description, updated_at)
		VALUES ('bootstrap_completed', 'true', 'Initial admin bootstrap completed', NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = 'true', updated_at = NOW()
	`
	_, err = tx.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to update system config: %w", err)
	}
	fmt.Println("   ✓ System configuration updated")

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
