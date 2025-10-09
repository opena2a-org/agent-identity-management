# Backend Features Without UI Controls

**Generated**: October 7, 2025
**Purpose**: Audit of backend API endpoints and services lacking corresponding frontend UI

This document tracks features that have complete backend implementation but are missing UI controls, making them untestable from the frontend.

---

## 🚨 Critical Priority (Block Core Workflows)

### 1. User Suspension/Activation Controls
**Backend Implementation**:
- Service: `AdminService.SuspendUser()` (line 102-118)
- Service: `AdminService.ActivateUser()` (line 121-143)
- Endpoint: None exposed yet

**Missing UI**:
- No "Suspend" button on active users in `/dashboard/admin/users`
- No "Activate" button on suspended users
- No suspension reason input field
- No suspension history display

**Impact**: Cannot test user suspension workflow, which is a critical admin feature

**Priority**: **HIGH**

---

### 2. User Deactivation Endpoint
**Backend Implementation**:
- Service: `AdminService.DeactivateUser()` (line 146-158)
- Route: `DELETE /api/v1/admin/users/:id` (line 570)

**Missing UI**:
- No "Deactivate" button on user cards in `/dashboard/admin/users`
- No confirmation dialog for permanent deactivation
- No deactivation reason field

**Impact**: Cannot permanently deactivate users (different from suspension)

**Priority**: **HIGH**

---

## 📊 Compliance & Reporting (Enterprise Features)

### 3. Compliance Framework Management
**Backend Implementation**:
- Endpoint: `GET /api/v1/compliance/frameworks` (line 600)
- Endpoint: `GET /api/v1/compliance/reports/:framework` (line 601)
- Endpoint: `POST /api/v1/compliance/scan/:framework` (line 602)

**Missing UI**:
- No compliance dashboard page
- No framework selector (SOC 2, HIPAA, GDPR)
- No compliance report viewer
- No scan initiation button

**Impact**: Cannot demonstrate enterprise compliance features

**Priority**: **MEDIUM**

---

### 4. Compliance Violation Management
**Backend Implementation**:
- Endpoint: `GET /api/v1/compliance/violations` (line 603)
- Endpoint: `POST /api/v1/compliance/remediate/:violation_id` (line 604)

**Missing UI**:
- No violations list view
- No violation severity indicators
- No remediation workflow UI
- No violation history

**Impact**: Cannot track or fix compliance issues

**Priority**: **MEDIUM**

---

### 5. Access Review Interface
**Backend Implementation**:
- Endpoint: `GET /api/v1/compliance/access-review` (line 596)

**Missing UI**:
- No access review dashboard
- No periodic review workflow
- No approval/revocation controls

**Impact**: Cannot perform quarterly access reviews

**Priority**: **MEDIUM**

---

### 6. Data Retention Policy View
**Backend Implementation**:
- Endpoint: `GET /api/v1/compliance/data-retention` (line 597)

**Missing UI**:
- No data retention settings page
- No retention period configuration
- No automated cleanup status

**Impact**: Cannot configure or view data retention policies

**Priority**: **LOW**

---

### 7. Audit Log Export
**Backend Implementation**:
- Endpoint: `GET /api/v1/compliance/audit-log/export` (line 595)

**Missing UI**:
- Audit logs page exists at `/dashboard/admin/audit-logs` ✅
- But no "Export" button on that page ❌
- No export format selector (CSV, JSON, PDF)
- No date range filter for export

**Impact**: Cannot export audit logs for compliance reporting

**Priority**: **MEDIUM**

---

## 🔐 Security Features

### 8. Security Threat Dashboard
**Backend Implementation**:
- Endpoint: `GET /api/v1/security/threats` (line 626)
- Endpoint: `GET /api/v1/security/anomalies` (line 627)
- Endpoint: `GET /api/v1/security/metrics` (line 628)

**Missing UI**:
- Security page exists at `/dashboard/security` ✅
- But likely missing threat visualization
- No anomaly detection alerts
- No security metrics graphs

**Action Required**: Review existing `/dashboard/security` page to verify what's implemented

**Priority**: **HIGH**

---

### 9. Security Incident Management
**Backend Implementation**:
- Endpoint: `GET /api/v1/security/incidents` (line 630)
- Endpoint: `POST /api/v1/security/incidents/:id/resolve` (line 631)

**Missing UI**:
- No incidents list view
- No incident severity/status badges
- No "Resolve" button per incident
- No incident timeline

**Impact**: Cannot manage security incidents

**Priority**: **HIGH**

---

### 10. Security Scanning
**Backend Implementation**:
- Endpoint: `GET /api/v1/security/scan/:id` (line 629)

**Missing UI**:
- No "Run Security Scan" button on agent details
- No scan results viewer
- No vulnerability report display

**Impact**: Cannot manually trigger security scans

**Priority**: **MEDIUM**

---

## 🤖 MCP Server Management

### 11. MCP Public Key Management
**Backend Implementation**:
- Endpoint: `POST /api/v1/mcp-servers/:id/keys` (line 616)

**Missing UI**:
- MCP servers page exists at `/dashboard/mcp` ✅
- But no "Add Public Key" button
- No public key list display per server
- No key rotation workflow

**Action Required**: Review existing `/dashboard/mcp` page

**Priority**: **HIGH** (Core MCP feature)

---

### 12. MCP Verification Status
**Backend Implementation**:
- Endpoint: `GET /api/v1/mcp-servers/:id/verification-status` (line 617)

**Missing UI**:
- No real-time verification status display
- No verification attempt history
- No failure reason display

**Impact**: Cannot troubleshoot MCP verification failures

**Priority**: **MEDIUM**

---

### 13. MCP Runtime Action Verification
**Backend Implementation**:
- Endpoint: `POST /api/v1/mcp-servers/:id/verify-action` (line 619)

