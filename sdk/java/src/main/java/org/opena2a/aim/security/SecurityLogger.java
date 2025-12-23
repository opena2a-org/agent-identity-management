package org.opena2a.aim.security;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.*;
import java.nio.file.*;
import java.time.Instant;
import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.*;

/**
 * Security-focused logger for AIM SDK - SOC/SIEM compatible.
 *
 * <p>Provides structured JSON logging for all security-relevant events.
 * Designed for integration with enterprise security monitoring systems.</p>
 *
 * <h2>Features:</h2>
 * <ul>
 *   <li>JSON Lines (JSONL) format for SIEM parsing</li>
 *   <li>Rotating file output with size limits</li>
 *   <li>Environment-based configuration</li>
 *   <li>Thread-safe with async logging option</li>
 *   <li>Persistent agent context</li>
 * </ul>
 *
 * <h2>Usage:</h2>
 * <pre>{@code
 * // Get the global instance
 * SecurityLogger logger = SecurityLogger.getInstance();
 *
 * // Configure for SOC integration
 * logger.configure(SecurityLogger.Config.builder()
 *     .logFile("/var/log/aim/security.log")
 *     .logLevel(EventSeverity.INFO)
 *     .build());
 *
 * // Set agent context (persists across all events)
 * logger.setAgentContext("agent-123", "my-agent", null, "org-456");
 *
 * // Log events
 * logger.logAuthentication(EventTypes.Authn.TOKEN_REFRESH, true, "Token refreshed", null);
 * logger.logAuthorization(EventTypes.Authz.CAPABILITY_GRANTED, "db:read", "users_table", true);
 * }</pre>
 *
 * <h2>Environment Variables:</h2>
 * <ul>
 *   <li>AIM_SECURITY_LOG_FILE - Path to security log file</li>
 *   <li>AIM_SECURITY_LOG_LEVEL - Log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)</li>
 *   <li>AIM_SECURITY_LOG_STDOUT - Include stdout logging (true/false)</li>
 *   <li>AIM_SECURITY_LOGGING_ENABLED - Enable/disable security logging (true/false)</li>
 * </ul>
 */
public class SecurityLogger {

    private static final Logger log = LoggerFactory.getLogger(SecurityLogger.class);
    private static volatile SecurityLogger instance;
    private static final Object lock = new Object();

    private volatile boolean enabled = true;
    private volatile EventSeverity minLevel = EventSeverity.INFO;
    private volatile PrintWriter fileWriter;
    private volatile boolean logToStdout = false;
    private volatile boolean logToStderr = true;
    private volatile String sessionId;

    // Persistent agent context
    private volatile String contextAgentId;
    private volatile String contextAgentName;
    private volatile String contextUserId;
    private volatile String contextOrganizationId;

    // Async logging
    private final ExecutorService asyncExecutor;
    private volatile boolean asyncEnabled = false;

    // File rotation
    private volatile Path logFilePath;
    private volatile long maxFileSize = 10 * 1024 * 1024; // 10MB
    private volatile int maxBackupCount = 5;

    private SecurityLogger() {
        this.asyncExecutor = Executors.newSingleThreadExecutor(r -> {
            Thread t = new Thread(r, "aim-security-logger");
            t.setDaemon(true);
            return t;
        });
        this.sessionId = UUID.randomUUID().toString();
        configureFromEnvironment();
    }

    /**
     * Get the singleton instance.
     */
    public static SecurityLogger getInstance() {
        if (instance == null) {
            synchronized (lock) {
                if (instance == null) {
                    instance = new SecurityLogger();
                }
            }
        }
        return instance;
    }

    /**
     * Configure security logging.
     */
    public void configure(Config config) {
        this.enabled = config.enabled;
        this.minLevel = config.logLevel;
        this.logToStdout = config.logToStdout;
        this.logToStderr = config.logToStderr;
        this.maxFileSize = config.maxFileSize;
        this.maxBackupCount = config.maxBackupCount;
        this.asyncEnabled = config.asyncEnabled;

        if (config.logFile != null && !config.logFile.isEmpty()) {
            try {
                this.logFilePath = Paths.get(config.logFile);
                Files.createDirectories(logFilePath.getParent());
                this.fileWriter = new PrintWriter(new BufferedWriter(
                        new FileWriter(logFilePath.toFile(), true)), true);
            } catch (IOException e) {
                log.warn("Could not open security log file: {}", e.getMessage());
            }
        }
    }

