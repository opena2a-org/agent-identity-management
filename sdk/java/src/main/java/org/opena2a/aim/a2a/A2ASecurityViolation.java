package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;

/**
 * Record of a security policy violation.
 *
 * <p>Violations are logged when security checks detect policy breaches.
 * In "monitor" mode, violations are logged but requests proceed.
 * In "strict" mode, violations block the request.</p>
 *
 * <p>Common violation types:</p>
 * <ul>
 *   <li>REVOKED_AGENT - Agent has been revoked</li>
 *   <li>BLOCKED_AGENT - Agent is on the blocked list</li>
 *   <li>TRUST_SCORE_TOO_LOW - Agent trust score below threshold</li>
 *   <li>SKILL_NOT_VERIFIED - Skill lacks verification attestations</li>
 *   <li>UNSIGNED_REQUEST - Request was not signed</li>
 *   <li>INVALID_NONCE - Nonce was missing, invalid, or reused</li>
 *   <li>TIMESTAMP_EXPIRED - Request timestamp too old</li>
 * </ul>
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ASecurityViolation {

    @JsonProperty("id")
    private String id;

    @JsonProperty("organizationId")
    private String organizationId;

    @JsonProperty("requestingAgentId")
    private String requestingAgentId;

    @JsonProperty("targetAgentId")
    private String targetAgentId;

    @JsonProperty("skillId")
    private String skillId;

    @JsonProperty("violationType")
    private String violationType;

    @JsonProperty("message")
    private String message;

    @JsonProperty("severity")
    private String severity;

    @JsonProperty("enforcementMode")
    private String enforcementMode;

    @JsonProperty("blocked")
    private boolean blocked;

    @JsonProperty("context")
    private String context;

    @JsonProperty("createdAt")
    private Instant createdAt;

    // Default constructor for Jackson
    public A2ASecurityViolation() {}

    // Getters
    public String getId() { return id; }
    public String getOrganizationId() { return organizationId; }
    public String getRequestingAgentId() { return requestingAgentId; }
    public String getTargetAgentId() { return targetAgentId; }
    public String getSkillId() { return skillId; }
    public String getViolationType() { return violationType; }
    public String getMessage() { return message; }
    public String getSeverity() { return severity; }
    public String getEnforcementMode() { return enforcementMode; }
    public boolean isBlocked() { return blocked; }
    public String getContext() { return context; }
    public Instant getCreatedAt() { return createdAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setOrganizationId(String organizationId) { this.organizationId = organizationId; }
    public void setRequestingAgentId(String requestingAgentId) { this.requestingAgentId = requestingAgentId; }
    public void setTargetAgentId(String targetAgentId) { this.targetAgentId = targetAgentId; }
    public void setSkillId(String skillId) { this.skillId = skillId; }
    public void setViolationType(String violationType) { this.violationType = violationType; }
    public void setMessage(String message) { this.message = message; }
    public void setSeverity(String severity) { this.severity = severity; }
    public void setEnforcementMode(String enforcementMode) { this.enforcementMode = enforcementMode; }
    public void setBlocked(boolean blocked) { this.blocked = blocked; }
    public void setContext(String context) { this.context = context; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }

    /**
     * Check if this is a critical severity violation.
     */
    public boolean isCritical() {
        return "critical".equalsIgnoreCase(severity);
    }

    /**
     * Check if this is a high severity violation.
     */
    public boolean isHighSeverity() {
        return "high".equalsIgnoreCase(severity) || isCritical();
    }

    @Override
    public String toString() {
        return "A2ASecurityViolation{" +
                "violationType='" + violationType + '\'' +
                ", requestingAgentId='" + requestingAgentId + '\'' +
                ", targetAgentId='" + targetAgentId + '\'' +
                ", severity='" + severity + '\'' +
                ", blocked=" + blocked +
                '}';
    }
}
