# ATC Verification Specification v1.0

**Status:** Draft
**Version:** 1.0.0
**Date:** 2026-04-13
**Authors:** OpenA2A Security Team

---

## 1. Overview

Agent Trust Certificates (ATCs) enable credential-free agent communication. An agent presents its ATC to a target service, which verifies it locally without contacting the Registry. No credential is exchanged, stored, or resolved.

This specification defines:
- ATC binary format and encoding
- Verification algorithm
- Transport: HTTP Authorization header
- Certificate Revocation List (CRL) caching
- Capability scoping
- Service discovery mechanism

### 1.1 Design Goals

| Goal | Rationale |
|------|-----------|
| Local verification | CR-007: Registry never in hot path. No call to the Registry during verification. |
| Dual signatures | Post-quantum readiness. Ed25519 for speed, ML-DSA for quantum resistance. |
| Short-lived | ATCs expire in minutes, not days. Reduces revocation window. |
| Capability-scoped | ATC declares what the agent can do. Service checks before acting. |
| Fail closed | Invalid, expired, or revoked ATC = 401. A revocation state that cannot be read = 503, never accepted. No fallback. |

### 1.2 Terminology

| Term | Definition |
|------|-----------|
| **ATC** | Agent Trust Certificate. A signed, time-bounded, capability-scoped identity assertion. |
| **Issuer** | The AIM instance that issued the ATC. Identified by its public key and issuer URI. |
| **CRL** | Certificate Revocation List. The signed list of the agents an authority revokes (Section 5). |
| **Capability** | A string scope (e.g., `secrets:resolve`, `scan:read`) that the ATC grants. |

---

## 2. ATC Structure

An ATC is a CBOR-encoded map with the following fields:

```
ATC = {
  1: tstr,          ; atcId — unique certificate identifier (UUID v4)
  2: tstr,          ; agentId — agent UUID
  3: tstr,          ; issuer — AIM instance URI (e.g., "https://aim.opena2a.org")
  4: uint,          ; issuedAt — Unix timestamp (seconds)
  5: uint,          ; expiresAt — Unix timestamp (seconds)
  6: [* tstr],      ; capabilities — list of granted scopes
  7: bstr,          ; issuerPublicKey — Ed25519 public key of issuer (32 bytes)
  8: bstr,          ; ed25519Signature — Ed25519 signature over fields 1-7
  9: ? bstr,        ; mldsaPublicKey — ML-DSA public key of issuer (optional)
  10: ? bstr,       ; mldsaSignature — ML-DSA signature over fields 1-7 (optional)
  11: ? [* Chain],  ; delegationChain — issuer chain for federated trust (optional)
  12: ? tstr,       ; pqcAlgorithm — ML-DSA variant: "ML-DSA-44", "ML-DSA-65", "ML-DSA-87"
}

Chain = {
  1: tstr,          ; issuerUri
  2: bstr,          ; issuerPublicKey (Ed25519)
  3: bstr,          ; signature over parent chain entry
}
```

### 2.1 Canonical Encoding

The **signed payload** is the CBOR-encoded map of fields 1-7 only, in canonical CBOR (deterministic encoding per RFC 8949 Section 4.2). Signatures in fields 8 and 10 are computed over this canonical byte sequence.

### 2.2 Size Constraints

| Constraint | Value |
|-----------|-------|
| Maximum ATC size | 8 KiB |
| Maximum capabilities | 64 entries |
| Maximum capability string length | 128 bytes |
| Maximum delegation chain depth | 5 |
| Maximum TTL | 1 hour |
| Recommended TTL for secrets resolution | 5 minutes |

---

## 3. Transport

### 3.1 Authorization Header

Agents present ATCs via the HTTP Authorization header:

```
Authorization: ATC <base64url-encoded-atc>
```

The `ATC` scheme is case-sensitive. The token is base64url-encoded (RFC 4648 Section 5, no padding).

### 3.2 Service Discovery

Services signal ATC support via response headers:

```
X-ATC-Supported: true
X-ATC-Version: 1.0
```

These headers MUST be present on all responses from ATC-capable endpoints, including error responses. Clients cache this per-origin for the duration of the session.

### 3.3 Static Registry

Known ATC-capable services are listed in `atc_native_services.json`:

