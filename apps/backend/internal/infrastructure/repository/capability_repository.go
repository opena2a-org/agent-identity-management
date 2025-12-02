package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/domain"
)

// CapabilityRepositoryPostgres implements CapabilityRepository using PostgreSQL
type CapabilityRepositoryPostgres struct {
	db *sqlx.DB
}

// NewCapabilityRepository creates a new capability repository
func NewCapabilityRepository(db *sqlx.DB) *CapabilityRepositoryPostgres {
	return &CapabilityRepositoryPostgres{db: db}
}

// CreateCapability creates a new agent capability
func (r *CapabilityRepositoryPostgres) CreateCapability(capability *domain.AgentCapability) error {
	scopeJSON, _ := json.Marshal(capability.CapabilityScope)

	query := `
		INSERT INTO agent_capabilities (
			id, agent_id, capability_type, capability_scope, granted_by, granted_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	capability.ID = uuid.New()
	capability.CreatedAt = time.Now()
	capability.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		capability.ID,
		capability.AgentID,
		capability.CapabilityType,
		scopeJSON,
		capability.GrantedBy,
		capability.GrantedAt,
		capability.CreatedAt,
		capability.UpdatedAt,
	)

	return err
}

// GetCapabilityByID retrieves a capability by ID
func (r *CapabilityRepositoryPostgres) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	query := `
		SELECT id, agent_id, capability_type, capability_scope, granted_by, granted_at, revoked_at, created_at, updated_at
		FROM agent_capabilities
		WHERE id = $1
	`

	var capability domain.AgentCapability
	var scopeJSON []byte
	var grantedBy uuid.NullUUID
	var revokedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&capability.ID,
		&capability.AgentID,
		&capability.CapabilityType,
		&scopeJSON,
		&grantedBy,
		&capability.GrantedAt,
		&revokedAt,
		&capability.CreatedAt,
		&capability.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if grantedBy.Valid {
		capability.GrantedBy = &grantedBy.UUID
	}
	if revokedAt.Valid {
		capability.RevokedAt = &revokedAt.Time
	}
	if len(scopeJSON) > 0 {
		json.Unmarshal(scopeJSON, &capability.CapabilityScope)
	}

	return &capability, nil
}

// GetCapabilitiesByAgentID retrieves all capabilities for an agent
func (r *CapabilityRepositoryPostgres) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	query := `
		SELECT id, agent_id, capability_type, capability_scope, granted_by, granted_at, revoked_at, created_at, updated_at
		FROM agent_capabilities
		WHERE agent_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var capabilities []*domain.AgentCapability
	for rows.Next() {
		var capability domain.AgentCapability
		var scopeJSON []byte
		var grantedBy uuid.NullUUID
		var revokedAt sql.NullTime

		err := rows.Scan(
			&capability.ID,
			&capability.AgentID,
			&capability.CapabilityType,
			&scopeJSON,
			&grantedBy,
			&capability.GrantedAt,
			&revokedAt,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if grantedBy.Valid {
			capability.GrantedBy = &grantedBy.UUID
		}
		if revokedAt.Valid {
			capability.RevokedAt = &revokedAt.Time
		}
		if len(scopeJSON) > 0 {
			json.Unmarshal(scopeJSON, &capability.CapabilityScope)
		}

		capabilities = append(capabilities, &capability)
	}

	return capabilities, nil
}

// GetActiveCapabilitiesByAgentID retrieves only non-revoked capabilities
func (r *CapabilityRepositoryPostgres) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	query := `
		SELECT id, agent_id, capability_type, capability_scope, granted_by, granted_at, revoked_at, created_at, updated_at
		FROM agent_capabilities
		WHERE agent_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var capabilities []*domain.AgentCapability
	for rows.Next() {
		var capability domain.AgentCapability
		var scopeJSON []byte
		var grantedBy uuid.NullUUID
		var revokedAt sql.NullTime

		err := rows.Scan(
			&capability.ID,
			&capability.AgentID,
			&capability.CapabilityType,
			&scopeJSON,
			&grantedBy,
			&capability.GrantedAt,
			&revokedAt,
			&capability.CreatedAt,
			&capability.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if grantedBy.Valid {
			capability.GrantedBy = &grantedBy.UUID
		}
		if revokedAt.Valid {
			capability.RevokedAt = &revokedAt.Time
		}
		if len(scopeJSON) > 0 {
			json.Unmarshal(scopeJSON, &capability.CapabilityScope)
		}

		capabilities = append(capabilities, &capability)
	}

	return capabilities, nil
}

// RevokeCapability marks a capability as revoked
func (r *CapabilityRepositoryPostgres) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	query := `
		UPDATE agent_capabilities
		SET revoked_at = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, revokedAt, time.Now(), id)
	return err
}

