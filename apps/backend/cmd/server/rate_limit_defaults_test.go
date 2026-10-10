package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A security limit never loosens on the default value of a setting, in code or
// in a shipped deployment file. The rate limiters read ENVIRONMENT, so the
// checks here read the environment every tracked compose file gives the server
// and every tracked file that could set ENVIRONMENT=test, the one explicit
// value that raises both limits.

// composeFileName matches the file names docker compose reads.
var composeFileName = regexp.MustCompile(`^(docker-)?compose([.-][^/]*)?\.ya?ml$`)

// composeRef is one compose interpolation: $$, $NAME, ${NAME}, or ${NAME}
// followed by :-, -, :?, ?, :+ or + and an argument.
var composeRef = regexp.MustCompile(`\$\$|\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-?+])([^}]*))?\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// interpolate resolves value the way docker compose does, with vars as the
// only variables set in the invoking environment.
func interpolate(value string, vars map[string]string) (string, error) {
	var missing error
	out := composeRef.ReplaceAllStringFunc(value, func(ref string) string {
		if ref == "$$" {
			return "$"
		}
		m := composeRef.FindStringSubmatch(ref)
		name, op, arg := m[1], m[2], m[3]
		if name == "" {
			name = m[4]
		}
		v, set := vars[name]
		nonEmpty := set && v != ""
		switch op {
		case ":-":
			if !nonEmpty {
				return arg
			}
		case "-":
			if !set {
				return arg
			}
		case ":?", "?":
			if (op == ":?" && !nonEmpty) || (op == "?" && !set) {
				missing = fmt.Errorf("%s requires %s to be set", ref, name)
			}
		case ":+":
			if nonEmpty {
				return arg
			}
			return ""
		case "+":
			if set {
				return arg
			}
			return ""
		}
		return v
	})
	return out, missing
}

// parseDotenv reads KEY=VALUE lines, as compose reads a .env or env_file.
func parseDotenv(content string) map[string]string {
	vars := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		vars[strings.TrimSpace(key)] = value
	}
	return vars
}

type limiterComposeService struct {
	Image       string    `yaml:"image"`
	Build       yaml.Node `yaml:"build"`
	Entrypoint  yaml.Node `yaml:"entrypoint"`
	Command     yaml.Node `yaml:"command"`
	Environment yaml.Node `yaml:"environment"`
	EnvFile     yaml.Node `yaml:"env_file"`
}

func nodeWords(n yaml.Node) []string {
	switch n.Kind {
	case yaml.ScalarNode:
		return strings.Fields(n.Value)
	case yaml.SequenceNode:
		var out []string
		for _, item := range n.Content {
			out = append(out, item.Value)
		}
		return out
	}
	return nil
}

// runsAIMServer reports whether svc runs the server: its image is the server
// image, or it builds the backend Dockerfile, and it does not replace the
// entrypoint with the one-shot bootstrap command.
func runsAIMServer(svc limiterComposeService) bool {
	server := strings.Contains(svc.Image, "aim-server") ||
		strings.Contains(svc.Image, "aim-backend") ||
		strings.Contains(svc.Image, "AIM_SERVER_IMAGE")
	if svc.Build.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(svc.Build.Content); i += 2 {
			if svc.Build.Content[i].Value == "dockerfile" && path.Base(svc.Build.Content[i+1].Value) == "Dockerfile.backend" {
				server = true
			}
		}
	}
	for _, w := range append(nodeWords(svc.Entrypoint), nodeWords(svc.Command)...) {
		if path.Base(w) == "aim-bootstrap" {
			return false
		}
	}
	return server
}

// envFilePaths lists the env_file entries of svc, relative to the repository.
func envFilePaths(dir string, n yaml.Node) []string {
	var nodes []*yaml.Node
	switch n.Kind {
	case yaml.ScalarNode:
		nodes = []*yaml.Node{&n}
	case yaml.SequenceNode:
		nodes = n.Content
	}
	var out []string
	for _, item := range nodes {
		p := item.Value
		if item.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(item.Content); i += 2 {
				if item.Content[i].Value == "path" {
					p = item.Content[i+1].Value
				}
			}
		}
		if p != "" {
			out = append(out, path.Clean(path.Join(dir, p)))
		}
	}
	return out
}

// serverEnvironment returns the ENVIRONMENT value the compose file at file
// gives svc when the invoking shell sets nothing, and whether it is set. Only
// committed files count: a tracked .env beside the compose file supplies
// interpolation variables, and a tracked env_file supplies values; an
// untracked one is the operator's own setting, not a shipped default.
func serverEnvironment(file string, svc limiterComposeService, tracked map[string]string) (string, bool, error) {
	dir := path.Dir(file)
	vars := map[string]string{}
	if dotenv, ok := tracked[path.Join(dir, ".env")]; ok {
		vars = parseDotenv(dotenv)
	}
	env := map[string]string{}
	for _, p := range envFilePaths(dir, svc.EnvFile) {
		if content, ok := tracked[p]; ok {
			for k, v := range parseDotenv(content) {
				env[k] = v
			}
		}
	}
	switch svc.Environment.Kind {
	case yaml.SequenceNode:
		for _, item := range svc.Environment.Content {
			if key, value, ok := strings.Cut(item.Value, "="); ok {
				env[key] = value
			} else if v, set := vars[item.Value]; set {
				env[item.Value] = v
			} else {
				delete(env, item.Value)
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(svc.Environment.Content); i += 2 {
			key, value := svc.Environment.Content[i].Value, svc.Environment.Content[i+1]
			if value.Tag == "!!null" {
				if v, set := vars[key]; set {
					env[key] = v
				} else {
					delete(env, key)
				}
				continue
			}
			env[key] = value.Value
		}
	}
	raw, set := env["ENVIRONMENT"]
	if !set {
		return "", false, nil
	}
	value, err := interpolate(raw, vars)
	return value, true, err
}

// trackedContents reads the committed content of every blob keep selects.
func trackedContents(t *testing.T, top string, blobs []treeBlob, keep func(string) bool) map[string]string {
	t.Helper()
	var wanted []treeBlob
	for _, b := range blobs {
		if keep(b.path) {
			wanted = append(wanted, b)
		}
	}
	contents := make(map[string]string, len(wanted))
	err := readBlobs(top, wanted, func(b treeBlob, content []byte) {
		contents[b.path] = string(content)
	})
	require.NoError(t, err)
	require.Len(t, contents, len(wanted), "read every selected blob")
	return contents
}

// withEnvironment sets ENVIRONMENT for one test, or unsets it when set is false.
func withEnvironment(t *testing.T, value string, set bool) {
	t.Helper()
	t.Setenv("ENVIRONMENT", value)
	if !set {
		require.NoError(t, os.Unsetenv("ENVIRONMENT"))
	}
}

// admittedRequests sends up to upTo requests from one client through h and
// returns how many it admitted before the first 429.
func admittedRequests(t *testing.T, h fiber.Handler, upTo int) int {
	t.Helper()
	app := fiber.New()
	app.Use(h)
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	for i := 0; i < upTo; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		require.NoError(t, err)
		resp.Body.Close()
		switch resp.StatusCode {
		case fiber.StatusOK:
		case fiber.StatusTooManyRequests:
			return i
		default:
			t.Fatalf("request %d: status %d, want 200 or 429", i+1, resp.StatusCode)
		}
	}
	return upTo
}

// TestComposeFilesGiveTheServerTheProductionRateLimits reads every compose file
// in the commit under test, computes the ENVIRONMENT each gives the server when
// the invoking shell sets none, and runs the real limiters under that value.
// Each must admit what it admits under ENVIRONMENT=production.
func TestComposeFilesGiveTheServerTheProductionRateLimits(t *testing.T) {
	top := repositoryTop(t)
	blobs := treeBlobs(t, top, "HEAD")

	withEnvironment(t, "production", true)
	prodGeneral := admittedRequests(t, middleware.RateLimitMiddleware(), 10000)
	prodStrict := admittedRequests(t, middleware.StrictRateLimitMiddleware(), 10000)
	require.Less(t, prodGeneral, 10000, "the general limiter admitted every request under production")
	require.Less(t, prodStrict, prodGeneral, "the strict limiter must admit fewer requests than the general one")

	composeFiles := trackedContents(t, top, blobs, func(p string) bool {
		return composeFileName.MatchString(path.Base(p))
	})
	require.NotEmpty(t, composeFiles, "no compose file in the commit under test; this check reads nothing")

	paths := make([]string, 0, len(composeFiles))
	for p := range composeFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	servers := 0
	for _, file := range paths {
		var parsed struct {
			Services map[string]limiterComposeService `yaml:"services"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(composeFiles[file]), &parsed), "parse %s", file)

		names := make([]string, 0, len(parsed.Services))
		for name := range parsed.Services {
			names = append(names, name)
		}
		sort.Strings(names)

		fileServers := 0
		for _, name := range names {
			svc := parsed.Services[name]
			if !runsAIMServer(svc) {
				continue
			}
			fileServers++
			servers++
			dir := path.Dir(file)
			tracked := trackedContents(t, top, blobs, func(p string) bool {
				if p == path.Join(dir, ".env") {
					return true
				}
				for _, ef := range envFilePaths(dir, svc.EnvFile) {
					if p == ef {
						return true
					}
				}
				return false
			})
			value, set, err := serverEnvironment(file, svc, tracked)
			if err != nil {
				// The file refuses to start until the operator sets a value, so
				// it ships no default to loosen anything on.
				t.Logf("%s: service %q has no default ENVIRONMENT: %v", file, name, err)
				continue
			}

			t.Run(file+"/"+name, func(t *testing.T) {
				t.Logf("%s gives service %q ENVIRONMENT=%q (set: %v) when the invoking shell sets none", file, name, value, set)
				withEnvironment(t, value, set)
				assert.Equal(t, prodGeneral, admittedRequests(t, middleware.RateLimitMiddleware(), prodGeneral+1),
					"%s: service %q runs the general limiter looser than production with ENVIRONMENT=%q", file, name, value)
				assert.Equal(t, prodStrict, admittedRequests(t, middleware.StrictRateLimitMiddleware(), prodStrict+1),
					"%s: service %q runs the strict limiter looser than production with ENVIRONMENT=%q", file, name, value)
			})
		}
		if fileServers == 0 {
			t.Logf("%s defines no server service", file)
		}
	}
	require.NotZero(t, servers, "no compose file defines the server; the check would pass by reading nothing")
}

