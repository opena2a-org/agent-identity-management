package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Canonical defaults for the seeded OpenA2A admin org and user. Domain is the
// stable identifier (UNIQUE on organizations); migrations 013 and 024 look up
// the row by this value.
const (
	defaultAdminEmail = "admin@opena2a.org"
	defaultAdminName  = "System Administrator"
	defaultOrgName    = "OpenA2A Admin"
	defaultOrgDomain  = "admin.opena2a.org"
	defaultMaxUsers   = 1000
	defaultMaxAgents  = 10000
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
	// Default: fill empty fields with canonical OpenA2A admin values, generate
	// a random AdminPassword if empty, and skip prompts. Idempotent: if the
	// system_config bootstrap_completed marker is already set, the run exits
	// successfully without touching the existing admin row.
	Default bool
	// passwordWasGenerated is set to true when applyDefaults generates a random
	// password. The final credential block highlights that the operator must
	// capture the password from this run's stdout.
	passwordWasGenerated bool
	// adminAlreadyExisted is set to true when runBootstrap's user INSERT hit
	// the ON CONFLICT DO NOTHING branch in --default mode. The stored
	// password does NOT match this run's hashed password, so the credential
	// block must be suppressed to avoid handing the operator a wrong
	// password.
	adminAlreadyExisted bool
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
	flag.BoolVar(&config.Default, "default", false, "Seed the canonical OpenA2A admin org+admin with a random password (idempotent). Fills in canonical values for unset --admin-*/--org-*/--max-* flags; generates a random --admin-password if empty. Implies --yes and exits 0 without touching existing state if already bootstrapped.")
	flag.Parse()

	// Print banner
	fmt.Print(banner)

	// In --default mode, fill in canonical values for unset fields and generate
	// a random password if --admin-password was not provided. This runs before
	// validateConfig so the validator sees populated values.
	if config.Default {
		if err := applyDefaultBootstrapValues(config); err != nil {
			log.Fatalf("❌ Failed to prepare default bootstrap values: %v", err)
		}
	}

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

	// Check if bootstrap already completed.
	// In --default mode this is the idempotency check: if the bootstrap_completed
	// marker is set, exit successfully without printing credentials or touching
	// the existing admin row. The Docker entrypoint can call `aim-bootstrap --default`
	// on every container start; subsequent runs become no-ops.
	if isBootstrapped(db) {
		if config.Default {
			fmt.Println("✓ Default admin already seeded — skipping (idempotent re-run)")
			return
		}
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
	if config.adminAlreadyExisted {
		// --default mode hit ON CONFLICT DO NOTHING. The password we hashed
		// for this run was NEVER stored. Printing config.AdminPassword would
		// lock out the operator. Suppress the credential block.
		fmt.Printf("\nℹ️  Admin user for %s already existed in this org. The bootstrap_completed marker is set; nothing else to do.\n", config.AdminEmail)
		fmt.Println("    If you have lost the original admin password, use the password-reset flow rather than re-running bootstrap.")
		return
	}
	fmt.Printf("\n🔐 Admin Credentials:\n")
	fmt.Printf("   Email:    %s\n", config.AdminEmail)
	fmt.Printf("   Password: %s\n", config.AdminPassword)
	if config.passwordWasGenerated {
		fmt.Println("\n⚠️  The password above was randomly generated by --default mode.")
		fmt.Println("    CAPTURE IT NOW — it is not stored anywhere and will not be re-printed.")
	}
	fmt.Printf("\n🌐 You can now log in at: http://localhost:3000/login\n")
	fmt.Println("\n⚠️  IMPORTANT: Please change the admin password after first login!")
}

// applyDefaultBootstrapValues fills in empty BootstrapConfig fields with the
// canonical OpenA2A admin values and generates a random password if none was
// supplied. Called only when --default is set. Mutates *config in place.
//
// Defaults are conservative: existing operator-supplied flag values are
// preserved (so an operator can run `aim-bootstrap --default --org-name=X` to
// override only the org name while keeping random password and canonical
// email).
func applyDefaultBootstrapValues(config *BootstrapConfig) error {
	if config.AdminEmail == "" {
		config.AdminEmail = defaultAdminEmail
	}
	// AdminName already defaults to "System Administrator" via flag.StringVar.
	// Treat the flag default as canonical; no override needed here.
	if config.OrgName == "" {
		config.OrgName = defaultOrgName
	}
	// OrgDomain defaults to "localhost" via flag.StringVar — replace with the
	// canonical admin domain when the operator didn't override.
	if config.OrgDomain == "" || config.OrgDomain == "localhost" {
		config.OrgDomain = defaultOrgDomain
	}
	// MaxUsers/MaxAgents flag defaults are 100/1000. The canonical admin org
	// uses higher caps; raise only if the operator didn't override.
	if config.MaxUsers == 100 {
		config.MaxUsers = defaultMaxUsers
	}
	if config.MaxAgents == 1000 {
		config.MaxAgents = defaultMaxAgents
	}
	config.SkipPrompts = true

	if config.AdminPassword == "" {
		// Operators can pre-set DEFAULT_ADMIN_PASSWORD in the environment so
		// the password is captured outside the migrate log (e.g. when sourcing
		// from scripts/gen-dev-secrets.sh into .env). Falls back to a fresh
		// random password if the env var is unset.
		if envPw := os.Getenv("DEFAULT_ADMIN_PASSWORD"); envPw != "" {
			config.AdminPassword = envPw
		} else {
			pw, err := generateRandomPassword()
			if err != nil {
				return fmt.Errorf("generate random password: %w", err)
			}
			config.AdminPassword = pw
			config.passwordWasGenerated = true
		}
	}
	return nil
}

// generateRandomPassword returns a 32-character password that satisfies the
// PasswordHasher.ValidatePassword rules (upper + lower + digit + special).
// One character at fixed indices 0-3 guarantees each class is present; the
// remaining 28 characters are drawn from the union set. A Fisher-Yates
// shuffle randomises position so the guarantee is not observable.
//
// Look-alike characters (I, O, l, 0, 1) are excluded to reduce capture errors
// when an operator reads the password from a deploy log. Effective alphabet is
// ~71 characters across the four classes.
func generateRandomPassword() (string, error) {
	const length = 32
	const (
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		lower   = "abcdefghijkmnopqrstuvwxyz"
		digit   = "23456789"
		special = "!@#$%^&*-_+=?"
	)
	all := upper + lower + digit + special

	rnd := make([]byte, length*2)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}

	pw := make([]byte, length)
	pw[0] = upper[int(rnd[0])%len(upper)]
	pw[1] = lower[int(rnd[1])%len(lower)]
	pw[2] = digit[int(rnd[2])%len(digit)]
	pw[3] = special[int(rnd[3])%len(special)]
	for i := 4; i < length; i++ {
		pw[i] = all[int(rnd[i])%len(all)]
	}

	// Fisher-Yates shuffle with the remaining random bytes so class positions
	// are not fixed at indices 0-3.
	for i := length - 1; i > 0; i-- {
		j := int(rnd[length+i]) % (i + 1)
		pw[i], pw[j] = pw[j], pw[i]
	}

	return string(pw), nil
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

// isBootstrapped returns true only when the system_config row explicitly says
// so. A database error (connection failed, permission denied, system_config
// table missing because migrations haven't run) is treated as "not
// bootstrapped" — but unlike the previous version, the error is logged so
// operators can distinguish "fresh DB" from "DB is reachable but the marker
// is unset" from "DB is unreachable, every run is racing the previous one."
//
// Note: this check is necessary-but-not-sufficient for idempotency. The
// runBootstrap user INSERT uses ON CONFLICT (organization_id, email) DO
// NOTHING in --default mode so a concurrent run that bypasses this check
// cannot silently rotate an existing admin's password.
func isBootstrapped(db *sql.DB) bool {
	var value string
	query := `SELECT value FROM system_config WHERE key = 'bootstrap_completed'`
	err := db.QueryRow(query).Scan(&value)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		// Distinguish from sql.ErrNoRows so operators see the real reason.
		// Stays "not bootstrapped" to preserve the existing semantic; the
		// runBootstrap INSERT's ON CONFLICT clause is the actual guard.
		log.Printf("⚠️  isBootstrapped: DB error (treating as not-yet-bootstrapped): %v", err)
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

	// 3. Create admin user.
	//
	// Conflict policy depends on mode:
	//   --default (idempotent re-runs): ON CONFLICT DO NOTHING. If an admin
	//     row already exists for this org+email, the run leaves it alone,
	//     RowsAffected returns 0, and the credential block is NOT printed —
	//     the printed password would not match the stored password and would
	//     lock out the operator. This is the TOCTOU mitigation for two
	//     concurrent `--default` invocations both passing isBootstrapped.
	//   non-default (explicit re-init): ON CONFLICT DO UPDATE. Operators
	//     running `aim-bootstrap --admin-email=X --admin-password=Y` again
	//     mean it; rotate the stored password to the new value.
	//
	// status='active' is set explicitly here. The users table default is
	// 'pending' (apps/backend/migrations/001_initial_schema.sql:49) which
	// blocks downstream approval flows that expect the seeded admin to be
	// active out of the gate. The pre-B2 migration 013 v1 set this column
	// explicitly; the move to Go preserves that behavior.
	fmt.Println("3️⃣  Creating admin user...")
	userID := uuid.New()
	providerID := fmt.Sprintf("local-%s", userID.String())

	var insertQuery string
	if config.Default {
		insertQuery = `
			INSERT INTO users (
				id, organization_id, email, name, role, provider, provider_id,
				password_hash, status, email_verified, force_password_change, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $10, NOW(), NOW()
			)
			ON CONFLICT (organization_id, email) DO NOTHING
		`
	} else {
		insertQuery = `
			INSERT INTO users (
				id, organization_id, email, name, role, provider, provider_id,
				password_hash, status, email_verified, force_password_change, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $10, NOW(), NOW()
			)
			ON CONFLICT (organization_id, email) DO UPDATE
			SET role = $5, password_hash = $8, status = 'active', email_verified = $9, force_password_change = $10, updated_at = NOW()
		`
	}

	result, err := tx.Exec(insertQuery,
		userID,
		orgID,
		config.AdminEmail,
		config.AdminName,
		domain.RoleAdmin,
		"local",
		providerID,
		passwordHash,
		true, // email_verified
		true, // force_password_change - user must change default password
	)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read insert result: %w", err)
	}
	if rowsAffected == 0 {
		// --default mode hit the DO NOTHING branch — admin already exists.
		// Signal the caller (via config.passwordWasGenerated reset) that the
		// credential print should be suppressed; the stored password is NOT
		// the one we just hashed.
		fmt.Printf("   ✓ Admin user already exists for %s in this org — leaving stored password unchanged\n", config.AdminEmail)
		config.adminAlreadyExisted = true
	} else {
		fmt.Printf("   ✓ Admin user created (ID: %s)\n", userID)
	}

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
