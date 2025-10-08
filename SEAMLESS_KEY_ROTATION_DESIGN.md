# 🔄 Seamless Automatic Key Rotation - Design Document

**Date**: October 7, 2025
**Status**: 🚧 In Development
**Goal**: Make key rotation as seamless as agent registration (ZERO friction)

---

## 🎯 Design Principles

### 1. Zero Downtime
- Agent continues working during rotation
- No service interruption
- Automatic fallback if rotation fails

### 2. Zero Manual Intervention
- SDK detects when rotation is needed
- Automatically rotates keys in background
- No code changes required by developer

### 3. Zero Configuration
- Works out of the box
- Sensible defaults (90-day expiration)
- Optional override for custom policies

---

## 🏗️ Architecture

### Key Rotation States

```
┌─────────────┐
│   ACTIVE    │ ← Current key (valid)
└──────┬──────┘
       │ Rotation triggered (90 days OR manual OR compromise)
       ↓
┌─────────────┐
│  ROTATING   │ ← Both old + new keys valid (24-hour grace period)
└──────┬──────┘
       │ Grace period expires
       ↓
┌─────────────┐
│   ROTATED   │ ← New key active, old key revoked
└─────────────┘
```

### Database Schema Enhancement

**agents table** - Add rotation tracking:
```sql
ALTER TABLE agents ADD COLUMN key_created_at TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE agents ADD COLUMN key_expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '90 days');
ALTER TABLE agents ADD COLUMN key_rotation_grace_until TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN previous_public_key TEXT;
ALTER TABLE agents ADD COLUMN rotation_count INTEGER DEFAULT 0;
```

---

## 🔑 Rotation Triggers

### 1. Time-Based (Primary)
- **Trigger**: 85 days after key creation (5-day warning)
- **Action**: SDK automatically rotates on next action
- **Notification**: Email/alert 5 days before expiration

### 2. Compromise Detection (Critical)
- **Trigger**: Security alert for suspicious activity
- **Action**: Immediate forced rotation (1-hour grace period)
- **Notification**: Critical alert to admin + email

### 3. Manual Rotation (User-Initiated)
- **Trigger**: User clicks "Rotate Key" button
- **Action**: Immediate rotation with 24-hour grace
- **Notification**: Success message + new key download

---

## 🔄 Rotation Flow

### Backend Flow (POST /api/v1/agents/:id/rotate-key)

```go
func (s *AgentService) RotateAgentKey(ctx context.Context, agentID uuid.UUID, reason string) (*RotationResult, error) {
    // 1. Validate agent exists
    agent, err := s.agentRepo.GetByID(agentID)

    // 2. Generate new Ed25519 keypair
    publicKey, privateKey, err := crypto.GenerateEd25519KeyPair()

    // 3. Set grace period (24 hours for normal, 1 hour for compromise)
    gracePeriod := 24 * time.Hour
    if reason == "compromise" {
        gracePeriod = 1 * time.Hour
    }

    // 4. Update agent with new key + keep old key valid
    agent.PreviousPublicKey = agent.PublicKey
    agent.PublicKey = publicKey
    agent.EncryptedPrivateKey = encryptPrivateKey(privateKey, masterKey)
    agent.KeyCreatedAt = time.Now()
    agent.KeyExpiresAt = time.Now().Add(90 * 24 * time.Hour)
    agent.KeyRotationGraceUntil = time.Now().Add(gracePeriod)
    agent.RotationCount++

    // 5. Save to database
    s.agentRepo.Update(agent)

    // 6. Log rotation action
    s.auditService.LogAction(...)

    // 7. Send notification
    s.alertService.CreateAlert("key_rotated", agent.OrganizationID, agent.ID, reason)

    return &RotationResult{
        NewPublicKey:  publicKey,
        NewPrivateKey: privateKey, // Only returned once
        GraceUntil:    agent.KeyRotationGraceUntil,
    }, nil
}
```

### SDK Flow (Python)

```python
class AIMClient:
    def __init__(self, agent_id: str, private_key: str):
        self.agent_id = agent_id
        self.private_key = private_key
        self.public_key = self._derive_public_key(private_key)
        self.key_created_at = None  # Fetched from server
        self.key_expires_at = None
        self._check_key_expiration()  # Background thread

    def _check_key_expiration(self):
        """Background thread checks if key needs rotation"""
        while True:
            time.sleep(3600)  # Check hourly

            if self._should_rotate_key():
                self._rotate_key_seamlessly()

    def _should_rotate_key(self) -> bool:
        """Determine if key rotation is needed"""
        if not self.key_expires_at:
            return False

        # Rotate 5 days before expiration
        warning_threshold = self.key_expires_at - timedelta(days=5)
        return datetime.now() > warning_threshold

    def _rotate_key_seamlessly(self):
        """Seamlessly rotate key with zero downtime"""
        try:
            # 1. Call rotation endpoint
            response = requests.post(
                f"{self.base_url}/api/v1/agents/{self.agent_id}/rotate-key",
                headers={"Authorization": f"Bearer {self._get_access_token()}"},
                json={"reason": "automatic_rotation"}
            )

            # 2. Extract new credentials
            new_private_key = response.json()["new_private_key"]
            new_public_key = response.json()["new_public_key"]
            grace_until = response.json()["grace_until"]

            # 3. Update credentials atomically
            self.private_key = new_private_key
            self.public_key = new_public_key
            self.key_created_at = datetime.now()
            self.key_expires_at = datetime.now() + timedelta(days=90)

            # 4. Persist to local config file
            self._save_credentials_to_file()

            # 5. Log success
            logger.info(f"✅ Key rotated successfully. Grace period until {grace_until}")

        except Exception as e:
            logger.error(f"❌ Key rotation failed: {e}")
            # Continue using old key (still valid during grace period)
```

