package org.opena2a.aim.atx;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * A CRL that is present but whose entry list is null must be treated as MALFORMED,
 * never as empty.
 *
 * <p>The difference is the whole control. {@code Crl(entries=null)} is the shape a
 * decoder produces when the feed carries its list under a different key — which is
 * exactly what the AIM server does today: it serves {@code revocations}, and
 * {@link AtxTrustAnchors.Crl} binds {@code entries}. Reading that as "nothing is
 * revoked" means every revoked credential verifies, with no error and no log line.
 *
 * <p>These three cases pin the boundary the guard must sit on, because two of the
 * three states look identical in the type system and must not behave identically:
 *
 * <ul>
 *   <li>absent CRL (null) — the stale-cache path; the caller's StalePolicy has already
 *       decided, so the verifier proceeds. Must stay accepting.</li>
 *   <li>empty CRL (non-null, zero entries) — a legitimate "nothing is revoked right
 *       now". Must stay accepting.</li>
 *   <li>malformed CRL (non-null, null entries) — must reject.</li>
 * </ul>
 *
 * <p>Reverting the guard in {@link LocalAtxVerifier} leaves the first two green and
 * flips only the third, which is what makes this suite worth having.
 */
class LocalAtxVerifierMalformedCrlTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final JsonFactory JSON = new JsonFactory();

    /** A credential the conformance suite pins as valid, so any rejection here is the CRL. */
    private static final String FIXTURE = "/atx-fixtures/baseline-valid.json";

    @Test
    void absentCrlStillVerifies() throws Exception {
        AtxVerificationResult r = verifyWithCrl(null);
        assertTrue(r.valid(),
                "an absent CRL is the stale-cache path and must not reject here; got: " + r.reason());
    }

    @Test
    void emptyCrlStillVerifies() throws Exception {
        AtxVerificationResult r = verifyWithCrl(new AtxTrustAnchors.Crl(List.of()));
        assertTrue(r.valid(),
                "an empty CRL means nothing is revoked and must verify; got: " + r.reason());
    }

    @Test
    void malformedCrlWithNullEntriesIsRejected() throws Exception {
        AtxVerificationResult r = verifyWithCrl(new AtxTrustAnchors.Crl(null));

        assertEquals(false, r.valid(),
                "a CRL with null entries was treated as empty — every revoked credential "
                        + "would verify with no error surfaced anywhere");
        assertEquals(RejectCategory.REVOKED, r.rejectCategory());
        assertTrue(r.reason().contains("malformed"),
                "the reason must say the list is malformed rather than implying this "
                        + "credential was revoked; got: " + r.reason());
    }

    /** Loads the pinned fixture and verifies its credential against {@code crl}. */
    private static AtxVerificationResult verifyWithCrl(AtxTrustAnchors.Crl crl) throws Exception {
        byte[] fixture = readFixture();
        JsonNode root = MAPPER.readTree(fixture);
        JsonNode vs = root.get("verifierState");

        Clock clock = Clock.fixed(Instant.parse(vs.get("clockRfc3339").asText()), ZoneOffset.UTC);

        List<String> trustedIssuers = new ArrayList<>();
        vs.get("trustedIssuers").forEach(n -> trustedIssuers.add(n.asText()));

        List<AtxPublicKey> publicKeys = new ArrayList<>();
        for (JsonNode k : vs.get("publicKeys")) {
            publicKeys.add(new AtxPublicKey(
                    k.get("algorithm").asText(),
                    k.get("publicKeyHex").asText(),
                    k.hasNonNull("keyId") ? k.get("keyId").asText() : null));
        }

        LocalAtxVerifier verifier =
                new LocalAtxVerifier(new AtxTrustAnchors(trustedIssuers, publicKeys, crl, clock));
        return verifier.verify(new String(rawAtxBytes(fixture), java.nio.charset.StandardCharsets.UTF_8));
    }

    private static byte[] readFixture() throws IOException {
        try (InputStream in = LocalAtxVerifierMalformedCrlTest.class.getResourceAsStream(FIXTURE)) {
            assertNotNull(in, "fixture not found on the test classpath: " + FIXTURE);
            return in.readAllBytes();
        }
    }

    /** Verbatim credential bytes, so strict-parse behaviour matches the conformance harness. */
    private static byte[] rawAtxBytes(byte[] fixtureBytes) throws IOException {
        try (JsonParser p = JSON.createParser(fixtureBytes)) {
            if (p.nextToken() != JsonToken.START_OBJECT) {
                throw new IOException("fixture is not a JSON object");
            }
            while (p.nextToken() != JsonToken.END_OBJECT) {
                String field = p.currentName();
                JsonToken valueStart = p.nextToken();
                if ("atx".equals(field)) {
                    long start = p.currentTokenLocation().getByteOffset();
                    p.skipChildren();
                    long end = p.currentLocation().getByteOffset();
                    return Arrays.copyOfRange(fixtureBytes, (int) start, (int) end);
                }
                if (valueStart == JsonToken.START_OBJECT || valueStart == JsonToken.START_ARRAY) {
                    p.skipChildren();
                }
            }
        }
        throw new IOException("fixture has no atx member");
    }
}
