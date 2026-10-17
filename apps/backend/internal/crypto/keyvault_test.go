package crypto

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAgentID is the agent row the existing round-trip tests store keys in.
var testAgentID = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")

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
	require.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "master key is required")
}

func TestNewKeyVault_InvalidBase64(t *testing.T) {
	kv, err := NewKeyVault("not-valid-base64!!!")
	require.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "failed to decode master key")
}

func TestNewKeyVault_WrongKeySize(t *testing.T) {
	// Create a 16-byte key (too short)
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	kv, err := NewKeyVault(shortKey)
	require.Error(t, err)
	assert.Nil(t, kv)
	assert.Contains(t, err.Error(), "master key must be 32 bytes")

	// Create a 64-byte key (too long)
	longKey := base64.StdEncoding.EncodeToString(make([]byte, 64))
	kv, err = NewKeyVault(longKey)
	require.Error(t, err)
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
	require.Error(t, err)
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

// TestNewKeyVaultFromEnv_NoMasterKeyNeedsExplicitDevelopment covers every
// ENVIRONMENT value a deployment can carry when KEYVAULT_MASTER_KEY is missing.
// A generated key changes at every start, so the server signing key changes
// and stored agent private keys can no longer be decrypted after a restart.
// Only an explicit development or test setting may accept that; an unset,
// misspelled or unrecognised value refuses to start.
func TestNewKeyVaultFromEnv_NoMasterKeyNeedsExplicitDevelopment(t *testing.T) {
	cases := []struct {
		name        string
		environment string
		unset       bool
		wantVault   bool
	}{
		{name: "unset", unset: true},
		{name: "empty", environment: ""},
		{name: "prod", environment: "prod"},
		{name: "production", environment: "production"},
		{name: "staging", environment: "staging"},
		{name: "capitalised development", environment: "Development"},
		{name: "testing", environment: "testing"},
		{name: "development", environment: "development", wantVault: true},
		{name: "test", environment: "test", wantVault: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KEYVAULT_MASTER_KEY", "")
			t.Setenv("ENVIRONMENT", tc.environment)
			if tc.unset {
				require.NoError(t, os.Unsetenv("ENVIRONMENT"))
			}

			first, err := NewKeyVaultFromEnv()
			if !tc.wantVault {
				require.Error(t, err)
				assert.Nil(t, first)
				assert.Contains(t, err.Error(), "KEYVAULT_MASTER_KEY")
				// The refusal names one remedy, the key. A value that
				// starts the server on a generated key is not a remedy.
				assert.NotContains(t, err.Error(), "ENVIRONMENT=development")
				assert.NotContains(t, err.Error(), "ENVIRONMENT=test")
				assert.Contains(t, err.Error(), "KEYVAULT_MASTER_KEY is required")
				assert.Contains(t, err.Error(), "openssl rand -base64 32")
				return
			}
			require.NoError(t, err)
			second, err := NewKeyVaultFromEnv()
			require.NoError(t, err)
			firstKey, err := DeriveSigningKey(first.masterKey, PurposeATCIssuer)
			require.NoError(t, err)
			secondKey, err := DeriveSigningKey(second.masterKey, PurposeATCIssuer)
			require.NoError(t, err)
			assert.NotEqual(t, firstKey.Public(), secondKey.Public(),
				"a generated master key is ephemeral by design")
		})
	}
}

// TestNewKeyVaultFromEnv_ErrorNeverContainsKey keeps a rejected master key
// value out of the startup error, which reaches the server log.
func TestNewKeyVaultFromEnv_ErrorNeverContainsKey(t *testing.T) {
	for _, value := range []string{
		base64.StdEncoding.EncodeToString([]byte("sixteen-byte-key")),
		"not-base64-but-secret-shaped-value",
	} {
		t.Setenv("KEYVAULT_MASTER_KEY", value)
		t.Setenv("ENVIRONMENT", "production")

		kv, err := NewKeyVaultFromEnv()
		require.Error(t, err)
		assert.Nil(t, kv)
		assert.NotContains(t, err.Error(), value)
	}
}

func TestEncryptPrivateKey_Success(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	privateKey := "test-private-key-data-here"
	encrypted, err := kv.EncryptPrivateKey(testAgentID, privateKey)
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

	encrypted1, err := kv.EncryptPrivateKey(testAgentID, privateKey)
	require.NoError(t, err)

	encrypted2, err := kv.EncryptPrivateKey(testAgentID, privateKey)
	require.NoError(t, err)

	// Each encryption should produce different ciphertext (due to random nonce)
	assert.NotEqual(t, encrypted1, encrypted2)
}

