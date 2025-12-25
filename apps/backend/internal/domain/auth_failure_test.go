package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthFailureType_Constants(t *testing.T) {
	tests := []struct {
		constant AuthFailureType
		expected string
	}{
		{AuthFailureInvalidCredentials, "invalid_credentials"},
		{AuthFailureAccountLocked, "account_locked"},
		{AuthFailureAccountDeactivated, "account_deactivated"},
		{AuthFailureInvalidToken, "invalid_token"},
		{AuthFailureExpiredToken, "expired_token"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, AuthFailureType(tt.expected), tt.constant)
		})
	}
}

func TestAuthFailure_Structure(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()
	userID := uuid.New()

	failure := AuthFailure{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		UserID:         &userID,
		Email:          "user@example.com",
		FailureType:    AuthFailureInvalidCredentials,
		IPAddress:      "192.168.1.100",
		UserAgent:      "Mozilla/5.0",
		Metadata: map[string]interface{}{
			"attempt_number": 3,
			"source":         "web",
		},
		CreatedAt: now,
	}

	assert.NotEqual(t, uuid.Nil, failure.ID)
	assert.Equal(t, &orgID, failure.OrganizationID)
	assert.Equal(t, &userID, failure.UserID)
	assert.Equal(t, "user@example.com", failure.Email)
	assert.Equal(t, AuthFailureInvalidCredentials, failure.FailureType)
	assert.Equal(t, "192.168.1.100", failure.IPAddress)
	assert.Equal(t, "Mozilla/5.0", failure.UserAgent)
	assert.Equal(t, 3, failure.Metadata["attempt_number"])
}

func TestAuthFailure_UnknownUser(t *testing.T) {
	failure := AuthFailure{
		ID:             uuid.New(),
		OrganizationID: nil, // Unknown org
		UserID:         nil, // Unknown user
		Email:          "unknown@attacker.com",
		FailureType:    AuthFailureInvalidCredentials,
		IPAddress:      "203.0.113.50",
		UserAgent:      "curl/7.81.0",
		CreatedAt:      time.Now(),
	}

	assert.Nil(t, failure.OrganizationID)
	assert.Nil(t, failure.UserID)
	assert.NotEmpty(t, failure.Email) // Email is always captured
}

func TestAuthFailure_InvalidToken(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	failure := AuthFailure{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		UserID:         &userID,
		Email:          "user@example.com",
		FailureType:    AuthFailureInvalidToken,
		IPAddress:      "10.0.0.50",
		UserAgent:      "API-Client/1.0",
		Metadata: map[string]interface{}{
			"token_type": "jwt",
			"error":      "signature_invalid",
		},
		CreatedAt: time.Now(),
	}

	assert.Equal(t, AuthFailureInvalidToken, failure.FailureType)
	assert.Equal(t, "signature_invalid", failure.Metadata["error"])
}

func TestAuthFailure_ExpiredToken(t *testing.T) {
	failure := AuthFailure{
		ID:          uuid.New(),
		Email:       "user@example.com",
		FailureType: AuthFailureExpiredToken,
		IPAddress:   "192.168.1.1",
		UserAgent:   "SDK/2.0",
		Metadata: map[string]interface{}{
			"expired_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		},
		CreatedAt: time.Now(),
	}

	assert.Equal(t, AuthFailureExpiredToken, failure.FailureType)
}

func TestAuthLockout_Structure(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()
	userID := uuid.New()

	lockout := AuthLockout{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		UserID:         &userID,
		Email:          "locked@example.com",
		FailedAttempts: 5,
		LockedAt:       now,
		LockedUntil:    now.Add(30 * time.Minute),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, lockout.ID)
	assert.Equal(t, &orgID, lockout.OrganizationID)
	assert.Equal(t, &userID, lockout.UserID)
	assert.Equal(t, "locked@example.com", lockout.Email)
	assert.Equal(t, 5, lockout.FailedAttempts)
	assert.Equal(t, now, lockout.LockedAt)
	assert.Equal(t, now.Add(30*time.Minute), lockout.LockedUntil)
	assert.Nil(t, lockout.UnlockedAt)
	assert.Nil(t, lockout.UnlockedBy)
}

func TestAuthLockout_AdminUnlock(t *testing.T) {
	now := time.Now()
	adminID := uuid.New()
	unlockTime := now.Add(15 * time.Minute)

	lockout := AuthLockout{
		ID:             uuid.New(),
		Email:          "locked@example.com",
		FailedAttempts: 5,
		LockedAt:       now,
		LockedUntil:    now.Add(30 * time.Minute),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Initially not unlocked
	assert.Nil(t, lockout.UnlockedAt)
	assert.Nil(t, lockout.UnlockedBy)

	// Simulate admin unlock
	lockout.UnlockedAt = &unlockTime
	lockout.UnlockedBy = &adminID
	lockout.UpdatedAt = unlockTime

	assert.Equal(t, &unlockTime, lockout.UnlockedAt)
	assert.Equal(t, &adminID, lockout.UnlockedBy)
}

func TestAuthLockout_IsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		lockedUntil    time.Time
		unlockedAt     *time.Time
		expectedActive bool
	}{
		{
			name:           "active lockout - future unlock time",
			lockedUntil:    now.Add(30 * time.Minute),
			unlockedAt:     nil,
			expectedActive: true,
		},
		{
			name:           "expired lockout - past unlock time",
			lockedUntil:    now.Add(-10 * time.Minute),
			unlockedAt:     nil,
			expectedActive: false,
		},
		{
			name:           "admin unlocked",
			lockedUntil:    now.Add(30 * time.Minute),
			unlockedAt:     func() *time.Time { t := now.Add(-5 * time.Minute); return &t }(),
			expectedActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockout := AuthLockout{
				ID:          uuid.New(),
				Email:       "test@example.com",
				LockedAt:    now.Add(-time.Hour),
				LockedUntil: tt.lockedUntil,
				UnlockedAt:  tt.unlockedAt,
			}

			// Check if lockout is still active
			isActive := lockout.UnlockedAt == nil && time.Now().Before(lockout.LockedUntil)
			assert.Equal(t, tt.expectedActive, isActive)
		})
	}
}

