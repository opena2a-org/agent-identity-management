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

    public String getId() {
        return id;
    }

    public String getName() {
        return name;
    }

    public String getUrl() {
        return url;
    }

    public String getDescription() {
        return description;
    }

    public String getVersion() {
        return version;
    }

    public String getPublicKey() {
        return publicKey;
    }

    public List<String> getCapabilities() {
        return capabilities;
    }

    public String getStatus() {
        return status;
    }

    public double getTrustScore() {
        return trustScore;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public Instant getLastAttestationAt() {
        return lastAttestationAt;
    }

    public Map<String, Object> getMetadata() {
        return metadata;
    }

    public static Builder builder() {
        return new Builder();
    }

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

        public Builder id(String id) {
            this.id = id;
            return this;
        }

        public Builder name(String name) {
            this.name = name;
            return this;
        }

        public Builder url(String url) {
            this.url = url;
            return this;
        }

        public Builder description(String description) {
            this.description = description;
            return this;
        }

        public Builder version(String version) {
            this.version = version;
            return this;
        }

        public Builder publicKey(String publicKey) {
            this.publicKey = publicKey;
            return this;
        }

        public Builder capabilities(List<String> capabilities) {
            this.capabilities = capabilities;
            return this;
        }

        public Builder status(String status) {
            this.status = status;
            return this;
        }

        public Builder trustScore(double trustScore) {
            this.trustScore = trustScore;
            return this;
        }

        public Builder createdAt(Instant createdAt) {
            this.createdAt = createdAt;
            return this;
        }

        public Builder lastAttestationAt(Instant lastAttestationAt) {
            this.lastAttestationAt = lastAttestationAt;
            return this;
        }

        public Builder metadata(Map<String, Object> metadata) {
            this.metadata = metadata;
            return this;
        }

        public MCPServerInfo build() {
            return new MCPServerInfo(this);
        }
    }
}
