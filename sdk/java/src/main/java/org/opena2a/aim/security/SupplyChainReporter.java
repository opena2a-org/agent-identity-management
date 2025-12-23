package org.opena2a.aim.security;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import org.opena2a.aim.integrations.mcp.discovery.MCPDiscoveryResult;
import org.opena2a.aim.integrations.mcp.discovery.MCPTool;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.file.*;
import java.time.Instant;
import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.*;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

/**
 * Supply chain security reporter for MCP servers.
 *
 * <p>Tracks MCP server versions, dependencies, and changes over time
 * to enable supply chain security monitoring and SBOM generation.</p>
 *
 * <h2>Features:</h2>
 * <ul>
 *   <li>NPM package version extraction for npx-based MCPs</li>
 *   <li>Version change tracking and alerting</li>
 *   <li>SBOM (Software Bill of Materials) generation</li>
 *   <li>Dependency vulnerability context</li>
 *   <li>Historical version tracking</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * SupplyChainReporter reporter = SupplyChainReporter.getInstance();
 *
 * // Record MCP server deployment
 * reporter.recordMcpServer("github", "npx -y @modelcontextprotocol/server-github",
 *     discoveryResult);
 *
 * // Generate SBOM
 * SBOM sbom = reporter.generateSBOM("my-agent");
 *
 * // Check for version changes
 * List<VersionChange> changes = reporter.getRecentVersionChanges(24); // hours
 * }</pre>
 *
 * <h2>Version Extraction:</h2>
 * <p>Automatically extracts versions from:</p>
 * <ul>
 *   <li>NPM packages: @scope/package@version or package@version</li>
 *   <li>Docker images: image:tag</li>
 *   <li>Python packages: package==version</li>
 *   <li>Custom version patterns</li>
 * </ul>
 */
public class SupplyChainReporter {

    private static final Logger log = LoggerFactory.getLogger(SupplyChainReporter.class);
    private static final ObjectMapper objectMapper = new ObjectMapper()
            .enable(SerializationFeature.INDENT_OUTPUT);
    private static volatile SupplyChainReporter instance;
    private static final Object lock = new Object();

    // Storage paths
    private static final Path STORAGE_DIR = Path.of(
            System.getProperty("user.home"), ".aim", "supply_chain");
    private static final Path SERVERS_FILE = STORAGE_DIR.resolve("mcp_servers.json");
    private static final Path HISTORY_FILE = STORAGE_DIR.resolve("version_history.json");
    private static final Path SBOM_DIR = STORAGE_DIR.resolve("sbom");

    // Version extraction patterns
    private static final Pattern NPM_PACKAGE_PATTERN = Pattern.compile(
            "@?([\\w-]+/)?[\\w-]+@([\\d.]+(?:-[\\w.]+)?)");
    private static final Pattern NPM_LATEST_PATTERN = Pattern.compile(
            "npx\\s+-y\\s+(@[\\w-]+/[\\w-]+|[\\w-]+)");
    private static final Pattern DOCKER_IMAGE_PATTERN = Pattern.compile(
            "([\\w.-]+(?:/[\\w.-]+)*):([\\w.-]+)");
    private static final Pattern PYTHON_PACKAGE_PATTERN = Pattern.compile(
            "([\\w-]+)==([\\d.]+)");

    // In-memory state
    private final Map<String, McpServerRecord> serverRecords = new ConcurrentHashMap<>();
    private final List<VersionChange> versionHistory = new CopyOnWriteArrayList<>();
    private final ExecutorService versionResolver = Executors.newFixedThreadPool(2);

    private SupplyChainReporter() {
        try {
            Files.createDirectories(STORAGE_DIR);
            Files.createDirectories(SBOM_DIR);
            loadState();
        } catch (IOException e) {
            log.warn("Failed to initialize supply chain storage: {}", e.getMessage());
        }
    }

    /**
     * Get the singleton instance.
     */
    public static SupplyChainReporter getInstance() {
        if (instance == null) {
            synchronized (lock) {
                if (instance == null) {
                    instance = new SupplyChainReporter();
                }
            }
        }
        return instance;
    }

