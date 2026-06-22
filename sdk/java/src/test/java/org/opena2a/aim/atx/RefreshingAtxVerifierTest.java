package org.opena2a.aim.atx;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.InputStream;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Integration: {@link RefreshingAtxVerifier} over a real signed credential (the
 * baseline-valid conformance fixture) with an async-refreshed {@link CrlCache}. Proves
 * the cache feeds the conformance-locked verifier without changing its accept/reject
 * behavior, and that stale-cache policy is applied correctly.
 */
class RefreshingAtxVerifierTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private record Fixture(Atx atx, List<String> issuers, List<AtxPublicKey> keys, Clock clock) {}

    private static Fixture baseline() throws Exception {
        JsonNode fixture;
        try (InputStream in = RefreshingAtxVerifierTest.class.getResourceAsStream("/atx-fixtures/baseline-valid.json")) {
            assertNotNull(in, "baseline-valid.json fixture not found");
            fixture = MAPPER.readTree(in);
        }
        JsonNode vs = fixture.get("verifierState");
        Atx atx = MAPPER.treeToValue(fixture.get("atx"), Atx.class);

        Clock clock = Clock.fixed(Instant.parse(vs.get("clockRfc3339").asText()), ZoneOffset.UTC);
        List<String> issuers = new ArrayList<>();
        vs.get("trustedIssuers").forEach(n -> issuers.add(n.asText()));
        List<AtxPublicKey> keys = new ArrayList<>();
        for (JsonNode k : vs.get("publicKeys")) {
            keys.add(new AtxPublicKey(
                    k.get("algorithm").asText(),
                    k.get("publicKeyHex").asText(),
                    k.hasNonNull("keyId") ? k.get("keyId").asText() : null));
        }
        return new Fixture(atx, issuers, keys, clock);
    }

    private static AtxTrustAnchors.Crl crlFor(String agentId) {
        return new AtxTrustAnchors.Crl(List.of(new AtxTrustAnchors.Crl.Entry(agentId, "key compromise")));
    }

    @Test
    void rejectsCredentialListedInFreshCachedCrl() throws Exception {
        Fixture f = baseline();
        CrlCache cache = CrlCache.builder(() -> crlFor(f.atx().agentId))
                .ttl(Duration.ofMillis(1000))
                .build();
        cache.refreshNow();

        RefreshingAtxVerifier verifier = new RefreshingAtxVerifier(f.issuers(), f.keys(), f.clock(), cache);
        AtxVerificationResult result = verifier.verify(f.atx());

        assertFalse(result.valid());
        assertEquals(RejectCategory.REVOKED, result.rejectCategory());
    }

    @Test
    void verifiesWhenAgentAbsentFromFreshCachedCrl() throws Exception {
        Fixture f = baseline();
        CrlCache cache = CrlCache.builder(() -> crlFor("some-other-agent"))
                .ttl(Duration.ofMillis(1000))
                .build();
        cache.refreshNow();

        RefreshingAtxVerifier verifier = new RefreshingAtxVerifier(f.issuers(), f.keys(), f.clock(), cache);
        assertTrue(verifier.verify(f.atx()).valid());
    }

    @Test
    void softOpensOnRevocationWhenCacheIsStale() throws Exception {
        Fixture f = baseline();
        AtomicLong clock = new AtomicLong(0);
        CrlCache cache = CrlCache.builder(() -> crlFor(f.atx().agentId)) // would revoke, but goes stale
                .ttl(Duration.ofMillis(1000))
                .nowMillis(clock::get)
                .build();
        cache.refreshNow();
        clock.set(1001); // stale

        RefreshingAtxVerifier verifier = new RefreshingAtxVerifier(f.issuers(), f.keys(), f.clock(), cache);
        // Revocation not enforced on a stale list; signature/expiry/issuer stay hard.
        assertTrue(verifier.verify(f.atx()).valid());
    }

    @Test
    void failsClosedOnStaleCacheWhenRejectPolicy() throws Exception {
        Fixture f = baseline();
        AtomicLong clock = new AtomicLong(0);
        CrlCache cache = CrlCache.builder(() -> new AtxTrustAnchors.Crl(List.of())) // empty: no real revocations
                .ttl(Duration.ofMillis(1000))
                .onStale(CrlCache.StalePolicy.REJECT)
                .nowMillis(clock::get)
                .build();
        cache.refreshNow();
        clock.set(1001); // stale

        RefreshingAtxVerifier verifier = new RefreshingAtxVerifier(f.issuers(), f.keys(), f.clock(), cache);
        AtxVerificationResult result = verifier.verify(f.atx());

        assertFalse(result.valid());
        assertEquals(RejectCategory.REVOKED, result.rejectCategory());
        assertTrue(result.reason().toLowerCase().contains("stale"));
    }
}
