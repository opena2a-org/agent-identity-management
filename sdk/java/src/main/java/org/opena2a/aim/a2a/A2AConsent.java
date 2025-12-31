package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * A2A Consent - GDPR/PSD2 compliant consent record for cross-agent data sharing.
 * Tracks user consent for data operations between agents.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2AConsent {

    @JsonProperty("id")
    private String id;

    @JsonProperty("userId")
    private String userId;

    @JsonProperty("sourceAgentId")
    private String sourceAgentId;

    @JsonProperty("targetAgentId")
    private String targetAgentId;

    @JsonProperty("purpose")
    private String purpose;

    @JsonProperty("dataTypes")
    private List<String> dataTypes;

    @JsonProperty("status")
    private ConsentStatus status;

    @JsonProperty("grantedAt")
    private Instant grantedAt;

    @JsonProperty("expiresAt")
    private Instant expiresAt;

    @JsonProperty("revokedAt")
    private Instant revokedAt;

    @JsonProperty("revocationReason")
    private String revocationReason;

    @JsonProperty("legalBasis")
    private String legalBasis;

    @JsonProperty("retentionPeriod")
    private String retentionPeriod;

    @JsonProperty("metadata")
    private Map<String, Object> metadata;

    @JsonProperty("createdAt")
    private Instant createdAt;

    @JsonProperty("updatedAt")
    private Instant updatedAt;

    // Default constructor for Jackson
    public A2AConsent() {}

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters
    public String getId() { return id; }
    public String getUserId() { return userId; }
    public String getSourceAgentId() { return sourceAgentId; }
    public String getTargetAgentId() { return targetAgentId; }
    public String getPurpose() { return purpose; }
    public List<String> getDataTypes() { return dataTypes; }
    public ConsentStatus getStatus() { return status; }
    public Instant getGrantedAt() { return grantedAt; }
    public Instant getExpiresAt() { return expiresAt; }
    public Instant getRevokedAt() { return revokedAt; }
    public String getRevocationReason() { return revocationReason; }
    public String getLegalBasis() { return legalBasis; }
    public String getRetentionPeriod() { return retentionPeriod; }
    public Map<String, Object> getMetadata() { return metadata; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setUserId(String userId) { this.userId = userId; }
    public void setSourceAgentId(String sourceAgentId) { this.sourceAgentId = sourceAgentId; }
    public void setTargetAgentId(String targetAgentId) { this.targetAgentId = targetAgentId; }
    public void setPurpose(String purpose) { this.purpose = purpose; }
    public void setDataTypes(List<String> dataTypes) { this.dataTypes = dataTypes; }
    public void setStatus(ConsentStatus status) { this.status = status; }
    public void setGrantedAt(Instant grantedAt) { this.grantedAt = grantedAt; }
    public void setExpiresAt(Instant expiresAt) { this.expiresAt = expiresAt; }
    public void setRevokedAt(Instant revokedAt) { this.revokedAt = revokedAt; }
    public void setRevocationReason(String revocationReason) { this.revocationReason = revocationReason; }
    public void setLegalBasis(String legalBasis) { this.legalBasis = legalBasis; }
    public void setRetentionPeriod(String retentionPeriod) { this.retentionPeriod = retentionPeriod; }
    public void setMetadata(Map<String, Object> metadata) { this.metadata = metadata; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Check if consent is currently active.
     */
    public boolean isActive() {
        if (status != ConsentStatus.GRANTED) {
            return false;
        }
        if (expiresAt != null && Instant.now().isAfter(expiresAt)) {
            return false;
        }
        return true;
    }

    /**
     * Check if consent is expired.
     */
    public boolean isExpired() {
        return expiresAt != null && Instant.now().isAfter(expiresAt);
    }

    /**
     * Check if a specific data type is covered by this consent.
     */
    public boolean coversDataType(String dataType) {
        if (dataTypes == null || dataTypes.isEmpty()) {
            return false;
        }
        return dataTypes.contains(dataType) || dataTypes.contains("*");
    }

    /**
     * Consent status enumeration.
     */
    public enum ConsentStatus {
        @JsonProperty("pending")
        PENDING,

        @JsonProperty("granted")
        GRANTED,

        @JsonProperty("denied")
        DENIED,

        @JsonProperty("revoked")
        REVOKED,

        @JsonProperty("expired")
        EXPIRED
    }

    /**
     * Builder for A2AConsent.
     */
    public static class Builder {
        private final A2AConsent consent = new A2AConsent();

        public Builder id(String id) { consent.id = id; return this; }
        public Builder userId(String userId) { consent.userId = userId; return this; }
        public Builder sourceAgentId(String sourceAgentId) { consent.sourceAgentId = sourceAgentId; return this; }
        public Builder targetAgentId(String targetAgentId) { consent.targetAgentId = targetAgentId; return this; }
        public Builder purpose(String purpose) { consent.purpose = purpose; return this; }
        public Builder dataTypes(List<String> dataTypes) { consent.dataTypes = dataTypes; return this; }
        public Builder status(ConsentStatus status) { consent.status = status; return this; }
        public Builder grantedAt(Instant grantedAt) { consent.grantedAt = grantedAt; return this; }
        public Builder expiresAt(Instant expiresAt) { consent.expiresAt = expiresAt; return this; }
        public Builder revokedAt(Instant revokedAt) { consent.revokedAt = revokedAt; return this; }
        public Builder revocationReason(String revocationReason) { consent.revocationReason = revocationReason; return this; }
        public Builder legalBasis(String legalBasis) { consent.legalBasis = legalBasis; return this; }
        public Builder retentionPeriod(String retentionPeriod) { consent.retentionPeriod = retentionPeriod; return this; }
        public Builder metadata(Map<String, Object> metadata) { consent.metadata = metadata; return this; }

        public A2AConsent build() { return consent; }
    }

    @Override
    public String toString() {
        return "A2AConsent{" +
                "id='" + id + '\'' +
                ", userId='" + userId + '\'' +
                ", sourceAgentId='" + sourceAgentId + '\'' +
                ", targetAgentId='" + targetAgentId + '\'' +
                ", purpose='" + purpose + '\'' +
                ", status=" + status +
                ", isActive=" + isActive() +
                '}';
    }
}
