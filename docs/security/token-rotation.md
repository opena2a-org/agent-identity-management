# Token Rotation - Enterprise Security Feature

## Overview

AIM uses **token rotation** to protect your organization from token theft and unauthorized access. This is an enterprise-grade security feature that is **required for SOC 2, HIPAA, and GDPR compliance**.

## What is Token Rotation?

Token rotation is a security mechanism where:
1. **Every time you use a refresh token** → backend issues a **NEW** refresh token
2. **The OLD refresh token is immediately invalidated** → cannot be reused
3. **Your credentials are automatically updated** with the new token

This prevents attackers from using stolen tokens, even if they manage to intercept or steal your credentials.

---

## Why Does This Happen?

### The Problem: Token Theft

Without token rotation:
- ❌ Stolen tokens work **forever** (until manual revocation)
- ❌ Attackers can use old tokens **indefinitely**
- ❌ No automatic protection against credential leaks

### The Solution: Token Rotation

With token rotation:
- ✅ Stolen tokens **expire immediately** after use
- ✅ Only the **most recently issued token** works
- ✅ **Automatic protection** against credential theft
- ✅ Complete **audit trail** of all token usage

---

## How It Works

### Normal Flow (No Token Theft)

```
1. User downloads SDK from portal
   → Gets refresh_token_v1

2. Agent uses refresh_token_v1 to get access token
   → Backend issues NEW refresh_token_v2
   → Backend revokes refresh_token_v1
   → SDK automatically saves refresh_token_v2

3. Agent uses refresh_token_v2 for next request
   → Backend issues NEW refresh_token_v3
   → Backend revokes refresh_token_v2
   → SDK automatically saves refresh_token_v3

... and so on
```

**Result**: Your agent works seamlessly. You never see the token rotation happening.

### Theft Scenario (Token Rotation Protects You)

```
1. User downloads SDK from portal
   → Gets refresh_token_v1

2. Attacker steals refresh_token_v1
   → Attacker has a copy of the token

3. Legitimate agent uses refresh_token_v1
   → Backend issues NEW refresh_token_v2
   → Backend revokes refresh_token_v1 ✅
   → SDK saves refresh_token_v2

4. Attacker tries to use stolen refresh_token_v1
   → ❌ REJECTED - Token has been revoked
   → ✅ Security alert generated
   → ✅ Your data is protected
```

**Result**: Token theft is **automatically detected and blocked**.

---

## Why You Might See "Token Expired" Errors

### Common Scenarios

#### Scenario 1: Using Old SDK Download

**What happened**:
- You downloaded SDK → got refresh_token_v1
- You used SDK once → token rotated to refresh_token_v2
- You try to use OLD SDK download → refresh_token_v1 is now revoked

**Solution**: Download fresh SDK with current credentials

#### Scenario 2: Multiple Copies of SDK

**What happened**:
- You have SDK running in two places (laptop + server)
- Laptop uses token → rotates to v2
- Server tries to use old token → rejected

**Solution**: Only run SDK from one location, OR download separate credentials for each location

#### Scenario 3: Manual Token Testing

**What happened**:
- You copied refresh_token to test manually
- Original SDK rotated the token
- Your manual test uses old token → rejected

**Solution**: Always use SDK - it handles rotation automatically

---

## How to Fix "Token Expired" Errors

### Quick Fix (5 minutes)

1. **Log in to AIM Portal**
   ```
   http://localhost:3000/auth/login
   ```
   Or your production URL: `https://aim.yourdomain.com/auth/login`

2. **Download Fresh SDK**
   - Navigate to: Dashboard → SDK Download
   - Or direct link: `http://localhost:3000/dashboard/sdk`
   - Click "Download SDK" for your language (Python, Go, etc.)

3. **Copy New Credentials**
   ```bash
   # Extract downloaded SDK
   unzip aim-sdk-python.zip

   # Copy credentials to your project
   cp -r aim-sdk-python/.aim ~/.aim/
   ```

4. **Restart Your Agent**
   ```bash
   # Your agent will automatically use new credentials
   python your_agent.py
   ```

That's it! Your agent now has fresh, valid credentials.

---

## Preventing Token Expiration

### Best Practices

