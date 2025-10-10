# Prompt for New Claude Session: Test Capability-Trust Score Integration

## Context Summary

You are continuing work on the **Agent Identity Management (AIM)** project. The previous Claude session completed the integration of agent capability detection with the trust scoring system. The integration is **code-complete and compiles successfully**, but needs comprehensive testing before deployment.

## What Was Already Done ✅

### 1. **Capability Risk Added to Trust Scoring** ✅
   - Added `CapabilityRisk` field to `TrustScoreFactors` (9th factor)
   - Rebalanced trust weights: capability risk gets 17% weight
   - File: `apps/backend/internal/domain/trust_score.go:19`

### 2. **Sophisticated Risk Scoring Algorithm Implemented** ✅
   - Baseline score: 0.7 (neutral for agents with no capabilities)
   - High-risk penalties: system:admin (-0.20), user:impersonate (-0.20), file:delete (-0.15)
   - Medium-risk penalties: file:write (-0.08), db:write (-0.08), api:call (-0.05)
   - Low-risk penalties: file:read (-0.03), db:query (-0.03), mcp:tool_use (-0.02)
   - Violation history penalties (last 30 days) with severity weighting
   - File: `apps/backend/internal/application/trust_calculator.go:262-346`

### 3. **Integration with Detection Service** ✅
   - Replaced naive trust calculation with comprehensive 9-factor algorithm
   - Now fetches full agent entity and calculates proper trust score
   - Stores trust score in both `agents` and `trust_scores` tables
   - File: `apps/backend/internal/application/detection_service.go:372-430`

### 4. **Dependency Injection Complete** ✅
   - TrustCalculator now receives CapabilityRepository
   - DetectionService now receives TrustCalculator and AgentRepository
   - All wired together in main.go
   - Files: `apps/backend/cmd/server/main.go:327-331, 417-421`

### 5. **Compilation Status** ✅
   - Code compiles successfully with no errors
   - All type signatures match
   - Dependency injection working correctly

### 6. **Documentation Created** ✅
   - Comprehensive integration guide: `CAPABILITY_TRUST_INTEGRATION.md`
   - Includes technical details, examples, testing checklist

## Current State of Trust Scoring

### Trust Score Formula (9 Factors)

```
Trust Score =
  18% Identity Verification +
  12% Certificate Validity +
  12% Repository Quality +
   8% Documentation Score +
   8% Community Trust +
  12% Security Audit +
   8% Update Frequency +
   5% Agent Age +
  17% Capability Risk ← NEW! ✅
```

### Capability Risk Scoring Logic

**Method**: `calculateCapabilityRisk(agent *domain.Agent) float64`

**Location**: `apps/backend/internal/application/trust_calculator.go:262-346`

```go
func (c *TrustCalculator) calculateCapabilityRisk(agent *domain.Agent) float64 {
	// Start with baseline score (no capabilities detected = neutral risk)
	score := 0.7 // Neutral baseline

	// Get active capabilities for the agent
	capabilities, err := c.capabilityRepo.GetActiveCapabilitiesByAgentID(agent.ID)
	if err != nil || len(capabilities) == 0 {
		return score // No capabilities data = neutral score
	}

	// Define high-risk capability types
	highRiskCapabilities := map[string]float64{
		domain.CapabilityFileDelete:      -0.15, // File deletion is high risk
		domain.CapabilitySystemAdmin:     -0.20, // System admin is very high risk
		domain.CapabilityUserImpersonate: -0.20, // Impersonation is very high risk
		domain.CapabilityDataExport:      -0.10, // Data export is moderate risk
	}

	mediumRiskCapabilities := map[string]float64{
		domain.CapabilityFileWrite:   -0.08,
		domain.CapabilityDBWrite:     -0.08,
		domain.CapabilityAPICall:     -0.05,
	}

	lowRiskCapabilities := map[string]float64{
		domain.CapabilityFileRead:    -0.03,
		domain.CapabilityDBQuery:     -0.03,
		domain.CapabilityMCPToolUse:  -0.02,
	}

	// Calculate risk based on capabilities
	for _, cap := range capabilities {
		// Check high-risk capabilities
		if penalty, exists := highRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		} else if penalty, exists := mediumRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		} else if penalty, exists := lowRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		}
	}

	// Get recent violations (last 30 days)
	violations, _, err := c.capabilityRepo.GetViolationsByAgentID(agent.ID, 100, 0)
	if err == nil && len(violations) > 0 {
		// Recent violations significantly impact trust
		recentViolations := 0
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

		for _, violation := range violations {
			if violation.CreatedAt.After(thirtyDaysAgo) {
				recentViolations++

				// Additional penalty based on violation severity
				switch violation.Severity {
				case domain.ViolationSeverityCritical:
					score -= 0.15
				case domain.ViolationSeverityHigh:
					score -= 0.10
				case domain.ViolationSeverityMedium:
					score -= 0.05
				case domain.ViolationSeverityLow:
					score -= 0.02
				}
			}
		}

		// Cap violations penalty
		if recentViolations > 10 {
			score -= 0.20 // Significant violation history
		} else if recentViolations > 5 {
			score -= 0.10
		}
	}

	// Ensure score stays within bounds [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}
```

