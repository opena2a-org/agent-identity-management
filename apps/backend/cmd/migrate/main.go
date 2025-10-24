package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	// ANSI color codes
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

type Migration struct {
	Version  string
	Filename string
	SQL      string
}

func main() {
	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable is required")
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
	}

	fmt.Printf("%s════════════════════════════════════════%s\n", colorCyan, colorReset)
	fmt.Printf("%s  AIM Database Migration System%s\n", colorCyan, colorReset)
	fmt.Printf("%s════════════════════════════════════════%s\n\n", colorCyan, colorReset)

	// Create schema_migrations table if it doesn't exist
	if err := ensureMigrationsTable(ctx, db); err != nil {
		log.Fatalf("❌ Failed to create migrations table: %v", err)
	}

	// Check if database is empty (fresh deployment)
	isFresh, err := isDatabaseFresh(ctx, db)
	if err != nil {
		log.Fatalf("❌ Failed to check database state: %v", err)
	}

	if isFresh {
		fmt.Printf("%s🆕 Fresh database detected%s\n", colorGreen, colorReset)
		fmt.Printf("   Using consolidated V1 schema for fast deployment\n\n")
		
		if err := applyConsolidatedSchema(ctx, db); err != nil {
			log.Fatalf("❌ Failed to apply consolidated schema: %v", err)
		}
	} else {
		fmt.Printf("%s📦 Existing database detected%s\n", colorYellow, colorReset)
		fmt.Printf("   Using incremental migrations\n\n")
		
		if err := applyIncrementalMigrations(ctx, db); err != nil {
			log.Fatalf("❌ Failed to apply incremental migrations: %v", err)
		}
	}

	fmt.Printf("\n%s════════════════════════════════════════%s\n", colorGreen, colorReset)
	fmt.Printf("%s  ✅ All migrations applied successfully%s\n", colorGreen, colorReset)
	fmt.Printf("%s════════════════════════════════════════%s\n\n", colorGreen, colorReset)
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

func isDatabaseFresh(ctx context.Context, db *sql.DB) (bool, error) {
	// Check if organizations table exists
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'organizations'
		)
	`).Scan(&exists)
	
	if err != nil {
		return false, err
	}

	return !exists, nil
}

func applyConsolidatedSchema(ctx context.Context, db *sql.DB) error {
	// Read V1 consolidated schema
	schemaPath := "migrations/V1__consolidated_schema.sql"
	
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read consolidated schema: %w", err)
	}

	fmt.Printf("%s⚡ Applying consolidated V1 schema...%s\n", colorBlue, colorReset)
	
	// Execute schema in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute consolidated schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("%s✓ Consolidated schema applied%s\n", colorGreen, colorReset)
	return nil
}

func applyIncrementalMigrations(ctx context.Context, db *sql.DB) error {
	// Get already applied migrations
	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Read all migration files
	migrations, err := readMigrationFiles("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	// Filter out already applied migrations
	pending := filterPendingMigrations(migrations, applied)

	if len(pending) == 0 {
		fmt.Printf("%s✓ No pending migrations%s\n", colorGreen, colorReset)
		return nil
	}

	fmt.Printf("%s📝 Found %d pending migration(s)%s\n\n", colorYellow, len(pending), colorReset)

	// Apply each pending migration
	for _, migration := range pending {
		fmt.Printf("%s▶ Applying: %s%s\n", colorBlue, migration.Filename, colorReset)
		
		if err := applyMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Filename, err)
		}

		fmt.Printf("%s  ✓ Applied%s\n", colorGreen, colorReset)
	}

	return nil
}

func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func readMigrationFiles(dir string) ([]Migration, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Skip consolidated schema (only for fresh deployments)
		if strings.HasPrefix(file.Name(), "V1__consolidated") {
			continue
		}

		// Read file content
		content, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file.Name(), err)
		}

		// Extract version from filename (e.g., "001_initial_schema.sql" -> "001")
		version := strings.TrimSuffix(file.Name(), ".sql")

		migrations = append(migrations, Migration{
			Version:  version,
			Filename: file.Name(),
			SQL:      string(content),
		})
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func filterPendingMigrations(migrations []Migration, applied map[string]bool) []Migration {
	var pending []Migration
	for _, m := range migrations {
		if !applied[m.Version] {
			pending = append(pending, m)
		}
	}
	return pending
}

func applyMigration(ctx context.Context, db *sql.DB, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return err
	}

	// Record migration
	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)",
		migration.Version, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}
