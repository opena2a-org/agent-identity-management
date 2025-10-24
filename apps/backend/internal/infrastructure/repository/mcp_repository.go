package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type MCPServerRepository struct {
	db *sql.DB
}

func NewMCPServerRepository(db *sql.DB) *MCPServerRepository {
	return &MCPServerRepository{db: db}
}

func (r *MCPServerRepository) Create(server *domain.MCPServer) error {
	query := `
		INSERT INTO mcp_servers (
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`

	// Marshal capabilities to JSON (database uses JSONB, not text array)
	capabilitiesJSON, err := json.Marshal(server.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	err = r.db.QueryRow(
		query,
		server.ID,
		server.OrganizationID,
		server.Name,
		server.Description,
		server.URL,
		server.Version,
		server.PublicKey,
		server.Status,
		server.IsVerified,
		server.VerificationURL,
		capabilitiesJSON, // Use JSON bytes instead of pq.Array
		server.TrustScore,
		server.RegisteredByAgent, // Can be nil for user-registered servers
		server.CreatedBy,          // ✅ FIXED: Added created_by field
		time.Now().UTC(),
		time.Now().UTC(),
	).Scan(&server.ID, &server.CreatedAt, &server.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create mcp server: %w", err)
	}

	return nil
}

func (r *MCPServerRepository) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	query := `
		SELECT
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, last_verified_at, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at,
			verification_method, attestation_count, confidence_score, last_attested_at
		FROM mcp_servers
		WHERE id = $1
	`

	server := &domain.MCPServer{}
	var capabilitiesJSON []byte

	err := r.db.QueryRow(query, id).Scan(
		&server.ID,
		&server.OrganizationID,
		&server.Name,
		&server.Description,
		&server.URL,
		&server.Version,
		&server.PublicKey,
		&server.Status,
		&server.IsVerified,
		&server.LastVerifiedAt,
		&server.VerificationURL,
		&capabilitiesJSON, // Read as JSON bytes
		&server.TrustScore,
		&server.RegisteredByAgent,
		&server.CreatedBy,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.VerificationMethod,
		&server.AttestationCount,
		&server.ConfidenceScore,
		&server.LastAttestedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server: %w", err)
	}

	// Unmarshal capabilities from JSONB
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &server.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return server, nil
}

