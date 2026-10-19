package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sealFormatV2 writes format v2, the format the vault wrote before data keys:
// the value sealed under the current master key itself. Its body is the
// vault's v2 writer of that time. The vault decrypts v2 and writes v3, so the
// tests use this to produce values in format v2.
func (kv *KeyVault) sealFormatV2(purpose string, agentID uuid.UUID, plaintext []byte) (string, error) {
	if agentID == uuid.Nil {
		return "", fmt.Errorf("agent ID is required to encrypt a private key")
	}

	gcm, err := kv.newGCM()
	if err != nil {
		return "", err
	}

	header := []byte{privateKeyFormatV2, kv.currentID}

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

// Format v3 as keyvault.go documents it, read here from its parts:
//
//	header (0x03, storage-key id)
//	|| data key nonce (12) || the 32-byte data key sealed under the master key, and its tag (48)
//	|| nonce (12) || the value sealed under the data key, and its tag
//
// The offsets and the additional data are written out here rather than taken
// from keyvault.go, so a change to the stored layout is a change to this file.
const (
	v3Version         = 0x03
	v3DataKeyNonceAt  = 2
	v3SealedDataKeyAt = 14
	v3NonceAt         = 62
	v3CiphertextAt    = 74
	v3DataKeySize     = 32
	v3TagSize         = 16
)

// v3DataKeyAAD is the additional data the data key is sealed with under the
// master key: the purpose label with "/data-key" appended, the header and the
// agent row.
func v3DataKeyAAD(purpose string, header []byte, agentID uuid.UUID) []byte {
	return []byte(purpose + "/data-key\x00" + string(header) + "\x00" + agentID.String())
}

// v3ValueAAD is the additional data the value is sealed with under its data
// key: the purpose label, the version byte and the agent row. It leaves out
// the storage-key id, so moving the data key to another master key leaves the
// sealed value as it is.
func v3ValueAAD(purpose string, agentID uuid.UUID) []byte {
	return []byte(purpose + "\x00\x03\x00" + agentID.String())
}

func testAEAD(t *testing.T, key []byte) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return gcm
}

// openV3Parts reads a stored value by the documented layout: it opens the data
// key under masterKey, then the value under the data key.
func openV3Parts(t *testing.T, masterKey []byte, agentID uuid.UUID, stored string) (raw, dataKey, plaintext []byte) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(stored)
	require.NoError(t, err)
	require.Greater(t, len(raw), v3CiphertextAt, "a v3 value holds a header, a sealed data key, a nonce and a ciphertext")
	require.Equal(t, byte(v3Version), raw[0], "version byte")

	dataKey, err = testAEAD(t, masterKey).Open(nil,
		raw[v3DataKeyNonceAt:v3SealedDataKeyAt], raw[v3SealedDataKeyAt:v3NonceAt],
		v3DataKeyAAD(agentPrivateKeyPurpose, raw[:v3DataKeyNonceAt], agentID))
	require.NoError(t, err, "the data key opens under the master key")
	require.Len(t, dataKey, v3DataKeySize)

	plaintext, err = testAEAD(t, dataKey).Open(nil,
		raw[v3NonceAt:v3CiphertextAt], raw[v3CiphertextAt:], v3ValueAAD(agentPrivateKeyPurpose, agentID))
	require.NoError(t, err, "the value opens under its data key")
	return raw, dataKey, plaintext
}

// sealV3ByLayout builds a v3 value from its documented parts, with the data
// key sealed for keyPurpose and keyRow and the value for valuePurpose and
// valueRow.
func sealV3ByLayout(t *testing.T, masterKey []byte, storageKeyID byte, keyPurpose string, keyRow uuid.UUID, valuePurpose string, valueRow uuid.UUID, plaintext string) string {
	t.Helper()
	dataKey := fixedTestKey("hand-built data key", v3DataKeySize)
	header := []byte{v3Version, storageKeyID}
	dataKeyNonce := bytes.Repeat([]byte{0x31}, v3SealedDataKeyAt-v3DataKeyNonceAt)
	nonce := bytes.Repeat([]byte{0x32}, v3CiphertextAt-v3NonceAt)

	out := append([]byte(nil), header...)
	out = append(out, dataKeyNonce...)
	out = testAEAD(t, masterKey).Seal(out, dataKeyNonce, dataKey, v3DataKeyAAD(keyPurpose, header, keyRow))
	out = append(out, nonce...)
	out = testAEAD(t, dataKey).Seal(out, nonce, []byte(plaintext), v3ValueAAD(valuePurpose, valueRow))
	return base64.StdEncoding.EncodeToString(out)
}

