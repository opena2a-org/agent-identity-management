# End-to-End Security Testing Prompt

## Context for New Claude Session

You are testing the **Priority 1 Security Features** for AIM's OAuth SDK download system. The previous session implemented:

1. **SDK Token Tracking** with SHA-256 hashing
2. **90-day token expiry** (reduced from 365 days)
3. **Token rotation** on every refresh
4. **Encrypted credential storage** in Python SDK
5. **Token revocation** endpoints
6. **Complete audit trail** with usage tracking

**Your task**: Verify everything works end-to-end through comprehensive testing.

## What Was Implemented

### Backend (Go)
- `migrations/022_create_sdk_tokens_table.up.sql` - Token tracking database
- `internal/domain/sdk_token.go` - Domain model
- `internal/infrastructure/repository/sdk_token_repository.go` - Repository
- `internal/application/sdk_token_service.go` - Business logic
- `internal/infrastructure/auth/jwt.go` - Token rotation support
- `internal/interfaces/http/handlers/sdk_handler.go` - Updated download handler
- `internal/interfaces/http/handlers/sdk_token_handler.go` - Revocation endpoints
- `internal/interfaces/http/handlers/auth_refresh_handler.go` - Refresh with rotation
- `cmd/server/main.go` - Wired up everything

### Frontend (TypeScript)
- No changes yet (endpoints ready for UI integration)

### Python SDK
- `sdks/python/aim_sdk/secure_storage.py` - Encrypted credential storage
- `sdks/python/aim_sdk/oauth.py` - Updated with token rotation support

### New API Endpoints

1. `POST /api/v1/auth/refresh` - Refresh with token rotation
2. `GET /api/v1/users/me/sdk-tokens` - List tokens
3. `GET /api/v1/users/me/sdk-tokens/count` - Active count
4. `POST /api/v1/users/me/sdk-tokens/:id/revoke` - Revoke one
5. `POST /api/v1/users/me/sdk-tokens/revoke-all` - Revoke all

## Prerequisites

- [ ] PostgreSQL running
- [ ] Redis running
- [ ] Backend binary built (`apps/backend/server`)
- [ ] Frontend running (`apps/web`)
- [ ] Google OAuth credentials configured

## Testing Plan

### Phase 1: Database Migration (5 min)

**Test**: SDK tokens table created correctly

```bash
# Start PostgreSQL if not running
brew services start postgresql@14

# Connect to database
psql -U postgres -d aim

# Check migration
\d sdk_tokens

# Expected output: Table with columns:
# - id, user_id, organization_id, token_hash, token_id
# - device_name, device_fingerprint, ip_address, user_agent
# - last_used_at, last_ip_address, usage_count
# - created_at, expires_at, revoked_at, revoke_reason, metadata
```

**Success Criteria**:
- ✅ Table exists
- ✅ All columns present with correct types
- ✅ Indexes created (check with `\di`)

### Phase 2: Backend API Testing (15 min)

#### 2.1 Start Backend Server

```bash
cd /Users/decimai/workspace/agent-identity-management
./apps/backend/server
```

**Verify**:
- ✅ "Server listening on :8080"
- ✅ "Database connection established"
- ✅ "Redis connection established"
- ✅ No compilation errors

#### 2.2 Test Health Check

```bash
curl http://localhost:8080/health
```

**Expected**: `{"status":"healthy","service":"agent-identity-management",...}`

#### 2.3 Test OAuth Login (Get Access Token)

1. Open browser: http://localhost:3000
2. Click "Sign in with Google"
3. Complete OAuth flow
4. Open browser console (F12)
5. Run: `localStorage.getItem('auth_token')`
6. Copy the token

```bash
# Save token for testing
export AUTH_TOKEN="your-access-token-here"
```

#### 2.4 Test SDK Download (Token Tracking)

```bash
# Download SDK (this should create a tracked token)
curl -X GET http://localhost:8080/api/v1/sdk/download \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -o test-sdk.zip

# Verify download
unzip -l test-sdk.zip | grep credentials.json
```

**Expected**:
- ✅ ZIP file downloaded
- ✅ Contains `.aim/credentials.json`

**Verify Token Tracked in Database**:
```sql
psql -U postgres -d aim -c "
SELECT
    id,
    LEFT(token_hash, 16) as token_hash_preview,
    token_id,
    ip_address,
    user_agent,
    expires_at,
    created_at
FROM sdk_tokens
ORDER BY created_at DESC
LIMIT 1;
"
```