    /**
     * Configure from environment variables.
     */
    private void configureFromEnvironment() {
        String logFile = System.getenv("AIM_SECURITY_LOG_FILE");
        String logLevel = System.getenv("AIM_SECURITY_LOG_LEVEL");
        String logStdout = System.getenv("AIM_SECURITY_LOG_STDOUT");
        String loggingEnabled = System.getenv("AIM_SECURITY_LOGGING_ENABLED");

        Config.Builder builder = Config.builder();
        if (logFile != null) builder.logFile(logFile);
        if (logLevel != null) {
            try {
                builder.logLevel(EventSeverity.valueOf(logLevel.toUpperCase()));
            } catch (IllegalArgumentException ignored) {}
        }
        if ("true".equalsIgnoreCase(logStdout)) builder.logToStdout(true);
        if ("false".equalsIgnoreCase(loggingEnabled)) builder.enabled(false);

        configure(builder.build());
    }

    /**
     * Set persistent agent context for all subsequent events.
     */
    public void setAgentContext(String agentId, String agentName, String userId, String organizationId) {
        this.contextAgentId = agentId;
        this.contextAgentName = agentName;
        this.contextUserId = userId;
        this.contextOrganizationId = organizationId;
    }

    /**
     * Clear agent context.
     */
    public void clearAgentContext() {
        this.contextAgentId = null;
        this.contextAgentName = null;
        this.contextUserId = null;
        this.contextOrganizationId = null;
    }

    /**
     * Start a new logging session.
     */
    public String startSession() {
        this.sessionId = UUID.randomUUID().toString();
        return this.sessionId;
    }

    // =========================================================================
    // AUTHENTICATION EVENTS
    // =========================================================================

    public void logAuthentication(EventTypes.Authn eventType, boolean success, String message, Map<String, Object> details) {
        logAuthentication(eventType, success, message, null, details, null);
    }

    public void logAuthentication(EventTypes.Authn eventType, boolean success, String message,
                                   String agentId, Map<String, Object> details, String error) {
        EventSeverity severity = success ? EventSeverity.INFO : EventSeverity.WARNING;
        if (eventType == EventTypes.Authn.TOKEN_REVOKED || eventType == EventTypes.Authn.TOKEN_EXPIRED) {
            severity = success ? EventSeverity.WARNING : EventSeverity.ERROR;
        }

        String msg = message != null ? message : "Authentication event: " + eventType.name();
        SecurityEvent event = createEvent(EventCategory.AUTHN, eventType, severity, msg)
                .agentId(agentId != null ? agentId : contextAgentId)
                .result(success ? "SUCCESS" : "FAILURE")
                .details(details)
                .error(error)
                .build();

        logEvent(event);
    }

    // =========================================================================
    // AUTHORIZATION EVENTS
    // =========================================================================

    public void logAuthorization(EventTypes.Authz eventType, String action, String resource, boolean granted) {
        logAuthorization(eventType, action, resource, granted, null, null, null);
    }

    /**
     * Log authorization event with details.
     */
    public void logAuthorizationEvent(EventTypes.Authz eventType, String action, String resource,
                                       boolean granted, Map<String, Object> details) {
        logAuthorization(eventType, action, resource, granted, null, details, null);
    }

    public void logAuthorization(EventTypes.Authz eventType, String action, String resource,
                                  boolean granted, String agentId, Map<String, Object> details, String error) {
        EventSeverity severity = granted ? EventSeverity.INFO : EventSeverity.WARNING;
        if (eventType == EventTypes.Authz.CAPABILITY_ESCALATION) {
            severity = EventSeverity.WARNING;
        }

        String msg = String.format("Authorization %s: %s%s - %s",
                eventType.name(), action, resource != null ? " on " + resource : "",
                granted ? "GRANTED" : "DENIED");

        SecurityEvent event = createEvent(EventCategory.AUTHZ, eventType, severity, msg)
                .agentId(agentId != null ? agentId : contextAgentId)
                .action(action)
                .resource(resource)
                .result(granted ? "GRANTED" : "DENIED")
                .details(details)
                .error(error)
                .build();

        logEvent(event);
    }

