package org.opena2a.aim.security;

/**
 * Risk levels for capability classification.
 *
 * <p>Used by RiskDetector to automatically classify capabilities
 * based on security impact and required approval workflows.</p>
 *
 * <ul>
 *   <li>CRITICAL: Requires executive approval, full audit trail</li>
 *   <li>HIGH: Requires manager approval, detailed logging</li>
 *   <li>MEDIUM: Standard approval, normal logging</li>
 *   <li>LOW: Auto-approved, minimal logging</li>
 * </ul>
 */
public enum RiskLevel {
    /**
     * Critical risk - system administration, data destruction, privileged access.
     * Examples: admin:*, system:delete, database:drop, credential:*
     */
    CRITICAL(4, "CRITICAL", "Requires executive approval"),

    /**
     * High risk - financial operations, user management, data export.
     * Examples: payment:*, user:delete, data:export, pii:*
     */
    HIGH(3, "HIGH", "Requires manager approval"),

    /**
     * Medium risk - data access, file operations, configuration changes.
     * Examples: db:read, user:read, file:write, config:update
     */
    MEDIUM(2, "MEDIUM", "Standard approval workflow"),

    /**
     * Low risk - read-only operations, caching, non-sensitive APIs.
     * Examples: api:call, cache:read, log:view
     */
    LOW(1, "LOW", "Auto-approved");

    private final int level;
    private final String name;
    private final String approvalRequirement;

    RiskLevel(int level, String name, String approvalRequirement) {
        this.level = level;
        this.name = name;
        this.approvalRequirement = approvalRequirement;
    }

    public int getLevel() {
        return level;
    }

    public String getName() {
        return name;
    }

    public String getApprovalRequirement() {
        return approvalRequirement;
    }

    /**
     * Check if this risk level is higher than another.
     */
    public boolean isHigherThan(RiskLevel other) {
        return this.level > other.level;
    }

    /**
     * Check if this risk level requires explicit approval.
     */
    public boolean requiresApproval() {
        return this.level >= MEDIUM.level;
    }

    /**
     * Check if this risk level requires detailed audit logging.
     */
    public boolean requiresAuditLog() {
        return this.level >= HIGH.level;
    }
}