// DeleteCapability removes a capability
func (r *CapabilityRepositoryPostgres) DeleteCapability(id uuid.UUID) error {
	query := `DELETE FROM agent_capabilities WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// CreateViolation creates a new capability violation record
func (r *CapabilityRepositoryPostgres) CreateViolation(violation *domain.CapabilityViolation) error {
	registeredJSON, _ := json.Marshal(violation.RegisteredCapabilities)
	metadataJSON, _ := json.Marshal(violation.RequestMetadata)

	query := `
		INSERT INTO capability_violations (
			id, agent_id, attempted_capability, registered_capabilities,
			severity, trust_score_impact, is_blocked, source_ip, request_metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	violation.ID = uuid.New()
	violation.CreatedAt = time.Now()

	_, err := r.db.Exec(query,
		violation.ID,
		violation.AgentID,
		violation.AttemptedCapability,
		registeredJSON,
		violation.Severity,
		violation.TrustScoreImpact,
		violation.IsBlocked,
		violation.SourceIP,
		metadataJSON,
		violation.CreatedAt,
	)

	return err
}

// GetViolationByID retrieves a violation by ID
func (r *CapabilityRepositoryPostgres) GetViolationByID(id uuid.UUID) (*domain.CapabilityViolation, error) {
	query := `
		SELECT cv.id, cv.agent_id, a.display_name as agent_name, cv.attempted_capability,
			cv.registered_capabilities, cv.severity, cv.trust_score_impact,
			cv.is_blocked, cv.source_ip, cv.request_metadata, cv.created_at
		FROM capability_violations cv
		LEFT JOIN agents a ON cv.agent_id = a.id
		WHERE cv.id = $1
	`

	var violation domain.CapabilityViolation
	var registeredJSON, metadataJSON []byte
	var agentName sql.NullString
	var sourceIP sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&violation.ID,
		&violation.AgentID,
		&agentName,
		&violation.AttemptedCapability,
		&registeredJSON,
		&violation.Severity,
		&violation.TrustScoreImpact,
		&violation.IsBlocked,
		&sourceIP,
		&metadataJSON,
		&violation.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if agentName.Valid {
		violation.AgentName = &agentName.String
	}
	if sourceIP.Valid {
		violation.SourceIP = &sourceIP.String
	}
	if len(registeredJSON) > 0 {
		json.Unmarshal(registeredJSON, &violation.RegisteredCapabilities)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &violation.RequestMetadata)
	}

	return &violation, nil
}

// GetViolationsByAgentID retrieves violations for a specific agent
func (r *CapabilityRepositoryPostgres) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM capability_violations WHERE agent_id = $1`
	r.db.QueryRow(countQuery, agentID).Scan(&total)

	// Get violations
	query := `
		SELECT cv.id, cv.agent_id, a.display_name as agent_name, cv.attempted_capability,
			cv.registered_capabilities, cv.severity, cv.trust_score_impact,
			cv.is_blocked, cv.source_ip, cv.request_metadata, cv.created_at
		FROM capability_violations cv
		LEFT JOIN agents a ON cv.agent_id = a.id
		WHERE cv.agent_id = $1
		ORDER BY cv.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, agentID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	violations := r.scanViolations(rows)
	return violations, total, nil
}

// GetRecentViolations retrieves violations from the last N minutes
func (r *CapabilityRepositoryPostgres) GetRecentViolations(orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	query := `
		SELECT cv.id, cv.agent_id, a.display_name as agent_name, cv.attempted_capability,
			cv.registered_capabilities, cv.severity, cv.trust_score_impact,
			cv.is_blocked, cv.source_ip, cv.request_metadata, cv.created_at
		FROM capability_violations cv
		LEFT JOIN agents a ON cv.agent_id = a.id
		WHERE a.organization_id = $1
		AND cv.created_at >= NOW() - INTERVAL '1 minute' * $2
		ORDER BY cv.created_at DESC
	`

	rows, err := r.db.Query(query, orgID, minutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanViolations(rows), nil
}

// GetViolationsByOrganization retrieves all violations for an organization
func (r *CapabilityRepositoryPostgres) GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	// Get total count
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM capability_violations cv
		JOIN agents a ON cv.agent_id = a.id
		WHERE a.organization_id = $1
	`
	r.db.QueryRow(countQuery, orgID).Scan(&total)

	// Get violations
	query := `
		SELECT cv.id, cv.agent_id, a.display_name as agent_name, cv.attempted_capability,
			cv.registered_capabilities, cv.severity, cv.trust_score_impact,
			cv.is_blocked, cv.source_ip, cv.request_metadata, cv.created_at
		FROM capability_violations cv
		LEFT JOIN agents a ON cv.agent_id = a.id
		WHERE a.organization_id = $1
		ORDER BY cv.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	violations := r.scanViolations(rows)
	return violations, total, nil
}