**Success Criteria**:
- ✅ One row inserted
- ✅ `token_hash` is 64-character hex string (SHA-256)
- ✅ `token_id` is UUID (JTI claim)
- ✅ `ip_address` is present
- ✅ `expires_at` is ~90 days from now
- ✅ `revoked_at` is NULL

#### 2.5 Test Token Rotation

**Extract refresh token from downloaded SDK**:
```bash
cd test-sdk && unzip -q ../test-sdk.zip
cat aim-sdk-python/.aim/credentials.json | python3 -c "import sys, json; print(json.load(sys.stdin)['refresh_token'])"
```

**Test refresh endpoint**:
```bash
# Save refresh token
export REFRESH_TOKEN="your-refresh-token-here"

# Call refresh endpoint
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" \
  | python3 -m json.tool
```

**Expected Response**:
```json
{
  "access_token": "new-access-token-xyz",
  "refresh_token": "new-refresh-token-abc",  ← DIFFERENT from input!
  "token_type": "Bearer",
  "expires_in": 86400
}
```

**Success Criteria**:
- ✅ Returns new `access_token`
- ✅ Returns new `refresh_token` (MUST be different from old one!)
- ✅ Old refresh token should be invalid (test below)

**Verify Old Token Invalidated**:
```bash
# Try to use old refresh token again (should fail)
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}"
```

**Expected**: Error (token invalid or expired)

#### 2.6 Test Token Revocation

**List SDK Tokens**:
```bash
curl -X GET http://localhost:8080/api/v1/users/me/sdk-tokens \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  | python3 -m json.tool
```

**Expected**: Array of tokens with metadata

**Get Active Count**:
```bash
curl -X GET http://localhost:8080/api/v1/users/me/sdk-tokens/count \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  | python3 -m json.tool
```

**Expected**: `{"active_count": 1}` (or more if multiple downloads)

**Revoke Specific Token**:
```bash
# Get token ID from list above
export TOKEN_ID="token-uuid-here"

curl -X POST http://localhost:8080/api/v1/users/me/sdk-tokens/$TOKEN_ID/revoke \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Testing revocation"}' \
  | python3 -m json.tool
```

**Expected**: `{"message": "Token revoked successfully"}`

**Verify Revocation in Database**:
```sql
psql -U postgres -d aim -c "
SELECT
    id,
    revoked_at,
    revoke_reason
FROM sdk_tokens
WHERE id = 'token-uuid-here';
"
```

**Success Criteria**:
- ✅ `revoked_at` is NOT NULL
- ✅ `revoke_reason` is "Testing revocation"

**Test Revoke All**:
```bash
curl -X POST http://localhost:8080/api/v1/users/me/sdk-tokens/revoke-all \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Testing bulk revocation"}' \
  | python3 -m json.tool
```

**Verify All Tokens Revoked**:
```sql
psql -U postgres -d aim -c "
SELECT COUNT(*) as total, COUNT(revoked_at) as revoked
FROM sdk_tokens
WHERE user_id = (
    SELECT id FROM users WHERE email = 'your-email@gmail.com'
);
"
```

**Expected**: `total = revoked` (all tokens revoked)

### Phase 3: Python SDK Testing (20 min)

#### 3.1 Install SDK with Security Dependencies

```bash
cd /Users/decimai/workspace/agent-identity-management/sdks/python

# Install with security dependencies
pip3 install -e ".[security]"

# Or manually
pip3 install -e .
pip3 install cryptography keyring
```

**Verify Installation**:
```bash
python3 -c "from aim_sdk.secure_storage import SecureCredentialStorage; print('✅ Secure storage available')"
```

#### 3.2 Extract SDK Credentials

```bash
cd ~/Downloads/
unzip test-sdk.zip
cd aim-sdk-python

# Move credentials to home directory
mkdir -p ~/.aim
cp .aim/credentials.json ~/.aim/

# Verify credentials exist
cat ~/.aim/credentials.json | python3 -m json.tool
```

#### 3.3 Test Encrypted Storage Migration

```python
# test_encrypted_storage.py
from aim_sdk.secure_storage import SecureCredentialStorage
import json
from pathlib import Path

# Load plaintext credentials
creds_path = Path.home() / ".aim" / "credentials.json"
with open(creds_path) as f:
    creds = json.load(f)

print("📄 Original credentials (plaintext):")
print(f"  - aim_url: {creds['aim_url']}")
print(f"  - user_id: {creds['user_id']}")
print(f"  - email: {creds['email']}")
print(f"  - refresh_token: {creds['refresh_token'][:20]}...")

# Create secure storage
storage = SecureCredentialStorage()

# Migrate to encrypted storage
if storage.cipher:
    print("\n🔐 Migrating to encrypted storage...")
    storage.migrate_to_encrypted()

    # Verify encrypted file exists
    encrypted_path = Path.home() / ".aim" / "credentials.encrypted"
    if encrypted_path.exists():
        print(f"✅ Encrypted file created: {encrypted_path}")
        print(f"   File size: {encrypted_path.stat().st_size} bytes")

        # Verify can load encrypted credentials
        loaded = storage.load_credentials()
        if loaded == creds:
            print("✅ Decryption successful! Credentials match.")
        else:
            print("❌ Decryption failed! Credentials don't match.")
    else:
        print("❌ Encrypted file not created")
else:
    print("⚠️  Encryption not available (install cryptography + keyring)")
```

