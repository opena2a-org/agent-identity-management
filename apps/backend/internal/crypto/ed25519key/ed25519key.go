// Package ed25519key answers whether 32 bytes are an acceptable Ed25519 public
// key. It is the one place that decides; every site that accepts or stores an
// Ed25519 public key, or verifies a signature under a stored one, asks it.
package ed25519key

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrNotAccepted is returned for a key that decodes to 32 bytes but that
// Acceptable refuses.
var ErrNotAccepted = errors.New("not an acceptable Ed25519 public key")

// smallOrderEncodings are the fourteen 32-byte encodings of the eight points of
// order 1, 2, 4 and 8 on edwards25519 (the small-order subgroup), counting the
// encodings the point decoder of crypto/ed25519 accepts although they are not
// canonical. A signature that verifies under one of them shows no private key.
//
// The list is worked out from the curve, not copied from a library: the y
// coordinates of the eight points are 0 (x = ±sqrt(-1)), 1 (x = 0), -1 (x = 0)
// and ±y8 (four points of order 8, y8 =
// 0x7a03ac9277fdc74ec6cc392cfa53202a0f67100d760b3cba4fd84d3d706a17c7). An
// encoding is y little-endian with the sign of x in the top bit, so each y has
// two encodings: ten for the canonical y (0, 1, p-1, y8, p-y8), and four more
// for y = p and y = p+1, which are below 2^255 and decode as 0 and 1. The test
// of this package checks each entry by the property itself: a fixed forged
// signature verifies under it for a suitable message.
var smallOrderEncodings = [...][ed25519.PublicKeySize]byte{
	// y = 0, x = ±sqrt(-1)
	mustHex32("0000000000000000000000000000000000000000000000000000000000000000"),
	mustHex32("0000000000000000000000000000000000000000000000000000000000000080"),
	// y = 1 (the identity), x = 0
	mustHex32("0100000000000000000000000000000000000000000000000000000000000000"),
	mustHex32("0100000000000000000000000000000000000000000000000000000000000080"),
	// y = p-1, x = 0 (order 2)
	mustHex32("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"),
	mustHex32("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	// y = y8 and y = p-y8 (order 8)
	mustHex32("c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a"),
	mustHex32("c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa"),
	mustHex32("26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05"),
	mustHex32("26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85"),
	// non-canonical: y = p (decodes as 0) and y = p+1 (decodes as 1)
	mustHex32("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"),
	mustHex32("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	mustHex32("eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"),
	mustHex32("eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
}

// SmallOrderEncodings returns a copy of the fourteen small-order encodings.
func SmallOrderEncodings() [][]byte {
	out := make([][]byte, len(smallOrderEncodings))
	for i := range smallOrderEncodings {
		out[i] = append([]byte(nil), smallOrderEncodings[i][:]...)
	}
	return out
}

// Acceptable reports whether key is an acceptable Ed25519 public key: 32 bytes
// whose y coordinate (little-endian, top bit masked) is below p = 2^255 - 19,
// and that is none of the fourteen small-order encodings.
func Acceptable(key []byte) bool {
	if len(key) != ed25519.PublicKeySize {
		return false
	}
	if !canonical(key) {
		return false
	}
	for i := range smallOrderEncodings {
		if string(key) == string(smallOrderEncodings[i][:]) {
			return false
		}
	}
	return true
}

// canonical reports whether the y coordinate of the 32-byte encoding, with the
// top bit masked, is below p = 2^255 - 19: that is, not in [p, 2^255).
func canonical(key []byte) bool {
	// p = 0x7fff...ffed. y >= p exactly when the masked top byte is 0x7f, bytes
	// 1 to 30 are all 0xff and the low byte is at least 0xed.
	if key[31]&0x7f != 0x7f {
		return true
	}
	for i := 1; i < 31; i++ {
		if key[i] != 0xff {
			return true
		}
	}
	return key[0] < 0xed
}

// Decode reads a standard-base64 Ed25519 public key and refuses one that is not
// 32 bytes or that Acceptable refuses. A strict decode is used when strict is
// set (no non-canonical padding or trailing bits).
func Decode(encoded string, strict bool) (ed25519.PublicKey, error) {
	enc := base64.StdEncoding
	if strict {
		enc = enc.Strict()
	}
	raw, err := enc.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("public key is not base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key is %d bytes, not %d", len(raw), ed25519.PublicKeySize)
	}
	if !Acceptable(raw) {
		return nil, ErrNotAccepted
	}
	return ed25519.PublicKey(raw), nil
}

func mustHex32(s string) [ed25519.PublicKeySize]byte {
	var out [ed25519.PublicKeySize]byte
	if len(s) != 2*len(out) {
		panic("ed25519key: bad small-order encoding")
	}
	for i := range out {
		out[i] = unhex(s[2*i])<<4 | unhex(s[2*i+1])
	}
	return out
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	panic("ed25519key: bad hex digit")
}
