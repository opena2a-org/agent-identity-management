# AIM MVP Implementation Plan 🚀

**Date**: October 21, 2025
**Production URL**: https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io
**Status**: Active production deployment with internal users
**Goal**: Clean up endpoints for MVP launch while maintaining backward compatibility

---

## 📊 CURRENT STATE (Before Changes)

### Current Backend Endpoints
**Total**: ~95 endpoints (including premium features we want to remove)

**Breakdown by Category**:
- ✅ Agent Management: 22 endpoints
- ✅ MCP Server Management: 15 endpoints
- ✅ User Management: 14 endpoints
- ✅ Security Monitoring: 8 endpoints
- ✅ Trust Scoring: 6 endpoints
- ✅ Monitoring (Verification Events): 9 endpoints
- ✅ Analytics: 6 endpoints
- ⚠️ Compliance: 12 endpoints (9 to remove for premium)
- ✅ API Keys: 5 endpoints (organization-wide)
- ⚠️ Agent API Keys: 2 endpoints (to remove - conflicts with premium)
- ✅ SDK & Tokens: 5 endpoints
- ✅ Tags: 10 endpoints
- ✅ Capabilities: 7 endpoints
- ✅ Webhooks: 5 endpoints
- ✅ MCP Auto-Detection: 3 endpoints
- ✅ Security Policies: 6 endpoints
- ✅ Organization Settings: 2 endpoints
- ⚠️ Analytics Advanced: 2 endpoints (to remove for premium)
- ⚠️ Security Incidents: 2 endpoints (to remove for premium)
- ⚠️ Security Scanning: 1 endpoint (to remove for premium)

### Current Frontend UI Pages
**Total**: ~92% endpoint coverage (78 out of 85 MVP endpoints have UI)

**Missing UI**:
- ❌ Agent Key Vault tab
- ❌ Agent API Keys tab (will be removed)
- ❌ Agent Violations tab
- ❌ Trust Score Trends page (deferred to v1.1)
- ❌ Bulk Remove MCPs button (deferred to v1.1)

---

## 🎯 TARGET STATE (After Changes)

### Target Backend Endpoints (Community Edition MVP)
**Total**: **70 endpoints** (down from 95)

**Removed**: 25 endpoints (moved to premium tier or deleted)

**Breakdown by Category**:
- ✅ Agent Management: 22 endpoints (no change)
- ✅ MCP Server Management: 14 endpoints (-1: removed duplicate audit logs)
- ✅ User Management: 14 endpoints (no change)
- ✅ Security Monitoring: 8 endpoints (no change)
- ✅ Trust Scoring: 5 endpoints (-1: trends page deferred to v1.1)
- ✅ Monitoring (Verification Events): 9 endpoints (no change)
- ✅ Analytics: 4 endpoints (-2: removed advanced analytics for premium)
- ✅ Compliance (Basic): 3 endpoints (-9: removed advanced compliance for premium)
- ✅ API Keys (Organization-wide): 5 endpoints (no change)
- ✅ SDK & Tokens: 5 endpoints (no change)
- ✅ Tags: 10 endpoints (no change)
- ✅ Capabilities: 7 endpoints (no change)
- ✅ Webhooks: 5 endpoints (no change)
- ✅ MCP Auto-Detection: 3 endpoints (no change)
- ✅ Security Policies: 6 endpoints (no change)
- ✅ Organization Settings: 2 endpoints (no change)

**Removed Endpoints**: -25 endpoints
- ❌ Agent API Keys: 2 endpoints (conflicts with premium)
- ❌ Advanced Compliance: 9 endpoints (premium feature)
- ❌ Advanced Analytics: 2 endpoints (premium feature)
- ❌ Security Incidents: 2 endpoints (premium feature)
- ❌ Security Scanning: 1 endpoint (premium feature)
- ❌ Trust Score Trends: 1 endpoint (deferred to v1.1)
- ❌ MCP Audit Logs: 1 endpoint (duplicate)
- ❌ Bulk Remove MCPs: 1 endpoint (deferred to v1.1)

### Target Frontend UI Coverage
**Total**: **100% coverage** (70 out of 70 MVP endpoints will have UI)

**New UI to Build**: 2 components
- ✅ Agent Violations tab (3-4 hours)
- ✅ Agent Key Vault tab (2-3 hours)

**UI to Remove**: 0 components (we never built the removed endpoints)

