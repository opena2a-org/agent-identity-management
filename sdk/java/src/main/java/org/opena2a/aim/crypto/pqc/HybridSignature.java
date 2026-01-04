package org.opena2a.aim.crypto.pqc;

import java.util.Base64;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

/**
 * Hybrid signature containing both Ed25519 and ML-DSA signatures.
 */
public class HybridSignature {

    private final Algorithm algorithm;
    private final byte[] ed25519Signature;
    private final byte[] mldsaSignature;
    private final long timestamp;

    public HybridSignature(Algorithm algorithm, byte[] ed25519Signature, byte[] mldsaSignature) {
        this(algorithm, ed25519Signature, mldsaSignature, System.currentTimeMillis());
    }

    public HybridSignature(Algorithm algorithm, byte[] ed25519Signature, byte[] mldsaSignature, long timestamp) {
        if (!algorithm.isHybrid()) {
            throw new IllegalArgumentException("Algorithm must be a hybrid algorithm: " + algorithm);
        }
        this.algorithm = Objects.requireNonNull(algorithm, "algorithm");
        this.ed25519Signature = Objects.requireNonNull(ed25519Signature, "ed25519Signature").clone();
        this.mldsaSignature = Objects.requireNonNull(mldsaSignature, "mldsaSignature").clone();
        this.timestamp = timestamp;
    }

    public Algorithm getAlgorithm() {
        return algorithm;
    }

    public byte[] getEd25519Signature() {
        return ed25519Signature.clone();
    }

    public byte[] getMldsaSignature() {
        return mldsaSignature.clone();
    }

    public long getTimestamp() {
        return timestamp;
    }

    /**
     * Get Ed25519 signature as Base64 encoded string
     */
    public String getEd25519SignatureBase64() {
        return Base64.getEncoder().encodeToString(ed25519Signature);
    }

    /**
     * Get ML-DSA signature as Base64 encoded string
     */
    public String getMldsaSignatureBase64() {
        return Base64.getEncoder().encodeToString(mldsaSignature);
    }

    /**
     * Encode signature for transmission (JSON-compatible map)
     */
    public Map<String, Object> encode() {
        Map<String, Object> encoded = new HashMap<>();
        encoded.put("alg", algorithm.getAlgorithmId());
        encoded.put("ed25519Sig", getEd25519SignatureBase64());
        encoded.put("mldsaSig", getMldsaSignatureBase64());
        encoded.put("ts", timestamp);
        return encoded;
    }

    /**
     * Decode signature from transmission format
     */
    public static HybridSignature decode(Map<String, Object> encoded) {
        Algorithm algorithm = Algorithm.fromString((String) encoded.get("alg"));
        byte[] ed25519Sig = Base64.getDecoder().decode((String) encoded.get("ed25519Sig"));
        byte[] mldsaSig = Base64.getDecoder().decode((String) encoded.get("mldsaSig"));
        long timestamp = ((Number) encoded.get("ts")).longValue();
        return new HybridSignature(algorithm, ed25519Sig, mldsaSig, timestamp);
    }

    @Override
    public String toString() {
        return String.format("HybridSignature[algorithm=%s, timestamp=%d]", algorithm, timestamp);
    }
}
