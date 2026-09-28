package cache

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The refresh route stores a client mark (or the legacy "1") as a denylist
// value through SetWithNX or Set, and reads it back through Get into a string
// to classify a reuse (unit 9933). Both encodings round-trip unchanged.
func TestRedisCache_RevocationValueRoundTrips(t *testing.T) {
	for _, value := range []string{"v1:0123456789abcdef0123456789abcdef", "1"} {
		db, mock := redismock.NewClientMock()
		c := &RedisCache{client: db}
		key := "revoked:jti:abc"

		mock.ExpectSetNX(key, []byte(`"`+value+`"`), time.Hour).SetVal(true)
		written, err := c.SetWithNX(context.Background(), key, value, time.Hour)
		require.NoError(t, err)
		assert.True(t, written)

		mock.ExpectGet(key).SetVal(`"` + value + `"`)
		var got string
		require.NoError(t, c.Get(context.Background(), key, &got))
		assert.Equal(t, value, got)
		assert.NoError(t, mock.ExpectationsWereMet())
	}
}
