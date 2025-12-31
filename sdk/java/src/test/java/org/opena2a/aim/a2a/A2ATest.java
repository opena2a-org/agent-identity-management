package org.opena2a.aim.a2a;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Comprehensive tests for A2A (Agent-to-Agent) protocol support in the Java SDK.
 * Tests all A2A data classes and their functionality.
 */
@DisplayName("A2A Protocol Tests")
class A2ATest {

    @Nested
    @DisplayName("A2AAgentCard Tests")
    class A2AAgentCardTests {

        @Test
        @DisplayName("Should create agent card with builder")
        void testAgentCardBuilder() {
            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .agentId("agent-456")
                    .name("Test Agent")
                    .description("A test agent for A2A protocol")
                    .url("https://agent.example.com")
                    .version("1.0.0")
                    .verified(true)
                    .build();

            assertEquals("card-123", card.getId());
            assertEquals("agent-456", card.getAgentId());
            assertEquals("Test Agent", card.getName());
            assertEquals("A test agent for A2A protocol", card.getDescription());
            assertEquals("https://agent.example.com", card.getUrl());
            assertEquals("1.0.0", card.getVersion());
            assertTrue(card.isVerified());
        }

        @Test
        @DisplayName("Should create agent card with provider")
        void testAgentCardWithProvider() {
            A2AAgentCard.Provider provider = new A2AAgentCard.Provider("Acme Corp", "https://acme.com");
            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .name("Acme Agent")
                    .provider(provider)
                    .build();

            assertNotNull(card.getProvider());
            assertEquals("Acme Corp", card.getProvider().getOrganization());
            assertEquals("https://acme.com", card.getProvider().getUrl());
        }

        @Test
        @DisplayName("Should create agent card with capabilities")
        void testAgentCardWithCapabilities() {
            A2AAgentCard.Capabilities capabilities = new A2AAgentCard.Capabilities();
            capabilities.setStreaming(true);
            capabilities.setPushNotifications(true);
            capabilities.setStateTransitionHistory(false);

            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .name("Capable Agent")
                    .capabilities(capabilities)
                    .build();

            assertNotNull(card.getCapabilities());
            assertTrue(card.getCapabilities().isStreaming());
            assertTrue(card.getCapabilities().isPushNotifications());
            assertFalse(card.getCapabilities().isStateTransitionHistory());
        }

        @Test
        @DisplayName("Should create agent card with skills")
        void testAgentCardWithSkills() {
            A2AAgentCard.Skill skill = new A2AAgentCard.Skill();
            skill.setId("skill-1");
            skill.setName("Data Analysis");
            skill.setDescription("Analyzes data patterns");
            skill.setTags(Arrays.asList("analytics", "ml"));
            skill.setExamples(Arrays.asList("Analyze sales data", "Find trends"));
            skill.setInputModes(Arrays.asList("text", "file"));
            skill.setOutputModes(Arrays.asList("text", "json"));

            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .skills(Arrays.asList(skill))
                    .build();

            assertNotNull(card.getSkills());
            assertEquals(1, card.getSkills().size());
            assertEquals("skill-1", card.getSkills().get(0).getId());
            assertEquals("Data Analysis", card.getSkills().get(0).getName());
            assertEquals(2, card.getSkills().get(0).getTags().size());
        }

        @Test
        @DisplayName("Should create agent card with authentication")
        void testAgentCardWithAuthentication() {
            A2AAgentCard.Authentication auth = new A2AAgentCard.Authentication();
            auth.setSchemes(Arrays.asList("bearer", "api_key"));
            auth.setCredentials("required");

            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .authentication(auth)
                    .build();

            assertNotNull(card.getAuthentication());
            assertEquals(2, card.getAuthentication().getSchemes().size());
            assertTrue(card.getAuthentication().getSchemes().contains("bearer"));
        }