    /**
     * Load state from disk.
     */
    private void loadState() {
        try {
            if (Files.exists(SERVERS_FILE)) {
                Map<String, McpServerRecord> loaded = objectMapper.readValue(
                        SERVERS_FILE.toFile(),
                        new TypeReference<Map<String, McpServerRecord>>() {});
                serverRecords.putAll(loaded);
            }
            if (Files.exists(HISTORY_FILE)) {
                List<VersionChange> loaded = objectMapper.readValue(
                        HISTORY_FILE.toFile(),
                        new TypeReference<List<VersionChange>>() {});
                versionHistory.addAll(loaded);
            }
        } catch (IOException e) {
            log.warn("Failed to load supply chain state: {}", e.getMessage());
        }
    }

    /**
     * Save state to disk.
     */
    private void saveState() {
        try {
            objectMapper.writeValue(SERVERS_FILE.toFile(), serverRecords);
            objectMapper.writeValue(HISTORY_FILE.toFile(), versionHistory);
        } catch (IOException e) {
            log.warn("Failed to save supply chain state: {}", e.getMessage());
        }
    }

    /**
     * Record an MCP server for supply chain tracking.
     *
     * @param serverName MCP server name
     * @param command Command used to start the server
     * @param discoveryResult Discovery result with capabilities
     * @return Version change if version changed from previous record
     */
    public Optional<VersionChange> recordMcpServer(String serverName, String command,
                                                    MCPDiscoveryResult discoveryResult) {
        McpServerRecord existing = serverRecords.get(serverName);
        String previousVersion = existing != null ? existing.version : null;

        // Extract version from command
        VersionInfo versionInfo = extractVersion(command);

        // Resolve actual version if using @latest
        if (versionInfo.isLatest) {
            versionInfo = resolveLatestVersion(versionInfo.packageName, command);
        }

        // Create or update record
        McpServerRecord record = new McpServerRecord();
        record.serverName = serverName;
        record.command = command;
        record.packageName = versionInfo.packageName;
        record.version = versionInfo.version;
        record.versionSource = versionInfo.source;
        record.firstSeen = existing != null ? existing.firstSeen : Instant.now().toString();
        record.lastSeen = Instant.now().toString();
        record.toolCount = discoveryResult.getTools() != null ? discoveryResult.getTools().size() : 0;
        record.resourceCount = discoveryResult.getResources() != null ? discoveryResult.getResources().size() : 0;
        record.promptCount = discoveryResult.getPrompts() != null ? discoveryResult.getPrompts().size() : 0;
        record.tools = discoveryResult.getTools() != null ?
                discoveryResult.getTools().stream().map(MCPTool::getName).collect(Collectors.toList()) :
                Collections.emptyList();

        serverRecords.put(serverName, record);

        // Check for version change
        Optional<VersionChange> change = Optional.empty();
        if (previousVersion != null && !previousVersion.equals(record.version)) {
            VersionChange vc = new VersionChange();
            vc.serverName = serverName;
            vc.packageName = record.packageName;
            vc.previousVersion = previousVersion;
            vc.newVersion = record.version;
            vc.detectedAt = Instant.now().toString();
            vc.toolCountBefore = existing.toolCount;
            vc.toolCountAfter = record.toolCount;

            versionHistory.add(vc);
            change = Optional.of(vc);

            // Log security event
            SecurityLogger.getInstance().logMcpEvent(
                    EventTypes.Mcp.MCP_CAPABILITY_DRIFT,
                    serverName,
                    String.format("MCP version changed: %s → %s", previousVersion, record.version),
                    Map.of(
                            "previousVersion", previousVersion,
                            "newVersion", record.version,
                            "packageName", record.packageName
                    ),
                    true
            );
        }

        saveState();
        return change;
    }