func TestDecryptPrivateKey_Success(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalKey := "my-super-secret-private-key"

	encrypted, err := kv.EncryptPrivateKey(testAgentID, originalKey)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(testAgentID, encrypted)
	require.NoError(t, err)
	assert.Equal(t, originalKey, decrypted)
}

func TestDecryptPrivateKey_InvalidBase64(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	_, err = kv.DecryptPrivateKey(testAgentID, "not-valid-base64!!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode ciphertext")
}

func TestDecryptPrivateKey_CiphertextTooShort(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	// A v2 header followed by fewer bytes than a nonce and tag
	shortCiphertext := base64.StdEncoding.EncodeToString([]byte{privateKeyFormatV2, defaultStorageKeyID, 's', 'h', 'o', 'r', 't'})
	_, err = kv.DecryptPrivateKey(testAgentID, shortCiphertext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestDecryptPrivateKey_TamperedCiphertext(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalKey := "my-secret-key"
	encrypted, err := kv.EncryptPrivateKey(testAgentID, originalKey)
	require.NoError(t, err)

	// Decode, tamper, re-encode
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	ciphertext[len(ciphertext)-1] ^= 0xFF // Flip bits in last byte
	tampered := base64.StdEncoding.EncodeToString(ciphertext)

	_, err = kv.DecryptPrivateKey(testAgentID, tampered)
	require.Error(t, err)
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
	encrypted, err := kv1.EncryptPrivateKey(testAgentID, originalKey)
	require.NoError(t, err)

	// Try to decrypt with different master key
	_, err = kv2.DecryptPrivateKey(testAgentID, encrypted)
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
	encrypted, err := kv.EncryptPrivateKey(testAgentID, originalPrivateKey)
	require.NoError(t, err)

	// Rotate to new key
	rotated, err := kv.RotatePrivateKey(testAgentID, encrypted, newMasterKey)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated)
	assert.NotEqual(t, encrypted, rotated)

	// Verify we can decrypt with new key
	newKv, err := NewKeyVault(newMasterKey)
	require.NoError(t, err)

	decrypted, err := newKv.DecryptPrivateKey(testAgentID, rotated)
	require.NoError(t, err)
	assert.Equal(t, originalPrivateKey, decrypted)
}

func TestRotatePrivateKey_InvalidEncryptedKey(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	newKeyBytes := make([]byte, 32)
	newMasterKey := base64.StdEncoding.EncodeToString(newKeyBytes)

	_, err = kv.RotatePrivateKey(testAgentID, "invalid-encrypted-data", newMasterKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt with old key")
}

func TestRotatePrivateKey_InvalidNewMasterKey(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	originalPrivateKey := "my-private-key"
	encrypted, err := kv.EncryptPrivateKey(testAgentID, originalPrivateKey)
	require.NoError(t, err)

	// Try to rotate with invalid new master key
	_, err = kv.RotatePrivateKey(testAgentID, encrypted, "invalid-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create new vault")
}

func TestEncryptDecrypt_RoundTrip_EmptyString(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	original := ""
	encrypted, err := kv.EncryptPrivateKey(testAgentID, original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(testAgentID, encrypted)
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

	encrypted, err := kv.EncryptPrivateKey(testAgentID, original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(testAgentID, encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

func TestEncryptDecrypt_RoundTrip_SpecialCharacters(t *testing.T) {
	masterKey := generateTestMasterKey()
	kv, err := NewKeyVault(masterKey)
	require.NoError(t, err)

	// Test with special characters that might cause issues
	original := "key-with-special-chars: \n\t\r\x00 日本語 emoji: 🔐"

	encrypted, err := kv.EncryptPrivateKey(testAgentID, original)
	require.NoError(t, err)

	decrypted, err := kv.DecryptPrivateKey(testAgentID, encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

// sealLegacyV1 produces a v1 ciphertext (nonce || AES-256-GCM with no
// additional data), the format stored before storage binding. The fixed nonce
// keeps the leading byte away from the v2 version byte.
func sealLegacyV1(t *testing.T, kv *KeyVault, plaintext string) string {
	t.Helper()
	gcm, err := kv.newGCM()
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	for i := range nonce {
		nonce[i] = byte(0x10 + i)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil))
}

func TestStorageAAD_Layout(t *testing.T) {
	agentID := uuid.MustParse("6F1C2A3B-4D5E-4F60-8A7B-9C0D1E2F3A4B")
	header := []byte{privateKeyFormatV2, defaultStorageKeyID}

	got := storageAAD(agentPrivateKeyPurpose, header, agentID)

	want := append([]byte("aim/agent-private-key\x00"), 0x02, 0x01, 0x00)
	want = append(want, "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b"...)
	assert.Equal(t, want, got)
}

func TestEncryptPrivateKey_V2HeaderAndRowBinding(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)
	agentID := uuid.New()

	encrypted, err := kv.EncryptPrivateKey(agentID, "row-bound-key")
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	require.Greater(t, len(raw), privateKeyHeaderSize)
	assert.Equal(t, privateKeyFormatV2, raw[0], "version byte")
	assert.Equal(t, defaultStorageKeyID, raw[1], "storage key id")

	// The ciphertext opens under exactly the documented additional data.
	gcm, err := kv.newGCM()
	require.NoError(t, err)
	nonce := raw[privateKeyHeaderSize : privateKeyHeaderSize+gcm.NonceSize()]
	body := raw[privateKeyHeaderSize+gcm.NonceSize():]
	aad := []byte("aim/agent-private-key\x00\x02\x01\x00" + agentID.String())
	plaintext, err := gcm.Open(nil, nonce, body, aad)
	require.NoError(t, err)
	assert.Equal(t, "row-bound-key", string(plaintext))

	_, err = gcm.Open(nil, nonce, body, nil)
	assert.Error(t, err, "v2 ciphertext must not open without additional data")
}

func TestDecryptPrivateKey_CopiedToAnotherRowIsRefused(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)
	rowA, rowB := uuid.New(), uuid.New()

	encryptedForA, err := kv.EncryptPrivateKey(rowA, "agent-a-private-key")
	require.NoError(t, err)

	got, err := kv.DecryptPrivateKey(rowB, encryptedForA)
	require.Error(t, err)
	assert.Empty(t, got)
	assert.NotContains(t, err.Error(), "agent-a-private-key")

	got, err = kv.DecryptPrivateKey(rowA, encryptedForA)
	require.NoError(t, err)
	assert.Equal(t, "agent-a-private-key", got)
}

func TestDecryptPrivateKey_WrongPurposeIsRefused(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)
	agentID := uuid.New()

	other, err := kv.sealV2("aim/some-other-purpose", agentID, []byte("not-a-private-key"))
	require.NoError(t, err)

	_, err = kv.DecryptPrivateKey(agentID, other)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

func TestDecryptPrivateKey_RefusesV1(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)

	legacy := sealLegacyV1(t, kv, "legacy-private-key")

	got, err := kv.DecryptPrivateKey(testAgentID, legacy)
	assert.ErrorIs(t, err, ErrUnsupportedPrivateKeyFormat)
	assert.Empty(t, got)
}

func TestDecryptPrivateKey_TamperedHeaderIsRefused(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)

	encrypted, err := kv.EncryptPrivateKey(testAgentID, "header-bound-key")
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)

	unknownKeyID := append([]byte(nil), raw...)
	unknownKeyID[1] = 0x7f
	_, err = kv.DecryptPrivateKey(testAgentID, base64.StdEncoding.EncodeToString(unknownKeyID))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown storage key id")

	otherVersion := append([]byte(nil), raw...)
	otherVersion[0] = 0x03
	_, err = kv.DecryptPrivateKey(testAgentID, base64.StdEncoding.EncodeToString(otherVersion))
	assert.ErrorIs(t, err, ErrUnsupportedPrivateKeyFormat)
}

func TestPrivateKey_NilAgentIDIsRefused(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)

	_, err = kv.EncryptPrivateKey(uuid.Nil, "key")
	assert.Error(t, err)

	encrypted, err := kv.EncryptPrivateKey(testAgentID, "key")
	require.NoError(t, err)
	_, err = kv.DecryptPrivateKey(uuid.Nil, encrypted)
	assert.Error(t, err)
}

func TestMigrateLegacyPrivateKey(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)
	rowA, rowB := uuid.New(), uuid.New()

	t.Run("v1 is re-encrypted as v2 bound to its own row", func(t *testing.T) {
		legacy := sealLegacyV1(t, kv, "legacy-key-a")

		migrated, changed, err := kv.MigrateLegacyPrivateKey(rowA, legacy)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.NotEqual(t, legacy, migrated)

		got, err := kv.DecryptPrivateKey(rowA, migrated)
		require.NoError(t, err)
		assert.Equal(t, "legacy-key-a", got)

		_, err = kv.DecryptPrivateKey(rowB, migrated)
		assert.Error(t, err)
	})

	t.Run("v2 for the same row is left unchanged", func(t *testing.T) {
		current, err := kv.EncryptPrivateKey(rowA, "current-key-a")
		require.NoError(t, err)

		out, changed, err := kv.MigrateLegacyPrivateKey(rowA, current)
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, current, out)
	})

	t.Run("v2 copied from another row is not re-bound", func(t *testing.T) {
		copied, err := kv.EncryptPrivateKey(rowA, "current-key-a")
		require.NoError(t, err)

		out, changed, err := kv.MigrateLegacyPrivateKey(rowB, copied)
		assert.Error(t, err)
		assert.False(t, changed)
		assert.Empty(t, out)
	})

	t.Run("unreadable value is an error", func(t *testing.T) {
		_, _, err := kv.MigrateLegacyPrivateKey(rowA, base64.StdEncoding.EncodeToString([]byte("garbage-that-is-not-a-ciphertext")))
		assert.Error(t, err)
	})
}