// Helper function to scan violation rows
func (r *CapabilityRepositoryPostgres) scanViolations(rows *sql.Rows) []*domain.CapabilityViolation {
	var violations []*domain.CapabilityViolation

	for rows.Next() {
		var violation domain.CapabilityViolation
		var registeredJSON, metadataJSON []byte
		var agentName sql.NullString
		var sourceIP sql.NullString

		rows.Scan(
			&violation.ID,
			&violation.AgentID,
			&agentName,
			&violation.AttemptedCapability,
			&registeredJSON,
			&violation.Severity,
			&violation.TrustScoreImpact,
			&violation.IsBlocked,
			&sourceIP,
			&metadataJSON,
			&violation.CreatedAt,
		)

		if agentName.Valid {
			violation.AgentName = &agentName.String
		}
		if sourceIP.Valid {
			violation.SourceIP = &sourceIP.String
		}
		if len(registeredJSON) > 0 {
			json.Unmarshal(registeredJSON, &violation.RegisteredCapabilities)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &violation.RequestMetadata)
		}

		violations = append(violations, &violation)
	}

	return violations
}

// ListCapabilityDefinitions returns all capability definitions (core + org-specific)
func (r *CapabilityRepositoryPostgres) ListCapabilityDefinitions(orgID *uuid.UUID) ([]*domain.CapabilityDefinition, error) {
	query := `
		SELECT id, namespace, action, capability_type, risk_level, display_name,
			   description, category, organization_id, created_at, updated_at
		FROM capability_definitions
		WHERE organization_id IS NULL
		   OR organization_id = $1
		ORDER BY capability_type DESC, namespace, action
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var definitions []*domain.CapabilityDefinition
	for rows.Next() {
		def, err := r.scanCapabilityDefinition(rows)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, def)
	}

	return definitions, nil
}

// GetCapabilityDefinition retrieves a specific capability definition
func (r *CapabilityRepositoryPostgres) GetCapabilityDefinition(namespace, action string, orgID *uuid.UUID) (*domain.CapabilityDefinition, error) {
	query := `
		SELECT id, namespace, action, capability_type, risk_level, display_name,
			   description, category, organization_id, created_at, updated_at
		FROM capability_definitions
		WHERE namespace = $1 AND action = $2
		  AND (organization_id IS NULL OR organization_id = $3)
		ORDER BY organization_id NULLS LAST
		LIMIT 1
	`

	row := r.db.QueryRow(query, namespace, action, orgID)

	var def domain.CapabilityDefinition
	var description, category sql.NullString
	var organizationID uuid.NullUUID

	err := row.Scan(
		&def.ID,
		&def.Namespace,
		&def.Action,
		&def.CapabilityType,
		&def.RiskLevel,
		&def.DisplayName,
		&description,
		&category,
		&organizationID,
		&def.CreatedAt,
		&def.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if description.Valid {
		def.Description = description.String
	}
	if category.Valid {
		def.Category = category.String
	}
	if organizationID.Valid {
		def.OrganizationID = &organizationID.UUID
	}

	return &def, nil
}

// CreateCapabilityDefinition creates a new capability definition
func (r *CapabilityRepositoryPostgres) CreateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	query := `
		INSERT INTO capability_definitions (
			id, namespace, action, capability_type, risk_level, display_name,
			description, category, organization_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (namespace, action, organization_id) DO NOTHING
	`

	def.ID = uuid.New()
	def.CreatedAt = time.Now()
	def.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		def.ID,
		def.Namespace,
		def.Action,
		def.CapabilityType,
		def.RiskLevel,
		def.DisplayName,
		def.Description,
		def.Category,
		def.OrganizationID,
		def.CreatedAt,
		def.UpdatedAt,
	)

	return err
}

// UpdateCapabilityDefinition updates an existing capability definition
func (r *CapabilityRepositoryPostgres) UpdateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	query := `
		UPDATE capability_definitions
		SET risk_level = $1, display_name = $2, description = $3,
			category = $4, updated_at = $5
		WHERE id = $6
	`

	def.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		def.RiskLevel,
		def.DisplayName,
		def.Description,
		def.Category,
		def.UpdatedAt,
		def.ID,
	)

	return err
}

// Helper function to scan a capability definition row
func (r *CapabilityRepositoryPostgres) scanCapabilityDefinition(rows *sql.Rows) (*domain.CapabilityDefinition, error) {
	var def domain.CapabilityDefinition
	var description, category sql.NullString
	var organizationID uuid.NullUUID

	err := rows.Scan(
		&def.ID,
		&def.Namespace,
		&def.Action,
		&def.CapabilityType,
		&def.RiskLevel,
		&def.DisplayName,
		&description,
		&category,
		&organizationID,
		&def.CreatedAt,
		&def.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if description.Valid {
		def.Description = description.String
	}
	if category.Valid {
		def.Category = category.String
	}
	if organizationID.Valid {
		def.OrganizationID = &organizationID.UUID
	}

	return &def, nil
}
