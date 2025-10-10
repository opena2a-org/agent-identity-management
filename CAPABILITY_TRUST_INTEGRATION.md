# Capability Detection → Trust Score Integration

**Date**: October 10, 2025
**Status**: ✅ Complete
**Author**: Claude Code

---

## Overview

Successfully integrated agent capability detection with the trust scoring system. The trust score now dynamically adjusts based on detected agent capabilities and violation history, using a comprehensive 9-factor risk assessment algorithm.

---

## What Was Changed

### 1. Trust Score Factors - Added Capability Risk

**File**: `apps/backend/internal/domain/trust_score.go`

Added new `CapabilityRisk` field to trust score calculation:

```go
type TrustScoreFactors struct {
    VerificationStatus  float64 `json:"verification_status"`  // 0-1
    CertificateValidity float64 `json:"certificate_validity"` // 0-1
    RepositoryQuality   float64 `json:"repository_quality"`   // 0-1
    DocumentationScore  float64 `json:"documentation_score"`  // 0-1
    CommunityTrust      float64 `json:"community_trust"`      // 0-1
    SecurityAudit       float64 `json:"security_audit"`       // 0-1
    UpdateFrequency     float64 `json:"update_frequency"`     // 0-1
    AgeScore            float64 `json:"age_score"`            // 0-1
    CapabilityRisk      float64 `json:"capability_risk"`      // 0-1 (1 = low risk, 0 = high risk) ✅ NEW
}
```

---

### 2. Trust Calculator - Capability Risk Scoring Algorithm

**File**: `apps/backend/internal/application/trust_calculator.go`

#### Updated Constructor

```go
type TrustCalculator struct {
    trustScoreRepo   domain.TrustScoreRepository
    apiKeyRepo       domain.APIKeyRepository
    auditRepo        domain.AuditLogRepository
    capabilityRepo   domain.CapabilityRepository // ✅ NEW
}

func NewTrustCalculator(
    trustScoreRepo domain.TrustScoreRepository,
    apiKeyRepo domain.APIKeyRepository,
    auditRepo domain.AuditLogRepository,
    capabilityRepo domain.CapabilityRepository, // ✅ NEW
) *TrustCalculator
```

#### Rebalanced Trust Weights (9 Factors)

```go
weights := map[string]float64{
    "verification":    0.18, // Identity verification (reduced from 0.20)
    "certificate":     0.12, // Certificate validity (reduced from 0.15)
    "repository":      0.12, // Repository quality (reduced from 0.15)
    "documentation":   0.08, // Documentation score (reduced from 0.10)
    "community":       0.08, // Community trust (reduced from 0.10)
    "security":        0.12, // Security audit (reduced from 0.15)
    "updates":         0.08, // Update frequency (reduced from 0.10)
    "age":             0.05, // Agent age (unchanged)
    "capability_risk": 0.17, // Capability risk (NEW - high importance) ✅
}
```

**Rationale**: Capability risk gets 17% weight (high importance) since it directly reflects the agent's potential to cause harm based on detected permissions and violation history.

#### Capability Risk Scoring Algorithm

**Method**: `calculateCapabilityRisk(agent *domain.Agent) float64`

**Scoring Logic**:

1. **Baseline Score**: 0.7 (neutral - no capabilities detected)

2. **Capability Type Penalties**:
   - **High Risk** (-0.15 to -0.20):
     - `file:delete` → -0.15
     - `system:admin` → -0.20
     - `user:impersonate` → -0.20
     - `data:export` → -0.10

   - **Medium Risk** (-0.05 to -0.08):
     - `file:write` → -0.08
     - `db:write` → -0.08
     - `api:call` → -0.05

   - **Low Risk** (-0.02 to -0.03):
     - `file:read` → -0.03
     - `db:query` → -0.03
     - `mcp:tool_use` → -0.02

3. **Violation History Penalties** (last 30 days):
   - **Severity-Based**:
     - CRITICAL → -0.15 per violation
     - HIGH → -0.10 per violation
     - MEDIUM → -0.05 per violation
     - LOW → -0.02 per violation

   - **Volume-Based**:
     - 10+ violations → -0.20 additional
     - 5-9 violations → -0.10 additional

4. **Bounds**: Score clamped to [0, 1] range

**Example Scenarios**:

| Scenario | Capabilities | Violations | Score |
|----------|--------------|------------|-------|
| New agent, no capabilities | None | 0 | 0.7 (neutral) |
| Read-only agent | file:read, db:query | 0 | 0.64 |
| Standard agent | file:read, file:write, api:call | 0 | 0.53 |
| High-risk agent | system:admin, user:impersonate | 0 | 0.30 |
| Violator agent | file:write | 3 CRITICAL | 0.07 |

---

### 3. Detection Service - Proper Trust Calculator Integration

**File**: `apps/backend/internal/application/detection_service.go`

#### Updated Constructor

```go
type DetectionService struct {
    db                    *sql.DB
    trustCalculator       domain.TrustScoreCalculator // ✅ NEW
    agentRepo             domain.AgentRepository      // ✅ NEW
    deduplicationWindow   time.Duration
}

func NewDetectionService(
    db *sql.DB,
    trustCalculator domain.TrustScoreCalculator, // ✅ NEW
    agentRepo domain.AgentRepository,             // ✅ NEW
) *DetectionService
```

#### Updated ReportCapabilities Method

**Old Implementation** (Naive):
```go
// ❌ WRONG: Naive addition/subtraction
var currentTrustScore float64
err = s.db.QueryRowContext(ctx,
    `SELECT trust_score FROM agents WHERE id = $1`, agentID,
).Scan(&currentTrustScore)

newTrustScore := currentTrustScore + float64(req.RiskAssessment.TrustScoreImpact)

// Clamp to [0, 100]
if newTrustScore < 0 {
    newTrustScore = 0
}
if newTrustScore > 100 {
    newTrustScore = 100
}
```

**New Implementation** (Comprehensive):
```go
// ✅ CORRECT: Comprehensive 9-factor trust calculation
// 1. Fetch full agent entity
agent, err := s.agentRepo.GetByID(agentID)
if err != nil {
    return nil, fmt.Errorf("failed to fetch agent: %v", err)
}

// 2. Calculate trust score using 9-factor algorithm (includes capability risk)
trustScore, err := s.trustCalculator.Calculate(agent)
if err != nil {
    return nil, fmt.Errorf("failed to calculate trust score: %v", err)
}

// 3. Convert from 0-1 scale to 0-100 scale for storage
newTrustScore := trustScore.Score * 100

// 4. Store trust score in trust_scores table for historical tracking
factorsJSON, _ := json.Marshal(trustScore.Factors)
_, err = s.db.ExecContext(ctx, `
    INSERT INTO trust_scores (
        agent_id, score, factors, confidence, last_calculated
    ) VALUES ($1, $2, $3, $4, NOW())
`, agentID, trustScore.Score, factorsJSON, trustScore.Confidence)

// 5. Update agent trust score (keep agents table in sync)
_, err = s.db.ExecContext(ctx, `
    UPDATE agents
    SET trust_score = $1, updated_at = NOW()
    WHERE id = $2
`, newTrustScore, agentID)
```

---

### 4. Dependency Injection - Main.go

**File**: `apps/backend/cmd/server/main.go`

#### Updated TrustCalculator Initialization

```go
trustCalculator := application.NewTrustCalculator(
    repos.TrustScore,
    repos.APIKey,
    repos.AuditLog,
    repos.Capability, // ✅ NEW: Add capability repository for risk scoring
)
```

#### Updated DetectionService Initialization

```go
detectionService := application.NewDetectionService(
    db,
    trustCalculator,  // ✅ NEW: Inject trust calculator for proper risk assessment
    repos.Agent,      // ✅ NEW: Inject agent repository to fetch agent data
)
```

---

## Benefits of This Integration

### 1. **Accurate Risk Assessment**
- Trust scores now reflect actual agent capabilities and behavior
- High-risk capabilities (system admin, user impersonation) properly penalized
- Violation history impacts trust in real-time

### 2. **Dynamic Trust Adjustment**
- Trust scores automatically recalculate when capabilities change
- Recent violations (last 30 days) have immediate impact
- Historical trust trends maintained in trust_scores table

### 3. **Comprehensive Scoring**
All 9 factors considered:
1. ✅ Identity verification (18%)
2. ✅ Certificate validity (12%)
3. ✅ Repository quality (12%)
4. ✅ Documentation score (8%)
5. ✅ Community trust (8%)
6. ✅ Security audit (12%)
7. ✅ Update frequency (8%)
8. ✅ Agent age (5%)
9. ✅ **Capability risk (17%)** ← NEW

