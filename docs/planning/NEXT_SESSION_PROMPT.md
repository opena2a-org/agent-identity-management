# 🚀 Next Claude Session: Implement AIM Phase 1

## Context

You are continuing work on **Agent Identity Management (AIM)** - the Stripe of AI Agent Identity. The vision and architecture have been fully designed. Now it's time to implement Phase 1: Core Foundation.

## What's Already Done ✅

### Completed Features
- ✅ Backend API (Go + Fiber)
- ✅ Frontend UI (Next.js 15)
- ✅ Agent registration with automatic Ed25519 key generation
- ✅ SDK download with embedded credentials
- ✅ Python SDK with Ed25519 signing
- ✅ Private key format compatibility fix (Go 64-byte → Python 32-byte seed)
- ✅ All 18 SDK tests passing

### Completed Documentation
- ✅ **SEAMLESS_AUTO_REGISTRATION.md** - Auto-registration design
- ✅ **UNIVERSAL_INTEGRATION_STRATEGY.md** - Framework integrations strategy
- ✅ **CHALLENGE_RESPONSE_VERIFICATION.md** - Cryptographic verification design
- ✅ **AIM_COMPLETE_IMPLEMENTATION_ROADMAP.md** - Complete implementation plan

## Your Mission: Implement Phase 1 (Core Foundation)

### Phase 1 Overview
Build the **zero-friction, cryptographically secure** foundation:

1. **Auto-Registration**: 1-line agent registration (no UI needed)
2. **Challenge-Response**: Cryptographic proof of key possession
3. **SDK Integration**: Automatic verification on first API call
4. **Local Credentials**: Store keys in `~/.aim/credentials/`

### Priority Order (MUST FOLLOW THIS SEQUENCE)

#### Task 1: Auto-Registration Backend (4-6 hours) 🔥 START HERE

**Goal**: Implement `POST /api/v1/agents/auto-register` endpoint

**Files to modify:**
1. `apps/backend/internal/interfaces/http/handlers/agent_handler.go`
   - Add `AutoRegister(c fiber.Ctx) error` method
   - Parse request with `X-AIM-API-Key` header
   - Call `agentService.CreateAgentWithAutoKeys()`
   - Return credentials (agent_id, public_key, private_key)

2. `apps/backend/internal/application/agent_service.go`
   - Add `CreateAgentWithAutoKeys()` method
   - Generate Ed25519 keypair (reuse existing crypto package)
   - Encrypt and store private key
   - Return both public and private keys

3. `apps/backend/internal/application/auth_service.go`
   - Add `GetOrganizationFromAPIKey(ctx, apiKey)` method
   - For now, use hardcoded org (future: API key lookup)

4. `apps/backend/cmd/server/main.go`
   - Register route: `POST /api/v1/agents/auto-register`
   - No auth middleware (uses API key header instead)

**API Spec:**
```http
POST /api/v1/agents/auto-register
X-AIM-API-Key: {org_api_key}
Content-Type: application/json

{
  "name": "my-agent",
  "agent_type": "ai_agent",
  "description": "My AI assistant",
  "capabilities": ["read_database", "send_email"],
  "hostname": "laptop.local",
  "platform": "darwin",
  "sdk_version": "1.0.0",
  "auto_approve": false
}

Response 201:
{
  "agent_id": "uuid",
  "public_key": "base64...",
  "private_key": "base64...",  // Returned ONCE, never again
  "status": "pending",
  "message": "Agent registered successfully. Pending admin approval.",
  "organization_id": "uuid",
  "created_at": "2025-10-07T..."
}
```

**Testing:**
```bash
# Test with curl
curl -X POST http://localhost:8080/api/v1/agents/auto-register \
  -H "X-AIM-API-Key: test-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-auto-agent",
    "agent_type": "ai_agent",
    "auto_approve": false
  }'

# Verify in database
psql -d aim -c "SELECT id, name, public_key IS NOT NULL, encrypted_private_key IS NOT NULL FROM agents ORDER BY created_at DESC LIMIT 1;"
```

**Success criteria:**
- [ ] Endpoint returns HTTP 201 with credentials
- [ ] Agent stored in database with encrypted private key
- [ ] Public key returned in response matches database
- [ ] Private key can be used to sign/verify (test with Python SDK)

---

#### Task 2: Challenge-Response Backend (6-8 hours)

**Goal**: Implement cryptographic verification endpoints

