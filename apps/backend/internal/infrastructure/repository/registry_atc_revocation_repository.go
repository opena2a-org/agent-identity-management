package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// RegistryATCRevocationRepository keeps the credential revocation requests the
// Registry has not answered yet, the registry_atc_revocations table. The rows
// outlive the server process and every replica reads the same ones. Every
// time it compares or computes is read from the database clock, so replicas
// whose clocks differ agree on which requests are due.
type RegistryATCRevocationRepository struct {
	db *sql.DB
}

// NewRegistryATCRevocationRepository creates the store.
func NewRegistryATCRevocationRepository(db *sql.DB) *RegistryATCRevocationRepository {
	return &RegistryATCRevocationRepository{db: db}
}

// storeRegistryATCRevocationSQL stores a request that is due at once. A
// request already stored for the agent is replaced: it takes the new
// request_id and starts its attempts again.
//
// Parameters: $1 agent_id, $2 request_id.
const storeRegistryATCRevocationSQL = `
	INSERT INTO registry_atc_revocations (agent_id, request_id, requested_at, attempts, next_attempt_at)
	VALUES ($1, $2, statement_timestamp(), 0, statement_timestamp())
	ON CONFLICT (agent_id) DO UPDATE
	SET request_id = EXCLUDED.request_id,
	    requested_at = EXCLUDED.requested_at,
	    attempts = 0,
	    next_attempt_at = EXCLUDED.next_attempt_at
`

// claimRegistryATCRevocationsSQL takes the due requests, the earliest due
// first, counts an attempt for each and makes it due again only after the
// hold. FOR UPDATE SKIP LOCKED leaves a row another claim is taking to that
// claim, and a row that claim has committed is read again with its new
// next_attempt_at, which is not due, so a second claim does not take a
// request while its hold lasts.
//
// Parameters: $1 the most requests taken, $2 the hold in seconds.
const claimRegistryATCRevocationsSQL = `
	UPDATE registry_atc_revocations AS r
	SET attempts = r.attempts + 1,
	    next_attempt_at = statement_timestamp() + make_interval(secs => $2)
	FROM (
		SELECT agent_id
		FROM registry_atc_revocations
		WHERE next_attempt_at <= statement_timestamp()
		ORDER BY next_attempt_at, agent_id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	) AS due
	WHERE r.agent_id = due.agent_id
	RETURNING r.agent_id, r.request_id, r.requested_at, r.attempts
`

// deleteRegistryATCRevocationSQL deletes a claimed request unless it was
// stored again since the claim. Parameters: $1 agent_id, $2 request_id.
const deleteRegistryATCRevocationSQL = `
	DELETE FROM registry_atc_revocations
	WHERE agent_id = $1 AND request_id = $2
`

// retryRegistryATCRevocationSQL makes a claimed request due again after a
// delay counted from now, unless it was stored again since the claim.
// Parameters: $1 agent_id, $2 request_id, $3 the delay in seconds.
const retryRegistryATCRevocationSQL = `
	UPDATE registry_atc_revocations
	SET next_attempt_at = statement_timestamp() + make_interval(secs => $3)
	WHERE agent_id = $1 AND request_id = $2
`

// Store stores a request for agentID that is due at once, under a new request
// ID. A request already stored for the agent is replaced, so its attempts
// start again and an answer to it does not delete this one.
func (r *RegistryATCRevocationRepository) Store(ctx context.Context, agentID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, storeRegistryATCRevocationSQL, agentID, uuid.New()); err != nil {
		return fmt.Errorf("store registry credential revocation request: %w", err)
	}
	return nil
}

// Claim takes up to limit of the requests that are due and counts an attempt
// for each. A claimed request is due again once hold has passed, so a request
// whose sender stopped before Delete or Retry is claimed again then, by any
// server.
func (r *RegistryATCRevocationRepository) Claim(ctx context.Context, limit int, hold time.Duration) ([]domain.RegistryATCRevocation, error) {
	rows, err := r.db.QueryContext(ctx, claimRegistryATCRevocationsSQL, limit, hold.Seconds())
	if err != nil {
		return nil, fmt.Errorf("claim registry credential revocation requests: %w", err)
	}
	defer rows.Close()

	claimed := make([]domain.RegistryATCRevocation, 0)
	for rows.Next() {
		var c domain.RegistryATCRevocation
		if err := rows.Scan(&c.AgentID, &c.RequestID, &c.RequestedAt, &c.Attempts); err != nil {
			return nil, fmt.Errorf("scan registry credential revocation request: %w", err)
		}
		claimed = append(claimed, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim registry credential revocation requests: %w", err)
	}
	return claimed, nil
}

// Delete deletes the claimed request, once the Registry has revoked the
// credentials or the attempts have ended. A request stored again since the
// claim stays.
func (r *RegistryATCRevocationRepository) Delete(ctx context.Context, c domain.RegistryATCRevocation) error {
	if _, err := r.db.ExecContext(ctx, deleteRegistryATCRevocationSQL, c.AgentID, c.RequestID); err != nil {
		return fmt.Errorf("delete registry credential revocation request: %w", err)
	}
	return nil
}

// Retry makes the claimed request due again once after has passed. A request
// stored again since the claim keeps its own schedule.
func (r *RegistryATCRevocationRepository) Retry(ctx context.Context, c domain.RegistryATCRevocation, after time.Duration) error {
	if _, err := r.db.ExecContext(ctx, retryRegistryATCRevocationSQL, c.AgentID, c.RequestID, after.Seconds()); err != nil {
		return fmt.Errorf("schedule registry credential revocation request: %w", err)
	}
	return nil
}
