package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// docsComposeService is the part of a compose service the documentation checks
// read.
type docsComposeService struct {
	ContainerName string    `yaml:"container_name"`
	Image         string    `yaml:"image"`
	Build         yaml.Node `yaml:"build"`
	Entrypoint    yaml.Node `yaml:"entrypoint"`
	Healthcheck   yaml.Node `yaml:"healthcheck"`
	Networks      yaml.Node `yaml:"networks"`
	Profiles      []string  `yaml:"profiles"`
}

type docsComposeFile struct {
	Services map[string]docsComposeService `yaml:"services"`
	Networks map[string]yaml.Node          `yaml:"networks"`
}

// composeProject is the project name docker compose gives the repository's
// stack in a clone, from the directory name; network names carry it.
const composeProject = "agent-identity-management"

var (
	// composeDefault matches ${VAR:-default} and ${VAR-default}.
	composeDefault = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*:?-([^}]*)\}`)
	// dockerCommand matches a docker or docker compose command where a shell
	// would start one: at the start of a line (after an optional "$ " prompt),
	// at the start of an inline code span, or after a pipe, &&, ; or $(.
	dockerCommand = regexp.MustCompile("(?:^\\s*(?:\\$\\s+)?|`|\\|\\s*|&&\\s*|;\\s*|\\$\\(\\s*)(docker(?:-compose|\\s+compose)?)\\s+([^`|;&)\\n]*)")
	// dockerPsGrep matches `docker ps ... | grep <pattern>`.
	dockerPsGrep = regexp.MustCompile("(?:^\\s*|`)docker\\s+ps\\b[^|\\n]*\\|\\s*grep\\s+(?:-\\S+\\s+)*[\"']?([A-Za-z0-9_.-]+)")
	// dockerRunName matches the container name `docker run --name` creates.
	dockerRunName = regexp.MustCompile(`docker\s+run\b.*--name[ =]([A-Za-z0-9_.-]+)`)
	yamlFence     = regexp.MustCompile("(?s)```ya?ml\n(.*?)```")
	plainFence    = regexp.MustCompile("(?s)```\n(.*?)```")
	// runtimeBinaries matches the runtime stage's copy of the built binaries
	// into its working directory.
	runtimeBinaries = regexp.MustCompile(`(?m)^COPY --from=builder ((?:/bin/aim-[a-z]+\s+)+)\./`)
	shellRedirect   = regexp.MustCompile(`^[0-9]*[<>]`)
)

// composeGlobalValueFlags are the docker compose options before the subcommand
// that take a value.
var composeGlobalValueFlags = map[string]bool{
	"-f": true, "--file": true, "-p": true, "--project-name": true, "--profile": true,
	"--env-file": true, "--project-directory": true, "--ansi": true, "--progress": true, "--parallel": true,
}

// composeServiceArgs lists the docker compose subcommands whose positional
// arguments are service names, with the options of each that take a value.
// The subcommands marked in composeFirstOnly take one service; what follows it
// is a command (exec, run) or a port.
var composeServiceArgs = map[string]map[string]bool{
	"up":      {"-t": true, "--timeout": true, "--scale": true, "--pull": true, "--exit-code-from": true, "--attach": true, "--no-attach": true, "--wait-timeout": true},
	"logs":    {"-n": true, "--tail": true, "--since": true, "--until": true, "--index": true},
	"restart": {"-t": true, "--timeout": true},
	"stop":    {"-t": true, "--timeout": true},
	"down":    {"-t": true, "--timeout": true, "--rmi": true},
	"start":   {},
	"ps":      {"--format": true, "--filter": true, "--status": true},
	"exec":    {"-e": true, "--env": true, "-u": true, "--user": true, "-w": true, "--workdir": true, "--index": true},
	"run":     {"-e": true, "--env": true, "-u": true, "--user": true, "-w": true, "--workdir": true, "--name": true, "-p": true, "--publish": true, "-v": true, "--volume": true, "--entrypoint": true, "-l": true, "--label": true, "--pull": true},
	"build":   {"--build-arg": true, "--builder": true, "--ssh": true, "--progress": true, "-m": true, "--memory": true},
	"pull":    {},
	"rm":      {},
	"kill":    {"-s": true, "--signal": true},
	"top":     {},
	"create":  {"--scale": true, "--pull": true},
	"pause":   {},
	"unpause": {},
	"port":    {"--index": true, "--protocol": true},
	"attach":  {"--index": true, "--detach-keys": true},
}

