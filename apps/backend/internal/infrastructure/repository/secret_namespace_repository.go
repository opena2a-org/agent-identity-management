package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/util/sqlarray"
)

type SecretNamespaceRepository struct {
	db *sql.DB
}

func NewSecretNamespaceRepository(db *sql.DB) *SecretNamespaceRepository {
	return &SecretNamespaceRepository{db: db}
}

func (r *SecretNamespaceRepository) Create(ns *secrets.SecretNamespace) error {
	now := time.Now()
	if ns.ID == uuid.Nil {
		ns.ID = uuid.New()
	}
	ns.CreatedAt = now
	ns.UpdatedAt = now

	query := `
		INSERT INTO secret_namespaces (id, agent_id, namespace, backend_type, operations, url_patterns, status, created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(query,
		ns.ID, ns.AgentID, ns.Namespace, string(ns.BackendType),
		pq.Array(sqlarray.NonNilStrings(ns.Operations)), pq.Array(sqlarray.NonNilStrings(ns.URLPatterns)),
		string(ns.Status), ns.CreatedAt, ns.UpdatedAt, ns.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret namespace: %w", err)
	}
	return nil
}

func (r *SecretNamespaceRepository) GetByID(id uuid.UUID) (*secrets.SecretNamespace, error) {
	query := `
		SELECT id, agent_id, namespace, backend_type, operations, url_patterns, status, created_at, updated_at, created_by
		FROM secret_namespaces WHERE id = $1
	`
	ns := &secrets.SecretNamespace{}
	var ops, patterns []string
	var backendType, status string

	err := r.db.QueryRow(query, id).Scan(
		&ns.ID, &ns.AgentID, &ns.Namespace, &backendType,
		pq.Array(&ops), pq.Array(&patterns),
		&status, &ns.CreatedAt, &ns.UpdatedAt, &ns.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret namespace: %w", err)
	}

	ns.BackendType = secrets.BackendType(backendType)
	ns.Status = secrets.NamespaceStatus(status)
	ns.Operations = ops
	ns.URLPatterns = patterns
	return ns, nil
}

func (r *SecretNamespaceRepository) GetByAgentAndName(agentID uuid.UUID, namespace string) (*secrets.SecretNamespace, error) {
	query := `
		SELECT id, agent_id, namespace, backend_type, operations, url_patterns, status, created_at, updated_at, created_by
		FROM secret_namespaces WHERE agent_id = $1 AND namespace = $2
	`
	ns := &secrets.SecretNamespace{}
	var ops, patterns []string
	var backendType, status string

	err := r.db.QueryRow(query, agentID, namespace).Scan(
		&ns.ID, &ns.AgentID, &ns.Namespace, &backendType,
		pq.Array(&ops), pq.Array(&patterns),
		&status, &ns.CreatedAt, &ns.UpdatedAt, &ns.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret namespace by agent and name: %w", err)
	}

	ns.BackendType = secrets.BackendType(backendType)
	ns.Status = secrets.NamespaceStatus(status)
	ns.Operations = ops
	ns.URLPatterns = patterns
	return ns, nil
}

func (r *SecretNamespaceRepository) ListByAgent(agentID uuid.UUID) ([]*secrets.SecretNamespace, error) {
	query := `
		SELECT id, agent_id, namespace, backend_type, operations, url_patterns, status, created_at, updated_at, created_by
		FROM secret_namespaces WHERE agent_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secret namespaces: %w", err)
	}
	defer rows.Close()

	var result []*secrets.SecretNamespace
	for rows.Next() {
		ns := &secrets.SecretNamespace{}
		var ops, patterns []string
		var backendType, status string

		if err := rows.Scan(
			&ns.ID, &ns.AgentID, &ns.Namespace, &backendType,
			pq.Array(&ops), pq.Array(&patterns),
			&status, &ns.CreatedAt, &ns.UpdatedAt, &ns.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret namespace: %w", err)
		}

		ns.BackendType = secrets.BackendType(backendType)
		ns.Status = secrets.NamespaceStatus(status)
		ns.Operations = ops
		ns.URLPatterns = patterns
		result = append(result, ns)
	}
	return result, rows.Err()
}

func (r *SecretNamespaceRepository) UpdateStatus(id uuid.UUID, status secrets.NamespaceStatus) error {
	query := `UPDATE secret_namespaces SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(query, string(status), time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update secret namespace status: %w", err)
	}
	return nil
}

func (r *SecretNamespaceRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM secret_namespaces WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete secret namespace: %w", err)
	}
	return nil
}