func TestRotatePrivateKey_KeepsRowBinding(t *testing.T) {
	kv, err := NewKeyVault(generateTestMasterKey())
	require.NoError(t, err)
	newKeyBytes := make([]byte, 32)
	for i := range newKeyBytes {
		newKeyBytes[i] = byte(200 - i)
	}
	newMasterKey := base64.StdEncoding.EncodeToString(newKeyBytes)
	newKv, err := NewKeyVault(newMasterKey)
	require.NoError(t, err)
	rowA, rowB := uuid.New(), uuid.New()

	encrypted, err := kv.EncryptPrivateKey(rowA, "rotating-key")
	require.NoError(t, err)

	rotated, err := kv.RotatePrivateKey(rowA, encrypted, newMasterKey)
	require.NoError(t, err)

	got, err := newKv.DecryptPrivateKey(rowA, rotated)
	require.NoError(t, err)
	assert.Equal(t, "rotating-key", got)

	_, err = newKv.DecryptPrivateKey(rowB, rotated)
	assert.Error(t, err)

	_, err = kv.RotatePrivateKey(rowB, encrypted, newMasterKey)
	assert.Error(t, err, "rotation must not re-bind a ciphertext to a different row")
}

// The server's master key line reads the vault's source and storage-key id:
// environment for a key in KEYVAULT_MASTER_KEY, ephemeral for the key generated
// in development, and storage-key id 1 when KEYVAULT_MASTER_KEY_ID is unset.
func TestNewKeyVaultFromEnv_MasterKeySourceAndStorageKeyID(t *testing.T) {
	t.Run("supplied in KEYVAULT_MASTER_KEY", func(t *testing.T) {
		t.Setenv("KEYVAULT_MASTER_KEY", generateTestMasterKey())
		t.Setenv("KEYVAULT_MASTER_KEY_ID", "")
		t.Setenv("ENVIRONMENT", "production")
		kv, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		assert.Equal(t, MasterKeySourceEnvironment, kv.MasterKeySource())
		assert.Equal(t, "environment", kv.MasterKeySource())
		assert.Equal(t, byte(1), kv.StorageKeyID())
	})

	t.Run("generated for development", func(t *testing.T) {
		t.Setenv("KEYVAULT_MASTER_KEY", "")
		t.Setenv("KEYVAULT_MASTER_KEY_ID", "")
		t.Setenv("ENVIRONMENT", "development")
		kv, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		assert.Equal(t, MasterKeySourceEphemeral, kv.MasterKeySource())
		assert.Equal(t, "ephemeral", kv.MasterKeySource())
		assert.Equal(t, byte(1), kv.StorageKeyID())
	})

	t.Run("passed to NewKeyVault", func(t *testing.T) {
		kv, err := NewKeyVault(generateTestMasterKey())
		require.NoError(t, err)
		assert.Empty(t, kv.MasterKeySource())
	})
}