**Database migration:**
```sql
-- Create file: apps/backend/migrations/016_challenges_table.up.sql
CREATE TABLE challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    nonce BYTEA NOT NULL,  -- 32 random bytes
    expires_at TIMESTAMPTZ NOT NULL,  -- 30 seconds from creation
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    verified_at TIMESTAMPTZ
);

CREATE INDEX idx_challenges_agent_id ON challenges(agent_id);
CREATE INDEX idx_challenges_expires_at ON challenges(expires_at);

ALTER TABLE agents
ADD COLUMN cryptographically_verified BOOLEAN DEFAULT FALSE,
ADD COLUMN verification_method VARCHAR(50),
ADD COLUMN last_verification_at TIMESTAMPTZ;
```

**Files to create:**
1. `apps/backend/internal/domain/challenge.go`
```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

type Challenge struct {
    ID         uuid.UUID  `json:"id"`
    AgentID    uuid.UUID  `json:"agent_id"`
    Nonce      []byte     `json:"-"`  // Never expose in JSON
    ExpiresAt  time.Time  `json:"expires_at"`
    Used       bool       `json:"used"`
    CreatedAt  time.Time  `json:"created_at"`
    VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

func (c *Challenge) IsExpired() bool {
    return time.Now().After(c.ExpiresAt)
}
```

2. `apps/backend/internal/infrastructure/repository/challenge_repository.go`
```go
package repository

type ChallengeRepository struct {
    db *sql.DB
}

func NewChallengeRepository(db *sql.DB) *ChallengeRepository {
    return &ChallengeRepository{db: db}
}

func (r *ChallengeRepository) Create(ctx context.Context, challenge *domain.Challenge) error {
    // INSERT INTO challenges ...
}

func (r *ChallengeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Challenge, error) {
    // SELECT * FROM challenges WHERE id = $1
}

func (r *ChallengeRepository) Update(ctx context.Context, challenge *domain.Challenge) error {
    // UPDATE challenges SET used = $1, verified_at = $2 WHERE id = $3
}
```

3. `apps/backend/internal/interfaces/http/handlers/agent_handler.go`
   - Add `RequestChallenge(c fiber.Ctx) error` method
   - Add `VerifyChallenge(c fiber.Ctx) error` method
   - See CHALLENGE_RESPONSE_VERIFICATION.md for full implementation

4. `apps/backend/cmd/server/main.go`
   - Register `GET /api/v1/agents/:id/challenge`
   - Register `POST /api/v1/agents/:id/verify-challenge`

**Testing:**
```bash
# 1. Request challenge
CHALLENGE=$(curl http://localhost:8080/api/v1/agents/{agent_id}/challenge)
echo $CHALLENGE | jq

# 2. Extract nonce and sign it
python3 << 'EOF'
import json, base64, sys
from nacl.signing import SigningKey

challenge = json.loads(sys.stdin.read())
nonce = base64.b64decode(challenge["nonce"])

# Use private key from auto-register response
private_key_b64 = "YOUR_PRIVATE_KEY_HERE"
private_key_bytes = base64.b64decode(private_key_b64)
seed = private_key_bytes[:32]  # Extract seed

signing_key = SigningKey(seed)
signature = signing_key.sign(nonce).signature
print(base64.b64encode(signature).decode())
EOF

# 3. Submit signature
curl -X POST http://localhost:8080/api/v1/agents/{agent_id}/verify-challenge \
  -H "Content-Type: application/json" \
  -d '{
    "challenge_id": "CHALLENGE_ID_FROM_STEP_1",
    "signature": "SIGNATURE_FROM_STEP_2"
  }'

# Expected: {"verified": true, "trust_score": 50, ...}

# 4. Verify in database
psql -d aim -c "SELECT id, name, cryptographically_verified, verification_method FROM agents WHERE id = 'AGENT_ID';"
```

**Success criteria:**
- [ ] Challenge endpoint returns nonce with 30s expiration
- [ ] Verify endpoint accepts valid Ed25519 signatures
- [ ] Invalid signatures are rejected
- [ ] Expired challenges are rejected
- [ ] Used challenges cannot be reused (replay protection)
- [ ] Agent marked as cryptographically verified after success

---

#### Task 3: Auto-Registration SDK (4-6 hours)

**Goal**: Implement `AIMClient.auto_register()` class method

**File to modify:**
`sdks/python/aim_sdk/client.py`

**Add these methods:**

