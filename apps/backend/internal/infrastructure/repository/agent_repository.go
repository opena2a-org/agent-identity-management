package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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
	strings := make([]string, 0)
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
		                    pqc_public_key, pqc_key_algorithm, hybrid_mode_enabled, pqc_key_created_at, pqc_key_expires_at, previous_pqc_public_key,
		                    created_at, updated_at, created_by, created_by_name, created_by_email, created_by_sdk_token_id, created_by_api_key_id,
		                    declared_purpose)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37)
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

	// Marshal declared_purpose to JSONB; SQL NULL (untyped nil) when not declared.
	var declaredPurposeParam interface{}
	if agent.DeclaredPurpose != nil {
		declaredPurposeJSON, mErr := json.Marshal(agent.DeclaredPurpose)
		if mErr != nil {
			return fmt.Errorf("failed to marshal declared_purpose: %w", mErr)
		}
		declaredPurposeParam = declaredPurposeJSON
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
		agent.PQCPublicKey,
		agent.PQCKeyAlgorithm,
		agent.HybridModeEnabled,
		agent.PQCKeyCreatedAt,
		agent.PQCKeyExpiresAt,
		agent.PreviousPQCPublicKey,
		agent.CreatedAt,
		agent.UpdatedAt,
		agent.CreatedBy,
		agent.CreatedByName,
		agent.CreatedByEmail,
		agent.CreatedBySDKTokenID,
		agent.CreatedByAPIKeyID,
		declaredPurposeParam,
	)

	return err
}

// GetByID retrieves an agent by ID
func (r *AgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version,
		       public_key, encrypted_private_key, key_algorithm, certificate_url, repository_url, documentation_url,
		       COALESCE(trust_score, 0), verified_at, talks_to, capabilities, metadata, created_at, updated_at, created_by, last_active,
		       key_created_at, key_expires_at, key_rotation_grace_until, previous_public_key, rotation_count,
		       pqc_public_key, pqc_key_algorithm, COALESCE(hybrid_mode_enabled, false), pqc_key_created_at, pqc_key_expires_at, previous_pqc_public_key,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''), created_by_sdk_token_id, created_by_api_key_id,
		       updated_by, COALESCE(updated_by_name, ''), COALESCE(updated_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
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
	talksToJSON := make([]byte, 0)
	capabilitiesJSON := make([]byte, 0)
	metadataJSON := make([]byte, 0)
	declaredPurposeJSON := make([]byte, 0)
	var lastActive sql.NullTime
	var keyCreatedAt sql.NullTime
	var keyExpiresAt sql.NullTime
	var keyRotationGraceUntil sql.NullTime
	var previousPublicKey sql.NullString
	var pqcPublicKey sql.NullString
	var pqcKeyAlgorithm sql.NullString
	var pqcKeyCreatedAt sql.NullTime
	var pqcKeyExpiresAt sql.NullTime
	var previousPQCPublicKey sql.NullString
	var createdBySDKTokenID sql.NullString
	var createdByAPIKeyID sql.NullString
	var updatedBy sql.NullString
	// `rotation_count` is nullable. GetByName already scans it through a NullInt32; this
	// method did not, so a NULL there failed the whole row on the auth path — the same
	// sibling-does-it-right divergence as `description` below, in the same method.
	var rotationCount sql.NullInt32
	// `description` is nullable. Scanning it into a plain string fails the entire row,
	// and GetByID is what the agent auth middlewares call — a failed read there is
	// reported as "Agent not found", which is indistinguishable from a revocation denial.
	var description sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&agent.ID,
		&agent.OrganizationID,
		&agent.Name,
		&agent.DisplayName,
		&description,
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
		&rotationCount,
		&pqcPublicKey,
		&pqcKeyAlgorithm,
		&agent.HybridModeEnabled,
		&pqcKeyCreatedAt,
		&pqcKeyExpiresAt,
		&previousPQCPublicKey,
		&agent.CreatedByName,
		&agent.CreatedByEmail,
		&createdBySDKTokenID,
		&createdByAPIKeyID,
		&updatedBy,
		&agent.UpdatedByName,
		&agent.UpdatedByEmail,
		&agent.CapabilityViolationCount,
		&agent.IsCompromised,
		&declaredPurposeJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}

	// Convert nullable fields
	if description.Valid {
		agent.Description = description.String
	}
	if rotationCount.Valid {
		agent.RotationCount = int(rotationCount.Int32)
	}
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
	if pqcPublicKey.Valid {
		agent.PQCPublicKey = &pqcPublicKey.String
	}
	if pqcKeyAlgorithm.Valid {
		agent.PQCKeyAlgorithm = &pqcKeyAlgorithm.String
	}
	if pqcKeyCreatedAt.Valid {
		agent.PQCKeyCreatedAt = &pqcKeyCreatedAt.Time
	}
	if pqcKeyExpiresAt.Valid {
		agent.PQCKeyExpiresAt = &pqcKeyExpiresAt.Time
	}
	if previousPQCPublicKey.Valid {
		agent.PreviousPQCPublicKey = &previousPQCPublicKey.String
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

	// Unmarshal declared_purpose from JSONB
	if len(declaredPurposeJSON) > 0 {
		if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
			return nil, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
		}
	}

	return agent, nil
}

