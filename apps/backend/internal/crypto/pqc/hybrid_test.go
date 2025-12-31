package pqc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestGenerateHybridKeyPair tests hybrid key pair generation for all variants
func TestGenerateHybridKeyPair(t *testing.T) {
	variants := []struct {
		mldsaVariant   Algorithm
		expectedHybrid Algorithm
	}{
		{AlgorithmMLDSA44, AlgorithmHybridEd25519MLDSA44},
		{AlgorithmMLDSA65, AlgorithmHybridEd25519MLDSA65},
		{AlgorithmMLDSA87, AlgorithmHybridEd25519MLDSA87},
	}

	for _, v := range variants {
		t.Run(string(v.mldsaVariant), func(t *testing.T) {
			keys, err := GenerateHybridKeyPair(v.mldsaVariant)
			if err != nil {
				t.Fatalf("GenerateHybridKeyPair failed: %v", err)
			}

			// Check hybrid algorithm
			if keys.Algorithm != v.expectedHybrid {
				t.Errorf("Algorithm: got %s, want %s", keys.Algorithm, v.expectedHybrid)
			}

			// Check Ed25519 keys
			if len(keys.Ed25519.PublicKey) != Ed25519PublicKeySize {
				t.Errorf("Ed25519 public key size: got %d, want %d", len(keys.Ed25519.PublicKey), Ed25519PublicKeySize)
			}
			if len(keys.Ed25519.PrivateKey) != Ed25519PrivateKeySize {
				t.Errorf("Ed25519 private key size: got %d, want %d", len(keys.Ed25519.PrivateKey), Ed25519PrivateKeySize)
			}

			// Check ML-DSA keys
			expectedPKSize, _ := GetExpectedPublicKeySize(v.mldsaVariant)
			if len(keys.MLDSA.PublicKey) != expectedPKSize {
				t.Errorf("ML-DSA public key size: got %d, want %d", len(keys.MLDSA.PublicKey), expectedPKSize)
			}

			// Check MLDSA algorithm
			if keys.MLDSA.Algorithm != v.mldsaVariant {
				t.Errorf("MLDSA algorithm: got %s, want %s", keys.MLDSA.Algorithm, v.mldsaVariant)
			}

			if keys.CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}
		})
	}
}

// TestGenerateHybridKeyPairUnsupported tests error handling for unsupported variants
func TestGenerateHybridKeyPairUnsupported(t *testing.T) {
	_, err := GenerateHybridKeyPair(AlgorithmEd25519)
	if err == nil {
		t.Error("Expected error for unsupported algorithm")
	}
}

// TestGenerateHybridKeyPair65 tests the convenience function
func TestGenerateHybridKeyPair65(t *testing.T) {
	keys, err := GenerateHybridKeyPair65()
	if err != nil {
		t.Fatalf("GenerateHybridKeyPair65 failed: %v", err)
	}

	if keys.Algorithm != AlgorithmHybridEd25519MLDSA65 {
		t.Errorf("Expected Ed25519+ML-DSA-65 algorithm, got %s", keys.Algorithm)
	}
}

// TestHybridSignVerify tests the sign/verify round-trip for hybrid signatures
func TestHybridSignVerify(t *testing.T) {
	variants := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, variant := range variants {
		t.Run(string(variant), func(t *testing.T) {
			keys, err := GenerateHybridKeyPair(variant)
			if err != nil {
				t.Fatalf("Key generation failed: %v", err)
			}

			message := []byte("Test message for hybrid signature verification")

			// Sign
			sig, err := HybridSign(keys, message)
			if err != nil {
				t.Fatalf("Signing failed: %v", err)
			}

			// Check signature components
			if len(sig.Ed25519) != Ed25519SignatureSize {
				t.Errorf("Ed25519 signature size: got %d, want %d", len(sig.Ed25519), Ed25519SignatureSize)
			}

			expectedSigSize, _ := GetExpectedSignatureSize(variant)
			if len(sig.MLDSA) != expectedSigSize {
				t.Errorf("ML-DSA signature size: got %d, want %d", len(sig.MLDSA), expectedSigSize)
			}

			// Verify
			pubKey := GetHybridPublicKey(keys)
			err = HybridVerify(pubKey, message, sig)
			if err != nil {
				t.Errorf("Verification failed: %v", err)
			}

			// Also test boolean version
			if !HybridVerifyBool(pubKey, message, sig) {
				t.Error("HybridVerifyBool returned false for valid signature")
			}
		})
	}
}

