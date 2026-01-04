package org.opena2a.aim.crypto.pqc;

import org.bouncycastle.crypto.AsymmetricCipherKeyPair;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.bouncycastle.pqc.crypto.mldsa.MLDSAKeyGenerationParameters;
import org.bouncycastle.pqc.crypto.mldsa.MLDSAKeyPairGenerator;
import org.bouncycastle.pqc.crypto.mldsa.MLDSAParameters;
import org.bouncycastle.pqc.crypto.mldsa.MLDSAPrivateKeyParameters;
import org.bouncycastle.pqc.crypto.mldsa.MLDSAPublicKeyParameters;
import org.bouncycastle.pqc.crypto.mldsa.MLDSASigner;

import java.security.SecureRandom;

/**
 * Post-Quantum Cryptographic operations for AIM SDK.
 * Implements ML-DSA (FIPS 204) using BouncyCastle.
 *
 * Provides:
 * - ML-DSA key generation (44/65/87 variants)
 * - ML-DSA signing and verification
 * - Hybrid Ed25519+ML-DSA for defense-in-depth
 */
public final class PQCOperations {

    private static final SecureRandom SECURE_RANDOM = new SecureRandom();

    private PQCOperations() {
        // Utility class
    }

    /**
     * Check if PQC is available (BouncyCastle with ML-DSA support)
     */
    public static boolean isPQCAvailable() {
        try {
            Class.forName("org.bouncycastle.pqc.crypto.mldsa.MLDSAParameters");
            return true;
        } catch (ClassNotFoundException e) {
            return false;
        }
    }

    /**
     * Get ML-DSA parameters for a given algorithm variant
     */
    private static MLDSAParameters getMLDSAParameters(Algorithm algorithm) {
        switch (algorithm.getMLDSAVariant()) {
            case ML_DSA_44:
                return MLDSAParameters.ml_dsa_44;
            case ML_DSA_65:
                return MLDSAParameters.ml_dsa_65;
            case ML_DSA_87:
                return MLDSAParameters.ml_dsa_87;
            default:
                return MLDSAParameters.ml_dsa_65;
        }
    }

    // ========== ML-DSA Operations ==========

    /**
     * Generate an ML-DSA key pair
     */
    public static MLDSAKeyPair generateMLDSAKeyPair(Algorithm variant) {
        if (!variant.isPostQuantum() || variant.isHybrid()) {
            variant = variant.getMLDSAVariant();
        }

        MLDSAParameters params = getMLDSAParameters(variant);
        MLDSAKeyPairGenerator keyGen = new MLDSAKeyPairGenerator();
        keyGen.init(new MLDSAKeyGenerationParameters(SECURE_RANDOM, params));

        AsymmetricCipherKeyPair keyPair = keyGen.generateKeyPair();
        MLDSAPublicKeyParameters publicKey = (MLDSAPublicKeyParameters) keyPair.getPublic();
        MLDSAPrivateKeyParameters privateKey = (MLDSAPrivateKeyParameters) keyPair.getPrivate();

        // Note: Store the seed (32 bytes) for private key reconstruction, not getEncoded()
        // The constructor expects the seed, not the full expanded private key
        return new MLDSAKeyPair(
                variant.getMLDSAVariant(),
                publicKey.getEncoded(),
                privateKey.getSeed()
        );
    }

    /**
     * Sign a message using ML-DSA
     */
    public static byte[] signMLDSA(Algorithm variant, byte[] privateKey, byte[] message) {
        try {
            MLDSAParameters params = getMLDSAParameters(variant);
            MLDSAPrivateKeyParameters privateKeyParams = new MLDSAPrivateKeyParameters(params, privateKey);

            MLDSASigner signer = new MLDSASigner();
            signer.init(true, privateKeyParams);
            signer.update(message, 0, message.length);
            return signer.generateSignature();
        } catch (org.bouncycastle.crypto.CryptoException e) {
            throw new RuntimeException("ML-DSA signing failed", e);
        }
    }

    /**
     * Verify an ML-DSA signature
     */
    public static boolean verifyMLDSA(Algorithm variant, byte[] publicKey, byte[] message, byte[] signature) {
        try {
            MLDSAParameters params = getMLDSAParameters(variant);
            MLDSAPublicKeyParameters publicKeyParams = new MLDSAPublicKeyParameters(params, publicKey);

            MLDSASigner signer = new MLDSASigner();
            signer.init(false, publicKeyParams);
            signer.update(message, 0, message.length);
            return signer.verifySignature(signature);
        } catch (Exception e) {
            return false;
        }
    }

    // ========== Ed25519 Operations ==========

    /**
     * Generate an Ed25519 key pair
     */
    public static byte[][] generateEd25519KeyPair() {
        Ed25519PrivateKeyParameters privateKey = new Ed25519PrivateKeyParameters(SECURE_RANDOM);
        Ed25519PublicKeyParameters publicKey = privateKey.generatePublicKey();
        return new byte[][] { publicKey.getEncoded(), privateKey.getEncoded() };
    }