---

## 🔧 IMPLEMENTATION CHECKLIST

### Phase 1: Backend Cleanup (Day 1) ⏱️ 4-6 hours

#### Step 1.1: Remove Premium Endpoints from Handlers
**Files to Modify**:

1. **Compliance Handler** (`apps/backend/internal/interfaces/http/handlers/compliance_handler.go`)
   ```go
   // ❌ REMOVE these methods:
   - GenerateComplianceReport()
   - ListComplianceReports()
   - RunComplianceCheck()
   - GetAccessReview()
   - ListAccessReviews()
   - GetDataRetentionPolicies()
   - ExportAuditLog()
   ```

2. **Analytics Handler** (`apps/backend/internal/interfaces/http/handlers/analytics_handler.go`)
   ```go
   // ❌ REMOVE these methods:
   - GenerateReport()
   - GetAgentActivity()
   ```

3. **Security Handler** (`apps/backend/internal/interfaces/http/handlers/security_handler.go`)
   ```go
   // ❌ REMOVE these methods:
   - GetIncidents()
   - ResolveIncident()
   - RunSecurityScan()
   ```

4. **Agent Handler** (`apps/backend/internal/interfaces/http/handlers/agent_security_endpoints.go`)
   ```go
   // ❌ REMOVE these methods:
   - GetAgentAPIKeys()
   - CreateAgentAPIKey()
   ```

5. **Trust Score Handler** (`apps/backend/internal/interfaces/http/handlers/trust_score_handler.go`)
   ```go
   // ❌ REMOVE this method:
   - GetTrustScoreTrends()
   ```

6. **MCP Handler** (`apps/backend/internal/interfaces/http/handlers/mcp_handler.go`)
   ```go
   // ❌ REMOVE this method:
   - GetMCPAuditLogs() // Duplicate of /admin/audit-logs with filter
   ```

7. **Agent Handler** (`apps/backend/internal/interfaces/http/handlers/agent_handler.go`)
   ```go
   // ❌ REMOVE this method:
   - BulkRemoveMCPServersFromAgent()
   ```

#### Step 1.2: Update Route Definitions
**File**: `apps/backend/cmd/server/main.go`

**Lines to DELETE**:
```go
// Compliance routes (line 884-900) - REMOVE 9 PREMIUM ENDPOINTS
❌ compliance.Get("/audit-log/export", h.Compliance.ExportAuditLog)
❌ compliance.Get("/audit-log/access-review", h.Compliance.GetAccessReviewFromAuditLog)
❌ compliance.Get("/audit-log/data-retention", h.Compliance.GetDataRetentionFromAuditLog)
❌ compliance.Get("/access-review", h.Compliance.GetAccessReview)
❌ compliance.Post("/check", h.Compliance.RunComplianceCheck)
❌ compliance.Post("/reports/generate", h.Compliance.GenerateComplianceReport)
❌ compliance.Get("/reports", h.Compliance.ListComplianceReports)
❌ compliance.Get("/access-reviews", h.Compliance.ListAccessReviews)
❌ compliance.Get("/data-retention", h.Compliance.GetDataRetentionPolicies)

// Analytics routes (line 933-942) - REMOVE 2 PREMIUM ENDPOINTS
❌ analytics.Get("/reports/generate", h.Analytics.GenerateReport)
❌ analytics.Get("/agents/activity", h.Analytics.GetAgentActivity)

// Security routes (line 920-930) - REMOVE 3 PREMIUM ENDPOINTS
❌ security.Get("/incidents", h.Security.GetIncidents)
❌ security.Post("/incidents/:id/resolve", h.Security.ResolveIncident)
❌ security.Get("/scan/:id", h.Security.RunSecurityScan)

// Trust score routes (line 823-828) - REMOVE 1 DEFERRED ENDPOINT
❌ trust.Get("/trends", h.TrustScore.GetTrustScoreTrends)

// Agent routes (line 776-811) - REMOVE 3 ENDPOINTS
❌ agents.Get("/:id/api-keys", h.Agent.GetAgentAPIKeys)
❌ agents.Post("/:id/api-keys", middleware.MemberMiddleware(), h.Agent.CreateAgentAPIKey)
❌ agents.Delete("/:id/mcp-servers/bulk", middleware.MemberMiddleware(), h.Agent.BulkRemoveMCPServersFromAgent)

// MCP routes (line 901-918) - REMOVE 1 DUPLICATE ENDPOINT
❌ mcpServers.Get("/:id/audit-logs", h.MCP.GetMCPAuditLogs)
```

