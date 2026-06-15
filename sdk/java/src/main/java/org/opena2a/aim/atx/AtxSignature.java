package org.opena2a.aim.atx;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

/**
 * A signature reference on an ATX credential.
 *
 * @param keyId     identifier of the signing key (informational; binding uses the
 *                  verifier's configured key keyId, never this attacker-supplied one)
 * @param algorithm "Ed25519" or "ML-DSA-65"
 * @param value     base64-encoded signature value
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public record AtxSignature(String keyId, String algorithm, String value) {}
