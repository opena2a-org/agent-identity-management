package main

// Startup Redis connection by REDIS_HOST: set to the empty string, the server
// makes no Redis connection at all. Unset, the localhost default is still tried.

import (
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
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
