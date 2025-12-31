// Package pqc provides post-quantum cryptographic operations for AIM.
//
// This package implements NIST-approved post-quantum algorithms:
// - ML-DSA (Dilithium) for digital signatures (FIPS 204)
// - Hybrid Ed25519+ML-DSA for defense-in-depth
//
// The implementation uses Cloudflare's circl library which provides
// optimized and audited implementations of these algorithms.
package pqc

import (
	"errors"
	"time"
)

// Algorithm represents a cryptographic algorithm identifier
type Algorithm string

// Supported algorithms
const (
	// Classical algorithms
	AlgorithmEd25519 Algorithm = "Ed25519"

	// Post-quantum algorithms (ML-DSA / Dilithium) - FIPS 204
	AlgorithmMLDSA44 Algorithm = "ML-DSA-44" // NIST Security Level 2
	AlgorithmMLDSA65 Algorithm = "ML-DSA-65" // NIST Security Level 3 (recommended)
	AlgorithmMLDSA87 Algorithm = "ML-DSA-87" // NIST Security Level 5

	// Hybrid algorithms (both signatures required)
	AlgorithmHybridEd25519MLDSA44 Algorithm = "Ed25519+ML-DSA-44"
	AlgorithmHybridEd25519MLDSA65 Algorithm = "Ed25519+ML-DSA-65" // Recommended
	AlgorithmHybridEd25519MLDSA87 Algorithm = "Ed25519+ML-DSA-87"
)

// Algorithm families
const (
	FamilyClassical   = "classical"
	FamilyPostQuantum = "pqc"
	FamilyHybrid      = "hybrid"
)

// Key and signature sizes for each ML-DSA variant
const (
	// ML-DSA-44 (Dilithium2) - Security Level 2
	MLDSA44PublicKeySize  = 1312
	MLDSA44PrivateKeySize = 2560
	MLDSA44SignatureSize  = 2420

	// ML-DSA-65 (Dilithium3) - Security Level 3
	MLDSA65PublicKeySize  = 1952
	MLDSA65PrivateKeySize = 4032
	MLDSA65SignatureSize  = 3309 // Actual size from circl library

	// ML-DSA-87 (Dilithium5) - Security Level 5
	MLDSA87PublicKeySize  = 2592
	MLDSA87PrivateKeySize = 4896
	MLDSA87SignatureSize  = 4627

	// Ed25519 sizes for reference
	Ed25519PublicKeySize  = 32
	Ed25519PrivateKeySize = 64
	Ed25519SignatureSize  = 64
)

// Common errors
var (
	ErrUnsupportedAlgorithm     = errors.New("unsupported algorithm")
	ErrInvalidPublicKeySize     = errors.New("invalid public key size")
	ErrInvalidPrivateKeySize    = errors.New("invalid private key size")
	ErrInvalidSignatureSize     = errors.New("invalid signature size")
	ErrSignatureVerifyFailed    = errors.New("signature verification failed")
	ErrEd25519SignatureInvalid  = errors.New("Ed25519 signature verification failed")
	ErrMLDSASignatureInvalid    = errors.New("ML-DSA signature verification failed")
	ErrHybridSignatureInvalid   = errors.New("hybrid signature verification failed: both signatures must be valid")
	ErrKeyGenerationFailed      = errors.New("key generation failed")
	ErrSigningFailed            = errors.New("signing failed")
	ErrMissingEd25519Signature  = errors.New("missing Ed25519 signature in hybrid mode")
	ErrMissingMLDSASignature    = errors.New("missing ML-DSA signature in hybrid mode")
	ErrMissingEd25519PublicKey  = errors.New("missing Ed25519 public key in hybrid mode")
	ErrMissingMLDSAPublicKey    = errors.New("missing ML-DSA public key in hybrid mode")
)

// KeyPair represents a generic cryptographic key pair
type KeyPair struct {
	Algorithm  Algorithm
	PublicKey  []byte
	PrivateKey []byte
	CreatedAt  time.Time
}