// TestHybridVerifyBothSignaturesRequired tests that BOTH signatures must be valid
func TestHybridVerifyBothSignaturesRequired(t *testing.T) {
	keys, err := GenerateHybridKeyPair65()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	message := []byte("Test message")
	sig, err := HybridSign(keys, message)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	pubKey := GetHybridPublicKey(keys)

	// Valid signature should pass
	if err := HybridVerify(pubKey, message, sig); err != nil {
		t.Errorf("Valid signature should pass: %v", err)
	}

	// Test with tampered Ed25519 signature
	t.Run("TamperedEd25519", func(t *testing.T) {
		tamperedSig := &HybridSignature{
			Ed25519:   make([]byte, len(sig.Ed25519)),
			MLDSA:     sig.MLDSA,
			Algorithm: sig.Algorithm,
			Timestamp: sig.Timestamp,
		}
		copy(tamperedSig.Ed25519, sig.Ed25519)
		tamperedSig.Ed25519[0] ^= 0x01

		err := HybridVerify(pubKey, message, tamperedSig)
		if err != ErrEd25519SignatureInvalid {
			t.Errorf("Expected ErrEd25519SignatureInvalid, got: %v", err)
		}
	})

	// Test with tampered ML-DSA signature
	t.Run("TamperedMLDSA", func(t *testing.T) {
		tamperedSig := &HybridSignature{
			Ed25519:   sig.Ed25519,
			MLDSA:     make([]byte, len(sig.MLDSA)),
			Algorithm: sig.Algorithm,
			Timestamp: sig.Timestamp,
		}
		copy(tamperedSig.MLDSA, sig.MLDSA)
		tamperedSig.MLDSA[0] ^= 0x01

		err := HybridVerify(pubKey, message, tamperedSig)
		if err != ErrMLDSASignatureInvalid {
			t.Errorf("Expected ErrMLDSASignatureInvalid, got: %v", err)
		}
	})

	// Test with both signatures tampered
	t.Run("BothTampered", func(t *testing.T) {
		tamperedSig := &HybridSignature{
			Ed25519:   make([]byte, len(sig.Ed25519)),
			MLDSA:     make([]byte, len(sig.MLDSA)),
			Algorithm: sig.Algorithm,
			Timestamp: sig.Timestamp,
		}
		copy(tamperedSig.Ed25519, sig.Ed25519)
		copy(tamperedSig.MLDSA, sig.MLDSA)
		tamperedSig.Ed25519[0] ^= 0x01
		tamperedSig.MLDSA[0] ^= 0x01

		// Ed25519 is verified first, so should fail with Ed25519 error
		err := HybridVerify(pubKey, message, tamperedSig)
		if err != ErrEd25519SignatureInvalid {
			t.Errorf("Expected ErrEd25519SignatureInvalid, got: %v", err)
		}
	})
}

// TestHybridVerifyTamperedMessage tests that verification fails for tampered messages
func TestHybridVerifyTamperedMessage(t *testing.T) {
	keys, err := GenerateHybridKeyPair65()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	originalMessage := []byte("Original message")
	sig, err := HybridSign(keys, originalMessage)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	pubKey := GetHybridPublicKey(keys)
	tamperedMessage := []byte("Tampered message")

	if err := HybridVerify(pubKey, tamperedMessage, sig); err == nil {
		t.Error("Verification should fail for tampered message")
	}
}

// TestHybridVerifyWrongKeys tests verification with wrong keys
func TestHybridVerifyWrongKeys(t *testing.T) {
	keys1, _ := GenerateHybridKeyPair65()
	keys2, _ := GenerateHybridKeyPair65()

	message := []byte("Test message")
	sig, _ := HybridSign(keys1, message)

	// Try to verify with keys2's public key
	pubKey2 := GetHybridPublicKey(keys2)

	if err := HybridVerify(pubKey2, message, sig); err == nil {
		t.Error("Verification should fail with wrong public key")
	}
}