func TestDefaultAuthFailureConfig(t *testing.T) {
	config := DefaultAuthFailureConfig()

	assert.Equal(t, 5, config.MaxAttempts)
	assert.Equal(t, 15, config.TimeWindowMins)
	assert.Equal(t, 30, config.LockoutMins)
	assert.Equal(t, 3, config.AlertThreshold)
}

func TestAuthFailureConfig_CustomValues(t *testing.T) {
	config := AuthFailureConfig{
		MaxAttempts:    10,
		TimeWindowMins: 30,
		LockoutMins:    60,
		AlertThreshold: 5,
	}

	assert.Equal(t, 10, config.MaxAttempts)
	assert.Equal(t, 30, config.TimeWindowMins)
	assert.Equal(t, 60, config.LockoutMins)
	assert.Equal(t, 5, config.AlertThreshold)
}

func TestAuthFailure_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	orgID := uuid.New()
	userID := uuid.New()

	failure := AuthFailure{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		UserID:         &userID,
		Email:          "test@example.com",
		FailureType:    AuthFailureInvalidCredentials,
		IPAddress:      "192.168.1.1",
		UserAgent:      "TestAgent/1.0",
		Metadata:       map[string]interface{}{"key": "value"},
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(failure)
	require.NoError(t, err)

	// Deserialize back
	var restored AuthFailure
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, failure.ID, restored.ID)
	assert.Equal(t, failure.Email, restored.Email)
	assert.Equal(t, failure.FailureType, restored.FailureType)
	assert.Equal(t, failure.IPAddress, restored.IPAddress)
}

func TestAuthLockout_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	lockout := AuthLockout{
		ID:             uuid.New(),
		Email:          "locked@example.com",
		FailedAttempts: 5,
		LockedAt:       now,
		LockedUntil:    now.Add(30 * time.Minute),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(lockout)
	require.NoError(t, err)

	// Deserialize back
	var restored AuthLockout
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, lockout.ID, restored.ID)
	assert.Equal(t, lockout.Email, restored.Email)
	assert.Equal(t, lockout.FailedAttempts, restored.FailedAttempts)
}

func TestAuthFailure_AllTypes(t *testing.T) {
	types := []AuthFailureType{
		AuthFailureInvalidCredentials,
		AuthFailureAccountLocked,
		AuthFailureAccountDeactivated,
		AuthFailureInvalidToken,
		AuthFailureExpiredToken,
	}

	for _, failureType := range types {
		t.Run(string(failureType), func(t *testing.T) {
			failure := AuthFailure{
				ID:          uuid.New(),
				Email:       "test@example.com",
				FailureType: failureType,
				IPAddress:   "127.0.0.1",
				UserAgent:   "Test",
				CreatedAt:   time.Now(),
			}

			assert.Equal(t, failureType, failure.FailureType)
			assert.NotEmpty(t, string(failureType))
		})
	}
}