var composeFirstOnly = map[string]bool{"exec": true, "run": true, "port": true, "attach": true}

// dockerContainerArgs lists the docker subcommands whose positional arguments
// are container names, with the options of each that take a value.
var dockerContainerArgs = map[string]map[string]bool{
	"exec":    {"-e": true, "--env": true, "--env-file": true, "-u": true, "--user": true, "-w": true, "--workdir": true, "--detach-keys": true},
	"logs":    {"-n": true, "--tail": true, "--since": true, "--until": true},
	"stats":   {"--format": true},
	"update":  {"-m": true, "--memory": true, "--memory-swap": true, "--memory-reservation": true, "--cpus": true, "-c": true, "--cpu-shares": true, "--restart": true, "--pids-limit": true},
	"restart": {"-t": true, "--time": true, "-s": true, "--signal": true},
	"stop":    {"-t": true, "--time": true, "-s": true, "--signal": true},
	"start":   {"--detach-keys": true},
	"kill":    {"-s": true, "--signal": true},
	"rm":      {},
	"inspect": {"-f": true, "--format": true, "--type": true},
	"top":     {},
	"pause":   {},
	"unpause": {},
	"port":    {},
	"wait":    {},
}

var dockerFirstOnly = map[string]bool{"exec": true, "logs": true, "top": true, "port": true}

func resolveComposeDefaults(s string) string {
	return composeDefault.ReplaceAllString(s, "$1")
}

func parseDocsCompose(raw []byte) (docsComposeFile, error) {
	var f docsComposeFile
	err := yaml.Unmarshal(raw, &f)
	return f, err
}

// runsBackendImage reports whether a service runs the backend image with its
// own command, so a command given to `exec` or `run` runs inside that image.
func runsBackendImage(svc docsComposeService) bool {
	if svc.Entrypoint.Kind != 0 {
		return false
	}
	if strings.HasPrefix(svc.Image, "ghcr.io/opena2a-org/aim-server") {
		return true
	}
	if svc.Build.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(svc.Build.Content); i += 2 {
			if svc.Build.Content[i].Value == "dockerfile" && filepath.Base(svc.Build.Content[i+1].Value) == "Dockerfile.backend" {
				return true
			}
		}
	}
	return false
}

// shellWords splits a command into the words a shell would give it: quotes
// group, and a comment, a redirect or a line continuation ends the command.
func shellWords(s string) []string {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	for i, w := range words {
		placeholder := strings.HasPrefix(w, "<") && strings.HasSuffix(w, ">")
		if w == "\\" || strings.HasPrefix(w, "#") || (!placeholder && shellRedirect.MatchString(w)) {
			return words[:i]
		}
	}
	return words
}

// isPlaceholder reports whether a word stands for a value the reader supplies.
func isPlaceholder(w string) bool {
	return strings.HasPrefix(w, "<") || strings.HasPrefix(w, "$") || strings.Contains(w, "{{")
}

// positionalArgs returns the arguments that are not options, skipping the
// value of each option in valueFlags. With firstOnly it stops at the first
// positional argument and returns the words after it as rest.
func positionalArgs(args []string, valueFlags map[string]bool, firstOnly bool) (pos []string, rest []string, flags map[string]bool) {
	flags = map[string]bool{}
	for i := 0; i < len(args); i++ {
		w := args[i]
		if strings.HasPrefix(w, "-") && len(w) > 1 {
			name, _, hasValue := strings.Cut(w, "=")
			flags[name] = true
			if valueFlags[name] && !hasValue {
				i++
			}
			continue
		}
		pos = append(pos, w)
		if firstOnly {
			return pos, args[i+1:], flags
		}
	}
	return pos, nil, flags
}

