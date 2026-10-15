package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a2aDemoAimURLOnly matches a CHANGELOG.md sentence saying one of the A2A
// multi-agent demo's programs reads `AIM_URL` and no stack port variable, and
// captures the program's language.
var a2aDemoAimURLOnly = regexp.MustCompile("The (Java|Python) demo still reads `AIM_URL` only")

// a2aDemoDirs names the directory under examples/a2a-multi-agent-demo that
// holds each language's program, and the extension of its source files.
var a2aDemoDirs = map[string]struct{ dir, ext string }{
	"Java":   {"java", ".java"},
	"Python": {"python", ".py"},
}

// a2aDemoAimURLOnlyFindings returns one line per CHANGELOG.md sentence that
// says a demo program reads `AIM_URL` only, for each program whose sources
// readsBackendPort reports as reading AIM_BACKEND_PORT.
func a2aDemoAimURLOnlyFindings(changelog string, readsBackendPort func(lang string) bool) []string {
	var findings []string
	for i, line := range strings.Split(changelog, "\n") {
		for _, m := range a2aDemoAimURLOnly.FindAllStringSubmatch(line, -1) {
			if readsBackendPort(m[1]) {
				findings = append(findings, fmt.Sprintf("CHANGELOG.md:%d: %s", i+1, m[0]))
			}
		}
	}
	return findings
}

// a2aDemoReadsBackendPort reports whether a source file of the language's
// A2A demo program names AIM_BACKEND_PORT.
func a2aDemoReadsBackendPort(t *testing.T, root, lang string) bool {
	t.Helper()

	demo, ok := a2aDemoDirs[lang]
	require.True(t, ok, "no A2A demo directory is known for %s", lang)

	found := false
	dir := filepath.Join(root, "examples", "a2a-multi-agent-demo", demo.dir)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found || d.IsDir() || filepath.Ext(path) != demo.ext {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = strings.Contains(string(raw), "AIM_BACKEND_PORT")
		return nil
	})
	require.NoError(t, err)
	return found
}

func TestA2ADemoAimURLOnlyFindings(t *testing.T) {
	sample := strings.Join([]string{
		"### Changed — the demo's Python scripts follow `AIM_BACKEND_PORT`",
		"",
		"- The scripts read the ports. The Java demo still reads `AIM_URL` only.",
		"- The Python demo still reads `AIM_URL` only.",
		"- The Java demo reads `AIM_URL` first.",
	}, "\n")

	readsPort := map[string]bool{"Java": true, "Python": false}
	assert.Equal(t, []string{
		"CHANGELOG.md:3: The Java demo still reads `AIM_URL` only",
	}, a2aDemoAimURLOnlyFindings(sample, func(lang string) bool { return readsPort[lang] }))
}

// A CHANGELOG.md sentence saying an A2A demo program reads `AIM_URL` only is
// false once that program's sources read AIM_BACKEND_PORT.
func TestChangelogA2ADemoAimURLOnlyMatchesTheDemoSources(t *testing.T) {
	root := aim03RepoRoot(t)
	findings := a2aDemoAimURLOnlyFindings(aim03ReadRepoFile(t, "CHANGELOG.md"), func(lang string) bool {
		return a2aDemoReadsBackendPort(t, root, lang)
	})
	assert.Empty(t, findings, "the demo program reads AIM_BACKEND_PORT; remove the sentence:\n%s", strings.Join(findings, "\n"))
}
