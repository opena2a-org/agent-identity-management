package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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

// ListByOrganization lists all connections for an organization's agents
func (r *AgentMCPConnectionRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	query := `
		SELECT c.id, c.agent_id, c.mcp_server_id, c.detection_id, c.connection_type,
		       c.first_connected_at, c.last_attested_at, c.attestation_count, c.is_active,
		       c.created_at, c.updated_at
		FROM agent_mcp_connections c
		JOIN agents a ON c.agent_id = a.id
		WHERE a.organization_id = $1
		ORDER BY c.last_attested_at DESC NULLS LAST, c.first_connected_at DESC
	`

	var connections []*domain.AgentMCPConnection
	err := r.db.SelectContext(ctx, &connections, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections for organization: %w", err)
	}

	return connections, nil
}

// AttestationTrendEntry represents a single day's attestation count
type AttestationTrendEntry struct {
	Date             string `db:"date" json:"date"`
	AttestationCount int    `db:"attestation_count" json:"attestationCount"`
	NewConnections   int    `db:"new_connections" json:"newConnections"`
}

// GetAttestationTrend returns daily attestation counts for the last N days
func (r *AgentMCPConnectionRepository) GetAttestationTrend(ctx context.Context, orgID uuid.UUID, days int) ([]AttestationTrendEntry, error) {
	query := `
		WITH date_series AS (
			SELECT generate_series(
				CURRENT_DATE - $2::integer * INTERVAL '1 day',
				CURRENT_DATE,
				'1 day'::interval
			)::date AS date
		),
		attestations_per_day AS (
			SELECT
				DATE(c.last_attested_at) AS date,
				COUNT(*) AS attestation_count
			FROM agent_mcp_connections c
			JOIN agents a ON c.agent_id = a.id
			WHERE a.organization_id = $1
			  AND c.last_attested_at >= CURRENT_DATE - $2::integer * INTERVAL '1 day'
			GROUP BY DATE(c.last_attested_at)
		),
		new_connections_per_day AS (
			SELECT
				DATE(c.first_connected_at) AS date,
				COUNT(*) AS new_connections
			FROM agent_mcp_connections c
			JOIN agents a ON c.agent_id = a.id
			WHERE a.organization_id = $1
			  AND c.first_connected_at >= CURRENT_DATE - $2::integer * INTERVAL '1 day'
			GROUP BY DATE(c.first_connected_at)
		)
		SELECT
			TO_CHAR(d.date, 'Mon DD') AS date,
			COALESCE(a.attestation_count, 0) AS attestation_count,
			COALESCE(n.new_connections, 0) AS new_connections
		FROM date_series d
		LEFT JOIN attestations_per_day a ON d.date = a.date
		LEFT JOIN new_connections_per_day n ON d.date = n.date
		ORDER BY d.date
	`

	var entries []AttestationTrendEntry
	err := r.db.SelectContext(ctx, &entries, query, orgID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get attestation trend: %w", err)
	}

	return entries, nil
}

// GetSupplyChainStats returns aggregate statistics for supply chain dashboard
type SupplyChainStats struct {
	TotalConnections    int `db:"total_connections" json:"totalConnections"`
	ActiveConnections   int `db:"active_connections" json:"activeConnections"`
	TotalAttestations   int `db:"total_attestations" json:"totalAttestations"`
	AttestationsLast24h int `db:"attestations_last_24h" json:"attestationsLast24h"`
	UniqueAgents        int `db:"unique_agents" json:"uniqueAgents"`
	UniqueMCPServers    int `db:"unique_mcp_servers" json:"uniqueMCPServers"`
}

func (r *AgentMCPConnectionRepository) GetSupplyChainStats(ctx context.Context, orgID uuid.UUID) (*SupplyChainStats, error) {
	query := `
		SELECT
			COUNT(*) AS total_connections,
			COUNT(*) FILTER (WHERE c.is_active = true) AS active_connections,
			COALESCE(SUM(c.attestation_count), 0) AS total_attestations,
			COUNT(*) FILTER (WHERE c.last_attested_at >= NOW() - INTERVAL '24 hours') AS attestations_last_24h,
			COUNT(DISTINCT c.agent_id) AS unique_agents,
			COUNT(DISTINCT c.mcp_server_id) AS unique_mcp_servers
		FROM agent_mcp_connections c
		JOIN agents a ON c.agent_id = a.id
		WHERE a.organization_id = $1
	`

	var stats SupplyChainStats
	err := r.db.GetContext(ctx, &stats, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get supply chain stats: %w", err)
	}

	return &stats, nil
}
