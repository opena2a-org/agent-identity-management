# Alerts Page - Critical Security Feature

**URL**: `https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/dashboard/admin/alerts`

## ⚠️ DO NOT DELETE - This is a Core Security Feature

### Purpose
The Alerts page is the **primary interface** for viewing security violations detected by AIM's enforced security policies.

### How It Works

#### 1. Alert Generation (Backend)
When capability violation enforcement is triggered:

**File**: `apps/backend/internal/application/agent_service.go` (lines 432-448)
```go
alert := &domain.Alert{
    ID:             uuid.New(),
    OrganizationID: agent.OrganizationID,
    AlertType:      domain.AlertSecurityBreach,
    Severity:       domain.AlertSeverityHigh,
    Title:          "Capability Violation Detected: AgentName",
    Description:    "Agent 'X' attempted unauthorized action 'Y'...",
    ResourceType:   "agent",
    ResourceID:     agentID,
    IsAcknowledged: false,
    CreatedAt:      time.Now(),
}

s.alertRepo.Create(alert)
```

#### 2. Alert Storage
- Alerts are stored in the `alerts` table in PostgreSQL
- Fields include: type, severity, title, description, resource info, acknowledgment status

#### 3. Alert Retrieval (API)
**Endpoint**: `GET /api/v1/admin/alerts`
**Handler**: `apps/backend/internal/interfaces/http/handlers/admin_handler.go` (line 594)

Supports filtering by:
- Severity (low, medium, high, critical)
- Status (acknowledged, unacknowledged)
- Pagination (limit, offset)

#### 4. Alert Display (Frontend)
**File**: `apps/web/app/dashboard/admin/alerts/page.tsx`

Features:
- Real-time alert list with severity badges
- Filter by severity and acknowledgment status
- Acknowledge alerts (mark as reviewed)
- Alert details with timestamp and description
- Alert type icons for visual clarity

### Real-World Example

**Scenario**: Agent "EmailBot" attempts capability violation

```
1. Agent "EmailBot" has capabilities: ["email:read", "email:send"]
2. Agent tries to call: DELETE /api/emails/bulk
3. Required capability: "email:delete" (NOT in agent's list)

Backend Response:
✅ Capability violation policy enforced
✅ Action blocked (returns 403 Forbidden)
✅ Alert created with:
   - Title: "Capability Violation Detected: EmailBot"
   - Severity: High
   - Description: "Agent 'EmailBot' attempted unauthorized action 'email:delete'
                  which is not in its capability list (allowed: email:read, email:send).
                  This matches the attack pattern of CVE-2025-32711 (EchoLeak).
                  Security Policy 'Capability Violation Detection' enforcement: BLOCKED."

Admin Dashboard:
✅ Alert appears on /dashboard/admin/alerts
✅ Admin sees red "High Severity" badge
✅ Admin can acknowledge after investigation
✅ Alert includes audit ID for tracking
```

### Alert Types (Current)

| Type | Severity | Trigger | When Created |
|------|----------|---------|--------------|
| `security_breach` | High | Capability violation | Agent exceeds granted capabilities |

### Alert Types (Planned - Post-MVP)

| Type | Severity | Trigger |
|------|----------|---------|
| `trust_score_low` | Medium | Trust score < 70 |
| `trust_score_critical` | Critical | Trust score < 50 |
| `auth_failure` | Medium | 5+ failed auth attempts in 15 min |
| `unusual_activity` | Medium | API rate spike (>1000/hr) |
| `data_exfiltration` | High | Data transfer > 100MB/hr |

### Why This Page is Essential

1. **Security Monitoring**: Only way to see when agents attempt unauthorized actions
2. **Incident Response**: Admins can investigate and respond to threats
3. **Audit Trail**: Provides evidence for compliance (SOC 2, HIPAA)
4. **Proof of Enforcement**: Shows investors/customers that security policies actually work
5. **Attack Prevention**: Early warning system for compromised agents

### For MVP Demo

**Demo Script**:
1. Show alerts page (should have sample alerts if any agent violations occurred)
2. Explain: "When an agent tries to exceed its capabilities, it's immediately blocked AND we generate an alert"
3. Show severity levels and acknowledgment workflow
4. Emphasize: "This prevents EchoLeak-style attacks where agents access data they shouldn't"

### Related Endpoints

**Backend** (`apps/backend/internal/interfaces/http/handlers/admin_handler.go`):
- `GET /api/v1/admin/alerts` - List alerts (line 594)
- `POST /api/v1/admin/alerts/:id/acknowledge` - Acknowledge alert
- `POST /api/v1/admin/alerts/:id/approve-drift` - Approve config drift

**Frontend** (`apps/web/lib/api.ts`):
- `api.getAlerts(limit, offset)` - Fetch alerts
- `api.acknowledgeAlert(alertId)` - Mark as acknowledged
- `api.approveDrift(alertId)` - Approve configuration change

### Database Schema

**Table**: `alerts`
```sql
CREATE TABLE alerts (
    id UUID PRIMARY KEY,
    organization_id UUID REFERENCES organizations(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    resource_type VARCHAR(50),
    resource_id UUID,
    is_acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

**Conclusion**: The Alerts page is the **heart of AIM's security monitoring**. Without it, admins would have no visibility into security violations. This is a **must-have** for any serious security product, especially for investor demos and enterprise sales.

**DO NOT DELETE THIS PAGE** - It's essential for the MVP and production use.
