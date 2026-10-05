package record

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// AlgEd25519 names the one signature algorithm records carry today.
const AlgEd25519 = "Ed25519"

// ErrSigning is wrapped by every error Sign returns.
var ErrSigning = errors.New("record: signing failed")

// PublicKey is the public half of a record signing key.
type PublicKey struct {
	// Alg is the signature algorithm. Only AlgEd25519 is supported.
	Alg string
	// Key is the raw public key.
	Key []byte
}

// KeyID is the key's identifier: the lowercase hex SHA-256 of the raw public
// key. A verifier computes it from the key it holds.
func (k PublicKey) KeyID() string {
	sum := sha256.Sum256(k.Key)
	return hex.EncodeToString(sum[:])
}

func (k PublicKey) validate() error {
	if k.Alg != AlgEd25519 {
		return fmt.Errorf("algorithm %q is not supported", k.Alg)
	}
	if len(k.Key) != ed25519.PublicKeySize {
		return fmt.Errorf("an %s public key is %d bytes, not %d", AlgEd25519, ed25519.PublicKeySize, len(k.Key))
	}
	return nil
}

// verify reports whether sig is this key's signature over message.
func (k PublicKey) verify(message, sig []byte) bool {
	if k.validate() != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(k.Key), message, sig)
}

// SigningKeyProvider is a KeyProvider that also returns the public half of the
// key it signs with. The key still signs through SignPayload alone, so the
// provider never signs a type or bytes a caller chose.
type SigningKeyProvider interface {
	KeyProvider
	// PublicKey returns the public half of the key SignPayload uses.
	PublicKey(ctx context.Context) (PublicKey, error)
}

// Record is a signed record as it is stored and exported: the envelope, the
// record hash, and each erasable part with its salt while it exists. Erasure
// removes a part and its salt and leaves the rest unchanged.
type Record struct {
	Envelope Envelope `json:"envelope"`
	// RecordHash is the lowercase hex SHA-256 of the envelope's payload bytes.
	RecordHash   string `json:"recordHash"`
	TenantPart   []byte `json:"tenantPart,omitempty"`
	TenantSalt   []byte `json:"tenantSalt,omitempty"`
	PersonalPart []byte `json:"personalPart,omitempty"`
	PersonalSalt []byte `json:"personalSalt,omitempty"`
}

// Sign seals a body in its envelope as a record of ClassRecordV1, signed by
// the provider's key. It refuses a genesis that names a first key other than
// the provider's, an envelope whose key identifier is not the provider's
// public key's, and a signature that does not verify under that key.
func Sign(ctx context.Context, b Body, kp SigningKeyProvider) (Record, error) {
	if len(b.canonical) == 0 {
		return Record{}, fmt.Errorf("%w: the body was not built by NewGenesis or NewRecord", ErrSigning)
	}
	if kp == nil {
		return Record{}, fmt.Errorf("%w: no key provider", ErrSigning)
	}
	key, err := kp.PublicKey(ctx)
	if err != nil {
		return Record{}, fmt.Errorf("%w: public key: %v", ErrSigning, err)
	}
	if err := key.validate(); err != nil {
		return Record{}, fmt.Errorf("%w: public key: %v", ErrSigning, err)
	}
	if b.recordType == TypeChainGenesis && b.firstKeyID != key.KeyID() {
		return Record{}, fmt.Errorf("%w: the genesis names a first key other than the provider's", ErrSigning)
	}
	payload := Payload{canonical: b.canonical}
	message, err := SigningInput(ClassRecordV1, payload)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrSigning, err)
	}
	env, err := Seal(ctx, kp, ClassRecordV1, payload)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrSigning, err)
	}
	if env.Signatures[0].KeyID != key.KeyID() {
		return Record{}, fmt.Errorf("%w: the provider names a key other than its public key", ErrSigning)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signatures[0].Sig)
	if err != nil || !key.verify(message, sig) {
		return Record{}, fmt.Errorf("%w: the provider's signature does not verify under its public key", ErrSigning)
	}
	return Record{
		Envelope:     env,
		RecordHash:   b.head.Hash,
		TenantPart:   append([]byte(nil), b.tenantPart...),
		TenantSalt:   append([]byte(nil), b.tenantSalt...),
		PersonalPart: append([]byte(nil), b.personalPart...),
		PersonalSalt: append([]byte(nil), b.personalSalt...),
	}, nil
}
