package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// deploymentDocs are the pages that tell a reader how to install, deploy and
// operate AIM: which compose file to start, which script to run, which
// manifest to apply and which page to read next.
var deploymentDocs = []string{
	"docs/DEPLOYMENT.md",
	"docs/guides/DEPLOYMENT_CHECKLIST.md",
	"docs/guides/INSTALLATION.md",
	"docs/guides/QUICK_DEPLOYMENT_REFERENCE.md",
	"docs/guides/START_HERE.md",
	"docs/quick-start.md",
	"docs/troubleshooting/README.md",
	"infrastructure/README.md",
	"infrastructure/DEPLOYMENT.md",
	"infrastructure/deployment/README.md",
}

// deploymentDocCloneDir is the directory `git clone` creates; a `cd` into it
// returns to the repository root.
const deploymentDocCloneDir = "agent-identity-management"

var (
	deploymentDocFence      = regexp.MustCompile("^\\s*```\\s*([A-Za-z]*)")
	deploymentDocLink       = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	deploymentDocScheme     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	deploymentDocComment    = regexp.MustCompile(`(^|\s)#.*$`)
	deploymentDocCommands   = regexp.MustCompile(`&&|\|\||[;|]`)
	deploymentDocTokenSplit = regexp.MustCompile("[\\s`\"'\\[\\],;|]+")
	deploymentDocCreateSpan = regexp.MustCompile("(?i)\\bcreate\\s+`([^`]+)`")
	deploymentDocRootFile   = regexp.MustCompile(`^/[A-Za-z0-9_.-]+\.(?:md|ya?ml|sh|json)$`)
	deploymentDocCompose    = regexp.MustCompile(`^docker-compose[A-Za-z0-9_.-]*\.ya?ml$`)
	deploymentDocEnvAssign  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// deploymentDocRef is one file or directory a page names, resolved to a path
// relative to the repository root.
type deploymentDocRef struct {
	file string
	line int
	path string
	how  string
}

// deploymentDocScan walks one page. The working directory of a shell block
// starts at the repository root and follows its `cd` commands; a `cd` to a
// directory outside the repository stops the checks that depend on it until
// the next `cd` back in.
type deploymentDocScan struct {
	root     string
	file     string
	topLevel map[string]bool
	created  map[string]bool
	seen     map[string]bool
	refs     []deploymentDocRef
	line     int
	cwd      string
	cwdKnown bool
}

// add records a reference once per line, so a compose file passed with -f is
// not reported again as a mention on the same line. A file the page has the
// reader write first, and a .env file, is not a repository file.
func (s *deploymentDocScan) add(p, how string) {
	key := fmt.Sprintf("%d %s", s.line, p)
	if s.seen[key] || s.isCreated(p) || deploymentDocEnvironmentFile(p) {
		return
	}
	s.seen[key] = true
	s.refs = append(s.refs, deploymentDocRef{s.file, s.line, p, how})
}

// isCreated reports whether the page has the reader write p, or a directory
// that holds it, before this line.
func (s *deploymentDocScan) isCreated(p string) bool {
	for ; p != "." && p != "/" && p != ""; p = path.Dir(p) {
		if s.created[p] {
			return true
		}
	}
	return false
}

// exists reports whether the repository carries p.
func (s *deploymentDocScan) exists(p string) bool {
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(p)))
	return err == nil
}

// atRoot reports whether the block's working directory is the repository
// root, where a relative path is also a path from the root.
func (s *deploymentDocScan) atRoot() bool {
	return s.cwdKnown && s.cwd == "."
}

// placeholder reports whether a token stands for a value the reader chooses.
func deploymentDocPlaceholder(tok string) bool {
	return strings.ContainsAny(tok, "<>{}*$~") || strings.Contains(tok, "...")
}

// environmentFile reports whether p names a .env file, which the reader
// writes and the repository never carries.
func deploymentDocEnvironmentFile(p string) bool {
	base := path.Base(p)
	return base == ".env" || (strings.HasPrefix(base, ".env.") && base != ".env.example")
}

