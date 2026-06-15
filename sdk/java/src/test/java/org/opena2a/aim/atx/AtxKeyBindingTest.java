package org.opena2a.aim.atx;

import org.junit.jupiter.api.Test;

import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.PrivateKey;
import java.security.Signature;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Key-to-issuer binding + signature-path edge cases for the Java verifier,
 * exercising the v1.1 verify path (which the pinned-hex/canonicalizer test does
 * not cover). Self-signs credentials so the only variable is the binding rule.
 */
class AtxKeyBindingTest {

    private static final String ISSUER_A = "did:opena2a:authority:a.example";
    private static final String ISSUER_B = "did:opena2a:authority:b.example";
    private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-06-01T00:00:00Z"), ZoneOffset.UTC);

    private record Key(PrivateKey priv, String rawHex) {}

    private static Key genKey() throws Exception {
        KeyPair kp = KeyPairGenerator.getInstance("Ed25519").generateKeyPair();
        byte[] spki = kp.getPublic().getEncoded();
        String rawHex = HexFormat.of().formatHex(Arrays.copyOfRange(spki, spki.length - 32, spki.length));
        return new Key(kp.getPrivate(), rawHex);
    }

    private static Atx base(String version, String issuerDid, List<String> issuerChain) {
        Atx a = new Atx();
        a.atcVersion = version;
        a.agentId = "agent-1";
        a.agentDid = "did:opena2a:agent:a/agent-1";
        a.publisher = "acme";
        a.publisherDid = "did:opena2a:publisher:acme";
        a.version = "1.0.0";
        a.contentHash = "sha256:abc";
        a.buildAttestation = "sha256:def";
        a.issuerDid = issuerDid;
        a.issuerChain = issuerChain;
        a.trustLevel = 2;
        a.trustScore = 0.9;
        a.issuedAt = "2026-05-20T00:00:00Z";
        a.expiresAt = "2026-06-20T00:00:00Z";
        a.capabilities = List.of("orders:read");
        a.scanSummary = Map.of("oasbLevel", "L2");
        return a;
    }

    private static void sign(Atx atx, PrivateKey priv, String keyId) throws Exception {
        byte[] payload = "1.1".equals(atx.atcVersion)
                ? AtxCanonicalizer.canonicalPayloadV11(atx)
                : AtxCanonicalizer.canonicalPayload(atx);
        Signature s = Signature.getInstance("Ed25519");
        s.initSign(priv);
        s.update(payload);
        atx.signatures = new ArrayList<>(List.of(
                new AtxSignature(keyId, "Ed25519", Base64.getEncoder().encodeToString(s.sign()))));
    }

    private static AtxTrustAnchors anchors(List<String> trusted, List<AtxPublicKey> keys) {
        return new AtxTrustAnchors(trusted, keys, null, CLOCK);
    }

    @Test
    void rejectsCrossIssuerSignatureV10() throws Exception {
        Key a = genKey();
        Key b = genKey();
        Atx atx = base("1.0", ISSUER_A, List.of(ISSUER_A));
        sign(atx, b.priv(), ISSUER_B + "#key-1"); // A-issued, signed by B's key
        AtxVerificationResult r = new LocalAtxVerifier(anchors(
                List.of(ISSUER_A, ISSUER_B),
                List.of(new AtxPublicKey("Ed25519", a.rawHex(), ISSUER_A + "#key-1"),
                        new AtxPublicKey("Ed25519", b.rawHex(), ISSUER_B + "#key-1")))).verify(atx);
        assertFalse(r.valid());
        assertEquals(RejectCategory.SIGNATURE_INVALID, r.rejectCategory());
    }

    @Test
    void rejectsCrossIssuerSignatureV11() throws Exception {
        Key a = genKey();
        Key b = genKey();
        Atx atx = base("1.1", ISSUER_A, List.of(ISSUER_A)); // B not in chain
        sign(atx, b.priv(), ISSUER_B + "#key-1");
        AtxVerificationResult r = new LocalAtxVerifier(anchors(
                List.of(ISSUER_A, ISSUER_B),
                List.of(new AtxPublicKey("Ed25519", a.rawHex(), ISSUER_A + "#key-1"),
                        new AtxPublicKey("Ed25519", b.rawHex(), ISSUER_B + "#key-1")))).verify(atx);
        assertFalse(r.valid());
        assertEquals(RejectCategory.SIGNATURE_INVALID, r.rejectCategory());
    }

    @Test
    void acceptsV11CrossOrgCosignatureInSignedChain() throws Exception {
        Key b = genKey();
        // issuer A, but B is a cosigning authority in the (signed) issuerChain.
        Atx atx = base("1.1", ISSUER_A, List.of(ISSUER_A, ISSUER_B));
        sign(atx, b.priv(), ISSUER_B + "#key-1");
        AtxVerificationResult r = new LocalAtxVerifier(anchors(
                List.of(ISSUER_A, ISSUER_B),
                List.of(new AtxPublicKey("Ed25519", b.rawHex(), ISSUER_B + "#key-1")))).verify(atx);
        assertTrue(r.valid(), "expected ACCEPT: " + r.reason());
    }

    @Test
    void unboundKeyWithoutDidUrlStaysEligible() throws Exception {
        Key a = genKey();
        Atx atx = base("1.0", ISSUER_A, List.of(ISSUER_A));
        sign(atx, a.priv(), "legacy-key"); // no '#': unbound
        AtxVerificationResult r = new LocalAtxVerifier(anchors(
                List.of(ISSUER_A),
                List.of(new AtxPublicKey("Ed25519", a.rawHex(), "legacy-key")))).verify(atx);
        assertTrue(r.valid(), "expected ACCEPT for unbound key (back-compat): " + r.reason());
    }

    @Test
    void rejectsUnknownSignatureAlgorithm() throws Exception {
        Key a = genKey();
        Atx atx = base("1.0", ISSUER_A, List.of(ISSUER_A));
        sign(atx, a.priv(), ISSUER_A + "#key-1");
        // append a signature with an unsupported algorithm — must reject, not ignore.
        atx.signatures.add(new AtxSignature("x", "Frobnicate-9000", "AAAA"));
        AtxVerificationResult r = new LocalAtxVerifier(anchors(
                List.of(ISSUER_A),
                List.of(new AtxPublicKey("Ed25519", a.rawHex(), ISSUER_A + "#key-1")))).verify(atx);
        assertFalse(r.valid());
        assertEquals(RejectCategory.SIGNATURE_INVALID, r.rejectCategory());
    }
}