    // =========================================================================
    // AGENT LIFECYCLE EVENTS
    // =========================================================================

    public void logAgentEvent(EventTypes.Agent eventType, String agentId, String agentName, String message) {
        logAgentEvent(eventType, agentId, agentName, message, null, null);
    }

    public void logAgentEvent(EventTypes.Agent eventType, String agentId, String agentName,
                               String message, Map<String, Object> details, String error) {
        EventSeverity severity = EventSeverity.INFO;
        if (eventType == EventTypes.Agent.AGENT_REGISTRATION_FAILED) {
            severity = EventSeverity.ERROR;
        } else if (eventType == EventTypes.Agent.AGENT_SUSPENDED || eventType == EventTypes.Agent.AGENT_DELETED) {
            severity = EventSeverity.WARNING;
        }

        String msg = message != null ? message : "Agent event: " + eventType.name() +
                (agentName != null ? " for " + agentName : "");

        SecurityEvent event = createEvent(EventCategory.AGENT, eventType, severity, msg)
                .agentId(agentId != null ? agentId : contextAgentId)
                .agentName(agentName != null ? agentName : contextAgentName)
                .details(details)
                .error(error)
                .build();

        logEvent(event);
    }

    /**
     * Log agent event with details and success indicator.
     */
    public void logAgentEvent(EventTypes.Agent eventType, String agentName, String message,
                               Map<String, Object> details, boolean success) {
        String error = success ? null : "Agent operation failed";
        logAgentEvent(eventType, null, agentName, message, details, success ? null : error);
    }

    // =========================================================================
    // CREDENTIAL EVENTS
    // =========================================================================

    public void logCredentialEvent(EventTypes.Cred eventType, String credentialType, boolean success) {
        logCredentialEvent(eventType, credentialType, success, null, null, null);
    }

    public void logCredentialEvent(EventTypes.Cred eventType, String credentialType, boolean success,
                                    String agentName, Map<String, Object> details, String error) {
        EventSeverity severity = success ? EventSeverity.INFO : EventSeverity.WARNING;
        if (eventType == EventTypes.Cred.CREDENTIAL_TYPE_MISMATCH ||
            eventType == EventTypes.Cred.SECURE_STORAGE_FAILED) {
            severity = EventSeverity.WARNING;
        }

        String msg = String.format("Credential %s: %s%s",
                eventType.name(), credentialType,
                agentName != null ? " for " + agentName : "");

        Map<String, Object> allDetails = new LinkedHashMap<>();
        allDetails.put("credentialType", credentialType);
        if (details != null) allDetails.putAll(details);

        SecurityEvent event = createEvent(EventCategory.CRED, eventType, severity, msg)
                .agentName(agentName != null ? agentName : contextAgentName)
                .result(success ? "SUCCESS" : "FAILURE")
                .details(allDetails)
                .error(error)
                .build();

        logEvent(event);
    }

    // =========================================================================
    // MCP EVENTS
    // =========================================================================

    public void logMcpEvent(EventTypes.Mcp eventType, String serverId, String serverName, boolean success) {
        logMcpEvent(eventType, serverId, serverName, null, null, success, null, null);
    }

    /**
     * Log MCP event with message and details.
     */
    public void logMcpEvent(EventTypes.Mcp eventType, String serverName, String message,
                             Map<String, Object> details, boolean success) {
        logMcpEvent(eventType, null, serverName, null, null, success, details, null);
    }