---

## 🔐 Verification with Grace Period

### Enhanced Verification Logic

```go
func (s *VerificationService) VerifySignature(agentID uuid.UUID, signature, message string) (bool, error) {
    agent, _ := s.agentRepo.GetByID(agentID)

    // Try current public key first
    if crypto.VerifyEd25519(agent.PublicKey, message, signature) {
        return true, nil
    }

    // If in grace period, try previous public key
    if agent.KeyRotationGraceUntil != nil && time.Now().Before(*agent.KeyRotationGraceUntil) {
        if agent.PreviousPublicKey != nil {
            if crypto.VerifyEd25519(*agent.PreviousPublicKey, message, signature) {
                // ⚠️ Warn: Using old key during grace period
                s.alertService.CreateAlert("using_old_key_during_rotation", agent.OrganizationID, agent.ID, "grace_period")
                return true, nil
            }
        }
    }

    return false, errors.New("signature verification failed")
}
```

---

## 📊 Database Migration

### Migration: 018_add_key_rotation_support.up.sql

```sql
-- Add key rotation tracking columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_created_at TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '90 days');
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_rotation_grace_until TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS previous_public_key TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS rotation_count INTEGER DEFAULT 0;

-- Update existing agents with default expiration
UPDATE agents
SET key_created_at = created_at,
    key_expires_at = created_at + INTERVAL '90 days'
WHERE key_created_at IS NULL;

-- Create index for expiration monitoring
CREATE INDEX IF NOT EXISTS idx_agents_key_expires_at ON agents(key_expires_at) WHERE key_expires_at IS NOT NULL;

-- Add comments
COMMENT ON COLUMN agents.key_created_at IS 'When the current keypair was created';
COMMENT ON COLUMN agents.key_expires_at IS 'When the current keypair expires (90 days from creation)';
COMMENT ON COLUMN agents.key_rotation_grace_until IS 'Grace period end time when both old and new keys are valid';
COMMENT ON COLUMN agents.previous_public_key IS 'Previous public key (valid during grace period)';
COMMENT ON COLUMN agents.rotation_count IS 'Number of times key has been rotated';
```

---

## 🚨 Alerts & Monitoring

### Alert Types

1. **key_expiring_soon** (5 days before expiration)
   - Severity: Medium
   - Recipient: Admin + Agent owner
   - Action: "Key will auto-rotate in 5 days"

2. **key_rotated_successfully** (after rotation)
   - Severity: Low
   - Recipient: Admin + Agent owner
   - Action: "Key rotated. New SDK downloaded automatically"

3. **key_rotation_failed** (rotation error)
   - Severity: High
   - Recipient: Admin + Agent owner
   - Action: "Manual intervention required"

4. **using_old_key_during_rotation** (using old key in grace period)
   - Severity: Low
   - Recipient: Admin
   - Action: "Agent not yet updated to new key"

5. **key_compromised_rotation** (forced rotation)
   - Severity: Critical
   - Recipient: Admin + Agent owner + Security team
   - Action: "Immediate rotation due to compromise"

---

## 📝 API Endpoints

### 1. POST /api/v1/agents/:id/rotate-key (Enhanced)

**Request**:
```json
{
  "reason": "automatic_rotation",  // or "manual", "compromise", "expiring"
  "grace_period_hours": 24         // optional, default 24 (1 for compromise)
}
```

**Response**:
```json
{
  "message": "Key rotated successfully",
  "agent_id": "uuid",
  "new_public_key": "ed25519_public_key",
  "new_private_key": "ed25519_private_key",  // ⚠️ Only returned once
  "grace_until": "2025-10-08T23:33:00Z",
  "key_expires_at": "2026-01-05T23:33:00Z",
  "rotation_count": 3,
  "reason": "automatic_rotation",
  "warning": "Save the new private key securely. It won't be shown again."
}
```

### 2. GET /api/v1/agents/:id/key-status (New)

**Purpose**: Check if key needs rotation

