package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEd25519KeyPair(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)
	assert.NotNil(t, kp)
	assert.NotNil(t, kp.PublicKey)
	assert.NotNil(t, kp.PrivateKey)

	// Check key sizes
	assert.Len(t, kp.PublicKey, ed25519.PublicKeySize)
	assert.Len(t, kp.PrivateKey, ed25519.PrivateKeySize)
}

func TestGenerateEd25519KeyPair_Unique(t *testing.T) {
	kp1, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	kp2, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	// Keys should be different
	assert.NotEqual(t, kp1.PublicKey, kp2.PublicKey)
	assert.NotEqual(t, kp1.PrivateKey, kp2.PrivateKey)
}

func TestEncodeKeyPair(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	encoded := EncodeKeyPair(kp)
	assert.NotNil(t, encoded)
	assert.NotEmpty(t, encoded.PublicKeyBase64)
	assert.NotEmpty(t, encoded.PrivateKeyBase64)
	assert.Equal(t, "Ed25519", encoded.Algorithm)

	// Verify they're valid base64
	_, err = base64.StdEncoding.DecodeString(encoded.PublicKeyBase64)
	assert.NoError(t, err)

	_, err = base64.StdEncoding.DecodeString(encoded.PrivateKeyBase64)
	assert.NoError(t, err)
}

func TestDecodePublicKey_Valid(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	encoded := EncodeKeyPair(kp)

	decodedPubKey, err := DecodePublicKey(encoded.PublicKeyBase64)
	require.NoError(t, err)
	assert.Equal(t, kp.PublicKey, decodedPubKey)
}

func TestDecodePublicKey_InvalidBase64(t *testing.T) {
	_, err := DecodePublicKey("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode public key")
}

func TestDecodePublicKey_WrongSize(t *testing.T) {
	// Encode wrong size data
	wrongSize := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err := DecodePublicKey(wrongSize)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid public key size")
}

func TestDecodePrivateKey_Valid(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	encoded := EncodeKeyPair(kp)

	decodedPrivKey, err := DecodePrivateKey(encoded.PrivateKeyBase64)
	require.NoError(t, err)
	assert.Equal(t, kp.PrivateKey, decodedPrivKey)
}

func TestDecodePrivateKey_InvalidBase64(t *testing.T) {
	_, err := DecodePrivateKey("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode private key")
}

func TestDecodePrivateKey_WrongSize(t *testing.T) {
	// Encode wrong size data
	wrongSize := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err := DecodePrivateKey(wrongSize)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key size")
}

func TestSignAndVerifyMessage(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature := SignMessage(kp.PrivateKey, message)

	// Signature should be valid
	assert.True(t, VerifySignature(kp.PublicKey, message, signature))
}

func TestVerifySignature_WrongMessage(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature := SignMessage(kp.PrivateKey, message)

	// Different message should not verify
	wrongMessage := []byte("Goodbye, World!")
	assert.False(t, VerifySignature(kp.PublicKey, wrongMessage, signature))
}

func TestVerifySignature_WrongKey(t *testing.T) {
	kp1, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	kp2, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature := SignMessage(kp1.PrivateKey, message)

	// Different key should not verify
	assert.False(t, VerifySignature(kp2.PublicKey, message, signature))
}

func TestVerifySignature_TamperedSignature(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature := SignMessage(kp.PrivateKey, message)

	// Tamper with signature
	signature[0] ^= 0xFF

	assert.False(t, VerifySignature(kp.PublicKey, message, signature))
}

func TestSignMessage_EmptyMessage(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte{}
	signature := SignMessage(kp.PrivateKey, message)

	// Empty message should still be signable and verifiable
	assert.True(t, VerifySignature(kp.PublicKey, message, signature))
}

func TestSignMessage_LargeMessage(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	// Create a large message (1MB)
	largeMessage := make([]byte, 1024*1024)
	for i := range largeMessage {
		largeMessage[i] = byte(i % 256)
	}

	signature := SignMessage(kp.PrivateKey, largeMessage)

	// Large message should still be signable and verifiable
	assert.True(t, VerifySignature(kp.PublicKey, largeMessage, signature))
}

func TestRoundTrip_EncodeDecodeSign(t *testing.T) {
	// Generate key pair
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	// Encode keys
	encoded := EncodeKeyPair(kp)

	// Decode keys
	pubKey, err := DecodePublicKey(encoded.PublicKeyBase64)
	require.NoError(t, err)

	privKey, err := DecodePrivateKey(encoded.PrivateKeyBase64)
	require.NoError(t, err)

	// Sign with decoded private key
	message := []byte("Test message for round trip")
	signature := SignMessage(privKey, message)

	// Verify with decoded public key
	assert.True(t, VerifySignature(pubKey, message, signature))
}

func TestSignatureSize(t *testing.T) {
	kp, err := GenerateEd25519KeyPair()
	require.NoError(t, err)

	message := []byte("Test message")
	signature := SignMessage(kp.PrivateKey, message)

	// Ed25519 signatures are always 64 bytes
	assert.Len(t, signature, ed25519.SignatureSize)
}