    public void logMcpEvent(EventTypes.Mcp eventType, String serverId, String serverName,
                             String toolName, String agentId, boolean success,
                             Map<String, Object> details, String error) {
        EventSeverity severity = success ? EventSeverity.INFO : EventSeverity.WARNING;
        if (eventType == EventTypes.Mcp.MCP_CAPABILITY_DRIFT) {
            severity = EventSeverity.WARNING;
        } else if (eventType == EventTypes.Mcp.MCP_ATTESTATION_FAILED) {
            severity = EventSeverity.ERROR;
        }

        StringBuilder msg = new StringBuilder("MCP ").append(eventType.name());
        if (serverName != null) msg.append(": ").append(serverName);
        if (toolName != null) msg.append(" - tool: ").append(toolName);

        Map<String, Object> mcpDetails = new LinkedHashMap<>();
        if (serverId != null) mcpDetails.put("serverId", serverId);
        if (serverName != null) mcpDetails.put("serverName", serverName);
        if (toolName != null) mcpDetails.put("toolName", toolName);
        if (details != null) mcpDetails.putAll(details);

        SecurityEvent event = createEvent(EventCategory.MCP, eventType, severity, msg.toString())
                .agentId(agentId != null ? agentId : contextAgentId)
                .result(success ? "SUCCESS" : "FAILURE")
                .details(mcpDetails)
                .error(error)
                .build();

        logEvent(event);
    }

    // =========================================================================
    // SECURITY EVENTS (HIGH PRIORITY)
    // =========================================================================

    public void logSecurityEvent(EventTypes.Security eventType, String message, EventSeverity severity) {
        logSecurityEvent(eventType, message, severity, null, null, null, null);
    }

    public void logSecurityEvent(EventTypes.Security eventType, String message, EventSeverity severity,
                                  String agentId, String sourceIp, Map<String, Object> details, String error) {
        SecurityEvent event = createEvent(EventCategory.SECURITY, eventType, severity, message)
                .agentId(agentId != null ? agentId : contextAgentId)
                .sourceIp(sourceIp)
                .details(details)
                .error(error)
                .build();

        logEvent(event);
    }

    // =========================================================================
    // CONVENIENCE METHODS
    // =========================================================================

    public void debug(String message) { debug(message, null); }
    public void debug(String message, Map<String, Object> details) {
        logAudit("DEBUG", EventSeverity.DEBUG, message, details);
    }

    public void info(String message) { info(message, null); }
    public void info(String message, Map<String, Object> details) {
        logAudit("INFO", EventSeverity.INFO, message, details);
    }

    public void warning(String message) { warning(message, null); }
    public void warning(String message, Map<String, Object> details) {
        logAudit("WARNING", EventSeverity.WARNING, message, details);
    }

    public void error(String message) { error(message, null); }
    public void error(String message, Map<String, Object> details) {
        logAudit("ERROR", EventSeverity.ERROR, message, details);
    }

    public void critical(String message) { critical(message, null); }
    public void critical(String message, Map<String, Object> details) {
        logAudit("CRITICAL", EventSeverity.CRITICAL, message, details);
    }

    private void logAudit(String eventType, EventSeverity severity, String message, Map<String, Object> details) {
        SecurityEvent event = createEvent(EventCategory.AUDIT, eventType, severity, message)
                .details(details)
                .build();
        logEvent(event);
    }

    // =========================================================================
    // CORE LOGGING
    // =========================================================================

    private SecurityEvent.Builder createEvent(EventCategory category, Enum<?> eventType,
                                               EventSeverity severity, String message) {
        return createEvent(category, eventType.name(), severity, message);
    }

    private SecurityEvent.Builder createEvent(EventCategory category, String eventType,
                                               EventSeverity severity, String message) {
        return SecurityEvent.builder()
                .category(category)
                .eventType(eventType)
                .severity(severity)
                .message(message)
                .sessionId(sessionId)
                .agentId(contextAgentId)
                .agentName(contextAgentName)
                .userId(contextUserId)
                .organizationId(contextOrganizationId);
    }

    private void logEvent(SecurityEvent event) {
        if (!enabled) return;
        if (!shouldLog(event.getSeverity())) return;

        if (asyncEnabled) {
            asyncExecutor.submit(() -> writeEvent(event));
        } else {
            writeEvent(event);
        }
    }

    private boolean shouldLog(String severity) {
        int eventLevel = severityToLevel(severity);
        int minLevelValue = severityToLevel(minLevel.getValue());
        return eventLevel >= minLevelValue;
    }

