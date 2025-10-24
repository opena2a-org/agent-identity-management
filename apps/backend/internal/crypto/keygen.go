package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// KeyPair represents an Ed25519 cryptographic key pair
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// KeyPairEncoded represents a key pair with base64-encoded keys
type KeyPairEncoded struct {
	PublicKeyBase64  string
	PrivateKeyBase64 string
	Algorithm        string
}

// GenerateEd25519KeyPair generates a new Ed25519 key pair
// Ed25519 is chosen for:
// - Faster signing/verification than RSA
// - Smaller key sizes (32 bytes public, 64 bytes private)
// - Resistance to timing attacks
// - Widely used in modern cryptographic applications
func GenerateEd25519KeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// EncodeKeyPair converts a KeyPair to base64-encoded strings
func EncodeKeyPair(kp *KeyPair) *KeyPairEncoded {
	return &KeyPairEncoded{
		PublicKeyBase64:  base64.StdEncoding.EncodeToString(kp.PublicKey),
		PrivateKeyBase64: base64.StdEncoding.EncodeToString(kp.PrivateKey),
		Algorithm:        "Ed25519",
	}
}

// DecodePublicKey decodes a base64-encoded public key
func DecodePublicKey(publicKeyBase64 string) (ed25519.PublicKey, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	return ed25519.PublicKey(publicKeyBytes), nil
}

// DecodePrivateKey decodes a base64-encoded private key
func DecodePrivateKey(privateKeyBase64 string) (ed25519.PrivateKey, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d bytes, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	return ed25519.PrivateKey(privateKeyBytes), nil
}

// SignMessage signs a message with a private key
func SignMessage(privateKey ed25519.PrivateKey, message []byte) []byte {
	return ed25519.Sign(privateKey, message)
}

// VerifySignature verifies a signature with a public key
func VerifySignature(publicKey ed25519.PublicKey, message, signature []byte) bool {
	return ed25519.Verify(publicKey, message, signature)
}