// Each value the vault seals gets a data key of its own. The data key is
// stored only sealed under the master key, the key-encryption key, and the
// value opens under its data key and not under the master key. A value in
// format v2, sealed under the master key alone as the vault sealed values
// before data keys, still opens.
func TestEncryptPrivateKey_SealsEachValueUnderItsOwnDataKey(t *testing.T) {
	masterKey := fixedTestKey("data key test master key", 32)
	kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)

	rowA, rowB := uuid.New(), uuid.New()
	values := []struct {
		name      string
		row       uuid.UUID
		plaintext string
	}{
		{"agent a", rowA, "agent-a-private-key"},
		{"agent b", rowB, "agent-b-private-key"},
		{"agent a sealed again", rowA, "agent-a-private-key"},
	}

	dataKeys := make([][]byte, 0, len(values))
	for _, v := range values {
		stored, err := kv.EncryptPrivateKey(v.row, v.plaintext)
		require.NoError(t, err, v.name)

		raw, dataKey, plaintext := openV3Parts(t, masterKey, v.row, stored)
		assert.Equal(t, byte(1), raw[1], "%s: storage key id", v.name)
		assert.Equal(t, v.plaintext, string(plaintext), v.name)
		assert.Len(t, raw, v3CiphertextAt+len(v.plaintext)+v3TagSize,
			"%s: the header, the sealed data key, the nonce and the sealed value, and nothing else", v.name)
		assert.False(t, bytes.Contains(raw, dataKey), "%s: the data key is stored only sealed", v.name)

		_, err = testAEAD(t, masterKey).Open(nil,
			raw[v3NonceAt:v3CiphertextAt], raw[v3CiphertextAt:], v3ValueAAD(agentPrivateKeyPurpose, v.row))
		assert.Error(t, err, "%s: the value is not sealed under the master key", v.name)
		_, err = testAEAD(t, masterKey).Open(nil,
			raw[v3DataKeyNonceAt:v3SealedDataKeyAt], raw[v3SealedDataKeyAt:v3NonceAt], nil)
		assert.Error(t, err, "%s: the data key does not open without its additional data", v.name)
		_, err = testAEAD(t, dataKey).Open(nil, raw[v3NonceAt:v3CiphertextAt], raw[v3CiphertextAt:], nil)
		assert.Error(t, err, "%s: the value does not open without its additional data", v.name)

		got, err := kv.DecryptPrivateKey(v.row, stored)
		require.NoError(t, err, v.name)
		assert.Equal(t, v.plaintext, got, v.name)

		for i, earlier := range dataKeys {
			assert.NotEqual(t, earlier, dataKey, "%s has the data key of %s", v.name, values[i].name)
		}
		dataKeys = append(dataKeys, dataKey)
	}

	before, err := kv.sealFormatV2(agentPrivateKeyPurpose, rowA, []byte("stored-before-data-keys"))
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(before)
	require.NoError(t, err)
	require.Equal(t, privateKeyFormatV2, raw[0], "sealFormatV2 writes format v2")
	got, err := kv.DecryptPrivateKey(rowA, before)
	require.NoError(t, err, "a value in format v2 still opens")
	assert.Equal(t, "stored-before-data-keys", got)

	_, err = kv.DecryptPrivateKey(rowB, before)
	assert.Error(t, err, "a value in format v2 opens only for its own row")
}

// A value built from the documented parts opens. It is refused when the data
// key or the value is sealed for another purpose or row, when the data key is
// sealed under another master key, and when the header names another storage
// key id or format.
func TestDecryptPrivateKey_OpensTheDocumentedDataKeyLayout(t *testing.T) {
	masterKey := fixedTestKey("data key layout master key", 32)
	otherKey := fixedTestKey("data key layout other master key", 32)
	kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)
	ring, err := NewKeyVaultRing(2, map[byte]string{
		1: base64.StdEncoding.EncodeToString(masterKey),
		2: base64.StdEncoding.EncodeToString(otherKey),
	})
	require.NoError(t, err)
	row, otherRow := uuid.New(), uuid.New()
	const purpose, otherPurpose = agentPrivateKeyPurpose, "aim/some-other-purpose"

	built := sealV3ByLayout(t, masterKey, 1, purpose, row, purpose, row, "layout-private-key")
	got, err := kv.DecryptPrivateKey(row, built)
	require.NoError(t, err, "a value built from the documented layout opens")
	assert.Equal(t, "layout-private-key", got)
	got, err = ring.DecryptPrivateKey(row, built)
	require.NoError(t, err, "a ring that holds the key under id 1 opens it")
	assert.Equal(t, "layout-private-key", got)

	refused := map[string]string{
		"data key sealed for another purpose": sealV3ByLayout(t, masterKey, 1, otherPurpose, row, purpose, row, "layout-private-key"),
		"value sealed for another purpose":    sealV3ByLayout(t, masterKey, 1, purpose, row, otherPurpose, row, "layout-private-key"),
		"data key sealed for another row":     sealV3ByLayout(t, masterKey, 1, purpose, otherRow, purpose, row, "layout-private-key"),
		"value sealed for another row":        sealV3ByLayout(t, masterKey, 1, purpose, row, purpose, otherRow, "layout-private-key"),
		"data key sealed under another key":   sealV3ByLayout(t, otherKey, 1, purpose, row, purpose, row, "layout-private-key"),
	}
	for name, stored := range refused {
		got, err := kv.DecryptPrivateKey(row, stored)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "failed to decrypt", name)
		assert.Empty(t, got, name)
	}

	// The storage key id is bound by the data key's additional data: naming
	// id 2 sends the data key to the other master key, and the value is
	// refused rather than read under the wrong key.
	raw, err := base64.StdEncoding.DecodeString(built)
	require.NoError(t, err)
	renamed := append([]byte(nil), raw...)
	renamed[1] = 2
	_, err = ring.DecryptPrivateKey(row, base64.StdEncoding.EncodeToString(renamed))
	assert.Error(t, err)
	_, err = kv.DecryptPrivateKey(row, base64.StdEncoding.EncodeToString(renamed))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown storage key id 2")

	// Read as format v2, the same bytes do not open.
	asV2 := append([]byte(nil), raw...)
	asV2[0] = privateKeyFormatV2
	_, err = kv.DecryptPrivateKey(row, base64.StdEncoding.EncodeToString(asV2))
	assert.Error(t, err)

	short := base64.StdEncoding.EncodeToString(append([]byte{v3Version, 1}, raw[v3DataKeyNonceAt:v3CiphertextAt+v3TagSize-1]...))
	_, err = kv.DecryptPrivateKey(row, short)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

