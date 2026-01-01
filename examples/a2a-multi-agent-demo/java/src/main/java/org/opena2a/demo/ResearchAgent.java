package org.opena2a.demo;

import org.opena2a.aim.AIMClient;
import org.opena2a.aim.a2a.A2AClient;
import org.opena2a.aim.a2a.A2ATrustScore;
import org.opena2a.aim.AgentType;

import java.util.*;

/**
 * Research Agent - Gathers information and collaborates with Analysis Agent.
 *
 * Demonstrates:
 * - Agent Card registration
 * - Intent-based discovery
 * - GDPR consent management
 * - Request signing
 * - Task logging
 * - Skill attestation
 * - Security policy checks
 */
public class ResearchAgent {

    private static final List<Map<String, Object>> SKILLS = List.of(
            Map.of(
                    "id", "web-research",
                    "name", "Web Research",
                    "description", "Gather information from web sources on specified topics",
                    "tags", List.of("research", "web", "data-gathering"),
                    "inputModes", List.of("text"),
                    "outputModes", List.of("text", "data")
            ),
            Map.of(
                    "id", "document-reading",
                    "name", "Document Reading",
                    "description", "Extract and process content from documents",
                    "tags", List.of("documents", "extraction", "reading"),
                    "inputModes", List.of("file", "text"),
                    "outputModes", List.of("text", "data")
            )
    );

    private final String agentName = "research-agent";
    private final AIMClient aimClient;
    private final A2AClient a2a;
    private String agentId;

    public ResearchAgent(String aimUrl, String apiKey) {
        System.out.println("[Research Agent] Registering with AIM...");

        this.aimClient = new AIMClient.Builder()
                .agentName(agentName)
                .aimUrl(aimUrl)
                .apiKey(apiKey)
                .agentType(AgentType.AI_AGENT)
                .description("Research agent that gathers information and collaborates with analysis agents")
                .build();

        this.a2a = new A2AClient(aimClient);
    }

    public void initialize() throws Exception {
        aimClient.register();
        this.agentId = aimClient.getAgentId();
        System.out.println("[Research Agent] Registered with ID: " + agentId.substring(0, 8) + "...");
    }

    public String getAgentId() {
        return agentId;
    }

    public Map<String, Object> registerAgentCard() {
        System.out.println("[Research Agent] Registering Agent Card...");

        try {
            for (Map<String, Object> skill : SKILLS) {
                a2a.registerSkill(skill);
                System.out.println("  - Registered skill: " + skill.get("name"));
            }

            Map<String, Object> card = a2a.getAgentCard();
            System.out.println("[Research Agent] Agent Card registered successfully");
            return card;
        } catch (Exception e) {
            System.out.println("[Research Agent] Card registration note: " + e.getMessage());
            return null;
        }
    }

    public Map<String, Object> discoverAnalysisAgent(String intent, double minTrust) {
        System.out.println("\n[Research Agent] Searching for agents capable of: '" + intent + "'");
        System.out.println("[Research Agent] Minimum trust score: " + minTrust);

        try {
            List<Map<String, Object>> capableAgents = a2a.routeByIntent(intent, minTrust);

            if (capableAgents == null || capableAgents.isEmpty()) {
                System.out.println("[Research Agent] No capable agents found via intent routing");
                List<Map<String, Object>> skills = a2a.searchSkills("sentiment analysis", 5);
                if (skills != null && !skills.isEmpty()) {
                    System.out.println("[Research Agent] Found " + skills.size() + " skills via search");
                    return skills.get(0);
                }
                return null;
            }

            Map<String, Object> bestAgent = capableAgents.get(0);
            System.out.println("[Research Agent] Found agent: " + bestAgent.getOrDefault("agentName", "Unknown"));
            System.out.println("[Research Agent] Skill: " + bestAgent.getOrDefault("skillName", "Unknown"));
            System.out.println("[Research Agent] Trust score: " + bestAgent.getOrDefault("trustScore", "N/A"));

            return bestAgent;
        } catch (Exception e) {
            System.out.println("[Research Agent] Discovery error: " + e.getMessage());
            return null;
        }
    }

