package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Get Tests with Mock
// ===========================

func TestRedisCache_Get_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"name": "test"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectGet("test:key").SetVal(string(jsonData))

	var result map[string]string
	err := cache.Get(context.Background(), "test:key", &result)

	require.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Get_CacheMiss(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectGet("nonexistent:key").RedisNil()

	var result map[string]string
	err := cache.Get(context.Background(), "nonexistent:key", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Get_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectGet("test:key").SetErr(errors.New("connection error"))

	var result map[string]string
	err := cache.Get(context.Background(), "test:key", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Get_InvalidJSON(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectGet("test:key").SetVal("not valid json")

	var result map[string]string
	err := cache.Get(context.Background(), "test:key", &result)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// Set Tests with Mock
// ===========================

func TestRedisCache_Set_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"name": "test"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectSet("test:key", jsonData, 5*time.Minute).SetVal("OK")

	err := cache.Set(context.Background(), "test:key", testData, 5*time.Minute)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Set_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"name": "test"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectSet("test:key", jsonData, 5*time.Minute).SetErr(errors.New("connection error"))

	err := cache.Set(context.Background(), "test:key", testData, 5*time.Minute)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Set_MarshalError(t *testing.T) {
	db, _ := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	// Channels cannot be marshaled to JSON
	badData := make(chan int)

	err := cache.Set(context.Background(), "test:key", badData, 5*time.Minute)

	assert.Error(t, err)
}

// ===========================
// Delete Tests with Mock
// ===========================

func TestRedisCache_Delete_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectDel("test:key").SetVal(1)

	err := cache.Delete(context.Background(), "test:key")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Delete_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectDel("test:key").SetErr(errors.New("connection error"))

	err := cache.Delete(context.Background(), "test:key")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// DeletePattern Tests with Mock
// ===========================

func TestRedisCache_DeletePattern_NoKeys(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectScan(0, "test:*", int64(100)).SetVal([]string{}, 0)

	err := cache.DeletePattern(context.Background(), "test:*")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_DeletePattern_WithKeys(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	keys := []string{"test:1", "test:2", "test:3"}
	mock.ExpectScan(0, "test:*", int64(100)).SetVal(keys, 0)
	mock.ExpectDel(keys...).SetVal(3)

	err := cache.DeletePattern(context.Background(), "test:*")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_DeletePattern_ScanError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectScan(0, "test:*", int64(100)).SetErr(errors.New("scan error"))

	err := cache.DeletePattern(context.Background(), "test:*")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_DeletePattern_DeleteError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	keys := []string{"test:1", "test:2"}
	mock.ExpectScan(0, "test:*", int64(100)).SetVal(keys, 0)
	mock.ExpectDel(keys...).SetErr(errors.New("delete error"))

	err := cache.DeletePattern(context.Background(), "test:*")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_DeletePattern_MultipleBatches(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	// First batch
	keys1 := []string{"test:1", "test:2"}
	mock.ExpectScan(0, "test:*", int64(100)).SetVal(keys1, 5) // cursor 5 means more data
	mock.ExpectDel(keys1...).SetVal(2)

	// Second batch
	keys2 := []string{"test:3"}
	mock.ExpectScan(5, "test:*", int64(100)).SetVal(keys2, 0) // cursor 0 means done
	mock.ExpectDel(keys2...).SetVal(1)

	err := cache.DeletePattern(context.Background(), "test:*")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// Exists Tests with Mock
// ===========================

func TestRedisCache_Exists_True(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectExists("test:key").SetVal(1)

	exists, err := cache.Exists(context.Background(), "test:key")

	require.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Exists_False(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectExists("test:key").SetVal(0)

	exists, err := cache.Exists(context.Background(), "test:key")

	require.NoError(t, err)
	assert.False(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Exists_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectExists("test:key").SetErr(errors.New("connection error"))

	exists, err := cache.Exists(context.Background(), "test:key")

	assert.Error(t, err)
	assert.False(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// Increment Tests with Mock
// ===========================

func TestRedisCache_Increment_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectIncr("counter:key").SetVal(5)

	result, err := cache.Increment(context.Background(), "counter:key")

	require.NoError(t, err)
	assert.Equal(t, int64(5), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_Increment_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectIncr("counter:key").SetErr(errors.New("connection error"))

	_, err := cache.Increment(context.Background(), "counter:key")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// IncrementBy Tests with Mock
// ===========================

func TestRedisCache_IncrementBy_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectIncrBy("counter:key", int64(10)).SetVal(15)

	result, err := cache.IncrementBy(context.Background(), "counter:key", 10)

	require.NoError(t, err)
	assert.Equal(t, int64(15), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_IncrementBy_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectIncrBy("counter:key", int64(10)).SetErr(errors.New("connection error"))

	_, err := cache.IncrementBy(context.Background(), "counter:key", 10)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// SetWithNX Tests with Mock
// ===========================

func TestRedisCache_SetWithNX_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"lock": "value"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectSetNX("lock:key", jsonData, 30*time.Second).SetVal(true)

	acquired, err := cache.SetWithNX(context.Background(), "lock:key", testData, 30*time.Second)

	require.NoError(t, err)
	assert.True(t, acquired)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_SetWithNX_AlreadyExists(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"lock": "value"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectSetNX("lock:key", jsonData, 30*time.Second).SetVal(false)

	acquired, err := cache.SetWithNX(context.Background(), "lock:key", testData, 30*time.Second)

	require.NoError(t, err)
	assert.False(t, acquired)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_SetWithNX_MarshalError(t *testing.T) {
	db, _ := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	// Channels cannot be marshaled
	badData := make(chan int)

	acquired, err := cache.SetWithNX(context.Background(), "lock:key", badData, 30*time.Second)

	assert.Error(t, err)
	assert.False(t, acquired)
}

func TestRedisCache_SetWithNX_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	testData := map[string]string{"lock": "value"}
	jsonData, _ := json.Marshal(testData)

	mock.ExpectSetNX("lock:key", jsonData, 30*time.Second).SetErr(errors.New("connection error"))

	acquired, err := cache.SetWithNX(context.Background(), "lock:key", testData, 30*time.Second)

	assert.Error(t, err)
	assert.False(t, acquired)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// GetTTL Tests with Mock
// ===========================

func TestRedisCache_GetTTL_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectTTL("test:key").SetVal(5 * time.Minute)

	ttl, err := cache.GetTTL(context.Background(), "test:key")

	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, ttl)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_GetTTL_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectTTL("test:key").SetErr(errors.New("connection error"))

	_, err := cache.GetTTL(context.Background(), "test:key")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// Close Tests
// ===========================

func TestRedisCache_Close_NilClient(t *testing.T) {
	// Test with a real mock client - Close() just delegates to client.Close()
	db, _ := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	// Close should work without error on mock client
	err := cache.Close()

	// Mock client close returns nil
	assert.NoError(t, err)
}

// ===========================
// Helper Methods Tests with Mock
// ===========================

func TestRedisCache_CacheAgent_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	agent := map[string]string{"id": "agent-123", "name": "TestAgent"}
	jsonData, _ := json.Marshal(agent)

	mock.ExpectSet(AgentCachePrefix+"agent-123", jsonData, AgentCacheTTL).SetVal("OK")

	err := cache.CacheAgent(context.Background(), "agent-123", agent)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_GetCachedAgent_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	agent := map[string]string{"id": "agent-123", "name": "TestAgent"}
	jsonData, _ := json.Marshal(agent)

	mock.ExpectGet(AgentCachePrefix + "agent-123").SetVal(string(jsonData))

	var result map[string]string
	err := cache.GetCachedAgent(context.Background(), "agent-123", &result)

	require.NoError(t, err)
	assert.Equal(t, "agent-123", result["id"])
	assert.Equal(t, "TestAgent", result["name"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_GetCachedAgent_NotFound(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectGet(AgentCachePrefix + "nonexistent").RedisNil()

	var result map[string]string
	err := cache.GetCachedAgent(context.Background(), "nonexistent", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache miss")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_InvalidateAgent_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	mock.ExpectDel(AgentCachePrefix + "agent-123").SetVal(1)

	err := cache.InvalidateAgent(context.Background(), "agent-123")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_InvalidateAgentList_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	pattern := AgentListCacheKey + "org-123" + "*"
	mock.ExpectScan(0, pattern, int64(100)).SetVal([]string{}, 0)

	err := cache.InvalidateAgentList(context.Background(), "org-123")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_CacheUser_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	user := map[string]string{"id": "user-123", "email": "test@example.com"}
	jsonData, _ := json.Marshal(user)

	mock.ExpectSet(UserCachePrefix+"user-123", jsonData, UserCacheTTL).SetVal("OK")

	err := cache.CacheUser(context.Background(), "user-123", user)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_GetCachedUser_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	user := map[string]string{"id": "user-123", "email": "test@example.com"}
	jsonData, _ := json.Marshal(user)

	mock.ExpectGet(UserCachePrefix + "user-123").SetVal(string(jsonData))

	var result map[string]string
	err := cache.GetCachedUser(context.Background(), "user-123", &result)

	require.NoError(t, err)
	assert.Equal(t, "user-123", result["id"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_CacheTrustScore_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	score := map[string]interface{}{"score": 0.95, "agentId": "agent-123"}
	jsonData, _ := json.Marshal(score)

	mock.ExpectSet(TrustScoreCachePrefix+"agent-123", jsonData, TrustScoreCacheTTL).SetVal("OK")

	err := cache.CacheTrustScore(context.Background(), "agent-123", score)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_GetCachedTrustScore_Success(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	score := map[string]interface{}{"score": 0.95, "agentId": "agent-123"}
	jsonData, _ := json.Marshal(score)

	mock.ExpectGet(TrustScoreCachePrefix + "agent-123").SetVal(string(jsonData))

	var result map[string]interface{}
	err := cache.GetCachedTrustScore(context.Background(), "agent-123", &result)

	require.NoError(t, err)
	assert.Equal(t, 0.95, result["score"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================
// RateLimit Tests with Mock
// ===========================

func TestRedisCache_RateLimit_FirstRequest(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	key := RateLimitPrefix + "user:123:endpoint"
	mock.ExpectIncr(key).SetVal(1)
	mock.ExpectExpire(key, 1*time.Minute).SetVal(true)

	allowed, err := cache.RateLimit(context.Background(), "user:123:endpoint", 100, 1*time.Minute)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_RateLimit_SubsequentRequest(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	key := RateLimitPrefix + "user:123:endpoint"
	mock.ExpectIncr(key).SetVal(50) // Not first request

	allowed, err := cache.RateLimit(context.Background(), "user:123:endpoint", 100, 1*time.Minute)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_RateLimit_Exceeded(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	key := RateLimitPrefix + "user:123:endpoint"
	mock.ExpectIncr(key).SetVal(101) // Over limit of 100

	allowed, err := cache.RateLimit(context.Background(), "user:123:endpoint", 100, 1*time.Minute)

	require.NoError(t, err)
	assert.False(t, allowed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCache_RateLimit_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	cache := &RedisCache{client: db}

	key := RateLimitPrefix + "user:123:endpoint"
	mock.ExpectIncr(key).SetErr(errors.New("connection error"))

	allowed, err := cache.RateLimit(context.Background(), "user:123:endpoint", 100, 1*time.Minute)

	assert.Error(t, err)
	assert.False(t, allowed)
	assert.NoError(t, mock.ExpectationsWereMet())
}
