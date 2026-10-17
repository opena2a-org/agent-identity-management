# Token Rotation - Security Feature

## Overview

AIM uses **token rotation** to protect your organization from token theft and unauthorized access. Rotating short-lived credentials is a control SOC 2, HIPAA and GDPR programs commonly expect.

## What is Token Rotation?

Token rotation is a security mechanism where:
1. **A refresh that rotates** → backend issues a **NEW** refresh token
2. **The OLD refresh token is invalidated** → cannot be reused
3. **Your credentials are automatically updated** with the new token

Which refreshes rotate depends on the token and on the server:
- A token from an SDK download is rotated on every refresh, in the database.
- A sign-in from `aim-sdk login` is rotated only by a server with Redis. A server without Redis
  answers a refresh with the refresh token it was given, and logout cannot revoke it. See
  [How do I know if my token has been rotated?](#q-how-do-i-know-if-my-token-has-been-rotated).

A stolen refresh token is refused once rotation has retired it. How long one keeps working before
that is in [The Solution: Token Rotation](#the-solution-token-rotation).

---

## Why Does This Happen?

### The Problem: Token Theft

A refresh at `POST /api/v1/auth/refresh` asks for the refresh token and nothing else, so whoever
holds a copy of a refresh token can refresh with it, as the agent does.

### The Solution: Token Rotation

A refresh that rotates retires the refresh token it was given. Where refreshes rotate (see
[What is Token Rotation?](#what-is-token-rotation)), how long a stolen copy keeps working depends
on who refreshes first:

- **The agent refreshes first.** The stolen copy is retired. When the thief presents it, it is
  refused and the token family ends, the agent's new token included (see the
  [theft scenario](#theft-scenario-token-rotation-protects-you)).
- **The thief refreshes first.** The thief now holds the newest refresh token and can keep
  refreshing while the agent is silent. The agent's next refresh presents the retired token, and
  that ends the family, the thief's token included.

While the agent is silent, the thief's tokens keep working at most this long:

- **A sign-in from `aim-sdk login`**: until `JWT_SESSION_MAX_AGE` after the sign-in (**8 hours**
  by default), however often it is refreshed. From then on its refresh token is refused, and so
  are its access tokens.
- **A token from an SDK download**: as long as it is refreshed at least every **90 days**. It has
  no maximum age, and each refresh issues a refresh token that lasts 90 days.
- **An access token**: at most `JWT_ACCESS_TTL` from when it was issued (**2 hours** by default).
  A refresh does not retire an access token, so one stolen before a refresh is still accepted
  after it.

On a server without Redis a sign-in from `aim-sdk login` is not rotated, so a stolen copy of its
refresh token keeps working as long as the agent's own, until the sign-in ends.

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
   → Backend revokes the whole token family, refresh_token_v2 included
   → Backend writes a refresh_token_reuse audit record
```

**Result**: The reused token is refused, and so is every token of its token family
from then on. The backend cannot tell which holder is the legitimate one, so the
agent's own refresh_token_v2 stops working too: download a fresh SDK or run
`aim-sdk login` to get new credentials. The reuse is recorded in the audit log;
no notification is sent.

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
client.verify_capability(...)
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
cp dev/.aim/sdk_credentials.json production/.aim/  # Will cause rotation conflicts
```

---

## For Administrators

### Security Benefits

Token rotation provides:

1. **Automatic Token Revocation**
   - A refresh that rotates retires the refresh token it was given
   - A retired refresh token presented again is refused and ends its token family
   - A stolen token is not refused at once: [The Solution: Token Rotation](#the-solution-token-rotation)
     says how long one keeps working

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

There is no configuration file for token rotation. The backend reads these environment variables:

- `REDIS_HOST` and `REDIS_PORT`: the Redis that records retired and revoked refresh tokens. With it,
  a sign-in from `aim-sdk login` is rotated on every refresh and logout revokes it. Without it
  (`REDIS_HOST` set to the empty string, or a Redis that could not be reached at startup), a sign-in
  is not rotated and logout cannot revoke it. A token from an SDK download is rotated either way.
- `AUTH_REVOCATION_FAIL_OPEN`: when the backend cannot read Redis to check a token, it refuses the
  token. Set it to `true` to accept the token instead.
- `JWT_ACCESS_TTL`: how long an access token lasts, and so how often the SDK refreshes.

The backend's startup log says which applies, in one of these lines:
```
Token revocation enabled (fail-open=false)
Token revocation disabled (no Redis configured): refresh tokens are not rotated and logout cannot revoke
Token revocation disabled (Redis unavailable): refresh tokens are not rotated and logout cannot revoke
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

**Dashboard** - View SDK tokens:
- Navigate to: Developers → Credentials → SDK tokens (`/dashboard/credentials#sdk-tokens`)
- See each token's last use and IP address; **Show revoked** lists revoked tokens too

**Audit log** - Review reused tokens (administrators):
- Navigate to: Organization → Audit log (`/dashboard/admin/audit-logs`)
- A rotated-out refresh token presented again is recorded as `refresh_token_reuse` and
  revokes its token family; each later refused token of that family is recorded as
  `refresh_session_revoked`
- Neither sends a notification, so review the audit log for these records

---

## FAQs

### Q: How often do tokens rotate?

**A:** A refresh token that rotates does so each time it is used to refresh the access token: a
token from an SDK download on every server, and a sign-in from `aim-sdk login` on a server with
Redis (see [Configuration](#configuration)).

Typical rotation frequency:
- Access tokens last **2 hours** by default (`JWT_ACCESS_TTL` on the server)
- The SDK refreshes an access token once it is within a minute of expiring → the refresh rotates the refresh token
- So a refresh token that rotates does so about **every 2 hours** during active use

### Q: Will my agent stop working during rotation?

**A:** No. Token rotation is **completely transparent** to your agent.

The SDK handles rotation automatically:
1. Old token used → backend issues new token
2. SDK automatically saves new token
3. Next request uses new token
4. **No interruption to your agent**

### Q: Can I disable token rotation?

**A:** Not for a token from an SDK download, which rotates on every refresh. A sign-in from
`aim-sdk login` rotates only on a server with Redis: one started with `REDIS_HOST` set to the empty
string does not rotate it, and cannot revoke it at logout. Run the server with Redis; rotation is a
control SOC 2, HIPAA and GDPR programs commonly expect.

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

2. **Download Fresh SDK** each time: sign in to the dashboard and download the
   SDK again from its SDK page (`/dashboard/sdk`). A downloaded SDK's credential
   cannot download another SDK; the download takes a sign-in. The flight-search
   example prints these steps:
   ```bash
   python3 examples/flight-search-agent/get_fresh_sdk.py
   ```

3. **Use Test Environment** with relaxed policies

### Q: How do I know if my token has been rotated?

**A:** The Python SDK prints one line when a refresh returns a new refresh token:

```
Token rotated successfully. Old refresh token revoked (new id: 461d130d...)
```

The SDK prints it after saving the new refresh token to its credentials file. It
does not print it for every refresh: the line depends on what the server answers,
and the `rotated` field of the refresh response says which answer it was.

- A sign-in from `aim-sdk login` is rotated only by a server with Redis. The
  server records the old refresh token as revoked there and answers with a new
  one (`"rotated": true`).
- A server without Redis, or one whose Redis refused the write, answers with the
  refresh token it was given (`"rotated": false`), and the SDK prints no line. A
  server without Redis logs `Token revocation disabled (no Redis configured)` at
  startup, and one whose Redis could not be reached logs
  `Token revocation disabled (Redis unavailable)`.
- A token from an SDK download is rotated in the database and does not need
  Redis.

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
       │    { "refreshToken": "old_token" }
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
    │    "accessToken": "new_access",    │
    │    "refreshToken": "new_refresh"   │ ◄── NEW!
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

**For supported customers**:
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

✅ **This is a control SOC 2, HIPAA and GDPR programs commonly expect**

**Bottom line**: Token rotation makes AIM more secure. The minor inconvenience of occasionally downloading fresh credentials is far outweighed by the security benefits.

---

**Last Updated**: October 18, 2025
**Version**: 1.0
**Status**: Production
