// Package hybrid provides post-quantum cryptographic hybrid signing capabilities.
//
// This package implements a crypto-agile signing framework that supports:
// - Classical algorithms (Ed25519)
// - Post-quantum algorithms (ML-DSA-65/Dilithium3)
// - Hybrid mode combining both for defense-in-depth
//
// The hybrid approach provides security against both classical and quantum
// attackers during the transition period to post-quantum cryptography.
package hybrid

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

// Algorithm represents a cryptographic signing algorithm.
type Algorithm string

const (
	// AlgorithmEd25519 is the classical Ed25519 signature algorithm.
	AlgorithmEd25519 Algorithm = "ed25519"

	// AlgorithmMLDSA65 is the post-quantum ML-DSA-65 (Dilithium3) algorithm.
	// NIST standardized in FIPS 204.
	AlgorithmMLDSA65 Algorithm = "mldsa65"

	// AlgorithmHybrid combines Ed25519 and ML-DSA-65 for defense-in-depth.
	AlgorithmHybrid Algorithm = "hybrid-ed25519-mldsa65"
)

// Signer is the interface for cryptographic signing operations.
// Implementations must be thread-safe.
type Signer interface {
	// Algorithm returns the algorithm identifier.
	Algorithm() Algorithm

	// PublicKey returns the base64-encoded public key.
	PublicKey() string

	// PublicKeyBytes returns the raw public key bytes.
	PublicKeyBytes() []byte

	// Sign signs a message and returns the base64-encoded signature.
	Sign(message []byte) (string, error)

	// SignBytes signs a message and returns raw signature bytes.
	SignBytes(message []byte) ([]byte, error)

	// Verify verifies a signature against a message.
	// signature should be base64-encoded.
	Verify(message []byte, signature string) (bool, error)

	// VerifyBytes verifies raw signature bytes against a message.
	VerifyBytes(message []byte, signature []byte) (bool, error)
}

// KeyPair holds the public and private keys for a signing algorithm.
type KeyPair struct {
	Algorithm  Algorithm `json:"algorithm"`
	PublicKey  string    `json:"public_key"`  // Base64 encoded
	PrivateKey string    `json:"private_key"` // Base64 encoded
}

// HybridKeyPair holds keys for both classical and PQC algorithms.
type HybridKeyPair struct {
	Algorithm        Algorithm `json:"algorithm"`
	ClassicalPublic  string    `json:"classical_public_key"`
	ClassicalPrivate string    `json:"classical_private_key"`
	PQCPublic        string    `json:"pqc_public_key"`
	PQCPrivate       string    `json:"pqc_private_key"`
}

// HybridSignature contains signatures from both algorithms.
type HybridSignature struct {
	Classical []byte // Ed25519 signature (64 bytes)
	PQC       []byte // ML-DSA-65 signature (~3309 bytes)
}

// EncodeHybridSignature encodes a hybrid signature to bytes.
// Format: [classical_len:4][classical_sig][pqc_sig]
func EncodeHybridSignature(sig *HybridSignature) []byte {
	result := make([]byte, 4+len(sig.Classical)+len(sig.PQC))

	// Write classical signature length (4 bytes, big endian)
	binary.BigEndian.PutUint32(result[0:4], uint32(len(sig.Classical)))

	// Write classical signature
	copy(result[4:4+len(sig.Classical)], sig.Classical)

	// Write PQC signature
	copy(result[4+len(sig.Classical):], sig.PQC)

	return result
}

// DecodeHybridSignature decodes bytes to a hybrid signature.
func DecodeHybridSignature(data []byte) (*HybridSignature, error) {
	if len(data) < 4 {
		return nil, errors.New("hybrid signature too short")
	}

	classicalLen := binary.BigEndian.Uint32(data[0:4])
	if len(data) < int(4+classicalLen) {
		return nil, errors.New("hybrid signature truncated")
	}

	return &HybridSignature{
		Classical: data[4 : 4+classicalLen],
		PQC:       data[4+classicalLen:],
	}, nil
}

// EncodeHybridSignatureBase64 encodes a hybrid signature to base64.
func EncodeHybridSignatureBase64(sig *HybridSignature) string {
	return base64.StdEncoding.EncodeToString(EncodeHybridSignature(sig))
}

// DecodeHybridSignatureBase64 decodes a base64 hybrid signature.
func DecodeHybridSignatureBase64(encoded string) (*HybridSignature, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return DecodeHybridSignature(data)
}

// Verifier can verify signatures without access to private keys.
type Verifier interface {
	// Algorithm returns the algorithm identifier.
	Algorithm() Algorithm

	// Verify verifies a base64-encoded signature.
	Verify(message []byte, signature string) (bool, error)

	// VerifyBytes verifies raw signature bytes.
	VerifyBytes(message []byte, signature []byte) (bool, error)
}

// NewVerifier creates a verifier from a public key.
func NewVerifier(algorithm Algorithm, publicKeyBase64 string) (Verifier, error) {
	switch algorithm {
	case AlgorithmEd25519:
		return NewEd25519VerifierFromPublicKey(publicKeyBase64)
	case AlgorithmMLDSA65:
		return NewMLDSAVerifierFromPublicKey(publicKeyBase64)
	case AlgorithmHybrid:
		return nil, errors.New("hybrid verifier requires both public keys, use NewHybridVerifier")
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}
}
