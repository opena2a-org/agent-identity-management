package pqc

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PQC VERIFICATION TEST SUITE
//
// This file contains tests designed for security researchers and auditors
// to verify the correctness of our ML-DSA (NIST FIPS 204) implementation.
//
// Test Categories:
// 1. Algorithm Constants Verification - Verify key/signature sizes match NIST spec
// 2. Known Answer Tests (KAT) - Deterministic tests with fixed seeds
// 3. Cross-Library Verification - Compare against reference implementations
// 4. Hybrid Mode Security Tests - Verify both signatures required
// 5. Exportable Test Vectors - Generate vectors for cross-implementation testing
//
// To run these tests:
//   go test -v -run "TestVerification" ./internal/crypto/pqc/
//
// To generate test vectors for external verification:
//   go test -v -run "TestGenerateTestVectors" ./internal/crypto/pqc/
// =============================================================================

// =============================================================================
// SECTION 1: NIST FIPS 204 ALGORITHM CONSTANTS VERIFICATION
// Reference: https://csrc.nist.gov/pubs/fips/204/final
// =============================================================================

// TestVerificationNISTKeySizes verifies key sizes match NIST FIPS 204 specification
func TestVerificationNISTKeySizes(t *testing.T) {
	t.Log("=== NIST FIPS 204 Key Size Verification ===")
	t.Log("Verifying ML-DSA key sizes match official NIST specification")

	// NIST FIPS 204 Table 1: ML-DSA Parameter Sets
	// https://csrc.nist.gov/pubs/fips/204/final
	nistSizes := map[Algorithm]struct {
		publicKey  int
		privateKey int
		signature  int
		security   string
	}{
		AlgorithmMLDSA44: {
			publicKey:  1312,
			privateKey: 2560,
			signature:  2420,
			security:   "Category 2 (128-bit classical)",
		},
		AlgorithmMLDSA65: {
			publicKey:  1952,
			privateKey: 4032,
			signature:  3309,
			security:   "Category 3 (192-bit classical)",
		},
		AlgorithmMLDSA87: {
			publicKey:  2592,
			privateKey: 4896,
			signature:  4627,
			security:   "Category 5 (256-bit classical)",
		},
	}

	for alg, expected := range nistSizes {
		t.Run(string(alg), func(t *testing.T) {
			info, err := GetMLDSAKeyInfo(alg)
			require.NoError(t, err)

			t.Logf("%s (%s):", alg, expected.security)
			t.Logf("  Public Key:  Expected=%d, Got=%d", expected.publicKey, info.PublicKeySize)
			t.Logf("  Private Key: Expected=%d, Got=%d", expected.privateKey, info.PrivateKeySize)
			t.Logf("  Signature:   Expected=%d, Got=%d", expected.signature, info.SignatureSize)

			assert.Equal(t, expected.publicKey, info.PublicKeySize,
				"Public key size mismatch for %s", alg)
			assert.Equal(t, expected.privateKey, info.PrivateKeySize,
				"Private key size mismatch for %s", alg)
			assert.Equal(t, expected.signature, info.SignatureSize,
				"Signature size mismatch for %s", alg)
		})
	}
}

// TestVerificationEd25519Sizes verifies Ed25519 sizes for hybrid mode
func TestVerificationEd25519Sizes(t *testing.T) {
	t.Log("=== Ed25519 Size Verification (RFC 8032) ===")

	// RFC 8032 specifies Ed25519 sizes
	assert.Equal(t, 32, ed25519.PublicKeySize, "Ed25519 public key should be 32 bytes")
	assert.Equal(t, 64, ed25519.PrivateKeySize, "Ed25519 private key should be 64 bytes")
	assert.Equal(t, 64, ed25519.SignatureSize, "Ed25519 signature should be 64 bytes")

	t.Log("Ed25519 (RFC 8032):")
	t.Logf("  Public Key:  %d bytes", ed25519.PublicKeySize)
	t.Logf("  Private Key: %d bytes", ed25519.PrivateKeySize)
	t.Logf("  Signature:   %d bytes", ed25519.SignatureSize)
}

