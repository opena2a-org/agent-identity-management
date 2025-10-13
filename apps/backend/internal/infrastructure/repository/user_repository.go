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
		INSERT INTO users (id, organization_id, email, name, avatar_url, role, provider, provider_id, password_hash, email_verified, oauth_provider, oauth_user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	now := time.Now()
	user.ID = uuid.New()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Default status to active if not set (status column doesn't exist in DB)
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
		user.Provider,
		user.ProviderID,
		user.PasswordHash,
		user.EmailVerified,
		user.Provider,   // oauth_provider (duplicate of provider for compatibility)
		user.ProviderID, // oauth_user_id (duplicate of provider_id for compatibility)
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, provider, provider_id,
		       last_login_at, created_at, updated_at, oauth_provider, oauth_user_id
		FROM users
		WHERE id = $1
	`

	user := &domain.User{}
	var oauthProvider, oauthUserID sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Provider,
		&user.ProviderID,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&oauthProvider,
		&oauthUserID,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Set default status to active since the column doesn't exist
	user.Status = domain.UserStatusActive

	return user, nil
}

// GetByEmail retrieves a user by email (includes password_hash for authentication)
func (r *UserRepository) GetByEmail(email string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, provider, provider_id,
		       password_hash, email_verified, force_password_change, last_login_at, 
		       created_at, updated_at, oauth_provider, oauth_user_id
		FROM users
		WHERE email = $1
	`

	user := &domain.User{}
	var oauthProvider, oauthUserID sql.NullString

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Provider,
		&user.ProviderID,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.ForcePasswordChange,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&oauthProvider,
		&oauthUserID,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	// Set default status to active since the column doesn't exist
	user.Status = domain.UserStatusActive

	return user, nil
}

// GetByProvider retrieves a user by provider and provider ID
func (r *UserRepository) GetByProvider(provider, providerID string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, provider, provider_id,
		       last_login_at, created_at, updated_at, oauth_provider, oauth_user_id
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`

	user := &domain.User{}
	var oauthProvider, oauthUserID sql.NullString

	err := r.db.QueryRow(query, provider, providerID).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Role,
		&user.Provider,
		&user.ProviderID,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&oauthProvider,
		&oauthUserID,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User doesn't exist yet
	}
	if err != nil {
		return nil, err
	}

	// Set default status to active since the column doesn't exist
	user.Status = domain.UserStatusActive

	return user, nil
}

// GetByOrganization retrieves all users in an organization
func (r *UserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, avatar_url, role, provider, provider_id,
		       last_login_at, created_at, updated_at, oauth_provider, oauth_user_id
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
		var oauthProvider, oauthUserID sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.OrganizationID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Role,
			&user.Provider,
			&user.ProviderID,
			&user.LastLoginAt,
			&user.CreatedAt,
			&user.UpdatedAt,
			&oauthProvider,
			&oauthUserID,
		)
		if err != nil {
			return nil, err
		}

		// Set default status to active since the column doesn't exist
		user.Status = domain.UserStatusActive
		users = append(users, user)
	}

	return users, nil
}

// GetByOrganizationAndStatus retrieves users in an organization with a specific status
// Since status column doesn't exist, we return all users and filter by the requested status
func (r *UserRepository) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	// Get all users in organization
	allUsers, err := r.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	// Filter by status (all users are considered active since status column doesn't exist)
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
		SET name = $1, avatar_url = $2, role = $3, last_login_at = $4, updated_at = $5
		WHERE id = $6
	`

	user.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		user.Name,
		user.AvatarURL,
		user.Role,
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
