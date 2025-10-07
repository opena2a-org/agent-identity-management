package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Get database connection string
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_USER", "postgres"),
		getEnv("POSTGRES_PASSWORD", "postgres"),
		getEnv("POSTGRES_DB", "identity"),
		getEnv("POSTGRES_SSL_MODE", "disable"),
	)

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		log.Fatal("Failed to create migrations table:", err)
	}

	// Get command
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "up":
		if err := migrateUp(db); err != nil {
			log.Fatal("Migration failed:", err)
		}
		log.Println("✅ Migrations completed successfully")
	case "down":
		steps := 1
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &steps)
		}
		if err := migrateDown(db, steps); err != nil {
			log.Fatal("Rollback failed:", err)
		}
		log.Println("✅ Rollback completed successfully")
	case "status":
		if err := migrationStatus(db); err != nil {
			log.Fatal("Status check failed:", err)
		}
	default:
		log.Fatalf("Unknown command: %s. Use 'up', 'down', or 'status'", command)
	}
}

func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			version VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(query)
	return err
}

func migrateUp(db *sql.DB) error {
	// Get migration files
	files, err := getMigrationFiles()
	if err != nil {
		return err
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	// Apply pending migrations
	for _, file := range files {
		version := getMigrationVersion(file)
		if applied[version] {
			log.Printf("⏭️  Skipping %s (already applied)", file)
			continue
		}

		log.Printf("🔄 Applying %s...", file)

		// Read migration file
		content, err := ioutil.ReadFile(filepath.Join("migrations", file))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		// Execute migration
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", file, err)
		}

		// Record migration
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		log.Printf("✅ Applied %s", file)
	}

	return nil
}

func migrateDown(db *sql.DB, steps int) error {
	// Get applied migrations in reverse order
	query := `SELECT version FROM schema_migrations ORDER BY applied_at DESC LIMIT $1`
	rows, err := db.Query(query, steps)
	if err != nil {
		return err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return err
		}
		versions = append(versions, version)
	}

	// Rollback migrations
	for _, version := range versions {
		log.Printf("🔄 Rolling back %s...", version)

		// For now, we don't support down migrations (would need separate files)
		// Just remove from migrations table
		if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", version, err)
		}

		log.Printf("✅ Rolled back %s", version)
	}

	log.Println("⚠️  Note: This only removes migration records. Manual cleanup may be needed.")
	return nil
}

func migrationStatus(db *sql.DB) error {
	files, err := getMigrationFiles()
	if err != nil {
		return err
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	log.Println("\n📋 Migration Status:")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, file := range files {
		version := getMigrationVersion(file)
		status := "❌ Pending"
		if applied[version] {
			status = "✅ Applied"
		}
		log.Printf("%s  %s", status, file)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("Total: %d migrations (%d applied, %d pending)\n",
		len(files), len(applied), len(files)-len(applied))

	return nil
}

func getMigrationFiles() ([]string, error) {
	files, err := ioutil.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(file.Name(), ".sql") {
			migrations = append(migrations, file.Name())
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
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

	return applied, nil
}

func getMigrationVersion(filename string) string {
	// Extract version from filename (e.g., "001_initial_schema.sql" -> "001")
	parts := strings.Split(filename, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return filename
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