```python
import os
import json
import socket
import sys
from pathlib import Path

class AIMClient:
    # ... existing code ...

    @classmethod
    def auto_register(
        cls,
        name: str,
        agent_type: str = "ai_agent",
        aim_url: Optional[str] = None,
        api_key: Optional[str] = None,
        auto_approve: bool = False,
        description: Optional[str] = None,
        capabilities: Optional[List[str]] = None,
        force_refresh: bool = False
    ) -> "AIMClient":
        """
        Automatically register agent or load existing credentials.

        First run: Registers with AIM, stores credentials locally
        Subsequent runs: Loads credentials from ~/.aim/credentials/{name}.json

        Args:
            name: Unique agent name
            agent_type: Type of agent (ai_agent, mcp_server, etc.)
            aim_url: AIM server URL (or from AIM_URL env var)
            api_key: Organization API key (or from AIM_API_KEY env var)
            auto_approve: Auto-approve in dev environments
            description: Agent description
            capabilities: List of actions agent can perform
            force_refresh: Force re-registration even if credentials exist

        Returns:
            Initialized AIMClient

        Raises:
            ConfigurationError: If AIM URL or API key not provided
            RegistrationError: If registration fails
        """
        # Get configuration from parameters or environment
        aim_url = aim_url or os.getenv("AIM_URL")
        if not aim_url:
            raise ConfigurationError(
                "AIM URL required. Provide via parameter or set AIM_URL environment variable."
            )

        api_key = api_key or os.getenv("AIM_API_KEY")
        if not api_key:
            raise ConfigurationError(
                "API key required. Provide via parameter or set AIM_API_KEY environment variable."
            )

        # Check for existing credentials
        creds_path = Path.home() / ".aim" / "credentials" / f"{name}.json"

        if creds_path.exists() and not force_refresh:
            # Load existing credentials
            print(f"📂 Loading credentials from {creds_path}")
            with open(creds_path) as f:
                creds = json.load(f)

            # Update last_used timestamp
            creds["last_used"] = datetime.now(timezone.utc).isoformat()
            with open(creds_path, "w") as f:
                json.dump(creds, f, indent=2)

            # Return initialized client
            return cls(
                agent_id=creds["agent_id"],
                public_key=creds["public_key"],
                private_key=creds["private_key"],
                aim_url=creds["aim_url"]
            )

        # First time - register with AIM
        print(f"🚀 Registering new agent '{name}' with AIM...")

        try:
            response = requests.post(
                f"{aim_url}/api/v1/agents/auto-register",
                headers={
                    "X-AIM-API-Key": api_key,
                    "Content-Type": "application/json"
                },
                json={
                    "name": name,
                    "agent_type": agent_type,
                    "description": description or f"Auto-registered agent: {name}",
                    "capabilities": capabilities or [],
                    "hostname": socket.gethostname(),
                    "platform": sys.platform,
                    "sdk_version": "1.0.0",
                    "auto_approve": auto_approve
                },
                timeout=30
            )

            if response.status_code == 409:
                # Agent already exists
                error_data = response.json()
                raise RegistrationError(
                    f"Agent '{name}' already exists (ID: {error_data.get('existing_agent_id')}). "
                    f"Use force_refresh=True to re-register or choose a different name."
                )

            response.raise_for_status()
            creds = response.json()

        except requests.exceptions.RequestException as e:
            raise RegistrationError(f"Failed to register agent: {e}")

        # Create credentials directory
        creds_path.parent.mkdir(parents=True, exist_ok=True)

        # Store credentials locally
        with open(creds_path, "w") as f:
            json.dump({
                "agent_id": creds["agent_id"],
                "agent_name": name,
                "public_key": creds["public_key"],
                "private_key": creds["private_key"],
                "organization_id": creds["organization_id"],
                "aim_url": aim_url,
                "status": creds["status"],
                "created_at": creds["created_at"],
                "last_used": datetime.now(timezone.utc).isoformat()
            }, f, indent=2)

        # Secure the file (user read-only)
        os.chmod(creds_path, 0o600)

        # Print success message
        print(f"✅ Agent '{name}' registered successfully!")
        print(f"   Agent ID: {creds['agent_id']}")
        print(f"   Status: {creds['status']}")
        if creds['status'] == 'pending':
            print(f"   ⏳ Pending admin approval")
        print(f"   📁 Credentials stored: {creds_path}")

        # Return initialized client
        return cls(
            agent_id=creds["agent_id"],
            public_key=creds["public_key"],
            private_key=creds["private_key"],
            aim_url=aim_url
        )
```

**Add new exception:**
```python
# In aim_sdk/exceptions.py
class RegistrationError(AIMException):
    """Raised when agent registration fails."""
    pass
```