// Rewrapping a value under an older storage-key id seals its data key under the
// current master key and leaves the sealed value as it is. A value in format
// v2 under an older id is sealed again, under a data key of its own.
func TestRewrapPrivateKey_MovesTheDataKeyToTheCurrentMasterKey(t *testing.T) {
	oldKey, newKey := fixedTestKey("data key rewrap old", 32), fixedTestKey("data key rewrap new", 32)
	before, err := NewKeyVault(base64.StdEncoding.EncodeToString(oldKey))
	require.NoError(t, err)
	ring, err := NewKeyVaultRing(2, map[byte]string{
		1: base64.StdEncoding.EncodeToString(oldKey),
		2: base64.StdEncoding.EncodeToString(newKey),
	})
	require.NoError(t, err)
	row := uuid.New()

	stored, err := before.EncryptPrivateKey(row, "rewrapped-private-key")
	require.NoError(t, err)
	storedRaw, dataKey, _ := openV3Parts(t, oldKey, row, stored)

	rewrapped, changed, err := ring.RewrapPrivateKey(row, stored)
	require.NoError(t, err)
	assert.True(t, changed)
	raw, movedKey, plaintext := openV3Parts(t, newKey, row, rewrapped)
	assert.Equal(t, byte(2), raw[1], "storage key id")
	assert.Equal(t, dataKey, movedKey, "the same data key, now sealed under the current master key")
	assert.Equal(t, storedRaw[v3NonceAt:], raw[v3NonceAt:], "the sealed value is unchanged")
	assert.Equal(t, "rewrapped-private-key", string(plaintext))

	_, err = testAEAD(t, oldKey).Open(nil,
		raw[v3DataKeyNonceAt:v3SealedDataKeyAt], raw[v3SealedDataKeyAt:v3NonceAt],
		v3DataKeyAAD(agentPrivateKeyPurpose, raw[:v3DataKeyNonceAt], row))
	assert.Error(t, err, "the data key is no longer sealed under the older master key")

	again, changed, err := ring.RewrapPrivateKey(row, rewrapped)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, rewrapped, again)

	// A value whose sealed part was altered is refused, not carried over to
	// the current master key.
	altered := append([]byte(nil), storedRaw...)
	altered[len(altered)-1] ^= 0x01
	_, _, err = ring.RewrapPrivateKey(row, base64.StdEncoding.EncodeToString(altered))
	assert.Error(t, err)

	v2, err := before.sealFormatV2(agentPrivateKeyPurpose, row, []byte("v2-private-key"))
	require.NoError(t, err)
	moved, changed, err := ring.RewrapPrivateKey(row, v2)
	require.NoError(t, err)
	assert.True(t, changed)
	raw, _, plaintext = openV3Parts(t, newKey, row, moved)
	assert.Equal(t, byte(2), raw[1], "storage key id")
	assert.Equal(t, "v2-private-key", string(plaintext))
}

// The one-time conversion of v1 values writes format v3, and leaves a value
// already in format v2 or v3 for its row as it is.
func TestMigrateLegacyPrivateKey_SealsV1UnderADataKey(t *testing.T) {
	masterKey := fixedTestKey("data key legacy master key", 32)
	kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(masterKey))
	require.NoError(t, err)
	row := uuid.New()

	migrated, changed, err := kv.MigrateLegacyPrivateKey(row, sealLegacyV1(t, kv, "legacy-private-key"))
	require.NoError(t, err)
	assert.True(t, changed)
	_, _, plaintext := openV3Parts(t, masterKey, row, migrated)
	assert.Equal(t, "legacy-private-key", string(plaintext))

	current, err := kv.EncryptPrivateKey(row, "current-private-key")
	require.NoError(t, err)
	v2, err := kv.sealFormatV2(agentPrivateKeyPurpose, row, []byte("v2-private-key"))
	require.NoError(t, err)
	for name, stored := range map[string]string{"format v3": current, "format v2": v2} {
		out, changed, err := kv.MigrateLegacyPrivateKey(row, stored)
		require.NoError(t, err, name)
		assert.False(t, changed, name)
		assert.Equal(t, stored, out, name)
	}
}
