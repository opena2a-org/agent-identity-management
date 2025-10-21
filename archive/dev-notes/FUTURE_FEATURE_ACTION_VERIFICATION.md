# AIM Future Feature: Action Verification with Basic Context Capture

**Status**: Planned for Future Release
**Priority**: High (Foundation for Premium Products)
**Estimated Effort**: 3-4 weeks
**Dependencies**: None (can be developed independently)

---

## 1. Feature Overview

### Purpose
Enable AIM to capture and verify every action an agent takes, providing:
- **Security**: Detect anomalous behavior (new resources, unusual frequency)
- **Audit Trail**: Complete "who did what when" log for compliance
- **Premium Foundation**: Event stream for AOF, ARPS, AIPEL, ADLPCE products
- **Trust Scoring**: Dynamic trust score adjustment based on behavior

### Scope
**What AIM Will Capture (Basic Context, Zero LLM Cost)**:
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "action": "read_file",
  "resource": "/etc/passwd",
  "context": {
    "caller_file": "main.py",
    "caller_line": 42,
    "caller_function": "process_config",
    "stack_trace": ["main.py:42", "config.py:105", "utils.py:23"],
    "timestamp": "2025-01-15T10:30:00Z"
  },
  "allowed": true,
  "risk_level": "medium",
  "trust_score_before": 0.85,
  "trust_score_after": 0.83,
  "timestamp": "2025-01-15T10:30:00.123456Z"
}
```

**What AIM Will NOT Capture (Reserved for Premium Products)**:
- ❌ Prompt/completion text (AOF)
- ❌ Model parameters (AOF)
- ❌ Token counts and costs (AOF)
- ❌ Distributed tracing (AOF)
- ❌ Attack detection logic (ARPS)
- ❌ PHI/PII scanning (ADLPCE)
- ❌ Policy enforcement (AIPEL)

---

## 2. Database Schema Changes

### 2.1 New Table: `action_verifications`

```sql
-- Migration: 001_add_action_verifications.sql
CREATE TABLE action_verifications (
    -- Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- WHO (Agent Identity)
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- WHAT (Action Details)
    action TEXT NOT NULL,  -- e.g., 'read_file', 'write_file', 'api_call', 'database_query'
    resource TEXT NOT NULL,  -- e.g., '/etc/passwd', 'https://api.stripe.com', 'users_table'

    -- WHERE (Execution Context - Basic, No LLM)
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Expected context fields:
    -- {
    --   "caller_file": "main.py",
    --   "caller_line": 42,
    --   "caller_function": "process_config",
    --   "stack_trace": ["main.py:42", "config.py:105"],
    --   "process_id": 12345,
    --   "thread_id": 67890
    -- }

    -- WHEN
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- DECISION
    allowed BOOLEAN NOT NULL,  -- Was this action allowed?
    risk_level TEXT NOT NULL DEFAULT 'low',  -- 'low', 'medium', 'high', 'critical'
    decision_reason TEXT,  -- Why was it allowed/blocked?

    -- TRUST IMPACT
    trust_score_before DECIMAL(5,2),
    trust_score_after DECIMAL(5,2),
    trust_score_delta DECIMAL(5,2) GENERATED ALWAYS AS (trust_score_after - trust_score_before) STORED,

    -- ANOMALY FLAGS
    is_anomaly BOOLEAN DEFAULT FALSE,
    anomaly_reasons TEXT[],  -- e.g., ['new_resource', 'unusual_frequency', 'sensitive_file']

    -- METADATA
    sdk_version TEXT,  -- Which SDK version sent this
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_risk_level CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT valid_trust_scores CHECK (
        trust_score_before >= 0 AND trust_score_before <= 1 AND
        trust_score_after >= 0 AND trust_score_after <= 1
    )
);

-- Indexes for Performance
CREATE INDEX idx_action_verifications_agent_timestamp
    ON action_verifications (agent_id, timestamp DESC);

CREATE INDEX idx_action_verifications_organization
    ON action_verifications (organization_id, timestamp DESC);

CREATE INDEX idx_action_verifications_anomalies
    ON action_verifications (agent_id, timestamp DESC)
    WHERE is_anomaly = TRUE;

CREATE INDEX idx_action_verifications_risk_level
    ON action_verifications (risk_level, timestamp DESC)
    WHERE risk_level IN ('high', 'critical');

CREATE INDEX idx_action_verifications_action_resource
    ON action_verifications (action, resource);

-- GIN index for fast JSONB context queries
CREATE INDEX idx_action_verifications_context_gin
    ON action_verifications USING GIN (context);

-- Partial index for blocked actions only
CREATE INDEX idx_action_verifications_blocked
    ON action_verifications (agent_id, timestamp DESC)
    WHERE allowed = FALSE;

