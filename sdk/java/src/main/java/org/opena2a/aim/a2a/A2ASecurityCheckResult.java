package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

/**
 * Result of an A2A security policy check.
 *
 * <p>Contains the decision (allowed/blocked) and any violations detected.
 * In "monitor" mode, violations are logged but requests are allowed.
 * In "strict" mode, any violation blocks the request.</p>
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ASecurityCheckResult {

    @JsonProperty("allowed")
    private boolean allowed;

    @JsonProperty("enforcementMode")
    private String enforcementMode;

    @JsonProperty("violations")
    private List<Violation> violations;

    @JsonProperty("denialReason")
    private String denialReason;

    @JsonProperty("targetAgentId")
    private String targetAgentId;

    @JsonProperty("skillId")
    private String skillId;

    @JsonProperty("requesterTrustScore")
    private Double requesterTrustScore;

    // Default constructor for Jackson
    public A2ASecurityCheckResult() {}

    // Getters
    public boolean isAllowed() { return allowed; }
    public String getEnforcementMode() { return enforcementMode; }
    public List<Violation> getViolations() { return violations; }
    public String getDenialReason() { return denialReason; }
    public String getTargetAgentId() { return targetAgentId; }
    public String getSkillId() { return skillId; }
    public Double getRequesterTrustScore() { return requesterTrustScore; }

    // Setters for Jackson
    public void setAllowed(boolean allowed) { this.allowed = allowed; }
    public void setEnforcementMode(String enforcementMode) { this.enforcementMode = enforcementMode; }
    public void setViolations(List<Violation> violations) { this.violations = violations; }
    public void setDenialReason(String denialReason) { this.denialReason = denialReason; }
    public void setTargetAgentId(String targetAgentId) { this.targetAgentId = targetAgentId; }
    public void setSkillId(String skillId) { this.skillId = skillId; }
    public void setRequesterTrustScore(Double requesterTrustScore) { this.requesterTrustScore = requesterTrustScore; }

    /**
     * Check if there are any violations.
     */
    public boolean hasViolations() {
        return violations != null && !violations.isEmpty();
    }

    /**
     * A detected security violation.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Violation {
        @JsonProperty("type")
        private String type;

        @JsonProperty("message")
        private String message;

        @JsonProperty("severity")
        private String severity;

        // Default constructor for Jackson
        public Violation() {}

        // Getters
        public String getType() { return type; }
        public String getMessage() { return message; }
        public String getSeverity() { return severity; }

        // Setters for Jackson
        public void setType(String type) { this.type = type; }
        public void setMessage(String message) { this.message = message; }
        public void setSeverity(String severity) { this.severity = severity; }

        @Override
        public String toString() {
            return "Violation{type='" + type + "', message='" + message + "', severity='" + severity + "'}";
        }
    }

    @Override
    public String toString() {
        return "A2ASecurityCheckResult{" +
                "allowed=" + allowed +
                ", enforcementMode='" + enforcementMode + '\'' +
                ", violations=" + violations +
                ", targetAgentId='" + targetAgentId + '\'' +
                '}';
    }
}