```json
{
  "services": [
    {
      "origin": "https://api.oa2a.org",
      "version": "1.0",
      "capabilities": ["registry:read", "registry:write", "trust:read"]
    },
    {
      "origin": "https://aim.opena2a.org",
      "version": "1.0",
      "capabilities": ["secrets:resolve", "agent:manage"]
    }
  ]
}
```

Discovery priority:
1. Static registry (checked first, zero latency)
2. HTTP header probe (cached per session)

---

## 4. Verification Algorithm

A service receiving an `Authorization: ATC` header MUST execute the following steps in order. If any step fails, return the HTTP status and error code Section 4.2 gives for it.

### 4.1 Steps

```
1. DECODE:    Base64url-decode the token. If decoding fails → 401 (atc_malformed).
2. PARSE:     CBOR-decode the payload. Validate required fields present → 401 (atc_malformed).
3. SIZE:      Check total size ≤ 8 KiB → 401 (atc_oversized).
4. EXPIRY:    Check expiresAt > now AND issuedAt ≤ now (30s clock skew allowed) → 401 (atc_expired).
5. ISSUER:    Check issuer URI is in the trusted issuers list → 401 (atc_untrusted_issuer).
6. ED25519:   Verify ed25519Signature over canonical fields 1-7 using issuerPublicKey → 401 (atc_signature_invalid).
7. ML-DSA:    If mldsaSignature present: verify using mldsaPublicKey and declared pqcAlgorithm → 401 (atc_pqc_signature_invalid).
              If not present: proceed (ML-DSA is optional in v1).
8. REVOCATION: Check the ATC's revocation state (Section 5; the AIM backend, Section 9.4) → 401 (atc_revoked).
              If that state cannot be read → 503 (atc_revocation_unknown) with Retry-After. Never accept.
9. CHAIN:     If delegationChain present: verify each link's signature → 401 (atc_chain_invalid).
10. CAPABILITY: Check requested operation is in capabilities list → 403 (atc_insufficient_scope).
```

### 4.2 Error Responses

All ATC verification errors return a JSON body:

```json
{
  "error": "atc_expired",
  "message": "ATC expired at 2026-04-13T05:30:00Z"
}
```

A `503` answer carries `Retry-After`, the number of seconds after which to send the request again. The AIM backend
answers an ATC whose revocation state it cannot read (Section 9.4) with:

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json; charset=utf-8
Retry-After: 5
X-ATC-Supported: true
X-ATC-Version: 1.0

