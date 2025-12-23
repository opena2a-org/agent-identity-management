package org.opena2a.aim.security;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Pattern;

/**
 * Automatic risk level detection from capability patterns.
 *
 * <p>Analyzes capability strings and MCP tool definitions to determine
 * appropriate risk levels for security classification and approval workflows.</p>
 *
 * <h2>Features:</h2>
 * <ul>
 *   <li>Pattern-based risk detection with regex support</li>
 *   <li>Keyword analysis for additional context</li>
 *   <li>Custom pattern registration for organization-specific needs</li>
 *   <li>Caching for performance optimization</li>
 *   <li>Aggregate risk calculation for capability sets</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * RiskDetector detector = RiskDetector.getInstance();
 *
 * // Single capability
 * RiskLevel level = detector.detectRisk("payment:process");
 * // Returns: HIGH
 *
 * // Multiple capabilities
 * RiskLevel aggregate = detector.detectAggregateRisk(List.of(
 *     "user:read", "payment:refund", "log:view"
 * ));
 * // Returns: HIGH (highest among all)
 *
 * // With context
 * RiskAssessment assessment = detector.assess("database:query",
 *     Map.of("table", "users", "columns", List.of("ssn", "dob")));
 * // Returns assessment with elevated risk due to PII columns
 * }</pre>
 *
 * <h2>Built-in Patterns:</h2>
 * <table>
 *   <tr><th>Risk Level</th><th>Patterns</th></tr>
 *   <tr><td>CRITICAL</td><td>admin:*, system:delete, database:drop, credential:*, root:*</td></tr>
 *   <tr><td>HIGH</td><td>payment:*, user:delete, data:export, pii:*, phi:*, pci:*</td></tr>
 *   <tr><td>MEDIUM</td><td>db:read, user:read, file:write, config:*, email:send</td></tr>
 *   <tr><td>LOW</td><td>api:call, cache:read, log:view, status:check</td></tr>
 * </table>
 */
public class RiskDetector {

    private static final Logger log = LoggerFactory.getLogger(RiskDetector.class);
    private static volatile RiskDetector instance;
    private static final Object lock = new Object();

    // Pattern caches for performance
    private final Map<String, RiskLevel> riskCache = new ConcurrentHashMap<>();
    private final Map<RiskLevel, List<Pattern>> patternsByLevel = new EnumMap<>(RiskLevel.class);

    // Keyword sets for context-aware detection
    private final Set<String> criticalKeywords = new HashSet<>();
    private final Set<String> highRiskKeywords = new HashSet<>();
    private final Set<String> mediumRiskKeywords = new HashSet<>();
    private final Set<String> sensitiveDataTypes = new HashSet<>();

    // Custom patterns added by users
    private final Map<RiskLevel, List<Pattern>> customPatterns = new EnumMap<>(RiskLevel.class);

    private RiskDetector() {
        initializePatterns();
        initializeKeywords();
    }

    /**
     * Get the singleton instance.
     */
    public static RiskDetector getInstance() {
        if (instance == null) {
            synchronized (lock) {
                if (instance == null) {
                    instance = new RiskDetector();
                }
            }
        }
        return instance;
    }

    /**
     * Initialize built-in risk patterns.
     */
    private void initializePatterns() {
        // CRITICAL patterns - system administration, data destruction
        patternsByLevel.put(RiskLevel.CRITICAL, Arrays.asList(
                Pattern.compile("^admin:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^root:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^system:(delete|destroy|wipe|drop).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^database:(drop|truncate|delete_all).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^credential:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^secret:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^key:(create|rotate|delete).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^iam:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^privilege:escalate.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^sudo:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^shell:execute.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^code:execute.*", Pattern.CASE_INSENSITIVE)
        ));

        // HIGH patterns - financial, user management, data export
        patternsByLevel.put(RiskLevel.HIGH, Arrays.asList(
                Pattern.compile("^payment:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^transaction:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^transfer:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^refund:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^user:(delete|suspend|ban|modify_role).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^account:(close|delete|suspend).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^data:(export|download|bulk_read).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^pii:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^phi:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^pci:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^hipaa:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^gdpr:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^encrypt:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^decrypt:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^backup:(restore|delete).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^audit:modify.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^compliance:override.*", Pattern.CASE_INSENSITIVE)
        ));

        // MEDIUM patterns - data access, configuration
        patternsByLevel.put(RiskLevel.MEDIUM, Arrays.asList(
                Pattern.compile("^db:(read|query|select).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^database:(read|query).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^user:(read|view|list).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^file:(write|create|update|delete).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^config:(update|modify|set).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^email:(send|forward).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^notification:send.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^webhook:(create|update).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^api:(create|update|delete).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^report:(generate|create).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^schedule:(create|modify).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^integration:(configure|update).*", Pattern.CASE_INSENSITIVE)
        ));

        // LOW patterns - read-only, status checks
        patternsByLevel.put(RiskLevel.LOW, Arrays.asList(
                Pattern.compile("^api:call.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^cache:(read|get).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^log:(view|read).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^status:(check|view).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^health:check.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^ping:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^version:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^info:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^metadata:(read|get).*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^search:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^list:.*", Pattern.CASE_INSENSITIVE),
                Pattern.compile("^count:.*", Pattern.CASE_INSENSITIVE)
        ));

        // Initialize custom patterns maps
        for (RiskLevel level : RiskLevel.values()) {
            customPatterns.put(level, new ArrayList<>());
        }
    }