-- Comments
COMMENT ON TABLE action_verifications IS 'Records every action attempted by agents with verification results';
COMMENT ON COLUMN action_verifications.context IS 'Basic execution context captured without LLM (caller, stack trace, etc.)';
COMMENT ON COLUMN action_verifications.risk_level IS 'Risk assessment based on pattern matching (no ML)';
```

### 2.2 New Table: `agent_behavioral_baselines`

```sql
-- Migration: 002_add_agent_baselines.sql
CREATE TABLE agent_behavioral_baselines (
    -- Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Timeframe for this baseline
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,

    -- Action Patterns (JSON for flexibility)
    common_actions JSONB,
    -- e.g., {"read_file": 0.45, "api_call": 0.35, "database_read": 0.20}

    common_resources JSONB,
    -- e.g., {"/var/log/app.log": 0.60, "api.stripe.com": 0.25}

    common_action_sequences TEXT[],
    -- e.g., ['read_config,decrypt_key,access_db']

    -- Frequency Patterns
    avg_actions_per_hour DECIMAL(10,2),
    peak_hours INTEGER[],  -- e.g., [9, 10, 11, 14, 15] (UTC hours when most active)

    -- Resource Access Patterns
    resource_count INTEGER,  -- How many unique resources accessed
    resource_diversity DECIMAL(5,2),  -- Entropy measure (0.0 = single resource, 1.0 = evenly distributed)

    -- Risk Patterns
    avg_risk_score DECIMAL(5,2),
    high_risk_action_rate DECIMAL(5,2),  -- Percentage of high/critical risk actions

    -- Statistical Bounds (for anomaly detection)
    actions_per_hour_mean DECIMAL(10,2),
    actions_per_hour_stddev DECIMAL(10,2),

    -- Metadata
    sample_size INTEGER,  -- How many actions in this baseline
    confidence_score DECIMAL(5,2),  -- How confident are we in this baseline? (0.0-1.0)

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    UNIQUE(agent_id, start_date, end_date),
    CONSTRAINT valid_date_range CHECK (end_date >= start_date)
);

-- Index
CREATE INDEX idx_agent_baselines_agent ON agent_behavioral_baselines (agent_id, end_date DESC);

-- Comments
COMMENT ON TABLE agent_behavioral_baselines IS 'Behavioral baselines per agent for anomaly detection';
COMMENT ON COLUMN agent_behavioral_baselines.common_actions IS 'Distribution of actions (percentages)';
COMMENT ON COLUMN agent_behavioral_baselines.resource_diversity IS 'Entropy measure of resource access pattern';
```

### 2.3 New Table: `risk_patterns`

```sql
-- Migration: 003_add_risk_patterns.sql
CREATE TABLE risk_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Pattern Definition
    pattern_name TEXT NOT NULL UNIQUE,
    pattern_type TEXT NOT NULL,  -- 'resource_pattern', 'action_pattern', 'frequency_pattern'

    -- Pattern Matching
    resource_regex TEXT,  -- e.g., '^/etc/(passwd|shadow|sudoers)$'
    action_list TEXT[],  -- e.g., ['delete_database', 'drop_table']

    -- Risk Assessment
    risk_level TEXT NOT NULL,  -- 'low', 'medium', 'high', 'critical'
    description TEXT,

    -- Auto-response
    auto_block BOOLEAN DEFAULT FALSE,  -- Automatically block if matched?

    -- Status
    enabled BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT valid_risk_level CHECK (risk_level IN ('low', 'medium', 'high', 'critical'))
);

-- Seed Data: Common Risk Patterns
INSERT INTO risk_patterns (pattern_name, pattern_type, resource_regex, risk_level, description, auto_block) VALUES
('system_auth_files', 'resource_pattern', '^/etc/(passwd|shadow|sudoers|group)$', 'critical', 'System authentication and authorization files', FALSE),
('ssh_keys', 'resource_pattern', '^/home/.*/\.ssh/(id_rsa|id_ed25519|authorized_keys)$', 'high', 'SSH private keys and authorized keys', FALSE),
('database_credentials', 'resource_pattern', '.*(database\.yml|db\.conf|connection\.conf)$', 'high', 'Database configuration files with credentials', FALSE),
('api_keys_env', 'resource_pattern', '.*\.env(\..+)?$', 'medium', 'Environment files potentially containing API keys', FALSE),
('log_files', 'resource_pattern', '.*\.log$', 'low', 'Log file access (generally safe)', FALSE),
('sensitive_api_endpoints', 'resource_pattern', '.*(stripe\.com|paypal\.com|payment).*', 'high', 'Payment processing API endpoints', FALSE);

-- Index
CREATE INDEX idx_risk_patterns_enabled ON risk_patterns (enabled) WHERE enabled = TRUE;

