package main

// GET /health/ready reports the commit the published image was built from.
//
// Four links carry that value, and each one breaks without an error: the
// publish workflow passes the commit as a build argument, the Dockerfile
// declares the argument and hands it to the linker as -X <symbol>=<value>, the
// symbol names a string variable of this package, and the handler reports that
// variable. The linker ignores -X for a symbol the package does not declare,
// so a wrong name builds cleanly and serves "commit": null.
//
// The cells below read the link flag out of the Dockerfile the workflow
// builds, build this package's sources with it, and read the handler's answer
// out of the built binary. A cell that cannot read a file it names fails; none
// of them skips.

import (
	"bytes"
	"context"
	"encoding/json"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Both paths are relative to this package directory.
	stampDockerfileRel = "../../../../infrastructure/docker/Dockerfile.backend"
	stampWorkflowRel   = "../../../../.github/workflows/docker-publish.yml"

	// stampWorkflowFileLine is how a build step of the workflow names the
	// Dockerfile above; stampWorkflowArgLine is the build argument it must pass.
	stampWorkflowFileLine = "file: infrastructure/docker/Dockerfile.backend"
	stampWorkflowArgLine  = "COMMIT=${{ github.sha }}"

	stampBuildArg = "COMMIT"

	// stampTestCommit has the shape of a commit sha and names no real commit.
	stampTestCommit = "89abcdef0123456789abcdef0123456789abcdef"

	// stampProbeFile is the name the probe source takes inside this package
	// for the probe build. No file of that name exists in the tree.
	stampProbeFile = "zz_health_ready_stamp_probe.go"
)

// stampProbeSource is compiled into the probe build next to this package's own
// sources. Its init answers GET /health/ready through the real handler, writes
// the body to the file named by the first argument and exits, so the server's
// main never runs: the probe opens no database and binds no port.
const stampProbeSource = `package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
)

func init() {
	fail := func(msg string) {
		os.Stderr.WriteString("health ready stamp probe: " + msg + "\n")
		os.Exit(3)
	}
	if len(os.Args) != 2 {
		fail("want exactly one argument, the file to write the body to")
	}
	app := fiber.New()
	app.Get("/health/ready", newHealthReadyHandler(func(context.Context) error { return nil }, nil))
	resp, err := app.Test(
		httptest.NewRequest(http.MethodGet, "/health/ready", nil),
		fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true},
	)
	if err != nil {
		fail(err.Error())
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(os.Args[1], body, 0o600); err != nil {
		fail(err.Error())
	}
	os.Exit(0)
}
`

var (
	stampShaPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	stampSymbolPattern  = regexp.MustCompile(`^main\.[A-Za-z_][A-Za-z0-9_]*$`)
	stampLdflagsPattern = regexp.MustCompile(`-ldflags(?:=|\s+)"([^"]*)"`)
)

type stampInstruction struct {
	line  int    // 1-based line the instruction starts on
	stage int    // 0 before the first FROM, then 1, 2, ...
	text  string // continuation lines joined by one space
}

