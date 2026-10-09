package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

type deployStepCompose struct {
	Services map[string]deployStepService `yaml:"services"`
}

type deployStepService struct {
	Image       string      `yaml:"image"`
	Ports       []string    `yaml:"ports"`
	Environment []string    `yaml:"environment"`
	DependsOn   interface{} `yaml:"depends_on"`
	Healthcheck interface{} `yaml:"healthcheck"`
}

// dependsOn reports whether svc lists dep under depends_on, and the condition
// it waits for: "service_started" for the short list form.
func (svc deployStepService) dependsOn(dep string) (bool, string) {
	switch d := svc.DependsOn.(type) {
	case []interface{}:
		for _, name := range d {
			if name == dep {
				return true, "service_started"
			}
		}
	case map[string]interface{}:
		entry, ok := d[dep]
		if !ok {
			return false, ""
		}
		if fields, ok := entry.(map[string]interface{}); ok {
			if cond, ok := fields["condition"].(string); ok {
				return true, cond
			}
		}
		return true, "service_started"
	}
	return false, ""
}

// guideSection returns the text from the second-level heading that starts
// with `heading` to the next second-level heading.
func guideSection(t *testing.T, guide, heading string) string {
	t.Helper()
	start := strings.Index(guide, "\n"+heading)
	require.NotEqual(t, -1, start, "the guide has no %q heading", heading)
	rest := guide[start+1:]
	if end := strings.Index(rest, "\n## "); end != -1 {
		return rest[:end]
	}
	return rest
}

var (
	fencedBlock        = regexp.MustCompile("(?s)```(\\w*)\n(.*?)```")
	envFileBlock       = regexp.MustCompile("(?s)cat > \\.env <<EOF\n(.*?)\nEOF\n")
	envFileLine        = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(?:\$\(openssl rand -(hex|base64) (\d+)\))?([^\s$()]*)$`)
	seededAdminLogLine = regexp.MustCompile(`(?m)^Seeded administrator .*$`)
	composeInterpolate = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?[-?][^}]*)?\}`)
	configEnvRead      = regexp.MustCompile(`getEnv\w*\("([A-Z_][A-Z0-9_]*)"`)
	healthHandlerSrc   = regexp.MustCompile(`(?s)app\.Get\("/health", func\(c fiber\.Ctx\) error \{\s*return c\.JSON\(fiber\.Map\{(.*?)\}\)\s*\}\)`)
	healthHandlerField = regexp.MustCompile(`"(\w+)":\s*([^,\n]+),`)
)

// documentedEnvFile returns the variables the step writes to .env, each with
// the value its line writes: one generated the way its openssl command
// generates one, followed by any literal text after the command.
func documentedEnvFile(t *testing.T, step string) map[string]string {
	t.Helper()
	block := envFileBlock.FindStringSubmatch(step)
	require.NotNil(t, block, "Step 1 writes no .env with cat > .env <<EOF")
	values := map[string]string{}
	for _, line := range strings.Split(block[1], "\n") {
		m := envFileLine.FindStringSubmatch(line)
		require.NotNil(t, m, "Step 1's .env line %q is not one this test can evaluate", line)
		generated := ""
		if m[2] != "" {
			n, err := strconv.Atoi(m[3])
			require.NoError(t, err)
			buf := make([]byte, n)
			_, err = rand.Read(buf)
			require.NoError(t, err)
			if m[2] == "hex" {
				generated = hex.EncodeToString(buf)
			} else {
				generated = base64.StdEncoding.EncodeToString(buf)
			}
		}
		values[m[1]] = generated + m[4]
	}
	return values
}

// serviceEnv returns a service's environment with ${NAME...} references
// replaced from .env, as docker compose does.
func serviceEnv(t *testing.T, name string, svc deployStepService, envFile map[string]string) map[string]string {
	t.Helper()
	env := map[string]string{}
	for _, entry := range svc.Environment {
		key, value, ok := strings.Cut(entry, "=")
		require.True(t, ok, "%s: environment entry %q has no value", name, entry)
		for _, ref := range composeInterpolate.FindAllStringSubmatch(value, -1) {
			_, defined := envFile[ref[1]]
			assert.True(t, defined, "%s: %s reads ${%s}, which the step does not write to .env", name, key, ref[1])
		}
		env[key] = composeInterpolate.ReplaceAllStringFunc(value, func(ref string) string {
			return envFile[composeInterpolate.FindStringSubmatch(ref)[1]]
		})
	}
	return env
}