-- Comments
COMMENT ON TABLE risk_patterns IS 'Predefined patterns for risk assessment (no ML, just regex matching)';
```

---

## 3. API Endpoints

### 3.1 Verify Action (SDK Endpoint)

**Endpoint**: `POST /api/v1/agents/{agent_id}/verify-action`

**Purpose**: SDK calls this to verify an action before execution

**Request**:
```json
{
  "action": "read_file",
  "resource": "/etc/passwd",
  "context": {
    "caller_file": "main.py",
    "caller_line": 42,
    "caller_function": "process_config",
    "stack_trace": ["main.py:42", "config.py:105"],
    "process_id": 12345,
    "thread_id": 67890
  }
}
```

**Response (Allowed)**:
```json
{
  "allowed": true,
  "risk_level": "medium",
  "reason": "System file access detected, but agent has appropriate trust score",
  "trust_score_before": 0.85,
  "trust_score_after": 0.83,
  "trust_score_delta": -0.02,
  "is_anomaly": false,
  "verification_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (Blocked)**:
```json
{
  "allowed": false,
  "risk_level": "critical",
  "reason": "Agent trust score too low for system file access",
  "trust_score_before": 0.45,
  "trust_score_after": 0.40,
  "trust_score_delta": -0.05,
  "is_anomaly": true,
  "anomaly_reasons": ["low_trust_score", "critical_resource"],
  "verification_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Response Codes**:
- `200 OK`: Verification completed (check `allowed` field)
- `401 Unauthorized`: Invalid agent credentials
- `404 Not Found`: Agent not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

**Rate Limiting**: 1000 requests/minute per agent

---

### 3.2 Get Action History

**Endpoint**: `GET /api/v1/agents/{agent_id}/actions`

**Purpose**: Retrieve action history for an agent

**Query Parameters**:
- `limit` (integer, default: 100, max: 1000): Number of records
- `offset` (integer, default: 0): Pagination offset
- `start_date` (ISO 8601): Filter by start date
- `end_date` (ISO 8601): Filter by end date
- `action` (string): Filter by action type
- `risk_level` (string): Filter by risk level
- `allowed` (boolean): Filter by allowed/blocked
- `is_anomaly` (boolean): Filter by anomaly flag

**Example Request**:
```
GET /api/v1/agents/550e8400-e29b-41d4-a716-446655440000/actions?limit=50&risk_level=high&is_anomaly=true
```

**Response**:
```json
{
  "total": 150,
  "limit": 50,
  "offset": 0,
  "actions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440002",
      "action": "read_file",
      "resource": "/etc/passwd",
      "context": {
        "caller_file": "main.py",
        "caller_line": 42,
        "caller_function": "process_config"
      },
      "allowed": true,
      "risk_level": "medium",
      "trust_score_before": 0.85,
      "trust_score_after": 0.83,
      "is_anomaly": false,
      "timestamp": "2025-01-15T10:30:00Z"
    }
  ]
}
```

---

### 3.3 Get Behavioral Baseline

**Endpoint**: `GET /api/v1/agents/{agent_id}/baseline`

**Purpose**: Get current behavioral baseline for an agent

**Query Parameters**:
- `days` (integer, default: 30): Number of days to analyze

**Response**:
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "baseline_period": {
    "start_date": "2024-12-15",
    "end_date": "2025-01-15"
  },
  "common_actions": {
    "read_file": 0.45,
    "api_call": 0.35,
    "database_read": 0.20
  },
  "common_resources": {
    "/var/log/app.log": 0.60,
    "api.stripe.com": 0.25,
    "database:users_table": 0.15
  },
  "avg_actions_per_hour": 12.5,
  "peak_hours": [9, 10, 11, 14, 15],
  "resource_count": 45,
  "resource_diversity": 0.72,
  "sample_size": 8500,
  "confidence_score": 0.95
}
```

---

### 3.4 Get Anomalies

**Endpoint**: `GET /api/v1/agents/{agent_id}/anomalies`

**Purpose**: Get recent anomalous actions for an agent

**Query Parameters**:
- `limit` (integer, default: 100, max: 1000)
- `days` (integer, default: 7): Last N days

**Response**:
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "period": {
    "start_date": "2025-01-08T00:00:00Z",
    "end_date": "2025-01-15T00:00:00Z"
  },
  "anomaly_count": 12,
  "anomalies": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440003",
      "action": "read_file",
      "resource": "/home/user/.ssh/id_rsa",
      "allowed": false,
      "risk_level": "high",
      "anomaly_reasons": ["new_resource", "sensitive_file"],
      "timestamp": "2025-01-14T15:22:00Z"
    }
  ]
}
```

---

## 4. SDK Implementation

### 4.1 Python SDK

**File**: `aim_sdk/verification.py`

```python
import sys
import traceback
import os
from typing import Dict, Any, Optional
from .client import AIMClient