**Total Lines to Delete**: ~25 route definitions

#### Step 1.3: Add Premium Feature Placeholders (Optional)
**For future premium tier**, add stubs that return 402 Payment Required:

```go
// Premium compliance endpoints (placeholder for future)
compliance.Get("/reports/generate", func(c fiber.Ctx) error {
    return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
        "error": "This feature requires AIM Premium",
        "feature": "Advanced Compliance Reporting",
        "upgrade_url": "https://opena2a.org/pricing",
    })
})
```

**Files to Create** (optional):
- `apps/backend/internal/interfaces/http/handlers/premium_placeholder_handler.go`

---

### Phase 2: Frontend Cleanup (Day 1) ⏱️ 2-3 hours

#### Step 2.1: Update API Client
**File**: `apps/web/lib/api.ts`

**Methods to REMOVE**:
```typescript
// ❌ DELETE these methods:
async generateComplianceReport(): Promise<void>
async getComplianceReports(): Promise<any[]>
async runComplianceCheck(): Promise<any>
async getAccessReviews(): Promise<any[]>
async getDataRetentionPolicies(): Promise<any>
async exportAuditLog(): Promise<Blob>

async generateAnalyticsReport(): Promise<void>
async getAgentActivity(): Promise<any>

async getSecurityIncidents(): Promise<any[]>
async resolveSecurityIncident(id: string): Promise<void>
async runSecurityScan(agentId: string): Promise<any>

async getTrustScoreTrends(): Promise<any>

async getAgentAPIKeys(agentId: string): Promise<any[]>
async createAgentAPIKey(agentId: string, data: any): Promise<any>

async bulkRemoveMCPServersFromAgent(agentId: string, mcpIds: string[]): Promise<void>

async getMCPAuditLogs(mcpId: string): Promise<any[]>
```

**Estimated deletions**: ~200 lines of code

#### Step 2.2: Add Premium Feature Placeholders (Optional)
**For pages that reference removed endpoints**, add "Upgrade to Premium" banners:

**Example - Compliance Page**:
```tsx
// apps/web/app/dashboard/admin/compliance/page.tsx

{/* Premium Feature Banner */}
<div className="bg-gradient-to-r from-purple-500 to-pink-500 p-6 rounded-lg text-white">
  <h3 className="text-lg font-semibold mb-2">
    💎 Advanced Compliance Reporting
  </h3>
  <p className="mb-4">
    Upgrade to AIM Premium for automated SOC 2, HIPAA, and GDPR compliance reports.
  </p>
  <button className="bg-white text-purple-600 px-4 py-2 rounded-lg font-semibold">
    Upgrade to Premium
  </button>
</div>
```

**Files to Update**:
- `apps/web/app/dashboard/admin/compliance/page.tsx`
- `apps/web/app/dashboard/security/page.tsx`

---

### Phase 3: Build New UI Components (Day 2-3) ⏱️ 5-7 hours

#### Step 3.1: Agent Violations Tab (3-4 hours)
**File to Create**: `apps/web/components/agent/violations-tab.tsx`

**What to Build**:
```tsx
import { useState, useEffect } from 'react';
import { api } from '@/lib/api';

export function ViolationsTab({ agentId }: { agentId: string }) {
  const [violations, setViolations] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchViolations();
  }, [agentId]);

  const fetchViolations = async () => {
    try {
      const response = await api.get(`/agents/${agentId}/violations`);
      setViolations(response.violations);
    } catch (error) {
      console.error('Failed to fetch violations:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <h3 className="text-lg font-semibold">Capability Violations</h3>

      {/* Violations table */}
      <div className="border rounded-lg">
        {violations.map((violation) => (
          <div key={violation.id} className="p-4 border-b">
            {/* Severity badge */}
            <SeverityBadge severity={violation.severity} />

            {/* Violation details */}
            <div className="mt-2">
              <p>Attempted: {violation.attempted_capability}</p>
              <p>When: {formatDateTime(violation.created_at)}</p>
              <p>Impact: {violation.trust_score_impact} points</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

**Files to Update**:
- Create: `apps/web/components/agent/violations-tab.tsx`
- Update: `apps/web/app/dashboard/agents/[id]/page.tsx` (add violations tab to agent detail page)

**API Method to Add**:
```typescript
// apps/web/lib/api.ts
async getAgentViolations(agentId: string, limit = 20, offset = 0): Promise<any> {
  const response = await this.request(`/agents/${agentId}/violations?limit=${limit}&offset=${offset}`);
  return response;
}
```

---

#### Step 3.2: Agent Key Vault Tab (2-3 hours)
**File to Create**: `apps/web/components/agent/key-vault-tab.tsx`

**What to Build**:
```tsx
import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import { Copy, Download } from 'lucide-react';

