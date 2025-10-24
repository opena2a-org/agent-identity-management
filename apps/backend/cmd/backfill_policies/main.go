package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

// Backfill default security policies for existing organizations
func main() {
	log.Println("🔄 Starting security policy backfill for existing organizations...")

	// Initialize database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
	}

	log.Println("✅ Database connected")

	ctx := context.Background()

	// Get all organizations
	var organizations []struct {
		ID   string
		Name string
	}

	rows, err := db.QueryContext(ctx, "SELECT id, name FROM organizations")
	if err != nil {
		log.Fatalf("❌ Failed to fetch organizations: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var org struct {
			ID   string
			Name string
		}
		if err := rows.Scan(&org.ID, &org.Name); err != nil {
			log.Printf("⚠️  Failed to scan organization: %v", err)
			continue
		}
		organizations = append(organizations, org)
	}

	log.Printf("📊 Found %d organizations to check\n", len(organizations))

	backfilledCount := 0
	skippedCount := 0

	// For each organization, check if they have policies
	for _, org := range organizations {
		// Check if organization already has policies
		var existingCount int
		err = db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM security_policies WHERE organization_id = $1",
			org.ID,
		).Scan(&existingCount)

		if err != nil {
			log.Printf("⚠️  Failed to check policies for organization %s: %v", org.Name, err)
			continue
		}

		if existingCount > 0 {
			log.Printf("⏭️  Organization '%s' already has %d policies, skipping", org.Name, existingCount)
			skippedCount++
			continue
		}

		// Get an admin user for this organization to use as created_by
		var adminUserID string
		err = db.QueryRowContext(ctx, `
			SELECT id FROM users
			WHERE organization_id = $1 AND role = 'admin'
			ORDER BY created_at ASC
			LIMIT 1
		`, org.ID).Scan(&adminUserID)

		if err != nil {
			log.Printf("⚠️  Failed to find admin user for organization %s: %v", org.Name, err)
			continue
		}

		// Create default policies for this organization
		log.Printf("➕ Creating default policies for organization: %s (admin user: %s)", org.Name, adminUserID)

		defaultPolicies := []struct {
			Name              string
			Description       string
			PolicyType        string
			EnforcementAction string
			SeverityThreshold string
			Rules             string
			AppliesTo         string
			IsEnabled         bool
			Priority          int
		}{
			{
				Name:              "Capability Violation Detection",
				Description:       "Alerts when agents attempt actions beyond their defined capabilities (e.g., EchoLeak attacks)",
				PolicyType:        "capability_violation",
				EnforcementAction: "alert_only",
				SeverityThreshold: "high",
				Rules:             `{"check_capability_match":true,"block_unauthorized":false}`,
				AppliesTo:         "all_agents",
				IsEnabled:         true,
				Priority:          100,
			},
			{
				Name:              "Low Trust Score Monitoring",
				Description:       "Monitors agents with trust scores below threshold for suspicious behavior",
				PolicyType:        "trust_score_low",
				EnforcementAction: "alert_only",
				SeverityThreshold: "medium",
				Rules:             `{"trust_threshold":70.0,"monitor_low_trust":true,"block_low_trust":false}`,
				AppliesTo:         "all_agents",
				IsEnabled:         true,
				Priority:          90,
			},
			{
				Name:              "Unusual Activity Detection",
				Description:       "Detects anomalous patterns in agent behavior (rate limits, unusual timing, etc.)",
				PolicyType:        "unusual_activity",
				EnforcementAction: "alert_only",
				SeverityThreshold: "medium",
				Rules:             `{"rate_limit_threshold":100,"detect_anomalies":true,"block_anomalies":false}`,
				AppliesTo:         "all_agents",
				IsEnabled:         true,
				Priority:          80,
			},
		}

		// Insert policies
		for _, policy := range defaultPolicies {
			_, err = db.ExecContext(ctx, `
				INSERT INTO security_policies (
					organization_id, name, description, policy_type,
					enforcement_action, severity_threshold, rules,
					applies_to, is_enabled, priority, created_by
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`,
				org.ID,
				policy.Name,
				policy.Description,
				policy.PolicyType,
				policy.EnforcementAction,
				policy.SeverityThreshold,
				policy.Rules,
				policy.AppliesTo,
				policy.IsEnabled,
				policy.Priority,
				adminUserID,
			)

			if err != nil {
				log.Printf("❌ Failed to create policy '%s' for organization %s: %v", policy.Name, org.Name, err)
				continue
			}

			log.Printf("   ✅ Created policy: %s (priority: %d, enforcement: %s)",
				policy.Name, policy.Priority, policy.EnforcementAction)
		}

		backfilledCount++
	}

	log.Println("\n" + strings.Repeat("=", 60))
	log.Printf("✅ Backfill complete!")
	log.Printf("📊 Organizations processed: %d", len(organizations))
	log.Printf("➕ Organizations backfilled: %d", backfilledCount)
	log.Printf("⏭️  Organizations skipped: %d", skippedCount)
	log.Println(strings.Repeat("=", 60))
}
