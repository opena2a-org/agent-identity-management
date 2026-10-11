package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// migrationsDir is read relative to the working directory, so the tool runs
// from apps/backend.
const migrationsDir = "migrations"

const (
	commandUp     = "up"
	commandStatus = "status"
	commandCreate = "create"
	commandHelp   = "help"
)

type command struct {
	name          string
	migrationName string
}

// unavailableCommands are verbs other migration tools offer that this one
// refuses, each with the reason and what to do instead.
var unavailableCommands = map[string]string{
	"down": "the migrations in migrations/ have no down scripts, so there is nothing to roll back. " +
		"To reverse a change, add a migration that undoes it (`migrate create <name>`) and run `migrate up`.",
	"force": "applied migrations are recorded by file name in schema_migrations, not as one version number, " +
		"so there is no version to force. `migrate status` lists the applied and pending migrations.",
	"drop": "the tool does not delete data. To reset a development database, drop and recreate it " +
		"(for example with dropdb and createdb), then run `migrate up`.",
}

var migrationNamePattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// parseArgs reads the command line. No arguments means up, which is how CI and
// the deploy scripts call the tool.
func parseArgs(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: commandUp}, nil
	}
	name, rest := args[0], args[1:]
	switch name {
	case commandUp, commandStatus:
		if len(rest) > 0 {
			return command{}, fmt.Errorf("migrate %s takes no arguments, got %q", name, strings.Join(rest, " "))
		}
		return command{name: name}, nil
	case commandCreate:
		if len(rest) != 1 {
			return command{}, fmt.Errorf("migrate create takes one argument, the migration name (for example add_column_to_agents)")
		}
		if !migrationNamePattern.MatchString(rest[0]) {
			return command{}, fmt.Errorf("migration name %q must be lowercase letters and digits joined by underscores (for example add_column_to_agents)", rest[0])
		}
		return command{name: commandCreate, migrationName: rest[0]}, nil
	case commandHelp, "-h", "--help":
		return command{name: commandHelp}, nil
	}
	if reason, ok := unavailableCommands[name]; ok {
		return command{}, fmt.Errorf("migrate %s is not available: %s", name, reason)
	}
	return command{}, fmt.Errorf("unknown command %q", name)
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: migrate [command]

Run from apps/backend with DATABASE_URL set (create needs neither the variable nor a database).

Commands:
  up             Apply every pending migration in migrations/ (the default when no command is given)
  status         List applied and pending migrations without changing the database
  create <name>  Write an empty migration file numbered one above the highest existing number
  help           Show this message
`)
}

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
	// Read the command before connecting, so a command this tool refuses never
	// reaches the database.
	cmd, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n", err)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch cmd.name {
	case commandHelp:
		printUsage(os.Stdout)
		return
	case commandCreate:
		path, err := createMigration(migrationsDir, cmd.migrationName)
		if err != nil {
			log.Fatalf("❌ Failed to create migration: %v", err)
		}
		fmt.Printf("Created %s\n", path)
		return
	}

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

	if cmd.name == commandStatus {
		if err := printStatus(ctx, db, migrationsDir, os.Stdout); err != nil {
			log.Fatalf("❌ Failed to read migration status: %v", err)
		}
		return
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

		// Prefer the consolidated V1 schema when shipped (fast path for prod
		// deployments). Repos that don't ship the consolidated file fall
		// through to the numbered incremental migrations so the tool still
		// works on a clean database — the dev / CI case.
		if _, err := os.Stat("migrations/V1__consolidated_schema.sql"); err == nil {
			fmt.Printf("   Using consolidated V1 schema for fast deployment\n\n")
			if err := applyConsolidatedSchema(ctx, db); err != nil {
				log.Fatalf("❌ Failed to apply consolidated schema: %v", err)
			}
		} else {
			fmt.Printf("   No V1 consolidated schema present; applying incremental migrations\n\n")
			if err := applyIncrementalMigrations(ctx, db); err != nil {
				log.Fatalf("❌ Failed to apply incremental migrations: %v", err)
			}
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

		// Use the full filename as the version key. cmd/server's startup
		// migration loop (runMigrations in cmd/server/main.go) does the same
		// — the two binaries share the schema_migrations table, so the keys
		// must match exactly. Trimming .sql here used to make cmd/server
		// re-apply every migration cmd/migrate had already run, because the
		// "001_initial_schema" row didn't satisfy the "001_initial_schema.sql"
		// lookup.
		version := file.Name()

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

// printStatus lists the applied and pending migrations. It only reads: on a
// database the tool has not run against, schema_migrations does not exist yet
// and every migration is pending.
func printStatus(ctx context.Context, db *sql.DB, dir string, w io.Writer) error {
	migrations, err := readMigrationFiles(dir)
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	var tableExists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&tableExists); err != nil {
		return fmt.Errorf("failed to look up schema_migrations: %w", err)
	}
	applied := map[string]bool{}
	if tableExists {
		if applied, err = getAppliedMigrations(ctx, db); err != nil {
			return fmt.Errorf("failed to get applied migrations: %w", err)
		}
	}
	pending := filterPendingMigrations(migrations, applied)

	lastApplied := ""
	known := make(map[string]bool, len(migrations))
	for _, m := range migrations {
		known[m.Version] = true
		if applied[m.Version] {
			lastApplied = m.Filename
		}
	}
	var unknown []string
	for version := range applied {
		if !known[version] {
			unknown = append(unknown, version)
		}
	}
	sort.Strings(unknown)

	fmt.Fprintf(w, "Migration files in %s/: %d\n", dir, len(migrations))
	if !tableExists {
		fmt.Fprintf(w, "schema_migrations does not exist yet; `migrate up` creates it\n")
	}
	if lastApplied != "" {
		fmt.Fprintf(w, "Applied: %d (last: %s)\n", len(migrations)-len(pending), lastApplied)
	} else {
		fmt.Fprintf(w, "Applied: 0\n")
	}
	fmt.Fprintf(w, "Pending: %d\n", len(pending))
	for _, m := range pending {
		fmt.Fprintf(w, "  pending  %s\n", m.Filename)
	}
	if len(unknown) > 0 {
		fmt.Fprintf(w, "Recorded in schema_migrations with no file in %s/: %d\n", dir, len(unknown))
		for _, version := range unknown {
			fmt.Fprintf(w, "  recorded %s\n", version)
		}
	}
	return nil
}

// createMigration writes an empty migration file numbered one above the
// highest number in dir and returns its path. It never overwrites a file.
func createMigration(dir, name string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", dir, err)
	}
	highest := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		number, _, ok := strings.Cut(entry.Name(), "_")
		if !ok || number == "" || strings.Trim(number, "0123456789") != "" {
			continue
		}
		n, err := strconv.Atoi(number)
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}

	path := filepath.Join(dir, fmt.Sprintf("%03d_%s.sql", highest+1, name))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	header := fmt.Sprintf("-- Migration %03d: %s\n"+
		"--\n"+
		"-- `migrate up` applies this file once, in one transaction, and records it in\n"+
		"-- schema_migrations by file name. There is no down script: to reverse this\n"+
		"-- change, add another migration.\n\n", highest+1, strings.ReplaceAll(name, "_", " "))
	if _, err := file.WriteString(header); err != nil {
		file.Close()
		return "", err
	}
	return path, file.Close()
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