func (r *MCPServerRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	query := `
		SELECT
			m.id, m.organization_id, m.name, m.description, m.url, m.version,
			m.public_key, m.status, m.is_verified, m.last_verified_at, m.verification_url,
			m.capabilities, m.trust_score, m.registered_by_agent, m.created_by, m.created_at, m.updated_at,
			m.verification_method, m.attestation_count, m.confidence_score, m.last_attested_at,
			COALESCE(COUNT(v.id), 0) AS verification_count
		FROM mcp_servers m
		LEFT JOIN verification_events v ON v.mcp_server_id = m.id
		WHERE m.organization_id = $1
		GROUP BY m.id, m.organization_id, m.name, m.description, m.url, m.version,
			m.public_key, m.status, m.is_verified, m.last_verified_at, m.verification_url,
			m.capabilities, m.trust_score, m.registered_by_agent, m.created_by, m.created_at, m.updated_at,
			m.verification_method, m.attestation_count, m.confidence_score, m.last_attested_at
		ORDER BY m.created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp servers: %w", err)
	}
	defer rows.Close()

	var servers []*domain.MCPServer
	for rows.Next() {
		server := &domain.MCPServer{}
		var capabilitiesJSON []byte

		err := rows.Scan(
			&server.ID,
			&server.OrganizationID,
			&server.Name,
			&server.Description,
			&server.URL,
			&server.Version,
			&server.PublicKey,
			&server.Status,
			&server.IsVerified,
			&server.LastVerifiedAt,
			&server.VerificationURL,
			&capabilitiesJSON, // Read as JSON bytes
			&server.TrustScore,
			&server.RegisteredByAgent,
			&server.CreatedBy,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.VerificationMethod,
			&server.AttestationCount,
			&server.ConfidenceScore,
			&server.LastAttestedAt,
			&server.VerificationCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server: %w", err)
		}

		// Unmarshal capabilities from JSONB
		if len(capabilitiesJSON) > 0 {
			if err := json.Unmarshal(capabilitiesJSON, &server.Capabilities); err != nil {
				return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (r *MCPServerRepository) GetByURL(url string) (*domain.MCPServer, error) {
	query := `
		SELECT
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, last_verified_at, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at,
			verification_method, attestation_count, confidence_score, last_attested_at
		FROM mcp_servers
		WHERE url = $1
	`

	server := &domain.MCPServer{}
	var capabilitiesJSON []byte

	err := r.db.QueryRow(query, url).Scan(
		&server.ID,
		&server.OrganizationID,
		&server.Name,
		&server.Description,
		&server.URL,
		&server.Version,
		&server.PublicKey,
		&server.Status,
		&server.IsVerified,
		&server.LastVerifiedAt,
		&server.VerificationURL,
		&capabilitiesJSON, // Read as JSON bytes
		&server.TrustScore,
		&server.RegisteredByAgent,
		&server.CreatedBy,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.VerificationMethod,
		&server.AttestationCount,
		&server.ConfidenceScore,
		&server.LastAttestedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server: %w", err)
	}

	// Unmarshal capabilities from JSONB
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &server.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return server, nil
}

func (r *MCPServerRepository) Update(server *domain.MCPServer) error {
	query := `
		UPDATE mcp_servers
		SET
			name = $1,
			description = $2,
			url = $3,
			version = $4,
			public_key = $5,
			status = $6,
			is_verified = $7,
			last_verified_at = $8,
			verification_url = $9,
			capabilities = $10,
			trust_score = $11,
			updated_at = $12
		WHERE id = $13
		RETURNING updated_at
	`

	// Marshal capabilities to JSON
	capabilitiesJSON, err := json.Marshal(server.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	err = r.db.QueryRow(
		query,
		server.Name,
		server.Description,
		server.URL,
		server.Version,
		server.PublicKey,
		server.Status,
		server.IsVerified,
		server.LastVerifiedAt,
		server.VerificationURL,
		capabilitiesJSON, // Use JSON bytes
		server.TrustScore,
		time.Now().UTC(),
		server.ID,
	).Scan(&server.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update mcp server: %w", err)
	}

	return nil
}

func (r *MCPServerRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM mcp_servers WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete mcp server: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("mcp server not found")
	}

	return nil
}

func (r *MCPServerRepository) List(limit, offset int) ([]*domain.MCPServer, error) {
	query := `
		SELECT
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, last_verified_at, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at
		FROM mcp_servers
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list mcp servers: %w", err)
	}
	defer rows.Close()

	var servers []*domain.MCPServer
	for rows.Next() {
		server := &domain.MCPServer{}
		var capabilitiesJSON []byte

		err := rows.Scan(
			&server.ID,
			&server.OrganizationID,
			&server.Name,
			&server.Description,
			&server.URL,
			&server.Version,
			&server.PublicKey,
			&server.Status,
			&server.IsVerified,
			&server.LastVerifiedAt,
			&server.VerificationURL,
			&capabilitiesJSON, // Read as JSON bytes
			&server.TrustScore,
			&server.RegisteredByAgent,
			&server.CreatedBy,
			&server.CreatedAt,
			&server.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mcp server: %w", err)
		}

		// Unmarshal capabilities from JSONB
		if len(capabilitiesJSON) > 0 {
			if err := json.Unmarshal(capabilitiesJSON, &server.Capabilities); err != nil {
				return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
			}
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (r *MCPServerRepository) GetVerificationStatus(id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	query := `
		SELECT
			id,
			is_verified,
			last_verified_at,
			trust_score,
			status,
			(SELECT COUNT(*) FROM mcp_server_keys WHERE server_id = $1) as public_key_count
		FROM mcp_servers
		WHERE id = $1
	`

	status := &domain.MCPServerVerificationStatus{}

	err := r.db.QueryRow(query, id).Scan(
		&status.ServerID,
		&status.IsVerified,
		&status.LastVerifiedAt,
		&status.TrustScore,
		&status.Status,
		&status.PublicKeyCount,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get verification status: %w", err)
	}

	return status, nil
}

// AddPublicKey adds a public key to an MCP server
func (r *MCPServerRepository) AddPublicKey(ctx context.Context, serverID uuid.UUID, publicKey string, keyType string) error {
	query := `
		INSERT INTO mcp_server_keys (id, server_id, public_key, key_type, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		uuid.New(),
		serverID,
		publicKey,
		keyType,
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to add public key: %w", err)
	}

	return nil
}

// VerifyServer performs cryptographic verification of an MCP server
func (r *MCPServerRepository) VerifyServer(ctx context.Context, serverID uuid.UUID) error {
	// Update server verification status
	query := `
		UPDATE mcp_servers
		SET
			is_verified = true,
			last_verified_at = $1,
			status = $2,
			updated_at = $1
		WHERE id = $3
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		time.Now().UTC(),
		domain.MCPServerStatusVerified,
		serverID,
	)

	if err != nil {
		return fmt.Errorf("failed to verify server: %w", err)
	}

	return nil
}
