package pqc

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

// TestGenerateMLDSAKeyPair tests key generation for all ML-DSA variants
func TestGenerateMLDSAKeyPair(t *testing.T) {
	tests := []struct {
		name           string
		algorithm      Algorithm
		expectedPubLen int
		expectedPrvLen int
	}{
		{
			name:           "ML-DSA-44",
			algorithm:      AlgorithmMLDSA44,
			expectedPubLen: MLDSA44PublicKeySize,
			expectedPrvLen: MLDSA44PrivateKeySize,
		},
		{
			name:           "ML-DSA-65",
			algorithm:      AlgorithmMLDSA65,
			expectedPubLen: MLDSA65PublicKeySize,
			expectedPrvLen: MLDSA65PrivateKeySize,
		},
		{
			name:           "ML-DSA-87",
			algorithm:      AlgorithmMLDSA87,
			expectedPubLen: MLDSA87PublicKeySize,
			expectedPrvLen: MLDSA87PrivateKeySize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := GenerateMLDSAKeyPair(tt.algorithm)
			if err != nil {
				t.Fatalf("GenerateMLDSAKeyPair failed: %v", err)
			}

			if keyPair.Algorithm != tt.algorithm {
				t.Errorf("Algorithm mismatch: got %s, want %s", keyPair.Algorithm, tt.algorithm)
			}

			if len(keyPair.PublicKey) != tt.expectedPubLen {
				t.Errorf("Public key size: got %d, want %d", len(keyPair.PublicKey), tt.expectedPubLen)
			}

			if len(keyPair.PrivateKey) != tt.expectedPrvLen {
				t.Errorf("Private key size: got %d, want %d", len(keyPair.PrivateKey), tt.expectedPrvLen)
			}

			if keyPair.CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}
		})
	}
}

// TestGenerateMLDSAKeyPairUnsupported tests error handling for unsupported algorithms
func TestGenerateMLDSAKeyPairUnsupported(t *testing.T) {
	_, err := GenerateMLDSAKeyPair(AlgorithmEd25519)
	if err == nil {
		t.Error("Expected error for unsupported algorithm")
	}
}

// TestGenerateMLDSA65KeyPair tests the convenience function
func TestGenerateMLDSA65KeyPair(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("GenerateMLDSA65KeyPair failed: %v", err)
	}

	if keyPair.Algorithm != AlgorithmMLDSA65 {
		t.Errorf("Expected ML-DSA-65 algorithm, got %s", keyPair.Algorithm)
	}
}

// TestSignVerifyMLDSA tests sign/verify round-trip for all variants
func TestSignVerifyMLDSA(t *testing.T) {
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			// Generate key pair
			keyPair, err := GenerateMLDSAKeyPair(alg)
			if err != nil {
				t.Fatalf("Key generation failed: %v", err)
			}

			// Test message
			message := []byte("Test message for ML-DSA signature verification")

			// Sign
			signature, err := SignMLDSA(alg, keyPair.PrivateKey, message)
			if err != nil {
				t.Fatalf("Signing failed: %v", err)
			}

			// Verify signature size
			expectedSigSize, _ := GetExpectedSignatureSize(alg)
			if len(signature) != expectedSigSize {
				t.Errorf("Signature size: got %d, want %d", len(signature), expectedSigSize)
			}

			// Verify
			err = VerifyMLDSA(alg, keyPair.PublicKey, message, signature)
			if err != nil {
				t.Errorf("Verification failed: %v", err)
			}

			// Also test boolean version
			if !VerifyMLDSABool(alg, keyPair.PublicKey, message, signature) {
				t.Error("VerifyMLDSABool returned false for valid signature")
			}
		})
	}
}

// TestSignVerifyMLDSAMultipleMessages tests signing multiple different messages
func TestSignVerifyMLDSAMultipleMessages(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	messages := []string{
		"First message",
		"Second message with different content",
		"A much longer message that contains more data to ensure the signature algorithm handles various message sizes correctly",
		"", // Empty message
		string(make([]byte, 10000)), // Large message
	}

	for i, msg := range messages {
		signature, err := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, []byte(msg))
		if err != nil {
			t.Fatalf("Signing message %d failed: %v", i, err)
		}

		if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, []byte(msg), signature); err != nil {
			t.Errorf("Verification failed for message %d: %v", i, err)
		}
	}
}

