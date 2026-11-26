package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
)

func TestED25519ServiceVerifyRefusesSmallOrderKeys(t *testing.T) {
	service := NewED25519Service()
	forged := make([]byte, ed25519.SignatureSize)
	forged[0] = 1
	for _, key := range ed25519key.SmallOrderEncodings() {
		var message []byte
		for i := 0; i < 10000 && message == nil; i++ {
			candidate := []byte(fmt.Sprintf("message-%d", i))
			if ed25519.Verify(ed25519.PublicKey(key), candidate, forged) {
				message = candidate
			}
		}
		require.NotNil(t, message, "%x", key)

		valid, err := service.Verify(base64.StdEncoding.EncodeToString(key), message, base64.StdEncoding.EncodeToString(forged))

		assert.False(t, valid, "%x", key)
		assert.ErrorIs(t, err, ed25519key.ErrNotAccepted, "%x", key)
	}
}