// The server prints the master key line, not the constructor, so no other
// caller of NewKeyVaultFromEnv prints the identity. On every path, accepted or
// refused, the constructor prints no master key line, no identity and no
// rendering of the key; a refused key gives an error and no vault.
func TestNewKeyVaultFromEnv_PrintsNoMasterKeyLine(t *testing.T) {
	supplied := fixedTestKey("constructor output supplied key", 32)
	allZero := make([]byte, 32)
	malformed := []byte(base64.StdEncoding.EncodeToString(fixedTestKey("constructor output malformed key", 32)))
	malformed[20] = '*'

	cases := []struct {
		name        string
		value       string
		environment string
		renderings  []string // nil for the generated key, read from the vault
		wantSource  string
		wantErr     string
	}{
		{
			name:        "supplied key",
			value:       base64.StdEncoding.EncodeToString(supplied),
			environment: "production",
			renderings:  keyRenderings(supplied),
			wantSource:  MasterKeySourceEnvironment,
		},
		{
			name:        "development branch",
			environment: "development",
			wantSource:  MasterKeySourceEphemeral,
		},
		{
			name:        "empty outside development and test",
			environment: "production",
			wantErr:     "KEYVAULT_MASTER_KEY is required",
		},
		{
			name:        "malformed",
			value:       string(malformed),
			environment: "production",
			renderings:  []string{string(malformed)},
			wantErr:     "failed to decode master key",
		},
		{
			name:        "all zero",
			value:       base64.StdEncoding.EncodeToString(allZero),
			environment: "production",
			renderings:  keyRenderings(allZero),
			wantErr:     "master key must not be all zero bytes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KEYVAULT_MASTER_KEY", tc.value)
			t.Setenv("KEYVAULT_MASTER_KEY_ID", "")
			t.Setenv("ENVIRONMENT", tc.environment)
			var kv *KeyVault
			var err error
			out := captureKeyVaultOutput(t, func() { kv, err = NewKeyVaultFromEnv() })
			assert.NotContains(t, out, "KeyVault: master key")
			assert.NotContains(t, out, "masterKeyIdentity")

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Nil(t, kv)
				assert.Contains(t, err.Error(), tc.wantErr)
				if tc.renderings != nil {
					assertNoKeyFragment(t, err.Error()+"\n"+out, tc.renderings)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, kv)
			assert.Equal(t, tc.wantSource, kv.MasterKeySource())
			renderings := tc.renderings
			if renderings == nil {
				// The development branch prints its warning, so the capture
				// saw the constructor's output.
				assert.Contains(t, out, "SECURITY WARNING: KEYVAULT_MASTER_KEY not set")
				renderings = keyRenderings(kv.masterKey)
			}
			assertNoKeyFragment(t, out, renderings)
			assert.NotContains(t, out, kv.MasterKeyIdentity())
		})
	}
}

