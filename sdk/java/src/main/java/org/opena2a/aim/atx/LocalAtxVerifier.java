package org.opena2a.aim.atx;

import com.fasterxml.jackson.core.StreamReadFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.json.JsonMapper;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.PublicKey;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.time.Clock;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * Local, offline ATX verifier. Cryptographically real (Ed25519) and interoperable
 * with the conformance fixtures; trust anchors are injected. Mirrors the
 * {@code @opena2a/atx-verify} (TS), {@code pkg/atcverify} (Go), and Python
 * reference verifiers, including key-to-issuer binding.
 *
 * <p>Scope: Ed25519 is verified fully. ML-DSA-65 presence is recorded but
 * verification is delegated (matches the TS/Python reference verifiers). A
 * production deployment wires the post-quantum half and the live
 * trusted-issuer/CRL anchors.
 */
public final class LocalAtxVerifier {

    private static final String SUPPORTED_V10 = "1.0";
    private static final String SUPPORTED_V11 = "1.1";
    private static final byte[] ED25519_SPKI_PREFIX = hexToBytes("302a300506032b6570032100");

    /**
     * Credential deserializer with strict duplicate detection enabled as
     * defense-in-depth. {@link AtxStrictParse#firstDuplicateMember} already
     * rejects exact AND fold-colliding duplicates ahead of this parse; this
     * feature additionally fails an exact-name duplicate at the databind layer,
     * so a bug in the pre-scan could not let one through.
     */
    private static final ObjectMapper CREDENTIAL_MAPPER = JsonMapper.builder()
            .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
            .build();

    private final AtxTrustAnchors anchors;

    public LocalAtxVerifier(AtxTrustAnchors anchors) {
        this.anchors = anchors;
    }

    /**
     * Verifies a credential from its raw JSON bytes, applying the strict parse
     * (reject a duplicate object member at any depth, folded — see
     * {@link AtxStrictParse}) before interpreting any field, then delegating to
     * {@link #verify(Atx)}. This is the entry point wire consumers should use:
     * the object-taking overload cannot see duplicate members a lenient
     * databind parse has already collapsed. A duplicate or otherwise unparseable
     * credential rejects as {@link RejectCategory#MALFORMED} (the SDK's
     * structural-parse category; the reference verifiers call it PARSE_ERROR),
     * with a reason naming the duplicate member.
     */
    public AtxVerificationResult verify(byte[] credentialJson) {
        if (credentialJson == null) {
            return AtxVerificationResult.reject(RejectCategory.MALFORMED, "credential is null");
        }
        Atx atx;
        try {
            String dup = AtxStrictParse.firstDuplicateMember(credentialJson);
            if (dup != null) {
                return AtxVerificationResult.reject(
                        RejectCategory.MALFORMED,
                        "credential contains duplicate member \"" + dup
                                + "\" (strict parse: the ATX credential is a signed body; "
                                + "RFC 8259 §4 duplicate names are parser-divergent)");
            }
            atx = CREDENTIAL_MAPPER.readValue(credentialJson, Atx.class);
        } catch (IOException e) {
            return AtxVerificationResult.reject(
                    RejectCategory.MALFORMED, "credential is not valid ATX JSON: " + e.getMessage());
        }
        // A bare JSON `null` (or any document Jackson maps to a null Atx) is not a
        // credential — reject rather than NPE in verify(Atx).
        if (atx == null) {
            return AtxVerificationResult.reject(
                    RejectCategory.MALFORMED, "credential is not valid ATX JSON: document is JSON null");
        }
        return verify(atx);
    }

    /** Verifies a credential from its raw JSON string. See {@link #verify(byte[])}. */
    public AtxVerificationResult verify(String credentialJson) {
        if (credentialJson == null) {
            return AtxVerificationResult.reject(RejectCategory.MALFORMED, "credential is null");
        }
        return verify(credentialJson.getBytes(StandardCharsets.UTF_8));
    }