    /**
     * Initialize keyword sets for context analysis.
     */
    private void initializeKeywords() {
        // Critical keywords
        criticalKeywords.addAll(Arrays.asList(
                "admin", "root", "sudo", "superuser", "privilege", "escalate",
                "drop", "truncate", "destroy", "wipe", "format",
                "credential", "secret", "key", "token", "password",
                "shell", "execute", "eval", "exec", "runtime"
        ));

        // High risk keywords
        highRiskKeywords.addAll(Arrays.asList(
                "payment", "transaction", "transfer", "refund", "charge", "billing",
                "delete", "remove", "suspend", "ban", "terminate",
                "export", "download", "bulk", "dump", "extract",
                "pii", "phi", "pci", "ssn", "social security",
                "credit card", "bank account", "routing number",
                "medical", "health", "diagnosis", "prescription",
                "encrypt", "decrypt", "certificate"
        ));

        // Medium risk keywords
        mediumRiskKeywords.addAll(Arrays.asList(
                "write", "update", "modify", "change", "alter",
                "create", "insert", "add", "configure", "set",
                "email", "send", "notify", "alert", "message",
                "file", "document", "attachment", "upload"
        ));

        // Sensitive data types
        sensitiveDataTypes.addAll(Arrays.asList(
                "ssn", "social_security", "tax_id", "ein",
                "credit_card", "card_number", "cvv", "expiry",
                "bank_account", "routing_number", "iban", "swift",
                "password", "secret", "api_key", "access_token",
                "dob", "date_of_birth", "birthdate",
                "address", "phone", "email",
                "medical_record", "diagnosis", "prescription",
                "biometric", "fingerprint", "facial_data"
        ));
    }

    /**
     * Detect risk level for a single capability.
     *
     * @param capability The capability string (e.g., "payment:process")
     * @return The detected risk level
     */
    public RiskLevel detectRisk(String capability) {
        if (capability == null || capability.isEmpty()) {
            return RiskLevel.LOW;
        }

        // Check cache first
        RiskLevel cached = riskCache.get(capability);
        if (cached != null) {
            return cached;
        }

        // Check patterns from highest to lowest risk
        RiskLevel detected = detectFromPatterns(capability);

        // Cache the result
        riskCache.put(capability, detected);

        log.debug("Risk detected for '{}': {}", capability, detected);
        return detected;
    }

    /**
     * Detect risk from pattern matching.
     */
    private RiskLevel detectFromPatterns(String capability) {
        String normalized = capability.toLowerCase().trim();

        // Check custom patterns first (allows overrides)
        for (RiskLevel level : Arrays.asList(RiskLevel.CRITICAL, RiskLevel.HIGH, RiskLevel.MEDIUM, RiskLevel.LOW)) {
            for (Pattern pattern : customPatterns.get(level)) {
                if (pattern.matcher(normalized).matches()) {
                    return level;
                }
            }
        }

        // Check built-in patterns
        for (RiskLevel level : Arrays.asList(RiskLevel.CRITICAL, RiskLevel.HIGH, RiskLevel.MEDIUM, RiskLevel.LOW)) {
            List<Pattern> patterns = patternsByLevel.get(level);
            if (patterns != null) {
                for (Pattern pattern : patterns) {
                    if (pattern.matcher(normalized).matches()) {
                        return level;
                    }
                }
            }
        }

        // Fallback to keyword analysis
        return detectFromKeywords(normalized);
    }

