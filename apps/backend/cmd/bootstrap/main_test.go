package main

import (
	"bytes"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lib/pq"
	serverconfig "github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// setDatabaseEnv sets every variable databaseURL reads, the ones not in vars to
// the empty string, which databaseURL reads as unset.
func setDatabaseEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, name := range []string{"DATABASE_URL", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSL_MODE"} {
		t.Setenv(name, vars[name])
	}
}

// The database to seed is the --database-url value, then the server's
// POSTGRES_* variables, whose password reaches the driver unchanged, then
// DATABASE_URL: the order aim-breakglass reads them in.
func TestDatabaseURL_FlagThenServerVariablesThenDatabaseURL(t *testing.T) {
	server := map[string]string{
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "6543",
		"POSTGRES_USER":     "aim",
		"POSTGRES_PASSWORD": `it's\`,
		"POSTGRES_DB":       "aim",
		"POSTGRES_SSL_MODE": "require",
	}

	setDatabaseEnv(t, server)
	dsn, err := databaseURL("")
	if err != nil {
		t.Fatalf("databaseURL: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("the connection string is not a URL: %v", err)
	}
	if got, _ := u.User.Password(); got != `it's\` {
		t.Errorf("the password is %q, want %q", got, `it's\`)
	}
	if u.Host != "db.internal:6543" || u.Path != "/aim" || u.User.Username() != "aim" || u.Query().Get("sslmode") != "require" {
		t.Errorf("the connection string is %s, want the server's POSTGRES_* settings", u.Redacted())
	}

	withURL := map[string]string{"DATABASE_URL": "postgres://aim@elsewhere/other"}
	for k, v := range server {
		withURL[k] = v
	}
	setDatabaseEnv(t, withURL)
	if got, _ := databaseURL(""); got != dsn {
		t.Errorf("with POSTGRES_HOST and DATABASE_URL set, databaseURL returned %q; the POSTGRES_* variables come before DATABASE_URL", got)
	}
	if got, _ := databaseURL("postgres://aim@flag/aim"); got != "postgres://aim@flag/aim" {
		t.Errorf("with --database-url given, databaseURL returned %q; the flag comes first", got)
	}

	setDatabaseEnv(t, map[string]string{"DATABASE_URL": "postgres://aim@elsewhere/other"})
	if got, _ := databaseURL(""); got != "postgres://aim@elsewhere/other" {
		t.Errorf("with only DATABASE_URL set, databaseURL returned %q", got)
	}
}

// With nothing configured, or a POSTGRES_HOST without the user and database it
// needs, databaseURL fails and names variables, never a value.
func TestDatabaseURL_ErrorsNameVariablesNotValues(t *testing.T) {
	const secret = "not-a-real-password-1234"

	setDatabaseEnv(t, nil)
	if _, err := databaseURL(""); err == nil || !strings.Contains(err.Error(), "POSTGRES_HOST") || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("with no database configured, databaseURL returned %v; want an error naming DATABASE_URL and POSTGRES_HOST", err)
	}

	setDatabaseEnv(t, map[string]string{"POSTGRES_HOST": "db", "POSTGRES_PASSWORD": secret})
	_, err := databaseURL("")
	if err == nil {
		t.Fatal("POSTGRES_HOST without POSTGRES_USER and POSTGRES_DB must be an error")
	}
	for _, name := range []string{"POSTGRES_USER", "POSTGRES_DB"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error %q does not name %s", err, name)
		}
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error carries the password: %q", err)
	}
}

// openDatabase accepts the URL built from the server's POSTGRES_* settings,
// for a host name and for the directory of a Unix socket, and opens nothing
// until the database is used.
func TestOpenDatabase_AcceptsTheURLOfTheServerVariables(t *testing.T) {
	for _, host := range []string{"db.internal", "/var/run/postgresql"} {
		database := serverconfig.DatabaseConfig{Host: host, Port: 6543, User: "aim", Password: `it's\`, Database: "aim", SSLMode: "disable"}
		db, err := openDatabase(database.URL())
		if err != nil {
			t.Errorf("POSTGRES_HOST %q: openDatabase refused the URL: %v", host, err)
			continue
		}
		db.Close()
	}
}

// For a connection string the driver does not accept, openDatabase answers a
// sentence that holds no part of the string. The driver's own error quotes
// the whole of a URL it cannot parse, and a part of a key=value string.
func TestOpenDatabase_RefusedConnectionStringIsNotQuoted(t *testing.T) {
	const (
		user     = "seed-user"
		database = "seed-db"
		secret   = "not-a-real-password-1234"
	)
	inURL := func(host string) string {
		u := url.URL{Scheme: "postgres", User: url.UserPassword(user, secret), Host: host, Path: "/" + database}
		return u.String()
	}
	keyValue := "host=db.internal user=" + user + " dbname=" + database + " password="
	cases := []struct {
		name string
		dsn  string
		// quoted is true when the driver's own error holds the password.
		quoted bool
	}{
		{"a URL whose host holds a space", inURL("db host:5432"), true},
		{"a URL whose host is a directory", inURL(net.JoinHostPort("/var/run/postgresql", "5432")), true},
		{"a key=value string whose password is two words", keyValue + "first " + secret, true},
		{"a key=value string whose quote does not end", keyValue + "'" + secret, false},
	}
	for _, c := range cases {
		_, driverErr := pq.NewConnector(c.dsn)
		if driverErr == nil {
			t.Errorf("%s: the driver accepts the string, so the case tests nothing", c.name)
			continue
		}
		if c.quoted && !strings.Contains(driverErr.Error(), secret) {
			t.Errorf("%s: the driver's error does not hold the password, so the case does not show what is kept out", c.name)
		}

		db, err := openDatabase(c.dsn)
		if err == nil {
			db.Close()
			t.Errorf("%s: openDatabase accepted the string", c.name)
			continue
		}
		if !errors.Is(err, errConnectionStringRefused) {
			t.Errorf("%s: openDatabase answered an error of its own making, want errConnectionStringRefused", c.name)
		}
		for _, part := range []string{secret, user, database, "db host", "db.internal", "/var/run/postgresql", "5432"} {
			if strings.Contains(err.Error(), part) {
				t.Errorf("%s: the error holds %q, a part of the connection string", c.name, part)
			}
		}
	}
}

// setDriverEnv sets the two variables the driver reads a setting from and
// accepts one value of, PGCLIENTENCODING and PGDATESTYLE, each to its value
// in vars, and unsets the one that vars does not hold. Unset is not empty: the
// driver refuses an encoding that is set and empty.
func setDriverEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, name := range []string{"PGCLIENTENCODING", "PGDATESTYLE"} {
		value, set := vars[name]
		t.Setenv(name, value)
		if !set {
			// The Setenv above restores the variable when the test ends.
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unset %s: %v", name, err)
			}
		}
	}
}

