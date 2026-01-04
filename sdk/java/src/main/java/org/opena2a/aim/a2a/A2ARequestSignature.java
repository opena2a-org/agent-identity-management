package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.Map;

/**
 * A2A Request Signature - Cryptographic signature for A2A requests.
 * Provides non-repudiation and anti-replay protection.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ARequestSignature {

    @JsonProperty("signature")
    private String signature;

    // Backend returns timestamp as Unix epoch (long), but we expose as Instant
    private Instant timestamp;

    // For JSON deserialization of numeric timestamp from backend
    @JsonProperty("timestamp")
    private Long timestampEpoch;

    @JsonProperty("nonce")
    private String nonce;

    @JsonProperty("algorithm")
    private String algorithm;

    @JsonProperty("agentId")
    private String agentId;

    @JsonProperty("publicKey")
    private String publicKey;

    @JsonProperty("headers")
    private Map<String, String> headers;

    // Default constructor for Jackson
    public A2ARequestSignature() {}

    // Constructor for building signatures
    public A2ARequestSignature(String signature, Instant timestamp, String nonce,
                                String algorithm, String agentId, String publicKey) {
        this.signature = signature;
        this.timestamp = timestamp;
        this.nonce = nonce;
        this.algorithm = algorithm;
        this.agentId = agentId;
        this.publicKey = publicKey;
    }

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters
    public String getSignature() { return signature; }

    /**
     * Get timestamp as Instant. Handles both Instant and epoch formats from backend.
     */
    public Instant getTimestamp() {
        if (timestamp != null) {
            return timestamp;
        }
        if (timestampEpoch != null) {
            // Backend returns Unix epoch in seconds
            return Instant.ofEpochSecond(timestampEpoch);
        }
        return null;
    }

    /**
     * Get raw epoch timestamp if available.
     */
    public Long getTimestampEpoch() {
        if (timestampEpoch != null) {
            return timestampEpoch;
        }
        return timestamp != null ? timestamp.getEpochSecond() : null;
    }

    public String getNonce() { return nonce; }
    public String getAlgorithm() { return algorithm; }
    public String getAgentId() { return agentId; }
    public String getPublicKey() { return publicKey; }
    public Map<String, String> getHeaders() { return headers; }

    // Setters for Jackson
    public void setSignature(String signature) { this.signature = signature; }
    public void setTimestamp(Instant timestamp) { this.timestamp = timestamp; }
    public void setNonce(String nonce) { this.nonce = nonce; }
    public void setAlgorithm(String algorithm) { this.algorithm = algorithm; }
    public void setAgentId(String agentId) { this.agentId = agentId; }
    public void setPublicKey(String publicKey) { this.publicKey = publicKey; }
    public void setHeaders(Map<String, String> headers) { this.headers = headers; }

    /**
     * Builder for A2ARequestSignature.
     */
    public static class Builder {
        private final A2ARequestSignature sig = new A2ARequestSignature();

        public Builder signature(String signature) { sig.signature = signature; return this; }
        public Builder timestamp(Instant timestamp) { sig.timestamp = timestamp; return this; }
        public Builder nonce(String nonce) { sig.nonce = nonce; return this; }
        public Builder algorithm(String algorithm) { sig.algorithm = algorithm; return this; }
        public Builder agentId(String agentId) { sig.agentId = agentId; return this; }
        public Builder publicKey(String publicKey) { sig.publicKey = publicKey; return this; }
        public Builder headers(Map<String, String> headers) { sig.headers = headers; return this; }

        public A2ARequestSignature build() { return sig; }
    }

    @Override
    public String toString() {
        return "A2ARequestSignature{" +
                "agentId='" + agentId + '\'' +
                ", algorithm='" + algorithm + '\'' +
                ", timestamp=" + timestamp +
                ", nonce='" + nonce + '\'' +
                '}';
    }
}
