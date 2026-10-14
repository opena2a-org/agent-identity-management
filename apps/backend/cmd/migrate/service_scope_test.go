package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serviceScopeStatement = `SELECT set_config('app.rls_service_scope', 'on', true)`

// Each migration transaction declares the service scope as its first
// statement, before the migration's own SQL.
func TestApplyMigrationDeclaresTheServiceScopeFirst(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	migration := Migration{Version: "001_first.sql", Filename: "001_first.sql", SQL: "UPDATE agents SET status = 'active';"}
	mock.ExpectBegin()
	mock.ExpectExec(serviceScopeStatement).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(migration.SQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)").
		WithArgs(migration.Version, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, applyMigration(context.Background(), db, migration))
	require.NoError(t, mock.ExpectationsWereMet())
}

// A refused declaration stops the migration before its SQL runs.
func TestApplyMigrationStopsWhenTheServiceScopeIsRefused(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(serviceScopeStatement).WillReturnError(assert.AnError)
	mock.ExpectRollback()

	err = applyMigration(context.Background(), db, Migration{Version: "001_first.sql", Filename: "001_first.sql", SQL: "SELECT 1;"})
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "001_first.sql")
	require.NoError(t, mock.ExpectationsWereMet())
}

// The consolidated schema's transaction declares the service scope first too.
func TestApplyConsolidatedSchemaDeclaresTheServiceScopeFirst(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "migrations"), 0o750))
	schema := "CREATE TABLE organizations (id uuid);"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "migrations", "V1__consolidated_schema.sql"), []byte(schema), 0o600))
	t.Chdir(dir)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(serviceScopeStatement).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(schema).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	require.NoError(t, applyConsolidatedSchema(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}