// A connection string the driver would accept but for PGCLIENTENCODING or
// PGDATESTYLE is answered with the name of that variable, and not as a refused
// connection string. The driver reads both from the environment, and a setting
// of the string comes before either. The answer holds no part of the string
// and not the variable's value.
func TestOpenDatabase_NamesTheEnvironmentVariableTheDriverRefuses(t *testing.T) {
	const (
		user     = "seed-user"
		database = "seed-db"
		secret   = "not-a-real-password-1234"
	)
	settings := serverconfig.DatabaseConfig{Host: "db.internal", Port: 6543, User: user, Password: secret, Database: database, SSLMode: "disable"}
	fromVariables := settings.URL()
	postgresqlScheme := "postgresql" + strings.TrimPrefix(fromVariables, "postgres")
	keyValue := "host=db.internal port=6543 user=" + user + " password=" + secret + " dbname=" + database
	socket := serverconfig.DatabaseConfig{Host: "/var/run/postgresql", Port: 6543, User: user, Password: secret, Database: database, SSLMode: "disable"}.URL()

	cases := []struct {
		name string
		env  map[string]string
		dsn  string
		// named holds the variables the answer names; none means the answer
		// is errConnectionStringRefused.
		named []string
		// accepted is true when the driver accepts the string.
		accepted bool
	}{
		{name: "PGCLIENTENCODING the driver refuses, the URL of the POSTGRES_* variables",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: fromVariables, named: []string{"PGCLIENTENCODING"}},
		{name: "PGCLIENTENCODING the driver refuses, a postgresql:// URL",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: postgresqlScheme, named: []string{"PGCLIENTENCODING"}},
		{name: "PGCLIENTENCODING the driver refuses, a key=value string",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: keyValue, named: []string{"PGCLIENTENCODING"}},
		{name: "PGCLIENTENCODING the driver refuses, the URL of a socket directory",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: socket, named: []string{"PGCLIENTENCODING"}},
		{name: "PGCLIENTENCODING set and empty",
			env: map[string]string{"PGCLIENTENCODING": ""}, dsn: fromVariables, named: []string{"PGCLIENTENCODING"}},
		{name: "PGDATESTYLE the driver refuses, the URL of the POSTGRES_* variables",
			env: map[string]string{"PGDATESTYLE": "German"}, dsn: fromVariables, named: []string{"PGDATESTYLE"}},
		{name: "PGDATESTYLE the driver refuses, a key=value string",
			env: map[string]string{"PGDATESTYLE": "German"}, dsn: keyValue, named: []string{"PGDATESTYLE"}},
		{name: "both variables refused",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1", "PGDATESTYLE": "German"}, dsn: fromVariables, named: []string{"PGCLIENTENCODING", "PGDATESTYLE"}},
		{name: "PGCLIENTENCODING refused and PGDATESTYLE accepted",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1", "PGDATESTYLE": "ISO, MDY"}, dsn: fromVariables, named: []string{"PGCLIENTENCODING"}},

		{name: "both variables at the value the driver accepts",
			env: map[string]string{"PGCLIENTENCODING": "UTF8", "PGDATESTYLE": "ISO, MDY"}, dsn: fromVariables, accepted: true},
		{name: "PGCLIENTENCODING refused, and the URL sets the encoding the driver accepts",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: fromVariables + "&client_encoding=UTF8", accepted: true},
		{name: "PGDATESTYLE refused, and the string sets the date style the driver accepts",
			env: map[string]string{"PGDATESTYLE": "German"}, dsn: keyValue + " datestyle='ISO, MDY'", accepted: true},

		{name: "no variable set, the URL sets an encoding the driver refuses",
			dsn: fromVariables + "&client_encoding=LATIN1"},
		{name: "PGCLIENTENCODING refused, and the URL sets an encoding the driver refuses",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: fromVariables + "&client_encoding=LATIN1"},
		{name: "PGDATESTYLE refused, and the string sets a date style the driver refuses",
			env: map[string]string{"PGDATESTYLE": "German"}, dsn: keyValue + " datestyle=German"},
		{name: "PGCLIENTENCODING refused, and a URL the driver cannot parse",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: strings.Replace(fromVariables, "db.internal", "db host", 1)},
		{name: "PGCLIENTENCODING refused, and a key=value string whose quote does not end",
			env: map[string]string{"PGCLIENTENCODING": "LATIN1"}, dsn: "host=db.internal user=" + user + " dbname=" + database + " password='" + secret},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setDriverEnv(t, c.env)

			_, driverErr := pq.NewConnector(c.dsn)
			if (driverErr == nil) != c.accepted {
				t.Fatalf("the driver answered %v, so the case does not test what its name says", driverErr != nil)
			}

			db, err := openDatabase(c.dsn)
			if c.accepted {
				if err != nil {
					t.Fatalf("openDatabase refused a string the driver accepts: %v", err)
				}
				db.Close()
				return
			}
			if err == nil {
				db.Close()
				t.Fatal("openDatabase accepted the string")
			}
			for _, part := range []string{secret, user, database, "db host", "db.internal", "/var/run/postgresql", "6543", "LATIN1", "German"} {
				if strings.Contains(err.Error(), part) {
					t.Errorf("the error holds %q, a part of the connection string or the value of a variable: %q", part, err)
				}
			}
			if len(c.named) == 0 {
				if !errors.Is(err, errConnectionStringRefused) {
					t.Errorf("openDatabase answered %q, want errConnectionStringRefused: the driver refuses this string whatever the environment holds", err)
				}
				return
			}
			if !errors.Is(err, errDriverEnvironmentRefused) || errors.Is(err, errConnectionStringRefused) {
				t.Errorf("openDatabase answered %q, want errDriverEnvironmentRefused: the driver accepts this string without the variable", err)
			}
			for _, name := range []string{"PGCLIENTENCODING", "PGDATESTYLE"} {
				named := strings.Contains(err.Error(), name)
				want := false
				for _, n := range c.named {
					want = want || n == name
				}
				if named != want {
					t.Errorf("the error %q names %s: %t, want %t", err, name, named, want)
				}
			}
		})
	}
}

