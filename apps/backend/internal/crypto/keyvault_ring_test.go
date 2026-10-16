package crypto

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ringTestKey returns a fixed base64 master key per label.
func ringTestKey(label string) string {
	return base64.StdEncoding.EncodeToString(fixedTestKey("ring test key "+label, 32))
}

func storageKeyIDOf(t *testing.T, encrypted string) byte {
	t.Helper()
	id, err := PrivateKeyStorageKeyID(encrypted)
	require.NoError(t, err)
	return id
}

func TestKeyVaultRing_DecryptsUnderEveryIDAndEncryptsUnderTheCurrentOne(t *testing.T) {
	oldKey, newKey := ringTestKey("one"), ringTestKey("two")

	before, err := NewKeyVault(oldKey)
	require.NoError(t, err)
	stored, err := before.EncryptPrivateKey(testAgentID, "stored-before-rotation")
	require.NoError(t, err)
	assert.Equal(t, byte(1), storageKeyIDOf(t, stored))

	ring, err := NewKeyVaultRing(2, map[byte]string{1: oldKey, 2: newKey})
	require.NoError(t, err)
	assert.Equal(t, byte(2), ring.StorageKeyID())
	assert.Equal(t, []byte{1, 2}, ring.StorageKeyIDs())

	got, err := ring.DecryptPrivateKey(testAgentID, stored)
	require.NoError(t, err, "a ciphertext under an older id in the ring decrypts")
	assert.Equal(t, "stored-before-rotation", got)

	fresh, err := ring.EncryptPrivateKey(testAgentID, "stored-after-rotation")
	require.NoError(t, err)
	assert.Equal(t, byte(2), storageKeyIDOf(t, fresh), "a new ciphertext carries the current id")
	got, err = ring.DecryptPrivateKey(testAgentID, fresh)
	require.NoError(t, err)
	assert.Equal(t, "stored-after-rotation", got)

	// The current key alone decrypts the new ciphertext, and only that key
	// encrypted it: a ring that holds key two under id 2 opens it.
	current, err := NewKeyVaultRing(2, map[byte]string{2: newKey})
	require.NoError(t, err)
	got, err = current.DecryptPrivateKey(testAgentID, fresh)
	require.NoError(t, err)
	assert.Equal(t, "stored-after-rotation", got)

	// An id outside the ring is refused: the vault from before the rotation
	// does not hold id 2, and the current key alone does not hold id 1.
	_, err = before.DecryptPrivateKey(testAgentID, fresh)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown storage key id 2")
	_, err = current.DecryptPrivateKey(testAgentID, stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown storage key id 1")
}

func TestKeyVaultRing_IDNamesTheKey(t *testing.T) {
	ring, err := NewKeyVaultRing(2, map[byte]string{1: ringTestKey("one"), 2: ringTestKey("two")})
	require.NoError(t, err)
	encrypted, err := ring.EncryptPrivateKey(testAgentID, "id-bound-key")
	require.NoError(t, err)

	// Relabelling a ciphertext with another id in the ring selects the other
	// key, and the header is part of the additional data: it does not open.
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	raw[1] = 1
	_, err = ring.DecryptPrivateKey(testAgentID, base64.StdEncoding.EncodeToString(raw))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")

	// The same key under another id does not open it either: the id is
	// authenticated, not only used to pick the key.
	swapped, err := NewKeyVaultRing(3, map[byte]string{3: ringTestKey("two")})
	require.NoError(t, err)
	raw[1] = 3
	_, err = swapped.DecryptPrivateKey(testAgentID, base64.StdEncoding.EncodeToString(raw))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

func TestNewKeyVaultRing_RefusesAnInvalidRing(t *testing.T) {
	one, two := ringTestKey("one"), ringTestKey("two")
	cases := []struct {
		name    string
		current byte
		keys    map[byte]string
		wantErr string
	}{
		{"current id 0", 0, map[byte]string{0: one}, "storage key id must be from 1 to 255"},
		{"id 0 beside the current id", 2, map[byte]string{0: one, 2: two}, "storage key id must be from 1 to 255"},
		{"no key for the current id", 3, map[byte]string{1: one, 2: two}, "no master key is set for the current storage key id 3"},
		{"one key under two ids", 2, map[byte]string{1: one, 2: one}, "storage key ids 1 and 2 hold the same master key"},
		{"a malformed older key names its id", 2, map[byte]string{1: "not-base64!", 2: two}, "storage key id 1: failed to decode master key"},
		{"an all-zero older key", 2, map[byte]string{1: base64.StdEncoding.EncodeToString(make([]byte, 32)), 2: two}, "storage key id 1: master key must not be all zero bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kv, err := NewKeyVaultRing(tc.current, tc.keys)
			require.Error(t, err)
			assert.Nil(t, kv)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRewrapPrivateKey_MovesAnOlderIDToTheCurrentOne(t *testing.T) {
	oldKey, newKey := ringTestKey("one"), ringTestKey("two")
	before, err := NewKeyVault(oldKey)
	require.NoError(t, err)
	ring, err := NewKeyVaultRing(2, map[byte]string{1: oldKey, 2: newKey})
	require.NoError(t, err)
	rowA, rowB := uuid.New(), uuid.New()

	stored, err := before.EncryptPrivateKey(rowA, "rewrapped-key")
	require.NoError(t, err)

	rewrapped, changed, err := ring.RewrapPrivateKey(rowA, stored)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, byte(2), storageKeyIDOf(t, rewrapped))
	got, err := ring.DecryptPrivateKey(rowA, rewrapped)
	require.NoError(t, err)
	assert.Equal(t, "rewrapped-key", got)

	// The rewrapped ciphertext stays bound to its row.
	_, err = ring.DecryptPrivateKey(rowB, rewrapped)
	assert.Error(t, err)
	_, _, err = ring.RewrapPrivateKey(rowB, stored)
	assert.Error(t, err, "a rewrap must not re-bind a ciphertext to a different row")

	// A value already under the current id comes back unchanged.
	again, changed, err := ring.RewrapPrivateKey(rowA, rewrapped)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, rewrapped, again)

	// An id outside the ring and a v1 value are refused.
	current, err := NewKeyVaultRing(3, map[byte]string{3: ringTestKey("three")})
	require.NoError(t, err)
	_, _, err = current.RewrapPrivateKey(rowA, stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown storage key id 1")

	_, _, err = ring.RewrapPrivateKey(rowA, sealLegacyV1(t, before, "legacy-key"))
	assert.ErrorIs(t, err, ErrUnsupportedPrivateKeyFormat)
}

func TestPrivateKeyStorageKeyID(t *testing.T) {
	ring, err := NewKeyVaultRing(7, map[byte]string{7: ringTestKey("seven")})
	require.NoError(t, err)
	encrypted, err := ring.EncryptPrivateKey(testAgentID, "header-read")
	require.NoError(t, err)

	id, err := PrivateKeyStorageKeyID(encrypted)
	require.NoError(t, err)
	assert.Equal(t, byte(7), id)

	_, err = PrivateKeyStorageKeyID(sealLegacyV1(t, ring, "legacy-key"))
	assert.ErrorIs(t, err, ErrUnsupportedPrivateKeyFormat)
	_, err = PrivateKeyStorageKeyID("not base64!")
	assert.Error(t, err)
	_, err = PrivateKeyStorageKeyID(base64.StdEncoding.EncodeToString([]byte{privateKeyFormatV2}))
	assert.Error(t, err)
}

func TestMigrateLegacyPrivateKey_ReadsV1UnderAnOlderKeyInTheRing(t *testing.T) {
	oldKey := ringTestKey("one")
	before, err := NewKeyVault(oldKey)
	require.NoError(t, err)
	legacy := sealLegacyV1(t, before, "legacy-private-key")

	ring, err := NewKeyVaultRing(2, map[byte]string{1: oldKey, 2: ringTestKey("two")})
	require.NoError(t, err)

	migrated, changed, err := ring.MigrateLegacyPrivateKey(testAgentID, legacy)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, byte(2), storageKeyIDOf(t, migrated))
	got, err := ring.DecryptPrivateKey(testAgentID, migrated)
	require.NoError(t, err)
	assert.Equal(t, "legacy-private-key", got)

	// A key outside the ring does not read it.
	other, err := NewKeyVaultRing(2, map[byte]string{2: ringTestKey("two")})
	require.NoError(t, err)
	_, _, err = other.MigrateLegacyPrivateKey(testAgentID, legacy)
	assert.Error(t, err)
}

func TestDerivedSigningKeysComeFromTheCurrentMasterKey(t *testing.T) {
	ring, err := NewKeyVaultRing(2, map[byte]string{1: ringTestKey("one"), 2: ringTestKey("two")})
	require.NoError(t, err)
	currentOnly, err := NewKeyVault(ringTestKey("two"))
	require.NoError(t, err)
	olderOnly, err := NewKeyVault(ringTestKey("one"))
	require.NoError(t, err)

	fromRing, err := LoadSigningKeyRing(ring, envFrom(nil))
	require.NoError(t, err)
	fromCurrent, err := LoadSigningKeyRing(currentOnly, envFrom(nil))
	require.NoError(t, err)
	fromOlder, err := LoadSigningKeyRing(olderOnly, envFrom(nil))
	require.NoError(t, err)

	for _, purpose := range signingPurposes {
		assert.Equal(t, fromCurrent.Key(purpose).KeyID, fromRing.Key(purpose).KeyID, "%s is derived from the current key", purpose)
		assert.NotEqual(t, fromOlder.Key(purpose).KeyID, fromRing.Key(purpose).KeyID, "%s moves when the current key changes", purpose)
	}
}

func TestNewKeyVaultFromEnv_ReadsTheRing(t *testing.T) {
	one, two, three := ringTestKey("one"), ringTestKey("two"), ringTestKey("three")

	t.Run("one key, no id: storage key id 1", func(t *testing.T) {
		kv, err := newKeyVaultFromEnv(envFrom(map[string]string{"KEYVAULT_MASTER_KEY": one, "ENVIRONMENT": "production"}))
		require.NoError(t, err)
		assert.Equal(t, byte(1), kv.StorageKeyID())
		assert.Equal(t, []byte{1}, kv.StorageKeyIDs())
	})

	t.Run("current key with its id and an older key", func(t *testing.T) {
		kv, err := newKeyVaultFromEnv(envFrom(map[string]string{
			"KEYVAULT_MASTER_KEY":    two,
			"KEYVAULT_MASTER_KEY_ID": "2",
			"KEYVAULT_MASTER_KEY_1":  one,
			"ENVIRONMENT":            "production",
		}))
		require.NoError(t, err)
		assert.Equal(t, byte(2), kv.StorageKeyID())
		assert.Equal(t, []byte{1, 2}, kv.StorageKeyIDs())

		before, err := NewKeyVault(one)
		require.NoError(t, err)
		stored, err := before.EncryptPrivateKey(testAgentID, "env-ring-key")
		require.NoError(t, err)
		got, err := kv.DecryptPrivateKey(testAgentID, stored)
		require.NoError(t, err)
		assert.Equal(t, "env-ring-key", got)
	})

	t.Run("the highest id 255 and a staged key above the current one", func(t *testing.T) {
		kv, err := newKeyVaultFromEnv(envFrom(map[string]string{
			"KEYVAULT_MASTER_KEY":     two,
			"KEYVAULT_MASTER_KEY_ID":  " 2 ",
			"KEYVAULT_MASTER_KEY_255": three,
		}))
		require.NoError(t, err)
		assert.Equal(t, byte(2), kv.StorageKeyID())
		assert.Equal(t, []byte{2, 255}, kv.StorageKeyIDs())
	})

	refusals := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"id 0", map[string]string{"KEYVAULT_MASTER_KEY": one, "KEYVAULT_MASTER_KEY_ID": "0"}, `KEYVAULT_MASTER_KEY_ID must be a storage key id from 1 to 255, got "0"`},
		{"id 256", map[string]string{"KEYVAULT_MASTER_KEY": one, "KEYVAULT_MASTER_KEY_ID": "256"}, `KEYVAULT_MASTER_KEY_ID must be a storage key id from 1 to 255, got "256"`},
		{"id not a number", map[string]string{"KEYVAULT_MASTER_KEY": one, "KEYVAULT_MASTER_KEY_ID": "two"}, `KEYVAULT_MASTER_KEY_ID must be a storage key id from 1 to 255, got "two"`},
		{"older variable at the current id", map[string]string{"KEYVAULT_MASTER_KEY": two, "KEYVAULT_MASTER_KEY_ID": "2", "KEYVAULT_MASTER_KEY_2": one}, "KEYVAULT_MASTER_KEY_2 is set, but 2 is the storage key id of KEYVAULT_MASTER_KEY"},
		{"older variable at the default id", map[string]string{"KEYVAULT_MASTER_KEY": two, "KEYVAULT_MASTER_KEY_1": one}, "KEYVAULT_MASTER_KEY_1 is set, but 1 is the storage key id of KEYVAULT_MASTER_KEY"},
		{"older key without a current key, in development", map[string]string{"KEYVAULT_MASTER_KEY_1": one, "ENVIRONMENT": "development"}, "KEYVAULT_MASTER_KEY is required when KEYVAULT_MASTER_KEY_1 holds an older master key"},
		{"malformed older key names its variable", map[string]string{"KEYVAULT_MASTER_KEY": two, "KEYVAULT_MASTER_KEY_ID": "2", "KEYVAULT_MASTER_KEY_1": "not-base64!"}, "KEYVAULT_MASTER_KEY_1: failed to decode master key"},
		{"one key under two ids", map[string]string{"KEYVAULT_MASTER_KEY": two, "KEYVAULT_MASTER_KEY_ID": "2", "KEYVAULT_MASTER_KEY_1": two}, "storage key ids 1 and 2 hold the same master key"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			kv, err := newKeyVaultFromEnv(envFrom(tc.env))
			require.Error(t, err)
			assert.Nil(t, kv)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRingMasterKeyEnvVar(t *testing.T) {
	assert.Equal(t, "KEYVAULT_MASTER_KEY_1", RingMasterKeyEnvVar(1))
	assert.Equal(t, "KEYVAULT_MASTER_KEY_255", RingMasterKeyEnvVar(255))
}
