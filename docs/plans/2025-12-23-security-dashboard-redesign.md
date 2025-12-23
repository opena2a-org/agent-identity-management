# Security Dashboard Redesign

**Date:** 2025-12-23
**Status:** Approved
**Target:** Allstate Demo + Silicon Valley Quality

## Overview

Redesign the Security Dashboard from a static metrics page into a "Security Command Center" that demonstrates AIM's value proposition: visibility, control, and protection of AI agents.

## Problem Statement

Current dashboard issues:
- "Blocked Threats: 0" pulls from wrong data source (alerts.is_acknowledged instead of capability_violations.is_blocked)
- Metrics don't tell the AIM value story
- Static, report-like feel doesn't convey active protection
- Generic charts without actionable insights

## Design Principles

1. **Tell the story** - Every metric should answer "Is AIM protecting us?"
2. **Show the value** - Blocked actions are trophies, display them prominently
3. **Make it alive** - Real-time feel, not a static report
4. **Enable action** - Inline actions, not just data viewing
5. **Executive-ready** - Screenshots should be board-presentation worthy

## Data Model Changes

### New Metrics Sources

| Metric | Source | Query |
|--------|--------|-------|
| Security Score | Calculated | `(avg_trust * 40) + (blocked_ratio * 30) + (alert_ratio * 30)` |
| Actions Blocked | capability_violations | `WHERE is_blocked = true` |
| Agents Monitored | agents | `WHERE status IN ('active', 'verified')` |
| Requires Attention | Combined | pending_requests + unacknowledged_alerts |
| Actions Today | agent_actions | `WHERE created_at >= TODAY` |
| Trust Percentage | agents | `% WHERE trust_score > 0.8` |

### New API Response Structure

```typescript
interface SecurityMetrics {
  securityScore: number;           // 0-100 calculated score
  securityGrade: 'A' | 'B' | 'C' | 'D' | 'F';
  actionsBlocked: number;          // From capability_violations
  actionsBlockedToday: number;
  agentsMonitored: number;
  agentsTrusted: number;           // Trust > 80%
  trustPercentage: number;
  actionsToday: number;            // Total actions processed
  requiresAttention: number;       // Pending items
  lastIncidentAt: string | null;   // Most recent blocked action

  // For charts
  protectionTimeline: Array<{
    date: string;
    actions: number;
    blocked: number;
  }>;

  riskByCategory: Array<{
    category: string;
    blocked: number;
    riskLevel: 'high' | 'medium' | 'low' | 'secure';
  }>;

  // For blocked actions list
  recentBlockedActions: Array<{
    id: string;
    agentId: string;
    agentName: string;
    attemptedCapability: string;
    details: string;
    trustImpact: number;
    createdAt: string;
  }>;
}
```

## UI Components

### 1. Hero Section - Security Pulse

Large, prominent section at top showing overall security posture.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  ╭──────────╮                                                           │
│  │    92    │   Your AI Fleet is Secure            🟢 All Systems      │
│  │  ──────  │   285 agents monitored • 13 threats blocked today        │
│  │  /100    │   Last incident: 3 days ago                              │
│  ╰──────────╯                                                           │
│   [View Details]                                [Take Action →]         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Components:**
- Animated circular progress gauge (SVG)
- Dynamic status message based on score
- System health indicator (green/yellow/red dot)
- Quick stats summary
- Call-to-action buttons

**Score Thresholds:**
- 90-100: "Secure" (green)
- 75-89: "Good" (light green)
- 60-74: "Needs Attention" (yellow)
- 40-59: "At Risk" (orange)
- 0-39: "Critical" (red)

### 2. Stat Cards Row

Four glass-morphism cards with key metrics:

| Card | Icon | Primary | Secondary |
|------|------|---------|-----------|
| Actions Blocked | Shield | 13 | ↑ 4 today |
| Agents Monitored | Bot | 285 | 98% trusted |
| Actions Processed | Zap | 12,847 | today |
| Requires Attention | Bell | 23 | pending |

**Design:**
- Semi-transparent background with blur
- Subtle gradient border
- Number count-up animation on load
- Hover lift effect

### 3. Charts Section

**Left: Protection Timeline**
- Dual-axis line chart
- Blue line: Total actions (volume)
- Red area: Blocked actions (protection)
- Insight box below with AI-generated summary
- Time range selector (7d, 30d, 90d)

**Right: Risk by Category**
- Horizontal bar chart grouped by capability type
- Color-coded risk levels
- Category labels with blocked counts
- Insight box with recommendations

### 4. Blocked Actions Section

Card-based list showing AIM's protection in action:

```
┌────────────────────────────────────────────────────────────────────────┐
│ 🔴 BLOCKED  agent-data-03 attempted file_system:read                   │
│             /sensitive/customer-data.csv                               │
│             2 hours ago • Capability not granted • Trust -5%           │
│                                                    [Review Agent]      │
└────────────────────────────────────────────────────────────────────────┘
```

### 5. Two-Column Bottom Section

**Left: Requires Attention Queue**
- Capability requests with inline approve/deny
- Alerts requiring acknowledgment
- Priority sorted

**Right: Live Activity Feed**
- Real-time agent actions
- Color-coded by status
- WebSocket-powered (Phase 2)

## Implementation Phases

### Phase 1: Demo-Ready (Priority)

1. **Backend** (`security_repository.go`):
   - Fix GetSecurityMetrics to query correct tables
   - Add security score calculation
   - Add blocked actions query
   - Add protection timeline data
   - Add risk by category aggregation

2. **Frontend** (`security/page.tsx`):
   - Hero section with animated score gauge
   - Redesigned stat cards
   - Blocked actions card view
   - Basic requires attention list

### Phase 2: Polish

1. Protection Timeline chart with insights
2. Risk by Category chart with insights
3. Inline actions (approve/deny)
4. Animations and micro-interactions
5. Dark mode refinements

### Phase 3: Wow Factor (Post-Demo)

1. WebSocket live activity feed
2. AI-generated insights
3. Keyboard shortcuts
4. Sound effects for blocked actions (optional)

## File Changes

| File | Changes |
|------|---------|
| `apps/backend/internal/infrastructure/repository/security_repository.go` | Rewrite GetSecurityMetrics |
| `apps/backend/internal/domain/security.go` | Update SecurityMetrics struct |
| `apps/web/app/dashboard/security/page.tsx` | Complete redesign |
| `apps/web/components/security/SecurityScoreGauge.tsx` | New component |
| `apps/web/components/security/StatCard.tsx` | New component |
| `apps/web/components/security/BlockedActionCard.tsx` | New component |
| `apps/web/components/security/ProtectionTimeline.tsx` | New component |
| `apps/web/components/security/RiskByCategory.tsx` | New component |
| `apps/web/components/security/AttentionQueue.tsx` | New component |

## Success Criteria

1. Security Score displays accurate calculated value
2. Actions Blocked shows real data from capability_violations
3. Dashboard loads in < 2 seconds
4. Executive can understand security posture in 5 seconds
5. "Wow" reaction from Allstate demo

## Appendix: Current Data

As of 2025-12-23:
- 285 agents (all active)
- 90% average trust score
- 27 total capability violations (13 blocked)
- 23 pending capability requests
- 238 unacknowledged alerts
