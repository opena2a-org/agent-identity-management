package org.opena2a.aim.integration.langchain4j;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Chat model listener for LangChain4j with AIM security integration.
 *
 * <p>Implements LangChain4j's listener pattern to provide:</p>
 * <ul>
 *   <li>Request/response logging for audit trail</li>
 *   <li>Token usage and latency tracking</li>
 *   <li>Error monitoring and alerting</li>
 *   <li>Model-specific metrics</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * AIMChatModelListener listener = new AIMChatModelListener(aimClient);
 *
 * // Manual usage
 * String requestId = listener.onRequest(messages, modelName);
 * // ... model call ...
 * listener.onResponse(requestId, response, inputTokens, outputTokens);
 *
 * // Or with wrapper
 * Response<AiMessage> response = listener.wrap(modelName, messages,
 *     () -> chatModel.generate(messages));
 * }</pre>
 *
 * <h2>Streaming Support:</h2>
 * <pre>{@code
 * String streamId = listener.onStreamStart(messages, modelName);
 * // ... streaming ...
 * listener.onStreamChunk(streamId, chunk);
 * listener.onStreamComplete(streamId, fullResponse, tokens);
 * }</pre>
 */
public class AIMChatModelListener {

    private static final Logger log = LoggerFactory.getLogger(AIMChatModelListener.class);

    private final AIMClient client;
    private final Config config;
    private final SecurityLogger securityLogger;

    // Active requests for timing
    private final Map<String, RequestContext> activeRequests = new ConcurrentHashMap<>();

    // Metrics per model
    private final Map<String, ModelMetrics> modelMetrics = new ConcurrentHashMap<>();

    /**
     * Creates a new listener with default configuration.
     *
     * @param client the AIM client for security logging
     */
    public AIMChatModelListener(AIMClient client) {
        this(client, new Config());
    }

    /**
     * Creates a new listener with custom configuration.
     *
     * @param client the AIM client for security logging
     * @param config the configuration for logging and metrics
     */
    public AIMChatModelListener(AIMClient client, Config config) {
        this.client = client;
        this.config = config;
        this.securityLogger = SecurityLogger.getInstance();
    }

    /**
     * Called when a chat request is made.
     *
     * @param messages The messages being sent (as string for simplicity)
     * @param modelName The model name
     * @return Request ID for tracking
     */
    public String onRequest(String messages, String modelName) {
        String requestId = UUID.randomUUID().toString();

        RequestContext ctx = new RequestContext();
        ctx.requestId = requestId;
        ctx.modelName = modelName;
        ctx.startTime = Instant.now();
        ctx.inputLength = messages != null ? messages.length() : 0;
        activeRequests.put(requestId, ctx);

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("requestId", requestId);
        details.put("model", modelName);
        details.put("inputLength", ctx.inputLength);

        if (config.logPrompts && messages != null) {
            String truncated = messages.length() > config.maxPromptLength ?
                    messages.substring(0, config.maxPromptLength) + "..." : messages;
            details.put("messages", truncated);
        }

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                "chat:request",
                modelName,
                true,
                details
        );

