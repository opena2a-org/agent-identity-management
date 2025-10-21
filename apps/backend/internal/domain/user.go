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

// UserStatus represents user account status
type UserStatus string

const (
	UserStatusPending     UserStatus = "pending"     // Awaiting admin approval
	UserStatusActive      UserStatus = "active"      // Can use system
	UserStatusSuspended   UserStatus = "suspended"   // Temporarily blocked
	UserStatusDeactivated UserStatus = "deactivated" // Permanently disabled
)

// User represents a platform user
type User struct {
	ID                     uuid.UUID   `json:"id"`
	OrganizationID         uuid.UUID   `json:"organization_id"`
	Email                  string      `json:"email"`
	Name                   string      `json:"name"`
	AvatarURL              *string     `json:"avatar_url"` // Nullable for local users
	Role                   UserRole    `json:"role"`
	Provider               string      `json:"provider"`     // Auth provider: "local", "google", "github", "microsoft"
	ProviderID             string      `json:"provider_id"`  // Provider-specific user ID
	Status                 UserStatus  `json:"status"` // pending, active, suspended, deactivated
	PasswordHash           *string     `json:"-"` // Never expose in JSON
	ForcePasswordChange    bool        `json:"force_password_change"`
	PasswordResetToken     *string     `json:"-"` // Never expose in JSON
	PasswordResetExpiresAt *time.Time  `json:"-"` // Never expose in JSON
	ApprovedBy             *uuid.UUID  `json:"approved_by,omitempty"` // Admin who approved this user
	ApprovedAt             *time.Time  `json:"approved_at,omitempty"` // When user was approved
	LastLoginAt            *time.Time  `json:"last_login_at"`
	DeletedAt              *time.Time  `json:"deleted_at,omitempty"` // When user was soft-deleted (deactivated)
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

// UserRepository defines the interface for user persistence
type UserRepository interface {
	Create(user *User) error
	GetByID(id uuid.UUID) (*User, error)
	GetByEmail(email string) (*User, error)
	GetByPasswordResetToken(resetToken string) (*User, error)
	GetByOrganization(orgID uuid.UUID) ([]*User, error)
	GetByOrganizationAndStatus(orgID uuid.UUID, status UserStatus) ([]*User, error)
	Update(user *User) error
	UpdateRole(id uuid.UUID, role UserRole) error
	Delete(id uuid.UUID) error
}
