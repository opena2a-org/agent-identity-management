package org.opena2a.aim.security;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;

import java.net.InetAddress;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;

/**
 * Structured security event for SOC/SIEM integration.
 *
 * <p>Designed for easy parsing and correlation in security tools.
 * All timestamps are ISO 8601 UTC format for cross-system compatibility.</p>
 *
 * <p>Compatible with: Splunk, ELK, Datadog, Sumo Logic, Azure Sentinel, AWS SecurityHub</p>
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
public class SecurityEvent {

    private static final ObjectMapper MAPPER = new ObjectMapper()
            .disable(SerializationFeature.INDENT_OUTPUT);
    private static final String SDK_VERSION = "aim-sdk-java@1.0.0";
    private static String HOSTNAME;

    static {
        try {
            HOSTNAME = InetAddress.getLocalHost().getHostName();
        } catch (Exception e) {
            HOSTNAME = "unknown";
        }
    }

    // Required fields
    private final String timestamp;
    private final String eventId;
    private final String category;
    private final String eventType;
    private final String severity;
    private final String message;

    // Context fields
    private final String sdkVersion;
    private final String hostname;
    private final long processId;
    private final String threadId;

    // Identity fields
    private String agentId;
    private String agentName;
    private String userId;
    private String organizationId;

    // Action fields
    private String action;
    private String resource;
    private String result;

    // Network fields
    private String sourceIp;
    private String destinationUrl;
    private String httpMethod;
    private Integer httpStatus;

    // Additional context
    private Map<String, Object> details;
    private String error;
    private String stackTrace;

    // Correlation fields
    private String correlationId;
    private String parentEventId;
    private String sessionId;

    private SecurityEvent(Builder builder) {
        this.timestamp = builder.timestamp != null ? builder.timestamp : Instant.now().toString();
        this.eventId = builder.eventId != null ? builder.eventId : UUID.randomUUID().toString();
        this.category = builder.category;
        this.eventType = builder.eventType;
        this.severity = builder.severity;
        this.message = builder.message;
        this.sdkVersion = SDK_VERSION;
        this.hostname = HOSTNAME;
        this.processId = ProcessHandle.current().pid();
        this.threadId = Thread.currentThread().getName();
        this.agentId = builder.agentId;
        this.agentName = builder.agentName;
        this.userId = builder.userId;
        this.organizationId = builder.organizationId;
        this.action = builder.action;
        this.resource = builder.resource;
        this.result = builder.result;
        this.sourceIp = builder.sourceIp;
        this.destinationUrl = builder.destinationUrl;
        this.httpMethod = builder.httpMethod;
        this.httpStatus = builder.httpStatus;
        this.details = builder.details;
        this.error = builder.error;
        this.stackTrace = builder.stackTrace;
        this.correlationId = builder.correlationId;
        this.parentEventId = builder.parentEventId;
        this.sessionId = builder.sessionId;
    }

    /**
     * Convert to compact JSON string for logging.
     */
    public String toJson() {
        try {
            return MAPPER.writeValueAsString(toMap());
        } catch (JsonProcessingException e) {
            return "{\"error\":\"serialization_failed\",\"message\":\"" + message + "\"}";
        }
    }

    /**
     * Convert to map, excluding null/empty fields.
     */
    public Map<String, Object> toMap() {
        Map<String, Object> map = new LinkedHashMap<>();

        // Required fields
        map.put("timestamp", timestamp);
        map.put("eventId", eventId);
        map.put("category", category);
        map.put("eventType", eventType);
        map.put("severity", severity);
        map.put("message", message);

        // Context fields
        map.put("sdkVersion", sdkVersion);
        map.put("hostname", hostname);
        map.put("processId", processId);
        map.put("threadId", threadId);

        // Identity fields
        if (agentId != null) map.put("agentId", agentId);
        if (agentName != null) map.put("agentName", agentName);
        if (userId != null) map.put("userId", userId);
        if (organizationId != null) map.put("organizationId", organizationId);

        // Action fields
        if (action != null) map.put("action", action);
        if (resource != null) map.put("resource", resource);
        if (result != null) map.put("result", result);

        // Network fields
        if (sourceIp != null) map.put("sourceIp", sourceIp);
        if (destinationUrl != null) map.put("destinationUrl", destinationUrl);
        if (httpMethod != null) map.put("httpMethod", httpMethod);
        if (httpStatus != null) map.put("httpStatus", httpStatus);

        // Additional context
        if (details != null && !details.isEmpty()) map.put("details", details);
        if (error != null) map.put("error", error);
        if (stackTrace != null) map.put("stackTrace", stackTrace);

        // Correlation fields
        if (correlationId != null) map.put("correlationId", correlationId);
        if (parentEventId != null) map.put("parentEventId", parentEventId);
        if (sessionId != null) map.put("sessionId", sessionId);

        return map;
    }

    // Getters
    public String getTimestamp() { return timestamp; }
    public String getEventId() { return eventId; }
    public String getCategory() { return category; }
    public String getEventType() { return eventType; }
    public String getSeverity() { return severity; }
    public String getMessage() { return message; }
    public String getSdkVersion() { return sdkVersion; }
    public String getHostname() { return hostname; }
    public long getProcessId() { return processId; }
    public String getThreadId() { return threadId; }
    public String getAgentId() { return agentId; }
    public String getAgentName() { return agentName; }
    public String getUserId() { return userId; }
    public String getOrganizationId() { return organizationId; }
    public String getAction() { return action; }
    public String getResource() { return resource; }
    public String getResult() { return result; }
    public String getSourceIp() { return sourceIp; }
    public String getDestinationUrl() { return destinationUrl; }
    public String getHttpMethod() { return httpMethod; }
    public Integer getHttpStatus() { return httpStatus; }
    public Map<String, Object> getDetails() { return details; }
    public String getError() { return error; }
    public String getStackTrace() { return stackTrace; }
    public String getCorrelationId() { return correlationId; }
    public String getParentEventId() { return parentEventId; }
    public String getSessionId() { return sessionId; }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String timestamp;
        private String eventId;
        private String category;
        private String eventType;
        private String severity;
        private String message;
        private String agentId;
        private String agentName;
        private String userId;
        private String organizationId;
        private String action;
        private String resource;
        private String result;
        private String sourceIp;
        private String destinationUrl;
        private String httpMethod;
        private Integer httpStatus;
        private Map<String, Object> details;
        private String error;
        private String stackTrace;
        private String correlationId;
        private String parentEventId;
        private String sessionId;

        public Builder timestamp(String timestamp) { this.timestamp = timestamp; return this; }
        public Builder eventId(String eventId) { this.eventId = eventId; return this; }
        public Builder category(EventCategory category) { this.category = category.getValue(); return this; }
        public Builder category(String category) { this.category = category; return this; }
        public Builder eventType(Enum<?> eventType) { this.eventType = eventType.name(); return this; }
        public Builder eventType(String eventType) { this.eventType = eventType; return this; }
        public Builder severity(EventSeverity severity) { this.severity = severity.getValue(); return this; }
        public Builder severity(String severity) { this.severity = severity; return this; }
        public Builder message(String message) { this.message = message; return this; }
        public Builder agentId(String agentId) { this.agentId = agentId; return this; }
        public Builder agentName(String agentName) { this.agentName = agentName; return this; }
        public Builder userId(String userId) { this.userId = userId; return this; }
        public Builder organizationId(String organizationId) { this.organizationId = organizationId; return this; }
        public Builder action(String action) { this.action = action; return this; }
        public Builder resource(String resource) { this.resource = resource; return this; }
        public Builder result(String result) { this.result = result; return this; }
        public Builder sourceIp(String sourceIp) { this.sourceIp = sourceIp; return this; }
        public Builder destinationUrl(String destinationUrl) { this.destinationUrl = destinationUrl; return this; }
        public Builder httpMethod(String httpMethod) { this.httpMethod = httpMethod; return this; }
        public Builder httpStatus(Integer httpStatus) { this.httpStatus = httpStatus; return this; }
        public Builder details(Map<String, Object> details) { this.details = details; return this; }
        public Builder addDetail(String key, Object value) {
            if (this.details == null) this.details = new LinkedHashMap<>();
            this.details.put(key, value);
            return this;
        }
        public Builder error(String error) { this.error = error; return this; }
        public Builder stackTrace(String stackTrace) { this.stackTrace = stackTrace; return this; }
        public Builder correlationId(String correlationId) { this.correlationId = correlationId; return this; }
        public Builder parentEventId(String parentEventId) { this.parentEventId = parentEventId; return this; }
        public Builder sessionId(String sessionId) { this.sessionId = sessionId; return this; }

        public SecurityEvent build() {
            if (category == null) throw new IllegalStateException("category is required");
            if (eventType == null) throw new IllegalStateException("eventType is required");
            if (severity == null) throw new IllegalStateException("severity is required");
            if (message == null) throw new IllegalStateException("message is required");
            return new SecurityEvent(this);
        }
    }
}
