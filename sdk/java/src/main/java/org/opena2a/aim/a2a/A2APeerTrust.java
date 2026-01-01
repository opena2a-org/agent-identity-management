package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * A2A Peer Trust - Trust relationship between two agents.
 * Represents bidirectional trust with verification status.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2APeerTrust {

    @JsonProperty("id")
    private String id;

    // Backend field names (primary)
    @JsonProperty("agentId")
    private String agentId;

    @JsonProperty("peerAgentId")
    private String peerAgentId;

    @JsonProperty("peerTrustScore")
    private Double peerTrustScore;

    @JsonProperty("successRate")
    private Double successRate;

    // Interaction stats from backend
    @JsonProperty("tasksInitiated")
    private int tasksInitiated;

    @JsonProperty("tasksInitiatedCompleted")
    private int tasksInitiatedCompleted;

    @JsonProperty("tasksReceived")
    private int tasksReceived;

    @JsonProperty("tasksReceivedCompleted")
    private int tasksReceivedCompleted;

    @JsonProperty("firstInteractionAt")
    private Instant firstInteractionAt;

    @JsonProperty("lastInteractionAt")
    private Instant lastInteractionAt;

    // Legacy SDK field names (for compatibility)
    @JsonProperty("sourceAgentId")
    private String sourceAgentId;

    @JsonProperty("targetAgentId")
    private String targetAgentId;

    @JsonProperty("trustLevel")
    private TrustLevel trustLevel;

    @JsonProperty("trustScore")
    private A2ATrustScore trustScore;

    @JsonProperty("verified")
    private boolean verified;

    @JsonProperty("verifiedAt")
    private Instant verifiedAt;

    @JsonProperty("bidirectional")
    private boolean bidirectional;

    @JsonProperty("allowedCapabilities")
    private List<String> allowedCapabilities;

    @JsonProperty("restrictions")
    private Map<String, Object> restrictions;

    @JsonProperty("createdAt")
    private Instant createdAt;

    @JsonProperty("updatedAt")
    private Instant updatedAt;

    // Default constructor for Jackson
    public A2APeerTrust() {}

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters - prefer backend fields, fall back to legacy
    public String getId() { return id; }

    /**
     * Get the agent ID. Uses backend field 'agentId' or legacy 'sourceAgentId'.
     */
    public String getAgentId() {
        return agentId != null ? agentId : sourceAgentId;
    }

    /**
     * Get the peer agent ID. Uses backend field 'peerAgentId' or legacy 'targetAgentId'.
     */
    public String getPeerAgentId() {
        return peerAgentId != null ? peerAgentId : targetAgentId;
    }

    /**
     * Get the peer trust score. Uses backend field 'peerTrustScore' or legacy nested 'trustScore'.
     */
    public Double getPeerTrustScore() {
        if (peerTrustScore != null) {
            return peerTrustScore;
        }
        return trustScore != null ? trustScore.getScore() : null;
    }

    public Double getSuccessRate() { return successRate; }
    public int getTasksInitiated() { return tasksInitiated; }
    public int getTasksInitiatedCompleted() { return tasksInitiatedCompleted; }
    public int getTasksReceived() { return tasksReceived; }
    public int getTasksReceivedCompleted() { return tasksReceivedCompleted; }
    public Instant getFirstInteractionAt() { return firstInteractionAt; }
    public Instant getLastInteractionAt() { return lastInteractionAt; }

    // Legacy getters (deprecated, use getAgentId/getPeerAgentId instead)
    @Deprecated
    public String getSourceAgentId() { return getAgentId(); }
    @Deprecated
    public String getTargetAgentId() { return getPeerAgentId(); }
    public TrustLevel getTrustLevel() { return trustLevel; }
    public A2ATrustScore getTrustScore() { return trustScore; }
    public boolean isVerified() { return verified; }
    public Instant getVerifiedAt() { return verifiedAt; }
    public boolean isBidirectional() { return bidirectional; }
    public List<String> getAllowedCapabilities() { return allowedCapabilities; }
    public Map<String, Object> getRestrictions() { return restrictions; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setAgentId(String agentId) { this.agentId = agentId; }
    public void setPeerAgentId(String peerAgentId) { this.peerAgentId = peerAgentId; }
    public void setPeerTrustScore(Double peerTrustScore) { this.peerTrustScore = peerTrustScore; }
    public void setSuccessRate(Double successRate) { this.successRate = successRate; }
    public void setTasksInitiated(int tasksInitiated) { this.tasksInitiated = tasksInitiated; }
    public void setTasksInitiatedCompleted(int tasksInitiatedCompleted) { this.tasksInitiatedCompleted = tasksInitiatedCompleted; }
    public void setTasksReceived(int tasksReceived) { this.tasksReceived = tasksReceived; }
    public void setTasksReceivedCompleted(int tasksReceivedCompleted) { this.tasksReceivedCompleted = tasksReceivedCompleted; }
    public void setFirstInteractionAt(Instant firstInteractionAt) { this.firstInteractionAt = firstInteractionAt; }
    public void setLastInteractionAt(Instant lastInteractionAt) { this.lastInteractionAt = lastInteractionAt; }
    // Legacy setters
    public void setSourceAgentId(String sourceAgentId) { this.sourceAgentId = sourceAgentId; }
    public void setTargetAgentId(String targetAgentId) { this.targetAgentId = targetAgentId; }
    public void setTrustLevel(TrustLevel trustLevel) { this.trustLevel = trustLevel; }
    public void setTrustScore(A2ATrustScore trustScore) { this.trustScore = trustScore; }
    public void setVerified(boolean verified) { this.verified = verified; }
    public void setVerifiedAt(Instant verifiedAt) { this.verifiedAt = verifiedAt; }
    public void setBidirectional(boolean bidirectional) { this.bidirectional = bidirectional; }
    public void setAllowedCapabilities(List<String> allowedCapabilities) { this.allowedCapabilities = allowedCapabilities; }
    public void setRestrictions(Map<String, Object> restrictions) { this.restrictions = restrictions; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Check if a capability is allowed by this peer trust.
     */
    public boolean isCapabilityAllowed(String capability) {
        if (allowedCapabilities == null || allowedCapabilities.isEmpty()) {
            return true; // No restrictions means all allowed
        }
        return allowedCapabilities.contains(capability) || allowedCapabilities.contains("*");
    }

    /**
     * Trust level enumeration.
     */
    public enum TrustLevel {
        @JsonProperty("none")
        NONE,

        @JsonProperty("low")
        LOW,

        @JsonProperty("medium")
        MEDIUM,

        @JsonProperty("high")
        HIGH,

        @JsonProperty("full")
        FULL
    }

    /**
     * Builder for A2APeerTrust.
     */
    public static class Builder {
        private final A2APeerTrust trust = new A2APeerTrust();

        public Builder id(String id) { trust.id = id; return this; }
        public Builder sourceAgentId(String sourceAgentId) { trust.sourceAgentId = sourceAgentId; return this; }
        public Builder targetAgentId(String targetAgentId) { trust.targetAgentId = targetAgentId; return this; }
        public Builder trustLevel(TrustLevel trustLevel) { trust.trustLevel = trustLevel; return this; }
        public Builder trustScore(A2ATrustScore trustScore) { trust.trustScore = trustScore; return this; }
        public Builder verified(boolean verified) { trust.verified = verified; return this; }
        public Builder verifiedAt(Instant verifiedAt) { trust.verifiedAt = verifiedAt; return this; }
        public Builder bidirectional(boolean bidirectional) { trust.bidirectional = bidirectional; return this; }
        public Builder allowedCapabilities(List<String> allowedCapabilities) { trust.allowedCapabilities = allowedCapabilities; return this; }
        public Builder restrictions(Map<String, Object> restrictions) { trust.restrictions = restrictions; return this; }

        public A2APeerTrust build() { return trust; }
    }

    @Override
    public String toString() {
        return "A2APeerTrust{" +
                "sourceAgentId='" + sourceAgentId + '\'' +
                ", targetAgentId='" + targetAgentId + '\'' +
                ", trustLevel=" + trustLevel +
                ", verified=" + verified +
                ", bidirectional=" + bidirectional +
                '}';
    }
}
