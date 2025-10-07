package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserRole represents user permission levels
type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleMember  UserRole = "member"
	RoleViewer  UserRole = "viewer"
)

// User represents a platform user
type User struct {
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	Email                 string     `json:"email"`
	Name                  string     `json:"name"`
	AvatarURL             *string    `json:"avatar_url"` // Nullable for local users
	Role                  UserRole   `json:"role"`
	Provider              string     `json:"provider"` // google, microsoft, okta, local
	ProviderID            string     `json:"provider_id"`
	PasswordHash           *string    `json:"-"` // Never expose in JSON
	EmailVerified          bool       `json:"email_verified"`
	ForcePasswordChange    bool       `json:"force_password_change"`
	PasswordResetToken     *string    `json:"-"` // Never expose in JSON
	PasswordResetExpiresAt *time.Time `json:"-"` // Never expose in JSON
	LastLoginAt            *time.Time `json:"last_login_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// UserRepository defines the interface for user persistence
type UserRepository interface {
	Create(user *User) error
	GetByID(id uuid.UUID) (*User, error)
	GetByEmail(email string) (*User, error)
	GetByProvider(provider, providerID string) (*User, error)
	GetByOrganization(orgID uuid.UUID) ([]*User, error)
	Update(user *User) error
	UpdateRole(id uuid.UUID, role UserRole) error
	Delete(id uuid.UUID) error
}
