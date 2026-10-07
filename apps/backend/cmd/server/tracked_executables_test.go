package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// executableMagic lists the leading bytes of compiled executables, objects and
// libraries. PE images are matched by isPEImage instead: "MZ" alone also
// starts ordinary text.
var executableMagic = []struct {
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
	{"ar archive", []byte("!<arch>\n")},
	{"WebAssembly", []byte{0x00, 'a', 's', 'm'}},
}

// isPEImage reports whether content is a PE/COFF image: an MZ header whose
// e_lfanew field (little-endian, offset 0x3c) points at a "PE\0\0" signature.
func isPEImage(content []byte) bool {
	if len(content) < 0x40 || !bytes.HasPrefix(content, []byte("MZ")) {
		return false
	}
	off := uint64(binary.LittleEndian.Uint32(content[0x3c:0x40]))
	return off+4 <= uint64(len(content)) && bytes.Equal(content[off:off+4], []byte("PE\x00\x00"))
}

// classifyTrackedBlob returns why a tracked file counts as an executable or
// object file, or "" when it does not. mode is the tree entry's mode and
// content the committed blob.
func classifyTrackedBlob(mode string, content []byte) string {
	for _, m := range executableMagic {
		if bytes.HasPrefix(content, m.prefix) {
			return m.format + " binary"
		}
	}
	if isPEImage(content) {
		return "PE/COFF binary"
	}
	if mode == "100755" && !bytes.HasPrefix(content, []byte("#!")) {
		return "executable mode without a #! line"
	}
	return ""
}

type treeBlob struct {
	mode, oid, path string
}

// trackedExecutables lists every blob in the tree of commit rev, in the
// repository at top, that classifyTrackedBlob flags, as "path: reason". It
// reads the committed blobs, not the working tree, so a file deleted or
// rewritten locally is judged by what the commit holds. Every path is read;
// there is no exemption list. Submodule entries are commits, not blobs, and
// are not read.
func trackedExecutables(t *testing.T, top, rev string) []string {
	t.Helper()
	out, err := gitIn(top, "ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		t.Fatalf("git ls-tree %s: %v", rev, err)
	}
	var blobs []treeBlob
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		meta, path, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			t.Fatalf("unexpected git ls-tree record %q", rec)
		}
		if fields[1] != "blob" {
			continue
		}
		blobs = append(blobs, treeBlob{mode: fields[0], oid: fields[2], path: path})
	}
	if len(blobs) == 0 {
		t.Fatalf("git ls-tree %s listed no blobs; a census that reads nothing proves nothing", rev)
	}

	var found []string
	read := 0
	err = readBlobs(top, blobs, func(b treeBlob, content []byte) {
		read++
		if reason := classifyTrackedBlob(b.mode, content); reason != "" {
			found = append(found, b.path+": "+reason)
		}
	})
	if err != nil {
		t.Fatalf("read blobs of %s: %v", rev, err)
	}
	if read != len(blobs) {
		t.Fatalf("read %d of %d blobs listed at %s", read, len(blobs), rev)
	}
	sort.Strings(found)
	return found
}

