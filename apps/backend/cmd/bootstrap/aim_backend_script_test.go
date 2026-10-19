package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

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

// adminRequest is one request seededAdminServer answered: the password it
// sent (currentPassword for a change), the bearer token it carried, the new
// password of a change, the status it got, the whole second it was made in,
// and, for a sign-in, the forcePasswordChange its answer carried.
type adminRequest struct {
	password, token, newPassword string
	status                       int
	second                       int64
	forceChange                  bool
}

// seededAdminServer answers POST /api/v1/auth/login/local and
// POST /api/v1/auth/change-password as the backend does for the admin
// aim-bootstrap --default seeds: a sign-in with the password the account
// holds is answered 200 with a token pair and user.forcePasswordChange, any
// other 401. A change must carry the bearer token of a sign-in (401
// otherwise) and the current password, and a new password that satisfies the
// server's rule (400 otherwise); on success it sets the password, clears the
// requirement, ends every token issued before it and answers 200 with a new
// pair. It records each request.
type seededAdminServer struct {
	mu          sync.Mutex
	password    string
	forceChange bool
	issued      int
	tokens      map[string]bool
	signIns     []adminRequest
	changes     []adminRequest
	// refuseChange, when not 0, is the status every change is answered with.
	refuseChange int
	// keepRequirement leaves forcePasswordChange set after a change.
	keepRequirement bool
}

func newSeededAdminServer(seededPassword string) *seededAdminServer {
	return &seededAdminServer{password: seededPassword, forceChange: true, tokens: map[string]bool{}}
}

// satisfiesPasswordRule is PasswordHasher.ValidatePassword: at least 8
// characters with an upper-case letter, a lower-case letter, a digit and one
// of the special characters it lists.
func satisfiesPasswordRule(password string) bool {
	var upper, lower, digit, special bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case strings.ContainsRune(`!@#$%^&*()_+-=[]{};':"\|,.<>/?`, r):
			special = true
		}
	}
	return len(password) >= 8 && upper && lower && digit && special
}

func (s *seededAdminServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login/local", s.signIn)
	mux.HandleFunc("POST /api/v1/auth/change-password", s.changePassword)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// issue mints a token pair. The values are plainly fake, and the test looks
// for their prefix in the script's output.
func (s *seededAdminServer) issue() (string, string) {
	s.issued++
	access := fmt.Sprintf("FAKE-TOKEN-%d", s.issued)
	s.tokens[access] = true
	return access, fmt.Sprintf("FAKE-REFRESH-%d", s.issued)
}

func (s *seededAdminServer) signIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := adminRequest{password: req.Password, second: time.Now().Unix()}
	if req.Email != "admin@opena2a.org" || req.Password != s.password {
		rec.status = http.StatusUnauthorized
		s.signIns = append(s.signIns, rec)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}
	access, refresh := s.issue()
	rec.status, rec.token, rec.forceChange = http.StatusOK, access, s.forceChange
	s.signIns = append(s.signIns, rec)
	writeJSON(w, http.StatusOK, map[string]any{
		"accessToken":  access,
		"refreshToken": refresh,
		"user":         map[string]any{"email": req.Email, "forcePasswordChange": s.forceChange},
	})
}

func (s *seededAdminServer) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	rec := adminRequest{password: req.CurrentPassword, token: token, newPassword: req.NewPassword, second: time.Now().Unix()}
	refuse := func(status int, message string) {
		rec.status = status
		s.changes = append(s.changes, rec)
		writeJSON(w, status, map[string]string{"error": message})
	}
	switch {
	case !s.tokens[token]:
		refuse(http.StatusUnauthorized, "Invalid or expired token")
	case s.refuseChange != 0:
		refuse(s.refuseChange, "currentPassword is incorrect")
	case req.CurrentPassword != s.password:
		refuse(http.StatusBadRequest, "currentPassword is incorrect")
	case !satisfiesPasswordRule(req.NewPassword):
		refuse(http.StatusBadRequest, "newPassword must be at least 8 characters with an upper-case letter, a lower-case letter, a digit and a special character")
	default:
		s.password = req.NewPassword
		if !s.keepRequirement {
			s.forceChange = false
		}
		// The change ends every session signed in before it.
		s.tokens = map[string]bool{}
		access, refresh := s.issue()
		rec.status = http.StatusOK
		s.changes = append(s.changes, rec)
		writeJSON(w, http.StatusOK, map[string]any{"message": "Password changed successfully", "accessToken": access, "refreshToken": refresh})
	}
}

// completePasswordChangeWithAimBackendScript runs
// aim_backend_complete_password_change against aimURL for an admin seeded
// with seededPassword, under the shell options and the EXIT trap the
// integration scripts set, with work as the run's directory. It returns the
// password the function leaves in admin_password (printed on standard output
// after the function, whose own output goes to standard error), the run's
// standard error, and the run's error.
func completePasswordChangeWithAimBackendScript(t *testing.T, work, aimURL, seededPassword string) (string, string, error) {
	t.Helper()
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `set -euo pipefail
aim_backend_name=aim-backend-test
repo_root="$1"
. "$repo_root/scripts/lib/aim-backend.sh"
work="$2"
logs="$work/logs"
mkdir -p "$logs"
aim_url="$3"
admin_password="$AIM_BACKEND_TEST_SEEDED_PASSWORD"
trap aim_backend_cleanup EXIT
aim_backend_complete_password_change 1>&2
printf '%s' "$admin_password"`, "bash", root, work, aimURL)
	// HOME is the run's directory, so curl reads no .curlrc of the developer.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + work, "AIM_BACKEND_TEST_SEEDED_PASSWORD=" + seededPassword}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