// backendImageCommands returns the paths of the programs the backend image
// ships, and fails when its runtime stage carries a Go toolchain, which the
// documentation's rule against `go` inside the backend container assumes it
// does not.
func backendImageCommands(t *testing.T) map[string]bool {
	t.Helper()
	dockerfile := aim03ReadRepoFile(t, "infrastructure/docker/Dockerfile.backend")
	lastFrom := strings.LastIndex(dockerfile, "\nFROM ")
	require.NotEqual(t, -1, lastFrom, "Dockerfile.backend has no runtime stage")
	runtime := dockerfile[lastFrom:]
	fromLine, _, _ := strings.Cut(strings.TrimPrefix(runtime, "\n"), "\n")
	require.NotContains(t, fromLine, "golang", "the backend's runtime stage is a Go image; the check that documents run no `go` in the backend container no longer holds")
	require.Contains(t, runtime, "\nWORKDIR /app\n", "the backend's runtime stage no longer works in /app")
	m := runtimeBinaries.FindStringSubmatch(runtime)
	require.NotNil(t, m, "the backend's runtime stage copies no aim-* binaries into /app; the Dockerfile scan is broken, not the documents")
	commands := map[string]bool{}
	for _, bin := range strings.Fields(m[1]) {
		name := filepath.Base(bin)
		commands["/app/"+name] = true
		commands["./"+name] = true
	}
	return commands
}

// checkBackendCommand returns why a command run inside the backend image does
// not run there, or "" when it may.
func checkBackendCommand(word string, shipped map[string]bool) string {
	if word == "go" {
		return "runs `go`, and the backend image has no Go toolchain"
	}
	if (strings.HasPrefix(word, "/app/") || strings.HasPrefix(word, "./")) && !shipped[word] {
		names := make([]string, 0, len(shipped))
		for p := range shipped {
			if strings.HasPrefix(p, "/app/") {
				names = append(names, p)
			}
		}
		sort.Strings(names)
		return fmt.Sprintf("runs %s, which the backend image does not ship (it ships %s)", word, strings.Join(names, ", "))
	}
	return ""
}