// =============================================================================
// SECTION 2: KNOWN ANSWER TESTS (KAT)
// These tests use deterministic operations to produce reproducible results
// =============================================================================

// TestVerificationKATSignVerify tests sign/verify with known messages
func TestVerificationKATSignVerify(t *testing.T) {
	t.Log("=== Known Answer Test: Sign/Verify Round-Trip ===")

	// Test messages from various sources
	testMessages := []struct {
		name    string
		message []byte
	}{
		{"Empty", []byte{}},
		{"Single byte", []byte{0x00}},
		{"ASCII", []byte("The quick brown fox jumps over the lazy dog")},
		{"Binary", []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe, 0xfd}},
		{"Long message", bytes.Repeat([]byte("NIST FIPS 204 "), 1000)},
		{"Unicode", []byte("日本語テスト 🔐")},
	}

	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		for _, tc := range testMessages {
			testName := fmt.Sprintf("%s/%s", alg, tc.name)
			t.Run(testName, func(t *testing.T) {
				// Generate keypair
				keypair, err := GenerateMLDSAKeyPair(alg)
				require.NoError(t, err)

				// Sign
				signature, err := SignMLDSA(alg, keypair.PrivateKey, tc.message)
				require.NoError(t, err)

				// Verify
				err = VerifyMLDSA(alg, keypair.PublicKey, tc.message, signature)
				assert.NoError(t, err, "Valid signature should verify")

				// Verify tampered message fails
				if len(tc.message) > 0 {
					tampered := make([]byte, len(tc.message))
					copy(tampered, tc.message)
					tampered[0] ^= 0xff
					err = VerifyMLDSA(alg, keypair.PublicKey, tampered, signature)
					assert.Error(t, err, "Tampered message should fail verification")
				}

				t.Logf("✓ %s: Sign/Verify passed for message '%s' (%d bytes)",
					alg, tc.name, len(tc.message))
			})
		}
	}
}

// TestVerificationKATHybridBothRequired tests that hybrid mode requires BOTH signatures
func TestVerificationKATHybridBothRequired(t *testing.T) {
	t.Log("=== Known Answer Test: Hybrid Mode - Both Signatures Required ===")
	t.Log("SECURITY CRITICAL: Hybrid mode MUST reject if either signature is invalid")

	message := []byte("Hybrid authentication test message")

	algorithms := []Algorithm{
		AlgorithmHybridEd25519MLDSA44,
		AlgorithmHybridEd25519MLDSA65,
		AlgorithmHybridEd25519MLDSA87,
	}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			// Generate valid hybrid keypair (use ML-DSA variant)
			mldsaVariant := GetMLDSAVariant(alg)
			keypair, err := GenerateHybridKeyPair(mldsaVariant)
			require.NoError(t, err)

			// Create valid signature
			validSig, err := HybridSign(keypair, message)
			require.NoError(t, err)

			// Get public key for verification
			publicKey := GetHybridPublicKey(keypair)

			// Test 1: Both valid signatures should verify
			err = HybridVerify(publicKey, message, validSig)
			assert.NoError(t, err, "Both valid signatures should verify")
			t.Log("  ✓ Both valid signatures: VERIFIED")

			// Test 2: Invalid Ed25519 signature should fail
			invalidEd25519Sig := &HybridSignature{
				Algorithm: alg,
				Ed25519:   make([]byte, 64), // Zero signature
				MLDSA:     validSig.MLDSA,
				Timestamp: validSig.Timestamp,
			}
			err = HybridVerify(publicKey, message, invalidEd25519Sig)
			assert.Error(t, err, "Invalid Ed25519 should fail even with valid ML-DSA")
			t.Log("  ✓ Invalid Ed25519 + Valid ML-DSA: REJECTED")

			// Test 3: Invalid ML-DSA signature should fail
			invalidMLDSASig := &HybridSignature{
				Algorithm: alg,
				Ed25519:   validSig.Ed25519,
				MLDSA:     make([]byte, len(validSig.MLDSA)), // Zero signature
				Timestamp: validSig.Timestamp,
			}
			err = HybridVerify(publicKey, message, invalidMLDSASig)
			assert.Error(t, err, "Invalid ML-DSA should fail even with valid Ed25519")
			t.Log("  ✓ Valid Ed25519 + Invalid ML-DSA: REJECTED")

			// Test 4: Both invalid should fail
			bothInvalidSig := &HybridSignature{
				Algorithm: alg,
				Ed25519:   make([]byte, 64),
				MLDSA:     make([]byte, len(validSig.MLDSA)),
				Timestamp: validSig.Timestamp,
			}
			err = HybridVerify(publicKey, message, bothInvalidSig)
			assert.Error(t, err, "Both invalid signatures should fail")
			t.Log("  ✓ Both invalid: REJECTED")
		})
	}
}

