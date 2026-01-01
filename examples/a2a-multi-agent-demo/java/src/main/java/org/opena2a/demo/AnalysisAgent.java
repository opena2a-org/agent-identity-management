package org.opena2a.demo;

import org.opena2a.aim.AIMClient;
import org.opena2a.aim.a2a.A2AClient;
import org.opena2a.aim.AgentType;

import java.util.*;

/**
 * Analysis Agent - Provides data analysis skills for A2A collaboration.
 *
 * Demonstrates:
 * - Agent registration with AIM
 * - Agent Card with multiple skills
 * - Request verification
 * - Skill execution
 */
public class AnalysisAgent {

    public static final List<Map<String, Object>> SKILLS = List.of(
            Map.of(
                    "id", "sentiment-analysis",
                    "name", "Sentiment Analysis",
                    "description", "Analyze sentiment of text data, returning positive/negative/neutral classification",
                    "tags", List.of("nlp", "sentiment", "text-analysis", "customer-feedback"),
                    "inputModes", List.of("text"),
                    "outputModes", List.of("text")
            ),
            Map.of(
                    "id", "summarization",
                    "name", "Text Summarization",
                    "description", "Summarize long text documents into concise summaries",
                    "tags", List.of("nlp", "summarization", "text", "content"),
                    "inputModes", List.of("text"),
                    "outputModes", List.of("text")
            ),
            Map.of(
                    "id", "trend-detection",
                    "name", "Trend Detection",
                    "description", "Detect trends and patterns in time-series data",
                    "tags", List.of("analytics", "trends", "data-analysis", "forecasting"),
                    "inputModes", List.of("data"),
                    "outputModes", List.of("data", "text")
            )
    );

    private static final List<String> POSITIVE_WORDS = List.of(
            "excellent", "great", "amazing", "good", "outstanding", "recommend", "happy", "love"
    );

    private static final List<String> NEGATIVE_WORDS = List.of(
            "bad", "terrible", "awful", "disappointed", "not satisfied", "poor", "hate"
    );

    private final String agentName = "analysis-agent";
    private final AIMClient aimClient;
    private final A2AClient a2a;
    private String agentId;

    public AnalysisAgent(String aimUrl, String apiKey) {
        System.out.println("[Analysis Agent] Registering with AIM...");

        this.aimClient = new AIMClient.Builder()
                .agentName(agentName)
                .aimUrl(aimUrl)
                .apiKey(apiKey)
                .agentType(AgentType.AI_AGENT)
                .description("Analysis agent providing sentiment analysis, summarization, and trend detection")
                .build();

        this.a2a = new A2AClient(aimClient);
    }

    public void initialize() throws Exception {
        aimClient.register();
        this.agentId = aimClient.getAgentId();
        System.out.println("[Analysis Agent] Registered with ID: " + agentId.substring(0, 8) + "...");
    }

    public String getAgentId() {
        return agentId;
    }

    public Map<String, Object> registerAgentCard() {
        System.out.println("[Analysis Agent] Registering Agent Card with " + SKILLS.size() + " skills...");

        try {
            for (Map<String, Object> skill : SKILLS) {
                a2a.registerSkill(skill);
                System.out.println("  - Registered skill: " + skill.get("name"));
            }

            Map<String, Object> card = a2a.getAgentCard();
            System.out.println("[Analysis Agent] Agent Card registered successfully");
            return card;
        } catch (Exception e) {
            System.out.println("[Analysis Agent] Card registration note: " + e.getMessage());
            return null;
        }
    }

    public Map<String, Object> verifyIncomingRequest(
            String agentId,
            long timestamp,
            String nonce,
            String signature,
            String requestHash
    ) {
        System.out.println("[Analysis Agent] Verifying request from " + agentId.substring(0, 8) + "...");

        try {
            Map<String, Object> params = Map.of(
                    "agentId", agentId,
                    "timestamp", timestamp,
                    "nonce", nonce,
                    "signature", signature,
                    "requestHash", requestHash
            );

            Map<String, Object> result = a2a.verifyRequest(params);

            if (Boolean.TRUE.equals(result.get("valid"))) {
                System.out.println("[Analysis Agent] Request verified! Agent: " + result.get("agentName"));
                System.out.println("[Analysis Agent] Trust score: " + result.getOrDefault("trustScore", "N/A"));
            } else {
                System.out.println("[Analysis Agent] Verification FAILED: " + result.get("error"));
            }

            return result;
        } catch (Exception e) {
            System.out.println("[Analysis Agent] Verification error: " + e.getMessage());
            return Map.of("valid", false, "error", e.getMessage());
        }
    }

