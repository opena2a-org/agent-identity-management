# 🔐 Authentication Guide - Ed25519 Cryptography

Understanding how AIM secures your agents with military-grade cryptography.

## What is Ed25519?

**Ed25519** is a modern, high-security public-key signature system that:
- ✅ **Military-Grade Security**: Used by Signal, WhatsApp, SSH
- ✅ **Fast**: 64,000+ signatures per second
- ✅ **Small Keys**: 32-byte keys (256 bits)
- ✅ **Collision-Resistant**: Practically impossible to forge signatures
- ✅ **No Known Vulnerabilities**: Unlike RSA, immune to quantum attacks (with caveats)

**Why Ed25519?**
- **RSA-2048**: Old, slow, large keys (2048 bits)
- **Ed25519**: Modern, fast, small keys (256 bits), same security level

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

# Auto-generates Ed25519 keypair
agent = secure("my-agent")

print(f"Agent ID: {agent.id}")
print(f"Public Key: {agent.public_key}")
# Private key is kept secret, never transmitted
```

### 2. Challenge-Response Authentication

Every time your agent performs an action, it proves its identity:

```
┌─────────────┐                    ┌─────────────┐
│   Agent     │                    │  AIM Server │
└─────────────┘                    └─────────────┘
      │                                   │
      │  1. Request to verify action      │
      ├──────────────────────────────────>│
      │                                   │
      │  2. Generate random challenge     │
      │     Challenge: "abc123xyz"        │
      │<──────────────────────────────────┤
      │                                   │
      │  3. Sign challenge with           │
      │     private key                   │
      │     Signature = sign(challenge,   │
      │                      private_key) │
      │                                   │
      │  4. Send signature                │
      ├──────────────────────────────────>│
      │                                   │
      │  5. Verify signature with         │
      │     public key                    │
      │     verify(challenge, signature,  │
      │            public_key) → valid?   │
      │                                   │
      │  6. If valid: Action approved     │
      │<──────────────────────────────────┤
      │                                   │
```

**Why Challenge-Response?**
- **Prevents Replay Attacks**: Old signatures can't be reused
- **Proves Identity**: Only holder of private key can create valid signature
- **No Password Transmission**: Private key never leaves your machine

---

## Key Management

### Generating Keys

#### Option 1: Auto-Generate (Easiest)

```python
from aim_sdk import secure

# AIM generates keypair automatically
agent = secure("my-agent")

# Private key is stored securely in environment
print(f"Store this private key: {agent.private_key}")
```

**⚠️ WARNING**: Save the private key! It's only shown once.

```bash
# Save to environment variable
export AIM_PRIVATE_KEY="<your-private-key>"

# Or save to .env file
echo "AIM_PRIVATE_KEY=<your-private-key>" >> .env
```

#### Option 2: Generate Manually

```python
from aim_sdk.crypto import generate_keypair

# Generate Ed25519 keypair
private_key, public_key = generate_keypair()

print(f"Private Key: {private_key}")  # Keep secret!
print(f"Public Key: {public_key}")    # Share with AIM

# Save private key securely
with open("~/.aim/private_key.pem", "w") as f:
    f.write(private_key)

# Use with agent
agent = secure("my-agent", private_key=private_key)
```

#### Option 3: Use Existing Key

```python
from aim_sdk import secure
import os

# Load from environment
private_key = os.getenv("AIM_PRIVATE_KEY")

agent = secure("my-agent", private_key=private_key)
```

---

### Storing Keys Securely

#### ✅ GOOD: Environment Variables

```bash
# .env file (NOT committed to git)
AIM_PRIVATE_KEY=302e020100300506032b657004220420...

# Load in Python
from dotenv import load_dotenv
load_dotenv()

private_key = os.getenv("AIM_PRIVATE_KEY")
agent = secure("my-agent", private_key=private_key)
```

#### ✅ GOOD: Secret Management System

```python
# AWS Secrets Manager
import boto3

secrets = boto3.client("secretsmanager")
response = secrets.get_secret_value(SecretId="aim/private-key")
private_key = response["SecretString"]

agent = secure("my-agent", private_key=private_key)
```

```python
# Azure Key Vault
from azure.identity import DefaultAzureCredential
from azure.keyvault.secrets import SecretClient

credential = DefaultAzureCredential()
client = SecretClient(vault_url="https://myvault.vault.azure.net/", credential=credential)
private_key = client.get_secret("aim-private-key").value

