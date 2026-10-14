package main

// Startup Redis connection by REDIS_HOST: set to the empty string, the server
// makes no Redis connection at all. Unset, the localhost default is still tried.

import (
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
)

// countingListener accepts on a loopback port, counts every connection and
// closes it at once, so a Redis ping against it fails fast instead of waiting.
func countingListener(t *testing.T) (port int, accepted func() int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var count atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			count.Add(1)
			_ = conn.Close()
		}
	}()

	accepted = func() int64 {
		_ = ln.Close()
		<-done
		return count.Load()
	}
	t.Cleanup(func() { accepted() })
	return ln.Addr().(*net.TCPAddr).Port, accepted
}

func loadConfigForRedisTest(t *testing.T, redisPort int) *config.Config {
	t.Helper()
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "aim")
	t.Setenv("POSTGRES_DB", "aim")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 64))
	t.Setenv("REDIS_PORT", strconv.Itoa(redisPort))
	cfg, err := config.Load()
	require.NoError(t, err)
	return cfg
}

func TestInitRedis_EmptyRedisHostMakesNoConnection(t *testing.T) {
	port, accepted := countingListener(t)
	t.Setenv("REDIS_HOST", "")

	client, err := initRedis(loadConfigForRedisTest(t, port))
	if client != nil {
		_ = client.Close()
	}

	assert.NoError(t, err, "an empty REDIS_HOST is not a connection failure")
	assert.Nil(t, client, "an empty REDIS_HOST gives no Redis client")
	assert.Equal(t, int64(0), accepted(), "an empty REDIS_HOST must not dial the localhost default")
}

func TestInitRedis_SetRedisHostIsDialed(t *testing.T) {
	port, accepted := countingListener(t)
	t.Setenv("REDIS_HOST", "127.0.0.1")

	client, err := initRedis(loadConfigForRedisTest(t, port))
	if client != nil {
		_ = client.Close()
	}

	assert.Error(t, err, "the listener closes every connection, so the startup ping fails")
	assert.Nil(t, client)
	assert.GreaterOrEqual(t, accepted(), int64(1), "a set REDIS_HOST is dialed")
}

// The cache line that follows the Redis line at startup gives the same reason:
// Redis turned off by an empty REDIS_HOST is not Redis that could not be reached.
func TestInitCache_EmptyRedisHostIsLoggedAsDisabled(t *testing.T) {
	port, _ := countingListener(t)
	t.Setenv("REDIS_HOST", "")
	cfg := loadConfigForRedisTest(t, port)
	client, err := initRedis(cfg)
	require.NoError(t, err)
	require.Nil(t, client)

	logged := captureLog(t)
	cacheService := initCache(cfg, client)

	assert.Nil(t, cacheService)
	assert.Equal(t, "ℹ️  Cache service skipped (Redis disabled)\n", logged.String())
}

// The startup summary's Redis line gives the reason too: with no connection,
// "disabled" is only for an empty REDIS_HOST. A failed ping names the address
// that was tried.
func TestRedisStartupSummary_EmptyRedisHostIsDisabled(t *testing.T) {
	port, _ := countingListener(t)
	t.Setenv("REDIS_HOST", "")
	cfg := loadConfigForRedisTest(t, port)
	client, err := initRedis(cfg)
	require.NoError(t, err)
	require.Nil(t, client)

	assert.Equal(t, "💾 Redis: disabled (running without caching)", redisStartupSummary(cfg, client))
}

func TestRedisStartupSummary_FailedRedisPingIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   bool
		value string
		host  string
	}{
		{name: "REDIS_HOST names a host", set: true, value: "127.0.0.1", host: "127.0.0.1"},
		{name: "REDIS_HOST is unset", host: "localhost"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port, _ := countingListener(t)
			// Setenv first, so the variable is restored after an unset.
			t.Setenv("REDIS_HOST", tc.value)
			if !tc.set {
				require.NoError(t, os.Unsetenv("REDIS_HOST"))
			}
			cfg := loadConfigForRedisTest(t, port)
			client, err := initRedis(cfg)
			require.Error(t, err, "the listener closes every connection, so the startup ping fails")
			require.Nil(t, client)

			assert.Equal(t,
				"💾 Redis: "+tc.host+":"+strconv.Itoa(port)+" (unavailable, running without caching)",
				redisStartupSummary(cfg, client))
		})
	}
}

func TestRedisStartupSummary_ConnectionIsConnected(t *testing.T) {
	port, _ := countingListener(t)
	t.Setenv("REDIS_HOST", "127.0.0.1")
	cfg := loadConfigForRedisTest(t, port)
	// The summary reads only whether there is a client, so one that has not
	// dialed stands in for a connection.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:" + strconv.Itoa(port)})
	t.Cleanup(func() { _ = client.Close() })

	assert.Equal(t, "💾 Redis: 127.0.0.1:"+strconv.Itoa(port)+" (connected)", redisStartupSummary(cfg, client))
}

