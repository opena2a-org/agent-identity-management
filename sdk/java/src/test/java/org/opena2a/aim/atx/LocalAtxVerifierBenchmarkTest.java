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
import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Warm-cache verification benchmark for the Java SDK (#292 acceptance / #317).
 *
 * Mirrors the TypeScript SDK's benchmark: with trust anchors and the revocation list
 * already cached, an offline verify should land in the sub-millisecond-to-low-millisecond
 * range. We assert a generous median bound (robust to CI variance) and print the measured
 * median / p99 so the figure is visible without making the test flaky.
 */
class LocalAtxVerifierBenchmarkTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final int WARMUP = 2_000;
    private static final int MEASURE = 10_000;
    /** Generous upper bound for the warm-cache median; real figure is ~tens of µs. */
    private static final long MEDIAN_BUDGET_NANOS = 2_000_000L; // 2 ms

    @Test
    void warmCacheVerifyIsSubMillisecondMedian() throws Exception {
        JsonNode fixture;
        try (InputStream in = getClass().getResourceAsStream("/atx-fixtures/baseline-valid.json")) {
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

        // Warm cache: a fresh, already-loaded revocation list (no network on the hot path).
        CrlCache cache = CrlCache.builder(() -> new AtxTrustAnchors.Crl(List.of()))
                .ttl(Duration.ofMinutes(5))
                .build();
        cache.refreshNow();
        RefreshingAtxVerifier verifier = new RefreshingAtxVerifier(issuers, keys, clock, cache);

        // Warm up the JIT and key/canonicalization paths.
        for (int i = 0; i < WARMUP; i++) {
            assertTrue(verifier.verify(atx).valid());
        }

        long[] samples = new long[MEASURE];
        for (int i = 0; i < MEASURE; i++) {
            long t0 = System.nanoTime();
            AtxVerificationResult r = verifier.verify(atx);
            samples[i] = System.nanoTime() - t0;
            if (!r.valid()) {
                throw new AssertionError("benchmark credential failed to verify");
            }
        }

        Arrays.sort(samples);
        long medianNanos = samples[MEASURE / 2];
        long p99Nanos = samples[(int) (MEASURE * 0.99)];
        System.out.printf(
                "[atx-bench] warm-cache verify: median=%.1fµs p99=%.1fµs (n=%d)%n",
                medianNanos / 1000.0, p99Nanos / 1000.0, MEASURE);

        assertTrue(
                medianNanos < MEDIAN_BUDGET_NANOS,
                "warm-cache verify median " + (medianNanos / 1000.0) + "µs exceeded budget "
                        + (MEDIAN_BUDGET_NANOS / 1000.0) + "µs");
    }
}
