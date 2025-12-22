package org.opena2a.aim.client;

import java.time.Instant;
import java.util.Map;

/**
 * Result of a capability verification request.
 */
public class VerificationResult {

    private final boolean verified;
    private final String status;
    private final String verificationId;
    private final String capability;
    private final String resource;
    private final Instant timestamp;
    private final Map<String, Object> metadata;

    public VerificationResult(boolean verified, String status, String verificationId,
                              String capability, String resource, Instant timestamp,
                              Map<String, Object> metadata) {
        this.verified = verified;
        this.status = status;
        this.verificationId = verificationId;
        this.capability = capability;
        this.resource = resource;
        this.timestamp = timestamp;
        this.metadata = metadata;
    }

    public boolean isVerified() {
        return verified;
    }

    public String getStatus() {
        return status;
    }

    public String getVerificationId() {
        return verificationId;
    }

    public String getCapability() {
        return capability;
    }

    public String getResource() {
        return resource;
    }

    public Instant getTimestamp() {
        return timestamp;
    }

    public Map<String, Object> getMetadata() {
        return metadata;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private boolean verified;
        private String status;
        private String verificationId;
        private String capability;
        private String resource;
        private Instant timestamp;
        private Map<String, Object> metadata;

        public Builder verified(boolean verified) {
            this.verified = verified;
            return this;
        }

        public Builder status(String status) {
            this.status = status;
            return this;
        }

        public Builder verificationId(String verificationId) {
            this.verificationId = verificationId;
            return this;
        }

        public Builder capability(String capability) {
            this.capability = capability;
            return this;
        }

        public Builder resource(String resource) {
            this.resource = resource;
            return this;
        }

        public Builder timestamp(Instant timestamp) {
            this.timestamp = timestamp;
            return this;
        }

        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        public VerificationResult build() {
            return new VerificationResult(verified, status, verificationId,
                    capability, resource, timestamp, metadata);
        }
    }
}