Run test:
```bash
python3 test_encrypted_storage.py
```

**Success Criteria**:
- ✅ "Encrypted file created"
- ✅ "Decryption successful"
- ✅ Plaintext file removed (optional, depends on implementation)

#### 3.4 Test Token Rotation in SDK

```python
# test_token_rotation.py
from aim_sdk.oauth import OAuthTokenManager
import time

# Create OAuth manager
manager = OAuthTokenManager()

print("🔄 Testing token rotation...")

# Get current refresh token
old_refresh = manager.credentials['refresh_token']
print(f"Old refresh token: {old_refresh[:20]}...")

# Force token refresh
print("\n📡 Forcing token refresh...")
access_token = manager._refresh_token()

if access_token:
    print(f"✅ Got new access token: {access_token[:20]}...")

    # Check if refresh token rotated
    new_refresh = manager.credentials['refresh_token']
    print(f"New refresh token: {new_refresh[:20]}...")

    if new_refresh != old_refresh:
        print("✅ Token rotation successful! Refresh token changed.")
    else:
        print("⚠️  Token rotation not implemented or server doesn't support it")
else:
    print("❌ Token refresh failed")
```

Run test:
```bash
python3 test_token_rotation.py
```

**Success Criteria**:
- ✅ "Got new access token"
- ✅ "Token rotation successful! Refresh token changed"
- ✅ New credentials saved (check file timestamp)

#### 3.5 Test Agent Registration with OAuth

```python
# test_oauth_agent_registration.py
from aim_sdk import register_agent

print("🧪 Testing OAuth-based agent registration...")

try:
    # Register agent (should use OAuth automatically)
    client = register_agent(
        name="test-security-agent",
        display_name="Test Security Agent",
        description="Testing Priority 1 security features",
        type="ai_agent",
        tags=["test", "security", "oauth"]
    )

    print(f"\n✅ Agent registered successfully!")
    print(f"   Agent ID: {client.agent_id}")
    print(f"   Public Key: {client.public_key[:50]}...")

    # Verify OAuth was used
    if "Using OAuth authentication" in str(client):
        print("✅ OAuth authentication confirmed")

except Exception as e:
    print(f"❌ Registration failed: {e}")
```

Run test:
```bash
python3 test_oauth_agent_registration.py
```

**Success Criteria**:
- ✅ "Auto-detected AIM URL"
- ✅ "Using OAuth authentication"
- ✅ "Agent registered successfully"
- ✅ Agent ID returned

#### 3.6 Verify Agent Ownership

```sql
psql -U postgres -d aim -c "
SELECT
    a.id as agent_id,
    a.name as agent_name,
    u.email as owner_email,
    u.google_id,
    o.name as organization_name,
    a.created_at
FROM agents a
JOIN users u ON a.created_by = u.id
JOIN organizations o ON a.organization_id = o.id
WHERE a.name = 'test-security-agent'
ORDER BY a.created_at DESC
LIMIT 1;
"
```

**CRITICAL Success Criteria**:
- ✅ `owner_email` is YOUR Google email (NOT default@example.com)
- ✅ `google_id` exists
- ✅ Agent linked to real authenticated user

### Phase 4: Security Validation (15 min)

#### 4.1 Test Expired Token Rejection

**Manually expire a token in database**:
```sql
psql -U postgres -d aim -c "
UPDATE sdk_tokens
SET expires_at = NOW() - INTERVAL '1 day'
WHERE id = (SELECT id FROM sdk_tokens ORDER BY created_at DESC LIMIT 1)
RETURNING id, expires_at;
"
```

**Try to use expired token**:
```bash
# Should fail with 401 Unauthorized
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$EXPIRED_REFRESH_TOKEN\"}"
```

**Expected**: 401 Unauthorized or "expired" error

#### 4.2 Test Revoked Token Rejection

**Revoke a token via API**:
```bash
curl -X POST http://localhost:8080/api/v1/users/me/sdk-tokens/$TOKEN_ID/revoke \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{"reason": "Testing revocation enforcement"}'
```

