package main

// GET /health/ready Redis status by configuration: notConfigured means no
// REDIS_HOST names a Redis. A configured Redis that failed, at startup or
// since, reads unavailable with degraded true.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthReadyRedisStatusByConfiguration(t *testing.T) {
	schema := aim07LoadSchema(t)

	cases := []struct {
		name         string
		configured   bool
		connected    bool
		pingErr      error
		wantStatus   string
		wantDegraded bool
	}{
		{name: "unset reads notConfigured", wantStatus: "notConfigured"},
		{name: "unset with the localhost default answering reads ok", connected: true, wantStatus: "ok"},
		{name: "set and connected reads ok", configured: true, connected: true, wantStatus: "ok"},
		{name: "set and failed at startup reads unavailable, degraded", configured: true, wantStatus: "unavailable", wantDegraded: true},
		{name: "set and failed since startup reads unavailable, degraded", configured: true, connected: true, pingErr: errors.New("connection reset"), wantStatus: "unavailable", wantDegraded: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			defer client.Close()
			if tc.pingErr != nil {
				mock.ExpectPing().SetErr(tc.pingErr)
			} else {
				mock.ExpectPing().SetVal("PONG")
			}
			if !tc.connected {
				client = nil // initRedis returns a nil client when the startup ping fails
			}

			status, _, body := aim07Get(t, aim07App(aim07CheckOK, redisReadyCheck(tc.configured, client)))
			require.Equal(t, http.StatusOK, status, "an optional Redis never makes the service not ready")
			aim07Validate(t, schema, body)

			m := aim07TopLevel(t, body)
			assert.Equal(t, true, m["ready"])
			assert.Equal(t, tc.wantDegraded, m["degraded"])
			redisStatus, redisRequired := aim07DepStatus(t, body, "redis")
			assert.Equal(t, tc.wantStatus, redisStatus)
			assert.False(t, redisRequired)
			if tc.wantStatus == "unavailable" {
				assert.Contains(t, string(body), `"reason":"dependency check failed"`)
				assert.NotContains(t, string(body), "startup connection failed", "error text must not reach the body")
			}
		})
	}
}
