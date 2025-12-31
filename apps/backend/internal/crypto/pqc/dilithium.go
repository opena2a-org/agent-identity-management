package pqc

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
)

// Key and signature sizes from the mldsa packages
const (
	// ML-DSA-44 sizes
	mldsa44PubKeySize  = 1312
	mldsa44PrivKeySize = 2560
	mldsa44SigSize     = 2420

	// ML-DSA-65 sizes
	mldsa65PubKeySize  = 1952
	mldsa65PrivKeySize = 4032
	mldsa65SigSize     = 3309 // Actual size from circl

	// ML-DSA-87 sizes
	mldsa87PubKeySize  = 2592
	mldsa87PrivKeySize = 4896
	mldsa87SigSize     = 4627
)

// GenerateMLDSAKeyPair generates a new ML-DSA (FIPS 204) key pair
//
// Algorithm variants:
// - ML-DSA-44: NIST Security Level 2
// - ML-DSA-65: NIST Security Level 3 (recommended)
// - ML-DSA-87: NIST Security Level 5
func GenerateMLDSAKeyPair(alg Algorithm) (*KeyPair, error) {
	var publicKey, privateKey []byte
	var err error

	switch alg {
	case AlgorithmMLDSA44:
		publicKey, privateKey, err = generateMLDSA44(rand.Reader)
	case AlgorithmMLDSA65:
		publicKey, privateKey, err = generateMLDSA65(rand.Reader)
	case AlgorithmMLDSA87:
		publicKey, privateKey, err = generateMLDSA87(rand.Reader)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}

	return &KeyPair{
		Algorithm:  alg,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func generateMLDSA44(rng io.Reader) ([]byte, []byte, error) {
	pk, sk, err := mldsa44.GenerateKey(rng)
	if err != nil {
		return nil, nil, err
	}
	return pk.Bytes(), sk.Bytes(), nil
}

func generateMLDSA65(rng io.Reader) ([]byte, []byte, error) {
	pk, sk, err := mldsa65.GenerateKey(rng)
	if err != nil {
		return nil, nil, err
	}
	return pk.Bytes(), sk.Bytes(), nil
}

func generateMLDSA87(rng io.Reader) ([]byte, []byte, error) {
	pk, sk, err := mldsa87.GenerateKey(rng)
	if err != nil {
		return nil, nil, err
	}
	return pk.Bytes(), sk.Bytes(), nil
}

// GenerateMLDSA65KeyPair is a convenience function to generate ML-DSA-65 keys (recommended)
func GenerateMLDSA65KeyPair() (*KeyPair, error) {
	return GenerateMLDSAKeyPair(AlgorithmMLDSA65)
}

// SignMLDSA signs a message using ML-DSA (FIPS 204)
func SignMLDSA(alg Algorithm, privateKey, message []byte) ([]byte, error) {
	switch alg {
	case AlgorithmMLDSA44:
		return signMLDSA44(privateKey, message)
	case AlgorithmMLDSA65:
		return signMLDSA65(privateKey, message)
	case AlgorithmMLDSA87:
		return signMLDSA87(privateKey, message)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

func signMLDSA44(privateKey, message []byte) ([]byte, error) {
	var sk mldsa44.PrivateKey
	if err := sk.UnmarshalBinary(privateKey); err != nil {
		return nil, fmt.Errorf("%w: invalid private key: %v", ErrSigningFailed, err)
	}

	sig := make([]byte, mldsa44.SignatureSize)
	// Sign with empty context (nil) and non-randomized mode for deterministic signatures
	if err := mldsa44.SignTo(&sk, message, nil, false, sig); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningFailed, err)
	}
	return sig, nil
}

func signMLDSA65(privateKey, message []byte) ([]byte, error) {
	var sk mldsa65.PrivateKey
	if err := sk.UnmarshalBinary(privateKey); err != nil {
		return nil, fmt.Errorf("%w: invalid private key: %v", ErrSigningFailed, err)
	}

	sig := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(&sk, message, nil, false, sig); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningFailed, err)
	}
	return sig, nil
}

func signMLDSA87(privateKey, message []byte) ([]byte, error) {
	var sk mldsa87.PrivateKey
	if err := sk.UnmarshalBinary(privateKey); err != nil {
		return nil, fmt.Errorf("%w: invalid private key: %v", ErrSigningFailed, err)
	}

	sig := make([]byte, mldsa87.SignatureSize)
	if err := mldsa87.SignTo(&sk, message, nil, false, sig); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningFailed, err)
	}
	return sig, nil
}

