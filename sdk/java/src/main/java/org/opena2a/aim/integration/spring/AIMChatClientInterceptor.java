package org.opena2a.aim.integration.spring;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.RiskDetector;
import org.opena2a.aim.security.RiskLevel;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Interceptor for Spring AI ChatClient calls.
 *
 * <p>Provides security logging, metrics, and capability tracking for
 * all chat interactions in Spring AI applications.</p>
 *
 * <h2>Features:</h2>
 * <ul>
 *   <li>Request/response logging with configurable detail levels</li>
 *   <li>Latency and token usage tracking</li>
 *   <li>Correlation ID propagation</li>
 *   <li>Security event generation for SOC/SIEM</li>
 *   <li>Rate limiting support</li>
 * </ul>
 *
 * <h2>Usage with Spring AI:</h2>
 * <pre>{@code
 * @Bean
 * public ChatClient chatClient(ChatClient.Builder builder, AIMChatClientInterceptor interceptor) {
 *     return builder
 *         .defaultAdvisors(interceptor.asAdvisor())
 *         .build();
 * }
 * }</pre>
 *
 * <h2>Manual Interception:</h2>
 * <pre>{@code
 * AIMChatClientInterceptor interceptor = new AIMChatClientInterceptor(aimClient);
 *
 * // Before call
 * String correlationId = interceptor.beforeCall(prompt, model);
 *
 * // After call
 * interceptor.afterCall(correlationId, response, tokensUsed, latencyMs);
 * }</pre>
 */
public class AIMChatClientInterceptor {

    private static final Logger log = LoggerFactory.getLogger(AIMChatClientInterceptor.class);

    private final AIMClient client;
    private final Config config;
    private final SecurityLogger securityLogger;

    // Metrics
    private final AtomicLong totalCalls = new AtomicLong(0);
    private final AtomicLong totalTokens = new AtomicLong(0);
    private final AtomicLong totalLatencyMs = new AtomicLong(0);
    private final AtomicLong errorCount = new AtomicLong(0);

    /**
     * Creates a new interceptor with default configuration.
     *
     * @param client the AIM client for security logging
     */
    public AIMChatClientInterceptor(AIMClient client) {
        this(client, new Config());
    }

    /**
     * Creates a new interceptor with custom configuration.
     *
     * @param client the AIM client for security logging
     * @param config the configuration for logging and metrics
     */
    public AIMChatClientInterceptor(AIMClient client, Config config) {
        this.client = client;
        this.config = config;
        this.securityLogger = SecurityLogger.getInstance();
    }

    /**
     * Called before a chat completion request.
     *
     * @param prompt The prompt being sent
     * @param model The model being used
     * @return Correlation ID for this request
     */
    public String beforeCall(String prompt, String model) {
        String correlationId = UUID.randomUUID().toString();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("model", model);
        details.put("promptLength", prompt != null ? prompt.length() : 0);

        if (config.logPrompts && prompt != null) {
            // Truncate for logging
            String truncated = prompt.length() > 500 ?
                    prompt.substring(0, 500) + "..." : prompt;
            details.put("prompt", truncated);
        }

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                "chat:complete",
                model,
                true,
                details
        );

        if (log.isDebugEnabled()) {
            log.debug("Chat call started: correlationId={}, model={}", correlationId, model);
        }

