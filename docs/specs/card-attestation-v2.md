# Agent Card Attestation v2: Third-Party Verification

**Status:** Draft
**Format label:** `opena2a-aim/card-attestation/v2`

---

## 1. Overview

When an agent card is registered or refreshed, the AIM server signs an attestation that the card belongs to the agent. The attestation is served inside the agent card, in `aim.attestation`. The public key that verifies it is served in the server's JSON Web Key Set at `/.well-known/jwks.json`.

This document is the procedure a third party follows to verify an attestation with those two documents alone: no account, no API key, and no call back to the server beyond fetching them.

| Document | Route | Authentication |
|----------|-------|----------------|
| Agent card | `GET /.well-known/agent.json?agentId=<agent-id>` | None |
| Agent card (owning organization) | `GET /api/v1/a2a/agents/<agent-id>/card` | Required |
| JWK Set | `GET /.well-known/jwks.json` | None |

```bash
curl -s "https://aim.example.com/.well-known/agent.json?agentId=$AGENT_ID" > card.json
curl -s https://aim.example.com/.well-known/jwks.json > jwks.json
node verify-card-attestation.mjs card.json jwks.json
```

[`verify-card-attestation.mjs`](verify-card-attestation.mjs) implements section 4 in Node.js 18 or later with no dependencies.

## 2. The Served Attestation

```json
{
  "aim": {
    "agentId": "7d3f2a9e-41c8-4b6d-9e0f-5a1b2c3d4e5f",
    "attestation": {
      "format": "opena2a-aim/card-attestation/v2",
      "issuer": "aim-server",
      "cardHash": "cd287966c0eea91e95d010f7da379df77f4504b34d7aca09ea6e77eb565a453e",
      "issuedAt": "2026-10-05T12:00:00Z",
      "expiresAt": "2026-10-06T12:00:00Z",
      "signature": "fD6Vp2yJHRX05JUhCVCVu/KInSIIkDGh2A2uYTroF64z1GPBlCJWhQY1GabUkJl2pwzmTEVzJDhetOtKhzoWAQ==",
      "keyId": "56475aa75463474c0285df5dbf2bcab73da651358839e9b77481b2eab107708c",
      "alg": "EdDSA"
    }
  }
}
```

| Member | Meaning |
|--------|---------|
| `aim.agentId` | The agent the card belongs to. Part of the signed payload. |
| `format` | The v2 label. An attestation without it was issued before this format and cannot be verified by a third party (section 6). |
| `issuer` | Always `aim-server`. |
| `cardHash` | Lowercase hex SHA-256 of the card bytes as registered: the JSON submitted inline, or the bytes fetched from the card URL. |
| `issuedAt`, `expiresAt` | The validity window, in UTC with whole seconds. |
| `signature` | Ed25519 signature, standard base64 with padding (64 bytes decoded). |
| `keyId` | The `kid` of the signing key in the JWK Set: the lowercase hex SHA-256 of the 32-byte public key. |
| `alg` | Always `EdDSA`. |

`cardHash` does not hash the served card. The served card is re-encoded by the server and carries the `aim` member, so its bytes differ from the registered card. To bind the attestation to card content, fetch the agent's own card from its URL and compare its SHA-256 with `cardHash`.

## 3. What Is Signed

The signing input is three parts joined without padding:

1. **The v2 label**, `opena2a-aim/card-attestation/v2`.
2. A newline, byte `0x0A`.
3. **Go's JSON form of the payload**: the bytes Go's `encoding/json` writes for the payload object. For v2 that is exactly

   ```text
   {"cardHash":"<cardHash>","agentId":"<agentId>","issuer":"<issuer>","issuedAt":"<issuedAt>","expiresAt":"<expiresAt>"}
   ```

   with the members in this order, no whitespace, and no trailing newline. Every value is ASCII that JSON writes without escapes: hex, a lowercase UUID, `aim-server`, and timestamps.

The signature is Ed25519 (RFC 8032) over the 32-byte SHA-256 digest of the signing input, not over the signing input itself.

**UTC normalisation.** `issuedAt` and `expiresAt` in the payload are written `YYYY-MM-DDTHH:MM:SSZ`: UTC, whole seconds, the letter `Z`, no fractional part. The server truncates both timestamps to whole seconds before it signs and stores them, so the served values are the signed values. A proxy or client library may re-render a timestamp with an offset, such as `2026-10-05T06:00:00-06:00`; convert it to UTC before rebuilding the payload. A served timestamp with a fractional second is malformed in v2.

## 4. Verification Procedure

Each step either continues or ends with the result named.

