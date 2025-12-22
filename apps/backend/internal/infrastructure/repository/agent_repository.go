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

// unmarshalTalksTo handles both string array and object array formats for talks_to field
// Format 1: ["uuid1", "uuid2"] - simple string array
// Format 2: [{"id": "uuid1", "name": "server-name"}] - object array
func unmarshalTalksTo(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Try format 1: string array
	var strings []string
	if err := json.Unmarshal(data, &strings); err == nil {
		return strings, nil
	}

	// Try format 2: object array
	var objects []map[string]interface{}
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
	}

	// Extract IDs and names from objects
	result := make([]string, 0, len(objects))
	for _, obj := range objects {
		if id, ok := obj["id"].(string); ok && id != "" {
			result = append(result, id)
		} else if name, ok := obj["name"].(string); ok && name != "" {
			result = append(result, name)
		}
	}
	return result, nil
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
		                    trust_score, talks_to, capabilities, metadata,
		                    key_created_at, key_expires_at, key_rotation_grace_until, previous_public_key, rotation_count,
		                    created_at, updated_at, created_by, created_by_name, created_by_email, created_by_sdk_token_id, created_by_api_key_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)
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

	// Marshal metadata to JSONB
	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if agent.Metadata == nil {
		metadataJSON = []byte("{}")
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
		agent.EncryptedPrivateKey,
		agent.KeyAlgorithm,
		agent.CertificateURL,
		agent.RepositoryURL,
		agent.DocumentationURL,
		agent.TrustScore,
		talksToJSON,
		capabilitiesJSON,
		metadataJSON,
		agent.KeyCreatedAt,
		agent.KeyExpiresAt,
		agent.KeyRotationGraceUntil,
		agent.PreviousPublicKey,
		agent.RotationCount,
		agent.CreatedAt,
		agent.UpdatedAt,
		agent.CreatedBy,
		agent.CreatedByName,
		agent.CreatedByEmail,
		agent.CreatedBySDKTokenID,
		agent.CreatedByAPIKeyID, // ✅ API key tracking for revocation
	)

	return err
}