// resolve joins a relative operand to the block's working directory. It
// reports false when the working directory is outside the repository or the
// operand leaves it.
func (s *deploymentDocScan) resolve(operand string) (string, bool) {
	if !s.cwdKnown || strings.HasPrefix(operand, "/") || deploymentDocPlaceholder(operand) {
		return "", false
	}
	p := path.Clean(path.Join(s.cwd, operand))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// repoPath returns the repository path a token names when it is written from
// the repository root: it starts with one of the root's directories
// (infrastructure/k8s/ingress.yaml, ./scripts/x.sh), it is a single file
// anchored at the root (/AIM_DEPLOYMENT_TEST_REPORT.md), or it is a compose
// file named on its own (docker-compose.prod.yml).
func (s *deploymentDocScan) repoPath(tok string) (string, bool) {
	tok = strings.TrimRight(tok, ".:!?")
	if tok == "" || strings.Contains(tok, "://") || deploymentDocPlaceholder(tok) {
		return "", false
	}
	if deploymentDocRootFile.MatchString(tok) || deploymentDocCompose.MatchString(tok) {
		return strings.TrimPrefix(tok, "/"), true
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(tok, "./"), "/")
	first, _, found := strings.Cut(rel, "/")
	if !found || !s.topLevel[first] {
		return "", false
	}
	return path.Clean(rel), true
}

// mentions checks every repository path a line writes, in prose, in a code
// span or in a code block, including comments and file contents.
func (s *deploymentDocScan) mentions(text string) {
	// A Kubernetes apiVersion such as apps/v1 is not a path.
	if strings.HasPrefix(strings.TrimSpace(text), "apiVersion:") {
		return
	}
	for _, m := range deploymentDocCreateSpan.FindAllStringSubmatch(text, -1) {
		if p, ok := s.repoPath(m[1]); ok {
			s.created[p] = true
		}
	}
	for _, tok := range deploymentDocTokenSplit.Split(text, -1) {
		tok = strings.Trim(tok, "()")
		if strings.Contains(tok, "://") {
			continue
		}
		// A compose volume is SOURCE:TARGET; the source is a repository path.
		for _, part := range strings.Split(tok, ":") {
			if p, ok := s.repoPath(part); ok {
				s.add(p, "names")
			}
		}
	}
}

// links checks every relative Markdown link on a prose line.
func (s *deploymentDocScan) links(text string) {
	for _, m := range deploymentDocLink.FindAllStringSubmatch(text, -1) {
		target := m[1]
		if strings.HasPrefix(target, "#") || deploymentDocScheme.MatchString(target) {
			continue
		}
		target, _, _ = strings.Cut(target, "#")
		target, _, _ = strings.Cut(target, "?")
		var p string
		if strings.HasPrefix(target, "/") {
			p = path.Clean(strings.TrimPrefix(target, "/"))
		} else {
			p = path.Clean(path.Join(path.Dir(s.file), target))
		}
		s.add(p, "links to")
	}
}

// command checks the files and directories one shell command uses: the
// directory it changes to, the script it runs, the file it copies and the
// compose file or manifest it passes with -f.
func (s *deploymentDocScan) command(cmd string) {
	words := strings.Fields(cmd)
	for i, w := range words {
		words[i] = strings.Trim(w, `"'`)
	}
	for len(words) > 0 && (words[0] == "sudo" || deploymentDocEnvAssign.MatchString(words[0])) {
		words = words[1:]
	}
	if len(words) == 0 {
		return
	}
	s.recordCreated(words)

	switch words[0] {
	case "cd":
		if len(words) < 2 {
			return
		}
		target := words[1]
		switch {
		case target == deploymentDocCloneDir:
			s.cwd, s.cwdKnown = ".", true
			return
		case target == "-":
			s.cwdKnown = false
			return
		}
		p, ok := s.resolve(target)
		if !ok || s.isCreated(p) {
			s.cwdKnown = false
			return
		}
		// A block that runs the backend in one terminal and the dashboard in
		// another writes `cd apps/web` after `cd apps/backend`: each starts
		// from the root.
		if !s.exists(p) && !strings.HasPrefix(target, "..") && s.exists(path.Clean(target)) {
			p = path.Clean(target)
		}
		s.add(p, "changes to")
		s.cwd = p
		return
	case "cp":
		args := deploymentDocOperands(words[1:])
		if len(args) == 2 {
			if p, ok := s.resolve(args[0]); ok {
				s.add(p, "copies")
			}
		}
		return
	case "kubectl":
		if len(words) > 1 && map[string]bool{"apply": true, "create": true, "delete": true, "replace": true, "diff": true}[words[1]] {
			s.fileFlags(words[2:], false)
		}
		return
	case "docker":
		if len(words) > 1 && words[1] == "compose" {
			s.fileFlags(words[2:], true)
		}
		return
	case "docker-compose":
		s.fileFlags(words[1:], true)
		return
	}

	script := words[0]
	if map[string]bool{"bash": true, "sh": true, "source": true, ".": true}[script] && len(words) > 1 {
		script = words[1]
	}
	if !strings.Contains(script, "/") {
		return
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(script, "./"), "/")
	if !strings.HasSuffix(script, ".sh") && !s.topLevel[first] {
		return
	}
	if p, ok := s.resolve(script); ok {
		s.add(p, "runs")
	}
}

// fileFlags checks the -f, --file and --filename operands of a kubectl or
// docker compose command. For docker compose, -f is the compose file only
// before the subcommand; after it, `logs -f` means follow.
func (s *deploymentDocScan) fileFlags(args []string, beforeSubcommandOnly bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if beforeSubcommandOnly {
				return
			}
			continue
		}
		var operand string
		switch {
		case a == "-f" || a == "--file" || a == "--filename":
			if i+1 < len(args) {
				operand = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--file="), strings.HasPrefix(a, "--filename="), strings.HasPrefix(a, "-f="):
			_, operand, _ = strings.Cut(a, "=")
		default:
			continue
		}
		if operand == "" || operand == "-" || deploymentDocScheme.MatchString(operand) {
			continue
		}
		if p, ok := s.resolve(operand); ok {
			s.add(p, "passes -f")
		}
	}
}

