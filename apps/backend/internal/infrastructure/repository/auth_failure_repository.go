package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AuthFailureRepository implements domain.AuthFailureRepository
type AuthFailureRepository struct {
	db *sql.DB
}

// NewAuthFailureRepository creates a new auth failure repository
func NewAuthFailureRepository(db *sql.DB) *AuthFailureRepository {
	return &AuthFailureRepository{db: db}
}

// RecordFailure records a failed authentication attempt
func (r *AuthFailureRepository) RecordFailure(failure *domain.AuthFailure) error {
	query := `
		INSERT INTO auth_failures (id, organization_id, user_id, email, failure_type, ip_address, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	if failure.ID == uuid.Nil {
		failure.ID = uuid.New()
	}
	if failure.CreatedAt.IsZero() {
		failure.CreatedAt = time.Now()
	}

	// Convert metadata to JSON
	var metadataJSON []byte
	var err error
	if failure.Metadata != nil {
		metadataJSON, err = json.Marshal(failure.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	_, err = r.db.Exec(query,
		failure.ID,
		failure.OrganizationID,
		failure.UserID,
		failure.Email,
		failure.FailureType,
		failure.IPAddress,
		failure.UserAgent,
		metadataJSON,
		failure.CreatedAt,
	)
	return err
}

// GetRecentFailures gets recent failures for an email within a time window
func (r *AuthFailureRepository) GetRecentFailures(email string, withinMinutes int) ([]*domain.AuthFailure, error) {
	query := `
		SELECT id, organization_id, user_id, email, failure_type, ip_address, user_agent, COALESCE(metadata, '{}'), created_at
		FROM auth_failures
		WHERE email = $1 AND created_at > NOW() - INTERVAL '1 minute' * $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, email, withinMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var failures []*domain.AuthFailure
	for rows.Next() {
		failure := &domain.AuthFailure{}
		var metadataJSON []byte
		var orgID, userID sql.NullString

		err := rows.Scan(
			&failure.ID,
			&orgID,
			&userID,
			&failure.Email,
			&failure.FailureType,
			&failure.IPAddress,
			&failure.UserAgent,
			&metadataJSON,
			&failure.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if orgID.Valid {
			id := uuid.MustParse(orgID.String)
			failure.OrganizationID = &id
		}
		if userID.Valid {
			id := uuid.MustParse(userID.String)
			failure.UserID = &id
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &failure.Metadata)
		}

		failures = append(failures, failure)
	}

	return failures, nil
}

