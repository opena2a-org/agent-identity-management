# AIM Missing UI Features - Enhancement Project Plan

**Project Owner**: Claude (Full Ownership)
**Start Date**: October 22, 2025
**Estimated Duration**: 5 Sprints (10 weeks)
**Goal**: Achieve 100% UI coverage for all user-facing backend endpoints

---

## Executive Summary

**Current State**: 60% UI coverage (70/116 endpoints)
**Target State**: 100% UI coverage for user-facing endpoints
**Impact**: Enhanced user experience, feature parity, production-ready application

**Features to Implement**:
1. ✅ Tags Management System (13 endpoints)
2. ✅ Agent Lifecycle Management (4 features)
3. ✅ Advanced Analytics (4 features)
4. ✅ Webhooks System (4 endpoints)
5. ✅ Compliance Details (3 features)
6. ✅ Alert Management Enhancements (1 feature)
7. ✅ Verification System Enhancements (3 features)

---

## Sprint Breakdown

### Sprint 1: Tags Management System (2 weeks)
**Priority**: HIGH
**Story Points**: 13
**Endpoints**: 13 (5 core + 8 agent/MCP)

#### User Stories
1. **As an admin**, I want to create and manage tags to organize agents and MCP servers
2. **As a user**, I want to filter agents by tags to find relevant resources quickly
3. **As a developer**, I want to apply tags to agents and MCPs to categorize them

#### Technical Tasks
- [ ] Create `/dashboard/tags` page with CRUD operations
- [ ] Implement tag creation modal with category selection
- [ ] Add tag list view with filtering and search
- [ ] Create tag detail/edit modal
- [ ] Implement tag deletion with confirmation
- [ ] Add tag widgets to Agent detail page (4 endpoints)
- [ ] Add tag widgets to MCP detail page (4 endpoints)
- [ ] Implement tag suggestions UI
- [ ] Add tag filtering to agent/MCP list pages
- [ ] Test with Chrome DevTools

#### Acceptance Criteria
- ✅ User can create tags with key, value, category, description, color
- ✅ User can view all tags in a searchable, filterable list
- ✅ User can edit existing tags
- ✅ User can delete tags (with confirmation)
- ✅ User can apply tags to agents from agent detail page
- ✅ User can apply tags to MCPs from MCP detail page
- ✅ User can see tag suggestions when adding tags
- ✅ User can filter agents/MCPs by tags
- ✅ All 13 tag endpoints verified in Chrome DevTools

#### Files to Create/Modify
```
apps/web/app/dashboard/tags/page.tsx              # Main tags page
apps/web/components/tags/tag-list.tsx             # Tag list component
apps/web/components/tags/tag-create-modal.tsx     # Create tag modal
apps/web/components/tags/tag-edit-modal.tsx       # Edit tag modal
apps/web/components/tags/tag-badge.tsx            # Tag badge component
apps/web/components/tags/tag-selector.tsx         # Tag selector for agents/MCPs
apps/web/components/agent/agent-tags-widget.tsx   # Agent tags widget
apps/web/components/mcp/mcp-tags-widget.tsx       # MCP tags widget
```

---

### Sprint 2: Agent Lifecycle Management (2 weeks)
**Priority**: HIGH
**Story Points**: 8
**Endpoints**: 4

#### User Stories
1. **As a manager**, I want to suspend/reactivate agents to control access
2. **As a security admin**, I want to rotate agent credentials for security
3. **As an admin**, I want to manually adjust trust scores when needed
4. **As a manager**, I want to recalculate trust scores on demand

#### Technical Tasks
- [ ] Add Suspend/Reactivate buttons to agent detail page
- [ ] Implement suspend agent functionality (`POST /api/v1/agents/:id/suspend`)
- [ ] Implement reactivate agent functionality (`POST /api/v1/agents/:id/reactivate`)
- [ ] Add Rotate Credentials button to Key Vault tab
- [ ] Implement credential rotation UI (`POST /api/v1/agents/:id/rotate-credentials`)
- [ ] Add Manual Trust Score adjustment (admin only) (`PUT /api/v1/agents/:id/trust-score`)
- [ ] Add Recalculate Trust Score button (`POST /api/v1/agents/:id/trust-score/recalculate`)
- [ ] Add confirmation modals for destructive actions
- [ ] Test with Chrome DevTools

#### Acceptance Criteria
- ✅ Manager can suspend agent (status changes to "suspended")
- ✅ Manager can reactivate suspended agent (status changes to "verified")
- ✅ Admin can rotate agent credentials (new keys generated)
- ✅ Admin can manually adjust trust score with reason
- ✅ User can trigger trust score recalculation
- ✅ All actions show confirmation modal
- ✅ All 4 endpoints verified in Chrome DevTools

