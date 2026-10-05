# PQC request contract fixtures

One file per PQC write route. Each holds the request body the Python SDK
sends and the backend reads:

| File | SDK method | Route | Backend request type |
|---|---|---|---|
| `register_pqc_key.json` | `AIMClient.register_pqc_key` | `POST /api/v1/agents/{id}/pqc-key` | `RegisterPQCKeyRequest` |
| `rotate_pqc_key.json` | `AIMClient.rotate_pqc_key` | `PUT /api/v1/agents/{id}/pqc-key` | `RotatePQCKeyRequest` |
| `set_hybrid_mode.json` | `AIMClient.set_hybrid_mode` | `POST /api/v1/agents/{id}/hybrid-mode` | `EnableHybridModeRequest` |

Two tests read the same files, so renaming a member on either side fails a test:

- `sdk/python/tests/test_pqc_request_contract.py` asserts the SDK sends exactly `body`.
- `apps/backend/internal/interfaces/http/handlers/pqc_request_contract_test.go`
  decodes `body` into the request type, rejecting unknown members, requires
  every member of the type to be present, and posts `body` to the handler.

The public keys are 1952 bytes of fixed filler, the ML-DSA-65 public key size.
They are not real keys: both routes check only the base64 encoding and the size.
