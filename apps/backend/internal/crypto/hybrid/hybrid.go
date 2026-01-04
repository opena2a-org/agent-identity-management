package hybrid

import (
	"encoding/base64"
	"fmt"
)

// HybridSigner combines Ed25519 and ML-DSA-65 for defense-in-depth.
//
// During the post-quantum transition period, hybrid signatures provide
// security against both classical and quantum attackers:
// - If quantum computers never materialize: Ed25519 remains secure
// - If quantum computers break Ed25519: ML-DSA-65 remains secure
// - Both signatures must verify for the hybrid to be valid
type HybridSigner struct {
	classical *Ed25519Signer
	pqc       *MLDSASigner
}

// NewHybridSigner generates new key pairs for both algorithms.
func NewHybridSigner() (*HybridSigner, error) {
	classical, err := NewEd25519Signer()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ed25519 signer: %w", err)
	}

	pqc, err := NewMLDSASigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create ML-DSA signer: %w", err)
	}

	return &HybridSigner{
		classical: classical,
		pqc:       pqc,
	}, nil
}

// NewHybridSignerFromKeys creates a hybrid signer from existing keys.
func NewHybridSignerFromKeys(keyPair *HybridKeyPair) (*HybridSigner, error) {
	classical, err := NewEd25519SignerFromKeys(keyPair.ClassicalPublic, keyPair.ClassicalPrivate)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ed25519 signer: %w", err)
	}

	pqc, err := NewMLDSASignerFromKeys(keyPair.PQCPublic, keyPair.PQCPrivate)
	if err != nil {
		return nil, fmt.Errorf("failed to create ML-DSA signer: %w", err)
	}

	return &HybridSigner{
		classical: classical,
		pqc:       pqc,
	}, nil
}

// Algorithm returns the algorithm identifier.
func (s *HybridSigner) Algorithm() Algorithm {
	return AlgorithmHybrid
}

// PublicKey returns the combined public keys as a base64-encoded hybrid format.
// Format: base64(classical_pub || pqc_pub)
func (s *HybridSigner) PublicKey() string {
	classical := s.classical.PublicKeyBytes()
	pqc := s.pqc.PublicKeyBytes()

	combined := make([]byte, len(classical)+len(pqc))
	copy(combined[:len(classical)], classical)
	copy(combined[len(classical):], pqc)

	return base64.StdEncoding.EncodeToString(combined)
}

// PublicKeyBytes returns the combined public key bytes.
func (s *HybridSigner) PublicKeyBytes() []byte {
	classical := s.classical.PublicKeyBytes()
	pqc := s.pqc.PublicKeyBytes()

	combined := make([]byte, len(classical)+len(pqc))
	copy(combined[:len(classical)], classical)
	copy(combined[len(classical):], pqc)

	return combined
}

// ClassicalPublicKey returns the Ed25519 public key.
func (s *HybridSigner) ClassicalPublicKey() string {
	return s.classical.PublicKey()
}

// PQCPublicKey returns the ML-DSA-65 public key.
func (s *HybridSigner) PQCPublicKey() string {
	return s.pqc.PublicKey()
}

// KeyPair returns the hybrid key pair for storage.
func (s *HybridSigner) KeyPair() *HybridKeyPair {
	return &HybridKeyPair{
		Algorithm:        AlgorithmHybrid,
		ClassicalPublic:  s.classical.PublicKey(),
		ClassicalPrivate: s.classical.PrivateKey(),
		PQCPublic:        s.pqc.PublicKey(),
		PQCPrivate:       s.pqc.PrivateKey(),
	}
}

// IsSimulated returns true if the PQC component is using placeholder implementation.
func (s *HybridSigner) IsSimulated() bool {
	return s.pqc.IsSimulated()
}

// Sign signs a message with both algorithms and returns the hybrid signature.
func (s *HybridSigner) Sign(message []byte) (string, error) {
	sig, err := s.SignBytes(message)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// SignBytes signs a message with both algorithms and returns encoded hybrid signature.
func (s *HybridSigner) SignBytes(message []byte) ([]byte, error) {
	// Sign with classical algorithm
	classicalSig, err := s.classical.SignBytes(message)
	if err != nil {
		return nil, fmt.Errorf("classical signing failed: %w", err)
	}

	// Sign with PQC algorithm
	pqcSig, err := s.pqc.SignBytes(message)
	if err != nil {
		return nil, fmt.Errorf("PQC signing failed: %w", err)
	}

	// Encode as hybrid signature
	hybrid := &HybridSignature{
		Classical: classicalSig,
		PQC:       pqcSig,
	}

	return EncodeHybridSignature(hybrid), nil
}

// Verify verifies both signatures in a hybrid signature.
// BOTH must be valid for the hybrid signature to be valid.
func (s *HybridSigner) Verify(message []byte, signatureB64 string) (bool, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}
	return s.VerifyBytes(message, signature)
}

