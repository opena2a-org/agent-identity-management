package application

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// UnopenableAgentPrivateKey names an agent whose stored private key does not open for the
// agent's own row (crypto.KeyVault.DecryptPrivateKey), so every request path that reads
// the key refuses it.
type UnopenableAgentPrivateKey struct {
	AgentID        uuid.UUID
	OrganizationID uuid.UUID
	// V1 reports a key in the v1 format, which binds no row. The startup conversion
	// (MigrateAgentPrivateKeysToV2) converts such a key while it is not recorded complete.
	// A key that is not V1 opens in none of the formats for its own row under the vault's
	// key ring, and rotating the agent's credentials replaces it.
	V1 bool
}

// AgentPrivateKeyCheck is the record of one CheckAgentPrivateKeys run. It names agents and
// their organizations and holds no key material.
type AgentPrivateKeyCheck struct {
	Read               int  // stored keys read
	ConversionComplete bool // schema_migrations holds the startup conversion's completion marker
	// Unopenable lists the agents whose stored key does not open for their own row, in
	// organization ID and then agent ID order.
	Unopenable []UnopenableAgentPrivateKey
}

// CheckAgentPrivateKeys reads every stored agent private key and lists the agents whose key
// does not open for their own row. The startup conversion reports such rows only as a
// count (AgentPrivateKeyMigrationResult); this names them. It writes nothing.
func CheckAgentPrivateKeys(ctx context.Context, db *sql.DB, kv *crypto.KeyVault) (AgentPrivateKeyCheck, error) {
	check := AgentPrivateKeyCheck{Unopenable: []UnopenableAgentPrivateKey{}}

	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
		agentPrivateKeyMigrationVersion,
	).Scan(&check.ConversionComplete); err != nil {
		return check, fmt.Errorf("failed to read migration state: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, organization_id, encrypted_private_key FROM agents
		 WHERE encrypted_private_key IS NOT NULL AND encrypted_private_key <> ''
		 ORDER BY organization_id, id`)
	if err != nil {
		return check, fmt.Errorf("failed to list stored private keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key UnopenableAgentPrivateKey
		var encrypted string
		if err := rows.Scan(&key.AgentID, &key.OrganizationID, &encrypted); err != nil {
			return check, fmt.Errorf("failed to read stored private key row: %w", err)
		}
		check.Read++
		if _, err := kv.DecryptPrivateKey(key.AgentID, encrypted); err == nil {
			continue
		}
		_, converted, err := kv.MigrateLegacyPrivateKey(key.AgentID, encrypted)
		key.V1 = err == nil && converted
		check.Unopenable = append(check.Unopenable, key)
	}
	if err := rows.Err(); err != nil {
		return check, fmt.Errorf("failed to list stored private keys: %w", err)
	}
	return check, nil
}
