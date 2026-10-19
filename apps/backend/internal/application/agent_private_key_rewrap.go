package application

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// agentPrivateKeyRewrapBatchSize is how many agent rows the sweep reads at a
// time. Each batch is read in full before any of its rows is written, so the
// sweep needs one database connection.
var agentPrivateKeyRewrapBatchSize = 500

// AgentPrivateKeyRewrapResult is the record of one sweep. It holds counts
// only: no agent ID and no key material.
type AgentPrivateKeyRewrapResult struct {
	StorageKeyID   byte // the storage-key id rows are re-encrypted to
	Skipped        bool // the key ring holds only the current key; no row was read
	AlreadyCurrent int  // rows already under the current id when read
	Rewritten      int  // rows this run moved to the current id
	Failures       int  // rows under an older id in the ring that were not moved: not readable, changed since read, or not written
	NotV2          int  // rows with no storage-key id to read: in neither format v2 nor format v3, or not decodable

	// Remaining counts, by storage-key id, the rows this run left under an id
	// other than the current one. Every other id in the key ring is listed,
	// with 0 when no row is left under it, and so is each id outside the ring
	// that a row was found under.
	Remaining map[byte]int
}

// RewrapAgentPrivateKeys moves each stored agent private key that is under an
// older storage-key id in the vault's key ring to the current id, at startup,
// and reports by id how many rows are still under other ids. A key in format
// v3 keeps its data key and ciphertext, and its data key is sealed again under
// the current master key; a key in format v2 is sealed again in format v3. When
// the ring holds only the current key it reads nothing.
//
// It is resumable: each row is rewritten on its own, with a compare-and-swap
// on the ciphertext it read, so a run that stops part way leaves every row
// readable under the ring, and the next start carries on with the rows still
// under older ids. A row under an id outside the ring is left as it is and
// counted. An older key can leave the ring once a run reports 0 under its id
// and every instance runs with the current key as KEYVAULT_MASTER_KEY.
func RewrapAgentPrivateKeys(ctx context.Context, db *sql.DB, kv *crypto.KeyVault) (AgentPrivateKeyRewrapResult, error) {
	current := kv.StorageKeyID()
	result := AgentPrivateKeyRewrapResult{StorageKeyID: current, Remaining: map[byte]int{}}

	ringIDs := kv.StorageKeyIDs()
	if len(ringIDs) == 1 {
		result.Skipped = true
		return result, nil
	}
	for _, id := range ringIDs {
		if id != current {
			result.Remaining[id] = 0
		}
	}

	type storedKey struct {
		id        uuid.UUID
		encrypted string
	}

	var after *uuid.UUID
	for {
		var (
			rows *sql.Rows
			err  error
		)
		if after == nil {
			rows, err = db.QueryContext(ctx,
				`SELECT id, encrypted_private_key FROM agents
				 WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> ''
				 ORDER BY id LIMIT $1`,
				agentPrivateKeyRewrapBatchSize)
		} else {
			rows, err = db.QueryContext(ctx,
				`SELECT id, encrypted_private_key FROM agents
				 WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> '' AND id > $1
				 ORDER BY id LIMIT $2`,
				*after, agentPrivateKeyRewrapBatchSize)
		}
		if err != nil {
			return result, fmt.Errorf("failed to list stored private keys: %w", err)
		}

		batch := make([]storedKey, 0, agentPrivateKeyRewrapBatchSize)
		for rows.Next() {
			var k storedKey
			if err := rows.Scan(&k.id, &k.encrypted); err != nil {
				rows.Close()
				return result, fmt.Errorf("failed to read stored private key row: %w", err)
			}
			batch = append(batch, k)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, fmt.Errorf("failed to list stored private keys: %w", err)
		}
		rows.Close()

		for _, k := range batch {
			id, err := crypto.PrivateKeyStorageKeyID(k.encrypted)
			if err != nil {
				result.NotV2++
				continue
			}
			if id == current {
				result.AlreadyCurrent++
				continue
			}
			if !kv.HasStorageKeyID(id) {
				result.Remaining[id]++
				continue
			}

			rewrapped, _, err := kv.RewrapPrivateKey(k.id, k.encrypted)
			if err != nil {
				result.Failures++
				result.Remaining[id]++
				continue
			}

			// Compare-and-swap: a row rewritten since it was read (a
			// credential rotation, or another instance running this sweep)
			// is left as it is and read again at the next start.
			res, err := db.ExecContext(ctx,
				`UPDATE agents SET encrypted_private_key = $1 WHERE id = $2 AND encrypted_private_key = $3`,
				rewrapped, k.id, k.encrypted)
			if err != nil {
				result.Failures++
				result.Remaining[id]++
				continue
			}
			if n, err := res.RowsAffected(); err != nil || n != 1 {
				result.Failures++
				result.Remaining[id]++
				continue
			}
			result.Rewritten++
		}

		if len(batch) < agentPrivateKeyRewrapBatchSize {
			break
		}
		last := batch[len(batch)-1].id
		after = &last
	}

	log.Printf("Agent private key sweep to storage key %d: alreadyCurrent=%d rewritten=%d failures=%d notV2=%d remaining=%s",
		current, result.AlreadyCurrent, result.Rewritten, result.Failures, result.NotV2, formatRemaining(result.Remaining))

	return result, nil
}

// formatRemaining writes the remaining counts as "id:count" pairs in id order.
func formatRemaining(remaining map[byte]int) string {
	ids := make([]int, 0, len(remaining))
	for id := range remaining {
		ids = append(ids, int(id))
	}
	if len(ids) == 0 {
		return "none"
	}
	slices.Sort(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d:%d", id, remaining[byte(id)]))
	}
	return strings.Join(parts, " ")
}
