package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeTalksTo_AnEmptyColumnANullAndAnEmptyArrayHoldNoEntries(t *testing.T) {
	for _, column := range []string{"", "null", "[]"} {
		entries, err := DecodeTalksTo([]byte(column))
		require.NoError(t, err, "column %q", column)
		assert.Empty(t, entries, "column %q", column)
	}
}

func TestDecodeTalksTo_ReadsAnArrayOfStrings(t *testing.T) {
	entries, err := DecodeTalksTo([]byte(`["filesystem","github"]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"filesystem", "github"}, entries)
}

func TestDecodeTalksTo_ReadsEachObjectAsItsIDElseItsName(t *testing.T) {
	entries, err := DecodeTalksTo([]byte(
		`[{"id":"4b1f0c2e-8d3a-4e5f-9a6b-7c8d9e0f1a2b","name":"filesystem"},{"name":"github"},{"id":"","name":"slack"},{"url":"https://mcp.example.com"}]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"4b1f0c2e-8d3a-4e5f-9a6b-7c8d9e0f1a2b", "github", "slack"}, entries,
		"an object without a non-empty id or name stands for no entry")
}

func TestDecodeTalksTo_RefusesAColumnThatIsNotAnArray(t *testing.T) {
	_, err := DecodeTalksTo([]byte(`{"name":"filesystem"}`))
	assert.Error(t, err)
}
