# 🔧 AIM Missing Features & Implementation Plan

**Last Updated**: October 23, 2025
**Status**: Planning Phase

## 📋 Overview

This document tracks all missing features discovered during the AIM platform review. These features are critical for achieving 100% feature parity with AIVF and making AIM investment-ready.

---

## 🚨 Priority 1: Critical Fixes

### 1. Fix Agent Verification Activity Chart
**Issue**: Chart shows "No Activity Data" despite having 13 agents
**Root Cause**: Auto-verified agents don't create entries in `verification_events` table
**Impact**: Dashboard looks broken to users

**Implementation**:
```go
// File: internal/application/agent_service.go
// Add after auto-verification logic:

if shouldAutoVerify {
    // Update agent status
    agent.Status = "verified"

    // ✅ CREATE VERIFICATION EVENT
    verificationEvent := &domain.VerificationEvent{
        OrganizationID: agent.OrganizationID,
        AgentID: &agent.ID,
        AgentName: &agent.Name,
        Protocol: "Ed25519",
        VerificationType: "auto_verification",
        Status: "success",
        Result: "verified",
        StartedAt: time.Now(),
        CompletedAt: &time.Now(),
        DurationMs: 0,
    }

    // Save verification event
    s.verificationEventService.CreateVerificationEvent(ctx, verificationEvent)
}
```

**Testing**:
1. Create a new agent
2. Verify agent is auto-verified
3. Check verification events table has new entry
4. Refresh dashboard - chart should show data

---

### 2. Create Security Policies Page
**Issue**: `/dashboard/security/policies` returns 404
**Root Cause**: Page doesn't exist in frontend
**Impact**: Users can't view or manage security policies

**Implementation**:

#### Backend (Already Exists)
- ✅ `GET /api/v1/security-policies` - List policies
- ✅ `POST /api/v1/security-policies` - Create policy
- ✅ `PUT /api/v1/security-policies/:id` - Update policy
- ✅ `DELETE /api/v1/security-policies/:id` - Delete policy

#### Frontend (Need to Create)
**File**: `apps/web/app/dashboard/security/policies/page.tsx`

```typescript
'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Shield, Plus, Edit, Trash2 } from 'lucide-react';

interface SecurityPolicy {
  id: string;
  name: string;
  description: string;
  policy_type: string;
  enabled: boolean;
  severity: 'low' | 'medium' | 'high' | 'critical';
  conditions: any;
  actions: any;
  created_at: string;
  updated_at: string;
}

export default function SecurityPoliciesPage() {
  const [policies, setPolicies] = useState<SecurityPolicy[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchPolicies();
  }, []);

  const fetchPolicies = async () => {
    const token = localStorage.getItem('auth_token');
    const response = await fetch('http://localhost:8080/api/v1/security-policies', {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
    const data = await response.json();
    setPolicies(data.policies || []);
    setLoading(false);
  };

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Security Policies</h1>
          <p className="text-muted-foreground">
            Manage security policies for agent behavior and access control
          </p>
        </div>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create Policy
        </Button>
      </div>

      <div className="grid gap-4">
        {policies.map((policy) => (
          <Card key={policy.id}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Shield className="h-5 w-5 text-blue-500" />
                  <div>
                    <CardTitle>{policy.name}</CardTitle>
                    <CardDescription>{policy.description}</CardDescription>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={policy.enabled ? 'default' : 'secondary'}>
                    {policy.enabled ? 'Enabled' : 'Disabled'}
                  </Badge>
                  <Badge variant={
                    policy.severity === 'critical' ? 'destructive' :
                    policy.severity === 'high' ? 'destructive' :
                    policy.severity === 'medium' ? 'default' : 'secondary'
                  }>
                    {policy.severity}
                  </Badge>
                  <Button variant="ghost" size="sm">
                    <Edit className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="sm">
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </CardHeader>
          </Card>
        ))}
      </div>
    </div>
  );
}
```

---

## 🎯 Priority 2: Agent Lifecycle Management Endpoints

### 3. Suspend Agent
**Endpoint**: `POST /api/v1/agents/:id/suspend`
**Status**: ❌ Missing
**Impact**: Can't temporarily disable misbehaving agents

**Backend Implementation**:
```go
// File: internal/interfaces/http/handlers/agent_handler.go

func (h *AgentHandler) SuspendAgent(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    orgID := c.Locals("organization_id").(uuid.UUID)

    // Get agent
    agent, err := h.agentService.GetAgent(c.Context(), agentID, orgID)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "Agent not found",
        })
    }

    // Update status to suspended
    agent.Status = "suspended"
    if err := h.agentService.UpdateAgent(c.Context(), agent); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to suspend agent",
        })
    }

    // Create audit log
    h.auditService.CreateAuditLog(c.Context(), &domain.AuditLog{
        OrganizationID: orgID,
        UserID:         c.Locals("user_id").(uuid.UUID),
        Action:         "agent.suspend",
        ResourceType:   "agent",
        ResourceID:     &agentID,
        Status:         "success",
    })

    return c.JSON(fiber.Map{
        "success": true,
        "message": "Agent suspended successfully",
        "agent":   agent,
    })
}
```

