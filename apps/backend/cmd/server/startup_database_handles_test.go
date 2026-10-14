package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serviceScopeStatement = `SELECT set_config('app.rls_service_scope', 'on', true)`

// Every migration transaction runMigrations opens declares the service scope
// as its first statement, before the migration's own SQL.
func TestRunMigrations_DeclaresTheServiceScopeFirstInEveryMigrationTransaction(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "migrations"), 0o750))
	files := map[string]string{
		"001_first.sql":  "CREATE TABLE first_table (id integer);",
		"002_second.sql": "UPDATE first_table SET id = 2;",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "migrations", name), []byte(content), 0o600))
	}
	t.Chdir(dir)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT to_regclass('schema_migrations') IS NOT NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	for _, name := range []string{"001_first.sql", "002_second.sql"} {
		mock.ExpectBegin()
		mock.ExpectExec(serviceScopeStatement).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(files[name]).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`INSERT INTO schema_migrations (version) VALUES ($1)`).
			WithArgs(name).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}

	require.NoError(t, runMigrations(db))
	require.NoError(t, mock.ExpectationsWereMet())
}

// A refused service-scope declaration stops the migration before its SQL
// runs, and rolls the transaction back.
func TestRunMigrations_StopsWhenTheServiceScopeIsRefused(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "migrations"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "migrations", "001_first.sql"), []byte("SELECT 1;"), 0o600))
	t.Chdir(dir)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT to_regclass('schema_migrations') IS NOT NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT version FROM schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec(serviceScopeStatement).WillReturnError(assert.AnError)
	mock.ExpectRollback()

	err = runMigrations(db)
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "001_first.sql")
	require.NoError(t, mock.ExpectationsWereMet())
}

// callsIn returns the calls in body whose function is named name, as a plain
// identifier or as a selector's last element.
func callsIn(body ast.Node, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == name {
				calls = append(calls, call)
			}
		case *ast.SelectorExpr:
			if fn.Sel.Name == name {
				calls = append(calls, call)
			}
		}
		return true
	})
	return calls
}

func funcDecl(t *testing.T, file, name string) *ast.FuncDecl {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("%s declares no func %s", file, name)
	return nil
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// The schema migrations and the one-time startup data migrations run on the
// migration handle, which main closes before the server listens; the request
// pool, which carries POSTGRES_POOL_ROLE and the census, reaches none of
// them. `aim-server migrate` runs on the migration handle too.
func TestMain_StartupMigrationsRunOnTheMigrationHandle(t *testing.T) {
	main := funcDecl(t, "main.go", "main")

	handles := map[string]string{} // variable -> the function it was assigned from
	ast.Inspect(main.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		if fn := identName(call.Fun); fn == "initMigrationHandle" || fn == "initRequestPool" {
			handles[identName(assign.Lhs[0])] = fn
		}
		return true
	})
	var migrationHandle, requestPool string
	for name, fn := range handles {
		switch fn {
		case "initMigrationHandle":
			migrationHandle = name
		case "initRequestPool":
			requestPool = name
		}
	}
	require.NotEmpty(t, migrationHandle, "main opens the migration handle with initMigrationHandle")
	require.NotEmpty(t, requestPool, "main opens the request pool with initRequestPool")

	for name, argument := range map[string]int{
		"runMigrations":               0,
		"seedAdminFromEnv":            0,
		"MigrateAgentPrivateKeysToV2": 1,
	} {
		calls := callsIn(main.Body, name)
		require.Len(t, calls, 1, "main calls %s once", name)
		assert.Equal(t, migrationHandle, identName(calls[0].Args[argument]), "%s receives the migration handle", name)
	}

	var closedAt, listenAt token.Pos
	for _, call := range callsIn(main.Body, "Close") {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && identName(sel.X) == migrationHandle {
			closedAt = call.Pos()
		}
	}
	for _, call := range callsIn(main, "Listen") {
		listenAt = call.Pos()
	}
	require.NotZero(t, closedAt, "main closes the migration handle")
	require.NotZero(t, listenAt, "main listens")
	assert.Less(t, closedAt, listenAt, "the migration handle is closed before the server listens")
	for _, call := range callsIn(main.Body, "MigrateAgentPrivateKeysToV2") {
		assert.Less(t, call.Pos(), closedAt, "the key migration runs before the handle is closed")
	}

	migrate := funcDecl(t, "migrate_command.go", "runMigrateCommand")
	assert.Len(t, callsIn(migrate.Body, "initMigrationHandle"), 1, "aim-server migrate opens the migration handle")
	assert.Empty(t, callsIn(migrate.Body, "initDatabase"), "aim-server migrate does not open the request pool")
	assert.Empty(t, callsIn(migrate.Body, "initRequestPool"), "aim-server migrate does not open the request pool")
}