// TestVerifyMLDSATamperedMessage tests that verification fails for tampered messages
func TestVerifyMLDSATamperedMessage(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	originalMessage := []byte("Original message")
	signature, err := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, originalMessage)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	// Tamper with message
	tamperedMessage := []byte("Tampered message")

	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, tamperedMessage, signature); err == nil {
		t.Error("Verification should fail for tampered message")
	}
}

// TestVerifyMLDSATamperedSignature tests that verification fails for tampered signatures
func TestVerifyMLDSATamperedSignature(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	message := []byte("Test message")
	signature, err := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, message)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	// Tamper with signature (flip a bit)
	tamperedSig := make([]byte, len(signature))
	copy(tamperedSig, signature)
	tamperedSig[0] ^= 0x01

	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, message, tamperedSig); err == nil {
		t.Error("Verification should fail for tampered signature")
	}
}

// TestVerifyMLDSAWrongKey tests that verification fails with wrong key
func TestVerifyMLDSAWrongKey(t *testing.T) {
	keyPair1, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation 1 failed: %v", err)
	}

	keyPair2, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation 2 failed: %v", err)
	}

	message := []byte("Test message")
	signature, err := SignMLDSA(AlgorithmMLDSA65, keyPair1.PrivateKey, message)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	// Try to verify with wrong public key
	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair2.PublicKey, message, signature); err == nil {
		t.Error("Verification should fail with wrong public key")
	}
}

// TestVerifyMLDSAInvalidKeySizes tests error handling for invalid key sizes
func TestVerifyMLDSAInvalidKeySizes(t *testing.T) {
	message := []byte("Test message")

	// Invalid public key size
	err := VerifyMLDSA(AlgorithmMLDSA65, []byte("short"), message, make([]byte, MLDSA65SignatureSize))
	if err == nil {
		t.Error("Should fail with invalid public key size")
	}

	// Generate valid key for signature test
	keyPair, _ := GenerateMLDSA65KeyPair()

	// Invalid signature size
	err = VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, message, []byte("short"))
	if err == nil {
		t.Error("Should fail with invalid signature size")
	}
}

// TestSignMLDSAInvalidPrivateKey tests error handling for invalid private key
func TestSignMLDSAInvalidPrivateKey(t *testing.T) {
	_, err := SignMLDSA(AlgorithmMLDSA65, []byte("invalid private key"), []byte("message"))
	if err == nil {
		t.Error("Should fail with invalid private key")
	}
}

// TestEncodeDecodeMLDSAKeys tests base64 encoding/decoding of keys
func TestEncodeDecodeMLDSAKeys(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	// Encode
	encoded := EncodeMLDSAKeyPair(keyPair)
	if encoded.Algorithm != AlgorithmMLDSA65 {
		t.Errorf("Encoded algorithm mismatch")
	}

	// Decode public key
	decodedPub, err := DecodeMLDSAPublicKey(AlgorithmMLDSA65, encoded.PublicKeyBase64)
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}

	if !bytes.Equal(decodedPub, keyPair.PublicKey) {
		t.Error("Decoded public key doesn't match original")
	}

	// Decode private key
	decodedPriv, err := DecodeMLDSAPrivateKey(AlgorithmMLDSA65, encoded.PrivateKeyBase64)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	if !bytes.Equal(decodedPriv, keyPair.PrivateKey) {
		t.Error("Decoded private key doesn't match original")
	}
}

// TestDecodeMLDSAInvalidBase64 tests error handling for invalid base64
func TestDecodeMLDSAInvalidBase64(t *testing.T) {
	_, err := DecodeMLDSAPublicKey(AlgorithmMLDSA65, "not-valid-base64!!!")
	if err == nil {
		t.Error("Should fail with invalid base64")
	}

	_, err = DecodeMLDSAPrivateKey(AlgorithmMLDSA65, "not-valid-base64!!!")
	if err == nil {
		t.Error("Should fail with invalid base64")
	}
}

