package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
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
	envFileLine        = regexp.MustCompile(`(?m)^([A-Z_][A-Z0-9_]*)=\$\(openssl rand -(hex|base64) (\d+)\)$`)
	composeInterpolate = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::?[-?][^}]*)?\}`)
	configEnvRead      = regexp.MustCompile(`getEnv\w*\("([A-Z_][A-Z0-9_]*)"`)
	healthHandlerSrc   = regexp.MustCompile(`(?s)app\.Get\("/health", func\(c fiber\.Ctx\) error \{\s*return c\.JSON\(fiber\.Map\{(.*?)\}\)\s*\}\)`)
	healthHandlerField = regexp.MustCompile(`"(\w+)":\s*([^,\n]+),`)
)

// documentedEnvFile returns the variables the step writes to .env, each with a
// value generated the way its openssl command generates one.
func documentedEnvFile(t *testing.T, step string) map[string]string {
	t.Helper()
	values := map[string]string{}
	for _, m := range envFileLine.FindAllStringSubmatch(step, -1) {
		n, err := strconv.Atoi(m[3])
		require.NoError(t, err)
		buf := make([]byte, n)
		_, err = rand.Read(buf)
		require.NoError(t, err)
		if m[2] == "hex" {
			values[m[1]] = hex.EncodeToString(buf)
		} else {
			values[m[1]] = base64.StdEncoding.EncodeToString(buf)
		}
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
