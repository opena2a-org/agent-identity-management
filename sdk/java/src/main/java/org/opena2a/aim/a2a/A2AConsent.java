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

    @JsonProperty("grantorAgentId")
    private String grantorAgentId;

    @JsonProperty("recipientAgentId")
    private String recipientAgentId;

    @JsonProperty("scope")
    private List<String> scope;

    @JsonProperty("purpose")
    private String purpose;

    @JsonProperty("dataTypes")
    private List<String> dataTypes;

    @JsonProperty("consentMethod")
    private String consentMethod;

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
    public String getGrantorAgentId() { return grantorAgentId; }
    public String getRecipientAgentId() { return recipientAgentId; }
    public List<String> getScope() { return scope; }
    public String getPurpose() { return purpose; }
    public List<String> getDataTypes() { return dataTypes; }
    public String getConsentMethod() { return consentMethod; }
    public ConsentStatus getStatus() { return status; }
    public Instant getGrantedAt() { return grantedAt; }
    public Instant getExpiresAt() { return expiresAt; }
    public Instant getRevokedAt() { return revokedAt; }
    public String getRevocationReason() { return revocationReason; }
    public String getRetentionPeriod() { return retentionPeriod; }
    public Map<String, Object> getMetadata() { return metadata; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setUserId(String userId) { this.userId = userId; }
    public void setGrantorAgentId(String grantorAgentId) { this.grantorAgentId = grantorAgentId; }
    public void setRecipientAgentId(String recipientAgentId) { this.recipientAgentId = recipientAgentId; }
    public void setScope(List<String> scope) { this.scope = scope; }
    public void setPurpose(String purpose) { this.purpose = purpose; }
    public void setDataTypes(List<String> dataTypes) { this.dataTypes = dataTypes; }
    public void setConsentMethod(String consentMethod) { this.consentMethod = consentMethod; }
    public void setStatus(ConsentStatus status) { this.status = status; }
    public void setGrantedAt(Instant grantedAt) { this.grantedAt = grantedAt; }
    public void setExpiresAt(Instant expiresAt) { this.expiresAt = expiresAt; }
    public void setRevokedAt(Instant revokedAt) { this.revokedAt = revokedAt; }
    public void setRevocationReason(String revocationReason) { this.revocationReason = revocationReason; }
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
        public Builder grantorAgentId(String grantorAgentId) { consent.grantorAgentId = grantorAgentId; return this; }
        public Builder recipientAgentId(String recipientAgentId) { consent.recipientAgentId = recipientAgentId; return this; }
        public Builder scope(List<String> scope) { consent.scope = scope; return this; }
        public Builder purpose(String purpose) { consent.purpose = purpose; return this; }
        public Builder dataTypes(List<String> dataTypes) { consent.dataTypes = dataTypes; return this; }
        public Builder consentMethod(String consentMethod) { consent.consentMethod = consentMethod; return this; }
        public Builder status(ConsentStatus status) { consent.status = status; return this; }
        public Builder grantedAt(Instant grantedAt) { consent.grantedAt = grantedAt; return this; }
        public Builder expiresAt(Instant expiresAt) { consent.expiresAt = expiresAt; return this; }
        public Builder revokedAt(Instant revokedAt) { consent.revokedAt = revokedAt; return this; }
        public Builder revocationReason(String revocationReason) { consent.revocationReason = revocationReason; return this; }
        public Builder retentionPeriod(String retentionPeriod) { consent.retentionPeriod = retentionPeriod; return this; }
        public Builder metadata(Map<String, Object> metadata) { consent.metadata = metadata; return this; }

        public A2AConsent build() { return consent; }
    }

    @Override
    public String toString() {
        return "A2AConsent{" +
                "id='" + id + '\'' +
                ", userId='" + userId + '\'' +
                ", grantorAgentId='" + grantorAgentId + '\'' +
                ", recipientAgentId='" + recipientAgentId + '\'' +
                ", purpose='" + purpose + '\'' +
                ", status=" + status +
                ", isActive=" + isActive() +
                '}';
    }
}