    public Map<String, Object> executeSkill(String skillId, Map<String, Object> inputData) {
        System.out.println("[Analysis Agent] Executing skill: " + skillId);

        return switch (skillId) {
            case "sentiment-analysis" -> analyzeSentiment((String) inputData.getOrDefault("text", ""));
            case "summarization" -> summarize((String) inputData.getOrDefault("text", ""));
            case "trend-detection" -> {
                @SuppressWarnings("unchecked")
                List<Map<String, Object>> data = (List<Map<String, Object>>) inputData.getOrDefault("data", List.of());
                yield detectTrends(data);
            }
            default -> Map.of("error", "Unknown skill: " + skillId);
        };
    }

    private Map<String, Object> analyzeSentiment(String text) {
        String textLower = text.toLowerCase();
        int positiveCount = (int) POSITIVE_WORDS.stream().filter(textLower::contains).count();
        int negativeCount = (int) NEGATIVE_WORDS.stream().filter(textLower::contains).count();

        String sentiment;
        double confidence;

        if (positiveCount > negativeCount) {
            sentiment = "positive";
            confidence = Math.min(0.5 + positiveCount * 0.1, 0.95);
        } else if (negativeCount > positiveCount) {
            sentiment = "negative";
            confidence = Math.min(0.5 + negativeCount * 0.1, 0.95);
        } else {
            sentiment = "neutral";
            confidence = 0.6;
        }

        return Map.of(
                "sentiment", sentiment,
                "confidence", Math.round(confidence * 100) / 100.0,
                "wordCount", text.split("\\s+").length,
                "positiveIndicators", positiveCount,
                "negativeIndicators", negativeCount
        );
    }

    private Map<String, Object> summarize(String text) {
        String[] sentences = Arrays.stream(text.split("\\."))
                .map(String::trim)
                .filter(s -> !s.isEmpty())
                .toArray(String[]::new);

        String[] summarySentences = sentences.length > 2
                ? Arrays.copyOfRange(sentences, 0, 2)
                : sentences;

        String summary = summarySentences.length > 0
                ? String.join(". ", summarySentences) + "."
                : text;

        return Map.of(
                "summary", summary,
                "originalLength", text.length(),
                "summaryLength", summary.length(),
                "compressionRatio", Math.round((double) summary.length() / Math.max(text.length(), 1) * 100) / 100.0,
                "sentenceCount", sentences.length
        );
    }

    private Map<String, Object> detectTrends(List<Map<String, Object>> data) {
        if (data == null || data.size() < 2) {
            return Map.of(
                    "trend", "insufficient_data",
                    "dataPoints", data != null ? data.size() : 0,
                    "confidence", 0.0
            );
        }

        List<Double> values = data.stream()
                .filter(Objects::nonNull)
                .map(d -> ((Number) d.getOrDefault("value", 0)).doubleValue())
                .toList();

        if (values.size() < 2) {
            return Map.of(
                    "trend", "insufficient_data",
                    "dataPoints", values.size(),
                    "confidence", 0.0
            );
        }

        int halfIndex = values.size() / 2;
        double firstHalfAvg = values.subList(0, halfIndex).stream()
                .mapToDouble(Double::doubleValue).average().orElse(0);
        double secondHalfAvg = values.subList(halfIndex, values.size()).stream()
                .mapToDouble(Double::doubleValue).average().orElse(0);

        double changePct = firstHalfAvg != 0
                ? ((secondHalfAvg - firstHalfAvg) / firstHalfAvg) * 100
                : 0;

        String trend;
        if (changePct > 5) {
            trend = "upward";
        } else if (changePct < -5) {
            trend = "downward";
        } else {
            trend = "stable";
        }

        return Map.of(
                "trend", trend,
                "changePercent", Math.round(changePct * 10) / 10.0,
                "dataPoints", values.size(),
                "startValue", values.get(0),
                "endValue", values.get(values.size() - 1),
                "confidence", 0.78
        );
    }

    public List<Map<String, Object>> getSkills() {
        return SKILLS;
    }

    public double getTrustScore() {
        try {
            var score = a2a.getA2ATrustScore(null);
            return score.getA2aTrustScore();
        } catch (Exception e) {
            return 0.0;
        }
    }
}
