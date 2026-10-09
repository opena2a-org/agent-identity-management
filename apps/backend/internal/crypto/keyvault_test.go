package crypto

import (
	"encoding/base64"
	"os"
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
				assert.Contains(t, err.Error(), "ENVIRONMENT=development")
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
	shortCiphertext := base64.StdEncoding.EncodeToString([]byte{privateKeyFormatV2, currentStorageKeyID, 's', 'h', 'o', 'r', 't'})
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
	header := []byte{privateKeyFormatV2, currentStorageKeyID}

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
	assert.Equal(t, currentStorageKeyID, raw[1], "storage key id")

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
