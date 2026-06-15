package org.opena2a.aim.atx;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.erdtman.jcs.JsonCanonicalizer;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.nio.charset.StandardCharsets;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.Map;

/**
 * Produces the signed canonical bytes for an ATX, byte-for-byte identical to
 * {@code @opena2a/atx-verify} (TS), {@code opena2a-registry/pkg/atcverify} (Go),
 * and the Python reference verifier. Byte agreement is pinned by
 * {@code atx-conformance/jcs-vectors}.
 *
 * <ul>
 *   <li>v1.0 ({@link #canonicalPayload}): an 11-field pipe-delimited string.</li>
 *   <li>v1.1 ({@link #canonicalPayloadV11}): JCS (RFC 8785) over the TBS projection.</li>
 * </ul>
 */
public final class AtxCanonicalizer {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final DateTimeFormatter RFC3339_SECONDS =
            DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'");

    private AtxCanonicalizer() {}

    /**
     * Mirror of {@code verify.go canonicalPayload()}:
     * {@code agentId|agentDid|version|contentHash|buildAttestation|issuerDid|trustLevel|%.6f trustScore|issuedAt|expiresAt|1.0}.
     */
    public static byte[] canonicalPayload(Atx atx) {
        String canonical = String.join("|",
                ns(atx.agentId),
                ns(atx.agentDid),
                ns(atx.version),
                ns(atx.contentHash),
                ns(atx.buildAttestation),
                ns(atx.issuerDid),
                Long.toString((long) atx.trustLevel),
                formatTrustScore(atx.trustScore),
                normalizeRfc3339(atx.issuedAt),
                normalizeRfc3339(atx.expiresAt),
                "1.0");
        return canonical.getBytes(StandardCharsets.UTF_8);
    }

    /**
     * Project an ATX into the v1.1 TBS and return JCS(TBS) (RFC 8785). Covers
     * capabilities, scanSummary, issuerChain, publisher, and behavioralProfile.
     * The projection (canonical empties, always-full scanSummary, %.6f string
     * trustScore, root-first issuerChain) matches the reference verifiers exactly.
     */
    public static byte[] canonicalPayloadV11(Atx atx) {
        ObjectNode tbs = MAPPER.createObjectNode();
        tbs.put("atcVersion", atx.atcVersion);
        tbs.put("agentId", atx.agentId);
        tbs.put("agentDid", atx.agentDid);
        tbs.put("publisher", ns(atx.publisher));
        tbs.put("publisherDid", ns(atx.publisherDid));
        tbs.put("version", atx.version);
        tbs.put("contentHash", atx.contentHash);
        tbs.put("buildAttestation", ns(atx.buildAttestation));
        tbs.set("capabilities", arrayOf(atx.capabilities));
        tbs.set("behavioralProfile", projectBehavioralProfile(atx.behavioralProfile));

        Map<String, Object> scan = atx.scanSummary != null ? atx.scanSummary : Map.of();
        ObjectNode scanNode = MAPPER.createObjectNode();
        scanNode.put("hma", asString(scan.get("hma")));
        scanNode.put("criticalFindings", asLong(scan.get("criticalFindings")));
        scanNode.put("highFindings", asLong(scan.get("highFindings")));
        scanNode.put("secretless", asString(scan.get("secretless")));
        scanNode.put("cryptoServe", asString(scan.get("cryptoServe")));
        scanNode.put("oasbLevel", asString(scan.get("oasbLevel")));
        tbs.set("scanSummary", scanNode);

        // trustScore is the %.6f string form so trustLevel is the only JSON number.
        tbs.put("trustScore", formatTrustScore(atx.trustScore));
        tbs.put("trustLevel", (long) atx.trustLevel);
        tbs.put("issuedAt", normalizeRfc3339(atx.issuedAt));
        tbs.put("expiresAt", normalizeRfc3339(atx.expiresAt));
        tbs.put("issuerDid", atx.issuerDid);
        tbs.set("issuerChain", arrayOf(atx.issuerChain));

        try {
            String json = MAPPER.writeValueAsString(tbs);
            String canonical = new JsonCanonicalizer(json).getEncodedString();
            return canonical.getBytes(StandardCharsets.UTF_8);
        } catch (Exception e) {
            throw new IllegalArgumentException("v1.1 canonicalization failed: " + e.getMessage(), e);
        }
    }

    /** Normalize an RFC 3339 timestamp to UTC "YYYY-MM-DDTHH:MM:SSZ" (seconds precision). */
    public static String normalizeRfc3339(String s) {
        try {
            return OffsetDateTime.parse(s).toInstant().atOffset(ZoneOffset.UTC).format(RFC3339_SECONDS);
        } catch (Exception e) {
            throw new IllegalArgumentException("invalid RFC 3339 timestamp: " + s, e);
        }
    }

    /**
     * behavioralProfile -> JSON null only when absent/null (matching the TS
     * `bp === null || bp === undefined` check); a present-but-empty object still
     * projects to the canonical three-field form.
     */
    private static com.fasterxml.jackson.databind.JsonNode projectBehavioralProfile(Map<String, Object> bp) {
        if (bp == null) {
            return MAPPER.nullNode();
        }
        ObjectNode node = MAPPER.createObjectNode();
        node.put("checksum", asString(bp.get("checksum")));
        Object generatedAt = bp.get("generatedAt");
        node.put("generatedAt", generatedAt instanceof String g && !g.isEmpty() ? normalizeRfc3339(g) : "");
        node.put("observationDays", asLong(bp.get("observationDays")));
        return node;
    }

    /**
     * Format trustScore as 6 decimals with round-half-to-even, byte-identical to
     * Go's {@code %.6f} and Python's {@code :.6f} (the signer's format). Java's
     * {@code String.format("%.6f")} uses HALF_UP and would diverge at the 7th
     * decimal, false-rejecting valid credentials.
     */
    private static String formatTrustScore(double trustScore) {
        if (!Double.isFinite(trustScore)) {
            throw new IllegalArgumentException("trustScore is not finite: " + trustScore);
        }
        return new BigDecimal(trustScore).setScale(6, RoundingMode.HALF_EVEN).toPlainString();
    }

    private static ArrayNode arrayOf(List<String> items) {
        ArrayNode arr = MAPPER.createArrayNode();
        if (items != null) {
            for (String item : items) {
                arr.add(item);
            }
        }
        return arr;
    }

    /** Null-safe string ("" when null), matching the TS `?? ''` projection. */
    private static String ns(String v) {
        return v == null ? "" : v;
    }

    private static String asString(Object v) {
        return v instanceof String s ? s : "";
    }

    /** Truncate to a 64-bit integer (matching TS Math.trunc); avoids 32-bit overflow. */
    private static long asLong(Object v) {
        return v instanceof Number n ? n.longValue() : 0L;
    }
}
