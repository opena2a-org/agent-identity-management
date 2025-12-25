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