// VerifyMLDSA verifies an ML-DSA signature
func VerifyMLDSA(alg Algorithm, publicKey, message, signature []byte) error {
	switch alg {
	case AlgorithmMLDSA44:
		return verifyMLDSA44(publicKey, message, signature)
	case AlgorithmMLDSA65:
		return verifyMLDSA65(publicKey, message, signature)
	case AlgorithmMLDSA87:
		return verifyMLDSA87(publicKey, message, signature)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

func verifyMLDSA44(publicKey, message, signature []byte) error {
	var pk mldsa44.PublicKey
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return fmt.Errorf("%w: invalid public key: %v", ErrInvalidPublicKeySize, err)
	}

	if !mldsa44.Verify(&pk, message, nil, signature) {
		return ErrMLDSASignatureInvalid
	}
	return nil
}

func verifyMLDSA65(publicKey, message, signature []byte) error {
	var pk mldsa65.PublicKey
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return fmt.Errorf("%w: invalid public key: %v", ErrInvalidPublicKeySize, err)
	}

	if !mldsa65.Verify(&pk, message, nil, signature) {
		return ErrMLDSASignatureInvalid
	}
	return nil
}

func verifyMLDSA87(publicKey, message, signature []byte) error {
	var pk mldsa87.PublicKey
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return fmt.Errorf("%w: invalid public key: %v", ErrInvalidPublicKeySize, err)
	}

	if !mldsa87.Verify(&pk, message, nil, signature) {
		return ErrMLDSASignatureInvalid
	}
	return nil
}

// VerifyMLDSABool is a convenience function that returns a boolean instead of error
func VerifyMLDSABool(alg Algorithm, publicKey, message, signature []byte) bool {
	return VerifyMLDSA(alg, publicKey, message, signature) == nil
}

// EncodeMLDSAKeyPair encodes an ML-DSA key pair to base64
func EncodeMLDSAKeyPair(kp *KeyPair) *EncodedKeyPair {
	return &EncodedKeyPair{
		Algorithm:        kp.Algorithm,
		PublicKeyBase64:  base64.StdEncoding.EncodeToString(kp.PublicKey),
		PrivateKeyBase64: base64.StdEncoding.EncodeToString(kp.PrivateKey),
		CreatedAt:        kp.CreatedAt,
	}
}

// EncodedKeyPair represents a key pair with base64-encoded keys
type EncodedKeyPair struct {
	Algorithm        Algorithm `json:"algorithm"`
	PublicKeyBase64  string    `json:"publicKey"`
	PrivateKeyBase64 string    `json:"privateKey,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// DecodeMLDSAPublicKey decodes a base64-encoded ML-DSA public key
func DecodeMLDSAPublicKey(alg Algorithm, publicKeyBase64 string) ([]byte, error) {
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	expectedSize, err := GetExpectedPublicKeySize(alg)
	if err != nil {
		return nil, err
	}

	if len(publicKey) != expectedSize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidPublicKeySize, expectedSize, len(publicKey))
	}

	return publicKey, nil
}

// DecodeMLDSAPrivateKey decodes a base64-encoded ML-DSA private key
func DecodeMLDSAPrivateKey(alg Algorithm, privateKeyBase64 string) ([]byte, error) {
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	var expectedSize int
	switch alg {
	case AlgorithmMLDSA44:
		expectedSize = mldsa44PrivKeySize
	case AlgorithmMLDSA65:
		expectedSize = mldsa65PrivKeySize
	case AlgorithmMLDSA87:
		expectedSize = mldsa87PrivKeySize
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}

	if len(privateKey) != expectedSize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidPrivateKeySize, expectedSize, len(privateKey))
	}

	return privateKey, nil
}

// GetMLDSAKeyInfo returns information about ML-DSA key sizes for a given algorithm
type MLDSAKeyInfo struct {
	Algorithm      Algorithm
	PublicKeySize  int
	PrivateKeySize int
	SignatureSize  int
	SecurityLevel  string
}

// GetMLDSAKeyInfo returns size information for an ML-DSA algorithm
func GetMLDSAKeyInfo(alg Algorithm) (*MLDSAKeyInfo, error) {
	switch alg {
	case AlgorithmMLDSA44:
		return &MLDSAKeyInfo{
			Algorithm:      alg,
			PublicKeySize:  mldsa44PubKeySize,
			PrivateKeySize: mldsa44PrivKeySize,
			SignatureSize:  mldsa44SigSize,
			SecurityLevel:  "NIST Level 2",
		}, nil
	case AlgorithmMLDSA65:
		return &MLDSAKeyInfo{
			Algorithm:      alg,
			PublicKeySize:  mldsa65PubKeySize,
			PrivateKeySize: mldsa65PrivKeySize,
			SignatureSize:  mldsa65SigSize,
			SecurityLevel:  "NIST Level 3",
		}, nil
	case AlgorithmMLDSA87:
		return &MLDSAKeyInfo{
			Algorithm:      alg,
			PublicKeySize:  mldsa87PubKeySize,
			PrivateKeySize: mldsa87PrivKeySize,
			SignatureSize:  mldsa87SigSize,
			SecurityLevel:  "NIST Level 5",
		}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

// GetActualSignatureSize returns the actual signature size from the circl library
func GetActualSignatureSize(alg Algorithm) (int, error) {
	switch alg {
	case AlgorithmMLDSA44:
		return mldsa44.SignatureSize, nil
	case AlgorithmMLDSA65:
		return mldsa65.SignatureSize, nil
	case AlgorithmMLDSA87:
		return mldsa87.SignatureSize, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}
