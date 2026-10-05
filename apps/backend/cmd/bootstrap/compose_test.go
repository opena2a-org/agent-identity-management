package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// composeService is the part of a docker compose service these tests read.
type composeService struct {
	Profiles    []string  `yaml:"profiles"`
	Restart     string    `yaml:"restart"`
	Entrypoint  yaml.Node `yaml:"entrypoint"`
	Command     yaml.Node `yaml:"command"`
	Environment yaml.Node `yaml:"environment"`
	DependsOn   yaml.Node `yaml:"depends_on"`
}

// repoRoot is the repository root, seen from apps/backend/cmd/bootstrap.
var repoRoot = filepath.Join("..", "..", "..", "..")

func readComposeServices(t *testing.T, path string) map[string]composeService {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var file struct {
		Services map[string]composeService `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(file.Services) == 0 {
		t.Fatalf("%s defines no services", path)
	}
	return file.Services
}

// environmentNames returns the variable names a service's environment sets, in
// either of the forms compose accepts: a list of NAME=value or a mapping.
func environmentNames(t *testing.T, env yaml.Node) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	switch env.Kind {
	case 0:
	case yaml.SequenceNode:
		for _, item := range env.Content {
			name, _, _ := strings.Cut(item.Value, "=")
			names[name] = true
		}
	case yaml.MappingNode:
		for i := 0; i < len(env.Content); i += 2 {
			names[env.Content[i].Value] = true
		}
	default:
		t.Fatalf("environment at line %d is neither a list nor a mapping", env.Line)
	}
	return names
}

// words returns the words of an entrypoint or command, list or string form.
func words(n yaml.Node) []string {
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

func runsBootstrap(s composeService) bool {
	for _, w := range append(words(s.Entrypoint), words(s.Command)...) {
		if filepath.Base(w) == "aim-bootstrap" {
			return true
		}
	}
	return false
}

// oneShot reports whether a service runs only when it is asked for: a profile
// keeps it out of `docker compose up`, and with no restart policy the container
// exits when the command returns.
func oneShot(s composeService) bool {
	return len(s.Profiles) > 0 && (s.Restart == "" || s.Restart == "no")
}

// TestCompose_DefaultAdminPasswordIsInNoLongRunningService holds the admin
// password that `aim-bootstrap --default` reads out of every service that keeps
// running. A long-running container keeps its environment for its whole life,
// where `docker inspect` and any process in the container can read it; the
// value is only needed for the one bootstrap run.
func TestCompose_DefaultAdminPasswordIsInNoLongRunningService(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(repoRoot, "docker-compose*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no docker-compose*.yml files under %s", repoRoot)
	}
	for _, path := range files {
		for name, svc := range readComposeServices(t, path) {
			if !environmentNames(t, svc.Environment)["DEFAULT_ADMIN_PASSWORD"] {
				continue
			}
			if !oneShot(svc) || !runsBootstrap(svc) {
				t.Errorf("%s: service %q carries DEFAULT_ADMIN_PASSWORD in its environment; only a one-shot service (a profile, no restart policy) that runs aim-bootstrap may", filepath.Base(path), name)
			}
		}
	}
}

// TestCompose_BootstrapServiceSeedsTheAdmin checks the developer stack's
// bootstrap step: `docker compose run --rm bootstrap` must reach the database
// the backend migrated and receive DEFAULT_ADMIN_PASSWORD, without starting on
// `docker compose up`.
func TestCompose_BootstrapServiceSeedsTheAdmin(t *testing.T) {
	services := readComposeServices(t, filepath.Join(repoRoot, "docker-compose.yml"))

	svc, ok := services["bootstrap"]
	if !ok {
		t.Fatal(`docker-compose.yml has no "bootstrap" service, so the documented "docker compose run --rm bootstrap" step fails`)
	}
	if !runsBootstrap(svc) {
		t.Error("the bootstrap service does not run aim-bootstrap")
	}
	if !strings.Contains(strings.Join(words(svc.Command), " "), "--default") {
		t.Error("the bootstrap service does not pass --default, so it does not seed the canonical admin")
	}
	if !oneShot(svc) {
		t.Errorf("the bootstrap service must be one-shot (a profile, no restart policy); profiles=%v restart=%q", svc.Profiles, svc.Restart)
	}
	env := environmentNames(t, svc.Environment)
	for _, name := range []string{"DATABASE_URL", "DEFAULT_ADMIN_PASSWORD"} {
		if !env[name] {
			t.Errorf("the bootstrap service does not set %s", name)
		}
	}
	// The backend applies the migrations at startup; the bootstrap writes to
	// tables they create.
	if svc.DependsOn.Kind != yaml.MappingNode {
		t.Fatal("the bootstrap service must depend on the backend with condition service_healthy")
	}
	var deps map[string]struct {
		Condition string `yaml:"condition"`
	}
	if err := svc.DependsOn.Decode(&deps); err != nil {
		t.Fatalf("decode depends_on: %v", err)
	}
	if deps["backend"].Condition != "service_healthy" {
		t.Errorf("the bootstrap service depends on backend with condition %q; want service_healthy", deps["backend"].Condition)
	}
}
