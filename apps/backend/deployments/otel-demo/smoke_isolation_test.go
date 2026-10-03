// Package oteldemo holds no Go code; this file tests the shell harnesses in
// this directory.
//
// smoke-test.sh and smoke-backend.sh boot containers of their own. They may
// only ever stop or remove what the same run created: a developer's running
// aim-otel-demo stack (fixed project and container names, data in named
// volumes) or a container the run did not start is never theirs to take down.
// Each test runs a script against a recording docker stub on PATH, so no real
// container is touched.
package oteldemo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The demo stack's fixed names, as a developer running `docker compose up -d`
// in this directory gets them.
var demoContainers = []string{"aim-otel-collector", "aim-tempo", "aim-prometheus", "aim-loki", "aim-grafana"}

const dockerStub = `#!/usr/bin/env bash
printf '%s|PREFIX=%s\n' "$*" "${AIM_OTEL_CONTAINER_PREFIX:-}" >> "$DOCKER_LOG"
args=" $* "
if [[ $args == *" compose "* && $args == *" up "* ]]; then exit "${STUB_UP_EXIT:-1}"; fi
if [[ $args == " run "* ]]; then
  [ "${STUB_RUN_EXIT:-0}" = 0 ] && echo stubcontainerid
  exit "${STUB_RUN_EXIT:-0}"
fi
exit 0
`

type smokeRun struct {
	exitCode int
	output   string
	calls    []string // one docker argv per entry, "|PREFIX=<value>" appended
}