        log.debug("Chat request started: requestId={}, model={}", requestId, modelName);
        return requestId;
    }

    /**
     * Called when a chat response is received.
     *
     * @param requestId The request ID from onRequest
     * @param response The response text
     * @param inputTokens Input token count
     * @param outputTokens Output token count
     */
    public void onResponse(String requestId, String response, int inputTokens, int outputTokens) {
        RequestContext ctx = activeRequests.remove(requestId);
        if (ctx == null) {
            log.warn("Unknown request ID: {}", requestId);
            return;
        }

        long latencyMs = Duration.between(ctx.startTime, Instant.now()).toMillis();
        int totalTokens = inputTokens + outputTokens;

        // Update metrics
        ModelMetrics metrics = modelMetrics.computeIfAbsent(ctx.modelName, k -> new ModelMetrics());
        metrics.requestCount.incrementAndGet();
        metrics.totalInputTokens.addAndGet(inputTokens);
        metrics.totalOutputTokens.addAndGet(outputTokens);
        metrics.totalLatencyMs.addAndGet(latencyMs);

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("requestId", requestId);
        details.put("model", ctx.modelName);
        details.put("inputTokens", inputTokens);
        details.put("outputTokens", outputTokens);
        details.put("totalTokens", totalTokens);
        details.put("latencyMs", latencyMs);

        if (config.logResponses && response != null) {
            String truncated = response.length() > config.maxResponseLength ?
                    response.substring(0, config.maxResponseLength) + "..." : response;
            details.put("response", truncated);
        }

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_EXECUTED,
                "chat:response",
                ctx.modelName,
                true,
                details
        );

        log.debug("Chat response received: requestId={}, tokens={}, latency={}ms",
                requestId, totalTokens, latencyMs);
    }

    /**
     * Called when a chat request fails.
     *
     * @param requestId The request ID
     * @param error The error that occurred
     */
    public void onError(String requestId, Throwable error) {
        RequestContext ctx = activeRequests.remove(requestId);

        ModelMetrics metrics = modelMetrics.computeIfAbsent(
                ctx != null ? ctx.modelName : "unknown", k -> new ModelMetrics());
        metrics.errorCount.incrementAndGet();

        Map<String, Object> details = new LinkedHashMap<>();
        details.put("requestId", requestId);
        if (ctx != null) {
            details.put("model", ctx.modelName);
            details.put("latencyMs", Duration.between(ctx.startTime, Instant.now()).toMillis());
        }
        details.put("errorType", error.getClass().getSimpleName());
        details.put("errorMessage", error.getMessage());

        securityLogger.logAuthorizationEvent(
                EventTypes.Authz.ACTION_DENIED,
                "chat:error",
                ctx != null ? ctx.modelName : "unknown",
                false,
                details
        );

        log.warn("Chat request failed: requestId={}, error={}", requestId, error.getMessage());
    }

    /**
     * Called when a streaming response starts.
     *
     * @param messages the messages being sent
     * @param modelName the model name
     * @return request ID for tracking the stream
     */
    public String onStreamStart(String messages, String modelName) {
        return onRequest(messages, modelName);
    }

    /**
     * Called for each streaming chunk (optional tracking).
     *
     * @param requestId the request ID from onStreamStart
     * @param chunk the chunk content
     */
    public void onStreamChunk(String requestId, String chunk) {
        RequestContext ctx = activeRequests.get(requestId);
        if (ctx != null) {
            ctx.outputLength += chunk != null ? chunk.length() : 0;
            ctx.chunkCount++;
        }
    }

    /**
     * Called when streaming completes.
     *
     * @param requestId the request ID from onStreamStart
     * @param fullResponse the complete response content
     * @param totalTokens the total tokens used
     */
    public void onStreamComplete(String requestId, String fullResponse, int totalTokens) {
        RequestContext ctx = activeRequests.get(requestId);
        int inputTokens = ctx != null ? estimateTokens(ctx.inputLength) : 0;
        int outputTokens = totalTokens - inputTokens;
        onResponse(requestId, fullResponse, inputTokens, outputTokens);
    }

    /**
     * Wrap a chat model call with automatic logging.
     *
     * @param <T> the return type of the chat action
     * @param modelName The model name
     * @param messages The input messages
     * @param action The actual model call
     * @return The response from the action
     */
    public <T> T wrap(String modelName, String messages, ChatAction<T> action) {
        String requestId = onRequest(messages, modelName);
        try {
            T result = action.execute();

            // Estimate tokens if not available
            int inputTokens = estimateTokens(messages != null ? messages.length() : 0);
            int outputTokens = result != null ? estimateTokens(result.toString().length()) : 0;
            onResponse(requestId, result != null ? result.toString() : null, inputTokens, outputTokens);

            return result;
        } catch (Exception e) {
            onError(requestId, e);
            throw e instanceof RuntimeException ? (RuntimeException) e : new RuntimeException(e);
        }
    }

    /**
     * Simple token estimation (~4 chars per token).
     */
    private int estimateTokens(int charCount) {
        return charCount / 4;
    }

    /**
     * Get metrics for all models.
     *
     * @return a map of model names to their metrics
     */
    public Map<String, Map<String, Object>> getAllMetrics() {
        Map<String, Map<String, Object>> result = new LinkedHashMap<>();
        for (Map.Entry<String, ModelMetrics> entry : modelMetrics.entrySet()) {
            result.put(entry.getKey(), entry.getValue().toMap());
        }
        return result;
    }

    /**
     * Get metrics for a specific model.
     *
     * @param modelName the model name to get metrics for
     * @return a map containing the model's metrics
     */
    public Map<String, Object> getMetrics(String modelName) {
        ModelMetrics m = modelMetrics.get(modelName);
        return m != null ? m.toMap() : new LinkedHashMap<>();
    }

    /**
     * Reset all metrics.
     */
    public void resetMetrics() {
        modelMetrics.clear();
    }

    // Internal classes

    private static class RequestContext {
        String requestId;
        String modelName;
        Instant startTime;
        int inputLength;
        int outputLength;
        int chunkCount;
    }

    private static class ModelMetrics {
        final AtomicLong requestCount = new AtomicLong(0);
        final AtomicLong totalInputTokens = new AtomicLong(0);
        final AtomicLong totalOutputTokens = new AtomicLong(0);
        final AtomicLong totalLatencyMs = new AtomicLong(0);
        final AtomicLong errorCount = new AtomicLong(0);

        Map<String, Object> toMap() {
            Map<String, Object> map = new LinkedHashMap<>();
            long count = requestCount.get();
            map.put("requestCount", count);
            map.put("totalInputTokens", totalInputTokens.get());
            map.put("totalOutputTokens", totalOutputTokens.get());
            map.put("totalTokens", totalInputTokens.get() + totalOutputTokens.get());
            map.put("errorCount", errorCount.get());
            if (count > 0) {
                map.put("avgInputTokens", totalInputTokens.get() / count);
                map.put("avgOutputTokens", totalOutputTokens.get() / count);
                map.put("avgLatencyMs", totalLatencyMs.get() / count);
                map.put("errorRate", (double) errorCount.get() / count);
            }
            return map;
        }
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
     * Configuration for the listener.
     */
    public static class Config {
        boolean logPrompts = false;
        boolean logResponses = false;
        int maxPromptLength = 500;
        int maxResponseLength = 500;

        /**
         * Creates a default configuration.
         */
        public Config() {}

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
        public Config maxPromptLength(int length) {
            this.maxPromptLength = length;
            return this;
        }

        /**
         * Sets the maximum response log length.
         *
         * @param length the maximum length in characters
         * @return this config for chaining
         */
        public Config maxResponseLength(int length) {
            this.maxResponseLength = length;
            return this;
        }
    }
}
