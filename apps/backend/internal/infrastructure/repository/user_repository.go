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
		INSERT INTO users (id, organization_id, email, name, avatar_url, role, status, provider, provider_id, password_hash, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	now := time.Now()
	user.ID = uuid.New()
	user.CreatedAt = now
	user.UpdatedAt = now

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
		user.Status,
		user.Provider,
		user.ProviderID,
		user.PasswordHash,
		user.EmailVerified,
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, status, provider, provider_id,
		       approved_by, approved_at, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &domain.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Status,
		&user.Provider,
		&user.ProviderID,
		&user.ApprovedBy,
		&user.ApprovedAt,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByEmail retrieves a user by email (includes password_hash for authentication)
func (r *UserRepository) GetByEmail(email string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, status, provider, provider_id,
		       password_hash, email_verified, force_password_change, approved_by, approved_at,
		       last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	user := &domain.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Status,
		&user.Provider,
		&user.ProviderID,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.ForcePasswordChange,
		&user.ApprovedBy,
		&user.ApprovedAt,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByProvider retrieves a user by provider and provider ID
func (r *UserRepository) GetByProvider(provider, providerID string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, status, provider, provider_id,
		       approved_by, approved_at, last_login_at, created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`

	user := &domain.User{}
	err := r.db.QueryRow(query, provider, providerID).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Status,
		&user.Provider,
		&user.ProviderID,
		&user.ApprovedBy,
		&user.ApprovedAt,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User doesn't exist yet
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByOrganization retrieves all users in an organization
func (r *UserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, status, provider, provider_id,
		       approved_by, approved_at, last_login_at, created_at, updated_at
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
		err := rows.Scan(
			&user.ID,
			&user.OrganizationID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Role,
			&user.Status,
			&user.Provider,
			&user.ProviderID,
			&user.ApprovedBy,
			&user.ApprovedAt,
			&user.LastLoginAt,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// GetByOrganizationAndStatus retrieves users in an organization with a specific status
func (r *UserRepository) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, status, provider, provider_id,
		       approved_by, approved_at, last_login_at, created_at, updated_at
		FROM users
		WHERE organization_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, orgID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		err := rows.Scan(
			&user.ID,
			&user.OrganizationID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Role,
			&user.Status,
			&user.Provider,
			&user.ProviderID,
			&user.ApprovedBy,
			&user.ApprovedAt,
			&user.LastLoginAt,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// Update updates a user
func (r *UserRepository) Update(user *domain.User) error {
	query := `
		UPDATE users
		SET name = $1, avatar_url = $2, role = $3, status = $4, approved_by = $5, approved_at = $6,
		    last_login_at = $7, updated_at = $8
		WHERE id = $9
	`

	user.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		user.Name,
		user.AvatarURL,
		user.Role,
		user.Status,
		user.ApprovedBy,
		user.ApprovedAt,
		user.LastLoginAt,
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
