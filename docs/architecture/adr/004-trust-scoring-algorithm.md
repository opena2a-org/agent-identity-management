# ADR 004: Trust Scoring Algorithm

**Status**: ✅ Accepted
**Date**: 2025-10-06
**Decision Makers**: AIM Architecture Team, ML Team, Security Team
**Stakeholders**: Product Team, Enterprise Customers

---

## Context

AIM needs a reliable, transparent, and fair system to evaluate the trustworthiness of AI agents and MCP servers. This trust score helps organizations make informed decisions about which agents to use in production.

### Requirements

1. **Objective**: Score should be based on measurable factors, not subjective opinions
2. **Transparent**: Users should understand how scores are calculated
3. **Fair**: All agents evaluated using same criteria
4. **Dynamic**: Scores update as agent behavior changes
5. **Fast**: Calculation must be <100ms to avoid API slowdowns
6. **Auditable**: Historical scores tracked for trend analysis

### Use Cases

- **Enterprise IT**: "Which AI agents are safe to deploy in production?"
- **Security Teams**: "Which agents pose the highest risk?"
- **Compliance Officers**: "Do our agents meet security standards?"
- **Developers**: "How can I improve my agent's trust score?"

---

## Decision

We will implement an **8-Factor ML-Powered Trust Scoring Algorithm** that combines:
1. Cryptographic verification status
2. Security audit results
3. Community trust ratings
4. Historical uptime
5. Security incident history
6. Compliance certifications
7. Activity patterns
8. Verification recency

### Algorithm Design

```
Trust Score = Σ (Factor_i × Weight_i) × 100

where:
- Trust Score: 0-100 (higher is better)
- Factor_i: Individual factor score (0.0 - 1.0)
- Weight_i: Factor weight (sum = 1.0)
```

### Factor Breakdown

| Factor | Weight | Description | Calculation |
|--------|--------|-------------|-------------|
| **Verification Status** | 30% | Has valid cryptographic certificate? | 1.0 if verified, 0.0 if not |
| **Security Audit Score** | 20% | Passed security audits? | (passed_audits / total_audits) |
| **Community Trust** | 15% | Average user rating | (avg_rating / 5.0) |
| **Uptime Percentage** | 15% | Historical uptime | (uptime_hours / total_hours) |
| **Incident History** | 10% | Security incidents (inverse) | 1.0 - (incidents / max_incidents) |
| **Compliance Score** | 5% | Regulatory compliance | (compliant_frameworks / total_frameworks) |
| **Activity Frequency** | 3% | Regular activity? | 1.0 if active last 7 days, decay otherwise |
| **Last Verified Date** | 2% | Recent verification? | 1.0 if <30 days, decay logarithmically |

---

## Implementation

### 1. Database Schema

```sql
-- Trust scores table (TimescaleDB for time-series)
CREATE TABLE trust_scores (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Overall score
    score           DECIMAL(5,2) NOT NULL CHECK (score >= 0 AND score <= 100),

    -- Individual factor scores
    verification_status   DECIMAL(3,2) CHECK (verification_status >= 0 AND verification_status <= 1),
    security_audit_score  DECIMAL(3,2) CHECK (security_audit_score >= 0 AND security_audit_score <= 1),
    community_trust       DECIMAL(3,2) CHECK (community_trust >= 0 AND community_trust <= 1),
    uptime_percentage     DECIMAL(3,2) CHECK (uptime_percentage >= 0 AND uptime_percentage <= 1),
    incident_history      DECIMAL(3,2) CHECK (incident_history >= 0 AND incident_history <= 1),
    compliance_score      DECIMAL(3,2) CHECK (compliance_score >= 0 AND compliance_score <= 1),
    activity_frequency    DECIMAL(3,2) CHECK (activity_frequency >= 0 AND activity_frequency <= 1),
    last_verified_date    DECIMAL(3,2) CHECK (last_verified_date >= 0 AND last_verified_date <= 1),

    -- Metadata
    calculated_at   TIMESTAMPTZ DEFAULT NOW(),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Convert to TimescaleDB hypertable for efficient time-series queries
SELECT create_hypertable('trust_scores', 'calculated_at');

-- Indexes for querying
CREATE INDEX idx_trust_scores_agent ON trust_scores(agent_id, calculated_at DESC);
CREATE INDEX idx_trust_scores_org ON trust_scores(organization_id, calculated_at DESC);
CREATE INDEX idx_trust_scores_score ON trust_scores(score DESC);
```

### 2. Go Implementation

