package org.opena2a.aim.mcp;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import org.opena2a.aim.integrations.mcp.discovery.MCPDiscoveryResult;
import org.opena2a.aim.integrations.mcp.discovery.MCPPrompt;
import org.opena2a.aim.integrations.mcp.discovery.MCPResource;
import org.opena2a.aim.integrations.mcp.discovery.MCPTool;
import org.opena2a.aim.security.EventTypes;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.file.*;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * Attestation cache with capability drift detection.
 *
 * <p>Caches MCP attestation results and detects when server capabilities
 * change between discovery runs - critical for enterprise security auditing.</p>
 *
 * <h2>Features:</h2>
 * <ul>
 *   <li>File-based persistent cache with TTL</li>
 *   <li>Capability drift detection (tools added/removed/modified)</li>
 *   <li>Resource and prompt tracking</li>
 *   <li>Automatic security event logging on drift</li>
 *   <li>Configurable drift severity thresholds</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * AttestationCache cache = AttestationCache.getInstance();
 *
 * // Store attestation
 * cache.store("github-mcp", discoveryResult, attestationId);
 *
 * // Retrieve and check for drift
 * Optional<CachedAttestation> cached = cache.get("github-mcp");
 * if (cached.isPresent()) {
 *     DriftReport drift = cache.detectDrift("github-mcp", newDiscoveryResult);
 *     if (drift.hasDrift()) {
 *         // Handle capability changes
 *     }
 * }
 * }</pre>
 *
 * <h2>Drift Detection:</h2>
 * <p>Detects the following changes:</p>
 * <ul>
 *   <li>Tools: added, removed, schema changes</li>
 *   <li>Resources: added, removed, URI changes</li>
 *   <li>Prompts: added, removed, argument changes</li>
 * </ul>
 */
public class AttestationCache {

    private static final Logger log = LoggerFactory.getLogger(AttestationCache.class);
    private static final ObjectMapper objectMapper = new ObjectMapper()
            .enable(SerializationFeature.INDENT_OUTPUT);
    private static volatile AttestationCache instance;
    private static final Object lock = new Object();

    // Cache configuration
    private static final Path CACHE_DIR = Path.of(
            System.getProperty("user.home"), ".aim", "attestation_cache");
    private static final long DEFAULT_TTL_MS = 24 * 60 * 60 * 1000; // 24 hours
    private static final String CACHE_EXTENSION = ".attestation.json";
    private static final String HISTORY_EXTENSION = ".history.json";

    // In-memory cache for fast access
    private final Map<String, CachedAttestation> memoryCache = new ConcurrentHashMap<>();

    // Drift detection configuration
    private boolean autoLogDrift = true;
    private int maxHistoryEntries = 100;

    private AttestationCache() {
        try {
            Files.createDirectories(CACHE_DIR);
            loadFromDisk();
        } catch (IOException e) {
            log.warn("Failed to initialize attestation cache directory: {}", e.getMessage());
        }
    }

    /**
     * Get the singleton instance.
     *
     * @return the singleton AttestationCache instance
     */
    public static AttestationCache getInstance() {
        if (instance == null) {
            synchronized (lock) {
                if (instance == null) {
                    instance = new AttestationCache();
                }
            }
        }
        return instance;
    }

    /**
     * Load existing cache entries from disk.
     */
    private void loadFromDisk() {
        try (var stream = Files.list(CACHE_DIR)) {
            stream.filter(p -> p.toString().endsWith(CACHE_EXTENSION))
                    .forEach(this::loadCacheFile);
        } catch (IOException e) {
            log.warn("Failed to load attestation cache: {}", e.getMessage());
        }
    }

    private void loadCacheFile(Path file) {
        try {
            CachedAttestation cached = objectMapper.readValue(file.toFile(), CachedAttestation.class);
            if (!cached.isExpired()) {
                memoryCache.put(cached.serverName, cached);
            }
        } catch (IOException e) {
            log.warn("Failed to load cache file {}: {}", file, e.getMessage());
        }
    }

