package org.opena2a.aim.crypto.pqc;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.BeforeEach;

import java.nio.charset.StandardCharsets;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Comprehensive tests for PQC (Post-Quantum Cryptography) operations.
 * Tests ML-DSA key generation, signing, verification, and hybrid operations.
 */
@DisplayName("PQC Operations")
class PQCOperationsTest {

    @Nested
    @DisplayName("Algorithm Enum")
    class AlgorithmTests {

        @Test
        @DisplayName("should identify post-quantum algorithms")
        void shouldIdentifyPostQuantumAlgorithms() {
            assertTrue(Algorithm.ML_DSA_44.isPostQuantum());
            assertTrue(Algorithm.ML_DSA_65.isPostQuantum());
            assertTrue(Algorithm.ML_DSA_87.isPostQuantum());
            assertTrue(Algorithm.ED25519_ML_DSA_44.isPostQuantum());
            assertTrue(Algorithm.ED25519_ML_DSA_65.isPostQuantum());
            assertTrue(Algorithm.ED25519_ML_DSA_87.isPostQuantum());
            assertFalse(Algorithm.ED25519.isPostQuantum());
        }

        @Test
        @DisplayName("should identify hybrid algorithms")
        void shouldIdentifyHybridAlgorithms() {
            assertTrue(Algorithm.ED25519_ML_DSA_44.isHybrid());
            assertTrue(Algorithm.ED25519_ML_DSA_65.isHybrid());
            assertTrue(Algorithm.ED25519_ML_DSA_87.isHybrid());
            assertFalse(Algorithm.ML_DSA_44.isHybrid());
            assertFalse(Algorithm.ML_DSA_65.isHybrid());
            assertFalse(Algorithm.ML_DSA_87.isHybrid());
            assertFalse(Algorithm.ED25519.isHybrid());
        }

        @Test
        @DisplayName("should get correct ML-DSA variant")
        void shouldGetCorrectMLDSAVariant() {
            assertEquals(Algorithm.ML_DSA_44, Algorithm.ED25519_ML_DSA_44.getMLDSAVariant());
            assertEquals(Algorithm.ML_DSA_65, Algorithm.ED25519_ML_DSA_65.getMLDSAVariant());
            assertEquals(Algorithm.ML_DSA_87, Algorithm.ED25519_ML_DSA_87.getMLDSAVariant());
            assertEquals(Algorithm.ML_DSA_44, Algorithm.ML_DSA_44.getMLDSAVariant());
            assertEquals(Algorithm.ML_DSA_65, Algorithm.ML_DSA_65.getMLDSAVariant());
            assertEquals(Algorithm.ML_DSA_87, Algorithm.ML_DSA_87.getMLDSAVariant());
        }

        @Test
        @DisplayName("should return correct algorithm ID")
        void shouldReturnCorrectAlgorithmId() {
            assertEquals("Ed25519", Algorithm.ED25519.getAlgorithmId());
            assertEquals("ML-DSA-44", Algorithm.ML_DSA_44.getAlgorithmId());
            assertEquals("ML-DSA-65", Algorithm.ML_DSA_65.getAlgorithmId());
            assertEquals("ML-DSA-87", Algorithm.ML_DSA_87.getAlgorithmId());
            assertEquals("Ed25519+ML-DSA-44", Algorithm.ED25519_ML_DSA_44.getAlgorithmId());
            assertEquals("Ed25519+ML-DSA-65", Algorithm.ED25519_ML_DSA_65.getAlgorithmId());
            assertEquals("Ed25519+ML-DSA-87", Algorithm.ED25519_ML_DSA_87.getAlgorithmId());
        }

        @Test
        @DisplayName("should parse algorithm from string")
        void shouldParseAlgorithmFromString() {
            assertEquals(Algorithm.ED25519, Algorithm.fromString("Ed25519"));
            assertEquals(Algorithm.ML_DSA_44, Algorithm.fromString("ML-DSA-44"));
            assertEquals(Algorithm.ML_DSA_65, Algorithm.fromString("ML-DSA-65"));
            assertEquals(Algorithm.ML_DSA_87, Algorithm.fromString("ML-DSA-87"));
            assertEquals(Algorithm.ED25519_ML_DSA_44, Algorithm.fromString("Ed25519+ML-DSA-44"));
            assertEquals(Algorithm.ED25519_ML_DSA_65, Algorithm.fromString("Ed25519+ML-DSA-65"));
            assertEquals(Algorithm.ED25519_ML_DSA_87, Algorithm.fromString("Ed25519+ML-DSA-87"));
        }

