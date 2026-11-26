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

func TestDecodePublicKeyAndVerifySignatureRefuseSmallOrderKeys(t *testing.T) {
	forged := make([]byte, ed25519.SignatureSize)
	forged[0] = 1
	for _, key := range ed25519key.SmallOrderEncodings() {
		_, err := DecodePublicKey(base64.StdEncoding.EncodeToString(key))
		assert.ErrorIs(t, err, ed25519key.ErrNotAccepted, "%x", key)

		var message []byte
		for i := 0; i < 10000 && message == nil; i++ {
			candidate := []byte(fmt.Sprintf("message-%d", i))
			if ed25519.Verify(ed25519.PublicKey(key), candidate, forged) {
				message = candidate
			}
		}
		require.NotNil(t, message, "%x", key)
		assert.False(t, VerifySignature(ed25519.PublicKey(key), message, forged), "%x", key)
	}
}
