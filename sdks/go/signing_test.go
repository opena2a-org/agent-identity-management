package aimsdk

import (
	"encoding/base64"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	if kp == nil {
		t.Fatal("GenerateKeyPair() returned nil keypair")
	}

	if len(kp.PublicKey) != 32 {
		t.Errorf("PublicKey length = %d, want 32", len(kp.PublicKey))
	}

	if len(kp.PrivateKey) != 64 {
		t.Errorf("PrivateKey length = %d, want 64", len(kp.PrivateKey))
	}
}

func TestSignAndVerify(t *testing.T) {
	// Generate keypair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	// Test message
	message := "test message for signing"

	// Sign message
	signature, err := kp.Sign(message)
	if err != nil {
		t.Fatalf("Sign() failed: %v", err)
	}

	if signature == "" {
		t.Fatal("Sign() returned empty signature")
	}

	// Verify signature
	valid, err := kp.Verify(message, signature)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}

	if !valid {
		t.Error("Verify() returned false for valid signature")
	}

	// Test invalid signature
	invalidSig := "invalid_signature_base64"
	valid, err = kp.Verify(message, invalidSig)
	if err == nil {
		t.Error("Verify() should fail for invalid base64 signature")
	}

	// Test wrong message
	validDifferentMsg, err := kp.Verify("different message", signature)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}

	if validDifferentMsg {
		t.Error("Verify() should return false for different message")
	}
}

func TestKeyPairFromBase64(t *testing.T) {
	// Generate original keypair
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	// Export private key as base64
	privateKeyB64 := original.PrivateKeyBase64()

	// Import from base64
	imported, err := NewKeyPairFromBase64(privateKeyB64)
	if err != nil {
		t.Fatalf("NewKeyPairFromBase64() failed: %v", err)
	}

	// Verify they match
	if original.PublicKeyBase64() != imported.PublicKeyBase64() {
		t.Error("Public keys don't match after import")
	}

	if original.PrivateKeyBase64() != imported.PrivateKeyBase64() {
		t.Error("Private keys don't match after import")
	}

	// Test signing with both
	message := "test message"

	sig1, err := original.Sign(message)
	if err != nil {
		t.Fatalf("Original Sign() failed: %v", err)
	}

	sig2, err := imported.Sign(message)
	if err != nil {
		t.Fatalf("Imported Sign() failed: %v", err)
	}

	// Signatures should be deterministic for Ed25519
	if sig1 != sig2 {
		t.Error("Signatures don't match between original and imported keypairs")
	}
}

func TestKeyPairFromPrivateKey32Bytes(t *testing.T) {
	// Generate keypair
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	// Get 32-byte seed
	seedB64 := original.SeedBase64()
	seedBytes, err := base64.StdEncoding.DecodeString(seedB64)
	if err != nil {
		t.Fatalf("Failed to decode seed: %v", err)
	}

	// Create keypair from 32-byte seed
	kp, err := NewKeyPairFromPrivateKey(seedBytes)
	if err != nil {
		t.Fatalf("NewKeyPairFromPrivateKey(32 bytes) failed: %v", err)
	}

	// Public keys should match
	if original.PublicKeyBase64() != kp.PublicKeyBase64() {
		t.Error("Public keys don't match when created from 32-byte seed")
	}
}

func TestKeyPairFromPrivateKey64Bytes(t *testing.T) {
	// Generate keypair
	original, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	// Get 64-byte private key
	privateKeyBytes, err := base64.StdEncoding.DecodeString(original.PrivateKeyBase64())
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// Create keypair from 64-byte private key
	kp, err := NewKeyPairFromPrivateKey(privateKeyBytes)
	if err != nil {
		t.Fatalf("NewKeyPairFromPrivateKey(64 bytes) failed: %v", err)
	}

	// Keys should match exactly
	if original.PublicKeyBase64() != kp.PublicKeyBase64() {
		t.Error("Public keys don't match when created from 64-byte private key")
	}

	if original.PrivateKeyBase64() != kp.PrivateKeyBase64() {
		t.Error("Private keys don't match when created from 64-byte private key")
	}
}

func TestSignPayload(t *testing.T) {
	// Generate keypair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}

	// Create test payload
	payload := map[string]interface{}{
		"name":       "test-agent",
		"type":       "ai_agent",
		"public_key": kp.PublicKeyBase64(),
	}

	// Sign payload
	signature, err := kp.SignPayload(payload)
	if err != nil {
		t.Fatalf("SignPayload() failed: %v", err)
	}

	if signature == "" {
		t.Fatal("SignPayload() returned empty signature")
	}

	// Verify signature is valid base64
	_, err = base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Errorf("SignPayload() returned invalid base64: %v", err)
	}
}

func TestClientSignMessage(t *testing.T) {
	// Create client
	config := Config{
		APIURL:  "http://localhost:8080",
		APIKey:  "test-key",
		AgentID: "test-agent-id",
	}
	client := NewClient(config)

	// Test without keypair - should fail
	_, err := client.SignMessage("test")
	if err == nil {
		t.Error("SignMessage() should fail when client has no keypair")
	}

	// Generate and set keypair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}
	client.SetKeyPair(kp)

	// Now signing should work
	message := "test message"
	signature, err := client.SignMessage(message)
	if err != nil {
		t.Fatalf("SignMessage() failed: %v", err)
	}

	if signature == "" {
		t.Fatal("SignMessage() returned empty signature")
	}

	// Verify signature
	valid, err := kp.Verify(message, signature)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}

	if !valid {
		t.Error("Signature verification failed")
	}
}

func TestGetPublicKey(t *testing.T) {
	// Create client without keypair
	config := Config{
		APIURL:  "http://localhost:8080",
		APIKey:  "test-key",
		AgentID: "test-agent-id",
	}
	client := NewClient(config)

	// Should return empty string
	if pk := client.GetPublicKey(); pk != "" {
		t.Errorf("GetPublicKey() = %q, want empty string", pk)
	}

	// Generate and set keypair
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() failed: %v", err)
	}
	client.SetKeyPair(kp)

	// Should return public key
	pk := client.GetPublicKey()
	if pk == "" {
		t.Error("GetPublicKey() returned empty string after setting keypair")
	}

	if pk != kp.PublicKeyBase64() {
		t.Error("GetPublicKey() doesn't match keypair's public key")
	}
}