    public Map<String, Object> checkSecurityPolicies(String targetAgentId, String skillId) {
        System.out.println("\n[Research Agent] Checking security policies...");
        System.out.println("[Research Agent] Target: " + targetAgentId.substring(0, 8) + "...");
        System.out.println("[Research Agent] Skill: " + skillId);

        try {
            Map<String, Object> result = a2a.checkSecurity(targetAgentId, skillId);

            if (Boolean.TRUE.equals(result.get("allowed"))) {
                System.out.println("[Research Agent] Security check PASSED");
                System.out.println("[Research Agent] Enforcement mode: " + result.getOrDefault("enforcementMode", "unknown"));
            } else {
                System.out.println("[Research Agent] Security check FAILED");
                @SuppressWarnings("unchecked")
                List<Map<String, String>> violations = (List<Map<String, String>>) result.getOrDefault("violations", List.of());
                for (Map<String, String> v : violations) {
                    System.out.println("  - " + v.get("type") + ": " + v.get("message"));
                }
            }

            return result;
        } catch (Exception e) {
            System.out.println("[Research Agent] Security check error: " + e.getMessage());
            return Map.of("allowed", true, "note", "Security check unavailable");
        }
    }

    public boolean requestUserConsent(
            String userId,
            String recipientAgentId,
            List<String> dataTypes,
            String purpose
    ) {
        System.out.println("\n[Research Agent] Requesting user consent...");
        System.out.println("[Research Agent] User: " + userId);
        System.out.println("[Research Agent] Purpose: " + purpose);
        System.out.println("[Research Agent] Data types: " + String.join(", ", dataTypes));

        try {
            boolean hasConsent = a2a.checkConsent(userId, recipientAgentId, "data_sharing");

            if (hasConsent) {
                System.out.println("[Research Agent] Consent already exists");
                return true;
            }

            var consent = a2a.recordConsent(
                    userId,
                    recipientAgentId,
                    List.of("data_sharing", "analysis"),
                    purpose,
                    dataTypes,
                    24
            );

            System.out.println("[Research Agent] Consent recorded: " + consent.getConsentId());
            return true;
        } catch (Exception e) {
            System.out.println("[Research Agent] Consent error: " + e.getMessage());
            return true; // Allow proceeding for demo
        }
    }

    public Map<String, Object> signAndCallSkill(
            String targetAgentId,
            String skillId,
            Map<String, Object> data,
            AnalysisAgent analysisAgent
    ) {
        System.out.println("\n[Research Agent] Preparing signed request...");
        System.out.println("[Research Agent] Skill: " + skillId);

        // Sign the request
        try {
            var signature = a2a.signRequest("POST", "/skills/" + skillId + "/invoke", data);
            System.out.println("[Research Agent] Request signed at: " + signature.getTimestamp());
            System.out.println("[Research Agent] Nonce: " + signature.getNonce().substring(0, 16) + "...");
        } catch (Exception e) {
            System.out.println("[Research Agent] Signing note: " + e.getMessage());
        }

        // Log the task
        String taskId = null;
        try {
            String externalTaskId = "demo-" + UUID.randomUUID().toString().substring(0, 8);
            var task = a2a.logTask(externalTaskId, targetAgentId, skillId);
            taskId = task.getId() != null ? task.getId() : task.getTaskId();
            System.out.println("[Research Agent] Task logged: " + taskId);
        } catch (Exception e) {
            System.out.println("[Research Agent] Task logging note: " + e.getMessage());
            taskId = "local-" + UUID.randomUUID().toString().substring(0, 8);
        }

        // Execute skill (direct call for demo)
        System.out.println("[Research Agent] Invoking skill...");
        Map<String, Object> result;
        if (analysisAgent != null) {
            result = analysisAgent.executeSkill(skillId, data);
        } else {
            result = Map.of(
                    "sentiment", "positive",
                    "confidence", 0.87,
                    "note", "Simulated response - Analysis Agent not provided"
            );
        }

        System.out.println("[Research Agent] Result: " + result);

        // Update task state
        if (taskId != null) {
            try {
                a2a.updateTaskState(taskId, "COMPLETED");
                System.out.println("[Research Agent] Task marked COMPLETED");
            } catch (Exception e) {
                System.out.println("[Research Agent] Task update note: " + e.getMessage());
            }
        }

        return result;
    }

