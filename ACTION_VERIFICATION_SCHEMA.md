# Action Verification Schema - The Black Box for AI Agents

## Vision

Capture **every action** an agent takes with full context to enable:
- Attack detection (agent doing things it shouldn't)
- Hallucination detection (agent behaving inconsistently)
- Compliance (complete audit trail)
- Research (first-ever real-world agent behavior dataset)

## Core Schema

```sql
-- Action Verifications Table (Time-Series Optimized)
CREATE TABLE action_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- WHO
    agent_id UUID NOT NULL REFERENCES agents(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),

    -- WHAT (The Action Triangle)
    action TEXT NOT NULL,  -- e.g., 'read_file', 'api_call', 'database_write', 'send_email'
    resource TEXT NOT NULL,  -- e.g., '/etc/passwd', 'stripe.com/api/charges', 'users_table'
    context JSONB NOT NULL,  -- { reason: "...", user_id: "...", request_id: "...", metadata: {...} }

    -- WHEN
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- DECISION
    allowed BOOLEAN NOT NULL,  -- Did we allow this action?
    risk_level TEXT NOT NULL DEFAULT 'low',  -- 'low', 'medium', 'high', 'critical'
    decision_reason TEXT,  -- Why was it allowed/blocked?

    -- TRUST CONTEXT
    trust_score_before DECIMAL(5,2),
    trust_score_after DECIMAL(5,2),
    trust_score_delta DECIMAL(5,2) GENERATED ALWAYS AS (trust_score_after - trust_score_before) STORED,

    -- ANOMALY DETECTION
    is_anomaly BOOLEAN DEFAULT FALSE,
    anomaly_score DECIMAL(5,2),  -- 0.0 = normal, 1.0 = highly anomalous
    anomaly_reasons TEXT[],  -- ['new_resource', 'unusual_time', 'high_frequency']
    baseline_deviation DECIMAL(5,2),  -- How far from agent's normal behavior?

    -- PERFORMANCE
    verification_duration_ms INTEGER,  -- How long did verification take?

    -- METADATA
    sdk_version TEXT,
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for High-Performance Queries
CREATE INDEX idx_action_verifications_agent_timestamp
    ON action_verifications (agent_id, timestamp DESC);

CREATE INDEX idx_action_verifications_anomalies
    ON action_verifications (agent_id, timestamp DESC)
    WHERE is_anomaly = TRUE;

CREATE INDEX idx_action_verifications_action_resource
    ON action_verifications (action, resource);

CREATE INDEX idx_action_verifications_risk_level
    ON action_verifications (risk_level, timestamp DESC)
    WHERE risk_level IN ('high', 'critical');

-- GIN index for fast JSONB context queries
CREATE INDEX idx_action_verifications_context_gin
    ON action_verifications USING GIN (context);

-- TimescaleDB Hypertable (for time-series optimization)
SELECT create_hypertable('action_verifications', 'timestamp', if_not_exists => TRUE);

-- Data Retention Policy (keep raw data for 90 days, aggregates forever)
SELECT add_retention_policy('action_verifications', INTERVAL '90 days');
```

## Behavioral Baselines (Per Agent)

```sql
-- Agent Behavioral Baseline Table
CREATE TABLE agent_behavioral_baselines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),

    -- Timeframe
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,

    -- Action Patterns
    common_actions JSONB,  -- { "read_file": 0.45, "api_call": 0.35, "database_read": 0.20 }
    common_resources JSONB,  -- { "/var/log/app.log": 0.60, "api.stripe.com": 0.25 }
    common_action_sequences TEXT[],  -- ['read_config,decrypt_key,access_db']

    -- Frequency Patterns
    avg_actions_per_hour DECIMAL(10,2),
    peak_hours INTEGER[],  -- [9, 10, 11, 14, 15] (active hours in UTC)

    -- Resource Access Patterns
    resource_count INTEGER,  -- How many unique resources accessed
    resource_diversity DECIMAL(5,2),  -- Entropy measure of resource access

    -- Risk Patterns
    avg_risk_score DECIMAL(5,2),
    high_risk_action_rate DECIMAL(5,2),

    -- Statistical Bounds (for anomaly detection)
    actions_per_hour_mean DECIMAL(10,2),
    actions_per_hour_stddev DECIMAL(10,2),

    -- Metadata
    sample_size INTEGER,  -- How many actions in this baseline
    confidence_score DECIMAL(5,2),  -- How confident are we in this baseline?

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Ensure one baseline per agent per time period
    UNIQUE(agent_id, start_date, end_date)
);

CREATE INDEX idx_agent_baselines_agent ON agent_behavioral_baselines (agent_id);
```

## Anomaly Detection Rules

```sql
-- Anomaly Detection Rules Table
CREATE TABLE anomaly_detection_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name TEXT NOT NULL,
    description TEXT,

    -- Rule Type
    rule_type TEXT NOT NULL,  -- 'new_resource', 'unusual_frequency', 'risky_action', 'context_mismatch'

    -- Conditions
    conditions JSONB NOT NULL,  -- Rule-specific conditions

    -- Severity
    severity TEXT NOT NULL,  -- 'low', 'medium', 'high', 'critical'

    -- Actions
    auto_block BOOLEAN DEFAULT FALSE,  -- Automatically block if triggered?
    alert_admins BOOLEAN DEFAULT TRUE,

    -- Status
    enabled BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Example Rules
INSERT INTO anomaly_detection_rules (name, description, rule_type, conditions, severity, auto_block) VALUES
(
    'First-Time Resource Access',
    'Agent accessing a resource for the first time',
    'new_resource',
    '{"lookback_days": 30}'::jsonb,
    'medium',
    FALSE
),
(
    'High-Frequency Spike',
    'Agent performing 10x more actions than usual',
    'unusual_frequency',
    '{"threshold_multiplier": 10, "time_window_minutes": 60}'::jsonb,
    'high',
    TRUE
),
(
    'Sensitive Resource Access',
    'Agent accessing credentials, keys, or PII',
    'risky_action',
    '{"resource_patterns": ["/credentials", "/keys", "/secrets", "*password*"]}'::jsonb,
    'critical',
    TRUE
);
```

## Query Examples

### 1. Get Agent's Action History
```sql
SELECT
    timestamp,
    action,
    resource,
    context->>'reason' as reason,
    allowed,
    risk_level,
    is_anomaly
FROM action_verifications
WHERE agent_id = 'agent-123'
ORDER BY timestamp DESC
LIMIT 100;
```

### 2. Detect Anomalies in Real-Time
```sql
-- Is this action anomalous for this agent?
WITH agent_baseline AS (
    SELECT
        common_actions,
        common_resources,
        actions_per_hour_mean,
        actions_per_hour_stddev
    FROM agent_behavioral_baselines
    WHERE agent_id = 'agent-123'
    ORDER BY end_date DESC
    LIMIT 1
)
SELECT
    CASE
        -- New resource?
        WHEN NOT (baseline.common_resources ? 'new-resource-path') THEN TRUE
        -- Unusual frequency?
        WHEN recent_action_count > (baseline.actions_per_hour_mean + 3 * baseline.actions_per_hour_stddev) THEN TRUE
        ELSE FALSE
    END as is_anomalous
FROM agent_baseline baseline;
```

### 3. Agent Behavior Over Time
```sql
SELECT
    DATE_TRUNC('day', timestamp) as day,
    COUNT(*) as total_actions,
    COUNT(DISTINCT action) as unique_actions,
    COUNT(DISTINCT resource) as unique_resources,
    AVG(CASE WHEN is_anomaly THEN 1 ELSE 0 END) * 100 as anomaly_rate_pct,
    AVG(trust_score_after) as avg_trust_score
FROM action_verifications
WHERE agent_id = 'agent-123'
  AND timestamp > NOW() - INTERVAL '30 days'
GROUP BY day
ORDER BY day;
```

### 4. Find Agents Behaving Suspiciously
```sql
-- Agents with high anomaly rates in last 24 hours
SELECT
    a.id,
    a.name,
    COUNT(*) as total_actions,
    COUNT(*) FILTER (WHERE av.is_anomaly) as anomaly_count,
    (COUNT(*) FILTER (WHERE av.is_anomaly)::DECIMAL / COUNT(*)) * 100 as anomaly_rate_pct,
    COUNT(*) FILTER (WHERE av.risk_level = 'critical') as critical_actions
FROM agents a
JOIN action_verifications av ON av.agent_id = a.id
WHERE av.timestamp > NOW() - INTERVAL '24 hours'
GROUP BY a.id, a.name
HAVING (COUNT(*) FILTER (WHERE av.is_anomaly)::DECIMAL / COUNT(*)) > 0.20  -- >20% anomaly rate
ORDER BY anomaly_rate_pct DESC;
```

### 5. Action Sequence Analysis (Attack Chains)
```sql
-- Find common action sequences (e.g., read_config → decrypt_key → access_db)
WITH action_sequences AS (
    SELECT
        agent_id,
        action,
        LAG(action, 1) OVER (PARTITION BY agent_id ORDER BY timestamp) as prev_action_1,
        LAG(action, 2) OVER (PARTITION BY agent_id ORDER BY timestamp) as prev_action_2
    FROM action_verifications
    WHERE timestamp > NOW() - INTERVAL '7 days'
)
SELECT
    CONCAT(prev_action_2, ' → ', prev_action_1, ' → ', action) as sequence,
    COUNT(*) as occurrence_count
FROM action_sequences
WHERE prev_action_2 IS NOT NULL
GROUP BY sequence
ORDER BY occurrence_count DESC
LIMIT 20;
```

## API Endpoints Needed

### 1. Record Action Verification (SDK calls this)
```
POST /api/v1/agents/{agent_id}/verify-action
{
  "action": "read_file",
  "resource": "/etc/passwd",
  "context": {
    "reason": "Reading system config for monitoring",
    "user_id": "user-123",
    "request_id": "req-abc"
  }
}

Response:
{
  "allowed": false,
  "risk_level": "critical",
  "reason": "Sensitive system file access detected",
  "trust_score_impact": -0.15
}
```

### 2. Get Agent Behavior Analytics
```
GET /api/v1/agents/{agent_id}/behavior-analytics?days=30

Response:
{
  "agent_id": "agent-123",
  "timeframe": { "start": "2025-01-01", "end": "2025-01-30" },
  "total_actions": 15420,
  "unique_actions": 12,
  "unique_resources": 45,
  "anomaly_rate": 0.02,
  "avg_trust_score": 0.87,
  "most_common_actions": [
    { "action": "read_file", "count": 8500, "percentage": 55.1 },
    { "action": "api_call", "count": 4200, "percentage": 27.2 }
  ],
  "risk_distribution": {
    "low": 98.1,
    "medium": 1.5,
    "high": 0.3,
    "critical": 0.1
  }
}
```

### 3. Get Anomalies
```
GET /api/v1/agents/{agent_id}/anomalies?days=7

Response:
{
  "agent_id": "agent-123",
  "anomaly_count": 12,
  "anomalies": [
    {
      "id": "anom-1",
      "timestamp": "2025-01-15T14:30:00Z",
      "action": "read_database",
      "resource": "user_credentials",
      "context": { "reason": "Data export" },
      "anomaly_score": 0.95,
      "anomaly_reasons": ["new_resource", "sensitive_data"],
      "was_blocked": true
    }
  ]
}
```

## Enforcement Strategy

### Option 1: SDK Wrapper (Easy)
```python
from aim_sdk import secure, AIMClient

# Method 1: One-line security (automatic verification)
secure(agent_id="agent-123", private_key=os.getenv("AIM_PRIVATE_KEY"))

# Now all file operations are automatically verified!
with open('/etc/passwd', 'r') as f:  # AIM intercepts this!
    data = f.read()
```

### Option 2: Explicit Verification (Full Control)
```python
client = AIMClient(agent_id="agent-123", private_key=os.getenv("AIM_PRIVATE_KEY"))

# Verify before action
result = client.verify_action(
    action="read_file",
    resource="/etc/passwd",
    context={"reason": "System monitoring", "user_id": "user-123"}
)

if result.allowed:
    with open('/etc/passwd', 'r') as f:
        data = f.read()
else:
    raise PermissionError(f"Action blocked: {result.reason}")
```

### Option 3: Framework Integration (Enterprise)
```python
# Built into LangChain, CrewAI, AutoGPT, etc.
from langchain import Agent
from aim_sdk.integrations.langchain import AIMSecurityWrapper

agent = Agent(...)
agent = AIMSecurityWrapper(agent, agent_id="agent-123")

# All agent actions now automatically verified!
```

## Value Propositions

### For Security Teams
- **"See exactly what your agents do and why"**
- Detect compromised agents before damage occurs
- Complete audit trail for incident response

### For AI Teams
- **"Understand agent drift and hallucinations"**
- Measure behavioral consistency over time
- Debug why agents fail

### For Compliance Teams
- **"SOC 2, HIPAA, GDPR compliance built-in"**
- Automated audit trails
- Prove agent behavior to auditors

### For Researchers
- **"First-ever real-world agent behavior dataset"**
- Study failure modes, attack patterns
- Publish papers on agent safety

### For Insurance
- **"Agent liability coverage requires this"**
- Prove agent was/wasn't compromised
- Reduce insurance premiums

## Next Steps

1. **Phase 1** (Now): Create action_verifications table
2. **Phase 2** (Week 1): Build SDK verify_action() function
3. **Phase 3** (Week 2): Build behavioral baseline calculation
4. **Phase 4** (Week 3): Build anomaly detection engine
5. **Phase 5** (Week 4): Build analytics dashboard
6. **Phase 6** (Month 2): ML-based predictive anomaly detection

---

**This is AIM's killer feature. The "Black Box for AI Agents."**
