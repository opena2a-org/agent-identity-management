package repository

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Handle nullable oauth_provider - convert pointer to value for SQL
	var oauthProvider interface{}
	if req.OAuthProvider != nil {
		oauthProvider = string(*req.OAuthProvider)
		fmt.Printf("DEBUG: Setting oauth_provider to: %v\n", oauthProvider)
	} else {
		oauthProvider = nil
		fmt.Printf("DEBUG: oauth_provider is nil\n")
	}

	query := `
		INSERT INTO user_registration_requests (
			id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			organization_id, status, requested_at, profile_picture_url,
			oauth_email_verified, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		req.ID,
		req.Email,
		req.FirstName,
		req.LastName,
		req.PasswordHash,
		oauthProvider, // Use the dereferenced value
		req.OAuthUserID,
		req.OrganizationID,
		req.Status,
		req.RequestedAt,
		req.ProfilePictureURL,
		req.OAuthEmailVerified,
		metadataJSON,
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
		SELECT id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, profile_picture_url, oauth_email_verified,
			   metadata, created_at, updated_at
		FROM user_registration_requests
		WHERE id = $1
	`

	var req domain.UserRegistrationRequest
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.PasswordHash,
		&req.OAuthProvider,
		&req.OAuthUserID,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.ProfilePictureURL,
		&req.OAuthEmailVerified,
		&metadataJSON,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("registration request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registration request: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &req.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &req, nil
}

func (r *OAuthRepositoryPostgres) GetRegistrationRequestByOAuth(
	ctx context.Context,
	provider domain.OAuthProvider,
	providerUserID string,
) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, profile_picture_url, oauth_email_verified,
			   metadata, created_at, updated_at
		FROM user_registration_requests
		WHERE oauth_provider = $1 AND oauth_user_id = $2
	`

	var req domain.UserRegistrationRequest
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, provider, providerUserID).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.PasswordHash,
		&req.OAuthProvider,
		&req.OAuthUserID,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.ProfilePictureURL,
		&req.OAuthEmailVerified,
		&metadataJSON,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registration request: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &req.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &req, nil
}

func (r *OAuthRepositoryPostgres) GetRegistrationRequestByEmail(
	ctx context.Context,
	email string,
) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, profile_picture_url, oauth_email_verified,
			   metadata, created_at, updated_at
		FROM user_registration_requests
		WHERE email = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var req domain.UserRegistrationRequest
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, email, domain.RegistrationStatusPending).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.PasswordHash,
		&req.OAuthProvider,
		&req.OAuthUserID,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.ProfilePictureURL,
		&req.OAuthEmailVerified,
		&metadataJSON,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get registration request: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &req.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &req, nil
}

// GetRegistrationRequestByEmailAnyStatus retrieves a registration request by email (any status)
func (r *OAuthRepositoryPostgres) GetRegistrationRequestByEmailAnyStatus(
	ctx context.Context,
	email string,
) (*domain.UserRegistrationRequest, error) {
	query := `
		SELECT id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, profile_picture_url, oauth_email_verified,
			   metadata, created_at, updated_at
		FROM user_registration_requests
		WHERE email = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var req domain.UserRegistrationRequest
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&req.ID,
		&req.Email,
		&req.FirstName,
		&req.LastName,
		&req.PasswordHash,
		&req.OAuthProvider,
		&req.OAuthUserID,
		&req.OrganizationID,
		&req.Status,
		&req.RequestedAt,
		&req.ReviewedAt,
		&req.ReviewedBy,
		&req.RejectionReason,
		&req.ProfilePictureURL,
		&req.OAuthEmailVerified,
		&metadataJSON,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("registration request not found")
	}
	if err != nil {
		return nil, err
	}

	// Parse metadata JSON
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &req.Metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
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
		SELECT id, email, first_name, last_name, password_hash, oauth_provider, oauth_user_id,
			   organization_id, status, requested_at, reviewed_at, reviewed_by,
			   rejection_reason, profile_picture_url, oauth_email_verified,
			   metadata, created_at, updated_at
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
		var metadataJSON []byte

		err := rows.Scan(
			&req.ID,
			&req.Email,
			&req.FirstName,
			&req.LastName,
			&req.PasswordHash,
			&req.OAuthProvider,
			&req.OAuthUserID,
			&req.OrganizationID,
			&req.Status,
			&req.RequestedAt,
			&req.ReviewedAt,
			&req.ReviewedBy,
			&req.RejectionReason,
			&req.ProfilePictureURL,
			&req.OAuthEmailVerified,
			&metadataJSON,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan registration request: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &req.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		requests = append(requests, &req)
	}

	return requests, total, nil
}

