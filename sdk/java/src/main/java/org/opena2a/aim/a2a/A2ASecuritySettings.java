package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.List;

/**
 * A2A security settings for an organization.
 *
 * <p>Controls security enforcement for agent-to-agent communications.
 * Two enforcement modes are available:</p>
 * <ul>
 *   <li><b>monitor</b> - Log violations but allow requests (default)</li>
 *   <li><b>strict</b> - Block requests that violate security policies</li>
 * </ul>
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ASecuritySettings {

    @JsonProperty("id")
    private String id;

    @JsonProperty("organizationId")
    private String organizationId;

    @JsonProperty("enforcementMode")
    private String enforcementMode;

    @JsonProperty("requireVerifiedSkills")
    private boolean requireVerifiedSkills;

    @JsonProperty("minTrustScore")
    private double minTrustScore;

    @JsonProperty("requireSignedRequests")
    private boolean requireSignedRequests;

    @JsonProperty("requireNonceValidation")
    private boolean requireNonceValidation;

    @JsonProperty("maxTimestampAge")
    private int maxTimestampAge;

    @JsonProperty("blockedAgentIds")
    private List<String> blockedAgentIds;

    @JsonProperty("allowedAgentIds")
    private List<String> allowedAgentIds;

    @JsonProperty("createdAt")
    private Instant createdAt;

    @JsonProperty("updatedAt")
    private Instant updatedAt;

    // Default constructor for Jackson
    public A2ASecuritySettings() {}

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters
    public String getId() { return id; }
    public String getOrganizationId() { return organizationId; }
    public String getEnforcementMode() { return enforcementMode; }
    public boolean isRequireVerifiedSkills() { return requireVerifiedSkills; }
    public double getMinTrustScore() { return minTrustScore; }
    public boolean isRequireSignedRequests() { return requireSignedRequests; }
    public boolean isRequireNonceValidation() { return requireNonceValidation; }
    public int getMaxTimestampAge() { return maxTimestampAge; }
    public List<String> getBlockedAgentIds() { return blockedAgentIds; }
    public List<String> getAllowedAgentIds() { return allowedAgentIds; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setOrganizationId(String organizationId) { this.organizationId = organizationId; }
    public void setEnforcementMode(String enforcementMode) { this.enforcementMode = enforcementMode; }
    public void setRequireVerifiedSkills(boolean requireVerifiedSkills) { this.requireVerifiedSkills = requireVerifiedSkills; }
    public void setMinTrustScore(double minTrustScore) { this.minTrustScore = minTrustScore; }
    public void setRequireSignedRequests(boolean requireSignedRequests) { this.requireSignedRequests = requireSignedRequests; }
    public void setRequireNonceValidation(boolean requireNonceValidation) { this.requireNonceValidation = requireNonceValidation; }
    public void setMaxTimestampAge(int maxTimestampAge) { this.maxTimestampAge = maxTimestampAge; }
    public void setBlockedAgentIds(List<String> blockedAgentIds) { this.blockedAgentIds = blockedAgentIds; }
    public void setAllowedAgentIds(List<String> allowedAgentIds) { this.allowedAgentIds = allowedAgentIds; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Check if enforcement mode is strict.
     */
    public boolean isStrictMode() {
        return "strict".equalsIgnoreCase(enforcementMode);
    }

    /**
     * Check if enforcement mode is monitor.
     */
    public boolean isMonitorMode() {
        return "monitor".equalsIgnoreCase(enforcementMode);
    }

    /**
     * Builder for A2ASecuritySettings.
     */
    public static class Builder {
        private final A2ASecuritySettings settings = new A2ASecuritySettings();

        public Builder id(String id) { settings.id = id; return this; }
        public Builder organizationId(String organizationId) { settings.organizationId = organizationId; return this; }
        public Builder enforcementMode(String enforcementMode) { settings.enforcementMode = enforcementMode; return this; }
        public Builder requireVerifiedSkills(boolean requireVerifiedSkills) { settings.requireVerifiedSkills = requireVerifiedSkills; return this; }
        public Builder minTrustScore(double minTrustScore) { settings.minTrustScore = minTrustScore; return this; }
        public Builder requireSignedRequests(boolean requireSignedRequests) { settings.requireSignedRequests = requireSignedRequests; return this; }
        public Builder requireNonceValidation(boolean requireNonceValidation) { settings.requireNonceValidation = requireNonceValidation; return this; }
        public Builder maxTimestampAge(int maxTimestampAge) { settings.maxTimestampAge = maxTimestampAge; return this; }
        public Builder blockedAgentIds(List<String> blockedAgentIds) { settings.blockedAgentIds = blockedAgentIds; return this; }
        public Builder allowedAgentIds(List<String> allowedAgentIds) { settings.allowedAgentIds = allowedAgentIds; return this; }

        public A2ASecuritySettings build() { return settings; }
    }

    @Override
    public String toString() {
        return "A2ASecuritySettings{" +
                "enforcementMode='" + enforcementMode + '\'' +
                ", requireVerifiedSkills=" + requireVerifiedSkills +
                ", minTrustScore=" + minTrustScore +
                ", requireSignedRequests=" + requireSignedRequests +
                '}';
    }
}
