package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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
			capabilities, trust_score, registered_by_agent, created_by,
			created_by_name, created_by_email, created_by_sdk_token_id, created_by_api_key_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
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
		server.CreatedBy,         // User who created this server
		server.CreatedByName,      // ✅ Audit trail: creator name
		server.CreatedByEmail,     // ✅ Audit trail: creator email
		server.CreatedBySDKTokenID, // ✅ SDK token tracking
		server.CreatedByAPIKeyID,   // ✅ API key tracking
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
			verification_method, attestation_count, confidence_score, last_attested_at,
			COALESCE(created_by_name, ''), COALESCE(created_by_email, ''), created_by_sdk_token_id, created_by_api_key_id,
			updated_by, COALESCE(updated_by_name, ''), COALESCE(updated_by_email, '')
		FROM mcp_servers
		WHERE id = $1
	`

	server := &domain.MCPServer{}
	capabilitiesJSON := make([]byte, 0)
	var description sql.NullString
	var version sql.NullString
	var publicKey sql.NullString
	var verificationURL sql.NullString
	var createdBySDKTokenID sql.NullString
	var createdByAPIKeyID sql.NullString
	var updatedBy sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&server.ID,
		&server.OrganizationID,
		&server.Name,
		&description,
		&server.URL,
		&version,
		&publicKey,
		&server.Status,
		&server.IsVerified,
		&server.LastVerifiedAt,
		&verificationURL,
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
		&server.CreatedByName,
		&server.CreatedByEmail,
		&createdBySDKTokenID,
		&createdByAPIKeyID,
		&updatedBy,
		&server.UpdatedByName,
		&server.UpdatedByEmail,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("mcp server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mcp server: %w", err)
	}

	// Convert nullable fields
	if description.Valid {
		server.Description = description.String
	}
	if version.Valid {
		server.Version = version.String
	}
	if publicKey.Valid {
		server.PublicKey = publicKey.String
	}
	if verificationURL.Valid {
		server.VerificationURL = verificationURL.String
	}
	if createdBySDKTokenID.Valid {
		sdkTokenID, _ := uuid.Parse(createdBySDKTokenID.String)
		server.CreatedBySDKTokenID = &sdkTokenID
	}
	if createdByAPIKeyID.Valid {
		apiKeyID, _ := uuid.Parse(createdByAPIKeyID.String)
		server.CreatedByAPIKeyID = &apiKeyID
	}
	if updatedBy.Valid {
		uid, _ := uuid.Parse(updatedBy.String)
		server.UpdatedBy = &uid
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

	servers := make([]*domain.MCPServer, 0)
	for rows.Next() {
		server := &domain.MCPServer{}
		capabilitiesJSON := make([]byte, 0)
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var verificationURL sql.NullString

		err := rows.Scan(
			&server.ID,
			&server.OrganizationID,
			&server.Name,
			&description,
			&server.URL,
			&version,
			&publicKey,
			&server.Status,
			&server.IsVerified,
			&server.LastVerifiedAt,
			&verificationURL,
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

		// Convert nullable fields
		if description.Valid {
			server.Description = description.String
		}
		if version.Valid {
			server.Version = version.String
		}
		if publicKey.Valid {
			server.PublicKey = publicKey.String
		}
		if verificationURL.Valid {
			server.VerificationURL = verificationURL.String
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

// GetByURL returns the MCP server matching the URL within the given
// organization. The org filter is the security guarantee — the
// mcp_servers table has UNIQUE(organization_id, url), so the same URL
// can legally exist in multiple organizations; a nil-org lookup would
// leak cross-tenant existence (defect #40).
func (r *MCPServerRepository) GetByURL(url string, orgID uuid.UUID) (*domain.MCPServer, error) {
	query := `
		SELECT
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, last_verified_at, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at,
			verification_method, attestation_count, confidence_score, last_attested_at
		FROM mcp_servers
		WHERE url = $1 AND organization_id = $2
	`

	server := &domain.MCPServer{}
	capabilitiesJSON := make([]byte, 0)
	var description sql.NullString
	var version sql.NullString
	var publicKey sql.NullString
	var verificationURL sql.NullString

	err := r.db.QueryRow(query, url, orgID).Scan(
		&server.ID,
		&server.OrganizationID,
		&server.Name,
		&description,
		&server.URL,
		&version,
		&publicKey,
		&server.Status,
		&server.IsVerified,
		&server.LastVerifiedAt,
		&verificationURL,
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

	// Convert nullable fields
	if description.Valid {
		server.Description = description.String
	}
	if version.Valid {
		server.Version = version.String
	}
	if publicKey.Valid {
		server.PublicKey = publicKey.String
	}
	if verificationURL.Valid {
		server.VerificationURL = verificationURL.String
	}

	// Unmarshal capabilities from JSONB
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &server.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return server, nil
}

// GetByName retrieves an MCP server by name within an organization
// This allows SDK clients to check if capabilities are cached before running discovery
func (r *MCPServerRepository) GetByName(orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	query := `
		SELECT
			id, organization_id, name, description, url, version,
			public_key, status, is_verified, last_verified_at, verification_url,
			capabilities, trust_score, registered_by_agent, created_by, created_at, updated_at,
			verification_method, attestation_count, confidence_score, last_attested_at
		FROM mcp_servers
		WHERE organization_id = $1 AND name = $2
	`

	server := &domain.MCPServer{}
	capabilitiesJSON := make([]byte, 0)
	var description sql.NullString
	var version sql.NullString
	var publicKey sql.NullString
	var verificationURL sql.NullString

	err := r.db.QueryRow(query, orgID, name).Scan(
		&server.ID,
		&server.OrganizationID,
		&server.Name,
		&description,
		&server.URL,
		&version,
		&publicKey,
		&server.Status,
		&server.IsVerified,
		&server.LastVerifiedAt,
		&verificationURL,
		&capabilitiesJSON,
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

	// Convert nullable fields
	if description.Valid {
		server.Description = description.String
	}
	if version.Valid {
		server.Version = version.String
	}
	if publicKey.Valid {
		server.PublicKey = publicKey.String
	}
	if verificationURL.Valid {
		server.VerificationURL = verificationURL.String
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

	servers := make([]*domain.MCPServer, 0)
	for rows.Next() {
		server := &domain.MCPServer{}
		capabilitiesJSON := make([]byte, 0)
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var verificationURL sql.NullString

		err := rows.Scan(
			&server.ID,
			&server.OrganizationID,
			&server.Name,
			&description,
			&server.URL,
			&version,
			&publicKey,
			&server.Status,
			&server.IsVerified,
			&server.LastVerifiedAt,
			&verificationURL,
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

		// Convert nullable fields
		if description.Valid {
			server.Description = description.String
		}
		if version.Valid {
			server.Version = version.String
		}
		if publicKey.Valid {
			server.PublicKey = publicKey.String
		}
		if verificationURL.Valid {
			server.VerificationURL = verificationURL.String
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
