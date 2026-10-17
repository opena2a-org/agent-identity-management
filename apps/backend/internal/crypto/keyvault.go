package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// KeyVault handles secure storage and retrieval of private keys
// Uses AES-256-GCM for encryption at rest, bound to the storing agent row
//
// It holds a ring of master keys keyed by storage-key id, the second byte of
// the v2 header. It decrypts under any id in the ring, encrypts only under the
// current id, and refuses an id outside the ring.
type KeyVault struct {
	masterKey []byte          // the current master key (AES-256, 32 bytes); derived signing keys come from it
	currentID byte            // the storage-key id of masterKey; every new ciphertext carries it
	ring      map[byte][]byte // every master key the vault decrypts under, by storage-key id, masterKey included
	source    string          // where masterKey came from, set by NewKeyVaultFromEnv; see MasterKeySource
}

// NewKeyVault creates a KeyVault holding one master key, under storage-key id 1
// The master key should be stored securely (e.g., environment variable, secrets manager)
func NewKeyVault(masterKeyBase64 string) (*KeyVault, error) {
	masterKey, err := decodeMasterKey(masterKeyBase64)
	if err != nil {
		return nil, err
	}
	return newKeyVaultRing(defaultStorageKeyID, map[byte][]byte{defaultStorageKeyID: masterKey})
}

// NewKeyVaultRing creates a KeyVault that decrypts under every master key in
// masterKeysBase64, keyed by storage-key id, and encrypts under the key at
// currentID. Ids run from 1 to 255, and no two ids may hold the same key.
func NewKeyVaultRing(currentID byte, masterKeysBase64 map[byte]string) (*KeyVault, error) {
	keys := make(map[byte][]byte, len(masterKeysBase64))
	for _, id := range sortedStorageKeyIDs(masterKeysBase64) {
		masterKey, err := decodeMasterKey(masterKeysBase64[id])
		if err != nil {
			return nil, fmt.Errorf("storage key id %d: %w", id, err)
		}
		keys[id] = masterKey
	}
	return newKeyVaultRing(currentID, keys)
}

func newKeyVaultRing(currentID byte, keys map[byte][]byte) (*KeyVault, error) {
	if currentID == 0 {
		return nil, errStorageKeyIDRange
	}
	current, ok := keys[currentID]
	if !ok {
		return nil, fmt.Errorf("no master key is set for the current storage key id %d", currentID)
	}

	ids := sortedStorageKeyIDs(keys)
	ring := make(map[byte][]byte, len(keys))
	for i, id := range ids {
		if id == 0 {
			return nil, errStorageKeyIDRange
		}
		// One key under two ids is no rotation: the sweep would report the
		// older id empty while the key it names is still in use.
		for _, earlier := range ids[:i] {
			if subtle.ConstantTimeCompare(keys[earlier], keys[id]) == 1 {
				return nil, fmt.Errorf("storage key ids %d and %d hold the same master key; each id needs its own key", earlier, id)
			}
		}
		ring[id] = keys[id]
	}

	return &KeyVault{
		masterKey: current,
		currentID: currentID,
		ring:      ring,
	}, nil
}

// decodeMasterKey decodes and checks one base64 master key.
func decodeMasterKey(masterKeyBase64 string) ([]byte, error) {
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

	return masterKey, nil
}

