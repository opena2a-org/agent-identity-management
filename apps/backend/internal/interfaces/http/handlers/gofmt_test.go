package handlers

import (
	"bytes"
	"go/format"
	"os"
	"testing"
)

// The handler files named here are gofmt-formatted: format.Source, which
// formats as gofmt does, leaves each one unchanged. Other files of the package
// are not formatted yet; a file joins the list when it is.
func TestHandlerFilesAreGofmtFormatted(t *testing.T) {
	for _, name := range []string{
		"agent_handler.go",
		"query_text.go",
		"tag_handler.go",
	} {
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
