package org.opena2a.aim.integrations.mcp.discovery;

import java.util.*;
import java.util.concurrent.*;
import java.util.stream.Collectors;

/**
 * Production-grade MCP discovery service with parallel execution and caching.
 *
 * <p>Features:</p>
 * <ul>
 *   <li>Parallel discovery of multiple MCP servers using CompletableFuture</li>
 *   <li>File-based caching with configurable TTL (default 1 hour)</li>
 *   <li>Graceful timeout handling per server</li>
 *   <li>Automatic retry on transient failures</li>
 *   <li>Thread-safe singleton pattern</li>
 * </ul>
 *
 * <p>Usage:</p>
 * <pre>{@code
 * Map<String, String> mcpCommands = Map.of("github", "npx -y @modelcontextprotocol/server-github");
 * Map<String, MCPDiscoveryResult> results = MCPDiscoveryService.getInstance()
 *     .discoverAll(mcpCommands);
 * }</pre>
 */
public class MCPDiscoveryService {

    private static final int DEFAULT_TIMEOUT_SECONDS = 30;
    private static final int DEFAULT_PARALLELISM = 4;
    private static final int MAX_RETRIES = 2;

    private static volatile MCPDiscoveryService instance;
    private static final Object lock = new Object();

    private final MCPDiscoveryCache cache;
    private final ExecutorService executor;
    private final int timeoutSeconds;
    private final boolean useCache;

    private MCPDiscoveryService(Builder builder) {
        this.cache = builder.cache != null ? builder.cache : new MCPDiscoveryCache();
        this.executor = builder.executor != null ? builder.executor :
                Executors.newFixedThreadPool(builder.parallelism, r -> {
                    Thread t = new Thread(r, "mcp-discovery");
                    t.setDaemon(true);
                    return t;
                });
        this.timeoutSeconds = builder.timeoutSeconds;
        this.useCache = builder.useCache;
    }

    /**
     * Get the singleton instance with default configuration.
     */
    public static MCPDiscoveryService getInstance() {
        if (instance == null) {
            synchronized (lock) {
                if (instance == null) {
                    instance = new Builder().build();
                }
            }
        }
        return instance;
    }

    /**
     * Create a custom instance with specific configuration.
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Discover capabilities from multiple MCP servers in parallel.
     *
     * @param mcpCommands Map of server name to command
     * @return Map of server name to discovery result
     */
    public Map<String, MCPDiscoveryResult> discoverAll(Map<String, String> mcpCommands) {
        if (mcpCommands == null || mcpCommands.isEmpty()) {
            return Collections.emptyMap();
        }

        // Submit all discoveries in parallel
        Map<String, CompletableFuture<MCPDiscoveryResult>> futures = new LinkedHashMap<>();
        for (Map.Entry<String, String> entry : mcpCommands.entrySet()) {
            String serverName = entry.getKey();
            String command = entry.getValue();

            CompletableFuture<MCPDiscoveryResult> future = CompletableFuture.supplyAsync(
                    () -> discoverWithRetry(serverName, command),
                    executor
            );
            futures.put(serverName, future);
        }

        // Collect results with timeout
        Map<String, MCPDiscoveryResult> results = new LinkedHashMap<>();
        for (Map.Entry<String, CompletableFuture<MCPDiscoveryResult>> entry : futures.entrySet()) {
            String serverName = entry.getKey();
            try {
                MCPDiscoveryResult result = entry.getValue().get(timeoutSeconds + 5, TimeUnit.SECONDS);
                results.put(serverName, result);
            } catch (TimeoutException e) {
                results.put(serverName, MCPDiscoveryResult.builder()
                        .serverName(serverName)
                        .error("Discovery timed out after " + timeoutSeconds + " seconds")
                        .build());
            } catch (Exception e) {
                results.put(serverName, MCPDiscoveryResult.builder()
                        .serverName(serverName)
                        .error("Discovery failed: " + e.getMessage())
                        .build());
            }
        }

        return results;
    }