#### Files to Modify
```
apps/web/app/dashboard/agents/[id]/page.tsx       # Add suspend/reactivate buttons
apps/web/components/agent/key-vault-tab.tsx       # Add rotate credentials
apps/web/components/agent/trust-score-card.tsx    # Add manual adjustment
```

---

### Sprint 3: Advanced Analytics (2 weeks)
**Priority**: MEDIUM
**Story Points**: 10
**Endpoints**: 4

#### User Stories
1. **As a business user**, I want to see trust score trends over time
2. **As an admin**, I want to view usage statistics for capacity planning
3. **As a manager**, I want to see agent activity timeline
4. **As a dashboard viewer**, I want comprehensive analytics

#### Technical Tasks
- [ ] Create Trust Score Trends chart component
- [ ] Implement `GET /api/v1/analytics/trends` endpoint integration
- [ ] Add Trust Score Trends to dashboard
- [ ] Create Usage Statistics page (`/dashboard/analytics/usage`)
- [ ] Implement `GET /api/v1/analytics/usage` endpoint integration
- [ ] Create Agent Activity timeline component
- [ ] Implement `GET /api/v1/analytics/agents/activity` endpoint integration
- [ ] Add Agent Activity tab to dashboard or separate page
- [ ] Add date range filters to all analytics
- [ ] Test with Chrome DevTools

#### Acceptance Criteria
- ✅ Dashboard shows trust score trends chart (line/area chart)
- ✅ Usage statistics page shows API calls, active users, etc.
- ✅ Agent activity timeline shows recent agent actions
- ✅ User can filter analytics by date range
- ✅ Charts are responsive and interactive
- ✅ All 4 analytics endpoints verified in Chrome DevTools

#### Files to Create/Modify
```
apps/web/app/dashboard/analytics/usage/page.tsx   # Usage statistics page
apps/web/components/analytics/trust-trends.tsx    # Trust trends chart
apps/web/components/analytics/usage-stats.tsx     # Usage stats component
apps/web/components/analytics/activity-timeline.tsx # Activity timeline
apps/web/app/dashboard/page.tsx                   # Add trends to dashboard
```

---

### Sprint 4: Webhooks System (1.5 weeks)
**Priority**: MEDIUM
**Story Points**: 8
**Endpoints**: 4

#### User Stories
1. **As an admin**, I want to create webhooks to integrate with external systems
2. **As a developer**, I want to manage webhook endpoints and test them
3. **As a security admin**, I want to monitor webhook deliveries

#### Technical Tasks
- [ ] Create `/dashboard/webhooks` page
- [ ] Implement webhook list view (`GET /api/v1/webhooks`)
- [ ] Create webhook creation modal (`POST /api/v1/webhooks`)
- [ ] Implement webhook detail view (`GET /api/v1/webhooks/:id`)
- [ ] Add webhook deletion (`DELETE /api/v1/webhooks/:id`)
- [ ] Add webhook testing UI (send test payload)
- [ ] Add webhook delivery logs view
- [ ] Implement webhook enable/disable toggle
- [ ] Test with Chrome DevTools

#### Acceptance Criteria
- ✅ User can create webhooks with URL, events, and secret
- ✅ User can view all webhooks in a list
- ✅ User can view webhook details and delivery history
- ✅ User can test webhooks with sample payload
- ✅ User can enable/disable webhooks
- ✅ User can delete webhooks
- ✅ All 4 webhook endpoints verified in Chrome DevTools

#### Files to Create
```
apps/web/app/dashboard/webhooks/page.tsx          # Webhooks list page
apps/web/components/webhook/webhook-list.tsx      # Webhook list component
apps/web/components/webhook/webhook-create-modal.tsx # Create webhook modal
apps/web/components/webhook/webhook-detail.tsx    # Webhook detail view
apps/web/components/webhook/webhook-test.tsx      # Webhook testing UI
```

---

### Sprint 5: Compliance & Verification Enhancements (1.5 weeks)
**Priority**: MEDIUM
**Story Points**: 7
**Endpoints**: 7

#### User Stories
1. **As a compliance officer**, I want to view access review reports
2. **As an admin**, I want to manage data retention policies
3. **As a security admin**, I want to resolve alerts
4. **As a viewer**, I want to see detailed verification information

