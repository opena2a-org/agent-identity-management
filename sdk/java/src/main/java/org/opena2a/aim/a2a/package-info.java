/**
 * A2A (Agent-to-Agent) Protocol Support for AIM.
 *
 * <p>This package provides support for Google's A2A protocol with AIM security enhancements:
 *
 * <ul>
 *   <li>{@link org.opena2a.aim.a2a.A2AClient} - Main client for A2A operations</li>
 *   <li>{@link org.opena2a.aim.a2a.A2AAgentCard} - Agent card manifest with capabilities and skills</li>
 *   <li>{@link org.opena2a.aim.a2a.A2ARequestSignature} - Ed25519 request signatures for non-repudiation</li>
 *   <li>{@link org.opena2a.aim.a2a.A2ATrustScore} - Behavioral trust scores between agents</li>
 *   <li>{@link org.opena2a.aim.a2a.A2APeerTrust} - Peer trust relationships</li>
 *   <li>{@link org.opena2a.aim.a2a.A2AConsent} - GDPR/PSD2 compliant consent management</li>
 *   <li>{@link org.opena2a.aim.a2a.A2AException} - A2A-specific exceptions</li>
 * </ul>
 *
 * <p>Example usage:
 * <pre>{@code
 * // Create AIM client
 * AIMClient aimClient = AIMClient.secure("my-agent");
 *
 * // Create A2A client
 * A2AClient a2a = new A2AClient(aimClient);
 *
 * // Register agent card
 * A2AAgentCard card = a2a.registerAgentCard("https://myagent.example.com/.well-known/agent.json");
 *
 * // Sign requests
 * A2ARequestSignature sig = a2a.signRequest("POST", "/tasks/send", requestBody);
 *
 * // Get trust score
 * A2ATrustScore trust = a2a.getTrustScore(targetAgentId);
 *
 * // Record consent
 * A2AConsent consent = a2a.recordConsent(userId, targetAgentId, "data-sharing",
 *     Arrays.asList("profile", "preferences"), "consent", expiresAt);
 * }</pre>
 *
 * @see <a href="https://google.github.io/A2A/">Google A2A Protocol</a>
 * @since 0.3.0
 */
package org.opena2a.aim.a2a;