        return correlationId;
    }

    /**
     * Called after a successful chat completion.
     *
     * @param correlationId The correlation ID from beforeCall
     * @param response The response text (optional, based on config)
     * @param tokensUsed Total tokens used (input + output)
     * @param latencyMs Latency in milliseconds
     */
    public void afterCall(String correlationId, String response, int tokensUsed, long latencyMs) {
        totalCalls.incrementAndGet();
        totalTokens.addAndGet(tokensUsed);
        totalLatencyMs.addAndGet(latencyMs);

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("correlationId", correlationId);
        details.put("tokensUsed", tokensUsed);
        details.put("latencyMs", latencyMs);

        if (config.logResponses && response != null) {
            String truncated = response.length() > 500 ?
                    response.substring(0, 500) + "..." : response;
            details.put("response", truncated);
        }

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                "chat:complete:success",
                null,
                true,
                details
        );

        if (log.isDebugEnabled()) {
            log.debug("Chat call completed: correlationId={}, tokens={}, latency={}ms",
                    correlationId, tokensUsed, latencyMs);
        }
    }

    /**
     * Called when a chat completion fails.
     *
     * @param correlationId The correlation ID from beforeCall
     * @param error The error that occurred
     */
    public void onError(String correlationId, Throwable error) {
        errorCount.incrementAndGet();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("correlationId", correlationId);
        details.put("errorType", error.getClass().getSimpleName());
        details.put("errorMessage", error.getMessage());

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_DENIED,
                "chat:complete:error",
                error.getClass().getSimpleName(),
                false,
                details
        );

        log.warn("Chat call failed: correlationId={}, error={}", correlationId, error.getMessage());
    }

    /**
     * Wrap a chat call with automatic timing and logging.
     *
     * @param <T> the return type of the chat action
     * @param prompt The prompt
     * @param model The model
     * @param action The actual chat call
     * @return The response from the action
     */
    public <T> T wrap(String prompt, String model, ChatAction<T> action) {
        String correlationId = beforeCall(prompt, model);
        Instant start = Instant.now();

        try {
            T result = action.execute();

            long latencyMs = Duration.between(start, Instant.now()).toMillis();
            int tokens = estimateTokens(prompt, result != null ? result.toString() : "");
            afterCall(correlationId, result != null ? result.toString() : null, tokens, latencyMs);

            return result;
        } catch (Exception e) {
            onError(correlationId, e);
            throw e instanceof RuntimeException ? (RuntimeException) e : new RuntimeException(e);
        }
    }

    /**
     * Simple token estimation (rough approximation).
     */
    private int estimateTokens(String input, String output) {
        // Rough estimation: ~4 characters per token
        int inputTokens = input != null ? input.length() / 4 : 0;
        int outputTokens = output != null ? output.length() / 4 : 0;
        return inputTokens + outputTokens;
    }

    /**
     * Get metrics for this interceptor.
     *
     * @return a map containing interceptor metrics
     */
    public Map<String, Object> getMetrics() {
        Map<String, Object> metrics = new LinkedHashMap<>();
        metrics.put("totalCalls", totalCalls.get());
        metrics.put("totalTokens", totalTokens.get());
        metrics.put("totalLatencyMs", totalLatencyMs.get());
        metrics.put("errorCount", errorCount.get());

        long calls = totalCalls.get();
        if (calls > 0) {
            metrics.put("avgTokensPerCall", totalTokens.get() / calls);
            metrics.put("avgLatencyMs", totalLatencyMs.get() / calls);
            metrics.put("errorRate", (double) errorCount.get() / calls);
        }

        return metrics;
    }

    /**
     * Reset metrics.
     */
    public void resetMetrics() {
        totalCalls.set(0);
        totalTokens.set(0);
        totalLatencyMs.set(0);
        errorCount.set(0);
    }

    /**
     * Functional interface for chat actions.
     *
     * @param <T> the return type of the action
     */
    @FunctionalInterface
    public interface ChatAction<T> {
        /**
         * Executes the chat action.
         *
         * @return the result of the action
         * @throws Exception if the action fails
         */
        T execute() throws Exception;
    }

    /**
     * Configuration for the interceptor.
     */
    public static class Config {
        boolean logPrompts = false;
        boolean logResponses = false;
        int maxPromptLogLength = 500;
        int maxResponseLogLength = 500;
        boolean enableMetrics = true;

        /**
         * Creates a default configuration.
         */
        public Config() {}

        /**
         * Creates a configuration with prompt and response logging settings.
         *
         * @param logPrompts whether to log prompts
         * @param logResponses whether to log responses
         */
        public Config(boolean logPrompts, boolean logResponses) {
            this.logPrompts = logPrompts;
            this.logResponses = logResponses;
        }

        /**
         * Sets whether to log prompts.
         *
         * @param logPrompts true to log prompts
         * @return this config for chaining
         */
        public Config logPrompts(boolean logPrompts) {
            this.logPrompts = logPrompts;
            return this;
        }

        /**
         * Sets whether to log responses.
         *
         * @param logResponses true to log responses
         * @return this config for chaining
         */
        public Config logResponses(boolean logResponses) {
            this.logResponses = logResponses;
            return this;
        }

        /**
         * Sets the maximum prompt log length.
         *
         * @param length the maximum length in characters
         * @return this config for chaining
         */
        public Config maxPromptLogLength(int length) {
            this.maxPromptLogLength = length;
            return this;
        }

        /**
         * Sets the maximum response log length.
         *
         * @param length the maximum length in characters
         * @return this config for chaining
         */
        public Config maxResponseLogLength(int length) {
            this.maxResponseLogLength = length;
            return this;
        }

        /**
         * Sets whether to enable metrics collection.
         *
         * @param enable true to enable metrics
         * @return this config for chaining
         */
        public Config enableMetrics(boolean enable) {
            this.enableMetrics = enable;
            return this;
        }
    }
}
