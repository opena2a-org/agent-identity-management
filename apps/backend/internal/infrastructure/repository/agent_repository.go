package repository

import (
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
		                    trust_score, capability_violation_count, is_compromised, talks_to, capabilities,
		                    created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`

	now := time.Now()
	agent.ID = uuid.New()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	if agent.TrustScore == 0 {
		agent.TrustScore = 100.0 // Default score (changed from 0.5 to 100.0)
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
		agent.CapabilityViolationCount,
		agent.IsCompromised,
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
		       trust_score, verified_at, last_capability_check_at, capability_violation_count,
		       is_compromised, talks_to, capabilities, created_at, updated_at, created_by
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
	var lastCapabilityCheck sql.NullTime
	var talksToJSON []byte
	var capabilitiesJSON []byte

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
		&lastCapabilityCheck,
		&agent.CapabilityViolationCount,
		&agent.IsCompromised,
		&talksToJSON,
		&capabilitiesJSON,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.CreatedBy,
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
	if lastCapabilityCheck.Valid {
		agent.LastCapabilityCheckAt = &lastCapabilityCheck.Time
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
		    last_capability_check_at = $14, capability_violation_count = $15,
		    is_compromised = $16, talks_to = $17, updated_at = $18
		WHERE id = $19
	`

	agent.UpdatedAt = time.Now()

	// Marshal talks_to to JSONB
	talksToJSON, err := json.Marshal(agent.TalksTo)
	if err != nil {
		return fmt.Errorf("failed to marshal talks_to: %w", err)
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
		agent.LastCapabilityCheckAt,
		agent.CapabilityViolationCount,
		agent.IsCompromised,
		talksToJSON,
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

// UpdateTrustScore updates an agent's trust score and increments violation count
func (r *AgentRepository) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	query := `
		UPDATE agents
		SET trust_score = $1, capability_violation_count = capability_violation_count + 1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(query, newScore, time.Now(), id)
	return err
}

// MarkAsCompromised marks an agent as potentially compromised
func (r *AgentRepository) MarkAsCompromised(id uuid.UUID) error {
	query := `
		UPDATE agents
		SET is_compromised = true, status = $1, updated_at = $2
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
