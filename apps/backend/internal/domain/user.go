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

// CanHoldSession reports whether the account may keep or mint a session. It is
// an allow-list — users.status has no CHECK constraint, so an unrecognised
// value must fail closed — and a soft-deleted user never qualifies. Used by
// token refresh and SDK token recovery; pending is admitted because login
// admits it.
func (u *User) CanHoldSession() bool {
	if u == nil || u.DeletedAt != nil {
		return false
	}
	return u.Status == UserStatusActive || u.Status == UserStatusPending
}

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
	PasswordResetToken     *string    `json:"-"`                    // SHA-256 digest of the reset token; never expose in JSON
	PasswordResetExpiresAt *time.Time `json:"-"`                    // Never expose in JSON
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
	GetByPasswordResetToken(tokenDigest string) (*User, error)
	GetByOrganization(orgID uuid.UUID) ([]*User, error)
	GetByOrganizationAndStatus(orgID uuid.UUID, status UserStatus) ([]*User, error)
	Update(user *User) error
	UpdateRole(id uuid.UUID, role UserRole) error
	// UpdateLastLogin records a sign-in by writing last_login_at and updated_at only, so a
	// sign-in never writes back the rest of a row it read before verifying the password.
	UpdateLastLogin(id uuid.UUID, at time.Time) error
	Delete(id uuid.UUID) error
	CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error)
	// CountByRoleAndStatus counts users across every organization with the given role and status
	// (soft-deleted users excluded); the registration path uses it to learn whether anyone can approve.
	CountByRoleAndStatus(role UserRole, status UserStatus) (int, error)
}
