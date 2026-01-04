package hybrid

import (
	"bytes"
	"testing"
)

func TestNewHybridSigner(t *testing.T) {
	signer, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create hybrid signer: %v", err)
	}

	if signer.Algorithm() != AlgorithmHybrid {
		t.Errorf("expected algorithm %s, got %s", AlgorithmHybrid, signer.Algorithm())
	}

	// Should be simulated (placeholder) mode
	if !signer.IsSimulated() {
		t.Error("expected simulated mode for placeholder implementation")
	}

	// Public key should be non-empty
	if signer.PublicKey() == "" {
		t.Error("expected non-empty public key")
	}

	// Key pair should be valid
	kp := signer.KeyPair()
	if kp.Algorithm != AlgorithmHybrid {
		t.Errorf("expected key pair algorithm %s, got %s", AlgorithmHybrid, kp.Algorithm)
	}
	if kp.ClassicalPublic == "" || kp.ClassicalPrivate == "" {
		t.Error("expected non-empty classical keys")
	}
	if kp.PQCPublic == "" || kp.PQCPrivate == "" {
		t.Error("expected non-empty PQC keys")
	}
}

func TestHybridSignerFromKeys(t *testing.T) {
	// Generate a signer first
	original, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create original signer: %v", err)
	}

	// Get key pair
	kp := original.KeyPair()

	// Create new signer from keys
	restored, err := NewHybridSignerFromKeys(kp)
	if err != nil {
		t.Fatalf("failed to create signer from keys: %v", err)
	}

	// Public keys should match
	if original.PublicKey() != restored.PublicKey() {
		t.Error("public keys don't match")
	}
}

func TestHybridSignAndVerify(t *testing.T) {
	signer, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	message := []byte("test message for hybrid signing")

	// Sign the message
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	if signature == "" {
		t.Error("expected non-empty signature")
	}

	// Verify the signature
	valid, err := signer.Verify(message, signature)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !valid {
		t.Error("expected valid signature")
	}
}

func TestHybridSignAndVerifyBytes(t *testing.T) {
	signer, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	message := []byte("another test message")

	// Sign the message
	signature, err := signer.SignBytes(message)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	// Should have reasonable size (4 + 64 + ~3309 bytes)
	if len(signature) < 100 {
		t.Errorf("signature too short: %d bytes", len(signature))
	}

	// Verify the signature
	valid, err := signer.VerifyBytes(message, signature)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !valid {
		t.Error("expected valid signature")
	}
}

func TestHybridSignatureInvalidMessage(t *testing.T) {
	signer, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	message := []byte("original message")
	tamperedMessage := []byte("tampered message")

	// Sign the message
	signature, err := signer.SignBytes(message)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	// Verify with tampered message should fail
	valid, err := signer.VerifyBytes(tamperedMessage, signature)
	if err != nil {
		t.Fatalf("verification error: %v", err)
	}

	if valid {
		t.Error("expected invalid signature for tampered message")
	}
}

func TestHybridVerifier(t *testing.T) {
	signer, err := NewHybridSigner()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// Create verifier from public keys
	verifier, err := NewHybridVerifier(
		signer.ClassicalPublicKey(),
		signer.PQCPublicKey(),
	)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	message := []byte("message to verify")

	// Sign with signer
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Verify with verifier
	valid, err := verifier.Verify(message, signature)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !valid {
		t.Error("expected valid signature")
	}

	// Verify with info
	info, err := verifier.VerifyWithInfo(message, signature)
	if err != nil {
		t.Fatalf("verification with info failed: %v", err)
	}

	if info.Algorithm != AlgorithmHybrid {
		t.Errorf("expected algorithm %s, got %s", AlgorithmHybrid, info.Algorithm)
	}
	if !info.ClassicalValid {
		t.Error("expected classical signature to be valid")
	}
	if !info.PQCValid {
		t.Error("expected PQC signature to be valid")
	}
}

func TestHybridSignatureEncoding(t *testing.T) {
	original := &HybridSignature{
		Classical: bytes.Repeat([]byte{0x42}, 64),
		PQC:       bytes.Repeat([]byte{0x43}, 3309),
	}

	// Encode
	encoded := EncodeHybridSignature(original)

	// Expected size: 4 (length) + 64 + 3309
	expectedSize := 4 + 64 + 3309
	if len(encoded) != expectedSize {
		t.Errorf("expected encoded size %d, got %d", expectedSize, len(encoded))
	}

	// Decode
	decoded, err := DecodeHybridSignature(encoded)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Compare
	if !bytes.Equal(original.Classical, decoded.Classical) {
		t.Error("classical signatures don't match")
	}
	if !bytes.Equal(original.PQC, decoded.PQC) {
		t.Error("PQC signatures don't match")
	}
}

func TestDecodeHybridSignatureErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{0x00, 0x01}},
		{"truncated", []byte{0x00, 0x00, 0x00, 0x40}}, // claims 64 bytes but only 4
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeHybridSignature(tt.data)
			if err == nil {
				t.Error("expected error for invalid data")
			}
		})
	}
}

func TestEd25519Standalone(t *testing.T) {
	signer, err := NewEd25519Signer()
	if err != nil {
		t.Fatalf("failed to create Ed25519 signer: %v", err)
	}

	if signer.Algorithm() != AlgorithmEd25519 {
		t.Errorf("expected algorithm %s, got %s", AlgorithmEd25519, signer.Algorithm())
	}

	message := []byte("Ed25519 test message")

	// Sign
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Verify
	valid, err := signer.Verify(message, signature)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !valid {
		t.Error("expected valid signature")
	}
}

func TestMLDSAStandalone(t *testing.T) {
	signer, err := NewMLDSASigner()
	if err != nil {
		t.Fatalf("failed to create ML-DSA signer: %v", err)
	}

	if signer.Algorithm() != AlgorithmMLDSA65 {
		t.Errorf("expected algorithm %s, got %s", AlgorithmMLDSA65, signer.Algorithm())
	}

	if !signer.IsSimulated() {
		t.Error("expected simulated mode")
	}

	message := []byte("ML-DSA test message")

	// Sign
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Verify (in simulated mode, this should work with same signer)
	valid, err := signer.Verify(message, signature)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !valid {
		t.Error("expected valid signature in simulated mode")
	}
}

func BenchmarkHybridSign(b *testing.B) {
	signer, err := NewHybridSigner()
	if err != nil {
		b.Fatalf("failed to create signer: %v", err)
	}

	message := []byte("benchmark message for hybrid signing performance testing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := signer.SignBytes(message)
		if err != nil {
			b.Fatalf("sign failed: %v", err)
		}
	}
}

func BenchmarkHybridVerify(b *testing.B) {
	signer, err := NewHybridSigner()
	if err != nil {
		b.Fatalf("failed to create signer: %v", err)
	}

	message := []byte("benchmark message for hybrid verification performance testing")
	signature, err := signer.SignBytes(message)
	if err != nil {
		b.Fatalf("failed to sign: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := signer.VerifyBytes(message, signature)
		if err != nil || !valid {
			b.Fatalf("verify failed")
		}
	}
}
