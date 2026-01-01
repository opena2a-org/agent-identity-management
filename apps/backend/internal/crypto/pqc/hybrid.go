package pqc

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// GenerateHybridKeyPair generates both Ed25519 and ML-DSA key pairs
// This provides defense-in-depth: if either algorithm is compromised,
// the other still provides security.
func GenerateHybridKeyPair(mldsaVariant Algorithm) (*HybridKeyPair, error) {
	// Validate ML-DSA variant
	if mldsaVariant != AlgorithmMLDSA44 && mldsaVariant != AlgorithmMLDSA65 && mldsaVariant != AlgorithmMLDSA87 {
		return nil, fmt.Errorf("%w: %s is not an ML-DSA variant", ErrUnsupportedAlgorithm, mldsaVariant)
	}

	// Generate Ed25519 key pair
	ed25519Pub, ed25519Priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate Ed25519 key: %v", ErrKeyGenerationFailed, err)
	}

	// Generate ML-DSA key pair
	mldsaKeyPair, err := GenerateMLDSAKeyPair(mldsaVariant)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate ML-DSA key: %v", ErrKeyGenerationFailed, err)
	}

	// Determine hybrid algorithm based on ML-DSA variant
	var hybridAlg Algorithm
	switch mldsaVariant {
	case AlgorithmMLDSA44:
		hybridAlg = AlgorithmHybridEd25519MLDSA44
	case AlgorithmMLDSA65:
		hybridAlg = AlgorithmHybridEd25519MLDSA65
	case AlgorithmMLDSA87:
		hybridAlg = AlgorithmHybridEd25519MLDSA87
	}

	now := time.Now().UTC()
	return &HybridKeyPair{
		Ed25519: &KeyPair{
			Algorithm:  AlgorithmEd25519,
			PublicKey:  ed25519Pub,
			PrivateKey: ed25519Priv,
			CreatedAt:  now,
		},
		MLDSA:     mldsaKeyPair,
		Algorithm: hybridAlg,
		CreatedAt: now,
	}, nil
}

// GenerateHybridKeyPair65 is a convenience function to generate Ed25519+ML-DSA-65 keys (recommended)
func GenerateHybridKeyPair65() (*HybridKeyPair, error) {
	return GenerateHybridKeyPair(AlgorithmMLDSA65)
}

// HybridSign creates both Ed25519 and ML-DSA signatures for a message
// Both signatures are required for verification in hybrid mode
func HybridSign(keys *HybridKeyPair, message []byte) (*HybridSignature, error) {
	if keys == nil {
		return nil, fmt.Errorf("%w: nil key pair", ErrSigningFailed)
	}
	if keys.Ed25519 == nil || keys.MLDSA == nil {
		return nil, fmt.Errorf("%w: incomplete hybrid key pair", ErrSigningFailed)
	}

	// Sign with Ed25519
	ed25519Sig := ed25519.Sign(keys.Ed25519.PrivateKey, message)

	// Sign with ML-DSA
	mldsaSig, err := SignMLDSA(keys.MLDSA.Algorithm, keys.MLDSA.PrivateKey, message)
	if err != nil {
		return nil, fmt.Errorf("%w: ML-DSA signing failed: %v", ErrSigningFailed, err)
	}

	return &HybridSignature{
		Ed25519:   ed25519Sig,
		MLDSA:     mldsaSig,
		Algorithm: keys.Algorithm,
		Timestamp: time.Now().Unix(),
	}, nil
}

// HybridVerify verifies both Ed25519 and ML-DSA signatures
// BOTH signatures must be valid for verification to succeed
// This is the core security property of hybrid mode
func HybridVerify(publicKey *HybridPublicKey, message []byte, sig *HybridSignature) error {
	if publicKey == nil || sig == nil {
		return ErrHybridSignatureInvalid
	}

	// Validate public keys are present
	if len(publicKey.Ed25519) == 0 {
		return ErrMissingEd25519PublicKey
	}
	if len(publicKey.MLDSA) == 0 {
		return ErrMissingMLDSAPublicKey
	}

	// Validate signatures are present
	if len(sig.Ed25519) == 0 {
		return ErrMissingEd25519Signature
	}
	if len(sig.MLDSA) == 0 {
		return ErrMissingMLDSASignature
	}

	// Verify Ed25519 signature first (faster)
	if !ed25519.Verify(publicKey.Ed25519, message, sig.Ed25519) {
		return ErrEd25519SignatureInvalid
	}

	// Verify ML-DSA signature
	mldsaVariant := GetMLDSAVariant(sig.Algorithm)
	if err := VerifyMLDSA(mldsaVariant, publicKey.MLDSA, message, sig.MLDSA); err != nil {
		return ErrMLDSASignatureInvalid
	}

	return nil
}

// HybridVerifyBool is a convenience function that returns a boolean
func HybridVerifyBool(publicKey *HybridPublicKey, message []byte, sig *HybridSignature) bool {
	return HybridVerify(publicKey, message, sig) == nil
}

