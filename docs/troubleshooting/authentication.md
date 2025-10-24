# Authentication Troubleshooting - Deep Dive

This guide provides in-depth troubleshooting for authentication issues in the AIM platform.

---

## Table of Contents

- [Understanding AIM Authentication](#understanding-aim-authentication)
- [Token Lifecycle](#token-lifecycle)
- [Common Authentication Errors](#common-authentication-errors)
- [Advanced Diagnostics](#advanced-diagnostics)
- [Security Considerations](#security-considerations)

---

## Understanding AIM Authentication

AIM uses a **dual authentication model**:

### 1. OAuth 2.0 (User Authentication)
- **Purpose**: Authenticate users accessing the platform
- **Flow**: Microsoft/Google OAuth → JWT access/refresh tokens
- **Used For**: Dashboard access, SDK downloads

### 2. Ed25519 Cryptographic Signing (Agent Authentication)
- **Purpose**: Authenticate agents performing actions
- **Flow**: Agent signs requests with private key
- **Used For**: Verification requests, action logging

### Combined Model

```
User (OAuth) → Downloads SDK → Agent (Ed25519 + OAuth)
```

An agent needs BOTH:
- ✅ OAuth tokens (from user's SDK download)
- ✅ Ed25519 keys (generated during agent registration)

---

## Token Lifecycle

### Phase 1: Initial Authentication

```
1. User logs in via OAuth (Google/Microsoft)
   → Backend issues access_token (1 hour TTL)
   → Backend issues refresh_token (90 days TTL)
   → Tokens stored in session/cookies

2. User downloads SDK
   → SDK includes current refresh_token
   → SDK includes sdk_token_id for tracking

3. Agent registers using secure()
   → SDK uses refresh_token to get access_token
   → Agent generates Ed25519 keypair
   → Public key sent to backend
   → Agent gets agent_id
```

### Phase 2: Active Use

```
4. Agent requests verification
   → SDK checks: access_token expired?
   → If expired: refresh using refresh_token
   → Backend returns NEW access_token + NEW refresh_token
   → SDK auto-saves NEW refresh_token (token rotation)
   → OLD refresh_token revoked in database

5. Verification request sent
   → Signed with Ed25519 private key
   → Includes access_token in header
   → Backend validates both signature AND token
```

### Phase 3: Token Rotation (Security Feature)

```
6. Every token refresh triggers rotation
   → refresh_token_v1 → refresh_token_v2
   → refresh_token_v1 marked as revoked
   → Only refresh_token_v2 works now

7. If refresh_token_v1 is used again
   → Backend returns 401 Unauthorized
   → Security alert generated
   → This indicates potential token theft
```

---

## Common Authentication Errors

### Error 1: "Token Expired" (Status: 401)

**Full Error**:
```
❌ Authentication Failed: Token Expired

Your SDK credentials have expired due to token rotation (security policy).

To fix this issue:
  1. Log in to AIM portal: http://localhost:3000/auth/login
  2. Download fresh SDK: http://localhost:3000/dashboard/sdk
  3. Copy new credentials to ~/.aim/credentials.json
```

**Root Cause**: Refresh token has been rotated/revoked.

**When This Happens**:
- After using SDK from multiple locations
- After long period of inactivity
- After token manually revoked by admin
- After security incident (automatic revocation)

**How to Fix**:

```bash
# Step 1: Log in to portal
open http://localhost:3000/auth/login

# Step 2: Download fresh SDK
# Click "Download SDK" in dashboard

# Step 3: Extract and copy credentials
unzip aim-sdk-python.zip
cp -r aim-sdk-python/.aim ~/.aim/

# Step 4: Verify credentials structure
cat ~/.aim/credentials.json | python -m json.tool

# Expected structure:
{
  "refresh_token": "...",      # Root level (OAuth)
  "sdk_token_id": "...",       # Root level (OAuth)
  "aim_url": "http://...",
  "your-agent": {
    "agent_id": "...",
    "private_key": "...",      # Agent level (Ed25519)
    "public_key": "..."
  }
}

# Step 5: Test authentication
python -c "
from aim_sdk import secure
client = secure('your-agent')
print(f'✅ Authenticated: {client.agent_id}')
"
```

**Prevention**:
- Don't copy credentials between machines
- Download separate SDK for each environment
- Let SDK handle token rotation automatically

---

### Error 2: "Invalid Credentials" (Status: 401)

**Full Error**:
```
❌ Authentication Failed: Invalid credentials format

Your agent credentials appear to be invalid or corrupted.
```

**Root Cause**: Credentials file is malformed or incomplete.

**Diagnostic Steps**:

```bash
# 1. Check file exists
ls -la ~/.aim/credentials.json
# Expected: -rw------- (permissions 600)

# 2. Validate JSON structure
python -m json.tool ~/.aim/credentials.json
# Should print formatted JSON, no errors

# 3. Check required fields
cat ~/.aim/credentials.json | jq 'keys'
# Expected: ["refresh_token", "sdk_token_id", "aim_url", "your-agent-name"]

# 4. Verify OAuth tokens present
cat ~/.aim/credentials.json | jq '. | has("refresh_token")'
# Expected: true

# 5. Verify agent keys present
cat ~/.aim/credentials.json | jq '.["your-agent"] | has("private_key")'
# Expected: true
```

**Common Issues**:

#### Issue A: Missing OAuth Tokens

```json
// ❌ WRONG - no OAuth tokens at root
{
  "your-agent": {
    "agent_id": "...",
    "private_key": "...",
    "public_key": "..."
  }
}

// ✅ CORRECT - OAuth tokens at root
{
  "refresh_token": "...",  // Added
  "sdk_token_id": "...",   // Added
  "your-agent": {
    "agent_id": "...",
    "private_key": "...",
    "public_key": "..."
  }
}
```

**Fix**: Download fresh SDK (contains both OAuth + agent keys)

#### Issue B: Corrupted Private Key

```bash
# Check private key format
cat ~/.aim/credentials.json | jq '.["your-agent"].private_key'

# Should be base64-encoded Ed25519 key
# Length: ~88 characters
# Example: "MC4CAQAwBQYDK2VwBCIEIGHxX..."

# If corrupted (wrong length, invalid chars):
# Download fresh SDK
```

#### Issue C: Wrong Agent Name

```python
# Code says:
client = secure("my-agent")

# But credentials.json has:
{
  "different-agent-name": { ... }  # Mismatch!
}

# Fix: Use correct agent name OR re-register
```

---

### Error 3: "Signature Verification Failed" (Status: 401)

**Full Error**:
```
❌ Authentication Failed: Invalid signature

Request signature verification failed.
```

**Root Cause**: Ed25519 signature invalid.

**Possible Causes**:

#### 1. Private Key Mismatch

```bash
# Check if public key in backend matches local private key

# Get agent's public key from backend
curl http://localhost:8080/api/v1/agents/YOUR_AGENT_ID \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq -r '.public_key'

# Get public key from credentials
cat ~/.aim/credentials.json | jq -r '.["your-agent"].public_key'

# Should be IDENTICAL

# If different:
# - Private key was regenerated
# - Or agent re-registered with different keys
# Fix: Download fresh SDK
```

#### 2. Request Tampering

```python
# ❌ Don't modify request after signing
request = client.sign_request(data)
request['data'] = 'modified'  # Signature now invalid!

# ✅ Sign the final request
final_data = {'action': 'search'}
request = client.sign_request(final_data)  # Don't modify after
```

#### 3. Clock Skew

```bash
# Check system time
date

# Sync with NTP
sudo ntpdate -s time.apple.com  # Mac
sudo ntpdate -s pool.ntp.org    # Linux

# Restart agent after time sync
```

---

### Error 4: "Access Token Expired" (Status: 401)

**Full Error**:
```
⚠️  Warning: Access token expired
```

**Root Cause**: Access token expired (1 hour TTL).

**Expected Behavior**: SDK should automatically refresh.

**If SDK Doesn't Refresh**:

```python
# Debug: Check if SDK has refresh token
from aim_sdk.oauth import OAuthTokenManager

manager = OAuthTokenManager()
print(f"Has credentials: {manager.has_credentials()}")
print(f"Has refresh token: {'refresh_token' in manager.credentials}")

# If False:
# - Credentials not loaded
# - Or refresh_token missing
# Fix: Download fresh SDK
```

**Manual Refresh** (for debugging):

```python
from aim_sdk.oauth import OAuthTokenManager

manager = OAuthTokenManager()
access_token = manager.get_access_token()

if access_token:
    print(f"✅ Access token refreshed")
else:
    print(f"❌ Refresh failed - check credentials")
```

---

## Advanced Diagnostics

### Check Token Status in Database

```sql
-- Connect to database
PGPASSWORD=postgres psql -h localhost -U postgres -d identity

-- Check your SDK token status
SELECT
  token_id,
  device_name,
  created_at,
  last_used_at,
  revoked_at IS NULL as is_active,
  revoke_reason
FROM sdk_tokens
WHERE user_id = 'YOUR_USER_ID'
ORDER BY created_at DESC
LIMIT 5;

-- Result interpretation:
-- is_active = true  → Token is valid
-- is_active = false → Token has been revoked (normal after rotation)
```

### Trace Token Rotation Chain

```sql
-- See token rotation history
WITH RECURSIVE token_chain AS (
  -- Start with current token
  SELECT
    token_id,
    rotated_from_token_id,
    created_at,
    revoked_at,
    1 as depth
  FROM sdk_tokens
  WHERE token_id = 'YOUR_CURRENT_TOKEN_ID'

  UNION ALL

  -- Follow rotation chain backwards
  SELECT
    t.token_id,
    t.rotated_from_token_id,
    t.created_at,
    t.revoked_at,
    tc.depth + 1
  FROM sdk_tokens t
  JOIN token_chain tc ON t.token_id = tc.rotated_from_token_id
  WHERE tc.depth < 10  -- Limit recursion
)
SELECT
  depth,
  token_id,
  created_at,
  revoked_at IS NULL as still_active
FROM token_chain
ORDER BY depth;

-- Shows complete rotation history back to original SDK download
```

### Monitor Token Refresh Attempts

```bash
# Watch backend logs for token refresh
docker logs -f identity-backend | grep "refresh"

# Look for:
# ✅ "Token refresh successful"
# ❌ "Token refresh failed: invalid token"
# 🔄 "Token rotated: old_token → new_token"
```

### Test Authentication Flow

```python
#!/usr/bin/env python3
"""Test complete authentication flow"""

from aim_sdk import secure
from aim_sdk.oauth import OAuthTokenManager
import time

print("=== AIM Authentication Test ===\n")

# Step 1: Check credentials exist
print("1. Checking credentials...")
manager = OAuthTokenManager()
if not manager.has_credentials():
    print("❌ No credentials found")
    print("   Download SDK from portal")
    exit(1)
print("✅ Credentials found")

# Step 2: Test token refresh
print("\n2. Testing token refresh...")
access_token = manager.get_access_token()
if not access_token:
    print("❌ Token refresh failed")
    print("   Token may be expired/revoked")
    exit(1)
print(f"✅ Access token obtained: {access_token[:20]}...")

# Step 3: Test agent registration/login
print("\n3. Testing agent authentication...")
try:
    client = secure("test-agent")
    print(f"✅ Agent authenticated: {client.agent_id}")
except Exception as e:
    print(f"❌ Agent authentication failed: {e}")
    exit(1)

# Step 4: Test verification flow
print("\n4. Testing verification flow...")
try:
    verification = client.verify_action(
        action_type="test_action",
        resource="test_resource",
        context={"test": True}
    )
    if verification and 'verification_id' in verification:
        print(f"✅ Verification successful: {verification['verification_id']}")
    else:
        print(f"⚠️  Verification returned unexpected response")
except Exception as e:
    print(f"❌ Verification failed: {e}")
    exit(1)

print("\n=== All Tests Passed ===")
```

---

## Security Considerations

### Token Storage Security

**Credentials File Permissions**:
```bash
# MUST be 600 (owner read/write only)
chmod 600 ~/.aim/credentials.json

# Verify
ls -la ~/.aim/credentials.json
# Should show: -rw------- (600)

# If not:
sudo chmod 600 ~/.aim/credentials.json
sudo chown $USER ~/.aim/credentials.json
```

**Never**:
- ❌ Commit credentials to Git
- ❌ Share credentials via email/Slack
- ❌ Store credentials in public locations
- ❌ Use credentials from untrusted sources

**Always**:
- ✅ Use file permissions 600
- ✅ Download SDK from official portal
- ✅ Rotate credentials after security incidents
- ✅ Use separate credentials per environment

### Detecting Token Theft

**Signs of Compromised Tokens**:

1. **Multiple Failed Refresh Attempts**
   ```sql
   -- Check for suspicious activity
   SELECT
     token_id,
     COUNT(*) as failed_attempts
   FROM audit_logs
   WHERE event_type = 'token_refresh_failed'
     AND created_at > NOW() - INTERVAL '1 hour'
   GROUP BY token_id
   HAVING COUNT(*) > 5;
   ```

2. **Revoked Token Usage**
   ```sql
   -- Alerts for using revoked tokens (suspicious!)
   SELECT
     token_id,
     user_id,
     attempted_at,
     source_ip
   FROM security_alerts
   WHERE alert_type = 'revoked_token_usage'
     AND created_at > NOW() - INTERVAL '24 hours';
   ```

3. **Unusual Geographic Locations**
   ```sql
   -- Token used from different IPs/locations
   SELECT
     token_id,
     COUNT(DISTINCT source_ip) as unique_ips,
     array_agg(DISTINCT source_ip) as ips
   FROM token_usage_logs
   WHERE created_at > NOW() - INTERVAL '1 hour'
   GROUP BY token_id
   HAVING COUNT(DISTINCT source_ip) > 2;
   ```

**Immediate Response**:
```bash
# Revoke all tokens for a user
curl -X POST http://localhost:8080/api/v1/admin/revoke-user-tokens \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"user_id": "COMPROMISED_USER_ID"}'

# Force password reset
curl -X POST http://localhost:8080/api/v1/admin/force-password-reset \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"user_id": "COMPROMISED_USER_ID"}'

# Notify user
# Investigate source of compromise
# Review access logs
```

---

## FAQ

### Q: Why do I need to download SDK repeatedly?

**A**: You don't, normally. If you're downloading SDK frequently, something is wrong:

**Normal**: Download SDK once per environment, use continuously
**Problem**: Downloading daily/weekly indicates token rotation issues

**Root Causes**:
- Using SDK from multiple locations (tokens conflict)
- Copying credentials between machines
- Manual token manipulation

**Solution**: Use SDK from ONE location, let it handle rotation automatically.

---

### Q: Can I extend token lifetime?

**A**: Token lifetimes are security-mandated:

- **Access Token**: 1 hour (cannot extend - security requirement)
- **Refresh Token**: 90 days (can configure, but not recommended)

**Why Short TTL**:
- Limits damage if token stolen
- Forces regular re-authentication
- Meets compliance requirements (SOC 2, HIPAA)

**If You Need Longer**:
- Use API keys (no expiration, different security model)
- Use service accounts with API keys
- OAuth tokens MUST have short TTL for security

---

### Q: What if I'm debugging and need stable tokens?

**A**: For development/debugging:

**Option 1**: Use API Keys (no rotation)
```python
client = AIMClient(
    agent_id="...",
    api_key="your-api-key",  # No token rotation
    aim_url="..."
)
```

**Option 2**: Use Test Environment
```bash
# Set up test environment with relaxed policies
export AIM_ENV=development
export TOKEN_ROTATION_ENABLED=false  # Dev only!
```

**Option 3**: Download Fresh SDK Each Session
```bash
# Automated script
./dev/get-fresh-tokens.sh
```

**Never Disable in Production**: Token rotation is required for security.

---

## Summary

### Quick Checklist for Auth Issues

- [ ] Backend running? (`curl http://localhost:8080/health`)
- [ ] Credentials exist? (`ls ~/.aim/credentials.json`)
- [ ] Credentials valid JSON? (`python -m json.tool ~/.aim/credentials.json`)
- [ ] Has refresh_token? (`cat ~/.aim/credentials.json | grep refresh_token`)
- [ ] Has private_key? (`cat ~/.aim/credentials.json | jq '.["agent-name"].private_key'`)
- [ ] Correct permissions? (`ls -la ~/.aim/credentials.json` → should be 600)
- [ ] Token not revoked? (Check database or download fresh SDK)
- [ ] System time correct? (`date`)

### When to Download Fresh SDK

Download fresh SDK if:
- ✅ "Token expired" error
- ✅ "Invalid credentials" error
- ✅ Haven't used SDK in 90+ days
- ✅ Copied credentials between machines
- ✅ Suspect token theft/compromise

Don't download fresh SDK if:
- ❌ Normal operation (let rotation work)
- ❌ Every debug session (use API keys for debugging)

---

**Need More Help?**

- Review [Token Rotation Guide](../security/token-rotation.md)
- Check [Main Troubleshooting Guide](./README.md)
- Submit issue on [GitHub](https://github.com/opena2a-org/agent-identity-management/issues)

---

**Last Updated**: October 18, 2025
**Version**: 1.0
**For**: AIM Platform v1.0+
