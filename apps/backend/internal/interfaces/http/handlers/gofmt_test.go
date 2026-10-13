package handlers

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Every Go file of the package, tests included, is gofmt-formatted:
// format.Source, which formats as gofmt does, leaves each one unchanged.
func TestHandlerFilesAreGofmtFormatted(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's Go files: %v", err)
	}
	// The list holds this file, so a run from another directory fails here
	// instead of checking nothing.
	if !slices.Contains(names, "gofmt_test.go") {
		t.Fatalf("the package's Go files were not found from this directory: %v", names)
	}
	for _, name := range names {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		formatted, err := format.Source(src)
		if err != nil {
			t.Fatalf("format %s: %v", name, err)
		}
		if !bytes.Equal(src, formatted) {
			t.Errorf("%s is not gofmt-formatted: run gofmt -w on it", name)
		}
	}
}