// TestDocumentedDockerCommandsUseNamesComposeDefines reads every docker and
// docker compose command in the repository's markdown and checks the names it
// gives against the compose files. A `docker compose` command must name a
// service of the compose file it addresses (`backend`, not the container name
// `aim-backend`); a `docker` command must name a container a compose file
// creates (`aim-postgres`, not `identity-postgres`); a network the guides
// inspect must be one the stack creates; and a command run in the backend
// container must be a program its image ships, never `go`. The guides named
// containers and services no compose file defines and ran `go run` in an image
// with no Go toolchain. The changelog is skipped: it quotes the old commands.
func TestDocumentedDockerCommandsUseNamesComposeDefines(t *testing.T) {
	root := aim03RepoRoot(t)
	shipped := backendImageCommands(t)

	composeFiles := map[string]docsComposeFile{}
	var docs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && docsWalkSkip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		switch {
		case strings.HasPrefix(name, "docker-compose") && strings.HasSuffix(name, ".yml"):
			raw, err := os.ReadFile(p) //nolint:gosec // G304: walking the repository's own compose files
			if err != nil {
				return err
			}
			f, err := parseDocsCompose(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			composeFiles[p] = f
		case strings.HasSuffix(name, ".md") && !(p == filepath.Join(root, "CHANGELOG.md")):
			docs = append(docs, p)
		}
		return nil
	})
	require.NoError(t, err)

	rootComposePath := filepath.Join(root, "docker-compose.yml")
	rootCompose, ok := composeFiles[rootComposePath]
	require.True(t, ok, "no docker-compose.yml at the repository root")
	require.Contains(t, rootCompose.Services, "backend", "docker-compose.yml has no backend service; the compose scan is broken, not the documents")

	containers := map[string]bool{}
	backendContainers := map[string]bool{}
	for _, f := range composeFiles {
		for _, svc := range f.Services {
			name := resolveComposeDefaults(svc.ContainerName)
			if name == "" {
				continue
			}
			containers[name] = true
			if runsBackendImage(svc) {
				backendContainers[name] = true
			}
		}
	}
	require.True(t, backendContainers["aim-backend"], "no compose file runs the backend image as aim-backend; the compose scan is broken, not the documents")

	networks := map[string]bool{}
	for name := range rootCompose.Networks {
		networks[composeProject+"_"+name] = true
	}
	for _, svc := range rootCompose.Services {
		if svc.Networks.Kind == 0 {
			networks[composeProject+"_default"] = true
		}
	}

	var findings []string
	composeChecked, dockerChecked := 0, 0
	for _, p := range docs {
		src, err := os.ReadFile(p) //nolint:gosec // G304: walking the repository's own documents
		require.NoError(t, err)
		doc := string(src)
		rel, _ := filepath.Rel(root, p)
		dir := filepath.Dir(p)

		// A document that writes its own compose file runs its compose
		// commands against that file, and docker against its containers.
		var inline map[string]docsComposeService
		known := map[string]bool{}
		for _, m := range yamlFence.FindAllStringSubmatch(doc, -1) {
			f, err := parseDocsCompose([]byte(m[1]))
			if err != nil || len(f.Services) == 0 {
				continue
			}
			if inline == nil {
				inline = map[string]docsComposeService{}
			}
			for name, svc := range f.Services {
				inline[name] = svc
				if svc.ContainerName != "" {
					known[resolveComposeDefaults(svc.ContainerName)] = true
				}
			}
		}
		for _, m := range dockerRunName.FindAllStringSubmatch(doc, -1) {
			known[m[1]] = true
		}
		isContainer := func(name string) bool { return containers[name] || known[name] }

		for i, line := range strings.Split(doc, "\n") {
			at := fmt.Sprintf("%s:%d", rel, i+1)
			for _, m := range dockerCommand.FindAllStringSubmatch(line, -1) {
				words := shellWords(m[2])
				if len(words) == 0 {
					continue
				}
				if m[1] == "docker" {
					sub, args := words[0], words[1:]
					if sub == "network" && len(args) > 1 && args[0] == "inspect" {
						pos, _, _ := positionalArgs(args[1:], map[string]bool{"-f": true, "--format": true}, false)
						for _, n := range pos {
							if strings.HasPrefix(n, composeProject+"_") {
								dockerChecked++
								if !networks[n] {
									findings = append(findings, fmt.Sprintf("%s: `docker network inspect %s`: the stack creates no network %s", at, n, n))
								}
							}
						}
						continue
					}
					valueFlags, ok := dockerContainerArgs[sub]
					if !ok {
						continue
					}
					pos, rest, _ := positionalArgs(args, valueFlags, dockerFirstOnly[sub])
					for _, c := range pos {
						if isPlaceholder(c) {
							continue
						}
						dockerChecked++
						if !isContainer(c) {
							findings = append(findings, fmt.Sprintf("%s: `docker %s %s`: no compose file creates a container named %s", at, sub, c, c))
						}
					}
					if sub == "exec" && len(pos) == 1 && backendContainers[pos[0]] && len(rest) > 0 {
						if why := checkBackendCommand(rest[0], shipped); why != "" {
							findings = append(findings, fmt.Sprintf("%s: `docker exec %s %s` %s", at, pos[0], rest[0], why))
						}
					}
					continue
				}

				// docker compose: global options, then the subcommand.
				file := ""
				j := 0
				for j < len(words) && strings.HasPrefix(words[j], "-") {
					flag, value, hasValue := strings.Cut(words[j], "=")
					if composeGlobalValueFlags[flag] && !hasValue && j+1 < len(words) {
						j++
						value = words[j]
					}
					if flag == "-f" || flag == "--file" {
						file = value
					}
					j++
				}
				if j >= len(words) {
					continue
				}
				sub, args := words[j], words[j+1:]
				valueFlags, ok := composeServiceArgs[sub]
				if !ok {
					continue
				}

				var services map[string]docsComposeService
				source := ""
				switch {
				case file != "":
					for _, candidate := range []string{filepath.Join(dir, file), filepath.Join(root, file)} {
						if f, ok := composeFiles[filepath.Clean(candidate)]; ok {
							services, source = f.Services, filepath.Base(candidate)
							break
						}
					}
				case inline != nil:
					services, source = inline, "the compose file this document writes"
				default:
					local := filepath.Join(dir, "docker-compose.yml")
					if f, ok := composeFiles[local]; ok {
						services, source = f.Services, filepath.Join(filepath.Base(dir), "docker-compose.yml")
					} else {
						services, source = rootCompose.Services, "docker-compose.yml"
					}
				}
				if services == nil {
					// The command addresses a compose file this repository
					// does not carry (one the reader writes).
					continue
				}

				pos, rest, flags := positionalArgs(args, valueFlags, composeFirstOnly[sub])
				for _, s := range pos {
					if isPlaceholder(s) {
						continue
					}
					composeChecked++
					if _, ok := services[s]; !ok {
						names := make([]string, 0, len(services))
						for n := range services {
							names = append(names, n)
						}
						sort.Strings(names)
						findings = append(findings, fmt.Sprintf("%s: `docker compose %s %s`: %s defines no service %s (its services: %s)", at, sub, s, source, s, strings.Join(names, ", ")))
					}
				}
				if (sub == "exec" || sub == "run") && len(pos) == 1 && len(rest) > 0 && !flags["--entrypoint"] {
					if svc, ok := services[pos[0]]; ok && runsBackendImage(svc) {
						if why := checkBackendCommand(rest[0], shipped); why != "" {
							findings = append(findings, fmt.Sprintf("%s: `docker compose %s %s %s` %s", at, sub, pos[0], rest[0], why))
						}
					}
				}
			}

			if m := dockerPsGrep.FindStringSubmatch(line); m != nil {
				dockerChecked++
				matched := false
				for name := range containers {
					matched = matched || strings.Contains(name, m[1])
				}
				for name := range known {
					matched = matched || strings.Contains(name, m[1])
				}
				if !matched {
					findings = append(findings, fmt.Sprintf("%s: `docker ps | grep %s`: no compose file creates a container whose name contains %s", at, m[1], m[1]))
				}
			}
		}
	}

	t.Logf("checked %d service names in docker compose commands and %d container and network names in docker commands", composeChecked, dockerChecked)
	require.NotZero(t, composeChecked, "no service name found in a documented docker compose command; the document scan is broken, not the documents")
	require.NotZero(t, dockerChecked, "no container name found in a documented docker command; the document scan is broken, not the documents")
	assert.Empty(t, findings, "each documented docker command must name what the compose files define:\n%s", strings.Join(findings, "\n"))
}

