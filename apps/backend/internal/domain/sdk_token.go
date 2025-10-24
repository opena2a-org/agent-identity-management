package domain

import (
	"time"

	"github.com/google/uuid"
)

// SDKToken represents a tracked SDK refresh token for security and revocation
type SDKToken struct {
	ID               uuid.UUID              `json:"id"`
	UserID           uuid.UUID              `json:"userId"`
	OrganizationID   uuid.UUID              `json:"organizationId"`
	TokenHash        string                 `json:"-"` // Never expose in JSON
	TokenID          string                 `json:"tokenId"`
	DeviceName       *string                `json:"deviceName,omitempty"`
	DeviceFingerprint *string               `json:"deviceFingerprint,omitempty"`
	IPAddress        *string                `json:"ipAddress,omitempty"`
	UserAgent        *string                `json:"userAgent,omitempty"`
	LastUsedAt       *time.Time             `json:"lastUsedAt,omitempty"`
	LastIPAddress    *string                `json:"lastIpAddress,omitempty"`
	UsageCount       int                    `json:"usageCount"`
	CreatedAt        time.Time              `json:"createdAt"`
	ExpiresAt        time.Time              `json:"expiresAt"`
	RevokedAt        *time.Time             `json:"revokedAt,omitempty"`
	RevokeReason     *string                `json:"revokeReason,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// IsActive returns true if token is not revoked and not expired
func (t *SDKToken) IsActive() bool {
	if t.RevokedAt != nil {
		return false
	}
	if time.Now().After(t.ExpiresAt) {
		return false
	}
	return true
}

// Revoke marks the token as revoked with a reason
func (t *SDKToken) Revoke(reason string) {
	now := time.Now()
	t.RevokedAt = &now
	t.RevokeReason = &reason
}

// RecordUsage updates the last used timestamp and IP address
func (t *SDKToken) RecordUsage(ipAddress string) {
	now := time.Now()
	t.LastUsedAt = &now
	t.LastIPAddress = &ipAddress
	t.UsageCount++
}

// SDKTokenRepository defines the interface for SDK token persistence
type SDKTokenRepository interface {
	// Create stores a new SDK token
	Create(token *SDKToken) error

	// GetByID retrieves a token by its ID
	GetByID(id uuid.UUID) (*SDKToken, error)

	// GetByTokenID retrieves a token by its JWT token ID (JTI claim)
	GetByTokenID(tokenID string) (*SDKToken, error)

	// GetByTokenHash retrieves a token by its hash
	GetByTokenHash(tokenHash string) (*SDKToken, error)

	// GetByUserID retrieves all tokens for a user
	GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*SDKToken, error)

	// GetByOrganizationID retrieves all tokens for an organization
	GetByOrganizationID(organizationID uuid.UUID, includeRevoked bool) ([]*SDKToken, error)

	// Update updates a token
	Update(token *SDKToken) error

	// Revoke marks a token as revoked
	Revoke(id uuid.UUID, reason string) error

	// RevokeByTokenHash marks a token as revoked using its hash
	RevokeByTokenHash(tokenHash string, reason string) error

	// RevokeAllForUser revokes all tokens for a user
	RevokeAllForUser(userID uuid.UUID, reason string) error

	// RecordUsage updates token usage statistics
	RecordUsage(tokenID string, ipAddress string) error

	// DeleteExpired removes expired tokens (cleanup job)
	DeleteExpired() error

	// GetActiveCount returns count of active tokens for a user
	GetActiveCount(userID uuid.UUID) (int, error)
}