// identityKnownAnswer is the identity of the master key whose bytes are 0x00
// to 0x1f.
const identityKnownAnswer = "3ad1f204acabb927f83759279994a5cf"

// countingMasterKey returns the 32 bytes 0x00 to 0x1f.
func countingMasterKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestMasterKeyIdentity_KnownAnswer(t *testing.T) {
	key := countingMasterKey()
	assert.Equal(t, identityKnownAnswer, masterKeyIdentity(key))

	kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	assert.Equal(t, identityKnownAnswer, kv.MasterKeyIdentity())
}

// Each construction below is one a mistaken implementation could use instead
// of HMAC-SHA256 keyed with the decoded key over the label, truncated to 16
// bytes. None gives the known answer, so TestMasterKeyIdentity_KnownAnswer
// fails for each of them.
func TestMasterKeyIdentity_KnownAnswerRefusesOtherConstructions(t *testing.T) {
	key := countingMasterKey()
	label := []byte(identityLabel)
	keyBase64 := base64.StdEncoding.EncodeToString(key)
	macOf := func(macKey, message []byte) []byte {
		mac := hmac.New(sha256.New, macKey)
		mac.Write(message)
		return mac.Sum(nil)
	}

	// The construction itself gives the known answer, so the comparisons
	// below are against the right value.
	require.Equal(t, identityKnownAnswer, hex.EncodeToString(macOf(key, label)[:16]))

	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	zeroBlock := make([]byte, aes.BlockSize)
	block.Encrypt(zeroBlock, zeroBlock)
	labelThenKey := sha256.Sum256(append(append([]byte{}, label...), key...))
	keyString := sha256.Sum256([]byte(keyBase64))

	others := []struct {
		name  string
		value string
	}{
		{"AES encryption of a zero block under the master key", hex.EncodeToString(zeroBlock)},
		{"the untruncated 32-byte output", hex.EncodeToString(macOf(key, label))},
		{"SHA-256 of the label followed by the key", hex.EncodeToString(labelThenKey[:16])},
		{"HMAC with key and label swapped", hex.EncodeToString(macOf(label, key)[:16])},
		{"HMAC keyed with the base64 string", hex.EncodeToString(macOf([]byte(keyBase64), label)[:16])},
		{"SHA-256 of the key string", hex.EncodeToString(keyString[:16])},
	}
	for _, other := range others {
		assert.NotEqual(t, identityKnownAnswer, other.value, other.name)
	}
}

