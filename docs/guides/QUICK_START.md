# Quick Start Guide - Agent Identity Management

**For impatient developers who want to get up and running in 5 minutes** ⚡

---

## Prerequisites

- Docker Desktop running ✅ (you have this)
- Go 1.23+ installed
- Node.js 18+ installed
- 15 minutes of your time

---

## Step 1: Configure OAuth (3 minutes)

### Google OAuth (Required)
1. Go to https://console.cloud.google.com/apis/credentials
2. Create OAuth 2.0 Client ID
3. Set authorized redirect URI: `http://localhost:8080/api/v1/auth/callback/google`
4. Copy Client ID and Client Secret

### Update .env
```bash
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Edit .env file
nano .env

# Add these lines:
GOOGLE_CLIENT_ID=<paste-your-client-id>
GOOGLE_CLIENT_SECRET=<paste-your-client-secret>

# Save and exit (Ctrl+X, Y, Enter)
```

---

## Step 2: Database Setup (2 minutes)

```bash
# Docker services are already running! ✅
# PostgreSQL: localhost:5432
# Redis: localhost:6379

# Run migrations
cd /Users/decimai/workspace/agent-identity-management/apps/backend
go run cmd/migrate/main.go up

# Expected output:
# ✅ Applied migration: 001_initial_schema
# ✅ Applied migration: 002_add_indexes
# ✅ All migrations complete
```

---

## Step 3: Start Backend (1 minute)

```bash
# Terminal 1
cd /Users/decimai/workspace/agent-identity-management/apps/backend
go run cmd/server/main.go

# Expected output:
# ✅ Server starting on :8080
# ✅ Database connected
# ✅ Redis connected
# ✅ Routes registered
```

**Verify**: Open http://localhost:8080/api/v1/health
- Should return: `{"status":"healthy","timestamp":"..."}`

---

## Step 4: Start Frontend (1 minute)

```bash
# Terminal 2
cd /Users/decimai/workspace/agent-identity-management/apps/web

# Install dependencies (first time only)
npm install

# Start dev server
npm run dev

# Expected output:
# ✅ Ready on http://localhost:3000
```

**Verify**: Open http://localhost:3000
- Should see landing page with "Sign in with Google" button

---

## Step 5: Test It! (2 minutes)

### Test 1: Health Check
```bash
curl http://localhost:8080/api/v1/health
```
**Expected**: `{"status":"healthy","timestamp":"..."}`

### Test 2: OAuth Flow
1. Open http://localhost:3000
2. Click "Sign in with Google"
3. Should redirect to Google OAuth
4. After login, should redirect to Dashboard

### Test 3: API Protection
```bash
curl http://localhost:8080/api/v1/agents
```
**Expected**: `401 Unauthorized` (because no auth token)

---

## Common Issues

### Issue: Backend won't start
**Solution**:
```bash
# Check if port 8080 is in use
lsof -i :8080

# Kill process if needed
kill -9 <PID>

# Restart backend
go run cmd/server/main.go
```

### Issue: Frontend won't start
**Solution**:
```bash
# Check if port 3000 is in use
lsof -i :3000

# Kill process if needed
kill -9 <PID>

# Clear cache and restart
rm -rf .next
npm run dev
```

### Issue: Database connection failed
**Solution**:
```bash
# Check PostgreSQL container
docker ps | grep postgres

# Restart if needed
docker compose restart postgres

# Wait 10 seconds
sleep 10

# Try migrations again
go run cmd/migrate/main.go up
```

### Issue: OAuth not working
**Check**:
1. Client ID and Secret correct in `.env`
2. Redirect URI matches exactly: `http://localhost:8080/api/v1/auth/callback/google`
3. OAuth consent screen configured in Google Cloud Console

---

## What's Running?

```
PostgreSQL:  localhost:5432  (aim-postgres)
Redis:       localhost:6379  (aim-redis)
Backend:     localhost:8080  (Go + Fiber)
Frontend:    localhost:3000  (Next.js)
```

---

## Next Steps

### Test the Platform
1. ✅ Sign in with Google OAuth
2. ✅ Create a test agent
3. ✅ Generate an API key
4. ✅ Check trust score
5. ✅ View audit logs (if admin)

### Run Tests
```bash
# Backend integration tests
cd apps/backend
go test ./tests/integration/... -v

# Frontend E2E tests
cd apps/web
npm run test:e2e
```

### Read Documentation
- `README.md` - Project overview
- `SETUP_GUIDE.md` - Detailed setup
- `API_REFERENCE.md` - API docs
- `MANUAL_TESTING_GUIDE.md` - Testing procedures

---

## Production Deployment

Ready for production? See:
- `DEPLOYMENT_CHECKLIST.md` - Complete deployment guide
- `PRODUCTION_READINESS.md` - Production readiness report

---

## Getting Help

- **Documentation**: All files in repository
- **Email**: info@opena2a.org
- **GitHub Issues**: (after public launch)

---

**That's it!** You now have Agent Identity Management running locally. 🎉

Total time: **~10 minutes** ⚡
