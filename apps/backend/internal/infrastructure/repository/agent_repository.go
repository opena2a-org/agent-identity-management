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

// AgentRepository implements domain.AgentRepository
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create creates a new agent
func (r *AgentRepository) Create(agent *domain.Agent) error {
	query := `
		INSERT INTO agents (id, organization_id, name, display_name, description, agent_type, status, version,
		                    public_key, encrypted_private_key, key_algorithm, certificate_url, repository_url, documentation_url,
		                    trust_score, talks_to, capabilities,
		                    created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`

	now := time.Now()
	agent.ID = uuid.New()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	if agent.TrustScore == 0 {
		agent.TrustScore = 0.5 // Default score (50% - middle of 0.0 to 1.0 range)
	}
	if agent.Status == "" {
		agent.Status = domain.AgentStatusPending
	}
	if agent.KeyAlgorithm == "" {
		agent.KeyAlgorithm = "Ed25519" // Default algorithm
	}

	// Marshal talks_to to JSONB
	talksToJSON, err := json.Marshal(agent.TalksTo)
	if err != nil {
		return fmt.Errorf("failed to marshal talks_to: %w", err)
	}

	// Marshal capabilities to JSONB
	capabilitiesJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	_, err = r.db.Exec(query,
		agent.ID,
		agent.OrganizationID,
		agent.Name,
		agent.DisplayName,
		agent.Description,
		agent.AgentType,
		agent.Status,
		agent.Version,
		agent.PublicKey,
		agent.EncryptedPrivateKey, // ✅ NEW: Store encrypted private key
		agent.KeyAlgorithm,
		agent.CertificateURL,
		agent.RepositoryURL,
		agent.DocumentationURL,
		agent.TrustScore,
		talksToJSON,
		capabilitiesJSON, // ✅ Store capabilities
		agent.CreatedAt,
		agent.UpdatedAt,
		agent.CreatedBy,
	)

	return err
}

// GetByID retrieves an agent by ID
func (r *AgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version,
		       public_key, encrypted_private_key, key_algorithm, certificate_url, repository_url, documentation_url,
		       trust_score, verified_at, talks_to, capabilities, created_at, updated_at, created_by, last_active
		FROM agents
		WHERE id = $1
	`

	agent := &domain.Agent{}
	var publicKey sql.NullString
	var encryptedPrivateKey sql.NullString
	var keyAlgorithm sql.NullString
	var certificateURL sql.NullString
	var repositoryURL sql.NullString
	var documentationURL sql.NullString
	var talksToJSON []byte
	var capabilitiesJSON []byte
	var lastActive sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&agent.ID,
		&agent.OrganizationID,
		&agent.Name,
		&agent.DisplayName,
		&agent.Description,
		&agent.AgentType,
		&agent.Status,
		&agent.Version,
		&publicKey,
		&encryptedPrivateKey,
		&keyAlgorithm,
		&certificateURL,
		&repositoryURL,
		&documentationURL,
		&agent.TrustScore,
		&agent.VerifiedAt,
		&talksToJSON,
		&capabilitiesJSON,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.CreatedBy,
		&lastActive,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}

	// Convert nullable fields
	if publicKey.Valid {
		agent.PublicKey = &publicKey.String
	}
	if encryptedPrivateKey.Valid {
		agent.EncryptedPrivateKey = &encryptedPrivateKey.String
	}
	if keyAlgorithm.Valid {
		agent.KeyAlgorithm = keyAlgorithm.String
	}
	if certificateURL.Valid {
		agent.CertificateURL = certificateURL.String
	}
	if repositoryURL.Valid {
		agent.RepositoryURL = repositoryURL.String
	}
	if documentationURL.Valid {
		agent.DocumentationURL = documentationURL.String
	}
	if lastActive.Valid {
		agent.LastActive = &lastActive.Time
	}

	// Unmarshal talks_to from JSONB
	if len(talksToJSON) > 0 {
		if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
		}
	}

	// Unmarshal capabilities from JSONB
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return agent, nil
}

// GetByOrganization retrieves all agents in an organization
func (r *AgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, created_at, updated_at, created_by
		FROM agents
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		err := rows.Scan(
			&agent.ID,
			&agent.OrganizationID,
			&agent.Name,
			&agent.DisplayName,
			&agent.Description,
			&agent.AgentType,
			&agent.Status,
			&agent.Version,
			&publicKey,
			&certificateURL,
			&repositoryURL,
			&documentationURL,
			&agent.TrustScore,
			&agent.VerifiedAt,
			&talksToJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if publicKey.Valid {
			agent.PublicKey = &publicKey.String
		}
		if certificateURL.Valid {
			agent.CertificateURL = certificateURL.String
		}
		if repositoryURL.Valid {
			agent.RepositoryURL = repositoryURL.String
		}
		if documentationURL.Valid {
			agent.DocumentationURL = documentationURL.String
		}

		// Unmarshal talks_to from JSONB
		if len(talksToJSON) > 0 {
			if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// Update updates an agent
func (r *AgentRepository) Update(agent *domain.Agent) error {
	query := `
		UPDATE agents
		SET display_name = $1, description = $2, agent_type = $3, status = $4, version = $5,
		    public_key = $6, encrypted_private_key = $7, key_algorithm = $8, certificate_url = $9, repository_url = $10,
		    documentation_url = $11, trust_score = $12, verified_at = $13,
		    talks_to = $14, capabilities = $15, updated_at = $16
		WHERE id = $17
	`

	agent.UpdatedAt = time.Now()

	// Marshal talks_to to JSONB
	talksToJSON, err := json.Marshal(agent.TalksTo)
	if err != nil {
		return fmt.Errorf("failed to marshal talks_to: %w", err)
	}

	// Marshal capabilities to JSONB
	capabilitiesJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	_, err = r.db.Exec(query,
		agent.DisplayName,
		agent.Description,
		agent.AgentType,
		agent.Status,
		agent.Version,
		agent.PublicKey,
		agent.EncryptedPrivateKey,
		agent.KeyAlgorithm,
		agent.CertificateURL,
		agent.RepositoryURL,
		agent.DocumentationURL,
		agent.TrustScore,
		agent.VerifiedAt,
		talksToJSON,
		capabilitiesJSON,
		agent.UpdatedAt,
		agent.ID,
	)

	return err
}

// Delete deletes an agent
func (r *AgentRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM agents WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// List lists all agents with pagination
func (r *AgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, created_at, updated_at, created_by
		FROM agents
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		err := rows.Scan(
			&agent.ID,
			&agent.OrganizationID,
			&agent.Name,
			&agent.DisplayName,
			&agent.Description,
			&agent.AgentType,
			&agent.Status,
			&agent.Version,
			&publicKey,
			&certificateURL,
			&repositoryURL,
			&documentationURL,
			&agent.TrustScore,
			&agent.VerifiedAt,
			&talksToJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if publicKey.Valid {
			agent.PublicKey = &publicKey.String
		}
		if certificateURL.Valid {
			agent.CertificateURL = certificateURL.String
		}
		if repositoryURL.Valid {
			agent.RepositoryURL = repositoryURL.String
		}
		if documentationURL.Valid {
			agent.DocumentationURL = documentationURL.String
		}

		// Unmarshal talks_to from JSONB
		if len(talksToJSON) > 0 {
			if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// UpdateTrustScore updates an agent's trust score
func (r *AgentRepository) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	query := `
		UPDATE agents
		SET trust_score = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(query, newScore, time.Now(), id)
	return err
}

