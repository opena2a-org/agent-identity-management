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

    /**
     * Returns whether the attestation was successful.
     *
     * @return true if attestation succeeded, false otherwise
     */
    public boolean isSuccess() {
        return success;
    }

    /**
     * Gets the unique attestation ID.
     *
     * @return the attestation ID
     */
    public String getAttestationId() {
        return attestationId;
    }

    /**
     * Gets the MCP server ID that was attested.
     *
     * @return the MCP server ID
     */
    public String getMcpServerId() {
        return mcpServerId;
    }

    /**
     * Gets the confidence score of the attestation.
     *
     * @return the confidence score (0.0 to 1.0)
     */
    public double getConfidenceScore() {
        return confidenceScore;
    }

    /**
     * Gets the total number of attestations for this server.
     *
     * @return the attestation count
     */
    public int getAttestationCount() {
        return attestationCount;
    }

    /**
     * Gets the list of capabilities that were attested.
     *
     * @return the list of attested capabilities
     */
    public List<String> getCapabilitiesAttested() {
        return capabilitiesAttested;
    }

    /**
     * Gets the timestamp when the attestation occurred.
     *
     * @return the attestation timestamp
     */
    public Instant getAttestedAt() {
        return attestedAt;
    }

    /**
     * Gets the discovery information from the attestation.
     *
     * @return the discovery data map
     */
    public Map<String, Object> getDiscovery() {
        return discovery;
    }

    /**
     * Gets the error message if attestation failed.
     *
     * @return the error message, or null if successful
     */
    public String getError() {
        return error;
    }

    /**
     * Creates a new builder for AttestationResult.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for creating AttestationResult instances.
     */
    public static class Builder {
        /** Creates a new builder with default settings. */
        public Builder() {}

        private boolean success;
        private String attestationId;
        private String mcpServerId;
        private double confidenceScore;
        private int attestationCount;
        private List<String> capabilitiesAttested;
        private Instant attestedAt;
        private Map<String, Object> discovery;
        private String error;

        /**
         * Sets whether the attestation was successful.
         *
         * @param success true if successful
         * @return this builder
         */
        public Builder success(boolean success) {
            this.success = success;
            return this;
        }

        /**
         * Sets the attestation ID.
         *
         * @param attestationId the attestation ID
         * @return this builder
         */
        public Builder attestationId(String attestationId) {
            this.attestationId = attestationId;
            return this;
        }

        /**
         * Sets the MCP server ID.
         *
         * @param mcpServerId the MCP server ID
         * @return this builder
         */
        public Builder mcpServerId(String mcpServerId) {
            this.mcpServerId = mcpServerId;
            return this;
        }

        /**
         * Sets the confidence score.
         *
         * @param confidenceScore the confidence score (0.0 to 1.0)
         * @return this builder
         */
        public Builder confidenceScore(double confidenceScore) {
            this.confidenceScore = confidenceScore;
            return this;
        }

        /**
         * Sets the attestation count.
         *
         * @param attestationCount the attestation count
         * @return this builder
         */
        public Builder attestationCount(int attestationCount) {
            this.attestationCount = attestationCount;
            return this;
        }

        /**
         * Sets the list of attested capabilities.
         *
         * @param capabilitiesAttested the list of capabilities
         * @return this builder
         */
        public Builder capabilitiesAttested(List<String> capabilitiesAttested) {
            this.capabilitiesAttested = capabilitiesAttested;
            return this;
        }

        /**
         * Sets the attestation timestamp.
         *
         * @param attestedAt the timestamp
         * @return this builder
         */
        public Builder attestedAt(Instant attestedAt) {
            this.attestedAt = attestedAt;
            return this;
        }

        /**
         * Sets the discovery information.
         *
         * @param discovery the discovery data
         * @return this builder
         */
        public Builder discovery(Map<String, Object> discovery) {
            this.discovery = discovery;
            return this;
        }

        /**
         * Sets the error message.
         *
         * @param error the error message
         * @return this builder
         */
        public Builder error(String error) {
            this.error = error;
            return this;
        }

        /**
         * Builds the AttestationResult instance.
         *
         * @return the built AttestationResult
         */
        public AttestationResult build() {
            return new AttestationResult(this);
        }
    }
}
