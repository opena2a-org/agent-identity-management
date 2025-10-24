package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache provides caching layer
type RedisCache struct {
	client *redis.Client
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(config *CacheConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("cache miss: %s", key)
	}
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set stores a value in cache with TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a value from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// DeletePattern deletes all keys matching a pattern
func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Exists checks if a key exists
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Increment increments a counter
func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// IncrementBy increments a counter by a specific amount
func (c *RedisCache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

// SetWithNX sets a value only if it doesn't exist (for distributed locks)
func (c *RedisCache) SetWithNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.client.SetNX(ctx, key, data, ttl).Result()
}

// GetTTL returns the remaining TTL of a key
func (c *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Common cache keys and TTLs
const (
	// Agent cache
	AgentCachePrefix    = "agent:"
	AgentCacheTTL       = 5 * time.Minute
	AgentListCacheKey   = "agents:org:"
	AgentListCacheTTL   = 2 * time.Minute

	// User cache
	UserCachePrefix     = "user:"
	UserCacheTTL        = 10 * time.Minute
	UserProviderPrefix  = "user:provider:"
	UserProviderTTL     = 10 * time.Minute

	// Trust score cache
	TrustScoreCachePrefix = "trust:"
	TrustScoreCacheTTL    = 15 * time.Minute

	// API key validation cache
	APIKeyValidPrefix = "apikey:valid:"
	APIKeyValidTTL    = 5 * time.Minute

	// Rate limiting
	RateLimitPrefix = "ratelimit:"
	RateLimitTTL    = 1 * time.Minute

	// Session cache
	SessionPrefix = "session:"
	SessionTTL    = 24 * time.Hour
)

// Helper functions for common cache operations

// CacheAgent caches an agent
func (c *RedisCache) CacheAgent(ctx context.Context, agentID string, agent interface{}) error {
	return c.Set(ctx, AgentCachePrefix+agentID, agent, AgentCacheTTL)
}

// GetCachedAgent retrieves a cached agent
func (c *RedisCache) GetCachedAgent(ctx context.Context, agentID string, dest interface{}) error {
	return c.Get(ctx, AgentCachePrefix+agentID, dest)
}

// InvalidateAgent removes an agent from cache
func (c *RedisCache) InvalidateAgent(ctx context.Context, agentID string) error {
	return c.Delete(ctx, AgentCachePrefix+agentID)
}

// InvalidateAgentList removes agent list cache for an organization
func (c *RedisCache) InvalidateAgentList(ctx context.Context, orgID string) error {
	return c.DeletePattern(ctx, AgentListCacheKey+orgID+"*")
}

// CacheUser caches a user
func (c *RedisCache) CacheUser(ctx context.Context, userID string, user interface{}) error {
	return c.Set(ctx, UserCachePrefix+userID, user, UserCacheTTL)
}

// GetCachedUser retrieves a cached user
func (c *RedisCache) GetCachedUser(ctx context.Context, userID string, dest interface{}) error {
	return c.Get(ctx, UserCachePrefix+userID, dest)
}

// CacheTrustScore caches a trust score
func (c *RedisCache) CacheTrustScore(ctx context.Context, agentID string, score interface{}) error {
	return c.Set(ctx, TrustScoreCachePrefix+agentID, score, TrustScoreCacheTTL)
}

// GetCachedTrustScore retrieves a cached trust score
func (c *RedisCache) GetCachedTrustScore(ctx context.Context, agentID string, dest interface{}) error {
	return c.Get(ctx, TrustScoreCachePrefix+agentID, dest)
}

// RateLimit implements rate limiting
func (c *RedisCache) RateLimit(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	fullKey := RateLimitPrefix + key
	count, err := c.Increment(ctx, fullKey)
	if err != nil {
		return false, err
	}

	if count == 1 {
		// First request, set expiration
		c.client.Expire(ctx, fullKey, window)
	}

	return count <= limit, nil
}
