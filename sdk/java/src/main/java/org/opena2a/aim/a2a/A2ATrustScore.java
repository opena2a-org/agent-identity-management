package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.Map;

/**
 * A2A Trust Score - Behavioral trust score between agent peers.
 * Tracks interaction history and calculates trust levels.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2ATrustScore {

    @JsonProperty("id")
    private String id;

    // Backend field: agent this score belongs to
    @JsonProperty("agentId")
    private String agentId;

    // Legacy SDK fields
    @JsonProperty("evaluatorAgentId")
    private String evaluatorAgentId;

    @JsonProperty("subjectAgentId")
    private String subjectAgentId;

    @JsonProperty("score")
    private double score;

    // Backend returns a2aTrustScore in responses
    @JsonProperty("a2aTrustScore")
    private Double a2aTrustScoreFromBackend;

    // Backend task statistics
    @JsonProperty("totalTasksAsClient")
    private int totalTasksAsClient;

    @JsonProperty("totalTasksAsRemote")
    private int totalTasksAsRemote;

    @JsonProperty("tasksCompleted")
    private int tasksCompleted;

    @JsonProperty("tasksFailed")
    private int tasksFailed;

    @JsonProperty("tasksCancelled")
    private int tasksCancelled;

    // Backend peer trust metrics
    @JsonProperty("peerTrustAverage")
    private Double peerTrustAverage;

    @JsonProperty("uniquePeersCount")
    private int uniquePeersCount;

    @JsonProperty("computedAt")
    private Instant computedAt;

    @JsonProperty("dataPoints")
    private int dataPoints;

    // Legacy SDK fields
    @JsonProperty("confidence")
    private double confidence;

    @JsonProperty("interactionCount")
    private int interactionCount;

    @JsonProperty("successfulInteractions")
    private int successfulInteractions;

    @JsonProperty("failedInteractions")
    private int failedInteractions;

    @JsonProperty("lastInteractionAt")
    private Instant lastInteractionAt;

    @JsonProperty("factors")
    private Map<String, Double> factors;

    @JsonProperty("createdAt")
    private Instant createdAt;

    @JsonProperty("updatedAt")
    private Instant updatedAt;

    // Default constructor for Jackson
    public A2ATrustScore() {}

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters
    public String getId() { return id; }

    /**
     * Get the agent ID. Uses backend 'agentId' or legacy 'subjectAgentId'.
     */
    public String getAgentId() {
        return agentId != null ? agentId : subjectAgentId;
    }

    public String getEvaluatorAgentId() { return evaluatorAgentId; }
    public String getSubjectAgentId() { return subjectAgentId; }

    /**
     * Get the trust score. Handles both 'score' and 'a2aTrustScore' backend responses.
     */
    public double getScore() {
        // If score is 0 but a2aTrustScore was provided, use that
        if (score == 0.0 && a2aTrustScoreFromBackend != null) {
            return a2aTrustScoreFromBackend;
        }
        return score;
    }

    // Backend task statistics
    public int getTotalTasksAsClient() { return totalTasksAsClient; }
    public int getTotalTasksAsRemote() { return totalTasksAsRemote; }
    public int getTasksCompleted() { return tasksCompleted; }
    public int getTasksFailed() { return tasksFailed; }
    public int getTasksCancelled() { return tasksCancelled; }
    public Double getPeerTrustAverage() { return peerTrustAverage; }
    public int getUniquePeersCount() { return uniquePeersCount; }
    public Instant getComputedAt() { return computedAt; }
    public int getDataPoints() { return dataPoints; }

    // Legacy SDK fields
    public double getConfidence() { return confidence; }
    public int getInteractionCount() { return interactionCount; }
    public int getSuccessfulInteractions() { return successfulInteractions; }
    public int getFailedInteractions() { return failedInteractions; }
    public Instant getLastInteractionAt() { return lastInteractionAt; }
    public Map<String, Double> getFactors() { return factors; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setAgentId(String agentId) { this.agentId = agentId; }
    public void setEvaluatorAgentId(String evaluatorAgentId) { this.evaluatorAgentId = evaluatorAgentId; }
    public void setSubjectAgentId(String subjectAgentId) { this.subjectAgentId = subjectAgentId; }
    public void setScore(double score) { this.score = score; }
    public void setA2aTrustScoreFromBackend(Double a2aTrustScore) { this.a2aTrustScoreFromBackend = a2aTrustScore; }
    // Backend task statistics setters
    public void setTotalTasksAsClient(int totalTasksAsClient) { this.totalTasksAsClient = totalTasksAsClient; }
    public void setTotalTasksAsRemote(int totalTasksAsRemote) { this.totalTasksAsRemote = totalTasksAsRemote; }
    public void setTasksCompleted(int tasksCompleted) { this.tasksCompleted = tasksCompleted; }
    public void setTasksFailed(int tasksFailed) { this.tasksFailed = tasksFailed; }
    public void setTasksCancelled(int tasksCancelled) { this.tasksCancelled = tasksCancelled; }
    public void setPeerTrustAverage(Double peerTrustAverage) { this.peerTrustAverage = peerTrustAverage; }
    public void setUniquePeersCount(int uniquePeersCount) { this.uniquePeersCount = uniquePeersCount; }
    public void setComputedAt(Instant computedAt) { this.computedAt = computedAt; }
    public void setDataPoints(int dataPoints) { this.dataPoints = dataPoints; }
    // Legacy SDK setters
    public void setConfidence(double confidence) { this.confidence = confidence; }
    public void setInteractionCount(int interactionCount) { this.interactionCount = interactionCount; }
    public void setSuccessfulInteractions(int successfulInteractions) { this.successfulInteractions = successfulInteractions; }
    public void setFailedInteractions(int failedInteractions) { this.failedInteractions = failedInteractions; }
    public void setLastInteractionAt(Instant lastInteractionAt) { this.lastInteractionAt = lastInteractionAt; }
    public void setFactors(Map<String, Double> factors) { this.factors = factors; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Check if this agent has high trust (score >= 0.8).
     */
    public boolean isHighTrust() {
        return score >= 0.8;
    }

    /**
     * Check if this agent has low trust (score < 0.3).
     */
    public boolean isLowTrust() {
        return score < 0.3;
    }

    /**
     * Get the success rate as a percentage.
     */
    public double getSuccessRate() {
        if (interactionCount == 0) return 0.0;
        return (double) successfulInteractions / interactionCount * 100.0;
    }

    /**
     * Builder for A2ATrustScore.
     */
    public static class Builder {
        private final A2ATrustScore score = new A2ATrustScore();

        public Builder id(String id) { score.id = id; return this; }
        public Builder evaluatorAgentId(String evaluatorAgentId) { score.evaluatorAgentId = evaluatorAgentId; return this; }
        public Builder subjectAgentId(String subjectAgentId) { score.subjectAgentId = subjectAgentId; return this; }
        public Builder score(double scoreValue) { score.score = scoreValue; return this; }
        public Builder confidence(double confidence) { score.confidence = confidence; return this; }
        public Builder interactionCount(int interactionCount) { score.interactionCount = interactionCount; return this; }
        public Builder successfulInteractions(int successfulInteractions) { score.successfulInteractions = successfulInteractions; return this; }
        public Builder failedInteractions(int failedInteractions) { score.failedInteractions = failedInteractions; return this; }
        public Builder lastInteractionAt(Instant lastInteractionAt) { score.lastInteractionAt = lastInteractionAt; return this; }
        public Builder factors(Map<String, Double> factors) { score.factors = factors; return this; }

        public A2ATrustScore build() { return score; }
    }

    @Override
    public String toString() {
        return "A2ATrustScore{" +
                "evaluatorAgentId='" + evaluatorAgentId + '\'' +
                ", subjectAgentId='" + subjectAgentId + '\'' +
                ", score=" + score +
                ", confidence=" + confidence +
                ", interactionCount=" + interactionCount +
                '}';
    }
}
