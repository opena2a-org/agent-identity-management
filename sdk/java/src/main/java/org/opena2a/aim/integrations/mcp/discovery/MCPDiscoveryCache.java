package org.opena2a.aim.integrations.mcp.discovery;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.*;

/**
 * File-based cache for MCP discovery results.
 *
 * Caches discovered capabilities to avoid expensive subprocess spawning on every agent restart.
 * Cache is invalidated when:
 * - TTL expires (default 1 hour)
 * - MCP command changes (detected via hash)
 * - Cache file is corrupted or missing
 *
 * Cache location: ~/.aim/mcp_cache/{serverName}.json
 */
public class MCPDiscoveryCache {

    private static final ObjectMapper objectMapper = new ObjectMapper()
            .enable(SerializationFeature.INDENT_OUTPUT);
    private static final long DEFAULT_TTL_MS = 60 * 60 * 1000; // 1 hour
    private static final Path CACHE_DIR = Path.of(System.getProperty("user.home"), ".aim", "mcp_cache");

    private final long ttlMs;

    public MCPDiscoveryCache() {
        this(DEFAULT_TTL_MS);
    }

    public MCPDiscoveryCache(long ttlMs) {
        this.ttlMs = ttlMs;
        ensureCacheDir();
    }

    /**
     * Get cached discovery result if valid.
     *
     * @param serverName MCP server name
     * @param command    MCP command used for discovery
     * @return Cached result or null if not found/expired/invalid
     */
    public MCPDiscoveryResult get(String serverName, String command) {
        Path cacheFile = getCacheFilePath(serverName);
        if (!Files.exists(cacheFile)) {
            return null;
        }

        try {
            Map<String, Object> cached = objectMapper.readValue(
                    cacheFile.toFile(),
                    new TypeReference<Map<String, Object>>() {}
            );

            // Check command hash (invalidate if command changed)
            String cachedCommandHash = (String) cached.get("commandHash");
            String currentCommandHash = hashCommand(command);
            if (!currentCommandHash.equals(cachedCommandHash)) {
                return null;
            }

            // Check TTL
            long cachedAt = ((Number) cached.get("cachedAt")).longValue();
            if (System.currentTimeMillis() - cachedAt > ttlMs) {
                return null;
            }

            // Reconstruct MCPDiscoveryResult
            return reconstructResult(cached);

        } catch (Exception e) {
            // Cache corrupted, delete it
            try {
                Files.deleteIfExists(cacheFile);
            } catch (IOException ignored) {}
            return null;
        }
    }

    /**
     * Store discovery result in cache.
     *
     * @param serverName MCP server name
     * @param command    MCP command used for discovery
     * @param result     Discovery result to cache
     */
    public void put(String serverName, String command, MCPDiscoveryResult result) {
        if (result == null || result.hasError()) {
            return; // Don't cache errors
        }

        Path cacheFile = getCacheFilePath(serverName);

        try {
            Map<String, Object> cacheEntry = new LinkedHashMap<>();
            cacheEntry.put("serverName", serverName);
            cacheEntry.put("commandHash", hashCommand(command));
            cacheEntry.put("cachedAt", System.currentTimeMillis());
            cacheEntry.put("serverInfo", Map.of(
                    "name", result.getServerName(),
                    "version", result.getServerVersion(),
                    "protocolVersion", result.getProtocolVersion()
            ));
            cacheEntry.put("connectionLatencyMs", result.getConnectionLatencyMs());
            cacheEntry.put("discoveryTimeMs", result.getDiscoveryTimeMs());

            // Serialize tools
            List<Map<String, Object>> tools = new ArrayList<>();
            for (MCPTool tool : result.getTools()) {
                Map<String, Object> t = new LinkedHashMap<>();
                t.put("name", tool.getName());
                t.put("description", tool.getDescription());
                t.put("inputSchema", tool.getInputSchema());
                tools.add(t);
            }
            cacheEntry.put("tools", tools);

            // Serialize resources
            List<Map<String, Object>> resources = new ArrayList<>();
            for (MCPResource resource : result.getResources()) {
                Map<String, Object> r = new LinkedHashMap<>();
                r.put("uri", resource.getUri());
                r.put("name", resource.getName());
                r.put("description", resource.getDescription());
                r.put("mimeType", resource.getMimeType());
                resources.add(r);
            }
            cacheEntry.put("resources", resources);

            // Serialize prompts
            List<Map<String, Object>> prompts = new ArrayList<>();
            for (MCPPrompt prompt : result.getPrompts()) {
                Map<String, Object> p = new LinkedHashMap<>();
                p.put("name", prompt.getName());
                p.put("description", prompt.getDescription());
                p.put("arguments", prompt.getArguments());
                prompts.add(p);
            }
            cacheEntry.put("prompts", prompts);

            objectMapper.writeValue(cacheFile.toFile(), cacheEntry);

        } catch (Exception e) {
            // Silently fail - caching is best-effort
        }
    }

