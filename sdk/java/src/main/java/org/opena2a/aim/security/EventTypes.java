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
        /** Token was refreshed successfully. */
        TOKEN_REFRESH,
        /** Token refresh failed. */
        TOKEN_REFRESH_FAILED,
        /** Token has expired. */
        TOKEN_EXPIRED,
        /** Token was revoked. */
        TOKEN_REVOKED,
        /** Token was recovered from storage. */
        TOKEN_RECOVERED,
        /** Credentials were loaded. */
        CREDENTIAL_LOAD,
        /** Credential load failed. */
        CREDENTIAL_LOAD_FAILED,
        /** SDK was initialized. */
        SDK_INITIALIZED
    }

    /**
     * Authorization event types.
     */
    public enum Authz {
        /** Capability was checked. */
        CAPABILITY_CHECK,
        /** Capability was granted. */
        CAPABILITY_GRANTED,
        /** Capability was denied. */
        CAPABILITY_DENIED,
        /** Capability escalation attempted. */
        CAPABILITY_ESCALATION,
        /** Action was executed. */
        ACTION_EXECUTED,
        /** Action was denied. */
        ACTION_DENIED,
        /** JIT access request initiated. */
        JIT_REQUEST,
        /** JIT access was approved. */
        JIT_APPROVED,
        /** JIT access was denied. */
        JIT_DENIED,
        /** JIT request timed out. */
        JIT_TIMEOUT
    }

    /**
     * Agent lifecycle event types.
     */
    public enum Agent {
        /** Agent was registered. */
        AGENT_REGISTERED,
        /** Agent registration failed. */
        AGENT_REGISTRATION_FAILED,
        /** Agent was loaded from cache. */
        AGENT_LOADED,
        /** Agent was updated. */
        AGENT_UPDATED,
        /** Agent was deleted. */
        AGENT_DELETED,
        /** Agent was suspended. */
        AGENT_SUSPENDED,
        /** Agent was reactivated. */
        AGENT_REACTIVATED,
        /** Agent has stale credentials. */
        AGENT_STALE_CREDENTIALS,
        /** Agent trust score changed. */
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