// HybridKeyPair contains both Ed25519 and ML-DSA key pairs
type HybridKeyPair struct {
	Ed25519   *KeyPair
	MLDSA     *KeyPair
	Algorithm Algorithm
	CreatedAt time.Time
}

// Signature represents a cryptographic signature
type Signature struct {
	Algorithm Algorithm
	Data      []byte
	Timestamp int64
}

// HybridSignature contains both Ed25519 and ML-DSA signatures
type HybridSignature struct {
	Ed25519   []byte    `json:"ed25519Sig"`
	MLDSA     []byte    `json:"mldsaSig"`
	Algorithm Algorithm `json:"alg"`
	Timestamp int64     `json:"ts"`
}

// HybridPublicKey contains both public keys for hybrid verification
type HybridPublicKey struct {
	Ed25519      []byte    `json:"ed25519Pub"`
	MLDSA        []byte    `json:"mldsaPub"`
	MLDSAVariant Algorithm `json:"mldsaVariant"` // e.g., ML-DSA-65
}

// AlgorithmInfo provides metadata about a cryptographic algorithm
type AlgorithmInfo struct {
	ID              Algorithm `json:"id"`
	Name            string    `json:"name"`
	Family          string    `json:"family"`
	SecurityLevel   string    `json:"securityLevel"`
	NISTStandard    string    `json:"nistStandard,omitempty"`
	PublicKeySize   int       `json:"publicKeySize"`
	PrivateKeySize  int       `json:"privateKeySize"`
	SignatureSize   int       `json:"signatureSize"`
	Recommended     bool      `json:"recommended"`
	QuantumResistant bool     `json:"quantumResistant"`
}

// SupportedAlgorithms returns information about all supported algorithms
func SupportedAlgorithms() []AlgorithmInfo {
	return []AlgorithmInfo{
		{
			ID:               AlgorithmEd25519,
			Name:             "Ed25519",
			Family:           FamilyClassical,
			SecurityLevel:    "128-bit classical",
			PublicKeySize:    Ed25519PublicKeySize,
			PrivateKeySize:   Ed25519PrivateKeySize,
			SignatureSize:    Ed25519SignatureSize,
			Recommended:      false,
			QuantumResistant: false,
		},
		{
			ID:               AlgorithmMLDSA44,
			Name:             "ML-DSA-44 (Dilithium2)",
			Family:           FamilyPostQuantum,
			SecurityLevel:    "NIST Level 2",
			NISTStandard:     "FIPS 204",
			PublicKeySize:    MLDSA44PublicKeySize,
			PrivateKeySize:   MLDSA44PrivateKeySize,
			SignatureSize:    MLDSA44SignatureSize,
			Recommended:      false,
			QuantumResistant: true,
		},
		{
			ID:               AlgorithmMLDSA65,
			Name:             "ML-DSA-65 (Dilithium3)",
			Family:           FamilyPostQuantum,
			SecurityLevel:    "NIST Level 3",
			NISTStandard:     "FIPS 204",
			PublicKeySize:    MLDSA65PublicKeySize,
			PrivateKeySize:   MLDSA65PrivateKeySize,
			SignatureSize:    MLDSA65SignatureSize,
			Recommended:      false,
			QuantumResistant: true,
		},
		{
			ID:               AlgorithmMLDSA87,
			Name:             "ML-DSA-87 (Dilithium5)",
			Family:           FamilyPostQuantum,
			SecurityLevel:    "NIST Level 5",
			NISTStandard:     "FIPS 204",
			PublicKeySize:    MLDSA87PublicKeySize,
			PrivateKeySize:   MLDSA87PrivateKeySize,
			SignatureSize:    MLDSA87SignatureSize,
			Recommended:      false,
			QuantumResistant: true,
		},
		{
			ID:               AlgorithmHybridEd25519MLDSA65,
			Name:             "Ed25519 + ML-DSA-65 (Hybrid)",
			Family:           FamilyHybrid,
			SecurityLevel:    "NIST Level 3 + 128-bit classical",
			NISTStandard:     "FIPS 204 (ML-DSA component)",
			PublicKeySize:    Ed25519PublicKeySize + MLDSA65PublicKeySize,
			PrivateKeySize:   Ed25519PrivateKeySize + MLDSA65PrivateKeySize,
			SignatureSize:    Ed25519SignatureSize + MLDSA65SignatureSize,
			Recommended:      true, // Recommended for production use
			QuantumResistant: true,
		},
	}
}

