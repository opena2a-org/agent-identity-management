package main

import (
	"strings"
	"testing"
)

// `aim-server migrate` with a missing database variable exits 1 with one line
// naming the variable instead of a panic trace.
func TestMigrateCommand_MissingDatabaseVariableIsOneLine(t *testing.T) {
	logs := captureLog(t)
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_DB", "aim")

	var code int
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("migrate panicked: %v", r)
			}
		}()
		code = runMigrateCommand()
	}()

	if code != 1 {
		t.Errorf("exit code %d; want 1", code)
	}
	out := strings.TrimSpace(logs.String())
	if strings.Count(out, "\n") != 0 || !strings.Contains(out, "POSTGRES_USER") {
		t.Errorf("want one line naming POSTGRES_USER, got:\n%s", out)
	}
}