// stampDockerfileInstructions splits a Dockerfile into instructions, joining
// continuation lines and dropping comment and blank lines.
func stampDockerfileInstructions(src string) []stampInstruction {
	var (
		out   []stampInstruction
		cur   []string
		start int
		stage int
	)
	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(cur) == 0 {
			start = i + 1
		}
		continued := strings.HasSuffix(line, `\`)
		cur = append(cur, strings.TrimSpace(strings.TrimSuffix(line, `\`)))
		if continued {
			continue
		}
		text := strings.Join(cur, " ")
		cur = nil
		if strings.HasPrefix(strings.ToUpper(text), "FROM ") {
			stage++
		}
		out = append(out, stampInstruction{line: start, stage: stage, text: text})
	}
	return out
}

// stampCommitSymbol reads the Dockerfile of the published image and returns
// the linker symbol its server build sets from the COMMIT build argument. It
// fails unless exactly one RUN builds ./cmd/server, that RUN carries exactly
// one -X operand whose whole value is ${COMMIT}, and the same build stage
// declares ARG COMMIT before it with no sha as its default.
func stampCommitSymbol(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(aim07Path(t, stampDockerfileRel))
	require.NoError(t, err, "the Dockerfile of the published image must be readable")
	instructions := stampDockerfileInstructions(string(b))

	serverBuild := -1
	for i, ins := range instructions {
		fields := strings.Fields(ins.text)
		if len(fields) == 0 || strings.ToUpper(fields[0]) != "RUN" {
			continue
		}
		for _, f := range fields[1:] {
			if f == "./cmd/server" {
				require.Equal(t, -1, serverBuild, "line %d: a second RUN builds ./cmd/server", ins.line)
				serverBuild = i
				break
			}
		}
	}
	require.NotEqual(t, -1, serverBuild, "no RUN instruction builds ./cmd/server")
	run := instructions[serverBuild]

	ldflags := stampLdflagsPattern.FindAllStringSubmatch(run.text, -1)
	require.Len(t, ldflags, 1, `line %d: the server build must carry exactly one -ldflags="..."`, run.line)

	wholeValue := map[string]bool{"${" + stampBuildArg + "}": true, "$" + stampBuildArg: true}
	var symbols []string
	fields := strings.Fields(ldflags[0][1])
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != "-X" {
			continue
		}
		symbol, value, ok := strings.Cut(fields[i+1], "=")
		if ok && wholeValue[value] {
			symbols = append(symbols, symbol)
		}
	}
	require.Len(t, symbols, 1,
		"line %d: the server build must carry exactly one -X <symbol>=${%s}; a build without it serves \"commit\": null",
		run.line, stampBuildArg)
	require.Regexp(t, stampSymbolPattern, symbols[0], "line %d: the stamped symbol must be a variable of package main", run.line)

	declared := 0
	for _, ins := range instructions[:serverBuild] {
		fields := strings.Fields(ins.text)
		if ins.stage != run.stage || len(fields) == 0 || strings.ToUpper(fields[0]) != "ARG" {
			continue
		}
		for _, f := range fields[1:] {
			name, def, _ := strings.Cut(f, "=")
			if name != stampBuildArg {
				continue
			}
			declared++
			assert.NotRegexp(t, stampShaPattern, strings.Trim(def, `"'`),
				"line %d: ARG %s must not default to a sha; a build given no commit has none to report", ins.line, stampBuildArg)
		}
	}
	require.Equal(t, 1, declared,
		"ARG %s must be declared exactly once in the build stage, before the server build on line %d", stampBuildArg, run.line)

	return symbols[0]
}

// stampBuildProbe builds every non-test source file of this package plus the
// probe, with the given linker flags, and returns the binary. The probe file
// exists only in the overlay and is named on the command line, so a build that
// does not see it fails instead of producing the real server.
func stampBuildProbe(t *testing.T, ldflags string) string {
	t.Helper()
	pkgDir := aim07Path(t, ".")
	pkg, err := build.ImportDir(pkgDir, 0)
	require.NoError(t, err)
	sources := append(append([]string{}, pkg.GoFiles...), pkg.CgoFiles...)
	require.Contains(t, sources, "health_ready.go")
	require.NotContains(t, sources, stampProbeFile, "the probe must exist only in the overlay")

	tmp := t.TempDir()
	probe := filepath.Join(tmp, "probe.go")
	require.NoError(t, os.WriteFile(probe, []byte(stampProbeSource), 0o600))
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(pkgDir, stampProbeFile): probe},
	})
	require.NoError(t, err)
	overlayPath := filepath.Join(tmp, "overlay.json")
	require.NoError(t, os.WriteFile(overlayPath, overlay, 0o600))

	bin := filepath.Join(tmp, "health-ready-stamp-probe")
	args := []string{"build", "-overlay", overlayPath}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", bin)
	args = append(args, sources...)
	args = append(args, stampProbeFile)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = pkgDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go %s\n%s", strings.Join(args, " "), out)
	return bin
}

// stampProbeBody runs a probe binary and returns the /health/ready body its
// handler answered with.
func stampProbeBody(t *testing.T, bin string) []byte {
	t.Helper()
	bodyPath := filepath.Join(t.TempDir(), "body.json")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, bodyPath)
	cmd.Dir = filepath.Dir(bin)
	cmd.Env = []string{} // the probe reads nothing from the environment
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	require.NoError(t, cmd.Run(), "the probe must exit 0: %s", output.String())

	body, err := os.ReadFile(bodyPath)
	require.NoError(t, err, "the probe must write the body")
	return body
}