#### Technical Tasks
- [ ] Add Access Review tab to compliance page (`GET /api/v1/compliance/access-review`)
- [ ] Add Data Retention tab to compliance page (`GET /api/v1/compliance/data-retention`)
- [ ] Implement compliance report export
- [ ] Add Resolve button to alerts page (`POST /api/v1/admin/alerts/:id/resolve`)
- [ ] Create verification detail modal
- [ ] Add verification filtering by agent/MCP
- [ ] Implement agent audit logs view (`GET /api/v1/agents/:id/audit-logs`)
- [ ] Test with Chrome DevTools

#### Acceptance Criteria
- ✅ Compliance page shows access review with user access history
- ✅ Compliance page shows data retention policies
- ✅ Admin can resolve alerts from alerts page
- ✅ User can view detailed verification information
- ✅ User can filter verifications by agent/MCP
- ✅ Agent detail page shows audit logs
- ✅ All 7 endpoints verified in Chrome DevTools

#### Files to Modify/Create
```
apps/web/app/dashboard/admin/compliance/page.tsx  # Add tabs
apps/web/components/compliance/access-review.tsx  # Access review component
apps/web/components/compliance/data-retention.tsx # Data retention component
apps/web/app/dashboard/admin/alerts/page.tsx      # Add resolve button
apps/web/components/verification/detail-modal.tsx # Verification detail
apps/web/components/agent/audit-logs-tab.tsx      # Audit logs tab
```

---

## Technical Architecture

### Component Structure
```
apps/web/
├── app/
│   ├── dashboard/
│   │   ├── tags/
│   │   │   └── page.tsx                 # NEW: Tags management
│   │   ├── webhooks/
│   │   │   └── page.tsx                 # NEW: Webhooks management
│   │   ├── analytics/
│   │   │   └── usage/
│   │   │       └── page.tsx             # NEW: Usage statistics
│   │   ├── agents/[id]/
│   │   │   └── page.tsx                 # MODIFY: Add lifecycle buttons, tags, audit logs
│   │   ├── mcp/[id]/
│   │   │   └── page.tsx                 # MODIFY: Add tags
│   │   ├── admin/
│   │   │   ├── compliance/
│   │   │   │   └── page.tsx             # MODIFY: Add access review, data retention
│   │   │   └── alerts/
│   │   │       └── page.tsx             # MODIFY: Add resolve button
│   │   └── page.tsx                     # MODIFY: Add trust trends
├── components/
│   ├── tags/                            # NEW: Tag components
│   ├── webhook/                         # NEW: Webhook components
│   ├── analytics/                       # NEW: Analytics components
│   ├── compliance/                      # NEW: Compliance components
│   ├── verification/                    # NEW: Verification components
│   ├── agent/                           # MODIFY: Add lifecycle, tags, audit
│   └── mcp/                             # MODIFY: Add tags
```

### API Client Updates
```typescript
// apps/web/lib/api.ts - Already has all methods, just need UI!
// Tags: ✅ 8 methods exist
// Webhooks: ❌ Need to add 4 methods
// Analytics: ❌ Need to add 3 methods
// Agent Lifecycle: ❌ Need to add 4 methods
// Compliance: ✅ 2 methods exist, need 1 more
```

---

## Testing Strategy

### Chrome DevTools Verification Checklist
For each feature:
1. ✅ Navigate to page in production
2. ✅ Open Chrome DevTools Network tab
3. ✅ Perform action (create, update, delete)
4. ✅ Verify correct endpoint called (method, URL, payload)
5. ✅ Verify response status (200/201)
6. ✅ Verify UI updates correctly
7. ✅ Take screenshot as proof
8. ✅ Document in verification report

### Test Scenarios by Feature

#### Tags Management
- [ ] Create tag → POST /api/v1/tags → Verify tag appears in list
- [ ] Edit tag → PUT /api/v1/tags/:id → Verify changes saved
- [ ] Delete tag → DELETE /api/v1/tags/:id → Verify tag removed
- [ ] Add tag to agent → POST /api/v1/agents/:id/tags → Verify tag on agent
- [ ] Remove tag from agent → DELETE /api/v1/agents/:id/tags/:tagId → Verify removed
- [ ] Get tag suggestions → GET /api/v1/agents/:id/tags/suggestions → Verify suggestions shown
- [ ] Filter agents by tag → Verify filtered results
- [ ] Search tags → GET /api/v1/tags/search → Verify search works