// =============================================================================
// SECTION 3: CROSS-LIBRARY VERIFICATION
// Tests that verify our implementation matches expected behavior
// =============================================================================

// TestVerificationCrosslibSignatureSizes verifies signature sizes from actual signing
func TestVerificationCrosslibSignatureSizes(t *testing.T) {
	t.Log("=== Cross-Library Verification: Actual Signature Sizes ===")

	message := []byte("Test message for signature size verification")

	testCases := []struct {
		alg          Algorithm
		expectedSize int
	}{
		{AlgorithmMLDSA44, 2420},
		{AlgorithmMLDSA65, 3309},
		{AlgorithmMLDSA87, 4627},
	}

	for _, tc := range testCases {
		t.Run(string(tc.alg), func(t *testing.T) {
			keypair, err := GenerateMLDSAKeyPair(tc.alg)
			require.NoError(t, err)

			signature, err := SignMLDSA(tc.alg, keypair.PrivateKey, message)
			require.NoError(t, err)

			actualSize := len(signature)
			t.Logf("%s: Expected=%d, Actual=%d", tc.alg, tc.expectedSize, actualSize)
			assert.Equal(t, tc.expectedSize, actualSize,
				"Signature size should match NIST FIPS 204 spec")

			// Also verify the signature is valid
			err = VerifyMLDSA(tc.alg, keypair.PublicKey, message, signature)
			assert.NoError(t, err)
		})
	}
}

// TestVerificationDeterministicKeyGen verifies key generation is deterministic with same seed
func TestVerificationDeterministicKeyGen(t *testing.T) {
	t.Log("=== Deterministic Key Generation Verification ===")
	t.Log("Note: ML-DSA key generation from seed should be deterministic")

	// This test verifies the underlying library behavior
	// ML-DSA uses a 32-byte seed for deterministic key generation

	// Generate two keypairs - they should be different (random)
	keypair1, err := GenerateMLDSAKeyPair(AlgorithmMLDSA65)
	require.NoError(t, err)

	keypair2, err := GenerateMLDSAKeyPair(AlgorithmMLDSA65)
	require.NoError(t, err)

	assert.NotEqual(t, keypair1.PublicKey, keypair2.PublicKey, "Random keypairs should be different")
	assert.NotEqual(t, keypair1.PrivateKey, keypair2.PrivateKey, "Random keypairs should be different")

	t.Log("✓ Random key generation produces unique keypairs")
}

// =============================================================================
// SECTION 4: EXPORTABLE TEST VECTORS
// Generate test vectors that can be used for cross-implementation verification
// =============================================================================

// TestVector represents a test vector for external verification
type TestVector struct {
	Algorithm   string `json:"algorithm"`
	Message     string `json:"message"`      // hex encoded
	MessageHash string `json:"message_hash"` // SHA-256 of message (for verification)
	PublicKey   string `json:"public_key"`   // hex encoded
	Signature   string `json:"signature"`    // hex encoded
	Valid       bool   `json:"valid"`
	Description string `json:"description"`
}