    private int severityToLevel(String severity) {
        return switch (severity) {
            case "DEBUG" -> 0;
            case "INFO" -> 1;
            case "WARNING" -> 2;
            case "ERROR" -> 3;
            case "CRITICAL" -> 4;
            default -> 1;
        };
    }

    private synchronized void writeEvent(SecurityEvent event) {
        String json = event.toJson();

        // Write to file
        if (fileWriter != null) {
            checkRotation();
            fileWriter.println(json);
        }

        // Write to stdout
        if (logToStdout) {
            System.out.println(json);
        }

        // Write to stderr for errors
        if (logToStderr && severityToLevel(event.getSeverity()) >= severityToLevel("ERROR")) {
            System.err.println(json);
        }
    }

    private void checkRotation() {
        if (logFilePath == null || fileWriter == null) return;

        try {
            long size = Files.size(logFilePath);
            if (size >= maxFileSize) {
                rotateLogFile();
            }
        } catch (IOException ignored) {}
    }

    private void rotateLogFile() {
        try {
            fileWriter.close();

            // Rotate existing backups
            for (int i = maxBackupCount - 1; i >= 1; i--) {
                Path older = Paths.get(logFilePath + "." + i);
                Path newer = Paths.get(logFilePath + "." + (i + 1));
                if (Files.exists(older)) {
                    Files.move(older, newer, StandardCopyOption.REPLACE_EXISTING);
                }
            }

            // Move current to .1
            Path backup = Paths.get(logFilePath + ".1");
            Files.move(logFilePath, backup, StandardCopyOption.REPLACE_EXISTING);

            // Open new file
            fileWriter = new PrintWriter(new BufferedWriter(
                    new FileWriter(logFilePath.toFile(), true)), true);

        } catch (IOException e) {
            log.warn("Log rotation failed: {}", e.getMessage());
        }
    }

    /**
     * Shutdown the security logger.
     */
    public void shutdown() {
        asyncExecutor.shutdown();
        try {
            asyncExecutor.awaitTermination(5, TimeUnit.SECONDS);
        } catch (InterruptedException e) {
            asyncExecutor.shutdownNow();
        }
        if (fileWriter != null) {
            fileWriter.close();
        }
    }

    // =========================================================================
    // CONFIGURATION
    // =========================================================================

    public static class Config {
        final boolean enabled;
        final EventSeverity logLevel;
        final String logFile;
        final boolean logToStdout;
        final boolean logToStderr;
        final long maxFileSize;
        final int maxBackupCount;
        final boolean asyncEnabled;

        private Config(Builder builder) {
            this.enabled = builder.enabled;
            this.logLevel = builder.logLevel;
            this.logFile = builder.logFile;
            this.logToStdout = builder.logToStdout;
            this.logToStderr = builder.logToStderr;
            this.maxFileSize = builder.maxFileSize;
            this.maxBackupCount = builder.maxBackupCount;
            this.asyncEnabled = builder.asyncEnabled;
        }

        public static Builder builder() { return new Builder(); }

        public static class Builder {
            private boolean enabled = true;
            private EventSeverity logLevel = EventSeverity.INFO;
            private String logFile;
            private boolean logToStdout = false;
            private boolean logToStderr = true;
            private long maxFileSize = 10 * 1024 * 1024; // 10MB
            private int maxBackupCount = 5;
            private boolean asyncEnabled = false;

            public Builder enabled(boolean enabled) { this.enabled = enabled; return this; }
            public Builder logLevel(EventSeverity level) { this.logLevel = level; return this; }
            public Builder logFile(String path) { this.logFile = path; return this; }
            public Builder logToStdout(boolean enabled) { this.logToStdout = enabled; return this; }
            public Builder logToStderr(boolean enabled) { this.logToStderr = enabled; return this; }
            public Builder maxFileSize(long bytes) { this.maxFileSize = bytes; return this; }
            public Builder maxBackupCount(int count) { this.maxBackupCount = count; return this; }
            public Builder asyncEnabled(boolean enabled) { this.asyncEnabled = enabled; return this; }
            public Config build() { return new Config(this); }
        }
    }
}