        @Test
        @DisplayName("should throw for unknown algorithm")
        void shouldThrowForUnknownAlgorithm() {
            assertThrows(IllegalArgumentException.class, () -> Algorithm.fromString("Unknown"));
        }
    }

    @Nested
    @DisplayName("Key Sizes")
    class KeySizesTests {

        @Test
        @DisplayName("should return correct ML-DSA-44 sizes")
        void shouldReturnCorrectMLDSA44Sizes() {
            assertEquals(1312, KeySizes.getPublicKeySize(Algorithm.ML_DSA_44));
            assertEquals(KeySizes.ML_DSA_SEED, KeySizes.getPrivateKeySize(Algorithm.ML_DSA_44)); // Seed size
            assertEquals(2420, KeySizes.getSignatureSize(Algorithm.ML_DSA_44));
        }

        @Test
        @DisplayName("should return correct ML-DSA-65 sizes")
        void shouldReturnCorrectMLDSA65Sizes() {
            assertEquals(1952, KeySizes.getPublicKeySize(Algorithm.ML_DSA_65));
            assertEquals(KeySizes.ML_DSA_SEED, KeySizes.getPrivateKeySize(Algorithm.ML_DSA_65)); // Seed size
            assertEquals(3309, KeySizes.getSignatureSize(Algorithm.ML_DSA_65));
        }

        @Test
        @DisplayName("should return correct ML-DSA-87 sizes")
        void shouldReturnCorrectMLDSA87Sizes() {
            assertEquals(2592, KeySizes.getPublicKeySize(Algorithm.ML_DSA_87));
            assertEquals(KeySizes.ML_DSA_SEED, KeySizes.getPrivateKeySize(Algorithm.ML_DSA_87)); // Seed size
            assertEquals(4627, KeySizes.getSignatureSize(Algorithm.ML_DSA_87));
        }

        @Test
        @DisplayName("should validate key sizes correctly")
        void shouldValidateKeySizesCorrectly() {
            byte[] validPublicKey = new byte[1952]; // ML-DSA-65
            byte[] invalidPublicKey = new byte[100];

            assertTrue(KeySizes.validatePublicKeySize(Algorithm.ML_DSA_65, validPublicKey));
            assertFalse(KeySizes.validatePublicKeySize(Algorithm.ML_DSA_65, invalidPublicKey));
            assertFalse(KeySizes.validatePublicKeySize(Algorithm.ML_DSA_65, null));
        }
    }

    @Nested
    @DisplayName("PQC Availability")
    class PQCAvailabilityTests {

        @Test
        @DisplayName("should detect PQC availability")
        void shouldDetectPQCAvailability() {
            // BouncyCastle 1.77 should have ML-DSA support
            assertTrue(PQCOperations.isPQCAvailable());
        }
    }

    @Nested
    @DisplayName("ML-DSA Key Generation")
    class MLDSAKeyGenerationTests {

        @Test
        @DisplayName("should generate ML-DSA-44 key pair")
        void shouldGenerateMLDSA44KeyPair() {
            MLDSAKeyPair keyPair = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_44);

            assertNotNull(keyPair);
            assertEquals(Algorithm.ML_DSA_44, keyPair.getAlgorithm());
            assertEquals(KeySizes.ML_DSA_44_PUBLIC_KEY, keyPair.getPublicKey().length);
            assertEquals(KeySizes.ML_DSA_44_PRIVATE_KEY, keyPair.getPrivateKey().length);
            assertNotNull(keyPair.getCreatedAt());
        }

        @Test
        @DisplayName("should generate ML-DSA-65 key pair")
        void shouldGenerateMLDSA65KeyPair() {
            MLDSAKeyPair keyPair = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);