// CountRecentFailures counts failures for an email within a time window
func (r *AuthFailureRepository) CountRecentFailures(email string, withinMinutes int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM auth_failures
		WHERE email = $1 AND created_at > NOW() - INTERVAL '1 minute' * $2
	`

	var count int
	err := r.db.QueryRow(query, email, withinMinutes).Scan(&count)
	return count, err
}

// ClearFailures clears failures for an email (called on successful login)
func (r *AuthFailureRepository) ClearFailures(email string) error {
	query := `DELETE FROM auth_failures WHERE email = $1`
	_, err := r.db.Exec(query, email)
	return err
}

// GetByOrganization gets failures by organization for admin dashboard
func (r *AuthFailureRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuthFailure, int, error) {
	countQuery := `SELECT COUNT(*) FROM auth_failures WHERE organization_id = $1`
	var total int
	if err := r.db.QueryRow(countQuery, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, organization_id, user_id, email, failure_type, ip_address, user_agent, COALESCE(metadata, '{}'), created_at
		FROM auth_failures
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var failures []*domain.AuthFailure
	for rows.Next() {
		failure := &domain.AuthFailure{}
		var metadataJSON []byte
		var orgIDNull, userID sql.NullString

		err := rows.Scan(
			&failure.ID,
			&orgIDNull,
			&userID,
			&failure.Email,
			&failure.FailureType,
			&failure.IPAddress,
			&failure.UserAgent,
			&metadataJSON,
			&failure.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if orgIDNull.Valid {
			id := uuid.MustParse(orgIDNull.String)
			failure.OrganizationID = &id
		}
		if userID.Valid {
			id := uuid.MustParse(userID.String)
			failure.UserID = &id
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &failure.Metadata)
		}

		failures = append(failures, failure)
	}

	return failures, total, nil
}

// CreateLockout creates a new lockout record
func (r *AuthFailureRepository) CreateLockout(lockout *domain.AuthLockout) error {
	query := `
		INSERT INTO auth_lockouts (id, organization_id, user_id, email, failed_attempts, locked_at, locked_until, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	if lockout.ID == uuid.Nil {
		lockout.ID = uuid.New()
	}
	now := time.Now()
	if lockout.CreatedAt.IsZero() {
		lockout.CreatedAt = now
	}
	if lockout.UpdatedAt.IsZero() {
		lockout.UpdatedAt = now
	}

	_, err := r.db.Exec(query,
		lockout.ID,
		lockout.OrganizationID,
		lockout.UserID,
		lockout.Email,
		lockout.FailedAttempts,
		lockout.LockedAt,
		lockout.LockedUntil,
		lockout.CreatedAt,
		lockout.UpdatedAt,
	)
	return err
}

// GetActiveLockout gets the active lockout for an email (if any)
func (r *AuthFailureRepository) GetActiveLockout(email string) (*domain.AuthLockout, error) {
	query := `
		SELECT id, organization_id, user_id, email, failed_attempts, locked_at, locked_until, unlocked_at, unlocked_by, created_at, updated_at
		FROM auth_lockouts
		WHERE email = $1 AND locked_until > NOW() AND unlocked_at IS NULL
		ORDER BY locked_at DESC
		LIMIT 1
	`

	lockout := &domain.AuthLockout{}
	var orgID, userID, unlockedBy sql.NullString
	var unlockedAt sql.NullTime

	err := r.db.QueryRow(query, email).Scan(
		&lockout.ID,
		&orgID,
		&userID,
		&lockout.Email,
		&lockout.FailedAttempts,
		&lockout.LockedAt,
		&lockout.LockedUntil,
		&unlockedAt,
		&unlockedBy,
		&lockout.CreatedAt,
		&lockout.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No active lockout
	}
	if err != nil {
		return nil, err
	}

	if orgID.Valid {
		id := uuid.MustParse(orgID.String)
		lockout.OrganizationID = &id
	}
	if userID.Valid {
		id := uuid.MustParse(userID.String)
		lockout.UserID = &id
	}
	if unlockedAt.Valid {
		lockout.UnlockedAt = &unlockedAt.Time
	}
	if unlockedBy.Valid {
		id := uuid.MustParse(unlockedBy.String)
		lockout.UnlockedBy = &id
	}

	return lockout, nil
}

// UnlockAccount unlocks an account (admin action)
func (r *AuthFailureRepository) UnlockAccount(email string, adminID uuid.UUID) error {
	query := `
		UPDATE auth_lockouts
		SET unlocked_at = NOW(), unlocked_by = $2, updated_at = NOW()
		WHERE email = $1 AND unlocked_at IS NULL
	`
	_, err := r.db.Exec(query, email, adminID)
	return err
}

// GetLockoutsByOrganization gets lockouts by organization for admin dashboard
func (r *AuthFailureRepository) GetLockoutsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuthLockout, int, error) {
	countQuery := `SELECT COUNT(*) FROM auth_lockouts WHERE organization_id = $1`
	var total int
	if err := r.db.QueryRow(countQuery, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, organization_id, user_id, email, failed_attempts, locked_at, locked_until, unlocked_at, unlocked_by, created_at, updated_at
		FROM auth_lockouts
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var lockouts []*domain.AuthLockout
	for rows.Next() {
		lockout := &domain.AuthLockout{}
		var orgIDNull, userID, unlockedBy sql.NullString
		var unlockedAt sql.NullTime

		err := rows.Scan(
			&lockout.ID,
			&orgIDNull,
			&userID,
			&lockout.Email,
			&lockout.FailedAttempts,
			&lockout.LockedAt,
			&lockout.LockedUntil,
			&unlockedAt,
			&unlockedBy,
			&lockout.CreatedAt,
			&lockout.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if orgIDNull.Valid {
			id := uuid.MustParse(orgIDNull.String)
			lockout.OrganizationID = &id
		}
		if userID.Valid {
			id := uuid.MustParse(userID.String)
			lockout.UserID = &id
		}
		if unlockedAt.Valid {
			lockout.UnlockedAt = &unlockedAt.Time
		}
		if unlockedBy.Valid {
			id := uuid.MustParse(unlockedBy.String)
			lockout.UnlockedBy = &id
		}

		lockouts = append(lockouts, lockout)
	}

	return lockouts, total, nil
}