// TestDecodeMLDSAInvalidSize tests error handling for valid base64 but wrong size
func TestDecodeMLDSAInvalidSize(t *testing.T) {
	shortKey := base64.StdEncoding.EncodeToString([]byte("short"))

	_, err := DecodeMLDSAPublicKey(AlgorithmMLDSA65, shortKey)
	if err == nil {
		t.Error("Should fail with invalid key size")
	}

	_, err = DecodeMLDSAPrivateKey(AlgorithmMLDSA65, shortKey)
	if err == nil {
		t.Error("Should fail with invalid key size")
	}
}

// TestGetMLDSAKeyInfo tests key info retrieval
func TestGetMLDSAKeyInfo(t *testing.T) {
	tests := []struct {
		alg       Algorithm
		pubSize   int
		privSize  int
		sigSize   int
		secLevel  string
	}{
		{AlgorithmMLDSA44, MLDSA44PublicKeySize, MLDSA44PrivateKeySize, MLDSA44SignatureSize, "NIST Level 2"},
		{AlgorithmMLDSA65, MLDSA65PublicKeySize, MLDSA65PrivateKeySize, MLDSA65SignatureSize, "NIST Level 3"},
		{AlgorithmMLDSA87, MLDSA87PublicKeySize, MLDSA87PrivateKeySize, MLDSA87SignatureSize, "NIST Level 5"},
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			info, err := GetMLDSAKeyInfo(tt.alg)
			if err != nil {
				t.Fatalf("GetMLDSAKeyInfo failed: %v", err)
			}

			if info.PublicKeySize != tt.pubSize {
				t.Errorf("PublicKeySize: got %d, want %d", info.PublicKeySize, tt.pubSize)
			}
			if info.PrivateKeySize != tt.privSize {
				t.Errorf("PrivateKeySize: got %d, want %d", info.PrivateKeySize, tt.privSize)
			}
			if info.SignatureSize != tt.sigSize {
				t.Errorf("SignatureSize: got %d, want %d", info.SignatureSize, tt.sigSize)
			}
			if info.SecurityLevel != tt.secLevel {
				t.Errorf("SecurityLevel: got %s, want %s", info.SecurityLevel, tt.secLevel)
			}
		})
	}
}

// TestGetMLDSAKeyInfoUnsupported tests error for unsupported algorithm
func TestGetMLDSAKeyInfoUnsupported(t *testing.T) {
	_, err := GetMLDSAKeyInfo(AlgorithmEd25519)
	if err == nil {
		t.Error("Should fail for non-ML-DSA algorithm")
	}
}

// TestKeyPairUniqueness tests that each key generation produces unique keys
func TestKeyPairUniqueness(t *testing.T) {
	const numPairs = 10

	keyPairs := make([]*KeyPair, numPairs)
	for i := 0; i < numPairs; i++ {
		kp, err := GenerateMLDSA65KeyPair()
		if err != nil {
			t.Fatalf("Key generation %d failed: %v", i, err)
		}
		keyPairs[i] = kp
	}

	// Check all pairs are unique
	for i := 0; i < numPairs; i++ {
		for j := i + 1; j < numPairs; j++ {
			if bytes.Equal(keyPairs[i].PublicKey, keyPairs[j].PublicKey) {
				t.Errorf("Key pairs %d and %d have same public key", i, j)
			}
			if bytes.Equal(keyPairs[i].PrivateKey, keyPairs[j].PrivateKey) {
				t.Errorf("Key pairs %d and %d have same private key", i, j)
			}
		}
	}
}

// TestSignatureUniqueness tests that signatures are non-deterministic (if applicable)
func TestSignatureUniqueness(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	message := []byte("Test message for signature uniqueness")

	// Generate multiple signatures for the same message
	// Note: Dilithium is deterministic, so signatures should be identical
	sig1, _ := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, message)
	sig2, _ := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, message)

	// For deterministic signatures, they should be equal
	if !bytes.Equal(sig1, sig2) {
		t.Log("Signatures are different (non-deterministic mode)")
	} else {
		t.Log("Signatures are identical (deterministic mode)")
	}

	// Both should verify
	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, message, sig1); err != nil {
		t.Errorf("First signature verification failed: %v", err)
	}
	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, message, sig2); err != nil {
		t.Errorf("Second signature verification failed: %v", err)
	}
}