        @Test
        @DisplayName("Should create agent card with AIM attestation")
        void testAgentCardWithAimAttestation() {
            Instant expiresAt = Instant.now().plus(24, ChronoUnit.HOURS);

            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .aimAttestation("eyJhbGciOiJFZDI1NTE5IiwidHlwIjoiSldUIn0...")
                    .aimAttestationExpiresAt(expiresAt)
                    .publicKey("MCowBQYDK2VwAyEA...")
                    .supportsAuthenticatedExtendedCard(true)
                    .build();

            assertNotNull(card.getAimAttestation());
            assertEquals(expiresAt, card.getAimAttestationExpiresAt());
            assertNotNull(card.getPublicKey());
            assertTrue(card.isSupportsAuthenticatedExtendedCard());
        }

        @Test
        @DisplayName("Should handle default input/output modes")
        void testDefaultModes() {
            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .defaultInputModes(Arrays.asList("text", "file"))
                    .defaultOutputModes(Arrays.asList("text", "json", "stream"))
                    .build();

            assertEquals(2, card.getDefaultInputModes().size());
            assertEquals(3, card.getDefaultOutputModes().size());
            assertTrue(card.getDefaultInputModes().contains("text"));
            assertTrue(card.getDefaultOutputModes().contains("stream"));
        }

        @Test
        @DisplayName("Should have working toString")
        void testAgentCardToString() {
            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .agentId("agent-456")
                    .name("Test Agent")
                    .url("https://example.com")
                    .verified(true)
                    .build();

            String str = card.toString();
            assertTrue(str.contains("card-123"));
            assertTrue(str.contains("agent-456"));
            assertTrue(str.contains("Test Agent"));
            assertTrue(str.contains("verified=true"));
        }
    }

    @Nested
    @DisplayName("A2ARequestSignature Tests")
    class A2ARequestSignatureTests {

        @Test
        @DisplayName("Should create request signature with builder")
        void testRequestSignatureBuilder() {
            Instant timestamp = Instant.now();

            A2ARequestSignature sig = A2ARequestSignature.builder()
                    .signature("base64-encoded-signature")
                    .timestamp(timestamp)
                    .nonce("unique-nonce-123")
                    .algorithm("Ed25519")
                    .agentId("agent-456")
                    .publicKey("MCowBQYDK2VwAyEA...")
                    .build();

            assertEquals("base64-encoded-signature", sig.getSignature());
            assertEquals(timestamp, sig.getTimestamp());
            assertEquals("unique-nonce-123", sig.getNonce());
            assertEquals("Ed25519", sig.getAlgorithm());
            assertEquals("agent-456", sig.getAgentId());
            assertEquals("MCowBQYDK2VwAyEA...", sig.getPublicKey());
        }

        @Test
        @DisplayName("Should create request signature with headers")
        void testRequestSignatureWithHeaders() {
            Map<String, String> headers = new HashMap<>();
            headers.put("X-AIM-Signature", "sig-value");
            headers.put("X-AIM-Timestamp", "1234567890");
            headers.put("X-AIM-Nonce", "nonce-value");

            A2ARequestSignature sig = A2ARequestSignature.builder()
                    .signature("sig")
                    .timestamp(Instant.now())
                    .nonce("nonce")
                    .algorithm("Ed25519")
                    .headers(headers)
                    .build();

            assertNotNull(sig.getHeaders());
            assertEquals(3, sig.getHeaders().size());
            assertEquals("sig-value", sig.getHeaders().get("X-AIM-Signature"));
        }

        @Test
        @DisplayName("Should handle constructor with parameters")
        void testRequestSignatureConstructor() {
            Instant timestamp = Instant.now();

            A2ARequestSignature sig = new A2ARequestSignature(
                    "signature",
                    timestamp,
                    "nonce",
                    "Ed25519",
                    "agent-id",
                    "public-key"
            );

            assertEquals("signature", sig.getSignature());
            assertEquals(timestamp, sig.getTimestamp());
            assertEquals("nonce", sig.getNonce());
            assertEquals("Ed25519", sig.getAlgorithm());
            assertEquals("agent-id", sig.getAgentId());
            assertEquals("public-key", sig.getPublicKey());
        }