// aim-bootstrap --default seeds the admin with force_password_change set, and
// the API admits that account's session only to GET /api/v1/auth/me and
// POST /api/v1/auth/change-password until the password changes. The script
// completes the change before the tests sign in: the change carries the
// sign-in's bearer token and the seeded password, the password the tests then
// sign in with is the one the change set, that sign-in is in a later whole
// second than the change (a token minted in the second of a change is
// refused) and requires no change, no password or token is printed, and the
// files that carried them are gone.
func TestAimBackendScript_CompletesTheSeededAdminsRequiredPasswordChange(t *testing.T) {
	const seeded = "FAKE-seeded-PLACEHOLDER-Aa1!"
	server := newSeededAdminServer(seeded)
	ts := httptest.NewServer(server.handler())
	defer ts.Close()
	work := t.TempDir()

	newPassword, out, err := completePasswordChangeWithAimBackendScript(t, work, ts.URL, seeded)
	if err != nil {
		t.Fatalf("aim_backend_complete_password_change stopped the run: %v\n%s", err, out)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.signIns) != 2 || len(server.changes) != 1 {
		t.Fatalf("the server answered %d sign-ins and %d changes; want 2 sign-ins around 1 change\n%s", len(server.signIns), len(server.changes), out)
	}
	first, change, proof := server.signIns[0], server.changes[0], server.signIns[1]
	if first.password != seeded || first.status != http.StatusOK {
		t.Errorf("the first sign-in sent password %q and was answered %d; want the seeded password, answered 200", first.password, first.status)
	}
	if change.token != first.token {
		t.Errorf("the change carried bearer token %q; the sign-in issued %q", change.token, first.token)
	}
	if change.password != seeded {
		t.Errorf("the change sent currentPassword %q; want the seeded password", change.password)
	}
	if change.newPassword == seeded || !satisfiesPasswordRule(change.newPassword) {
		t.Errorf("the change sent newPassword %q; want one that differs from the seeded password and satisfies the server's rule", change.newPassword)
	}
	if change.status != http.StatusOK {
		t.Errorf("the change was answered %d; want 200", change.status)
	}
	if newPassword != change.newPassword {
		t.Errorf("admin_password is %q after the function; the change set %q", newPassword, change.newPassword)
	}
	if proof.password != newPassword || proof.status != http.StatusOK {
		t.Errorf("the sign-in after the change sent password %q and was answered %d; want the new password, answered 200", proof.password, proof.status)
	}
	if proof.forceChange {
		t.Errorf("the sign-in after the change still carried forcePasswordChange true")
	}
	if proof.second <= change.second {
		t.Errorf("the sign-in after the change was made in second %d, the change in second %d; the function must wait for the next whole second", proof.second, change.second)
	}
	for _, secret := range []string{seeded, newPassword, "FAKE-TOKEN", "FAKE-REFRESH"} {
		if strings.Contains(out, secret) {
			t.Errorf("the output holds %q:\n%s", secret, out)
		}
	}
	if !strings.Contains(out, "Completing the admin's required password change") {
		t.Errorf("the output does not say what the step does:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(work, "auth")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s/auth, which held the request bodies and the token, remains (stat: %v)", work, err)
	}
}

// A sign-in or a change the server refuses, and a change after which the
// account still requires one, stop the run with exit 1 and a message that
// names the route and the status, holds no password or token, and leaves no
// request file behind.
func TestAimBackendScript_StopsTheRunWhenTheRequiredPasswordChangeDoesNotComplete(t *testing.T) {
	const seeded = "FAKE-seeded-PLACEHOLDER-Aa1!"
	for _, tc := range []struct {
		name             string
		configure        func(*seededAdminServer)
		message          string
		signIns, changes int
	}{
		{"the seeded password is refused", func(s *seededAdminServer) { s.password = "FAKE-other-PLACEHOLDER-Aa1!" },
			"the seeded admin's sign-in, POST /api/v1/auth/login/local, answered 401", 1, 0},
		{"the change is refused", func(s *seededAdminServer) { s.refuseChange = http.StatusBadRequest },
			"POST /api/v1/auth/change-password answered 400", 1, 1},
		{"the account still requires a change", func(s *seededAdminServer) { s.keepRequirement = true },
			"still requires a password change after POST /api/v1/auth/change-password answered 200", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newSeededAdminServer(seeded)
			tc.configure(server)
			ts := httptest.NewServer(server.handler())
			defer ts.Close()
			work := t.TempDir()

			_, out, err := completePasswordChangeWithAimBackendScript(t, work, ts.URL, seeded)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("want exit 1, got %v; output:\n%s", err, out)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("the output does not say %q:\n%s", tc.message, out)
			}
			server.mu.Lock()
			defer server.mu.Unlock()
			if len(server.signIns) != tc.signIns || len(server.changes) != tc.changes {
				t.Errorf("the server answered %d sign-ins and %d changes; want %d and %d", len(server.signIns), len(server.changes), tc.signIns, tc.changes)
			}
			secrets := []string{"PLACEHOLDER", "FAKE-TOKEN", "FAKE-REFRESH"}
			for _, c := range server.changes {
				secrets = append(secrets, c.newPassword)
			}
			for _, secret := range secrets {
				if strings.Contains(out, secret) {
					t.Errorf("the output holds %q:\n%s", secret, out)
				}
			}
			if _, err := os.Stat(filepath.Join(work, "auth")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s/auth, which held the request bodies and the token, remains (stat: %v)", work, err)
			}
		})
	}
}