    /**
     * Store an attestation in the cache.
     *
     * @param serverName MCP server name
     * @param discoveryResult Discovery result to cache
     * @param attestationId Attestation ID from AIM server
     * @return Drift report if previous attestation existed
     */
    public Optional<DriftReport> store(String serverName, MCPDiscoveryResult discoveryResult,
                                        String attestationId) {
        return store(serverName, discoveryResult, attestationId, DEFAULT_TTL_MS);
    }

    /**
     * Store an attestation with custom TTL.
     *
     * @param serverName MCP server name
     * @param discoveryResult Discovery result to cache
     * @param attestationId Attestation ID from AIM server
     * @param ttlMs Time-to-live in milliseconds
     * @return Drift report if previous attestation existed
     */
    public Optional<DriftReport> store(String serverName, MCPDiscoveryResult discoveryResult,
                                        String attestationId, long ttlMs) {
        // Check for drift before storing
        Optional<DriftReport> drift = Optional.empty();
        CachedAttestation existing = memoryCache.get(serverName);
        if (existing != null) {
            drift = Optional.of(detectDrift(existing, discoveryResult));
            if (drift.get().hasDrift() && autoLogDrift) {
                logDriftEvent(serverName, drift.get());
            }
        }

        // Create new cached attestation
        CachedAttestation cached = new CachedAttestation();
        cached.serverName = serverName;
        cached.attestationId = attestationId;
        cached.cachedAt = Instant.now().toString();
        cached.expiresAt = Instant.now().plusMillis(ttlMs).toString();
        cached.toolCount = discoveryResult.getTools() != null ? discoveryResult.getTools().size() : 0;
        cached.resourceCount = discoveryResult.getResources() != null ? discoveryResult.getResources().size() : 0;
        cached.promptCount = discoveryResult.getPrompts() != null ? discoveryResult.getPrompts().size() : 0;
        cached.contentHash = computeContentHash(discoveryResult);
        cached.toolSignatures = computeToolSignatures(discoveryResult);
        cached.resourceSignatures = computeResourceSignatures(discoveryResult);
        cached.promptSignatures = computePromptSignatures(discoveryResult);

        // Store in memory
        memoryCache.put(serverName, cached);

        // Persist to disk
        persistToDisk(serverName, cached);

        // Record in history
        recordHistory(serverName, cached, drift.orElse(null));

        return drift;
    }

    /**
     * Get a cached attestation.
     *
     * @param serverName the MCP server name
     * @return the cached attestation if present and not expired
     */
    public Optional<CachedAttestation> get(String serverName) {
        CachedAttestation cached = memoryCache.get(serverName);
        if (cached != null && !cached.isExpired()) {
            return Optional.of(cached);
        }
        return Optional.empty();
    }

    /**
     * Check if a valid attestation exists for a server.
     *
     * @param serverName the MCP server name
     * @return true if a valid (non-expired) attestation exists
     */
    public boolean hasValidAttestation(String serverName) {
        return get(serverName).isPresent();
    }

    /**
     * Detect drift between cached attestation and new discovery result.
     *
     * @param serverName the MCP server name
     * @param newResult the new discovery result to compare
     * @return drift report detailing any changes
     */
    public DriftReport detectDrift(String serverName, MCPDiscoveryResult newResult) {
        CachedAttestation cached = memoryCache.get(serverName);
        if (cached == null) {
            return DriftReport.noPreviousAttestation(serverName);
        }
        return detectDrift(cached, newResult);
    }

    /**
     * Detect drift between cached and new discovery results.
     */
    private DriftReport detectDrift(CachedAttestation cached, MCPDiscoveryResult newResult) {
        DriftReport report = new DriftReport(cached.serverName);

        // Compute new signatures
        Map<String, String> newToolSigs = computeToolSignatures(newResult);
        Map<String, String> newResourceSigs = computeResourceSignatures(newResult);
        Map<String, String> newPromptSigs = computePromptSignatures(newResult);

        // Detect tool changes
        detectMapChanges(cached.toolSignatures, newToolSigs, "tool", report);

        // Detect resource changes
        detectMapChanges(cached.resourceSignatures, newResourceSigs, "resource", report);

        // Detect prompt changes
        detectMapChanges(cached.promptSignatures, newPromptSigs, "prompt", report);

        return report;
    }