// TestLargeMessage tests signing and verification of large messages
func TestLargeMessage(t *testing.T) {
	keyPair, err := GenerateMLDSA65KeyPair()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	// Generate a large message (1MB)
	largeMessage := make([]byte, 1024*1024)
	if _, err := rand.Read(largeMessage); err != nil {
		t.Fatalf("Failed to generate random message: %v", err)
	}

	signature, err := SignMLDSA(AlgorithmMLDSA65, keyPair.PrivateKey, largeMessage)
	if err != nil {
		t.Fatalf("Signing large message failed: %v", err)
	}

	if err := VerifyMLDSA(AlgorithmMLDSA65, keyPair.PublicKey, largeMessage, signature); err != nil {
		t.Errorf("Verification of large message failed: %v", err)
	}
}

// BenchmarkMLDSAKeyGeneration benchmarks key generation
func BenchmarkMLDSAKeyGeneration(b *testing.B) {
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		b.Run(string(alg), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := GenerateMLDSAKeyPair(alg)
				if err != nil {
					b.Fatalf("Key generation failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMLDSASigning benchmarks signing operations
func BenchmarkMLDSASigning(b *testing.B) {
	message := []byte("Benchmark message for ML-DSA signing performance")

	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		keyPair, _ := GenerateMLDSAKeyPair(alg)

		b.Run(string(alg), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SignMLDSA(alg, keyPair.PrivateKey, message)
				if err != nil {
					b.Fatalf("Signing failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMLDSAVerification benchmarks verification operations
func BenchmarkMLDSAVerification(b *testing.B) {
	message := []byte("Benchmark message for ML-DSA verification performance")

	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		keyPair, _ := GenerateMLDSAKeyPair(alg)
		signature, _ := SignMLDSA(alg, keyPair.PrivateKey, message)

		b.Run(string(alg), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := VerifyMLDSA(alg, keyPair.PublicKey, message, signature)
				if err != nil {
					b.Fatalf("Verification failed: %v", err)
				}
			}
		})
	}
}

// TestGetActualSignatureSize tests signature size retrieval from circl library
func TestGetActualSignatureSize(t *testing.T) {
	tests := []struct {
		alg      Algorithm
		expected int
	}{
		{AlgorithmMLDSA44, MLDSA44SignatureSize},
		{AlgorithmMLDSA65, MLDSA65SignatureSize},
		{AlgorithmMLDSA87, MLDSA87SignatureSize},
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			size, err := GetActualSignatureSize(tt.alg)
			if err != nil {
				t.Fatalf("GetActualSignatureSize failed: %v", err)
			}
			if size != tt.expected {
				t.Errorf("Size mismatch: got %d, want %d", size, tt.expected)
			}
		})
	}
}

// TestGetActualSignatureSizeUnsupported tests error for unsupported algorithm
func TestGetActualSignatureSizeUnsupported(t *testing.T) {
	_, err := GetActualSignatureSize(AlgorithmEd25519)
	if err == nil {
		t.Error("Should fail for non-ML-DSA algorithm")
	}
}

// TestSignMLDSAAllVariantsInvalidKey tests signing with invalid key for all variants
func TestSignMLDSAAllVariantsInvalidKey(t *testing.T) {
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}
	invalidKey := []byte("invalid private key too short")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			_, err := SignMLDSA(alg, invalidKey, []byte("message"))
			if err == nil {
				t.Errorf("Should fail with invalid private key for %s", alg)
			}
		})
	}
}

// TestSignMLDSAUnsupportedAlgorithm tests signing with unsupported algorithm
func TestSignMLDSAUnsupportedAlgorithm(t *testing.T) {
	_, err := SignMLDSA(AlgorithmEd25519, []byte("key"), []byte("message"))
	if err == nil {
		t.Error("Should fail with unsupported algorithm")
	}
}

// TestVerifyMLDSAUnsupportedAlgorithm tests verification with unsupported algorithm
func TestVerifyMLDSAUnsupportedAlgorithm(t *testing.T) {
	err := VerifyMLDSA(AlgorithmEd25519, []byte("key"), []byte("message"), []byte("sig"))
	if err == nil {
		t.Error("Should fail with unsupported algorithm")
	}
}

// TestVerifyMLDSAAllVariantsInvalidKey tests verification with invalid key for all variants
func TestVerifyMLDSAAllVariantsInvalidKey(t *testing.T) {
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			invalidKey := []byte("short")
			sigSize, _ := GetExpectedSignatureSize(alg)
			err := VerifyMLDSA(alg, invalidKey, []byte("message"), make([]byte, sigSize))
			if err == nil {
				t.Errorf("Should fail with invalid public key for %s", alg)
			}
		})
	}
}

