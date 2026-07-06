package org.opena2a.aim.atx;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

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

/**
 * Conformance gate: run {@link LocalAtxVerifier} against the OpenA2A ATX
 * conformance fixtures (verbatim copies from atx-conformance/fixtures/, including
 * their pinned Ed25519 signatures and issuer public keys). Proves this verifier
 * accepts/rejects exactly the credentials the Go and Python reference verifiers do.
 *
 * <p>We assert the machine contract (verifyResult + rejectCategory), NOT the
 * fixtures' reasonContains — that is the reference verifiers' own wording. Where
 * the reference verifiers report {@code PARSE_ERROR} (strict-parse rejections),
 * this SDK reports {@link RejectCategory#MALFORMED} — the SDK RejectCategory
 * union (shared with {@code @opena2a/atx-verify}) has no PARSE_ERROR, and
 * MALFORMED is its structural-parse category — so those fixtures are pinned to
 * MALFORMED here.
 */
class LocalAtxVerifierConformanceTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @ParameterizedTest(name = "{0} -> {1}{2}")
    @CsvSource({
            "baseline-valid.json,                  ACCEPT, ",
            "baseline-valid-hybrid.json,           ACCEPT, ",
            "threshold-2of3-cosignature.json,      ACCEPT, ",
            "tampered-signature.json,              REJECT, SIGNATURE_INVALID",
            "expired.json,                         REJECT, EXPIRED",
            "revoked.json,                         REJECT, REVOKED",
            "wrong-issuer.json,                    REJECT, UNTRUSTED_ISSUER",
            "malformed-schema.json,                REJECT, UNSUPPORTED_VERSION",
            "cross-issuer-key.json,                REJECT, SIGNATURE_INVALID",
            "v1_1-baseline-valid.json,             ACCEPT, ",
            "v1_1-baseline-valid-hybrid.json,      ACCEPT, ",
            "v1_1-declared-purpose-valid.json,     ACCEPT, ",
            "v1_1-tampered-capabilities.json,      REJECT, SIGNATURE_INVALID",
            "v1_1-tampered-declared-purpose.json,  REJECT, SIGNATURE_INVALID",
            "v1_1-cross-issuer-key.json,           REJECT, SIGNATURE_INVALID",
            "v1_1-declared-purpose-empty-whitespace.json, ACCEPT, ",
            "v1_1-declared-purpose-array-injected.json,   REJECT, SIGNATURE_INVALID",
            "v1_1-declared-purpose-string-injected.json,  REJECT, SIGNATURE_INVALID",
            // Strict-parse rejections. The suite pins these PARSE_ERROR; this SDK
            // has no such category and reports MALFORMED (see class javadoc).
            "v1_1-duplicate-purpose-member.json,          REJECT, MALFORMED",
            "v1_1-case-variant-member.json,               REJECT, MALFORMED",
    })
    void fixture(String file, String expectedResult, String expectedRejectCategory) throws Exception {
        byte[] fixtureBytes;
        try (InputStream in = getClass().getResourceAsStream("/atx-fixtures/" + file.trim())) {
            assertNotNull(in, "fixture not found: " + file);
            fixtureBytes = in.readAllBytes();
        }

        JsonNode fixture = MAPPER.readTree(fixtureBytes);
        JsonNode vs = fixture.get("verifierState");

        // Verify from the RAW credential bytes so the strict parse (duplicate /
        // fold-colliding members at any depth) runs before any field is
        // interpreted — the object-taking verify(Atx) overload cannot see members
        // a lenient databind parse has already collapsed. The fixture wrapper
        // itself stays lenient; only the atx credential is strict-parsed.
        byte[] atxBytes = rawAtxBytes(fixtureBytes);
        AtxVerificationResult result = new LocalAtxVerifier(anchorsFrom(vs)).verify(atxBytes);

        if ("ACCEPT".equals(expectedResult)) {
            assertEquals(true, result.valid(), "expected ACCEPT, got: " + result.reason());
        } else {
            assertEquals(false, result.valid(), "expected REJECT but verifier accepted");
            if (expectedRejectCategory != null && !expectedRejectCategory.isBlank()) {
                assertEquals(RejectCategory.valueOf(expectedRejectCategory.trim()), result.rejectCategory());
            }
        }
    }

    /**
     * The raw-bytes entry point must reject every non-credential document as
     * MALFORMED — never throw. Covers the JSON literal {@code null} (Jackson maps
     * it to a Java null), bare scalars, a top-level array, empty/whitespace input,
     * and null arguments.
     */
    @Test
    void degenerateCredentialsRejectMalformed() {
        LocalAtxVerifier v = new LocalAtxVerifier(
                new AtxTrustAnchors(List.of(), List.of(), null, Clock.systemUTC()));
        for (String bad : new String[] {"null", "5", "\"x\"", "true", "[]", "", "   "}) {
            AtxVerificationResult r = v.verify(bad);
            assertEquals(false, r.valid(), "expected reject for: [" + bad + "]");
            assertEquals(RejectCategory.MALFORMED, r.rejectCategory(), "expected MALFORMED for: [" + bad + "]");
        }
        assertEquals(RejectCategory.MALFORMED, v.verify((String) null).rejectCategory());
        assertEquals(RejectCategory.MALFORMED, v.verify((byte[]) null).rejectCategory());
    }

    private static AtxTrustAnchors anchorsFrom(JsonNode vs) {
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

        AtxTrustAnchors.Crl crl = null;
        JsonNode crlNode = vs.get("crl");
        if (crlNode != null && crlNode.hasNonNull("entries")) {
            List<AtxTrustAnchors.Crl.Entry> entries = new ArrayList<>();
            for (JsonNode e : crlNode.get("entries")) {
                entries.add(new AtxTrustAnchors.Crl.Entry(
                        e.get("agentId").asText(),
                        e.hasNonNull("reason") ? e.get("reason").asText() : null));
            }
            crl = new AtxTrustAnchors.Crl(entries);
        }

        return new AtxTrustAnchors(trustedIssuers, publicKeys, crl, clock);
    }

    /**
     * Extracts the verbatim bytes of the fixture's {@code atx} credential value,
     * preserving any duplicate / case-variant members that a tree parse would
     * collapse. Uses the streaming parser's byte offsets: the slice runs from the
     * credential's opening brace to just past its matching close.
     */
    private static final JsonFactory JSON = new JsonFactory();

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
