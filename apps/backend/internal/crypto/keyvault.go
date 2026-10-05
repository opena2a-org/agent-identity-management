package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
)

// KeyVault handles secure storage and retrieval of private keys
// Uses AES-256-GCM for encryption at rest, bound to the storing agent row
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

// Stored agent private keys use format v2:
//
//	header (version byte, storage-key id) || nonce || AES-256-GCM ciphertext and tag
//
// The header travels in cleartext and is authenticated as part of the
// additional data, which also binds the purpose label and the ID of the agent
// row the ciphertext is stored in:
//
//	"aim/agent-private-key" 0x00 header 0x00 agents.id (canonical lowercase)
//
// A ciphertext copied from one agent row to another therefore fails to decrypt.
// Callers pass the ID of the row they loaded, never an ID taken from a request.
const (
	agentPrivateKeyPurpose = "aim/agent-private-key"

	privateKeyFormatV2 byte = 0x02

	// currentStorageKeyID names the single master key in use. It becomes a
	// lookup into a storage key ring when one exists.
	currentStorageKeyID byte = 0x01

	privateKeyHeaderSize = 2
)

// ErrUnsupportedPrivateKeyFormat is returned when a stored private key is not
// in format v2, including a v1 ciphertext that predates storage binding.
var ErrUnsupportedPrivateKeyFormat = errors.New("unsupported private key format")

func (kv *KeyVault) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(kv.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}

// storageAAD builds the additional data that binds a ciphertext to its purpose,
// its header and the agent row it is stored in.
func storageAAD(purpose string, header []byte, agentID uuid.UUID) []byte {
	id := agentID.String() // canonical lowercase form
	aad := make([]byte, 0, len(purpose)+1+len(header)+1+len(id))
	aad = append(aad, purpose...)
	aad = append(aad, 0x00)
	aad = append(aad, header...)
	aad = append(aad, 0x00)
	aad = append(aad, id...)
	return aad
}

func (kv *KeyVault) sealV2(purpose string, agentID uuid.UUID, plaintext []byte) (string, error) {
	if agentID == uuid.Nil {
		return "", fmt.Errorf("agent ID is required to encrypt a private key")
	}

	gcm, err := kv.newGCM()
	if err != nil {
		return "", err
	}

	header := []byte{privateKeyFormatV2, currentStorageKeyID}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	out := make([]byte, 0, len(header)+len(nonce)+len(plaintext)+gcm.Overhead())
	out = append(out, header...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, storageAAD(purpose, header, agentID))

	return base64.StdEncoding.EncodeToString(out), nil
}

func (kv *KeyVault) openV2(purpose string, agentID uuid.UUID, encoded string) ([]byte, error) {
	if agentID == uuid.Nil {
		return nil, fmt.Errorf("agent ID is required to decrypt a private key")
	}

	gcm, err := kv.newGCM()
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	if len(data) < privateKeyHeaderSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	header := data[:privateKeyHeaderSize]
	if header[0] != privateKeyFormatV2 {
		return nil, ErrUnsupportedPrivateKeyFormat
	}
	if header[1] != currentStorageKeyID {
		return nil, fmt.Errorf("unknown storage key id %d", header[1])
	}

	rest := data[privateKeyHeaderSize:]
	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := rest[:nonceSize], rest[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, storageAAD(purpose, header, agentID))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

// EncryptPrivateKey encrypts a private key for storage in the agent row with
// the given ID, using AES-256-GCM with the row bound as additional data.
func (kv *KeyVault) EncryptPrivateKey(agentID uuid.UUID, privateKeyBase64 string) (string, error) {
	return kv.sealV2(agentPrivateKeyPurpose, agentID, []byte(privateKeyBase64))
}

// DecryptPrivateKey decrypts the private key stored in the agent row with the
// given ID. agentID must come from the row the ciphertext was loaded from. A
// ciphertext stored for a different row, or one in the unbound v1 format, is
// refused.
func (kv *KeyVault) DecryptPrivateKey(agentID uuid.UUID, encryptedPrivateKey string) (string, error) {
	plaintext, err := kv.openV2(agentPrivateKeyPurpose, agentID, encryptedPrivateKey)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// MigrateLegacyPrivateKey re-encrypts a v1 private key (AES-256-GCM with no
// additional data) to format v2 bound to agentID. A value that is already v2
// for agentID is returned unchanged with migrated=false. Only the one-time
// startup migration calls this; request paths use DecryptPrivateKey, which
// never reads v1.
func (kv *KeyVault) MigrateLegacyPrivateKey(agentID uuid.UUID, encryptedPrivateKey string) (string, bool, error) {
	if _, err := kv.openV2(agentPrivateKeyPurpose, agentID, encryptedPrivateKey); err == nil {
		return encryptedPrivateKey, false, nil
	}

	plaintext, err := kv.openLegacyV1(encryptedPrivateKey)
	if err != nil {
		return "", false, err
	}

	migrated, err := kv.sealV2(agentPrivateKeyPurpose, agentID, plaintext)
	if err != nil {
		return "", false, err
	}
	return migrated, true, nil
}

// openLegacyV1 decrypts the v1 format: nonce || ciphertext with no additional
// data. It is reachable only through MigrateLegacyPrivateKey.
func (kv *KeyVault) openLegacyV1(encryptedPrivateKey string) ([]byte, error) {
	gcm, err := kv.newGCM()
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

// RotatePrivateKey decrypts with old key, re-encrypts with new key. The
// ciphertext stays bound to the same agent row.
func (kv *KeyVault) RotatePrivateKey(agentID uuid.UUID, encryptedPrivateKey string, newMasterKeyBase64 string) (string, error) {
	// Decrypt with current master key
	privateKeyBase64, err := kv.DecryptPrivateKey(agentID, encryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Create new vault with new master key
	newVault, err := NewKeyVault(newMasterKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to create new vault: %w", err)
	}

	// Encrypt with new master key
	newEncrypted, err := newVault.EncryptPrivateKey(agentID, privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newEncrypted, nil
}