// TestFleetGovernanceDeployStepStartsTheServer reads the fleet governance
// guide's Step 1 as docker compose would and checks that the server it
// describes starts. The step used to set DATABASE_URL, which the server does
// not read, so the server stopped at startup on the missing POSTGRES_HOST; its
// sample JWT_SECRET was under the 32 characters the server requires; the
// dashboard was given API_URL, which it does not read; and the /health output
// it showed was not the server's. A guide or a server that drifts apart on any
// of these fails here.
func TestFleetGovernanceDeployStepStartsTheServer(t *testing.T) {
	guide := aim03ReadRepoFile(t, "docs/use-cases/fleet-governance.md")
	step := guideSection(t, guide, "## Step 1")

	var composeSrc, healthJSON string
	for _, block := range fencedBlock.FindAllStringSubmatchIndex(step, -1) {
		lang, body := step[block[2]:block[3]], step[block[4]:block[5]]
		if lang == "yaml" && composeSrc == "" {
			composeSrc = body
		}
		if lang == "json" && strings.Contains(step[:block[0]], "/health") && healthJSON == "" {
			healthJSON = body
		}
	}
	require.NotEmpty(t, composeSrc, "Step 1 shows no docker-compose.yml")

	var compose deployStepCompose
	require.NoError(t, yaml.Unmarshal([]byte(composeSrc), &compose), "Step 1's docker-compose.yml must parse")
	server, ok := compose.Services["aim-server"]
	require.True(t, ok, "Step 1 defines no aim-server service; Step 4 sets AIM_BASE_URL on it by that name")

	envFile := documentedEnvFile(t, step)
	serverEnv := serviceEnv(t, "aim-server", server, envFile)

	// Start from an environment holding none of what the server reads, then
	// set exactly what the compose file gives the container.
	configSrc := aim03ReadRepoFile(t, "apps/backend/internal/config/config.go")
	read := []string{"KEYVAULT_MASTER_KEY", "ENVIRONMENT"}
	for _, m := range configEnvRead.FindAllStringSubmatch(configSrc, -1) {
		read = append(read, m[1])
	}
	for _, name := range read {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
	for k, v := range serverEnv {
		t.Setenv(k, v)
	}

	var cfg *config.Config
	var loadErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("the server stops at startup with Step 1's environment: %v", r)
			}
		}()
		cfg, loadErr = config.Load()
	}()
	require.NoError(t, loadErr, "the server refuses Step 1's environment")

	// KEYVAULT_MASTER_KEY encrypts the agent private keys stored in the
	// database. Without it the server makes a new key on each start, and keys
	// stored before a restart no longer decrypt.
	assert.NotEmpty(t, serverEnv["KEYVAULT_MASTER_KEY"], "Step 1 must give the server a KEYVAULT_MASTER_KEY")
	_, err := crypto.NewKeyVaultFromEnv()
	assert.NoError(t, err, "the server refuses Step 1's KEYVAULT_MASTER_KEY")

	assert.Contains(t, server.Ports, cfg.Server.Port+":"+cfg.Server.Port, "the server listens on %s inside the container", cfg.Server.Port)

	db, ok := compose.Services[cfg.Database.Host]
	require.True(t, ok, "POSTGRES_HOST %q names no service in Step 1", cfg.Database.Host)
	dbEnv := serviceEnv(t, cfg.Database.Host, db, envFile)
	assert.Equal(t, 5432, cfg.Database.Port, "the postgres image listens on 5432")
	assert.Equal(t, dbEnv["POSTGRES_USER"], cfg.Database.User, "the server's database user is the one the database creates")
	assert.Equal(t, dbEnv["POSTGRES_PASSWORD"], cfg.Database.Password, "the server's database password is the one the database sets")
	assert.Equal(t, dbEnv["POSTGRES_DB"], cfg.Database.Database, "the server's database is the one the database creates")
	// The server connects once at start and exits if the database is not yet
	// accepting connections, which on a first start takes a few seconds.
	assert.NotNil(t, db.Healthcheck, "the database service needs a healthcheck for the server to wait on")
	listed, condition := server.dependsOn(cfg.Database.Host)
	assert.True(t, listed && condition == "service_healthy",
		"aim-server must wait for %s with condition service_healthy, got listed=%v condition=%q", cfg.Database.Host, listed, condition)

	// Without Redis the server runs with token revocation off, and Step 4
	// says a revoked token stops working.
	_, ok = compose.Services[cfg.Redis.Host]
	assert.True(t, ok, "REDIS_HOST %q names no service in Step 1", cfg.Redis.Host)

	// Every image the step pulls from Docker Hub is one the release publishes.
	publish := aim03ReadRepoFile(t, ".github/workflows/docker-publish.yml")
	for name, svc := range compose.Services {
		repo, _, _ := strings.Cut(svc.Image, ":")
		if strings.HasPrefix(repo, "opena2a/") {
			assert.Contains(t, publish, "images: "+repo+"\n", "%s: the release publishes no %s image", name, repo)
			assert.Contains(t, step, "docker pull "+repo+"\n", "%s: Step 1 does not pull %s", name, repo)
		}
	}

	dashboard, ok := compose.Services["aim-dashboard"]
	require.True(t, ok, "Step 1 defines no aim-dashboard service")
	webSrc := aim03ReadRepoFile(t, "apps/web/next.config.ts") + aim03ReadRepoFile(t, "apps/web/lib/api.ts")
	for key := range serviceEnv(t, "aim-dashboard", dashboard, envFile) {
		assert.Contains(t, webSrc, "process.env."+key, "aim-dashboard: the dashboard does not read %s", key)
	}

	require.NotEmpty(t, healthJSON, "Step 1 shows no /health output")
	var documented map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(healthJSON), &documented), "the documented /health output must be JSON")

	// The handler is inline in main.go, pinned there by the AIM-07 tests, so
	// its fields are read from the source.
	handler := healthHandlerSrc.FindStringSubmatch(aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go"))
	require.NotNil(t, handler, "main.go mounts no GET /health handler returning a fiber.Map")
	served := map[string]string{}
	for _, field := range healthHandlerField.FindAllStringSubmatch(handler[1], -1) {
		served[field[1]] = field[2]
	}
	documentedKeys := make([]string, 0, len(documented))
	for k := range documented {
		documentedKeys = append(documentedKeys, k)
	}
	servedKeys := make([]string, 0, len(served))
	for k := range served {
		servedKeys = append(servedKeys, k)
	}
	sort.Strings(documentedKeys)
	sort.Strings(servedKeys)
	assert.Equal(t, servedKeys, documentedKeys, "the documented /health output has the fields the server returns")
	assert.Equal(t, `"healthy"`, served["status"])
	assert.Equal(t, "healthy", documented["status"])
	assert.Equal(t, `"agent-identity-management"`, served["service"])
	assert.Equal(t, "agent-identity-management", documented["service"])
	assert.Equal(t, "time.Now().UTC()", served["time"])
	if ts, ok := documented["time"].(string); assert.True(t, ok, "the documented time is a string") {
		_, err := time.Parse(time.RFC3339Nano, ts)
		assert.NoError(t, err, "the documented time is in the form a UTC time.Time takes in JSON")
	}
}

