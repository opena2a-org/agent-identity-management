package repository

import (
	"context"
	"database/sql"
	"errors"
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

// SetAgentPublicKeyTx stores the Ed25519 public key an agent registered for
// itself, in tx. The key it replaces becomes previous_public_key, and
// rotation_count goes up by one. The encrypted private key is left as it is.
func SetAgentPublicKeyTx(ctx context.Context, tx *sql.Tx, agent *domain.Agent) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE agents
		   SET previous_public_key = COALESCE(public_key, previous_public_key),
		       public_key = $2, key_algorithm = $3, key_created_at = $4, key_expires_at = $5,
		       rotation_count = rotation_count + 1, updated_at = NOW()
		 WHERE id = $1`,
		agent.ID, agent.PublicKey, agent.KeyAlgorithm, agent.KeyCreatedAt, agent.KeyExpiresAt)
	if err != nil {
		return fmt.Errorf("set agent public key: %w", err)
	}
	return oneRow(res, "set agent public key")
}

// SetAgentPQCKeyTx stores an agent's PQC public key, its algorithm and the
// hybrid mode flag in tx.
func SetAgentPQCKeyTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, publicKey, algorithm string, createdAt time.Time, hybrid bool) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE agents
		   SET pqc_public_key = $2, pqc_key_algorithm = $3, pqc_key_created_at = $4,
		       hybrid_mode_enabled = $5, updated_at = NOW()
		 WHERE id = $1`,
		id, publicKey, algorithm, createdAt, hybrid)
	if err != nil {
		return fmt.Errorf("set agent PQC key: %w", err)
	}
	return oneRow(res, "set agent PQC key")
}

// RotateAgentPQCKeyTx stores an agent's new PQC public key in tx. The key it
// replaces becomes previous_pqc_public_key, and rotation_count goes up by one.
func RotateAgentPQCKeyTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, publicKey, algorithm string, createdAt time.Time) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE agents
		   SET previous_pqc_public_key = pqc_public_key,
		       pqc_public_key = $2, pqc_key_algorithm = $3, pqc_key_created_at = $4,
		       rotation_count = rotation_count + 1, updated_at = NOW()
		 WHERE id = $1`,
		id, publicKey, algorithm, createdAt)
	if err != nil {
		return fmt.Errorf("rotate agent PQC key: %w", err)
	}
	return oneRow(res, "rotate agent PQC key")
}

// expiredKeyPredicate selects an agent whose key expired before $2, whose
// rotation grace window, if any, has closed, and which is neither suspended
// ($1) nor revoked ($3).
const expiredKeyPredicate = `
		key_expires_at IS NOT NULL
		  AND key_expires_at < $2
		  AND (key_rotation_grace_until IS NULL OR key_rotation_grace_until <= $2)
		  AND status NOT IN ($1, $3)`

// AgentRef names an agent and its organization.
type AgentRef struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
}

// AgentsWithExpiredKeys returns the agents SuspendAgentsWithExpiredKeys would
// suspend at now, without changing them.
func (r *AgentRepository) AgentsWithExpiredKeys(ctx context.Context, now time.Time) ([]AgentRef, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id FROM agents WHERE`+expiredKeyPredicate+`
		ORDER BY organization_id, id`,
		string(domain.AgentStatusSuspended), now, string(domain.AgentStatusRevoked))
	if err != nil {
		return nil, fmt.Errorf("agents with expired keys: %w", err)
	}
	defer rows.Close()
	refs := []AgentRef{}
	for rows.Next() {
		var ref AgentRef
		if err := rows.Scan(&ref.ID, &ref.OrganizationID); err != nil {
			return nil, fmt.Errorf("agents with expired keys: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("agents with expired keys: %w", err)
	}
	return refs, nil
}

// SuspendAgentWithExpiredKeyTx suspends one agent in tx if its key is still
// expired at now, and reports whether it did. An agent whose key was rotated,
// or which was suspended or revoked, since it was selected is left as it is.
func SuspendAgentWithExpiredKeyTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, now time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE agents SET status = $1, updated_at = $2 WHERE id = $4 AND`+expiredKeyPredicate,
		string(domain.AgentStatusSuspended), now, string(domain.AgentStatusRevoked), id)
	if err != nil {
		return false, fmt.Errorf("suspend agent with expired key: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("suspend agent with expired key: %w", err)
	}
	return n == 1, nil
}

// CreateCapabilityRequestTx runs the statement of the capability request
// repository's Create in tx: a pending request. A request that already has an
// id keeps it, so the id is known before the statement runs.
func CreateCapabilityRequestTx(ctx context.Context, tx *sql.Tx, req *domain.CapabilityRequest) error {
	return insertCapabilityRequest(ctx, tx, req)
}

// ErrCapabilityRequestNotPending is returned by DecideCapabilityRequestTx for
// a request that is not pending.
var ErrCapabilityRequestNotPending = errors.New("capability request is not pending")

// DecideCapabilityRequestTx moves a pending capability request to status in
// tx, naming its reviewer. A request that is no longer pending is left as it
// is, and the error wraps ErrCapabilityRequestNotPending.
func DecideCapabilityRequestTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, status domain.CapabilityRequestStatus, reviewedBy uuid.UUID) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE capability_requests
		   SET status = $2, reviewed_by = $3, reviewed_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND status = $4`,
		id, status, reviewedBy, domain.CapabilityRequestStatusPending)
	if err != nil {
		return fmt.Errorf("decide capability request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("decide capability request: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("decide capability request %s: %w", id, ErrCapabilityRequestNotPending)
	}
	return nil
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
