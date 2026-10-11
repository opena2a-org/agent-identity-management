package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgsAcceptsEachCommand(t *testing.T) {
	cases := []struct {
		args []string
		want command
	}{
		{nil, command{name: commandUp}},
		{[]string{"up"}, command{name: commandUp}},
		{[]string{"status"}, command{name: commandStatus}},
		{[]string{"create", "add_column_to_agents"}, command{name: commandCreate, migrationName: "add_column_to_agents"}},
		{[]string{"help"}, command{name: commandHelp}},
		{[]string{"-h"}, command{name: commandHelp}},
		{[]string{"--help"}, command{name: commandHelp}},
	}
	for _, c := range cases {
		got, err := parseArgs(c.args)
		require.NoError(t, err, "%q", c.args)
		assert.Equal(t, c.want, got, "%q", c.args)
	}
}

func TestParseArgsRefusesWhatItCannotDo(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"down", "1"}, "migrate down is not available: the migrations in migrations/ have no down scripts"},
		{[]string{"down"}, "migrate down is not available"},
		{[]string{"force", "001"}, "migrate force is not available"},
		{[]string{"drop"}, "migrate drop is not available: the tool does not delete data"},
		{[]string{"frobnicate"}, `unknown command "frobnicate"`},
		{[]string{"up", "1"}, `migrate up takes no arguments, got "1"`},
		{[]string{"status", "now"}, `migrate status takes no arguments, got "now"`},
		{[]string{"create"}, "migrate create takes one argument"},
		{[]string{"create", "a", "b"}, "migrate create takes one argument"},
		{[]string{"create", "Add-Column"}, `migration name "Add-Column" must be lowercase letters and digits joined by underscores`},
		{[]string{"create", "../escape"}, `migration name "../escape" must be lowercase`},
		{[]string{"create", ""}, `migration name "" must be lowercase`},
	}
	for _, c := range cases {
		_, err := parseArgs(c.args)
		require.Error(t, err, "%q", c.args)
		assert.Contains(t, err.Error(), c.want, "%q", c.args)
	}
}

// runMain runs main() in a child copy of the test binary with the given
// arguments, from dir, with DATABASE_URL naming a port nothing listens on: a
// command that reached the database would fail to ping it.
func runMain(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"AIM_MIGRATE_HELPER_PROCESS=1",
		"AIM_MIGRATE_HELPER_ARGS="+strings.Join(args, "\x1f"),
		"DATABASE_URL=postgres://aim@127.0.0.1:1/none?sslmode=disable&connect_timeout=2",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return code, stdout.String(), stderr.String()
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("AIM_MIGRATE_HELPER_PROCESS") != "1" {
		t.Skip("runs main() for runMain")
	}
	os.Args = []string{"migrate"}
	if raw := os.Getenv("AIM_MIGRATE_HELPER_ARGS"); raw != "" {
		os.Args = append(os.Args, strings.Split(raw, "\x1f")...)
	}
	main()
	os.Exit(0)
}

func TestMainRefusesACommandBeforeConnecting(t *testing.T) {
	for _, args := range [][]string{{"down", "1"}, {"force", "001"}, {"drop"}, {"frobnicate"}, {"status", "now"}} {
		code, stdout, stderr := runMain(t, t.TempDir(), args...)
		assert.Equal(t, 2, code, "%q: stderr %s", args, stderr)
		assert.Empty(t, stdout, "%q", args)
		assert.Contains(t, stderr, "Usage: migrate [command]", "%q", args)
		assert.NotContains(t, stderr, "Failed to ping database", "%q: a refused command must not reach the database", args)
	}
}

func TestMainStatusReachesTheDatabase(t *testing.T) {
	code, _, stderr := runMain(t, t.TempDir(), "status")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "Failed to ping database")
}

func TestMainCreateNeedsNoDatabase(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "migrations"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "migrations", "001_initial_schema.sql"), []byte("SELECT 1;\n"), 0o644))

	code, stdout, stderr := runMain(t, dir, "create", "add_column_to_agents")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "Created "+filepath.Join("migrations", "002_add_column_to_agents.sql")+"\n", stdout)
	assert.FileExists(t, filepath.Join(dir, "migrations", "002_add_column_to_agents.sql"))
}

func TestCreateMigrationNumbersAfterTheHighestFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001_initial_schema.sql", "009_b.sql", "010_c.sql", "V1__consolidated_schema.sql", "notes.txt", "README.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o644))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, "999_dir.sql"), 0o755))

	path, err := createMigration(dir, "add_column_to_agents")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "011_add_column_to_agents.sql"), path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(content), "-- Migration 011: add column to agents\n"), string(content))

	migrations, err := readMigrationFiles(dir)
	require.NoError(t, err)
	assert.Equal(t, "011_add_column_to_agents.sql", migrations[len(migrations)-1].Filename, "the new file sorts last, so up applies it last")

	path, err = createMigration(dir, "add_index")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "012_add_index.sql"), path)
}

func TestCreateMigrationNeedsTheDirectory(t *testing.T) {
	_, err := createMigration(filepath.Join(t.TempDir(), "missing"), "x")
	require.Error(t, err)
}

func writeMigrations(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o644))
	}
	return dir
}

// The mock answers only the statements expected below, so a status that
// created a table or applied a migration fails on the unexpected statement.
func TestPrintStatusOnADatabaseWithoutTheTable(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT to_regclass('schema_migrations') IS NOT NULL").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	dir := writeMigrations(t, "001_a.sql", "002_b.sql")
	var out bytes.Buffer
	require.NoError(t, printStatus(context.Background(), db, dir, &out))
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, "Migration files in "+dir+"/: 2\n"+
		"schema_migrations does not exist yet; `migrate up` creates it\n"+
		"Applied: 0\n"+
		"Pending: 2\n"+
		"  pending  001_a.sql\n"+
		"  pending  002_b.sql\n", out.String())
}

func TestPrintStatusListsPendingAndUnknownRecords(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT to_regclass('schema_migrations') IS NOT NULL").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT version FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("001_a.sql").AddRow("002_b.sql").AddRow("001_a"))

	dir := writeMigrations(t, "001_a.sql", "002_b.sql", "003_c.sql")
	var out bytes.Buffer
	require.NoError(t, printStatus(context.Background(), db, dir, &out))
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, "Migration files in "+dir+"/: 3\n"+
		"Applied: 2 (last: 002_b.sql)\n"+
		"Pending: 1\n"+
		"  pending  003_c.sql\n"+
		"Recorded in schema_migrations with no file in "+dir+"/: 1\n"+
		"  recorded 001_a\n", out.String())
}

func TestPrintStatusWithEveryMigrationApplied(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT to_regclass('schema_migrations') IS NOT NULL").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT version FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("002_b.sql").AddRow("001_a.sql"))

	dir := writeMigrations(t, "001_a.sql", "002_b.sql")
	var out bytes.Buffer
	require.NoError(t, printStatus(context.Background(), db, dir, &out))
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, "Migration files in "+dir+"/: 2\n"+
		"Applied: 2 (last: 002_b.sql)\n"+
		"Pending: 0\n", out.String())
}
