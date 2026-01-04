package org.opena2a.aim.crypto.pqc;

import java.time.Instant;
import java.util.Base64;
import java.util.Objects;

/**
 * ML-DSA key pair for post-quantum cryptographic operations.
 */
public class MLDSAKeyPair {

    private final Algorithm algorithm;
    private final byte[] publicKey;
    private final byte[] privateKey;
    private final Instant createdAt;

    public MLDSAKeyPair(Algorithm algorithm, byte[] publicKey, byte[] privateKey) {
        this(algorithm, publicKey, privateKey, Instant.now());
    }

    public MLDSAKeyPair(Algorithm algorithm, byte[] publicKey, byte[] privateKey, Instant createdAt) {
        if (!algorithm.isPostQuantum() || algorithm.isHybrid()) {
            throw new IllegalArgumentException("Algorithm must be a pure ML-DSA variant: " + algorithm);
        }
        this.algorithm = Objects.requireNonNull(algorithm, "algorithm");
        this.publicKey = Objects.requireNonNull(publicKey, "publicKey").clone();
        this.privateKey = Objects.requireNonNull(privateKey, "privateKey").clone();
        this.createdAt = Objects.requireNonNull(createdAt, "createdAt");
    }

    public Algorithm getAlgorithm() {
        return algorithm;
    }

    public byte[] getPublicKey() {
        return publicKey.clone();
    }

    public byte[] getPrivateKey() {
        return privateKey.clone();
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    /**
     * Get public key as Base64 encoded string
     */
    public String getPublicKeyBase64() {
        return Base64.getEncoder().encodeToString(publicKey);
    }

    /**
     * Get private key as Base64 encoded string
     */
    public String getPrivateKeyBase64() {
        return Base64.getEncoder().encodeToString(privateKey);
    }

    @Override
    public String toString() {
        return String.format("MLDSAKeyPair[algorithm=%s, publicKeySize=%d, createdAt=%s]",
                algorithm, publicKey.length, createdAt);
    }
}