// HybridVerifyWithSeparateKeys verifies using separate Ed25519 and ML-DSA public keys
func HybridVerifyWithSeparateKeys(ed25519PubKey, mldsaPubKey, message []byte, sig *HybridSignature) error {
	return HybridVerify(&HybridPublicKey{
		Ed25519:      ed25519PubKey,
		MLDSA:        mldsaPubKey,
		MLDSAVariant: GetMLDSAVariant(sig.Algorithm),
	}, message, sig)
}

// EncodedHybridKeyPair represents a hybrid key pair with base64-encoded keys
type EncodedHybridKeyPair struct {
	Algorithm            Algorithm `json:"algorithm"`
	Ed25519PublicKey     string    `json:"ed25519PublicKey"`
	Ed25519PrivateKey    string    `json:"ed25519PrivateKey,omitempty"`
	MLDSAPublicKey       string    `json:"mldsaPublicKey"`
	MLDSAPrivateKey      string    `json:"mldsaPrivateKey,omitempty"`
	MLDSAVariant         Algorithm `json:"mldsaVariant"`
	CreatedAt            time.Time `json:"createdAt"`
}

// EncodeHybridKeyPair encodes a hybrid key pair to base64
func EncodeHybridKeyPair(keys *HybridKeyPair) *EncodedHybridKeyPair {
	return &EncodedHybridKeyPair{
		Algorithm:         keys.Algorithm,
		Ed25519PublicKey:  base64.StdEncoding.EncodeToString(keys.Ed25519.PublicKey),
		Ed25519PrivateKey: base64.StdEncoding.EncodeToString(keys.Ed25519.PrivateKey),
		MLDSAPublicKey:    base64.StdEncoding.EncodeToString(keys.MLDSA.PublicKey),
		MLDSAPrivateKey:   base64.StdEncoding.EncodeToString(keys.MLDSA.PrivateKey),
		MLDSAVariant:      keys.MLDSA.Algorithm,
		CreatedAt:         keys.CreatedAt,
	}
}

// EncodeHybridPublicKeys encodes the public keys only (safe for transmission)
func EncodeHybridPublicKeys(keys *HybridKeyPair) *EncodedHybridPublicKey {
	return &EncodedHybridPublicKey{
		Algorithm:        keys.Algorithm,
		Ed25519PublicKey: base64.StdEncoding.EncodeToString(keys.Ed25519.PublicKey),
		MLDSAPublicKey:   base64.StdEncoding.EncodeToString(keys.MLDSA.PublicKey),
		MLDSAVariant:     keys.MLDSA.Algorithm,
	}
}

// EncodedHybridPublicKey contains only the public keys (safe for transmission)
type EncodedHybridPublicKey struct {
	Algorithm        Algorithm `json:"algorithm"`
	Ed25519PublicKey string    `json:"ed25519PublicKey"`
	MLDSAPublicKey   string    `json:"mldsaPublicKey"`
	MLDSAVariant     Algorithm `json:"mldsaVariant"`
}

// DecodeHybridPublicKey decodes base64-encoded hybrid public keys
func DecodeHybridPublicKey(encoded *EncodedHybridPublicKey) (*HybridPublicKey, error) {
	ed25519Pub, err := base64.StdEncoding.DecodeString(encoded.Ed25519PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Ed25519 public key: %w", err)
	}

	if len(ed25519Pub) != Ed25519PublicKeySize {
		return nil, fmt.Errorf("%w: Ed25519 key expected %d bytes, got %d",
			ErrInvalidPublicKeySize, Ed25519PublicKeySize, len(ed25519Pub))
	}

	mldsaPub, err := base64.StdEncoding.DecodeString(encoded.MLDSAPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ML-DSA public key: %w", err)
	}

	expectedMLDSASize, err := GetExpectedPublicKeySize(encoded.MLDSAVariant)
	if err != nil {
		return nil, err
	}

	if len(mldsaPub) != expectedMLDSASize {
		return nil, fmt.Errorf("%w: ML-DSA key expected %d bytes, got %d",
			ErrInvalidPublicKeySize, expectedMLDSASize, len(mldsaPub))
	}

	return &HybridPublicKey{
		Ed25519:      ed25519Pub,
		MLDSA:        mldsaPub,
		MLDSAVariant: encoded.MLDSAVariant,
	}, nil
}

// EncodedHybridSignature represents a hybrid signature with base64-encoded data
type EncodedHybridSignature struct {
	Algorithm        Algorithm `json:"alg"`
	Ed25519Signature string    `json:"ed25519Sig"`
	MLDSASignature   string    `json:"mldsaSig"`
	Timestamp        int64     `json:"ts"`
}