    public AtxVerificationResult verify(Atx atx) {
        Clock clock = anchors.clock() != null ? anchors.clock() : Clock.systemUTC();
        Instant now = clock.instant();

        // Step 1: schema version.
        if (!SUPPORTED_V10.equals(atx.atcVersion) && !SUPPORTED_V11.equals(atx.atcVersion)) {
            return AtxVerificationResult.reject(
                    RejectCategory.UNSUPPORTED_VERSION, "unsupported atxVersion " + atx.atcVersion);
        }
        boolean isV11 = SUPPORTED_V11.equals(atx.atcVersion);

        // Step 2: expiry.
        Instant expires;
        try {
            expires = OffsetDateTime.parse(atx.expiresAt).toInstant();
        } catch (Exception e) {
            return AtxVerificationResult.reject(RejectCategory.MALFORMED, "expiresAt is not a valid timestamp");
        }
        if (now.isAfter(expires)) {
            return AtxVerificationResult.reject(
                    RejectCategory.EXPIRED, "expired at " + AtxCanonicalizer.normalizeRfc3339(atx.expiresAt));
        }

        // Step 3: revocation (ATX field + federated CRL).
        if (atx.revoked) {
            return AtxVerificationResult.reject(RejectCategory.REVOKED, "credential revoked field is true");
        }
        // A CRL that is present but whose entry list is null is MALFORMED, not empty.
        // Reading it as "nothing is revoked" is a silent fail-open: it is the shape a
        // decoder produces when the feed's JSON carries the list under a different key,
        // so the one case that most needs rejecting looks identical to a clean list.
        // Absent (null) CRL is a different thing — that is the stale-cache path, and the
        // caller's StalePolicy has already decided it before reaching here.
        if (anchors.crl() != null && anchors.crl().entries() == null) {
            return AtxVerificationResult.reject(
                    RejectCategory.REVOKED, "revocation list is malformed (null entries); refusing to treat as empty");
        }
        if (anchors.crl() != null) {
            for (AtxTrustAnchors.Crl.Entry entry : anchors.crl().entries()) {
                if (entry.agentId() != null && entry.agentId().equals(atx.agentId)) {
                    return AtxVerificationResult.reject(
                            RejectCategory.REVOKED, "agent appears on CRL: " + ns(entry.reason()));
                }
            }
        }

        // Step 4: issuer trust.
        if (anchors.trustedIssuers() == null || !anchors.trustedIssuers().contains(atx.issuerDid)) {
            return AtxVerificationResult.reject(
                    RejectCategory.UNTRUSTED_ISSUER, "issuer DID " + atx.issuerDid + " is not trusted");
        }

        // Step 5: signature verification (Ed25519 fully; ML-DSA-65 presence recorded).
        byte[] payload;
        try {
            payload = isV11 ? AtxCanonicalizer.canonicalPayloadV11(atx) : AtxCanonicalizer.canonicalPayload(atx);
        } catch (Exception e) {
            return AtxVerificationResult.reject(
                    RejectCategory.MALFORMED,
                    (isV11 ? "v1.1 canonicalization failed: " : "canonicalization failed: ") + e.getMessage());
        }

        // Key-to-issuer binding: a key may verify a signature for this credential
        // only if its keyId controller DID is one of the credential's authorities.
        Set<String> authoritySet = authoritySetFor(atx, isV11);
        List<PublicKey> edKeys = new ArrayList<>();
        if (anchors.publicKeys() != null) {
            for (AtxPublicKey k : anchors.publicKeys()) {
                if (!"Ed25519".equals(k.algorithm()) || !keyEligible(k.keyId(), authoritySet)) {
                    continue;
                }
                PublicKey pk = ed25519FromRawHex(k.publicKeyHex());
                if (pk != null) {
                    edKeys.add(pk);
                }
            }
        }

        boolean edVerified = false;
        boolean mldsaPresent = false;
        if (atx.signatures != null) {
            for (AtxSignature sig : atx.signatures) {
                if ("Ed25519".equals(sig.algorithm())) {
                    byte[] sigBytes;
                    try {
                        sigBytes = Base64.getDecoder().decode(sig.value());
                    } catch (Exception e) {
                        return AtxVerificationResult.reject(
                                RejectCategory.SIGNATURE_INVALID,
                                "signature " + ns(sig.keyId()) + " has invalid base64");
                    }
                    boolean ok = false;
                    for (PublicKey key : edKeys) {
                        if (ed25519Verify(key, payload, sigBytes)) {
                            ok = true;
                            break;
                        }
                    }
                    if (!ok) {
                        return AtxVerificationResult.reject(
                                RejectCategory.SIGNATURE_INVALID,
                                "Ed25519 signature " + ns(sig.keyId()) + " did not verify");
                    }
                    edVerified = true;
                } else if ("ML-DSA-65".equals(sig.algorithm())) {
                    // Presence recorded; PQC verification delegated. Not silently skipped.
                    mldsaPresent = true;
                } else {
                    // Unknown algorithm: ATX defines only Ed25519 and ML-DSA-65. Reject
                    // (rather than ignore) to block typo / Unicode-lookalike bypasses of
                    // this switch. Matches the Go reference + conformance verifiers.
                    return AtxVerificationResult.reject(
                            RejectCategory.SIGNATURE_INVALID,
                            "unsupported signature algorithm: " + sig.algorithm());
                }
            }
        }

        if (!edVerified) {
            return AtxVerificationResult.reject(RejectCategory.SIGNATURE_INVALID, "no Ed25519 signature verified");
        }

        ResolutionContext context = new ResolutionContext(
                atx.agentId,
                atx.agentDid,
                atx.issuerDid,
                atx.issuerChain != null ? atx.issuerChain : List.of(atx.issuerDid),
                atx.trustLevel,
                atx.trustScore,
                atx.capabilities != null ? atx.capabilities : List.of(),
                atx.scanSummary != null && atx.scanSummary.get("oasbLevel") instanceof String s ? s : null,
                atx.jurisdiction,
                isV11);
        return AtxVerificationResult.accept(context, mldsaPresent);
    }

