# 🔐 Authentication Guide - Ed25519 Cryptography

How the Python SDK proves an agent's identity to AIM.

## What is Ed25519?

**Ed25519** is a modern public-key signature system:
- **Widely deployed**: Used by Signal, WhatsApp, SSH
- **Fast**: Signing and verification take microseconds
- **Small Keys**: 32-byte keys (256 bits)
- **Unforgeable**: Without the private key, a valid signature cannot be produced

**Quantum attacks:** Ed25519, like RSA, is not resistant to quantum attacks. A large enough quantum computer running Shor's algorithm could recover a private key from its public key. For signatures designed to resist that, see [Post-Quantum Cryptography](../guides/PQC.md) (ML-DSA, alone or combined with Ed25519).

**Why Ed25519?**
- **RSA-2048**: Old, slow, large keys (2048 bits)
- **Ed25519**: Modern, fast, small keys (256 bits), comparable security level

---

## How Authentication Works

### 1. Agent Registration (One-Time Setup)

```
┌─────────────┐                    ┌─────────────┐
│   Agent     │                    │  AIM Server │
└─────────────┘                    └─────────────┘
      │                                   │
      │  1. Generate Ed25519 keypair      │
      │     Private Key: [secret]         │
      │     Public Key: [public]          │
      │                                   │
      │  2. Register agent with           │
      │     public key                    │
      ├──────────────────────────────────>│
      │                                   │
      │  3. Store public key in database  │
      │     Agent ID: abc-123             │
      │     Public Key: [public]          │
      │<──────────────────────────────────┤
      │                                   │
```

**Code Example**:
```python
from aim_sdk import secure

# Generates an Ed25519 keypair locally on first registration
agent = secure("my-agent")

print(f"Agent ID: {agent.agent_id}")
print(f"Public Key: {agent.public_key}")
# The private key never leaves this machine
```

### 2. Signed Requests

There is no separate challenge round trip. Every request the SDK sends to AIM
carries a fresh signature in four headers:

| Header | Value |
|--------|-------|
| `X-Agent-ID` | The agent's ID |
| `X-Public-Key` | The agent's Ed25519 public key (base64) |
| `X-Timestamp` | Current Unix time in seconds |
| `X-Signature` | Ed25519 signature (base64) of the message below |

The signed message is the request's method, path (with its query string),
timestamp and, when there is one, the JSON body with sorted keys, joined by
newlines:

```
POST
/api/v1/agents/abc-123/verify-capability
1760140800
{"capability": "db:read", ...}
```

The server rebuilds the same message, checks the signature against the public
key it stored at registration, and refuses a timestamp more than 30 seconds
from its own clock.

**What this gives you**
- **Proves Identity**: Only the holder of the private key can create a valid signature
- **Limits Replay**: A captured request stops being accepted once its timestamp is out of range
- **Binds the Request**: Changing the method, path, query or body breaks the signature
- **No Password Transmission**: The private key never leaves your machine

---

## Key Management

### Generating Keys

#### Option 1: Let `secure()` Generate Them

`secure()` saves each agent's credentials, including its private key, in
`~/.aim/agents/<name>.json`. The directory is created with mode `0700` and
the file with mode `0600`, so other users on the machine cannot read them.
The next `secure("my-agent")` loads the saved keys instead of registering
again.

The SDK's own credentials, which `secure()` registers the agent with, are in
`~/.aim/sdk_credentials.json` after `aim-sdk login`. With the SDK downloaded
from the dashboard, they are in `.aim/sdk_credentials.json` in the download.

#### Option 2: Use the Keypair `secure()` Saved

`secure()` takes no private key. When it registers an agent, it saves the agent ID and keypair to `~/.aim/agents/<name>.json`. To sign with a keypair, pass it to `AIMClient` together with the agent ID and public key.

```python
from aim_sdk import AIMClient, load_agent_credentials

# Saved by secure("my-agent") in ~/.aim/agents/my-agent.json
creds = load_agent_credentials("my-agent")

print(f"Agent ID: {creds['agent_id']}")
print(f"Public Key: {creds['public_key']}")  # Share with AIM
# creds["private_key"] is the secret: never print or log it

# Use with agent
agent = AIMClient(
    agent_id=creds["agent_id"],
    public_key=creds["public_key"],
    private_key=creds["private_key"],
    aim_url=creds["aim_url"]
)
```

#### Option 3: Use Existing Key