var identityForm = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestMasterKeyIdentity_Behaviour(t *testing.T) {
	mint := func(t *testing.T) string {
		t.Helper()
		key := make([]byte, 32)
		_, err := rand.Read(key)
		require.NoError(t, err)
		return base64.StdEncoding.EncodeToString(key)
	}

	t.Run("two keys minted at run time give different identities", func(t *testing.T) {
		first, err := NewKeyVault(mint(t))
		require.NoError(t, err)
		second, err := NewKeyVault(mint(t))
		require.NoError(t, err)
		assert.Regexp(t, identityForm, first.MasterKeyIdentity())
		assert.Regexp(t, identityForm, second.MasterKeyIdentity())
		assert.NotEqual(t, first.MasterKeyIdentity(), second.MasterKeyIdentity())
	})

	t.Run("one key gives one identity across two constructions", func(t *testing.T) {
		key := mint(t)
		t.Setenv("KEYVAULT_MASTER_KEY", key)
		t.Setenv("KEYVAULT_MASTER_KEY_ID", "")
		t.Setenv("ENVIRONMENT", "production")
		first, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		second, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		direct, err := NewKeyVault(key)
		require.NoError(t, err)
		assert.Regexp(t, identityForm, first.MasterKeyIdentity())
		assert.Equal(t, first.MasterKeyIdentity(), second.MasterKeyIdentity())
		assert.Equal(t, first.MasterKeyIdentity(), direct.MasterKeyIdentity())
	})

	t.Run("two development constructions give different identities", func(t *testing.T) {
		t.Setenv("KEYVAULT_MASTER_KEY", "")
		t.Setenv("KEYVAULT_MASTER_KEY_ID", "")
		t.Setenv("ENVIRONMENT", "development")
		first, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		second, err := NewKeyVaultFromEnv()
		require.NoError(t, err)
		assert.Regexp(t, identityForm, first.MasterKeyIdentity())
		assert.Regexp(t, identityForm, second.MasterKeyIdentity())
		assert.NotEqual(t, first.MasterKeyIdentity(), second.MasterKeyIdentity())
	})

	t.Run("a ring vault names its current key", func(t *testing.T) {
		older, current := mint(t), mint(t)
		ring, err := NewKeyVaultRing(2, map[byte]string{1: older, 2: current})
		require.NoError(t, err)
		currentOnly, err := NewKeyVault(current)
		require.NoError(t, err)
		olderOnly, err := NewKeyVault(older)
		require.NoError(t, err)
		assert.Equal(t, currentOnly.MasterKeyIdentity(), ring.MasterKeyIdentity())
		assert.NotEqual(t, olderOnly.MasterKeyIdentity(), ring.MasterKeyIdentity())
	})
}

// The identity is not a signing key: the label is not a signing purpose, and
// the purposes the server derives and publishes keys for are the two it had.
func TestMasterKeyIdentity_LabelIsNotASigningPurpose(t *testing.T) {
	_, err := DeriveSigningKey(countingMasterKey(), SigningPurpose(identityLabel))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no signing key is derived for purpose")

	assert.Equal(t, []SigningPurpose{PurposeCardAttestation, PurposeATCIssuer}, signingPurposes)
	assert.Equal(t, []SigningPurpose{PurposeCardAttestation, PurposeATCIssuer}, publishedSigningPurposes)
}