export function KeyVaultTab({ agentId }: { agentId: string }) {
  const [keyVault, setKeyVault] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchKeyVault();
  }, [agentId]);

  const fetchKeyVault = async () => {
    try {
      const response = await api.get(`/agents/${agentId}/key-vault`);
      setKeyVault(response);
    } catch (error) {
      console.error('Failed to fetch key vault:', error);
    } finally {
      setLoading(false);
    }
  };

  const copyPublicKey = () => {
    navigator.clipboard.writeText(keyVault.public_key);
  };

  return (
    <div className="space-y-6">
      <h3 className="text-lg font-semibold">🔐 Cryptographic Key Vault</h3>

      {/* Public Key */}
      <div>
        <label className="block text-sm font-medium mb-2">Public Key (Ed25519)</label>
        <div className="flex items-center gap-2">
          <code className="flex-1 p-3 bg-gray-100 rounded text-xs break-all">
            {keyVault.public_key}
          </code>
          <button onClick={copyPublicKey} className="p-2 hover:bg-gray-100 rounded">
            <Copy className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Key Details */}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-600">Created</label>
          <p>{formatDateTime(keyVault.key_created_at)}</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-600">Expires</label>
          <p>{formatDateTime(keyVault.key_expires_at)}</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-600">Algorithm</label>
          <p>{keyVault.key_algorithm}</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-600">Rotations</label>
          <p>{keyVault.rotation_count} times</p>
        </div>
      </div>

      {/* Premium Upsell */}
      <div className="bg-gradient-to-r from-purple-500 to-pink-500 p-6 rounded-lg text-white">
        <h4 className="text-lg font-semibold mb-2">
          💎 Upgrade to Premium Secrets Management
        </h4>
        <p className="mb-4">
          Store and auto-rotate third-party API keys (Stripe, OpenAI, AWS), database passwords, and OAuth tokens.
        </p>
        <button className="bg-white text-purple-600 px-4 py-2 rounded-lg font-semibold">
          Learn More
        </button>
      </div>
    </div>
  );
}
```

**Files to Update**:
- Create: `apps/web/components/agent/key-vault-tab.tsx`
- Update: `apps/web/app/dashboard/agents/[id]/page.tsx` (add key vault tab to agent detail page)

**API Method to Add**:
```typescript
// apps/web/lib/api.ts
async getAgentKeyVault(agentId: string): Promise<any> {
  const response = await this.request(`/agents/${agentId}/key-vault`);
  return response;
}
```

---

### Phase 4: Documentation Updates (Day 3) ⏱️ 2-3 hours

#### Step 4.1: Update OpenAPI/Swagger Docs
**File**: `apps/backend/docs/swagger.yaml` (if exists)

**Actions**:
- Remove deleted endpoint definitions
- Add deprecation notices for removed endpoints
- Update API count (95 → 70 endpoints)

#### Step 4.2: Update README Files
**Files to Update**:

1. **Main README** (`README.md`)
   ```markdown
   ## Features (Community Edition)

   - ✅ 70+ API endpoints for complete identity management
   - ✅ Agent & MCP server registration and verification
   - ✅ Real-time trust scoring and monitoring
   - ✅ Security threat detection and alerts
   - ✅ Capability-based access control
   - ✅ Comprehensive audit logging
   - ✅ Self-hosted with zero vendor lock-in

   ## Premium Features (Coming Soon)

   - 💎 Advanced compliance reporting (SOC 2, HIPAA, GDPR)
   - 💎 Centralized secrets management
   - 💎 Automated security scanning
   - 💎 Custom analytics and reporting
   ```

2. **API Documentation** (`docs/api/README.md`)
   - Update endpoint count
   - Remove premium endpoint examples
   - Add migration guide for removed endpoints

3. **Changelog** (`CHANGELOG.md`)
   ```markdown
   ## [1.0.0] - 2025-01-22

   ### Removed (Moved to Premium Tier)
   - Advanced compliance reporting endpoints (9 endpoints)
   - Advanced analytics endpoints (2 endpoints)
   - Security incident management (2 endpoints)
   - Automated security scanning (1 endpoint)
   - Agent-scoped API keys (2 endpoints)

   ### Added
   - Agent violations tab in UI
   - Agent key vault tab in UI
   - Premium feature upgrade banners

   ### Deferred to v1.1
   - Trust score trends page
   - Bulk remove MCPs functionality
   ```

---

### Phase 5: Azure Production Deployment (Day 4) ⏱️ 3-4 hours

#### Step 5.1: Pre-Deployment Checklist
**Before deploying to production**:

- [ ] All tests passing locally
- [ ] Backend compiles without errors
- [ ] Frontend builds successfully
- [ ] Database migrations tested (if any)
- [ ] Environment variables verified
- [ ] Backup current production database
- [ ] Create rollback plan

#### Step 5.2: Deployment Steps

**Production Environment**:
- **Frontend**: https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io
- **Backend**: https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io
- **Database**: aim-prod-db-1760993976.postgres.database.azure.com
- **Resource Group**: aim-production-rg
- **Registry**: aimprodacr1760993976.azurecr.io

**Step 1: Build and Push Docker Images**
```bash
# Navigate to project root
cd /Users/decimai/workspace/agent-identity-management