// readBlobs streams the blobs through one `git cat-file --batch` and hands
// each one's content to fn, in order.
func readBlobs(top string, blobs []treeBlob, fn func(treeBlob, []byte)) error {
	var in strings.Builder
	for _, b := range blobs {
		in.WriteString(b.oid + "\n")
	}
	cmd := exec.Command("git", "-C", top, "cat-file", "--batch") //nolint:gosec // G204: fixed binary; top comes from git rev-parse or t.TempDir
	cmd.Stdin = strings.NewReader(in.String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r := bufio.NewReader(stdout)
	for _, b := range blobs {
		header, err := r.ReadString('\n')
		if err != nil {
			_ = cmd.Wait()
			return fmt.Errorf("%s: cat-file header: %w", b.path, err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[0] != b.oid || fields[1] != "blob" {
			_ = cmd.Wait()
			return fmt.Errorf("%s: unexpected cat-file header %q", b.path, strings.TrimSpace(header))
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			_ = cmd.Wait()
			return fmt.Errorf("%s: cat-file size %q: %w", b.path, fields[2], err)
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			_ = cmd.Wait()
			return fmt.Errorf("%s: cat-file content: %w", b.path, err)
		}
		if _, err := r.Discard(1); err != nil { // the newline after each object
			_ = cmd.Wait()
			return fmt.Errorf("%s: cat-file terminator: %w", b.path, err)
		}
		fn(b, content)
	}
	return cmd.Wait()
}

// TestTheRepositoryTracksNoCompiledExecutables is a repository hygiene
// census. Prebuilt server binaries were once committed next to the source;
// they were platform-specific, unused by the container build and carried the
// build host's file paths. This fails if any blob in the commit under test is
// a compiled executable, object or library, or has the executable bit without
// being a script.
func TestTheRepositoryTracksNoCompiledExecutables(t *testing.T) {
	top := repositoryTop(t)

	t.Run("census of the committed tree", func(t *testing.T) {
		if found := trackedExecutables(t, top, "HEAD"); len(found) > 0 {
			t.Errorf("tracked compiled or non-script executable files (remove with `git rm --cached`, "+
				"build them from source instead):\n  %s", strings.Join(found, "\n  "))
		}
	})

	t.Run("former binary paths are ignored", func(t *testing.T) {
		for _, p := range []string{
			"apps/backend/aim-server",
			"apps/backend/bin/aim-backend",
			"apps/backend/bin/server",
			"apps/backend/cmd/server/server",
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
// committed tree and fires: a scratch repository with one planted file per
// format must report exactly those files. The plants are written at run time;
// no binary is committed to this repository.
func TestTrackedExecutableCensusFlagsAPlantedBinary(t *testing.T) {
	requireGit(t)
	top := t.TempDir()
	noHooks := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		base := []string{
			"-c", "core.hooksPath=" + noHooks,
			"-c", "core.fileMode=true",
			"-c", "commit.gpgsign=false",
			"-c", "user.name=census",
			"-c", "user.email=census@example.invalid",
		}
		if _, err := gitIn(top, append(base, args...)...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	git("init", "--quiet")

	type plant struct {
		path    string
		content []byte
		mode    os.FileMode
	}
	flagged := []plant{
		{"bin/macho", append([]byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01}, make([]byte, 64)...), 0o755},
		{"bin/macho-be", []byte{0xfe, 0xed, 0xfa, 0xcf, 0, 0, 0, 0}, 0o644},
		{"bin/macho32", []byte{0xce, 0xfa, 0xed, 0xfe, 7, 0, 0, 0}, 0o644},
		{"bin/macho32-be", []byte{0xfe, 0xed, 0xfa, 0xce, 0, 0, 0, 7}, 0o644},
		{"bin/fat", []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2}, 0o755},
		{"bin/fat64", []byte{0xca, 0xfe, 0xba, 0xbf, 0, 0, 0, 2}, 0o644},
		{"bin/fat-le", []byte{0xbe, 0xba, 0xfe, 0xca, 2, 0, 0, 0}, 0o644},
		{"bin/elf", []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}, 0o755},
		{"bin/tool.exe", syntheticPEHeader(), 0o644},
		{"lib/libx.a", []byte("!<arch>\nfoo.o/"), 0o644},
		{"web/mod.wasm", []byte{0x00, 'a', 's', 'm', 1, 0, 0, 0}, 0o644},
		{"bin/data", []byte("plain text marked executable\n"), 0o755},
		// Deleted from the working tree after the commit below: the census
		// judges the commit, so it is still reported.
		{"bin/gone-elf", []byte{0x7f, 'E', 'L', 'F', 1, 1, 1, 0}, 0o644},
	}
	clean := []plant{
		{"scripts/run.sh", []byte("#!/usr/bin/env bash\necho ok\n"), 0o755},
		{"README.md", []byte("# readme\n"), 0o644},
		{"empty", nil, 0o644},
		{"img.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o644},
		// Starts with "MZ" but carries no PE header.
		{"docs/mz.txt", []byte("MZ is the two-byte signature of a DOS stub; this file only names it.\n"), 0o644},
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
		git("add", "--", p.path)
		// Record the executable bit even where the filesystem does not.
		if p.mode&0o111 != 0 {
			git("update-index", "--chmod=+x", "--", p.path)
		}
	}
	git("commit", "--quiet", "--no-verify", "-m", "plant")
	if err := os.Remove(filepath.Join(top, "bin", "gone-elf")); err != nil {
		t.Fatal(err)
	}

	found := trackedExecutables(t, top, "HEAD")
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

// syntheticPEHeader returns the smallest byte string isPEImage accepts: an MZ
// header whose e_lfanew points at a "PE\0\0" signature right after it.
func syntheticPEHeader() []byte {
	b := make([]byte, 0x40, 0x48)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 0x40)
	return append(b, 'P', 'E', 0, 0, 0x64, 0x86, 0, 0)
}

// requireGit fails the test when git is not on PATH. The census reads the
// repository through git; without it nothing would be read, and a census that
// read nothing must not report as passing.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is not on PATH: this repository hygiene check reads the repository through git and cannot run without it")
	}
}

func repositoryTop(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	requireGit(t)
	out, err := gitIn(filepath.Dir(thisFile), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("not running from a git checkout (%v): the tracked-executable census reads the committed tree and cannot run on an export", err)
	}
	return strings.TrimSpace(out)
}