        @Test
        @DisplayName("Should have working toString")
        void testRequestSignatureToString() {
            A2ARequestSignature sig = A2ARequestSignature.builder()
                    .agentId("agent-123")
                    .algorithm("Ed25519")
                    .nonce("nonce-value")
                    .timestamp(Instant.now())
                    .build();

            String str = sig.toString();
            assertTrue(str.contains("agent-123"));
            assertTrue(str.contains("Ed25519"));
            assertTrue(str.contains("nonce-value"));
        }
    }

    @Nested
    @DisplayName("A2ATrustScore Tests")
    class A2ATrustScoreTests {

        @Test
        @DisplayName("Should create trust score with builder")
        void testTrustScoreBuilder() {
            A2ATrustScore score = A2ATrustScore.builder()
                    .id("score-123")
                    .evaluatorAgentId("evaluator-agent")
                    .subjectAgentId("subject-agent")
                    .score(0.85)
                    .confidence(0.92)
                    .interactionCount(100)
                    .successfulInteractions(85)
                    .failedInteractions(15)
                    .build();

            assertEquals("score-123", score.getId());
            assertEquals("evaluator-agent", score.getEvaluatorAgentId());
            assertEquals("subject-agent", score.getSubjectAgentId());
            assertEquals(0.85, score.getScore(), 0.001);
            assertEquals(0.92, score.getConfidence(), 0.001);
            assertEquals(100, score.getInteractionCount());
            assertEquals(85, score.getSuccessfulInteractions());
            assertEquals(15, score.getFailedInteractions());
        }

        @Test
        @DisplayName("Should calculate success rate correctly")
        void testSuccessRateCalculation() {
            A2ATrustScore score = A2ATrustScore.builder()
                    .interactionCount(100)
                    .successfulInteractions(75)
                    .failedInteractions(25)
                    .build();

            assertEquals(75.0, score.getSuccessRate(), 0.001);
        }

        @Test
        @DisplayName("Should handle zero interactions for success rate")
        void testSuccessRateWithZeroInteractions() {
            A2ATrustScore score = A2ATrustScore.builder()
                    .interactionCount(0)
                    .successfulInteractions(0)
                    .failedInteractions(0)
                    .build();

            assertEquals(0.0, score.getSuccessRate(), 0.001);
        }

        @Test
        @DisplayName("Should identify high trust score")
        void testHighTrust() {
            A2ATrustScore highTrust = A2ATrustScore.builder()
                    .score(0.85)
                    .build();

            assertTrue(highTrust.isHighTrust());
            assertFalse(highTrust.isLowTrust());
        }

        @Test
        @DisplayName("Should identify low trust score")
        void testLowTrust() {
            A2ATrustScore lowTrust = A2ATrustScore.builder()
                    .score(0.2)
                    .build();

            assertFalse(lowTrust.isHighTrust());
            assertTrue(lowTrust.isLowTrust());
        }

        @Test
        @DisplayName("Should handle trust score with factors")
        void testTrustScoreWithFactors() {
            Map<String, Double> factors = new HashMap<>();
            factors.put("reliability", 0.9);
            factors.put("response_time", 0.85);
            factors.put("accuracy", 0.95);

            A2ATrustScore score = A2ATrustScore.builder()
                    .score(0.9)
                    .factors(factors)
                    .build();

            assertNotNull(score.getFactors());
            assertEquals(3, score.getFactors().size());
            assertEquals(0.9, score.getFactors().get("reliability"), 0.001);
        }

        @Test
        @DisplayName("Should handle last interaction timestamp")
        void testLastInteractionAt() {
            Instant lastInteraction = Instant.now().minus(1, ChronoUnit.HOURS);

            A2ATrustScore score = A2ATrustScore.builder()
                    .lastInteractionAt(lastInteraction)
                    .build();

            assertEquals(lastInteraction, score.getLastInteractionAt());
        }

        @Test
        @DisplayName("Should have working toString")
        void testTrustScoreToString() {
            A2ATrustScore score = A2ATrustScore.builder()
                    .evaluatorAgentId("evaluator")
                    .subjectAgentId("subject")
                    .score(0.85)
                    .confidence(0.9)
                    .interactionCount(50)
                    .build();

            String str = score.toString();
            assertTrue(str.contains("evaluator"));
            assertTrue(str.contains("subject"));
            assertTrue(str.contains("0.85"));
        }
    }

