package org.opena2a.aim.security;

/**
 * Security event type enums for SOC/SIEM integration.
 * Each enum represents specific events within a category.
 */
public final class EventTypes {

    private EventTypes() {} // Utility class

    /**
     * Authentication event types.
     */
    public enum Authn {
        TOKEN_REFRESH,
        TOKEN_REFRESH_FAILED,
        TOKEN_EXPIRED,
        TOKEN_REVOKED,
        TOKEN_RECOVERED,
        CREDENTIAL_LOAD,
        CREDENTIAL_LOAD_FAILED,
        SDK_INITIALIZED
    }

    /**
     * Authorization event types.
     */
    public enum Authz {
        CAPABILITY_CHECK,
        CAPABILITY_GRANTED,
        CAPABILITY_DENIED,
        CAPABILITY_ESCALATION,
        ACTION_EXECUTED,
        ACTION_DENIED,
        JIT_REQUEST,
        JIT_APPROVED,
        JIT_DENIED,
        JIT_TIMEOUT
    }

    /**
     * Agent lifecycle event types.
     */
    public enum Agent {
        AGENT_REGISTERED,
        AGENT_REGISTRATION_FAILED,
        AGENT_LOADED,
        AGENT_UPDATED,
        AGENT_DELETED,
        AGENT_SUSPENDED,
        AGENT_REACTIVATED,
        AGENT_STALE_CREDENTIALS,
        TRUST_SCORE_CHANGED
    }

    /**
     * Credential management event types.
     */
    public enum Cred {
        CREDENTIAL_CREATED,
        CREDENTIAL_LOADED,
        CREDENTIAL_SAVED,
        CREDENTIAL_MIGRATED,
        CREDENTIAL_DELETED,
        CREDENTIAL_ROTATED,
        CREDENTIAL_TYPE_MISMATCH,
        SECURE_STORAGE_ENABLED,
        SECURE_STORAGE_FAILED
    }

    /**
     * MCP integration event types.
     */
    public enum Mcp {
        MCP_SERVER_CONNECTED,
        MCP_SERVER_DISCONNECTED,
        MCP_TOOL_USED,
        MCP_ATTESTATION_CREATED,
        MCP_ATTESTATION_FAILED,
        MCP_CAPABILITY_DRIFT,
        MCP_CONNECTION_RECORDED,
        MCP_DISCOVERY_STARTED,
        MCP_DISCOVERY_COMPLETED,
        MCP_DISCOVERY_FAILED,
        MCP_DISCOVERY_CACHED
    }

    /**
     * Security-specific event types.
     */
    public enum Security {
        ANOMALY_DETECTED,
        POLICY_VIOLATION,
        SIGNATURE_VERIFICATION_FAILED,
        UNAUTHORIZED_ACCESS_ATTEMPT,
        RATE_LIMIT_EXCEEDED,
        SUSPICIOUS_PATTERN,
        ENCRYPTION_FAILURE
    }
}
