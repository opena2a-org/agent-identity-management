package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// scripts/quickstart.sh writes ADMIN_EMAIL and ADMIN_PASSWORD to the .env it
// creates, the backend seeds the first administrator from them at startup
// (seedAdmin), and the script prints them as the dashboard login. These tests
// run the script against docker, curl and sleep stubs: docker answers
// `compose ... logs` with a backend log built from the lines seedAdmin and main
// write, and curl answers the compose download and the health probe. openssl is
// the real one unless a test replaces it.

// mainSeedRefusal starts the line main logs when seedAdmin returns an error.
const mainSeedRefusal = "Failed to seed administrator from ADMIN_PASSWORD: "

// quickstartRefusedPassword has upper and lower case letters and a digit but
// no special character, like a base64 draw in which neither + nor / occurs.
const quickstartRefusedPassword = "FAKEnoSpecial0"

// quickstartDockerStub: no earlier install's volume exists, every compose
// command succeeds, and `compose ... logs` prints the file $STUB_BACKEND_LOG.
const quickstartDockerStub = `#!/usr/bin/env bash
case "${1:-}" in
  info) exit 0 ;;
  volume) exit 1 ;;
  compose)
    for a in "$@"; do
      if [ "$a" = logs ]; then cat "$STUB_BACKEND_LOG"; exit 0; fi
    done
    exit 0 ;;
esac
exit 1
`

// quickstartCurlStub writes a placeholder for a download (-o FILE) and answers
// the health probe.
const quickstartCurlStub = `#!/usr/bin/env bash
out="" prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out=$a; prev=$a; done
if [ -n "$out" ]; then printf 'services: {}\n' > "$out"; fi
exit 0
`

// quickstartOpenSSLStub answers `openssl rand -hex N` and `openssl rand -base64
// N` with output of the length the real command prints, every character of it
// a lower-case a: a draw with no upper-case letter, digit or special character.
const quickstartOpenSSLStub = `#!/usr/bin/env bash
[ "${1:-}" = rand ] || exit 1
case "${2:-}" in
  -hex) len=$((2 * $3)) ;;
  -base64) len=$((($3 + 2) / 3 * 4)) ;;
  *) exit 1 ;;
esac
printf '%*s\n' "$len" '' | tr ' ' a
`

type quickstartHarness struct {
	script string // scripts/quickstart.sh
	work   string // this test's temporary directory
	bin    string // the stubs, first on PATH
	runs   int
}

func newQuickstartHarness(t *testing.T) *quickstartHarness {
	t.Helper()
	requirePublicRepositoryFiles(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatal("bash is not on PATH: these tests run scripts/quickstart.sh and cannot run without it")
	}
	h := &quickstartHarness{
		script: filepath.Join(aim03RepoRoot(t), "scripts", "quickstart.sh"),
		work:   t.TempDir(),
	}
	h.bin = filepath.Join(h.work, "bin")
	if err := os.Mkdir(h.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	h.stub(t, "docker", quickstartDockerStub)
	h.stub(t, "curl", quickstartCurlStub)
	h.stub(t, "sleep", "#!/usr/bin/env bash\nexit 0\n")
	return h
}

func (h *quickstartHarness) stub(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.bin, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

type quickstartRun struct {
	dir    string // the directory the script ran in
	output string // what it printed, stdout and stderr
	code   int    // its exit status
}

// run starts quickstart.sh in a new directory with only PATH, HOME and the
// docker stub's log file in its environment. backendLog is what `docker
// compose logs` prints; prepare, when not nil, sets the directory up first.
func (h *quickstartHarness) run(t *testing.T, backendLog string, prepare func(dir string)) quickstartRun {
	t.Helper()
	h.runs++
	dir := filepath.Join(h.work, fmt.Sprintf("run%d", h.runs))
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := dir + ".backend.log"
	if err := os.WriteFile(logFile, []byte(backendLog), 0o600); err != nil {
		t.Fatal(err)
	}
	if prepare != nil {
		prepare(dir)
	}
	cmd := exec.Command("bash", h.script)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + h.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + h.work,
		"STUB_BACKEND_LOG=" + logFile,
	}
	out, err := cmd.CombinedOutput()
	run := quickstartRun{dir: dir, output: string(out)}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running quickstart.sh: %v", err)
		}
		run.code = exitErr.ExitCode()
	}
	return run
}

