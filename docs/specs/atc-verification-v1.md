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
| Local verification | CR-007: Registry never in hot path. No network call during verification. |
| Dual signatures | Post-quantum readiness. Ed25519 for speed, ML-DSA for quantum resistance. |
| Short-lived | ATCs expire in minutes, not days. Reduces revocation window. |
| Capability-scoped | ATC declares what the agent can do. Service checks before acting. |
| Fail closed | Invalid, expired, or revoked ATC = 401. No fallback. |

### 1.2 Terminology

| Term | Definition |
|------|-----------|
| **ATC** | Agent Trust Certificate. A signed, time-bounded, capability-scoped identity assertion. |
| **Issuer** | The AIM instance that issued the ATC. Identified by its public key and issuer URI. |
| **CRL** | Certificate Revocation List. Cached locally, refreshed every 5 minutes. |
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

A service receiving an `Authorization: ATC` header MUST execute the following steps in order. If any step fails, return HTTP 401 with the corresponding error code.

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
8. CRL:       Check atcId against cached CRL → 401 (atc_revoked).
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

| Error Code | HTTP Status | Meaning |
|-----------|------------|---------|
| `atc_malformed` | 401 | Cannot decode or parse ATC |
| `atc_oversized` | 401 | ATC exceeds 8 KiB |
| `atc_expired` | 401 | ATC is expired or not yet valid |
| `atc_untrusted_issuer` | 401 | Issuer not in trusted list |
| `atc_signature_invalid` | 401 | Ed25519 signature verification failed |
| `atc_pqc_signature_invalid` | 401 | ML-DSA signature verification failed |
| `atc_revoked` | 401 | ATC ID found on CRL |
| `atc_chain_invalid` | 401 | Delegation chain verification failed |
| `atc_insufficient_scope` | 403 | ATC lacks required capability |

---

## 5. Certificate Revocation List (CRL)

### 5.1 CRL Endpoint

```
GET https://registry.opena2a.org/api/v1/crl/latest
```

Response:

```json
{
  "version": 42,
  "generatedAt": "2026-04-13T05:00:00Z",
  "expiresAt": "2026-04-13T05:05:00Z",
  "revokedATCs": [
    {
      "atcId": "550e8400-e29b-41d4-a716-446655440000",
      "revokedAt": "2026-04-13T04:58:00Z",
      "reason": "agent_compromised"
    }
  ]
}
```

### 5.2 Caching

| Parameter | Value |
|-----------|-------|
| Cache TTL | 5 minutes |
| Refresh strategy | Background refresh at 4 minutes (before expiry) |
| Stale-while-revalidate | Serve stale CRL for up to 1 minute if refresh fails |
| Maximum stale age | 6 minutes (hard limit, fail closed after) |

### 5.3 Revocation Reasons

| Reason | Meaning |
|--------|---------|
| `agent_compromised` | Agent's private key may be exposed |
| `agent_deregistered` | Agent has been removed from AIM |
| `issuer_rotated` | Issuer key rotated, all outstanding ATCs invalid |
| `manual_revocation` | Administrative revocation |

---

## 6. Capability Scoping

### 6.1 Capability Format

Capabilities use colon-separated hierarchical scopes:

```
<domain>:<action>[:<resource>]
```

Examples:
- `secrets:resolve` — resolve any credential
- `secrets:resolve:github` — resolve credentials in the "github" namespace only
- `scan:read` — read scan results
- `registry:write` — write to registry
- `agent:manage` — manage agent lifecycle

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
- CRL refresh every 5 minutes
- Agents should request new ATCs per-operation, not reuse

### 8.3 Issuer Compromise

If an issuer's private key is compromised:
1. Issue `issuer_rotated` CRL entry for all outstanding ATCs
2. Rotate issuer key pair
3. Update trusted issuer configuration on all services
4. Re-issue ATCs for all active agents with new key

### 8.4 Quantum Resistance

ML-DSA signatures are optional in v1 but RECOMMENDED. Services MUST verify ML-DSA signatures when present. A future spec version may require ML-DSA.

---

## 9. Implementation Notes

### 9.1 JWT Shim Compatibility

The AIM backend currently uses a JWT-based ATC shim (`jwt_shim.go`). During migration:
- Services accept both `Authorization: Bearer <jwt>` and `Authorization: ATC <base64url-atc>`
- JWT path continues to work for agents that haven't upgraded
- ATC path takes precedence when both are possible

### 9.2 Performance

| Operation | Target |
|-----------|--------|
| ATC verification (Ed25519 only) | < 100 microseconds |
| ATC verification (Ed25519 + ML-DSA-65) | < 5 milliseconds |
| CRL lookup | < 1 microsecond (in-memory map) |
| Full resolve with ATC | < 10 milliseconds p99 |

### 9.3 Go Implementation Reference

```go
// ATCVerifier interface (already exists in domain/atc/verifier.go)
type ATCVerifier interface {
    Verify(rawToken string) (*ATCClaims, error)
    IsRevoked(atcID string) (bool, error)
}

// New: ATCIssuer for creating real ATCs
type ATCIssuer interface {
    Issue(agentID uuid.UUID, capabilities []string, ttl time.Duration) ([]byte, error)
}
```

---

## 10. Conformance

A service is ATC-v1 conformant if it:
1. Accepts `Authorization: ATC` headers
2. Implements all 10 verification steps from Section 4.1
3. Returns standardized error codes from Section 4.2
4. Caches CRL per Section 5.2
5. Emits `X-ATC-Supported: true` and `X-ATC-Version: 1.0` headers
6. Fails closed on any verification error
