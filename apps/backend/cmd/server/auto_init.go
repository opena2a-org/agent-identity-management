package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/seed"
)

// adminSeedOrgDomain is the organization migration 013 creates and
// `aim-bootstrap --default` seeds its administrator into.
const adminSeedOrgDomain = "admin.opena2a.org"

// seedAdmin creates the first administrator from ADMIN_EMAIL and
// ADMIN_PASSWORD (admin) when the database has no administrator and no account
// with that email. It never changes an existing account, so a password changed
// in the dashboard survives a restart with ADMIN_PASSWORD still set. Like
// `aim-bootstrap --default`, the seeded password must be changed at first sign-in.
//
// The start that creates the administrator also adds seed.DefaultPolicies to
// its organization, created by that administrator, in the transaction that
// inserts it, as `aim-bootstrap --default` does. Migrations 015 and 052 add
// those policies only when admin@opena2a.org already exists, and after the
// migrations a fresh database has no user until this seed or the bootstrap
// creates one. A start that finds an administrator adds none, so a policy an
// administrator deleted or renamed is not added back.
func seedAdmin(db *sql.DB, admin config.AdminSeedConfig) error {
	adminPassword := admin.Password
	if adminPassword == "" {
		return nil
	}

	adminEmail := admin.Email

	var exists bool
	if err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM users WHERE role = 'admin' OR LOWER(email) = LOWER($1))`,
		adminEmail,
	).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check for an existing administrator: %w", err)
	}
	if exists {
		log.Printf("ADMIN_PASSWORD not applied: an administrator or an account for %s already exists. It only seeds the first administrator; change passwords from the dashboard.", adminEmail)
		return nil
	}

	passwordHash, err := auth.NewPasswordHasher().HashPassword(adminPassword)
	if err != nil {
		return fmt.Errorf("ADMIN_PASSWORD was not used to seed %s: %w", adminEmail, err)
	}

	var orgID uuid.UUID
	if err := db.QueryRow(`SELECT id FROM organizations WHERE domain = $1`, adminSeedOrgDomain).Scan(&orgID); err != nil {
		return fmt.Errorf("failed to find the %s organization to seed %s into: %w", adminSeedOrgDomain, adminEmail, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to seed administrator %s: %w", adminEmail, err)
	}
	defer func() { _ = tx.Rollback() }()

	userID := uuid.New()
	result, err := tx.Exec(`
		INSERT INTO users (
			id, organization_id, email, name, role, provider, provider_id,
			password_hash, status, email_verified, force_password_change, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 'admin', 'local', $5, $6, 'active', TRUE, TRUE, NOW(), NOW()
		)
		ON CONFLICT (organization_id, email) DO NOTHING
	`,
		userID, orgID, adminEmail, admin.Name,
		fmt.Sprintf("local-%s", userID), passwordHash,
	)
	if err != nil {
		return fmt.Errorf("failed to seed administrator %s: %w", adminEmail, err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to seed administrator %s: %w", adminEmail, err)
	}

	var policies int64
	if created > 0 {
		policies, err = seed.AddDefaultPolicies(context.Background(), tx, orgID, userID)
		if err != nil {
			return fmt.Errorf("failed to add the default security policies to the %s organization: %w", adminSeedOrgDomain, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to seed administrator %s: %w", adminEmail, err)
	}

	if created > 0 {
		log.Printf("Seeded administrator %s from ADMIN_PASSWORD (change the password at first sign-in)", adminEmail)
		if present := int64(len(seed.DefaultPolicies)) - policies; present > 0 {
			log.Printf("Added %d default security policies to the %s organization; %d it already had are left unchanged", policies, adminSeedOrgDomain, present)
		} else {
			log.Printf("Added %d default security policies to the %s organization", policies, adminSeedOrgDomain)
		}
	}
	return nil
}
