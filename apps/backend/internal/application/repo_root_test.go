package application

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRoot resolves the repository root from this file's own location, so a
// scan does not depend on the working directory `go test` was invoked from.
// The marker is apps/backend/go.mod beneath the candidate directory: it exists
// in every tree that carries this backend, including deployments that have no
// root package.json.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file's path")

	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "apps", "backend", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repository root above %s", thisFile)
	return ""
}
