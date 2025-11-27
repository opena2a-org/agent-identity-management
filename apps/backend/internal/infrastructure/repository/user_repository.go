package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// UserRepository implements domain.UserRepository
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *domain.User) error {
	query := `
		INSERT INTO users (id, organization_id, email, name, avatar_url, role, provider, provider_id, password_hash, status, force_password_change, approved_by, approved_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	now := time.Now()

	// Only set ID if not already set
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	// Only set timestamps if not already set
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}

	// Default status to active if not set
	if user.Status == "" {
		user.Status = domain.UserStatusActive
	}

	_, err := r.db.Exec(query,
		user.ID,
		user.OrganizationID,
		user.Email,
		user.Name,
		user.AvatarURL,
		user.Role,
		user.Provider,    // Added provider
		user.ProviderID,  // Added provider_id
		user.PasswordHash,
		user.Status,
		user.ForcePasswordChange,
		user.ApprovedBy,
		user.ApprovedAt,
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role,
		       password_hash, force_password_change, last_login_at,
		       status, created_at, updated_at, approved_by, approved_at
		FROM users
		WHERE id = $1
	`

	user := &domain.User{}
	var status sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.PasswordHash,
		&user.ForcePasswordChange,
		&user.LastLoginAt,
		&status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ApprovedBy,
		&user.ApprovedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Set status from database or default to active
	if status.Valid {
		user.Status = domain.UserStatus(status.String)
	} else {
		user.Status = domain.UserStatusActive
	}

	return user, nil
}

// GetByEmail retrieves a user by email (includes password_hash for authentication)
func (r *UserRepository) GetByEmail(email string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role,
		       password_hash, force_password_change, last_login_at,
		       status, created_at, updated_at, approved_by, approved_at
		FROM users
		WHERE email = $1
	`

	user := &domain.User{}
	var status sql.NullString

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.PasswordHash,
		&user.ForcePasswordChange,
		&user.LastLoginAt,
		&status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ApprovedBy,
		&user.ApprovedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Set status from database or default to active
	if status.Valid {
		user.Status = domain.UserStatus(status.String)
	} else {
		user.Status = domain.UserStatusActive
	}

	return user, nil
}

// GetByPasswordResetToken retrieves a user by password reset token
func (r *UserRepository) GetByPasswordResetToken(resetToken string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role,
		       password_hash, force_password_change, last_login_at,
		       password_reset_token, password_reset_expires_at,
		       status, created_at, updated_at, approved_by, approved_at, deleted_at
		FROM users
		WHERE password_reset_token = $1
		  AND password_reset_expires_at > NOW()
		  AND deleted_at IS NULL
	`

	user := &domain.User{}
	var status sql.NullString

	err := r.db.QueryRow(query, resetToken).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.PasswordHash,
		&user.ForcePasswordChange,
		&user.LastLoginAt,
		&user.PasswordResetToken,
		&user.PasswordResetExpiresAt,
		&status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ApprovedBy,
		&user.ApprovedAt,
		&user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid or expired reset token")
	}
	if err != nil {
		return nil, err
	}

	// Set status from database or default to active
	if status.Valid {
		user.Status = domain.UserStatus(status.String)
	} else {
		user.Status = domain.UserStatusActive
	}

	return user, nil
}

// GetByOrganization retrieves all users in an organization
func (r *UserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role,
		       last_login_at, status, created_at, updated_at
		FROM users
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		var status sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.OrganizationID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Role,
			&user.LastLoginAt,
			&status,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Set status from database or default to active
		if status.Valid {
			user.Status = domain.UserStatus(status.String)
		} else {
			user.Status = domain.UserStatusActive
		}
		users = append(users, user)
	}

	return users, nil
}

// GetByOrganizationAndStatus retrieves users in an organization with a specific status
func (r *UserRepository) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	// Get all users in organization and filter by status
	allUsers, err := r.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	var filteredUsers []*domain.User
	for _, user := range allUsers {
		if user.Status == status {
			filteredUsers = append(filteredUsers, user)
		}
	}

	return filteredUsers, nil
}

// Update updates a user
func (r *UserRepository) Update(user *domain.User) error {
	query := `
		UPDATE users
		SET name = $1, avatar_url = $2, role = $3, password_hash = $4,
		    force_password_change = $5, last_login_at = $6,
		    status = $7, approved_by = $8, approved_at = $9,
		    password_reset_token = $10, password_reset_expires_at = $11,
		    updated_at = $12
		WHERE id = $13
	`

	user.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		user.Name,
		user.AvatarURL,
		user.Role,
		user.PasswordHash,
		user.ForcePasswordChange,
		user.LastLoginAt,
		user.Status,
		user.ApprovedBy,
		user.ApprovedAt,
		user.PasswordResetToken,
		user.PasswordResetExpiresAt,
		user.UpdatedAt,
		user.ID,
	)

	return err
}

// UpdateRole updates a user's role
func (r *UserRepository) UpdateRole(id uuid.UUID, role domain.UserRole) error {
	query := `UPDATE users SET role = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(query, role, time.Now(), id)
	return err
}

// Delete deletes a user
func (r *UserRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// CountActiveUsers returns the count of users who logged in within the specified minutes
func (r *UserRepository) CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM users
		WHERE organization_id = $1
		  AND last_login_at >= NOW() - INTERVAL '1 minute' * $2
		  AND status = 'active'
	`

	var count int
	err := r.db.QueryRow(query, orgID, withinMinutes).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active users: %w", err)
	}

	return count, nil
}
