package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnore;
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
 *
 * <p>The members carry the names the server reads and answers with. The server replaces every
 * setting with the ones an update carries, and stores a member the update leaves out, or sends as
 * null, as false, 0, an empty string or no agent IDs. To change some settings, read them with
 * {@link A2AClient#getSecuritySettings()}, change them on that object, and pass it to
 * {@link A2AClient#updateSecuritySettings(A2ASecuritySettings)}.</p>
 *
 * <p>{@code id}, {@code organizationId}, {@code createdBy}, {@code updatedBy}, {@code createdAt}
 * and {@code updatedAt} describe the stored record. They are read from the server's answer and
 * are not sent in an update.</p>
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ASecuritySettings {

    @JsonProperty(value = "id", access = JsonProperty.Access.WRITE_ONLY)
    private String id;

    @JsonProperty(value = "organizationId", access = JsonProperty.Access.WRITE_ONLY)
    private String organizationId;

    @JsonProperty("enforcementMode")
    private String enforcementMode;

    @JsonProperty("requireVerifiedSkills")
    private boolean requireVerifiedSkills;

    @JsonProperty("minTrustScore")
    private double minTrustScore;

    @JsonProperty("minAttestationCount")
    private int minAttestationCount;

    @JsonProperty("requireRequestSigning")
    private boolean requireRequestSigning;

    @JsonProperty("requireNonceValidation")
    private boolean requireNonceValidation;

    @JsonProperty("nonceValiditySeconds")
    private int nonceValiditySeconds;

    @JsonProperty("allowedAgentIds")
    private List<String> allowedAgentIds;

    @JsonProperty("blockedAgentIds")
    private List<String> blockedAgentIds;

    @JsonProperty("maxRequestsPerMinute")
    private int maxRequestsPerMinute;

    @JsonProperty("maxRequestsPerHour")
    private int maxRequestsPerHour;

    @JsonProperty(value = "createdBy", access = JsonProperty.Access.WRITE_ONLY)
    private String createdBy;

    @JsonProperty(value = "updatedBy", access = JsonProperty.Access.WRITE_ONLY)
    private String updatedBy;

    @JsonProperty(value = "createdAt", access = JsonProperty.Access.WRITE_ONLY)
    private Instant createdAt;

    @JsonProperty(value = "updatedAt", access = JsonProperty.Access.WRITE_ONLY)
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
    public int getMinAttestationCount() { return minAttestationCount; }
    public boolean isRequireRequestSigning() { return requireRequestSigning; }
    public boolean isRequireNonceValidation() { return requireNonceValidation; }
    public int getNonceValiditySeconds() { return nonceValiditySeconds; }
    public List<String> getAllowedAgentIds() { return allowedAgentIds; }
    public List<String> getBlockedAgentIds() { return blockedAgentIds; }
    public int getMaxRequestsPerMinute() { return maxRequestsPerMinute; }
    public int getMaxRequestsPerHour() { return maxRequestsPerHour; }
    public String getCreatedBy() { return createdBy; }
    public String getUpdatedBy() { return updatedBy; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setOrganizationId(String organizationId) { this.organizationId = organizationId; }
    public void setEnforcementMode(String enforcementMode) { this.enforcementMode = enforcementMode; }
    public void setRequireVerifiedSkills(boolean requireVerifiedSkills) { this.requireVerifiedSkills = requireVerifiedSkills; }
    public void setMinTrustScore(double minTrustScore) { this.minTrustScore = minTrustScore; }
    public void setMinAttestationCount(int minAttestationCount) { this.minAttestationCount = minAttestationCount; }
    public void setRequireRequestSigning(boolean requireRequestSigning) { this.requireRequestSigning = requireRequestSigning; }
    public void setRequireNonceValidation(boolean requireNonceValidation) { this.requireNonceValidation = requireNonceValidation; }
    public void setNonceValiditySeconds(int nonceValiditySeconds) { this.nonceValiditySeconds = nonceValiditySeconds; }
    public void setAllowedAgentIds(List<String> allowedAgentIds) { this.allowedAgentIds = allowedAgentIds; }
    public void setBlockedAgentIds(List<String> blockedAgentIds) { this.blockedAgentIds = blockedAgentIds; }
    public void setMaxRequestsPerMinute(int maxRequestsPerMinute) { this.maxRequestsPerMinute = maxRequestsPerMinute; }
    public void setMaxRequestsPerHour(int maxRequestsPerHour) { this.maxRequestsPerHour = maxRequestsPerHour; }
    public void setCreatedBy(String createdBy) { this.createdBy = createdBy; }
    public void setUpdatedBy(String updatedBy) { this.updatedBy = updatedBy; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Check if enforcement mode is strict.
     */
    @JsonIgnore
    public boolean isStrictMode() {
        return "strict".equalsIgnoreCase(enforcementMode);
    }

    /**
     * Check if enforcement mode is monitor.
     */
    @JsonIgnore
    public boolean isMonitorMode() {
        return "monitor".equalsIgnoreCase(enforcementMode);
    }

    /**
     * Builder for A2ASecuritySettings. A settings member the builder is not given is sent as
     * false, 0 or null, and the server stores it as the class description says. {@code id} and
     * {@code organizationId} are not sent in an update.
     */
    public static class Builder {
        private final A2ASecuritySettings settings = new A2ASecuritySettings();

        public Builder id(String id) { settings.id = id; return this; }
        public Builder organizationId(String organizationId) { settings.organizationId = organizationId; return this; }
        public Builder enforcementMode(String enforcementMode) { settings.enforcementMode = enforcementMode; return this; }
        public Builder requireVerifiedSkills(boolean requireVerifiedSkills) { settings.requireVerifiedSkills = requireVerifiedSkills; return this; }
        public Builder minTrustScore(double minTrustScore) { settings.minTrustScore = minTrustScore; return this; }
        public Builder minAttestationCount(int minAttestationCount) { settings.minAttestationCount = minAttestationCount; return this; }
        public Builder requireRequestSigning(boolean requireRequestSigning) { settings.requireRequestSigning = requireRequestSigning; return this; }
        public Builder requireNonceValidation(boolean requireNonceValidation) { settings.requireNonceValidation = requireNonceValidation; return this; }
        public Builder nonceValiditySeconds(int nonceValiditySeconds) { settings.nonceValiditySeconds = nonceValiditySeconds; return this; }
        public Builder allowedAgentIds(List<String> allowedAgentIds) { settings.allowedAgentIds = allowedAgentIds; return this; }
        public Builder blockedAgentIds(List<String> blockedAgentIds) { settings.blockedAgentIds = blockedAgentIds; return this; }
        public Builder maxRequestsPerMinute(int maxRequestsPerMinute) { settings.maxRequestsPerMinute = maxRequestsPerMinute; return this; }
        public Builder maxRequestsPerHour(int maxRequestsPerHour) { settings.maxRequestsPerHour = maxRequestsPerHour; return this; }

        public A2ASecuritySettings build() { return settings; }
    }

    @Override
    public String toString() {
        return "A2ASecuritySettings{" +
                "enforcementMode='" + enforcementMode + '\'' +
                ", requireVerifiedSkills=" + requireVerifiedSkills +
                ", minTrustScore=" + minTrustScore +
                ", requireRequestSigning=" + requireRequestSigning +
                '}';
    }
}
