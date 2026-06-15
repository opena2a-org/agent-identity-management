package org.opena2a.aim.atx;

import java.util.List;

/**
 * Context derived from a <em>verified</em> ATX. Contains no backend information.
 *
 * @param signedCapabilities true when the credential is v1.1+, i.e.
 *                           capabilities/scanSummary/issuerChain are covered by the
 *                           signature and may be trusted for authorization. False
 *                           for v1.0, where those fields are forgeable by the holder.
 */
public record ResolutionContext(
        String agentId,
        String agentDid,
        String issuerDid,
        List<String> issuerChain,
        int trustLevel,
        double trustScore,
        List<String> capabilities,
        String oasbLevel,
        List<String> jurisdiction,
        boolean signedCapabilities) {}
