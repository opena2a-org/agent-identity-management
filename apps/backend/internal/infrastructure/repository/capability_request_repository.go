package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/domain"
)

type capabilityRequestRepository struct {
	db *sqlx.DB
}

// NewCapabilityRequestRepository creates a new PostgreSQL capability request repository
func NewCapabilityRequestRepository(db *sqlx.DB) domain.CapabilityRequestRepository {
	return &capabilityRequestRepository{db: db}
}

func (r *capabilityRequestRepository) Create(req *domain.CapabilityRequest) error {
	query := `
		INSERT INTO capability_requests (
			id, agent_id, capability_type, reason, status,
			requested_by, requested_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	now := time.Now()
	req.ID = uuid.New()
	req.CreatedAt = now
	req.UpdatedAt = now
	req.RequestedAt = now
	req.Status = domain.CapabilityRequestStatusPending

	_, err := r.db.Exec(
		query,
		req.ID,
		req.AgentID,
		req.CapabilityType,
		req.Reason,
		req.Status,
		req.RequestedBy,
		req.RequestedAt,
		req.CreatedAt,
		req.UpdatedAt,
	)

	return err
}

func (r *capabilityRequestRepository) GetByID(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
	query := `
		SELECT
			cr.id,
			cr.agent_id,
			cr.capability_type,
			cr.reason,
			cr.status,
			cr.requested_by,
			cr.reviewed_by,
			cr.requested_at,
			cr.reviewed_at,
			cr.created_at,
			cr.updated_at,
			a.name AS agent_name,
			a.display_name AS agent_display_name,
			u1.email AS requested_by_email,
			u2.email AS reviewed_by_email
		FROM capability_requests cr
		INNER JOIN agents a ON cr.agent_id = a.id
		INNER JOIN users u1 ON cr.requested_by = u1.id
		LEFT JOIN users u2 ON cr.reviewed_by = u2.id
		WHERE cr.id = $1
	`

	var req domain.CapabilityRequestWithDetails
	err := r.db.Get(&req, query, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("capability request not found")
	}
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (r *capabilityRequestRepository) List(filter domain.CapabilityRequestFilter) ([]*domain.CapabilityRequestWithDetails, error) {
	query := `
		SELECT
			cr.id,
			cr.agent_id,
			cr.capability_type,
			cr.reason,
			cr.status,
			cr.requested_by,
			cr.reviewed_by,
			cr.requested_at,
			cr.reviewed_at,
			cr.created_at,
			cr.updated_at,
			a.name AS agent_name,
			a.display_name AS agent_display_name,
			u1.email AS requested_by_email,
			u2.email AS reviewed_by_email
		FROM capability_requests cr
		INNER JOIN agents a ON cr.agent_id = a.id
		INNER JOIN users u1 ON cr.requested_by = u1.id
		LEFT JOIN users u2 ON cr.reviewed_by = u2.id
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	// Apply filters
	if filter.OrganizationID != nil {
		query += fmt.Sprintf(" AND a.organization_id = $%d", argPos)
		args = append(args, *filter.OrganizationID)
		argPos++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND cr.status = $%d", argPos)
		args = append(args, *filter.Status)
		argPos++
	}

	if filter.AgentID != nil {
		query += fmt.Sprintf(" AND cr.agent_id = $%d", argPos)
		args = append(args, *filter.AgentID)
		argPos++
	}

	// Order by requested_at DESC (newest first)
	query += " ORDER BY cr.requested_at DESC"

	// Apply pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Offset)
	}

	var requests []*domain.CapabilityRequestWithDetails
	err := r.db.Select(&requests, query, args...)
	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *capabilityRequestRepository) UpdateStatus(id uuid.UUID, status domain.CapabilityRequestStatus, reviewedBy uuid.UUID) error {
	query := `
		UPDATE capability_requests
		SET
			status = $1,
			reviewed_by = $2,
			reviewed_at = $3,
			updated_at = $4
		WHERE id = $5
	`

	now := time.Now()
	result, err := r.db.Exec(query, status, reviewedBy, now, now, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("capability request not found")
	}

	return nil
}

func (r *capabilityRequestRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM capability_requests WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("capability request not found")
	}

	return nil
}
