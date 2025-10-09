# 🔐 Capability-Based Access Control & Behavioral Verification
**Date**: October 7, 2025
**Status**: ⚠️ **CRITICAL GAPS IDENTIFIED - ENTERPRISE BLOCKER**

---

## 🚨 Problem Statement

**AIM is currently missing foundational enterprise features** that prevent real-world adoption:

1. **No Capability Registration** - Agents register without declaring what they CAN do
2. **No Permission Verification** - Actions are not matched against declared capabilities
3. **No Behavioral Anomaly Detection** - No ML-based scoring for out-of-bounds behavior
4. **No Communication Mapping** - No knowledge of who agents talk to and what they say
5. **No Governance Framework** - Missing compliance and audit trails for capability enforcement

### Why This Matters

**Enterprises cannot adopt AI agents without these features because**:
- **Compliance**: SOC 2, HIPAA, GDPR require proving what agents CAN and CANNOT do
- **Security**: Compromised agents must be detectable when they act outside capabilities
- **Governance**: Security teams need alerts when agents attempt unauthorized actions
- **Trust**: Organizations need evidence that agents respect boundaries

---

## 🎯 Vision: AIM as the Industry Standard

**"If an agent or MCP becomes compromised, AIM should detect it and take action."**

AIM should be the **definitive platform that connects everything regarding agents and MCPs** in a world where there are fragmented frameworks for trusting AI agents and ensuring MCPs are secured.

### Key Differentiators

1. **Capability Registry** - Central source of truth for what each agent can do
2. **Behavioral Scoring** - ML-powered algorithms that detect capability violations
3. **Policy Enforcement** - Real-time allow/deny decisions based on capability match
4. **Communication Graph** - Who talks to whom, what permissions they need
5. **Incident Response** - Automated alerts and actions when anomalies detected

---

## 📊 Current State vs. Desired State

### What We Have Now ❌

```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-agent",
  "status": "verified",
  "trust_score": 75,
  "public_key": "base64-key",
  "registered_at": "2025-10-07T00:00:00Z"
}
```

**Problem**: No information about what this agent is ALLOWED to do.

### What We Need ✅

```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "customer-support-bot",
  "status": "verified",
  "trust_score": 75,
  "public_key": "base64-key",

  // ✅ NEW: Capability Declaration
  "capabilities": {
    "declared": [
      {
        "action_type": "read_database",
        "resources": ["customers", "orders"],
        "permissions": ["SELECT"],
        "risk_level": "medium",
        "purpose": "Customer support queries"
      },
      {
        "action_type": "send_email",
        "resources": ["customer_notifications"],
        "permissions": ["SEND"],
        "risk_level": "low",
        "purpose": "Order confirmations and support responses"
      }
    ],
    "prohibited": [
      {
        "action_type": "delete_database",
        "reason": "Customer support bot has no delete permissions"
      },
      {
        "action_type": "send_email",
        "resources": ["internal_team"],
        "reason": "Cannot send internal emails"
      }
    ]
  },

  // ✅ NEW: Communication Mapping
  "communication": {
    "talks_to": [
      {
        "service": "stripe_api",
        "purpose": "Process refunds",
        "data_shared": ["customer_id", "order_id", "amount"],
        "frequency": "on_demand"
      },
      {
        "service": "sendgrid_api",
        "purpose": "Send customer emails",
        "data_shared": ["customer_email", "message_body"],
        "frequency": "frequent"
      }
    ]
  },

  // ✅ NEW: Behavioral Baseline
  "behavioral_baseline": {
    "typical_actions_per_day": 150,
    "typical_action_types": ["read_database", "send_email"],
    "typical_resources": ["customers", "orders", "customer_notifications"],
    "typical_times": {
      "peak_hours": [9, 10, 11, 14, 15, 16],
      "off_hours_expected": false
    }
  },

  "registered_at": "2025-10-07T00:00:00Z"
}
```

---