            assertNotNull(keyPair);
            assertEquals(Algorithm.ML_DSA_65, keyPair.getAlgorithm());
            assertEquals(KeySizes.ML_DSA_65_PUBLIC_KEY, keyPair.getPublicKey().length);
            assertEquals(KeySizes.ML_DSA_65_PRIVATE_KEY, keyPair.getPrivateKey().length);
        }

        @Test
        @DisplayName("should generate ML-DSA-87 key pair")
        void shouldGenerateMLDSA87KeyPair() {
            MLDSAKeyPair keyPair = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_87);

            assertNotNull(keyPair);
            assertEquals(Algorithm.ML_DSA_87, keyPair.getAlgorithm());
            assertEquals(KeySizes.ML_DSA_87_PUBLIC_KEY, keyPair.getPublicKey().length);
            assertEquals(KeySizes.ML_DSA_87_PRIVATE_KEY, keyPair.getPrivateKey().length);
        }

        @Test
        @DisplayName("should generate unique key pairs")
        void shouldGenerateUniqueKeyPairs() {
            MLDSAKeyPair keyPair1 = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);
            MLDSAKeyPair keyPair2 = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);

            assertFalse(java.util.Arrays.equals(keyPair1.getPublicKey(), keyPair2.getPublicKey()));
            assertFalse(java.util.Arrays.equals(keyPair1.getPrivateKey(), keyPair2.getPrivateKey()));
        }
    }

    @Nested
    @DisplayName("ML-DSA Sign and Verify")
    class MLDSASignVerifyTests {

        private MLDSAKeyPair keyPair;
        private byte[] message;

        @BeforeEach
        void setUp() {
            keyPair = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);
            message = "Test message for ML-DSA signing".getBytes(StandardCharsets.UTF_8);
        }

        @Test
        @DisplayName("should sign and verify message")
        void shouldSignAndVerifyMessage() {
            byte[] signature = PQCOperations.signMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPrivateKey(),
                    message
            );

            assertNotNull(signature);
            assertEquals(KeySizes.ML_DSA_65_SIGNATURE, signature.length);

            boolean valid = PQCOperations.verifyMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPublicKey(),
                    message,
                    signature
            );

            assertTrue(valid);
        }

        @Test
        @DisplayName("should reject invalid signature")
        void shouldRejectInvalidSignature() {
            byte[] signature = PQCOperations.signMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPrivateKey(),
                    message
            );

            // Corrupt the signature
            signature[0] ^= 0xFF;

            boolean valid = PQCOperations.verifyMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPublicKey(),
                    message,
                    signature
            );

            assertFalse(valid);
        }

        @Test
        @DisplayName("should reject tampered message")
        void shouldRejectTamperedMessage() {
            byte[] signature = PQCOperations.signMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPrivateKey(),
                    message
            );

            byte[] tamperedMessage = "Tampered message".getBytes(StandardCharsets.UTF_8);

            boolean valid = PQCOperations.verifyMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPublicKey(),
                    tamperedMessage,
                    signature
            );

            assertFalse(valid);
        }

        @Test
        @DisplayName("should reject wrong public key")
        void shouldRejectWrongPublicKey() {
            byte[] signature = PQCOperations.signMLDSA(
                    Algorithm.ML_DSA_65,
                    keyPair.getPrivateKey(),
                    message
            );

            MLDSAKeyPair otherKeyPair = PQCOperations.generateMLDSAKeyPair(Algorithm.ML_DSA_65);

            boolean valid = PQCOperations.verifyMLDSA(
                    Algorithm.ML_DSA_65,
                    otherKeyPair.getPublicKey(),
                    message,
                    signature
            );

            assertFalse(valid);
        }
    }

    @Nested
    @DisplayName("Hybrid Key Generation")
    class HybridKeyGenerationTests {

        @Test
        @DisplayName("should generate hybrid ML-DSA-65 key pair")
        void shouldGenerateHybridKeyPair() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);

            assertNotNull(keyPair);
            assertEquals(Algorithm.ED25519_ML_DSA_65, keyPair.getAlgorithm());
            assertEquals(Algorithm.ML_DSA_65, keyPair.getMldsaVariant());

            // Ed25519 keys
            assertEquals(KeySizes.ED25519_PUBLIC_KEY, keyPair.getEd25519PublicKey().length);
            assertEquals(KeySizes.ED25519_PRIVATE_KEY, keyPair.getEd25519PrivateKey().length);

            // ML-DSA keys
            assertEquals(KeySizes.ML_DSA_65_PUBLIC_KEY, keyPair.getMldsaPublicKey().length);
            assertEquals(KeySizes.ML_DSA_65_PRIVATE_KEY, keyPair.getMldsaPrivateKey().length);
        }

        @Test
        @DisplayName("should generate hybrid key pair for all variants")
        void shouldGenerateHybridKeyPairForAllVariants() {
            for (Algorithm alg : new Algorithm[]{
                    Algorithm.ED25519_ML_DSA_44,
                    Algorithm.ED25519_ML_DSA_65,
                    Algorithm.ED25519_ML_DSA_87
            }) {
                HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(alg);
                assertNotNull(keyPair);
                assertEquals(alg, keyPair.getAlgorithm());
            }
        }

        @Test
        @DisplayName("should get hybrid public key from key pair")
        void shouldGetHybridPublicKey() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);
            HybridPublicKey publicKey = keyPair.getPublicKey();

            assertNotNull(publicKey);
            assertArrayEquals(keyPair.getEd25519PublicKey(), publicKey.getEd25519PublicKey());
            assertArrayEquals(keyPair.getMldsaPublicKey(), publicKey.getMldsaPublicKey());
            assertEquals(keyPair.getMldsaVariant(), publicKey.getMldsaVariant());
        }
    }

    @Nested
    @DisplayName("Hybrid Sign and Verify")
    class HybridSignVerifyTests {

        private HybridKeyPair keyPair;
        private byte[] message;

        @BeforeEach
        void setUp() {
            keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);
            message = "Test message for hybrid signing".getBytes(StandardCharsets.UTF_8);
        }

        @Test
        @DisplayName("should sign and verify with hybrid signature")
        void shouldSignAndVerifyWithHybridSignature() {
            HybridSignature signature = PQCOperations.hybridSign(keyPair, message);

            assertNotNull(signature);
            assertEquals(keyPair.getAlgorithm(), signature.getAlgorithm());
            assertEquals(KeySizes.ED25519_SIGNATURE, signature.getEd25519Signature().length);
            assertEquals(KeySizes.ML_DSA_65_SIGNATURE, signature.getMldsaSignature().length);

            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), message, signature);
            assertTrue(valid);
        }

        @Test
        @DisplayName("should reject if Ed25519 signature is invalid")
        void shouldRejectIfEd25519SignatureInvalid() {
            HybridSignature signature = PQCOperations.hybridSign(keyPair, message);

            // Corrupt the Ed25519 signature
            byte[] corruptedEd25519Sig = signature.getEd25519Signature();
            corruptedEd25519Sig[0] ^= 0xFF;
            HybridSignature corruptedSig = new HybridSignature(
                    signature.getAlgorithm(),
                    corruptedEd25519Sig,
                    signature.getMldsaSignature(),
                    signature.getTimestamp()
            );

            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), message, corruptedSig);
            assertFalse(valid);
        }

        @Test
        @DisplayName("should reject if ML-DSA signature is invalid")
        void shouldRejectIfMLDSASignatureInvalid() {
            HybridSignature signature = PQCOperations.hybridSign(keyPair, message);

            // Corrupt the ML-DSA signature
            byte[] corruptedMldsaSig = signature.getMldsaSignature();
            corruptedMldsaSig[0] ^= 0xFF;
            HybridSignature corruptedSig = new HybridSignature(
                    signature.getAlgorithm(),
                    signature.getEd25519Signature(),
                    corruptedMldsaSig,
                    signature.getTimestamp()
            );

            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), message, corruptedSig);
            assertFalse(valid);
        }

        @Test
        @DisplayName("should reject tampered message")
        void shouldRejectTamperedMessage() {
            HybridSignature signature = PQCOperations.hybridSign(keyPair, message);
            byte[] tamperedMessage = "Tampered message".getBytes(StandardCharsets.UTF_8);

            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), tamperedMessage, signature);
            assertFalse(valid);
        }

        @Test
        @DisplayName("should sign and verify empty message")
        void shouldSignAndVerifyEmptyMessage() {
            byte[] emptyMessage = new byte[0];

            HybridSignature signature = PQCOperations.hybridSign(keyPair, emptyMessage);
            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), emptyMessage, signature);

            assertTrue(valid);
        }

        @Test
        @DisplayName("should sign and verify large message")
        void shouldSignAndVerifyLargeMessage() {
            byte[] largeMessage = new byte[1024 * 1024]; // 1MB
            new java.util.Random().nextBytes(largeMessage);

            HybridSignature signature = PQCOperations.hybridSign(keyPair, largeMessage);
            boolean valid = PQCOperations.hybridVerify(keyPair.getPublicKey(), largeMessage, signature);

            assertTrue(valid);
        }
    }

    @Nested
    @DisplayName("Signature Encoding")
    class SignatureEncodingTests {

        @Test
        @DisplayName("should encode and decode hybrid signature")
        void shouldEncodeAndDecodeHybridSignature() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);
            byte[] message = "Test message".getBytes(StandardCharsets.UTF_8);
            HybridSignature original = PQCOperations.hybridSign(keyPair, message);

            Map<String, Object> encoded = original.encode();
            HybridSignature decoded = HybridSignature.decode(encoded);

            assertEquals(original.getAlgorithm(), decoded.getAlgorithm());
            assertArrayEquals(original.getEd25519Signature(), decoded.getEd25519Signature());
            assertArrayEquals(original.getMldsaSignature(), decoded.getMldsaSignature());
            assertEquals(original.getTimestamp(), decoded.getTimestamp());
        }

        @Test
        @DisplayName("should provide Base64 encoded signatures")
        void shouldProvideBase64EncodedSignatures() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);
            byte[] message = "Test message".getBytes(StandardCharsets.UTF_8);
            HybridSignature signature = PQCOperations.hybridSign(keyPair, message);

            String ed25519Base64 = signature.getEd25519SignatureBase64();
            String mldsaBase64 = signature.getMldsaSignatureBase64();

            assertNotNull(ed25519Base64);
            assertNotNull(mldsaBase64);
            assertFalse(ed25519Base64.isEmpty());
            assertFalse(mldsaBase64.isEmpty());
        }
    }

    @Nested
    @DisplayName("Request Headers")
    class RequestHeadersTests {

        @Test
        @DisplayName("should create hybrid request headers")
        void shouldCreateHybridRequestHeaders() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);

            Map<String, String> headers = PQCOperations.createHybridRequestHeaders(
                    keyPair,
                    "POST",
                    "/api/v1/agents/verify",
                    "{\"action\": \"test\"}",
                    null
            );

            assertNotNull(headers);
            assertEquals(keyPair.getAlgorithm().getAlgorithmId(), headers.get("X-Algorithm"));
            assertNotNull(headers.get("X-Signature-Ed25519"));
            assertNotNull(headers.get("X-Signature-MLDSA"));
            assertNotNull(headers.get("X-Timestamp"));
        }

        @Test
        @DisplayName("should handle empty body")
        void shouldHandleEmptyBody() {
            HybridKeyPair keyPair = PQCOperations.generateHybridKeyPair(Algorithm.ED25519_ML_DSA_65);

            Map<String, String> headers = PQCOperations.createHybridRequestHeaders(
                    keyPair,
                    "GET",
                    "/api/v1/agents/info",
                    null,
                    null
            );

            assertNotNull(headers);
            assertNotNull(headers.get("X-Signature-Ed25519"));
            assertNotNull(headers.get("X-Signature-MLDSA"));
        }
    }

    @Nested
    @DisplayName("Ed25519 Operations")
    class Ed25519OperationsTests {

        @Test
        @DisplayName("should generate Ed25519 key pair")
        void shouldGenerateEd25519KeyPair() {
            byte[][] keys = PQCOperations.generateEd25519KeyPair();

            assertNotNull(keys);
            assertEquals(2, keys.length);
            assertEquals(KeySizes.ED25519_PUBLIC_KEY, keys[0].length);
            assertEquals(KeySizes.ED25519_PRIVATE_KEY, keys[1].length);
        }

        @Test
        @DisplayName("should sign and verify with Ed25519")
        void shouldSignAndVerifyWithEd25519() {
            byte[][] keys = PQCOperations.generateEd25519KeyPair();
            byte[] publicKey = keys[0];
            byte[] privateKey = keys[1];
            byte[] message = "Test message".getBytes(StandardCharsets.UTF_8);

            byte[] signature = PQCOperations.signEd25519(privateKey, message);

            assertNotNull(signature);
            assertEquals(KeySizes.ED25519_SIGNATURE, signature.length);

            boolean valid = PQCOperations.verifyEd25519(publicKey, message, signature);
            assertTrue(valid);
        }
    }
}