```go
// internal/application/trust_service.go
package application

import (
    "context"
    "math"
    "time"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/domain/entities"
)

type TrustFactors struct {
    VerificationStatus  float64
    SecurityAuditScore  float64
    CommunityTrust      float64
    UptimePercentage    float64
    IncidentHistory     float64
    ComplianceScore     float64
    ActivityFrequency   float64
    LastVerifiedDate    float64
}

type TrustService struct {
    agentRepo       interfaces.AgentRepository
    auditRepo       interfaces.AuditRepository
    incidentRepo    interfaces.IncidentRepository
    trustScoreRepo  interfaces.TrustScoreRepository
}

// CalculateTrustScore computes the trust score for an agent
func (s *TrustService) CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (float64, error) {
    // 1. Fetch agent
    agent, err := s.agentRepo.GetByID(ctx, agentID)
    if err != nil {
        return 0, err
    }

    // 2. Calculate individual factors
    factors, err := s.calculateFactors(ctx, agent)
    if err != nil {
        return 0, err
    }

    // 3. Apply weighted formula
    score := 0.0
    score += factors.VerificationStatus * 0.30
    score += factors.SecurityAuditScore * 0.20
    score += factors.CommunityTrust * 0.15
    score += factors.UptimePercentage * 0.15
    score += factors.IncidentHistory * 0.10
    score += factors.ComplianceScore * 0.05
    score += factors.ActivityFrequency * 0.03
    score += factors.LastVerifiedDate * 0.02

    // 4. Convert to 0-100 scale
    finalScore := score * 100

    // 5. Store in database
    trustScore := &entities.TrustScore{
        AgentID:            agentID,
        OrganizationID:     agent.OrganizationID,
        Score:              finalScore,
        VerificationStatus: factors.VerificationStatus,
        SecurityAuditScore: factors.SecurityAuditScore,
        CommunityTrust:     factors.CommunityTrust,
        UptimePercentage:   factors.UptimePercentage,
        IncidentHistory:    factors.IncidentHistory,
        ComplianceScore:    factors.ComplianceScore,
        ActivityFrequency:  factors.ActivityFrequency,
        LastVerifiedDate:   factors.LastVerifiedDate,
        CalculatedAt:       time.Now(),
    }

    if err := s.trustScoreRepo.Create(ctx, trustScore); err != nil {
        return 0, err
    }

    return finalScore, nil
}

// calculateFactors computes individual factor scores
func (s *TrustService) calculateFactors(ctx context.Context, agent *entities.Agent) (*TrustFactors, error) {
    factors := &TrustFactors{}

    // Factor 1: Verification Status (30%)
    factors.VerificationStatus = s.calculateVerificationStatus(agent)

    // Factor 2: Security Audit Score (20%)
    auditScore, err := s.calculateSecurityAuditScore(ctx, agent.ID)
    if err != nil {
        return nil, err
    }
    factors.SecurityAuditScore = auditScore

    // Factor 3: Community Trust (15%)
    communityTrust, err := s.calculateCommunityTrust(ctx, agent.ID)
    if err != nil {
        return nil, err
    }
    factors.CommunityTrust = communityTrust

    // Factor 4: Uptime Percentage (15%)
    uptime, err := s.calculateUptimePercentage(ctx, agent.ID)
    if err != nil {
        return nil, err
    }
    factors.UptimePercentage = uptime

    // Factor 5: Incident History (10% - inverse)
    incidentScore, err := s.calculateIncidentHistory(ctx, agent.ID)
    if err != nil {
        return nil, err
    }
    factors.IncidentHistory = incidentScore

    // Factor 6: Compliance Score (5%)
    complianceScore, err := s.calculateComplianceScore(ctx, agent.ID)
    if err != nil {
        return nil, err
    }
    factors.ComplianceScore = complianceScore

    // Factor 7: Activity Frequency (3%)
    activityScore := s.calculateActivityFrequency(agent)
    factors.ActivityFrequency = activityScore

    // Factor 8: Last Verified Date (2%)
    verificationRecency := s.calculateVerificationRecency(agent)
    factors.LastVerifiedDate = verificationRecency

    return factors, nil
}

// Factor 1: Verification Status
func (s *TrustService) calculateVerificationStatus(agent *entities.Agent) float64 {
    if agent.LastVerifiedAt != nil {
        return 1.0 // Verified
    }
    return 0.0 // Not verified
}

// Factor 2: Security Audit Score
func (s *TrustService) calculateSecurityAuditScore(ctx context.Context, agentID uuid.UUID) (float64, error) {
    audits, err := s.auditRepo.GetSecurityAudits(ctx, agentID)
    if err != nil {
        return 0, err
    }

    if len(audits) == 0 {
        return 0.0 // No audits performed
    }

    passedAudits := 0
    for _, audit := range audits {
        if audit.Passed {
            passedAudits++
        }
    }

    return float64(passedAudits) / float64(len(audits)), nil
}

// Factor 3: Community Trust (average user rating)
func (s *TrustService) calculateCommunityTrust(ctx context.Context, agentID uuid.UUID) (float64, error) {
    avgRating, err := s.agentRepo.GetAverageRating(ctx, agentID)
    if err != nil {
        return 0, err
    }

    // Normalize to 0-1 scale (assuming 5-star rating)
    return avgRating / 5.0, nil
}

// Factor 4: Uptime Percentage
func (s *TrustService) calculateUptimePercentage(ctx context.Context, agentID uuid.UUID) (float64, error) {
    // Query last 30 days of uptime data
    uptime, err := s.agentRepo.GetUptimePercentage(ctx, agentID, 30*24*time.Hour)
    if err != nil {
        return 0, err
    }

    return uptime, nil
}

// Factor 5: Incident History (inverse - fewer incidents = higher score)
func (s *TrustService) calculateIncidentHistory(ctx context.Context, agentID uuid.UUID) (float64, error) {
    incidents, err := s.incidentRepo.GetIncidentCount(ctx, agentID, 90*24*time.Hour)
    if err != nil {
        return 0, err
    }

    // Inverse scoring (0 incidents = 1.0, 10+ incidents = 0.0)
    maxIncidents := 10.0
    if incidents >= int(maxIncidents) {
        return 0.0
    }

    return 1.0 - (float64(incidents) / maxIncidents), nil
}

// Factor 6: Compliance Score
func (s *TrustService) calculateComplianceScore(ctx context.Context, agentID uuid.UUID) (float64, error) {
    compliance, err := s.agentRepo.GetComplianceStatus(ctx, agentID)
    if err != nil {
        return 0, err
    }

    // Check compliance with frameworks (SOC 2, HIPAA, GDPR, etc.)
    totalFrameworks := 4.0 // SOC 2, HIPAA, GDPR, ISO 27001
    compliantFrameworks := 0.0

    if compliance.SOC2Compliant {
        compliantFrameworks++
    }
    if compliance.HIPAACompliant {
        compliantFrameworks++
    }
    if compliance.GDPRCompliant {
        compliantFrameworks++
    }
    if compliance.ISO27001Compliant {
        compliantFrameworks++
    }

    return compliantFrameworks / totalFrameworks, nil
}

// Factor 7: Activity Frequency
func (s *TrustService) calculateActivityFrequency(agent *entities.Agent) float64 {
    // Check if agent has been active in last 7 days
    sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

    if agent.UpdatedAt.After(sevenDaysAgo) {
        return 1.0 // Active recently
    }

    // Decay logarithmically based on inactivity
    daysSinceUpdate := time.Since(agent.UpdatedAt).Hours() / 24
    if daysSinceUpdate > 365 {
        return 0.0 // Inactive for over a year
    }

    // Logarithmic decay
    return 1.0 - (math.Log(daysSinceUpdate) / math.Log(365))
}

// Factor 8: Verification Recency
func (s *TrustService) calculateVerificationRecency(agent *entities.Agent) float64 {
    if agent.LastVerifiedAt == nil {
        return 0.0 // Never verified
    }

    daysSinceVerification := time.Since(*agent.LastVerifiedAt).Hours() / 24

    if daysSinceVerification <= 30 {
        return 1.0 // Verified within last 30 days
    }

    // Logarithmic decay (0.0 after 365 days)
    if daysSinceVerification >= 365 {
        return 0.0
    }

    return 1.0 - (math.Log(daysSinceVerification) / math.Log(365))
}
```