    /**
     * Detect changes between old and new signature maps.
     */
    private void detectMapChanges(Map<String, String> oldSigs, Map<String, String> newSigs,
                                   String itemType, DriftReport report) {
        if (oldSigs == null) oldSigs = Collections.emptyMap();
        if (newSigs == null) newSigs = Collections.emptyMap();

        // Find added items
        for (String name : newSigs.keySet()) {
            if (!oldSigs.containsKey(name)) {
                report.addChange(DriftChange.added(itemType, name));
            }
        }

        // Find removed items
        for (String name : oldSigs.keySet()) {
            if (!newSigs.containsKey(name)) {
                report.addChange(DriftChange.removed(itemType, name));
            }
        }

        // Find modified items
        for (Map.Entry<String, String> entry : oldSigs.entrySet()) {
            String name = entry.getKey();
            String oldSig = entry.getValue();
            String newSig = newSigs.get(name);
            if (newSig != null && !oldSig.equals(newSig)) {
                report.addChange(DriftChange.modified(itemType, name));
            }
        }
    }

    /**
     * Compute content hash for entire discovery result.
     */
    private String computeContentHash(MCPDiscoveryResult result) {
        try {
            String json = objectMapper.writeValueAsString(result);
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(json.getBytes());
            return Base64.getEncoder().encodeToString(hash);
        } catch (Exception e) {
            return "hash-error";
        }
    }

    /**
     * Compute signatures for each tool.
     */
    private Map<String, String> computeToolSignatures(MCPDiscoveryResult result) {
        Map<String, String> sigs = new LinkedHashMap<>();
        if (result.getTools() != null) {
            for (MCPTool tool : result.getTools()) {
                String sig = computeToolSignature(tool);
                sigs.put(tool.getName(), sig);
            }
        }
        return sigs;
    }