func (r *OAuthRepositoryPostgres) UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE user_registration_requests
		SET status = $1, reviewed_at = $2, reviewed_by = $3,
			rejection_reason = $4, updated_at = $5, metadata = $6
		WHERE id = $7
	`

	_, err = r.db.ExecContext(ctx, query,
		req.Status,
		req.ReviewedAt,
		req.ReviewedBy,
		req.RejectionReason,
		req.UpdatedAt,
		metadataJSON,
		req.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update registration request: %w", err)
	}

	return nil
}

// OAuth connections

func (r *OAuthRepositoryPostgres) CreateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error {
	// Convert profile data to JSON
	profileDataJSON, err := json.Marshal(conn.ProfileData)
	if err != nil {
		return fmt.Errorf("failed to marshal profile data: %w", err)
	}

	query := `
		INSERT INTO oauth_connections (
			id, user_id, provider, provider_user_id, provider_email,
			access_token_hash, refresh_token_hash, token_expires_at,
			profile_data, last_used_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		conn.ID,
		conn.UserID,
		conn.Provider,
		conn.ProviderUserID,
		conn.ProviderEmail,
		conn.AccessTokenHash,
		conn.RefreshTokenHash,
		conn.TokenExpiresAt,
		profileDataJSON,
		conn.LastUsedAt,
		conn.CreatedAt,
		conn.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create OAuth connection: %w", err)
	}

	return nil
}

func (r *OAuthRepositoryPostgres) GetOAuthConnection(
	ctx context.Context,
	provider domain.OAuthProvider,
	providerUserID string,
) (*domain.OAuthConnection, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, provider_email,
			   access_token_hash, refresh_token_hash, token_expires_at,
			   profile_data, last_used_at, created_at, updated_at
		FROM oauth_connections
		WHERE provider = $1 AND provider_user_id = $2
	`

	var conn domain.OAuthConnection
	var profileDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, provider, providerUserID).Scan(
		&conn.ID,
		&conn.UserID,
		&conn.Provider,
		&conn.ProviderUserID,
		&conn.ProviderEmail,
		&conn.AccessTokenHash,
		&conn.RefreshTokenHash,
		&conn.TokenExpiresAt,
		&profileDataJSON,
		&conn.LastUsedAt,
		&conn.CreatedAt,
		&conn.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth connection: %w", err)
	}

	// Unmarshal profile data
	if len(profileDataJSON) > 0 {
		if err := json.Unmarshal(profileDataJSON, &conn.ProfileData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal profile data: %w", err)
		}
	}

	return &conn, nil
}

func (r *OAuthRepositoryPostgres) GetOAuthConnectionsByUser(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthConnection, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, provider_email,
			   access_token_hash, refresh_token_hash, token_expires_at,
			   profile_data, last_used_at, created_at, updated_at
		FROM oauth_connections
		WHERE user_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list OAuth connections: %w", err)
	}
	defer rows.Close()

	var connections []*domain.OAuthConnection
	for rows.Next() {
		var conn domain.OAuthConnection
		var profileDataJSON []byte

		err := rows.Scan(
			&conn.ID,
			&conn.UserID,
			&conn.Provider,
			&conn.ProviderUserID,
			&conn.ProviderEmail,
			&conn.AccessTokenHash,
			&conn.RefreshTokenHash,
			&conn.TokenExpiresAt,
			&profileDataJSON,
			&conn.LastUsedAt,
			&conn.CreatedAt,
			&conn.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan OAuth connection: %w", err)
		}

		// Unmarshal profile data
		if len(profileDataJSON) > 0 {
			if err := json.Unmarshal(profileDataJSON, &conn.ProfileData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal profile data: %w", err)
			}
		}

		connections = append(connections, &conn)
	}

	return connections, nil
}

func (r *OAuthRepositoryPostgres) UpdateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error {
	// Convert profile data to JSON
	profileDataJSON, err := json.Marshal(conn.ProfileData)
	if err != nil {
		return fmt.Errorf("failed to marshal profile data: %w", err)
	}

	query := `
		UPDATE oauth_connections
		SET access_token_hash = $1, refresh_token_hash = $2,
			token_expires_at = $3, profile_data = $4,
			last_used_at = $5, updated_at = $6
		WHERE id = $7
	`

	_, err = r.db.ExecContext(ctx, query,
		conn.AccessTokenHash,
		conn.RefreshTokenHash,
		conn.TokenExpiresAt,
		profileDataJSON,
		conn.LastUsedAt,
		conn.UpdatedAt,
		conn.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update OAuth connection: %w", err)
	}

	return nil
}

func (r *OAuthRepositoryPostgres) DeleteOAuthConnection(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM oauth_connections WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete OAuth connection: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("OAuth connection not found")
	}

	return nil
}
