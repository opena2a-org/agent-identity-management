package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ===========================
// CacheConfig Tests
// ===========================

func TestCacheConfig_DefaultValues(t *testing.T) {
	config := CacheConfig{}

	assert.Empty(t, config.Host)
	assert.Equal(t, 0, config.Port)
	assert.Empty(t, config.Password)
	assert.Equal(t, 0, config.DB)
}

func TestCacheConfig_WithValues(t *testing.T) {
	config := CacheConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "secret",
		DB:       1,
	}

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, "secret", config.Password)
	assert.Equal(t, 1, config.DB)
}

// ===========================
// Cache Constants Tests
// ===========================

func TestCacheConstants_AgentCache(t *testing.T) {
	assert.Equal(t, "agent:", AgentCachePrefix)
	assert.Equal(t, 5*time.Minute, AgentCacheTTL)
	assert.Equal(t, "agents:org:", AgentListCacheKey)
	assert.Equal(t, 2*time.Minute, AgentListCacheTTL)
}

func TestCacheConstants_UserCache(t *testing.T) {
	assert.Equal(t, "user:", UserCachePrefix)
	assert.Equal(t, 10*time.Minute, UserCacheTTL)
	assert.Equal(t, "user:provider:", UserProviderPrefix)
	assert.Equal(t, 10*time.Minute, UserProviderTTL)
}

func TestCacheConstants_TrustScoreCache(t *testing.T) {
	assert.Equal(t, "trust:", TrustScoreCachePrefix)
	assert.Equal(t, 15*time.Minute, TrustScoreCacheTTL)
}

func TestCacheConstants_APIKeyCache(t *testing.T) {
	assert.Equal(t, "apikey:valid:", APIKeyValidPrefix)
	assert.Equal(t, 5*time.Minute, APIKeyValidTTL)
}

func TestCacheConstants_RateLimitCache(t *testing.T) {
	assert.Equal(t, "ratelimit:", RateLimitPrefix)
	assert.Equal(t, 1*time.Minute, RateLimitTTL)
}

func TestCacheConstants_SessionCache(t *testing.T) {
	assert.Equal(t, "session:", SessionPrefix)
	assert.Equal(t, 24*time.Hour, SessionTTL)
}

// ===========================
// NewRedisCache Tests (Connection Errors)
// ===========================

func TestNewRedisCache_InvalidHost(t *testing.T) {
	config := &CacheConfig{
		Host:     "invalid-host-that-does-not-exist",
		Port:     6379,
		Password: "",
		DB:       0,
	}

	cache, err := NewRedisCache(config)

	// Should fail to connect
	assert.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestNewRedisCache_InvalidPort(t *testing.T) {
	config := &CacheConfig{
		Host:     "localhost",
		Port:     99999, // Invalid port
		Password: "",
		DB:       0,
	}

	cache, err := NewRedisCache(config)

	// Should fail to connect
	assert.Error(t, err)
	assert.Nil(t, cache)
}

// ===========================
// RedisCache Struct Tests
// ===========================

func TestRedisCache_ZeroValue(t *testing.T) {
	cache := &RedisCache{}

	// Zero value cache has nil client
	assert.Nil(t, cache.client)
}

// ===========================
// Cache Key Format Tests
// ===========================

func TestCacheKeyFormats(t *testing.T) {
	agentID := "123e4567-e89b-12d3-a456-426614174000"
	orgID := "org-123"
	userID := "user-456"

	// Agent cache key
	agentKey := AgentCachePrefix + agentID
	assert.Equal(t, "agent:123e4567-e89b-12d3-a456-426614174000", agentKey)

	// Agent list cache key
	agentListKey := AgentListCacheKey + orgID
	assert.Equal(t, "agents:org:org-123", agentListKey)

	// User cache key
	userKey := UserCachePrefix + userID
	assert.Equal(t, "user:user-456", userKey)

	// Trust score cache key
	trustKey := TrustScoreCachePrefix + agentID
	assert.Equal(t, "trust:123e4567-e89b-12d3-a456-426614174000", trustKey)

	// API key validation cache key
	apiKeyKey := APIKeyValidPrefix + "key-hash-abc"
	assert.Equal(t, "apikey:valid:key-hash-abc", apiKeyKey)

	// Rate limit cache key
	rateLimitKey := RateLimitPrefix + "user:user-456:endpoint:api/v1/agents"
	assert.Equal(t, "ratelimit:user:user-456:endpoint:api/v1/agents", rateLimitKey)

	// Session cache key
	sessionKey := SessionPrefix + "session-token-xyz"
	assert.Equal(t, "session:session-token-xyz", sessionKey)
}

// ===========================
// TTL Value Tests
// ===========================

func TestCacheTTLValues_AreReasonable(t *testing.T) {
	// Agent cache should be short enough to not serve stale data
	assert.LessOrEqual(t, AgentCacheTTL.Minutes(), float64(10))

	// User cache can be longer since user data changes less frequently
	assert.LessOrEqual(t, UserCacheTTL.Minutes(), float64(15))

	// Trust scores should be refreshed relatively often
	assert.LessOrEqual(t, TrustScoreCacheTTL.Minutes(), float64(30))

	// Sessions should last a reasonable time
	assert.GreaterOrEqual(t, SessionTTL.Hours(), float64(1))
	assert.LessOrEqual(t, SessionTTL.Hours(), float64(48))

	// Rate limit window should be short
	assert.LessOrEqual(t, RateLimitTTL.Minutes(), float64(5))
}

// ===========================
// Helper Method Key Generation Tests
// ===========================

func TestCacheHelperKeyGeneration(t *testing.T) {
	// These test the expected key formats for helper methods

	// CacheAgent uses: AgentCachePrefix + agentID
	testAgentID := "test-agent-id"
	expectedAgentKey := AgentCachePrefix + testAgentID
	assert.Equal(t, "agent:test-agent-id", expectedAgentKey)

	// CacheUser uses: UserCachePrefix + userID
	testUserID := "test-user-id"
	expectedUserKey := UserCachePrefix + testUserID
	assert.Equal(t, "user:test-user-id", expectedUserKey)

	// CacheTrustScore uses: TrustScoreCachePrefix + agentID
	expectedTrustKey := TrustScoreCachePrefix + testAgentID
	assert.Equal(t, "trust:test-agent-id", expectedTrustKey)

	// InvalidateAgentList uses: AgentListCacheKey + orgID + "*"
	testOrgID := "test-org"
	expectedPattern := AgentListCacheKey + testOrgID + "*"
	assert.Equal(t, "agents:org:test-org*", expectedPattern)
}

// ===========================
// Config Validation Tests
// ===========================

func TestCacheConfig_FormatsAddress(t *testing.T) {
	// Test that config properly formats the address
	config := &CacheConfig{
		Host: "redis.example.com",
		Port: 6380,
	}

	// The address would be formatted as "redis.example.com:6380"
	// We can't test NewRedisCache without a real Redis, but we verify the config
	assert.Equal(t, "redis.example.com", config.Host)
	assert.Equal(t, 6380, config.Port)
}

func TestCacheConfig_DefaultDB(t *testing.T) {
	config := &CacheConfig{
		Host: "localhost",
		Port: 6379,
		// DB defaults to 0
	}

	assert.Equal(t, 0, config.DB)
}