// TestInstallationGuideComposePsOutputIsTheComposeFile pins the sample
// `docker compose ps` output in the installation guide to docker-compose.yml:
// one row for each service `docker compose up -d` starts, under the container
// name the file gives it, marked healthy when the service has a healthcheck.
func TestInstallationGuideComposePsOutputIsTheComposeFile(t *testing.T) {
	guide := aim03ReadRepoFile(t, "docs/guides/INSTALLATION.md")
	start := strings.Index(guide, "### 3. Start Services")
	require.NotEqual(t, -1, start, "the installation guide has no Start Services step")
	step := guide[start:]
	if end := strings.Index(step[1:], "\n### "); end != -1 {
		step = step[:end+1]
	}
	assert.Contains(t, step, "docker compose ps --format 'table {{.Name}}\\t{{.Service}}\\t{{.Status}}'\n",
		"the step prints the container name, the service name and the status")

	at := strings.Index(step, "Expected output")
	require.NotEqual(t, -1, at, "the step shows no expected output")
	block := plainFence.FindStringSubmatch(step[at:])
	require.NotNil(t, block, "the step's expected output is not in a code block")
	rows := strings.Split(strings.TrimSpace(block[1]), "\n")
	require.Equal(t, []string{"NAME", "SERVICE", "STATUS"}, strings.Fields(rows[0]), "the header docker compose prints for that format")

	compose, err := parseDocsCompose([]byte(aim03ReadRepoFile(t, "docker-compose.yml")))
	require.NoError(t, err)
	want := map[string]string{}
	for name, svc := range compose.Services {
		if len(svc.Profiles) > 0 {
			continue // `docker compose up -d` does not start it
		}
		want[name] = svc.ContainerName
	}
	got := map[string]string{}
	for _, row := range rows[1:] {
		fields := strings.Fields(row)
		require.GreaterOrEqual(t, len(fields), 3, "row %q has no NAME, SERVICE and STATUS", row)
		got[fields[1]] = fields[0]
		if svc, ok := compose.Services[fields[1]]; ok {
			assert.Equal(t, svc.Healthcheck.Kind != 0, strings.HasSuffix(row, "(healthy)"),
				"row %q is marked healthy exactly when the service has a healthcheck", row)
		}
	}
	assert.Equal(t, want, got, "the sample lists each service docker compose up -d starts, under its container name")
}