// deploymentDocOperands drops a command's flags.
func deploymentDocOperands(words []string) []string {
	var out []string
	for _, w := range words {
		if !strings.HasPrefix(w, "-") {
			out = append(out, w)
		}
	}
	return out
}

// recordCreated notes the files a command writes, so a later step that reads
// one does not count as naming a repository file.
func (s *deploymentDocScan) recordCreated(words []string) {
	mark := func(operand string) {
		if p, ok := s.resolve(operand); ok {
			s.created[p] = true
		}
	}
	for i, w := range words {
		switch {
		case (w == ">" || w == ">>" || w == "-o" || w == "--output" || w == "tee") && i+1 < len(words):
			mark(words[i+1])
		case strings.HasPrefix(w, ">") && len(strings.TrimLeft(w, ">")) > 0:
			mark(strings.TrimLeft(w, ">"))
		}
	}
	switch words[0] {
	case "unzip":
		// unzip NAME.zip extracts into NAME.
		for _, a := range deploymentDocOperands(words[1:]) {
			if strings.HasSuffix(a, ".zip") {
				mark(strings.TrimSuffix(a, ".zip"))
			}
		}
	case "mkdir", "touch":
		for _, a := range deploymentDocOperands(words[1:]) {
			mark(a)
		}
	case "cp":
		if args := deploymentDocOperands(words[1:]); len(args) == 2 {
			mark(args[1])
		}
	}
}

