# AIM MVP Changes - October 21, 2025

## 📊 Summary of Changes

**Before**: 95 backend endpoints, 92% frontend UI coverage
**After**: 70 backend endpoints, 100% frontend UI coverage
**Removed**: 25 endpoints (premium tier + deferred features)
**Added**: 2 new UI tabs (Violations + Key Vault)

---

## ✅ What We're Keeping (70 Endpoints)

### Core Agent Management (12 endpoints)
- POST/GET/PUT/DELETE `/agents` - Full CRUD
- POST `/agents/:id/verify` - Manual verification
- POST `/agents/:id/suspend` - Suspend agent
- POST `/agents/:id/reactivate` - Reactivate agent
- POST `/agents/:id/rotate-credentials` - Rotate keys
- GET `/agents/:id/sdk` - Download SDK
- GET `/agents/:id/credentials` - View credentials
- GET `/agents/:id/audit-logs` - Audit trail

### MCP Server Management (11 endpoints)
- POST/GET/PUT/DELETE `/mcp-servers` - Full CRUD
- POST `/mcp-servers/:id/verify` - Verify MCP
- POST `/mcp-servers/:id/keys` - Add verification keys
- GET `/mcp-servers/:id/verification-status` - Check status
- GET `/mcp-servers/:id/capabilities` - List capabilities
- GET `/mcp-servers/:id/agents` - Connected agents
- GET `/mcp-servers/:id/verification-events` - Event history

### Agent-MCP Relationships (5 endpoints)
- GET `/agents/:id/mcp-servers` - List connected MCPs
- PUT `/agents/:id/mcp-servers` - Update connections
- DELETE `/agents/:id/mcp-servers/:mcp_id` - Remove single MCP
- POST `/agents/:id/mcp-servers/detect` - Auto-detect MCPs
- GET `/agents/:id/capabilities` - Agent capabilities

### Trust Scoring (6 endpoints)
- GET `/trust-score/agents/:id` - Current score
- GET `/trust-score/agents/:id/history` - Score history
- POST `/trust-score/calculate/:id` - Recalculate
- GET `/agents/:id/trust-score` - Agent trust score
- GET `/agents/:id/trust-score/history` - Trust history
- POST `/agents/:id/trust-score/recalculate` - Force recalc

### Security Monitoring (Basic) (7 endpoints)
- GET `/security/threats` - Threat list
- GET `/security/anomalies` - Anomaly detection
- GET `/security/metrics` - Security metrics
- GET `/admin/alerts` - Alert management
- GET `/admin/alerts/unacknowledged/count` - Alert count
- POST `/admin/alerts/:id/acknowledge` - Acknowledge alert
- POST `/admin/alerts/:id/resolve` - Resolve alert

### Monitoring & Events (9 endpoints)
- GET `/verification-events/` - All events
- GET `/verification-events/recent` - Recent events
- GET `/verification-events/statistics` - Event stats
- GET `/verification-events/stats` - Stats summary
- GET `/verification-events/agent/:id` - Agent events
- GET `/verification-events/mcp/:id` - MCP events
- POST `/verifications/` - Create verification
- GET `/verifications/:id` - Get verification
- POST `/verifications/:id/result` - Submit result

### User Management (13 endpoints)
- POST `/public/register` - User registration
- POST `/public/login` - User login
- GET `/auth/me` - Current user
- POST `/auth/change-password` - Change password
- POST `/auth/refresh` - Refresh token
- GET `/admin/users` - List users
- GET `/admin/users/pending` - Pending approvals
- POST `/admin/users/:id/approve` - Approve user
- POST `/admin/users/:id/reject` - Reject user
- PUT `/admin/users/:id/role` - Change role
- POST `/admin/users/:id/deactivate` - Deactivate
- POST `/admin/users/:id/activate` - Activate
- DELETE `/admin/users/:id` - Delete user

### ⭐ NEW: Security Features (2 endpoints - UI to be built)
- ✅ GET `/agents/:id/violations` - Capability violations (NEW UI TAB)
- ✅ GET `/agents/:id/key-vault` - Key vault info (NEW UI TAB)

