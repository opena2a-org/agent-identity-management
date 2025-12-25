package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPIKeyStruct(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(90 * 24 * time.Hour)
	keyID := uuid.New()
	orgID := uuid.New()
	agentID := uuid.New()
	createdBy := uuid.New()

	apiKey := APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		AgentID:        agentID,
		AgentName:      "test-agent",
		Name:           "Production API Key",
		KeyHash:        "sha256hashvalue",
		Prefix:         "aim_live",
		LastUsedAt:     &now,
		ExpiresAt:      &expiresAt,
		IsActive:       true,
		CreatedAt:      now,
		CreatedBy:      createdBy,
	}

	assert.Equal(t, keyID, apiKey.ID)
	assert.Equal(t, orgID, apiKey.OrganizationID)
	assert.Equal(t, agentID, apiKey.AgentID)
	assert.Equal(t, "test-agent", apiKey.AgentName)
	assert.Equal(t, "Production API Key", apiKey.Name)
	assert.Equal(t, "sha256hashvalue", apiKey.KeyHash)
	assert.Equal(t, "aim_live", apiKey.Prefix)
	assert.True(t, apiKey.IsActive)
}

func TestAPIKeyWithExpiration(t *testing.T) {
	now := time.Now()

	// Expired key
	expiredTime := now.Add(-24 * time.Hour)
	expiredKey := APIKey{
		ID:        uuid.New(),
		Name:      "Expired Key",
		ExpiresAt: &expiredTime,
		IsActive:  true,
	}

	assert.True(t, expiredKey.ExpiresAt.Before(now))

	// Future expiration
	futureTime := now.Add(30 * 24 * time.Hour)
	validKey := APIKey{
		ID:        uuid.New(),
		Name:      "Valid Key",
		ExpiresAt: &futureTime,
		IsActive:  true,
	}

	assert.True(t, validKey.ExpiresAt.After(now))
}

func TestAPIKeyWithNoExpiration(t *testing.T) {
	apiKey := APIKey{
		ID:        uuid.New(),
		Name:      "Non-expiring Key",
		ExpiresAt: nil,
		IsActive:  true,
	}

	assert.Nil(t, apiKey.ExpiresAt)
}

func TestAPIKeyActiveStatus(t *testing.T) {
	activeKey := APIKey{
		ID:       uuid.New(),
		Name:     "Active Key",
		IsActive: true,
	}

	revokedKey := APIKey{
		ID:       uuid.New(),
		Name:     "Revoked Key",
		IsActive: false,
	}

	assert.True(t, activeKey.IsActive)
	assert.False(t, revokedKey.IsActive)
}

func TestAPIKeyLastUsed(t *testing.T) {
	// Never used
	neverUsedKey := APIKey{
		ID:         uuid.New(),
		Name:       "Never Used Key",
		LastUsedAt: nil,
	}

	assert.Nil(t, neverUsedKey.LastUsedAt)

	// Recently used
	now := time.Now()
	recentlyUsedKey := APIKey{
		ID:         uuid.New(),
		Name:       "Recently Used Key",
		LastUsedAt: &now,
	}

	assert.NotNil(t, recentlyUsedKey.LastUsedAt)
	assert.Equal(t, &now, recentlyUsedKey.LastUsedAt)
}

func TestAPIKeyPrefix(t *testing.T) {
	prefixes := []string{
		"aim_live",
		"aim_test",
		"aim_dev_",
		"sk_live_",
		"pk_test_",
	}

	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			apiKey := APIKey{
				ID:     uuid.New(),
				Name:   "Key with prefix " + prefix,
				Prefix: prefix,
			}
			assert.Equal(t, prefix, apiKey.Prefix)
		})
	}
}

func TestAPIKeyWithNilOptionalFields(t *testing.T) {
	apiKey := APIKey{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AgentID:        uuid.New(),
		Name:           "Minimal Key",
		KeyHash:        "hash",
		Prefix:         "aim_",
		IsActive:       true,
		CreatedAt:      time.Now(),
		CreatedBy:      uuid.New(),
	}

	// Verify nil optional fields don't cause issues
	assert.Nil(t, apiKey.LastUsedAt)
	assert.Nil(t, apiKey.ExpiresAt)
	assert.Empty(t, apiKey.AgentName) // Not set, defaults to empty
}
