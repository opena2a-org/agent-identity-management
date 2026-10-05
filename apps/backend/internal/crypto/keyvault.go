package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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

	// A key of 32 zero bytes passes the length check but is a placeholder,
	// not a secret: anyone can decrypt what it encrypts.
	if subtle.ConstantTimeCompare(masterKey, make([]byte, len(masterKey))) == 1 {
		return nil, fmt.Errorf("master key must not be all zero bytes; generate one with `openssl rand -base64 32`")
	}

	return &KeyVault{
		masterKey: masterKey,
	}, nil
}

// ephemeralKeyEnvironments are the ENVIRONMENT values that may run on a
// generated master key. Matching is exact: an unset, misspelled or unrecognised
// value is never treated as development.
var ephemeralKeyEnvironments = map[string]bool{
	"development": true,
	"test":        true,
}

// NewKeyVaultFromEnv creates a KeyVault using a master key from environment variable
// SECURITY: Master key MUST be set everywhere except an explicit development or test environment
func NewKeyVaultFromEnv() (*KeyVault, error) {
	masterKeyBase64 := os.Getenv("KEYVAULT_MASTER_KEY")
	environment := os.Getenv("ENVIRONMENT")

	if masterKeyBase64 == "" {
		// A generated key changes at every restart, which changes the server
		// signing key and leaves stored agent private keys undecryptable, so
		// only an explicit development or test setting may accept it.
		if !ephemeralKeyEnvironments[environment] {
			return nil, fmt.Errorf("SECURITY ERROR: KEYVAULT_MASTER_KEY is required when ENVIRONMENT=%q; "+
				"set KEYVAULT_MASTER_KEY (generate one with: openssl rand -base64 32), "+
				"or set ENVIRONMENT=development to use a key that is regenerated at every start", environment)
		}

		// Generate a new master key for development only
		// SECURITY: This key is ephemeral and will be different on each restart
		// This is acceptable for development but NOT for production
		fmt.Println("⚠️  SECURITY WARNING: KEYVAULT_MASTER_KEY not set")
		fmt.Printf("   Generating ephemeral master key because ENVIRONMENT=%s.\n", environment)
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

// GetServerSigningKey derives a deterministic Ed25519 server signing key from the master key.
// This key is used for server-issued attestations and should NOT be the agent's own key.
// The derivation uses HKDF-like key stretching via SHA-256 with a domain separator.
func (kv *KeyVault) GetServerSigningKey() ed25519.PrivateKey {
	// Derive a 32-byte seed from master key using SHA-256 with domain separator
	h := sha256.New()
	h.Write([]byte("aim-server-attestation-signing-key-v1:"))
	h.Write(kv.masterKey)
	seed := h.Sum(nil) // 32 bytes

	// Generate Ed25519 key from seed (deterministic)
	return ed25519.NewKeyFromSeed(seed)
}

// GetServerSigningPublicKey returns the public key corresponding to the server signing key.
func (kv *KeyVault) GetServerSigningPublicKey() ed25519.PublicKey {
	privateKey := kv.GetServerSigningKey()
	return privateKey.Public().(ed25519.PublicKey)
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