// MarkAsCompromised marks an agent as potentially compromised by setting status to suspended
func (r *AgentRepository) MarkAsCompromised(id uuid.UUID) error {
	query := `
		UPDATE agents
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(query, domain.AgentStatusSuspended, time.Now(), id)
	return err
}

// GetByMCPServer retrieves all agents that talk to a specific MCP server
func (r *AgentRepository) GetByMCPServer(mcpServerID uuid.UUID, orgID uuid.UUID) ([]*domain.Agent, error) {
	// Query agents where talks_to JSONB array contains the MCP server ID (as string)
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, created_at, updated_at, created_by
		FROM agents
		WHERE organization_id = $1
		  AND talks_to @> $2::jsonb
		ORDER BY created_at DESC
	`

	// Convert MCP server ID to JSON string format for JSONB comparison
	mcpServerJSON := fmt.Sprintf(`["%s"]`, mcpServerID.String())

	rows, err := r.db.Query(query, orgID, mcpServerJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		err := rows.Scan(
			&agent.ID,
			&agent.OrganizationID,
			&agent.Name,
			&agent.DisplayName,
			&agent.Description,
			&agent.AgentType,
			&agent.Status,
			&agent.Version,
			&publicKey,
			&certificateURL,
			&repositoryURL,
			&documentationURL,
			&agent.TrustScore,
			&agent.VerifiedAt,
			&talksToJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if publicKey.Valid {
			agent.PublicKey = &publicKey.String
		}
		if certificateURL.Valid {
			agent.CertificateURL = certificateURL.String
		}
		if repositoryURL.Valid {
			agent.RepositoryURL = repositoryURL.String
		}
		if documentationURL.Valid {
			agent.DocumentationURL = documentationURL.String
		}

		// Unmarshal talks_to from JSONB
		if len(talksToJSON) > 0 {
			if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// GetByMCPServerName gets agents by MCP server NAME (in addition to ID)
// This is crucial because agent.talks_to often contains MCP server names, not IDs
func (r *AgentRepository) GetByMCPServerName(mcpServerName string, orgID uuid.UUID) ([]*domain.Agent, error) {
	// Query agents where talks_to JSONB array contains the MCP server name (as string)
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, created_at, updated_at, created_by
		FROM agents
		WHERE organization_id = $1
		  AND talks_to @> $2::jsonb
		ORDER BY created_at DESC
	`

	// Convert MCP server name to JSON string format for JSONB comparison
	mcpServerJSON := fmt.Sprintf(`["%s"]`, mcpServerName)

	rows, err := r.db.Query(query, orgID, mcpServerJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		err := rows.Scan(
			&agent.ID,
			&agent.OrganizationID,
			&agent.Name,
			&agent.DisplayName,
			&agent.Description,
			&agent.AgentType,
			&agent.Status,
			&agent.Version,
			&publicKey,
			&certificateURL,
			&repositoryURL,
			&documentationURL,
			&agent.TrustScore,
			&agent.VerifiedAt,
			&talksToJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if publicKey.Valid {
			agent.PublicKey = &publicKey.String
		}
		if certificateURL.Valid {
			agent.CertificateURL = certificateURL.String
		}
		if repositoryURL.Valid {
			agent.RepositoryURL = repositoryURL.String
		}
		if documentationURL.Valid {
			agent.DocumentationURL = documentationURL.String
		}

		// Unmarshal talks_to from JSONB
		if len(talksToJSON) > 0 {
			if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}


// GetByName gets an agent by name within an organization
func (r *AgentRepository) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version,
		       public_key, certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       created_at, updated_at, created_by, encrypted_private_key, key_algorithm,
		       key_created_at, key_expires_at, key_rotation_grace_until, previous_public_key, rotation_count,
		       talks_to, capabilities
		FROM agents
		WHERE organization_id = $1 AND name = $2
		LIMIT 1
	`

	agent := &domain.Agent{}
	var publicKey sql.NullString
	var certificateURL sql.NullString
	var repositoryURL sql.NullString
	var documentationURL sql.NullString
	var version sql.NullString
	var verifiedAt sql.NullTime
	var encryptedPrivateKey sql.NullString
	var keyAlgorithm sql.NullString
	var keyCreatedAt sql.NullTime
	var keyExpiresAt sql.NullTime
	var keyRotationGraceUntil sql.NullTime
	var previousPublicKey sql.NullString
	var rotationCount sql.NullInt32
	var talksToJSON []byte
	var capabilitiesJSON []byte

	err := r.db.QueryRow(query, orgID, name).Scan(
		&agent.ID,
		&agent.OrganizationID,
		&agent.Name,
		&agent.DisplayName,
		&agent.Description,
		&agent.AgentType,
		&agent.Status,
		&version,
		&publicKey,
		&certificateURL,
		&repositoryURL,
		&documentationURL,
		&agent.TrustScore,
		&verifiedAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.CreatedBy,
		&encryptedPrivateKey,
		&keyAlgorithm,
		&keyCreatedAt,
		&keyExpiresAt,
		&keyRotationGraceUntil,
		&previousPublicKey,
		&rotationCount,
		&talksToJSON,
		&capabilitiesJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}

	// Convert nullable fields
	if version.Valid {
		agent.Version = version.String
	}
	if publicKey.Valid {
		agent.PublicKey = &publicKey.String
	}
	if certificateURL.Valid {
		agent.CertificateURL = certificateURL.String
	}
	if repositoryURL.Valid {
		agent.RepositoryURL = repositoryURL.String
	}
	if documentationURL.Valid {
		agent.DocumentationURL = documentationURL.String
	}
	if verifiedAt.Valid {
		agent.VerifiedAt = &verifiedAt.Time
	}
	if encryptedPrivateKey.Valid {
		agent.EncryptedPrivateKey = &encryptedPrivateKey.String
	}
	if keyAlgorithm.Valid {
		agent.KeyAlgorithm = keyAlgorithm.String
	}
	if keyCreatedAt.Valid {
		agent.KeyCreatedAt = &keyCreatedAt.Time
	}
	if keyExpiresAt.Valid {
		agent.KeyExpiresAt = &keyExpiresAt.Time
	}
	if keyRotationGraceUntil.Valid {
		agent.KeyRotationGraceUntil = &keyRotationGraceUntil.Time
	}
	if previousPublicKey.Valid {
		agent.PreviousPublicKey = &previousPublicKey.String
	}
	if rotationCount.Valid {
		agent.RotationCount = int(rotationCount.Int32)
	}

	// Unmarshal JSONB fields
	if len(talksToJSON) > 0 {
		if err := json.Unmarshal(talksToJSON, &agent.TalksTo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
		}
	}
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	return agent, nil
}


// UpdateLastActive updates the last_active timestamp for an agent
func (r *AgentRepository) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	query := `
		UPDATE agents
		SET last_active = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(query, agentID)
	if err != nil {
		return fmt.Errorf("failed to update agent last_active: %w", err)
	}

	return nil
}
