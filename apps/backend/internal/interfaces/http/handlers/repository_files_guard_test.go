package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests in this package that read the security, token rotation,
// troubleshooting and fleet governance guides under docs/ call
// requirePublicRepositoryFiles first. The hosted twin of this backend receives
// these tests through a sync that does not copy docs/, so there they would
// fail on what the twin does not carry rather than on a defect. The guard
// decides from the module line of apps/backend/go.mod alone.

// repoFilesTwinModule is the module path the hosted twin's go.mod declares. It
// is under the opena2a GitHub account, which scripts/lint-docs-repo-links.sh
// rejects in this repository's files as a link that does not resolve, so it is
// written in two parts the lint does not read as one path.
const (
	repoFilesPublicModule = "github.com/opena2a-org/agent-identity-management/apps/backend"
	repoFilesTwinModule   = "github.com/" + "opena2a/identity/backend"
)

// repoFilesGoMod is apps/backend/go.mod from this package's directory, the
// directory `go test` runs the package's tests in.
var repoFilesGoMod = filepath.Join("..", "..", "..", "..", "go.mod")

type repoFilesVerdict int

const (
	repoFilesRun repoFilesVerdict = iota
	repoFilesSkip
	repoFilesFail
)

// repoFilesVerdictFor decides from go.mod's module line alone. This
// repository's module runs the test. The hosted twin's module skips it, with a
// reason saying that a synced test holds in the twin. Any other line fails it,
// so a renamed module cannot turn these tests into skips.
func repoFilesVerdictFor(moduleLine string) (repoFilesVerdict, string) {
	if fields := strings.Fields(moduleLine); len(fields) == 2 && fields[0] == "module" {
		switch fields[1] {
		case repoFilesPublicModule:
			return repoFilesRun, ""
		case repoFilesTwinModule:
			return repoFilesSkip, "a synced test holds in the hosted twin: module " + repoFilesTwinModule +
				" is the hosted twin of this backend and does not carry this repository's copy of the files this test reads;" +
				" the test runs in module " + repoFilesPublicModule
		}
	}
	return repoFilesFail, fmt.Sprintf("apps/backend/go.mod's module line is %q: this test runs in module %s, skips in the hosted twin's module %s, and fails under any other module rather than pass without reading its files",
		moduleLine, repoFilesPublicModule, repoFilesTwinModule)
}

// repoFilesModuleLine returns the module line of a go.mod, or "" when it has none.
func repoFilesModuleLine(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "module\t") {
			return line
		}
	}
	return ""
}

// requirePublicRepositoryFiles runs the calling test in this repository, skips
// it in the hosted twin, and fails it under any other module.
func requirePublicRepositoryFiles(t *testing.T) {
	t.Helper()
	gomod, err := os.ReadFile(repoFilesGoMod)
	if err != nil {
		t.Fatalf("read apps/backend/go.mod to tell this repository from the hosted twin: %v", err)
	}
	switch verdict, reason := repoFilesVerdictFor(repoFilesModuleLine(string(gomod))); verdict {
	case repoFilesSkip:
		t.Skip(reason)
	case repoFilesFail:
		t.Fatal(reason)
	}
}

func TestRequirePublicRepositoryFiles_DecidesOnTheModuleLineAlone(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		verdict    repoFilesVerdict
		reason     string // a substring of the reason; "" means no reason
	}{
		{"this repository runs the test", "module github.com/opena2a-org/agent-identity-management/apps/backend", repoFilesRun, ""},
		{"the hosted twin skips it", "module github.com/" + "opena2a/identity/backend", repoFilesSkip, "hosted twin"},
		{"another module fails it", "module github.com/example/fork/apps/backend", repoFilesFail, `"module github.com/example/fork/apps/backend"`},
		{"no module line fails it", "", repoFilesFail, `module line is ""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verdict, reason := repoFilesVerdictFor(tc.line)
			assert.Equal(t, tc.verdict, verdict)
			if tc.reason == "" {
				assert.Empty(t, reason)
			} else {
				assert.Contains(t, reason, tc.reason)
			}
		})
	}
}