    /**
     * Invalidate cache for a specific server.
     */
    public void invalidate(String serverName) {
        try {
            Files.deleteIfExists(getCacheFilePath(serverName));
        } catch (IOException ignored) {}
    }

    /**
     * Clear entire cache.
     */
    public void clear() {
        try {
            if (Files.exists(CACHE_DIR)) {
                Files.list(CACHE_DIR)
                        .filter(p -> p.toString().endsWith(".json"))
                        .forEach(p -> {
                            try {
                                Files.delete(p);
                            } catch (IOException ignored) {}
                        });
            }
        } catch (IOException ignored) {}
    }

    private Path getCacheFilePath(String serverName) {
        // Sanitize server name for filesystem
        String safeName = serverName.replaceAll("[^a-zA-Z0-9_-]", "_");
        return CACHE_DIR.resolve(safeName + ".json");
    }

    private void ensureCacheDir() {
        try {
            Files.createDirectories(CACHE_DIR);
        } catch (IOException ignored) {}
    }

    private String hashCommand(String command) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(command.getBytes(StandardCharsets.UTF_8));
            StringBuilder hex = new StringBuilder();
            for (int i = 0; i < 8; i++) { // First 8 bytes = 16 hex chars
                hex.append(String.format("%02x", hash[i]));
            }
            return hex.toString();
        } catch (NoSuchAlgorithmException e) {
            return String.valueOf(command.hashCode());
        }
    }

    @SuppressWarnings("unchecked")
    private MCPDiscoveryResult reconstructResult(Map<String, Object> cached) {
        Map<String, Object> serverInfo = (Map<String, Object>) cached.get("serverInfo");

        MCPDiscoveryResult.Builder builder = MCPDiscoveryResult.builder()
                .serverName((String) serverInfo.get("name"))
                .serverVersion((String) serverInfo.get("version"))
                .protocolVersion((String) serverInfo.get("protocolVersion"))
                .connectionLatencyMs(((Number) cached.get("connectionLatencyMs")).longValue())
                .discoveryTimeMs(((Number) cached.get("discoveryTimeMs")).longValue());

        // Reconstruct tools
        List<Map<String, Object>> tools = (List<Map<String, Object>>) cached.get("tools");
        if (tools != null) {
            for (Map<String, Object> t : tools) {
                builder.addTool(new MCPTool(
                        (String) t.get("name"),
                        (String) t.get("description"),
                        (Map<String, Object>) t.get("inputSchema")
                ));
            }
        }

        // Reconstruct resources
        List<Map<String, Object>> resources = (List<Map<String, Object>>) cached.get("resources");
        if (resources != null) {
            for (Map<String, Object> r : resources) {
                builder.addResource(new MCPResource(
                        (String) r.get("uri"),
                        (String) r.get("name"),
                        (String) r.get("description"),
                        (String) r.get("mimeType")
                ));
            }
        }

        // Reconstruct prompts
        List<Map<String, Object>> prompts = (List<Map<String, Object>>) cached.get("prompts");
        if (prompts != null) {
            for (Map<String, Object> p : prompts) {
                builder.addPrompt(new MCPPrompt(
                        (String) p.get("name"),
                        (String) p.get("description"),
                        (List<Map<String, Object>>) p.get("arguments")
                ));
            }
        }

        return builder.build();
    }
}
