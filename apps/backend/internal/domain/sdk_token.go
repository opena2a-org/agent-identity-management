package domain

import (
	"time"

	"github.com/google/uuid"
)

// Revoke reasons the server writes on its own. A row retired by rotation and
// presented again is a reuse; a row revoked with its family was ended by one;
// a row revoked for its family's age was presented past the family's maximum
// age (SDK_TOKEN_FAMILY_MAX_AGE).
const (
	SDKTokenRevokeReasonRotation      = "token_rotation"
	SDKTokenRevokeReasonFamilyRevoked = "token_family_revoked"
	SDKTokenRevokeReasonFamilyMaxAge  = "family_max_age"
)

// SDKTokenRevokeReasonPasswordChanged is the revoke reason of a row that was
// not yet revoked when its user's password was changed or reset. The change
// refuses every token minted before it, so the row's family is over.
const SDKTokenRevokeReasonPasswordChanged = "password_changed"

// SDKTokenMetadataFamilyCreatedAt is the metadata key under which a row
// carries its family's birth, the created_at of the row the download wrote,
// as an RFC 3339 time. A rotation copies it to the successor row, so the
// birth survives the root row; a row without it is its family's root.
const SDKTokenMetadataFamilyCreatedAt = "familyCreatedAt"

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

// RevokedFor reports whether the row was revoked with the given reason.
func (t *SDKToken) RevokedFor(reason string) bool {
	return t.RevokedAt != nil && t.RevokeReason != nil && *t.RevokeReason == reason
}

// FamilyCreatedAt is the birth of the family the row belongs to: the
// familyCreatedAt its metadata carries, or the row's own created_at when the
// metadata carries none or an unreadable value (the row is then read as its
// family's root, so an absent value never lengthens the family's life).
func (t *SDKToken) FamilyCreatedAt() time.Time {
	if raw, ok := t.Metadata[SDKTokenMetadataFamilyCreatedAt].(string); ok {
		if born, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return born
		}
	}
	return t.CreatedAt
}

// SetFamilyCreatedAt writes the family's birth into the row's metadata.
func (t *SDKToken) SetFamilyCreatedAt(born time.Time) {
	if t.Metadata == nil {
		t.Metadata = map[string]interface{}{}
	}
	t.Metadata[SDKTokenMetadataFamilyCreatedAt] = born.UTC().Format(time.RFC3339Nano)
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

	// GetByParentID retrieves the row that rotation created from the user's
	// row parentID: the row naming parentID in its parent_token metadata.
	// Rotation retires a row once, so at most one row descends from it;
	// where more than one does, the earliest created is returned
	GetByParentID(userID uuid.UUID, parentID uuid.UUID) (*SDKToken, error)

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

	// Rotate revokes the active token whose hash is oldTokenHash and stores
	// next in one transaction: both happen or neither does
	Rotate(oldTokenHash string, reason string, next *SDKToken) error

	// RevokeAllForUser revokes all tokens for a user
	RevokeAllForUser(userID uuid.UUID, reason string) error

	// RevokeFamily revokes every active row of a user's SDK-download token
	// family: the download's row (its token ID is the family's id) and every
	// row that descends from it by rotation (each names the row it replaced
	// in its parent_token metadata)
	RevokeFamily(userID uuid.UUID, familyID string, reason string) error

	// RecordUsage updates token usage statistics
	RecordUsage(tokenID string, ipAddress string) error

	// DeleteExpired removes expired tokens (cleanup job)
	DeleteExpired() error

	// GetActiveCount returns count of active tokens for a user
	GetActiveCount(userID uuid.UUID) (int, error)
}
