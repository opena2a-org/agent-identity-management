package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/opena2a/identity/backend/internal/domain"
)

// AgentMCPConnectionRepository handles database operations for agent-MCP connections
type AgentMCPConnectionRepository struct {
	db *sqlx.DB
}

// NewAgentMCPConnectionRepository creates a new repository instance
func NewAgentMCPConnectionRepository(db *sqlx.DB) *AgentMCPConnectionRepository {
	return &AgentMCPConnectionRepository{db: db}
}

// Create creates a new agent-MCP connection
func (r *AgentMCPConnectionRepository) Create(ctx context.Context, connection *domain.AgentMCPConnection) error {
	query := `
		INSERT INTO agent_mcp_connections (
			id, agent_id, mcp_server_id, detection_id, connection_type,
			first_connected_at, last_attested_at, attestation_count, is_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5::text, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (agent_id, mcp_server_id)
		DO UPDATE SET
			is_active = EXCLUDED.is_active,
			last_attested_at = EXCLUDED.last_attested_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()
	err := r.db.QueryRowContext(
		ctx,
		query,
		connection.ID,
		connection.AgentID,
		connection.MCPServerID,
		connection.DetectionID,
		string(connection.ConnectionType), // Cast ConnectionType to string
		connection.FirstConnectedAt,
		connection.LastAttestedAt,
		connection.AttestationCount,
		connection.IsActive,
		now,
		now,
	).Scan(&connection.ID, &connection.CreatedAt, &connection.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create agent-MCP connection: %w", err)
	}

	return nil
}

// GetByAgentAndMCPServer retrieves a connection by agent and MCP server IDs
func (r *AgentMCPConnectionRepository) GetByAgentAndMCPServer(ctx context.Context, agentID, mcpServerID uuid.UUID) (*domain.AgentMCPConnection, error) {
	query := `
		SELECT id, agent_id, mcp_server_id, detection_id, connection_type,
		       first_connected_at, last_attested_at, attestation_count, is_active,
		       created_at, updated_at
		FROM agent_mcp_connections
		WHERE agent_id = $1 AND mcp_server_id = $2
	`

	var connection domain.AgentMCPConnection
	err := r.db.GetContext(ctx, &connection, query, agentID, mcpServerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent-MCP connection: %w", err)
	}

	return &connection, nil
}

// ListByMCPServer lists all active connections for an MCP server
func (r *AgentMCPConnectionRepository) ListByMCPServer(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	query := `
		SELECT id, agent_id, mcp_server_id, detection_id, connection_type,
		       first_connected_at, last_attested_at, attestation_count, is_active,
		       created_at, updated_at
		FROM agent_mcp_connections
		WHERE mcp_server_id = $1 AND is_active = true
		ORDER BY first_connected_at DESC
	`

	var connections []*domain.AgentMCPConnection
	err := r.db.SelectContext(ctx, &connections, query, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections for MCP server: %w", err)
	}

	return connections, nil
}

// ListByAgent lists all active connections for an agent
func (r *AgentMCPConnectionRepository) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	query := `
		SELECT id, agent_id, mcp_server_id, detection_id, connection_type,
		       first_connected_at, last_attested_at, attestation_count, is_active,
		       created_at, updated_at
		FROM agent_mcp_connections
		WHERE agent_id = $1 AND is_active = true
		ORDER BY first_connected_at DESC
	`

	var connections []*domain.AgentMCPConnection
	err := r.db.SelectContext(ctx, &connections, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections for agent: %w", err)
	}

	return connections, nil
}

// UpdateAttestation updates the attestation count and timestamp
func (r *AgentMCPConnectionRepository) UpdateAttestation(ctx context.Context, agentID, mcpServerID uuid.UUID) error {
	query := `
		UPDATE agent_mcp_connections
		SET
			last_attested_at = $1,
			attestation_count = attestation_count + 1,
			updated_at = $1
		WHERE agent_id = $2 AND mcp_server_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), agentID, mcpServerID)
	if err != nil {
		return fmt.Errorf("failed to update attestation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no connection found for agent %s and MCP server %s", agentID, mcpServerID)
	}

	return nil
}

// Delete soft-deletes a connection by setting is_active to false
func (r *AgentMCPConnectionRepository) Delete(ctx context.Context, agentID, mcpServerID uuid.UUID) error {
	query := `
		UPDATE agent_mcp_connections
		SET is_active = false, updated_at = $1
		WHERE agent_id = $2 AND mcp_server_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), agentID, mcpServerID)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no connection found for agent %s and MCP server %s", agentID, mcpServerID)
	}

	return nil
}
