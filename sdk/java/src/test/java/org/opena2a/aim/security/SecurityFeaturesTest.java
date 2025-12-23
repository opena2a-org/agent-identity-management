package org.opena2a.aim.security;

import org.junit.jupiter.api.*;

import java.util.*;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Integration tests for security features.
 */
class SecurityFeaturesTest {

    // =========================================================================
    // SECURITY LOGGER
    // =========================================================================

    @Test
    @DisplayName("SecurityLogger singleton instance works")
    void testSecurityLoggerSingleton() {
        SecurityLogger logger1 = SecurityLogger.getInstance();
        SecurityLogger logger2 = SecurityLogger.getInstance();

        assertSame(logger1, logger2);
        assertNotNull(logger1);
    }

    @Test
    @DisplayName("SecurityLogger can set agent context")
    void testSecurityLoggerAgentContext() {
        SecurityLogger logger = SecurityLogger.getInstance();

        assertDoesNotThrow(() -> {
            logger.setAgentContext("agent-123", "test-agent", "user-456", "org-789");
        });

        assertDoesNotThrow(() -> {
            logger.clearAgentContext();
        });
    }

    @Test
    @DisplayName("SecurityLogger convenience methods work")
    void testSecurityLoggerConvenienceMethods() {
        SecurityLogger logger = SecurityLogger.getInstance();

        assertDoesNotThrow(() -> {
            logger.debug("Debug message");
            logger.info("Info message");
            logger.warning("Warning message");
            logger.error("Error message");
        });
    }

    @Test
    @DisplayName("SecurityLogger configuration works")
    void testSecurityLoggerConfiguration() {
        SecurityLogger logger = SecurityLogger.getInstance();

        assertDoesNotThrow(() -> {
            SecurityLogger.Config config = SecurityLogger.Config.builder()
                    .enabled(true)
                    .logLevel(EventSeverity.DEBUG)
                    .logToStdout(false)
                    .logToStderr(false)
                    .build();
            logger.configure(config);
        });
    }

    @Test
    @DisplayName("SecurityLogger session management works")
    void testSecurityLoggerSessionManagement() {
        SecurityLogger logger = SecurityLogger.getInstance();

        String session1 = logger.startSession();
        String session2 = logger.startSession();

        assertNotNull(session1);
        assertNotNull(session2);
        assertNotEquals(session1, session2);
    }

    // =========================================================================
    // RISK DETECTOR
    // =========================================================================

    @Test
    @DisplayName("RiskDetector singleton instance works")
    void testRiskDetectorSingleton() {
        RiskDetector detector1 = RiskDetector.getInstance();
        RiskDetector detector2 = RiskDetector.getInstance();

        assertSame(detector1, detector2);
        assertNotNull(detector1);
    }

    @Test
    @DisplayName("RiskDetector detects HIGH risk for payment capabilities")
    void testRiskDetectorHighPayment() {
        RiskDetector detector = RiskDetector.getInstance();

        // payment:* matches HIGH pattern (^payment:.*)
        RiskLevel level = detector.detectRisk("payment:process");
        assertEquals(RiskLevel.HIGH, level);
    }

    @Test
    @DisplayName("RiskDetector detects CRITICAL risk for admin capabilities")
    void testRiskDetectorCriticalAdmin() {
        RiskDetector detector = RiskDetector.getInstance();

        // admin:* matches CRITICAL pattern (^admin:.*)
        RiskLevel level = detector.detectRisk("admin:delete_user");
        assertEquals(RiskLevel.CRITICAL, level);
    }

    @Test
    @DisplayName("RiskDetector detects MEDIUM risk for write operations")
    void testRiskDetectorMediumWrite() {
        RiskDetector detector = RiskDetector.getInstance();

        // db:write falls to keyword analysis, "write" is MEDIUM keyword
        RiskLevel level = detector.detectRisk("db:write");
        assertEquals(RiskLevel.MEDIUM, level);
    }

