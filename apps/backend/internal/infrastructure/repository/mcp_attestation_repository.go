package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type MCPAttestationRepository struct {
	db *sql.DB
}

func NewMCPAttestationRepository(db *sql.DB) *MCPAttestationRepository {
	return &MCPAttestationRepository{db: db}
}

// ==================== Attestation Operations ====================

func (r *MCPAttestationRepository) CreateAttestation(attestation *domain.MCPAttestation) error {
	query := `
		INSERT INTO mcp_attestations (
			id, mcp_server_id, agent_id, attestation_data, signature,
			signature_verified, verified_at, expires_at, is_valid, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	// Marshal attestation data to JSON
	attestationJSON, err := json.Marshal(attestation.AttestationData)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation data: %w", err)
	}

	err = r.db.QueryRow(
		query,
		attestation.ID,
		attestation.MCPServerID,
		attestation.AgentID,
		attestationJSON,
		attestation.Signature,
		attestation.SignatureVerified,
		attestation.VerifiedAt,
		attestation.ExpiresAt,
		attestation.IsValid,
		time.Now().UTC(),
	).Scan(&attestation.ID, &attestation.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create attestation: %w", err)
	}

	return nil
}

func (r *MCPAttestationRepository) GetAttestationByID(id uuid.UUID) (*domain.MCPAttestation, error) {
	query := `
		SELECT
			id, mcp_server_id, agent_id, attestation_data, signature,
			signature_verified, verified_at, expires_at, is_valid, created_at
		FROM mcp_attestations
		WHERE id = $1
	`

	attestation := &domain.MCPAttestation{}
	var attestationJSON []byte

	err := r.db.QueryRow(query, id).Scan(
		&attestation.ID,
		&attestation.MCPServerID,
		&attestation.AgentID,
		&attestationJSON,
		&attestation.Signature,
		&attestation.SignatureVerified,
		&attestation.VerifiedAt,
		&attestation.ExpiresAt,
		&attestation.IsValid,
		&attestation.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("attestation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attestation: %w", err)
	}

	// Unmarshal attestation data
	if err := json.Unmarshal(attestationJSON, &attestation.AttestationData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestation data: %w", err)
	}

	return attestation, nil
}

func (r *MCPAttestationRepository) GetAttestationsByMCP(mcpServerID uuid.UUID) ([]*domain.MCPAttestation, error) {
	query := `
		SELECT
			a.id, a.mcp_server_id, a.agent_id, a.attestation_data, a.signature,
			a.signature_verified, a.verified_at, a.expires_at, a.is_valid, a.created_at,
			ag.name AS agent_name,
			ag.trust_score AS agent_trust_score
		FROM mcp_attestations a
		LEFT JOIN agents ag ON ag.id = a.agent_id
		WHERE a.mcp_server_id = $1
		ORDER BY a.verified_at DESC
	`

	rows, err := r.db.Query(query, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attestations: %w", err)
	}
	defer rows.Close()

	var attestations []*domain.MCPAttestation
	for rows.Next() {
		attestation := &domain.MCPAttestation{}
		var attestationJSON []byte

		err := rows.Scan(
			&attestation.ID,
			&attestation.MCPServerID,
			&attestation.AgentID,
			&attestationJSON,
			&attestation.Signature,
			&attestation.SignatureVerified,
			&attestation.VerifiedAt,
			&attestation.ExpiresAt,
			&attestation.IsValid,
			&attestation.CreatedAt,
			&attestation.AgentName,
			&attestation.AgentTrustScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attestation: %w", err)
		}

		// Unmarshal attestation data
		if err := json.Unmarshal(attestationJSON, &attestation.AttestationData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attestation data: %w", err)
		}

		attestations = append(attestations, attestation)
	}

	return attestations, nil
}

func (r *MCPAttestationRepository) GetValidAttestationsByMCP(mcpServerID uuid.UUID) ([]*domain.MCPAttestation, error) {
	query := `
		SELECT
			a.id, a.mcp_server_id, a.agent_id, a.attestation_data, a.signature,
			a.signature_verified, a.verified_at, a.expires_at, a.is_valid, a.created_at,
			ag.name AS agent_name,
			ag.trust_score AS agent_trust_score
		FROM mcp_attestations a
		LEFT JOIN agents ag ON ag.id = a.agent_id
		WHERE a.mcp_server_id = $1
			AND a.is_valid = true
			AND a.expires_at > NOW()
		ORDER BY a.verified_at DESC
	`

	rows, err := r.db.Query(query, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get valid attestations: %w", err)
	}
	defer rows.Close()

	var attestations []*domain.MCPAttestation
	for rows.Next() {
		attestation := &domain.MCPAttestation{}
		var attestationJSON []byte

		err := rows.Scan(
			&attestation.ID,
			&attestation.MCPServerID,
			&attestation.AgentID,
			&attestationJSON,
			&attestation.Signature,
			&attestation.SignatureVerified,
			&attestation.VerifiedAt,
			&attestation.ExpiresAt,
			&attestation.IsValid,
			&attestation.CreatedAt,
			&attestation.AgentName,
			&attestation.AgentTrustScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attestation: %w", err)
		}

		// Unmarshal attestation data
		if err := json.Unmarshal(attestationJSON, &attestation.AttestationData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attestation data: %w", err)
		}

		attestations = append(attestations, attestation)
	}

	return attestations, nil
}

func (r *MCPAttestationRepository) GetAttestationsByAgent(agentID uuid.UUID) ([]*domain.MCPAttestation, error) {
	query := `
		SELECT
			a.id, a.mcp_server_id, a.agent_id, a.attestation_data, a.signature,
			a.signature_verified, a.verified_at, a.expires_at, a.is_valid, a.created_at
		FROM mcp_attestations a
		WHERE a.agent_id = $1
		ORDER BY a.verified_at DESC
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attestations: %w", err)
	}
	defer rows.Close()

	var attestations []*domain.MCPAttestation
	for rows.Next() {
		attestation := &domain.MCPAttestation{}
		var attestationJSON []byte

		err := rows.Scan(
			&attestation.ID,
			&attestation.MCPServerID,
			&attestation.AgentID,
			&attestationJSON,
			&attestation.Signature,
			&attestation.SignatureVerified,
			&attestation.VerifiedAt,
			&attestation.ExpiresAt,
			&attestation.IsValid,
			&attestation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attestation: %w", err)
		}

		// Unmarshal attestation data
		if err := json.Unmarshal(attestationJSON, &attestation.AttestationData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attestation data: %w", err)
		}

		attestations = append(attestations, attestation)
	}

	return attestations, nil
}