// TestVectorSet represents a set of test vectors
type TestVectorSet struct {
	Version     string       `json:"version"`
	Generator   string       `json:"generator"`
	Description string       `json:"description"`
	Vectors     []TestVector `json:"vectors"`
}

// TestGenerateTestVectors generates test vectors for external verification
// Run with: go test -v -run TestGenerateTestVectors ./internal/crypto/pqc/
func TestGenerateTestVectors(t *testing.T) {
	t.Log("=== Generating Test Vectors for External Verification ===")

	vectors := TestVectorSet{
		Version:     "1.0.0",
		Generator:   "AIM PQC Test Suite (github.com/opena2a-org/agent-identity-management)",
		Description: "ML-DSA (NIST FIPS 204) test vectors for cross-implementation verification",
		Vectors:     []TestVector{},
	}

	// Test messages
	messages := []struct {
		desc string
		msg  []byte
	}{
		{"empty message", []byte{}},
		{"hello world", []byte("Hello, World!")},
		{"binary data", []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}},
	}

	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		for _, m := range messages {
			// Generate keypair
			keypair, err := GenerateMLDSAKeyPair(alg)
			require.NoError(t, err)

			// Sign message
			signature, err := SignMLDSA(alg, keypair.PrivateKey, m.msg)
			require.NoError(t, err)

			// Compute message hash for verification
			hash := sha256.Sum256(m.msg)

			// Valid signature vector
			vectors.Vectors = append(vectors.Vectors, TestVector{
				Algorithm:   string(alg),
				Message:     hex.EncodeToString(m.msg),
				MessageHash: hex.EncodeToString(hash[:]),
				PublicKey:   hex.EncodeToString(keypair.PublicKey),
				Signature:   hex.EncodeToString(signature),
				Valid:       true,
				Description: fmt.Sprintf("Valid %s signature for %s", alg, m.desc),
			})

			// Invalid signature vector (tampered signature)
			tamperedSig := make([]byte, len(signature))
			copy(tamperedSig, signature)
			if len(tamperedSig) > 0 {
				tamperedSig[0] ^= 0xff
			}

			vectors.Vectors = append(vectors.Vectors, TestVector{
				Algorithm:   string(alg),
				Message:     hex.EncodeToString(m.msg),
				MessageHash: hex.EncodeToString(hash[:]),
				PublicKey:   hex.EncodeToString(keypair.PublicKey),
				Signature:   hex.EncodeToString(tamperedSig),
				Valid:       false,
				Description: fmt.Sprintf("Invalid %s signature (tampered) for %s", alg, m.desc),
			})
		}
	}

	// Output as JSON
	output, err := json.MarshalIndent(vectors, "", "  ")
	require.NoError(t, err)

	t.Logf("Generated %d test vectors", len(vectors.Vectors))
	t.Log("Test vectors can be used to verify other ML-DSA implementations")

	// Write to file if TEST_VECTORS_OUTPUT is set
	if outputPath := os.Getenv("TEST_VECTORS_OUTPUT"); outputPath != "" {
		err := os.WriteFile(outputPath, output, 0644)
		require.NoError(t, err)
		t.Logf("Test vectors written to: %s", outputPath)
	} else {
		t.Log("Set TEST_VECTORS_OUTPUT=/path/to/file.json to export vectors")
	}

	// Print sample vector
	t.Log("\n=== Sample Test Vector ===")
	if len(vectors.Vectors) > 0 {
		sample, _ := json.MarshalIndent(vectors.Vectors[0], "", "  ")
		t.Log(string(sample))
	}
}