// TestDecodeMLDSAPrivateKeyAllVariants tests decoding private key for all variants
func TestDecodeMLDSAPrivateKeyAllVariants(t *testing.T) {
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			keyPair, err := GenerateMLDSAKeyPair(alg)
			if err != nil {
				t.Fatalf("Key generation failed: %v", err)
			}

			encoded := base64.StdEncoding.EncodeToString(keyPair.PrivateKey)
			decoded, err := DecodeMLDSAPrivateKey(alg, encoded)
			if err != nil {
				t.Fatalf("DecodeMLDSAPrivateKey failed: %v", err)
			}

			if !bytes.Equal(decoded, keyPair.PrivateKey) {
				t.Error("Decoded private key doesn't match original")
			}
		})
	}
}

// TestDecodeMLDSAPrivateKeyUnsupportedAlgorithm tests error for unsupported algorithm
func TestDecodeMLDSAPrivateKeyUnsupportedAlgorithm(t *testing.T) {
	validBase64 := base64.StdEncoding.EncodeToString(make([]byte, 100))
	_, err := DecodeMLDSAPrivateKey(AlgorithmEd25519, validBase64)
	if err == nil {
		t.Error("Should fail with unsupported algorithm")
	}
}

// TestDecodeMLDSAPublicKeyUnsupportedAlgorithm tests error for unsupported algorithm
func TestDecodeMLDSAPublicKeyUnsupportedAlgorithm(t *testing.T) {
	validBase64 := base64.StdEncoding.EncodeToString(make([]byte, 100))
	_, err := DecodeMLDSAPublicKey(Algorithm("INVALID"), validBase64)
	if err == nil {
		t.Error("Should fail with unsupported algorithm")
	}
}

// TestGetExpectedPublicKeySize tests public key size retrieval
func TestGetExpectedPublicKeySize(t *testing.T) {
	tests := []struct {
		alg      Algorithm
		expected int
	}{
		{AlgorithmEd25519, Ed25519PublicKeySize},
		{AlgorithmMLDSA44, MLDSA44PublicKeySize},
		{AlgorithmMLDSA65, MLDSA65PublicKeySize},
		{AlgorithmMLDSA87, MLDSA87PublicKeySize},
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			size, err := GetExpectedPublicKeySize(tt.alg)
			if err != nil {
				t.Fatalf("GetExpectedPublicKeySize failed: %v", err)
			}
			if size != tt.expected {
				t.Errorf("Size mismatch: got %d, want %d", size, tt.expected)
			}
		})
	}
}

// TestGetExpectedPublicKeySizeUnsupported tests error for unsupported algorithm
func TestGetExpectedPublicKeySizeUnsupported(t *testing.T) {
	_, err := GetExpectedPublicKeySize(Algorithm("INVALID"))
	if err == nil {
		t.Error("Should fail for invalid algorithm")
	}
}

// TestGetExpectedSignatureSize tests signature size retrieval
func TestGetExpectedSignatureSize(t *testing.T) {
	tests := []struct {
		alg      Algorithm
		expected int
	}{
		{AlgorithmEd25519, Ed25519SignatureSize},
		{AlgorithmMLDSA44, MLDSA44SignatureSize},
		{AlgorithmMLDSA65, MLDSA65SignatureSize},
		{AlgorithmMLDSA87, MLDSA87SignatureSize},
	}

	for _, tt := range tests {
		t.Run(string(tt.alg), func(t *testing.T) {
			size, err := GetExpectedSignatureSize(tt.alg)
			if err != nil {
				t.Fatalf("GetExpectedSignatureSize failed: %v", err)
			}
			if size != tt.expected {
				t.Errorf("Size mismatch: got %d, want %d", size, tt.expected)
			}
		})
	}
}

// TestGetExpectedSignatureSizeUnsupported tests error for unsupported algorithm
func TestGetExpectedSignatureSizeUnsupported(t *testing.T) {
	_, err := GetExpectedSignatureSize(Algorithm("INVALID"))
	if err == nil {
		t.Error("Should fail for invalid algorithm")
	}
}
