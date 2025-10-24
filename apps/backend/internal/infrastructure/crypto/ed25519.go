package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// ED25519Service handles ED25519 cryptographic operations
type ED25519Service struct{}

// NewED25519Service creates a new ED25519 service
func NewED25519Service() *ED25519Service {
	return &ED25519Service{}
}

// KeyPair represents an ED25519 keypair
type KeyPair struct {
	PublicKey  string `json:"public_key"`  // Base64 encoded
	PrivateKey string `json:"private_key"` // Base64 encoded
}

// GenerateKeyPair generates a new ED25519 keypair
func (s *ED25519Service) GenerateKeyPair() (*KeyPair, error) {
	// Generate ED25519 keypair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Encode keys to base64 for storage/transmission
	return &KeyPair{
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}, nil
}

// Sign signs a message with a private key
func (s *ED25519Service) Sign(privateKeyB64 string, message []byte) (string, error) {
	// Decode private key from base64
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// Verify private key length
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	// Sign the message
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), message)

	// Return base64 encoded signature
	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify verifies a signature with a public key
func (s *ED25519Service) Verify(publicKeyB64 string, message []byte, signatureB64 string) (bool, error) {
	// Decode public key from base64
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode public key: %w", err)
	}

	// Verify public key length
	if len(publicKey) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature
	valid := ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
	return valid, nil
}

// GenerateChallenge generates a random challenge for verification
func (s *ED25519Service) GenerateChallenge() (string, error) {
	// Generate 32 bytes of random data
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Return base64 encoded challenge
	return base64.StdEncoding.EncodeToString(challenge), nil
}