    public Map<String, Object> attestSkillQuality(
            String targetAgentId,
            String skillId,
            double confidence,
            Map<String, Object> evidence
    ) {
        System.out.println("\n[Research Agent] Creating skill attestation...");
        System.out.println("[Research Agent] Skill: " + skillId);
        System.out.println("[Research Agent] Confidence: " + confidence);

        try {
            Map<String, Object> attestation = a2a.attestSkill(
                    targetAgentId,
                    skillId,
                    "SKILL_VERIFICATION",
                    confidence,
                    evidence
            );

            System.out.println("[Research Agent] Attestation created: " + attestation.getOrDefault("id", "N/A"));

            // Check consensus status
            try {
                var consensus = a2a.getConsensusStatus(targetAgentId, skillId);
                System.out.println("[Research Agent] Skill verified: " + consensus.getOrDefault("isVerified", false));
                System.out.println("[Research Agent] Attestation count: " + consensus.getOrDefault("attestationCount", 0));
                System.out.println("[Research Agent] Unique attesters: " + consensus.getOrDefault("uniqueAttesters", 0));
            } catch (Exception e) {
                System.out.println("[Research Agent] Consensus check note: " + e.getMessage());
            }

            return attestation;
        } catch (Exception e) {
            System.out.println("[Research Agent] Attestation error: " + e.getMessage());
            return null;
        }
    }

    public A2ATrustScore getTrustScore(String agentId) {
        String target = agentId != null ? agentId : this.agentId;
        String targetLabel = target.equals(this.agentId) ? "self" : target.substring(0, 8) + "...";

        System.out.println("\n[Research Agent] Getting trust score for " + targetLabel + "...");

        try {
            A2ATrustScore score = a2a.getA2ATrustScore(agentId);
            System.out.printf("[Research Agent] Trust score: %.2f%n", score.getA2aTrustScore());
            System.out.println("[Research Agent] Peer trust avg: " +
                    (score.getPeerTrustAverage() != null ? score.getPeerTrustAverage() : "N/A"));
            System.out.println("[Research Agent] Tasks completed: " +
                    (score.getTasksCompleted() != null ? score.getTasksCompleted() : 0));
            return score;
        } catch (Exception e) {
            System.out.println("[Research Agent] Trust score note: " + e.getMessage());
            return null;
        }
    }

    public Map<String, Object> getPeerTrust(String peerAgentId) {
        System.out.println("\n[Research Agent] Getting peer trust with " + peerAgentId.substring(0, 8) + "...");

        try {
            var trust = a2a.getPeerTrust(peerAgentId);
            System.out.printf("[Research Agent] Peer trust score: %.2f%n",
                    trust.getPeerTrustScore() != null ? trust.getPeerTrustScore() : 0.0);
            System.out.println("[Research Agent] Success rate: " +
                    (trust.getSuccessRate() != null ? trust.getSuccessRate() : "N/A"));
            return Map.of(
                    "peerTrustScore", trust.getPeerTrustScore(),
                    "successRate", trust.getSuccessRate()
            );
        } catch (Exception e) {
            System.out.println("[Research Agent] Peer trust note: " + e.getMessage());
            return null;
        }
    }
}
