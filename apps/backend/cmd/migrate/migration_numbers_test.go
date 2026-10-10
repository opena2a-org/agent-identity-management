package main

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedMigrationNumbers lists the one number two released migrations share.
// Both runners key a migration by its full file name, so renaming either file
// would apply it again on every database that recorded it under this name.
var sharedMigrationNumbers = map[string][]string{
	"032": {"032_add_changed_by_to_trust_score_history.sql", "032_sync_trust_scores.sql"},
}

// Every migration file starts with a number no other file uses. Two files with
// one number both apply, but their order then rests on the rest of the name.
func TestMigrationNumbersAreUnique(t *testing.T) {
	migrations, err := readMigrationFiles(filepath.Join("..", "..", "migrations"))
	require.NoError(t, err)
	require.NotEmpty(t, migrations, "the scan must find the migrations directory")

	byNumber := make(map[string][]string)
	for _, m := range migrations {
		number, _, ok := strings.Cut(m.Filename, "_")
		require.True(t, ok && number != "" && strings.Trim(number, "0123456789") == "",
			"%s: a migration file name starts with its number and an underscore", m.Filename)
		byNumber[number] = append(byNumber[number], m.Filename)
	}
	require.Contains(t, byNumber, "032", "the scan must see the shared pair, so a pass is not a scan that matched nothing")

	for number, names := range byNumber {
		if len(names) == 1 {
			continue
		}
		sort.Strings(names)
		assert.Equal(t, sharedMigrationNumbers[number], names,
			"migration number %s is used by %d files; give the newer file the next free number", number, len(names))
	}
}
