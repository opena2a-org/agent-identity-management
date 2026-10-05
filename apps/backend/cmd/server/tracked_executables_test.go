package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// trackedExecutableAllowlist names tracked files that may be compiled code or
// carry the executable bit without a `#!` line. It is empty: the source tree
// builds everything it ships. An entry needs the path and the reason it cannot
// be built from source.
var trackedExecutableAllowlist = map[string]string{}

// binaryMagic lists the leading bytes of compiled executables, objects and
// libraries.
var binaryMagic = []struct {
	format string
	prefix []byte
}{
	{"Mach-O", []byte{0xfe, 0xed, 0xfa, 0xce}},
	{"Mach-O", []byte{0xfe, 0xed, 0xfa, 0xcf}},
	{"Mach-O", []byte{0xce, 0xfa, 0xed, 0xfe}},
	{"Mach-O", []byte{0xcf, 0xfa, 0xed, 0xfe}},
	{"Mach-O universal or Java class", []byte{0xca, 0xfe, 0xba, 0xbe}},
	{"Mach-O universal", []byte{0xca, 0xfe, 0xba, 0xbf}},
	{"Mach-O universal", []byte{0xbe, 0xba, 0xfe, 0xca}},
	{"ELF", []byte{0x7f, 'E', 'L', 'F'}},
	{"PE/COFF", []byte{'M', 'Z'}},
	{"ar archive", []byte("!<arch>\n")},
	{"WebAssembly", []byte{0x00, 'a', 's', 'm'}},
}

// classifyTrackedFile returns why a tracked file counts as an executable or
// object file, or "" when it does not. mode is the git index mode and head the
// file's leading bytes.
func classifyTrackedFile(mode string, head []byte) string {
	for _, m := range binaryMagic {
		if bytes.HasPrefix(head, m.prefix) {
			return m.format + " binary"
		}
	}
	if mode == "100755" && !bytes.HasPrefix(head, []byte("#!")) {
		return "executable mode without a #! line"
	}
	return ""
}

// trackedExecutables lists every file in the index of the repository at top
// that classifyTrackedFile flags, as "path: reason", skipping allowlisted
// paths. Symlinks and submodules are not read.
func trackedExecutables(t *testing.T, top string) []string {
	t.Helper()
	out, err := gitIn(top, "ls-files", "--stage", "-z")
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var found []string
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			t.Fatalf("unexpected git ls-files record %q", rec)
		}
		mode := strings.Fields(meta)[0]
		if mode != "100644" && mode != "100755" {
			continue
		}
		if _, ok := trackedExecutableAllowlist[path]; ok {
			continue
		}
		head, err := readHead(filepath.Join(top, filepath.FromSlash(path)), 8)
		if os.IsNotExist(err) {
			continue // deleted in the working tree; the index entry goes with the next commit
		}
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if reason := classifyTrackedFile(mode, head); reason != "" {
			found = append(found, path+": "+reason)
		}
	}
	sort.Strings(found)
	return found
}

func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path comes from git ls-files of the repository under test
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	got, err := io.ReadFull(f, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return buf[:got], err
}

// TestTheRepositoryTracksNoCompiledExecutables is a repository hygiene
// census. Prebuilt server binaries were once committed next to the source;
// they were platform-specific, unused by the container build and carried the
// build host's file paths. This fails if any tracked file is a compiled
// executable, object or library, or has the executable bit without being a
// script.
func TestTheRepositoryTracksNoCompiledExecutables(t *testing.T) {
	top := repositoryTop(t)

	t.Run("census of the tracked tree", func(t *testing.T) {
		if found := trackedExecutables(t, top); len(found) > 0 {
			t.Errorf("tracked compiled or non-script executable files (remove with `git rm --cached`, "+
				"build them from source instead):\n  %s", strings.Join(found, "\n  "))
		}
	})

	t.Run("former binary paths are ignored", func(t *testing.T) {
		for _, p := range []string{
			"apps/backend/aim-server",
			"apps/backend/bin/aim-backend",
			"apps/backend/bin/server",
			"apps/backend/main",
			"apps/backend/server",
		} {
			if _, err := gitIn(top, "check-ignore", "--no-index", "--quiet", "--", p); err != nil {
				t.Errorf("no ignore rule covers %s; a local `go build -o` there would show in git status", p)
			}
		}
	})
}

// TestTrackedExecutableCensusFlagsAPlantedBinary proves the census reads the
// index and fires: a scratch repository with one planted file per format must
// report exactly those files.
func TestTrackedExecutableCensusFlagsAPlantedBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	top := t.TempDir()
	if _, err := gitIn(top, "init", "--quiet"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	type plant struct {
		path    string
		content []byte
		mode    os.FileMode
	}
	flagged := []plant{
		{"bin/macho", append([]byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01}, make([]byte, 64)...), 0o755},
		{"bin/macho-be", []byte{0xfe, 0xed, 0xfa, 0xcf, 0, 0, 0, 0}, 0o644},
		{"bin/fat", []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2}, 0o755},
		{"bin/elf", []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}, 0o755},
		{"bin/tool.exe", []byte("MZ\x90\x00\x03\x00\x00\x00"), 0o644},
		{"lib/libx.a", []byte("!<arch>\nfoo.o/"), 0o644},
		{"web/mod.wasm", []byte{0x00, 'a', 's', 'm', 1, 0, 0, 0}, 0o644},
		{"bin/data", []byte("plain text marked executable\n"), 0o755},
	}
	clean := []plant{
		{"scripts/run.sh", []byte("#!/usr/bin/env bash\necho ok\n"), 0o755},
		{"README.md", []byte("# readme\n"), 0o644},
		{"empty", nil, 0o644},
		{"img.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o644},
	}
	for _, p := range append(append([]plant{}, flagged...), clean...) {
		full := filepath.Join(top, filepath.FromSlash(p.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, p.content, p.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(full, p.mode); err != nil {
			t.Fatal(err)
		}
		if _, err := gitIn(top, "-c", "core.fileMode=true", "add", "--", p.path); err != nil {
			t.Fatalf("git add %s: %v", p.path, err)
		}
	}
	// Record the executable bit even where the filesystem does not.
	for _, p := range append(append([]plant{}, flagged...), clean...) {
		if p.mode&0o111 != 0 {
			if _, err := gitIn(top, "update-index", "--chmod=+x", "--", p.path); err != nil {
				t.Fatalf("git update-index %s: %v", p.path, err)
			}
		}
	}

	found := trackedExecutables(t, top)
	var gotPaths []string
	for _, f := range found {
		path, _, _ := strings.Cut(f, ": ")
		gotPaths = append(gotPaths, path)
	}
	sort.Strings(gotPaths)
	var wantPaths []string
	for _, p := range flagged {
		wantPaths = append(wantPaths, p.path)
	}
	sort.Strings(wantPaths)
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Errorf("census flagged %v, want %v", found, wantPaths)
	}
}

func repositoryTop(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	out, err := gitIn(filepath.Dir(thisFile), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Skip("not running from a git checkout")
	}
	return strings.TrimSpace(out)
}