**Testing:**
```python
# test_auto_register.py
import os
from aim_sdk import AIMClient

# Set environment variables
os.environ["AIM_URL"] = "http://localhost:8080"
os.environ["AIM_API_KEY"] = "test-api-key"

# First run - registers
print("=== First run (should register) ===")
client1 = AIMClient.auto_register("test-auto-agent")
print(f"Client 1 agent_id: {client1.agent_id}\n")

# Second run - loads from file
print("=== Second run (should load from file) ===")
client2 = AIMClient.auto_register("test-auto-agent")
print(f"Client 2 agent_id: {client2.agent_id}\n")

# Verify both clients have same agent_id
assert client1.agent_id == client2.agent_id
print("✅ Auto-registration works correctly!")

# Check credentials file exists
import pathlib
creds_file = pathlib.Path.home() / ".aim" / "credentials" / "test-auto-agent.json"
assert creds_file.exists()
print(f"✅ Credentials stored at: {creds_file}")
```

**Success criteria:**
- [ ] First run registers agent and stores credentials
- [ ] Second run loads credentials from file (no API call)
- [ ] Credentials file created with chmod 600
- [ ] Environment variables work (AIM_URL, AIM_API_KEY)
- [ ] Force refresh works
- [ ] Helpful console output for developers

---

#### Task 4: Challenge-Response SDK (4-6 hours)

**Goal**: Automatic cryptographic verification on first API call

**File to modify:**
`sdks/python/aim_sdk/client.py`

**Add these methods:**

```python
class AIMClient:
    def __init__(self, ...):
        # ... existing initialization ...
        self._cryptographically_verified = False  # NEW

    def _ensure_verified(self):
        """
        Ensures agent is cryptographically verified.
        Automatically performs challenge-response on first call.
        """
        if self._cryptographically_verified:
            return  # Already verified

        print("🔐 Performing cryptographic verification...")

        try:
            # Step 1: Request challenge
            challenge_response = self._make_request(
                method="GET",
                endpoint=f"/api/v1/agents/{self.agent_id}/challenge"
            )

            challenge_id = challenge_response["challenge_id"]
            nonce_b64 = challenge_response["nonce"]

            # Step 2: Sign nonce with private key
            nonce = base64.b64decode(nonce_b64)
            signature = self.signing_key.sign(nonce).signature
            signature_b64 = base64.b64encode(signature).decode('utf-8')

            # Step 3: Submit signature
            verify_response = self._make_request(
                method="POST",
                endpoint=f"/api/v1/agents/{self.agent_id}/verify-challenge",
                data={
                    "challenge_id": challenge_id,
                    "signature": signature_b64
                }
            )

            if verify_response.get("verified"):
                self._cryptographically_verified = True
                print(f"✅ Agent cryptographically verified!")
                print(f"   Trust Score: {verify_response.get('trust_score')}")
                print(f"   Verified At: {verify_response.get('verified_at')}")
            else:
                raise VerificationError(
                    f"Cryptographic verification failed: {verify_response.get('error')}"
                )

        except Exception as e:
            raise VerificationError(
                f"Failed to complete cryptographic verification: {e}\n"
                f"Ensure you have the correct private key from agent registration."
            )

    def verify_action(self, action_type, resource=None, context=None, timeout_seconds=300):
        """
        Request verification for an action from AIM.

        On first call, automatically performs cryptographic verification.
        """
        # Ensure agent is cryptographically verified FIRST
        self._ensure_verified()

        # ... existing verification logic continues unchanged ...
```

**Testing:**
```python
# test_challenge_response.py
import os
from aim_sdk import AIMClient

# Setup
os.environ["AIM_URL"] = "http://localhost:8080"
os.environ["AIM_API_KEY"] = "test-api-key"

# Auto-register
client = AIMClient.auto_register("test-verification-agent")

# First action triggers verification
@client.perform_action("read_file", resource="/data/test.txt")
def read_file():
    return "file contents"

print("=== Calling function (should trigger verification) ===")
result = read_file()

# Expected console output:
# 🔐 Performing cryptographic verification...
# ✅ Agent cryptographically verified!
#    Trust Score: 50
#    Verified At: 2025-10-07T...

print(f"✅ Test passed! Result: {result}")
```

