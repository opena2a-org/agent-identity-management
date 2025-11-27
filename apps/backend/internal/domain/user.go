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
	ID                     uuid.UUID  `json:"id"`
	OrganizationID         uuid.UUID  `json:"organizationId"`
	Email                  string     `json:"email"`
	Name                   string     `json:"name"`
	AvatarURL              *string    `json:"avatarUrl"` // Nullable for local users
	Role                   UserRole   `json:"role"`
	Provider               string     `json:"provider"`   // Auth provider: "local", "google", "github", "microsoft"
	ProviderID             string     `json:"providerId"` // Provider-specific user ID
	Status                 UserStatus `json:"status"`     // pending, active, suspended, deactivated
	PasswordHash           *string    `json:"-"`          // Never expose in JSON
	ForcePasswordChange    bool       `json:"forcePasswordChange"`
	PasswordResetToken     *string    `json:"-"` // Never expose in JSON
	PasswordResetExpiresAt *time.Time `json:"-"` // Never expose in JSON
	ApprovedBy             *uuid.UUID `json:"approvedBy,omitempty"` // Admin who approved this user
	ApprovedAt             *time.Time `json:"approvedAt,omitempty"` // When user was approved
	LastLoginAt            *time.Time `json:"lastLoginAt"`
	DeletedAt              *time.Time `json:"deletedAt,omitempty"` // When user was soft-deleted (deactivated)
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
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
	CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error)
}