    @Test
    @DisplayName("RiskDetector detects MEDIUM risk for read operations")
    void testRiskDetectorMediumRead() {
        RiskDetector detector = RiskDetector.getInstance();

        // db:read matches MEDIUM pattern (^db:(read|query|select).*)
        RiskLevel level = detector.detectRisk("db:read");
        assertEquals(RiskLevel.MEDIUM, level);
    }

    @Test
    @DisplayName("RiskDetector detects HIGH risk for delete operations")
    void testRiskDetectorHighDelete() {
        RiskDetector detector = RiskDetector.getInstance();

        // db:delete falls to keyword analysis, "delete" is HIGH keyword
        RiskLevel level = detector.detectRisk("db:delete");
        assertEquals(RiskLevel.HIGH, level);
    }

    @Test
    @DisplayName("RiskDetector returns LOW risk for unknown capabilities")
    void testRiskDetectorLowUnknown() {
        RiskDetector detector = RiskDetector.getInstance();

        RiskLevel level = detector.detectRisk("unknown:capability");
        assertEquals(RiskLevel.LOW, level);
    }

    @Test
    @DisplayName("RiskDetector aggregate risk works")
    void testRiskDetectorAggregateRisk() {
        RiskDetector detector = RiskDetector.getInstance();

        // Highest is payment:process (HIGH)
        RiskLevel level = detector.detectAggregateRisk(List.of("db:read", "db:write", "payment:process"));
        assertEquals(RiskLevel.HIGH, level);
    }

    @Test
    @DisplayName("RiskDetector aggregate risk returns CRITICAL for admin capabilities")
    void testRiskDetectorAggregateRiskCritical() {
        RiskDetector detector = RiskDetector.getInstance();

        // admin:delete is CRITICAL, which is the highest
        RiskLevel level = detector.detectAggregateRisk(List.of("db:read", "admin:delete"));
        assertEquals(RiskLevel.CRITICAL, level);
    }

    // =========================================================================
    // RISK LEVEL
    // =========================================================================

    @Test
    @DisplayName("RiskLevel comparison works")
    void testRiskLevelComparison() {
        assertTrue(RiskLevel.CRITICAL.isHigherThan(RiskLevel.HIGH));
        assertTrue(RiskLevel.HIGH.isHigherThan(RiskLevel.MEDIUM));
        assertTrue(RiskLevel.MEDIUM.isHigherThan(RiskLevel.LOW));

        assertFalse(RiskLevel.LOW.isHigherThan(RiskLevel.MEDIUM));
    }

    @Test
    @DisplayName("RiskLevel requiresApproval works")
    void testRiskLevelRequiresApproval() {
        assertTrue(RiskLevel.CRITICAL.requiresApproval());
        assertTrue(RiskLevel.HIGH.requiresApproval());
        assertTrue(RiskLevel.MEDIUM.requiresApproval());
        assertFalse(RiskLevel.LOW.requiresApproval());
    }

    // =========================================================================
    // SECURE CREDENTIAL STORAGE
    // =========================================================================

    @Test
    @DisplayName("SecureCredentialStorage singleton instance works")
    void testSecureCredentialStorageSingleton() {
        SecureCredentialStorage storage1 = SecureCredentialStorage.getInstance();
        SecureCredentialStorage storage2 = SecureCredentialStorage.getInstance();

        assertSame(storage1, storage2);
        assertNotNull(storage1);
    }

    // =========================================================================
    // EVENT TYPES
    // =========================================================================

    @Test
    @DisplayName("EventTypes enums exist")
    void testEventTypesExist() {
        assertNotNull(EventTypes.Authn.class);
        assertNotNull(EventTypes.Authz.class);
        assertNotNull(EventTypes.Agent.class);
        assertNotNull(EventTypes.Mcp.class);
        assertNotNull(EventTypes.Cred.class);
        assertNotNull(EventTypes.Security.class);
    }

    @Test
    @DisplayName("EventSeverity enum values exist")
    void testEventSeverityExists() {
        assertNotNull(EventSeverity.DEBUG);
        assertNotNull(EventSeverity.INFO);
        assertNotNull(EventSeverity.WARNING);
        assertNotNull(EventSeverity.ERROR);
        assertNotNull(EventSeverity.CRITICAL);
    }
}
