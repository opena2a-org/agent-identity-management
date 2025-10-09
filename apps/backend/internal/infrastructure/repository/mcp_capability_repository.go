package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type MCPServerCapabilityRepository struct {
	db *sql.DB
}

func NewMCPServerCapabilityRepository(db *sql.DB) *MCPServerCapabilityRepository {
	return &MCPServerCapabilityRepository{db: db}
}

func (r *MCPServerCapabilityRepository) Create(capability *domain.MCPServerCapability) error {
	query := `
		INSERT INTO mcp_server_capabilities (
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		capability.ID,
		capability.MCPServerID,
		capability.Name,
		capability.CapabilityType,
		capability.Description,
		capability.CapabilitySchema,
		capability.DetectedAt,
		capability.LastVerifiedAt,
		capability.IsActive,
		time.Now().UTC(),
		time.Now().UTC(),
	).Scan(&capability.ID, &capability.CreatedAt, &capability.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create mcp server capability: %w", err)
	}

	return nil
}

func (r *MCPServerCapabilityRepository) GetByID(id uuid.UUID) (*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE id = $1
	`

	capability := &domain.MCPServerCapability{}

	err := r.db.QueryRow(query, id).Scan(
		&capability.ID,
		&capability.MCPServerID,
		&capability.Name,
		&capability.CapabilityType,
		&capability.Description,
		&capability.CapabilitySchema,
		&capability.DetectedAt,
		&capability.LastVerifiedAt,
		&capability.IsActive,
		&capability.CreatedAt,
		&capability.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server capability not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server capability: %w", err)
	}

	return capability, nil
}

func (r *MCPServerCapabilityRepository) GetByServerID(serverID uuid.UUID) ([]*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE mcp_server_id = $1 AND is_active = true
		ORDER BY capability_type, name
	`

	rows, err := r.db.Query(query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp server capabilities: %w", err)
	}
	defer rows.Close()

	var capabilities []*domain.MCPServerCapability
	for rows.Next() {
		capability := &domain.MCPServerCapability{}

		err := rows.Scan(
			&capability.ID,
			&capability.MCPServerID,
			&capability.Name,
			&capability.CapabilityType,
			&capability.Description,
			&capability.CapabilitySchema,
			&capability.DetectedAt,
			&capability.LastVerifiedAt,
			&capability.IsActive,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server capability: %w", err)
		}

		capabilities = append(capabilities, capability)
	}

	return capabilities, nil
}

func (r *MCPServerCapabilityRepository) GetByServerIDAndType(serverID uuid.UUID, capType domain.MCPCapabilityType) ([]*domain.MCPServerCapability, error) {
	query := `
		SELECT
			id, mcp_server_id, name, capability_type, description,
			capability_schema, detected_at, last_verified_at, is_active,
			created_at, updated_at
		FROM mcp_server_capabilities
		WHERE mcp_server_id = $1 AND capability_type = $2 AND is_active = true
		ORDER BY name
	`

	rows, err := r.db.Query(query, serverID, capType)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp server capabilities by type: %w", err)
	}
	defer rows.Close()

	var capabilities []*domain.MCPServerCapability
	for rows.Next() {
		capability := &domain.MCPServerCapability{}

		err := rows.Scan(
			&capability.ID,
			&capability.MCPServerID,
			&capability.Name,
			&capability.CapabilityType,
			&capability.Description,
			&capability.CapabilitySchema,
			&capability.DetectedAt,
			&capability.LastVerifiedAt,
			&capability.IsActive,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server capability: %w", err)
		}

		capabilities = append(capabilities, capability)
	}

	return capabilities, nil
}

func (r *MCPServerCapabilityRepository) Update(capability *domain.MCPServerCapability) error {
	query := `
		UPDATE mcp_server_capabilities
		SET
			name = $1,
			description = $2,
			capability_schema = $3,
			last_verified_at = $4,
			is_active = $5,
			updated_at = $6
		WHERE id = $7
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		capability.Name,
		capability.Description,
		capability.CapabilitySchema,
		capability.LastVerifiedAt,
		capability.IsActive,
		time.Now().UTC(),
		capability.ID,
	).Scan(&capability.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update mcp server capability: %w", err)
	}

	return nil
}

func (r *MCPServerCapabilityRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM mcp_server_capabilities WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete mcp server capability: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server capability not found")
	}

	return nil
}

func (r *MCPServerCapabilityRepository) DeleteByServerID(serverID uuid.UUID) error {
	query := `DELETE FROM mcp_server_capabilities WHERE mcp_server_id = $1`

	_, err := r.db.Exec(query, serverID)
	if err != nil {
		return fmt.Errorf("failed to delete capabilities for server: %w", err)
	}

	return nil
}