// TestHybridVerifyMissingSignatures tests error handling for missing signatures
func TestHybridVerifyMissingSignatures(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	pubKey := GetHybridPublicKey(keys)
	message := []byte("Test message")

	// Missing Ed25519 signature
	t.Run("MissingEd25519", func(t *testing.T) {
		sig := &HybridSignature{
			Ed25519:   nil,
			MLDSA:     make([]byte, MLDSA65SignatureSize),
			Algorithm: AlgorithmHybridEd25519MLDSA65,
		}
		if err := HybridVerify(pubKey, message, sig); err != ErrMissingEd25519Signature {
			t.Errorf("Expected ErrMissingEd25519Signature, got: %v", err)
		}
	})

	// Missing ML-DSA signature
	t.Run("MissingMLDSA", func(t *testing.T) {
		sig := &HybridSignature{
			Ed25519:   make([]byte, Ed25519SignatureSize),
			MLDSA:     nil,
			Algorithm: AlgorithmHybridEd25519MLDSA65,
		}
		if err := HybridVerify(pubKey, message, sig); err != ErrMissingMLDSASignature {
			t.Errorf("Expected ErrMissingMLDSASignature, got: %v", err)
		}
	})
}

// TestHybridVerifyMissingPublicKeys tests error handling for missing public keys
func TestHybridVerifyMissingPublicKeys(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	message := []byte("Test message")
	sig, _ := HybridSign(keys, message)

	// Missing Ed25519 public key
	t.Run("MissingEd25519PubKey", func(t *testing.T) {
		pubKey := &HybridPublicKey{
			Ed25519:      nil,
			MLDSA:        keys.MLDSA.PublicKey,
			MLDSAVariant: AlgorithmMLDSA65,
		}
		if err := HybridVerify(pubKey, message, sig); err != ErrMissingEd25519PublicKey {
			t.Errorf("Expected ErrMissingEd25519PublicKey, got: %v", err)
		}
	})

	// Missing ML-DSA public key
	t.Run("MissingMLDSAPubKey", func(t *testing.T) {
		pubKey := &HybridPublicKey{
			Ed25519:      keys.Ed25519.PublicKey,
			MLDSA:        nil,
			MLDSAVariant: AlgorithmMLDSA65,
		}
		if err := HybridVerify(pubKey, message, sig); err != ErrMissingMLDSAPublicKey {
			t.Errorf("Expected ErrMissingMLDSAPublicKey, got: %v", err)
		}
	})
}

// TestHybridVerifyWithSeparateKeys tests verification with separate key parameters
func TestHybridVerifyWithSeparateKeys(t *testing.T) {
	keys, err := GenerateHybridKeyPair65()
	if err != nil {
		t.Fatalf("Key generation failed: %v", err)
	}

	message := []byte("Test message")
	sig, _ := HybridSign(keys, message)

	err = HybridVerifyWithSeparateKeys(
		keys.Ed25519.PublicKey,
		keys.MLDSA.PublicKey,
		message,
		sig,
	)
	if err != nil {
		t.Errorf("HybridVerifyWithSeparateKeys failed: %v", err)
	}
}

// TestEncodeDecodeHybridKeyPair tests encoding/decoding of hybrid key pairs
func TestEncodeDecodeHybridKeyPair(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()

	encoded := EncodeHybridKeyPair(keys)

	if encoded.Algorithm != keys.Algorithm {
		t.Errorf("Algorithm mismatch")
	}
	if encoded.MLDSAVariant != keys.MLDSA.Algorithm {
		t.Errorf("MLDSA variant mismatch")
	}

	// Verify public keys can be decoded
	encodedPubOnly := EncodeHybridPublicKeys(keys)
	decoded, err := DecodeHybridPublicKey(encodedPubOnly)
	if err != nil {
		t.Fatalf("DecodeHybridPublicKey failed: %v", err)
	}

	if !bytes.Equal(decoded.Ed25519, keys.Ed25519.PublicKey) {
		t.Error("Decoded Ed25519 public key doesn't match")
	}
	if !bytes.Equal(decoded.MLDSA, keys.MLDSA.PublicKey) {
		t.Error("Decoded ML-DSA public key doesn't match")
	}
}

