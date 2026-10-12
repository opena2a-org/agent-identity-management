// Package smoke holds no Go code; this file tests the shell harness in this
// directory.
package smoke

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const script = "rest-list-endpoints-smoke.sh"

// The smoke reaches its throwaway Postgres over the published port with psql
// and pg_isready. A docker exec line carrying a shell variable is what
// hackmyagent reports as "Docker exec with variable interpolation", and the
// container and database names come from the environment.
func TestRestListSmokeRunsNoDockerExec(t *testing.T) {
	src, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.Contains(line, "docker exec") {
			t.Errorf("%s:%d runs docker exec: %s", script, i+1, strings.TrimSpace(line))
		}
	}
}

// A container or database name outside the allowed characters stops the run
// before any command receives it: no docker call is made.
func TestRestListSmokeRefusesAnUnsafeName(t *testing.T) {
	for _, tc := range []struct{ name, value, message string }{
		{"SMOKE_PG_CONTAINER", "aim;touch smoke-pwned", "FAIL: SMOKE_PG_CONTAINER must match"},
		{"SMOKE_PG_CONTAINER", "aim listsmoke", "FAIL: SMOKE_PG_CONTAINER must match"},
		{"SMOKE_PG_DB", "identity$(id)", "FAIL: SMOKE_PG_DB must match"},
		{"SMOKE_PG_DB", "identity-db", "FAIL: SMOKE_PG_DB must match"},
	} {
		t.Run(tc.name+"="+tc.value, func(t *testing.T) {
			stubs := t.TempDir()
			calls := filepath.Join(stubs, "calls.log")
			for _, tool := range []string{"docker", "psql", "pg_isready", "go", "curl", "jq", "openssl"} {
				stub := "#!/usr/bin/env bash\necho \"" + tool + " $*\" >> \"$STUB_CALLS\"\n"
				if err := os.WriteFile(filepath.Join(stubs, tool), []byte(stub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			abs, err := filepath.Abs(script)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", abs)
			cmd.Env = []string{
				"PATH=" + stubs + ":/usr/bin:/bin",
				"HOME=" + t.TempDir(),
				"STUB_CALLS=" + calls,
				tc.name + "=" + tc.value,
			}
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("want exit 1, got %v; output:\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.message) {
				t.Errorf("output does not say %q:\n%s", tc.message, out)
			}
			if strings.Contains(string(out), tc.value) {
				t.Errorf("output repeats the refused value:\n%s", out)
			}
			if logged, err := os.ReadFile(calls); err == nil {
				t.Errorf("the run called a tool before refusing the name:\n%s", logged)
			}
		})
	}
}