class ActionVerifier:
    """Captures and verifies agent actions with AIM"""

    def __init__(self, client: AIMClient):
        self.client = client

    def verify_action(
        self,
        action: str,
        resource: str,
        context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Verify an action before execution

        Args:
            action: Type of action (e.g., 'read_file', 'api_call')
            resource: Resource being accessed (e.g., '/etc/passwd', 'https://api.stripe.com')
            context: Optional additional context (auto-captured if not provided)

        Returns:
            Verification result with 'allowed' boolean
        """
        # Auto-capture context if not provided
        if context is None:
            context = self._capture_context()

        # Send verification request
        response = self.client._post(
            f'/agents/{self.client.agent_id}/verify-action',
            json={
                'action': action,
                'resource': resource,
                'context': context
            }
        )

        return response

    def _capture_context(self) -> Dict[str, Any]:
        """Auto-capture execution context without LLM"""
        # Get caller frame (skip verify_action and _capture_context)
        frame = sys._getframe(2)

        # Extract stack trace
        stack = traceback.extract_stack()
        stack_trace = [
            f"{s.filename}:{s.lineno}"
            for s in stack[:-2]  # Exclude this function and verify_action
        ]

        return {
            'caller_file': frame.f_code.co_filename,
            'caller_line': frame.f_lineno,
            'caller_function': frame.f_code.co_name,
            'stack_trace': stack_trace[-5:],  # Last 5 frames
            'process_id': os.getpid(),
        }

    async def verify_action_async(
        self,
        action: str,
        resource: str,
        context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Async version of verify_action"""
        if context is None:
            context = self._capture_context()

        response = await self.client._post_async(
            f'/agents/{self.client.agent_id}/verify-action',
            json={
                'action': action,
                'resource': resource,
                'context': context
            }
        )

        return response
```

**File**: `aim_sdk/interceptors.py`

```python
import builtins
from typing import Any
from .verification import ActionVerifier

_original_open = builtins.open
_verifier: ActionVerifier = None

def install_interceptors(verifier: ActionVerifier):
    """Install automatic verification for common operations"""
    global _verifier
    _verifier = verifier

    # Intercept file operations
    builtins.open = _wrapped_open

def _wrapped_open(file, mode='r', *args, **kwargs):
    """Wrapped open() with automatic verification"""
    if _verifier:
        action = 'write_file' if any(c in mode for c in 'wax+') else 'read_file'

        result = _verifier.verify_action(
            action=action,
            resource=str(file)
        )

        if not result['allowed']:
            raise PermissionError(
                f"AIM blocked {action} on {file}: {result['reason']}"
            )

    return _original_open(file, mode, *args, **kwargs)
```

**Usage Example**:
```python
from aim_sdk import AIMClient, ActionVerifier, install_interceptors

# Initialize client
client = AIMClient(
    agent_id="550e8400-e29b-41d4-a716-446655440000",
    private_key="your-private-key"
)

# Option 1: Explicit verification
verifier = ActionVerifier(client)
result = verifier.verify_action(
    action="read_file",
    resource="/etc/passwd"
)

if result['allowed']:
    with open('/etc/passwd', 'r') as f:
        data = f.read()
else:
    print(f"Blocked: {result['reason']}")

# Option 2: Automatic interception
install_interceptors(verifier)
with open('/etc/passwd', 'r') as f:  # Automatically verified!
    data = f.read()
```

---

### 4.2 JavaScript/TypeScript SDK

**File**: `aim-sdk/src/verification.ts`

```typescript
import { AIMClient } from './client';

export interface VerificationContext {
  caller_file?: string;
  caller_line?: number;
  caller_function?: string;
  stack_trace?: string[];
  [key: string]: any;
}

export interface VerificationResult {
  allowed: boolean;
  risk_level: 'low' | 'medium' | 'high' | 'critical';
  reason: string;
  trust_score_before: number;
  trust_score_after: number;
  trust_score_delta: number;
  is_anomaly: boolean;
  anomaly_reasons?: string[];
  verification_id: string;
}

export class ActionVerifier {
  constructor(private client: AIMClient) {}

  async verifyAction(
    action: string,
    resource: string,
    context?: VerificationContext
  ): Promise<VerificationResult> {
    // Auto-capture context if not provided
    const finalContext = context || this.captureContext();

    const response = await this.client.post<VerificationResult>(
      `/agents/${this.client.agentId}/verify-action`,
      {
        action,
        resource,
        context: finalContext
      }
    );

    return response;
  }

  private captureContext(): VerificationContext {
    // Capture stack trace
    const stack = new Error().stack;
    if (!stack) return {};

    const lines = stack.split('\n');
    const stackTrace = lines
      .slice(3) // Skip Error, captureContext, verifyAction
      .map(line => {
        const match = line.match(/at\s+(.+)\s+\((.+):(\d+):(\d+)\)/);
        if (match) {
          return `${match[2]}:${match[3]}`;
        }
        return line.trim();
      })
      .slice(0, 5); // Last 5 frames

    return {
      stack_trace: stackTrace,
      process_id: process.pid
    };
  }
}
```

**Usage Example**:
```typescript
import { AIMClient, ActionVerifier } from '@aim/sdk';

const client = new AIMClient({
  agentId: '550e8400-e29b-41d4-a716-446655440000',
  privateKey: 'your-private-key'
});

const verifier = new ActionVerifier(client);

// Verify before file operation
const result = await verifier.verifyAction(
  'read_file',
  '/etc/passwd'
);

if (result.allowed) {
  const data = await fs.readFile('/etc/passwd', 'utf-8');
} else {
  console.error(`Blocked: ${result.reason}`);
}
```

---

### 4.3 Go SDK

**File**: `aim-sdk-go/verification.go`

```go
package aim

import (
	"context"
	"runtime"
	"os"
	"fmt"
)

// VerificationContext contains execution context
type VerificationContext struct {
	CallerFile     string   `json:"caller_file,omitempty"`
	CallerLine     int      `json:"caller_line,omitempty"`
	CallerFunction string   `json:"caller_function,omitempty"`
	StackTrace     []string `json:"stack_trace,omitempty"`
	ProcessID      int      `json:"process_id,omitempty"`
}

// VerificationResult is the result of action verification
type VerificationResult struct {
	Allowed          bool     `json:"allowed"`
	RiskLevel        string   `json:"risk_level"`
	Reason           string   `json:"reason"`
	TrustScoreBefore float64  `json:"trust_score_before"`
	TrustScoreAfter  float64  `json:"trust_score_after"`
	TrustScoreDelta  float64  `json:"trust_score_delta"`
	IsAnomaly        bool     `json:"is_anomaly"`
	AnomalyReasons   []string `json:"anomaly_reasons,omitempty"`
	VerificationID   string   `json:"verification_id"`
}

// ActionVerifier handles action verification
type ActionVerifier struct {
	client *Client
}

// NewActionVerifier creates a new verifier
func NewActionVerifier(client *Client) *ActionVerifier {
	return &ActionVerifier{client: client}
}

// VerifyAction verifies an action before execution
func (v *ActionVerifier) VerifyAction(
	ctx context.Context,
	action string,
	resource string,
	context *VerificationContext,
) (*VerificationResult, error) {
	// Auto-capture context if not provided
	if context == nil {
		context = v.captureContext()
	}

	payload := map[string]interface{}{
		"action":   action,
		"resource": resource,
		"context":  context,
	}

	var result VerificationResult
	err := v.client.post(
		ctx,
		fmt.Sprintf("/agents/%s/verify-action", v.client.agentID),
		payload,
		&result,
	)

	return &result, err
}

// captureContext auto-captures execution context
func (v *ActionVerifier) captureContext() *VerificationContext {
	pc := make([]uintptr, 10)
	n := runtime.Callers(3, pc) // Skip runtime.Callers, captureContext, VerifyAction
	frames := runtime.CallersFrames(pc[:n])

	var stackTrace []string
	var firstFrame *runtime.Frame

	for {
		frame, more := frames.Next()
		if firstFrame == nil {
			f := frame
			firstFrame = &f
		}
		stackTrace = append(stackTrace, fmt.Sprintf("%s:%d", frame.File, frame.Line))
		if !more || len(stackTrace) >= 5 {
			break
		}
	}

	ctx := &VerificationContext{
		StackTrace: stackTrace,
		ProcessID:  os.Getpid(),
	}

	if firstFrame != nil {
		ctx.CallerFile = firstFrame.File
		ctx.CallerLine = firstFrame.Line
		ctx.CallerFunction = firstFrame.Function
	}

	return ctx
}
```

**Usage Example**:
```go
package main

import (
	"context"
	"fmt"
	aim "github.com/opena2a/aim-sdk-go"
)

func main() {
	ctx := context.Background()

	client := aim.NewClient(aim.Config{
		AgentID:    "550e8400-e29b-41d4-a716-446655440000",
		PrivateKey: "your-private-key",
	})

	verifier := aim.NewActionVerifier(client)

	// Verify before file operation
	result, err := verifier.VerifyAction(
		ctx,
		"read_file",
		"/etc/passwd",
		nil, // Auto-capture context
	)

	if err != nil {
		panic(err)
	}

	if result.Allowed {
		// Proceed with file operation
		data, _ := os.ReadFile("/etc/passwd")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Blocked: %s\n", result.Reason)
	}
}
```

---

## 5. Backend Implementation

### 5.1 Verification Service

**File**: `apps/backend/internal/verification/service.go`

```go
package verification

import (
	"context"
	"database/sql"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	db            *sql.DB
	riskPatterns  []RiskPattern
	baselineCache map[string]*Baseline // agent_id -> baseline
}

type VerificationRequest struct {
	AgentID  uuid.UUID
	Action   string
	Resource string
	Context  map[string]interface{}
}

type VerificationResult struct {
	Allowed          bool
	RiskLevel        string
	Reason           string
	TrustScoreBefore float64
	TrustScoreAfter  float64
	TrustScoreDelta  float64
	IsAnomaly        bool
	AnomalyReasons   []string
	VerificationID   uuid.UUID
}

type RiskPattern struct {
	Name          string
	ResourceRegex *regexp.Regexp
	RiskLevel     string
	AutoBlock     bool
}

type Baseline struct {
	CommonActions   map[string]float64
	CommonResources map[string]float64
	AvgActionsPerHour float64
	ActionsPerHourStdDev float64
}

func NewService(db *sql.DB) *Service {
	s := &Service{
		db:            db,
		baselineCache: make(map[string]*Baseline),
	}
	s.loadRiskPatterns()
	return s
}

func (s *Service) VerifyAction(ctx context.Context, req VerificationRequest) (*VerificationResult, error) {
	// 1. Get agent's current trust score
	trustScoreBefore, err := s.getAgentTrustScore(ctx, req.AgentID)
	if err != nil {
		return nil, err
	}

	// 2. Assess risk level (pattern matching, no ML)
	riskLevel, matchedPattern := s.assessRiskLevel(req.Action, req.Resource)

	// 3. Check for anomalies (behavioral baseline)
	isAnomaly, anomalyReasons := s.detectAnomalies(ctx, req.AgentID, req.Action, req.Resource)

	// 4. Make allow/deny decision
	allowed := s.makeDecision(trustScoreBefore, riskLevel, isAnomaly, matchedPattern)

	// 5. Calculate new trust score
	trustScoreAfter := s.updateTrustScore(trustScoreBefore, riskLevel, allowed, isAnomaly)

	// 6. Build decision reason
	reason := s.buildReason(allowed, riskLevel, trustScoreBefore, isAnomaly, anomalyReasons)

	// 7. Record verification
	verificationID, err := s.recordVerification(ctx, VerificationRecord{
		AgentID:          req.AgentID,
		Action:           req.Action,
		Resource:         req.Resource,
		Context:          req.Context,
		Allowed:          allowed,
		RiskLevel:        riskLevel,
		TrustScoreBefore: trustScoreBefore,
		TrustScoreAfter:  trustScoreAfter,
		IsAnomaly:        isAnomaly,
		AnomalyReasons:   anomalyReasons,
		Timestamp:        time.Now(),
	})

	if err != nil {
		return nil, err
	}

	// 8. Update agent's trust score in database
	if err := s.updateAgentTrustScore(ctx, req.AgentID, trustScoreAfter); err != nil {
		return nil, err
	}

	return &VerificationResult{
		Allowed:          allowed,
		RiskLevel:        riskLevel,
		Reason:           reason,
		TrustScoreBefore: trustScoreBefore,
		TrustScoreAfter:  trustScoreAfter,
		TrustScoreDelta:  trustScoreAfter - trustScoreBefore,
		IsAnomaly:        isAnomaly,
		AnomalyReasons:   anomalyReasons,
		VerificationID:   verificationID,
	}, nil
}

func (s *Service) assessRiskLevel(action, resource string) (string, *RiskPattern) {
	// Check against risk patterns
	for _, pattern := range s.riskPatterns {
		if pattern.ResourceRegex.MatchString(resource) {
			return pattern.RiskLevel, &pattern
		}
	}
	return "low", nil
}

func (s *Service) detectAnomalies(ctx context.Context, agentID uuid.UUID, action, resource string) (bool, []string) {
	// Get agent's baseline
	baseline := s.getBaseline(ctx, agentID)
	if baseline == nil {
		return false, nil // No baseline yet
	}

	var anomalyReasons []string

	// Check 1: New resource?
	if _, exists := baseline.CommonResources[resource]; !exists {
		anomalyReasons = append(anomalyReasons, "new_resource")
	}

	// Check 2: New action type?
	if _, exists := baseline.CommonActions[action]; !exists {
		anomalyReasons = append(anomalyReasons, "new_action_type")
	}

	// Check 3: Frequency spike? (simplified - check recent hour)
	recentCount := s.getRecentActionCount(ctx, agentID, 1*time.Hour)
	if float64(recentCount) > baseline.AvgActionsPerHour + 3*baseline.ActionsPerHourStdDev {
		anomalyReasons = append(anomalyReasons, "unusual_frequency")
	}

	return len(anomalyReasons) > 0, anomalyReasons
}

func (s *Service) makeDecision(trustScore float64, riskLevel string, isAnomaly bool, pattern *RiskPattern) bool {
	// Auto-block patterns
	if pattern != nil && pattern.AutoBlock {
		return false
	}

	// Risk-based decision
	switch riskLevel {
	case "critical":
		// Critical resources require high trust
		return trustScore >= 0.80 && !isAnomaly
	case "high":
		// High-risk requires good trust
		return trustScore >= 0.60
	case "medium":
		// Medium-risk requires decent trust
		return trustScore >= 0.40
	case "low":
		// Low-risk almost always allowed
		return trustScore >= 0.20
	default:
		return true
	}
}

func (s *Service) updateTrustScore(current float64, riskLevel string, allowed bool, isAnomaly bool) float64 {
	delta := 0.0

	// Penalty for high-risk actions
	if riskLevel == "critical" {
		delta -= 0.02
	} else if riskLevel == "high" {
		delta -= 0.01
	}

	// Penalty for anomalies
	if isAnomaly {
		delta -= 0.02
	}

	// Penalty for blocked actions
	if !allowed {
		delta -= 0.05
	}

	// Small reward for normal behavior
	if allowed && !isAnomaly && (riskLevel == "low" || riskLevel == "medium") {
		delta += 0.001
	}

	newScore := current + delta

	// Clamp to [0, 1]
	if newScore < 0 {
		newScore = 0
	} else if newScore > 1 {
		newScore = 1
	}

	return newScore
}

func (s *Service) buildReason(allowed bool, riskLevel string, trustScore float64, isAnomaly bool, anomalyReasons []string) string {
	if !allowed {
		if isAnomaly {
			return fmt.Sprintf("Blocked: Anomalous behavior detected (%v)", anomalyReasons)
		}
		return fmt.Sprintf("Blocked: Trust score (%.2f) too low for %s risk action", trustScore, riskLevel)
	}

	if isAnomaly {
		return fmt.Sprintf("Allowed with caution: Anomaly detected but trust score sufficient")
	}

	return fmt.Sprintf("Allowed: Normal %s risk action", riskLevel)
}
```

---

### 5.2 Baseline Calculation (Background Job)

**File**: `apps/backend/internal/verification/baseline.go`

```go
package verification

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CalculateBaselines runs daily to compute behavioral baselines
func (s *Service) CalculateBaselines(ctx context.Context) error {
	// Get all active agents
	agents, err := s.getActiveAgents(ctx)
	if err != nil {
		return err
	}

	for _, agentID := range agents {
		if err := s.calculateAgentBaseline(ctx, agentID); err != nil {
			// Log error but continue with other agents
			log.Errorf("Failed to calculate baseline for agent %s: %v", agentID, err)
		}
	}

	return nil
}

func (s *Service) calculateAgentBaseline(ctx context.Context, agentID uuid.UUID) error {
	// Look back 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	// Get all actions in this period
	actions, err := s.getActions(ctx, agentID, startDate, endDate)
	if err != nil {
		return err
	}

	if len(actions) < 100 {
		// Not enough data for baseline
		return nil
	}

	// Calculate action distribution
	actionCounts := make(map[string]int)
	resourceCounts := make(map[string]int)

	for _, action := range actions {
		actionCounts[action.Action]++
		resourceCounts[action.Resource]++
	}

	totalActions := len(actions)
	commonActions := make(map[string]float64)
	commonResources := make(map[string]float64)

	for action, count := range actionCounts {
		commonActions[action] = float64(count) / float64(totalActions)
	}

	for resource, count := range resourceCounts {
		commonResources[resource] = float64(count) / float64(totalActions)
	}

	// Calculate frequency patterns
	hourCounts := make(map[int]int)
	for _, action := range actions {
		hour := action.Timestamp.Hour()
		hourCounts[hour]++
	}

	// Find peak hours
	var peakHours []int
	avgPerHour := float64(totalActions) / 24.0
	for hour, count := range hourCounts {
		if float64(count) > avgPerHour * 1.5 {
			peakHours = append(peakHours, hour)
		}
	}

	// Calculate statistical measures
	actionsPerHour := make([]float64, 24)
	for hour, count := range hourCounts {
		actionsPerHour[hour] = float64(count)
	}

	mean, stddev := calculateStats(actionsPerHour)

	// Store baseline
	baseline := Baseline{
		AgentID:               agentID,
		StartDate:             startDate,
		EndDate:               endDate,
		CommonActions:         commonActions,
		CommonResources:       commonResources,
		PeakHours:             peakHours,
		ResourceCount:         len(resourceCounts),
		ResourceDiversity:     calculateEntropy(resourceCounts),
		AvgActionsPerHour:     mean,
		ActionsPerHourStdDev:  stddev,
		SampleSize:            totalActions,
		ConfidenceScore:       calculateConfidence(totalActions),
	}

	return s.storeBaseline(ctx, baseline)
}

func calculateStats(values []float64) (mean, stddev float64) {
	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	// Calculate standard deviation
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))
	stddev = math.Sqrt(variance)

	return
}