// TestEncodeDecodeHybridSignature tests encoding/decoding of hybrid signatures
func TestEncodeDecodeHybridSignature(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	message := []byte("Test message")
	sig, _ := HybridSign(keys, message)

	// Encode
	encoded := EncodeHybridSignature(sig)

	// Decode
	decoded, err := DecodeHybridSignature(encoded)
	if err != nil {
		t.Fatalf("DecodeHybridSignature failed: %v", err)
	}

	if !bytes.Equal(decoded.Ed25519, sig.Ed25519) {
		t.Error("Decoded Ed25519 signature doesn't match")
	}
	if !bytes.Equal(decoded.MLDSA, sig.MLDSA) {
		t.Error("Decoded ML-DSA signature doesn't match")
	}
	if decoded.Algorithm != sig.Algorithm {
		t.Error("Decoded algorithm doesn't match")
	}

	// Verify decoded signature works
	pubKey := GetHybridPublicKey(keys)
	if err := HybridVerify(pubKey, message, decoded); err != nil {
		t.Errorf("Decoded signature verification failed: %v", err)
	}
}

// TestMarshalUnmarshalHybridSignature tests JSON marshaling/unmarshaling
func TestMarshalUnmarshalHybridSignature(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	message := []byte("Test message")
	sig, _ := HybridSign(keys, message)

	// Marshal to JSON
	data, err := MarshalHybridSignature(sig)
	if err != nil {
		t.Fatalf("MarshalHybridSignature failed: %v", err)
	}

	// Verify it's valid JSON
	var jsonCheck map[string]interface{}
	if err := json.Unmarshal(data, &jsonCheck); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Unmarshal
	unmarshaled, err := UnmarshalHybridSignature(data)
	if err != nil {
		t.Fatalf("UnmarshalHybridSignature failed: %v", err)
	}

	// Verify signature still works
	pubKey := GetHybridPublicKey(keys)
	if err := HybridVerify(pubKey, message, unmarshaled); err != nil {
		t.Errorf("Unmarshaled signature verification failed: %v", err)
	}
}

// TestValidateHybridKeyPair tests key pair validation
func TestValidateHybridKeyPair(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()

	// Valid key pair should pass
	if err := ValidateHybridKeyPair(keys); err != nil {
		t.Errorf("Valid key pair should pass validation: %v", err)
	}

	// Nil key pair
	if err := ValidateHybridKeyPair(nil); err == nil {
		t.Error("Nil key pair should fail validation")
	}

	// Missing Ed25519
	invalidKeys := &HybridKeyPair{
		Ed25519:   nil,
		MLDSA:     keys.MLDSA,
		Algorithm: keys.Algorithm,
	}
	if err := ValidateHybridKeyPair(invalidKeys); err == nil {
		t.Error("Missing Ed25519 should fail validation")
	}

	// Missing MLDSA
	invalidKeys = &HybridKeyPair{
		Ed25519:   keys.Ed25519,
		MLDSA:     nil,
		Algorithm: keys.Algorithm,
	}
	if err := ValidateHybridKeyPair(invalidKeys); err == nil {
		t.Error("Missing MLDSA should fail validation")
	}

	// Wrong Ed25519 public key size
	invalidKeys = &HybridKeyPair{
		Ed25519: &KeyPair{
			Algorithm:  AlgorithmEd25519,
			PublicKey:  []byte("short"),
			PrivateKey: keys.Ed25519.PrivateKey,
		},
		MLDSA:     keys.MLDSA,
		Algorithm: keys.Algorithm,
	}
	if err := ValidateHybridKeyPair(invalidKeys); err == nil {
		t.Error("Invalid Ed25519 public key size should fail validation")
	}
}

// TestConstantTimeCompare tests constant-time comparison
func TestConstantTimeCompare(t *testing.T) {
	a := []byte("test data")
	b := []byte("test data")
	c := []byte("different")

	if !ConstantTimeCompare(a, b) {
		t.Error("Same data should compare equal")
	}

	if ConstantTimeCompare(a, c) {
		t.Error("Different data should not compare equal")
	}
}

// TestHybridSignNilKeys tests error handling for nil keys
func TestHybridSignNilKeys(t *testing.T) {
	message := []byte("Test message")

	// Nil key pair
	_, err := HybridSign(nil, message)
	if err == nil {
		t.Error("Signing with nil keys should fail")
	}

	// Incomplete key pair
	incompleteKeys := &HybridKeyPair{
		Ed25519: nil,
		MLDSA:   nil,
	}
	_, err = HybridSign(incompleteKeys, message)
	if err == nil {
		t.Error("Signing with incomplete keys should fail")
	}
}

