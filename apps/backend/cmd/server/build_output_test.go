package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTheLocallyBuiltServerBinaryIsNeitherTrackedNorStageable is a repository
// hygiene tripwire.
//
// `go build` run in this directory writes the executable to ./server. A copy
// built on a developer machine was once committed: a platform-specific binary
// that the container image never used (Dockerfile.backend builds from source)
// and that carried the build host's file paths. These cells fail if a copy is
// tracked again or if the ignore rule that keeps a fresh build out of
// `git status` goes away.
func TestTheLocallyBuiltServerBinaryIsNeitherTrackedNorStageable(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	if out, err := gitIn(dir, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(out) != "true" {
		t.Skip("not running from a git checkout")
	}

	t.Run("not tracked", func(t *testing.T) {
		out, err := gitIn(dir, "ls-files", "--full-name", "--", "server")
		if err != nil {
			t.Fatalf("git ls-files: %v", err)
		}
		if tracked := strings.TrimSpace(out); tracked != "" {
			t.Errorf("a compiled server binary is tracked at %s; remove it with `git rm --cached`", tracked)
		}
	})

	t.Run("ignored", func(t *testing.T) {
		// --no-index evaluates the ignore rules even for a path that is tracked.
		if _, err := gitIn(dir, "check-ignore", "--no-index", "--quiet", "--", "server"); err != nil {
			t.Errorf("no ignore rule covers %s; a local `go build` would leave it untracked in git status",
				filepath.Join("apps", "backend", "cmd", "server", "server"))
		}
	})
}

func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec // G204: fixed binary; dir comes from runtime.Caller and args are literals
	out, err := cmd.Output()
	return string(out), err
}