## Your Tasks

### 🎯 Task 1: Write Unit Tests for Capability Risk Calculation

**Objective**: Write comprehensive unit tests for the `calculateCapabilityRisk()` method to ensure correct scoring logic.

#### File to Create

**Path**: `apps/backend/internal/application/trust_calculator_test.go`

#### Test Cases to Implement

```go
package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock CapabilityRepository
type MockCapabilityRepository struct {
	mock.Mock
}

func (m *MockCapabilityRepository) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.Capability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Capability), args.Error(1)
}

func (m *MockCapabilityRepository) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

// Test 1: Agent with no capabilities (neutral baseline)
func TestCalculateCapabilityRisk_NoCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.Capability{}, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Agent with no capabilities should have neutral score of 0.7")
	mockRepo.AssertExpectations(t)
}

// Test 2: Agent with only low-risk capabilities
func TestCalculateCapabilityRisk_LowRiskOnly(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: Low-risk capabilities (file:read, db:query)
	capabilities := []*domain.Capability{
		{CapabilityType: domain.CapabilityFileRead, IsRevoked: false},
		{CapabilityType: domain.CapabilityDBQuery, IsRevoked: false},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.03 - 0.03 // 0.64
	assert.Equal(t, expectedScore, score, "Low-risk capabilities should have minor penalties")
	mockRepo.AssertExpectations(t)
}

// Test 3: Agent with high-risk capabilities
func TestCalculateCapabilityRisk_HighRiskCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: High-risk capabilities (system:admin, user:impersonate)
	capabilities := []*domain.Capability{
		{CapabilityType: domain.CapabilitySystemAdmin, IsRevoked: false},
		{CapabilityType: domain.CapabilityUserImpersonate, IsRevoked: false},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.20 - 0.20 // 0.30
	assert.Equal(t, expectedScore, score, "High-risk capabilities should have major penalties")
	mockRepo.AssertExpectations(t)
}

// Test 4: Agent with recent CRITICAL violations
func TestCalculateCapabilityRisk_CriticalViolations(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: Single low-risk capability
	capabilities := []*domain.Capability{
		{CapabilityType: domain.CapabilityFileRead, IsRevoked: false},
	}

	// Mock: 3 recent CRITICAL violations (last 7 days)
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -5), // 5 days ago
		},
		{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -7), // 7 days ago
		},
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 3, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.03 - (3 * 0.15) // 0.7 - 0.03 - 0.45 = 0.22
	assert.Equal(t, expectedScore, score, "CRITICAL violations should heavily impact trust")
	mockRepo.AssertExpectations(t)
}

// Test 5: Agent with many violations (volume penalty)
func TestCalculateCapabilityRisk_HighViolationVolume(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.Capability{}, nil)

	// Mock: 12 recent LOW violations (triggers volume penalty)
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 12)
	for i := 0; i < 12; i++ {
		violations[i] = &domain.CapabilityViolation{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityLow,
			CreatedAt: now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 12, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - (12 * 0.02) - 0.20 // 0.7 - 0.24 - 0.20 = 0.26
	assert.Equal(t, expectedScore, score, "High violation volume should trigger additional penalty")
	mockRepo.AssertExpectations(t)
}

// Test 6: Score bounds enforcement (cannot go below 0)
func TestCalculateCapabilityRisk_ScoreBounds(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: All high-risk capabilities
	capabilities := []*domain.Capability{
		{CapabilityType: domain.CapabilitySystemAdmin, IsRevoked: false},
		{CapabilityType: domain.CapabilityUserImpersonate, IsRevoked: false},
		{CapabilityType: domain.CapabilityFileDelete, IsRevoked: false},
		{CapabilityType: domain.CapabilityDataExport, IsRevoked: false},
	}

	// Mock: Many CRITICAL violations
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 20)
	for i := 0; i < 20; i++ {
		violations[i] = &domain.CapabilityViolation{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 20, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.0, score, "Score should never go below 0")
	mockRepo.AssertExpectations(t)
}

// Test 7: Old violations should not impact score
func TestCalculateCapabilityRisk_OldViolationsIgnored(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.Capability{}, nil)

	// Mock: Violations older than 30 days
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -35), // 35 days ago (outside 30-day window)
		},
		{
			AgentID:   agent.ID,
			Severity:  domain.ViolationSeverityCritical,
			CreatedAt: now.AddDate(0, 0, -60), // 60 days ago
		},
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 2, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Violations older than 30 days should not impact score")
	mockRepo.AssertExpectations(t)
}
```