// TestHybridMultipleMessages tests signing multiple different messages
func TestHybridMultipleMessages(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	pubKey := GetHybridPublicKey(keys)

	messages := []string{
		"First message",
		"Second message with different content",
		"A much longer message that contains more data",
		"", // Empty message
	}

	for i, msg := range messages {
		sig, err := HybridSign(keys, []byte(msg))
		if err != nil {
			t.Fatalf("Signing message %d failed: %v", i, err)
		}

		if err := HybridVerify(pubKey, []byte(msg), sig); err != nil {
			t.Errorf("Verification failed for message %d: %v", i, err)
		}
	}
}

// TestHybridLargeMessage tests signing large messages
func TestHybridLargeMessage(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()

	// 1MB message
	largeMessage := make([]byte, 1024*1024)
	if _, err := rand.Read(largeMessage); err != nil {
		t.Fatalf("Failed to generate random message: %v", err)
	}

	sig, err := HybridSign(keys, largeMessage)
	if err != nil {
		t.Fatalf("Signing large message failed: %v", err)
	}

	pubKey := GetHybridPublicKey(keys)
	if err := HybridVerify(pubKey, largeMessage, sig); err != nil {
		t.Errorf("Verification of large message failed: %v", err)
	}
}

// TestInteroperabilityWithStandardEd25519 tests that the Ed25519 component is standard-compliant
func TestInteroperabilityWithStandardEd25519(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	message := []byte("Test message for Ed25519 interoperability")

	sig, _ := HybridSign(keys, message)

	// Verify Ed25519 component using standard library
	ed25519PubKey := ed25519.PublicKey(keys.Ed25519.PublicKey)
	if !ed25519.Verify(ed25519PubKey, message, sig.Ed25519) {
		t.Error("Ed25519 component should be verifiable with standard library")
	}
}