### Other Core Features (5 endpoints)
- GET `/capabilities` - List capabilities
- POST `/agents/:id/capabilities` - Grant capability
- DELETE `/agents/:id/capabilities/:id` - Revoke capability
- GET `/admin/organization/settings` - Org settings
- PUT `/admin/organization/settings` - Update settings

---

## ❌ What We're Removing (25 Endpoints)

### Analytics & Reporting (Moved to Premium) - 5 endpoints
```
❌ GET  /analytics/reports/generate       → Premium Analytics
❌ GET  /analytics/reports                → Premium Analytics
❌ GET  /analytics/reports/:id            → Premium Analytics
❌ GET  /analytics/reports/:id/download   → Premium Analytics
❌ GET  /analytics/usage/advanced         → Premium Analytics
```

### Compliance Advanced (Moved to Premium) - 5 endpoints
```
❌ GET  /compliance/audit-log/export              → Premium Compliance
❌ POST /compliance/reports/generate              → Premium Compliance
❌ GET  /compliance/reports                       → Premium Compliance
❌ GET  /compliance/reports/:id                   → Premium Compliance
❌ POST /compliance/access-reviews/start          → Premium Compliance
❌ GET  /compliance/access-reviews/pending        → Premium Compliance
```

### Security Advanced (Moved to Premium) - 6 endpoints
```
❌ GET  /security/incidents                       → Premium Security
❌ POST /security/incidents/:id/resolve           → Premium Security
❌ GET  /security/scans                           → Premium Security
❌ POST /security/scans/run                       → Premium Security
❌ GET  /security/vulnerabilities                 → Premium Security
❌ POST /security/vulnerabilities/:id/remediate   → Premium Security
```

### Trust Score Analytics (Deferred to v1.1) - 1 endpoint
```
❌ GET  /trust-score/trends                       → Deferred to v1.1
```

### Agent API Keys (Moved to Premium Secrets) - 2 endpoints
```
❌ GET  /agents/:id/api-keys                      → Premium Secrets Vault
❌ POST /agents/:id/api-keys                      → Premium Secrets Vault
```

### MCP Audit Logs (Duplicate) - 1 endpoint
```
❌ GET  /mcp-servers/:id/audit-logs               → Use /admin/audit-logs with filter
```

### Agent-MCP Bulk Operations (Deferred to v1.1) - 1 endpoint
```
❌ DELETE /agents/:id/mcp-servers/bulk            → Deferred to v1.1
```

### Capability Requests (Deferred) - 1 endpoint
```
❌ POST /capability-requests/                     → Deferred to v1.1
```

### Drift Approval (Covered by alerts) - 1 endpoint
```
❌ POST /admin/alerts/:id/approve-drift           → Use acknowledge instead
```

### Webhook Testing (Low priority) - 1 endpoint
```
❌ POST /webhooks/:id/test                        → Deferred to v1.1
```

### SDK Auto-Detection (Backend only) - 1 endpoint
```
❌ POST /detection/agents/:id/capabilities/report → SDK internal use
```

---

## 🆕 New UI Components to Build

### 1. Violations Tab (Agent Detail Page)
**Endpoint**: `GET /agents/:id/violations`
**Location**: `apps/web/components/agent/violations-tab.tsx`
**Effort**: 3-4 hours

**Features**:
- Table of capability violations
- Severity badges (Critical, High, Medium, Low)
- Filter by date range, severity
- Pagination (10 per page)
- Export to CSV
- Auto-refresh every 30 seconds

**Sample Data**:
```json
{
  "violations": [
    {
      "id": "v-123",
      "attempted_capability": "database.write",
      "severity": "high",
      "trust_score_impact": -5,
      "is_blocked": true,
      "created_at": "2025-10-21T14:30:00Z"
    }
  ],
  "total": 15
}
```