    /**
     * Extract version info from command.
     */
    private VersionInfo extractVersion(String command) {
        VersionInfo info = new VersionInfo();

        // Check for explicit NPM version (@package@version)
        Matcher npmMatcher = NPM_PACKAGE_PATTERN.matcher(command);
        if (npmMatcher.find()) {
            info.packageName = npmMatcher.group().split("@")[0] +
                    (npmMatcher.group().startsWith("@") ? "@" + npmMatcher.group(1) : "");
            info.version = npmMatcher.group(2);
            info.source = "npm_explicit";
            return info;
        }

        // Check for npx -y @latest pattern
        Matcher latestMatcher = NPM_LATEST_PATTERN.matcher(command);
        if (latestMatcher.find()) {
            info.packageName = latestMatcher.group(1);
            info.version = "latest";
            info.isLatest = true;
            info.source = "npm_latest";
            return info;
        }

        // Check for Docker image
        Matcher dockerMatcher = DOCKER_IMAGE_PATTERN.matcher(command);
        if (dockerMatcher.find()) {
            info.packageName = dockerMatcher.group(1);
            info.version = dockerMatcher.group(2);
            info.source = "docker";
            return info;
        }

        // Check for Python package
        Matcher pythonMatcher = PYTHON_PACKAGE_PATTERN.matcher(command);
        if (pythonMatcher.find()) {
            info.packageName = pythonMatcher.group(1);
            info.version = pythonMatcher.group(2);
            info.source = "pip";
            return info;
        }

        // Unable to extract version
        info.packageName = "unknown";
        info.version = "unknown";
        info.source = "undetected";
        return info;
    }

    /**
     * Resolve the actual version of an @latest npm package.
     */
    private VersionInfo resolveLatestVersion(String packageName, String command) {
        VersionInfo info = new VersionInfo();
        info.packageName = packageName;
        info.source = "npm_resolved";

        try {
            ProcessBuilder pb = new ProcessBuilder("npm", "view", packageName, "version");
            pb.redirectErrorStream(true);
            Process process = pb.start();

            try (BufferedReader reader = new BufferedReader(
                    new InputStreamReader(process.getInputStream()))) {
                String version = reader.readLine();
                if (version != null && !version.isEmpty()) {
                    info.version = version.trim();
                    return info;
                }
            }

            process.waitFor(10, TimeUnit.SECONDS);
        } catch (Exception e) {
            log.debug("Failed to resolve npm version for {}: {}", packageName, e.getMessage());
        }

        // Fallback - couldn't resolve
        info.version = "latest-unresolved";
        info.source = "npm_unresolved";
        return info;
    }

    /**
     * Generate SBOM (Software Bill of Materials) for an agent.
     *
     * @param agentName Agent name for the SBOM
     * @return Generated SBOM
     */
    public SBOM generateSBOM(String agentName) {
        SBOM sbom = new SBOM();
        sbom.agentName = agentName;
        sbom.generatedAt = Instant.now().toString();
        sbom.sdkVersion = "aim-sdk-java@1.0.0";
        sbom.format = "CycloneDX-lite";
        sbom.components = new ArrayList<>();

        // Add SDK as component
        SBOMComponent sdkComponent = new SBOMComponent();
        sdkComponent.type = "library";
        sdkComponent.name = "aim-sdk-java";
        sdkComponent.version = "1.0.0";
        sdkComponent.purl = "pkg:maven/org.opena2a/aim-sdk@1.0.0";
        sbom.components.add(sdkComponent);

        // Add all MCP servers as components
        for (McpServerRecord record : serverRecords.values()) {
            SBOMComponent component = new SBOMComponent();
            component.type = "mcp-server";
            component.name = record.serverName;
            component.packageName = record.packageName;
            component.version = record.version;
            component.versionSource = record.versionSource;
            component.firstSeen = record.firstSeen;
            component.lastSeen = record.lastSeen;
            component.capabilities = Map.of(
                    "tools", record.toolCount,
                    "resources", record.resourceCount,
                    "prompts", record.promptCount
            );

            // Generate PURL based on source
            if ("npm_explicit".equals(record.versionSource) || "npm_resolved".equals(record.versionSource)) {
                component.purl = "pkg:npm/" + record.packageName + "@" + record.version;
            } else if ("docker".equals(record.versionSource)) {
                component.purl = "pkg:docker/" + record.packageName + "@" + record.version;
            } else if ("pip".equals(record.versionSource)) {
                component.purl = "pkg:pypi/" + record.packageName + "@" + record.version;
            }

            sbom.components.add(component);
        }

        sbom.componentCount = sbom.components.size();

        // Save SBOM to file
        saveSBOM(agentName, sbom);

        return sbom;
    }

