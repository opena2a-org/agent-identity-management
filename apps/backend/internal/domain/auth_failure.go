package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuthFailureType represents the type of authentication failure
type AuthFailureType string

const (
	AuthFailureInvalidCredentials AuthFailureType = "invalid_credentials"
	AuthFailureAccountLocked      AuthFailureType = "account_locked"
	AuthFailureAccountDeactivated AuthFailureType = "account_deactivated"
	AuthFailureInvalidToken       AuthFailureType = "invalid_token"
	AuthFailureExpiredToken       AuthFailureType = "expired_token"
)

// AuthFailure represents a failed authentication attempt
type AuthFailure struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID *uuid.UUID      `json:"organizationId,omitempty"` // Nullable for unknown users
	UserID         *uuid.UUID      `json:"userId,omitempty"`         // Nullable for unknown users
	Email          string          `json:"email"`                    // Email attempted (always captured)
	FailureType    AuthFailureType `json:"failureType"`
	IPAddress      string          `json:"ipAddress"`
	UserAgent      string          `json:"userAgent"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// AuthLockout represents an account lockout due to failed attempts
type AuthLockout struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organizationId,omitempty"`
	UserID         *uuid.UUID `json:"userId,omitempty"`
	Email          string     `json:"email"`
	FailedAttempts int        `json:"failedAttempts"`
	LockedAt       time.Time  `json:"lockedAt"`
	LockedUntil    time.Time  `json:"lockedUntil"`
	UnlockedAt     *time.Time `json:"unlockedAt,omitempty"` // Set when admin unlocks
	UnlockedBy     *uuid.UUID `json:"unlockedBy,omitempty"` // Admin who unlocked
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// AuthFailureRepository defines the interface for auth failure persistence
type AuthFailureRepository interface {
	// Record a failed authentication attempt
	RecordFailure(failure *AuthFailure) error

	// Get recent failures for an email within a time window
	GetRecentFailures(email string, withinMinutes int) ([]*AuthFailure, error)

	// Count failures for an email within a time window
	CountRecentFailures(email string, withinMinutes int) (int, error)

	// Clear failures for an email (called on successful login)
	ClearFailures(email string) error

	// Get failures by organization for admin dashboard
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*AuthFailure, int, error)

	// Lockout management
	CreateLockout(lockout *AuthLockout) error
	GetActiveLockout(email string) (*AuthLockout, error)
	UnlockAccount(email string, adminID uuid.UUID) error
	GetLockoutsByOrganization(orgID uuid.UUID, limit, offset int) ([]*AuthLockout, int, error)
}

// AuthFailureConfig holds configuration for failed authentication monitoring
type AuthFailureConfig struct {
	MaxAttempts     int           // Maximum failed attempts before lockout (default: 5)
	TimeWindowMins  int           // Time window for counting attempts (default: 15)
	LockoutMins     int           // Lockout duration in minutes (default: 30)
	AlertThreshold  int           // Number of failures before creating alert (default: 3)
}

// DefaultAuthFailureConfig returns the default configuration
func DefaultAuthFailureConfig() AuthFailureConfig {
	return AuthFailureConfig{
		MaxAttempts:    5,
		TimeWindowMins: 15,
		LockoutMins:    30,
		AlertThreshold: 3,
	}
}