// The token revocation line gives the cache line's reason too. "no Redis
// configured" is only for an empty REDIS_HOST: a Redis that was tried and could
// not be reached is unavailable, whether REDIS_HOST names it or is unset.
func TestRevocationStartupLine_GivesTheCacheLinesReason(t *testing.T) {
	const notRotated = ": refresh tokens are not rotated and logout cannot revoke"
	for _, tc := range []struct {
		name  string
		set   bool
		value string
		want  string
	}{
		{name: "REDIS_HOST is empty", set: true, value: "", want: "ℹ️  Token revocation disabled (no Redis configured)" + notRotated},
		{name: "REDIS_HOST names a host", set: true, value: "127.0.0.1", want: "ℹ️  Token revocation disabled (Redis unavailable)" + notRotated},
		{name: "REDIS_HOST is unset", want: "ℹ️  Token revocation disabled (Redis unavailable)" + notRotated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port, _ := countingListener(t)
			// Setenv first, so the variable is restored after an unset.
			t.Setenv("REDIS_HOST", tc.value)
			if !tc.set {
				require.NoError(t, os.Unsetenv("REDIS_HOST"))
			}
			cfg := loadConfigForRedisTest(t, port)
			client, _ := initRedis(cfg)
			require.Nil(t, client, "the listener closes every connection, so there is no Redis client")
			logged := captureLog(t)
			cacheService := initCache(cfg, client)
			require.Nil(t, cacheService)

			assert.Equal(t, tc.want, tokenRevocationStartupLine(cfg, client, cacheService, false))
			cacheLine := strings.TrimSuffix(logged.String(), "\n")
			reason := cacheLine[strings.Index(cacheLine, "("):]
			if cfg.Redis.Disabled {
				assert.Equal(t, "(Redis disabled)", reason)
			} else {
				assert.Contains(t, tc.want, reason, "the revocation line gives the cache line's reason %s", reason)
			}
		})
	}

	t.Run("a connection whose cache could not be built", func(t *testing.T) {
		port, _ := countingListener(t)
		t.Setenv("REDIS_HOST", "127.0.0.1")
		cfg := loadConfigForRedisTest(t, port)
		// The line reads only whether there is a client, so one that has not
		// dialed stands in for a connection.
		client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:" + strconv.Itoa(port)})
		t.Cleanup(func() { _ = client.Close() })
		assert.Equal(t, "ℹ️  Token revocation disabled (cache initialization failed)"+notRotated,
			tokenRevocationStartupLine(cfg, client, nil, false))
	})

	t.Run("a cache service", func(t *testing.T) {
		port, _ := countingListener(t)
		t.Setenv("REDIS_HOST", "127.0.0.1")
		cfg := loadConfigForRedisTest(t, port)
		client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:" + strconv.Itoa(port)})
		t.Cleanup(func() { _ = client.Close() })
		assert.Equal(t, "ℹ️  Token revocation enabled (fail-open=true)", tokenRevocationStartupLine(cfg, client, &cache.RedisCache{}, true))
		assert.Equal(t, "ℹ️  Token revocation enabled (fail-open=false)", tokenRevocationStartupLine(cfg, client, &cache.RedisCache{}, false))
	})
}

// The token rotation guide quotes the revocation line for each case as the
// server writes it, after the log prefix.
func TestTokenRotationGuideQuotesTheRevocationStartupLines(t *testing.T) {
	requirePublicRepositoryFiles(t)

	guide, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "security", "token-rotation.md"))
	require.NoError(t, err)
	block := regexp.MustCompile("(?s)The backend's startup log says which applies, in one of these lines:\n```\n(.*?)\n```").FindSubmatch(guide)
	require.NotNil(t, block, "the guide's Configuration section quotes the startup lines in one fenced block")

	t.Setenv("REDIS_HOST", "127.0.0.1")
	cfg := loadConfigForRedisTest(t, 1)
	disabled := *cfg
	disabled.Redis.Disabled = true
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })

	var written []string
	for _, line := range []string{
		tokenRevocationStartupLine(cfg, client, &cache.RedisCache{}, false),
		tokenRevocationStartupLine(&disabled, nil, nil, false),
		tokenRevocationStartupLine(cfg, nil, nil, false),
	} {
		written = append(written, strings.TrimPrefix(line, "ℹ️  "))
	}
	assert.Equal(t, written, strings.Split(string(block[1]), "\n"))
}

func TestInitCache_FailedRedisPingIsLoggedAsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   bool
		value string
	}{
		{name: "REDIS_HOST names a host", set: true, value: "127.0.0.1"},
		{name: "REDIS_HOST is unset"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port, _ := countingListener(t)
			// Setenv first, so the variable is restored after an unset.
			t.Setenv("REDIS_HOST", tc.value)
			if !tc.set {
				require.NoError(t, os.Unsetenv("REDIS_HOST"))
			}
			cfg := loadConfigForRedisTest(t, port)
			client, err := initRedis(cfg)
			require.Error(t, err, "the listener closes every connection, so the startup ping fails")
			require.Nil(t, client)

			logged := captureLog(t)
			cacheService := initCache(cfg, client)

			assert.Nil(t, cacheService)
			assert.Equal(t, "ℹ️  Cache service skipped (Redis unavailable)\n", logged.String())
		})
	}
}
