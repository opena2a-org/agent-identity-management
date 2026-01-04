package org.opena2a.aim.crypto.pqc;

/**
 * Supported cryptographic algorithms for AIM SDK.
 * Includes classical (Ed25519), post-quantum (ML-DSA), and hybrid algorithms.
 */
public enum Algorithm {
    /**
     * Classical Ed25519 signature algorithm
     */
    ED25519("Ed25519", false, false),

    /**
     * ML-DSA-44 (NIST Level 2) - Post-quantum signature
     */
    ML_DSA_44("ML-DSA-44", true, false),

    /**
     * ML-DSA-65 (NIST Level 3) - Post-quantum signature (recommended)
     */
    ML_DSA_65("ML-DSA-65", true, false),

    /**
     * ML-DSA-87 (NIST Level 5) - Post-quantum signature
     */
    ML_DSA_87("ML-DSA-87", true, false),

    /**
     * Hybrid Ed25519 + ML-DSA-44 (NIST Level 2)
     */
    ED25519_ML_DSA_44("Ed25519+ML-DSA-44", true, true),

    /**
     * Hybrid Ed25519 + ML-DSA-65 (NIST Level 3) - recommended
     */
    ED25519_ML_DSA_65("Ed25519+ML-DSA-65", true, true),

    /**
     * Hybrid Ed25519 + ML-DSA-87 (NIST Level 5)
     */
    ED25519_ML_DSA_87("Ed25519+ML-DSA-87", true, true);

    private final String algorithmId;
    private final boolean postQuantum;
    private final boolean hybrid;

    Algorithm(String algorithmId, boolean postQuantum, boolean hybrid) {
        this.algorithmId = algorithmId;
        this.postQuantum = postQuantum;
        this.hybrid = hybrid;
    }

    /**
     * Get the algorithm identifier string (e.g., "Ed25519+ML-DSA-65")
     */
    public String getAlgorithmId() {
        return algorithmId;
    }

    /**
     * Check if this is a post-quantum algorithm
     */
    public boolean isPostQuantum() {
        return postQuantum;
    }

    /**
     * Check if this is a hybrid algorithm (classical + PQC)
     */
    public boolean isHybrid() {
        return hybrid;
    }

    /**
     * Get the ML-DSA variant for this algorithm.
     * Returns the ML-DSA algorithm component for hybrid algorithms.
     */
    public Algorithm getMLDSAVariant() {
        switch (this) {
            case ML_DSA_44:
            case ED25519_ML_DSA_44:
                return ML_DSA_44;
            case ML_DSA_65:
            case ED25519_ML_DSA_65:
                return ML_DSA_65;
            case ML_DSA_87:
            case ED25519_ML_DSA_87:
                return ML_DSA_87;
            default:
                return ML_DSA_65; // Default to recommended
        }
    }

    /**
     * Parse algorithm from string identifier
     */
    public static Algorithm fromString(String algorithmId) {
        for (Algorithm alg : values()) {
            if (alg.algorithmId.equals(algorithmId)) {
                return alg;
            }
        }
        throw new IllegalArgumentException("Unknown algorithm: " + algorithmId);
    }

    @Override
    public String toString() {
        return algorithmId;
    }
}