// IsValidAlgorithm checks if the given algorithm is supported
func IsValidAlgorithm(alg Algorithm) bool {
	switch alg {
	case AlgorithmEd25519,
		AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87,
		AlgorithmHybridEd25519MLDSA44, AlgorithmHybridEd25519MLDSA65, AlgorithmHybridEd25519MLDSA87:
		return true
	default:
		return false
	}
}

// IsHybridAlgorithm checks if the algorithm is a hybrid algorithm
func IsHybridAlgorithm(alg Algorithm) bool {
	switch alg {
	case AlgorithmHybridEd25519MLDSA44, AlgorithmHybridEd25519MLDSA65, AlgorithmHybridEd25519MLDSA87:
		return true
	default:
		return false
	}
}

// IsPQCAlgorithm checks if the algorithm is post-quantum
func IsPQCAlgorithm(alg Algorithm) bool {
	switch alg {
	case AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87,
		AlgorithmHybridEd25519MLDSA44, AlgorithmHybridEd25519MLDSA65, AlgorithmHybridEd25519MLDSA87:
		return true
	default:
		return false
	}
}

// GetMLDSAVariant extracts the ML-DSA variant from a hybrid algorithm
func GetMLDSAVariant(alg Algorithm) Algorithm {
	switch alg {
	case AlgorithmHybridEd25519MLDSA44:
		return AlgorithmMLDSA44
	case AlgorithmHybridEd25519MLDSA65:
		return AlgorithmMLDSA65
	case AlgorithmHybridEd25519MLDSA87:
		return AlgorithmMLDSA87
	default:
		return alg
	}
}

// GetExpectedPublicKeySize returns the expected public key size for an algorithm
func GetExpectedPublicKeySize(alg Algorithm) (int, error) {
	switch alg {
	case AlgorithmEd25519:
		return Ed25519PublicKeySize, nil
	case AlgorithmMLDSA44:
		return MLDSA44PublicKeySize, nil
	case AlgorithmMLDSA65:
		return MLDSA65PublicKeySize, nil
	case AlgorithmMLDSA87:
		return MLDSA87PublicKeySize, nil
	default:
		return 0, ErrUnsupportedAlgorithm
	}
}

// GetPureAlgorithm extracts the pure ML-DSA algorithm from a hybrid or returns the algorithm as-is
// This is useful when storing the algorithm in the database (we store pure, not hybrid)
func GetPureAlgorithm(alg Algorithm) Algorithm {
	switch alg {
	case AlgorithmHybridEd25519MLDSA44:
		return AlgorithmMLDSA44
	case AlgorithmHybridEd25519MLDSA65:
		return AlgorithmMLDSA65
	case AlgorithmHybridEd25519MLDSA87:
		return AlgorithmMLDSA87
	default:
		return alg
	}
}

// GetExpectedSignatureSize returns the expected signature size for an algorithm
func GetExpectedSignatureSize(alg Algorithm) (int, error) {
	switch alg {
	case AlgorithmEd25519:
		return Ed25519SignatureSize, nil
	case AlgorithmMLDSA44:
		return MLDSA44SignatureSize, nil
	case AlgorithmMLDSA65:
		return MLDSA65SignatureSize, nil
	case AlgorithmMLDSA87:
		return MLDSA87SignatureSize, nil
	default:
		return 0, ErrUnsupportedAlgorithm
	}
}