**Success criteria:**
- [ ] First action call triggers automatic challenge-response
- [ ] Verification happens transparently (developer doesn't see it)
- [ ] Subsequent calls skip verification (already verified)
- [ ] Invalid keys fail with clear error message
- [ ] Helpful console output for debugging

---

## Testing Requirements

### End-to-End Test

```python
# test_end_to_end.py
"""
Complete end-to-end test of Phase 1:
1. Auto-register agent (first run)
2. Load credentials (second run)
3. Automatic cryptographic verification
4. Perform actual action verification
"""

import os
from aim_sdk import AIMClient

# Setup
os.environ["AIM_URL"] = "http://localhost:8080"
os.environ["AIM_API_KEY"] = "test-api-key"

print("=== Test 1: Auto-Registration ===")
client = AIMClient.auto_register("e2e-test-agent")
print(f"✅ Registered agent: {client.agent_id}\n")

print("=== Test 2: Credential Loading ===")
client2 = AIMClient.auto_register("e2e-test-agent")
assert client.agent_id == client2.agent_id
print("✅ Credentials loaded from file\n")

print("=== Test 3: Challenge-Response Verification ===")
@client.perform_action("read_database", resource="users_table")
def get_users():
    return ["user1", "user2", "user3"]

result = get_users()
print(f"✅ Action verified and executed: {result}\n")

print("=== Test 4: Subsequent Actions (No Re-Verification) ===")
result2 = get_users()
print(f"✅ Second call (no verification): {result2}\n")

print("🎉 All tests passed!")
```

**Run this test after completing all 4 tasks. It should pass 100%.**

---

## Success Criteria for Phase 1

Before moving to Phase 2, verify ALL of these:

### Backend
- [ ] `POST /api/v1/agents/auto-register` endpoint works
- [ ] `GET /api/v1/agents/:id/challenge` endpoint works
- [ ] `POST /api/v1/agents/:id/verify-challenge` endpoint works
- [ ] Database migrations applied successfully
- [ ] Agents created via auto-register have encrypted private keys
- [ ] Challenge-response marks agents as verified

### SDK
- [ ] `AIMClient.auto_register()` method works
- [ ] Credentials stored in `~/.aim/credentials/` with chmod 600
- [ ] Subsequent runs load credentials from file
- [ ] `_ensure_verified()` automatically called on first action
- [ ] Challenge-response happens transparently
- [ ] All 18 existing SDK tests still pass

### End-to-End
- [ ] Can register agent with 1 line of code
- [ ] Can verify agent cryptographically
- [ ] Can perform actions after verification
- [ ] No manual steps required
- [ ] Total time < 1 second for verification

### Documentation
- [ ] Update README with auto-register example
- [ ] Add troubleshooting section
- [ ] Document environment variables

---

## Key Files to Reference

### Design Documents
1. **SEAMLESS_AUTO_REGISTRATION.md** - Auto-registration spec
2. **CHALLENGE_RESPONSE_VERIFICATION.md** - Challenge-response spec
3. **AIM_COMPLETE_IMPLEMENTATION_ROADMAP.md** - Overall roadmap

### Existing Code
1. `apps/backend/internal/interfaces/http/handlers/agent_handler.go` - Existing handlers
2. `apps/backend/internal/application/agent_service.go` - Existing agent service
3. `apps/backend/internal/crypto/keygen.go` - Ed25519 key generation
4. `sdks/python/aim_sdk/client.py` - Existing SDK client
5. `sdks/python/tests/test_client.py` - Existing SDK tests

---

## Important Reminders

### Code Quality
- Follow existing code patterns in the codebase
- Add comments explaining crypto operations
- Use meaningful variable names
- Add error handling with helpful messages

### Security
- Never log private keys or signatures
- Use constant-time comparison for signatures
- Validate all inputs
- Use prepared SQL statements (prevent injection)

### Developer Experience
- Print helpful console messages
- Provide clear error messages
- Make errors actionable ("do X to fix")
- Keep API responses consistent

### Testing
- Test happy path AND error cases
- Test with invalid keys
- Test with expired challenges
- Test replay attacks
- Test performance (< 1 second)

---

## Atomic Habits Applied

### Make it OBVIOUS
- Clear console output showing what's happening
- Helpful error messages
- Documentation with examples

### Make it EASY
- 1 line of code to register
- Automatic verification
- No manual steps

### Make it ATTRACTIVE
- Beautiful console output (emojis: ✅ 🔐 📂 🚀)
- Works like magic
- Feels professional

### Make it SATISFYING
- Instant feedback (< 1s)
- Clear success messages
- Progress indicators

---

## Final Checklist Before Starting

- [ ] Read all 4 design documents
- [ ] Understand the complete vision
- [ ] Backend server is running
- [ ] Database is accessible
- [ ] You have a plan for each task
- [ ] You understand the testing requirements

---

**Estimated Total Time**: 18-26 hours (4-6 hours per task)

**When Done**: Create documentation showing the complete workflow working end-to-end. Then we move to Phase 2: Framework Integrations (LangChain, CrewAI, MCP).

**Let's build the future of AI agent identity!** 🚀