// runSmoke copies script and the compose file into a scratch tree (so a
// developer's .env or build artifacts in this directory play no part) and runs
// it with stubs for every external tool it checks for.
func runSmoke(t *testing.T, script string, env ...string) smokeRun {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "backend", "deployments", "otel-demo")
	stubs := filepath.Join(root, "stubs")
	for _, d := range []string{dir, stubs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{script, "docker-compose.yml"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub := func(name, body string) {
		if err := os.WriteFile(filepath.Join(stubs, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub("docker", dockerStub)
	stub("curl", "#!/usr/bin/env bash\nexit 1\n")
	stub("lsof", "#!/usr/bin/env bash\nexit \"${STUB_LSOF_EXIT:-1}\"\n")
	for _, ok := range []string{"jq", "go", "psql", "pg_isready"} {
		stub(ok, "#!/usr/bin/env bash\nexit 0\n")
	}

	log := filepath.Join(root, "docker.log")
	// A script that gets past compose up waits minutes on health checks the
	// stub never answers; cut it off rather than hang the suite.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(dir, script))
	cmd.Env = append([]string{
		"PATH=" + stubs + string(os.PathListSeparator) + "/usr/bin:/bin",
		"HOME=" + root,
		"DOCKER_LOG=" + log,
		"JWT_SECRET=stub-not-a-secret",
	}, env...)
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("%s ran past compose up and hung on health checks:\n%s", script, out)
	}
	r := smokeRun{output: string(out)}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		r.exitCode = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v", script, err)
	}
	if b, err := os.ReadFile(log); err == nil {
		r.calls = strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	return r
}

var projectFlag = regexp.MustCompile(`(?:^| )-p ([^ |]+)`)

// project returns the single compose project every compose call of the run
// named, failing when a compose call named none or two calls disagree.
func (r smokeRun) project(t *testing.T) string {
	t.Helper()
	proj := ""
	for _, c := range r.calls {
		if !strings.HasPrefix(c, "compose ") {
			continue
		}
		m := projectFlag.FindStringSubmatch(c)
		if m == nil {
			t.Fatalf("compose call without -p acts on whatever project the compose file names (aim-otel-demo): %q", c)
		}
		if proj != "" && m[1] != proj {
			t.Fatalf("compose calls name two projects, %q and %q", proj, m[1])
		}
		proj = m[1]
	}
	if proj == "" {
		t.Fatalf("no compose call recorded; calls: %q\noutput:\n%s", r.calls, r.output)
	}
	return proj
}

// assertOnlyOwnTeardown fails on any call that stops or removes something
// before the run created anything, and on any call naming the demo stack.
func (r smokeRun) assertOnlyOwnTeardown(t *testing.T) {
	t.Helper()
	created := false
	for _, c := range r.calls {
		args := strings.SplitN(c, "|", 2)[0]
		for _, name := range append(demoContainers, "aim-otel-demo") {
			for _, f := range strings.Fields(args) {
				if f == name {
					t.Errorf("call names the developer's demo stack (%s): %q", name, c)
				}
			}
		}
		if strings.Contains(" "+args+" ", " up ") || strings.HasPrefix(args, "run ") {
			created = true
			continue
		}
		destructive := strings.HasPrefix(args, "rm ") || strings.HasPrefix(args, "stop ") || strings.HasPrefix(args, "kill ") ||
			strings.Contains(" "+args+" ", " down ")
		if destructive && !created {
			t.Errorf("stops or removes before this run created anything, so it can only hit someone else's containers: %q", c)
		}
	}
}

func TestComposeContainerNamesTakeAPrefix(t *testing.T) {
	b, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s*container_name:\s*(\S+)\s*$`)
	got := re.FindAllStringSubmatch(string(b), -1)
	if len(got) != len(demoContainers) {
		t.Fatalf("want %d container_name lines, got %d", len(demoContainers), len(got))
	}
	for i, m := range got {
		const prefix = "${AIM_OTEL_CONTAINER_PREFIX:-aim}-"
		if !strings.HasPrefix(m[1], prefix) {
			t.Errorf("container_name %q does not start with %s, so a smoke run collides with the demo stack's container", m[1], prefix)
			continue
		}
		// Unset, the prefix keeps the names a developer already has.
		if def := "aim-" + strings.TrimPrefix(m[1], prefix); def != demoContainers[i] {
			t.Errorf("default container name %q, want %q", def, demoContainers[i])
		}
	}
}

func TestSmokeTestUsesItsOwnProjectAndRemovesOnlyIt(t *testing.T) {
	a := runSmoke(t, "smoke-test.sh")
	if a.exitCode != 2 {
		t.Fatalf("want exit 2 (compose up failed in the stub), got %d\n%s", a.exitCode, a.output)
	}
	a.assertOnlyOwnTeardown(t)
	proj := a.project(t)

	var up, down string
	for _, c := range a.calls {
		switch {
		case strings.HasPrefix(c, "compose ") && strings.Contains(c, " up "):
			up = c
		case strings.HasPrefix(c, "compose ") && strings.Contains(c, " down"):
			down = c
		}
	}
	if !strings.HasSuffix(up, "|PREFIX="+proj) {
		t.Errorf("compose up must name containers after the run's project %q: %q", proj, up)
	}
	if down == "" || !strings.Contains(down, " -v") {
		t.Errorf("exit must take down the run's own project with its volumes; got %q", down)
	}

	if b := runSmoke(t, "smoke-test.sh"); b.project(t) == proj {
		t.Errorf("two runs share compose project %q", proj)
	}
}

func TestSmokeTestLeavesABusyPortAlone(t *testing.T) {
	// A running demo stack holds the ports. The run must say so and stop,
	// not take the stack down to free them.
	r := runSmoke(t, "smoke-test.sh", "STUB_LSOF_EXIT=0")
	if r.exitCode != 1 {
		t.Fatalf("want exit 1 on a port conflict, got %d\n%s", r.exitCode, r.output)
	}
	r.assertOnlyOwnTeardown(t)
	for _, c := range r.calls {
		if strings.HasPrefix(c, "rm ") || strings.Contains(c, " down") {
			t.Errorf("port conflict must not remove anything: %q", c)
		}
	}
	if !strings.Contains(r.output, "docker compose stop") {
		t.Errorf("port-conflict message should name the data-preserving stop; got:\n%s", r.output)
	}
}

func TestSmokeTestKeepStacksPrintsItsOwnTeardown(t *testing.T) {
	r := runSmoke(t, "smoke-test.sh", "KEEP_STACKS=1")
	proj := r.project(t)
	for _, c := range r.calls {
		if strings.Contains(c, " down") {
			t.Errorf("KEEP_STACKS=1 must leave the stack up: %q", c)
		}
	}
	if want := "docker compose -p " + proj + " down -v"; !strings.Contains(r.output, want) {
		t.Errorf("KEEP_STACKS=1 output should print %q; got:\n%s", want, r.output)
	}
}

func TestSmokeBackendUsesItsOwnProjectAndPostgres(t *testing.T) {
	a := runSmoke(t, "smoke-backend.sh")
	if a.exitCode != 2 {
		t.Fatalf("want exit 2 (compose up failed in the stub), got %d\n%s", a.exitCode, a.output)
	}
	a.assertOnlyOwnTeardown(t)
	proj := a.project(t)

	pg := ""
	for _, c := range a.calls {
		if strings.HasPrefix(c, "run ") {
			m := regexp.MustCompile(`--name ([^ |]+)`).FindStringSubmatch(c)
			if m == nil {
				t.Fatalf("docker run without --name: %q", c)
			}
			pg = m[1]
		}
	}
	if pg == "aim-smoke-postgres" || !strings.HasPrefix(pg, proj) {
		t.Errorf("throwaway Postgres %q should be named after the run's project %q", pg, proj)
	}
	removedPG, downOwn := false, false
	for _, c := range a.calls {
		if strings.HasPrefix(c, "rm ") && strings.Contains(c, " "+pg+"|") {
			removedPG = true
		}
		if strings.HasPrefix(c, "compose -p "+proj+" ") && strings.Contains(c, " down") && strings.Contains(c, " -v") {
			downOwn = true
		}
	}
	if !removedPG {
		t.Errorf("exit must remove the run's own Postgres %q; calls: %q", pg, a.calls)
	}
	if !downOwn {
		t.Errorf("exit must take down the run's own project with its volumes; calls: %q", a.calls)
	}

	if b := runSmoke(t, "smoke-backend.sh"); b.project(t) == proj {
		t.Errorf("two runs share compose project %q", proj)
	}
}

func TestSmokeBackendNeverRemovesAPostgresItDidNotStart(t *testing.T) {
	// docker run fails, as it does when SMOKE_PG_CONTAINER names a container
	// that already exists. That container is not the run's to remove.
	r := runSmoke(t, "smoke-backend.sh", "STUB_RUN_EXIT=125", "SMOKE_PG_CONTAINER=someone-elses-postgres")
	if r.exitCode != 2 {
		t.Fatalf("want exit 2 (docker run failed in the stub), got %d\n%s", r.exitCode, r.output)
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "rm ") {
			t.Errorf("removes a container this run did not start: %q", c)
		}
	}
}
