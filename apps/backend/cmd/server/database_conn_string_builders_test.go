package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// passwordSetting matches a format string that writes a password setting of a
// key=value connection string from an argument.
var passwordSetting = regexp.MustCompile(`password=%[a-z]`)

// The backend builds a key=value PostgreSQL connection string in one place,
// databaseConnString, which quotes and escapes every value. A second builder
// that writes password=%s with no quoting refuses a password with a single
// quote and drops a backslash from one; the unused internal database package
// that did was removed.
func TestOnlyDatabaseConnStringBuildsAKeyValueConnectionString(t *testing.T) {
	backend, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	var builders []string
	err = filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != backend && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules") {
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
		if passwordSetting.Match(src) {
			rel, err := filepath.Rel(backend, path)
			if err != nil {
				return err
			}
			builders = append(builders, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"cmd/server/main.go"}, builders,
		"build a key=value connection string with databaseConnString, or a postgres:// URL with config.DatabaseConfig.URL")
	main, err := os.ReadFile("main.go")
	require.NoError(t, err)
	assert.Len(t, passwordSetting.FindAll(main, -1), 1, "main.go writes the password setting once, in databaseConnString")
	assert.Contains(t, string(main), "connStringValue(database.Password)", "databaseConnString quotes the password")
}
