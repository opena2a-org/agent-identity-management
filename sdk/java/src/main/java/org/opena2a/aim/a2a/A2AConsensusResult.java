package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * Result of a multi-agent consensus verification check.
 *
 * <p>Skills become "verified" when they meet the consensus thresholds:</p>
 * <ul>
 *   <li>3+ unique attesting agents</li>
 *   <li>2+ unique organization owners</li>
 *   <li>60.0+ confidence score</li>
 * </ul>
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2AConsensusResult {

    @JsonProperty("skillId")
    private String skillId;

    @JsonProperty("agentId")
    private String agentId;

    @JsonProperty("attestationCount")
    private int attestationCount;

    @JsonProperty("uniqueAttesters")
    private int uniqueAttesters;

    @JsonProperty("uniqueOwners")
    private int uniqueOwners;

    @JsonProperty("confidenceScore")
    private double confidenceScore;

    @JsonProperty("isVerified")
    private boolean isVerified;

    // Default constructor for Jackson
    public A2AConsensusResult() {}

    // Getters
    public String getSkillId() { return skillId; }
    public String getAgentId() { return agentId; }
    public int getAttestationCount() { return attestationCount; }
    public int getUniqueAttesters() { return uniqueAttesters; }
    public int getUniqueOwners() { return uniqueOwners; }
    public double getConfidenceScore() { return confidenceScore; }
    public boolean isVerified() { return isVerified; }

    // Setters for Jackson
    public void setSkillId(String skillId) { this.skillId = skillId; }
    public void setAgentId(String agentId) { this.agentId = agentId; }
    public void setAttestationCount(int attestationCount) { this.attestationCount = attestationCount; }
    public void setUniqueAttesters(int uniqueAttesters) { this.uniqueAttesters = uniqueAttesters; }
    public void setUniqueOwners(int uniqueOwners) { this.uniqueOwners = uniqueOwners; }
    public void setConfidenceScore(double confidenceScore) { this.confidenceScore = confidenceScore; }
    public void setVerified(boolean verified) { isVerified = verified; }

    /**
     * Check if more attestations are needed for verification.
     */
    public boolean needsMoreAttesters() {
        return uniqueAttesters < 3;
    }

    /**
     * Check if more organization owners are needed.
     */
    public boolean needsMoreOwners() {
        return uniqueOwners < 2;
    }

    /**
     * Get the number of additional attesters needed.
     */
    public int attestersNeeded() {
        return Math.max(0, 3 - uniqueAttesters);
    }

    /**
     * Get the number of additional owners needed.
     */
    public int ownersNeeded() {
        return Math.max(0, 2 - uniqueOwners);
    }

    @Override
    public String toString() {
        return "A2AConsensusResult{" +
                "skillId='" + skillId + '\'' +
                ", isVerified=" + isVerified +
                ", attestationCount=" + attestationCount +
                ", uniqueAttesters=" + uniqueAttesters +
                ", uniqueOwners=" + uniqueOwners +
                ", confidenceScore=" + confidenceScore +
                '}';
    }
}