{"error":"atc_revocation_unknown","message":"the revocation state of this ATC could not be read"}
```

| Error Code | HTTP Status | Meaning | Next step |
|-----------|------------|---------|-----------|
| `atc_malformed` | 401 | Cannot decode or parse ATC | Send the ATC as Sections 2 and 3.1 define it: a CBOR map, base64url encoded without padding |
| `atc_oversized` | 401 | ATC exceeds 8 KiB | Request an ATC of at most 8 KiB, with fewer capabilities or a shorter delegation chain |
| `atc_expired` | 401 | ATC is expired or not yet valid | Request a new ATC from the issuer. If the ATC is not yet valid, compare the clocks of the issuer and the service |
| `atc_untrusted_issuer` | 401 | Issuer not in trusted list | Present an ATC from an issuer the service trusts (Section 7), or ask the service's operator to add the issuer |
| `atc_signature_invalid` | 401 | Ed25519 signature verification failed | Request a new ATC from the issuer and send it unchanged |
| `atc_pqc_signature_invalid` | 401 | ML-DSA signature verification failed | Request a new ATC from the issuer and send it unchanged |
| `atc_revoked` | 401 | ATC is revoked (Section 5; the AIM backend, Section 9.4) | Do not send the ATC again. A new ATC for the same agent is refused as well while the agent stays revoked |
| `atc_revocation_unknown` | 503 | The revocation state could not be read, so the ATC is not accepted. The response carries `Retry-After` | Send the request again after the `Retry-After` seconds, with the same ATC while it is valid. The ATC was not judged, so a new ATC does not help |
| `atc_chain_invalid` | 401 | Delegation chain verification failed | Request an ATC whose delegation chain verifies link by link up to an issuer the service trusts |
| `atc_insufficient_scope` | 403 | ATC lacks required capability | Request an ATC that grants the capability the operation needs (Section 6) |

---

## 5. Certificate Revocation List (CRL)

An authority publishes the agents it revokes as a signed JSON list at the address its discovery document names. This
section gives the contract the AIM backend's list client (`apps/backend/internal/infrastructure/atc/crl_client.go`)
reads: where it finds the list (Section 5.1), the form it accepts (Section 5.2), the signatures (Section 5.3) and
bounds (Section 5.4) it checks, and how long it answers from a list it holds (Section 5.5).

The list revokes agents, not single ATCs: each entry names an agent by its DID in `agentDid`, and the client reads no
ATC identifier from an entry. For step 8, an ATC is revoked when the list names its agent. The DID AIM gives an agent
is `did:aip:aim_` followed by the agent's id, the `agentId` of its ATCs (Section 2).

With `REGISTRY_URL` set, the AIM backend reads the list in the background (Section 5.5), and no request it answers
fetches the list. `GET /metrics` and `GET /health/ready` read the list it holds: `/metrics` gives its age and its
entries as `aim_atc_revocation_list_age_seconds` and `aim_atc_revocation_list_entries`, and `/health/ready` gives in
`dependencies.revocationState` whether it is fit to answer from. Its ATC verifier does not read it: it reads step 8
from its own store (Section 9.4).

### 5.1 CRL Endpoint

The client has no fixed list address. It reads the authority's discovery document at `GET /.well-known/atp`, or at
`GET /.well-known/opena2a` when the first answers `404`, on the scheme, host and port of the authority's base URL
(`REGISTRY_URL` in the AIM backend) and not under a path that URL carries. It reads two members of that document:

- `endpoints.revocations`: the list address, resolved against the address the document was read at. It must give an
  `https` address with a host, and the document must set it unless an override replaces it (below).
- `publicKeys`: the keys a list may be signed with, each an object with `keyId`, `algorithm` and `publicKeyHex`. The
  client uses a key that has a `keyId`, an `algorithm` of `Ed25519` or `ML-DSA-65`, and a `publicKeyHex` that is the
  hex of a raw public key of that algorithm's size: 32 bytes for `Ed25519`, 1,952 bytes for `ML-DSA-65`. It uses no
  key written otherwise, and a `keyId` listed twice refuses the document.

A discovery document of this form, with a placeholder in place of its key:

```json
{
  "endpoints": {
    "revocations": "/api/v1/trust/revocations"
  },
  "publicKeys": [
    {
      "keyId": "did:web:registry.example.com#list-1",
      "algorithm": "Ed25519",
      "publicKeyHex": "<hex of the 32-byte Ed25519 public key>"
    }
  ]
}
```

The client reads the list with `GET` at that address. When the list it holds was read from the same address with an
`ETag`, under the keys the client uses from the discovery document now (the same `keyId`s, each with the same
`algorithm` and public key), the request carries that `ETag` in `If-None-Match`, and a `304` answer counts as a read of
the list held. When the address or the keys differ, the request carries no `If-None-Match`, and the list is read in
full and checked under the keys used now. The client follows no redirect to an address that is not `https`. In the AIM
backend, `ATC_CRL_ENDPOINT`, read only when `REGISTRY_URL` is set, replaces the address `endpoints.revocations` names,
and the keys still come from the discovery document.

### 5.2 List Form

The list is a JSON object in valid UTF-8, with nothing but white space after it, in which no object names a member
twice:

| Member | Form |
|--------|------|
| `revocations` | Required. An array of entries |
| `entryCount` | Required. The number of entries in `revocations`, an integer written without a fraction or an exponent |
| `thisUpdate` | Required. An RFC 3339 time (Section 5.4) |
| `nextUpdate` | Required. An RFC 3339 time (Section 5.4) |
| `signatures` | Required. An array of objects with string `keyId`, `algorithm` and `value` (Section 5.3) |

Other members are allowed, and the signatures cover them. An entry is an object:

| Member | Form |
|--------|------|
| `agentDid` | Required. The DID of the revoked agent, a string of 1 to 512 bytes |
| `revokedAt` | Required. An RFC 3339 time in UTC, written with `Z` |
| `reason` | Optional. A string (Section 5.6) |
| `revokedByKeyId` | Optional. A string of at most 576 bytes |
| `transparencyLogIndex` | Optional. An integer from 0 to 2^63-1 |

The client reads no other member of an entry. A list of this form that revokes one agent, with a placeholder in place of
its signature:

```json
{
  "revocations": [
    {
      "agentDid": "did:aip:aim_550e8400-e29b-41d4-a716-446655440000",
      "revokedAt": "2026-10-18T04:58:00Z",
      "reason": "compromised",
      "revokedByKeyId": "did:web:registry.example.com#operator-1",
      "transparencyLogIndex": 42
    }
  ],
  "entryCount": 1,
  "thisUpdate": "2026-10-18T05:00:00Z",
  "nextUpdate": "2026-10-18T05:05:00Z",
  "signatures": [
    {
      "keyId": "did:web:registry.example.com#list-1",
      "algorithm": "Ed25519",
      "value": "<base64 of the Ed25519 signature>"
    }
  ]
}
```

### 5.3 Signatures

The signatures cover the RFC 8785 (JSON Canonicalization Scheme) form of the list without its `signatures` member.
The client passes over a signature whose `keyId` is not a key it uses from the discovery document (Section 5.1). A
signature by a key it uses refuses the list when its `algorithm` is not the key's, when its `value` is not standard
base64, with or without padding, or when it does not verify. At least one `Ed25519` signature by a key the client uses
must verify: an `ML-DSA-65` signature is verified when present, and is not enough alone.

### 5.4 Bounds

| Bound | Value |
|-------|-------|
| Discovery document | At most 65,536 bytes |
| List body | At most 12,645,536 bytes, and a longer body is refused without being read whole |
| Members other than `revocations`, `signatures` included | At most 65,536 bytes in canonical form |
| Entries in `revocations` | At most 10,000 |
| One entry | At most 1,258 bytes in canonical form |
| Arrays and objects in the list | Nested at most 16 levels deep, the list's own object included |

The list body bound is 65,536 bytes for the members other than `revocations`, plus 1,258 bytes for each of 10,000
entries.

The client refuses a list whose `thisUpdate` is more than 60 seconds after its clock, whose `nextUpdate` is not after
its `thisUpdate`, whose `nextUpdate` passed more than 60 seconds ago, or whose `thisUpdate` is before that of the list
it holds.

### 5.5 Caching

The client answers from the list it holds, and reads no address to answer. A refresh reads the discovery document and
the list, and the client holds the list when it passes Sections 5.2 to 5.4. A refresh that fails, or whose list is
refused, leaves the list held as it was.

| Parameter | Value |
|-----------|-------|
| Answered from | The list held, until 5 minutes after its last read and until 60 seconds after its `nextUpdate`, whichever comes first |
| No list to answer from | The revocation state is unknown, which is never an answer of not revoked (step 8: `503`, `atc_revocation_unknown`) |

With `REGISTRY_URL` set, the AIM backend refreshes the list in the background, first when the server starts:

| Parameter | Value |
|-----------|-------|
| Next refresh, after one that holds a list or reads the list held again | 4 minutes later, or 60 seconds before the `nextUpdate` of the list held when that is sooner |
| Next refresh, after one that fails or whose list is refused, or that would start in less than 1 second | After a wait drawn at random from 1 to 30 seconds |
| One refresh | Ended after 30 seconds |
| One request | Ended after 10 seconds, the timeout of the HTTP client the list client builds for itself when it is given none, as in the AIM backend |

### 5.6 Revocation Reasons

An entry's `reason`, when present, is a string. The client checks its type and does not read its value: an agent is
revoked when the list names it, whatever its entry's `reason`.

---

## 6. Capability Scoping

### 6.1 Capability Format

Capabilities use colon-separated hierarchical scopes:

```
<domain>:<action>[:<resource>]
```

Examples:
- `secrets:resolve`: resolve any credential
- `secrets:resolve:github`: resolve credentials in the "github" namespace only
- `scan:read`: read scan results
- `registry:write`: write to registry
- `agent:manage`: manage agent lifecycle

### 6.2 Wildcard

The `*` capability grants all scopes. It MUST only be issued to first-party AIM system agents.

### 6.3 Matching Rules

A capability `A` satisfies a requirement `B` if:
1. `A` equals `B` exactly, OR
2. `A` is `*`, OR
3. `A` is a prefix of `B` when split on `:` (e.g., `secrets:resolve` satisfies `secrets:resolve:github`)

---

## 7. Trusted Issuers

Each verifying service maintains a list of trusted issuer public keys. This list is configured at deployment, not fetched at runtime (CR-007).

```json
{
  "trustedIssuers": [
    {
      "uri": "https://aim.opena2a.org",
      "publicKey": "<base64-ed25519-public-key>",
      "mldsaPublicKey": "<base64-mldsa-public-key>",
      "mldsaAlgorithm": "ML-DSA-65"
    }
  ]
}
```

Key rotation: when an issuer rotates keys, both old and new keys are trusted during a transition period equal to the maximum ATC TTL (1 hour). After the transition, old keys are removed.

---

## 8. Security Considerations

### 8.1 Replay Protection

ATCs are time-bounded. The maximum TTL of 1 hour limits the replay window. For secrets resolution, the recommended TTL of 5 minutes further reduces exposure.

Services MAY implement additional replay protection by tracking recently-seen ATC IDs within their expiry window.

### 8.2 Stolen ATC

A stolen ATC is usable until expiry or CRL update (up to 5 minutes). Mitigations:
- Short TTL (5 minutes for secrets)
- CRL refresh per Section 5.5
- Agents should request new ATCs per-operation, not reuse

### 8.3 Issuer Compromise

If an issuer's private key is compromised:
1. Rotate issuer key pair
2. Update trusted issuer configuration on all services
3. Re-issue ATCs for all active agents with new key

The revocation list (Section 5) has no entry for this: its entries revoke agents, not ATCs or issuer keys.

### 8.4 Quantum Resistance

ML-DSA signatures are optional in v1 but RECOMMENDED. Services MUST verify ML-DSA signatures when present. A future spec version may require ML-DSA.

---

## 9. Implementation Notes

### 9.1 Token Format

The AIM backend accepts the CBOR-encoded ATC defined in this spec and no other token format as an ATC. A JWT
presented as an ATC, in an `Authorization: ATC` header or in the secrets resolve request body, is refused; the header
path answers `401` with `atc_malformed`. A request without an `Authorization: ATC` header passes to the route's other
authentication middleware.

### 9.2 Performance

| Operation | Target |
|-----------|--------|
| ATC verification (Ed25519 only) | < 100 microseconds |
| ATC verification (Ed25519 + ML-DSA-65) | < 5 milliseconds |
| Revocation read (AIM backend, Section 9.4) | one store read per verification, ended after 2 seconds |
| Full resolve with ATC | < 10 milliseconds p99 |

### 9.3 Go Implementation Reference

```go
// ATCVerifier interface (already exists in domain/atc/verifier.go)
type ATCVerifier interface {
    Verify(rawToken string) (*ATCClaims, error)
}

