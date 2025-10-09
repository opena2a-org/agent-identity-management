# Automatic Capability Verification System - Design Document

## Overview
This document outlines the design for AIM's automatic capability verification system, which captures agent/MCP identity signatures during registration and monitors for out-of-scope actions.

## Core Concepts

### 1. Identity Signature Capture (Registration)
When an agent or MCP server registers with AIM, the system automatically:
- Generates a cryptographic key pair (or accepts user-provided public key)
- Stores the public key in the database
- Associates the key with the agent/MCP identity
- **No manual verification needed** - automatic and seamless

### 2. Capability Declaration
During or after registration, agents/MCPs declare their intended capabilities:
- **Capability**: A specific action or resource type the service intends to access
- Examples:
  - `file:read` - Read files
  - `file:write` - Write files
  - `api:call` - Make API calls
  - `db:query` - Query database
  - `user:impersonate` - Act on behalf of users
  - `data:export` - Export data

### 3. Action Verification Flow
Every time a service communicates with AIM:
1. Service signs request with its private key
2. AIM verifies signature using stored public key (identity verification)
3. Service declares what action it wants to perform
4. AIM checks if action is within registered capability scope
5. If **out-of-scope**: Log incident, decrease trust score, optionally block

### 4. Trust Score Impact
- **In-scope action**: No change to trust score
- **First out-of-scope action**: -10 points, warning logged
- **Second out-of-scope action**: -20 points, alert generated
- **Third out-of-scope action**: -30 points, service marked as "potentially compromised"
- **Persistent violations**: Service automatically suspended

## Database Schema

### New Table: `agent_capabilities`
```sql
CREATE TABLE agent_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL, -- e.g., "file:read", "api:call"
    capability_scope JSONB, -- Additional scope restrictions
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_agent_capability UNIQUE(agent_id, capability_type)
);

CREATE INDEX idx_agent_capabilities_agent_id ON agent_capabilities(agent_id);
CREATE INDEX idx_agent_capabilities_type ON agent_capabilities(capability_type);
```

### New Table: `capability_violations`
```sql
CREATE TABLE capability_violations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    attempted_capability VARCHAR(100) NOT NULL,
    registered_capabilities JSONB, -- Snapshot of what was registered
    severity VARCHAR(20) NOT NULL DEFAULT 'medium', -- low, medium, high, critical
    trust_score_impact INT NOT NULL DEFAULT -10,
    is_blocked BOOLEAN NOT NULL DEFAULT false,
    source_ip VARCHAR(45),
    request_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_severity CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE INDEX idx_capability_violations_agent_id ON capability_violations(agent_id);
CREATE INDEX idx_capability_violations_created_at ON capability_violations(created_at);
CREATE INDEX idx_capability_violations_severity ON capability_violations(severity);
```

### Update `agents` table:
```sql
ALTER TABLE agents
ADD COLUMN public_key TEXT, -- Ed25519 or RSA public key
ADD COLUMN key_algorithm VARCHAR(20) DEFAULT 'Ed25519', -- Ed25519, RSA, ECDSA
ADD COLUMN last_capability_check_at TIMESTAMPTZ,
ADD COLUMN capability_violation_count INT DEFAULT 0,
ADD COLUMN is_compromised BOOLEAN DEFAULT false;
```

## API Endpoints

### 1. Register Agent with Capabilities
```
POST /api/v1/agents/register
```
**Request:**
```json
{
  "displayName": "File Processing Agent",
  "type": "ai_agent",
  "publicKey": "base64-encoded-public-key",
  "keyAlgorithm": "Ed25519",
  "capabilities": [
    "file:read",
    "file:write",
    "api:call"
  ]
}
```

**Response:**
```json
{
  "id": "agt_123",
  "displayName": "File Processing Agent",
  "publicKey": "...",
  "capabilities": [
    {
      "id": "cap_456",
      "type": "file:read",
      "grantedAt": "2025-01-20T10:00:00Z"
    }
  ],
  "trustScore": 100.0
}
```

### 2. Verify Action (Internal Middleware)
```
POST /api/v1/internal/verify-action
```
**Request:**
```json
{
  "agentId": "agt_123",
  "signature": "base64-encoded-signature",
  "requestPayload": "base64-encoded-payload",
  "requestedCapability": "file:read",
  "metadata": {
    "sourceIp": "192.168.1.10",
    "requestId": "req_789"
  }
}
```

**Response:**
```json
{
  "isValid": true,
  "isAuthorized": true,
  "inScope": true,
  "trustScore": 100.0
}
```

### 3. Get Agent Capabilities
```
GET /api/v1/agents/:id/capabilities
```

### 4. Add Capability
```
POST /api/v1/agents/:id/capabilities
```

### 5. Revoke Capability
```
DELETE /api/v1/agents/:id/capabilities/:capabilityId
```

### 6. List Capability Violations
```
GET /api/v1/capability-violations
```

## Backend Services

