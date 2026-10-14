package main

// GET /health/ready may also report the agent-identity-management commit a
// hosted build was synced from, as the optional key "syncedFrom". A build that
// is not stamped with it, or is stamped with anything other than a full
// lowercase 40-hex sha, leaves the key out of the body.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncedFromTestCommit has the shape of a commit sha and names no real commit.
const syncedFromTestCommit = "fedcba9876543210fedcba9876543210fedcba98"

func TestHealthReadySyncedFromStamp(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("a build stamped with -X main.buildSyncedFrom reports that commit", func(t *testing.T) {
		body := stampProbeBody(t, stampBuildProbe(t, "-X main.buildSyncedFrom="+syncedFromTestCommit))
		aim07Validate(t, schema, body)
		syncedFrom, present := aim07TopLevel(t, body)["syncedFrom"]
		require.True(t, present, "a stamped build must carry the syncedFrom key: %s", body)
		assert.Equal(t, syncedFromTestCommit, syncedFrom)
	})

	t.Run("an unstamped or malformed stamp leaves the key out", func(t *testing.T) {
		prev := buildSyncedFrom
		t.Cleanup(func() { buildSyncedFrom = prev })
		for _, value := range []string{
			"",
			"dev",
			syncedFromTestCommit[:7],
			syncedFromTestCommit + "0",
			strings.ToUpper(syncedFromTestCommit),
			" " + syncedFromTestCommit,
		} {
			buildSyncedFrom = value
			_, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))
			aim07Validate(t, schema, body)
			_, present := aim07TopLevel(t, body)["syncedFrom"]
			assert.False(t, present, "stamp %q must leave syncedFrom out of the body: %s", value, body)
		}
	})
}