**Try to use revoked token**:
```bash
# Should fail
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REVOKED_REFRESH_TOKEN\"}"
```

**Expected**: 401 Unauthorized

#### 4.3 Test Token Hash Security

**Verify tokens are hashed in database**:
```sql
psql -U postgres -d aim -c "
SELECT
    LEFT(token_hash, 20) as hash_preview,
    LENGTH(token_hash) as hash_length,
    token_id
FROM sdk_tokens
ORDER BY created_at DESC
LIMIT 3;
"
```

**Success Criteria**:
- ✅ `hash_length` = 64 (SHA-256 produces 64 hex chars)
- ✅ `hash_preview` contains only hex characters (0-9, a-f)
- ✅ Different tokens have different hashes

#### 4.4 Test 90-Day Expiry

```sql
psql -U postgres -d aim -c "
SELECT
    created_at,
    expires_at,
    EXTRACT(DAY FROM (expires_at - created_at)) as days_until_expiry
FROM sdk_tokens
WHERE revoked_at IS NULL
ORDER BY created_at DESC
LIMIT 5;
"
```

**Success Criteria**:
- ✅ `days_until_expiry` ≈ 90 (allow ±1 for rounding)
- ✅ NOT 365 days (old behavior)

### Phase 5: Dashboard UI Verification (Optional - 10 min)

If frontend is running, verify in browser:

1. **Navigate**: http://localhost:3000/dashboard/settings (or wherever SDK tokens will be shown)
2. **Verify**: Can see list of SDK tokens
3. **Test**: Revoke button works
4. **Verify**: Revoked token disappears or shows as revoked

## Final Verification Checklist

### Database
- [ ] `sdk_tokens` table exists
- [ ] Tokens have SHA-256 hashes (64 chars)
- [ ] Tokens expire in 90 days (not 365)
- [ ] Revoked tokens have `revoked_at` timestamp
- [ ] Usage tracking works (last_used_at, usage_count)

### Backend API
- [ ] SDK download creates tracked token
- [ ] `/auth/refresh` returns new refresh token
- [ ] Old refresh tokens are invalidated
- [ ] `/users/me/sdk-tokens` lists tokens
- [ ] `/users/me/sdk-tokens/:id/revoke` revokes token
- [ ] `/users/me/sdk-tokens/revoke-all` revokes all
- [ ] Expired tokens are rejected
- [ ] Revoked tokens are rejected

### Python SDK
- [ ] Encrypted storage works (cryptography + keyring)
- [ ] Token rotation updates credentials file
- [ ] OAuth authentication auto-detects URL
- [ ] Agent registration uses OAuth
- [ ] Agents linked to real user (not default)

### Security
- [ ] Credentials encrypted at rest (if dependencies installed)
- [ ] Tokens hashed in database (SHA-256)
- [ ] Token rotation working (new token on refresh)
- [ ] Revocation immediate (tokens rejected)
- [ ] 90-day expiry enforced
- [ ] Audit trail complete (who, when, why)

## Expected Issues & Solutions

### Issue 1: "cryptography not installed"
**Solution**: `pip install cryptography keyring`

### Issue 2: "Keyring backend not available"
**Solution**: Falls back to plaintext (expected on some systems)

### Issue 3: "Token refresh returns same token"
**Solution**: Check backend logs, verify RefreshTokenPair() is called

### Issue 4: "Agent still owned by default user"
**Solution**: Check OptionalAuthMiddleware is extracting JWT correctly

## Success Metrics

**✅ Priority 1 Security Complete** if:
- 90-day expiry verified (database query)
- Token rotation verified (different tokens returned)
- Encrypted storage verified (.encrypted file exists)
- Revocation verified (tokens rejected after revoke)
- Token hashing verified (SHA-256 in database)
- Audit trail verified (complete metadata)

## What to Report Back

1. **Pass/Fail Status** for each phase
2. **Database Query Results** showing:
   - Token hashes (first 20 chars)
   - Expiry dates (90 days)
   - Revocation timestamps
   - Agent ownership (real user email)
3. **API Response Examples** showing token rotation
4. **Any Errors** with full stack traces
5. **Screenshots** of dashboard (if applicable)

## Next Steps After Testing

If all tests pass:
1. Merge `feature/sdk-token-security` to main
2. Deploy to staging environment
3. Run penetration tests
4. Update user documentation
5. Announce security improvements

If tests fail:
1. Document failures with logs
2. Create GitHub issues
3. Fix issues in same branch
4. Re-run tests

---

Good luck with testing! This is the culmination of Priority 1 security work. 🔒