    /**
     * Save SBOM to file.
     */
    private void saveSBOM(String agentName, SBOM sbom) {
        try {
            String date = LocalDate.now().format(DateTimeFormatter.ISO_LOCAL_DATE);
            String filename = agentName + "_" + date + "_sbom.json";
            Path file = SBOM_DIR.resolve(filename);
            objectMapper.writeValue(file.toFile(), sbom);
            log.info("SBOM saved to: {}", file);
        } catch (IOException e) {
            log.warn("Failed to save SBOM: {}", e.getMessage());
        }
    }

    /**
     * Get recent version changes.
     *
     * @param hoursBack Number of hours to look back
     * @return List of version changes within the time window
     */
    public List<VersionChange> getRecentVersionChanges(int hoursBack) {
        Instant cutoff = Instant.now().minusSeconds(hoursBack * 3600L);
        return versionHistory.stream()
                .filter(vc -> {
                    try {
                        return Instant.parse(vc.detectedAt).isAfter(cutoff);
                    } catch (Exception e) {
                        return false;
                    }
                })
                .collect(Collectors.toList());
    }

    /**
     * Get all recorded MCP servers.
     */
    public Map<String, McpServerRecord> getAllServers() {
        return Collections.unmodifiableMap(serverRecords);
    }

    /**
     * Get a specific server record.
     */
    public Optional<McpServerRecord> getServer(String serverName) {
        return Optional.ofNullable(serverRecords.get(serverName));
    }

    /**
     * Get supply chain summary statistics.
     */
    public Map<String, Object> getSummary() {
        Map<String, Object> summary = new LinkedHashMap<>();
        summary.put("serverCount", serverRecords.size());
        summary.put("totalVersionChanges", versionHistory.size());
        summary.put("recentChanges24h", getRecentVersionChanges(24).size());
        summary.put("recentChanges7d", getRecentVersionChanges(24 * 7).size());

        // Version sources breakdown
        Map<String, Long> sourceBreakdown = serverRecords.values().stream()
                .collect(Collectors.groupingBy(r -> r.versionSource, Collectors.counting()));
        summary.put("versionSources", sourceBreakdown);

        // Capability totals
        int totalTools = serverRecords.values().stream().mapToInt(r -> r.toolCount).sum();
        int totalResources = serverRecords.values().stream().mapToInt(r -> r.resourceCount).sum();
        int totalPrompts = serverRecords.values().stream().mapToInt(r -> r.promptCount).sum();
        summary.put("totalTools", totalTools);
        summary.put("totalResources", totalResources);
        summary.put("totalPrompts", totalPrompts);

        return summary;
    }

    /**
     * Shutdown the reporter.
     */
    public void shutdown() {
        versionResolver.shutdown();
        saveState();
    }

    // Data classes

    private static class VersionInfo {
        String packageName;
        String version;
        String source;
        boolean isLatest;
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class McpServerRecord {
        public String serverName;
        public String command;
        public String packageName;
        public String version;
        public String versionSource;
        public String firstSeen;
        public String lastSeen;
        public int toolCount;
        public int resourceCount;
        public int promptCount;
        public List<String> tools;
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class VersionChange {
        public String serverName;
        public String packageName;
        public String previousVersion;
        public String newVersion;
        public String detectedAt;
        public int toolCountBefore;
        public int toolCountAfter;

        public Map<String, Object> toMap() {
            Map<String, Object> map = new LinkedHashMap<>();
            map.put("serverName", serverName);
            map.put("packageName", packageName);
            map.put("previousVersion", previousVersion);
            map.put("newVersion", newVersion);
            map.put("detectedAt", detectedAt);
            map.put("toolCountChange", toolCountAfter - toolCountBefore);
            return map;
        }
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class SBOM {
        public String agentName;
        public String generatedAt;
        public String sdkVersion;
        public String format;
        public int componentCount;
        public List<SBOMComponent> components;
    }

    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class SBOMComponent {
        public String type;
        public String name;
        public String packageName;
        public String version;
        public String versionSource;
        public String purl;
        public String firstSeen;
        public String lastSeen;
        public Map<String, Integer> capabilities;
    }
}
