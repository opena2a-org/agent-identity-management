package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The statements below each write only the columns of one lifecycle change,
// in the caller's transaction, so the change and its authorization_transition
// record commit together and the statement cannot write back a column some
// other change set meanwhile.

// SetAgentStatusTx sets an agent's status in tx. A non-nil verifiedAt also
// sets verified_at.
func SetAgentStatusTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, status domain.AgentStatus, verifiedAt *time.Time) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE agents
		   SET status = $2, verified_at = COALESCE($3, verified_at), updated_at = NOW()
		 WHERE id = $1`, id, status, verifiedAt)
	if err != nil {
		return fmt.Errorf("set agent status: %w", err)
	}
	return oneRow(res, "set agent status")
}

// RotateAgentKeyTx stores agent's new Ed25519 key in tx. The key it replaces
// becomes previous_public_key, and rotation_count goes up by one.
func RotateAgentKeyTx(ctx context.Context, tx *sql.Tx, agent *domain.Agent) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE agents
		   SET previous_public_key = COALESCE(public_key, previous_public_key),
		       public_key = $2, encrypted_private_key = $3, key_algorithm = $4,
		       key_created_at = $5, key_expires_at = $6,
		       rotation_count = rotation_count + 1, updated_at = NOW()
		 WHERE id = $1`,
		agent.ID, agent.PublicKey, agent.EncryptedPrivateKey, agent.KeyAlgorithm,
		agent.KeyCreatedAt, agent.KeyExpiresAt)
	if err != nil {
		return fmt.Errorf("rotate agent key: %w", err)
	}
	return oneRow(res, "rotate agent key")
}

func oneRow(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if n != 1 {
		return fmt.Errorf("%s: %d rows changed, not 1", what, n)
	}
	return nil
}
