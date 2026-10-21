package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSDKToken_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		token    SDKToken
		expected bool
	}{
		{
			name: "active token",
			token: SDKToken{
				ID:        uuid.New(),
				ExpiresAt: time.Now().Add(24 * time.Hour),
				RevokedAt: nil,
			},
			expected: true,
		},
		{
			name: "revoked token",
			token: SDKToken{
				ID:        uuid.New(),
				ExpiresAt: time.Now().Add(24 * time.Hour),
				RevokedAt: func() *time.Time { t := time.Now(); return &t }(),
			},
			expected: false,
		},
		{
			name: "expired token",
			token: SDKToken{
				ID:        uuid.New(),
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				RevokedAt: nil,
			},
			expected: false,
		},
		{
			name: "revoked and expired token",
			token: SDKToken{
				ID:        uuid.New(),
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				RevokedAt: func() *time.Time { t := time.Now(); return &t }(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsActive()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSDKToken_Revoke(t *testing.T) {
	token := SDKToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Verify initially not revoked
	assert.Nil(t, token.RevokedAt)
	assert.Nil(t, token.RevokeReason)

	// Revoke token
	token.Revoke("User requested revocation")

	// Verify revoked
	assert.NotNil(t, token.RevokedAt)
	assert.NotNil(t, token.RevokeReason)
	assert.Equal(t, "User requested revocation", *token.RevokeReason)
	assert.False(t, token.IsActive())
}

func TestSDKToken_RecordUsage(t *testing.T) {
	token := SDKToken{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		UsageCount: 0,
	}

	// Verify initially no usage
	assert.Nil(t, token.LastUsedAt)
	assert.Nil(t, token.LastIPAddress)
	assert.Equal(t, 0, token.UsageCount)

	// Record usage
	token.RecordUsage("192.168.1.100")

	// Verify usage recorded
	assert.NotNil(t, token.LastUsedAt)
	assert.NotNil(t, token.LastIPAddress)
	assert.Equal(t, "192.168.1.100", *token.LastIPAddress)
	assert.Equal(t, 1, token.UsageCount)

	// Record more usage
	token.RecordUsage("10.0.0.50")

	assert.Equal(t, "10.0.0.50", *token.LastIPAddress)
	assert.Equal(t, 2, token.UsageCount)
}

func TestSDKToken_FullLifecycle(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	orgID := uuid.New()
	deviceName := "MacBook Pro"

	// Create token
	token := SDKToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: orgID,
		TokenHash:      "abc123hash",
		TokenID:        "jti_12345",
		DeviceName:     &deviceName,
		CreatedAt:      now,
		ExpiresAt:      now.Add(90 * 24 * time.Hour), // 90 days
	}

	// Initially active
	assert.True(t, token.IsActive())

	// Use token
	token.RecordUsage("192.168.1.1")
	assert.Equal(t, 1, token.UsageCount)
	assert.True(t, token.IsActive())

	// Use token again
	token.RecordUsage("192.168.1.2")
	assert.Equal(t, 2, token.UsageCount)

	// Revoke token
	token.Revoke("Security incident")
	assert.False(t, token.IsActive())
	assert.Equal(t, "Security incident", *token.RevokeReason)
}

func TestSDKToken_WithMetadata(t *testing.T) {
	token := SDKToken{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Metadata: map[string]interface{}{
			"browser":  "Chrome",
			"os":       "macOS",
			"version":  "1.0.0",
			"location": "US",
		},
	}

	assert.NotNil(t, token.Metadata)
	assert.Equal(t, "Chrome", token.Metadata["browser"])
	assert.Equal(t, "macOS", token.Metadata["os"])
	assert.Equal(t, "1.0.0", token.Metadata["version"])
}

func TestSDKToken_DeviceInfo(t *testing.T) {
	deviceName := "iPhone 15 Pro"
	deviceFingerprint := "fp_abc123"
	userAgent := "AIM-SDK/1.0 iOS/17.0"
	ipAddress := "203.0.113.50"

	token := SDKToken{
		ID:                uuid.New(),
		UserID:            uuid.New(),
		DeviceName:        &deviceName,
		DeviceFingerprint: &deviceFingerprint,
		UserAgent:         &userAgent,
		IPAddress:         &ipAddress,
	}

	assert.Equal(t, "iPhone 15 Pro", *token.DeviceName)
	assert.Equal(t, "fp_abc123", *token.DeviceFingerprint)
	assert.Equal(t, "AIM-SDK/1.0 iOS/17.0", *token.UserAgent)
	assert.Equal(t, "203.0.113.50", *token.IPAddress)
}

func TestSDKToken_ExpirationEdgeCases(t *testing.T) {
	// Token that expires exactly now
	now := time.Now()
	tokenExact := SDKToken{
		ID:        uuid.New(),
		ExpiresAt: now,
	}
	// Due to timing, this might be active or not
	// The key is that IsActive() doesn't panic

	_ = tokenExact.IsActive()

	// Token that expired a millisecond ago
	tokenJustExpired := SDKToken{
		ID:        uuid.New(),
		ExpiresAt: now.Add(-1 * time.Millisecond),
	}
	assert.False(t, tokenJustExpired.IsActive())

	// Token that expires in a millisecond
	tokenAboutToExpire := SDKToken{
		ID:        uuid.New(),
		ExpiresAt: now.Add(100 * time.Millisecond),
	}
	assert.True(t, tokenAboutToExpire.IsActive())
}

// A row's family was born when the row the download wrote was created. The
// birth travels in the row's metadata under familyCreatedAt; a row without
// it, or with a value that is not a time, is its family's root and is born
// at its own created_at.
func TestSDKToken_FamilyCreatedAt(t *testing.T) {
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	born := time.Date(2025, 11, 20, 8, 30, 15, 123456789, time.UTC)

	tests := []struct {
		name     string
		metadata map[string]interface{}
		expected time.Time
	}{
		{name: "no metadata", metadata: nil, expected: created},
		{name: "metadata without a birth", metadata: map[string]interface{}{"browser": "Chrome"}, expected: created},
		{name: "a birth that is not a string", metadata: map[string]interface{}{SDKTokenMetadataFamilyCreatedAt: 1763627415}, expected: created},
		{name: "a birth that is not a time", metadata: map[string]interface{}{SDKTokenMetadataFamilyCreatedAt: "yesterday"}, expected: created},
		{name: "a birth in RFC 3339", metadata: map[string]interface{}{SDKTokenMetadataFamilyCreatedAt: born.Format(time.RFC3339Nano)}, expected: born},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := SDKToken{ID: uuid.New(), CreatedAt: created, Metadata: tt.metadata}
			assert.True(t, tt.expected.Equal(token.FamilyCreatedAt()), "got %s", token.FamilyCreatedAt())
		})
	}
}

// SetFamilyCreatedAt writes the birth as an RFC 3339 time in UTC, into
// metadata it creates when the row has none, beside the keys the row already
// carries; FamilyCreatedAt reads it back.
func TestSDKToken_SetFamilyCreatedAt(t *testing.T) {
	born := time.Date(2025, 11, 20, 8, 30, 15, 123456789, time.FixedZone("MST", -7*3600))

	t.Run("a row without metadata", func(t *testing.T) {
		token := SDKToken{ID: uuid.New(), CreatedAt: time.Now()}
		token.SetFamilyCreatedAt(born)
		assert.Equal(t, "2025-11-20T15:30:15.123456789Z", token.Metadata[SDKTokenMetadataFamilyCreatedAt])
		assert.True(t, born.Equal(token.FamilyCreatedAt()))
	})

	t.Run("a row with metadata", func(t *testing.T) {
		token := SDKToken{ID: uuid.New(), CreatedAt: time.Now(), Metadata: map[string]interface{}{"browser": "Chrome"}}
		token.SetFamilyCreatedAt(born)
		assert.Equal(t, "Chrome", token.Metadata["browser"])
		assert.Equal(t, "2025-11-20T15:30:15.123456789Z", token.Metadata[SDKTokenMetadataFamilyCreatedAt])
		assert.True(t, born.Equal(token.FamilyCreatedAt()))
	})
}

func TestSDKToken_RevokedForNeedsTheRevocationAndItsReason(t *testing.T) {
	now := time.Now()
	reason, other := "password_changed", "user_revoked"

	assert.True(t, (&SDKToken{RevokedAt: &now, RevokeReason: &reason}).RevokedFor(reason))
	assert.False(t, (&SDKToken{}).RevokedFor(reason), "a token that is not revoked")
	assert.False(t, (&SDKToken{RevokedAt: &now}).RevokedFor(reason), "a token revoked without a reason")
	assert.False(t, (&SDKToken{RevokedAt: &now, RevokeReason: &other}).RevokedFor(reason), "a token revoked for another reason")
	assert.False(t, (&SDKToken{RevokeReason: &reason}).RevokedFor(reason), "a reason without a revocation time")
}