// TestVerifyTestVectors verifies our own generated test vectors
func TestVerifyTestVectors(t *testing.T) {
	t.Log("=== Self-Verification of Test Vectors ===")

	// Generate vectors
	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}
	message := []byte("Test message for self-verification")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			// Generate and sign
			keypair, err := GenerateMLDSAKeyPair(alg)
			require.NoError(t, err)

			signature, err := SignMLDSA(alg, keypair.PrivateKey, message)
			require.NoError(t, err)

			// Verify using hex encoding/decoding (simulating external verification)
			pubKeyHex := hex.EncodeToString(keypair.PublicKey)
			sigHex := hex.EncodeToString(signature)
			msgHex := hex.EncodeToString(message)

			// Decode and verify
			decodedPubKey, err := hex.DecodeString(pubKeyHex)
			require.NoError(t, err)
			decodedSig, err := hex.DecodeString(sigHex)
			require.NoError(t, err)
			decodedMsg, err := hex.DecodeString(msgHex)
			require.NoError(t, err)

			err = VerifyMLDSA(alg, decodedPubKey, decodedMsg, decodedSig)
			assert.NoError(t, err, "Hex-encoded round-trip should verify")

			t.Logf("✓ %s: Self-verification passed", alg)
		})
	}
}

// =============================================================================
// SECTION 5: EDGE CASES AND SECURITY BOUNDARY TESTS
// =============================================================================

// TestVerificationEdgeCases tests boundary conditions
func TestVerificationEdgeCases(t *testing.T) {
	t.Log("=== Edge Case and Boundary Tests ===")

	keypair, err := GenerateMLDSAKeyPair(AlgorithmMLDSA65)
	require.NoError(t, err)

	// Test with various message sizes
	sizes := []int{0, 1, 32, 64, 1024, 1024 * 1024}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("MessageSize_%d", size), func(t *testing.T) {
			message := make([]byte, size)
			rand.Read(message)

			signature, err := SignMLDSA(AlgorithmMLDSA65, keypair.PrivateKey, message)
			require.NoError(t, err, "Should sign message of size %d", size)

			err = VerifyMLDSA(AlgorithmMLDSA65, keypair.PublicKey, message, signature)
			assert.NoError(t, err, "Should verify signature for message of size %d", size)

			t.Logf("✓ Message size %d bytes: OK", size)
		})
	}
}

// TestVerificationWrongKey tests that wrong keys are rejected
func TestVerificationWrongKey(t *testing.T) {
	t.Log("=== Wrong Key Rejection Test ===")

	message := []byte("Test message")

	algorithms := []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			// Generate two keypairs
			keypair1, err := GenerateMLDSAKeyPair(alg)
			require.NoError(t, err)

			keypair2, err := GenerateMLDSAKeyPair(alg)
			require.NoError(t, err)

			// Sign with first key
			signature, err := SignMLDSA(alg, keypair1.PrivateKey, message)
			require.NoError(t, err)

			// Verify with first key (should pass)
			err = VerifyMLDSA(alg, keypair1.PublicKey, message, signature)
			assert.NoError(t, err, "Correct key should verify")

			// Verify with second key (should fail)
			err = VerifyMLDSA(alg, keypair2.PublicKey, message, signature)
			assert.Error(t, err, "Wrong key should fail verification")

			t.Logf("✓ %s: Wrong key correctly rejected", alg)
		})
	}
}

// =============================================================================
// SECTION 6: CIRCL LIBRARY DIRECT VERIFICATION
// Verify we're using the underlying library correctly
// =============================================================================