// deploymentDocReferences returns every repository file and directory a page
// names: in a shell command, in a relative Markdown link, or as a path written
// from the repository root anywhere on the page.
func deploymentDocReferences(t *testing.T, root, file string) []deploymentDocRef {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	topLevel := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "node_modules" {
			topLevel[e.Name()] = true
		}
	}

	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file))) //nolint:gosec // G304: a fixed page list under the repository root
	require.NoError(t, err, "read %s", file)

	s := &deploymentDocScan{root: root, file: file, topLevel: topLevel, created: map[string]bool{}, seen: map[string]bool{}}
	inFence, shell := false, false
	var code, raw strings.Builder
	start := 0
	for i, line := range strings.Split(string(src), "\n") {
		if m := deploymentDocFence.FindStringSubmatch(line); m != nil {
			if inFence {
				inFence = false
				continue
			}
			inFence = true
			lang := strings.ToLower(m[1])
			shell = lang == "" || lang == "bash" || lang == "sh" || lang == "shell" || lang == "console" || lang == "zsh"
			s.cwd, s.cwdKnown = ".", true
			code.Reset()
			raw.Reset()
			continue
		}
		if !inFence || !shell {
			s.line = i + 1
			if !inFence {
				// A link target is relative to the page, not to the root.
				s.links(line)
				line = deploymentDocLink.ReplaceAllString(line, "]")
			}
			s.mentions(line)
			continue
		}

		// A shell command continues onto the next line after a trailing
		// backslash; it is checked as one command, at its first line.
		if code.Len() == 0 {
			start = i + 1
		}
		raw.WriteString(line + " ")
		stripped := strings.TrimSpace(deploymentDocComment.ReplaceAllString(line, ""))
		if strings.HasSuffix(stripped, `\`) {
			code.WriteString(strings.TrimSuffix(stripped, `\`) + " ")
			continue
		}
		code.WriteString(stripped)
		s.line = start
		startedAtRoot := s.atRoot()
		for _, cmd := range deploymentDocCommands.Split(code.String(), -1) {
			s.command(strings.TrimPrefix(strings.TrimSpace(cmd), "$ "))
		}
		// Away from the root, a relative path is relative to the directory the
		// block changed to, which the commands above already resolved.
		if startedAtRoot && s.atRoot() {
			s.mentions(raw.String())
		}
		code.Reset()
		raw.Reset()
	}
	return s.refs
}

// TestDeploymentDocsNameOnlyFilesInTheRepository reads the install, deploy and
// troubleshooting pages and fails on every file or directory they name that the
// repository does not carry. The pages told readers to run ./deploy.sh and
// ./scripts/migrate-to-postgres.sh, to start docker-compose.prod.yml and
// docker-compose.monitoring.yml, to apply infrastructure/k8s/*.yaml and to
// cd into infrastructure/azure, none of which the repository carries.
func TestDeploymentDocsNameOnlyFilesInTheRepository(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)
	total := 0
	for _, file := range deploymentDocs {
		for _, ref := range deploymentDocReferences(t, root, file) {
			total++
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ref.path))); err != nil {
				t.Errorf("%s:%d %s %s, which is not in the repository", ref.file, ref.line, ref.how, ref.path)
			}
		}
	}
	require.Greater(t, total, 40, "the pages name far more repository paths than the scan found; the scan is broken, not the pages")
	t.Logf("checked %d file and directory references in %d pages", total, len(deploymentDocs))
}

// TestDeploymentDocReferencesFollowTheBlocksWorkingDirectory pins the scan:
// each shape a page uses to name a file is found, resolved against the page
// or the directory the block has changed to, and a file the page has the
// reader write first, a .env file and a placeholder are not reported.
func TestDeploymentDocReferencesFollowTheBlocksWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"apps/backend", "apps/web", "docs/sdk", "infrastructure/scripts"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o750))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "sdk", "python.md"), nil, 0o600))
	page := strings.Join([]string{
		"See [the guide](../missing/README.md), [the SDK](./sdk/python.md), `infrastructure/k8s/` and `/REPORT.md`.",
		"- [ ] Create `/apps/web/app/settings/page.tsx`, then open `apps/web/app/settings/page.tsx`.",
		"```bash",
		"git clone https://github.com/opena2a-org/agent-identity-management.git",
		"cd agent-identity-management",
		"./deploy.sh production # comment naming infrastructure/k8s/hpa.yaml",
		"docker compose -f docker-compose.prod.yml up -d",
		"docker compose logs -f backend",
		"kubectl apply -f infrastructure/k8s/postgres.yaml",
		"kubectl logs -f deploy/backend",
		"cp .env.example .env && cp .env.backup .env",
		"kubectl apply -f <manifests>/backend.yaml",
		"docker compose \\",
		"  -f docker-compose.monitoring.yml up -d",
		"docker run -v $(pwd)/prometheus-aim.yml:/etc/prometheus/prometheus.yml prom/prometheus",
		"unzip aim-sdk-python.zip && cp -r aim-sdk-python/.aim ~/.aim/",
		"cd apps/backend && ./scripts/run.sh",
		"curl -sSL https://example.com/x.sh -o ./made.sh && bash ./made.sh",
		"cd apps/web",
		"mkdir -p my-agent && cd my-agent && ./scripts/ignored.sh",
		"```",
		"```bash",
		"cd infrastructure/azure",
		"./deploy.sh",
		"```",
		"```yaml",
		"# infrastructure/k8s/ingress.yaml",
		"apiVersion: apps/v1",
		"volumes:",
		"  - ./infrastructure/nginx/nginx.conf:/etc/nginx/nginx.conf",
		"```",
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "PAGE.md"), []byte(page), 0o600))

	var got []string
	for _, ref := range deploymentDocReferences(t, root, "docs/PAGE.md") {
		got = append(got, fmt.Sprintf("%d %s %s", ref.line, ref.how, ref.path))
	}
	require.Equal(t, []string{
		"1 links to missing/README.md",
		"1 links to docs/sdk/python.md",
		"1 names infrastructure/k8s",
		"1 names REPORT.md",
		"6 runs deploy.sh",
		"6 names infrastructure/k8s/hpa.yaml",
		"7 passes -f docker-compose.prod.yml",
		"9 passes -f infrastructure/k8s/postgres.yaml",
		"11 copies .env.example",
		"13 passes -f docker-compose.monitoring.yml",
		"17 changes to apps/backend",
		"17 runs apps/backend/scripts/run.sh",
		"19 changes to apps/web",
		"23 changes to infrastructure/azure",
		"24 runs infrastructure/azure/deploy.sh",
		"27 names infrastructure/k8s/ingress.yaml",
		"30 names infrastructure/nginx/nginx.conf",
	}, got)
}
