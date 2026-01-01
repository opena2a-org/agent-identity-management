# Post-Quantum Cryptography (PQC) Support

AIM provides post-quantum cryptography support using NIST FIPS 204 compliant ML-DSA (Module-Lattice Digital Signature Algorithm, formerly Dilithium) signatures. This ensures your agent identities remain secure against future quantum computer attacks.

## Overview

Traditional cryptographic algorithms like Ed25519 and RSA may be vulnerable to attacks by sufficiently powerful quantum computers. AIM addresses this with:

- **Pure ML-DSA signatures**: Full post-quantum security using only ML-DSA
- **Hybrid Ed25519+ML-DSA signatures**: Defense-in-depth combining classical and post-quantum cryptography (recommended for transition period)

## Supported Algorithms

| Algorithm | Security Level | Use Case |
|-----------|----------------|----------|
| **ML-DSA-44** | NIST Level 2 (128-bit classical) | Smaller signatures, good for constrained environments |
| **ML-DSA-65** | NIST Level 3 (192-bit classical) | **Recommended** - balanced security and performance |
| **ML-DSA-87** | NIST Level 5 (256-bit classical) | Maximum security for highly sensitive applications |
| **Ed25519+ML-DSA-65** | Hybrid | **Recommended** - defense-in-depth during transition |

### Key and Signature Sizes

| Algorithm | Public Key | Private Key | Signature |
|-----------|------------|-------------|-----------|
| Ed25519 | 32 bytes | 32 bytes | 64 bytes |
| ML-DSA-44 | 1,312 bytes | 2,560 bytes | 2,420 bytes |
| ML-DSA-65 | 1,952 bytes | 4,032 bytes | 3,309 bytes |
| ML-DSA-87 | 2,592 bytes | 4,896 bytes | 4,627 bytes |

## Installation

### Python SDK

```bash
# Install liboqs-python for PQC support
pip install liboqs-python

# Or install AIM SDK with PQC extras
pip install aim-sdk[pqc]
```

**macOS users**: You may need to install liboqs system library:
```bash
brew install liboqs
```

### TypeScript SDK

```bash
npm install @opena2a/aim-sdk
```

PQC support is built-in using pure JavaScript implementation.

## Usage

### Python SDK

#### Check PQC Availability

```python
from aim_sdk.crypto.pqc import is_pqc_available, get_pqc_availability_info

# Simple check
if is_pqc_available():
    print("PQC support available!")

# Detailed info
info = get_pqc_availability_info()
print(f"Available: {info['available']}")
print(f"Version: {info.get('liboqs_version')}")
print(f"Algorithms: {info.get('enabled_sig_mechanisms')}")
```

#### Generate ML-DSA Keys

```python
from aim_sdk.crypto.pqc import generate_mldsa_keypair, Algorithm

# Generate ML-DSA-65 keypair (recommended)
keypair = generate_mldsa_keypair(Algorithm.MLDSA_65)

print(f"Public key (base64): {keypair.public_key_b64}")
print(f"Private key (base64): {keypair.private_key_b64}")
print(f"Algorithm: {keypair.algorithm}")
```

#### Generate Hybrid Keys

```python
from aim_sdk.crypto.pqc import generate_hybrid_keypair, Algorithm

# Generate Ed25519+ML-DSA-65 hybrid keypair (recommended)
keypair = generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_65)

# Access both key types
print(f"Ed25519 public key: {keypair.ed25519_public_key_b64}")
print(f"ML-DSA public key: {keypair.mldsa_public_key_b64}")
```

#### Sign and Verify with ML-DSA

```python
from aim_sdk.crypto.pqc import (
    generate_mldsa_keypair,
    sign_mldsa,
    verify_mldsa,
    Algorithm
)

# Generate keys
keypair = generate_mldsa_keypair(Algorithm.MLDSA_65)

# Sign a message
message = b"Agent action: db:read on customers table"
signature = sign_mldsa(keypair.private_key, message, keypair.algorithm)

# Verify the signature
is_valid = verify_mldsa(keypair.public_key, message, signature, keypair.algorithm)
print(f"Signature valid: {is_valid}")
```

#### Hybrid Signing (Recommended)

```python
from aim_sdk.crypto.pqc import (
    generate_hybrid_keypair,
    sign_hybrid,
    verify_hybrid,
    Algorithm
)

# Generate hybrid keypair
keypair = generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_65)

# Sign with both algorithms
message = b"Critical operation: payment:process"
signature = sign_hybrid(
    keypair.ed25519_private_key,
    keypair.mldsa_private_key,
    message,
    keypair.algorithm
)

print(f"Ed25519 signature: {signature.ed25519_signature_b64}")
print(f"ML-DSA signature: {signature.mldsa_signature_b64}")
print(f"Timestamp: {signature.timestamp}")

# Verify BOTH signatures (defense-in-depth)
is_valid, reason = verify_hybrid(
    keypair.ed25519_public_key,
    keypair.mldsa_public_key,
    message,
    signature.ed25519_signature,
    signature.mldsa_signature,
    keypair.algorithm
)

print(f"Valid: {is_valid}, Reason: {reason}")
```

### TypeScript SDK