```python
from aim_sdk import AIMClient
import os

# Load from environment
private_key = os.getenv("AIM_PRIVATE_KEY")

agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=private_key,
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

---

### Storing Keys Securely

#### ✅ GOOD: Environment Variables

```bash
# .env file (NOT committed to git)
AIM_AGENT_ID=550e8400-e29b-41d4-a716-446655440000
AIM_PUBLIC_KEY=<base64 public key>
AIM_PRIVATE_KEY=<base64 private key>

# Load in Python
from dotenv import load_dotenv
load_dotenv()

private_key = os.getenv("AIM_PRIVATE_KEY")
agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=private_key,
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

#### ✅ GOOD: Secret Management System

```python
# AWS Secrets Manager
import boto3

secrets = boto3.client("secretsmanager")
response = secrets.get_secret_value(SecretId="aim/private-key")
private_key = response["SecretString"]

agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=private_key,
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

```python
# Azure Key Vault
from azure.identity import DefaultAzureCredential
from azure.keyvault.secrets import SecretClient

credential = DefaultAzureCredential()
client = SecretClient(vault_url="https://myvault.vault.azure.net/", credential=credential)
private_key = client.get_secret("aim-private-key").value

agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=private_key,
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

#### ❌ BAD: Hardcoded in Code

```python
# ❌ NEVER DO THIS
agent = AIMClient(
    agent_id="550e8400-e29b-41d4-a716-446655440000",
    public_key="hardcoded-public-key",
    private_key="hardcoded-private-key",  # Hardcoded!
    aim_url="http://localhost:8080"
)
```

#### ❌ BAD: Committed to Git

```python
# ❌ NEVER COMMIT .env FILES
# Add to .gitignore:
.env
.env.local
*.pem
private_key.txt
```

---

## Key Rotation

The Python SDK has no key rotation call. Rotate an agent's Ed25519 keys from
the dashboard, or with `POST /api/v1/agents/{id}/rotate-credentials` and a
member's access token. The server generates the new keypair and keeps the
previous public key for a grace period; download the agent's credentials again
afterwards so the SDK signs with the new key.

For post-quantum keys the SDK does have calls: `register_pqc_key()`,
`rotate_pqc_key()` and `set_hybrid_mode()`. See the
[Post-Quantum Cryptography guide](../guides/PQC.md).

---

## Signing and Verifying

The SDK signs every request for you; you do not call a signing function to use
AIM. To sign or verify data yourself, `aim_sdk.crypto` provides ML-DSA and
hybrid (Ed25519 + ML-DSA) functions. They need `liboqs-python`;
`is_pqc_available()` says whether it is installed.

```python
from aim_sdk.crypto import (
    generate_mldsa_keypair,
    is_pqc_available,
    sign_mldsa,
    verify_mldsa,
)

if is_pqc_available():
    keypair = generate_mldsa_keypair()

    message = b"delete_user(id=12345)"
    signature = sign_mldsa(keypair.private_key, message)

    print(verify_mldsa(keypair.public_key, message, signature))                    # True
    print(verify_mldsa(keypair.public_key, b"delete_user(id=99999)", signature))   # False
```

For Ed25519 alone, use PyNaCl (an SDK dependency) directly:

```python
from nacl.signing import SigningKey

signing_key = SigningKey.generate()
signed = signing_key.sign(b"delete_user(id=12345)")
signing_key.verify_key.verify(signed)  # raises BadSignatureError if tampered
```

---

## Security Best Practices

### 1. Never Share Private Keys

```python
# ✅ GOOD
print(f"Agent ID: {agent.agent_id}")
print(f"Public Key: {agent.public_key}")  # Safe to share
```

Never print, log or paste the contents of `~/.aim/agents/<name>.json`.

### 2. Use Different Keys Per Environment

```python
# ✅ GOOD - Separate keys per environment
# (one agent per environment: its ID, public key and private key belong together)
if os.getenv("ENVIRONMENT") == "production":
    prefix = "AIM_PROD"
elif os.getenv("ENVIRONMENT") == "staging":
    prefix = "AIM_STAGING"
else:
    prefix = "AIM_DEV"

agent = AIMClient(
    agent_id=os.getenv(f"{prefix}_AGENT_ID"),
    public_key=os.getenv(f"{prefix}_PUBLIC_KEY"),
    private_key=os.getenv(f"{prefix}_PRIVATE_KEY"),
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

### 3. Rotate Keys Regularly

```python
# ✅ GOOD - Rotate every 90 days
key_rotation_schedule = {
    "development": timedelta(days=30),
    "staging": timedelta(days=60),
    "production": timedelta(days=90)
}

