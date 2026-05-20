package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

type SecretAuditRepository struct {
	db *sql.DB
}

func NewSecretAuditRepository(db *sql.DB) *SecretAuditRepository {
	return &SecretAuditRepository{db: db}
}

func (r *SecretAuditRepository) Create(entry *secrets.SecretAuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	entry.CreatedAt = time.Now()

	query := `
		INSERT INTO secret_audit_log (id, namespace_id, agent_id, operation, result, deny_reason, atc_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(query,
		entry.ID, entry.NamespaceID, entry.AgentID, entry.Operation,
		entry.Result, entry.DenyReason, entry.ATCID, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret audit entry: %w", err)
	}
	return nil
}

func (r *SecretAuditRepository) ListByNamespace(namespaceID uuid.UUID, limit, offset int) ([]*secrets.SecretAuditEntry, error) {
	query := `
		SELECT id, namespace_id, agent_id, operation, result, COALESCE(deny_reason, ''), COALESCE(atc_id, ''), created_at
		FROM secret_audit_log WHERE namespace_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	return r.scanEntries(r.db.Query(query, namespaceID, limit, offset))
}

func (r *SecretAuditRepository) ListByAgent(agentID uuid.UUID, since *time.Time, limit, offset int) ([]*secrets.SecretAuditEntry, error) {
	if since != nil {
		query := `
			SELECT id, namespace_id, agent_id, operation, result, COALESCE(deny_reason, ''), COALESCE(atc_id, ''), created_at
			FROM secret_audit_log WHERE agent_id = $1 AND created_at >= $2
			ORDER BY created_at DESC LIMIT $3 OFFSET $4
		`
		return r.scanEntries(r.db.Query(query, agentID, *since, limit, offset))
	}

	query := `
		SELECT id, namespace_id, agent_id, operation, result, COALESCE(deny_reason, ''), COALESCE(atc_id, ''), created_at
		FROM secret_audit_log WHERE agent_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	return r.scanEntries(r.db.Query(query, agentID, limit, offset))
}

func (r *SecretAuditRepository) scanEntries(rows *sql.Rows, err error) ([]*secrets.SecretAuditEntry, error) {
	if err != nil {
		return nil, fmt.Errorf("failed to query secret audit log: %w", err)
	}
	defer rows.Close()

	result := make([]*secrets.SecretAuditEntry, 0)
	for rows.Next() {
		entry := &secrets.SecretAuditEntry{}
		if err := rows.Scan(
			&entry.ID, &entry.NamespaceID, &entry.AgentID, &entry.Operation,
			&entry.Result, &entry.DenyReason, &entry.ATCID, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret audit entry: %w", err)
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}