    private String computeToolSignature(MCPTool tool) {
        StringBuilder sb = new StringBuilder();
        sb.append(tool.getName()).append("|");
        sb.append(tool.getDescription() != null ? tool.getDescription() : "").append("|");
        if (tool.getInputSchema() != null) {
            sb.append(tool.getInputSchema().toString());
        }
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(sb.toString().getBytes());
            return Base64.getEncoder().encodeToString(hash).substring(0, 12);
        } catch (Exception e) {
            return "sig-error";
        }
    }

    /**
     * Compute signatures for each resource.
     */
    private Map<String, String> computeResourceSignatures(MCPDiscoveryResult result) {
        Map<String, String> sigs = new LinkedHashMap<>();
        if (result.getResources() != null) {
            for (MCPResource resource : result.getResources()) {
                String sig = computeResourceSignature(resource);
                sigs.put(resource.getName(), sig);
            }
        }
        return sigs;
    }

    private String computeResourceSignature(MCPResource resource) {
        StringBuilder sb = new StringBuilder();
        sb.append(resource.getName()).append("|");
        sb.append(resource.getUri() != null ? resource.getUri() : "").append("|");
        sb.append(resource.getDescription() != null ? resource.getDescription() : "").append("|");
        sb.append(resource.getMimeType() != null ? resource.getMimeType() : "");
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(sb.toString().getBytes());
            return Base64.getEncoder().encodeToString(hash).substring(0, 12);
        } catch (Exception e) {
            return "sig-error";
        }
    }

    /**
     * Compute signatures for each prompt.
     */
    private Map<String, String> computePromptSignatures(MCPDiscoveryResult result) {
        Map<String, String> sigs = new LinkedHashMap<>();
        if (result.getPrompts() != null) {
            for (MCPPrompt prompt : result.getPrompts()) {
                String sig = computePromptSignature(prompt);
                sigs.put(prompt.getName(), sig);
            }
        }
        return sigs;
    }

    private String computePromptSignature(MCPPrompt prompt) {
        StringBuilder sb = new StringBuilder();
        sb.append(prompt.getName()).append("|");
        sb.append(prompt.getDescription() != null ? prompt.getDescription() : "").append("|");
        if (prompt.getArguments() != null) {
            for (Map<String, Object> arg : prompt.getArguments()) {
                Object name = arg.get("name");
                Object required = arg.get("required");
                sb.append(name != null ? name : "").append(":");
                sb.append(required != null ? required : false).append(",");
            }
        }
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(sb.toString().getBytes());
            return Base64.getEncoder().encodeToString(hash).substring(0, 12);
        } catch (Exception e) {
            return "sig-error";
        }
    }

    /**
     * Persist attestation to disk.
     */
    private void persistToDisk(String serverName, CachedAttestation cached) {
        try {
            String safeName = serverName.replaceAll("[^a-zA-Z0-9_-]", "_");
            Path file = CACHE_DIR.resolve(safeName + CACHE_EXTENSION);
            objectMapper.writeValue(file.toFile(), cached);
        } catch (IOException e) {
            log.warn("Failed to persist attestation cache: {}", e.getMessage());
        }
    }

    /**
     * Record attestation in history for audit trail.
     */
    private void recordHistory(String serverName, CachedAttestation cached, DriftReport drift) {
        try {
            String safeName = serverName.replaceAll("[^a-zA-Z0-9_-]", "_");
            Path historyFile = CACHE_DIR.resolve(safeName + HISTORY_EXTENSION);

            List<HistoryEntry> history;
            if (Files.exists(historyFile)) {
                history = objectMapper.readValue(historyFile.toFile(),
                        new TypeReference<List<HistoryEntry>>() {});
            } else {
                history = new ArrayList<>();
            }

            // Add new entry
            HistoryEntry entry = new HistoryEntry();
            entry.timestamp = cached.cachedAt;
            entry.attestationId = cached.attestationId;
            entry.contentHash = cached.contentHash;
            entry.toolCount = cached.toolCount;
            entry.resourceCount = cached.resourceCount;
            entry.promptCount = cached.promptCount;
            if (drift != null && drift.hasDrift()) {
                entry.driftDetected = true;
                entry.changes = drift.getChanges().stream()
                        .map(DriftChange::toString)
                        .collect(Collectors.toList());
            }
            history.add(entry);

            // Trim history if too long
            while (history.size() > maxHistoryEntries) {
                history.remove(0);
            }

            objectMapper.writeValue(historyFile.toFile(), history);

        } catch (IOException e) {
            log.warn("Failed to record attestation history: {}", e.getMessage());
        }
    }

    /**
     * Log drift event to security logger.
     */
    private void logDriftEvent(String serverName, DriftReport drift) {
        SecurityLogger logger = SecurityLogger.getInstance();
        Map<String, Object> details = new LinkedHashMap<>();
        details.put("serverName", serverName);
        details.put("toolsAdded", drift.countByType("tool", DriftChange.ChangeType.ADDED));
        details.put("toolsRemoved", drift.countByType("tool", DriftChange.ChangeType.REMOVED));
        details.put("toolsModified", drift.countByType("tool", DriftChange.ChangeType.MODIFIED));
        details.put("resourcesAdded", drift.countByType("resource", DriftChange.ChangeType.ADDED));
        details.put("resourcesRemoved", drift.countByType("resource", DriftChange.ChangeType.REMOVED));
        details.put("changes", drift.getChanges().stream()
                .map(DriftChange::toString)
                .collect(Collectors.toList()));

        logger.logMcpEvent(
                EventTypes.Mcp.MCP_CAPABILITY_DRIFT,
                serverName,
                "Capability drift detected",
                details,
                true
        );
    }

    /**
     * Get attestation history for a server.
     *
     * @param serverName the MCP server name
     * @return list of history entries
     */
    public List<HistoryEntry> getHistory(String serverName) {
        try {
            String safeName = serverName.replaceAll("[^a-zA-Z0-9_-]", "_");
            Path historyFile = CACHE_DIR.resolve(safeName + HISTORY_EXTENSION);
            if (Files.exists(historyFile)) {
                return objectMapper.readValue(historyFile.toFile(),
                        new TypeReference<List<HistoryEntry>>() {});
            }
        } catch (IOException e) {
            log.warn("Failed to read history: {}", e.getMessage());
        }
        return Collections.emptyList();
    }

    /**
     * Invalidate cache for a server.
     *
     * @param serverName the MCP server name to invalidate
     */
    public void invalidate(String serverName) {
        memoryCache.remove(serverName);
        try {
            String safeName = serverName.replaceAll("[^a-zA-Z0-9_-]", "_");
            Files.deleteIfExists(CACHE_DIR.resolve(safeName + CACHE_EXTENSION));
        } catch (IOException e) {
            log.warn("Failed to delete cache file: {}", e.getMessage());
        }
    }

    /**
     * Clear all cached attestations.
     */
    public void clearAll() {
        memoryCache.clear();
        try (var stream = Files.list(CACHE_DIR)) {
            stream.filter(p -> p.toString().endsWith(CACHE_EXTENSION))
                    .forEach(p -> {
                        try {
                            Files.delete(p);
                        } catch (IOException ignored) {}
                    });
        } catch (IOException ignored) {}
    }

    /**
     * Get all cached server names.
     *
     * @return set of cached server names
     */
    public Set<String> getCachedServers() {
        return new HashSet<>(memoryCache.keySet());
    }

    /**
     * Get cache statistics.
     *
     * @return map of statistics
     */
    public Map<String, Object> getStatistics() {
        Map<String, Object> stats = new LinkedHashMap<>();
        stats.put("cachedServers", memoryCache.size());
        stats.put("servers", memoryCache.keySet());

        int totalTools = 0, totalResources = 0, totalPrompts = 0;
        for (CachedAttestation cached : memoryCache.values()) {
            totalTools += cached.toolCount;
            totalResources += cached.resourceCount;
            totalPrompts += cached.promptCount;
        }
        stats.put("totalTools", totalTools);
        stats.put("totalResources", totalResources);
        stats.put("totalPrompts", totalPrompts);

        return stats;
    }

    /**
     * Set whether to automatically log drift events.
     *
     * @param autoLogDrift true to enable automatic drift logging
     */
    public void setAutoLogDrift(boolean autoLogDrift) {
        this.autoLogDrift = autoLogDrift;
    }

    /**
     * Set maximum history entries to retain.
     *
     * @param maxHistoryEntries maximum number of history entries
     */
    public void setMaxHistoryEntries(int maxHistoryEntries) {
        this.maxHistoryEntries = maxHistoryEntries;
    }

    /**
     * Cached attestation data.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class CachedAttestation {
        /** The MCP server name. */
        public String serverName;
        /** The attestation ID from AIM server. */
        public String attestationId;
        /** Timestamp when cached. */
        public String cachedAt;
        /** Timestamp when cache expires. */
        public String expiresAt;
        /** Number of tools discovered. */
        public int toolCount;
        /** Number of resources discovered. */
        public int resourceCount;
        /** Number of prompts discovered. */
        public int promptCount;
        /** Content hash for quick comparison. */
        public String contentHash;
        /** Signatures for each tool. */
        public Map<String, String> toolSignatures;
        /** Signatures for each resource. */
        public Map<String, String> resourceSignatures;
        /** Signatures for each prompt. */
        public Map<String, String> promptSignatures;

        /** Creates a new empty cached attestation. */
        public CachedAttestation() {}

        /**
         * Check if this attestation has expired.
         *
         * @return true if expired
         */
        public boolean isExpired() {
            try {
                Instant expires = Instant.parse(expiresAt);
                return Instant.now().isAfter(expires);
            } catch (Exception e) {
                return true;
            }
        }
    }

    /**
     * History entry for audit trail.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class HistoryEntry {
        /** Timestamp of this history entry. */
        public String timestamp;
        /** The attestation ID. */
        public String attestationId;
        /** Content hash at this point. */
        public String contentHash;
        /** Number of tools. */
        public int toolCount;
        /** Number of resources. */
        public int resourceCount;
        /** Number of prompts. */
        public int promptCount;
        /** Whether drift was detected. */
        public boolean driftDetected;
        /** List of change descriptions. */
        public List<String> changes;

        /** Creates a new empty history entry. */
        public HistoryEntry() {}
    }

    /**
     * Report of detected capability drift.
     */
    public static class DriftReport {
        private final String serverName;
        private final List<DriftChange> changes = new ArrayList<>();
        private final boolean hasPreviousAttestation;

        /**
         * Creates a new drift report.
         *
         * @param serverName the MCP server name
         */
        public DriftReport(String serverName) {
            this.serverName = serverName;
            this.hasPreviousAttestation = true;
        }

        private DriftReport(String serverName, boolean hasPrevious) {
            this.serverName = serverName;
            this.hasPreviousAttestation = hasPrevious;
        }

        /**
         * Create a report indicating no previous attestation exists.
         *
         * @param serverName the MCP server name
         * @return a drift report with no previous attestation
         */
        public static DriftReport noPreviousAttestation(String serverName) {
            return new DriftReport(serverName, false);
        }

        /**
         * Add a change to this report.
         *
         * @param change the drift change to add
         */
        public void addChange(DriftChange change) {
            changes.add(change);
        }

        /**
         * Check if any drift was detected.
         *
         * @return true if changes were detected
         */
        public boolean hasDrift() {
            return !changes.isEmpty();
        }

        /**
         * Check if a previous attestation existed.
         *
         * @return true if previous attestation existed
         */
        public boolean hasPreviousAttestation() {
            return hasPreviousAttestation;
        }

        /**
         * Get the server name.
         *
         * @return the MCP server name
         */
        public String getServerName() {
            return serverName;
        }

        /**
         * Get all changes.
         *
         * @return unmodifiable list of changes
         */
        public List<DriftChange> getChanges() {
            return Collections.unmodifiableList(changes);
        }

        /**
         * Count changes by item type and change type.
         *
         * @param itemType the item type (tool, resource, prompt)
         * @param changeType the change type (ADDED, REMOVED, MODIFIED)
         * @return count of matching changes
         */
        public int countByType(String itemType, DriftChange.ChangeType changeType) {
            return (int) changes.stream()
                    .filter(c -> c.getItemType().equals(itemType) && c.getChangeType() == changeType)
                    .count();
        }

        /**
         * Convert to map representation.
         *
         * @return map representation
         */
        public Map<String, Object> toMap() {
            Map<String, Object> map = new LinkedHashMap<>();
            map.put("serverName", serverName);
            map.put("hasDrift", hasDrift());
            map.put("hasPreviousAttestation", hasPreviousAttestation);
            map.put("changeCount", changes.size());
            map.put("changes", changes.stream().map(DriftChange::toMap).collect(Collectors.toList()));
            return map;
        }
    }

    /**
     * Single drift change.
     */
    public static class DriftChange {
        /**
         * Type of change detected.
         */
        public enum ChangeType {
            /** Item was added. */
            ADDED,
            /** Item was removed. */
            REMOVED,
            /** Item was modified. */
            MODIFIED
        }

        private final ChangeType changeType;
        private final String itemType;
        private final String itemName;

        private DriftChange(ChangeType changeType, String itemType, String itemName) {
            this.changeType = changeType;
            this.itemType = itemType;
            this.itemName = itemName;
        }

        /**
         * Create an ADDED change.
         *
         * @param itemType the item type
         * @param itemName the item name
         * @return the drift change
         */
        public static DriftChange added(String itemType, String itemName) {
            return new DriftChange(ChangeType.ADDED, itemType, itemName);
        }

        /**
         * Create a REMOVED change.
         *
         * @param itemType the item type
         * @param itemName the item name
         * @return the drift change
         */
        public static DriftChange removed(String itemType, String itemName) {
            return new DriftChange(ChangeType.REMOVED, itemType, itemName);
        }

        /**
         * Create a MODIFIED change.
         *
         * @param itemType the item type
         * @param itemName the item name
         * @return the drift change
         */
        public static DriftChange modified(String itemType, String itemName) {
            return new DriftChange(ChangeType.MODIFIED, itemType, itemName);
        }

        /**
         * Get the change type.
         *
         * @return the change type
         */
        public ChangeType getChangeType() { return changeType; }

        /**
         * Get the item type.
         *
         * @return the item type
         */
        public String getItemType() { return itemType; }

        /**
         * Get the item name.
         *
         * @return the item name
         */
        public String getItemName() { return itemName; }

        /**
         * Convert to map representation.
         *
         * @return map representation
         */
        public Map<String, String> toMap() {
            Map<String, String> map = new LinkedHashMap<>();
            map.put("changeType", changeType.name());
            map.put("itemType", itemType);
            map.put("itemName", itemName);
            return map;
        }

        @Override
        public String toString() {
            return changeType.name() + " " + itemType + ": " + itemName;
        }
    }
}
