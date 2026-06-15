package org.opena2a.aim.atx;

import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Byte-agreement gate: the Java canonicalizer must reproduce the exact bytes the
 * TS/Go/Python verifiers produce. The pinned v1.1 hex comes from
 * atx-conformance/jcs-vectors (via @opena2a/atx-verify atx.test.ts). If Java's JCS
 * or projection is off by a byte, these fail — proving interop before any signing.
 */
class AtxCanonicalizerTest {

    // Pinned in atx-conformance/jcs-vectors/vectors/01-baseline.json; copied verbatim
    // from @opena2a/atx-verify src/atx.test.ts V11_BASELINE_CANONICAL_HEX.
    private static final String V11_BASELINE_CANONICAL_HEX =
            "7b226167656e74446964223a226469643a6f70656e6132613a6167656e743a6167656e745f636f6e666f726d616e63655f746573745f303031222c226167656e744964223a226167656e745f636f6e666f726d616e63655f746573745f303031222c2261746356657273696f6e223a22312e31222c226265686176696f72616c50726f66696c65223a7b22636865636b73756d223a227368613235363a676869373839222c2267656e6572617465644174223a22323032362d30352d31395430303a30303a30305a222c226f62736572766174696f6e44617973223a31347d2c226275696c644174746573746174696f6e223a2268747470733a2f2f736c73612e6465762f70726f76656e616e63652f7631236f70656e6132612d636f6e666f726d616e6365222c226361706162696c6974696573223a5b22726561643a7075626c6963222c2277726974653a6f776e6564225d2c22636f6e74656e7448617368223a2230303030313131313232323233333333343434343535353536363636373737373838383839393939616161616262626263636363646464646565656566666666222c22657870697265734174223a22323039392d31322d33315432333a35393a35395a222c226973737565644174223a22323032362d30352d32335430303a30303a30305a222c22697373756572436861696e223a5b226469643a6f70656e6132613a617574686f726974793a6f70656e6132612e6f72672d726f6f74222c226469643a6f70656e6132613a617574686f726974793a6f70656e6132612e6f7267225d2c22697373756572446964223a226469643a6f70656e6132613a617574686f726974793a6f70656e6132612e6f7267222c227075626c6973686572223a226f70656e6132612d636f6e666f726d616e6365222c227075626c6973686572446964223a226469643a6f70656e6132613a7075626c69736865723a6f70656e6132612d636f6e666f726d616e6365222c227363616e53756d6d617279223a7b22637269746963616c46696e64696e6773223a302c2263727970746f5365727665223a226e6f2d7765616b2d63727970746f222c226869676846696e64696e6773223a302c22686d61223a22706173736564222c226f6173624c6576656c223a224c31222c227365637265746c657373223a22636c65616e227d2c2274727573744c6576656c223a342c22747275737453636f7265223a2238372e353030303030222c2276657273696f6e223a22312e302e30227d";

    /** The credential that projects to the jcs-vectors baseline vector. */
    private static Atx v11Baseline() {
        Atx a = new Atx();
        a.atcVersion = "1.1";
        a.agentId = "agent_conformance_test_001";
        a.agentDid = "did:opena2a:agent:agent_conformance_test_001";
        a.publisher = "opena2a-conformance";
        a.publisherDid = "did:opena2a:publisher:opena2a-conformance";
        a.version = "1.0.0";
        a.contentHash = "0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff";
        a.buildAttestation = "https://slsa.dev/provenance/v1#opena2a-conformance";
        a.issuerDid = "did:opena2a:authority:opena2a.org";
        a.issuerChain = List.of("did:opena2a:authority:opena2a.org-root", "did:opena2a:authority:opena2a.org");
        a.trustLevel = 4;
        a.trustScore = 87.5;
        a.issuedAt = "2026-05-23T00:00:00Z";
        a.expiresAt = "2099-12-31T23:59:59Z";
        a.capabilities = List.of("read:public", "write:owned");
        a.behavioralProfile = Map.of(
                "checksum", "sha256:ghi789",
                "generatedAt", "2026-05-19T00:00:00Z",
                "observationDays", 14);
        a.scanSummary = Map.of(
                "hma", "passed",
                "criticalFindings", 0,
                "highFindings", 0,
                "secretless", "clean",
                "cryptoServe", "no-weak-crypto",
                "oasbLevel", "L1");
        return a;
    }

    @Test
    void reproducesPinnedV11BaselineCanonicalBytes() {
        String hex = HexFormat.of().formatHex(AtxCanonicalizer.canonicalPayloadV11(v11Baseline()));
        assertEquals(V11_BASELINE_CANONICAL_HEX, hex,
                "v1.1 JCS(TBS) bytes diverge from the cross-language pinned vector");
    }

    @Test
    void v10PipeStringMatchesGoPythonOrder() {
        Atx a = new Atx();
        a.atcVersion = "1.0";
        a.agentId = "aim_orders_reader";
        a.agentDid = "did:opena2a:agent:acme/orders-reader";
        a.version = "1.0.0";
        a.contentHash = "sha256:abc123";
        a.buildAttestation = "sha256:def456";
        a.issuerDid = "did:opena2a:authority:opena2a.org";
        a.trustLevel = 4;
        a.trustScore = 0.5;
        a.issuedAt = "2026-05-25T00:00:00Z";
        a.expiresAt = "2026-06-08T00:00:00Z";

        String payload = new String(AtxCanonicalizer.canonicalPayload(a), StandardCharsets.UTF_8);
        assertEquals(
                "aim_orders_reader|did:opena2a:agent:acme/orders-reader|1.0.0|sha256:abc123|"
                        + "sha256:def456|did:opena2a:authority:opena2a.org|4|0.500000|"
                        + "2026-05-25T00:00:00Z|2026-06-08T00:00:00Z|1.0",
                payload);
    }

    @Test
    void trustScoreRoundsHalfToEvenLikeGoAndPython() {
        // 0.1234565 -> "0.123456" with round-half-even (Go %.6f, Python :.6f, JS
        // toFixed). Java's String.format("%.6f") would give "0.123457" (HALF_UP)
        // and false-reject. Guards the signer-parity rounding fix.
        Atx a = new Atx();
        a.atcVersion = "1.0";
        a.agentId = "a";
        a.agentDid = "d";
        a.version = "1.0.0";
        a.contentHash = "h";
        a.issuerDid = "did:opena2a:authority:opena2a.org";
        a.trustLevel = 1;
        a.trustScore = 0.1234565;
        a.issuedAt = "2026-05-25T00:00:00Z";
        a.expiresAt = "2026-06-08T00:00:00Z";
        String payload = new String(AtxCanonicalizer.canonicalPayload(a), StandardCharsets.UTF_8);
        assertTrue(payload.contains("|0.123456|"), "expected round-half-even 0.123456, got: " + payload);
    }

    @Test
    void normalizesRfc3339ToSecondsUtcZ() {
        assertEquals("2026-06-08T00:00:00Z", AtxCanonicalizer.normalizeRfc3339("2026-06-08T00:00:00.123Z"));
        assertEquals("2026-06-08T00:00:00Z", AtxCanonicalizer.normalizeRfc3339("2026-06-08T02:00:00+02:00"));
    }
}