### 1. CapabilityService
```go
type CapabilityService struct {
    capabilityRepo domain.CapabilityRepository
    agentRepo      domain.AgentRepository
    auditRepo      domain.AuditLogRepository
}

func (s *CapabilityService) VerifyAction(
    agentID uuid.UUID,
    requestedCapability string,
    signature []byte,
    payload []byte,
) (*VerificationResult, error) {
    // 1. Verify signature (identity)
    agent, err := s.agentRepo.GetByID(agentID)
    if err != nil {
        return nil, err
    }

    if !verifySignature(agent.PublicKey, signature, payload) {
        return &VerificationResult{IsValid: false}, nil
    }

    // 2. Check capability scope
    capabilities, err := s.capabilityRepo.GetByAgentID(agentID)
    if err != nil {
        return nil, err
    }

    inScope := hasCapability(capabilities, requestedCapability)

    if !inScope {
        // 3. Log violation
        violation := &domain.CapabilityViolation{
            AgentID:             agentID,
            AttemptedCapability: requestedCapability,
            RegisteredCapabilities: capabilitiesToJSON(capabilities),
            Severity:            calculateSeverity(agent),
            TrustScoreImpact:   -10,
        }
        s.capabilityRepo.CreateViolation(violation)

        // 4. Decrease trust score
        newTrustScore := agent.TrustScore - 10
        s.agentRepo.UpdateTrustScore(agentID, newTrustScore)

        // 5. Log to audit
        s.auditRepo.Create(&domain.AuditLog{
            OrganizationID: agent.OrganizationID,
            Action:         "capability_violation",
            TargetType:     "agent",
            TargetID:       agentID,
            Severity:       "high",
            Description:    fmt.Sprintf("Agent attempted %s (not in scope)", requestedCapability),
        })
    }

    return &VerificationResult{
        IsValid:      true,
        IsAuthorized: inScope,
        InScope:      inScope,
        TrustScore:   agent.TrustScore,
    }, nil
}
```

### 2. Signature Verification
```go
func verifySignature(publicKey string, signature []byte, payload []byte) bool {
    // Ed25519 verification
    pubKey, err := decodePublicKey(publicKey)
    if err != nil {
        return false
    }

    return ed25519.Verify(pubKey, payload, signature)
}
```

## Frontend Integration

### 1. Security Page Updates
- Add "Capability Violations" section
- Display recent violations with:
  - Agent name
  - Attempted capability
  - Registered capabilities
  - Trust score impact
  - Timestamp

### 2. Agent Detail Page Updates
- Show registered capabilities list
- Show violation history
- Allow admins to add/revoke capabilities

### 3. Registration Flow Updates
- Add capability selection during agent registration
- Provide UI for declaring capabilities
- Generate or accept public key

## Migration Plan

### Phase 1: Database Schema (Week 1)
1. Create `agent_capabilities` table
2. Create `capability_violations` table
3. Update `agents` table with new columns
4. Create indexes

### Phase 2: Backend Implementation (Week 2)
1. Implement CapabilityService
2. Implement CapabilityRepository
3. Create signature verification logic
4. Add middleware for action verification

### Phase 3: API Endpoints (Week 2-3)
1. Implement capability CRUD endpoints
2. Implement violation tracking endpoints
3. Update agent registration endpoint
4. Add verification endpoint

### Phase 4: Frontend Integration (Week 3-4)
1. Update Security page with violations
2. Update agent registration flow
3. Add capability management UI
4. Add violation alerts

### Phase 5: Testing & Rollout (Week 4)
1. Unit tests for all services
2. Integration tests for verification flow
3. End-to-end tests
4. Documentation
5. Gradual rollout to existing agents

## Security Considerations

### 1. Key Storage
- **Never** store private keys in AIM
- Only store public keys
- Private keys remain with agent/MCP owners

### 2. Signature Verification
- Use industry-standard algorithms (Ed25519 recommended)
- Verify signatures on every request
- Implement replay attack prevention (nonces)

### 3. Trust Score Thresholds
- Automatic suspension at trust score < 30
- Automatic alert generation at < 50
- Automatic review required at < 70

### 4. False Positives
- Allow admins to mark violations as false positives
- Don't penalize trust score for false positives
- Track false positive rate per agent

## Success Metrics

1. **Adoption Rate**: % of agents with registered capabilities
2. **Violation Detection Rate**: # of out-of-scope actions detected per day
3. **False Positive Rate**: % of violations marked as false positives
4. **Trust Score Impact**: Average trust score decrease for violators
5. **Compromise Detection**: # of compromised agents detected early

## Future Enhancements

1. **Dynamic Capability Requests**: Agents can request temporary capabilities
2. **Capability Templates**: Pre-defined capability sets for common agent types
3. **ML-Based Anomaly Detection**: Use ML to detect suspicious patterns
4. **Capability Inheritance**: Parent-child relationships for capability delegation
5. **Time-Based Capabilities**: Capabilities that expire after a certain time

---

**Status**: Design Complete - Ready for Implementation
**Created**: January 20, 2025
**Last Updated**: January 20, 2025
**Owner**: AIM Core Team