rotate_if_older_than = key_rotation_schedule[os.getenv("ENVIRONMENT")]
```

### 4. Use Secret Management

```python
# ✅ GOOD - Use AWS Secrets Manager
import boto3

def get_private_key():
    secrets = boto3.client("secretsmanager")
    response = secrets.get_secret_value(SecretId="aim/private-key")
    return response["SecretString"]

agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=get_private_key(),
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)
```

### 5. Watch for Signs of Key Compromise

```python
# ✅ GOOD - Alert when the trust score drops
def check_for_key_compromise(agent):
    result = agent.get_trust_score()

    if result["factors"]["securityAlerts"] < 0.50:
        send_security_alert("Security alerts are lowering this agent's trust score")

    history = agent.get_trust_score_history(limit=10)
    drops = [
        entry for entry in history["history"]
        if entry.get("previousScore") is not None
        and entry["previousScore"] - entry["trustScore"] > 0.10
    ]
    if drops:
        send_security_alert(f"{len(drops)} trust score drops of more than 0.10")
```

If you suspect a key is compromised, rotate it (see [Key Rotation](#key-rotation))
or suspend the agent in the dashboard.

---

## Troubleshooting

### Issue: "Authentication failed"

**Error**: `AuthenticationError: Authentication failed - invalid agent credentials`

The server answered 401: the signature did not verify, the timestamp was out of
range, or the agent is unknown.

**Causes**:
1. The saved keys do not match the public key AIM holds (for example after a rotation)
2. The system clock is more than 30 seconds off
3. The agent was deleted or its credentials revoked

**Solution**:
```python
# Compare with the agent's public key in the AIM dashboard
print(f"Agent ID: {agent.agent_id}")
print(f"Public Key: {agent.public_key}")

# Check clock synchronization
from datetime import datetime, timezone
print(f"System time (UTC): {datetime.now(timezone.utc)}")
```

If the keys no longer match, register again with `secure("my-agent", force_new=True)`.

### Issue: "Forbidden"

**Error**: `AuthenticationError: Forbidden - insufficient permissions`

The server answered 403: the credentials are valid but the route needs a role
the agent does not have (several management routes accept only a member's or
admin's access token).

---

## Examples

### Complete Authentication Flow

```python
from aim_sdk import AIMClient, load_agent_credentials, secure
import os

# 1. Register and generate the keypair (first time only)
if not os.getenv("AIM_PRIVATE_KEY"):
    print("No existing key found. Registering the agent...")
    secure("production-agent", aim_url=os.getenv("AIM_URL", "http://localhost:8080"))
    creds = load_agent_credentials("production-agent")

    print(f"✅ Keypair saved to ~/.aim/agents/production-agent.json")
    print(f"Agent ID: {creds['agent_id']}")
    print(f"Public Key: {creds['public_key']}")
    print()
    print("⚠️  Move the agent ID and keypair into your environment or secret manager:")
    print(f"export AIM_AGENT_ID='{creds['agent_id']}'")
    print(f"export AIM_PUBLIC_KEY='{creds['public_key']}'")
    print("export AIM_PRIVATE_KEY=<private_key in ~/.aim/agents/production-agent.json>")
    exit(0)

# 2. Load private key from environment
private_key = os.getenv("AIM_PRIVATE_KEY")

# 3. Secure agent
agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=private_key,
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
)

print(f"✅ Agent secured!")
print(f"Agent ID: {agent.agent_id}")
print(f"Public Key: {agent.public_key}")
print(f"Trust Score: {agent.get_trust_score()['score']:.2f}")

# 4. Every call below is sent as a signed request
@agent.perform_action(risk_level="low")
def get_data(id: int):
    # This action is automatically:
    # 1. Verified with AIM in a signed request
    # 2. Logged to the audit trail
    return {"data": f"Data for {id}"}

# Use your agent
result = get_data(12345)
print(f"Result: {result}")
```

---

## Next Steps

- **[Auto-Detection Guide →](./auto-detection.md)** - Automatic MCP server discovery
- **[Trust Scoring Guide →](./trust-scoring.md)** - 9-factor trust algorithm
- **[Python SDK Guide →](./python.md)** - Complete SDK reference

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
