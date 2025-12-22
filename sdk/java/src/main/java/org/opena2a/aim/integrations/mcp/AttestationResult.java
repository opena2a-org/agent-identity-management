package org.opena2a.aim.integrations.mcp;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * Result of an MCP server attestation.
 */
public class AttestationResult {

    private final boolean success;
    private final String attestationId;
    private final String mcpServerId;
    private final double confidenceScore;
    private final int attestationCount;
    private final List<String> capabilitiesAttested;
    private final Instant attestedAt;
    private final Map<String, Object> discovery;
    private final String error;

    private AttestationResult(Builder builder) {
        this.success = builder.success;
        this.attestationId = builder.attestationId;
        this.mcpServerId = builder.mcpServerId;
        this.confidenceScore = builder.confidenceScore;
        this.attestationCount = builder.attestationCount;
        this.capabilitiesAttested = builder.capabilitiesAttested;
        this.attestedAt = builder.attestedAt;
        this.discovery = builder.discovery;
        this.error = builder.error;
    }

    public boolean isSuccess() {
        return success;
    }

    public String getAttestationId() {
        return attestationId;
    }

    public String getMcpServerId() {
        return mcpServerId;
    }

    public double getConfidenceScore() {
        return confidenceScore;
    }

    public int getAttestationCount() {
        return attestationCount;
    }

    public List<String> getCapabilitiesAttested() {
        return capabilitiesAttested;
    }

    public Instant getAttestedAt() {
        return attestedAt;
    }

    public Map<String, Object> getDiscovery() {
        return discovery;
    }

    public String getError() {
        return error;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private boolean success;
        private String attestationId;
        private String mcpServerId;
        private double confidenceScore;
        private int attestationCount;
        private List<String> capabilitiesAttested;
        private Instant attestedAt;
        private Map<String, Object> discovery;
        private String error;

        public Builder success(boolean success) {
            this.success = success;
            return this;
        }

        public Builder attestationId(String attestationId) {
            this.attestationId = attestationId;
            return this;
        }

        public Builder mcpServerId(String mcpServerId) {
            this.mcpServerId = mcpServerId;
            return this;
        }

        public Builder confidenceScore(double confidenceScore) {
            this.confidenceScore = confidenceScore;
            return this;
        }

        public Builder attestationCount(int attestationCount) {
            this.attestationCount = attestationCount;
            return this;
        }

        public Builder capabilitiesAttested(List<String> capabilitiesAttested) {
            this.capabilitiesAttested = capabilitiesAttested;
            return this;
        }

        public Builder attestedAt(Instant attestedAt) {
            this.attestedAt = attestedAt;
            return this;
        }

        public Builder discovery(Map<String, Object> discovery) {
            this.discovery = discovery;
            return this;
        }

        public Builder error(String error) {
            this.error = error;
            return this;
        }

        public AttestationResult build() {
            return new AttestationResult(this);
        }
    }
}