#### Running the Tests

```bash
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Run specific test file
go test -v ./internal/application/trust_calculator_test.go

# Run with coverage
go test -cover ./internal/application/trust_calculator_test.go

# Run all tests
go test -v ./...
```

**Expected Results**:
- ✅ All 7+ tests pass
- ✅ Test coverage > 90% for calculateCapabilityRisk method
- ✅ Boundary conditions tested (score = 0, score = 0.7, score = 1.0)
- ✅ All capability types tested (high, medium, low risk)
- ✅ Violation scenarios tested (severity, volume, age)

---

### 🎯 Task 2: End-to-End Testing with Chrome DevTools MCP

**Objective**: Verify the complete flow from SDK capability reporting to trust score update using Chrome DevTools MCP for UI testing.

#### Prerequisites

1. **Backend Running**: `http://localhost:8080`
2. **Frontend Running**: `http://localhost:3000`
3. **Database**: PostgreSQL with all migrations applied
4. **User Logged In**: Test user with authentication token

#### 2.1: Capture Baseline Metrics

```typescript
// Navigate to agents dashboard
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents"
})

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Take screenshot
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/test-screenshots/capability-baseline.png"
})

// Note the current trust scores for existing agents
```

#### 2.2: Register Test Agent via SDK

**Option A: Use Existing Python SDK**

```bash
cd /Users/decimai/workspace/agent-identity-management
python3 -c "
from aim_sdk import AIMClient

client = AIMClient()
agent = client.register_agent(
    name='capability-test-agent',
    agent_type='ai_agent',
    description='Test agent for capability detection and trust scoring'
)

print(f'Agent ID: {agent[\"id\"]}')
print(f'Initial Trust Score: {agent.get(\"trust_score\", \"N/A\")}')
"
```

**Note the Agent ID** - you'll need it for subsequent steps.

#### 2.3: Report Low-Risk Capabilities

```python
from aim_sdk import AIMClient
import time

client = AIMClient()
agent_id = "YOUR_AGENT_ID_FROM_STEP_2.2"

# Report low-risk capabilities
print("Reporting low-risk capabilities...")
client.report_capability_detection(
    agent_id=agent_id,
    capabilities=["file:read", "db:query"],
    detection_method="auto",
    confidence=0.95
)

time.sleep(2)  # Wait for processing

# Fetch updated agent to see trust score
agent = client.get_agent(agent_id)
print(f"Trust Score after low-risk capabilities: {agent['trust_score']}")
```

**Expected Result**:
- Trust score should decrease slightly (capability risk: 0.7 - 0.03 - 0.03 = 0.64)
- Overall trust score: ~60-65 (assuming other factors are decent)

#### 2.4: Report High-Risk Capabilities

```python
from aim_sdk import AIMClient
import time

client = AIMClient()
agent_id = "YOUR_AGENT_ID"

# Report high-risk capabilities
print("Reporting high-risk capabilities...")
client.report_capability_detection(
    agent_id=agent_id,
    capabilities=["system:admin", "user:impersonate", "file:delete"],
    detection_method="auto",
    confidence=0.99
)

time.sleep(2)

# Fetch updated agent
agent = client.get_agent(agent_id)
print(f"Trust Score after high-risk capabilities: {agent['trust_score']}")
```

