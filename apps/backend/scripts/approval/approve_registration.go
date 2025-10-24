package main

// To run this script: cd scripts/approval && go run approve_registration.go <email>

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run approve_registration.go <email>")
		fmt.Println("Example: go run approve_registration.go user@example.com")
		os.Exit(1)
	}

	email := os.Args[1]

	// Database connection - try multiple connection strings
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Use the same defaults as the server
		dbURL = "postgres://postgres:postgres@localhost:5432/identity?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Find pending registration request
	var requestID uuid.UUID
	var firstName, lastName string
	var oauthProvider, oauthUserID string
	var emailVerified bool
	var profilePictureURL sql.NullString

	err = db.QueryRowContext(ctx, `
		SELECT id, first_name, last_name, oauth_provider, oauth_user_id, oauth_email_verified, profile_picture_url
		FROM user_registration_requests 
		WHERE email = $1 AND status = 'pending'
		ORDER BY requested_at DESC
		LIMIT 1
	`, email).Scan(&requestID, &firstName, &lastName, &oauthProvider, &oauthUserID, &emailVerified, &profilePictureURL)

	if err == sql.ErrNoRows {
		log.Fatalf("No pending registration request found for email: %s", email)
	}
	if err != nil {
		log.Fatalf("Failed to find registration request: %v", err)
	}

	fmt.Printf("Found pending registration request:\n")
	fmt.Printf("  ID: %s\n", requestID)
	fmt.Printf("  Email: %s\n", email)
	fmt.Printf("  Name: %s %s\n", firstName, lastName)
	fmt.Printf("  Provider: %s\n", oauthProvider)

	// Find or create organization
	var orgID uuid.UUID
	err = db.QueryRowContext(ctx, `
		SELECT id FROM organizations 
		WHERE domain = $1 
		LIMIT 1
	`, getEmailDomain(email)).Scan(&orgID)

	if err == sql.ErrNoRows {
		// Create organization
		orgID = uuid.New()
		_, err = db.ExecContext(ctx, `
			INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active, auto_approve_sso, created_at, updated_at)
			VALUES ($1, $2, $3, 'free', 100, 10, true, true, NOW(), NOW())
		`, orgID, getEmailDomain(email), getEmailDomain(email))
		if err != nil {
			log.Fatalf("Failed to create organization: %v", err)
		}
		fmt.Printf("Created organization: %s\n", getEmailDomain(email))
	} else if err != nil {
		log.Fatalf("Failed to find organization: %v", err)
	}

	// Check if this is the first user (make them admin)
	var userCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE organization_id = $1
	`, orgID).Scan(&userCount)
	if err != nil {
		log.Fatalf("Failed to count users: %v", err)
	}

	role := "viewer"
	if userCount == 0 {
		role = "admin"
		fmt.Println("Making user admin (first user in organization)")
	}

	// Create user
	userID := uuid.New()
	fullName := firstName
	if lastName != "" {
		if fullName != "" {
			fullName += " "
		}
		fullName += lastName
	}
	if fullName == "" {
		fullName = email
	}

	now := time.Now()
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, organization_id, email, name, role, provider, provider_id, email_verified, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active', $10, $11)
	`, userID, orgID, email, fullName, role, oauthProvider, oauthUserID, emailVerified, profilePictureURL, now, now)

	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	// Update registration request to approved
	_, err = db.ExecContext(ctx, `
		UPDATE user_registration_requests 
		SET status = 'approved', reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, requestID)

	if err != nil {
		log.Fatalf("Failed to update registration request: %v", err)
	}

	fmt.Printf("✅ Successfully approved registration and created user!\n")
	fmt.Printf("   User ID: %s\n", userID)
	fmt.Printf("   Role: %s\n", role)
	fmt.Printf("   Organization: %s\n", orgID)
	fmt.Printf("\nYou can now log in with Google OAuth!\n")
}

func getEmailDomain(email string) string {
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			return email[i+1:]
		}
	}
	return email
}