// VerifyBytes verifies both signatures in a hybrid signature.
func (s *HybridSigner) VerifyBytes(message []byte, signature []byte) (bool, error) {
	// Decode hybrid signature
	hybrid, err := DecodeHybridSignature(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode hybrid signature: %w", err)
	}

	// Verify classical signature
	classicalValid, err := s.classical.VerifyBytes(message, hybrid.Classical)
	if err != nil {
		return false, fmt.Errorf("classical verification failed: %w", err)
	}
	if !classicalValid {
		return false, nil
	}

	// Verify PQC signature
	pqcValid, err := s.pqc.VerifyBytes(message, hybrid.PQC)
	if err != nil {
		return false, fmt.Errorf("PQC verification failed: %w", err)
	}

	// Both must be valid
	return classicalValid && pqcValid, nil
}

// HybridVerifier verifies hybrid signatures without private key access.
type HybridVerifier struct {
	classical *Ed25519Verifier
	pqc       *MLDSAVerifier
}

// NewHybridVerifier creates a verifier from both public keys.
func NewHybridVerifier(classicalPublicB64, pqcPublicB64 string) (*HybridVerifier, error) {
	classical, err := NewEd25519VerifierFromPublicKey(classicalPublicB64)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ed25519 verifier: %w", err)
	}

	pqc, err := NewMLDSAVerifierFromPublicKey(pqcPublicB64)
	if err != nil {
		return nil, fmt.Errorf("failed to create ML-DSA verifier: %w", err)
	}

	return &HybridVerifier{
		classical: classical,
		pqc:       pqc,
	}, nil
}

// Algorithm returns the algorithm identifier.
func (v *HybridVerifier) Algorithm() Algorithm {
	return AlgorithmHybrid
}

// Verify verifies both signatures in a hybrid signature.
func (v *HybridVerifier) Verify(message []byte, signatureB64 string) (bool, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}
	return v.VerifyBytes(message, signature)
}

// VerifyBytes verifies both signatures in a hybrid signature.
func (v *HybridVerifier) VerifyBytes(message []byte, signature []byte) (bool, error) {
	// Decode hybrid signature
	hybrid, err := DecodeHybridSignature(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode hybrid signature: %w", err)
	}

	// Verify classical signature
	classicalValid, err := v.classical.VerifyBytes(message, hybrid.Classical)
	if err != nil {
		return false, fmt.Errorf("classical verification failed: %w", err)
	}
	if !classicalValid {
		return false, nil
	}

	// Verify PQC signature
	pqcValid, err := v.pqc.VerifyBytes(message, hybrid.PQC)
	if err != nil {
		return false, fmt.Errorf("PQC verification failed: %w", err)
	}

	// Both must be valid
	return classicalValid && pqcValid, nil
}

// SignatureInfo contains details about signature verification.
type SignatureInfo struct {
	Algorithm          Algorithm `json:"algorithm"`
	ClassicalValid     bool      `json:"classical_valid"`
	PQCValid           bool      `json:"pqc_valid"`
	ClassicalAlgorithm string    `json:"classical_algorithm"`
	PQCAlgorithm       string    `json:"pqc_algorithm"`
	SignatureSize      int       `json:"signature_size"`
}

// VerifyWithInfo verifies and returns detailed information.
func (v *HybridVerifier) VerifyWithInfo(message []byte, signatureB64 string) (*SignatureInfo, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Decode hybrid signature
	hybrid, err := DecodeHybridSignature(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hybrid signature: %w", err)
	}

	info := &SignatureInfo{
		Algorithm:          AlgorithmHybrid,
		ClassicalAlgorithm: string(AlgorithmEd25519),
		PQCAlgorithm:       string(AlgorithmMLDSA65),
		SignatureSize:      len(signature),
	}

	// Verify classical
	classicalValid, _ := v.classical.VerifyBytes(message, hybrid.Classical)
	info.ClassicalValid = classicalValid

	// Verify PQC
	pqcValid, _ := v.pqc.VerifyBytes(message, hybrid.PQC)
	info.PQCValid = pqcValid

	return info, nil
}