**Frontend Implementation**:
```typescript
// File: apps/web/app/dashboard/agents/[id]/page.tsx
// Add to agent detail page:

const handleSuspend = async () => {
  const confirmed = confirm('Are you sure you want to suspend this agent?');
  if (!confirmed) return;

  const token = localStorage.getItem('auth_token');
  const response = await fetch(`http://localhost:8080/api/v1/agents/${agent.id}/suspend`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  });

  if (response.ok) {
    toast.success('Agent suspended successfully');
    fetchAgent(); // Refresh agent data
  }
};

// In JSX:
<Button
  variant="destructive"
  onClick={handleSuspend}
  disabled={agent.status === 'suspended'}
>
  <Ban className="mr-2 h-4 w-4" />
  Suspend Agent
</Button>
```

---

### 4. Reactivate Agent
**Endpoint**: `POST /api/v1/agents/:id/reactivate`
**Status**: ❌ Missing
**Impact**: Can't re-enable suspended agents

**Backend Implementation**:
```go
func (h *AgentHandler) ReactivateAgent(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    orgID := c.Locals("organization_id").(uuid.UUID)

    agent, err := h.agentService.GetAgent(c.Context(), agentID, orgID)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "Agent not found",
        })
    }

    // Update status to verified (reactivate)
    agent.Status = "verified"
    if err := h.agentService.UpdateAgent(c.Context(), agent); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to reactivate agent",
        })
    }

    return c.JSON(fiber.Map{
        "success": true,
        "message": "Agent reactivated successfully",
        "agent":   agent,
    })
}
```

---

### 5. Rotate Credentials
**Endpoint**: `POST /api/v1/agents/:id/rotate-credentials`
**Status**: ❌ Missing
**Impact**: Can't rotate compromised credentials

**Backend Implementation**:
```go
func (h *AgentHandler) RotateCredentials(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    orgID := c.Locals("organization_id").(uuid.UUID)

    agent, err := h.agentService.GetAgent(c.Context(), agentID, orgID)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "Agent not found",
        })
    }

    // Generate new Ed25519 key pair
    keyPair, err := crypto.GenerateEd25519KeyPair()
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to generate new keys",
        })
    }

    encodedKeys := crypto.EncodeKeyPair(keyPair)
    encryptedPrivateKey, err := h.keyVault.EncryptPrivateKey(encodedKeys.PrivateKeyBase64)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to encrypt private key",
        })
    }

    // Update agent keys
    agent.PublicKey = &encodedKeys.PublicKeyBase64
    agent.EncryptedPrivateKey = &encryptedPrivateKey

    if err := h.agentService.UpdateAgent(c.Context(), agent); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to rotate credentials",
        })
    }

    return c.JSON(fiber.Map{
        "success": true,
        "message": "Credentials rotated successfully",
        "new_public_key": encodedKeys.PublicKeyBase64,
    })
}
```

---

## 📊 Priority 3: Compliance Page Updates

### 6. Update Compliance Page Endpoint List
**Issue**: Compliance page doesn't show all available endpoints
**File**: `apps/web/app/dashboard/admin/compliance/page.tsx`

**Missing Endpoints to Add**:
1. Agent Lifecycle Management
   - `POST /api/v1/agents/:id/suspend`
   - `POST /api/v1/agents/:id/reactivate`
   - `POST /api/v1/agents/:id/rotate-credentials`

2. Webhook Management
   - `POST /api/v1/webhooks/:id/disable`
   - `POST /api/v1/webhooks/:id/enable`
   - `POST /api/v1/webhooks/:id/test`

3. Security Features
   - `GET /api/v1/security-policies`
   - `POST /api/v1/security-policies`
   - `PUT /api/v1/security-policies/:id`
   - `DELETE /api/v1/security-policies/:id`

---

## 📈 Implementation Timeline

### Week 1: Critical Fixes
- [ ] Day 1-2: Fix Verification Activity Chart
- [ ] Day 3-4: Create Security Policies Page
- [ ] Day 5: Testing and bug fixes

### Week 2: Agent Lifecycle Endpoints
- [ ] Day 1: Suspend Agent endpoint + UI
- [ ] Day 2: Reactivate Agent endpoint + UI
- [ ] Day 3: Rotate Credentials endpoint + UI
- [ ] Day 4-5: Testing and integration

### Week 3: Polish & Compliance
- [ ] Day 1-2: Update Compliance page
- [ ] Day 3-4: End-to-end testing
- [ ] Day 5: Documentation updates

---

## ✅ Success Criteria

1. **Verification Activity Chart**
   - Shows data for all auto-verified agents
   - Monthly aggregation working correctly
   - Chart renders properly with data

2. **Security Policies Page**
   - Lists all policies from database
   - Can create, edit, delete policies
   - Proper role-based access control

3. **Agent Lifecycle Endpoints**
   - All 4 endpoints implemented and tested
   - UI integrated into agent detail pages
   - Audit logs created for all actions

4. **Compliance Page**
   - Shows all 60+ endpoints
   - Grouped by category
   - Accurate endpoint counts

---

## 🎯 Next Steps

1. Review and approve this implementation plan
2. Prioritize features based on user needs
3. Begin implementation starting with Priority 1 items
4. Create pull requests for each feature
5. Update TASK.md with progress

**Questions?** Contact the development team or review CLAUDE_CONTEXT.md for more details.