#### 1. Let the SDK Handle It

✅ **Correct**:
```python
from aim_sdk import secure

# SDK automatically manages token rotation
client = secure("my-agent")

# No manual token management needed!
client.verify_action(...)
```

❌ **Incorrect**:
```python
# Don't manually manage tokens
refresh_token = load_token_from_file()
access_token = manually_refresh(refresh_token)  # Token rotation breaks this
```

#### 2. Use One SDK Instance Per Location

✅ **Correct**:
- Laptop: Download SDK once, use continuously
- Server: Download separate SDK, use continuously

❌ **Incorrect**:
- Copy laptop's credentials to server (will conflict during rotation)

#### 3. Regenerate Credentials for Each Environment

✅ **Correct**:
```bash
# Development
Download SDK → dev credentials

# Staging
Download SDK → staging credentials

# Production
Download SDK → production credentials
```

❌ **Incorrect**:
```bash
# Don't share credentials across environments
cp dev/.aim/credentials.json production/.aim/  # Will cause rotation conflicts
```

---

## For Enterprise Administrators

### Security Benefits

Token rotation provides:

1. **Automatic Token Revocation**
   - Every token use triggers revocation of previous token
   - No manual revocation needed
   - Immediate protection against stolen credentials

2. **Complete Audit Trail**
   - Every token issuance logged in database
   - Track which tokens are active vs revoked
   - See when each token was last used

3. **SOC 2 Compliance**
   - Meets **Access Control** requirements
   - Meets **Logical Security** requirements
   - Provides complete audit trail for auditors

4. **HIPAA Compliance**
   - Meets **Authentication** requirements (§164.312(d))
   - Meets **Audit Controls** requirements (§164.312(b))
   - Prevents unauthorized access to PHI

5. **GDPR Compliance**
   - Meets **Security of Processing** requirements
   - Implements **appropriate technical measures**
   - Protects against unauthorized access

### Configuration

Token rotation is **enabled by default** and **cannot be disabled** for security reasons.

Configuration options:
```yaml
# Backend configuration (apps/backend/config/security.yaml)
token_rotation:
  enabled: true          # Always true (required for compliance)
  rotation_policy: always  # Rotate on every refresh
  track_revocations: true  # Log all revocations
```

### Monitoring Token Rotation

**Database Query** - Check active vs revoked tokens:
```sql
SELECT
  COUNT(*) FILTER (WHERE revoked_at IS NULL) as active_tokens,
  COUNT(*) FILTER (WHERE revoked_at IS NOT NULL) as revoked_tokens,
  COUNT(*) as total_tokens
FROM sdk_tokens;
```

**Dashboard** - View token rotation events:
- Navigate to: Admin → Security → Token Activity
- See real-time token issuance and revocation

**Alerts** - Monitor suspicious activity:
- Alert when revoked token is used (potential theft)
- Alert on unusual token rotation patterns
- Track tokens that haven't rotated in 30+ days

---

## FAQs

### Q: How often do tokens rotate?

**A:** Tokens rotate **every time they're used** to refresh the access token.

Typical rotation frequency:
- Access tokens expire every **1 hour**
- Agent refreshes access token → triggers rotation
- So tokens typically rotate **once per hour** during active use

### Q: Will my agent stop working during rotation?

**A:** No. Token rotation is **completely transparent** to your agent.

The SDK handles rotation automatically:
1. Old token used → backend issues new token
2. SDK automatically saves new token
3. Next request uses new token
4. **No interruption to your agent**

### Q: Can I disable token rotation?

**A:** No. Token rotation is **required for enterprise security** and compliance (SOC 2, HIPAA, GDPR).

If you need long-lived credentials:
- Use **API keys** instead (different security model)
- Or use **service accounts** with API keys
- OAuth tokens MUST rotate for security

### Q: What if I'm debugging and need stable tokens?

**A:** For development/debugging, you have options:

1. **Use API Keys** (no rotation):
   ```python
   client = AIMClient(
       agent_id="...",
       api_key="your-api-key",  # No rotation
       aim_url="..."
   )
   ```

2. **Download Fresh SDK** each time:
   ```bash
   # Get fresh tokens for each debug session
   ./get_fresh_sdk.sh
   ```

