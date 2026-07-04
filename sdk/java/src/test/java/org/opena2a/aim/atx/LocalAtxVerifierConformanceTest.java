package org.opena2a.aim.atx;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

import java.io.InputStream;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
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
 * fixtures' reasonContains — that is the reference verifiers' own wording.
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
    })
    void fixture(String file, String expectedResult, String expectedRejectCategory) throws Exception {
        JsonNode fixture;
        try (InputStream in = getClass().getResourceAsStream("/atx-fixtures/" + file.trim())) {
            assertNotNull(in, "fixture not found: " + file);
            fixture = MAPPER.readTree(in);
        }

        JsonNode vs = fixture.get("verifierState");
        Atx atx = MAPPER.treeToValue(fixture.get("atx"), Atx.class);

        AtxVerificationResult result = new LocalAtxVerifier(anchorsFrom(vs)).verify(atx);

        if ("ACCEPT".equals(expectedResult)) {
            assertEquals(true, result.valid(), "expected ACCEPT, got: " + result.reason());
        } else {
            assertEquals(false, result.valid(), "expected REJECT but verifier accepted");
            if (expectedRejectCategory != null && !expectedRejectCategory.isBlank()) {
                assertEquals(RejectCategory.valueOf(expectedRejectCategory.trim()), result.rejectCategory());
            }
        }
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
}