func sortedStorageKeyIDs[V any](keys map[byte]V) []byte {
	ids := make([]byte, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// StorageKeyID returns the storage-key id that every new ciphertext carries.
func (kv *KeyVault) StorageKeyID() byte {
	return kv.currentID
}

// StorageKeyIDs returns every storage-key id the vault decrypts under, in
// ascending order.
func (kv *KeyVault) StorageKeyIDs() []byte {
	return sortedStorageKeyIDs(kv.ring)
}

// HasStorageKeyID reports whether the vault decrypts under id.
func (kv *KeyVault) HasStorageKeyID(id byte) bool {
	_, ok := kv.ring[id]
	return ok
}

const (
	// MasterKeySourceEnvironment is the source of a current master key read
	// from KEYVAULT_MASTER_KEY.
	MasterKeySourceEnvironment = "environment"
	// MasterKeySourceEphemeral is the source of a current master key generated
	// at start because ENVIRONMENT is development or test.
	MasterKeySourceEphemeral = "ephemeral"
)

// MasterKeySource returns where the current master key of a vault built by
// NewKeyVaultFromEnv came from: MasterKeySourceEnvironment or
// MasterKeySourceEphemeral. It is empty for a vault built from keys passed to
// NewKeyVault or NewKeyVaultRing.
func (kv *KeyVault) MasterKeySource() string {
	return kv.source
}

// MasterKeyIdentity returns the identity of the current master key, the key
// every new ciphertext is encrypted under: 32 lowercase hex characters. It
// names the key without revealing it, so two starts can be compared; it is not
// a key and is used for nothing else.
func (kv *KeyVault) MasterKeyIdentity() string {
	return masterKeyIdentity(kv.masterKey)
}

// masterKeyIdentity is the leftmost 16 bytes of HMAC-SHA256 keyed with the
// decoded master key over a fixed label, in lowercase hex.
func masterKeyIdentity(masterKey []byte) string {
	mac := hmac.New(sha256.New, masterKey)
	mac.Write([]byte("opena2a-aim/keyvault-master-key/v1/key-id"))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// ephemeralKeyEnvironments are the ENVIRONMENT values that may run on a
// generated master key. Matching is exact: an unset, misspelled or unrecognised
// value is never treated as development.
var ephemeralKeyEnvironments = map[string]bool{
	"development": true,
	"test":        true,
}

const (
	// MasterKeyEnvVar holds the current master key: every new ciphertext is
	// encrypted under it.
	MasterKeyEnvVar = "KEYVAULT_MASTER_KEY"
	// MasterKeyIDEnvVar holds the storage-key id of MasterKeyEnvVar, a decimal
	// number from 1 to 255. Unset, it is 1.
	MasterKeyIDEnvVar = "KEYVAULT_MASTER_KEY_ID"
)

var errStorageKeyIDRange = errors.New("storage key id must be from 1 to 255")

// RingMasterKeyEnvVar returns the variable that holds the master key for a
// storage-key id other than the current one, for example KEYVAULT_MASTER_KEY_1.
// The vault decrypts under it and never encrypts under it.
func RingMasterKeyEnvVar(id byte) string {
	return MasterKeyEnvVar + "_" + strconv.Itoa(int(id))
}

// parseStorageKeyID reads the value of MasterKeyIDEnvVar.
func parseStorageKeyID(value string) (byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultStorageKeyID, nil
	}
	id, err := strconv.ParseUint(value, 10, 8)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("%s must be a storage key id from 1 to 255, got %q", MasterKeyIDEnvVar, value)
	}
	return byte(id), nil
}

// NewKeyVaultFromEnv creates a KeyVault using a master key from environment variable
// SECURITY: Master key MUST be set everywhere except an explicit development or test environment
//
// KEYVAULT_MASTER_KEY is the current key and KEYVAULT_MASTER_KEY_ID its
// storage-key id (1 when unset). Each KEYVAULT_MASTER_KEY_<id> adds the key
// for another id, which the vault decrypts under and never encrypts under.
func NewKeyVaultFromEnv() (*KeyVault, error) {
	return newKeyVaultFromEnv(os.Getenv)
}

func newKeyVaultFromEnv(getenv func(string) string) (*KeyVault, error) {
	masterKeyBase64 := getenv(MasterKeyEnvVar)
	environment := getenv("ENVIRONMENT")

	currentID, err := parseStorageKeyID(getenv(MasterKeyIDEnvVar))
	if err != nil {
		return nil, err
	}

	set := make(map[byte]string)
	for n := 1; n <= 255; n++ {
		if value := getenv(RingMasterKeyEnvVar(byte(n))); value != "" {
			set[byte(n)] = value
		}
	}
	ids := sortedStorageKeyIDs(set)

	if masterKeyBase64 == "" && len(ids) > 0 {
		// A generated current key would encrypt new rows under a key that is
		// gone at the next start, while the older keys suggest a rotation.
		return nil, fmt.Errorf("SECURITY ERROR: %s is required when %s holds an older master key", MasterKeyEnvVar, RingMasterKeyEnvVar(ids[0]))
	}

	keys := make(map[byte][]byte, len(ids)+1)
	for _, id := range ids {
		name := RingMasterKeyEnvVar(id)
		if id == currentID {
			return nil, fmt.Errorf("%s is set, but %d is the storage key id of %s (%s); set the current key only in %s",
				name, id, MasterKeyEnvVar, MasterKeyIDEnvVar, MasterKeyEnvVar)
		}
		masterKey, err := decodeMasterKey(set[id])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		keys[id] = masterKey
	}

	source := MasterKeySourceEnvironment
	if masterKeyBase64 == "" {
		source = MasterKeySourceEphemeral

		// A generated key changes at every restart, which changes the server
		// signing key and leaves stored agent private keys undecryptable, so
		// only an explicit development or test setting may accept it.
		if !ephemeralKeyEnvironments[environment] {
			return nil, fmt.Errorf("SECURITY ERROR: KEYVAULT_MASTER_KEY is required when ENVIRONMENT=%q; "+
				"set KEYVAULT_MASTER_KEY to the base64 encoding of 32 random bytes "+
				"(generate one with: openssl rand -base64 32) and keep it across restarts", environment)
		}

		// Generate a new master key for development only
		// SECURITY: This key is ephemeral and will be different on each restart
		// This is acceptable for development but NOT for production
		fmt.Println("SECURITY WARNING: KEYVAULT_MASTER_KEY not set")
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

	current, err := decodeMasterKey(masterKeyBase64)
	if err != nil {
		return nil, err
	}
	keys[currentID] = current
	kv, err := newKeyVaultRing(currentID, keys)
	if err != nil {
		return nil, err
	}
	kv.source = source
	return kv, nil
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
//
// The storage-key id names the master key in the vault's ring that encrypted
// the ciphertext; it is never 0.
const (
	agentPrivateKeyPurpose = "aim/agent-private-key"

	privateKeyFormatV2 byte = 0x02

	// defaultStorageKeyID is the storage-key id of a vault built from one
	// master key, and of KEYVAULT_MASTER_KEY when KEYVAULT_MASTER_KEY_ID is unset.
	defaultStorageKeyID byte = 0x01

	privateKeyHeaderSize = 2
)

// ErrUnsupportedPrivateKeyFormat is returned when a stored private key is not
// in format v2, including a v1 ciphertext that predates storage binding.
var ErrUnsupportedPrivateKeyFormat = errors.New("unsupported private key format")

// newGCM returns the AEAD for the current master key.
func (kv *KeyVault) newGCM() (cipher.AEAD, error) {
	return gcmFor(kv.masterKey)
}

func gcmFor(masterKey []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(masterKey)
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

func (kv *KeyVault) openV2(purpose string, agentID uuid.UUID, encoded string) ([]byte, error) {
	if agentID == uuid.Nil {
		return nil, fmt.Errorf("agent ID is required to decrypt a private key")
	}

	data, err := decodeV2(encoded)
	if err != nil {
		return nil, err
	}
	header := data[:privateKeyHeaderSize]
	masterKey, ok := kv.ring[header[1]]
	if !ok {
		return nil, fmt.Errorf("unknown storage key id %d", header[1])
	}

	gcm, err := gcmFor(masterKey)
	if err != nil {
		return nil, err
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

// decodeV2 decodes a stored ciphertext and checks that it starts with a v2
// header.
func decodeV2(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}
	if len(data) < privateKeyHeaderSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	if data[0] != privateKeyFormatV2 {
		return nil, ErrUnsupportedPrivateKeyFormat
	}
	return data, nil
}

// PrivateKeyStorageKeyID returns the storage-key id in the header of a stored
// private key, without decrypting it. A value that is not in format v2 returns
// ErrUnsupportedPrivateKeyFormat.
func PrivateKeyStorageKeyID(encryptedPrivateKey string) (byte, error) {
	data, err := decodeV2(encryptedPrivateKey)
	if err != nil {
		return 0, err
	}
	return data[1], nil
}

// RewrapPrivateKey re-encrypts the private key stored in the agent row with
// the given ID under the current storage-key id, from any id in the ring. A
// value already under the current id is returned unchanged with
// rewrapped=false. The ciphertext stays bound to the same row, and the key
// pair is unchanged.
func (kv *KeyVault) RewrapPrivateKey(agentID uuid.UUID, encryptedPrivateKey string) (string, bool, error) {
	id, err := PrivateKeyStorageKeyID(encryptedPrivateKey)
	if err != nil {
		return "", false, err
	}
	if id == kv.currentID {
		return encryptedPrivateKey, false, nil
	}

	plaintext, err := kv.openV2(agentPrivateKeyPurpose, agentID, encryptedPrivateKey)
	if err != nil {
		return "", false, err
	}
	rewrapped, err := kv.sealV2(agentPrivateKeyPurpose, agentID, plaintext)
	if err != nil {
		return "", false, err
	}
	return rewrapped, true, nil
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
// data. It is reachable only through MigrateLegacyPrivateKey. v1 names no
// storage-key id, so it tries the current key and then each other key in the
// ring; the GCM tag refuses every key but the one that encrypted it.
func (kv *KeyVault) openLegacyV1(encryptedPrivateKey string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	order := []byte{kv.currentID}
	for _, id := range kv.StorageKeyIDs() {
		if id != kv.currentID {
			order = append(order, id)
		}
	}

	var lastErr error
	for _, id := range order {
		gcm, err := gcmFor(kv.ring[id])
		if err != nil {
			return nil, err
		}

		nonceSize := gcm.NonceSize()
		if len(ciphertext) < nonceSize {
			return nil, fmt.Errorf("ciphertext too short")
		}

		nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]

		plaintext, err := gcm.Open(nil, nonce, sealed, nil)
		if err == nil {
			return plaintext, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("failed to decrypt: %w", lastErr)
}

// RotatePrivateKey decrypts with old key, re-encrypts with new key. The
// ciphertext stays bound to the same agent row. The result carries the
// storage-key id of a vault built from newMasterKeyBase64 alone (1). The
// server rotates through its key ring instead: NewKeyVaultRing and
// RewrapPrivateKey.
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