    /**
     * Discover capabilities from a single MCP server.
     *
     * @param serverName Server name for display/caching
     * @param command    MCP server command
     * @return Discovery result
     */
    public MCPDiscoveryResult discover(String serverName, String command) {
        return discoverWithRetry(serverName, command);
    }

    /**
     * Discover with caching and retry logic.
     */
    private MCPDiscoveryResult discoverWithRetry(String serverName, String command) {
        // Check cache first
        if (useCache) {
            MCPDiscoveryResult cached = cache.get(serverName, command);
            if (cached != null) {
                return cached;
            }
        }

        // Attempt discovery with retries
        MCPDiscoveryResult result = null;
        Exception lastError = null;

        for (int attempt = 0; attempt <= MAX_RETRIES; attempt++) {
            try {
                result = MCPClient.discover(command, timeoutSeconds);
                if (result.isSuccess()) {
                    // Cache successful result
                    if (useCache) {
                        cache.put(serverName, command, result);
                    }
                    return result;
                }
                // Non-success result, might be retryable
                if (attempt < MAX_RETRIES && isRetryable(result.getError())) {
                    Thread.sleep(500 * (attempt + 1)); // Exponential backoff
                    continue;
                }
                return result;
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return MCPDiscoveryResult.builder()
                        .serverName(serverName)
                        .error("Discovery interrupted")
                        .build();
            } catch (Exception e) {
                lastError = e;
                if (attempt < MAX_RETRIES && isRetryable(e)) {
                    try {
                        Thread.sleep(500 * (attempt + 1));
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            }
        }

        return MCPDiscoveryResult.builder()
                .serverName(serverName)
                .error(lastError != null ? lastError.getMessage() : "Unknown error")
                .build();
    }

    /**
     * Check if an error is retryable.
     */
    private boolean isRetryable(String error) {
        if (error == null) return false;
        String lower = error.toLowerCase();
        return lower.contains("timeout") ||
               lower.contains("connection") ||
               lower.contains("temporarily") ||
               lower.contains("econnreset") ||
               lower.contains("econnrefused");
    }

    private boolean isRetryable(Exception e) {
        return isRetryable(e.getMessage());
    }

    /**
     * Invalidate cache for a specific server.
     */
    public void invalidateCache(String serverName) {
        cache.invalidate(serverName);
    }

    /**
     * Clear all cached discovery results.
     */
    public void clearCache() {
        cache.clear();
    }

    /**
     * Shutdown the discovery service.
     * Call this when the application is shutting down.
     */
    public void shutdown() {
        executor.shutdown();
        try {
            if (!executor.awaitTermination(5, TimeUnit.SECONDS)) {
                executor.shutdownNow();
            }
        } catch (InterruptedException e) {
            executor.shutdownNow();
            Thread.currentThread().interrupt();
        }
    }

    /**
     * Builder for MCPDiscoveryService.
     */
    public static class Builder {
        private MCPDiscoveryCache cache;
        private ExecutorService executor;
        private int parallelism = DEFAULT_PARALLELISM;
        private int timeoutSeconds = DEFAULT_TIMEOUT_SECONDS;
        private boolean useCache = true;

        public Builder cache(MCPDiscoveryCache cache) {
            this.cache = cache;
            return this;
        }

        public Builder executor(ExecutorService executor) {
            this.executor = executor;
            return this;
        }

        public Builder parallelism(int parallelism) {
            this.parallelism = parallelism;
            return this;
        }

        public Builder timeoutSeconds(int timeoutSeconds) {
            this.timeoutSeconds = timeoutSeconds;
            return this;
        }

        public Builder useCache(boolean useCache) {
            this.useCache = useCache;
            return this;
        }

        public MCPDiscoveryService build() {
            return new MCPDiscoveryService(this);
        }
    }
}
