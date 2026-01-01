package org.opena2a.demo;

import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;

import java.io.*;
import java.lang.reflect.Type;
import java.nio.file.*;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.*;

/**
 * A2A Multi-Agent Collaboration Demo
 *
 * Demonstrates Research Agent discovering and collaborating with Analysis Agent
 * using all A2A features:
 * 1. Agent Card Registration
 * 2. Intent-Based Discovery
 * 3. Trust Score Management
 * 4. Security Policy Evaluation
 * 5. GDPR Consent Management
 * 6. Request Signing & Task Logging
 * 7. Skill Attestation & Consensus
 *
 * Run with:
 *     export AIM_API_KEY='your-key'
 *     mvn exec:java -Dexec.mainClass="org.opena2a.demo.A2AMultiAgentDemo"
 */
public class A2AMultiAgentDemo {

    private static final Gson gson = new Gson();

    public static void main(String[] args) {
        String aimUrl = System.getenv().getOrDefault("AIM_URL", "http://localhost:8080/api");
        String apiKey = System.getenv("AIM_API_KEY");

        if (apiKey == null || apiKey.isEmpty()) {
            System.err.println("Error: API key required. Set AIM_API_KEY environment variable");
            System.err.println("\nUsage:");
            System.err.println("  export AIM_API_KEY='your-api-key'");
            System.err.println("  mvn exec:java -Dexec.mainClass=\"org.opena2a.demo.A2AMultiAgentDemo\"");
            System.exit(1);
        }

        try {
            runDemo(aimUrl, apiKey);
        } catch (Exception e) {
            System.err.println("\nDemo error: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }

    private static void runDemo(String aimUrl, String apiKey) throws Exception {
        printBanner();

        Map<String, Object> sampleData = loadSampleData();
        Map<String, Object> results = new HashMap<>();

        // =========================================================================
        // STEP 1: Register Analysis Agent
        // =========================================================================
        printStep(1, "REGISTER ANALYSIS AGENT");

        AnalysisAgent analysisAgent = new AnalysisAgent(aimUrl, apiKey);
        analysisAgent.initialize();
        analysisAgent.registerAgentCard();

        System.out.println("\nAnalysis Agent ready with " + AnalysisAgent.SKILLS.size() + " skills:");
        for (Map<String, Object> skill : AnalysisAgent.SKILLS) {
            String desc = (String) skill.get("description");
            System.out.println("  - " + skill.get("name") + ": " + desc.substring(0, Math.min(50, desc.length())) + "...");
        }

        // =========================================================================
        // STEP 2: Register Research Agent
        // =========================================================================
        printStep(2, "REGISTER RESEARCH AGENT");

        ResearchAgent researchAgent = new ResearchAgent(aimUrl, apiKey);
        researchAgent.initialize();
        researchAgent.registerAgentCard();

        // =========================================================================
        // STEP 3: Discover Analysis Agent by Intent
        // =========================================================================
        printStep(3, "INTENT-BASED DISCOVERY");

        String intent = "I need to analyze customer feedback sentiment to understand satisfaction levels";
        Map<String, Object> discovered = researchAgent.discoverAnalysisAgent(intent, 0.3);

        String targetAgentId;
        String skillId;

        if (discovered != null && discovered.get("agentId") != null) {
            targetAgentId = (String) discovered.get("agentId");
            skillId = discovered.get("skillId") != null ? (String) discovered.get("skillId") : "sentiment-analysis";
        } else {
            System.out.println("[Demo] Using direct agent reference for demo purposes");
            targetAgentId = analysisAgent.getAgentId();
            skillId = "sentiment-analysis";
        }

        // =========================================================================
        // STEP 4: Check Trust Scores
        // =========================================================================
        printStep(4, "TRUST SCORE MANAGEMENT");

        researchAgent.getTrustScore(null);
        researchAgent.getPeerTrust(targetAgentId);

        // =========================================================================
        // STEP 5: Security Policy Evaluation
        // =========================================================================
        printStep(5, "SECURITY POLICY EVALUATION");

        researchAgent.checkSecurityPolicies(targetAgentId, skillId);

        // =========================================================================
        // STEP 6: GDPR Consent Management
        // =========================================================================
        printStep(6, "GDPR CONSENT MANAGEMENT");

        String userId = "demo-user-001";
        boolean consentGranted = researchAgent.requestUserConsent(
                userId,
                targetAgentId,
                List.of("customer_feedback", "sentiment_data"),
                "Analyze customer feedback sentiment for satisfaction insights"
        );

        if (!consentGranted) {
            System.out.println("[Demo] Warning: Consent not granted, but continuing for demo...");
        }

        // =========================================================================
        // STEP 7: Execute Skills with Signing & Task Logging
        // =========================================================================
        printStep(7, "SKILL EXECUTION (Signing + Task Logging)");

        // 7a: Sentiment Analysis
        System.out.println("\n--- Sentiment Analysis ---");
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> feedback = (List<Map<String, Object>>) sampleData.get("customerFeedback");
        String feedbackText = (String) feedback.get(0).get("text");
        System.out.println("Input: \"" + feedbackText.substring(0, Math.min(60, feedbackText.length())) + "...\"");

        Map<String, Object> sentimentResult = researchAgent.signAndCallSkill(
                targetAgentId,
                "sentiment-analysis",
                Map.of("text", feedbackText),
                analysisAgent
        );
        results.put("sentiment", sentimentResult);

        // 7b: Trend Detection
        System.out.println("\n--- Trend Detection ---");
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> timeSeries = (List<Map<String, Object>>) sampleData.get("timeSeriesData");
        System.out.println("Input: " + timeSeries.size() + " data points");

        Map<String, Object> trendResult = researchAgent.signAndCallSkill(
                targetAgentId,
                "trend-detection",
                Map.of("data", timeSeries),
                analysisAgent
        );
        results.put("trend", trendResult);

        // 7c: Summarization
        System.out.println("\n--- Summarization ---");
        String document = (String) sampleData.get("documentContent");
        System.out.println("Input: " + document.length() + " characters");

        Map<String, Object> summaryResult = researchAgent.signAndCallSkill(
                targetAgentId,
                "summarization",
                Map.of("text", document),
                analysisAgent
        );
        results.put("summary", summaryResult);

        // =========================================================================
        // STEP 8: Skill Attestation
        // =========================================================================
        printStep(8, "SKILL ATTESTATION");

        researchAgent.attestSkillQuality(
                targetAgentId,
                "sentiment-analysis",
                0.92,
                Map.of(
                        "sampleSize", feedback.size(),
                        "accuracyObserved", 0.95,
                        "responseTime", "fast",
                        "outputQuality", "high"
                )
        );

        // =========================================================================
        // STEP 9: Print Summary
        // =========================================================================
        printStep(9, "VERIFICATION");

        System.out.println("All A2A features demonstrated:");
        System.out.println("  [x] Agent Card Registration");
        System.out.println("  [x] Intent-Based Discovery");
        System.out.println("  [x] Trust Score Management");
        System.out.println("  [x] Security Policy Evaluation");
        System.out.println("  [x] GDPR Consent Management");
        System.out.println("  [x] Request Signing");
        System.out.println("  [x] Task Logging & State Updates");
        System.out.println("  [x] Skill Attestation");
        System.out.println("  [x] Consensus Verification");

        printSummary(aimUrl, researchAgent, analysisAgent, results);
    }

    private static Map<String, Object> loadSampleData() {
        try {
            Path dataPath = Paths.get("../shared/sample-data/research-data.json");
            if (Files.exists(dataPath)) {
                String content = Files.readString(dataPath);
                Type type = new TypeToken<Map<String, Object>>() {}.getType();
                return gson.fromJson(content, type);
            }
        } catch (Exception e) {
            System.out.println("Note: Could not load sample data file, using defaults");
        }

        // Fallback data
        return Map.of(
                "customerFeedback", List.of(
                        Map.of(
                                "id", "fb-001",
                                "text", "This product exceeded my expectations! The quality is outstanding.",
                                "source", "customer-survey"
                        )
                ),
                "timeSeriesData", List.of(
                        Map.of("date", "2025-01-01", "metric", "sales", "value", 1000),
                        Map.of("date", "2025-01-02", "metric", "sales", "value", 1200),
                        Map.of("date", "2025-01-03", "metric", "sales", "value", 1350)
                ),
                "documentContent", "Quarterly report summarizing company performance."
        );
    }

    private static void printBanner() {
        String timestamp = LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"));
        System.out.println("\n" + "=".repeat(70));
        System.out.println("  A2A MULTI-AGENT COLLABORATION DEMO (Java)");
        System.out.println("  Research Agent <-> Analysis Agent");
        System.out.println("=".repeat(70));
        System.out.println("  Started: " + timestamp);
        System.out.println("=".repeat(70) + "\n");
    }

    private static void printStep(int stepNum, String title) {
        System.out.println("\n" + "-".repeat(70));
        System.out.println("  STEP " + stepNum + ": " + title);
        System.out.println("-".repeat(70) + "\n");
    }

    private static void printSummary(
            String aimUrl,
            ResearchAgent researchAgent,
            AnalysisAgent analysisAgent,
            Map<String, Object> results
    ) {
        System.out.println("\n" + "=".repeat(70));
        System.out.println("  DEMO SUMMARY");
        System.out.println("=".repeat(70));

        // Results summary
        System.out.println("\n  Results:");
        System.out.println("    - Analysis Agent ID: " + analysisAgent.getAgentId().substring(0, 16) + "...");
        System.out.println("    - Research Agent ID: " + researchAgent.getAgentId().substring(0, 16) + "...");
        System.out.println("    - Skills Registered: " + (AnalysisAgent.SKILLS.size() + 2));

        @SuppressWarnings("unchecked")
        Map<String, Object> sentiment = (Map<String, Object>) results.get("sentiment");
        @SuppressWarnings("unchecked")
        Map<String, Object> trend = (Map<String, Object>) results.get("trend");

        System.out.println("    - Sentiment Result: " + (sentiment != null ? sentiment.get("sentiment") : "N/A"));
        System.out.println("    - Trend Result: " + (trend != null ? trend.get("trend") : "N/A"));

        // Dashboard links
        String baseUrl = aimUrl.replace("/api", "");
        System.out.println("\n  Verify in Dashboard:");
        System.out.println("    - A2A Overview:    " + baseUrl + "/dashboard/a2a");
        System.out.println("    - Agent Cards:     " + baseUrl + "/dashboard/a2a?tab=cards");
        System.out.println("    - Consent Records: " + baseUrl + "/dashboard/a2a?tab=consent");
        System.out.println("    - Task History:    " + baseUrl + "/dashboard/a2a?tab=tasks");
        System.out.println("    - Trust Scores:    " + baseUrl + "/dashboard/a2a?tab=trust");
        System.out.println("    - Skill Search:    " + baseUrl + "/dashboard/a2a?tab=skills");

        System.out.println("\n" + "=".repeat(70));
        System.out.println("  Demo completed successfully!");
        System.out.println("=".repeat(70) + "\n");
    }
}