### 2. Key Vault Tab (Agent Detail Page)
**Endpoint**: `GET /agents/:id/key-vault`
**Location**: `apps/web/components/agent/key-vault-tab.tsx`
**Effort**: 2-3 hours

**Features**:
- Display public key (with copy button)
- Show expiration date with countdown timer
- Key algorithm (Ed25519)
- Rotation history (count)
- Last rotated timestamp
- **Premium upsell banner** for managed secrets

**Sample Data**:
```json
{
  "agent_id": "a-123",
  "public_key": "MCowBQYDK2VwAyEA...",
  "key_algorithm": "Ed25519",
  "key_created_at": "2025-01-15T10:30:00Z",
  "key_expires_at": "2026-01-15T10:30:00Z",
  "rotation_count": 3,
  "has_previous_public_key": true
}
```

---

## 🎯 Why These Changes?

### Clear Product Tiers
**Community Edition (Free)**:
- Complete core features (70 endpoints)
- Agent + MCP management
- Basic security monitoring
- Trust scoring
- Compliance basics

**Premium Edition ($199-499/mo)**:
- Everything in Community
- Advanced analytics & reporting
- Advanced compliance (audit exports, reviews)
- Security incident management
- Secrets management vault
- Agent-scoped API keys

### Revenue Protection
**Problem**: If free tier has agent-scoped API keys, enterprises won't pay for premium secrets management.

**Solution**: Remove Agent API Keys from free tier → Move to premium secrets vault

**Key Vault (free) vs Secrets Vault (premium)**:
- **Free**: Shows AIM-generated Ed25519 keypair info only
- **Premium**: Stores/rotates third-party secrets (Stripe, AWS, OpenAI keys)

**No overlap** → Clear premium value proposition

---

## 📋 Implementation Checklist

### Phase 1: Backend Cleanup (4-6 hours)
- [ ] Remove 25 endpoint handlers from Go code
- [ ] Update `main.go` route definitions
- [ ] Delete handler methods from:
  - `compliance_handler.go` (6 methods)
  - `security_handler.go` (6 methods)
  - `analytics_handler.go` (5 methods)
  - `agent_security_endpoints.go` (2 methods - API keys only, keep key vault)
  - `agent_mcp_handler.go` (1 method - bulk delete)
- [ ] Run tests: `go test ./...`

### Phase 2: Frontend Cleanup (2-3 hours)
- [ ] Remove ~200 lines from `lib/api.ts`
- [ ] Delete methods for removed endpoints
- [ ] Add premium upsell banners to compliance/security pages
- [ ] Update TypeScript types

### Phase 3: Build New UI (5-7 hours)
- [ ] Create `ViolationsTab` component (3-4 hours)
- [ ] Create `KeyVaultTab` component (2-3 hours)
- [ ] Update agent detail page layout
- [ ] Add tab navigation
- [ ] Test with Chrome DevTools MCP

### Phase 4: Documentation (2-3 hours)
- [ ] Update README with new endpoint count
- [ ] Update OpenAPI/Swagger docs
- [ ] Create CHANGELOG entry
- [ ] Update API documentation site

### Phase 5: Azure Deployment (3-4 hours)
- [ ] Build Docker images (linux/amd64)
- [ ] Push to ACR: `aimprodacr1760993976.azurecr.io`
- [ ] Update container apps
- [ ] Run smoke tests
- [ ] Notify internal team

**Total Effort**: 16-23 hours (2-3 days)

---

## 🚀 Deployment Strategy

### Pre-Deployment
1. Create database backup
2. Tag current version: `git tag v0.9.0`
3. Build images with version tags
4. Test in staging first (if available)

### Deployment Window
**Recommended**: Weekend (low traffic)

### Rollback Plan
If issues occur:
1. Revert container images to previous version
2. Restore database from backup
3. Notify team

### Post-Deployment
1. Monitor logs for errors
2. Check endpoint response times
3. Verify new UI tabs work
4. Send release notes to internal team

---

**Created**: October 21, 2025
**Status**: Ready for Implementation
**Next Step**: Begin Phase 1 (Backend Cleanup)