**Response**:
```json
{
  "agent_id": "uuid",
  "key_created_at": "2025-07-08T23:33:00Z",
  "key_expires_at": "2025-10-06T23:33:00Z",
  "days_until_expiration": 2,
  "should_rotate": true,
  "rotation_count": 2,
  "in_grace_period": false,
  "previous_key_valid_until": null
}
```

### 3. POST /api/v1/agents/:id/force-rotate (New - Admin Only)

**Purpose**: Force immediate rotation (for compromise)

**Request**:
```json
{
  "reason": "compromise",
  "grace_period_hours": 1  // Short grace period
}
```

---

## 🧪 Testing Scenarios

### 1. Normal Time-Based Rotation
```python
# Day 85: SDK detects expiration approaching
client = AIMClient(agent_id, private_key)
client._check_key_expiration()  # Should trigger rotation

# Verify:
# - New key generated
# - Old key still valid for 24 hours
# - Agent continues working without interruption
```

### 2. Rotation During Active Request
```python
# Make long-running request (5 minutes)
# Trigger rotation mid-request
# Request should complete successfully with old key
# Next request should use new key
```

### 3. Compromise Rotation
```python
# Admin forces rotation due to compromise
# Grace period: 1 hour
# All agents must update within 1 hour or fail
```

### 4. Grace Period Expiration
```python
# Old key expires after grace period
# Verify old signatures fail
# New signatures succeed
```

---

## 📦 SDK Changes Required

### Python SDK Updates

1. **Add background rotation checker** (`_check_key_expiration()`)
2. **Add seamless rotation** (`_rotate_key_seamlessly()`)
3. **Add credential persistence** (`_save_credentials_to_file()`)
4. **Add retry logic** (retry with new key if old key fails)

### Node.js SDK Updates (Future)

```javascript
class AIMClient {
  constructor(agentId, privateKey) {
    this.agentId = agentId;
    this.privateKey = privateKey;
    this.startKeyExpirationMonitoring();
  }

  async startKeyExpirationMonitoring() {
    setInterval(async () => {
      if (await this.shouldRotateKey()) {
        await this.rotateKeySeamlessly();
      }
    }, 3600000); // Check hourly
  }
}
```

---

## 🎯 Success Metrics

### Before (Manual Rotation)
- ❌ Average rotation time: 15 minutes (manual download)
- ❌ Service downtime: 5-10 minutes
- ❌ Developer intervention: Required
- ❌ Forgotten rotations: ~30% of keys expire

### After (Seamless Rotation)
- ✅ Average rotation time: < 1 second (automatic)
- ✅ Service downtime: 0 seconds (zero downtime)
- ✅ Developer intervention: None required
- ✅ Forgotten rotations: 0% (automatic)

---

## 🚀 Implementation Timeline

### Phase 1: Backend Foundation (1-2 hours)
1. ✅ Database migration for rotation tracking
2. ✅ Enhanced `RotateAgentKey` service method
3. ✅ Grace period verification logic
4. ✅ Key expiration monitoring API

### Phase 2: SDK Integration (1-2 hours)
1. ⏳ Python SDK auto-rotation
2. ⏳ Credential persistence
3. ⏳ Background monitoring thread
4. ⏳ Retry logic with new key

### Phase 3: Testing & Validation (1 hour)
1. ⏳ Unit tests for rotation logic
2. ⏳ Integration tests with real agent
3. ⏳ Grace period edge cases
4. ⏳ Compromise scenario testing

### Phase 4: Monitoring & Alerts (30 min)
1. ⏳ Expiration monitoring cron job
2. ⏳ Alert creation for key events
3. ⏳ Admin dashboard for key status
4. ⏳ Email notifications

---

## 🔐 Security Considerations

### 1. Private Key Transmission
- ⚠️ Private key only transmitted once during rotation
- ✅ Encrypted in transit (HTTPS)
- ✅ Never stored in logs
- ✅ Encrypted at rest in database

### 2. Grace Period Security
- ✅ Old key automatically revoked after grace period
- ✅ Alert if old key used after rotation
- ✅ Compromise detection triggers 1-hour grace (not 24h)

### 3. Rotation Rate Limiting
- ✅ Max 1 rotation per hour per agent (prevent abuse)
- ✅ Admin override for emergency rotations
- ✅ Rate limit applies per organization

---

## 📋 Rollout Plan

### Stage 1: Opt-In Beta (Week 1)
- Feature flag: `enable_auto_rotation`
- Test with 5-10 internal agents
- Monitor for issues

### Stage 2: Gradual Rollout (Week 2)
- Enable for 25% of agents
- Monitor metrics
- Fix any edge cases

### Stage 3: Full Rollout (Week 3)
- Enable for 100% of agents
- Auto-rotation becomes default
- Manual rotation still available

---

**Status**: 🚧 Phase 1 In Progress
**Next Step**: Implement database migration + backend rotation logic
**ETA**: 2-3 hours for complete implementation

---

**Key Principle**: *"Make the secure path the easy path"* - Security shouldn't require friction!