func calculateEntropy(counts map[string]int) float64 {
	total := 0
	for _, count := range counts {
		total += count
	}

	entropy := 0.0
	for _, count := range counts {
		if count > 0 {
			p := float64(count) / float64(total)
			entropy -= p * math.Log2(p)
		}
	}

	// Normalize to [0, 1]
	maxEntropy := math.Log2(float64(len(counts)))
	if maxEntropy > 0 {
		entropy /= maxEntropy
	}

	return entropy
}

func calculateConfidence(sampleSize int) float64 {
	// Confidence increases with sample size
	// 100 samples = 0.5, 1000 samples = 0.8, 10000+ samples = 0.95
	if sampleSize < 100 {
		return 0.3
	} else if sampleSize < 500 {
		return 0.5
	} else if sampleSize < 1000 {
		return 0.7
	} else if sampleSize < 5000 {
		return 0.85
	} else {
		return 0.95
	}
}
```

---

## 6. Event Publishing

### 6.1 Event Bus Integration

**File**: `apps/backend/internal/events/publisher.go`

```go
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventPublisher interface {
	PublishActionVerified(ctx context.Context, event ActionVerifiedEvent) error
	PublishAnomalyDetected(ctx context.Context, event AnomalyDetectedEvent) error
}

type ActionVerifiedEvent struct {
	EventID          uuid.UUID              `json:"event_id"`
	EventType        string                 `json:"event_type"`
	Timestamp        time.Time              `json:"timestamp"`
	AgentID          uuid.UUID              `json:"agent_id"`
	OrganizationID   uuid.UUID              `json:"organization_id"`
	Action           string                 `json:"action"`
	Resource         string                 `json:"resource"`
	Context          map[string]interface{} `json:"context"`
	Allowed          bool                   `json:"allowed"`
	RiskLevel        string                 `json:"risk_level"`
	TrustScoreBefore float64                `json:"trust_score_before"`
	TrustScoreAfter  float64                `json:"trust_score_after"`
	IsAnomaly        bool                   `json:"is_anomaly"`
	VerificationID   uuid.UUID              `json:"verification_id"`
}