// EncodeHybridSignature encodes a hybrid signature to base64
func EncodeHybridSignature(sig *HybridSignature) *EncodedHybridSignature {
	return &EncodedHybridSignature{
		Algorithm:        sig.Algorithm,
		Ed25519Signature: base64.StdEncoding.EncodeToString(sig.Ed25519),
		MLDSASignature:   base64.StdEncoding.EncodeToString(sig.MLDSA),
		Timestamp:        sig.Timestamp,
	}
}

// DecodeHybridSignature decodes a base64-encoded hybrid signature
func DecodeHybridSignature(encoded *EncodedHybridSignature) (*HybridSignature, error) {
	ed25519Sig, err := base64.StdEncoding.DecodeString(encoded.Ed25519Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Ed25519 signature: %w", err)
	}

	if len(ed25519Sig) != Ed25519SignatureSize {
		return nil, fmt.Errorf("%w: Ed25519 signature expected %d bytes, got %d",
			ErrInvalidSignatureSize, Ed25519SignatureSize, len(ed25519Sig))
	}

	mldsaSig, err := base64.StdEncoding.DecodeString(encoded.MLDSASignature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ML-DSA signature: %w", err)
	}

	mldsaVariant := GetMLDSAVariant(encoded.Algorithm)
	expectedSigSize, err := GetExpectedSignatureSize(mldsaVariant)
	if err != nil {
		return nil, err
	}

	if len(mldsaSig) != expectedSigSize {
		return nil, fmt.Errorf("%w: ML-DSA signature expected %d bytes, got %d",
			ErrInvalidSignatureSize, expectedSigSize, len(mldsaSig))
	}

	return &HybridSignature{
		Ed25519:   ed25519Sig,
		MLDSA:     mldsaSig,
		Algorithm: encoded.Algorithm,
		Timestamp: encoded.Timestamp,
	}, nil
}

// MarshalHybridSignature serializes a hybrid signature to JSON
func MarshalHybridSignature(sig *HybridSignature) ([]byte, error) {
	encoded := EncodeHybridSignature(sig)
	return json.Marshal(encoded)
}

// UnmarshalHybridSignature deserializes a hybrid signature from JSON
func UnmarshalHybridSignature(data []byte) (*HybridSignature, error) {
	var encoded EncodedHybridSignature
	if err := json.Unmarshal(data, &encoded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hybrid signature: %w", err)
	}
	return DecodeHybridSignature(&encoded)
}

// ConstantTimeCompare performs constant-time comparison of two byte slices
// This prevents timing attacks when comparing signatures
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// GetHybridPublicKey extracts the public key component from a hybrid key pair
func GetHybridPublicKey(keys *HybridKeyPair) *HybridPublicKey {
	return &HybridPublicKey{
		Ed25519:      keys.Ed25519.PublicKey,
		MLDSA:        keys.MLDSA.PublicKey,
		MLDSAVariant: keys.MLDSA.Algorithm,
	}
}

// ValidateHybridKeyPair validates that a hybrid key pair is complete and consistent
func ValidateHybridKeyPair(keys *HybridKeyPair) error {
	if keys == nil {
		return fmt.Errorf("nil hybrid key pair")
	}
	if keys.Ed25519 == nil {
		return fmt.Errorf("missing Ed25519 key pair")
	}
	if keys.MLDSA == nil {
		return fmt.Errorf("missing ML-DSA key pair")
	}

	// Validate Ed25519 key sizes
	if len(keys.Ed25519.PublicKey) != Ed25519PublicKeySize {
		return fmt.Errorf("%w: Ed25519 public key", ErrInvalidPublicKeySize)
	}
	if len(keys.Ed25519.PrivateKey) != Ed25519PrivateKeySize {
		return fmt.Errorf("%w: Ed25519 private key", ErrInvalidPrivateKeySize)
	}

	// Validate ML-DSA key sizes
	info, err := GetMLDSAKeyInfo(keys.MLDSA.Algorithm)
	if err != nil {
		return err
	}
	if len(keys.MLDSA.PublicKey) != info.PublicKeySize {
		return fmt.Errorf("%w: ML-DSA public key expected %d, got %d",
			ErrInvalidPublicKeySize, info.PublicKeySize, len(keys.MLDSA.PublicKey))
	}
	if len(keys.MLDSA.PrivateKey) != info.PrivateKeySize {
		return fmt.Errorf("%w: ML-DSA private key expected %d, got %d",
			ErrInvalidPrivateKeySize, info.PrivateKeySize, len(keys.MLDSA.PrivateKey))
	}

	// Validate algorithm consistency
	if !IsHybridAlgorithm(keys.Algorithm) {
		return fmt.Errorf("algorithm %s is not a hybrid algorithm", keys.Algorithm)
	}

	expectedMLDSA := GetMLDSAVariant(keys.Algorithm)
	if keys.MLDSA.Algorithm != expectedMLDSA {
		return fmt.Errorf("ML-DSA variant mismatch: expected %s, got %s",
			expectedMLDSA, keys.MLDSA.Algorithm)
	}

	return nil
}