3. **Use Test Environment** with relaxed policies

### Q: How do I know if my token has been rotated?

**A:** The SDK will log rotation events:

```
🔄 Token rotated successfully - new credentials saved
```

You'll see this message when:
- Token refresh succeeds
- Backend issued a new refresh token
- SDK saved the new token to credentials

### Q: What happens if rotation fails?

**A:** The SDK handles failures gracefully:

1. **Network Error** → SDK retries automatically
2. **Token Revoked** → SDK throws `TokenExpiredError` with helpful message
3. **Server Error** → SDK retries with exponential backoff

You'll see clear error messages with instructions to fix.

---

## Technical Details

### Token Rotation Flow

```
┌─────────────┐
│   Agent     │
│    SDK      │
└──────┬──────┘
       │
       │ 1. POST /api/v1/auth/refresh
       │    { "refresh_token": "old_token" }
       │
       ▼
┌─────────────────┐
│  AIM Backend    │
│  (Go Server)    │
└─────────┬───────┘
          │
          │ 2. Validate old_token
          │ 3. Generate new_token
          │ 4. Revoke old_token in DB
          │ 5. Return new tokens
          │
          ▼
    ┌──────────────┐
    │  Response:   │
    │  {           │
    │    "access_token": "new_access",   │
    │    "refresh_token": "new_refresh"  │ ◄── NEW!
    │  }           │
    └──────┬───────┘
           │
           ▼
    ┌────────────────┐
    │  SDK Auto-Save │
    │  credentials   │
    │  with new      │
    │  refresh_token │
    └────────────────┘
```

### Database Schema

```sql
CREATE TABLE sdk_tokens (
  id UUID PRIMARY KEY,
  token_id TEXT UNIQUE NOT NULL,  -- Unique ID for this token
  token_hash TEXT NOT NULL,        -- SHA-256 hash of refresh token
  user_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  device_name TEXT,

  -- Token lifecycle
  created_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,

  -- Revocation tracking
  revoked_at TIMESTAMPTZ,          -- When token was revoked
  revoke_reason TEXT,              -- Why (rotation, manual, security)

  -- Audit trail
  rotated_from_token_id TEXT,     -- Previous token in rotation chain
  rotated_to_token_id TEXT        -- Next token (if rotated)
);

-- Indexes for performance
CREATE INDEX idx_sdk_tokens_revoked
  ON sdk_tokens(revoked_at)
  WHERE revoked_at IS NULL;  -- Find active tokens quickly
```

### Security Considerations

**Token Storage**:
- Tokens stored as **SHA-256 hashes** in database
- Original tokens never logged or stored in plaintext
- Even database admins cannot recover original tokens

**Revocation Checking**:
- Every refresh request checks `revoked_at` field
- Revoked tokens rejected with 401 Unauthorized
- Attempt logged for security monitoring

**Rotation Chain**:
- Each token knows its predecessor (`rotated_from_token_id`)
- Complete audit trail of token lineage
- Can trace back to original SDK download

---

## Support

### Need Help?

**If you see token expired errors**:
1. Follow the [Quick Fix guide](#quick-fix-5-minutes) above
2. Check the [Troubleshooting Guide](../troubleshooting/authentication.md)
3. Review your token activity in the dashboard

**For enterprise customers**:
- Contact your AIM administrator
- Submit support ticket with agent ID
- Include error messages and timestamps

**For open-source users**:
- Check [GitHub Issues](https://github.com/opena2a-org/agent-identity-management/issues)
- Review [documentation](https://docs.aim.example.com)
- Ask in community Discord/Slack

---

## Summary

✅ **Token rotation is a security feature, not a bug**

✅ **It protects your organization from token theft**

✅ **The SDK handles rotation automatically**

✅ **You only need to download fresh SDK when you see errors**

✅ **This is required for enterprise compliance (SOC 2, HIPAA, GDPR)**

**Bottom line**: Token rotation makes AIM more secure for enterprise deployment. The minor inconvenience of occasionally downloading fresh credentials is far outweighed by the security benefits.

---

**Last Updated**: October 18, 2025
**Version**: 1.0
**Status**: Production