#### Agent Lifecycle
- [ ] Suspend agent → POST /api/v1/agents/:id/suspend → Verify status = suspended
- [ ] Reactivate agent → POST /api/v1/agents/:id/reactivate → Verify status = verified
- [ ] Rotate credentials → POST /api/v1/agents/:id/rotate-credentials → Verify new keys
- [ ] Adjust trust score → PUT /api/v1/agents/:id/trust-score → Verify score updated
- [ ] Recalculate trust score → POST /api/v1/agents/:id/trust-score/recalculate → Verify calculation

#### Advanced Analytics
- [ ] View trust trends → GET /api/v1/analytics/trends → Verify chart displays
- [ ] View usage stats → GET /api/v1/analytics/usage → Verify stats shown
- [ ] View agent activity → GET /api/v1/analytics/agents/activity → Verify timeline
- [ ] Filter by date range → Verify filtered data

#### Webhooks
- [ ] Create webhook → POST /api/v1/webhooks → Verify webhook created
- [ ] List webhooks → GET /api/v1/webhooks → Verify webhooks shown
- [ ] View webhook detail → GET /api/v1/webhooks/:id → Verify details displayed
- [ ] Delete webhook → DELETE /api/v1/webhooks/:id → Verify webhook deleted
- [ ] Test webhook → Verify test payload sent

#### Compliance
- [ ] View access review → GET /api/v1/compliance/access-review → Verify data shown
- [ ] View data retention → GET /api/v1/compliance/data-retention → Verify policies shown
- [ ] Resolve alert → POST /api/v1/admin/alerts/:id/resolve → Verify alert resolved
- [ ] View agent audit logs → GET /api/v1/agents/:id/audit-logs → Verify logs shown

---

## Risk Assessment

### High Risks
1. **Breaking Changes**: Modifying existing pages might break current functionality
   - **Mitigation**: Test existing features after each change, use feature flags

2. **State Management**: Complex state across multiple components
   - **Mitigation**: Use React Query for server state, Zustand for client state

3. **Performance**: Large lists (tags, webhooks) might be slow
   - **Mitigation**: Implement pagination, virtual scrolling, debounced search

### Medium Risks
1. **UI/UX Consistency**: New components might not match existing design
   - **Mitigation**: Use Shadcn/ui components, follow existing patterns

2. **Backend Compatibility**: Endpoints might have unexpected behavior
   - **Mitigation**: Test with Chrome DevTools, validate responses

### Low Risks
1. **Browser Compatibility**: Modern features might not work in old browsers
   - **Mitigation**: Target modern browsers (Chrome, Firefox, Safari, Edge)

---

## Success Metrics

### Quantitative
- ✅ **100% UI coverage** for user-facing endpoints (target: 90/116)
- ✅ **0 critical bugs** in production
- ✅ **<100ms API response time** maintained
- ✅ **100% test coverage** for new components
- ✅ **All endpoints verified** with Chrome DevTools

### Qualitative
- ✅ **Improved user experience** - Users can manage all features from UI
- ✅ **Feature parity** - All backend capabilities exposed in UI
- ✅ **Professional UI** - Consistent design, polished interactions
- ✅ **Production-ready** - Ready for investor demo and user adoption

---

## Timeline

| Sprint | Duration | Features | Endpoints | Completion |
|--------|----------|----------|-----------|------------|
| Sprint 1 | 2 weeks | Tags Management | 13 | Week 2 |
| Sprint 2 | 2 weeks | Agent Lifecycle | 4 | Week 4 |
| Sprint 3 | 2 weeks | Advanced Analytics | 4 | Week 6 |
| Sprint 4 | 1.5 weeks | Webhooks | 4 | Week 7.5 |
| Sprint 5 | 1.5 weeks | Compliance & Verification | 7 | Week 9 |
| Testing & Documentation | 1 week | All features | 32 | Week 10 |

**Total**: 10 weeks to 100% UI coverage

---

## Next Steps

### Immediate Actions (Today)
1. ✅ Verify Swagger documentation alignment
2. ✅ Create feature branches for each sprint
3. ✅ Set up Chrome DevTools testing environment
4. ✅ Begin Sprint 1: Tags Management System

### This Week
1. Implement Tags Management UI (all 13 endpoints)
2. Test with Chrome DevTools
3. Create verification report for Sprint 1
4. Plan Sprint 2 detailed tasks

### This Month
1. Complete Sprints 1-2 (Tags + Agent Lifecycle)
2. Begin Sprint 3 (Advanced Analytics)
3. Weekly progress reports
4. Continuous Chrome DevTools verification

---

**Project Status**: 🚀 READY TO START
**Owner**: Claude (Full Ownership)
**Next Task**: Verify Swagger documentation alignment
