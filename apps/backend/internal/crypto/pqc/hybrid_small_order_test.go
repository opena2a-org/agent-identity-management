package pqc

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
)

// A hybrid signature whose ML-DSA half is genuine and whose Ed25519 half is the
// forged signature that plain ed25519.Verify accepts under a small-order key is
// refused: the Ed25519 key is not an acceptable key.
func TestHybridVerifyRefusesASmallOrderEd25519Key(t *testing.T) {
	keys, err := GenerateHybridKeyPair(AlgorithmMLDSA65)
	if err != nil {
		t.Fatal(err)
	}
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
		if message == nil {
			t.Fatalf("no message forges under %x", key)
		}
		sig, err := HybridSign(keys, message)
		if err != nil {
			t.Fatal(err)
		}
		sig.Ed25519 = forged
		pub := &HybridPublicKey{Ed25519: key, MLDSA: keys.MLDSA.PublicKey, MLDSAVariant: AlgorithmMLDSA65}

		if err := HybridVerify(pub, message, sig); !errors.Is(err, ErrEd25519SignatureInvalid) {
			t.Errorf("%x: err = %v, want ErrEd25519SignatureInvalid", key, err)
		}
	}
}