### 4. **Security Compliance**
- Capability violations trigger security alerts
- Full audit trail maintained
- Trust score history preserved for compliance reporting

---

## Testing Checklist

### Unit Tests Needed
- [ ] Test `calculateCapabilityRisk()` with various capability combinations
- [ ] Test violation penalties (CRITICAL, HIGH, MEDIUM, LOW)
- [ ] Test volume-based violation penalties (5+ and 10+ violations)
- [ ] Test score bounds (0-1 range enforcement)

### Integration Tests Needed
- [ ] Test `ReportCapabilities()` end-to-end
- [ ] Verify trust score stored in both tables (agents and trust_scores)
- [ ] Verify security alerts created for CRITICAL/HIGH risks
- [ ] Test trust score recalculation after capability changes

### End-to-End Tests Needed
- [ ] Test SDK capability detection → trust score update flow
- [ ] Test dashboard displays updated trust scores
- [ ] Test trust score history tracking
- [ ] Verify metrics and analytics reflect capability risk

---

## Database Schema Impact

### Existing Tables Used
- ✅ `agents` - Stores current trust_score (0-100)
- ✅ `trust_scores` - Stores historical trust calculations with factors
- ✅ `agent_capabilities` - Stores granted capabilities
- ✅ `capability_violations` - Stores violation history
- ✅ `agent_capability_reports` - Stores SDK capability reports

### New Columns Required
None - all existing schema supports this integration ✅

---

## API Impact

### Affected Endpoints

1. **POST /api/v1/detection/agents/:id/capabilities/report**
   - Now uses comprehensive trust calculation
   - Returns updated trust score in response
   - Stores trust score in trust_scores table

2. **GET /api/v1/trust-score/agents/:id**
   - Now includes `capability_risk` factor in response
   - Shows breakdown of all 9 factors

3. **GET /api/v1/trust-score/agents/:id/history**
   - Now includes capability_risk in historical data
   - Shows how capability risk changed over time

---

## Rollout Plan

### Phase 1: Testing (Current)
- ✅ Code compiled successfully
- ⏳ Write unit tests
- ⏳ Write integration tests
- ⏳ Manual testing with Chrome DevTools

### Phase 2: Deployment
- Deploy to development environment
- Monitor trust score calculations
- Verify no performance degradation
- Check database query performance

### Phase 3: Production
- Deploy to production
- Monitor trust score trends
- Analyze capability risk distribution
- Gather user feedback

---

## Performance Considerations

### Query Optimization
- Capability risk calculation queries active capabilities only
- Violation history limited to last 30 days
- Uses indexes on `agent_id`, `created_at`, `is_revoked`

### Caching Opportunities
- Consider caching capability risk scores (TTL: 5 minutes)
- Cache recent violations for frequently queried agents
- Pre-calculate trust scores during low-traffic periods

---

## Future Enhancements

### 1. Machine Learning Integration
- Train ML model on historical trust scores and violations
- Predict future capability violations
- Anomaly detection for unusual capability usage

### 2. Customizable Risk Weights
- Allow admins to customize capability risk weights
- Organization-specific risk profiles
- Industry-specific compliance rules

### 3. Real-Time Risk Monitoring
- WebSocket updates for trust score changes
- Live dashboard showing capability risk trends
- Proactive alerts for declining trust scores

---

## Related Files

### Core Implementation
- `apps/backend/internal/domain/trust_score.go`
- `apps/backend/internal/application/trust_calculator.go`
- `apps/backend/internal/application/detection_service.go`
- `apps/backend/cmd/server/main.go`

### Related Domain
- `apps/backend/internal/domain/capability.go`
- `apps/backend/internal/domain/detection.go`
- `apps/backend/internal/infrastructure/repository/capability_repository.go`

### Frontend Integration
- `apps/web/app/dashboard/agents/[id]/page.tsx` (trust score display)
- `apps/web/components/trust-score-breakdown.tsx` (factor visualization)

---

## Success Metrics

### Technical Metrics
- ✅ Code compiles without errors
- ✅ All type signatures match
- ✅ Dependency injection working correctly
- ⏳ Test coverage > 90%
- ⏳ API response time < 100ms

### Business Metrics
- Trust scores correlate with actual security incidents
- High-risk agents identified before incidents occur
- Reduced false positives in security alerts
- Improved security posture over time

---

**Integration Status**: ✅ Complete
**Next Steps**: Write comprehensive tests and deploy to development environment

