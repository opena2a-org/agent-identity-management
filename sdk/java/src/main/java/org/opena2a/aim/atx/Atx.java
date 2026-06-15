package org.opena2a.aim.atx;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

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
