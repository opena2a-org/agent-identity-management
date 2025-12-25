package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewED25519Service(t *testing.T) {
	service := NewED25519Service()
	assert.NotNil(t, service)
}

func TestED25519Service_GenerateKeyPair(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)
	assert.NotNil(t, keyPair)
	assert.NotEmpty(t, keyPair.PublicKey)
	assert.NotEmpty(t, keyPair.PrivateKey)

	// Verify keys are valid base64
	pubKey, err := base64.StdEncoding.DecodeString(keyPair.PublicKey)
	require.NoError(t, err)
	assert.Len(t, pubKey, ed25519.PublicKeySize)

	privKey, err := base64.StdEncoding.DecodeString(keyPair.PrivateKey)
	require.NoError(t, err)
	assert.Len(t, privKey, ed25519.PrivateKeySize)
}

func TestED25519Service_GenerateKeyPair_Unique(t *testing.T) {
	service := NewED25519Service()

	kp1, err := service.GenerateKeyPair()
	require.NoError(t, err)

	kp2, err := service.GenerateKeyPair()
	require.NoError(t, err)

	assert.NotEqual(t, kp1.PublicKey, kp2.PublicKey)
	assert.NotEqual(t, kp1.PrivateKey, kp2.PrivateKey)
}

func TestED25519Service_Sign(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature, err := service.Sign(keyPair.PrivateKey, message)
	require.NoError(t, err)
	assert.NotEmpty(t, signature)

	// Signature should be valid base64
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	require.NoError(t, err)
	assert.Len(t, sigBytes, ed25519.SignatureSize)
}

func TestED25519Service_Sign_InvalidPrivateKey(t *testing.T) {
	service := NewED25519Service()

	message := []byte("Hello, World!")

	// Invalid base64
	_, err := service.Sign("not-valid-base64!!!", message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode private key")

	// Wrong size
	wrongSize := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err = service.Sign(wrongSize, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key size")
}

func TestED25519Service_Verify(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature, err := service.Sign(keyPair.PrivateKey, message)
	require.NoError(t, err)

	valid, err := service.Verify(keyPair.PublicKey, message, signature)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestED25519Service_Verify_WrongMessage(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature, err := service.Sign(keyPair.PrivateKey, message)
	require.NoError(t, err)

	wrongMessage := []byte("Goodbye, World!")
	valid, err := service.Verify(keyPair.PublicKey, wrongMessage, signature)
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestED25519Service_Verify_WrongKey(t *testing.T) {
	service := NewED25519Service()

	kp1, err := service.GenerateKeyPair()
	require.NoError(t, err)

	kp2, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature, err := service.Sign(kp1.PrivateKey, message)
	require.NoError(t, err)

	// Verify with different public key should fail
	valid, err := service.Verify(kp2.PublicKey, message, signature)
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestED25519Service_Verify_InvalidPublicKey(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")
	signature, err := service.Sign(keyPair.PrivateKey, message)
	require.NoError(t, err)

	// Invalid base64
	_, err = service.Verify("not-valid-base64!!!", message, signature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode public key")

	// Wrong size
	wrongSize := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err = service.Verify(wrongSize, message, signature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid public key size")
}

func TestED25519Service_Verify_InvalidSignature(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	message := []byte("Hello, World!")

	// Invalid base64 signature
	_, err = service.Verify(keyPair.PublicKey, message, "not-valid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode signature")
}

func TestED25519Service_GenerateChallenge(t *testing.T) {
	service := NewED25519Service()

	challenge, err := service.GenerateChallenge()
	require.NoError(t, err)
	assert.NotEmpty(t, challenge)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(challenge)
	require.NoError(t, err)
	assert.Len(t, decoded, 32)
}

func TestED25519Service_GenerateChallenge_Unique(t *testing.T) {
	service := NewED25519Service()

	challenge1, err := service.GenerateChallenge()
	require.NoError(t, err)

	challenge2, err := service.GenerateChallenge()
	require.NoError(t, err)

	assert.NotEqual(t, challenge1, challenge2)
}

func TestED25519Service_SignAndVerifyChallenge(t *testing.T) {
	service := NewED25519Service()

	// Generate keypair
	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	// Generate challenge
	challenge, err := service.GenerateChallenge()
	require.NoError(t, err)

	// Sign challenge
	signature, err := service.Sign(keyPair.PrivateKey, []byte(challenge))
	require.NoError(t, err)

	// Verify signature
	valid, err := service.Verify(keyPair.PublicKey, []byte(challenge), signature)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestED25519Service_EmptyMessage(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	// Empty message should work
	message := []byte{}
	signature, err := service.Sign(keyPair.PrivateKey, message)
	require.NoError(t, err)

	valid, err := service.Verify(keyPair.PublicKey, message, signature)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestED25519Service_LargeMessage(t *testing.T) {
	service := NewED25519Service()

	keyPair, err := service.GenerateKeyPair()
	require.NoError(t, err)

	// Large message (1MB)
	largeMessage := make([]byte, 1024*1024)
	for i := range largeMessage {
		largeMessage[i] = byte(i % 256)
	}

	signature, err := service.Sign(keyPair.PrivateKey, largeMessage)
	require.NoError(t, err)

	valid, err := service.Verify(keyPair.PublicKey, largeMessage, signature)
	require.NoError(t, err)
	assert.True(t, valid)
}