type AnomalyDetectedEvent struct {
	EventID        uuid.UUID `json:"event_id"`
	EventType      string    `json:"event_type"`
	Timestamp      time.Time `json:"timestamp"`
	AgentID        uuid.UUID `json:"agent_id"`
	Action         string    `json:"action"`
	Resource       string    `json:"resource"`
	AnomalyReasons []string  `json:"anomaly_reasons"`
	RiskLevel      string    `json:"risk_level"`
	Blocked        bool      `json:"blocked"`
}

// KafkaPublisher implements EventPublisher using Kafka
type KafkaPublisher struct {
	producer KafkaProducer
}

func (p *KafkaPublisher) PublishActionVerified(ctx context.Context, event ActionVerifiedEvent) error {
	event.EventID = uuid.New()
	event.EventType = "aim.action.verified"
	event.Timestamp = time.Now()

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.producer.Publish(ctx, "aim.action.verified", payload)
}

func (p *KafkaPublisher) PublishAnomalyDetected(ctx context.Context, event AnomalyDetectedEvent) error {
	event.EventID = uuid.New()
	event.EventType = "aim.anomaly.detected"
	event.Timestamp = time.Now()

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.producer.Publish(ctx, "aim.anomaly.detected", payload)
}
```

---

## 7. Testing Requirements

### 7.1 Unit Tests

**Test Coverage Target**: 90%+

**Key Test Files**:
- `apps/backend/internal/verification/service_test.go`
- `apps/backend/internal/verification/baseline_test.go`
- `aim_sdk/tests/test_verification.py`
- `aim-sdk/src/__tests__/verification.test.ts`
- `aim-sdk-go/verification_test.go`

**Test Cases**:

1. **Risk Assessment**:
   - Test pattern matching for known sensitive files
   - Test default risk level (low) for unknown resources
   - Test all risk levels (low, medium, high, critical)

2. **Anomaly Detection**:
   - Test new resource detection
   - Test frequency spike detection
   - Test baseline calculation with various sample sizes

3. **Decision Making**:
   - Test allow/block logic for different trust scores and risk levels
   - Test auto-block patterns
   - Test anomaly-based blocking

4. **Trust Score Updates**:
   - Test trust score decreases for high-risk actions
   - Test trust score decreases for anomalies
   - Test trust score increases for normal behavior
   - Test clamping to [0, 1] range

5. **Context Capture**:
   - Test automatic stack trace capture (Python, JS, Go)
   - Test caller file/line/function extraction
   - Test context in verification requests

---

### 7.2 Integration Tests

**Test Scenarios**:

1. **End-to-End Verification Flow**:
   - Agent registers
   - Agent calls verify_action
   - AIM assesses risk
   - AIM records verification
   - AIM publishes event
   - Trust score updated

2. **Baseline Calculation**:
   - Agent performs 1000 actions over 30 days
   - Baseline job runs
   - Baseline calculated correctly
   - Anomaly detection works based on baseline

3. **Event Publishing**:
   - Verification recorded
   - Event published to Kafka
   - External consumer receives event

---

### 7.3 Load Testing

**Performance Targets**:
- 1000 verifications/second per backend instance
- P50 latency: < 10ms
- P95 latency: < 50ms
- P99 latency: < 100ms

**Load Test Scenarios**:
- Simulate 10,000 agents
- Each agent performs 10 actions/minute
- Total: 100,000 verifications/minute = 1,667/second
- Run for 1 hour

**Tools**: k6, Locust

---

## 8. Migration Plan

### Phase 1: Database Setup (Week 1)
- ✅ Create migration files
- ✅ Add action_verifications table
- ✅ Add agent_behavioral_baselines table
- ✅ Add risk_patterns table
- ✅ Seed risk patterns
- ✅ Test migrations on dev/staging

### Phase 2: Backend Implementation (Week 1-2)
- ✅ Implement verification service
- ✅ Implement risk assessment logic
- ✅ Implement anomaly detection
- ✅ Implement trust score updates
- ✅ Implement API endpoints
- ✅ Add event publishing
- ✅ Write unit tests

### Phase 3: SDK Implementation (Week 2-3)
- ✅ Implement Python SDK
- ✅ Implement JavaScript/TypeScript SDK
- ✅ Implement Go SDK
- ✅ Add context auto-capture
- ✅ Add automatic interceptors (optional)
- ✅ Write SDK tests
- ✅ Create SDK examples

### Phase 4: Background Jobs (Week 3)
- ✅ Implement baseline calculation job
- ✅ Schedule daily baseline updates
- ✅ Add monitoring for job execution

### Phase 5: Testing & Optimization (Week 3-4)
- ✅ Integration testing
- ✅ Load testing
- ✅ Performance optimization
- ✅ Security review

### Phase 6: Documentation & Launch (Week 4)
- ✅ API documentation
- ✅ SDK documentation
- ✅ Migration guide
- ✅ Release notes
- ✅ Deploy to production

---

## 9. Rollout Strategy

### Step 1: Beta Release (Internal)
- Deploy to staging
- Invite 5-10 internal test agents
- Collect feedback
- Fix bugs

### Step 2: Limited Release (Early Adopters)
- Deploy to production
- Enable for opt-in users only
- Monitor performance and errors
- Iterate based on feedback

### Step 3: General Availability
- Enable by default for new agents
- Provide migration guide for existing agents
- Announce feature publicly
- Monitor adoption metrics

---

## 10. Monitoring & Observability

### Metrics to Track:

**Business Metrics**:
- Total verifications per day
- Blocked actions per day
- Anomaly rate (%)
- Average trust score (all agents)

**Performance Metrics**:
- Verification latency (P50, P95, P99)
- Database query time
- Event publishing latency
- Baseline calculation time

**Error Metrics**:
- Failed verifications (rate)
- Database errors
- Event publishing failures
- SDK errors

**Dashboards**:
- Real-time verification dashboard
- Anomaly detection dashboard
- Trust score trends
- Performance metrics

---

## 11. Success Criteria

### Technical Success:
- ✅ 99.9% uptime for verification API
- ✅ < 50ms P95 latency
- ✅ < 0.1% error rate
- ✅ All SDKs working correctly

### Business Success:
- ✅ 50%+ of active agents using verification
- ✅ 1M+ verifications per month
- ✅ < 5% blocked actions (not too restrictive)
- ✅ Positive user feedback

### Premium Product Enablement:
- ✅ AOF consuming AIM events successfully
- ✅ ARPS using AIM data for attack detection
- ✅ Clear upgrade path from AIM to premium products

---

## 12. Future Enhancements (Post-Launch)

### Phase 2 Features:
- ML-based anomaly detection (replace simple rules)
- Predictive risk scoring (predict attacks before they happen)
- Agent collaboration patterns (multi-agent workflows)
- Custom risk patterns (user-defined rules)
- Verification policies (allow/block rules per agent/org)

### Premium Integration:
- Deep integration with AOF (enriched telemetry)
- ARPS attack detection triggers
- AIPEL policy enforcement hooks
- ASA threat correlation

---

## 13. Open Questions (To Be Resolved)

1. **Storage Retention**: Default retention period for self-hosted? (Recommendation: No limit, user decides)

2. **Event Bus**: Which technology? (Recommendation: Kafka for production, NATS for simplicity)

3. **Baseline Update Frequency**: Daily vs. weekly? (Recommendation: Daily for first 30 days, then weekly)

4. **Trust Score Initial Value**: New agents start at 0.5, 0.7, or 1.0? (Recommendation: 0.7 - benefit of doubt)

5. **Anomaly Threshold**: How many anomalies before blocking? (Recommendation: Block on first critical anomaly, warn on 3+ medium)

---

## 14. Resources Required

### Engineering:
- 1 Backend Engineer (3-4 weeks)
- 1 Frontend Engineer (1 week for dashboard updates)
- 1 SDK Engineer (2 weeks for 3 SDKs)
- 1 QA Engineer (1 week for testing)

### Infrastructure:
- Kafka/NATS cluster (if not already available)
- Additional database storage (~100GB/month for 10k agents)
- Monitoring tools (Prometheus, Grafana)

### Total Estimated Effort: **3-4 weeks** with a team of 3-4 engineers

---

**This document is ready for implementation when the team is ready to build this feature.**