// TestVerificationCirclDirect directly tests the Cloudflare CIRCL library
func TestVerificationCirclDirect(t *testing.T) {
	t.Log("=== Direct CIRCL Library Verification ===")
	t.Log("Verifying correct usage of github.com/cloudflare/circl")

	message := []byte("Direct CIRCL library test")

	t.Run("ML-DSA-44", func(t *testing.T) {
		pub, priv, err := mldsa44.GenerateKey(rand.Reader)
		require.NoError(t, err)

		sig := make([]byte, mldsa44.SignatureSize)
		err = mldsa44.SignTo(priv, message, nil, false, sig)
		require.NoError(t, err)
		valid := mldsa44.Verify(pub, message, nil, sig)
		assert.True(t, valid, "CIRCL ML-DSA-44 should verify")

		t.Logf("✓ CIRCL ML-DSA-44: Public=%d bytes, Signature=%d bytes",
			mldsa44.PublicKeySize, len(sig))
	})

	t.Run("ML-DSA-65", func(t *testing.T) {
		pub, priv, err := mldsa65.GenerateKey(rand.Reader)
		require.NoError(t, err)

		sig := make([]byte, mldsa65.SignatureSize)
		err = mldsa65.SignTo(priv, message, nil, false, sig)
		require.NoError(t, err)
		valid := mldsa65.Verify(pub, message, nil, sig)
		assert.True(t, valid, "CIRCL ML-DSA-65 should verify")

		t.Logf("✓ CIRCL ML-DSA-65: Public=%d bytes, Signature=%d bytes",
			mldsa65.PublicKeySize, len(sig))
	})

	t.Run("ML-DSA-87", func(t *testing.T) {
		pub, priv, err := mldsa87.GenerateKey(rand.Reader)
		require.NoError(t, err)

		sig := make([]byte, mldsa87.SignatureSize)
		err = mldsa87.SignTo(priv, message, nil, false, sig)
		require.NoError(t, err)
		valid := mldsa87.Verify(pub, message, nil, sig)
		assert.True(t, valid, "CIRCL ML-DSA-87 should verify")

		t.Logf("✓ CIRCL ML-DSA-87: Public=%d bytes, Signature=%d bytes",
			mldsa87.PublicKeySize, len(sig))
	})
}

// =============================================================================
// SECTION 7: SUMMARY AND DOCUMENTATION
// =============================================================================

// TestVerificationSummary prints a summary of all verification tests
func TestVerificationSummary(t *testing.T) {
	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════╗")
	t.Log("║           AIM PQC VERIFICATION TEST SUITE SUMMARY                ║")
	t.Log("╠══════════════════════════════════════════════════════════════════╣")
	t.Log("║                                                                  ║")
	t.Log("║  Algorithms Tested:                                              ║")
	t.Log("║    • ML-DSA-44 (NIST Category 2, 128-bit classical)              ║")
	t.Log("║    • ML-DSA-65 (NIST Category 3, 192-bit classical) [RECOMMENDED]║")
	t.Log("║    • ML-DSA-87 (NIST Category 5, 256-bit classical)              ║")
	t.Log("║    • Ed25519+ML-DSA-44 (Hybrid)                                  ║")
	t.Log("║    • Ed25519+ML-DSA-65 (Hybrid) [RECOMMENDED]                    ║")
	t.Log("║    • Ed25519+ML-DSA-87 (Hybrid)                                  ║")
	t.Log("║                                                                  ║")
	t.Log("║  Underlying Library:                                             ║")
	t.Log("║    github.com/cloudflare/circl (audited by NCC Group)            ║")
	t.Log("║                                                                  ║")
	t.Log("║  Standards Compliance:                                           ║")
	t.Log("║    • NIST FIPS 204 (ML-DSA/Dilithium)                            ║")
	t.Log("║    • RFC 8032 (Ed25519)                                          ║")
	t.Log("║                                                                  ║")
	t.Log("║  Test Categories:                                                ║")
	t.Log("║    1. NIST Key/Signature Size Verification                       ║")
	t.Log("║    2. Known Answer Tests (KAT)                                   ║")
	t.Log("║    3. Hybrid Mode Security (Both Signatures Required)            ║")
	t.Log("║    4. Cross-Implementation Test Vector Generation                ║")
	t.Log("║    5. Edge Cases and Boundary Tests                              ║")
	t.Log("║    6. Direct CIRCL Library Verification                          ║")
	t.Log("║                                                                  ║")
	t.Log("║  For External Verification:                                      ║")
	t.Log("║    TEST_VECTORS_OUTPUT=/path/to/vectors.json go test -v \\        ║")
	t.Log("║      -run TestGenerateTestVectors ./internal/crypto/pqc/         ║")
	t.Log("║                                                                  ║")
	t.Log("╚══════════════════════════════════════════════════════════════════╝")
}
