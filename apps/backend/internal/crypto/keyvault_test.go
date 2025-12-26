package crypto

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestMasterKey creates a valid 32-byte base64-encoded master key for testing
func generateTestMasterKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewKeyVault_Valid(t *testing.T) {
	masterKey := generateTestMasterKey()

	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)
	assert.NotNil(t, kv)
	assert.Len(t, kv.masterKey, 32)
}

func TestNewKeyVault_EmptyKey(t *testing.T) {
	kv, err := NewKeyVault("")
	assert.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "master key is required")
}

func TestNewKeyVault_InvalidBase64(t *testing.T) {
	kv, err := NewKeyVault("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "failed to decode master key")
}

func TestNewKeyVault_WrongKeySize(t *testing.T) {
	// Create a 16-byte key (too short)
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	kv, err := NewKeyVault(shortKey)
	assert.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "master key must be 32 bytes")

	// Create a 64-byte key (too long)
	longKey := base64.StdEncoding.EncodeToString(make([]byte, 64))
	kv, err = NewKeyVault(longKey)
	assert.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "master key must be 32 bytes")
}

func TestNewKeyVaultFromEnv_WithEnvSet(t *testing.T) {
	masterKey := generateTestMasterKey()
	os.Setenv("KEYVAULT_MASTER_KEY", masterKey)
	defer os.Unsetenv("KEYVAULT_MASTER_KEY")

	kv, err := NewKeyVaultFromEnv()
	require.NoError(t, err)
	assert.NotNil(t, kv)
}

func TestNewKeyVaultFromEnv_ProductionRequiresKey(t *testing.T) {
	os.Unsetenv("KEYVAULT_MASTER_KEY")
	os.Setenv("ENVIRONMENT", "production")
	defer os.Unsetenv("ENVIRONMENT")

	kv, err := NewKeyVaultFromEnv()
	assert.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "SECURITY ERROR")
}

func TestNewKeyVaultFromEnv_DevelopmentGeneratesKey(t *testing.T) {
	os.Unsetenv("KEYVAULT_MASTER_KEY")
	os.Setenv("ENVIRONMENT", "development")
	defer os.Unsetenv("ENVIRONMENT")

	kv, err := NewKeyVaultFromEnv()
	require.NoError(t, err)
	assert.NotNil(t, kv)
	assert.Len(t, kv.masterKey, 32)
}

func TestEncryptPrivateKey_Success(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	privateKey := "test-private-key-data-here"
	encrypted, err := kv.EncryptPrivateKey(privateKey)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, privateKey, encrypted)

	// Verify it's valid base64
	_, err = base64.StdEncoding.DecodeString(encrypted)
	assert.NoError(t, err)
}

func TestEncryptPrivateKey_DifferentEachTime(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	privateKey := "test-private-key-data"

	encrypted1, err := kv.EncryptPrivateKey(privateKey)
	require.NoError(t, err)

	encrypted2, err := kv.EncryptPrivateKey(privateKey)
	require.NoError(t, err)

	// Each encryption should produce different ciphertext (due to random nonce)
	assert.NotEqual(t, encrypted1, encrypted2)
}

func TestDecryptPrivateKey_Success(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalKey := "my-super-secret-private-key"

	encrypted, err := kv.EncryptPrivateKey(originalKey)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, originalKey, decrypted)
}

func TestDecryptPrivateKey_InvalidBase64(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	_, err = kv.DecryptPrivateKey("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode ciphertext")
}

func TestDecryptPrivateKey_CiphertextTooShort(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	// Create a very short ciphertext (shorter than nonce size)
	shortCiphertext := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = kv.DecryptPrivateKey(shortCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestDecryptPrivateKey_TamperedCiphertext(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalKey := "my-secret-key"
	encrypted, err := kv.EncryptPrivateKey(originalKey)
	require.NoError(t, err)

	// Decode, tamper, re-encode
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	ciphertext[len(ciphertext)-1] ^= 0xFF // Flip bits in last byte
	tampered := base64.StdEncoding.EncodeToString(ciphertext)

	_, err = kv.DecryptPrivateKey(tampered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

func TestDecryptPrivateKey_WrongMasterKey(t *testing.T) {
	masterKey1 := generateTestMasterKey()
	kv1, err := NewKeyVault(masterKey1)
	require.NoError(t, err)

	// Create a different master key
	differentKey := make([]byte, 32)
	for i := range differentKey {
		differentKey[i] = byte(255 - i)
	}
	masterKey2 := base64.StdEncoding.EncodeToString(differentKey)
	kv2, err := NewKeyVault(masterKey2)
	require.NoError(t, err)

	originalKey := "my-secret-key"
	encrypted, err := kv1.EncryptPrivateKey(originalKey)
	require.NoError(t, err)

	// Try to decrypt with different master key
	_, err = kv2.DecryptPrivateKey(encrypted)
	assert.Error(t, err)
}

func TestRotatePrivateKey_Success(t *testing.T) {
	// Create old master key
	oldMasterKey := generateTestMasterKey()
	kv, err := NewKeyVault(oldMasterKey)
	require.NoError(t, err)

	// Create new master key
	newKeyBytes := make([]byte, 32)
	for i := range newKeyBytes {
		newKeyBytes[i] = byte(255 - i)
	}
	newMasterKey := base64.StdEncoding.EncodeToString(newKeyBytes)

	// Encrypt with old key
	originalPrivateKey := "my-original-private-key"
	encrypted, err := kv.EncryptPrivateKey(originalPrivateKey)
	require.NoError(t, err)

	// Rotate to new key
	rotated, err := kv.RotatePrivateKey(encrypted, newMasterKey)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated)
	assert.NotEqual(t, encrypted, rotated)

	// Verify we can decrypt with new key
	newKv, err := NewKeyVault(newMasterKey)
	require.NoError(t, err)

	decrypted, err := newKv.DecryptPrivateKey(rotated)
	require.NoError(t, err)
	assert.Equal(t, originalPrivateKey, decrypted)
}

func TestRotatePrivateKey_InvalidEncryptedKey(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	newKeyBytes := make([]byte, 32)
	newMasterKey := base64.StdEncoding.EncodeToString(newKeyBytes)

	_, err = kv.RotatePrivateKey("invalid-encrypted-data", newMasterKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt with old key")
}

func TestRotatePrivateKey_InvalidNewMasterKey(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalPrivateKey := "my-private-key"
	encrypted, err := kv.EncryptPrivateKey(originalPrivateKey)
	require.NoError(t, err)

	// Try to rotate with invalid new master key
	_, err = kv.RotatePrivateKey(encrypted, "invalid-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create new vault")
}

func TestEncryptDecrypt_RoundTrip_EmptyString(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	original := ""
	encrypted, err := kv.EncryptPrivateKey(original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

func TestEncryptDecrypt_RoundTrip_LargeData(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	// Create a large private key (1KB)
	largeKey := make([]byte, 1024)
	for i := range largeKey {
		largeKey[i] = byte(i % 256)
	}
	original := base64.StdEncoding.EncodeToString(largeKey)

	encrypted, err := kv.EncryptPrivateKey(original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

func TestEncryptDecrypt_RoundTrip_SpecialCharacters(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	// Test with special characters that might cause issues
	original := "key-with-special-chars: \n\t\r\x00 日本語 emoji: 🔐"

	encrypted, err := kv.EncryptPrivateKey(original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}