// GetByOrganization retrieves all agents in an organization
func (r *AgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	agents, _, err := r.GetByOrganizationPaged(orgID, 0, 0)
	return agents, err
}

// GetByOrganizationPaged retrieves one page of an organization's agents
// plus the total agent count for the organization. limit <= 0 returns
// all agents.
func (r *AgentRepository) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, COALESCE(trust_score, 0), verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
		FROM agents
		WHERE organization_id = $1
		ORDER BY created_at DESC, id
	`
	args := []interface{}{orgID}
	if limit > 0 {
		query += " LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		agent := &domain.Agent{}
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		talksToJSON := make([]byte, 0)
		metadataJSON := make([]byte, 0)
		declaredPurposeJSON := make([]byte, 0)
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
			&declaredPurposeJSON,
		)
		if err != nil {
			return nil, 0, err
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
			return nil, 0, err
		}

		// Unmarshal metadata from JSONB
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Unmarshal declared_purpose from JSONB
		if len(declaredPurposeJSON) > 0 {
			if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
			}
		}

		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	total := len(agents)
	if limit > 0 {
		if err := r.db.QueryRow(`SELECT COUNT(*) FROM agents WHERE organization_id = $1`, orgID).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	return agents, total, nil
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
		    previous_public_key = $21, rotation_count = $22,
		    pqc_public_key = $23, pqc_key_algorithm = $24, hybrid_mode_enabled = $25,
		    pqc_key_created_at = $26, pqc_key_expires_at = $27, previous_pqc_public_key = $28,
		    declared_purpose = $29
		WHERE id = $30
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

	// Marshal declared_purpose to JSONB; SQL NULL (untyped nil) when not declared.
	var declaredPurposeParam interface{}
	if agent.DeclaredPurpose != nil {
		b, mErr := json.Marshal(agent.DeclaredPurpose)
		if mErr != nil {
			return fmt.Errorf("failed to marshal declared_purpose: %w", mErr)
		}
		declaredPurposeParam = b
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
		agent.PQCPublicKey,
		agent.PQCKeyAlgorithm,
		agent.HybridModeEnabled,
		agent.PQCKeyCreatedAt,
		agent.PQCKeyExpiresAt,
		agent.PreviousPQCPublicKey,
		declaredPurposeParam,
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

// ListRevokedIDs returns one page of revoked agent ids, newest first.
//
// The revocation list is served on an unauthenticated route, so the `status = 'revoked'`
// predicate has to run in SQL. Filtering List's results in Go instead means every request
// reads and materialises every agent row — twenty-four columns including three JSONB
// ones — to emit the revoked subset. At 20,000 agents that was hundreds of milliseconds
// per request (~320ms and ~630ms in two measurements of differently-shaped rows), which
// an unauthenticated caller controls the cost of. The cost scales with the table rather
// than with the number of revocations, which is the property that matters; the exact
// figure is row-shape dependent and is not pinned here because nothing can check it.
//
// Only the id is selected. Nothing else on the row belongs on a public CRL, and selecting
// less also means this method cannot be broken by a nullable column it does not read.
func (r *AgentRepository) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	if limit <= 0 {
		return nil, ErrNonPositiveLimit
	}

	// (created_at, id) is a total order, so an offset walk cannot skip or repeat rows
	// that share a timestamp.
	const query = `
		SELECT id
		FROM agents
		WHERE status = $1
		ORDER BY created_at DESC, id
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, string(domain.AgentStatusRevoked), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// ErrNonPositiveLimit is returned by List when it is asked for a page of zero or
// fewer rows. See List for why this is an error rather than an interpretation.
var ErrNonPositiveLimit = errors.New("agent repository: limit must be positive; pass an explicit page size")