// dotenv returns the variables in the run's aim/.env, or none when the run
// wrote no .env.
func (r quickstartRun) dotenv() map[string]string {
	vars := map[string]string{}
	b, err := os.ReadFile(filepath.Join(r.dir, "aim", ".env"))
	if err != nil {
		return vars
	}
	for _, line := range strings.Split(string(b), "\n") {
		if name, value, ok := strings.Cut(line, "="); ok {
			vars[name] = value
		}
	}
	return vars
}

// report is the run's output with its ADMIN_PASSWORD replaced, for a failure
// message.
func (r quickstartRun) report() string {
	out := r.output
	if password := r.dotenv()["ADMIN_PASSWORD"]; password != "" {
		out = strings.ReplaceAll(out, password, "<ADMIN_PASSWORD>")
	}
	return out
}

// writeExistingInstall lays out the directory an earlier run left: the compose
// file and a .env with this ADMIN_PASSWORD.
func writeExistingInstall(t *testing.T, password string) func(dir string) {
	return func(dir string) {
		install := filepath.Join(dir, "aim")
		if err := os.Mkdir(install, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(install, "docker-compose.quickstart.yml"), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		dotenv := "ADMIN_EMAIL=admin@opena2a.org\nADMIN_PASSWORD=" + password + "\n"
		if err := os.WriteFile(filepath.Join(install, ".env"), []byte(dotenv), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// backendLog renders lines as `docker compose logs backend` prints the
// backend's log: the container's name, then the standard logger's timestamp.
func backendLog(lines ...string) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("aim-backend  | 2026/10/18 09:00:00 " + line + "\n")
	}
	return b.String()
}

func expectAdminOrganization(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(adminSeedOrgDomain).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))
}

// seedStartLine returns the line one backend start logs for the seed of
// password into the database expect describes: seedAdmin's own line, or the
// line main logs for the error seedAdmin returns.
func seedStartLine(t *testing.T, password string, expect func(sqlmock.Sqlmock)) string {
	t.Helper()
	db, mock := newSeedMock(t)
	expect(mock)
	var seedErr error
	line := logLines("ADMIN_PASSWORD", func() {
		seedErr = seedAdmin(db, config.AdminSeedConfig{Email: "admin@opena2a.org", Password: password, Name: "System Administrator"})
	})
	if seedErr != nil {
		if line != "" {
			t.Fatalf("seedAdmin logged %q and returned an error", line)
		}
		return mainSeedRefusal + seedErr.Error()
	}
	if line == "" {
		t.Fatal("seedAdmin returned no error and logged no line")
	}
	return line
}

// The line the refusal tests build from mainSeedRefusal is the line main
// writes when seedAdmin returns an error.
func TestQuickstart_MainLogsTheSeedRefusalTheScriptReads(t *testing.T) {
	requirePublicRepositoryFiles(t)
	mainSource := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	want := "if err := seedAdmin(migrationDB, cfg.Admin); err != nil {\n\t\tlog.Printf(\"" + mainSeedRefusal + "%v\", err)"
	if !strings.Contains(mainSource, want) {
		t.Fatalf("main.go no longer logs a seedAdmin error as %q followed by the error; scripts/quickstart.sh reads that line from the backend's log", mainSeedRefusal)
	}
}