**Missing UI**:
- This is a programmatic API (called by MCP servers)
- May not need UI, but status should be visible

**Priority**: **LOW** (API-only feature)

---

## 🎯 Agent Management

### 14. Agent Runtime Action Verification
**Backend Implementation**:
- Endpoint: `POST /api/v1/agents/:id/verify-action` (line 539)
- Endpoint: `POST /api/v1/agents/:id/log-action/:audit_id` (line 540)

**Missing UI**:
- These are programmatic APIs (called by agents)
- Action verification history should be visible
- No action log viewer on agent details page

**Priority**: **MEDIUM**

---

### 15. Agent Verification
**Backend Implementation**:
- Endpoint: `POST /api/v1/agents/:id/verify` (line 537)

**Missing UI**:
- Agents page exists at `/dashboard/agents` ✅
- But no "Verify Agent" button on agent cards
- No verification badge display
- No re-verification option

**Action Required**: Review existing `/dashboard/agents` page

**Priority**: **HIGH**

---

## 📊 Analytics & Monitoring

### 16. Agent Activity Monitoring
**Backend Implementation**:
- Endpoint: `GET /api/v1/analytics/agents/activity` (line 641)

**Missing UI**:
- Monitoring page exists at `/dashboard/monitoring` ✅
- But likely missing agent activity graphs
- No activity heatmap
- No timeline view

**Action Required**: Review existing `/dashboard/monitoring` page

**Priority**: **MEDIUM**

---

### 17. Report Generation
**Backend Implementation**:
- Endpoint: `GET /api/v1/analytics/reports/generate` (line 640)

**Missing UI**:
- No report type selector
- No date range picker for reports
- No download button for generated reports

**Impact**: Cannot generate custom analytics reports

**Priority**: **LOW**

---

## 🔔 Webhook Management

### 18. Webhook Testing
**Backend Implementation**:
- Endpoint: `POST /api/v1/webhooks/:id/test` (line 651)

**Missing UI**:
- No "Test Webhook" button on webhook list
- No test result display
- No request/response viewer

**Impact**: Cannot verify webhook configuration works

**Priority**: **MEDIUM**

---

## 📈 Trust Score Features

### 19. Trust Score Calculation Trigger
**Backend Implementation**:
- Endpoint: `POST /api/v1/trust-score/calculate/:id` (line 553)

**Missing UI**:
- No "Recalculate Trust Score" button on agent details
- No manual calculation trigger

**Impact**: Cannot manually recalculate trust scores

**Priority**: **LOW** (Usually automatic)

---

### 20. Trust Score Trends
**Backend Implementation**:
- Endpoint: `GET /api/v1/trust-score/trends` (line 556)

**Missing UI**:
- Dashboard likely shows current scores
- But no trend graphs over time
- No comparison across agents

**Priority**: **MEDIUM**

---

## 📝 Verification Events (Real-time Monitoring)

### 21. Verification Event CRUD
**Backend Implementation**:
- Endpoint: `GET /api/v1/verification-events/` (line 657)
- Endpoint: `GET /api/v1/verification-events/recent` (line 658)
- Endpoint: `GET /api/v1/verification-events/statistics` (line 659)
- Endpoint: `DELETE /api/v1/verification-events/:id` (line 662)

**Missing UI**:
- No verification events page at all
- No real-time event stream
- No event statistics dashboard
- No event deletion controls

**Impact**: Cannot monitor real-time verification activity

**Priority**: **HIGH** (Core monitoring feature)

---

## 🔑 API Key Features

### 22. API Key Expiration Management
**Backend Implementation**:
- Database has expiration tracking
- Service: `APIKeyService` has expiration logic

**Missing UI**:
- API keys page exists at `/dashboard/api-keys` ✅
- But no expiration date display
- No "Extend Expiration" button
- No expiration warning indicators

**Action Required**: Review existing `/dashboard/api-keys` page

**Priority**: **MEDIUM**

---

## Summary Statistics

**Total Features Without UI**: 22
**Critical Priority**: 2
**High Priority**: 6
**Medium Priority**: 10
**Low Priority**: 4

**Existing Pages to Review**:
1. `/dashboard/security` - Verify threat/incident displays
2. `/dashboard/mcp` - Verify key management UI
3. `/dashboard/agents` - Verify verification controls
4. `/dashboard/monitoring` - Verify activity monitoring
5. `/dashboard/api-keys` - Verify expiration displays

---

## Recommended Implementation Order

### Phase 1: Critical Admin Controls (Week 1)
1. User suspension/activation buttons
2. User deactivation workflow
3. Security incident management
4. Verification events dashboard

### Phase 2: MCP & Agent Features (Week 2)
5. MCP public key management
6. Agent verification controls
7. Agent action log viewer
8. Webhook testing UI

### Phase 3: Compliance & Reporting (Week 3)
9. Compliance framework dashboard
10. Violation management
11. Audit log export button
12. Access review interface

### Phase 4: Analytics & Polish (Week 4)
13. Trust score trends graphs
14. Agent activity monitoring
15. Report generation UI
16. Data retention policy viewer

---

## Notes

- **Review Existing Pages**: Several pages exist but need verification of what features are actually implemented
- **API-Only Features**: Some features (like runtime action verification) are meant for programmatic access and may not need UI
- **Enterprise Focus**: Many missing features are enterprise-grade (compliance, advanced security) which aligns with investment-ready goals
- **Testing Impact**: Missing UI for core features (suspension, verification) blocks E2E testing workflows

---

**Next Steps**:
1. Review existing frontend pages to verify what's already implemented
2. Prioritize based on user feedback and demo requirements
3. Create GitHub issues for each missing feature
4. Assign to sprint backlog based on implementation order