    /**
     * DIDs whose keys may sign this credential: the issuer always, plus the
     * issuerChain authorities for v1.1 (where issuerChain is covered by the
     * signature). v1.0 issuerChain is unsigned and therefore forgeable, so it is
     * NOT trusted as a signer source — only the issuer is.
     */
    private static Set<String> authoritySetFor(Atx atx, boolean isV11) {
        Set<String> set = new HashSet<>();
        set.add(atx.issuerDid);
        if (isV11 && atx.issuerChain != null) {
            set.addAll(atx.issuerChain);
        }
        return set;
    }

    /**
     * Whether a key may verify a signature for an issuer in {@code authoritySet}.
     * Binding applies only to keys whose keyId is a DID-URL (contains '#'): such a
     * key is eligible only if its controller is in the set. A key with no '#'
     * fragment is unbound (legacy single-issuer configs) and stays eligible.
     */
    private static boolean keyEligible(String keyId, Set<String> authoritySet) {
        if (keyId == null || keyId.indexOf('#') < 0) {
            return true;
        }
        return authoritySet.contains(keyId.substring(0, keyId.indexOf('#')));
    }

    /** Build a public key from a raw 32-byte Ed25519 public key (hex), or null. */
    private static PublicKey ed25519FromRawHex(String hex) {
        try {
            byte[] raw = hexToBytes(hex);
            if (raw.length != 32) {
                return null;
            }
            byte[] spki = new byte[ED25519_SPKI_PREFIX.length + raw.length];
            System.arraycopy(ED25519_SPKI_PREFIX, 0, spki, 0, ED25519_SPKI_PREFIX.length);
            System.arraycopy(raw, 0, spki, ED25519_SPKI_PREFIX.length, raw.length);
            return KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(spki));
        } catch (Exception e) {
            return null;
        }
    }

    private static boolean ed25519Verify(PublicKey key, byte[] payload, byte[] signature) {
        try {
            Signature verifier = Signature.getInstance("Ed25519");
            verifier.initVerify(key);
            verifier.update(payload);
            return verifier.verify(signature);
        } catch (Exception e) {
            return false;
        }
    }

    private static byte[] hexToBytes(String hex) {
        int len = hex.length();
        byte[] out = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            out[i / 2] = (byte) Integer.parseInt(hex.substring(i, i + 2), 16);
        }
        return out;
    }

    private static String ns(String v) {
        return v == null ? "" : v;
    }
}