// TestFleetGovernanceDeployStepSeedsAnAdministrator checks that the stack Step
// 1 starts has the account Step 5 signs in to the dashboard with. The server
// creates no user by itself: it seeds its first administrator from
// ADMIN_EMAIL and ADMIN_PASSWORD, and only from a password with upper- and
// lower-case letters, a digit and a special character. The step used to set
// neither, so its stack had no account to sign in with.
func TestFleetGovernanceDeployStepSeedsAnAdministrator(t *testing.T) {
	guide := aim03ReadRepoFile(t, "docs/use-cases/fleet-governance.md")
	step := guideSection(t, guide, "## Step 1")

	var composeSrc string
	for _, m := range fencedBlock.FindAllStringSubmatch(step, -1) {
		if m[1] == "yaml" {
			composeSrc = m[2]
			break
		}
	}
	require.NotEmpty(t, composeSrc, "Step 1 shows no docker-compose.yml")
	var compose deployStepCompose
	require.NoError(t, yaml.Unmarshal([]byte(composeSrc), &compose), "Step 1's docker-compose.yml must parse")
	server, ok := compose.Services["aim-server"]
	require.True(t, ok, "Step 1 defines no aim-server service")

	// The password is random, so the rule is checked against many of the .env
	// files the step's command can write, not one.
	hasher := auth.NewPasswordHasher()
	var serverEnv map[string]string
	for i := 0; i < 200; i++ {
		serverEnv = serviceEnv(t, "aim-server", server, documentedEnvFile(t, step))
		require.NotEmpty(t, serverEnv["ADMIN_PASSWORD"], "Step 1 gives the server no ADMIN_PASSWORD, so it seeds no administrator")
		require.NoError(t, hasher.ValidatePassword(serverEnv["ADMIN_PASSWORD"]),
			"the server refuses the ADMIN_PASSWORD %q Step 1 wrote and seeds no administrator", serverEnv["ADMIN_PASSWORD"])
	}
	adminEmail := serverEnv["ADMIN_EMAIL"]
	require.NotEmpty(t, adminEmail, "Step 1 gives the server no ADMIN_EMAIL, so the reader is not told which account to sign in with")

	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD", serverEnv["ADMIN_PASSWORD"])
	t.Setenv("ADMIN_NAME", serverEnv["ADMIN_NAME"])
	db, mock := newSeedMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM users WHERE role = 'admin' OR LOWER(email) = LOWER($1))`)).
		WithArgs(adminEmail).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(adminSeedOrgDomain).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(
			sqlmock.AnyArg(), // id
			"11111111-1111-1111-1111-111111111111",
			adminEmail,
			sqlmock.AnyArg(), // name
			sqlmock.AnyArg(), // provider_id
			bcryptOf(serverEnv["ADMIN_PASSWORD"]),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	var logged bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logged)
	err := seedAdminFromEnv(db)
	log.SetOutput(prev)
	require.NoError(t, err, "the server does not seed an administrator from Step 1's environment")

	// The log line the step tells the reader to look for is the one the
	// server writes.
	documentedLog := seededAdminLogLine.FindString(step)
	require.NotEmpty(t, documentedLog, "Step 1 does not show the log line that confirms the administrator was seeded")
	assert.Contains(t, logged.String(), documentedLog, "the server logs a different line when it seeds the administrator")

	signIn := guideSection(t, guide, "## Step 5")
	for _, name := range []string{"ADMIN_EMAIL", "ADMIN_PASSWORD"} {
		assert.Contains(t, signIn, name, "Step 5 opens the dashboard without saying to sign in with %s from Step 1", name)
	}
}