// helperPassword is the database password the child processes below are
// given. It holds only characters a URL leaves as they are, so a line that
// quotes the connection URL holds it as written here.
const helperPassword = "not-a-real-password-1234"

// runMain runs main() with --default --yes in a child copy of the test
// binary, from an empty directory, with no variable set but the ones given.
// It returns the exit code and what the child wrote, standard output and
// standard error together.
func runMain(t *testing.T, env map[string]string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"AIM_BOOTSTRAP_HELPER_PROCESS=1"}
	for name, value := range env {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("the child process did not run: %v", err)
	}
	return code, output.String()
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("AIM_BOOTSTRAP_HELPER_PROCESS") != "1" {
		t.Skip("runs main() for runMain")
	}
	os.Args = []string{"aim-bootstrap", "--default", "--yes"}
	main()
	os.Exit(0)
}

// With POSTGRES_HOST naming the directory of a Unix socket, aim-bootstrap
// dials the socket of POSTGRES_PORT in that directory. The directory here
// holds no socket, so the dial fails, and the line that says so holds no
// password.
func TestMainDialsTheUnixSocketInAPostgresHostDirectory(t *testing.T) {
	dir := t.TempDir()
	code, output := runMain(t, map[string]string{
		"POSTGRES_HOST":     dir,
		"POSTGRES_PORT":     "6543",
		"POSTGRES_USER":     "aim",
		"POSTGRES_PASSWORD": helperPassword,
		"POSTGRES_DB":       "aim",
	})
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if want := "Failed to ping database: dial unix " + filepath.Join(dir, ".s.PGSQL.6543"); !strings.Contains(output, want) {
		t.Errorf("the output does not hold %q", want)
	}
	if strings.Contains(output, errConnectionStringRefused.Error()) {
		t.Error("the driver refused the connection string built for a socket directory")
	}
	if strings.Contains(output, helperPassword) {
		t.Error("the output holds the database password")
	}
}

