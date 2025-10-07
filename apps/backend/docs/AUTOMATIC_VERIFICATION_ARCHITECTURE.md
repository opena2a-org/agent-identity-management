# Automatic Verification Architecture

## Overview
AIM implements automatic cryptographic verification for agents and MCP servers using ED25519 public-key cryptography and challenge-response protocols. Verification happens automatically on creation and continuously in the background.

## Key Principles

1. **Automatic, Not Manual**: Verification happens automatically - no "Verify" button needed
2. **Cryptographically Secure**: Uses ED25519 signatures, not just database flags
3. **Continuous Verification**: Background jobs re-verify every 24 hours
4. **Trust Score Integration**: Verification results feed into trust score algorithm
5. **Security Event Generation**: Failed verifications trigger security alerts

## Architecture Components

### 1. Keypair Generation Service

**Location**: `internal/infrastructure/crypto/keypair.go`

**Responsibilities**:
- Generate ED25519 keypair for new agents/MCPs
- Export private key for user download (one-time only)
- Store public key in database
- Support BYO (Bring Your Own) keys for advanced users

**API**:
```go
type KeypairService interface {
    GenerateKeypair() (*Keypair, error)
    VerifySignature(publicKey []byte, message []byte, signature []byte) (bool, error)
    SignMessage(privateKey []byte, message []byte) ([]byte, error)
}

type Keypair struct {
    PublicKey  []byte
    PrivateKey []byte  // Only returned once, never stored
}
```

### 2. Challenge-Response Protocol

**Flow**:
```
1. User creates agent/MCP with URL endpoint
2. System generates random challenge (UUID + timestamp)
3. System sends POST /verify-challenge to agent URL with:
   {
     "challenge": "uuid-timestamp",
     "expires_at": "2025-10-06T22:00:00Z"
   }
4. Agent signs challenge with private key
5. Agent responds with:
   {
     "signature": "base64-encoded-signature",
     "public_key": "base64-encoded-public-key"
   }
6. System verifies signature matches stored public key
7. System marks agent as "Verified" or "Failed"
```

**Security**:
- Challenges expire after 5 minutes
- Each challenge can only be used once
- Failed attempts are rate-limited (max 3 per hour)
- All verification attempts are logged for audit

### 3. Verification States

**Agent/MCP Status**:
- `pending` - Created, verification in progress (0-30 seconds)
- `verified` - Challenge-response succeeded
- `verification_failed` - Challenge-response failed
- `verification_expired` - No response within timeout
- `suspended` - Too many failed verifications (manual review required)
- `revoked` - Manually revoked by admin

**Database Schema Updates**:
```sql
ALTER TABLE agents ADD COLUMN verification_status VARCHAR(50) DEFAULT 'pending';
ALTER TABLE agents ADD COLUMN verification_attempts INT DEFAULT 0;
ALTER TABLE agents ADD COLUMN last_verification_attempt TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN next_verification_due TIMESTAMPTZ;

ALTER TABLE mcp_servers ADD COLUMN verification_status VARCHAR(50) DEFAULT 'pending';
ALTER TABLE mcp_servers ADD COLUMN verification_attempts INT DEFAULT 0;
ALTER TABLE mcp_servers ADD COLUMN last_verification_attempt TIMESTAMPTZ;
ALTER TABLE mcp_servers ADD COLUMN next_verification_due TIMESTAMPTZ;
```

### 4. Background Verification Service

**Location**: `internal/application/background/verification_scheduler.go`

**Responsibilities**:
- Run every hour
- Find agents/MCPs with `next_verification_due < NOW()`
- Perform challenge-response verification
- Update trust scores based on results
- Generate security events for failures
- Set `next_verification_due = NOW() + 24 hours` on success

**Cron Schedule**: `0 * * * *` (hourly)

### 5. Trust Score Integration

**Verification Metrics**:
- Verification success rate (last 30 days): 30% weight
- Time since last successful verification: 15% weight
- Response time to challenges: 10% weight
- Consecutive successful verifications: 10% weight

**Trust Score Formula**:
```
trust_score = (
    verification_success_rate * 0.30 +
    recency_score * 0.15 +
    response_time_score * 0.10 +
    consistency_score * 0.10 +
    capability_match_score * 0.20 +
    uptime_score * 0.15
) * 100
```

### 6. Security Event Generation

**Trigger Events**:
- Verification failure (severity: medium)
- 3+ consecutive failures (severity: high)
- Signature mismatch (severity: critical)
- Challenge timeout (severity: low)
- Public key rotation without notice (severity: high)