// A draw whose random characters are all lower-case letters holds no
// upper-case letter, digit or special character. The ADMIN_PASSWORD
// quickstart.sh writes from it still meets the backend's password rule.
func TestQuickstart_AdminPasswordMeetsThePasswordRuleWhenTheDrawIsAllLowerCase(t *testing.T) {
	h := newQuickstartHarness(t)
	h.stub(t, "openssl", quickstartOpenSSLStub)

	run := h.run(t, "", nil)
	if run.code != 0 {
		t.Fatalf("quickstart.sh exited %d:\n%s", run.code, run.report())
	}
	password := run.dotenv()["ADMIN_PASSWORD"]
	if err := auth.NewPasswordHasher().ValidatePassword(password); err != nil {
		t.Fatalf("the backend's password rule refuses the ADMIN_PASSWORD quickstart.sh wrote (%d characters): %v", len(password), err)
	}
}

// With the real openssl, the ADMIN_PASSWORD of each of 24 runs meets the
// backend's password rule and is printed as the login, and seedAdmin creates
// the administrator from the first run's .env.
func TestQuickstart_GeneratedAdminPasswordsSeedTheAdministrator(t *testing.T) {
	h := newQuickstartHarness(t)
	hasher := auth.NewPasswordHasher()

	const runs = 24
	var first map[string]string
	refused := 0
	for i := range runs {
		run := h.run(t, "", nil)
		if run.code != 0 {
			t.Fatalf("run %d: quickstart.sh exited %d:\n%s", i, run.code, run.report())
		}
		vars := run.dotenv()
		password := vars["ADMIN_PASSWORD"]
		if !strings.Contains(run.output, "Login:      "+vars["ADMIN_EMAIL"]+" / "+password+"\n") {
			t.Errorf("run %d does not print the ADMIN_EMAIL and ADMIN_PASSWORD of its .env as the login", i)
		}
		if err := hasher.ValidatePassword(password); err != nil {
			refused++
			t.Errorf("run %d: the backend's password rule refuses the generated ADMIN_PASSWORD (%d characters): %v", i, len(password), err)
		}
		if first == nil {
			first = vars
		}
	}
	t.Logf("%d of %d generated ADMIN_PASSWORD values fail the backend's password rule", refused, runs)

	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	expectAdminOrganization(mock)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(
			sqlmock.AnyArg(), // id
			"11111111-1111-1111-1111-111111111111",
			first["ADMIN_EMAIL"],
			"System Administrator",
			sqlmock.AnyArg(), // provider_id
			bcryptOf(first["ADMIN_PASSWORD"]),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	seed := config.AdminSeedConfig{Email: first["ADMIN_EMAIL"], Password: first["ADMIN_PASSWORD"], Name: "System Administrator"}
	if err := seedAdmin(db, seed); err != nil {
		t.Fatalf("seedAdmin refuses the ADMIN_PASSWORD of the first run's .env: %v", err)
	}
}

// The backend logs a refused seed and starts anyway, so GET /health answers
// either way. quickstart.sh reads the seed line of the backend's latest start
// from its log: after a refusal it prints the backend's reason and the step
// that fixes it, prints no credentials, and exits 1.
func TestQuickstart_StopsWhenTheBackendRefusedTheAdminSeed(t *testing.T) {
	h := newQuickstartHarness(t)

	refusedPassword := seedStartLine(t, quickstartRefusedPassword, func(m sqlmock.Sqlmock) {
		expectAdminExists(m, false)
	})
	noOrganization := seedStartLine(t, seedTestPassword, func(m sqlmock.Sqlmock) {
		expectAdminExists(m, false)
		m.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
			WithArgs(adminSeedOrgDomain).
			WillReturnError(errors.New("sql: no rows in result set"))
	})
	seeded := seedStartLine(t, seedTestPassword, func(m sqlmock.Sqlmock) {
		expectAdminExists(m, false)
		expectAdminOrganization(m)
		m.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnResult(sqlmock.NewResult(0, 1))
	})
	notApplied := seedStartLine(t, seedTestPassword, func(m sqlmock.Sqlmock) {
		expectAdminExists(m, true)
	})
	for name, line := range map[string]string{"refused password": refusedPassword, "missing organization": noOrganization} {
		if !strings.HasPrefix(line, mainSeedRefusal) {
			t.Fatalf("the %s start logged %q, not a refusal", name, line)
		}
	}
	if !strings.Contains(refusedPassword, auth.ErrPasswordTooWeak.Error()) {
		t.Fatalf("the refused password's line %q does not give the password rule", refusedPassword)
	}

	const loginPrefix = "Login:"
	passwordFix := "Change ADMIN_PASSWORD in ./aim/.env so the backend accepts it, then run this script again."
	restartFix := "cd aim && docker compose -f docker-compose.quickstart.yml restart backend"
	for _, tc := range []struct {
		name    string
		log     string
		prepare func(dir string)
		code    int
		says    []string
	}{
		{
			name: "a first run whose password the backend refused",
			log:  backendLog(refusedPassword),
			code: 1,
			says: []string{strings.TrimPrefix(refusedPassword, mainSeedRefusal), passwordFix},
		},
		{
			name: "a first run whose backend found no admin organization",
			log:  backendLog(noOrganization),
			code: 1,
			says: []string{strings.TrimPrefix(noOrganization, mainSeedRefusal), restartFix},
		},
		{
			name:    "an existing install whose backend refused its password",
			log:     backendLog(refusedPassword),
			prepare: writeExistingInstall(t, quickstartRefusedPassword),
			code:    1,
			says:    []string{strings.TrimPrefix(refusedPassword, mainSeedRefusal), passwordFix},
		},
		{
			name: "a first run the backend seeded",
			log:  backendLog(seeded),
			code: 0,
			says: []string{"Login:      admin@opena2a.org / "},
		},
		{
			name: "a first run whose backend logged no seed line",
			log:  "",
			code: 0,
			says: []string{"Login:      admin@opena2a.org / "},
		},
		{
			name:    "an existing install seeded after a refusal at an earlier start",
			log:     backendLog(noOrganization, seeded),
			prepare: writeExistingInstall(t, seedTestPassword),
			code:    0,
			says:    []string{"Login:      credentials from initial setup (see .env)"},
		},
		{
			name:    "an existing install that found an administrator after a refusal at an earlier start",
			log:     backendLog(refusedPassword, notApplied),
			prepare: writeExistingInstall(t, quickstartRefusedPassword),
			code:    0,
			says:    []string{"Login:      credentials from initial setup (see .env)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := h.run(t, tc.log, tc.prepare)
			if run.code != tc.code {
				t.Fatalf("quickstart.sh exited %d, want %d:\n%s", run.code, tc.code, run.report())
			}
			for _, want := range tc.says {
				if !strings.Contains(run.output, want) {
					t.Errorf("the output lacks %q:\n%s", want, run.report())
				}
			}
			password := run.dotenv()["ADMIN_PASSWORD"]
			if tc.code == 0 {
				if tc.prepare == nil && !strings.Contains(run.output, "Login:      admin@opena2a.org / "+password+"\n") {
					t.Errorf("the output does not give the ADMIN_PASSWORD of the .env as the login:\n%s", run.report())
				}
				return
			}
			if strings.Contains(run.output, loginPrefix) {
				t.Errorf("the output gives a login after the backend refused the seed:\n%s", run.report())
			}
			if password == "" {
				t.Errorf("the run left no ADMIN_PASSWORD in aim/.env")
			} else if strings.Contains(run.output, password) {
				t.Errorf("the output carries the ADMIN_PASSWORD after the backend refused it:\n%s", run.report())
			}
			if strings.Contains(run.output, "AIM is running!") {
				t.Errorf("the output says AIM is running after the backend refused the seed:\n%s", run.report())
			}
		})
	}
}