**Expected Result**:
- Trust score should drop significantly (capability risk: 0.7 - 0.20 - 0.20 - 0.15 = 0.15)
- Overall trust score: ~30-40 (high-risk agent)

#### 2.5: Verify Trust Score in Dashboard

```typescript
// Navigate to agent detail page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/YOUR_AGENT_ID"
})

// Take snapshot to see trust score breakdown
mcp__chrome-devtools__take_snapshot()

// Take screenshot
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/test-screenshots/capability-high-risk.png"
})

// Look for trust score display and capability risk factor breakdown
```

**What to Look For**:
- Trust score displayed prominently
- Capability risk factor shown (should be ~0.15 or 15%)
- Other factors unchanged (verification, certificate, etc.)

#### 2.6: Verify Trust Score History in Database

```bash
# Connect to PostgreSQL
psql -h localhost -U postgres -d agent_identity

# Query trust score history
SELECT
    id,
    agent_id,
    score,
    confidence,
    (factors->>'capability_risk')::float AS capability_risk,
    last_calculated,
    created_at
FROM trust_scores
WHERE agent_id = 'YOUR_AGENT_ID'
ORDER BY created_at DESC;

# Should show multiple entries with changing capability_risk values
```

**Expected Output**:
```
                  id                  |              agent_id              | score | confidence | capability_risk |   last_calculated   |      created_at
--------------------------------------+------------------------------------+-------+------------+-----------------+---------------------+---------------------
 uuid-1                               | your-agent-id                      |  0.35 |       0.85 |            0.15 | 2025-10-10 14:30:00 | 2025-10-10 14:30:00
 uuid-2                               | your-agent-id                      |  0.62 |       0.85 |            0.64 | 2025-10-10 14:25:00 | 2025-10-10 14:25:00
 uuid-3                               | your-agent-id                      |  0.68 |       0.85 |            0.70 | 2025-10-10 14:20:00 | 2025-10-10 14:20:00
```

#### 2.7: Test Capability Violations Impact

**Create a violation** (requires backend API or direct database insert):

```sql
-- Insert a CRITICAL violation
INSERT INTO capability_violations (
    agent_id, capability_type, attempted_action,
    severity, description, detected_at
) VALUES (
    'YOUR_AGENT_ID',
    'system:admin',
    'attempted unauthorized system configuration',
    'critical',
    'Agent attempted to modify system settings without authorization',
    NOW()
);
```

**Trigger trust score recalculation**:

```python
from aim_sdk import AIMClient

client = AIMClient()
agent_id = "YOUR_AGENT_ID"

# Report capabilities again (this triggers recalculation)
client.report_capability_detection(
    agent_id=agent_id,
    capabilities=["system:admin"],
    detection_method="auto",
    confidence=0.99
)

# Fetch updated agent
agent = client.get_agent(agent_id)
print(f"Trust Score after violation: {agent['trust_score']}")
```

**Expected Result**:
- Trust score should drop further (violation penalty: -0.15)
- Capability risk now: 0.15 - 0.15 = 0.00 (floor)
- Overall trust score: < 30 (very high risk)

#### 2.8: Verify Security Alerts Created

```typescript
// Navigate to alerts page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/alerts"
})

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Look for security alerts related to capability risks
```

**Expected Alerts**:
- CRITICAL severity alert for system:admin capability
- HIGH severity alert for user:impersonate capability
- Alert for capability violation

---

### 🎯 Task 3: Integration Test for ReportCapabilities Endpoint

**Objective**: Write automated integration test for the complete capability reporting flow.

#### File to Create

**Path**: `apps/backend/tests/integration/capability_reporting_test.go`

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/opena2a/identity/backend/internal/domain"
)