    /**
     * Sign a message using Ed25519
     */
    public static byte[] signEd25519(byte[] privateKey, byte[] message) {
        Ed25519PrivateKeyParameters privateKeyParams = new Ed25519PrivateKeyParameters(privateKey, 0);
        Ed25519Signer signer = new Ed25519Signer();
        signer.init(true, privateKeyParams);
        signer.update(message, 0, message.length);
        return signer.generateSignature();
    }

    /**
     * Verify an Ed25519 signature
     */
    public static boolean verifyEd25519(byte[] publicKey, byte[] message, byte[] signature) {
        try {
            Ed25519PublicKeyParameters publicKeyParams = new Ed25519PublicKeyParameters(publicKey, 0);
            Ed25519Signer signer = new Ed25519Signer();
            signer.init(false, publicKeyParams);
            signer.update(message, 0, message.length);
            return signer.verifySignature(signature);
        } catch (Exception e) {
            return false;
        }
    }

    // ========== Hybrid Operations ==========

    /**
     * Generate a hybrid Ed25519+ML-DSA key pair
     */
    public static HybridKeyPair generateHybridKeyPair(Algorithm variant) {
        if (!variant.isHybrid()) {
            // Convert to hybrid variant
            switch (variant.getMLDSAVariant()) {
                case ML_DSA_44:
                    variant = Algorithm.ED25519_ML_DSA_44;
                    break;
                case ML_DSA_87:
                    variant = Algorithm.ED25519_ML_DSA_87;
                    break;
                default:
                    variant = Algorithm.ED25519_ML_DSA_65;
            }
        }

        // Generate Ed25519 key pair
        byte[][] ed25519Keys = generateEd25519KeyPair();
        byte[] ed25519PublicKey = ed25519Keys[0];
        byte[] ed25519PrivateKey = ed25519Keys[1];

        // Generate ML-DSA key pair
        MLDSAKeyPair mldsaKeyPair = generateMLDSAKeyPair(variant.getMLDSAVariant());

        return new HybridKeyPair(
                variant,
                ed25519PublicKey,
                ed25519PrivateKey,
                mldsaKeyPair.getPublicKey(),
                mldsaKeyPair.getPrivateKey()
        );
    }

    /**
     * Sign a message using hybrid Ed25519+ML-DSA.
     * Creates both signatures for defense-in-depth.
     */
    public static HybridSignature hybridSign(HybridKeyPair keys, byte[] message) {
        // Sign with Ed25519
        byte[] ed25519Sig = signEd25519(keys.getEd25519PrivateKey(), message);

        // Sign with ML-DSA
        byte[] mldsaSig = signMLDSA(keys.getMldsaVariant(), keys.getMldsaPrivateKey(), message);

        return new HybridSignature(keys.getAlgorithm(), ed25519Sig, mldsaSig);
    }

    /**
     * Verify a hybrid signature.
     * BOTH signatures must be valid for verification to succeed.
     */
    public static boolean hybridVerify(HybridPublicKey publicKey, byte[] message, HybridSignature signature) {
        // Verify Ed25519 signature first (faster)
        boolean ed25519Valid = verifyEd25519(
                publicKey.getEd25519PublicKey(),
                message,
                signature.getEd25519Signature()
        );

        if (!ed25519Valid) {
            return false;
        }

        // Verify ML-DSA signature
        boolean mldsaValid = verifyMLDSA(
                publicKey.getMldsaVariant(),
                publicKey.getMldsaPublicKey(),
                message,
                signature.getMldsaSignature()
        );

        // BOTH must be valid
        return mldsaValid;
    }

    /**
     * Create request signature headers for hybrid mode.
     * Returns headers compatible with the Go backend.
     */
    public static java.util.Map<String, String> createHybridRequestHeaders(
            HybridKeyPair keys,
            String method,
            String path,
            String body,
            Long timestamp) {

        long ts = timestamp != null ? timestamp : System.currentTimeMillis();

        // Create body hash
        String bodyHash = "";
        if (body != null && !body.isEmpty()) {
            bodyHash = hashToHex(body.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        }

        // Create message to sign
        String message = String.format("%d:%s:%s:%s", ts, method.toUpperCase(), path, bodyHash);
        byte[] messageBytes = message.getBytes(java.nio.charset.StandardCharsets.UTF_8);

        // Sign with hybrid
        HybridSignature signature = hybridSign(keys, messageBytes);

        java.util.Map<String, String> headers = new java.util.HashMap<>();
        headers.put("X-Agent-ID", ""); // To be filled by caller
        headers.put("X-Algorithm", keys.getAlgorithm().getAlgorithmId());
        headers.put("X-Signature-Ed25519", signature.getEd25519SignatureBase64());
        headers.put("X-Signature-MLDSA", signature.getMldsaSignatureBase64());
        headers.put("X-Timestamp", String.valueOf(ts));

        return headers;
    }

    /**
     * Hash data to hex string using SHA-256
     */
    private static String hashToHex(byte[] data) {
        try {
            java.security.MessageDigest digest = java.security.MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(data);
            StringBuilder hexString = new StringBuilder();
            for (byte b : hash) {
                String hex = Integer.toHexString(0xff & b);
                if (hex.length() == 1) {
                    hexString.append('0');
                }
                hexString.append(hex);
            }
            return hexString.toString();
        } catch (java.security.NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 not available", e);
        }
    }
}
