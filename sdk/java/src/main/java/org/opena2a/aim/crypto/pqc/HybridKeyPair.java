package org.opena2a.aim.crypto.pqc;

import java.time.Instant;
import java.util.Base64;
import java.util.Objects;

/**
 * Hybrid key pair containing both Ed25519 and ML-DSA keys.
 * Used for defense-in-depth cryptographic operations.
 */
public class HybridKeyPair {

    private final Algorithm algorithm;
    private final byte[] ed25519PublicKey;
    private final byte[] ed25519PrivateKey;
    private final byte[] mldsaPublicKey;
    private final byte[] mldsaPrivateKey;
    private final Algorithm mldsaVariant;
    private final Instant createdAt;

    public HybridKeyPair(
            Algorithm algorithm,
            byte[] ed25519PublicKey,
            byte[] ed25519PrivateKey,
            byte[] mldsaPublicKey,
            byte[] mldsaPrivateKey) {
        this(algorithm, ed25519PublicKey, ed25519PrivateKey, mldsaPublicKey, mldsaPrivateKey, Instant.now());
    }

    public HybridKeyPair(
            Algorithm algorithm,
            byte[] ed25519PublicKey,
            byte[] ed25519PrivateKey,
            byte[] mldsaPublicKey,
            byte[] mldsaPrivateKey,
            Instant createdAt) {

        if (!algorithm.isHybrid()) {
            throw new IllegalArgumentException("Algorithm must be a hybrid algorithm: " + algorithm);
        }

        this.algorithm = Objects.requireNonNull(algorithm, "algorithm");
        this.ed25519PublicKey = Objects.requireNonNull(ed25519PublicKey, "ed25519PublicKey").clone();
        this.ed25519PrivateKey = Objects.requireNonNull(ed25519PrivateKey, "ed25519PrivateKey").clone();
        this.mldsaPublicKey = Objects.requireNonNull(mldsaPublicKey, "mldsaPublicKey").clone();
        this.mldsaPrivateKey = Objects.requireNonNull(mldsaPrivateKey, "mldsaPrivateKey").clone();
        this.mldsaVariant = algorithm.getMLDSAVariant();
        this.createdAt = Objects.requireNonNull(createdAt, "createdAt");
    }

    public Algorithm getAlgorithm() {
        return algorithm;
    }

    public byte[] getEd25519PublicKey() {
        return ed25519PublicKey.clone();
    }

    public byte[] getEd25519PrivateKey() {
        return ed25519PrivateKey.clone();
    }

    public byte[] getMldsaPublicKey() {
        return mldsaPublicKey.clone();
    }

    public byte[] getMldsaPrivateKey() {
        return mldsaPrivateKey.clone();
    }

    public Algorithm getMldsaVariant() {
        return mldsaVariant;
    }

    public Instant getCreatedAt() {
        return createdAt;
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
     * Get the hybrid public key for verification
     */
    public HybridPublicKey getPublicKey() {
        return new HybridPublicKey(ed25519PublicKey, mldsaPublicKey, mldsaVariant);
    }

    @Override
    public String toString() {
        return String.format("HybridKeyPair[algorithm=%s, mldsaVariant=%s, createdAt=%s]",
                algorithm, mldsaVariant, createdAt);
    }
}