```typescript
import {
  isPqcAvailable,
  generateMLDSAKeyPair,
  generateHybridKeyPair,
  signMLDSA,
  verifyMLDSA,
  signHybrid,
  verifyHybrid,
  Algorithm
} from '@opena2a/aim-sdk';

// Check availability
console.log('PQC available:', isPqcAvailable());

// Generate ML-DSA keypair
const mldsaKeypair = await generateMLDSAKeyPair(Algorithm.MLDSA_65);

// Generate hybrid keypair
const hybridKeypair = await generateHybridKeyPair(Algorithm.HYBRID_ED25519_MLDSA_65);

// Sign and verify
const message = new TextEncoder().encode('Agent action data');
const signature = await signHybrid(
  hybridKeypair.ed25519PrivateKey,
  hybridKeypair.mldsaPrivateKey,
  message,
  hybridKeypair.algorithm
);

const { isValid, reason } = await verifyHybrid(
  hybridKeypair.ed25519PublicKey,
  hybridKeypair.mldsaPublicKey,
  message,
  signature.ed25519Signature,
  signature.mldsaSignature,
  hybridKeypair.algorithm
);
```

## Backend PQC Agent Registration

Register an agent with PQC support via the API:

```bash
curl -X POST https://aim.example.com/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pqc-secure-agent",
    "displayName": "PQC Secure Agent",
    "publicKey": "BASE64_ED25519_PUBLIC_KEY",
    "pqcPublicKey": "BASE64_MLDSA65_PUBLIC_KEY",
    "pqcKeyAlgorithm": "ML-DSA-65",
    "hybridModeEnabled": true,
    "capabilities": ["db:read", "api:call"]
  }'
```

### Hybrid Mode Authentication

When `hybridModeEnabled` is true, the agent must provide both signatures:

```bash
# Challenge-response with hybrid signatures
curl -X POST https://aim.example.com/api/v1/agents/{id}/verify \
  -H "Content-Type: application/json" \
  -d '{
    "challenge": "CHALLENGE_FROM_SERVER",
    "signature": "ED25519_SIGNATURE",
    "pqcSignature": "MLDSA_SIGNATURE"
  }'
```

## Security Recommendations

### 1. Use Hybrid Mode During Transition

During the cryptographic transition period (now through ~2030), use hybrid Ed25519+ML-DSA:

- **Why**: If either algorithm is broken, the other provides protection
- **Ed25519**: Battle-tested, widely deployed, fast
- **ML-DSA**: Quantum-resistant, NIST standardized

### 2. Choose the Right Security Level

| Use Case | Recommended Algorithm |
|----------|----------------------|
| General purpose | ML-DSA-65 / Ed25519+ML-DSA-65 |
| Resource-constrained | ML-DSA-44 / Ed25519+ML-DSA-44 |
| Maximum security | ML-DSA-87 / Ed25519+ML-DSA-87 |
| Financial/healthcare | ML-DSA-87 / Ed25519+ML-DSA-87 |

### 3. Key Rotation

PQC keys should be rotated regularly:

```python
# Python SDK - Key rotation
from aim_sdk.crypto.pqc import generate_hybrid_keypair

# Generate new keys
new_keypair = generate_hybrid_keypair(Algorithm.HYBRID_ED25519_MLDSA_65)

# Update agent with new PQC key (API call)
client.update_agent_pqc_key(
    agent_id="agent-uuid",
    pqc_public_key=new_keypair.mldsa_public_key_b64,
    pqc_algorithm="ML-DSA-65"
)
```

### 4. Verify Both Signatures

In hybrid mode, **always verify both signatures**:

```python
is_valid, reason = verify_hybrid(...)

if not is_valid:
    # BOTH signatures must be valid
    raise SecurityError(f"Hybrid verification failed: {reason}")
```

## Performance Considerations

| Operation | Ed25519 | ML-DSA-65 | Hybrid |
|-----------|---------|-----------|--------|
| Key generation | ~0.1ms | ~1-2ms | ~2ms |
| Signing | ~0.1ms | ~2-3ms | ~3ms |
| Verification | ~0.2ms | ~1-2ms | ~2ms |

ML-DSA operations are ~10-20x slower than Ed25519, but still very fast for most applications.

## Error Handling

```python
from aim_sdk.crypto.pqc import (
    PQCError,
    PQCNotAvailableError,
    PQCSignatureError,
    PQCKeyError
)

try:
    signature = sign_mldsa(private_key, message, algorithm)
except PQCNotAvailableError:
    print("Install liboqs-python: pip install liboqs-python")
except PQCSignatureError as e:
    print(f"Signing failed: {e}")
except PQCKeyError as e:
    print(f"Invalid key: {e}")
except PQCError as e:
    print(f"PQC operation failed: {e}")
```

## References

- [NIST FIPS 204 (ML-DSA Standard)](https://csrc.nist.gov/pubs/fips/204/final)
- [Open Quantum Safe (liboqs)](https://openquantumsafe.org/)
- [Post-Quantum Cryptography Migration Guide](https://www.nist.gov/pqcrypto)
- [CISA Post-Quantum Cryptography Initiative](https://www.cisa.gov/quantum)