func TestReportCapabilities_UpdatesTrustScore(t *testing.T) {
	// Setup test environment
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create test agent
	agent := createTestAgent(t, &domain.Agent{
		Name:        "capability-test-agent",
		Type:        "ai_agent",
		Description: "Test agent for capability reporting",
		Status:      domain.AgentStatusVerified,
	})

	// 2. Get initial trust score
	initialTrustScore := getAgentTrustScore(t, agent.ID)
	assert.Greater(t, initialTrustScore, 0.0, "Initial trust score should be > 0")

	// 3. Report low-risk capabilities
	capabilityReport := &domain.AgentCapabilityReport{
		DetectedAt: time.Now().Format(time.RFC3339),
		Environment: domain.ProgrammingEnvironment{
			Language: "python",
			Version:  "3.11",
		},
		Capabilities: domain.AgentCapabilities{
			FileSystem: []string{"file:read"},
			Database:   []string{"db:query"},
		},
		RiskAssessment: domain.RiskAssessment{
			OverallRiskScore:   85, // Low risk
			RiskLevel:          "LOW",
			TrustScoreImpact:   -5,
			Alerts:             []domain.SecurityAlert{},
		},
	}

	response := reportCapabilities(t, agent.ID, capabilityReport)
	assert.True(t, response.Success)

	// 4. Verify trust score decreased (low-risk capabilities)
	updatedTrustScore := getAgentTrustScore(t, agent.ID)
	assert.Less(t, updatedTrustScore, initialTrustScore, "Trust score should decrease after capability detection")

	// 5. Report high-risk capabilities
	highRiskReport := &domain.AgentCapabilityReport{
		DetectedAt: time.Now().Format(time.RFC3339),
		Environment: domain.ProgrammingEnvironment{
			Language: "python",
			Version:  "3.11",
		},
		Capabilities: domain.AgentCapabilities{
			FileSystem: []string{"file:delete"},
			System:     []string{"system:admin"},
			Users:      []string{"user:impersonate"},
		},
		RiskAssessment: domain.RiskAssessment{
			OverallRiskScore: 25, // High risk
			RiskLevel:        "CRITICAL",
			TrustScoreImpact: -30,
			Alerts: []domain.SecurityAlert{
				{
					Severity:       "CRITICAL",
					Capability:     "system:admin",
					Message:        "Agent has system administrator privileges",
					Recommendation: "Review and restrict system access",
				},
			},
		},
	}

	response = reportCapabilities(t, agent.ID, highRiskReport)
	assert.True(t, response.Success)

	// 6. Verify trust score dropped significantly
	finalTrustScore := getAgentTrustScore(t, agent.ID)
	assert.Less(t, finalTrustScore, updatedTrustScore, "Trust score should drop significantly with high-risk capabilities")
	assert.Less(t, finalTrustScore, 40.0, "High-risk agent should have trust score < 40")

	// 7. Verify trust score history recorded
	history := getTrustScoreHistory(t, agent.ID)
	assert.GreaterOrEqual(t, len(history), 2, "Should have multiple trust score entries")

	// 8. Verify capability_risk factor present in latest score
	latestScore := history[0]
	assert.NotNil(t, latestScore.Factors.CapabilityRisk)
	assert.Less(t, latestScore.Factors.CapabilityRisk, 0.5, "Capability risk should be < 0.5 for high-risk agent")
}

func TestReportCapabilities_CreatesSecurityAlerts(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	agent := createTestAgent(t, &domain.Agent{
		Name:   "alert-test-agent",
		Type:   "ai_agent",
		Status: domain.AgentStatusVerified,
	})

	// Report CRITICAL capabilities
	capabilityReport := &domain.AgentCapabilityReport{
		DetectedAt: time.Now().Format(time.RFC3339),
		Environment: domain.ProgrammingEnvironment{
			Language: "python",
			Version:  "3.11",
		},
		Capabilities: domain.AgentCapabilities{
			System: []string{"system:admin"},
			Users:  []string{"user:impersonate"},
		},
		RiskAssessment: domain.RiskAssessment{
			OverallRiskScore: 15,
			RiskLevel:        "CRITICAL",
			TrustScoreImpact: -40,
			Alerts: []domain.SecurityAlert{
				{
					Severity:       "CRITICAL",
					Capability:     "system:admin",
					Message:        "Agent has system administrator privileges",
					Recommendation: "Immediate review required",
				},
				{
					Severity:       "HIGH",
					Capability:     "user:impersonate",
					Message:        "Agent can impersonate other users",
					Recommendation: "Restrict impersonation capabilities",
				},
			},
		},
	}

	response := reportCapabilities(t, agent.ID, capabilityReport)
	assert.True(t, response.Success)

	// Verify security alerts created
	alerts := getSecurityAlerts(t, agent.ID)
	assert.GreaterOrEqual(t, len(alerts), 2, "Should have created security alerts")

	// Verify alert severity
	hasCritical := false
	hasHigh := false
	for _, alert := range alerts {
		if alert.Severity == "CRITICAL" {
			hasCritical = true
		}
		if alert.Severity == "HIGH" {
			hasHigh = true
		}
	}

	assert.True(t, hasCritical, "Should have CRITICAL alert")
	assert.True(t, hasHigh, "Should have HIGH alert")
}