// BenchmarkHybridKeyGeneration benchmarks hybrid key generation
func BenchmarkHybridKeyGeneration(b *testing.B) {
	variants := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, variant := range variants {
		b.Run(string(variant), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := GenerateHybridKeyPair(variant)
				if err != nil {
					b.Fatalf("Key generation failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkHybridSigning benchmarks hybrid signing
func BenchmarkHybridSigning(b *testing.B) {
	message := []byte("Benchmark message for hybrid signing performance")

	variants := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, variant := range variants {
		keys, _ := GenerateHybridKeyPair(variant)

		b.Run(string(variant), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := HybridSign(keys, message)
				if err != nil {
					b.Fatalf("Signing failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkHybridVerification benchmarks hybrid verification
func BenchmarkHybridVerification(b *testing.B) {
	message := []byte("Benchmark message for hybrid verification performance")

	variants := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, variant := range variants {
		keys, _ := GenerateHybridKeyPair(variant)
		pubKey := GetHybridPublicKey(keys)
		sig, _ := HybridSign(keys, message)

		b.Run(string(variant), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := HybridVerify(pubKey, message, sig)
				if err != nil {
					b.Fatalf("Verification failed: %v", err)
				}
			}
		})
	}
}

// TestDecodeHybridPublicKeyInvalidBase64 tests decoding with invalid base64
func TestDecodeHybridPublicKeyInvalidBase64(t *testing.T) {
	// Invalid Ed25519 base64
	encoded := &EncodedHybridPublicKey{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519PublicKey: "not-valid-base64!!!",
		MLDSAPublicKey:   "dGVzdA==",
		MLDSAVariant:     AlgorithmMLDSA65,
	}
	_, err := DecodeHybridPublicKey(encoded)
	if err == nil {
		t.Error("Should fail with invalid Ed25519 base64")
	}

	// Invalid MLDSA base64 (with valid Ed25519)
	keys, _ := GenerateHybridKeyPair65()
	encodedPub := EncodeHybridPublicKeys(keys)
	encodedPub.MLDSAPublicKey = "not-valid-base64!!!"
	_, err = DecodeHybridPublicKey(encodedPub)
	if err == nil {
		t.Error("Should fail with invalid MLDSA base64")
	}
}

// TestDecodeHybridPublicKeyInvalidSize tests decoding with invalid key sizes
func TestDecodeHybridPublicKeyInvalidSize(t *testing.T) {
	import64 := func(data []byte) string {
		return base64.StdEncoding.EncodeToString(data)
	}

	// Invalid Ed25519 size
	encoded := &EncodedHybridPublicKey{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519PublicKey: import64([]byte("short")),
		MLDSAPublicKey:   import64(make([]byte, MLDSA65PublicKeySize)),
		MLDSAVariant:     AlgorithmMLDSA65,
	}
	_, err := DecodeHybridPublicKey(encoded)
	if err == nil {
		t.Error("Should fail with invalid Ed25519 key size")
	}

	// Invalid MLDSA size
	encoded = &EncodedHybridPublicKey{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519PublicKey: import64(make([]byte, Ed25519PublicKeySize)),
		MLDSAPublicKey:   import64([]byte("short")),
		MLDSAVariant:     AlgorithmMLDSA65,
	}
	_, err = DecodeHybridPublicKey(encoded)
	if err == nil {
		t.Error("Should fail with invalid MLDSA key size")
	}
}

// TestDecodeHybridPublicKeyUnsupportedVariant tests decoding with unsupported variant
func TestDecodeHybridPublicKeyUnsupportedVariant(t *testing.T) {
	import64 := func(data []byte) string {
		return base64.StdEncoding.EncodeToString(data)
	}

	encoded := &EncodedHybridPublicKey{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519PublicKey: import64(make([]byte, Ed25519PublicKeySize)),
		MLDSAPublicKey:   import64(make([]byte, 100)),
		MLDSAVariant:     Algorithm("INVALID"),
	}
	_, err := DecodeHybridPublicKey(encoded)
	if err == nil {
		t.Error("Should fail with unsupported MLDSA variant")
	}
}

// TestDecodeHybridSignatureInvalidBase64 tests decoding signature with invalid base64
func TestDecodeHybridSignatureInvalidBase64(t *testing.T) {
	// Invalid Ed25519 signature base64
	encoded := &EncodedHybridSignature{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519Signature: "not-valid-base64!!!",
		MLDSASignature:   "dGVzdA==",
		Timestamp:        1234567890,
	}
	_, err := DecodeHybridSignature(encoded)
	if err == nil {
		t.Error("Should fail with invalid Ed25519 signature base64")
	}

	// Invalid MLDSA signature base64
	encoded = &EncodedHybridSignature{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519Signature: base64.StdEncoding.EncodeToString(make([]byte, Ed25519SignatureSize)),
		MLDSASignature:   "not-valid-base64!!!",
		Timestamp:        1234567890,
	}
	_, err = DecodeHybridSignature(encoded)
	if err == nil {
		t.Error("Should fail with invalid MLDSA signature base64")
	}
}

// TestDecodeHybridSignatureInvalidSize tests decoding signature with invalid sizes
func TestDecodeHybridSignatureInvalidSize(t *testing.T) {
	import64 := func(data []byte) string {
		return base64.StdEncoding.EncodeToString(data)
	}

	// Invalid Ed25519 signature size
	encoded := &EncodedHybridSignature{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519Signature: import64([]byte("short")),
		MLDSASignature:   import64(make([]byte, MLDSA65SignatureSize)),
		Timestamp:        1234567890,
	}
	_, err := DecodeHybridSignature(encoded)
	if err == nil {
		t.Error("Should fail with invalid Ed25519 signature size")
	}

	// Invalid MLDSA signature size
	encoded = &EncodedHybridSignature{
		Algorithm:        AlgorithmHybridEd25519MLDSA65,
		Ed25519Signature: import64(make([]byte, Ed25519SignatureSize)),
		MLDSASignature:   import64([]byte("short")),
		Timestamp:        1234567890,
	}
	_, err = DecodeHybridSignature(encoded)
	if err == nil {
		t.Error("Should fail with invalid MLDSA signature size")
	}
}

// TestDecodeHybridSignatureUnsupportedAlgorithm tests decoding with unsupported algorithm
func TestDecodeHybridSignatureUnsupportedAlgorithm(t *testing.T) {
	import64 := func(data []byte) string {
		return base64.StdEncoding.EncodeToString(data)
	}

	encoded := &EncodedHybridSignature{
		Algorithm:        Algorithm("INVALID"),
		Ed25519Signature: import64(make([]byte, Ed25519SignatureSize)),
		MLDSASignature:   import64(make([]byte, 100)),
		Timestamp:        1234567890,
	}
	_, err := DecodeHybridSignature(encoded)
	if err == nil {
		t.Error("Should fail with unsupported algorithm")
	}
}

// TestUnmarshalHybridSignatureInvalidJSON tests unmarshaling invalid JSON
func TestUnmarshalHybridSignatureInvalidJSON(t *testing.T) {
	_, err := UnmarshalHybridSignature([]byte("not valid json"))
	if err == nil {
		t.Error("Should fail with invalid JSON")
	}
}

// TestHybridVerifyNilInputs tests error handling for nil inputs
func TestHybridVerifyNilInputs(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()
	pubKey := GetHybridPublicKey(keys)
	message := []byte("test")
	sig, _ := HybridSign(keys, message)

	// Nil public key
	if err := HybridVerify(nil, message, sig); err != ErrHybridSignatureInvalid {
		t.Errorf("Expected ErrHybridSignatureInvalid for nil public key, got: %v", err)
	}

	// Nil signature
	if err := HybridVerify(pubKey, message, nil); err != ErrHybridSignatureInvalid {
		t.Errorf("Expected ErrHybridSignatureInvalid for nil signature, got: %v", err)
	}
}

// TestValidateHybridKeyPairMoreErrors tests additional validation errors
func TestValidateHybridKeyPairMoreErrors(t *testing.T) {
	keys, _ := GenerateHybridKeyPair65()

	// Wrong Ed25519 private key size
	t.Run("InvalidEd25519PrivateKeySize", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519: &KeyPair{
				Algorithm:  AlgorithmEd25519,
				PublicKey:  keys.Ed25519.PublicKey,
				PrivateKey: []byte("short"),
			},
			MLDSA:     keys.MLDSA,
			Algorithm: keys.Algorithm,
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with invalid Ed25519 private key size")
		}
	})

	// Wrong MLDSA public key size
	t.Run("InvalidMLDSAPublicKeySize", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519: keys.Ed25519,
			MLDSA: &KeyPair{
				Algorithm:  AlgorithmMLDSA65,
				PublicKey:  []byte("short"),
				PrivateKey: keys.MLDSA.PrivateKey,
			},
			Algorithm: keys.Algorithm,
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with invalid MLDSA public key size")
		}
	})

	// Wrong MLDSA private key size
	t.Run("InvalidMLDSAPrivateKeySize", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519: keys.Ed25519,
			MLDSA: &KeyPair{
				Algorithm:  AlgorithmMLDSA65,
				PublicKey:  keys.MLDSA.PublicKey,
				PrivateKey: []byte("short"),
			},
			Algorithm: keys.Algorithm,
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with invalid MLDSA private key size")
		}
	})

	// Non-hybrid algorithm
	t.Run("NonHybridAlgorithm", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519:   keys.Ed25519,
			MLDSA:     keys.MLDSA,
			Algorithm: AlgorithmMLDSA65, // Not a hybrid algorithm
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with non-hybrid algorithm")
		}
	})

	// MLDSA variant mismatch
	t.Run("MLDSAVariantMismatch", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519: keys.Ed25519,
			MLDSA: &KeyPair{
				Algorithm:  AlgorithmMLDSA44, // Wrong variant
				PublicKey:  keys.MLDSA.PublicKey,
				PrivateKey: keys.MLDSA.PrivateKey,
			},
			Algorithm: AlgorithmHybridEd25519MLDSA65,
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with MLDSA variant mismatch")
		}
	})

	// Invalid MLDSA algorithm for GetMLDSAKeyInfo
	t.Run("InvalidMLDSAAlgorithmForInfo", func(t *testing.T) {
		invalid := &HybridKeyPair{
			Ed25519: keys.Ed25519,
			MLDSA: &KeyPair{
				Algorithm:  Algorithm("INVALID"),
				PublicKey:  keys.MLDSA.PublicKey,
				PrivateKey: keys.MLDSA.PrivateKey,
			},
			Algorithm: keys.Algorithm,
		}
		if err := ValidateHybridKeyPair(invalid); err == nil {
			t.Error("Should fail with invalid MLDSA algorithm")
		}
	})
}

