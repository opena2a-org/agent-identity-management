package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// migrateCommandName is the argument that runs the migrate command instead of
// the server: `aim-server migrate`.
const migrateCommandName = "migrate"

// runMigrateCommand applies the pending migrations and exits without starting
// the server. It reads only the POSTGRES_* variables and is meant to run as the
// role that owns the schema, so the server itself can connect as a role that
// owns nothing.
//
// With POSTGRES_APP_USER set it first provisions that role for the server
// (provisionAppRole), with POSTGRES_APP_PASSWORD as its password. It returns
// the process exit code: 0 done, 1 failed.
func runMigrateCommand() int {
	db, err := initDatabase(&config.Config{Database: config.LoadDatabase()})
	if err != nil {
		log.Printf("❌ Failed to connect to database: %v", err)
		return 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	appUser := os.Getenv("POSTGRES_APP_USER")
	if appUser != "" {
		if err := provisionAppRole(ctx, db, appUser, os.Getenv("POSTGRES_APP_PASSWORD")); err != nil {
			log.Printf("❌ Application role %s: %v", appUser, err)
			return 1
		}
	}

	if err := runMigrations(db); err != nil {
		log.Printf("❌ Database migrations failed: %v", err)
		return 1
	}
	log.Println("✅ Database migrations completed successfully")

	if appUser != "" {
		superuser, bypassRLS, owned, err := appRoleState(ctx, db, appUser)
		if err != nil {
			log.Printf("❌ Application role %s: reading its attributes: %v", appUser, err)
			return 1
		}
		log.Printf("✅ Application role %s: superuser=%t bypassrls=%t ownedRelations=%d", appUser, superuser, bypassRLS, owned)
	}
	return 0
}

// provisionAppRole creates or updates the login role the server connects as:
// not a superuser, no BYPASSRLS, no CREATE on schema public, and read and write
// access to the tables and sequences the connected role creates there. Database
// grants and row-level security bind such a role; they bind neither a
// superuser, nor a BYPASSRLS role, nor a table's owner.
//
// Run it as the role the migrations run as, before them, so the tables they
// create are granted through the default privileges. Every existing table is
// granted only when the role is created; a later run restates the role's
// attributes, password and default privileges and leaves table grants alone,
// so a privilege a migration revokes from the role stays revoked.
func provisionAppRole(ctx context.Context, db *sql.DB, name, password string) error {
	if password == "" {
		return errors.New("POSTGRES_APP_PASSWORD is not set")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var connectedAs string
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT current_user, EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, name,
	).Scan(&connectedAs, &exists); err != nil {
		return err
	}
	if name == connectedAs {
		return errors.New("POSTGRES_APP_USER names the role this command connects as (POSTGRES_USER); the server needs a different role")
	}

	role := pq.QuoteIdentifier(name)
	verb := "ALTER"
	if !exists {
		verb = "CREATE"
	}
	statements := []string{
		verb + " ROLE " + role + " WITH LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD " + pq.QuoteLiteral(password),
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC, " + role,
		"GRANT USAGE ON SCHEMA public TO " + role,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO " + role,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO " + role,
	}
	if !exists {
		statements = append(statements,
			"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "+role,
			"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO "+role,
		)
	}
	for i, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			step := stmt
			if i == 0 {
				step = verb + " ROLE " + role // the statement carries the password
			}
			return fmt.Errorf("%s: %w", step, err)
		}
	}
	return tx.Commit()
}

// appRoleState reads what decides whether database grants and row-level
// security bind a role: its superuser and BYPASSRLS attributes, and how many
// relations of the current database it owns.
func appRoleState(ctx context.Context, db *sql.DB, name string) (superuser, bypassRLS bool, ownedRelations int, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT r.rolsuper, r.rolbypassrls,
		       (SELECT count(*) FROM pg_class c WHERE c.relowner = r.oid)
		  FROM pg_roles r
		 WHERE r.rolname = $1`, name,
	).Scan(&superuser, &bypassRLS, &ownedRelations)
	return superuser, bypassRLS, ownedRelations, err
}
