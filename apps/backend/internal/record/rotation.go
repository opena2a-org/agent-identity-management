package record

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// The reasons a key rotation gives: the values of opena2a.rotation_reason.
const (
	// RotationScheduled: the key reached the end of its scheduled period.
	RotationScheduled = "scheduled"
	// RotationCustodyChange: who can reach the key changed.
	RotationCustodyChange = "custody_change"
	// RotationExposure: the key is believed to be exposed. The record states
	// when the exposure is believed to have begun.
	RotationExposure = "exposure"
)

func isRotationReason(s string) bool {
	return s == RotationScheduled || s == RotationCustodyChange || s == RotationExposure
}

// KeyRotation is the input of a key rotation record: the last record of its
// chain that the key in force signs.
type KeyRotation struct {
	// NextKey signs every later record of the chain.
	NextKey PublicKey
	// Reason is RotationScheduled, RotationCustodyChange or RotationExposure.
	Reason string
	// ExposureBegan is when the exposure of the key being retired is believed
	// to have begun. It is set when Reason is RotationExposure and only then.
	ExposureBegan time.Time
	// Draft carries the record's own members. Its Type is TypeKeyRotation or
	// empty.
	Draft Draft
}

// NewKeyRotation builds the key rotation record that follows prev: sequence
// prev.Seq plus one, committing to prev.Hash. Its retained part names the next
// key (key id, algorithm and public key), the reason and, on exposure, when
// the exposure is believed to have begun, so erasing the record's tenant and
// personal parts leaves its chain verifying. The key in force signs it and is
// retired by it: Sign refuses a rotation that names the signing key.
func NewKeyRotation(prev Head, r KeyRotation) (Body, error) {
	if err := checkHead(prev); err != nil {
		return Body{}, err
	}
	d := r.Draft
	if d.Type == "" {
		d.Type = TypeKeyRotation
	}
	if d.Type != TypeKeyRotation {
		return Body{}, fmt.Errorf("%w: a key rotation has type %s, not %q", ErrInvalidRecord, TypeKeyRotation, d.Type)
	}
	if err := r.NextKey.validate(); err != nil {
		return Body{}, fmt.Errorf("%w: next key: %v", ErrInvalidRecord, err)
	}
	if !isRotationReason(r.Reason) {
		return Body{}, fmt.Errorf("%w: the rotation reason %q is not %s, %s or %s",
			ErrInvalidRecord, r.Reason, RotationScheduled, RotationCustodyChange, RotationExposure)
	}
	own := map[string]any{
		"next_key":        keyMember(r.NextKey),
		"rotation_reason": r.Reason,
	}
	switch {
	case r.Reason == RotationExposure:
		began, err := FormatTimestamp(r.ExposureBegan)
		if err != nil {
			return Body{}, fmt.Errorf("%w: a rotation on exposure states when the exposure began: %v", ErrInvalidRecord, err)
		}
		own["exposure_began_at"] = began
	case !r.ExposureBegan.IsZero():
		return Body{}, fmt.Errorf("%w: only a rotation on exposure states when an exposure began", ErrInvalidRecord)
	}
	b, err := seal(prev.ChainID, prev.Seq+1, prev.Hash, d, owned{retained: own})
	if err != nil {
		return Body{}, err
	}
	b.nextKeyID = r.NextKey.KeyID()
	return b, nil
}

// keyRing is what a verifier holds of a chain's keys at one position: the key
// in force, and each key an earlier rotation retired.
type keyRing struct {
	current PublicKey
	retired []retiredKey
}

// retiredKey is a key and the sequence number of the rotation that retired it.
type retiredKey struct {
	key PublicKey
	seq int64
}

// retiredSigner returns the retired key whose valid signature the envelope
// carries, if any.
func (r keyRing) retiredSigner(env Envelope) (retiredKey, bool) {
	for _, old := range r.retired {
		if checkSignatures(env, old.key) == "" {
			return old, true
		}
	}
	return retiredKey{}, false
}

// VerifySignature makes a keyRing the SignatureVerifier of the keys the chain
// named: the key in force and each retired key, found by identifier.
func (r keyRing) VerifySignature(keyID string, message, signature []byte) error {
	if r.current.KeyID() == keyID {
		return r.current.VerifySignature(keyID, message, signature)
	}
	for _, old := range r.retired {
		if old.key.KeyID() == keyID {
			return old.key.VerifySignature(keyID, message, signature)
		}
	}
	return ErrUnknownKey
}

// used reports whether the chain has signed with the key keyID names.
func (r keyRing) used(keyID string) bool {
	if r.current.KeyID() == keyID {
		return true
	}
	for _, old := range r.retired {
		if old.key.KeyID() == keyID {
			return true
		}
	}
	return false
}

// rotate returns the ring after h, a key rotation that verified under the key
// in force, or why the rotation cannot be followed.
func (r keyRing) rotate(h header) (keyRing, string) {
	if h.nextKey == nil {
		return r, "the key rotation names no next key"
	}
	next, err := h.nextKey.key()
	if err != nil {
		return r, "the next key: " + err.Error()
	}
	if r.used(next.KeyID()) {
		return r, "the next key is a key the chain has already signed with"
	}
	if h.rotationReason == nil || !isRotationReason(*h.rotationReason) {
		return r, fmt.Sprintf("opena2a.rotation_reason is not %s, %s or %s", RotationScheduled, RotationCustodyChange, RotationExposure)
	}
	exposure := *h.rotationReason == RotationExposure
	switch {
	case exposure && h.exposureBeganAt == nil:
		return r, "a rotation on exposure does not state when the exposure began"
	case !exposure && h.exposureBeganAt != nil:
		return r, "a rotation that is not on exposure states when an exposure began"
	case exposure && !isCanonicalTimestamp(*h.exposureBeganAt):
		return r, "opena2a.exposure_began_at is not a timestamp in canonical form"
	}
	retired := append(append([]retiredKey(nil), r.retired...), retiredKey{key: r.current, seq: h.seq})
	return keyRing{current: next, retired: retired}, ""
}

// keyAfter returns the key the record after these bytes is signed with if the
// bytes are what they claim: the next key they name when they are a key
// rotation, and key otherwise. It only tells a changed record from a bad
// signature; no record is accepted under the key it returns.
func keyAfter(payload []byte, key PublicKey) PublicKey {
	h, err := parseHeader(payload)
	if err != nil || h.recordType != TypeKeyRotation || h.nextKey == nil {
		return key
	}
	next, err := h.nextKey.key()
	if err != nil {
		return key
	}
	return next
}

// key returns the key a key member names, or why it names none.
func (k namedKey) key() (PublicKey, error) {
	raw, err := hex.DecodeString(k.publicKey)
	if err != nil || hex.EncodeToString(raw) != k.publicKey {
		return PublicKey{}, errors.New("public_key is not lowercase hex")
	}
	key := PublicKey{Alg: k.alg, Key: raw}
	if err := key.validate(); err != nil {
		return PublicKey{}, err
	}
	if key.KeyID() != k.keyID {
		return PublicKey{}, errors.New("keyid is not the SHA-256 of public_key")
	}
	return key, nil
}

// isCanonicalTimestamp reports whether s is a timestamp as FormatTimestamp
// renders it.
func isCanonicalTimestamp(s string) bool {
	t, err := time.Parse(timestampLayout, s)
	if err != nil {
		return false
	}
	formatted, err := FormatTimestamp(t)
	return err == nil && formatted == s
}