// The census reads these file kinds: the ones that can set a process
// environment for a deployment, a container or a script.
var envCensusFileName = regexp.MustCompile(`^Dockerfile|\.Dockerfile$|\.(ya?ml|sh|json|toml|tf|tfvars|mk|go)$|^Makefile$|^\.env`)

var (
	assignsEnvironmentTest = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])ENVIRONMENT["']?[ \t]*[:=][ \t]*["']?(?:\$\{ENVIRONMENT:?-)?test(?:["'}\s,;)]|$)`)
	dockerfileEnvTest      = regexp.MustCompile(`^\s*ENV\s+ENVIRONMENT\s+["']?test(?:["'\s]|$)`)
	namesEnvironment       = regexp.MustCompile(`^\s*(?:-\s*)?name:\s*["']?ENVIRONMENT["']?\s*$`)
	valueIsTest            = regexp.MustCompile(`^\s*value:\s*["']?test["']?\s*$`)
	// In Go source only a call or a whole environment entry sets the value;
	// prose that names the setting in a comment or a message does not.
	goSetsEnvironmentTest = regexp.MustCompile(`Setenv\(\s*"ENVIRONMENT"\s*,\s*"test"\s*\)|"ENVIRONMENT=test"`)
)

// environmentTestLines returns the 1-based numbers of the lines in the file at
// p, holding content, that set ENVIRONMENT to test. Whole-line comments set
// nothing and are skipped.
func environmentTestLines(p, content string) []int {
	goSource := strings.HasSuffix(p, ".go")
	lines := strings.Split(content, "\n")
	var hits []int
	for i, line := range lines {
		if goSource {
			if goSetsEnvironmentTest.MatchString(line) {
				hits = append(hits, i+1)
			}
			continue
		}
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if assignsEnvironmentTest.MatchString(line) || dockerfileEnvTest.MatchString(line) {
			hits = append(hits, i+1)
			continue
		}
		if i > 0 && namesEnvironment.MatchString(lines[i-1]) && valueIsTest.MatchString(line) {
			hits = append(hits, i+1)
		}
	}
	return hits
}

