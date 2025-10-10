package aimsdk

import (
	"testing"
)

func TestGenerateEd25519Keypair(t *testing.T) {
	privKey, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	if privKey == nil {
		t.Fatal("Private key is nil")
	}

	if pubKey == nil {
		t.Fatal("Public key is nil")
	}

	// Check key sizes
	if len(privKey) != 64 {
		t.Errorf("Expected private key size 64, got %d", len(privKey))
	}

	if len(pubKey) != 32 {
		t.Errorf("Expected public key size 32, got %d", len(pubKey))
	}
}

func TestSignRequest(t *testing.T) {
	// Generate keypair
	privKey, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Test data
	data := map[string]interface{}{
		"agent_id":  "test-agent-123",
		"timestamp": "2025-10-09T12:00:00Z",
		"type":      "ai_agent",
	}

	// Sign the data
	signature, err := SignRequest(privKey, data)
	if err != nil {
		t.Fatalf("Failed to sign request: %v", err)
	}

	if signature == "" {
		t.Fatal("Signature is empty")
	}

	// Verify signature
	valid := VerifySignature(pubKey, data, signature)
	if !valid {
		t.Error("Signature verification failed")
	}
}

func TestSignRequestWithSortedKeys(t *testing.T) {
	// Generate keypair
	privKey, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Test data with different key order
	data1 := map[string]interface{}{
		"z_field": "last",
		"a_field": "first",
		"m_field": "middle",
	}

	data2 := map[string]interface{}{
		"a_field": "first",
		"m_field": "middle",
		"z_field": "last",
	}

	// Sign both versions
	sig1, err := SignRequest(privKey, data1)
	if err != nil {
		t.Fatalf("Failed to sign data1: %v", err)
	}

	sig2, err := SignRequest(privKey, data2)
	if err != nil {
		t.Fatalf("Failed to sign data2: %v", err)
	}

	// Signatures should be identical (sorted keys)
	if sig1 != sig2 {
		t.Error("Signatures differ for same data with different key order")
	}

	// Both should verify
	if !VerifySignature(pubKey, data1, sig1) {
		t.Error("Signature 1 verification failed")
	}

	if !VerifySignature(pubKey, data2, sig2) {
		t.Error("Signature 2 verification failed")
	}
}

func TestVerifySignatureWithInvalidSignature(t *testing.T) {
	// Generate keypair
	_, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	data := map[string]interface{}{
		"test": "data",
	}

	// Invalid signature
	invalidSig := "invalid-signature-base64"

	// Should fail verification
	valid := VerifySignature(pubKey, data, invalidSig)
	if valid {
		t.Error("Invalid signature should not verify")
	}
}

func TestVerifySignatureWithTamperedData(t *testing.T) {
	// Generate keypair
	privKey, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Original data
	originalData := map[string]interface{}{
		"value": "original",
	}

	// Sign original data
	signature, err := SignRequest(privKey, originalData)
	if err != nil {
		t.Fatalf("Failed to sign request: %v", err)
	}

	// Tampered data
	tamperedData := map[string]interface{}{
		"value": "tampered",
	}

	// Verification should fail for tampered data
	valid := VerifySignature(pubKey, tamperedData, signature)
	if valid {
		t.Error("Tampered data should not verify with original signature")
	}
}

func TestEncodeDecodePublicKey(t *testing.T) {
	// Generate keypair
	_, pubKey, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Encode to base64
	encoded := EncodePublicKey(pubKey)
	if encoded == "" {
		t.Fatal("Encoded public key is empty")
	}

	// Decode back
	decoded, err := DecodePublicKey(encoded)
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}

	// Should match original
	if string(decoded) != string(pubKey) {
		t.Error("Decoded public key does not match original")
	}
}

func TestEncodeDecodePrivateKey(t *testing.T) {
	// Generate keypair
	privKey, _, err := GenerateEd25519Keypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Encode to base64
	encoded := EncodePrivateKey(privKey)
	if encoded == "" {
		t.Fatal("Encoded private key is empty")
	}

	// Decode back
	decoded, err := DecodePrivateKey(encoded)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// Should match original
	if string(decoded) != string(privKey) {
		t.Error("Decoded private key does not match original")
	}
}

func TestDecodeInvalidPublicKey(t *testing.T) {
	// Invalid base64
	_, err := DecodePublicKey("!@#$%^&*()")
	if err == nil {
		t.Error("Should fail to decode invalid base64")
	}

	// Invalid size (too short)
	shortKey := "aGVsbG8=" // "hello" in base64
	_, err = DecodePublicKey(shortKey)
	if err == nil {
		t.Error("Should fail to decode key with invalid size")
	}
}

func TestDecodeInvalidPrivateKey(t *testing.T) {
	// Invalid base64
	_, err := DecodePrivateKey("!@#$%^&*()")
	if err == nil {
		t.Error("Should fail to decode invalid base64")
	}

	// Invalid size (too short)
	shortKey := "aGVsbG8=" // "hello" in base64
	_, err = DecodePrivateKey(shortKey)
	if err == nil {
		t.Error("Should fail to decode key with invalid size")
	}
}