## 🏗️ Architecture: Capability-Based Access Control

### 1. Registration Phase - Capability Declaration

**When an agent registers**, it MUST declare:

```python
from aim_sdk import register_agent

agent = register_agent(
    name="customer-support-bot",
    aim_url="https://aim.company.com",

    # ✅ NEW: Capability Declaration
    capabilities=[
        {
            "action_type": "read_database",
            "resources": ["customers", "orders"],
            "permissions": ["SELECT"],
            "risk_level": "medium",
            "purpose": "Customer support queries",
            "max_frequency": "1000/hour"
        },
        {
            "action_type": "send_email",
            "resources": ["customer_notifications"],
            "permissions": ["SEND"],
            "risk_level": "low",
            "purpose": "Order confirmations",
            "max_frequency": "500/hour"
        }
    ],

    # ✅ NEW: Communication Mapping
    talks_to=[
        {
            "service": "stripe_api",
            "purpose": "Process refunds",
            "data_shared": ["customer_id", "order_id", "amount"],
            "requires_approval": True  # High-risk external API
        },
        {
            "service": "sendgrid_api",
            "purpose": "Send customer emails",
            "data_shared": ["customer_email", "message_body"],
            "requires_approval": False  # Low-risk, approved service
        }
    ]
)
```

### 2. Verification Phase - Capability Matching

**When an agent requests verification**, AIM performs:

```python
# Agent requests verification
verification = aim_client.verify_action(
    action_type="delete_database",  # ⚠️ NOT in declared capabilities!
    resource="customers",
    context={"reason": "cleanup"}
)

# ✅ AIM's NEW Capability Matching Algorithm:
# 1. Check if action_type is in declared capabilities
# 2. Check if resource is in allowed resources for this action
# 3. Check if frequency is within limits
# 4. Check if time of day is expected
# 5. Calculate behavioral anomaly score
# 6. Make policy decision (allow/deny/alert)

# ❌ Result: DENIED + ALERT
{
    "verified": False,
    "status": "denied",
    "reason": "Action 'delete_database' not in declared capabilities",
    "alert_created": True,
    "alert_id": "alert-123",
    "severity": "high",
    "recommended_action": "Investigate potential compromise"
}
```

### 3. Monitoring Phase - Behavioral Scoring

**AIM continuously monitors** and scores agent behavior:

```python
# ✅ NEW: Behavioral Anomaly Detection

def calculate_behavioral_score(agent, action):
    """
    ML-powered scoring algorithm that detects anomalies.

    Factors:
    1. Capability Match Score (0-100)
       - Is action in declared capabilities? (+50)
       - Is resource in allowed resources? (+30)
       - Is permission level allowed? (+20)

    2. Frequency Anomaly Score (0-100)
       - Is frequency within normal range? (+50)
       - Is this a spike compared to baseline? (-50 if spike)

    3. Temporal Anomaly Score (0-100)
       - Is this a normal time for this agent? (+50)
       - Off-hours activity for daytime-only agent? (-50)

    4. Communication Anomaly Score (0-100)
       - Is agent talking to expected services? (+50)
       - Is agent talking to NEW, unknown services? (-50)

    5. Historical Trust Score (0-100)
       - Based on past successful verifications
       - Decays when anomalies detected

    Final Score = weighted_average([
        (capability_match, 0.30),
        (frequency_anomaly, 0.20),
        (temporal_anomaly, 0.15),
        (communication_anomaly, 0.20),
        (historical_trust, 0.15)
    ])
    """
    pass
```

### 4. Policy Enforcement - Real-Time Decisions

**AIM makes real-time allow/deny decisions**:

```python
# ✅ NEW: Policy Engine

class PolicyEngine:
    def evaluate(self, agent, action, behavioral_score):
        """
        Policy Decision Matrix:

        | Behavioral Score | Capability Match | Decision       | Action          |
        |------------------|------------------|----------------|-----------------|
        | > 80             | Yes              | ALLOW          | None            |
        | 60-80            | Yes              | ALLOW + WARN   | Low alert       |
        | < 60             | Yes              | DENY + ALERT   | Medium alert    |
        | Any              | No               | DENY + ALERT   | High alert      |
        """

        # Check capability match
        capability_match = self.check_capability(agent, action)

        if not capability_match:
            # ❌ Action not in capabilities - IMMEDIATE DENIAL
            return {
                "decision": "DENY",
                "reason": "Action not in declared capabilities",
                "alert": {
                    "severity": "high",
                    "message": f"Agent {agent.name} attempted unauthorized action: {action.type}",
                    "recommended_action": "Investigate potential compromise"
                }
            }

        if behavioral_score < 60:
            # ❌ Low behavioral score - DENY + ALERT
            return {
                "decision": "DENY",
                "reason": "Behavioral anomaly detected",
                "alert": {
                    "severity": "medium",
                    "message": f"Agent {agent.name} behavior anomaly (score: {behavioral_score})",
                    "recommended_action": "Review agent activity logs"
                }
            }

        if behavioral_score < 80:
            # ⚠️ Moderate score - ALLOW but WARN
            return {
                "decision": "ALLOW",
                "alert": {
                    "severity": "low",
                    "message": f"Agent {agent.name} moderate behavioral score: {behavioral_score}",
                    "recommended_action": "Monitor closely"
                }
            }

        # ✅ High score + capability match - ALLOW
        return {
            "decision": "ALLOW"
        }
```

---

## 🗄️ Database Schema Changes

### New Tables Needed

#### 1. `agent_capabilities`
```sql
CREATE TABLE agent_capabilities (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(id),
    action_type VARCHAR(100) NOT NULL,
    resources TEXT[],  -- Array of allowed resource names
    permissions TEXT[],  -- Array of permission levels (SELECT, INSERT, UPDATE, DELETE, etc.)
    risk_level VARCHAR(20),  -- low, medium, high, critical
    purpose TEXT,  -- Human-readable purpose
    max_frequency VARCHAR(50),  -- "1000/hour", "100/day", etc.
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(agent_id, action_type, resources)  -- Prevent duplicates
);
```

#### 2. `agent_communications`
```sql
CREATE TABLE agent_communications (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(id),
    service_name VARCHAR(200) NOT NULL,
    purpose TEXT,
    data_shared TEXT[],  -- Array of data fields shared
    requires_approval BOOLEAN DEFAULT FALSE,
    frequency_estimate VARCHAR(50),  -- "frequent", "occasional", "rare"
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(agent_id, service_name)
);
```

