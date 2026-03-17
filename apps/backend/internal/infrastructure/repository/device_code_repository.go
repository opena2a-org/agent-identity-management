package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// DeviceCodeRepository implements domain.DeviceCodeRepository using PostgreSQL.
type DeviceCodeRepository struct {
	db *sql.DB
}

// NewDeviceCodeRepository creates a new DeviceCodeRepository.
func NewDeviceCodeRepository(db *sql.DB) *DeviceCodeRepository {
	return &DeviceCodeRepository{db: db}
}

// Create inserts a new device authorization code record.
func (r *DeviceCodeRepository) Create(ctx context.Context, dc *domain.DeviceCode) error {
	query := `
		INSERT INTO device_codes (
			id, device_code, user_code, client_id, scope, verification_uri,
			expires_at, interval_seconds, status, user_id, organization_id,
			ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	if dc.ID == uuid.Nil {
		dc.ID = uuid.New()
	}
	if dc.CreatedAt.IsZero() {
		dc.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx, query,
		dc.ID,
		dc.DeviceCode,
		dc.UserCode,
		dc.ClientID,
		dc.Scope,
		dc.VerificationURI,
		dc.ExpiresAt,
		dc.IntervalSeconds,
		dc.Status,
		dc.UserID,
		dc.OrganizationID,
		dc.IPAddress,
		dc.UserAgent,
		dc.CreatedAt,
	)
	return err
}

// GetByDeviceCode retrieves a device code record by the device_code value.
func (r *DeviceCodeRepository) GetByDeviceCode(ctx context.Context, deviceCode string) (*domain.DeviceCode, error) {
	query := `
		SELECT id, device_code, user_code, client_id, scope, verification_uri,
			expires_at, interval_seconds, status, user_id, organization_id,
			ip_address, user_agent, approved_at, created_at
		FROM device_codes
		WHERE device_code = $1
	`

	dc := &domain.DeviceCode{}
	err := r.db.QueryRowContext(ctx, query, deviceCode).Scan(
		&dc.ID,
		&dc.DeviceCode,
		&dc.UserCode,
		&dc.ClientID,
		&dc.Scope,
		&dc.VerificationURI,
		&dc.ExpiresAt,
		&dc.IntervalSeconds,
		&dc.Status,
		&dc.UserID,
		&dc.OrganizationID,
		&dc.IPAddress,
		&dc.UserAgent,
		&dc.ApprovedAt,
		&dc.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device code not found")
	}
	return dc, err
}

// GetByUserCode retrieves a device code record by the user_code value.
func (r *DeviceCodeRepository) GetByUserCode(ctx context.Context, userCode string) (*domain.DeviceCode, error) {
	query := `
		SELECT id, device_code, user_code, client_id, scope, verification_uri,
			expires_at, interval_seconds, status, user_id, organization_id,
			ip_address, user_agent, approved_at, created_at
		FROM device_codes
		WHERE user_code = $1
	`

	dc := &domain.DeviceCode{}
	err := r.db.QueryRowContext(ctx, query, userCode).Scan(
		&dc.ID,
		&dc.DeviceCode,
		&dc.UserCode,
		&dc.ClientID,
		&dc.Scope,
		&dc.VerificationURI,
		&dc.ExpiresAt,
		&dc.IntervalSeconds,
		&dc.Status,
		&dc.UserID,
		&dc.OrganizationID,
		&dc.IPAddress,
		&dc.UserAgent,
		&dc.ApprovedAt,
		&dc.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user code not found")
	}
	return dc, err
}

// Approve marks a device code as approved, associating it with a user and organization.
func (r *DeviceCodeRepository) Approve(ctx context.Context, userCode string, userID uuid.UUID, orgID uuid.UUID) error {
	query := `
		UPDATE device_codes
		SET status = $1, user_id = $2, organization_id = $3, approved_at = $4
		WHERE user_code = $5 AND status = $6
	`

	result, err := r.db.ExecContext(ctx, query,
		domain.DeviceCodeStatusApproved,
		userID,
		orgID,
		time.Now(),
		userCode,
		domain.DeviceCodeStatusPending,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no pending device code found for user code")
	}
	return nil
}

// Deny marks a device code as denied.
func (r *DeviceCodeRepository) Deny(ctx context.Context, userCode string) error {
	query := `
		UPDATE device_codes
		SET status = $1
		WHERE user_code = $2 AND status = $3
	`

	result, err := r.db.ExecContext(ctx, query,
		domain.DeviceCodeStatusDenied,
		userCode,
		domain.DeviceCodeStatusPending,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no pending device code found for user code")
	}
	return nil
}

// CleanupExpired removes device codes that have passed their expiration time.
func (r *DeviceCodeRepository) CleanupExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM device_codes WHERE expires_at < $1`
	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
