package application

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// agentPrivateKeyMigrationVersion marks, in schema_migrations, that every
// stored agent private key has been converted to the storage-bound v2 format.
const agentPrivateKeyMigrationVersion = "keyvault/agent-private-key-v2"

// AgentPrivateKeyMigrationResult is the record of one migration run. It holds
// counts only: no agent ID and no key material.
type AgentPrivateKeyMigrationResult struct {
	AlreadyComplete bool // an earlier run finished; nothing was read
	V1Read          int  // v1 ciphertexts decrypted
	V2Written       int  // rows rewritten in v2, bound to their own ID
	AlreadyV2       int  // rows already in v2 for their own ID
	Failures        int  // rows not readable as v1 or v2 for their own ID, or not rewritten
	Recorded        bool // the run finished with no failures and is marked complete
}

// MigrateAgentPrivateKeysToV2 converts each stored agent private key from v1
// (no additional data) to v2 (bound to the agent row's ID), once. The request
// path refuses v1, so this runs at startup before the server accepts requests.
//
// It is marked complete only when every row converts. Until then it runs again
// at the next startup and touches only the rows still in v1. A row that cannot
// be read stays refused; rotating that agent's credentials replaces it.
func MigrateAgentPrivateKeysToV2(ctx context.Context, db *sql.DB, kv *crypto.KeyVault) (AgentPrivateKeyMigrationResult, error) {
	var result AgentPrivateKeyMigrationResult

	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
		agentPrivateKeyMigrationVersion,
	).Scan(&result.AlreadyComplete); err != nil {
		return result, fmt.Errorf("failed to read migration state: %w", err)
	}
	if result.AlreadyComplete {
		return result, nil
	}

	type storedKey struct {
		id        uuid.UUID
		encrypted string
	}
	stored := make([]storedKey, 0)

	rows, err := db.QueryContext(ctx,
		`SELECT id, encrypted_private_key FROM agents
		 WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> ''`)
	if err != nil {
		return result, fmt.Errorf("failed to list stored private keys: %w", err)
	}
	for rows.Next() {
		var k storedKey
		if err := rows.Scan(&k.id, &k.encrypted); err != nil {
			rows.Close()
			return result, fmt.Errorf("failed to read stored private key row: %w", err)
		}
		stored = append(stored, k)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("failed to list stored private keys: %w", err)
	}
	rows.Close()

	for _, k := range stored {
		migrated, changed, err := kv.MigrateLegacyPrivateKey(k.id, k.encrypted)
		if err != nil {
			result.Failures++
			continue
		}
		if !changed {
			result.AlreadyV2++
			continue
		}
		result.V1Read++

		// Compare-and-swap: a row rewritten since it was read (a rotation, or
		// another instance running this migration) is left as it is and
		// confirmed by the next run.
		res, err := db.ExecContext(ctx,
			`UPDATE agents SET encrypted_private_key = $1 WHERE id = $2 AND encrypted_private_key = $3`,
			migrated, k.id, k.encrypted)
		if err != nil {
			result.Failures++
			continue
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			result.Failures++
			continue
		}
		result.V2Written++
	}

	var recordErr error
	if result.Failures == 0 {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`,
			agentPrivateKeyMigrationVersion,
		); err != nil {
			recordErr = fmt.Errorf("failed to record migration: %w", err)
		} else {
			result.Recorded = true
		}
	}

	log.Printf("Agent private key migration to v2: v1Read=%d v2Written=%d alreadyV2=%d failures=%d recorded=%t",
		result.V1Read, result.V2Written, result.AlreadyV2, result.Failures, result.Recorded)
	if !result.Recorded {
		log.Printf("Agent private key migration incomplete; it runs again at the next startup. Rows that fail stay refused until the agent's credentials are rotated.")
	}

	return result, recordErr
}