# Set Azure subscription (CRITICAL!)
az account set --subscription 1b1e58e7-97db-4b08-b3d9-ee8e7867bcb9

# Login to Azure Container Registry
az acr login --name aimprodacr1760993976

# Build backend (linux/amd64 for Azure!)
docker buildx build --platform linux/amd64 \
  -f apps/backend/Dockerfile \
  -t aimprodacr1760993976.azurecr.io/aim-backend:v1.0.0 \
  -t aimprodacr1760993976.azurecr.io/aim-backend:latest \
  --push \
  ./apps/backend

# Build frontend (linux/amd64 for Azure!)
docker buildx build --platform linux/amd64 \
  -f apps/web/Dockerfile \
  -t aimprodacr1760993976.azurecr.io/aim-frontend:v1.0.0 \
  -t aimprodacr1760993976.azurecr.io/aim-frontend:latest \
  --push \
  ./apps/web
```

**Step 2: Update Container Apps**
```bash
# Update backend container app
az containerapp update \
  --name aim-prod-backend \
  --resource-group aim-production-rg \
  --image aimprodacr1760993976.azurecr.io/aim-backend:v1.0.0

# Update frontend container app
az containerapp update \
  --name aim-prod-frontend \
  --resource-group aim-production-rg \
  --image aimprodacr1760993976.azurecr.io/aim-frontend:v1.0.0
```

**Step 3: Verify Deployment**
```bash
# Check backend health
curl https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/health

# Check frontend
curl https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io

# Check container app status
az containerapp show \
  --name aim-prod-backend \
  --resource-group aim-production-rg \
  --query "properties.provisioningState"
```

**Step 4: Smoke Test Production**
```bash
# Test authentication
curl https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/auth/me \
  -H "Authorization: Bearer $PROD_TOKEN"

# Test agent list
curl https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/agents \
  -H "Authorization: Bearer $PROD_TOKEN"

# Verify removed endpoints return 404
curl https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/compliance/reports/generate \
  -H "Authorization: Bearer $PROD_TOKEN"
# Expected: 404 Not Found
```

#### Step 5.3: Rollback Plan (If Deployment Fails)

**Rollback to Previous Version**:
```bash
# Find previous image tag
az acr repository show-tags \
  --name aimprodacr1760993976 \
  --repository aim-backend \
  --orderby time_desc \
  --top 5

# Rollback backend
az containerapp update \
  --name aim-prod-backend \
  --resource-group aim-production-rg \
  --image aimprodacr1760993976.azurecr.io/aim-backend:PREVIOUS_TAG

# Rollback frontend
az containerapp update \
  --name aim-prod-frontend \
  --resource-group aim-production-rg \
  --image aimprodacr1760993976.azurecr.io/aim-frontend:PREVIOUS_TAG
