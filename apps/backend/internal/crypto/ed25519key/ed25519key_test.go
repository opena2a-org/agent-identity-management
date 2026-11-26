package ed25519key

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math/big"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key/ed25519keytest"
)

// forgedSignature is the signature (R = the identity point, S = 0) that
// verifies under a small-order key for any message whose hash is a multiple of
// the key's order.
func forgedSignature() []byte {
	sig := make([]byte, ed25519.SignatureSize)
	sig[0] = 1
	return sig
}

// forgingMessage finds a message for which plain ed25519.Verify accepts the
// forged signature under key.
func forgingMessage(key []byte) ([]byte, bool) {
	sig := forgedSignature()
	for i := 0; i < 4096; i++ {
		msg := []byte{byte(i), byte(i >> 8)}
		if ed25519.Verify(ed25519.PublicKey(key), msg, sig) {
			return msg, true
		}
	}
	return nil, false
}

func TestSmallOrderListHasFourteenDistinctEncodingsThatForge(t *testing.T) {
	list := SmallOrderEncodings()
	if len(list) != 14 {
		t.Fatalf("list has %d encodings, want 14", len(list))
	}
	seen := map[string]bool{}
	for _, enc := range list {
		if seen[string(enc)] {
			t.Fatalf("%x listed twice", enc)
		}
		seen[string(enc)] = true
		if _, ok := forgingMessage(enc); !ok {
			t.Errorf("no message found for which the forged signature verifies under %x: not a small-order encoding", enc)
		}
		if Acceptable(enc) {
			t.Errorf("%x is accepted", enc)
		}
	}
}

func TestGenuineKeysAreAccepted(t *testing.T) {
	for i := 0; i < 50; i++ {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if !Acceptable(pub) {
			t.Fatalf("genuine key %x refused", pub)
		}
		msg := []byte("hello")
		if !ed25519.Verify(pub, msg, ed25519.Sign(priv, msg)) {
			t.Fatal("genuine signature does not verify")
		}
	}
}

func TestCanonical(t *testing.T) {
	cases := []struct {
		name string
		key  []byte
		want bool
	}{
		{"wrong length", make([]byte, 31), false},
		{"empty", nil, false},
		{"y = p-1 is canonical (but small order)", hex32("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		{"y = p", hex32("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		{"y = p with the sign bit", hex32("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		{"y = 2^255-1", hex32("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		{"y just below p", hex32("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		{"y = 2", hex32("0200000000000000000000000000000000000000000000000000000000000000"), true},
	}
	for _, c := range cases {
		if len(c.key) != 32 {
			if Acceptable(c.key) {
				t.Errorf("%s: accepted", c.name)
			}
			continue
		}
		if got := canonical(c.key); got != c.want {
			t.Errorf("%s: canonical = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDecode(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := Decode(base64.StdEncoding.EncodeToString(pub), true); err != nil {
		t.Fatalf("genuine key refused: %v", err)
	}
	if _, err := Decode("not base64!", false); err == nil {
		t.Error("garbage accepted")
	}
	if _, err := Decode(base64.StdEncoding.EncodeToString(pub[:31]), false); err == nil {
		t.Error("31 bytes accepted")
	}
	for _, enc := range SmallOrderEncodings() {
		_, err := Decode(base64.StdEncoding.EncodeToString(enc), false)
		if !errors.Is(err, ErrNotAccepted) {
			t.Errorf("%x: err = %v, want ErrNotAccepted", enc, err)
		}
	}
}

func hex32(s string) []byte {
	a := mustHex32(s)
	return a[:]
}

func TestOffCurveEncodingIsRefused(t *testing.T) {
	key := ed25519keytest.OffCurve()
	if len(key) != 32 || !canonical(key) {
		t.Fatalf("test key %x is not a canonical 32-byte encoding", key)
	}
	for _, small := range SmallOrderEncodings() {
		if string(small) == string(key) {
			t.Fatalf("test key %x is a small-order encoding", key)
		}
	}
	if Acceptable(key) {
		t.Errorf("off-curve %x accepted", key)
	}
	withSign := append([]byte(nil), key...)
	withSign[31] |= 0x80
	if Acceptable(withSign) {
		t.Errorf("off-curve %x with the sign bit accepted", withSign)
	}
	if _, err := Decode(base64.StdEncoding.EncodeToString(key), true); !errors.Is(err, ErrNotAccepted) {
		t.Errorf("Decode err = %v, want ErrNotAccepted", err)
	}
}

// Acceptable agrees with an independent decoder (the RFC 8032 square-root
// method in ed25519keytest) for the small canonical y values.
func TestAcceptableAgreesWithCurveForManyY(t *testing.T) {
	onCurve, offCurve := 0, 0
	for n := 2; n < 400; n++ {
		key := make([]byte, 32)
		key[0], key[1] = byte(n), byte(n>>8)
		y := new(big.Int).SetBytes([]byte{byte(n >> 8), byte(n)})
		small := false
		for _, s := range SmallOrderEncodings() {
			if string(s) == string(key) {
				small = true
			}
		}
		if small {
			continue
		}
		want := ed25519keytest.OnCurve(y)
		if got := Acceptable(key); got != want {
			t.Fatalf("y=%d: Acceptable = %v, curve says %v", n, got, want)
		}
		if want {
			onCurve++
		} else {
			offCurve++
		}
	}
	if onCurve == 0 || offCurve == 0 {
		t.Fatalf("sample has %d on-curve and %d off-curve", onCurve, offCurve)
	}
}
