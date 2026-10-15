package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isPrintedPictograph reports whether r is one of the characters that
// sdk/python/tests/test_no_emoji_in_console_output.py keeps out of the Python
// SDK's console output: the same blocks and the same four code points.
func isPrintedPictograph(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF) ||
		(r >= 0x2B00 && r <= 0x2BFF) ||
		r == 0xFE0F || r == 0x2139 || r == 0x23F3 || r == 0x23F8
}

// printLinesWithPictographs reads every Go file under root that is not a test
// and returns, as path:line: text, each line that calls fmt. or log. and
// carries a pictographic mark, with the paths of the files it read.
func printLinesWithPictographs(root string) (marked, read []string, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		read = append(read, rel)
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "fmt.") && !strings.Contains(line, "log.") {
				continue
			}
			if strings.IndexFunc(line, isPrintedPictograph) != -1 {
				marked = append(marked, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	return marked, read, err
}

// The backend's print and log lines say what happened in words: no line of a
// non-test Go file under apps/backend that calls fmt. or log. carries a
// pictographic mark.
func TestBackendPrintAndLogLinesCarryNoPictographs(t *testing.T) {
	marked, read, err := printLinesWithPictographs(filepath.Join("..", ".."))
	require.NoError(t, err)
	for _, file := range []string{"cmd/server/main.go", "cmd/bootstrap/main.go", "internal/application/mcp_attestation_service.go"} {
		require.Contains(t, read, file, "the walk did not read %s", file)
	}
	assert.Empty(t, marked, "print and log lines of the backend that carry a pictographic mark:\n%s", strings.Join(marked, "\n"))
}

// The scan reports a planted mark on a print or log line, in a nested package,
// and leaves a test file and a line that neither prints nor logs alone.
func TestPrintLinesWithPictographs_ReportsAPlantedMark(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	write("internal/service/service.go", "package service\n\nfunc run() {\n\tlog.Printf(\"✅ done\")\n\tfmt.Println(\"⚠️  careful\")\n\tfmt.Println(\"done\")\n}\n")
	write("internal/service/service_test.go", "package service\n\nfunc show() { fmt.Println(\"✅ in a test\") }\n")
	write("internal/domain/names.go", "package domain\n\nconst Mark = \"✅\" // not printed here\n")

	marked, read, err := printLinesWithPictographs(root)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"internal/domain/names.go", "internal/service/service.go"}, read)
	assert.Equal(t, []string{
		"internal/service/service.go:4: log.Printf(\"✅ done\")",
		"internal/service/service.go:5: fmt.Println(\"⚠️  careful\")",
	}, marked)
}
