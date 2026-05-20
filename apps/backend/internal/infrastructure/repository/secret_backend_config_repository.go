package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

type SecretBackendConfigRepository struct {
	db *sql.DB
}

func NewSecretBackendConfigRepository(db *sql.DB) *SecretBackendConfigRepository {
	return &SecretBackendConfigRepository{db: db}
}

func (r *SecretBackendConfigRepository) Create(config *secrets.SecretBackendConfig) error {
	now := time.Now()
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	config.CreatedAt = now
	config.UpdatedAt = now

	configJSON, err := json.Marshal(config.ConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal backend config: %w", err)
	}

	query := `
		INSERT INTO secret_backend_configs (id, agent_id, backend_type, config_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.db.Exec(query,
		config.ID, config.AgentID, string(config.BackendType),
		configJSON, config.CreatedAt, config.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret backend config: %w", err)
	}
	return nil
}

func (r *SecretBackendConfigRepository) GetByAgentAndType(agentID uuid.UUID, backendType secrets.BackendType) (*secrets.SecretBackendConfig, error) {
	query := `
		SELECT id, agent_id, backend_type, config_json, created_at, updated_at
		FROM secret_backend_configs WHERE agent_id = $1 AND backend_type = $2
	`
	config := &secrets.SecretBackendConfig{}
	var bt string
	configJSON := make([]byte, 0)

	err := r.db.QueryRow(query, agentID, string(backendType)).Scan(
		&config.ID, &config.AgentID, &bt, &configJSON,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret backend config: %w", err)
	}

	config.BackendType = secrets.BackendType(bt)
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &config.ConfigJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal backend config JSON: %w", err)
		}
	}
	return config, nil
}

func (r *SecretBackendConfigRepository) ListByAgent(agentID uuid.UUID) ([]*secrets.SecretBackendConfig, error) {
	query := `
		SELECT id, agent_id, backend_type, config_json, created_at, updated_at
		FROM secret_backend_configs WHERE agent_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secret backend configs: %w", err)
	}
	defer rows.Close()

	result := make([]*secrets.SecretBackendConfig, 0)
	for rows.Next() {
		config := &secrets.SecretBackendConfig{}
		var bt string
		configJSON := make([]byte, 0)

		if err := rows.Scan(
			&config.ID, &config.AgentID, &bt, &configJSON,
			&config.CreatedAt, &config.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret backend config: %w", err)
		}

		config.BackendType = secrets.BackendType(bt)
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &config.ConfigJSON); err != nil {
				return nil, fmt.Errorf("failed to unmarshal backend config JSON: %w", err)
			}
		}
		result = append(result, config)
	}
	return result, rows.Err()
}

func (r *SecretBackendConfigRepository) Update(config *secrets.SecretBackendConfig) error {
	config.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(config.ConfigJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal backend config: %w", err)
	}

	query := `UPDATE secret_backend_configs SET config_json = $1, updated_at = $2 WHERE id = $3`
	_, err = r.db.Exec(query, configJSON, config.UpdatedAt, config.ID)
	if err != nil {
		return fmt.Errorf("failed to update secret backend config: %w", err)
	}
	return nil
}

func (r *SecretBackendConfigRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM secret_backend_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete secret backend config: %w", err)
	}
	return nil
}
