package org.opena2a.aim.integrations.mcp;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * Information about an MCP server registered with AIM.
 */
public class MCPServerInfo {

    private final String id;
    private final String name;
    private final String url;
    private final String description;
    private final String version;
    private final String publicKey;
    private final List<String> capabilities;
    private final String status;
    private final double trustScore;
    private final Instant createdAt;
    private final Instant lastAttestationAt;
    private final Map<String, Object> metadata;

    private MCPServerInfo(Builder builder) {
        this.id = builder.id;
        this.name = builder.name;
        this.url = builder.url;
        this.description = builder.description;
        this.version = builder.version;
        this.publicKey = builder.publicKey;
        this.capabilities = builder.capabilities;
        this.status = builder.status;
        this.trustScore = builder.trustScore;
        this.createdAt = builder.createdAt;
        this.lastAttestationAt = builder.lastAttestationAt;
        this.metadata = builder.metadata;
    }

    /**
     * Gets the unique server ID.
     *
     * @return the server ID
     */
    public String getId() {
        return id;
    }

    /**
     * Gets the server name.
     *
     * @return the server name
     */
    public String getName() {
        return name;
    }

    /**
     * Gets the server URL.
     *
     * @return the server URL
     */
    public String getUrl() {
        return url;
    }

    /**
     * Gets the server description.
     *
     * @return the description
     */
    public String getDescription() {
        return description;
    }

    /**
     * Gets the server version.
     *
     * @return the version string
     */
    public String getVersion() {
        return version;
    }

    /**
     * Gets the server's public key.
     *
     * @return the public key
     */
    public String getPublicKey() {
        return publicKey;
    }

    /**
     * Gets the list of server capabilities.
     *
     * @return the capabilities list
     */
    public List<String> getCapabilities() {
        return capabilities;
    }

    /**
     * Gets the server status.
     *
     * @return the status string
     */
    public String getStatus() {
        return status;
    }

    /**
     * Gets the server's trust score.
     *
     * @return the trust score (0.0 to 1.0)
     */
    public double getTrustScore() {
        return trustScore;
    }

    /**
     * Gets the timestamp when the server was created.
     *
     * @return the creation timestamp
     */
    public Instant getCreatedAt() {
        return createdAt;
    }

    /**
     * Gets the timestamp of the last attestation.
     *
     * @return the last attestation timestamp
     */
    public Instant getLastAttestationAt() {
        return lastAttestationAt;
    }

    /**
     * Gets the server metadata.
     *
     * @return the metadata map
     */
    public Map<String, Object> getMetadata() {
        return metadata;
    }

    /**
     * Creates a new builder for MCPServerInfo.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for creating MCPServerInfo instances.
     */
    public static class Builder {
        private String id;
        private String name;
        private String url;
        private String description;
        private String version;
        private String publicKey;
        private List<String> capabilities;
        private String status;
        private double trustScore;
        private Instant createdAt;
        private Instant lastAttestationAt;
        private Map<String, Object> metadata;

        /**
         * Sets the server ID.
         *
         * @param id the server ID
         * @return this builder
         */
        public Builder id(String id) {
            this.id = id;
            return this;
        }

        /**
         * Sets the server name.
         *
         * @param name the server name
         * @return this builder
         */
        public Builder name(String name) {
            this.name = name;
            return this;
        }

        /**
         * Sets the server URL.
         *
         * @param url the server URL
         * @return this builder
         */
        public Builder url(String url) {
            this.url = url;
            return this;
        }

        /**
         * Sets the server description.
         *
         * @param description the description
         * @return this builder
         */
        public Builder description(String description) {
            this.description = description;
            return this;
        }

        /**
         * Sets the server version.
         *
         * @param version the version string
         * @return this builder
         */
        public Builder version(String version) {
            this.version = version;
            return this;
        }

        /**
         * Sets the server's public key.
         *
         * @param publicKey the public key
         * @return this builder
         */
        public Builder publicKey(String publicKey) {
            this.publicKey = publicKey;
            return this;
        }

        /**
         * Sets the server capabilities.
         *
         * @param capabilities the capabilities list
         * @return this builder
         */
        public Builder capabilities(List<String> capabilities) {
            this.capabilities = capabilities;
            return this;
        }

        /**
         * Sets the server status.
         *
         * @param status the status string
         * @return this builder
         */
        public Builder status(String status) {
            this.status = status;
            return this;
        }

        /**
         * Sets the trust score.
         *
         * @param trustScore the trust score (0.0 to 1.0)
         * @return this builder
         */
        public Builder trustScore(double trustScore) {
            this.trustScore = trustScore;
            return this;
        }

        /**
         * Sets the creation timestamp.
         *
         * @param createdAt the creation timestamp
         * @return this builder
         */
        public Builder createdAt(Instant createdAt) {
            this.createdAt = createdAt;
            return this;
        }

        /**
         * Sets the last attestation timestamp.
         *
         * @param lastAttestationAt the last attestation timestamp
         * @return this builder
         */
        public Builder lastAttestationAt(Instant lastAttestationAt) {
            this.lastAttestationAt = lastAttestationAt;
            return this;
        }

        /**
         * Sets the server metadata.
         *
         * @param metadata the metadata map
         * @return this builder
         */
        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        /**
         * Builds the MCPServerInfo instance.
         *
         * @return the built MCPServerInfo
         */
        public MCPServerInfo build() {
            return new MCPServerInfo(this);
        }
    }
}