1. Read `aim.agentId` and `aim.attestation` from the card. If `format` is not `opena2a-aim/card-attestation/v2` or `alg` is not `EdDSA`, stop: **unsupported-format**.
2. Normalise `issuedAt` and `expiresAt` to UTC as in section 3. Check that `cardHash` is 64 lowercase hex characters, `aim.agentId` is a lowercase UUID, `issuer` is `aim-server`, and `signature` decodes to 64 bytes. If any check fails, stop: **malformed**.
3. In the JWK Set, find the key whose `kid` equals `keyId`, whose `purpose` is `card-attestation`, and whose `kty` and `crv` are `OKP` and `Ed25519`. Keys with `status` `active` and `retired` both qualify. If there is none, stop: **unknown-key**. A key published for another purpose, such as `atc-issuer`, never verifies a card attestation.
4. Decode the key's `x` member (base64url, no padding) to the 32-byte public key.
5. Build the payload from the values of steps 1 and 2, then the signing input, then its SHA-256 digest (section 3).
6. Verify the Ed25519 signature over the digest with the public key. If it fails: **signature-invalid**. Otherwise: **verified**.
7. Compare `expiresAt` with your own clock. A verified attestation whose `expiresAt` has passed is expired; the owner refreshes it with `POST /api/v1/a2a/agents/<agent-id>/card/refresh`.

## 5. Conformance Vector

[`card-attestation-v2-vector.json`](card-attestation-v2-vector.json) holds a complete worked example: a fixed Ed25519 seed, the card bytes, the JWK Set the server publishes for that seed, the intermediate values (`cardHash`, payload, signing input, digest), and seven cases with the result a conforming verifier returns for each.

The seed is public test data. Never configure it as a signing key.

```bash
node verify-card-attestation.mjs --vector card-attestation-v2-vector.json
```

```text
ok   the attestation as the server serves it: verified
ok   timestamps written with a UTC offset are normalised to Z before rebuilding the payload: verified
ok   a changed card hash does not verify: signature-invalid
ok   a different agent ID does not verify: signature-invalid
ok   a timestamp with a fractional second is malformed: malformed
ok   a key ID that is not in the JWK Set is an unknown key: unknown-key
ok   an attestation without the v2 format label cannot be verified by a third party: unsupported-format
```

The same vector checked with OpenSSL 3 instead of the script:

```bash
V=card-attestation-v2-vector.json
jq -j '.intermediate.signingInput' "$V" | openssl dgst -sha256 -binary > digest.bin
jq -r '.cases[0].attestation.signature' "$V" | base64 -d > sig.bin
(printf '\x30\x2a\x30\x05\x06\x03\x2b\x65\x70\x03\x21\x00'
 jq -r '.jwks.keys[0].x' "$V" | tr '_-' '/+' | sed 's/$/=/' | base64 -d) > pub.der
openssl pkeyutl -verify -pubin -keyform DER -inkey pub.der -rawin -in digest.bin -sigfile sig.bin
# Signature Verified Successfully
```

The server's test suite signs the vector's inputs with the seed and checks that the result matches the first case exactly, so the vector changes only with a new format label.

## 6. Attestations Issued Before v2

Attestations issued before the v2 format carry no `format` member. Their signed `issuedAt` was not stored, so the served card cannot rebuild their payload. They expire within their validity window (`A2A_ATTESTATION_VALIDITY_HOURS`, 24 hours by default), and refreshing one re-signs it in v2.

## 7. Key Rotation

The card-attestation key is `AIM_SIGNING_KEY_CARD_ATTESTATION`, a base64 32-byte Ed25519 seed. When it is unset, the key is derived from `KEYVAULT_MASTER_KEY` with HKDF-SHA256. See "Server Signing Keys" in [`DEPLOYMENT.md`](../DEPLOYMENT.md).

The server does not verify card attestations itself; third parties do, with the JWK Set. An attestation verifies only while the JWK Set publishes the key that signed it, so a rotation keeps the old key published as `retired` until every attestation it signed has expired.

**Rotating the card-attestation key:**

1. Before the restart, copy the `x` member of the active `card-attestation` key from the JWK Set:

   ```bash
   curl -s https://aim.example.com/.well-known/jwks.json \
     | jq -r '.keys[] | select(.purpose == "card-attestation" and .status == "active") | .x'
   ```

2. Set `AIM_SIGNING_KEY_CARD_ATTESTATION` to the new seed and list the copied value in `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED` (comma-separated if there are several; the variable accepts the base64url `x` form as copied).
3. Restart. The JWK Set publishes the new key as `active` and the old key as `retired` with its original `kid`, so attestations it signed still verify.
4. Once the validity window has passed (24 hours by default), remove the entry from `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED`.

**Rotating `KEYVAULT_MASTER_KEY` moves every derived key.** Every signing key that is not provisioned is derived from the master key, so a new master key gives each of them a new key and a new `kid` at the next start. Attestations signed under the old derived card-attestation key then return **unknown-key**, and ATCs signed under the old derived ATC-issuer key stop verifying. Before rotating the master key, copy the `x` of the active `card-attestation` and `atc-issuer` keys from the JWK Set and list each in its `_RETIRED` variable (`AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED`, `AIM_SIGNING_KEY_ATC_ISSUER_RETIRED`), as in step 1. Provisioning both keys, as recommended for production, keeps them fixed when the master key rotates.
