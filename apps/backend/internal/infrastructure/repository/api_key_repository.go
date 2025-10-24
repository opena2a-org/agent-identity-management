package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) Create(key *domain.APIKey) error {
	query := `
		INSERT INTO api_keys (id, organization_id, agent_id, name, key_hash, prefix, expires_at, is_active, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now()
	}

	_, err := r.db.Exec(query,
		key.ID,
		key.OrganizationID,
		key.AgentID,
		key.Name,
		key.KeyHash,
		key.Prefix,
		key.ExpiresAt,
		key.IsActive,
		key.CreatedAt,
		key.CreatedBy,
	)
	return err
}

func (r *APIKeyRepository) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	query := `
		SELECT id, organization_id, agent_id, name, key_hash, prefix, last_used_at, expires_at, is_active, created_at, created_by
		FROM api_keys
		WHERE id = $1
	`

	key := &domain.APIKey{}
	err := r.db.QueryRow(query, id).Scan(
		&key.ID,
		&key.OrganizationID,
		&key.AgentID,
		&key.Name,
		&key.KeyHash,
		&key.Prefix,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.IsActive,
		&key.CreatedAt,
		&key.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("api key not found")
	}
	return key, err
}

func (r *APIKeyRepository) GetByHash(hash string) (*domain.APIKey, error) {
	query := `
		SELECT id, organization_id, agent_id, name, key_hash, prefix, last_used_at, expires_at, is_active, created_at, created_by
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true
	`

	key := &domain.APIKey{}
	err := r.db.QueryRow(query, hash).Scan(
		&key.ID,
		&key.OrganizationID,
		&key.AgentID,
		&key.Name,
		&key.KeyHash,
		&key.Prefix,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.IsActive,
		&key.CreatedAt,
		&key.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return key, err
}

func (r *APIKeyRepository) GetByAgent(agentID uuid.UUID) ([]*domain.APIKey, error) {
	query := `
		SELECT id, organization_id, agent_id, name, key_hash, prefix, last_used_at, expires_at, is_active, created_at, created_by
		FROM api_keys
		WHERE agent_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		key := &domain.APIKey{}
		err := rows.Scan(
			&key.ID,
			&key.OrganizationID,
			&key.AgentID,
			&key.Name,
			&key.KeyHash,
			&key.Prefix,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.IsActive,
			&key.CreatedAt,
			&key.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *APIKeyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.APIKey, error) {
	query := `
		SELECT
			k.id, k.organization_id, k.agent_id, k.name, k.key_hash, k.prefix,
			k.last_used_at, k.expires_at, k.is_active, k.created_at, k.created_by,
			a.name as agent_name
		FROM api_keys k
		LEFT JOIN agents a ON k.agent_id = a.id
		WHERE k.organization_id = $1
		ORDER BY k.created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		key := &domain.APIKey{}
		var agentName sql.NullString
		err := rows.Scan(
			&key.ID,
			&key.OrganizationID,
			&key.AgentID,
			&key.Name,
			&key.KeyHash,
			&key.Prefix,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.IsActive,
			&key.CreatedAt,
			&key.CreatedBy,
			&agentName,
		)
		if err != nil {
			return nil, err
		}
		if agentName.Valid {
			key.AgentName = agentName.String
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *APIKeyRepository) Revoke(id uuid.UUID) error {
	query := `UPDATE api_keys SET is_active = false WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *APIKeyRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *APIKeyRepository) UpdateLastUsed(id uuid.UUID) error {
	query := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}