// A connection string the driver does not accept ends the run with the
// sentence of errConnectionStringRefused, from the POSTGRES_* variables and
// from DATABASE_URL alike, and no line of the output holds the password. A
// host with a space makes such a string: a URL's host cannot hold one.
func TestMainReportsARefusedConnectionStringWithoutQuotingIt(t *testing.T) {
	refused := url.URL{Scheme: "postgres", User: url.UserPassword("aim", helperPassword), Host: "db host:5432", Path: "/aim"}
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"the POSTGRES_* variables", map[string]string{
			"POSTGRES_HOST":     "db host",
			"POSTGRES_USER":     "aim",
			"POSTGRES_PASSWORD": helperPassword,
			"POSTGRES_DB":       "aim",
		}},
		{"DATABASE_URL", map[string]string{"DATABASE_URL": refused.String()}},
	}
	for _, c := range cases {
		code, output := runMain(t, c.env)
		if code != 1 {
			t.Errorf("%s: exit code %d, want 1", c.name, code)
		}
		if want := "Failed to connect to database: " + errConnectionStringRefused.Error(); !strings.Contains(output, want) {
			t.Errorf("%s: the output does not hold %q", c.name, want)
		}
		if strings.Contains(output, helperPassword) {
			t.Errorf("%s: the output holds the database password", c.name)
		}
	}
}

