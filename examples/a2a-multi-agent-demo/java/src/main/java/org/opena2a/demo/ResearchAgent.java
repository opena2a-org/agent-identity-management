package org.opena2a.demo;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.a2a.A2AClient;
import org.opena2a.aim.a2a.A2AAgentCard;
import org.opena2a.aim.a2a.A2ATrustScore;
import org.opena2a.aim.a2a.A2APeerTrust;
import org.opena2a.aim.a2a.A2ASecurityCheckResult;
import org.opena2a.aim.a2a.A2AConsent;
import org.opena2a.aim.a2a.A2ARequestSignature;
import org.opena2a.aim.a2a.A2AConsensusResult;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
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

    public static final List<A2AAgentCard.Skill> SKILLS;

    static {
        var webResearchSkill = new A2AAgentCard.Skill();
        webResearchSkill.setId("web-research");
        webResearchSkill.setName("Web Research");
        webResearchSkill.setDescription("Gather information from web sources on specified topics");
        webResearchSkill.setTags(List.of("research", "web", "data-gathering"));
        webResearchSkill.setInputModes(List.of("text"));
        webResearchSkill.setOutputModes(List.of("text", "data"));

        var documentReadingSkill = new A2AAgentCard.Skill();
        documentReadingSkill.setId("document-reading");
        documentReadingSkill.setName("Document Reading");
        documentReadingSkill.setDescription("Extract and process content from documents");
        documentReadingSkill.setTags(List.of("documents", "extraction", "reading"));
        documentReadingSkill.setInputModes(List.of("file", "text"));
        documentReadingSkill.setOutputModes(List.of("text", "data"));

        SKILLS = List.of(webResearchSkill, documentReadingSkill);
    }

    private final String agentName = "research-agent";
    private AIMClient aimClient;
    private A2AClient a2a;
    private final ObjectMapper objectMapper = new ObjectMapper();
    private String agentId;

    private final String aimUrl;

    public ResearchAgent(String aimUrl) {
        this.aimUrl = aimUrl;
    }

    public ResearchAgent() {
        this(null);
    }

    public void initialize() throws Exception {
        System.out.println("[Research Agent] Registering with AIM...");

        // SDK handles auth and registration automatically via bundled credentials
        this.aimClient = AIMClient.secure(agentName);
        this.a2a = new A2AClient(aimClient);
        this.agentId = aimClient.getAgentId();

        System.out.println("[Research Agent] Registered with ID: " + agentId.substring(0, 8) + "...");
    }

    public String getAgentId() {
        return agentId;
    }

    public A2AAgentCard registerAgentCard() {
        System.out.println("[Research Agent] Registering Agent Card...");

        try {
            for (A2AAgentCard.Skill skill : SKILLS) {
                a2a.registerSkill(skill);
                System.out.println("  - Registered skill: " + skill.getName());
            }

            A2AAgentCard card = a2a.getAgentCard(agentId);
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

    public A2ASecurityCheckResult checkSecurityPolicies(String targetAgentId, String skillId) {
        System.out.println("\n[Research Agent] Checking security policies...");
        System.out.println("[Research Agent] Target: " + targetAgentId.substring(0, 8) + "...");
        System.out.println("[Research Agent] Skill: " + skillId);

        try {
            A2ASecurityCheckResult result = a2a.checkSecurity(targetAgentId, skillId);

            if (result.isAllowed()) {
                System.out.println("[Research Agent] Security check PASSED");
                System.out.println("[Research Agent] Enforcement mode: " +
                        (result.getEnforcementMode() != null ? result.getEnforcementMode() : "unknown"));
            } else {
                System.out.println("[Research Agent] Security check FAILED");
                if (result.getViolations() != null) {
                    for (var v : result.getViolations()) {
                        System.out.println("  - " + v.getType() + ": " + v.getMessage());
                    }
                }
            }

            return result;
        } catch (Exception e) {
            System.out.println("[Research Agent] Security check error: " + e.getMessage());
            return null;
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
            // Check if consent already exists
            Map<String, Object> consentCheck = a2a.checkConsent(userId, recipientAgentId, purpose, dataTypes.get(0));

            if (Boolean.TRUE.equals(consentCheck.get("hasConsent"))) {
                System.out.println("[Research Agent] Consent already exists");
                return true;
            }

            // Record new consent (expires in 24 hours)
            Instant expiresAt = Instant.now().plus(24, ChronoUnit.HOURS);
            A2AConsent consent = a2a.recordConsent(
                    userId,
                    recipientAgentId,
                    purpose,
                    dataTypes,
                    "consent",
                    expiresAt
            );

            System.out.println("[Research Agent] Consent recorded: " + consent.getId());
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
            String jsonBody = objectMapper.writeValueAsString(data);
            A2ARequestSignature signature = a2a.signRequest("POST", "/skills/" + skillId + "/invoke", jsonBody);
            System.out.println("[Research Agent] Request signed at: " + signature.getTimestamp());
            String nonce = signature.getNonce();
            System.out.println("[Research Agent] Nonce: " + (nonce != null && nonce.length() > 16 ? nonce.substring(0, 16) + "..." : nonce));
        } catch (Exception e) {
            System.out.println("[Research Agent] Signing note: " + e.getMessage());
        }

        // Log the task
        String taskId = null;
        try {
            String externalTaskId = "demo-" + UUID.randomUUID().toString().substring(0, 8);
            Map<String, Object> task = a2a.logTask(targetAgentId, externalTaskId, skillId, "SUBMITTED");
            taskId = (String) task.getOrDefault("id", task.get("taskId"));
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
        if (taskId != null && !taskId.startsWith("local-")) {
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
                A2AConsensusResult consensus = a2a.getConsensusStatus(targetAgentId, skillId);
                System.out.println("[Research Agent] Skill verified: " + consensus.isVerified());
                System.out.println("[Research Agent] Attestation count: " + consensus.getAttestationCount());
                System.out.println("[Research Agent] Unique attesters: " + consensus.getUniqueAttesters());
            } catch (Exception e) {
                System.out.println("[Research Agent] Consensus check note: " + e.getMessage());
            }

            return attestation;
        } catch (Exception e) {
            System.out.println("[Research Agent] Attestation error: " + e.getMessage());
            return null;
        }
    }

    public A2ATrustScore getTrustScore() {
        System.out.println("\n[Research Agent] Getting trust score for self...");

        try {
            A2ATrustScore score = a2a.getTrustScore(agentId);
            System.out.printf("[Research Agent] Trust score: %.2f%n", score.getScore());
            System.out.printf("[Research Agent] Success rate: %.2f%%%n", score.getSuccessRate());
            System.out.println("[Research Agent] Interaction count: " + score.getInteractionCount());
            return score;
        } catch (Exception e) {
            System.out.println("[Research Agent] Trust score note: " + e.getMessage());
            return null;
        }
    }

    public A2APeerTrust getPeerTrust(String peerAgentId) {
        System.out.println("\n[Research Agent] Getting peer trust with " + peerAgentId.substring(0, 8) + "...");

        try {
            A2APeerTrust trust = a2a.getPeerTrust(peerAgentId);
            A2ATrustScore trustScore = trust.getTrustScore();
            if (trustScore != null) {
                System.out.printf("[Research Agent] Peer trust score: %.2f%n", trustScore.getScore());
                System.out.printf("[Research Agent] Success rate: %.2f%%%n", trustScore.getSuccessRate());
            } else {
                System.out.println("[Research Agent] Peer trust score: N/A (no score yet)");
            }
            System.out.println("[Research Agent] Trust level: " + trust.getTrustLevel());
            return trust;
        } catch (Exception e) {
            System.out.println("[Research Agent] Peer trust note: " + e.getMessage());
            return null;
        }
    }
}
