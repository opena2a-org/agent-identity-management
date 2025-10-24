# AIM Platform - Troubleshooting Guide

**Quick Navigation**:
- [Authentication Issues](#authentication-issues)
- [Agent Registration Problems](#agent-registration-problems)
- [Dashboard Issues](#dashboard-issues)
- [Performance Problems](#performance-problems)
- [Network & Connectivity](#network--connectivity)
- [Common Error Messages](#common-error-messages)

---

## Authentication Issues

### ❌ "Token Expired" Error

**Symptoms**:
```
❌ Authentication Failed: Token Expired

Your SDK credentials have expired due to token rotation (security policy).
```

**Root Cause**: Your refresh token has been rotated or revoked (this is normal security behavior).

**Quick Fix** (5 minutes):

1. **Log in to portal**:
   ```bash
   open http://localhost:3000/auth/login
   # Or: https://aim.yourdomain.com/auth/login
   ```

2. **Download fresh SDK**:
   - Navigate to: Dashboard → SDK Download
   - Click "Download SDK" for Python/Go/etc.

3. **Copy new credentials**:
   ```bash
   # Extract SDK
   unzip aim-sdk-python.zip

   # Copy credentials
   cp -r aim-sdk-python/.aim ~/.aim/

   # Verify permissions
   chmod 600 ~/.aim/credentials.json
   ```

4. **Restart your agent**:
   ```bash
   python your_agent.py
   ```

**Why This Happens**:
- AIM uses **token rotation** for security (SOC 2 / HIPAA compliant)
- When you use a refresh token → backend issues NEW token
- OLD token is revoked → prevents token theft
- This is **correct behavior**, not a bug

**See**: [Token Rotation Documentation](../security/token-rotation.md)

---

### ❌ "Invalid Credentials" Error

**Symptoms**:
```
❌ Authentication Failed: Invalid credentials format

Your agent credentials appear to be invalid or corrupted.
```

**Common Causes**:

#### 1. Missing Credentials File

**Check**:
```bash
ls -la ~/.aim/credentials.json
```

**Fix**:
```bash
# Download SDK from portal
# Extract and copy credentials
cp -r aim-sdk-python/.aim ~/.aim/
```

#### 2. Corrupted Credentials

**Check**:
```bash
# Verify JSON is valid
cat ~/.aim/credentials.json | python -m json.tool
```

**Fix**:
```bash
# Download fresh SDK if corrupted
# JSON parse errors indicate corruption
```

#### 3. Missing OAuth Tokens

**Check**:
```bash
# Credentials should have both agent keys AND OAuth tokens
cat ~/.aim/credentials.json | grep -E "(refresh_token|private_key)"
```

**Expected Structure**:
```json
{
  "refresh_token": "...",     // OAuth token (root level)
  "sdk_token_id": "...",      // OAuth token ID
  "aim_url": "http://...",
  "your-agent-name": {
    "agent_id": "...",
    "private_key": "...",     // Agent Ed25519 key
    "public_key": "..."
  }
}
```

**Fix**:
```bash
# Download fresh SDK to get complete credentials
```

---

### ❌ OAuth Login Fails

**Symptoms**:
- Redirect loop during Microsoft/Google login
- "Invalid state parameter" error
- Blank page after OAuth callback

**Troubleshooting Steps**:

#### 1. Check Environment Variables

```bash
# Backend .env file
cat apps/backend/.env

# Required:
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
MICROSOFT_CLIENT_ID=...
MICROSOFT_CLIENT_SECRET=...
OAUTH_REDIRECT_URL=http://localhost:3000/auth/callback
```

#### 2. Verify OAuth App Configuration

**Google OAuth**:
- Go to: Google Cloud Console → APIs & Credentials
- Check authorized redirect URIs include:
  ```
  http://localhost:3000/auth/callback
  https://yourdomain.com/auth/callback
  ```

**Microsoft OAuth**:
- Go to: Azure Portal → App Registrations
- Check redirect URIs match your configuration

#### 3. Clear Browser Cookies

```bash
# Chrome: Cmd+Shift+Delete (Mac) or Ctrl+Shift+Delete (Windows)
# Clear cookies for localhost:3000
```

#### 4. Check Backend Logs

```bash
# Docker
docker logs identity-backend | grep -i oauth

# Local development
tail -f apps/backend/logs/oauth.log
```

---

## Agent Registration Problems

### ❌ Agent Registration Fails

**Symptoms**:
```python
from aim_sdk import secure

client = secure("my-agent")  # Fails here
```

**Common Causes**:

#### 1. Backend Not Running

**Check**:
```bash
curl http://localhost:8080/health

# Expected: {"status": "healthy"}
```

**Fix**:
```bash
# Start backend
docker compose up -d

# Or local development
cd apps/backend
go run cmd/server/main.go
```

#### 2. Missing Credentials

**Check**:
```bash
ls ~/.aim/credentials.json
```

**Fix**: Download SDK from portal (see [Authentication Issues](#authentication-issues))

#### 3. Network Error

**Check**:
```bash
# Test connectivity
curl -v http://localhost:8080/api/v1/health

# Check firewall
sudo lsof -i :8080
```

**Fix**:
```bash
# Ensure no firewall blocking
# Check Docker networking
docker network inspect agent-identity-management_default
```

---

### ❌ Auto-Detection Not Working

**Symptoms**:
- Agent registers but shows 0 capabilities
- Expected MCPs not detected

**Troubleshooting**:

#### 1. Check Code Analysis

Auto-detection scans your code for patterns. Ensure your code is analyzable:

```python
# ✅ This is detectable
def send_email(to, subject, body):
    smtp.send(to, subject, body)

# ❌ This is harder to detect
exec("send_email(...)")
```

#### 2. Manual Capability Addition

If auto-detection misses capabilities, add manually:

```python
client = secure(
    "my-agent",
    auto_detect_capabilities=False,  # Disable auto-detection
    capabilities=[
        {"name": "send_email", "risk_level": "medium"},
        {"name": "read_database", "risk_level": "low"}
    ]
)
```

#### 3. Check Claude Desktop Config

For MCP auto-detection, ensure Claude Desktop config exists:

```bash
# Mac
ls ~/Library/Application\ Support/Claude/claude_desktop_config.json

# Windows
ls %APPDATA%/Claude/claude_desktop_config.json
```

---

## Dashboard Issues

### ❌ Empty Dashboard Tabs

**Symptoms**:
- "Recent Activity" tab shows no data
- "Trust History" shows no chart
- "Connections" shows no MCPs

**Root Causes**:

#### 1. No Verification Events (Most Common)

**Cause**: Agent hasn't performed any verified actions yet.

**Fix**: Run your agent to create verification events:

```python
# Perform an action that requires verification
client.verify_action(
    action_type="search_flights",
    resource="NYC",
    context={"risk_level": "low"}
)
```

**After**: Tabs will populate with data

#### 2. Token Expired (Common)

**Cause**: Authentication failing → no events created

**Symptoms**:
```
⚠️  Verification error: Authentication failed
```

**Fix**: Get fresh credentials (see [Token Expired](#-token-expired-error))

#### 3. Agent Not Active

**Check**:
```sql
-- Database query
SELECT is_active, status
FROM agents
WHERE id = 'your-agent-id';
```

**Fix**:
```bash
# Reactivate agent via dashboard
# Or API call
curl -X PATCH http://localhost:8080/api/v1/agents/YOUR_ID \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"is_active": true}'
```

---

### ❌ Dashboard Not Loading

**Symptoms**:
- Blank page
- "Failed to fetch" errors
- Dashboard stuck on loading spinner

**Troubleshooting**:

#### 1. Check Browser Console

```
F12 → Console Tab
```

Common errors:
- `CORS error` → Backend CORS misconfigured
- `Network error` → Backend not running
- `401 Unauthorized` → Need to log in

#### 2. Verify Backend Running

```bash
curl http://localhost:8080/health

# Expected: {"status": "healthy"}
```

#### 3. Check Frontend Running

```bash
curl http://localhost:3000

# Should return HTML
```

#### 4. Clear Browser Cache

```bash
# Hard reload: Cmd+Shift+R (Mac) or Ctrl+Shift+F5 (Windows)
```

---

## Performance Problems

### ⚠️ Slow API Responses

**Symptoms**:
- API calls taking > 2 seconds
- Dashboard loading slowly
- Timeouts

**Diagnosis**:

#### 1. Check Database Performance

```sql
-- Find slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

#### 2. Check Backend Logs

```bash
docker logs identity-backend | grep "slow"
```

#### 3. Monitor Resource Usage

```bash
# Docker stats
docker stats identity-backend identity-postgres

# CPU/Memory usage
```

**Common Solutions**:

```sql
-- Add missing indexes
CREATE INDEX idx_verification_events_agent_id
  ON verification_events(agent_id);

-- Vacuum database
VACUUM ANALYZE;
```

---

### ⚠️ High Memory Usage

**Symptoms**:
- Backend using > 1GB RAM
- OOM (Out of Memory) errors

**Diagnosis**:

```bash
# Check memory usage
docker stats identity-backend

# Check Go memory profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof -http=:8081 heap.prof
```

**Solutions**:

```bash
# Increase Docker memory limit
docker update --memory 2g identity-backend

# Tune Go GC
export GOGC=50  # More aggressive garbage collection
```

---

## Network & Connectivity

### ❌ Cannot Connect to Backend

**Symptoms**:
```
Error: Connection refused (localhost:8080)
```

**Troubleshooting**:

#### 1. Backend Not Running

```bash
docker ps | grep identity-backend

# If not running:
docker compose up -d identity-backend
```

#### 2. Port Already in Use

```bash
# Check what's using port 8080
sudo lsof -i :8080

# Kill the process
kill -9 <PID>

# Restart backend
docker compose restart identity-backend
```

#### 3. Docker Networking

```bash
# Inspect network
docker network inspect agent-identity-management_default

# Recreate network
docker compose down
docker compose up -d
```

---

### ❌ Database Connection Fails

**Symptoms**:
```
Error: Failed to connect to PostgreSQL
```

**Troubleshooting**:

#### 1. Check PostgreSQL Running

```bash
docker ps | grep identity-postgres

# Check logs
docker logs identity-postgres
```

#### 2. Verify Connection String

```bash
# Backend .env
cat apps/backend/.env | grep DATABASE_URL

# Expected format:
# postgresql://user:password@localhost:5432/identity
```

#### 3. Test Connection Manually

```bash
# Using psql
PGPASSWORD=postgres psql -h localhost -U postgres -d identity

# Or using Docker
docker exec -it identity-postgres psql -U postgres -d identity
```

---

## Common Error Messages

### "CORS policy: No 'Access-Control-Allow-Origin' header"

**Cause**: Frontend running on different origin than backend expects

**Fix**:
```go
// apps/backend/internal/api/middleware/cors.go
AllowOrigins: []string{
    "http://localhost:3000",  // Add your frontend URL
    "https://yourdomain.com",
}
```

---

### "jwt: signature is invalid"

**Cause**: JWT secret mismatch between services

**Fix**:
```bash
# Ensure same JWT_SECRET in all services
cat apps/backend/.env | grep JWT_SECRET

# Generate new secret if needed
openssl rand -base64 32
```

---

### "database is locked"

**Cause**: SQLite in production (not supported)

**Fix**: Use PostgreSQL for production:
```bash
# Migration script
./scripts/migrate-to-postgres.sh
```

---

### "redis connection timeout"

**Cause**: Redis not running or unreachable

**Fix**:
```bash
# Check Redis
docker ps | grep redis

# Start Redis
docker compose up -d redis

# Test connection
redis-cli ping  # Should return: PONG
```

---

## Diagnostic Commands

### Quick Health Check

```bash
#!/bin/bash
echo "=== AIM Platform Health Check ==="

# Backend
echo -n "Backend (8080): "
curl -s http://localhost:8080/health | jq -r '.status'

# Frontend
echo -n "Frontend (3000): "
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000

# PostgreSQL
echo -n "PostgreSQL (5432): "
docker exec identity-postgres pg_isready

# Redis
echo -n "Redis (6379): "
docker exec identity-redis redis-cli ping

echo "=== Docker Containers ==="
docker ps --format "table {{.Names}}\t{{.Status}}"
```

---

### Collect Debug Information

```bash
#!/bin/bash
# Save this as debug-info.sh
echo "Collecting AIM debug information..."

mkdir -p debug-output

# System info
docker version > debug-output/docker-version.txt
docker compose version > debug-output/compose-version.txt

# Container status
docker ps -a > debug-output/containers.txt
docker stats --no-stream > debug-output/container-stats.txt

# Logs (last 500 lines)
docker logs --tail=500 identity-backend > debug-output/backend.log 2>&1
docker logs --tail=500 identity-frontend > debug-output/frontend.log 2>&1
docker logs --tail=500 identity-postgres > debug-output/postgres.log 2>&1

# Network
docker network inspect agent-identity-management_default > debug-output/network.json

# Database
docker exec identity-postgres psql -U postgres -d identity \
  -c "SELECT version();" > debug-output/postgres-version.txt

echo "Debug info saved to debug-output/"
echo "Please attach debug-output/ when reporting issues"
```

---

## Getting Help

### Before Asking for Help

1. ✅ Check this troubleshooting guide
2. ✅ Review error messages carefully
3. ✅ Collect debug information (see above)
4. ✅ Search existing GitHub issues

### How to Report Issues

**Include**:
- Error messages (full stack trace)
- Steps to reproduce
- Environment details (OS, Docker version)
- Debug logs (from debug-info.sh)
- Configuration (sanitize secrets!)

**Template**:
```markdown
## Issue Description
[What went wrong]

## Steps to Reproduce
1. ...
2. ...
3. ...

## Expected Behavior
[What should happen]

## Actual Behavior
[What actually happens]

## Environment
- OS: macOS 14.0
- Docker: 24.0.0
- AIM Version: v1.0.0

## Logs
[Attach debug-output/ or paste relevant logs]
```

### Support Channels

**Enterprise Customers**:
- Submit support ticket at: support@yourdomain.com
- Include agent ID and organization ID
- Priority support with SLA

**Open Source Users**:
- GitHub Issues: https://github.com/opena2a-org/agent-identity-management/issues
- Community Discord: [link]
- Documentation: https://docs.aim.example.com

---

## Additional Resources

- [Token Rotation Guide](../security/token-rotation.md)
- [API Documentation](../api/README.md)
- [SDK Documentation](../sdk/README.md)
- [Deployment Guide](../deployment/README.md)
- [Security Best Practices](../security/best-practices.md)

---

**Last Updated**: October 18, 2025
**Version**: 1.0
**For**: AIM Platform v1.0+

**Found this guide helpful?** ⭐ Star our [GitHub repo](https://github.com/opena2a-org/agent-identity-management)
