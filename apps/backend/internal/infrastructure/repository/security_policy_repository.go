package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// SecurityPolicyRepository implements domain.SecurityPolicyRepository
type SecurityPolicyRepository struct {
	db *sql.DB
}

// NewSecurityPolicyRepository creates a new security policy repository
func NewSecurityPolicyRepository(db *sql.DB) *SecurityPolicyRepository {
	return &SecurityPolicyRepository{db: db}
}

// Create creates a new security policy
func (r *SecurityPolicyRepository) Create(policy *domain.SecurityPolicy) error {
	query := `
		INSERT INTO security_policies (id, organization_id, name, description, policy_type, enforcement_action, severity_threshold, rules, applies_to, is_enabled, priority, created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	if policy.UpdatedAt.IsZero() {
		policy.UpdatedAt = time.Now()
	}

	// Marshal rules to JSON
	rulesJSON, err := json.Marshal(policy.Rules)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(query,
		policy.ID,
		policy.OrganizationID,
		policy.Name,
		policy.Description,
		policy.PolicyType,
		policy.EnforcementAction,
		policy.SeverityThreshold,
		rulesJSON,
		policy.AppliesTo,
		policy.IsEnabled,
		policy.Priority,
		policy.CreatedAt,
		policy.UpdatedAt,
		policy.CreatedBy,
	)
	return err
}

// GetByID retrieves a security policy by ID
func (r *SecurityPolicyRepository) GetByID(id uuid.UUID) (*domain.SecurityPolicy, error) {
	query := `
		SELECT id, organization_id, name, description, policy_type, enforcement_action, severity_threshold, rules, applies_to, is_enabled, priority, created_at, updated_at, created_by
		FROM security_policies
		WHERE id = $1
	`

	var policy domain.SecurityPolicy
	var rulesJSON []byte

	err := r.db.QueryRow(query, id).Scan(
		&policy.ID,
		&policy.OrganizationID,
		&policy.Name,
		&policy.Description,
		&policy.PolicyType,
		&policy.EnforcementAction,
		&policy.SeverityThreshold,
		&rulesJSON,
		&policy.AppliesTo,
		&policy.IsEnabled,
		&policy.Priority,
		&policy.CreatedAt,
		&policy.UpdatedAt,
		&policy.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal rules JSON
	if err := json.Unmarshal(rulesJSON, &policy.Rules); err != nil {
		policy.Rules = make(map[string]interface{})
	}

	return &policy, nil
}

// GetByOrganization retrieves all security policies for an organization
func (r *SecurityPolicyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	query := `
		SELECT id, organization_id, name, description, policy_type, enforcement_action, severity_threshold, rules, applies_to, is_enabled, priority, created_at, updated_at, created_by
		FROM security_policies
		WHERE organization_id = $1
		ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []*domain.SecurityPolicy{}
	for rows.Next() {
		var policy domain.SecurityPolicy
		var rulesJSON []byte

		if err := rows.Scan(
			&policy.ID,
			&policy.OrganizationID,
			&policy.Name,
			&policy.Description,
			&policy.PolicyType,
			&policy.EnforcementAction,
			&policy.SeverityThreshold,
			&rulesJSON,
			&policy.AppliesTo,
			&policy.IsEnabled,
			&policy.Priority,
			&policy.CreatedAt,
			&policy.UpdatedAt,
			&policy.CreatedBy,
		); err != nil {
			return nil, err
		}

		// Unmarshal rules JSON
		if err := json.Unmarshal(rulesJSON, &policy.Rules); err != nil {
			policy.Rules = make(map[string]interface{})
		}

		policies = append(policies, &policy)
	}

	return policies, rows.Err()
}

// GetActiveByOrganization retrieves all active security policies for an organization
func (r *SecurityPolicyRepository) GetActiveByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	query := `
		SELECT id, organization_id, name, description, policy_type, enforcement_action, severity_threshold, rules, applies_to, is_enabled, priority, created_at, updated_at, created_by
		FROM security_policies
		WHERE organization_id = $1 AND is_enabled = true
		ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []*domain.SecurityPolicy{}
	for rows.Next() {
		var policy domain.SecurityPolicy
		var rulesJSON []byte

		if err := rows.Scan(
			&policy.ID,
			&policy.OrganizationID,
			&policy.Name,
			&policy.Description,
			&policy.PolicyType,
			&policy.EnforcementAction,
			&policy.SeverityThreshold,
			&rulesJSON,
			&policy.AppliesTo,
			&policy.IsEnabled,
			&policy.Priority,
			&policy.CreatedAt,
			&policy.UpdatedAt,
			&policy.CreatedBy,
		); err != nil {
			return nil, err
		}

		// Unmarshal rules JSON
		if err := json.Unmarshal(rulesJSON, &policy.Rules); err != nil {
			policy.Rules = make(map[string]interface{})
		}

		policies = append(policies, &policy)
	}

	return policies, rows.Err()
}

// GetByType retrieves security policies by type for an organization
func (r *SecurityPolicyRepository) GetByType(orgID uuid.UUID, policyType domain.PolicyType) ([]*domain.SecurityPolicy, error) {
	query := `
		SELECT id, organization_id, name, description, policy_type, enforcement_action, severity_threshold, rules, applies_to, is_enabled, priority, created_at, updated_at, created_by
		FROM security_policies
		WHERE organization_id = $1 AND policy_type = $2 AND is_enabled = true
		ORDER BY priority DESC, created_at DESC
	`

	rows, err := r.db.Query(query, orgID, policyType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []*domain.SecurityPolicy{}
	for rows.Next() {
		var policy domain.SecurityPolicy
		var rulesJSON []byte

		if err := rows.Scan(
			&policy.ID,
			&policy.OrganizationID,
			&policy.Name,
			&policy.Description,
			&policy.PolicyType,
			&policy.EnforcementAction,
			&policy.SeverityThreshold,
			&rulesJSON,
			&policy.AppliesTo,
			&policy.IsEnabled,
			&policy.Priority,
			&policy.CreatedAt,
			&policy.UpdatedAt,
			&policy.CreatedBy,
		); err != nil {
			return nil, err
		}

		// Unmarshal rules JSON
		if err := json.Unmarshal(rulesJSON, &policy.Rules); err != nil {
			policy.Rules = make(map[string]interface{})
		}

		policies = append(policies, &policy)
	}

	return policies, rows.Err()
}

// Update updates a security policy
func (r *SecurityPolicyRepository) Update(policy *domain.SecurityPolicy) error {
	query := `
		UPDATE security_policies
		SET name = $1, description = $2, policy_type = $3, enforcement_action = $4, severity_threshold = $5, rules = $6, applies_to = $7, is_enabled = $8, priority = $9, updated_at = $10
		WHERE id = $11
	`

	policy.UpdatedAt = time.Now()

	// Marshal rules to JSON
	rulesJSON, err := json.Marshal(policy.Rules)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(query,
		policy.Name,
		policy.Description,
		policy.PolicyType,
		policy.EnforcementAction,
		policy.SeverityThreshold,
		rulesJSON,
		policy.AppliesTo,
		policy.IsEnabled,
		policy.Priority,
		policy.UpdatedAt,
		policy.ID,
	)
	return err
}

// Delete deletes a security policy
func (r *SecurityPolicyRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM security_policies WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}