**Event Structure**:
```go
type SecurityEvent struct {
    ID              uuid.UUID
    OrganizationID  uuid.UUID
    AgentID         uuid.UUID
    EventType       string  // "verification_failure", "signature_mismatch", etc.
    Severity        string  // "low", "medium", "high", "critical"
    Description     string
    Remediation     string
    DetectedAt      time.Time
    IsAcknowledged  bool
}
```

## User Experience Changes

### Before (Current - Bad UX):
1. User creates agent/MCP
2. System saves to database, marks as "pending"
3. User sees agent in list with "Pending" status
4. User must click "Verify" button manually
5. System marks as "verified" (no real verification!)

### After (New - Excellent UX):
1. User creates agent/MCP with endpoint URL
2. System generates keypair
3. System shows modal: "Your Private Key (Save This!)" with download button
4. User saves private key file
5. System immediately attempts verification in background
6. UI shows "Verifying..." status with spinner
7. After 5-30 seconds, status updates to "Verified ✓" or "Verification Failed ❌"
8. If failed, UI shows reason and "Retry Verification" button

### UI Changes:

**Remove**:
- Manual "Verify" button

**Add**:
- Verification status badge: `Verifying...`, `Verified ✓`, `Failed ❌`
- Trust score indicator (0-100 with color gradient)
- "Last verified" timestamp
- "Next verification" countdown
- Security events notification badge

**MCP/Agent Registration Flow**:
1. Form with Name, Description, URL
2. Option: "Generate keypair" (default) or "Use existing key"
3. Submit creates agent and triggers verification
4. Modal shows private key (if generated) - must download
5. Status updates in real-time via WebSocket

## Implementation Plan

### Phase 1: Core Cryptography (1-2 hours)
- [ ] Implement KeypairService with ED25519
- [ ] Add challenge generation/verification functions
- [ ] Write unit tests for crypto operations

### Phase 2: Challenge-Response Protocol (2-3 hours)
- [ ] Implement HTTP client for challenge requests
- [ ] Add retry logic with exponential backoff
- [ ] Handle timeouts and errors gracefully
- [ ] Create verification event logging

### Phase 3: Database Migrations (30 mins)
- [ ] Add verification status columns
- [ ] Add verification attempt tracking
- [ ] Add next_verification_due scheduling

### Phase 4: Automatic Verification (2-3 hours)
- [ ] Integrate verification into agent creation
- [ ] Integrate verification into MCP creation
- [ ] Add asynchronous verification (goroutine)
- [ ] Add WebSocket status updates

### Phase 5: Background Scheduler (2 hours)
- [ ] Implement cron job for re-verification
- [ ] Add verification queue (Redis-backed)
- [ ] Implement rate limiting
- [ ] Add monitoring and alerting

### Phase 6: Trust Score Enhancement (1-2 hours)
- [ ] Update trust score algorithm
- [ ] Add verification metrics
- [ ] Recalculate scores after verification

### Phase 7: Security Events (1-2 hours)
- [ ] Define event types and severities
- [ ] Implement event generation
- [ ] Add event notification system
- [ ] Create security dashboard

### Phase 8: Frontend Updates (2-3 hours)
- [ ] Remove "Verify" button
- [ ] Add status badges
- [ ] Add private key download modal
- [ ] Add real-time status updates (WebSocket)
- [ ] Add retry verification option

### Phase 9: End-to-End Testing (2-3 hours)
- [ ] Test full agent registration flow
- [ ] Test full MCP registration flow
- [ ] Test background re-verification
- [ ] Test security event generation
- [ ] Test trust score calculations

## Benefits for Investors

1. **Enterprise-Grade Security**: Real cryptographic verification, not security theater
2. **Zero-Touch Operation**: Automatic verification reduces operational overhead
3. **Continuous Compliance**: 24-hour re-verification ensures ongoing compliance
4. **Proactive Security**: Alerts on suspicious activity before incidents occur
5. **Audit Trail**: Complete verification history for SOC 2/HIPAA compliance

## Competitive Advantages

- Most identity management systems use basic API key auth
- AIM uses public-key cryptography with challenge-response
- Automatic verification eliminates human error
- Continuous re-verification catches compromised agents quickly
- Trust scoring provides risk-based access control

## Success Metrics

- Verification success rate: > 95%
- Average verification time: < 10 seconds
- False positive rate: < 1%
- Security event response time: < 5 minutes
- Trust score accuracy: > 90% (measured by incident correlation)