// List returns one page of agents. limit MUST be positive.
//
// This method previously appended `LIMIT $1 OFFSET $2` unconditionally, so a zero limit
// reached Postgres as `LIMIT 0` and returned no rows at all. Both callers passed (0, 0)
// meaning "all" — one of them says so in a comment — because the sibling method
// GetByOrganizationPaged above documents exactly that convention. The two conventions
// collided here, and the revocation endpoint that reads this method served an empty list
// to every caller as a result.
//
// Adopting the sibling's "non-positive means all" convention would have fixed that and is
// the obvious repair, but it trades a silent-empty bug for a silent-everything one: an
// unbounded full-table scan behind whichever caller next passes a zero, including an
// unauthenticated route. Both failure directions are silent. Erroring is the only option
// that is neither, and it fails visibly, which is the property the empty list lacked.
func (r *AgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	if limit <= 0 {
		return nil, ErrNonPositiveLimit
	}

	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, COALESCE(trust_score, 0), verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(created_by_name, ''), COALESCE(created_by_email, ''),
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
		FROM agents
		ORDER BY created_at DESC, id
		LIMIT $1 OFFSET $2
	`

	// `id` breaks ties in the sort. Without it `created_at DESC` alone is not a total
	// order, and rows sharing a timestamp can land on both sides of a page boundary or
	// on neither — a caller walking offsets silently skips or repeats them. That matters
	// now that the revocation list pages through this method.
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		agent := &domain.Agent{}
		// `description` is nullable and has to be scanned through a NullString, exactly as
		// GetByOrganizationPaged does above. Scanning it straight into a string fails the
		// entire row with "converting NULL to string is unsupported". That could not fire
		// while the LIMIT bug held, because no row was ever scanned — fixing the limit
		// without this turns the empty list into a 500 for any deployment holding a single
		// agent with no description.
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		talksToJSON := make([]byte, 0)
		metadataJSON := make([]byte, 0)
		declaredPurposeJSON := make([]byte, 0)
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
			&declaredPurposeJSON,
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

		// Unmarshal declared_purpose from JSONB
		if len(declaredPurposeJSON) > 0 {
			if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
				return nil, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
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
	// Query agents where talks_to JSONB array contains an object with matching id
	// The talks_to field can be:
	// 1. An array of strings: ["uuid1", "uuid2"]
	// 2. An array of objects: [{"id": "uuid1", "name": "server-name"}]
	// We need to check for both formats
	query := `
		SELECT id, organization_id, name, display_name, description, agent_type, status, version, public_key,
		       certificate_url, repository_url, documentation_url, COALESCE(trust_score, 0), verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
		FROM agents
		WHERE organization_id = $1
		  AND (
		    talks_to @> $2::jsonb
		    OR talks_to @> $3::jsonb
		  )
		ORDER BY created_at DESC, id
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

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		agent := &domain.Agent{}
		// `description` is nullable; scanning it into a plain string fails the whole row.
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		talksToJSON := make([]byte, 0)
		metadataJSON := make([]byte, 0)
		declaredPurposeJSON := make([]byte, 0)
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
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
			&declaredPurposeJSON,
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

		// Unmarshal declared_purpose from JSONB
		if len(declaredPurposeJSON) > 0 {
			if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
				return nil, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
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
		       certificate_url, repository_url, documentation_url, COALESCE(trust_score, 0), verified_at,
		       talks_to, metadata, created_at, updated_at, created_by,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
		FROM agents
		WHERE organization_id = $1
		  AND (
		    talks_to @> $2::jsonb
		    OR talks_to @> $3::jsonb
		  )
		ORDER BY created_at DESC, id
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

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		agent := &domain.Agent{}
		// `description` is nullable; scanning it into a plain string fails the whole row.
		var description sql.NullString
		var version sql.NullString
		var publicKey sql.NullString
		var certificateURL sql.NullString
		var repositoryURL sql.NullString
		var documentationURL sql.NullString
		talksToJSON := make([]byte, 0)
		metadataJSON := make([]byte, 0)
		declaredPurposeJSON := make([]byte, 0)
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
			&agent.CapabilityViolationCount,
			&agent.IsCompromised,
			&declaredPurposeJSON,
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

		// Unmarshal declared_purpose from JSONB
		if len(declaredPurposeJSON) > 0 {
			if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
				return nil, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
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
		       public_key, certificate_url, repository_url, documentation_url, COALESCE(trust_score, 0), verified_at,
		       created_at, updated_at, created_by, encrypted_private_key, key_algorithm,
		       key_created_at, key_expires_at, key_rotation_grace_until, previous_public_key, rotation_count,
		       talks_to, capabilities, metadata,
		       COALESCE(capability_violation_count, 0), COALESCE(is_compromised, false), declared_purpose
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
	// `description` is nullable; scanning it into a plain string fails the whole row.
	var description sql.NullString
	talksToJSON := make([]byte, 0)
	capabilitiesJSON := make([]byte, 0)
	metadataJSON := make([]byte, 0)
	declaredPurposeJSON := make([]byte, 0)

	err := r.db.QueryRow(query, orgID, name).Scan(
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
		&declaredPurposeJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
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

	// Unmarshal declared_purpose from JSONB
	if len(declaredPurposeJSON) > 0 {
		if err := json.Unmarshal(declaredPurposeJSON, &agent.DeclaredPurpose); err != nil {
			return nil, fmt.Errorf("failed to unmarshal declared_purpose: %w", err)
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

// UpdateHeartbeat updates the last heartbeat timestamp for an agent
func (r *AgentRepository) UpdateHeartbeat(ctx context.Context, agentID uuid.UUID) error {
	query := `UPDATE agents SET last_heartbeat = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, agentID)
	return err
}

// GetStaleAgents returns agents whose heartbeat is older than the given time
func (r *AgentRepository) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) {
	query := `
		SELECT id, organization_id, name, display_name, status, agent_type, COALESCE(trust_score, 0),
		       last_heartbeat, last_active, created_at, updated_at
		FROM agents
		WHERE last_heartbeat IS NOT NULL
		  AND last_heartbeat < $1
		  AND status NOT IN ('revoked', 'suspended')
		ORDER BY last_heartbeat ASC`
	rows, err := r.db.QueryContext(ctx, query, staleSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		a := &domain.Agent{}
		var lastHeartbeat sql.NullTime
		var lastActive sql.NullTime
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &a.DisplayName, &a.Status,
			&a.AgentType, &a.TrustScore, &lastHeartbeat, &lastActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if lastHeartbeat.Valid {
			a.LastHeartbeat = &lastHeartbeat.Time
		}
		if lastActive.Valid {
			a.LastActive = &lastActive.Time
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// GetByIDs returns agents matching the given IDs
// GetByIDs returns the agents among ids that belong to callerOrgID.
//
// SECURITY: callerOrgID is REQUIRED and the predicate runs in SQL. This query was
// `FROM agents WHERE id IN (...)` with no organization filter while selecting
// organization_id, and its only caller — POST /api/v1/agents/bulk-status — returned status,
// trust score and last-active for whatever ids the CALLER put in the request body. Any
// authenticated user could read those fields for any agent in any organization by naming
// its UUID.
//
// Filtering after the load would be worse than useless here: it would still pull other
// organizations' rows into this process, and the next caller to forget the filter gets the
// unfiltered set. Ids belonging to another organization are simply absent from the result,
// which is also the correct answer for an id that does not exist — a caller cannot tell the
// two apart, so this does not become an existence oracle.
func (r *AgentRepository) GetByIDs(ctx context.Context, callerOrgID uuid.UUID, ids []uuid.UUID) ([]*domain.Agent, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// Fail closed: a uuid.Nil caller org means the auth chain did not populate it and the
	// handler proceeded anyway. Matching Nil against real rows would disclose them.
	if callerOrgID == uuid.Nil {
		return nil, fmt.Errorf("agent lookup requires a caller organization")
	}
	// Build query with proper placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	args = append(args, callerOrgID)
	query := fmt.Sprintf(`
		SELECT id, organization_id, name, display_name, status, agent_type, COALESCE(trust_score, 0),
		       last_heartbeat, last_active, created_at, updated_at
		FROM agents WHERE id IN (%s) AND organization_id = $%d`,
		strings.Join(placeholders, ","), len(ids)+1)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]*domain.Agent, 0)
	for rows.Next() {
		a := &domain.Agent{}
		var lastHeartbeat sql.NullTime
		var lastActive sql.NullTime
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &a.DisplayName, &a.Status,
			&a.AgentType, &a.TrustScore, &lastHeartbeat, &lastActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if lastHeartbeat.Valid {
			a.LastHeartbeat = &lastHeartbeat.Time
		}
		if lastActive.Valid {
			a.LastActive = &lastActive.Time
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
