package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AgentRequestNonceRepository is the admission store for signed
// action-request statements, the agent_request_nonces table. It is shared by
// both mounts of the verification route and by every replica, because it is
// the database.
type AgentRequestNonceRepository struct {
	db *sql.DB
}

// NewAgentRequestNonceRepository creates the admission store.
func NewAgentRequestNonceRepository(db *sql.DB) *AgentRequestNonceRepository {
	return &AgentRequestNonceRepository{db: db}
}

var _ domain.ActionRequestNonceRepository = (*AgentRequestNonceRepository)(nil)

// admissionStatement is the window check and the nonce insert as ONE
// statement, so the clock reading the window compares against is the reading
// stored as admitted_at, and two requests carrying the same nonce cannot both
// pass a check before either inserts.
//
// The clock is statement_timestamp(), read once: now() would be the start of
// the transaction, not of this statement. The signed timestamp arrives as a
// parsed timestamptz parameter, never as the client's string, which PostgreSQL
// would accept in forms such as 'now' or 'infinity'.
//
// signedAt is the SQL expression for T. Production passes the parameter $4;
// the window tests pass an offset from the reading, because the database clock
// cannot be injected and the boundary cells must be exact.
//
// Parameters: $1 agent_id, $2 organization_id, $3 nonce, $4 T, $5 the window
// in seconds.
func admissionStatement(signedAt string) string {
	return `
		WITH reading AS (
			SELECT statement_timestamp() AS db_now
		), statement AS (
			SELECT db_now, ` + signedAt + ` AS signed_at
			FROM reading
		), verdict AS (
			SELECT db_now, signed_at,
			       CASE
			           WHEN signed_at < db_now - make_interval(secs => $5) THEN 'behind'
			           WHEN signed_at > db_now + make_interval(secs => $5) THEN 'ahead'
			           ELSE 'inside'
			       END AS side
			FROM statement
		), admitted AS (
			INSERT INTO agent_request_nonces (agent_id, organization_id, nonce, expires_at, admitted_at)
			SELECT $1, $2, $3, signed_at + make_interval(secs => $5), db_now
			FROM verdict
			WHERE side = 'inside'
			ON CONFLICT (agent_id, nonce) DO NOTHING
			RETURNING 1
		)
		SELECT side, (SELECT count(*) FROM admitted) FROM verdict
	`
}

var admitActionRequestNonceSQL = admissionStatement("$4::timestamptz")

// Admit runs the admission statement in its own transaction under
// SET LOCAL statement_timeout, so a statement blocked behind another
// transaction's uncommitted insert of the same key fails within
// domain.ActionRequestAdmissionTimeout instead of holding the request open.
// Any error means nothing was admitted.
func (r *AgentRequestNonceRepository) Admit(ctx context.Context, agentID, organizationID uuid.UUID, nonce []byte, signedAt time.Time) (domain.ActionRequestAdmission, error) {
	return r.admit(ctx, admitActionRequestNonceSQL, agentID, organizationID, nonce, signedAt)
}

func (r *AgentRequestNonceRepository) admit(ctx context.Context, query string, agentID, organizationID uuid.UUID, nonce []byte, signedAt any) (domain.ActionRequestAdmission, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%dms'",
		domain.ActionRequestAdmissionTimeout.Milliseconds())); err != nil {
		return 0, fmt.Errorf("bound admission statement: %w", err)
	}

	var side string
	var inserted int64
	err = tx.QueryRowContext(ctx, query, agentID, organizationID, nonce, signedAt,
		domain.ActionRequestWindowSeconds).Scan(&side, &inserted)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "57014" && ctx.Err() == nil {
			return 0, domain.ErrActionRequestAdmissionTimedOut
		}
		return 0, fmt.Errorf("admission statement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit admission: %w", err)
	}

	switch side {
	case "behind":
		return domain.ActionRequestBehindClock, nil
	case "ahead":
		return domain.ActionRequestAheadOfClock, nil
	case "inside":
		if inserted == 1 {
			return domain.ActionRequestAdmitted, nil
		}
		return domain.ActionRequestNonceReused, nil
	default:
		return 0, fmt.Errorf("admission statement returned an unknown side")
	}
}

// ListOrganizationIDs returns every organization id. The purge visits each
// one, so every statement on agent_request_nonces carries the organization
// predicate.
func (r *AgentRequestNonceRepository) ListOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM organizations ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list organizations for nonce purge: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan organization id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// purgeStatement deletes one organization's nonces past expires_at + K. The
// cutoff is taken from the database clock in SQL, the clock admission reads,
// never from this process.
const purgeStatement = `
	DELETE FROM agent_request_nonces
	WHERE organization_id = $1
	  AND expires_at < statement_timestamp() - make_interval(secs => $2)
`

// PurgeOrganization deletes one organization's nonces that no request can be
// admitted with any more, and returns the count.
func (r *AgentRequestNonceRepository) PurgeOrganization(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	result, err := r.db.ExecContext(ctx, purgeStatement, organizationID, domain.ActionRequestNonceRetentionSeconds)
	if err != nil {
		return 0, fmt.Errorf("purge action-request nonces: %w", err)
	}
	return result.RowsAffected()
}