    @Nested
    @DisplayName("A2APeerTrust Tests")
    class A2APeerTrustTests {

        @Test
        @DisplayName("Should create peer trust with builder")
        void testPeerTrustBuilder() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .id("trust-123")
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .trustLevel(A2APeerTrust.TrustLevel.HIGH)
                    .verified(true)
                    .bidirectional(true)
                    .build();

            assertEquals("trust-123", trust.getId());
            assertEquals("agent-A", trust.getSourceAgentId());
            assertEquals("agent-B", trust.getTargetAgentId());
            assertEquals(A2APeerTrust.TrustLevel.HIGH, trust.getTrustLevel());
            assertTrue(trust.isVerified());
            assertTrue(trust.isBidirectional());
        }

        @Test
        @DisplayName("Should handle allowed capabilities")
        void testAllowedCapabilities() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .allowedCapabilities(Arrays.asList("read_data", "write_data", "execute_task"))
                    .build();

            assertNotNull(trust.getAllowedCapabilities());
            assertEquals(3, trust.getAllowedCapabilities().size());
            assertTrue(trust.isCapabilityAllowed("read_data"));
            assertTrue(trust.isCapabilityAllowed("write_data"));
        }

        @Test
        @DisplayName("Should allow any capability when list is empty")
        void testAllowAllCapabilitiesWhenEmpty() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .build();

            assertTrue(trust.isCapabilityAllowed("any_capability"));
        }

        @Test
        @DisplayName("Should allow any capability with wildcard")
        void testAllowAllCapabilitiesWithWildcard() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .allowedCapabilities(Arrays.asList("*"))
                    .build();

            assertTrue(trust.isCapabilityAllowed("read_data"));
            assertTrue(trust.isCapabilityAllowed("any_random_capability"));
        }

        @Test
        @DisplayName("Should deny capability not in list")
        void testDenyCapabilityNotInList() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .allowedCapabilities(Arrays.asList("read_data"))
                    .build();

            assertTrue(trust.isCapabilityAllowed("read_data"));
            assertFalse(trust.isCapabilityAllowed("write_data"));
        }

        @Test
        @DisplayName("Should handle trust levels")
        void testTrustLevels() {
            for (A2APeerTrust.TrustLevel level : A2APeerTrust.TrustLevel.values()) {
                A2APeerTrust trust = A2APeerTrust.builder()
                        .trustLevel(level)
                        .build();
                assertEquals(level, trust.getTrustLevel());
            }
        }

        @Test
        @DisplayName("Should handle restrictions map")
        void testRestrictions() {
            Map<String, Object> restrictions = new HashMap<>();
            restrictions.put("max_requests_per_hour", 100);
            restrictions.put("allowed_data_types", Arrays.asList("text", "json"));
            restrictions.put("require_encryption", true);

            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .restrictions(restrictions)
                    .build();

            assertNotNull(trust.getRestrictions());
            assertEquals(100, trust.getRestrictions().get("max_requests_per_hour"));
            assertTrue((Boolean) trust.getRestrictions().get("require_encryption"));
        }

        @Test
        @DisplayName("Should handle associated trust score")
        void testAssociatedTrustScore() {
            A2ATrustScore score = A2ATrustScore.builder()
                    .score(0.88)
                    .confidence(0.9)
                    .build();

            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .trustScore(score)
                    .build();

            assertNotNull(trust.getTrustScore());
            assertEquals(0.88, trust.getTrustScore().getScore(), 0.001);
        }

        @Test
        @DisplayName("Should have working toString")
        void testPeerTrustToString() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("source")
                    .targetAgentId("target")
                    .trustLevel(A2APeerTrust.TrustLevel.HIGH)
                    .verified(true)
                    .bidirectional(false)
                    .build();

            String str = trust.toString();
            assertTrue(str.contains("source"));
            assertTrue(str.contains("target"));
            assertTrue(str.contains("HIGH"));
        }
    }

    @Nested
    @DisplayName("A2AConsent Tests")
    class A2AConsentTests {

        @Test
        @DisplayName("Should create consent with builder")
        void testConsentBuilder() {
            Instant now = Instant.now();
            Instant expires = now.plus(30, ChronoUnit.DAYS);

            A2AConsent consent = A2AConsent.builder()
                    .id("consent-123")
                    .userId("user-456")
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .purpose("Data sharing for analytics")
                    .dataTypes(Arrays.asList("profile", "preferences"))
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .grantedAt(now)
                    .expiresAt(expires)
                    .legalBasis("consent")
                    .retentionPeriod("90 days")
                    .build();

            assertEquals("consent-123", consent.getId());
            assertEquals("user-456", consent.getUserId());
            assertEquals("agent-A", consent.getSourceAgentId());
            assertEquals("agent-B", consent.getTargetAgentId());
            assertEquals("Data sharing for analytics", consent.getPurpose());
            assertEquals(2, consent.getDataTypes().size());
            assertEquals(A2AConsent.ConsentStatus.GRANTED, consent.getStatus());
            assertEquals("consent", consent.getLegalBasis());
        }

        @Test
        @DisplayName("Should identify active consent")
        void testActiveConsent() {
            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .expiresAt(Instant.now().plus(1, ChronoUnit.DAYS))
                    .build();

            assertTrue(consent.isActive());
            assertFalse(consent.isExpired());
        }

        @Test
        @DisplayName("Should identify expired consent")
        void testExpiredConsent() {
            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .expiresAt(Instant.now().minus(1, ChronoUnit.DAYS))
                    .build();

            assertFalse(consent.isActive());
            assertTrue(consent.isExpired());
        }

        @Test
        @DisplayName("Should identify revoked consent")
        void testRevokedConsent() {
            Instant revokedAt = Instant.now().minus(1, ChronoUnit.HOURS);

            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.REVOKED)
                    .revokedAt(revokedAt)
                    .revocationReason("User requested deletion")
                    .build();

            assertEquals(A2AConsent.ConsentStatus.REVOKED, consent.getStatus());
            assertFalse(consent.isActive());
            assertEquals("User requested deletion", consent.getRevocationReason());
        }

        @Test
        @DisplayName("Should check data type coverage")
        void testDataTypeCoverage() {
            A2AConsent consent = A2AConsent.builder()
                    .dataTypes(Arrays.asList("profile", "preferences", "activity"))
                    .build();

            assertTrue(consent.coversDataType("profile"));
            assertTrue(consent.coversDataType("preferences"));
            assertFalse(consent.coversDataType("financial"));
        }

        @Test
        @DisplayName("Should handle wildcard data type coverage")
        void testWildcardDataTypeCoverage() {
            A2AConsent consent = A2AConsent.builder()
                    .dataTypes(Arrays.asList("*"))
                    .build();

            assertTrue(consent.coversDataType("profile"));
            assertTrue(consent.coversDataType("financial"));
            assertTrue(consent.coversDataType("anything"));
        }

        @Test
        @DisplayName("Should return false for empty data types")
        void testEmptyDataTypes() {
            A2AConsent consent = A2AConsent.builder().build();

            assertFalse(consent.coversDataType("profile"));
        }

        @Test
        @DisplayName("Should handle consent statuses")
        void testConsentStatuses() {
            for (A2AConsent.ConsentStatus status : A2AConsent.ConsentStatus.values()) {
                A2AConsent consent = A2AConsent.builder()
                        .status(status)
                        .build();
                assertEquals(status, consent.getStatus());
            }
        }

        @Test
        @DisplayName("Should handle metadata")
        void testConsentMetadata() {
            Map<String, Object> metadata = new HashMap<>();
            metadata.put("consent_version", "2.0");
            metadata.put("collection_method", "explicit_click");
            metadata.put("ip_address", "192.168.1.1");

            A2AConsent consent = A2AConsent.builder()
                    .metadata(metadata)
                    .build();

            assertNotNull(consent.getMetadata());
            assertEquals("2.0", consent.getMetadata().get("consent_version"));
        }

        @Test
        @DisplayName("Should handle consent without expiration")
        void testConsentWithoutExpiration() {
            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .build();

            assertTrue(consent.isActive());
            assertFalse(consent.isExpired());
        }

        @Test
        @DisplayName("Should have working toString")
        void testConsentToString() {
            A2AConsent consent = A2AConsent.builder()
                    .id("consent-123")
                    .userId("user-456")
                    .sourceAgentId("source")
                    .targetAgentId("target")
                    .purpose("testing")
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .build();

            String str = consent.toString();
            assertTrue(str.contains("consent-123"));
            assertTrue(str.contains("user-456"));
            assertTrue(str.contains("GRANTED"));
        }
    }

    @Nested
    @DisplayName("A2AException Tests")
    class A2AExceptionTests {

        @Test
        @DisplayName("Should create exception with message")
        void testExceptionWithMessage() {
            A2AException ex = new A2AException("Test error message");

            assertEquals("Test error message", ex.getMessage());
            assertEquals("A2A_ERROR", ex.getErrorCode());
            assertEquals(500, ex.getStatusCode());
        }

        @Test
        @DisplayName("Should create exception with message and error code")
        void testExceptionWithMessageAndErrorCode() {
            A2AException ex = new A2AException("Invalid signature", "A2A_SIGNATURE_INVALID");

            assertEquals("Invalid signature", ex.getMessage());
            assertEquals("A2A_SIGNATURE_INVALID", ex.getErrorCode());
            assertEquals(500, ex.getStatusCode());
        }

        @Test
        @DisplayName("Should create exception with full details")
        void testExceptionWithFullDetails() {
            A2AException ex = new A2AException("Not found", "A2A_NOT_FOUND", 404);

            assertEquals("Not found", ex.getMessage());
            assertEquals("A2A_NOT_FOUND", ex.getErrorCode());
            assertEquals(404, ex.getStatusCode());
        }

        @Test
        @DisplayName("Should create exception with cause")
        void testExceptionWithCause() {
            RuntimeException cause = new RuntimeException("Original error");
            A2AException ex = new A2AException("Wrapped error", cause);

            assertEquals("Wrapped error", ex.getMessage());
            assertEquals("A2A_ERROR", ex.getErrorCode());
            assertSame(cause, ex.getCause());
        }

        @Test
        @DisplayName("Should create exception with error code and cause")
        void testExceptionWithErrorCodeAndCause() {
            RuntimeException cause = new RuntimeException("Original error");
            A2AException ex = new A2AException("Signature verification failed", "A2A_SIGNATURE_VERIFY_ERROR", cause);

            assertEquals("Signature verification failed", ex.getMessage());
            assertEquals("A2A_SIGNATURE_VERIFY_ERROR", ex.getErrorCode());
            assertSame(cause, ex.getCause());
        }

        @Test
        @DisplayName("Should be throwable")
        void testExceptionIsThrowable() {
            assertThrows(A2AException.class, () -> {
                throw new A2AException("Test error", "TEST_ERROR", 500);
            });
        }
    }

    @Nested
    @DisplayName("Edge Case Tests")
    class EdgeCaseTests {

        @Test
        @DisplayName("Should handle empty skills list")
        void testEmptySkillsList() {
            A2AAgentCard card = A2AAgentCard.builder()
                    .id("card-123")
                    .skills(Arrays.asList())
                    .build();

            assertNotNull(card.getSkills());
            assertTrue(card.getSkills().isEmpty());
        }

        @Test
        @DisplayName("Should handle null values in agent card")
        void testNullValuesInAgentCard() {
            A2AAgentCard card = A2AAgentCard.builder().build();

            assertNull(card.getId());
            assertNull(card.getName());
            assertNull(card.getProvider());
            assertNull(card.getCapabilities());
            assertNull(card.getSkills());
        }

        @Test
        @DisplayName("Should handle boundary trust scores")
        void testBoundaryTrustScores() {
            A2ATrustScore zeroScore = A2ATrustScore.builder().score(0.0).build();
            A2ATrustScore maxScore = A2ATrustScore.builder().score(1.0).build();
            A2ATrustScore boundaryScore = A2ATrustScore.builder().score(0.8).build();

            assertEquals(0.0, zeroScore.getScore(), 0.001);
            assertEquals(1.0, maxScore.getScore(), 0.001);
            assertTrue(maxScore.isHighTrust());
            assertTrue(zeroScore.isLowTrust());
            assertTrue(boundaryScore.isHighTrust()); // 0.8 is exactly the high trust threshold
        }

        @Test
        @DisplayName("Should handle trust score at threshold boundaries")
        void testTrustScoreThresholdBoundaries() {
            // Test exactly at low trust boundary (0.3)
            A2ATrustScore atLowBoundary = A2ATrustScore.builder().score(0.3).build();
            assertFalse(atLowBoundary.isLowTrust()); // 0.3 is NOT low trust (< 0.3)

            // Test just below low trust boundary
            A2ATrustScore belowLowBoundary = A2ATrustScore.builder().score(0.29).build();
            assertTrue(belowLowBoundary.isLowTrust());

            // Test exactly at high trust boundary (0.8)
            A2ATrustScore atHighBoundary = A2ATrustScore.builder().score(0.8).build();
            assertTrue(atHighBoundary.isHighTrust()); // 0.8 IS high trust (>= 0.8)

            // Test just below high trust boundary
            A2ATrustScore belowHighBoundary = A2ATrustScore.builder().score(0.79).build();
            assertFalse(belowHighBoundary.isHighTrust());
        }

        @Test
        @DisplayName("Should handle peer trust with null trust score")
        void testPeerTrustWithNullTrustScore() {
            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .build();

            assertNull(trust.getTrustScore());
        }

        @Test
        @DisplayName("Should handle consent timestamps")
        void testConsentTimestamps() {
            Instant created = Instant.now().minus(10, ChronoUnit.DAYS);
            Instant granted = Instant.now().minus(5, ChronoUnit.DAYS);
            Instant expires = Instant.now().plus(25, ChronoUnit.DAYS);

            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.GRANTED)
                    .grantedAt(granted)
                    .expiresAt(expires)
                    .build();
            consent.setCreatedAt(created);
            consent.setUpdatedAt(granted);

            assertEquals(created, consent.getCreatedAt());
            assertEquals(granted, consent.getGrantedAt());
            assertEquals(expires, consent.getExpiresAt());
        }

        @Test
        @DisplayName("Should handle pending consent status")
        void testPendingConsentStatus() {
            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.PENDING)
                    .build();

            assertEquals(A2AConsent.ConsentStatus.PENDING, consent.getStatus());
            assertFalse(consent.isActive()); // Pending is not active
        }

        @Test
        @DisplayName("Should handle denied consent status")
        void testDeniedConsentStatus() {
            A2AConsent consent = A2AConsent.builder()
                    .status(A2AConsent.ConsentStatus.DENIED)
                    .build();

            assertEquals(A2AConsent.ConsentStatus.DENIED, consent.getStatus());
            assertFalse(consent.isActive()); // Denied is not active
        }

        @Test
        @DisplayName("Should handle all trust levels")
        void testAllTrustLevels() {
            A2APeerTrust.TrustLevel[] levels = A2APeerTrust.TrustLevel.values();
            assertEquals(5, levels.length);

            // Verify all expected levels exist
            assertNotNull(A2APeerTrust.TrustLevel.NONE);
            assertNotNull(A2APeerTrust.TrustLevel.LOW);
            assertNotNull(A2APeerTrust.TrustLevel.MEDIUM);
            assertNotNull(A2APeerTrust.TrustLevel.HIGH);
            assertNotNull(A2APeerTrust.TrustLevel.FULL);
        }

        @Test
        @DisplayName("Should handle verified at timestamp")
        void testVerifiedAtTimestamp() {
            Instant verifiedAt = Instant.now().minus(1, ChronoUnit.HOURS);

            A2APeerTrust trust = A2APeerTrust.builder()
                    .sourceAgentId("agent-A")
                    .targetAgentId("agent-B")
                    .verified(true)
                    .verifiedAt(verifiedAt)
                    .build();

            assertTrue(trust.isVerified());
            assertEquals(verifiedAt, trust.getVerifiedAt());
        }
    }
}
