# AIM Trust Scoring System

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Trust Score Fundamentals](#trust-score-fundamentals)
3. [8-Factor Trust Algorithm](#8-factor-trust-algorithm)
4. [Score Calculation](#score-calculation)
5. [Trust Score Lifecycle](#trust-score-lifecycle)
6. [Security Implications](#security-implications)
7. [Monitoring & Alerts](#monitoring--alerts)
8. [Best Practices](#best-practices)

---

## Overview

The AIM Trust Scoring System is a **real-time risk assessment mechanism** that assigns a numerical score (0-100) to each AI agent and MCP server based on their behavior, verification history, and security posture. Trust scores are used to:

- **Identify compromised agents** through sudden score drops
- **Enforce security policies** based on trust thresholds
- **Guide access control decisions** for high-risk operations
- **Provide transparency** into agent security posture
- **Enable predictive security** through ML-powered trend analysis

### Key Features

- **Real-Time Updates**: Scores recalculate on every significant event
- **8-Factor Algorithm**: Comprehensive assessment across multiple dimensions
- **Historical Tracking**: Complete audit trail of score changes
- **Automated Alerts**: Proactive notifications on score drops
- **ML-Powered**: Machine learning detects anomalies and trends

---

## Trust Score Fundamentals

### Score Range

```
0-100 Scale (Higher is better)

90-100: Excellent - Highly trusted, minimal restrictions
75-89:  Good - Trusted with routine monitoring
50-74:  Fair - Moderate risk, enhanced monitoring
25-49:  Poor - High risk, restricted access
0-24:   Critical - Severe risk, immediate review required
```

### Initial Trust Score

When an agent is first registered:
- **Starting Score**: 50 (neutral)
- **Increases**: Through successful verifications
- **Decreases**: Through failures or suspicious activity
- **Stabilizes**: After 10+ verification events (learning phase)

### Trust Score Drift

**Normal Drift**:
- ±5 points: Expected variation from routine operations
- Gradual changes: Natural score evolution over time

**Anomalous Drift**:
- >10 points in < 1 hour: Potential security incident
- >20 points in < 24 hours: Critical security alert
- Sudden drops: Trigger automated threat detection

---

## 8-Factor Trust Algorithm

The AIM trust score is calculated using 8 weighted factors:

| Factor | Weight | Description |
|--------|--------|-------------|
| **Verification History** | 25% | Success rate and consistency of cryptographic verifications |
| **Activity Patterns** | 15% | Behavioral analysis and anomaly detection |
| **Age & Stability** | 10% | Time since registration and uptime history |
| **Failure Rate** | 20% | Recent verification failures and error patterns |
| **Security Events** | 15% | Triggered security alerts and threat detections |
| **Compliance Score** | 5% | Adherence to security policies and best practices |
| **MCP Connections** | 5% | Trust scores of connected MCP servers |
| **User Actions** | 5% | Manual trust adjustments by administrators |

---

## Score Calculation

### Factor 1: Verification History (25%)

**What it measures**: The agent's cryptographic verification track record.

**Calculation**:
```go
func calculateVerificationScore(agent *Agent) float64 {
    if agent.TotalVerifications == 0 {
        return 50.0 // Neutral for new agents
    }

    successRate := float64(agent.SuccessfulVerifications) / float64(agent.TotalVerifications)

    // Recent verifications weighted more heavily
    recentSuccessRate := calculateRecentSuccessRate(agent, 24 * time.Hour)

    // Combine all-time and recent success rates
    score := (successRate * 0.6) + (recentSuccessRate * 0.4)

    // Convert to 0-100 scale
    return score * 100
}
```

**Examples**:
- 100 successful verifications, 0 failures → 100/100 points
- 80 successful, 20 failures → 80/100 points
- No verifications yet → 50/100 points (neutral)

**Impact**:
- Each successful verification: +0.5 to +2 points
- Each failed verification: -2 to -5 points
- Consecutive failures: Exponential penalty (-10 to -20 points)

### Factor 2: Activity Patterns (15%)

**What it measures**: Behavioral consistency and anomaly detection.

**Calculation**:
```go
func calculateActivityScore(agent *Agent) float64 {
    patterns := analyzePatterns(agent)

    score := 100.0

    // Penalize anomalies
    if patterns.UnusualTimeActivity {
        score -= 15.0 // Activity outside normal hours
    }

    if patterns.SpikeInRequests {
        score -= 20.0 // Sudden increase in API calls
    }

    if patterns.UnusualGeolocation {
        score -= 10.0 // Access from unexpected location
    }

    if patterns.SuspiciousActions {
        score -= 25.0 // Actions flagged as suspicious
    }

    return math.Max(score, 0)
}
```

**Monitored Patterns**:
- **Time-based**: Activity during unusual hours (e.g., 2 AM)
- **Frequency-based**: Spike in API requests (>100% increase)
- **Geolocation**: Access from new countries/IPs
- **Behavioral**: Actions deviating from historical patterns

**Examples**:
- Consistent 9-5 activity → 100/100 points
- Occasional weekend activity → 90/100 points
- Sudden 3 AM spike → 70/100 points
- Multiple anomalies → 40/100 points or lower

### Factor 3: Age & Stability (10%)

**What it measures**: Time since registration and continuous uptime.

**Calculation**:
```go
func calculateAgeScore(agent *Agent) float64 {
    daysSinceCreation := time.Since(agent.CreatedAt).Hours() / 24

    // Age component (0-50 points over 90 days)
    ageScore := math.Min(daysSinceCreation / 90.0 * 50, 50)

    // Stability component (0-50 points)
    uptimePercentage := calculateUptime(agent)
    stabilityScore := uptimePercentage * 50

    return ageScore + stabilityScore
}
```

**Age Curve**:
- 0-7 days: 10-20 points (new agent, limited trust)
- 8-30 days: 20-35 points (building history)
- 31-90 days: 35-50 points (established agent)
- 90+ days: 50 points (mature, trusted)

**Stability Metrics**:
- Uptime %: Percentage of time agent is responsive
- Continuous operation: No unexplained downtime
- Consistent availability: Regular verification checks pass

### Factor 4: Failure Rate (20%)

**What it measures**: Recent verification failures and error patterns.

**Calculation**:
```go
func calculateFailureScore(agent *Agent) float64 {
    recentFailures := getRecentFailures(agent, 7 * 24 * time.Hour) // Last 7 days

    if len(recentFailures) == 0 {
        return 100.0
    }

    // Base penalty for each failure
    score := 100.0 - (float64(len(recentFailures)) * 5.0)

    // Additional penalty for consecutive failures
    consecutiveFailures := countConsecutiveFailures(recentFailures)
    if consecutiveFailures >= 3 {
        score -= 20.0
    }

    // Time-based decay (older failures matter less)
    for _, failure := range recentFailures {
        age := time.Since(failure.Timestamp)
        if age < 24 * time.Hour {
            score -= 3.0 // Recent failures heavily penalized
        } else if age < 72 * time.Hour {
            score -= 2.0
        } else {
            score -= 1.0
        }
    }

    return math.Max(score, 0)
}
```

**Failure Types**:
- **Signature Failures**: Invalid Ed25519 signatures (-10 points each)
- **Timeout Failures**: Agent didn't respond to challenge (-5 points each)
- **Network Failures**: Connection issues (-3 points each)
- **Configuration Errors**: Misconfigured agent (-7 points each)

**Severity Levels**:
- 1-2 failures: Minor impact (-5 to -10 points)
- 3-5 failures: Moderate impact (-15 to -25 points)
- 6-10 failures: Major impact (-30 to -50 points)
- 10+ failures: Critical impact (-60+ points)

### Factor 5: Security Events (15%)

**What it measures**: Security threats and alerts triggered by the agent.

**Calculation**:
```go
func calculateSecurityScore(agent *Agent) float64 {
    threats := getSecurityThreats(agent, 30 * 24 * time.Hour) // Last 30 days

    score := 100.0

    for _, threat := range threats {
        switch threat.Severity {
        case "critical":
            score -= 30.0
        case "high":
            score -= 20.0
        case "medium":
            score -= 10.0
        case "low":
            score -= 5.0
        }

        // If threat is resolved, partial recovery
        if threat.IsResolved {
            score += (threat.PenaltyAmount * 0.3)
        }
    }

    return math.Max(score, 0)
}
```

**Security Event Types**:
- **Trust Score Drop**: Sudden decrease detected (-15 points)
- **Anomalous Behavior**: Actions flagged by ML model (-20 points)
- **Unauthorized Access**: Attempt to access restricted resources (-25 points)
- **Compliance Violation**: Policy breach detected (-10 points)

**Alert Severity Impact**:
- Low: -5 points (informational)
- Medium: -10 points (investigation needed)
- High: -20 points (immediate attention)
- Critical: -30 points (potential breach)

### Factor 6: Compliance Score (5%)

**What it measures**: Adherence to organizational security policies.

**Calculation**:
```go
func calculateComplianceScore(agent *Agent) float64 {
    policies := getApplicablePolicies(agent)

    compliantPolicies := 0
    for _, policy := range policies {
        if agent.MeetsPolicy(policy) {
            compliantPolicies++
        }
    }

    return (float64(compliantPolicies) / float64(len(policies))) * 100
}
```

**Compliance Checks**:
- **Verification Frequency**: Agent verified at least weekly (+100%)
- **API Key Rotation**: Keys rotated every 90 days (+100%)
- **Public Key Update**: Keys updated when needed (+100%)
- **Metadata Completeness**: All required fields populated (+100%)
- **Connection Security**: Only connects to verified MCP servers (+100%)

**Examples**:
- All policies met: 100/100 points
- 4 of 5 policies met: 80/100 points
- 2 of 5 policies met: 40/100 points

### Factor 7: MCP Connections (5%)

**What it measures**: Trust scores of connected MCP servers.

**Calculation**:
```go
func calculateMCPScore(agent *Agent) float64 {
    mcpServers := agent.GetConnectedMCPServers()

    if len(mcpServers) == 0 {
        return 50.0 // Neutral for agents without MCP connections
    }

    totalScore := 0.0
    for _, mcp := range mcpServers {
        totalScore += mcp.TrustScore
    }

    averageScore := totalScore / float64(len(mcpServers))

    // Weight by verification status
    verifiedCount := countVerifiedMCPs(mcpServers)
    verificationBonus := (float64(verifiedCount) / float64(len(mcpServers))) * 10

    return math.Min(averageScore + verificationBonus, 100)
}
```

**MCP Trust Influence**:
- Connected to high-trust MCPs (90+): Positive influence (+5 points)
- Connected to medium-trust MCPs (60-89): Neutral (no change)
- Connected to low-trust MCPs (<60): Negative influence (-10 points)
- Connected to unverified MCPs: Penalty (-5 points per unverified)

**Examples**:
- 3 MCPs all at 95 trust → 100/100 points
- 2 MCPs at 80, 1 at 60 → 73/100 points
- 1 MCP at 40 trust → 40/100 points

### Factor 8: User Actions (5%)

**What it measures**: Manual trust adjustments by administrators.

**Calculation**:
```go
func calculateUserActionScore(agent *Agent) float64 {
    adjustments := agent.GetTrustAdjustments()

    score := 50.0 // Start neutral

    for _, adjustment := range adjustments {
        // Only recent adjustments (last 90 days) count
        if time.Since(adjustment.Timestamp) < 90 * 24 * time.Hour {
            score += adjustment.Value
        }
    }

    return clamp(score, 0, 100)
}
```

**Admin Adjustments**:
- **Manual Increase**: Admin trusts agent more (+10 to +20 points)
- **Manual Decrease**: Admin distrusts agent (-10 to -30 points)
- **Temporary Boost**: Needed for critical operation (+5 for 24 hours)
- **Temporary Restriction**: Pending investigation (-15 for 7 days)

**Use Cases**:
- Newly deployed critical agent → Manual +15 boost
- Agent flagged by SOC team → Manual -20 penalty
- Post-incident recovery → Gradual manual increases

---

## Trust Score Lifecycle

### Phase 1: Registration (Day 0)

```
Initial State:
- Trust Score: 50 (neutral)
- Status: "New Agent"
- Verifications: 0
- Risk Level: Medium
```

**Recommendations**:
- Perform initial verification within 24 hours
- Monitor closely for first 7 days
- Limit access to non-critical resources

### Phase 2: Learning (Days 1-30)

```
Learning Phase:
- Trust Score: 50-75 (building trust)
- Status: "Learning"
- Verifications: 1-50
- Risk Level: Medium to Low
```

**Characteristics**:
- Trust score increases with successful verifications
- Behavioral patterns are being established
- Anomaly detection calibrating baselines

**Recommendations**:
- Weekly verifications minimum
- Review activity patterns weekly
- Grant incremental access based on trust score

### Phase 3: Established (Days 31-90)

```
Established Phase:
- Trust Score: 75-90 (trusted)
- Status: "Established"
- Verifications: 50-200
- Risk Level: Low
```

**Characteristics**:
- Consistent behavioral patterns
- Stable trust score (±3 points)
- Mature verification history

**Recommendations**:
- Monthly verification checks
- Automated monitoring sufficient
- Full access to appropriate resources

### Phase 4: Mature (Day 90+)

```
Mature Phase:
- Trust Score: 85-100 (highly trusted)
- Status: "Mature"
- Verifications: 200+
- Risk Level: Very Low
```

**Characteristics**:
- Long-term consistent behavior
- Minimal security events
- High compliance score

**Recommendations**:
- Quarterly comprehensive reviews
- Minimal manual oversight needed
- Trusted for critical operations

---

## Security Implications

### Trust-Based Access Control

Trust scores can enforce security policies:

```go
// Example: Restrict high-risk operations to high-trust agents
func canPerformCriticalOperation(agent *Agent) bool {
    return agent.TrustScore >= 85
}

// Example: Require additional verification for medium-trust agents
func requiresAdditionalVerification(agent *Agent) bool {
    return agent.TrustScore < 75
}

// Example: Block low-trust agents from sensitive resources
func canAccessSensitiveData(agent *Agent) bool {
    return agent.TrustScore >= 60
}
```

### Trust Thresholds

Recommended thresholds for different operations:

| Operation | Minimum Trust Score | Additional Requirements |
|-----------|---------------------|-------------------------|
| Read public data | 0 | None |
| Write non-critical data | 50 | Recent verification |
| Access sensitive data | 75 | 2FA enabled |
| Execute critical operations | 85 | Admin approval |
| Admin-level access | 95 | Manual approval + audit |

### Automated Responses

Based on trust score changes:

```go
// Automated security responses
func handleTrustScoreChange(agent *Agent, oldScore, newScore float64) {
    delta := newScore - oldScore

    // Critical drop
    if delta <= -20 {
        // Immediate actions
        suspendAgent(agent)
        alertSecurityTeam(agent, "Critical trust score drop")
        requireReAuthentication(agent)
    }

    // Significant drop
    if delta <= -10 {
        // Enhanced monitoring
        increaseVerificationFrequency(agent)
        flagForReview(agent)
        notifyAgentOwner(agent)
    }

    // Gradual improvement
    if delta >= 10 && newScore >= 85 {
        // Restore privileges
        restoreNormalAccess(agent)
        notifyAgentOwner(agent)
    }
}
```

---

## Monitoring & Alerts

### Trust Score Alerts

**Alert Triggers**:

| Trigger | Condition | Alert Level |
|---------|-----------|-------------|
| Critical Drop | Score drops >20 points in <1 hour | Critical |
| Significant Drop | Score drops >10 points in <6 hours | High |
| Below Threshold | Score falls below 50 | Medium |
| Consecutive Failures | 3+ verification failures | High |
| Security Event | Critical threat detected | Critical |

**Alert Actions**:
- Email notification to agent owner
- Slack message to security channel
- Dashboard notification
- SMS for critical alerts (optional)

### Dashboard Metrics

**Trust Score Dashboard** displays:
- Current trust score with historical trend
- Breakdown by 8 factors
- Recent score changes (last 7 days)
- Comparison to organization average
- Risk level indicator

**Example Dashboard View**:
```
Agent: production-agent-1
Current Trust Score: 87 / 100 (Trusted)

Factor Breakdown:
├─ Verification History: 92% (23 points)
├─ Activity Patterns: 88% (13 points)
├─ Age & Stability: 90% (9 points)
├─ Failure Rate: 85% (17 points)
├─ Security Events: 95% (14 points)
├─ Compliance Score: 100% (5 points)
├─ MCP Connections: 80% (4 points)
└─ User Actions: 40% (2 points)

Recent Changes:
- Oct 9: +2 (successful verification)
- Oct 8: -5 (verification timeout)
- Oct 7: +3 (improved compliance)

Risk Level: Low
Next Review: Oct 14, 2025
```

---

## Best Practices

### For Developers

1. **Verify Regularly**: Aim for weekly verifications minimum
2. **Monitor Trust Score**: Set up alerts for drops >10 points
3. **Maintain Compliance**: Keep all policies satisfied
4. **Update Keys**: Rotate keys every 90 days
5. **Review Connections**: Ensure MCP servers are trusted

### For Security Teams

1. **Set Thresholds**: Define trust score requirements for operations
2. **Investigate Drops**: Review any drop >15 points
3. **Trend Analysis**: Monitor trust score trends across all agents
4. **Baseline Behavior**: Establish normal patterns for each agent
5. **Incident Response**: Have playbooks for low-trust agents

### For Administrators

1. **Regular Reviews**: Quarterly comprehensive agent reviews
2. **Policy Updates**: Keep security policies up to date
3. **Manual Adjustments**: Use sparingly and document reasons
4. **Compliance Audits**: Monthly compliance score reviews
5. **Reporting**: Generate monthly trust score reports

---

## API Endpoints

### Get Trust Score

```bash
GET /api/v1/agents/{id}/trust-score

Response:
{
  "agent_id": "uuid",
  "trust_score": 87.5,
  "last_updated": "2025-10-10T14:30:00Z",
  "factors": {
    "verification_history": 92.0,
    "activity_patterns": 88.0,
    "age_stability": 90.0,
    "failure_rate": 85.0,
    "security_events": 95.0,
    "compliance_score": 100.0,
    "mcp_connections": 80.0,
    "user_actions": 40.0
  },
  "risk_level": "low"
}
```

### Get Trust Score History

```bash
GET /api/v1/agents/{id}/trust-score/history?days=30

Response:
{
  "agent_id": "uuid",
  "history": [
    {
      "timestamp": "2025-10-10T14:30:00Z",
      "score": 87.5,
      "change": 2.0,
      "reason": "Successful verification"
    },
    {
      "timestamp": "2025-10-09T10:15:00Z",
      "score": 85.5,
      "change": -5.0,
      "reason": "Verification timeout"
    }
  ]
}
```

---

## References

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [SECURITY.md](SECURITY.md) - Security best practices
- [API Documentation](API.md) - Complete API reference

---

**Maintained by**: OpenA2A Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026
