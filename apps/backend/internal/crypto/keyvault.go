package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// KeyVault handles secure storage and retrieval of private keys
// Uses AES-256-GCM for encryption at rest
type KeyVault struct {
	masterKey []byte // AES-256 key (32 bytes)
}

// NewKeyVault creates a new KeyVault instance
// The master key should be stored securely (e.g., environment variable, secrets manager)
func NewKeyVault(masterKeyBase64 string) (*KeyVault, error) {
	if masterKeyBase64 == "" {
		return nil, fmt.Errorf("master key is required")
	}

	masterKey, err := base64.StdEncoding.DecodeString(masterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode master key: %w", err)
	}

	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (AES-256), got %d bytes", len(masterKey))
	}

	return &KeyVault{
		masterKey: masterKey,
	}, nil
}

// NewKeyVaultFromEnv creates a KeyVault using a master key from environment variable
// SECURITY: Master key MUST be set in production environments
func NewKeyVaultFromEnv() (*KeyVault, error) {
	masterKeyBase64 := os.Getenv("KEYVAULT_MASTER_KEY")
	environment := os.Getenv("ENVIRONMENT")

	if masterKeyBase64 == "" {
		// In production, master key is REQUIRED
		if environment == "production" {
			return nil, fmt.Errorf("SECURITY ERROR: KEYVAULT_MASTER_KEY environment variable is required in production")
		}

		// Generate a new master key for development only
		// SECURITY: This key is ephemeral and will be different on each restart
		// This is acceptable for development but NOT for production
		fmt.Println("⚠️  SECURITY WARNING: KEYVAULT_MASTER_KEY not set")
		fmt.Println("   Generating ephemeral master key for development only.")
		fmt.Println("   Set KEYVAULT_MASTER_KEY environment variable in production!")

		masterKey := make([]byte, 32)
		if _, err := rand.Read(masterKey); err != nil {
			return nil, fmt.Errorf("failed to generate master key: %w", err)
		}
		masterKeyBase64 = base64.StdEncoding.EncodeToString(masterKey)

		// SECURITY: Never print the actual key - only a notification
		fmt.Println("   Generated ephemeral 256-bit AES key for this session.")
	}

	return NewKeyVault(masterKeyBase64)
}

// EncryptPrivateKey encrypts a private key using AES-256-GCM
func (kv *KeyVault) EncryptPrivateKey(privateKeyBase64 string) (string, error) {
	block, err := aes.NewCipher(kv.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the private key
	plaintext := []byte(privateKeyBase64)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return base64-encoded encrypted data
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPrivateKey decrypts an encrypted private key
func (kv *KeyVault) DecryptPrivateKey(encryptedPrivateKey string) (string, error) {
	block, err := aes.NewCipher(kv.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// RotatePrivateKey decrypts with old key, re-encrypts with new key
func (kv *KeyVault) RotatePrivateKey(encryptedPrivateKey string, newMasterKeyBase64 string) (string, error) {
	// Decrypt with current master key
	privateKeyBase64, err := kv.DecryptPrivateKey(encryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Create new vault with new master key
	newVault, err := NewKeyVault(newMasterKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to create new vault: %w", err)
	}

	// Encrypt with new master key
	newEncrypted, err := newVault.EncryptPrivateKey(privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newEncrypted, nil
}