// GetByID retrieves an agent by ID
func (r *AgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version,
		       public_key, encrypted_private_key, key_algorithm, certificate_url, repository_url, documentation_url,
		       trust_score, verified_at, talks_to, capabilities, metadata, created_at, updated_at, created_by, last_active,
		       key_created_at, key_expires_at, key_rotation_grace_until, previous_public_key, rotation_count,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''), created_by_sdk_token_id, created_by_api_key_id,
		       updated_by, COALESCE(updated_by_name, ''), COALESCE(updated_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
		FROM agents
		WHERE id = $1
	`

	agent := &domain.Agent{}
	var version sql.NullString
	var publicKey sql.NullString
	var encryptedPrivateKey sql.NullString
	var keyAlgorithm sql.NullString
	var certificateURL sql.NullString
	var repositoryURL sql.NullString
	var documentationURL sql.NullString
	var talksToJSON []byte
	var capabilitiesJSON []byte
	var metadataJSON []byte
	var lastActive sql.NullTime
	var keyCreatedAt sql.NullTime
	var keyExpiresAt sql.NullTime
	var keyRotationGraceUntil sql.NullTime
	var previousPublicKey sql.NullString
	var createdBySDKTokenID sql.NullString
	var createdByAPIKeyID sql.NullString
	var updatedBy sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&agent.ID,
		&agent.OrganizationID,
		&agent.Name,
		&agent.DisplayName,
		&agent.Description,
		&agent.AgentType,
		&agent.Status,
		&version,
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
		&metadataJSON,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.CreatedBy,
		&lastActive,
		&keyCreatedAt,
		&keyExpiresAt,
		&keyRotationGraceUntil,
		&previousPublicKey,
		&agent.RotationCount,
		&agent.CreatedByName,
		&agent.CreatedByEmail,
		&createdBySDKTokenID,
		&createdByAPIKeyID,
		&updatedBy,
		&agent.UpdatedByName,
		&agent.UpdatedByEmail,
		&agent.CapabilityViolationCount,
		&agent.IsCompromised,
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
	if createdBySDKTokenID.Valid {
		sdkTokenID, _ := uuid.Parse(createdBySDKTokenID.String)
		agent.CreatedBySDKTokenID = &sdkTokenID
	}
	if createdByAPIKeyID.Valid {
		apiKeyID, _ := uuid.Parse(createdByAPIKeyID.String)
		agent.CreatedByAPIKeyID = &apiKeyID
	}
	if updatedBy.Valid {
		uid, _ := uuid.Parse(updatedBy.String)
		agent.UpdatedBy = &uid
	}

	// Unmarshal talks_to from JSONB
	if len(talksToJSON) > 0 {
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}
	}

	// Unmarshal capabilities from JSONB
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}

	// Unmarshal metadata from JSONB
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return agent, nil
}

// GetByOrganization retrieves all agents in an organization
func (r *AgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
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
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		var metadataJSON []byte
		err := rows.Scan(
			&agent.ID,
			&agent.OrganizationID,
			&agent.Name,
			&agent.DisplayName,
			&description,
			&agent.AgentType,
			&agent.Status,
			&version,
			&publicKey,
			&certificateURL,
			&repositoryURL,
			&documentationURL,
			&agent.TrustScore,
			&agent.VerifiedAt,
			&talksToJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
			&agent.CreatedByName,
			&agent.CreatedByEmail,
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if description.Valid {
			agent.Description = description.String
		}
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

		// Unmarshal talks_to from JSONB (handles both string and object formats)
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata from JSONB
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
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
		    talks_to = $14, capabilities = $15, metadata = $16, updated_at = $17,
		    key_created_at = $18, key_expires_at = $19, key_rotation_grace_until = $20,
		    previous_public_key = $21, rotation_count = $22
		WHERE id = $23
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

	// Marshal metadata to JSONB
	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if agent.Metadata == nil {
		metadataJSON = []byte("{}")
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
		metadataJSON,
		agent.UpdatedAt,
		agent.KeyCreatedAt,
		agent.KeyExpiresAt,
		agent.KeyRotationGraceUntil,
		agent.PreviousPublicKey,
		agent.RotationCount,
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
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
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
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		var metadataJSON []byte
		err := rows.Scan(
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
			&agent.VerifiedAt,
			&talksToJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
			&agent.CreatedByName,
			&agent.CreatedByEmail,
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
		)
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

		// Unmarshal talks_to from JSONB (handles both string and object formats)
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata from JSONB
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
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

// IncrementViolationCount increments an agent's capability violation count
func (r *AgentRepository) IncrementViolationCount(id uuid.UUID) error {
	query := `
		UPDATE agents
		SET capability_violation_count = capability_violation_count + 1, updated_at = $1
		WHERE id = $2
	`
	_, err := r.db.Exec(query, time.Now(), id)
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
	// Query agents where talks_to JSONB array contains an object with matching id
	// The talks_to field can be:
	// 1. An array of strings: ["uuid1", "uuid2"]
	// 2. An array of objects: [{"id": "uuid1", "name": "server-name"}]
	// We need to check for both formats
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
		FROM agents
		WHERE organization_id = $1
		  AND (
		    talks_to @> $2::jsonb
		    OR talks_to @> $3::jsonb
		  )
		ORDER BY created_at DESC
	`

	// Format 1: Simple string array ["uuid"]
	mcpServerStringJSON := fmt.Sprintf(`["%s"]`, mcpServerID.String())
	// Format 2: Object array with id field [{"id": "uuid"}]
	mcpServerObjectJSON := fmt.Sprintf(`[{"id": "%s"}]`, mcpServerID.String())

	rows, err := r.db.Query(query, orgID, mcpServerStringJSON, mcpServerObjectJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		var metadataJSON []byte
		err := rows.Scan(
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
			&agent.VerifiedAt,
			&talksToJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
		)
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

		// Unmarshal talks_to from JSONB (handles both string and object formats)
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata from JSONB
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// GetByMCPServerName gets agents by MCP server NAME (in addition to ID)
// This is crucial because agent.talks_to often contains MCP server names, not IDs
func (r *AgentRepository) GetByMCPServerName(mcpServerName string, orgID uuid.UUID) ([]*domain.Agent, error) {
	// Query agents where talks_to JSONB array contains the MCP server name
	// The talks_to field can be:
	// 1. An array of strings: ["server-name"]
	// 2. An array of objects: [{"id": "uuid", "name": "server-name"}]
	// We need to check for both formats
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, trust_score, verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
		FROM agents
		WHERE organization_id = $1
		  AND (
		    talks_to @> $2::jsonb
		    OR talks_to @> $3::jsonb
		  )
		ORDER BY created_at DESC
	`

	// Format 1: Simple string array ["name"]
	mcpServerStringJSON := fmt.Sprintf(`["%s"]`, mcpServerName)
	// Format 2: Object array with name field [{"name": "server-name"}]
	mcpServerObjectJSON := fmt.Sprintf(`[{"name": "%s"}]`, mcpServerName)

	rows, err := r.db.Query(query, orgID, mcpServerStringJSON, mcpServerObjectJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		agent := &domain.Agent{}
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		var talksToJSON []byte
		var metadataJSON []byte
		err := rows.Scan(
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
			&agent.VerifiedAt,
			&talksToJSON,
			&metadataJSON,
			&agent.CreatedAt,
			&agent.UpdatedAt,
			&agent.CreatedBy,
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
		)
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

		// Unmarshal talks_to from JSONB (handles both string and object formats)
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata from JSONB
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
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
		       talks_to, capabilities, metadata,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false)
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
	var metadataJSON []byte

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
		&metadataJSON,
		&agent.CapabilityViolationCount,
		&agent.IsCompromised,
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
		agent.TalksTo, err = unmarshalTalksTo(talksToJSON)
		if err != nil {
			return nil, err
		}
	}
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
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