agent = secure("my-agent", private_key=private_key)
```

#### ❌ BAD: Hardcoded in Code

```python
# ❌ NEVER DO THIS
agent = secure(
    "my-agent",
    private_key="302e020100300506032b657004220420..."  # Hardcoded!
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

Rotate keys regularly for security.

### Step 1: Generate New Keypair

```python
from aim_sdk.crypto import generate_keypair

# Generate new keypair
new_private_key, new_public_key = generate_keypair()

print(f"New Private Key: {new_private_key}")
print(f"New Public Key: {new_public_key}")
```

### Step 2: Update Agent in AIM Dashboard

1. **Login** to AIM Dashboard: http://localhost:3000
2. **Navigate**: Agents → Your Agent → Settings
3. **Click**: "Rotate Keys"
4. **Paste**: New public key
5. **Save**: Confirm rotation

### Step 3: Update Your Application

```python
# Update environment variable
# export AIM_PRIVATE_KEY="<new-private-key>"

# Or update secret in secret manager
secrets.put_secret_value(
    SecretId="aim/private-key",
    SecretString=new_private_key
)

# Restart application to load new key
```

### Automated Key Rotation

```python
from aim_sdk import secure
from aim_sdk.crypto import generate_keypair
from datetime import datetime, timedelta

def rotate_key_if_needed(agent):
    """Rotate key every 90 days"""
    key_age = datetime.now() - agent.key_created_at

    if key_age > timedelta(days=90):
        print("Key is older than 90 days, rotating...")

        # Generate new keypair
        new_private_key, new_public_key = generate_keypair()

        # Update agent
        agent.rotate_key(new_public_key)

        # Save new private key
        save_to_secret_manager("aim/private-key", new_private_key)

        print("✅ Key rotated successfully")

# Check daily
rotate_key_if_needed(agent)
```

---

## Signing and Verifying

### How Signatures Work

```python
from aim_sdk.crypto import sign, verify

# Data to sign
message = "delete_user(id=12345)"

# Sign with private key
signature = sign(message, private_key)
print(f"Signature: {signature}")

# Verify with public key
is_valid = verify(message, signature, public_key)
print(f"Valid: {is_valid}")  # True

# Tampered message fails verification
is_valid = verify("delete_user(id=99999)", signature, public_key)
print(f"Valid: {is_valid}")  # False
```

### Timestamp Signing (Prevents Replay Attacks)

```python
from aim_sdk.crypto import sign_with_timestamp, verify_with_timestamp
from datetime import datetime

# Sign with current timestamp
message = "delete_user(id=12345)"
signature, timestamp = sign_with_timestamp(message, private_key)

print(f"Signature: {signature}")
print(f"Timestamp: {timestamp}")

# Verify signature (must be recent)
is_valid = verify_with_timestamp(
    message,
    signature,
    timestamp,
    public_key,
    max_age_seconds=300  # Must be within 5 minutes
)

print(f"Valid: {is_valid}")  # True if within 5 minutes

# Old signatures are rejected
is_valid = verify_with_timestamp(
    message,
    signature,
    timestamp - timedelta(minutes=10),  # 10 minutes old
    public_key,
    max_age_seconds=300
)

print(f"Valid: {is_valid}")  # False (too old)
```

---

## Security Best Practices

### 1. Never Share Private Keys

```python
# ✅ GOOD
print(f"Agent ID: {agent.id}")
print(f"Public Key: {agent.public_key}")  # Safe to share

# ❌ BAD
print(f"Private Key: {agent.private_key}")  # NEVER print/log!
```

### 2. Use Different Keys Per Environment

```python
# ✅ GOOD - Separate keys per environment
if os.getenv("ENVIRONMENT") == "production":
    private_key = os.getenv("AIM_PROD_PRIVATE_KEY")
elif os.getenv("ENVIRONMENT") == "staging":
    private_key = os.getenv("AIM_STAGING_PRIVATE_KEY")
else:
    private_key = os.getenv("AIM_DEV_PRIVATE_KEY")

agent = secure("my-agent", private_key=private_key)
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

agent = secure("my-agent", private_key=get_private_key())
```

### 5. Implement Key Compromise Detection

```python
# ✅ GOOD - Monitor for suspicious activity
def check_for_key_compromise(agent):
    """Check for signs of compromised key"""
    trust_score = agent.get_trust_score(detailed=True)

    # Check for anomalies
    if trust_score["factors"]["drift_detection"] < 0.50:
        send_security_alert("Possible key compromise detected")
        rotate_key_immediately(agent)

    # Check for failed verifications
    audit_logs = agent.get_audit_logs(limit=100)
    failed_verifications = [log for log in audit_logs if log["status"] == "failed"]

    if len(failed_verifications) > 10:
        send_security_alert("High rate of failed verifications")
        investigate_and_rotate_if_needed(agent)
```

---

## Advanced Topics

### Multi-Signature Support

Use multiple keys for critical operations.

```python
from aim_sdk.crypto import multisig_sign, multisig_verify

# Generate 3 keypairs
keys = [generate_keypair() for _ in range(3)]
private_keys = [key[0] for key in keys]
public_keys = [key[1] for key in keys]

# Sign with 2-of-3 threshold
message = "transfer $1,000,000"
signatures = [sign(message, key) for key in private_keys[:2]]

# Verify (requires 2 signatures)
is_valid = multisig_verify(
    message,
    signatures,
    public_keys,
    threshold=2
)

print(f"Valid: {is_valid}")  # True
```

### Hardware Security Modules (HSM)

Use HSM for production key storage.

```python
from aim_sdk.crypto import HSMSigner

# Initialize HSM signer
signer = HSMSigner(
    hsm_endpoint="https://hsm.yourcompany.com",
    key_id="aim-agent-key-001",
    credentials=credentials
)

# Sign using HSM (private key never leaves HSM)
signature = signer.sign("delete_user(id=12345)")

# Use with AIM
agent = secure("my-agent", signer=signer)
```

---

## Troubleshooting

### Issue: "Invalid signature"

**Error**: `AIMAuthenticationError: Signature verification failed`

**Causes**:
1. Wrong private key being used
2. Message was tampered with
3. Clock skew (timestamp too old/new)

**Solution**:
```python
# Verify you're using correct key
print(f"Agent ID: {agent.id}")
print(f"Public Key: {agent.public_key}")

# Check in AIM dashboard that public key matches

# Check clock synchronization
from datetime import datetime
print(f"System time: {datetime.now()}")
# Ensure within 5 minutes of actual time
```

### Issue: "Key not found"

**Error**: `AIMAuthenticationError: No private key found`

**Solution**:
```python
# Check environment variable is set
import os
private_key = os.getenv("AIM_PRIVATE_KEY")

if private_key is None:
    print("❌ AIM_PRIVATE_KEY not set!")
    print("Set it with: export AIM_PRIVATE_KEY='your-key'")
else:
    print("✅ AIM_PRIVATE_KEY is set")
```

### Issue: "Weak key detected"

**Error**: `AIMAuthenticationError: Key strength insufficient`

**Cause**: Using non-Ed25519 key or corrupted key

**Solution**:
```python
# Generate new Ed25519 keypair
from aim_sdk.crypto import generate_keypair

private_key, public_key = generate_keypair()

# Verify key format
print(f"Private key length: {len(private_key)}")  # Should be 64-88 chars
print(f"Public key length: {len(public_key)}")    # Should be 64-88 chars
```

---

## Compliance & Auditing

### SOC 2 Requirements

AIM's Ed25519 authentication meets SOC 2 Type II requirements:

1. **CC6.1 - Logical Access Security**
   - ✅ Cryptographic authentication (Ed25519)
   - ✅ No password-based authentication
   - ✅ Multi-factor authentication support

2. **CC6.2 - Access Authorization**
   - ✅ Agent identity verification
   - ✅ Immutable audit trail
   - ✅ Challenge-response prevents impersonation

3. **CC6.7 - Data Encryption**
   - ✅ Ed25519 signatures (256-bit security)
   - ✅ TLS for data in transit
   - ✅ No plaintext credentials

### HIPAA Compliance

```python
# Export HIPAA compliance report
report = agent.export_compliance_report(
    report_type="hipaa",
    start_date="2025-10-01T00:00:00Z",
    end_date="2025-10-31T23:59:59Z",
    format="pdf"
)

# Report includes:
# - All authentication events
# - Failed authentication attempts
# - Key rotation history
# - Access patterns and anomalies
```

### GDPR Requirements

```python
# Right to Access (GDPR Article 15)
audit_logs = agent.get_audit_logs(
    start_date="2025-01-01T00:00:00Z"
)

# Export for user
print("All authentication events for user:")
for log in audit_logs:
    print(f"{log['timestamp']} - {log['action_name']}")
```

---

## Examples

### Complete Authentication Flow

```python
from aim_sdk import secure
from aim_sdk.crypto import generate_keypair
import os

# 1. Generate keypair (first time only)
if not os.getenv("AIM_PRIVATE_KEY"):
    print("No existing key found. Generating new keypair...")
    private_key, public_key = generate_keypair()

    print(f"✅ Keypair generated!")
    print(f"Public Key: {public_key}")
    print(f"Private Key: {private_key}")
    print()
    print("⚠️  Save your private key securely:")
    print(f"export AIM_PRIVATE_KEY='{private_key}'")
    exit(0)

# 2. Load private key from environment
private_key = os.getenv("AIM_PRIVATE_KEY")

# 3. Secure agent
agent = secure(
    name="production-agent",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=private_key
)

print(f"✅ Agent secured!")
print(f"Agent ID: {agent.id}")
print(f"Public Key: {agent.public_key}")
print(f"Trust Score: {agent.get_trust_score()}")

# 4. All actions are now authenticated
@agent.track_action(risk_level="low")
def get_data(id: int):
    # This action is automatically:
    # 1. Verified with challenge-response
    # 2. Signed with Ed25519
    # 3. Logged to audit trail
    return {"data": f"Data for {id}"}

# Use your agent
result = get_data(12345)
print(f"Result: {result}")
```

---

## Next Steps

- **[Auto-Detection Guide →](./auto-detection.md)** - Automatic MCP server discovery
- **[Trust Scoring Guide →](./trust-scoring.md)** - 8-factor trust algorithm
- **[Python SDK Guide →](./python.md)** - Complete SDK reference

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