// With POSTGRES_* variables the driver accepts and a PGCLIENTENCODING or a
// PGDATESTYLE it refuses, the run ends with the name of that variable and not
// with the sentence of errConnectionStringRefused: the connection string is
// not what the driver refused. No line of the output holds the password.
func TestMainNamesTheEnvironmentVariableTheDriverRefuses(t *testing.T) {
	cases := []struct {
		variable string
		value    string
		want     string
	}{
		{"PGCLIENTENCODING", "LATIN1", "PGCLIENTENCODING must be unset or UTF8"},
		{"PGDATESTYLE", "German", `PGDATESTYLE must be unset or "ISO, MDY"`},
	}
	for _, c := range cases {
		code, output := runMain(t, map[string]string{
			"POSTGRES_HOST":     "db.internal",
			"POSTGRES_USER":     "aim",
			"POSTGRES_PASSWORD": helperPassword,
			"POSTGRES_DB":       "aim",
			c.variable:          c.value,
		})
		if code != 1 {
			t.Errorf("%s: exit code %d, want 1", c.variable, code)
		}
		if want := "Failed to connect to database: " + errDriverEnvironmentRefused.Error() + ": " + c.want; !strings.Contains(output, want) {
			t.Errorf("%s: the output does not hold %q", c.variable, want)
		}
		if strings.Contains(output, errConnectionStringRefused.Error()) {
			t.Errorf("%s: the output reports a refused connection string", c.variable)
		}
		if strings.Contains(output, helperPassword) {
			t.Errorf("%s: the output holds the database password", c.variable)
		}
	}
}

// TestApplyDefaultBootstrapValues_FillsEmptyFields confirms that --default mode
// fills every empty canonical field and that operator-supplied flag values are
// preserved.
func TestApplyDefaultBootstrapValues_FillsEmptyFields(t *testing.T) {
	t.Parallel()

	cfg := &BootstrapConfig{
		AdminName: "System Administrator", // flag default
		OrgDomain: "localhost",            // flag default
		MaxUsers:  100,                    // flag default
		MaxAgents: 1000,                   // flag default
	}

	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	if cfg.AdminEmail != defaultAdminEmail {
		t.Errorf("AdminEmail = %q, want %q", cfg.AdminEmail, defaultAdminEmail)
	}
	if cfg.OrgName != defaultOrgName {
		t.Errorf("OrgName = %q, want %q", cfg.OrgName, defaultOrgName)
	}
	if cfg.OrgDomain != defaultOrgDomain {
		t.Errorf("OrgDomain = %q, want %q", cfg.OrgDomain, defaultOrgDomain)
	}
	if cfg.MaxUsers != defaultMaxUsers {
		t.Errorf("MaxUsers = %d, want %d", cfg.MaxUsers, defaultMaxUsers)
	}
	if cfg.MaxAgents != defaultMaxAgents {
		t.Errorf("MaxAgents = %d, want %d", cfg.MaxAgents, defaultMaxAgents)
	}
	if !cfg.SkipPrompts {
		t.Error("SkipPrompts = false, want true (--default implies --yes)")
	}
	if cfg.AdminPassword == "" {
		t.Error("AdminPassword is empty after applyDefaultBootstrapValues; should be auto-generated")
	}
	if !cfg.passwordWasGenerated {
		t.Error("passwordWasGenerated = false; expected true when password was auto-filled")
	}
}

// TestApplyDefaultBootstrapValues_PreservesOperatorOverrides confirms that an
// operator running `aim-bootstrap --default --org-name=X --admin-password=Y`
// keeps the explicit X and Y instead of overwriting them with canonicals.
func TestApplyDefaultBootstrapValues_PreservesOperatorOverrides(t *testing.T) {
	t.Parallel()

	cfg := &BootstrapConfig{
		AdminEmail:    "custom@example.com",
		AdminPassword: "OperatorChosenP@ssword1",
		OrgName:       "CustomOrg",
		OrgDomain:     "custom.example.com",
		MaxUsers:      500,
		MaxAgents:     5000,
	}

	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	if cfg.AdminEmail != "custom@example.com" {
		t.Errorf("AdminEmail clobbered: got %q", cfg.AdminEmail)
	}
	if cfg.AdminPassword != "OperatorChosenP@ssword1" {
		t.Errorf("AdminPassword clobbered: got %q", cfg.AdminPassword)
	}
	if cfg.passwordWasGenerated {
		t.Error("passwordWasGenerated = true; expected false when password was operator-supplied")
	}
	if cfg.OrgName != "CustomOrg" {
		t.Errorf("OrgName clobbered: got %q", cfg.OrgName)
	}
	if cfg.OrgDomain != "custom.example.com" {
		t.Errorf("OrgDomain clobbered: got %q", cfg.OrgDomain)
	}
	if cfg.MaxUsers != 500 {
		t.Errorf("MaxUsers clobbered: got %d", cfg.MaxUsers)
	}
	if cfg.MaxAgents != 5000 {
		t.Errorf("MaxAgents clobbered: got %d", cfg.MaxAgents)
	}
}