func (r *MCPAttestationRepository) InvalidateAttestation(id uuid.UUID) error {
	query := `
		UPDATE mcp_attestations
		SET is_valid = false
		WHERE id = $1
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to invalidate attestation: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("attestation not found")
	}

	return nil
}

func (r *MCPAttestationRepository) InvalidateExpiredAttestations() error {
	query := `
		UPDATE mcp_attestations
		SET is_valid = false
		WHERE expires_at < NOW() AND is_valid = true
	`

	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to invalidate expired attestations: %w", err)
	}

	return nil
}

// ==================== Connection Operations ====================

func (r *MCPAttestationRepository) CreateConnection(connection *domain.AgentMCPConnection) error {
	query := `
		INSERT INTO agent_mcp_connections (
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (agent_id, mcp_server_id) DO UPDATE SET
			last_attested_at = EXCLUDED.last_attested_at,
			attestation_count = EXCLUDED.attestation_count,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		connection.ID,
		connection.AgentID,
		connection.MCPServerID,
		connection.DetectionID,
		connection.ConnectionType,
		connection.FirstConnectedAt,
		connection.LastAttestedAt,
		connection.AttestationCount,
		connection.IsActive,
		time.Now().UTC(),
		time.Now().UTC(),
	).Scan(&connection.ID, &connection.CreatedAt, &connection.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	return nil
}

func (r *MCPAttestationRepository) GetConnectionByID(id uuid.UUID) (*domain.AgentMCPConnection, error) {
	query := `
		SELECT
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count,
			is_active, created_at, updated_at
		FROM agent_mcp_connections
		WHERE id = $1
	`

	connection := &domain.AgentMCPConnection{}

	err := r.db.QueryRow(query, id).Scan(
		&connection.ID,
		&connection.AgentID,
		&connection.MCPServerID,
		&connection.DetectionID,
		&connection.ConnectionType,
		&connection.FirstConnectedAt,
		&connection.LastAttestedAt,
		&connection.AttestationCount,
		&connection.IsActive,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("connection not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	return connection, nil
}

func (r *MCPAttestationRepository) GetConnectionByAgentAndMCP(agentID, mcpServerID uuid.UUID) (*domain.AgentMCPConnection, error) {
	query := `
		SELECT
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count,
			is_active, created_at, updated_at
		FROM agent_mcp_connections
		WHERE agent_id = $1 AND mcp_server_id = $2
	`

	connection := &domain.AgentMCPConnection{}

	err := r.db.QueryRow(query, agentID, mcpServerID).Scan(
		&connection.ID,
		&connection.AgentID,
		&connection.MCPServerID,
		&connection.DetectionID,
		&connection.ConnectionType,
		&connection.FirstConnectedAt,
		&connection.LastAttestedAt,
		&connection.AttestationCount,
		&connection.IsActive,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Return nil without error if not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	return connection, nil
}

func (r *MCPAttestationRepository) GetConnectionsByAgent(agentID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	query := `
		SELECT
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count,
			is_active, created_at, updated_at
		FROM agent_mcp_connections
		WHERE agent_id = $1
		ORDER BY last_attested_at DESC NULLS LAST
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}
	defer rows.Close()

	var connections []*domain.AgentMCPConnection
	for rows.Next() {
		connection := &domain.AgentMCPConnection{}

		err := rows.Scan(
			&connection.ID,
			&connection.AgentID,
			&connection.MCPServerID,
			&connection.DetectionID,
			&connection.ConnectionType,
			&connection.FirstConnectedAt,
			&connection.LastAttestedAt,
			&connection.AttestationCount,
			&connection.IsActive,
			&connection.CreatedAt,
			&connection.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}

		connections = append(connections, connection)
	}

	return connections, nil
}

func (r *MCPAttestationRepository) GetConnectionsByMCP(mcpServerID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	query := `
		SELECT
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count,
			is_active, created_at, updated_at
		FROM agent_mcp_connections
		WHERE mcp_server_id = $1
		ORDER BY last_attested_at DESC NULLS LAST
	`

	rows, err := r.db.Query(query, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}
	defer rows.Close()

	var connections []*domain.AgentMCPConnection
	for rows.Next() {
		connection := &domain.AgentMCPConnection{}

		err := rows.Scan(
			&connection.ID,
			&connection.AgentID,
			&connection.MCPServerID,
			&connection.DetectionID,
			&connection.ConnectionType,
			&connection.FirstConnectedAt,
			&connection.LastAttestedAt,
			&connection.AttestationCount,
			&connection.IsActive,
			&connection.CreatedAt,
			&connection.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}

		connections = append(connections, connection)
	}

	return connections, nil
}

func (r *MCPAttestationRepository) UpdateConnection(connection *domain.AgentMCPConnection) error {
	query := `
		UPDATE agent_mcp_connections
		SET
			last_attested_at = $1,
			attestation_count = $2,
			is_active = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		query,
		connection.LastAttestedAt,
		connection.AttestationCount,
		connection.IsActive,
		time.Now().UTC(),
		connection.ID,
	).Scan(&connection.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	return nil
}

func (r *MCPAttestationRepository) DeleteConnection(id uuid.UUID) error {
	query := `DELETE FROM agent_mcp_connections WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found")
	}

	return nil
}

// ==================== Confidence Score Operations ====================

func (r *MCPAttestationRepository) UpdateMCPConfidenceScore(
	mcpServerID uuid.UUID,
	score float64,
	attestationCount int,
	lastAttestedAt time.Time,
) error {
	query := `
		UPDATE mcp_servers
		SET
			confidence_score = $1,
			attestation_count = $2,
			last_attested_at = $3,
			updated_at = $4
		WHERE id = $5
	`

	result, err := r.db.Exec(
		query,
		score,
		attestationCount,
		lastAttestedAt,
		time.Now().UTC(),
		mcpServerID,
	)

	if err != nil {
		return fmt.Errorf("failed to update confidence score: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server not found")
	}

	return nil
}