    /**
     * Detect risk from keyword presence.
     */
    private RiskLevel detectFromKeywords(String capability) {
        String[] parts = capability.toLowerCase().split("[:\\-_\\s]+");

        for (String part : parts) {
            if (criticalKeywords.contains(part)) {
                return RiskLevel.CRITICAL;
            }
        }

        for (String part : parts) {
            if (highRiskKeywords.contains(part)) {
                return RiskLevel.HIGH;
            }
        }

        for (String part : parts) {
            if (mediumRiskKeywords.contains(part)) {
                return RiskLevel.MEDIUM;
            }
        }

        return RiskLevel.LOW;
    }

    /**
     * Detect aggregate risk level for multiple capabilities.
     * Returns the highest risk level among all capabilities.
     *
     * @param capabilities List of capability strings
     * @return The highest detected risk level
     */
    public RiskLevel detectAggregateRisk(List<String> capabilities) {
        if (capabilities == null || capabilities.isEmpty()) {
            return RiskLevel.LOW;
        }

        RiskLevel highest = RiskLevel.LOW;
        for (String capability : capabilities) {
            RiskLevel level = detectRisk(capability);
            if (level.isHigherThan(highest)) {
                highest = level;
            }
            if (highest == RiskLevel.CRITICAL) {
                break; // Can't go higher
            }
        }

        return highest;
    }

    /**
     * Perform a detailed risk assessment with context.
     *
     * @param capability The capability string
     * @param context Additional context (e.g., table names, columns, parameters)
     * @return Detailed risk assessment
     */
    public RiskAssessment assess(String capability, Map<String, Object> context) {
        RiskLevel baseLevel = detectRisk(capability);
        List<String> factors = new ArrayList<>();
        RiskLevel adjustedLevel = baseLevel;

        // Analyze context for risk factors
        if (context != null) {
            // Check for sensitive data in context
            adjustedLevel = analyzeContext(context, baseLevel, factors);
        }

        return new RiskAssessment(capability, baseLevel, adjustedLevel, factors);
    }

    /**
     * Analyze context for additional risk factors.
     */
    private RiskLevel analyzeContext(Map<String, Object> context, RiskLevel baseLevel, List<String> factors) {
        RiskLevel adjusted = baseLevel;

        // Check for sensitive table names
        Object table = context.get("table");
        if (table != null) {
            String tableName = table.toString().toLowerCase();
            if (tableName.contains("user") || tableName.contains("customer") ||
                    tableName.contains("account") || tableName.contains("payment")) {
                if (adjusted.getLevel() < RiskLevel.MEDIUM.getLevel()) {
                    adjusted = RiskLevel.MEDIUM;
                    factors.add("Accessing sensitive table: " + tableName);
                }
            }
        }

        // Check for sensitive columns
        Object columns = context.get("columns");
        if (columns instanceof List) {
            @SuppressWarnings("unchecked")
            List<String> columnList = (List<String>) columns;
            for (String column : columnList) {
                String col = column.toLowerCase();
                if (sensitiveDataTypes.stream().anyMatch(col::contains)) {
                    if (adjusted.getLevel() < RiskLevel.HIGH.getLevel()) {
                        adjusted = RiskLevel.HIGH;
                        factors.add("Accessing sensitive column: " + column);
                    }
                }
            }
        }

        // Check for bulk operations
        Object limit = context.get("limit");
        if (limit != null) {
            try {
                int limitVal = Integer.parseInt(limit.toString());
                if (limitVal > 1000) {
                    if (adjusted.getLevel() < RiskLevel.MEDIUM.getLevel()) {
                        adjusted = RiskLevel.MEDIUM;
                        factors.add("Bulk operation with limit: " + limitVal);
                    }
                }
                if (limitVal > 10000) {
                    if (adjusted.getLevel() < RiskLevel.HIGH.getLevel()) {
                        adjusted = RiskLevel.HIGH;
                        factors.add("Large bulk operation with limit: " + limitVal);
                    }
                }
            } catch (NumberFormatException ignored) {
            }
        }

        // Check for external destinations
        Object destination = context.get("destination");
        if (destination != null) {
            String dest = destination.toString().toLowerCase();
            if (dest.contains("external") || dest.contains("public") ||
                    dest.contains("s3://") || dest.contains("https://")) {
                if (adjusted.getLevel() < RiskLevel.HIGH.getLevel()) {
                    adjusted = RiskLevel.HIGH;
                    factors.add("External destination: " + destination);
                }
            }
        }

        return adjusted;
    }

