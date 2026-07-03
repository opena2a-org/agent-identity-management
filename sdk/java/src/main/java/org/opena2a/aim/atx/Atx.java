package org.opena2a.aim.atx;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.JsonNode;

import java.util.List;
import java.util.Map;

/**
 * The ATX (Agent Trust eXtension) credential, as parsed for verification. Only the
 * fields the canonicalizer or verifier touch are typed; everything else is
 * tolerated (fixtures carry id, transparencyLogIndex, revokedAt, createdAt, ...).
 *
 * <p>ATX is the current name for the credential formerly called ATC; fixtures use
 * the {@code atcVersion} field, dual-supporting "1.0" and "1.1".
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Atx {
    public String atcVersion;
    public String agentId;
    public String agentDid;
    public String publisher;
    public String publisherDid;
    public String version;
    public String contentHash;
    public String buildAttestation;
    public List<String> capabilities;
    /**
     * Declared purpose (atx-spec §1.5); null when absent. Kept as a raw tree:
     * the v1.1 TBS passes a present, non-empty object through verbatim for JCS
     * to re-canonicalize (§1.3a.2 rule 5).
     */
    public JsonNode declaredPurpose;
    /** Observed-behavior summary; null when absent. Read field-by-field in the v1.1 TBS. */
    public Map<String, Object> behavioralProfile;
    public Map<String, Object> scanSummary;
    public int trustLevel;
    public double trustScore;
    public String issuedAt;
    public String expiresAt;
    public String issuerDid;
    public List<String> issuerChain;
    public List<String> jurisdiction;
    public boolean revoked;
    public List<AtxSignature> signatures;
}