```

---

## 📊 SUMMARY OF CHANGES

### Backend Changes
| Category | Before | After | Change |
|----------|--------|-------|--------|
| **Total Endpoints** | 95 | 70 | -25 endpoints |
| **Core Features** | 85 | 70 | -15 premium endpoints |
| **Premium Features** | 10 | 0 | Moved to future premium tier |

### Frontend Changes
| Category | Before | After | Change |
|----------|--------|-------|--------|
| **UI Coverage** | 92% (78/85) | 100% (70/70) | +2 new tabs |
| **New Components** | 0 | 2 | +Violations tab, +Key Vault tab |
| **Removed Pages** | 0 | 0 | No pages removed |

### Endpoints Removed (25 total)
1. ❌ **Advanced Compliance** (9 endpoints) - Premium feature
2. ❌ **Advanced Analytics** (2 endpoints) - Premium feature
3. ❌ **Security Incidents** (2 endpoints) - Premium feature
4. ❌ **Security Scanning** (1 endpoint) - Premium feature
5. ❌ **Agent API Keys** (2 endpoints) - Conflicts with premium secrets vault
6. ❌ **Trust Score Trends** (1 endpoint) - Deferred to v1.1
7. ❌ **MCP Audit Logs** (1 endpoint) - Duplicate, use /admin/audit-logs
8. ❌ **Bulk Remove MCPs** (1 endpoint) - Deferred to v1.1

### New UI Components (2 total)
1. ✅ **Agent Violations Tab** - Security monitoring (3-4 hours)
2. ✅ **Agent Key Vault Tab** - Cryptographic key info (2-3 hours)

---

## ⏱️ TOTAL IMPLEMENTATION TIME

| Phase | Duration | Status |
|-------|----------|--------|
| **Phase 1**: Backend Cleanup | 4-6 hours | Not started |
| **Phase 2**: Frontend Cleanup | 2-3 hours | Not started |
| **Phase 3**: Build New UI | 5-7 hours | Not started |
| **Phase 4**: Documentation | 2-3 hours | Not started |
| **Phase 5**: Azure Deployment | 3-4 hours | Not started |
| **TOTAL** | **16-23 hours** | **2-3 days** |

---

## 🚨 RISKS & MITIGATION

### Risk 1: Internal Users Affected by Removed Endpoints
**Impact**: Internal team may have scripts calling removed endpoints
**Mitigation**:
- Check production logs for endpoint usage before removal
- Send email notification to internal team 48 hours before deployment
- Provide migration guide for removed endpoints

### Risk 2: Breaking Changes in Production
**Impact**: Production deployment fails, users cannot access AIM
**Mitigation**:
- Full backup of production database before deployment
- Rollback plan ready (previous Docker images tagged)
- Deploy during low-traffic window (weekend)

### Risk 3: Missing Dependencies in New UI Components
**Impact**: New violations/key vault tabs crash frontend
**Mitigation**:
- Test new tabs locally before deployment
- Add error boundaries around new components
- Graceful fallback if API returns error

---

## 📋 DEPLOYMENT CHECKLIST

### Pre-Deployment (48 hours before)
- [ ] Notify internal team of upcoming changes
- [ ] Backup production database
- [ ] Test all changes locally
- [ ] Review removed endpoint usage in production logs
- [ ] Create rollback plan document

### Deployment Day
- [ ] Set correct Azure subscription
- [ ] Build Docker images (linux/amd64)
- [ ] Push to Azure Container Registry
- [ ] Update backend container app
- [ ] Update frontend container app
- [ ] Verify health endpoints
- [ ] Smoke test critical flows
- [ ] Check for errors in logs

### Post-Deployment
- [ ] Monitor error rates for 24 hours
- [ ] Verify new UI tabs work correctly
- [ ] Confirm removed endpoints return 404
- [ ] Update documentation site
- [ ] Send completion email to internal team

---

## 🎯 SUCCESS CRITERIA

✅ **Backend**: 70 endpoints active, 25 endpoints removed
✅ **Frontend**: 100% UI coverage (all 70 endpoints have UI)
✅ **Production**: Zero downtime during deployment
✅ **Users**: Internal team can still access all core features
✅ **Documentation**: All docs updated with new endpoint count

---

**Generated**: October 21, 2025
**Status**: Ready for Implementation
**Next Step**: Phase 1 - Backend Cleanup (4-6 hours)
