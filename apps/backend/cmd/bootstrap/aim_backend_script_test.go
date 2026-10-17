package main

import (
	"bytes"
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lib/pq"
)

// aimBackendScript is the library the SDK integration scripts source to start
// a backend. It hands each backend process DATABASE_URL as it is, which
// cmd/migrate opens, and POSTGRES_* variables it takes from that URL, which
// the server reads and aim-bootstrap reads before DATABASE_URL.
var aimBackendScript = filepath.Join(repoRoot, "scripts", "lib", "aim-backend.sh")

// parseWithAimBackendScript runs aim_backend_parse_database_url on databaseURL
// under the shell options the integration scripts set, and returns the
// POSTGRES_* variables aim_backend_start builds from the pg_* values it sets:
// POSTGRES_HOST="$pg_host", POSTGRES_PORT="$pg_port" and so on. When the
// function stops the run, it returns the run's output and error instead.
func parseWithAimBackendScript(t *testing.T, databaseURL string) (map[string]string, string, error) {
	t.Helper()
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `set -euo pipefail
aim_backend_name=aim-backend-test
repo_root="$1"
. "$repo_root/scripts/lib/aim-backend.sh"
aim_backend_parse_database_url
printf '%s\0' "$pg_host" "$pg_port" "$pg_user" "$pg_password" "$pg_db" "$pg_sslmode"`, "bash", root)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "DATABASE_URL=" + databaseURL}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, stdout.String() + stderr.String(), err
	}
	values := strings.Split(strings.TrimSuffix(stdout.String(), "\x00"), "\x00")
	if len(values) != 6 {
		t.Fatalf("%s printed %d values, want 6", aimBackendScript, len(values))
	}
	return map[string]string{
		"POSTGRES_HOST":     values[0],
		"POSTGRES_PORT":     values[1],
		"POSTGRES_USER":     values[2],
		"POSTGRES_PASSWORD": values[3],
		"POSTGRES_DB":       values[4],
		"POSTGRES_SSL_MODE": values[5],
	}, "", nil
}

// lib/pq reads the user, the password and the database name of DATABASE_URL
// percent-decoded for cmd/migrate. The server reads POSTGRES_USER,
// POSTGRES_PASSWORD and POSTGRES_DB as they are, and aim-bootstrap builds its
// URL from the POSTGRES_* variables when POSTGRES_HOST is set, so the script
// must hand both the decoded values. Before it decoded them, an at-sign
// written %40 in the password let the migrations pass and got aim-bootstrap
// and the server refused by password authentication.
func TestAimBackendScript_HandsTheBackendTheUserPasswordAndDatabaseMigrateReads(t *testing.T) {
	for _, tc := range []struct{ name, databaseURL string }{
		{"an at-sign in the password", "postgres://aim:FAKE%40PLACEHOLDER@127.0.0.1:5432/aim?sslmode=disable"},
		{"an encoded percent sign is decoded once", "postgres://aim:FAKE%2540PLACEHOLDER@127.0.0.1:5432/aim?sslmode=disable"},
		{"lower-case hex, a colon and a slash", "postgresql://aim:FAKE%3a%2FPLACEHOLDER@db.internal:6543/aim?sslmode=require"},
		{"an at-sign in the user and a hyphen in the database name", "postgres://aim%40corp:FAKE-PLACEHOLDER@db.internal:6543/aim%2Ddb?sslmode=require"},
		{"a two-byte UTF-8 character", "postgres://aim:FAKE%C3%A9PLACEHOLDER@127.0.0.1:5432/aim?sslmode=disable"},
		{"nothing encoded and a plus sign", "postgres://aim:FAKE+PLACEHOLDER@127.0.0.1:5432/aim?sslmode=disable"},
		{"no password", "postgres://aim@127.0.0.1:5432/aim?sslmode=disable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.databaseURL)
			if err != nil {
				t.Fatalf("the test's DATABASE_URL is not a URL: %v", err)
			}
			password, _ := u.User.Password()
			vars, out, err := parseWithAimBackendScript(t, tc.databaseURL)
			if err != nil {
				t.Fatalf("aim_backend_parse_database_url stopped the run: %v\n%s", err, out)
			}
			if vars["POSTGRES_USER"] != u.User.Username() || vars["POSTGRES_PASSWORD"] != password || vars["POSTGRES_DB"] != strings.TrimPrefix(u.Path, "/") {
				t.Errorf("the script hands the server user %q, password %q, database %q; lib/pq reads user %q, password %q, database %q from DATABASE_URL",
					vars["POSTGRES_USER"], vars["POSTGRES_PASSWORD"], vars["POSTGRES_DB"], u.User.Username(), password, strings.TrimPrefix(u.Path, "/"))
			}

			vars["DATABASE_URL"] = tc.databaseURL
			setDatabaseEnv(t, vars)
			dsn, err := databaseURL("")
			if err != nil {
				t.Fatalf("aim-bootstrap finds no database in the script's variables: %v", err)
			}
			got, err := pq.ParseURL(dsn)
			if err != nil {
				t.Fatalf("lib/pq refuses aim-bootstrap's connection string: %v", err)
			}
			want, err := pq.ParseURL(tc.databaseURL)
			if err != nil {
				t.Fatalf("lib/pq refuses the test's DATABASE_URL: %v", err)
			}
			if got != want {
				t.Errorf("lib/pq reads aim-bootstrap's connection string as\n  %s\nand DATABASE_URL, which cmd/migrate opens, as\n  %s", got, want)
			}
		})
	}
}

// A % in the user, the password or the database name that does not begin a
// %XX makes lib/pq refuse DATABASE_URL, and a NUL byte, %00, is one that no
// environment variable can carry. The script stops the run with a message
// that names the part of the URL and quotes none of it.
func TestAimBackendScript_RefusesAnEscapeItCannotHandOn(t *testing.T) {
	for _, tc := range []struct {
		name, databaseURL, message string
		driverRefuses              bool
	}{
		{"a % before two characters that are not hex", "postgres://aim:FAKE%zzPLACEHOLDER@127.0.0.1:5432/aim", "the password in DATABASE_URL holds a % that does not begin a percent-encoded byte", true},
		{"a % at the end of the user", "postgres://aim%:FAKE-PLACEHOLDER@127.0.0.1:5432/aim", "the user in DATABASE_URL holds a % that does not begin a percent-encoded byte", true},
		{"a % before one hex digit at the end of the database name", "postgres://aim:FAKE-PLACEHOLDER@127.0.0.1:5432/aim%4", "the database name in DATABASE_URL holds a % that does not begin a percent-encoded byte", true},
		{"an encoded NUL byte", "postgres://aim:FAKE%00PLACEHOLDER@127.0.0.1:5432/aim", "the password in DATABASE_URL holds %00", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pq.ParseURL(tc.databaseURL); (err != nil) != tc.driverRefuses {
				t.Fatalf("lib/pq's answer to the test's DATABASE_URL: %v; want refused=%v", err, tc.driverRefuses)
			}
			_, out, err := parseWithAimBackendScript(t, tc.databaseURL)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("want exit 1, got %v; output:\n%s", err, out)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("the output does not say %q:\n%s", tc.message, out)
			}
			if strings.Contains(out, "PLACEHOLDER") {
				t.Errorf("the output holds the password:\n%s", out)
			}
		})
	}
}