// TestGenerateRandomPassword_PassesValidator runs the random-password generator
// many times and asserts every output passes PasswordHasher.ValidatePassword.
// The 4-char-class guarantee in generateRandomPassword should make this
// deterministic, not probabilistic — a single failure is a real defect.
func TestGenerateRandomPassword_PassesValidator(t *testing.T) {
	t.Parallel()
	hasher := auth.NewPasswordHasher()

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		pw, err := generateRandomPassword()
		if err != nil {
			t.Fatalf("iteration %d: generateRandomPassword: %v", i, err)
		}
		if err := hasher.ValidatePassword(pw); err != nil {
			t.Fatalf("iteration %d: validator rejected %q: %v", i, pw, err)
		}
	}
}

// TestGenerateRandomPassword_Uniqueness asserts that the generator does not
// return the same password twice in a tight loop. A handful of collisions
// across 1000 iterations would indicate the rand source is unseeded or the
// length is too small.
func TestGenerateRandomPassword_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		pw, err := generateRandomPassword()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[pw]; dup {
			t.Fatalf("collision at iteration %d: password %q already seen", i, pw)
		}
		seen[pw] = struct{}{}
	}
}

// TestGenerateRandomPassword_LengthAndCharset asserts the contract documented
// in the generateRandomPassword comment: exactly 32 chars, drawn from the
// look-alike-free union charset, with at least one char from each class.
func TestGenerateRandomPassword_LengthAndCharset(t *testing.T) {
	t.Parallel()
	pw, err := generateRandomPassword()
	if err != nil {
		t.Fatalf("generateRandomPassword: %v", err)
	}
	if got, want := len(pw), 32; got != want {
		t.Errorf("len = %d, want %d", got, want)
	}

	// No look-alike chars permitted.
	for _, ch := range "IOl01" {
		if strings.ContainsRune(pw, ch) {
			t.Errorf("password %q contains look-alike char %q", pw, ch)
		}
	}

	// Each class present at least once.
	classes := map[string]*regexp.Regexp{
		"upper":   regexp.MustCompile(`[A-Z]`),
		"lower":   regexp.MustCompile(`[a-z]`),
		"digit":   regexp.MustCompile(`[0-9]`),
		"special": regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]`),
	}
	for name, re := range classes {
		if !re.MatchString(pw) {
			t.Errorf("password %q missing %s class", pw, name)
		}
	}
}

// TestSignInURL_NamesARealDashboardPage pins the sign-in URL bootstrap prints
// to a page the dashboard serves: the path is /auth/login and a Next.js page
// exists for it under apps/web/app. A path with no page (such as /login) only
// reaches the sign-in form through a redirect.
func TestSignInURL_NamesARealDashboardPage(t *testing.T) {
	t.Parallel()

	u, err := url.Parse(signInURL)
	if err != nil {
		t.Fatalf("signInURL %q does not parse: %v", signInURL, err)
	}
	if u.Path != "/auth/login" {
		t.Errorf("signInURL path = %q, want %q", u.Path, "/auth/login")
	}

	page := filepath.Join("..", "..", "..", "web", "app", filepath.FromSlash(strings.TrimPrefix(u.Path, "/")), "page.tsx")
	if _, err := os.Stat(page); err != nil {
		t.Errorf("no dashboard page for %s (looked for %s): %v", u.Path, page, err)
	}
}

// TestSignInMessage_IsPlainText confirms the printed sign-in line carries the
// URL and no emoji or other non-ASCII characters.
func TestSignInMessage_IsPlainText(t *testing.T) {
	t.Parallel()

	msg := signInMessage()
	if !strings.HasSuffix(msg, signInURL) {
		t.Errorf("signInMessage() = %q, want it to end with %q", msg, signInURL)
	}
	for _, r := range msg {
		if r < 0x20 || r > 0x7e {
			t.Errorf("signInMessage() contains non-ASCII rune %U: %q", r, msg)
			break
		}
	}
}

// TestPrintBootstrapResult_SuppliedPasswordIsNotPrinted covers
// `aim-bootstrap --admin-password=Y`: the operator already knows Y, so the
// closing summary names the admin account without echoing Y into stdout.
func TestPrintBootstrapResult_SuppliedPasswordIsNotPrinted(t *testing.T) {
	t.Parallel()

	const supplied = "OperatorChosenP@ssword1"
	cfg := &BootstrapConfig{
		AdminEmail:    "custom@example.com",
		AdminPassword: supplied,
	}

	var out bytes.Buffer
	printBootstrapResult(&out, cfg)

	if strings.Contains(out.String(), supplied) {
		t.Errorf("output contains the supplied password:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Admin Credentials") {
		t.Errorf("output prints the credential block for a supplied password:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "custom@example.com") {
		t.Errorf("output does not name the admin account:\n%s", out.String())
	}
}

// TestPrintBootstrapResult_EnvSuppliedPasswordIsNotPrinted covers
// `aim-bootstrap --default` with DEFAULT_ADMIN_PASSWORD set, which exists so
// the password is kept out of the deploy log.
func TestPrintBootstrapResult_EnvSuppliedPasswordIsNotPrinted(t *testing.T) {
	const supplied = "EnvChosenP@ssword1"
	t.Setenv("DEFAULT_ADMIN_PASSWORD", supplied)

	cfg := &BootstrapConfig{
		AdminName: "System Administrator",
		OrgDomain: "localhost",
		MaxUsers:  100,
		MaxAgents: 1000,
	}
	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	var out bytes.Buffer
	printBootstrapResult(&out, cfg)

	if strings.Contains(out.String(), supplied) {
		t.Errorf("output contains the DEFAULT_ADMIN_PASSWORD value:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Admin Credentials") {
		t.Errorf("output prints the credential block for a supplied password:\n%s", out.String())
	}
}

// TestPrintBootstrapResult_GeneratedPasswordIsPrinted covers
// `aim-bootstrap --default` with no password supplied: this run's stdout is
// the only place the generated password can be read from.
func TestPrintBootstrapResult_GeneratedPasswordIsPrinted(t *testing.T) {
	t.Setenv("DEFAULT_ADMIN_PASSWORD", "")

	cfg := &BootstrapConfig{
		AdminName: "System Administrator",
		OrgDomain: "localhost",
		MaxUsers:  100,
		MaxAgents: 1000,
	}
	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	var out bytes.Buffer
	printBootstrapResult(&out, cfg)

	for _, want := range []string{
		"Admin Credentials",
		"Email:    " + defaultAdminEmail,
		"Password: " + cfg.AdminPassword,
		"CAPTURE IT NOW",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q", want)
		}
	}
}

// TestPrintBootstrapResult_ExistingAdminPrintsNoPassword covers the
// --default re-run whose admin INSERT was a no-op: the generated password was
// never stored, so printing it would hand the operator a wrong password.
func TestPrintBootstrapResult_ExistingAdminPrintsNoPassword(t *testing.T) {
	t.Parallel()

	const generated = "NeverStoredP@ssword1"
	cfg := &BootstrapConfig{
		AdminEmail:           defaultAdminEmail,
		AdminPassword:        generated,
		passwordWasGenerated: true,
		adminAlreadyExisted:  true,
	}

	var out bytes.Buffer
	printBootstrapResult(&out, cfg)

	if strings.Contains(out.String(), generated) {
		t.Errorf("output contains a password that was never stored:\n%s", out.String())
	}
}
