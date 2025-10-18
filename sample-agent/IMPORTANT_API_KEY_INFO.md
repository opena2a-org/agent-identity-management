# ⚠️ IMPORTANT: API Key Information

## The Problem

The API keys shown in the dashboard (`aim_live_Jc1Yj3u` and `aim_live_yXBE5we`) are **PREFIXES ONLY**, not the full keys!

### How API Keys Work

1. **Full Key Format**: `aim_live_` + 43 characters of base64
   - Example: `aim_live_yXBE5weABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890`

2. **What's Stored in Database**: SHA-256 hash of the full key

3. **What's Shown in Dashboard**: Only the first 16 characters (prefix) for security

### Why Your Keys Don't Work

The keys `aim_live_Jc1Yj3u` and `aim_live_yXBE5we` are incomplete. They're only 15-16 characters, but the full key should be ~52 characters total.

## Solution: Create a New API Key

### Option 1: Via Dashboard (Recommended)

1. Go to: http://localhost:3000/dashboard/api-keys
2. Click "Create API Key"
3. Enter a name (e.g., "SDK Test Key")
4. Select an agent
5. Click "Generate"
6. **COPY THE FULL KEY IMMEDIATELY** - it will look like:
   ```
   aim_live_abc123def456ghi789jkl012mno345pqr678stu901vwx234yz
   ```
7. The full key is only shown once!

### Option 2: Via API (if you have a JWT token)

```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "YOUR_AGENT_ID",
    "name": "SDK Test Key",
    "expires_in_days": 365
  }'
```

The response will contain the full API key in the `key` field.

### Option 3: Direct Database Query (Development Only)

If you have database access, you can create a key directly:

```sql
-- This is for development/testing only!
-- Generate a proper key using the backend API in production

INSERT INTO api_keys (
  id,
  organization_id,
  agent_id,
  name,
  key_hash,
  prefix,
  is_active,
  created_by,
  created_at,
  updated_at
) VALUES (
  gen_random_uuid(),
  'YOUR_ORG_ID',
  'YOUR_AGENT_ID',
  'Test SDK Key',
  '1LlgywJ/gd5Phy+n7XMXrM0og4aghIaywfOYqW3eUGo=',  -- Hash of aim_live_yXBE5we
  'aim_live_yXBE5we',
  true,
  'YOUR_USER_ID',
  NOW(),
  NOW()
);
```

But you still need the full key, not just the prefix!

## Next Steps

1. Create a new API key via the dashboard
2. Copy the FULL key (should be ~52 characters)
3. Update `sample-agent/agent.js` with the full key
4. Run `npm start`

The full key will look something like:
```
aim_live_yXBE5weQzRvT8uN2pL5mK9jH3gF6dS1aW4xC7bV0nM8
```

Not just:
```
aim_live_yXBE5we  ❌ (This is incomplete!)
```


