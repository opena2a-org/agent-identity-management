package org.opena2a.aim.crypto.pqc;

import java.util.Base64;
import java.util.Objects;

/**
 * Hybrid public key for verification containing both Ed25519 and ML-DSA public keys.
 */
public class HybridPublicKey {

    private final byte[] ed25519PublicKey;
    private final byte[] mldsaPublicKey;
    private final Algorithm mldsaVariant;

    public HybridPublicKey(byte[] ed25519PublicKey, byte[] mldsaPublicKey, Algorithm mldsaVariant) {
        this.ed25519PublicKey = Objects.requireNonNull(ed25519PublicKey, "ed25519PublicKey").clone();
        this.mldsaPublicKey = Objects.requireNonNull(mldsaPublicKey, "mldsaPublicKey").clone();
        this.mldsaVariant = Objects.requireNonNull(mldsaVariant, "mldsaVariant");
    }

    public byte[] getEd25519PublicKey() {
        return ed25519PublicKey.clone();
    }

    public byte[] getMldsaPublicKey() {
        return mldsaPublicKey.clone();
    }

    public Algorithm getMldsaVariant() {
        return mldsaVariant;
    }

    /**
     * Get Ed25519 public key as Base64 encoded string
     */
    public String getEd25519PublicKeyBase64() {
        return Base64.getEncoder().encodeToString(ed25519PublicKey);
    }

    /**
     * Get ML-DSA public key as Base64 encoded string
     */
    public String getMldsaPublicKeyBase64() {
        return Base64.getEncoder().encodeToString(mldsaPublicKey);
    }

    /**
     * Get the full algorithm identifier
     */
    public Algorithm getAlgorithm() {
        switch (mldsaVariant) {
            case ML_DSA_44:
                return Algorithm.ED25519_ML_DSA_44;
            case ML_DSA_65:
                return Algorithm.ED25519_ML_DSA_65;
            case ML_DSA_87:
                return Algorithm.ED25519_ML_DSA_87;
            default:
                return Algorithm.ED25519_ML_DSA_65;
        }
    }

    /**
     * Create from Base64 encoded strings
     */
    public static HybridPublicKey fromBase64(String ed25519PublicKey, String mldsaPublicKey, Algorithm mldsaVariant) {
        return new HybridPublicKey(
                Base64.getDecoder().decode(ed25519PublicKey),
                Base64.getDecoder().decode(mldsaPublicKey),
                mldsaVariant
        );
    }

    @Override
    public String toString() {
        return String.format("HybridPublicKey[algorithm=%s, ed25519Size=%d, mldsaSize=%d]",
                getAlgorithm(), ed25519PublicKey.length, mldsaPublicKey.length);
    }
}