// TestGetMLDSAVariant tests ML-DSA variant extraction from hybrid algorithms
func TestGetMLDSAVariant(t *testing.T) {
	tests := []struct {
		input    Algorithm
		expected Algorithm
	}{
		{AlgorithmHybridEd25519MLDSA44, AlgorithmMLDSA44},
		{AlgorithmHybridEd25519MLDSA65, AlgorithmMLDSA65},
		{AlgorithmHybridEd25519MLDSA87, AlgorithmMLDSA87},
		{AlgorithmMLDSA65, AlgorithmMLDSA65}, // Non-hybrid returns itself
		{AlgorithmEd25519, AlgorithmEd25519}, // Non-hybrid returns itself
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := GetMLDSAVariant(tt.input)
			if result != tt.expected {
				t.Errorf("GetMLDSAVariant(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsValidAlgorithm tests algorithm validation
func TestIsValidAlgorithm(t *testing.T) {
	validAlgorithms := []Algorithm{
		AlgorithmEd25519,
		AlgorithmMLDSA44,
		AlgorithmMLDSA65,
		AlgorithmMLDSA87,
		AlgorithmHybridEd25519MLDSA44,
		AlgorithmHybridEd25519MLDSA65,
		AlgorithmHybridEd25519MLDSA87,
	}

	for _, alg := range validAlgorithms {
		if !IsValidAlgorithm(alg) {
			t.Errorf("IsValidAlgorithm(%s) = false, want true", alg)
		}
	}

	if IsValidAlgorithm(Algorithm("INVALID")) {
		t.Error("IsValidAlgorithm(INVALID) = true, want false")
	}
}

// TestIsHybridAlgorithm tests hybrid algorithm detection
func TestIsHybridAlgorithm(t *testing.T) {
	hybridAlgorithms := []Algorithm{
		AlgorithmHybridEd25519MLDSA44,
		AlgorithmHybridEd25519MLDSA65,
		AlgorithmHybridEd25519MLDSA87,
	}

	for _, alg := range hybridAlgorithms {
		if !IsHybridAlgorithm(alg) {
			t.Errorf("IsHybridAlgorithm(%s) = false, want true", alg)
		}
	}

	nonHybridAlgorithms := []Algorithm{
		AlgorithmEd25519,
		AlgorithmMLDSA44,
		AlgorithmMLDSA65,
		AlgorithmMLDSA87,
	}

	for _, alg := range nonHybridAlgorithms {
		if IsHybridAlgorithm(alg) {
			t.Errorf("IsHybridAlgorithm(%s) = true, want false", alg)
		}
	}
}

// TestIsPQCAlgorithm tests PQC algorithm detection
func TestIsPQCAlgorithm(t *testing.T) {
	pqcAlgorithms := []Algorithm{
		AlgorithmMLDSA44,
		AlgorithmMLDSA65,
		AlgorithmMLDSA87,
		AlgorithmHybridEd25519MLDSA44,
		AlgorithmHybridEd25519MLDSA65,
		AlgorithmHybridEd25519MLDSA87,
	}

	for _, alg := range pqcAlgorithms {
		if !IsPQCAlgorithm(alg) {
			t.Errorf("IsPQCAlgorithm(%s) = false, want true", alg)
		}
	}

	if IsPQCAlgorithm(AlgorithmEd25519) {
		t.Error("IsPQCAlgorithm(Ed25519) = true, want false")
	}
}

// TestSupportedAlgorithms tests the SupportedAlgorithms function
func TestSupportedAlgorithms(t *testing.T) {
	algs := SupportedAlgorithms()
	if len(algs) == 0 {
		t.Error("SupportedAlgorithms returned empty list")
	}

	// Verify the recommended algorithm is present
	foundRecommended := false
	for _, alg := range algs {
		if alg.Recommended {
			foundRecommended = true
			if alg.ID != AlgorithmHybridEd25519MLDSA65 {
				t.Errorf("Recommended algorithm should be Ed25519+ML-DSA-65, got %s", alg.ID)
			}
		}
	}

	if !foundRecommended {
		t.Error("No recommended algorithm found")
	}
}
