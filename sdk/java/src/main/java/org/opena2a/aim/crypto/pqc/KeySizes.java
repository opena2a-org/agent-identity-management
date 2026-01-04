package org.opena2a.aim.crypto.pqc;

/**
 * Key and signature sizes for cryptographic algorithms.
 * Values match NIST FIPS 204 specification.
 *
 * Note: Private key sizes are for the SEED (32 bytes), which is what we store
 * for portability. The constructor reconstructs the full expanded key from seed.
 */
public final class KeySizes {

    private KeySizes() {
        // Utility class
    }

    // Ed25519 sizes
    public static final int ED25519_PUBLIC_KEY = 32;
    public static final int ED25519_PRIVATE_KEY = 32;
    public static final int ED25519_SIGNATURE = 64;

    // ML-DSA seed size (same for all variants)
    public static final int ML_DSA_SEED = 32;

    // ML-DSA-44 sizes (NIST Level 2)
    public static final int ML_DSA_44_PUBLIC_KEY = 1312;
    public static final int ML_DSA_44_PRIVATE_KEY = ML_DSA_SEED;  // Store seed for reconstruction
    public static final int ML_DSA_44_SIGNATURE = 2420;

    // ML-DSA-65 sizes (NIST Level 3)
    public static final int ML_DSA_65_PUBLIC_KEY = 1952;
    public static final int ML_DSA_65_PRIVATE_KEY = ML_DSA_SEED;  // Store seed for reconstruction
    public static final int ML_DSA_65_SIGNATURE = 3309;

    // ML-DSA-87 sizes (NIST Level 5)
    public static final int ML_DSA_87_PUBLIC_KEY = 2592;
    public static final int ML_DSA_87_PRIVATE_KEY = ML_DSA_SEED;  // Store seed for reconstruction
    public static final int ML_DSA_87_SIGNATURE = 4627;

    /**
     * Get expected public key size for algorithm
     */
    public static int getPublicKeySize(Algorithm algorithm) {
        switch (algorithm.getMLDSAVariant()) {
            case ML_DSA_44:
                return ML_DSA_44_PUBLIC_KEY;
            case ML_DSA_65:
                return ML_DSA_65_PUBLIC_KEY;
            case ML_DSA_87:
                return ML_DSA_87_PUBLIC_KEY;
            default:
                return ED25519_PUBLIC_KEY;
        }
    }

    /**
     * Get expected private key size for algorithm
     */
    public static int getPrivateKeySize(Algorithm algorithm) {
        switch (algorithm.getMLDSAVariant()) {
            case ML_DSA_44:
                return ML_DSA_44_PRIVATE_KEY;
            case ML_DSA_65:
                return ML_DSA_65_PRIVATE_KEY;
            case ML_DSA_87:
                return ML_DSA_87_PRIVATE_KEY;
            default:
                return ED25519_PRIVATE_KEY;
        }
    }

    /**
     * Get expected signature size for algorithm
     */
    public static int getSignatureSize(Algorithm algorithm) {
        switch (algorithm.getMLDSAVariant()) {
            case ML_DSA_44:
                return ML_DSA_44_SIGNATURE;
            case ML_DSA_65:
                return ML_DSA_65_SIGNATURE;
            case ML_DSA_87:
                return ML_DSA_87_SIGNATURE;
            default:
                return ED25519_SIGNATURE;
        }
    }

    /**
     * Validate public key size matches expected
     */
    public static boolean validatePublicKeySize(Algorithm algorithm, byte[] publicKey) {
        return publicKey != null && publicKey.length == getPublicKeySize(algorithm);
    }

    /**
     * Validate private key size matches expected
     */
    public static boolean validatePrivateKeySize(Algorithm algorithm, byte[] privateKey) {
        return privateKey != null && privateKey.length == getPrivateKeySize(algorithm);
    }

    /**
     * Validate signature size matches expected
     */
    public static boolean validateSignatureSize(Algorithm algorithm, byte[] signature) {
        return signature != null && signature.length == getSignatureSize(algorithm);
    }
}