// censusExempt reports whether a tracked path may set ENVIRONMENT=test: the CI
// workflows, which opt the end-to-end job in, and the repository's own tests.
func censusExempt(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(p, ".github/workflows/") ||
		strings.HasSuffix(base, "_test.go") ||
		(strings.HasPrefix(base, "test-") && strings.HasSuffix(base, ".sh"))
}

func TestEnvironmentTestCensusMatcher(t *testing.T) {
	for _, tc := range []struct{ path, line string }{
		{".env.example", "ENVIRONMENT=test"},
		{"run.sh", "export ENVIRONMENT=test"},
		{"compose.yml", "      - ENVIRONMENT=test"},
		{"compose.yml", "      ENVIRONMENT: test"},
		{"deploy.yaml", `      ENVIRONMENT: "test"`},
		{"task.json", `  "ENVIRONMENT": "test",`},
		{"compose.yml", "      - ENVIRONMENT=${ENVIRONMENT:-test}"},
		{"Dockerfile", "ENV ENVIRONMENT=test"},
		{"Dockerfile.backend", "ENV ENVIRONMENT test"},
		{"run.sh", "docker run -e ENVIRONMENT=test aim-server"},
		{"deploy.sh", "  --set-env-vars ENVIRONMENT=test,PORT=8080"},
		{"main.tf", `    ENVIRONMENT = "test"`},
		{"main.go", `	os.Setenv("ENVIRONMENT", "test")`},
		{"main.go", `	cmd.Env = append(cmd.Env, "ENVIRONMENT=test")`},
	} {
		assert.Equal(t, []int{1}, environmentTestLines(tc.path, tc.line), "must flag %q in %s", tc.line, tc.path)
	}
	assert.Equal(t, []int{2}, environmentTestLines("deploy.yaml", "        - name: ENVIRONMENT\n          value: \"test\""),
		"must flag a Kubernetes name/value pair")

	for _, tc := range []struct{ path, line string }{
		{".env.example", "ENVIRONMENT=development"},
		{".env.example", "ENVIRONMENT=production"},
		{".env.example", "ENVIRONMENT=testing"},
		{".env.example", "TEST_ENVIRONMENT=test"},
		{"run.sh", "AIM_ENVIRONMENT=test"},
		{"compose.yml", "      - ENVIRONMENT=${ENVIRONMENT:-development}"},
		{"compose.yml", "      # ENVIRONMENT=test is for CI only"},
		{"main.tf", "  // ENVIRONMENT = \"test\" is for CI only"},
		{"main.go", `	if os.Getenv("ENVIRONMENT") == "test" {`},
		{"main.go", "		Max: rateLimitMax(10), // 10 req/min (100 under ENVIRONMENT=test)"},
		{"main.go", `	log.Printf("limits raised because ENVIRONMENT=test. Never set ENVIRONMENT=test in a deployment.")`},
	} {
		assert.Empty(t, environmentTestLines(tc.path, tc.line), "must not flag %q in %s", tc.line, tc.path)
	}
}

// TestNoShippedFileSetsEnvironmentTest is the census: ENVIRONMENT=test raises
// both rate limits tenfold, so it stays an explicit opt-in that only the CI
// workflows and the repository's tests may type.
func TestNoShippedFileSetsEnvironmentTest(t *testing.T) {
	top := repositoryTop(t)
	contents := trackedContents(t, top, treeBlobs(t, top, "HEAD"), func(p string) bool {
		return envCensusFileName.MatchString(path.Base(p))
	})
	require.NotEmpty(t, contents, "the census selected no files; it would pass by reading nothing")

	var found, exempt []string
	for p, content := range contents {
		for _, n := range environmentTestLines(p, content) {
			hit := fmt.Sprintf("%s:%d", p, n)
			if censusExempt(p) {
				exempt = append(exempt, hit)
			} else {
				found = append(found, hit)
			}
		}
	}
	sort.Strings(found)
	sort.Strings(exempt)
	t.Logf("read %d files; exempt settings: %v", len(contents), exempt)
	assert.Empty(t, found, "these tracked files set ENVIRONMENT=test, which raises both rate limits tenfold; "+
		"only the CI workflows and tests may set it:\n  %s", strings.Join(found, "\n  "))
}