---

## Consequences

### Positive

1. **Transparency**:
   - Users see exactly why an agent has a certain score
   - Factor breakdown displayed in UI
   - Audit trail of score changes

2. **Fairness**:
   - All agents evaluated using same criteria
   - No bias in scoring algorithm
   - Weighted factors based on security importance

3. **Actionable**:
   - Developers know how to improve scores
   - Organizations can set minimum trust score policies
   - Security teams can identify risky agents

4. **Dynamic**:
   - Scores update as agent behavior changes
   - Historical trends visible
   - Automatic recalculation triggers

5. **Fast**:
   - Calculation <100ms (pre-computed factors cached)
   - No blocking API calls
   - Efficient database queries

### Negative

1. **Complexity**:
   - 8 factors to calculate and maintain
   - Requires data from multiple sources
   - Complex testing scenarios

2. **Data Requirements**:
   - Accurate uptime tracking needed
   - Incident reporting must be reliable
   - Community ratings can be gamed

3. **Tuning Needed**:
   - Weights may need adjustment over time
   - Factor calculations may evolve
   - Algorithm versioning required

### Mitigation

1. **Algorithm Versioning**:
   ```sql
   ALTER TABLE trust_scores ADD COLUMN algorithm_version INTEGER DEFAULT 1;
   ```

2. **A/B Testing**:
   - Test algorithm changes on subset of agents
   - Compare before/after scores
   - Validate with security team

3. **Gaming Prevention**:
   - Verified ratings only (no anonymous)
   - Rate limiting on ratings
   - Anomaly detection for suspicious patterns

---

## Alternatives Considered

### 1. Simple Binary (Verified / Not Verified)
**Rejected because**:
- Too simplistic for enterprise needs
- Doesn't differentiate between verified agents
- No incentive for continuous improvement

### 2. Pure ML Model (Black Box)
**Rejected because**:
- Not transparent (users don't know why)
- Difficult to debug and tune
- Requires large training dataset

### 3. Manual Curation
**Rejected because**:
- Doesn't scale to thousands of agents
- Subjective and inconsistent
- Slow to update

---

## References

- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [ISO/IEC 27001](https://www.iso.org/isoiec-27001-information-security.html)

---

**Last Updated**: October 6, 2025
**Related ADRs**: ADR-001 (Technology Stack), ADR-002 (Clean Architecture)
