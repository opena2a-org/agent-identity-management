package hybrid

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Ed25519Signer implements the Signer interface using Ed25519.
type Ed25519Signer struct {
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

// NewEd25519Signer generates a new Ed25519 key pair and returns a signer.
func NewEd25519Signer() (*Ed25519Signer, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}

	return &Ed25519Signer{
		publicKey:  publicKey,
		privateKey: privateKey,
	}, nil
}

// NewEd25519SignerFromKeys creates a signer from existing base64-encoded keys.
func NewEd25519SignerFromKeys(publicKeyB64, privateKeyB64 string) (*Ed25519Signer, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d",
			ed25519.PublicKeySize, len(publicKeyBytes))
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d",
			ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	return &Ed25519Signer{
		publicKey:  ed25519.PublicKey(publicKeyBytes),
		privateKey: ed25519.PrivateKey(privateKeyBytes),
	}, nil
}

// Algorithm returns the algorithm identifier.
func (s *Ed25519Signer) Algorithm() Algorithm {
	return AlgorithmEd25519
}

// PublicKey returns the base64-encoded public key.
func (s *Ed25519Signer) PublicKey() string {
	return base64.StdEncoding.EncodeToString(s.publicKey)
}

// PublicKeyBytes returns the raw public key bytes.
func (s *Ed25519Signer) PublicKeyBytes() []byte {
	return s.publicKey
}

// PrivateKey returns the base64-encoded private key.
func (s *Ed25519Signer) PrivateKey() string {
	return base64.StdEncoding.EncodeToString(s.privateKey)
}

// KeyPair returns the key pair for storage.
func (s *Ed25519Signer) KeyPair() *KeyPair {
	return &KeyPair{
		Algorithm:  AlgorithmEd25519,
		PublicKey:  s.PublicKey(),
		PrivateKey: s.PrivateKey(),
	}
}

// Sign signs a message and returns the base64-encoded signature.
func (s *Ed25519Signer) Sign(message []byte) (string, error) {
	sig, err := s.SignBytes(message)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// SignBytes signs a message and returns raw signature bytes.
func (s *Ed25519Signer) SignBytes(message []byte) ([]byte, error) {
	if s.privateKey == nil {
		return nil, fmt.Errorf("no private key available for signing")
	}
	return ed25519.Sign(s.privateKey, message), nil
}

// Verify verifies a base64-encoded signature against a message.
func (s *Ed25519Signer) Verify(message []byte, signatureB64 string) (bool, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}
	return s.VerifyBytes(message, signature)
}

// VerifyBytes verifies raw signature bytes against a message.
func (s *Ed25519Signer) VerifyBytes(message []byte, signature []byte) (bool, error) {
	if len(signature) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature size: expected %d, got %d",
			ed25519.SignatureSize, len(signature))
	}
	return ed25519.Verify(s.publicKey, message, signature), nil
}

// Ed25519Verifier verifies Ed25519 signatures without private key access.
type Ed25519Verifier struct {
	publicKey ed25519.PublicKey
}

// NewEd25519VerifierFromPublicKey creates a verifier from a base64-encoded public key.
func NewEd25519VerifierFromPublicKey(publicKeyB64 string) (*Ed25519Verifier, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d",
			ed25519.PublicKeySize, len(publicKeyBytes))
	}

	return &Ed25519Verifier{
		publicKey: ed25519.PublicKey(publicKeyBytes),
	}, nil
}

// Algorithm returns the algorithm identifier.
func (v *Ed25519Verifier) Algorithm() Algorithm {
	return AlgorithmEd25519
}

// Verify verifies a base64-encoded signature against a message.
func (v *Ed25519Verifier) Verify(message []byte, signatureB64 string) (bool, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}
	return v.VerifyBytes(message, signature)
}

// VerifyBytes verifies raw signature bytes against a message.
func (v *Ed25519Verifier) VerifyBytes(message []byte, signature []byte) (bool, error) {
	if len(signature) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature size: expected %d, got %d",
			ed25519.SignatureSize, len(signature))
	}
	return ed25519.Verify(v.publicKey, message, signature), nil
}
