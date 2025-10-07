# AIM Production Implementation Plan

**Owner**: Senior Production Engineer (Autonomous)
**Date**: October 6, 2025
**Goal**: Make AIM production-ready and fully tested

---

## Strategy Overview

### Phase 1: Real OAuth Configuration (30 min)
1. Check for existing Google OAuth credentials in gcloud
2. Create OAuth consent screen if needed
3. Configure OAuth redirect URIs
4. Update backend .env with real credentials
5. Restart backend with real config

### Phase 2: End-to-End Testing with Chrome DevTools (2 hours)
1. Test landing page renders
2. Test login flow (OAuth initiation)
3. Test OAuth callback handling
4. Test JWT token generation and storage
5. Test authenticated dashboard access
6. Test agent registration form
7. Test API key generation
8. Document all bugs found

### Phase 3: Fix Critical Bugs (4 hours)
1. Fix any authentication issues
2. Fix any CORS issues
3. Fix any JWT token handling issues
4. Fix any form submission issues
5. Retest all flows

### Phase 4: Create Seed Data & Documentation (2 hours)
1. Create seed data script (test org, users, agents)
2. Write QUICKSTART.md with screenshots
3. Write TROUBLESHOOTING.md
4. Update README.md with production setup

### Phase 5: Production Readiness Checklist (1 hour)
1. Verify all 62+ endpoints work
2. Verify database schema complete
3. Verify tests pass
4. Create deployment checklist
5. Final production readiness report

**Total Estimated Time**: 9.5 hours

---

## Detailed Execution Plan

### Step 1: Check Existing OAuth Credentials
```bash
# Check if gcloud is configured
gcloud config list

# List existing OAuth credentials
gcloud projects list
gcloud config get-value project

# Check for existing OAuth clients
gcloud services enable iamcredentials.googleapis.com
gcloud alpha iap oauth-clients list 2>/dev/null || echo "No existing clients"
```

### Step 2: Create/Configure Google OAuth
```bash
# Option A: Use existing credentials from .env
# Check if there are already credentials stored

# Option B: Create new OAuth credentials
# - Enable Google OAuth API
# - Create OAuth 2.0 Client ID
# - Add redirect URIs:
#   - http://localhost:8080/api/v1/auth/callback/google
#   - http://localhost:3000/auth/callback

# Option C: Use gcloud to create OAuth client
```

### Step 3: Update Backend Configuration
- Update apps/backend/.env with REAL credentials
- Ensure JWT_SECRET is cryptographically secure
- Verify CORS_ORIGINS matches frontend

### Step 4: Comprehensive Chrome DevTools Testing

#### Test Suite 1: Landing Page
```
1. Navigate to http://localhost:3000
2. Take screenshot
3. Verify page loads
4. Check for console errors
5. Verify "Sign In" button exists
```

#### Test Suite 2: OAuth Flow
```
1. Click "Sign In" button
2. Verify redirect to /login
3. Click "Continue with Google"
4. Verify redirect to Google OAuth
5. Complete OAuth (sign in with Google)
6. Verify callback to /auth/callback
7. Verify JWT token stored as 'aim_token'
8. Verify redirect to /dashboard
```

#### Test Suite 3: Dashboard
```
1. Verify dashboard renders
2. Check for user info display
3. Verify navigation menu
4. Check for stats/metrics
5. Verify no console errors
```

#### Test Suite 4: Agent Registration
```
1. Navigate to /dashboard/agents/new
2. Fill out form (name, type, description)
3. Submit form
4. Verify API call to POST /api/v1/agents
5. Verify success message
6. Verify redirect to agent list
7. Verify new agent appears in list
```

#### Test Suite 5: API Key Generation
```
1. Navigate to /dashboard/api-keys
2. Click "Generate API Key"
3. Fill out form (name, expiration)
4. Submit form
5. Verify API call to POST /api/v1/api-keys
6. Verify API key displayed (only once)
7. Copy API key
8. Verify key appears in list (hashed)
```

### Step 5: Bug Documentation & Fixes
For each bug found:
- Document exact reproduction steps
- Identify root cause
- Implement fix
- Retest to verify fix
- Mark as resolved

### Step 6: Create Seed Data
```sql
-- Seed data for testing
INSERT INTO organizations (id, name, slug, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'Test Organization', 'test-org', NOW(), NOW());

INSERT INTO users (id, organization_id, email, display_name, role, created_at, updated_at)
VALUES
  ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   'admin@aim.test', 'Test Admin', 'admin', NOW(), NOW()),
  ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111',
   'member@aim.test', 'Test Member', 'member', NOW(), NOW());

INSERT INTO agents (id, organization_id, name, agent_type, is_verified, created_at, updated_at)
VALUES
  ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111',
   'Test AI Agent', 'ai_agent', true, NOW(), NOW());
```

### Step 7: Documentation
- QUICKSTART.md (with screenshots from Chrome DevTools)
- API_EXAMPLES.md (curl examples for all endpoints)
- TROUBLESHOOTING.md (common issues and solutions)
- DEPLOYMENT.md (production deployment steps)

---

## Success Criteria

### Authentication ✅
- [ ] User can click "Sign In"
- [ ] User redirected to Google OAuth
- [ ] User can authorize AIM
- [ ] JWT token generated and stored
- [ ] User redirected to dashboard
- [ ] Dashboard shows user info
- [ ] User can logout

### Agent Management ✅
- [ ] User can view agent list
- [ ] User can create new agent
- [ ] Agent appears in list immediately
- [ ] User can view agent details
- [ ] User can update agent
- [ ] User can delete agent (with confirmation)
- [ ] Trust score calculated automatically

### API Key Management ✅
- [ ] User can generate API key
- [ ] API key displayed once (security)
- [ ] API key appears in list (hashed)
- [ ] User can revoke API key
- [ ] Revoked keys cannot authenticate

### Security ✅
- [ ] All API calls include JWT token
- [ ] Unauthorized requests return 401
- [ ] RBAC enforced (admin vs member)
- [ ] Multi-tenancy enforced (org isolation)
- [ ] API keys hashed with SHA-256
- [ ] No secrets in frontend code

### Performance ✅
- [ ] Landing page loads < 2s
- [ ] Dashboard loads < 1s
- [ ] API responses < 100ms (p95)
- [ ] No memory leaks
- [ ] No console errors

---

## Timeline

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| OAuth Setup | 30 min | Real Google OAuth working |
| Chrome DevTools Testing | 2 hours | All flows tested, bugs documented |
| Bug Fixes | 4 hours | All critical bugs fixed |
| Seed Data & Docs | 2 hours | Seed data script, QUICKSTART.md |
| Final Verification | 1 hour | Production readiness confirmed |
| **TOTAL** | **9.5 hours** | **Production-ready AIM** |

---

## Execution Log

### 2025-10-06 14:30 - Starting Implementation
- Created implementation plan
- Starting Phase 1: OAuth configuration