#### 3. `behavioral_baselines`
```sql
CREATE TABLE behavioral_baselines (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL UNIQUE REFERENCES agents(id),
    typical_actions_per_day INTEGER,
    typical_action_types TEXT[],
    typical_resources TEXT[],
    typical_peak_hours INTEGER[],  -- Array of hours (0-23)
    off_hours_expected BOOLEAN DEFAULT FALSE,
    last_calculated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

#### 4. `capability_violations`
```sql
CREATE TABLE capability_violations (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(id),
    verification_event_id UUID REFERENCES verification_events(id),
    action_type VARCHAR(100),
    resource VARCHAR(255),
    violation_type VARCHAR(50),  -- "undeclared_action", "unauthorized_resource", "frequency_limit", etc.
    behavioral_score DECIMAL(5,2),
    policy_decision VARCHAR(20),  -- "ALLOW", "DENY", "WARN"
    alert_created BOOLEAN,
    alert_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 🔢 Behavioral Scoring Algorithm

### Multi-Factor Scoring Model

```python
class BehavioralScorer:
    """
    8-Factor Trust Algorithm for Enterprise AI Governance

    This algorithm detects compromised agents by comparing real-time
    behavior against declared capabilities and historical baselines.
    """

    WEIGHTS = {
        "capability_match": 0.25,      # Is action in declared capabilities?
        "resource_match": 0.20,         # Is resource allowed for this action?
        "permission_match": 0.15,       # Does agent have required permissions?
        "frequency_anomaly": 0.15,      # Is frequency within normal range?
        "temporal_anomaly": 0.10,       # Is this a normal time for agent?
        "communication_anomaly": 0.05,  # Is agent talking to expected services?
        "historical_trust": 0.05,       # Track record of successful verifications
        "risk_adjustment": 0.05         # Adjust based on declared risk level
    }

    def calculate_score(self, agent, action, context):
        """Calculate behavioral score (0-100)."""

        scores = {}

        # 1. Capability Match (25%)
        scores["capability_match"] = self._check_capability_match(
            agent.capabilities,
            action.type
        )

        # 2. Resource Match (20%)
        scores["resource_match"] = self._check_resource_match(
            agent.capabilities,
            action.type,
            action.resource
        )

        # 3. Permission Match (15%)
        scores["permission_match"] = self._check_permission_match(
            agent.capabilities,
            action.type,
            action.permissions_required
        )

        # 4. Frequency Anomaly (15%)
        scores["frequency_anomaly"] = self._check_frequency(
            agent,
            action.type,
            context.recent_action_count
        )

        # 5. Temporal Anomaly (10%)
        scores["temporal_anomaly"] = self._check_temporal_pattern(
            agent.behavioral_baseline,
            context.current_hour,
            context.current_day_of_week
        )

        # 6. Communication Anomaly (5%)
        scores["communication_anomaly"] = self._check_communication_pattern(
            agent.communications,
            action.target_service
        )

        # 7. Historical Trust (5%)
        scores["historical_trust"] = self._get_historical_trust(agent)

        # 8. Risk Adjustment (5%)
        scores["risk_adjustment"] = self._adjust_for_risk(
            action.risk_level,
            scores
        )

        # Weighted average
        final_score = sum(
            scores[factor] * self.WEIGHTS[factor]
            for factor in self.WEIGHTS
        )

        return {
            "final_score": final_score,
            "component_scores": scores,
            "policy_decision": self._make_policy_decision(final_score, scores)
        }

    def _check_capability_match(self, capabilities, action_type):
        """Check if action is in declared capabilities."""
        for cap in capabilities:
            if cap.action_type == action_type:
                return 100  # Perfect match
        return 0  # Not declared

    def _check_resource_match(self, capabilities, action_type, resource):
        """Check if resource is allowed for this action."""
        for cap in capabilities:
            if cap.action_type == action_type:
                if resource in cap.resources or "*" in cap.resources:
                    return 100  # Allowed resource
                return 0  # Not allowed
        return 0  # Action not found

    def _check_frequency(self, agent, action_type, recent_count):
        """Check if frequency is within normal range."""
        baseline = agent.behavioral_baseline
        expected_count = baseline.typical_actions_per_day / 24  # Per hour

        if recent_count <= expected_count * 1.5:
            return 100  # Normal frequency
        elif recent_count <= expected_count * 3:
            return 50  # Elevated but acceptable
        else:
            return 0  # Anomalous spike

    # ... additional scoring methods
```

---

## 📡 API Enhancements

### 1. Register Agent with Capabilities

**New Registration Endpoint**:

```http
POST /api/v1/public/agents/register
Content-Type: application/json

{
  "name": "customer-support-bot",
  "display_name": "Customer Support Bot",
  "agent_type": "ai_agent",

  // ✅ NEW: Capability Declaration
  "capabilities": [
    {
      "action_type": "read_database",
      "resources": ["customers", "orders"],
      "permissions": ["SELECT"],
      "risk_level": "medium",
      "purpose": "Customer support queries",
      "max_frequency": "1000/hour"
    }
  ],

  // ✅ NEW: Communication Mapping
  "communications": [
    {
      "service": "stripe_api",
      "purpose": "Process refunds",
      "data_shared": ["customer_id", "order_id"],
      "requires_approval": true
    }
  ]
}
```

### 2. Verify Action with Capability Check

**Enhanced Verification Response**:

```json
{
  "id": "verification-123",
  "status": "approved",
  "trust_score": 85.5,

  // ✅ NEW: Behavioral Scoring
  "behavioral_score": {
    "final_score": 85.5,
    "component_scores": {
      "capability_match": 100,
      "resource_match": 100,
      "permission_match": 100,
      "frequency_anomaly": 80,
      "temporal_anomaly": 90,
      "communication_anomaly": 100,
      "historical_trust": 85,
      "risk_adjustment": 95
    }
  },

  // ✅ NEW: Policy Decision
  "policy": {
    "decision": "ALLOW",
    "reason": "All checks passed, score above threshold",
    "alert_created": false
  },

  "expires_at": "2025-10-08T00:00:00Z"
}
```

### 3. New Compliance Endpoints

```http
# Get agent capabilities
GET /api/v1/agents/{agent_id}/capabilities

# Get capability violations
GET /api/v1/agents/{agent_id}/violations?days=7

# Get behavioral baseline
GET /api/v1/agents/{agent_id}/behavioral-baseline

# Get communication graph
GET /api/v1/agents/{agent_id}/communications

# Generate compliance report
GET /api/v1/compliance/agent-capabilities-report?organization_id=xxx
```

---

## 🚨 Alerting & Incident Response

### Alert Types

```python
class AlertType(Enum):
    # Capability Violations
    UNDECLARED_ACTION = "undeclared_action"
    UNAUTHORIZED_RESOURCE = "unauthorized_resource"
    PERMISSION_VIOLATION = "permission_violation"

    # Behavioral Anomalies
    FREQUENCY_SPIKE = "frequency_spike"
    OFF_HOURS_ACTIVITY = "off_hours_activity"
    UNUSUAL_COMMUNICATION = "unusual_communication"

    # Security Events
    LOW_BEHAVIORAL_SCORE = "low_behavioral_score"
    REPEATED_VIOLATIONS = "repeated_violations"
    POTENTIAL_COMPROMISE = "potential_compromise"

class AlertSeverity(Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"
```

### Incident Response Workflow

```python
# When a violation is detected:
if behavioral_score < 60 or not capability_match:
    # 1. Create alert
    alert = create_alert(
        agent_id=agent.id,
        type=AlertType.UNDECLARED_ACTION,
        severity=AlertSeverity.HIGH,
        message=f"Agent attempted unauthorized action: {action.type}",
        metadata={
            "behavioral_score": behavioral_score,
            "action_type": action.type,
            "resource": action.resource
        }
    )

    # 2. Notify security team
    notify_security_team(alert)

    # 3. Optional: Auto-suspend agent
    if policy.auto_suspend_on_violation:
        suspend_agent(agent.id, reason="Capability violation detected")

    # 4. Create incident
    incident = create_incident(
        agent_id=agent.id,
        alert_id=alert.id,
        type="capability_violation",
        recommended_action="Investigate agent compromise"
    )
```

---

## 📊 Dashboard Enhancements

### New Dashboard Views

#### 1. Capability Compliance Dashboard
- **Agent Capability Matrix**: Table showing all agents and their declared capabilities
- **Violation Heatmap**: Visual of which agents have most violations
- **Risk Distribution**: Chart showing agents by risk level
- **Compliance Score**: Organization-wide capability compliance percentage

#### 2. Behavioral Monitoring Dashboard
- **Behavioral Score Trends**: Time-series graph of agent behavioral scores
- **Anomaly Timeline**: Timeline of detected anomalies
- **Frequency Analysis**: Charts showing action frequency vs. baseline
- **Temporal Patterns**: Heatmap of agent activity by hour/day

#### 3. Communication Graph Dashboard
- **Service Dependency Map**: Visual graph of agents and services they talk to
- **Data Flow Diagram**: Shows what data is shared with which services
- **Approval Status**: Which communications require approval
- **Unknown Services Alert**: Agents talking to undeclared services

---

## 🎯 Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- [ ] Database schema for capabilities, communications, baselines
- [ ] Update agent registration to accept capabilities
- [ ] Create capability storage and retrieval APIs
- [ ] Basic capability matching algorithm (exact match only)

### Phase 2: Behavioral Scoring (Week 3-4)
- [ ] Implement 8-factor behavioral scoring algorithm
- [ ] Create baseline calculation service
- [ ] Build frequency tracking system
- [ ] Temporal anomaly detection

### Phase 3: Policy Engine (Week 5-6)
- [ ] Build policy evaluation engine
- [ ] Implement alert creation system
- [ ] Create incident response workflows
- [ ] Add auto-suspension capabilities

### Phase 4: Dashboard & Reporting (Week 7-8)
- [ ] Capability compliance dashboard
- [ ] Behavioral monitoring dashboard
- [ ] Communication graph visualization
- [ ] Compliance report generation

### Phase 5: ML Enhancement (Week 9-10)
- [ ] Train ML models on historical verification data
- [ ] Implement anomaly detection algorithms
- [ ] Build adaptive baseline adjustment
- [ ] Create predictive threat scoring

---

## 🏆 Success Metrics

### Technical Metrics
- **Capability Coverage**: % of agents with declared capabilities (Target: 100%)
- **Violation Detection Rate**: % of unauthorized actions detected (Target: 100%)
- **False Positive Rate**: % of legitimate actions flagged (Target: <5%)
- **Alert Response Time**: Time to create alert after violation (Target: <1s)

### Business Metrics
- **Compliance Ready**: % of customers ready for SOC 2/HIPAA audit (Target: 100%)
- **Security Confidence**: % of security teams confident in agent governance (Target: >90%)
- **Incident Prevention**: # of compromises prevented by capability enforcement
- **Adoption Rate**: % of organizations using capability-based controls

---

## 💡 Innovative Features

### 1. Auto-Learning Baselines
- AIM automatically learns typical behavior over 30 days
- Adjusts baselines as agent usage evolves
- Detects gradual capability creep

### 2. Communication Approval Workflow
- High-risk external APIs require human approval before first use
- Approvals cached for 30 days for frequently used services
- Revocable at any time

### 3. Capability Templates
- Pre-built capability sets for common agent types
- "Customer Support Agent" template with standard capabilities
- "Data Analytics Agent" template with read-only database access
- Accelerates compliant agent development

### 4. Behavioral Fingerprinting
- Each agent has unique behavioral "fingerprint"
- Detects agent impersonation or credential theft
- ML-powered similarity scoring

---

## 🚀 Competitive Advantage

**Why This Makes AIM the Industry Standard:**

1. **First to Market**: No existing solution provides capability-based governance for AI agents
2. **Comprehensive**: Covers registration, verification, monitoring, and response
3. **ML-Powered**: Behavioral scoring adapts and learns from agent behavior
4. **Compliance-Ready**: Built for SOC 2, HIPAA, GDPR from day one
5. **Open Source**: Community can extend and audit the algorithms
6. **Enterprise-Grade**: Designed for 10,000+ agents in production

---

## 📝 Conclusion

**This enhancement transforms AIM from a basic identity verification system into a comprehensive AI governance platform.**

With capability-based access control and behavioral verification:
- ✅ Enterprises can prove compliance to auditors
- ✅ Security teams can detect compromised agents
- ✅ Developers can build agents with confidence
- ✅ Organizations can adopt AI at scale
- ✅ AIM becomes THE industry standard for AI agent governance

**Next Steps**: Prioritize Phase 1 implementation and begin building the foundation for enterprise AI governance.

---

**Last Updated**: October 7, 2025
**Author**: AIM Development Team
**Project**: Agent Identity Management (OpenA2A)
**Repository**: https://github.com/opena2a-org/agent-identity-management