// stampRequireNullCommit asserts the body carries the key "commit" with the
// JSON value null: not an absent key, an empty string, or a placeholder sha.
func stampRequireNullCommit(t *testing.T, body []byte) {
	t.Helper()
	commit, present := aim07TopLevel(t, body)["commit"]
	require.True(t, present, "the body must carry the commit key: %s", body)
	assert.Nil(t, commit, "an unknown commit must be reported as JSON null: %s", body)
	assert.Contains(t, string(body), `"commit":null`)
}

func TestHealthReadyCommitStamp(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("the publish workflow passes its commit sha to every build of the published backend image", func(t *testing.T) {
		b, err := os.ReadFile(aim07Path(t, stampWorkflowRel))
		require.NoError(t, err, "the publish workflow must be readable")
		lines := strings.Split(string(b), "\n")
		indent := func(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

		// One block per YAML sequence item: a build step and its `with:` map
		// fall in one block.
		var blocks [][]string
		named := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == stampWorkflowFileLine {
				named++
			}
			if strings.HasPrefix(trimmed, "- ") || len(blocks) == 0 {
				blocks = append(blocks, nil)
			}
			blocks[len(blocks)-1] = append(blocks[len(blocks)-1], line)
		}

		builds, pushes := 0, 0
		for _, block := range blocks {
			var buildsImage, usesBuildAction, pushesImage bool
			var commitArgs []string
			for i, line := range block {
				trimmed := strings.TrimSpace(line)
				switch {
				case trimmed == stampWorkflowFileLine:
					buildsImage = true
				case strings.HasPrefix(strings.TrimPrefix(trimmed, "- "), "uses: docker/build-push-action@"):
					usesBuildAction = true
				case trimmed == "push: true":
					pushesImage = true
				case trimmed == "build-args: |":
					for _, arg := range block[i+1:] {
						if strings.TrimSpace(arg) == "" {
							continue
						}
						if indent(arg) <= indent(line) {
							break
						}
						if a := strings.TrimSpace(arg); strings.HasPrefix(a, stampBuildArg+"=") {
							commitArgs = append(commitArgs, a)
						}
					}
				}
			}
			if !buildsImage {
				continue
			}
			builds++
			if pushesImage {
				pushes++
			}
			assert.True(t, usesBuildAction, "a step that names the backend Dockerfile must be a docker/build-push-action step:\n%s", strings.Join(block, "\n"))
			assert.Equal(t, []string{stampWorkflowArgLine}, commitArgs,
				"the build step must pass the commit the workflow runs on, once:\n%s", strings.Join(block, "\n"))
		}
		require.Positive(t, named, "the workflow must build %s", stampWorkflowFileLine)
		assert.Equal(t, named, builds, "every `%s` line must sit in a build step this cell read", stampWorkflowFileLine)
		assert.Positive(t, pushes, "the step that pushes the backend image must be among them")
	})

	t.Run("a build with the Dockerfile's link flag reports that commit", func(t *testing.T) {
		symbol := stampCommitSymbol(t)
		body := stampProbeBody(t, stampBuildProbe(t, "-X "+symbol+"="+stampTestCommit))
		aim07Validate(t, schema, body)
		assert.Equal(t, stampTestCommit, aim07TopLevel(t, body)["commit"],
			"the handler must report the value the linker set on %s: %s", symbol, body)
	})

	t.Run("a build that is given no commit reports null", func(t *testing.T) {
		symbol := stampCommitSymbol(t)
		for name, ldflags := range map[string]string{
			"no link flag": "",
			"the Dockerfile's flag with COMMIT unset": "-X " + symbol + "=",
		} {
			t.Run(name, func(t *testing.T) {
				body := stampProbeBody(t, stampBuildProbe(t, ldflags))
				aim07Validate(t, schema, body)
				stampRequireNullCommit(t, body)
			})
		}
	})

	t.Run("a value that is not a full commit sha is reported as null", func(t *testing.T) {
		prev := buildCommit
		t.Cleanup(func() { buildCommit = prev })
		for _, value := range []string{
			"",
			"unknown",
			"dev",
			"main",
			stampTestCommit[:7],
			stampTestCommit[:39],
			stampTestCommit + "0",
			strings.ToUpper(stampTestCommit),
			"g" + stampTestCommit[1:],
			stampTestCommit + "\n",
			" " + stampTestCommit,
		} {
			buildCommit = value
			_, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))
			aim07Validate(t, schema, body)
			stampRequireNullCommit(t, body)
		}
	})
}
