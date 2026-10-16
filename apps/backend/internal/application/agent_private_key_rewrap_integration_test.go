//go:build integration

package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two master key rotations against a real agents table. Every stored agent
// private key decrypts before the first rotation, after each rotation but
// before its sweep, part way through a sweep, and after each sweep; the sweep
// leaves no row under the older id, and a key that left the ring is no longer
// needed.
//
// The storage key ids 0xA1 to 0xA3 are used by no other test, so rows other
// tests store in the same database are not rewritten and do not change the
// counts asserted for these ids.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the
// AIM schema already applied (run migrations first).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestRewrapAgentPrivateKeys_TwoRotations ./internal/application/...
func TestRewrapAgentPrivateKeys_TwoRotations(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping master key rotation test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	orgID, userID := seedOrgAndUser(t, db, ctx, "key-rotation")

	const idOne, idTwo, idThree byte = 0xA1, 0xA2, 0xA3
	keyOne, keyTwo, keyThree := randomMasterKey(t), randomMasterKey(t), randomMasterKey(t)
	ring := func(current byte, keys map[byte]string) *crypto.KeyVault {
		t.Helper()
		kv, err := crypto.NewKeyVaultRing(current, keys)
		require.NoError(t, err)
		return kv
	}

	agents := make([]uuid.UUID, 0, 7)
	want := map[uuid.UUID]string{}
	t.Cleanup(func() {
		for _, id := range agents {
			_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, id)
		}
	})
	insertAgent := func(kv *crypto.KeyVault) {
		t.Helper()
		id := uuid.New()
		plaintext := "agent-private-key-" + id.String()[:8]
		encrypted, err := kv.EncryptPrivateKey(id, plaintext)
		require.NoError(t, err)
		agents = append(agents, id)
		want[id] = plaintext
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status, trust_score, created_by, created_at, updated_at, encrypted_private_key)
			 VALUES ($1, $2, $3, $4, 'ai_agent', 'verified', 0.5, $5, NOW(), NOW(), $6)`,
			id, orgID, "key-rotation-agent-"+id.String()[:8], "Key Rotation Agent", userID, encrypted)
		require.NoError(t, err)
	}
	stored := func(id uuid.UUID) string {
		t.Helper()
		var encrypted string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT encrypted_private_key FROM agents WHERE id = $1`, id).Scan(&encrypted))
		return encrypted
	}
	// decryptsEveryRow reads each agent's row as the request path does and
	// returns how many rows are under each storage key id.
	decryptsEveryRow := func(stage string, kv *crypto.KeyVault) map[byte]int {
		t.Helper()
		ids := map[byte]int{}
		for _, id := range agents {
			encrypted := stored(id)
			got, err := kv.DecryptPrivateKey(id, encrypted)
			require.NoError(t, err, "%s: agent %s decrypts", stage, id)
			assert.Equal(t, want[id], got, "%s: agent %s", stage, id)
			keyID, err := crypto.PrivateKeyStorageKeyID(encrypted)
			require.NoError(t, err)
			ids[keyID]++
		}
		return ids
	}

	// Before any rotation: one key, six agents.
	ringOne := ring(idOne, map[byte]string{idOne: keyOne})
	for i := 0; i < 6; i++ {
		insertAgent(ringOne)
	}
	assert.Equal(t, map[byte]int{idOne: 6}, decryptsEveryRow("before the first rotation", ringOne))
	keptUnderOne := stored(agents[0])

	// First rotation: key two becomes current, key one stays in the ring.
	ringTwo := ring(idTwo, map[byte]string{idOne: keyOne, idTwo: keyTwo})
	assert.Equal(t, map[byte]int{idOne: 6}, decryptsEveryRow("after the first rotation, before its sweep", ringTwo))

	// An agent registered after the rotation is stored under the current id.
	insertAgent(ringTwo)
	// A sweep that stopped part way: two rows already moved.
	for _, id := range agents[:2] {
		rewrapped, changed, err := ringTwo.RewrapPrivateKey(id, stored(id))
		require.NoError(t, err)
		require.True(t, changed)
		_, err = db.ExecContext(ctx, `UPDATE agents SET encrypted_private_key = $1 WHERE id = $2`, rewrapped, id)
		require.NoError(t, err)
	}
	assert.Equal(t, map[byte]int{idOne: 4, idTwo: 3}, decryptsEveryRow("part way through the first sweep", ringTwo))

	// An instance still on the configuration from before the rotation does
	// not hold id 2: the id is outside its ring and refused.
	_, err = ringOne.DecryptPrivateKey(agents[0], stored(agents[0]))
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unknown storage key id %d", idTwo))

	result, err := RewrapAgentPrivateKeys(ctx, db, ringTwo)
	require.NoError(t, err)
	assert.Equal(t, idTwo, result.StorageKeyID)
	assert.Equal(t, 4, result.Rewritten, "the sweep carries on with the rows still under id 1")
	assert.Equal(t, 0, result.Failures)
	assert.Equal(t, 0, result.Remaining[idOne])
	assert.Contains(t, result.Remaining, idOne, "the older id is reported at 0")
	assert.Equal(t, map[byte]int{idTwo: 7}, decryptsEveryRow("after the first sweep", ringTwo))

	// Running it again changes nothing.
	again, err := RewrapAgentPrivateKeys(ctx, db, ringTwo)
	require.NoError(t, err)
	assert.Equal(t, 0, again.Rewritten)
	assert.Equal(t, 0, again.Remaining[idOne])

	// Second rotation: the sweep read 0 under id 1, so key one leaves the ring.
	ringThree := ring(idThree, map[byte]string{idTwo: keyTwo, idThree: keyThree})
	assert.Equal(t, map[byte]int{idTwo: 7}, decryptsEveryRow("after the second rotation, before its sweep", ringThree))

	result, err = RewrapAgentPrivateKeys(ctx, db, ringThree)
	require.NoError(t, err)
	assert.Equal(t, idThree, result.StorageKeyID)
	assert.Equal(t, 7, result.Rewritten)
	assert.Equal(t, 0, result.Failures)
	assert.Equal(t, 0, result.Remaining[idTwo])
	assert.NotContains(t, result.Remaining, idOne, "no row is left under the id that left the ring")
	assert.Equal(t, map[byte]int{idThree: 7}, decryptsEveryRow("after the second sweep", ringThree))

	// After both rotations the newest key alone reads every row, and a
	// ciphertext still under the first id would be refused.
	ringNewest := ring(idThree, map[byte]string{idThree: keyThree})
	assert.Equal(t, map[byte]int{idThree: 7}, decryptsEveryRow("with the newest key alone", ringNewest))
	_, err = ringNewest.DecryptPrivateKey(agents[0], keptUnderOne)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unknown storage key id %d", idOne))

	skipped, err := RewrapAgentPrivateKeys(ctx, db, ringNewest)
	require.NoError(t, err)
	assert.True(t, skipped.Skipped)
}

func randomMasterKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}