// Helper functions

func reportCapabilities(t *testing.T, agentID uuid.UUID, report *domain.AgentCapabilityReport) *domain.CapabilityReportResponse {
	jsonData, _ := json.Marshal(report)

	req, _ := http.NewRequest(
		"POST",
		"http://localhost:8080/api/v1/detection/agents/"+agentID.String()+"/capabilities/report",
		bytes.NewBuffer(jsonData),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", getTestAPIKey(t))

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response domain.CapabilityReportResponse
	json.NewDecoder(resp.Body).Decode(&response)

	return &response
}

func getAgentTrustScore(t *testing.T, agentID uuid.UUID) float64 {
	req, _ := http.NewRequest(
		"GET",
		"http://localhost:8080/api/v1/agents/"+agentID.String(),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+getTestJWT(t))

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var agent domain.Agent
	json.NewDecoder(resp.Body).Decode(&agent)

	return agent.TrustScore
}

func getTrustScoreHistory(t *testing.T, agentID uuid.UUID) []*domain.TrustScore {
	req, _ := http.NewRequest(
		"GET",
		"http://localhost:8080/api/v1/trust-score/agents/"+agentID.String()+"/history",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+getTestJWT(t))

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var history []*domain.TrustScore
	json.NewDecoder(resp.Body).Decode(&history)

	return history
}

func getSecurityAlerts(t *testing.T, agentID uuid.UUID) []*domain.Alert {
	req, _ := http.NewRequest(
		"GET",
		"http://localhost:8080/api/v1/admin/alerts?agent_id="+agentID.String(),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+getTestJWT(t))

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var alerts []*domain.Alert
	json.NewDecoder(resp.Body).Decode(&alerts)

	return alerts
}
```

#### Run Integration Tests

```bash
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Run capability reporting tests
go test -v ./tests/integration/capability_reporting_test.go

# Run all integration tests
go test -v ./tests/integration/...
```

---

### 🎯 Task 4: Monitor in Development Environment

**Objective**: Deploy to development environment and monitor trust score calculations.

#### 4.1: Deploy Backend

```bash
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Build Docker image
docker build -f infrastructure/docker/Dockerfile.backend -t aim-backend:dev .

# Run with Docker Compose
cd /Users/decimai/workspace/agent-identity-management
docker compose -f docker-compose.dev.yml up -d backend

# Check logs
docker compose -f docker-compose.dev.yml logs -f backend
```

#### 4.2: Deploy Frontend

```bash
cd /Users/decimai/workspace/agent-identity-management/apps/web

# Build Docker image
docker build -f infrastructure/docker/Dockerfile.frontend -t aim-frontend:dev .

# Run with Docker Compose
cd /Users/decimai/workspace/agent-identity-management
docker compose -f docker-compose.dev.yml up -d frontend

# Check logs
docker compose -f docker-compose.dev.yml logs -f frontend
```

#### 4.3: Monitor Trust Score Calculations

**Set up monitoring queries**:

```sql
-- Trust score distribution by capability risk
SELECT
    CASE
        WHEN (factors->>'capability_risk')::float >= 0.7 THEN 'Low Risk'
        WHEN (factors->>'capability_risk')::float >= 0.5 THEN 'Medium Risk'
        WHEN (factors->>'capability_risk')::float >= 0.3 THEN 'High Risk'
        ELSE 'Critical Risk'
    END AS risk_category,
    COUNT(*) as agent_count,
    AVG(score) as avg_trust_score
FROM trust_scores ts
INNER JOIN (
    SELECT agent_id, MAX(created_at) as latest
    FROM trust_scores
    GROUP BY agent_id
) latest ON ts.agent_id = latest.agent_id AND ts.created_at = latest.latest
GROUP BY risk_category;

-- Recent trust score changes
SELECT
    a.name,
    ts.score as current_score,
    (ts.factors->>'capability_risk')::float as capability_risk,
    ts.last_calculated
FROM trust_scores ts
INNER JOIN agents a ON ts.agent_id = a.id
ORDER BY ts.last_calculated DESC
LIMIT 20;

-- Agents with declining trust scores
SELECT
    a.name,
    ts1.score as previous_score,
    ts2.score as current_score,
    (ts2.score - ts1.score) as change,
    ts2.last_calculated
FROM trust_scores ts1
INNER JOIN trust_scores ts2 ON ts1.agent_id = ts2.agent_id
INNER JOIN agents a ON ts1.agent_id = a.id
WHERE ts2.created_at > ts1.created_at
  AND (ts2.score - ts1.score) < -0.1  -- Dropped by > 10%
ORDER BY ts2.last_calculated DESC
LIMIT 20;
```

#### 4.4: Performance Monitoring

**Create monitoring dashboard** (Grafana, Prometheus, or custom):

Key metrics to track:
1. **Trust Score Calculation Time**: Should be < 100ms
2. **Capability Risk Calculation Time**: Should be < 50ms
3. **Database Query Performance**: Monitor slow queries
4. **API Response Time**: `/detection/agents/:id/capabilities/report` endpoint
5. **Error Rate**: Track calculation failures

---

### 🎯 Task 5: Production Deployment Checklist

**Objective**: Prepare for and execute production deployment.

#### Pre-Deployment Checklist

- [ ] **All Tests Pass**
  - [ ] Unit tests: `go test ./...`
  - [ ] Integration tests: `go test ./tests/integration/...`
  - [ ] End-to-end tests with Chrome DevTools

- [ ] **Code Review**
  - [ ] Review trust calculator implementation
  - [ ] Review detection service changes
  - [ ] Review dependency injection

- [ ] **Database Migration**
  - [ ] Verify trust_scores table exists
  - [ ] Verify capability_risk column in factors JSON
  - [ ] Backup production database

- [ ] **Performance Testing**
  - [ ] Load test capability reporting endpoint
  - [ ] Verify trust score calculation performance
  - [ ] Check database query performance

- [ ] **Security Review**
  - [ ] Verify capability violations are logged
  - [ ] Verify security alerts are created
  - [ ] Check audit trail completeness

- [ ] **Documentation**
  - [ ] Update API documentation
  - [ ] Update user guide
  - [ ] Update admin guide

#### Deployment Steps

```bash
# 1. Backup production database
pg_dump -h prod-db -U postgres agent_identity > backup_$(date +%Y%m%d).sql

# 2. Build production Docker images
docker build -f infrastructure/docker/Dockerfile.backend -t aim-backend:v1.2.0 .
docker build -f infrastructure/docker/Dockerfile.frontend -t aim-frontend:v1.2.0 .

# 3. Tag and push to registry
docker tag aim-backend:v1.2.0 registry.example.com/aim-backend:v1.2.0
docker tag aim-frontend:v1.2.0 registry.example.com/aim-frontend:v1.2.0
docker push registry.example.com/aim-backend:v1.2.0
docker push registry.example.com/aim-frontend:v1.2.0

# 4. Deploy to Kubernetes
kubectl apply -f infrastructure/k8s/backend-deployment.yaml
kubectl apply -f infrastructure/k8s/frontend-deployment.yaml

# 5. Monitor deployment
kubectl rollout status deployment/aim-backend
kubectl rollout status deployment/aim-frontend

# 6. Verify health checks
curl https://api.aim.example.com/health
curl https://api.aim.example.com/health/ready

# 7. Monitor logs
kubectl logs -f deployment/aim-backend
```

#### Post-Deployment Verification

```bash
# 1. Run smoke tests
curl -X POST https://api.aim.example.com/api/v1/detection/agents/TEST_AGENT_ID/capabilities/report \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d @capability-report.json

# 2. Verify trust scores updating
psql -h prod-db -U postgres -d agent_identity -c "
SELECT COUNT(*) FROM trust_scores WHERE created_at > NOW() - INTERVAL '5 minutes';
"

# 3. Check error logs
kubectl logs deployment/aim-backend | grep ERROR

# 4. Monitor metrics
# Open Grafana dashboard and verify:
# - Trust score calculation time < 100ms
# - No error rate spike
# - API response times normal
```

#### Rollback Plan

If issues are detected:

```bash
# Rollback Kubernetes deployment
kubectl rollout undo deployment/aim-backend
kubectl rollout undo deployment/aim-frontend

# Verify rollback
kubectl rollout status deployment/aim-backend
kubectl rollout status deployment/aim-frontend

# Restore database if needed
psql -h prod-db -U postgres agent_identity < backup_20251010.sql
```

---

## Success Criteria

### Unit Tests ✅
- [ ] All 7+ unit tests pass
- [ ] Test coverage > 90% for calculateCapabilityRisk
- [ ] All edge cases tested

### Integration Tests ✅
- [ ] ReportCapabilities endpoint test passes
- [ ] Trust score updates correctly
- [ ] Security alerts created
- [ ] Trust score history recorded

### End-to-End Tests ✅
- [ ] SDK capability reporting works
- [ ] Dashboard displays updated trust scores
- [ ] Trust score breakdown shows capability_risk factor
- [ ] Database shows historical trust scores

### Performance ✅
- [ ] Trust score calculation < 100ms
- [ ] API response time < 200ms
- [ ] No database query bottlenecks

### Production Deployment ✅
- [ ] Successful deployment with no downtime
- [ ] Trust scores calculating correctly
- [ ] No error rate increase
- [ ] All monitoring metrics green

---

## Environment Details

### Working Directory
```
/Users/decimai/workspace/agent-identity-management/
```

### Key Files
- Trust Calculator: `apps/backend/internal/application/trust_calculator.go`
- Detection Service: `apps/backend/internal/application/detection_service.go`
- Trust Score Domain: `apps/backend/internal/domain/trust_score.go`
- Main Initialization: `apps/backend/cmd/server/main.go`
- Integration Doc: `CAPABILITY_TRUST_INTEGRATION.md`

### Database
- **Host**: localhost
- **Port**: 5432
- **Database**: agent_identity
- **User**: postgres

### API Endpoints
- Report Capabilities: `POST /api/v1/detection/agents/:id/capabilities/report`
- Get Trust Score: `GET /api/v1/trust-score/agents/:id`
- Trust Score History: `GET /api/v1/trust-score/agents/:id/history`

### Frontend URLs
- Dashboard: `http://localhost:3000/dashboard`
- Agents: `http://localhost:3000/dashboard/agents`
- Alerts: `http://localhost:3000/dashboard/alerts`

---

## Important Notes

### 1. Trust Score Scale Conversion
- Internal calculation: **0-1 scale** (trust_calculator.go)
- Database storage: **0-100 scale** (agents table)
- Frontend display: **0-100 scale** (dashboard)

**Conversion in detection_service.go:386**:
```go
newTrustScore := trustScore.Score * 100
```

### 2. Capability Risk is Inverted
- **1.0 = Low Risk** (good - no high-risk capabilities)
- **0.5 = Medium Risk** (some concerning capabilities)
- **0.0 = High Risk** (many dangerous capabilities and violations)

### 3. Violation Window
- Only violations from **last 30 days** impact trust score
- Older violations are ignored
- Implemented in: `trust_calculator.go:309`

### 4. Security Alerts
- Only **CRITICAL** and **HIGH** severity alerts are created
- Implemented in: `detection_service.go:432-443`

---

## Questions to Ask User If Stuck

1. **Database Connection Issues**: "Can you verify PostgreSQL is running and migrations are applied?"
2. **API Key Authentication**: "Do you have a valid SDK API key for testing?"
3. **Frontend Not Loading**: "Can you confirm the frontend is running on port 3000?"
4. **Test Failures**: "Can you share the exact error message from the failing test?"
5. **Performance Issues**: "Are there any slow query logs in PostgreSQL?"

---

## Timeline Estimate

- **Task 1** (Unit Tests): 45-60 minutes
- **Task 2** (E2E Testing): 60-90 minutes
- **Task 3** (Integration Tests): 30-45 minutes
- **Task 4** (Dev Monitoring): 30 minutes
- **Task 5** (Production): 60 minutes
- **Total**: 4-5 hours

---

## Final Note

The capability-trust score integration is **code-complete and compiles successfully**. Your mission is to:
1. **Prove it works** through comprehensive testing
2. **Monitor it in dev** to catch any edge cases
3. **Deploy to production** with confidence

Good luck! 🚀