    /**
     * Register a custom risk pattern.
     *
     * @param level The risk level for matches
     * @param pattern Regex pattern to match capabilities
     */
    public void registerPattern(RiskLevel level, String pattern) {
        customPatterns.get(level).add(Pattern.compile(pattern, Pattern.CASE_INSENSITIVE));
        riskCache.clear(); // Clear cache to apply new pattern
        log.info("Registered custom {} pattern: {}", level, pattern);
    }

    /**
     * Register custom patterns for an organization.
     *
     * @param patterns Map of risk level to pattern strings
     */
    public void registerPatterns(Map<RiskLevel, List<String>> patterns) {
        for (Map.Entry<RiskLevel, List<String>> entry : patterns.entrySet()) {
            for (String pattern : entry.getValue()) {
                registerPattern(entry.getKey(), pattern);
            }
        }
    }

    /**
     * Add custom keywords to a risk category.
     *
     * @param level Risk level to add keywords to
     * @param keywords Keywords to add
     */
    public void addKeywords(RiskLevel level, String... keywords) {
        Set<String> targetSet;
        switch (level) {
            case CRITICAL:
                targetSet = criticalKeywords;
                break;
            case HIGH:
                targetSet = highRiskKeywords;
                break;
            case MEDIUM:
                targetSet = mediumRiskKeywords;
                break;
            default:
                return; // LOW doesn't use keywords for elevation
        }
        targetSet.addAll(Arrays.asList(keywords));
        riskCache.clear();
    }

    /**
     * Add sensitive data type patterns.
     *
     * @param dataTypes Data type names to consider sensitive
     */
    public void addSensitiveDataTypes(String... dataTypes) {
        sensitiveDataTypes.addAll(Arrays.asList(dataTypes));
    }

    /**
     * Clear the risk detection cache.
     */
    public void clearCache() {
        riskCache.clear();
    }

    /**
     * Get statistics about detected risks.
     */
    public Map<String, Object> getStatistics() {
        Map<RiskLevel, Integer> distribution = new EnumMap<>(RiskLevel.class);
        for (RiskLevel level : RiskLevel.values()) {
            distribution.put(level, 0);
        }

        for (RiskLevel level : riskCache.values()) {
            distribution.merge(level, 1, Integer::sum);
        }

        Map<String, Object> stats = new LinkedHashMap<>();
        stats.put("cacheSize", riskCache.size());
        stats.put("distribution", distribution);
        stats.put("customPatternCount", customPatterns.values().stream()
                .mapToInt(List::size).sum());
        return stats;
    }

    /**
     * Result of a detailed risk assessment.
     */
    public static class RiskAssessment {
        private final String capability;
        private final RiskLevel baseLevel;
        private final RiskLevel adjustedLevel;
        private final List<String> factors;

        public RiskAssessment(String capability, RiskLevel baseLevel,
                              RiskLevel adjustedLevel, List<String> factors) {
            this.capability = capability;
            this.baseLevel = baseLevel;
            this.adjustedLevel = adjustedLevel;
            this.factors = factors != null ? new ArrayList<>(factors) : new ArrayList<>();
        }

        public String getCapability() {
            return capability;
        }

        public RiskLevel getBaseLevel() {
            return baseLevel;
        }

        public RiskLevel getAdjustedLevel() {
            return adjustedLevel;
        }

        public List<String> getFactors() {
            return Collections.unmodifiableList(factors);
        }

        public boolean wasElevated() {
            return adjustedLevel.isHigherThan(baseLevel);
        }

        public Map<String, Object> toMap() {
            Map<String, Object> map = new LinkedHashMap<>();
            map.put("capability", capability);
            map.put("baseLevel", baseLevel.getName());
            map.put("adjustedLevel", adjustedLevel.getName());
            map.put("elevated", wasElevated());
            if (!factors.isEmpty()) {
                map.put("factors", factors);
            }
            return map;
        }
    }
}