// RevocationSource is read once for each Verify call (domain/atc/verifier.go)
type RevocationSource interface {
    AgentRevoked(ctx context.Context, agentID uuid.UUID) (bool, error)
}

// New: ATCIssuer for creating real ATCs
type ATCIssuer interface {
    Issue(agentID uuid.UUID, capabilities []string, ttl time.Duration) ([]byte, error)
}
```

### 9.4 Revocation in the AIM backend

The AIM backend reads step 8 from its own store, not from the list in Section 5, and `ATC_CRL_ENDPOINT` has no
effect on it:

- Each verification reads the status of the agent the ATC's `agentId` names once, a token the verifier has cached
  included, and ends the read after 2 seconds.
- The ATC is revoked (`401`, `atc_revoked`) when no agent has that id, or when the agent's status is not `verified`
  or `pending`. The backend's signature, API-key and service-token authentication refuses the same statuses.
- When that read fails or has not answered after 2 seconds, the answer is `503` with `Retry-After: 5` and
  `{"error":"atc_revocation_unknown","message":"the revocation state of this ATC could not be read"}`, both for an
  `Authorization: ATC` header and for an `atcId` in the body of `POST /api/v1/secrets/resolve`. In the second case
  the secrets audit entry is a denial whose reason names `atc_revocation_unknown`.

---

## 10. Conformance

A service is ATC-v1 conformant if it:
1. Accepts `Authorization: ATC` headers
2. Implements all 10 verification steps from Section 4.1
3. Returns standardized error codes from Section 4.2
4. Caches CRL per Section 5.5
5. Emits `X-ATC-Supported: true` and `X-ATC-Version: 1.0` headers
6. Fails closed on any verification error
