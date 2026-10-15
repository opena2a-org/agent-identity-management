package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A2ADelegationRepository stores delegations between agents in the
// a2a_delegation table.
type A2ADelegationRepository struct {
	db *sql.DB
}

var _ domain.A2ADelegationRepository = (*A2ADelegationRepository)(nil)

// NewA2ADelegationRepository creates a new A2ADelegationRepository
func NewA2ADelegationRepository(db *sql.DB) *A2ADelegationRepository {
	return &A2ADelegationRepository{db: db}
}

// activeA2ADelegationIndex is the partial unique index that allows one active
// delegation per delegator, delegate and organization (migration 070).
const activeA2ADelegationIndex = "idx_a2a_delegation_unique"

// a2aDelegationColumns is the column list every query returns, in the order
// scanA2ADelegation reads it. The table leaves several columns nullable; a
// NULL reads as the value that grants nothing: no chain depth, no further
// delegation, not active.
const a2aDelegationColumns = `id, organization_id, delegator_agent_id, delegate_agent_id,
		COALESCE(skill_ids, '[]'::jsonb), COALESCE(max_chain_depth, 0),
		trust_attenuation_factor, min_delegated_trust_score,
		COALESCE(can_delegate_further, FALSE), COALESCE(requires_consent, TRUE),
		COALESCE(is_active, FALSE), valid_from, valid_until, created_by, created_at, updated_at`

// Create stores the delegation and fills it from the stored row.
func (r *A2ADelegationRepository) Create(ctx context.Context, d *domain.A2ADelegation) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	skills := d.SkillIDs
	if skills == nil {
		skills = []string{}
	}
	skillsJSON, err := json.Marshal(skills)
	if err != nil {
		return fmt.Errorf("failed to encode delegation skills: %w", err)
	}
	var createdBy uuid.NullUUID
	if d.CreatedBy != nil {
		createdBy = uuid.NullUUID{UUID: *d.CreatedBy, Valid: true}
	}

	query := `
		INSERT INTO a2a_delegation (
			id, organization_id, delegator_agent_id, delegate_agent_id, skill_ids, max_chain_depth,
			trust_attenuation_factor, min_delegated_trust_score, can_delegate_further, is_active,
			valid_from, valid_until, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING ` + a2aDelegationColumns

	stored, err := scanA2ADelegation(r.db.QueryRowContext(ctx, query,
		d.ID, d.OrganizationID, d.DelegatorAgentID, d.DelegateAgentID, skillsJSON, d.MaxChainDepth,
		d.TrustAttenuationFactor, d.MinDelegatedTrustScore, d.CanDelegateFurther, d.IsActive,
		d.ValidFrom, nullTime(d.ValidUntil), createdBy,
	))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == activeA2ADelegationIndex {
			return domain.ErrA2ADelegationActive
		}
		return fmt.Errorf("failed to create delegation: %w", err)
	}
	*d = *stored
	return nil
}

// GetByIDs returns the organization's delegations among ids. The organization
// predicate is in the query: another organization's delegation is absent from
// the result, as one that does not exist is.
func (r *A2ADelegationRepository) GetByIDs(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) ([]*domain.A2ADelegation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("delegation lookup requires a caller organization")
	}
	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}
	query := `SELECT ` + a2aDelegationColumns + `
		FROM a2a_delegation
		WHERE organization_id = $1 AND id = ANY($2::uuid[])`
	return r.queryA2ADelegations(ctx, query, orgID, pq.Array(idStrings))
}

// ListByOrganization returns the organization's delegations, newest first.
func (r *A2ADelegationRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ADelegation, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("delegation list requires a caller organization")
	}
	query := `SELECT ` + a2aDelegationColumns + `
		FROM a2a_delegation
		WHERE organization_id = $1
		ORDER BY created_at DESC, id
		LIMIT $2 OFFSET $3`
	return r.queryA2ADelegations(ctx, query, orgID, limit, offset)
}

// Revoke marks the organization's delegation inactive and returns it. A
// delegation that is already inactive is returned unchanged but for
// updated_at.
func (r *A2ADelegationRepository) Revoke(ctx context.Context, orgID, id uuid.UUID) (*domain.A2ADelegation, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("delegation revoke requires a caller organization")
	}
	query := `
		UPDATE a2a_delegation
		SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND organization_id = $2
		RETURNING ` + a2aDelegationColumns
	d, err := scanA2ADelegation(r.db.QueryRowContext(ctx, query, id, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrA2ADelegationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to revoke delegation: %w", err)
	}
	return d, nil
}

func (r *A2ADelegationRepository) queryA2ADelegations(ctx context.Context, query string, args ...interface{}) ([]*domain.A2ADelegation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to read delegations: %w", err)
	}
	defer rows.Close()

	delegations := make([]*domain.A2ADelegation, 0)
	for rows.Next() {
		d, err := scanA2ADelegation(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to read delegation: %w", err)
		}
		delegations = append(delegations, d)
	}
	return delegations, rows.Err()
}

// a2aDelegationRow is a *sql.Row or *sql.Rows.
type a2aDelegationRow interface {
	Scan(dest ...interface{}) error
}

// scanA2ADelegation reads one row of a2aDelegationColumns.
func scanA2ADelegation(row a2aDelegationRow) (*domain.A2ADelegation, error) {
	d := &domain.A2ADelegation{}
	var skillsJSON []byte
	var validUntil, createdAt, updatedAt sql.NullTime
	var createdBy uuid.NullUUID
	if err := row.Scan(
		&d.ID, &d.OrganizationID, &d.DelegatorAgentID, &d.DelegateAgentID,
		&skillsJSON, &d.MaxChainDepth,
		&d.TrustAttenuationFactor, &d.MinDelegatedTrustScore,
		&d.CanDelegateFurther, &d.RequiresConsent,
		&d.IsActive, &d.ValidFrom, &validUntil, &createdBy, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(skillsJSON, &d.SkillIDs); err != nil {
		return nil, fmt.Errorf("delegation %s: skill_ids is not a list of strings: %w", d.ID, err)
	}
	if validUntil.Valid {
		t := validUntil.Time
		d.ValidUntil = &t
	}
	if createdBy.Valid {
		id := createdBy.UUID
		d.CreatedBy = &id
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	return d, nil
}
