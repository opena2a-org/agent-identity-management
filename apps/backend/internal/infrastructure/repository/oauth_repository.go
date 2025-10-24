package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/domain"
)

type OAuthRepositoryPostgres struct {
	db *sqlx.DB
}

func NewOAuthRepositoryPostgres(db *sqlx.DB) *OAuthRepositoryPostgres {
	return &OAuthRepositoryPostgres{db: db}
}

// Registration requests

func (r *OAuthRepositoryPostgres) CreateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	// Note: OAuth columns have been removed from the schema
	// This repository now only handles email/password registrations
	query := `
		INSERT INTO user_registration_requests (
			id, email, first_name, last_name,
			organization_id, status, requested_at,
			password_hash, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		req.ID,
		req.Email,
		req.FirstName,
		req.LastName,
		req.OrganizationID,
		req.Status,
		req.RequestedAt,
		req.PasswordHash,
		req.CreatedAt,
		req.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	return nil
}

func (r *OAuthRepositoryPostgres) GetRegistrationRequest(ctx context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, password_hash, created_at, updated_at
		FROM user_registration_requests
		WHERE id = $1
	`

	var req domain.UserRegistrationRequest

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.PasswordHash,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("registration request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registration request: %w", err)
	}

	return &req, nil
}

// GetRegistrationRequestByOAuth is deprecated and no longer used (OAuth removed)
func (r *OAuthRepositoryPostgres) GetRegistrationRequestByOAuth(
	ctx context.Context,
	provider domain.OAuthProvider,
	providerUserID string,
) (*domain.UserRegistrationRequest, error) {
	// OAuth has been removed - this method is kept for interface compatibility
	// but will always return nil
	return nil, fmt.Errorf("OAuth registration is no longer supported")
}

func (r *OAuthRepositoryPostgres) GetRegistrationRequestByEmail(
	ctx context.Context,
	email string,
) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, password_hash, created_at, updated_at
		FROM user_registration_requests
		WHERE email = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var req domain.UserRegistrationRequest

	err := r.db.QueryRowContext(ctx, query, email, domain.RegistrationStatusPending).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.PasswordHash,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registration request: %w", err)
	}

	return &req, nil
}

// GetRegistrationRequestByEmailAnyStatus retrieves a registration request by email (any status)
func (r *OAuthRepositoryPostgres) GetRegistrationRequestByEmailAnyStatus(
	ctx context.Context,
	email string,
) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, password_hash, created_at, updated_at
		FROM user_registration_requests
		WHERE email = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var req domain.UserRegistrationRequest

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.PasswordHash,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("registration request not found")
	}
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (r *OAuthRepositoryPostgres) ListPendingRegistrationRequests(
	ctx context.Context,
	orgID uuid.UUID,
	limit, offset int,
) ([]*domain.UserRegistrationRequest, int, error) {
	// Count total
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM user_registration_requests
		WHERE status = $1 AND (organization_id = $2 OR organization_id IS NULL)
	`
	if err := r.db.QueryRowContext(ctx, countQuery, domain.RegistrationStatusPending, orgID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count registration requests: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, email, first_name, last_name,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, password_hash, created_at, updated_at
		FROM user_registration_requests
		WHERE status = $1 AND (organization_id = $2 OR organization_id IS NULL)
		ORDER BY requested_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.QueryContext(ctx, query, domain.RegistrationStatusPending, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list registration requests: %w", err)
	}
	defer rows.Close()

	var requests []*domain.UserRegistrationRequest
	for rows.Next() {
		var req domain.UserRegistrationRequest

		err := rows.Scan(
			&req.ID,
			&req.Email,
			&req.FirstName,
			&req.LastName,
			&req.OrganizationID,
			&req.Status,
			&req.RequestedAt,
			&req.ReviewedAt,
			&req.ReviewedBy,
			&req.RejectionReason,
			&req.PasswordHash,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan registration request: %w", err)
		}

		requests = append(requests, &req)
	}

	return requests, total, nil
}

func (r *OAuthRepositoryPostgres) UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	query := `
		UPDATE user_registration_requests
		SET status = $1, reviewed_at = $2, reviewed_by = $3,
			rejection_reason = $4, updated_at = $5
		WHERE id = $6
	`

	_, err := r.db.ExecContext(ctx, query,
		req.Status,
		req.ReviewedAt,
		req.ReviewedBy,
		req.RejectionReason,
		req.UpdatedAt,
		req.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update registration request: %w", err)
	}

	return nil
}

// OAuth connection methods - deprecated (OAuth removed, oauth_connections table dropped)
// These methods are kept for interface compatibility but return errors

func (r *OAuthRepositoryPostgres) CreateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error {
	return fmt.Errorf("OAuth connections are no longer supported")
}

func (r *OAuthRepositoryPostgres) GetOAuthConnection(
	ctx context.Context,
	provider domain.OAuthProvider,
	providerUserID string,
) (*domain.OAuthConnection, error) {
	return nil, fmt.Errorf("OAuth connections are no longer supported")
}

func (r *OAuthRepositoryPostgres) GetOAuthConnectionsByUser(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthConnection, error) {
	return nil, fmt.Errorf("OAuth connections are no longer supported")
}

func (r *OAuthRepositoryPostgres) UpdateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error {
	return fmt.Errorf("OAuth connections are no longer supported")
}

func (r *OAuthRepositoryPostgres) DeleteOAuthConnection(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf("OAuth connections are no longer supported")
}
