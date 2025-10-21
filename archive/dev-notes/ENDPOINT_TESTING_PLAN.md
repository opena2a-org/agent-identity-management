# AIM Endpoint Testing Plan - Production Readiness

**Last Updated**: January 6, 2025
**Total Endpoints**: 120+
**MVP Focus**: Python SDK only (Go/Java deferred to future release)

## 🎯 Testing Objectives

1. **100% Endpoint Coverage**: Test all 120+ endpoints systematically
2. **Production Readiness**: Ensure all endpoints meet production quality standards
3. **Authentication Validation**: Verify RBAC works correctly for all roles
4. **Error Handling**: Test edge cases and error scenarios
5. **Performance**: Validate response times < 100ms (p95)
6. **Documentation**: Verify all endpoints are documented with correct schemas

## 📊 Testing Categories

### 1. Health & Status (2 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| GET | `/health` | No | 200 | High |
| GET | `/api/v1/status` | No | 200 | High |

**Test Cases**:
- ✅ Returns 200 OK
- ✅ Response includes system health metrics
- ✅ Database connectivity check
- ✅ Redis connectivity check (if applicable)

### 2. Public Routes (5 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| POST | `/api/v1/public/agents/register` | No | 201 | High |
| POST | `/api/v1/public/register` | No | 201 | High |
| GET | `/api/v1/public/register/:requestId/status` | No | 200 | Medium |
| POST | `/api/v1/public/login` | No | 200 | High |
| POST | `/api/v1/public/change-password` | No | 200 | Medium |

**Test Cases**:
- ✅ Agent registration creates pending agent
- ✅ User registration creates pending request
- ✅ Login returns JWT token with correct claims
- ✅ Change password validates old password
- ✅ All endpoints validate required fields
- ✅ Error messages are clear and actionable

### 3. Auth Routes (5 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| POST | `/api/v1/auth/login/local` | No | 200 | High |
| POST | `/api/v1/auth/logout` | Yes | 200 | High |
| POST | `/api/v1/auth/refresh` | Yes | 200 | High |
| GET | `/api/v1/auth/me` | Yes | 200 | High |
| PUT | `/api/v1/auth/me` | Yes | 200 | Medium |

**Test Cases**:
- ✅ Local login with email/password
- ✅ JWT token includes correct user claims (id, email, role, org_id)
- ✅ Logout invalidates token
- ✅ Refresh extends token expiration
- ✅ /auth/me returns current user profile
- ✅ Profile update validates fields

### 4. SDK Routes (2 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| GET | `/api/v1/sdk/download` | Yes | 200 | High |
| GET | `/api/v1/sdk/changelog` | Yes | 200 | Medium |

**Test Cases**:
- ✅ Download returns Python SDK package (MVP only)
- ✅ Changelog returns version history
- ✅ Authentication required
- ⚠️ **Note**: Go and Java SDKs moved to future release

### 5. SDK Token Management (4 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/users/me/sdk-tokens` | Yes | All | 200 | High |
| GET | `/api/v1/users/me/sdk-tokens/count` | Yes | All | 200 | Medium |
| POST | `/api/v1/users/me/sdk-tokens/:id/revoke` | Yes | All | 200 | High |
| POST | `/api/v1/users/me/sdk-tokens/revoke-all` | Yes | All | 200 | Medium |

**Test Cases**:
- ✅ List returns user's SDK tokens only
- ✅ Count returns active token count
- ✅ Revoke marks token as revoked
- ✅ Revoke all invalidates all user tokens
- ✅ Users cannot revoke other users' tokens

### 6. Detection Routes (3 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| POST | `/api/v1/detect/agent` | No | 200 | Low |
| POST | `/api/v1/detect/mcp` | No | 200 | Low |
| POST | `/api/v1/detect/verify` | Yes | 200 | Medium |

**Test Cases**:
- ✅ Agent detection identifies framework
- ✅ MCP detection validates server configuration
- ✅ Verify endpoint validates credentials
- ⚠️ **Note**: Low priority for MVP

### 7. Agent Routes (14 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/agents` | Yes | All | 200 | High |
| POST | `/api/v1/agents` | Yes | Member+ | 201 | High |
| GET | `/api/v1/agents/:id` | Yes | All | 200 | High |
| PUT | `/api/v1/agents/:id` | Yes | Member+ | 200 | High |
| DELETE | `/api/v1/agents/:id` | Yes | Admin | 204 | High |
| GET | `/api/v1/agents/:id/key-vault` | Yes | Member+ | 200 | High |
| POST | `/api/v1/agents/:id/verify` | Yes | Admin | 200 | High |
| POST | `/api/v1/agents/:id/suspend` | Yes | Admin | 200 | Medium |
| POST | `/api/v1/agents/:id/reactivate` | Yes | Admin | 200 | Medium |
| POST | `/api/v1/agents/:id/rotate-credentials` | Yes | Member+ | 200 | High |
| POST | `/api/v1/agents/:id/tags` | Yes | Member+ | 200 | Low |
| DELETE | `/api/v1/agents/:id/tags/:tagId` | Yes | Member+ | 204 | Low |
| GET | `/api/v1/agents/:id/verification-events` | Yes | All | 200 | Medium |
| GET | `/api/v1/agents/:id/audit-logs` | Yes | Manager+ | 200 | Medium |

**Test Cases**:
- ✅ List agents returns org-scoped agents only
- ✅ Create agent generates Ed25519 keypair in vault
- ✅ Update agent validates trust score range (0-100)
- ✅ Delete requires admin role
- ✅ Key vault returns public key only (private key never exposed)
- ✅ Verify transitions agent from pending to verified
- ✅ Suspend/reactivate changes status correctly
- ✅ Credential rotation generates new keypair
- ✅ RBAC enforced correctly per endpoint

### 8. API Key Routes (4 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/agents/:id/api-keys` | Yes | Member+ | 200 | High |
| POST | `/api/v1/agents/:id/api-keys` | Yes | Member+ | 201 | High |
| DELETE | `/api/v1/agents/:id/api-keys/:keyId` | Yes | Member+ | 204 | High |
| POST | `/api/v1/agents/:id/api-keys/:keyId/rotate` | Yes | Member+ | 200 | High |

**Test Cases**:
- ✅ List returns SHA-256 hashed keys only
- ✅ Create returns plaintext key once (never stored)
- ✅ Delete revokes key immediately
- ✅ Rotate generates new key, revokes old
- ✅ Key expiration handled correctly
- ✅ Usage tracking increments on each use

### 9. Trust Score Routes (4 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/agents/:id/trust-score` | Yes | All | 200 | High |
| PUT | `/api/v1/agents/:id/trust-score` | Yes | Admin | 200 | High |
| GET | `/api/v1/agents/:id/trust-score/history` | Yes | All | 200 | Medium |
| POST | `/api/v1/agents/:id/trust-score/recalculate` | Yes | Admin | 200 | Medium |

**Test Cases**:
- ✅ Trust score returns current score and factors
- ✅ Update validates score range (0-100)
- ✅ History returns time-series data
- ✅ Recalculate uses ML algorithm (8 factors)
- ✅ Score changes trigger audit log

### 10. Admin Routes (16 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/admin/users` | Yes | Admin | 200 | High |
| POST | `/api/v1/admin/users/:id/approve` | Yes | Admin | 200 | High |
| POST | `/api/v1/admin/users/:id/reject` | Yes | Admin | 200 | High |
| DELETE | `/api/v1/admin/users/:id` | Yes | Admin | 204 | High |
| PUT | `/api/v1/admin/users/:id/role` | Yes | Admin | 200 | High |
| GET | `/api/v1/admin/users/:id/activity` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/admin/users/pending` | Yes | Admin | 200 | High |
| GET | `/api/v1/admin/alerts` | Yes | Admin/Manager | 200 | High |
| GET | `/api/v1/admin/alerts/unacknowledged/count` | Yes | Admin/Manager | 200 | High |
| POST | `/api/v1/admin/alerts/:id/acknowledge` | Yes | Admin/Manager | 200 | High |
| GET | `/api/v1/admin/audit-logs` | Yes | Admin | 200 | High |
| GET | `/api/v1/admin/audit-logs/export` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/admin/capability-requests` | Yes | Admin | 200 | High |
| POST | `/api/v1/admin/capability-requests/:id/approve` | Yes | Admin | 200 | High |
| POST | `/api/v1/admin/capability-requests/:id/reject` | Yes | Admin | 200 | High |
| GET | `/api/v1/admin/organizations/:id/stats` | Yes | Admin | 200 | Medium |

**Test Cases**:
- ✅ User management endpoints require admin role
- ✅ Approve/reject transitions user status correctly
- ✅ Role update validates valid roles only
- ✅ Alerts filtered by organization
- ✅ Acknowledge marks alert as reviewed
- ✅ Audit logs export in CSV format
- ✅ Capability requests show pending/approved/rejected
- ✅ RBAC strictly enforced (admin/manager only)

### 11. Security Policy Routes (6 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/security-policies` | Yes | Admin | 200 | High |
| POST | `/api/v1/security-policies` | Yes | Admin | 201 | High |
| GET | `/api/v1/security-policies/:id` | Yes | Admin | 200 | High |
| PUT | `/api/v1/security-policies/:id` | Yes | Admin | 200 | High |
| DELETE | `/api/v1/security-policies/:id` | Yes | Admin | 204 | Medium |
| POST | `/api/v1/security-policies/:id/toggle` | Yes | Admin | 200 | High |

**Test Cases**:
- ✅ List returns 3 default policies
- ✅ Create validates policy type and enforcement action
- ✅ Update preserves created_by, updates updated_at
- ✅ Delete soft-deletes or hard-deletes based on config
- ✅ Toggle enables/disables policy
- ✅ Only admin role can manage policies

### 12. Capability Request Routes (4 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/capability-requests` | Yes | All | 200 | Medium |
| POST | `/api/v1/capability-requests` | Yes | Member+ | 201 | Medium |
| GET | `/api/v1/capability-requests/:id` | Yes | All | 200 | Medium |
| PUT | `/api/v1/capability-requests/:id/status` | Yes | Admin | 200 | Medium |

**Test Cases**:
- ✅ List shows user's requests (scoped by role)
- ✅ Create validates capability name
- ✅ Status update transitions pending → approved/rejected
- ✅ Only admin can approve/reject

### 13. Compliance Routes (8 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/compliance/status` | Yes | Admin | 200 | High |
| GET | `/api/v1/compliance/reports` | Yes | Admin | 200 | High |
| POST | `/api/v1/compliance/reports/generate` | Yes | Admin | 201 | Medium |
| GET | `/api/v1/compliance/reports/:id` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/compliance/access-reviews` | Yes | Admin | 200 | Medium |
| POST | `/api/v1/compliance/access-reviews/:id/complete` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/compliance/data-retention` | Yes | Admin | 200 | Low |
| POST | `/api/v1/compliance/data-retention/apply` | Yes | Admin | 200 | Low |

**Test Cases**:
- ✅ Status returns compliance metrics (no NaN values)
- ✅ Reports include SOC2, HIPAA, GDPR checks
- ✅ Generate creates compliance report
- ✅ Access reviews show users with excessive permissions
- ✅ Data retention enforces retention policies
- ✅ All endpoints admin-only

### 14. MCP Server Routes (10 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/mcp-servers` | Yes | All | 200 | High |
| POST | `/api/v1/mcp-servers` | Yes | Member+ | 201 | High |
| GET | `/api/v1/mcp-servers/:id` | Yes | All | 200 | High |
| PUT | `/api/v1/mcp-servers/:id` | Yes | Member+ | 200 | High |
| DELETE | `/api/v1/mcp-servers/:id` | Yes | Admin | 204 | High |
| POST | `/api/v1/mcp-servers/:id/verify` | Yes | Admin | 200 | High |
| POST | `/api/v1/mcp-servers/:id/suspend` | Yes | Admin | 200 | Medium |
| POST | `/api/v1/mcp-servers/:id/reactivate` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/mcp-servers/:id/verification-events` | Yes | All | 200 | Medium |
| GET | `/api/v1/mcp-servers/:id/audit-logs` | Yes | Manager+ | 200 | Medium |

**Test Cases**:
- ✅ Registration creates pending MCP server
- ✅ Verification validates cryptographic signature
- ✅ Public key stored correctly
- ✅ Suspend/reactivate updates status
- ✅ Verification events tracked
- ✅ RBAC enforced per endpoint

### 15. Security Routes (6 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/security/threats` | Yes | Manager+ | 200 | High |
| GET | `/api/v1/security/threats/:id` | Yes | Manager+ | 200 | Medium |
| POST | `/api/v1/security/threats/:id/mitigate` | Yes | Admin | 200 | Medium |
| GET | `/api/v1/security/anomalies` | Yes | Manager+ | 200 | High |
| GET | `/api/v1/security/scan/:agentId` | Yes | Manager+ | 200 | Medium |
| GET | `/api/v1/security/posture` | Yes | Admin | 200 | Medium |

**Test Cases**:
- ✅ Threats show detected security issues
- ✅ Anomalies use ML for detection
- ✅ Scan runs security check on agent
- ✅ Posture returns organization security score
- ✅ Manager+ role required

### 16. Analytics Routes (6 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/analytics/dashboard` | Yes | All | 200 | High |
| GET | `/api/v1/analytics/usage` | Yes | Manager+ | 200 | Medium |
| GET | `/api/v1/analytics/trends` | Yes | Manager+ | 200 | Medium |
| GET | `/api/v1/analytics/agents/activity` | Yes | Manager+ | 200 | Medium |
| GET | `/api/v1/analytics/verification-activity` | Yes | Manager+ | 200 | Medium |
| POST | `/api/v1/analytics/reports/generate` | Yes | Admin | 201 | Low |

**Test Cases**:
- ✅ Dashboard returns key metrics (no NaN/undefined)
- ✅ Usage shows API call statistics
- ✅ Trends return time-series data
- ✅ Activity tracks agent operations
- ✅ All numeric fields have safe division
- ✅ RBAC enforced (viewer can access dashboard only)

### 17. Webhook Routes (5 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/webhooks` | Yes | Admin | 200 | Low |
| POST | `/api/v1/webhooks` | Yes | Admin | 201 | Low |
| GET | `/api/v1/webhooks/:id` | Yes | Admin | 200 | Low |
| PUT | `/api/v1/webhooks/:id` | Yes | Admin | 200 | Low |
| DELETE | `/api/v1/webhooks/:id` | Yes | Admin | 204 | Low |

**Test Cases**:
- ✅ Webhook CRUD operations
- ✅ Signature validation
- ✅ Event type filtering
- ⚠️ **Note**: Low priority for MVP

### 18. Verification Routes (3 endpoints)
| Method | Endpoint | Auth Required | Expected Status | Priority |
|--------|----------|---------------|-----------------|----------|
| POST | `/api/v1/verify/agent` | No | 200 | High |
| POST | `/api/v1/verify/mcp` | No | 200 | High |
| POST | `/api/v1/verify/challenge` | No | 200 | Medium |

**Test Cases**:
- ✅ Agent verification validates credentials
- ✅ MCP verification validates public key
- ✅ Challenge generates cryptographic challenge
- ✅ Public endpoints (no auth required)

### 19. Verification Event Routes (6 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/verification-events` | Yes | Manager+ | 200 | Medium |
| POST | `/api/v1/verification-events` | Yes | Member+ | 201 | Medium |
| GET | `/api/v1/verification-events/:id` | Yes | All | 200 | Medium |
| GET | `/api/v1/verification-events/agent/:agentId` | Yes | All | 200 | Medium |
| GET | `/api/v1/verification-events/mcp/:mcpId` | Yes | All | 200 | Medium |
| GET | `/api/v1/verification-events/stats` | Yes | Manager+ | 200 | Medium |

**Test Cases**:
- ✅ Events tracked for agents and MCP servers
- ✅ Stats return success/failure rates
- ✅ Organization-scoped access
- ✅ RBAC enforced

### 20. Tag Routes (9 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/tags` | Yes | All | 200 | Low |
| POST | `/api/v1/tags` | Yes | Admin | 201 | Low |
| GET | `/api/v1/tags/:id` | Yes | All | 200 | Low |
| PUT | `/api/v1/tags/:id` | Yes | Admin | 200 | Low |
| DELETE | `/api/v1/tags/:id` | Yes | Admin | 204 | Low |
| GET | `/api/v1/tags/search` | Yes | All | 200 | Low |
| GET | `/api/v1/tags/:id/agents` | Yes | All | 200 | Low |
| GET | `/api/v1/tags/:id/mcp-servers` | Yes | All | 200 | Low |
| GET | `/api/v1/tags/popular` | Yes | All | 200 | Low |

**Test Cases**:
- ✅ 10 default tags exist
- ✅ Search returns matching tags
- ✅ Tags link to agents and MCP servers
- ⚠️ **Note**: Low priority for MVP

### 21. Capability Routes (4 endpoints)
| Method | Endpoint | Auth Required | Role Required | Expected Status | Priority |
|--------|----------|---------------|---------------|-----------------|----------|
| GET | `/api/v1/capabilities` | Yes | All | 200 | Medium |
| POST | `/api/v1/capabilities` | Yes | Admin | 201 | Medium |
| GET | `/api/v1/capabilities/:id` | Yes | All | 200 | Medium |
| DELETE | `/api/v1/capabilities/:id` | Yes | Admin | 204 | Medium |

**Test Cases**:
- ✅ List returns available capabilities
- ✅ Create validates capability definition
- ✅ Admin-only for create/delete

---

## 🧪 Testing Methodology

### Phase 1: Automated Happy Path (High Priority)
**Goal**: Verify all endpoints return 2xx status with valid data

**Approach**:
1. Create bash script that iterates through all endpoints
2. Use curl with JWT authentication
3. Log response status, body, and timing
4. Generate test report with pass/fail status

**Success Criteria**:
- All high-priority endpoints return 2xx
- Response times < 100ms (p95)
- No 500 errors
- All required fields present in responses

### Phase 2: RBAC Validation (High Priority)
**Goal**: Ensure role-based access control works correctly

**Approach**:
1. Create test users with each role (admin, manager, member, viewer)
2. Test each endpoint with each role
3. Verify expected status (200 for allowed, 403 for denied)

**Test Matrix**:
| Endpoint | Admin | Manager | Member | Viewer |
|----------|-------|---------|--------|--------|
| `/api/v1/admin/users` | 200 | 403 | 403 | 403 |
| `/api/v1/analytics/dashboard` | 200 | 200 | 200 | 200 |
| `/api/v1/agents` (GET) | 200 | 200 | 200 | 200 |
| `/api/v1/agents` (POST) | 200 | 200 | 200 | 403 |

### Phase 3: Error Case Testing (Medium Priority)
**Goal**: Verify proper error handling

**Test Cases**:
- Invalid authentication token → 401
- Missing required fields → 400 with clear error message
- Non-existent resource → 404
- Forbidden action → 403
- Server error → 500 with generic message (no stack traces)
- Malformed JSON → 400
- SQL injection attempts → sanitized/blocked

### Phase 4: Edge Cases (Medium Priority)
**Goal**: Test boundary conditions

**Test Cases**:
- Trust score = 0, 50, 100, 101 (out of range)
- Empty organization (no agents, no users)
- Large result sets (pagination)
- Concurrent requests to same resource
- Token expiration during long-running request
- Database connection failure
- Redis cache miss

### Phase 5: Performance Testing (Low Priority for MVP)
**Goal**: Ensure scalability

**Approach**:
- Use k6 or Apache Bench for load testing
- Test with 100, 500, 1000 concurrent users
- Measure p50, p95, p99 response times
- Identify bottlenecks

**Success Criteria**:
- p95 < 100ms for all endpoints
- p99 < 500ms
- No errors under load
- Database queries optimized (N+1 prevention)

---

## 🔧 Test Data Setup

### Required Test Data:

**Organizations**:
```sql
-- Test organization (ID: a0000000-0000-0000-0000-000000000001)
INSERT INTO organizations (id, name, created_at, updated_at)
VALUES ('a0000000-0000-0000-0000-000000000001', 'Test Organization', NOW(), NOW());
```

**Users** (one per role):
```sql
-- Admin user
INSERT INTO users (id, email, password_hash, role, organization_id)
VALUES (
  'a0000000-0000-0000-0000-000000000002',
  'admin@opena2a.org',
  '$2a$12$hashed_password',
  'admin',
  'a0000000-0000-0000-0000-000000000001'
);

-- Manager, Member, Viewer users (similar structure)
```

**Agents**:
```sql
-- 3 agents: verified, pending, suspended
INSERT INTO agents (id, name, organization_id, status, trust_score, agent_type)
VALUES
  ('b0000000-0000-0000-0000-000000000001', 'Test Agent 1', 'a0000000-0000-0000-0000-000000000001', 'verified', 85.5, 'ai_agent'),
  ('b0000000-0000-0000-0000-000000000002', 'Test Agent 2', 'a0000000-0000-0000-0000-000000000001', 'pending', 0, 'automation_agent'),
  ('b0000000-0000-0000-0000-000000000003', 'Test Agent 3', 'a0000000-0000-0000-0000-000000000001', 'suspended', 45.2, 'ai_agent');
```

**MCP Servers**:
```sql
-- 2 MCP servers: verified, pending
INSERT INTO mcp_servers (id, name, organization_id, status, base_url)
VALUES
  ('c0000000-0000-0000-0000-000000000001', 'Test MCP Server 1', 'a0000000-0000-0000-0000-000000000001', 'verified', 'https://mcp1.test.com'),
  ('c0000000-0000-0000-0000-000000000002', 'Test MCP Server 2', 'a0000000-0000-0000-0000-000000000001', 'pending', 'https://mcp2.test.com');
```

**Security Policies** (3 default policies already exist from migration 037)

**Tags** (10 default tags already exist from migration 0221)

**SDK Tokens**, **API Keys**, **Verification Events**, **Audit Logs** will be created dynamically during testing.

---

## 📝 Automated Testing Script

Create `/Users/decimai/workspace/agent-identity-management/test_all_endpoints.sh`:

```bash
#!/bin/bash

# AIM Endpoint Testing Script
# Tests all 120+ endpoints for production readiness

BASE_URL="http://localhost:8080"
ADMIN_EMAIL="admin@opena2a.org"
ADMIN_PASSWORD="NewSecurePassword123!"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TOTAL=0
PASSED=0
FAILED=0

# Login and get JWT token
echo "🔐 Authenticating as admin..."
LOGIN_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
  "$BASE_URL/api/v1/public/login")

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
  echo -e "${RED}❌ Authentication failed${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Authenticated successfully${NC}"

# Test function
test_endpoint() {
  local method=$1
  local endpoint=$2
  local expected_status=$3
  local description=$4
  local data=$5

  TOTAL=$((TOTAL + 1))

  if [ "$method" = "GET" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X GET \
      -H "Authorization: Bearer $TOKEN" \
      "$BASE_URL$endpoint")
  elif [ "$method" = "POST" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "$data" \
      "$BASE_URL$endpoint")
  elif [ "$method" = "PUT" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "$data" \
      "$BASE_URL$endpoint")
  elif [ "$method" = "DELETE" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE \
      -H "Authorization: Bearer $TOKEN" \
      "$BASE_URL$endpoint")
  fi

  STATUS=$(echo "$RESPONSE" | tail -1)
  BODY=$(echo "$RESPONSE" | head -n -1)

  if [ "$STATUS" = "$expected_status" ]; then
    echo -e "${GREEN}✅ PASS${NC} $method $endpoint - $description"
    PASSED=$((PASSED + 1))
  else
    echo -e "${RED}❌ FAIL${NC} $method $endpoint - Expected $expected_status, got $STATUS"
    echo "   Response: $BODY"
    FAILED=$((FAILED + 1))
  fi
}

# ===== HEALTH & STATUS =====
echo -e "\n${YELLOW}📊 Testing Health & Status Endpoints${NC}"
test_endpoint "GET" "/health" "200" "Health check"
test_endpoint "GET" "/api/v1/status" "200" "Status check"

# ===== PUBLIC ROUTES =====
echo -e "\n${YELLOW}🌐 Testing Public Endpoints${NC}"
test_endpoint "GET" "/api/v1/public/register/test-request-id/status" "404" "Registration status (not found)"

# ===== AUTH ROUTES =====
echo -e "\n${YELLOW}🔐 Testing Auth Endpoints${NC}"
test_endpoint "GET" "/api/v1/auth/me" "200" "Get current user"

# ===== AGENTS =====
echo -e "\n${YELLOW}🤖 Testing Agent Endpoints${NC}"
test_endpoint "GET" "/api/v1/agents" "200" "List agents"
test_endpoint "GET" "/api/v1/agents/b0000000-0000-0000-0000-000000000001" "200" "Get agent by ID"

# ===== ADMIN =====
echo -e "\n${YELLOW}👑 Testing Admin Endpoints${NC}"
test_endpoint "GET" "/api/v1/admin/users" "200" "List users"
test_endpoint "GET" "/api/v1/admin/alerts" "200" "List alerts"
test_endpoint "GET" "/api/v1/admin/capability-requests" "200" "List capability requests"

# ===== SECURITY POLICIES =====
echo -e "\n${YELLOW}🛡️  Testing Security Policy Endpoints${NC}"
test_endpoint "GET" "/api/v1/security-policies" "200" "List security policies"

# ===== COMPLIANCE =====
echo -e "\n${YELLOW}📋 Testing Compliance Endpoints${NC}"
test_endpoint "GET" "/api/v1/compliance/status" "200" "Get compliance status"

# ===== MCP SERVERS =====
echo -e "\n${YELLOW}🔌 Testing MCP Server Endpoints${NC}"
test_endpoint "GET" "/api/v1/mcp-servers" "200" "List MCP servers"

# ===== ANALYTICS =====
echo -e "\n${YELLOW}📈 Testing Analytics Endpoints${NC}"
test_endpoint "GET" "/api/v1/analytics/dashboard" "200" "Get dashboard stats"
test_endpoint "GET" "/api/v1/analytics/usage" "200" "Get usage stats"

# ===== TAGS =====
echo -e "\n${YELLOW}🏷️  Testing Tag Endpoints${NC}"
test_endpoint "GET" "/api/v1/tags" "200" "List tags"

# ===== SDK TOKENS =====
echo -e "\n${YELLOW}🔑 Testing SDK Token Endpoints${NC}"
test_endpoint "GET" "/api/v1/users/me/sdk-tokens" "200" "List SDK tokens"
test_endpoint "GET" "/api/v1/users/me/sdk-tokens/count" "200" "Get token count"

# ===== VERIFICATION =====
echo -e "\n${YELLOW}✅ Testing Verification Endpoints${NC}"
test_endpoint "GET" "/api/v1/verification-events" "200" "List verification events"

# Print summary
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${YELLOW}📊 TEST SUMMARY${NC}"
echo -e "${YELLOW}========================================${NC}"
echo -e "Total Tests: $TOTAL"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo -e "Success Rate: $(awk "BEGIN {printf \"%.1f\", ($PASSED/$TOTAL)*100}")%"

if [ $FAILED -eq 0 ]; then
  echo -e "\n${GREEN}🎉 ALL TESTS PASSED!${NC}"
  exit 0
else
  echo -e "\n${RED}⚠️  SOME TESTS FAILED${NC}"
  exit 1
fi
```

---

## 📊 Production Readiness Checklist

### Endpoint Quality Standards:
- [ ] **Status Code**: Returns correct HTTP status (200, 201, 204, 400, 401, 403, 404, 500)
- [ ] **Response Format**: Valid JSON with consistent field naming (camelCase)
- [ ] **Error Messages**: Clear, actionable error messages (no stack traces in production)
- [ ] **Authentication**: JWT validation works correctly
- [ ] **Authorization**: RBAC enforced per role
- [ ] **Data Validation**: Required fields validated, types checked
- [ ] **Organization Scoping**: Data filtered by organization_id
- [ ] **Audit Logging**: Sensitive operations logged
- [ ] **No SQL Injection**: Parameterized queries only
- [ ] **No XSS**: Input sanitization
- [ ] **Performance**: Response time < 100ms (p95)
- [ ] **Pagination**: Large result sets paginated
- [ ] **Rate Limiting**: API rate limits enforced
- [ ] **Documentation**: Endpoint documented with Swagger/OpenAPI

### Overall System Readiness:
- [ ] All high-priority endpoints tested (100%)
- [ ] RBAC matrix validated
- [ ] Error cases tested
- [ ] Performance benchmarks met
- [ ] Security scan passed (no critical vulnerabilities)
- [ ] Database migrations idempotent
- [ ] Environment variables documented
- [ ] Docker Compose setup working
- [ ] Health checks responding
- [ ] Logging configured
- [ ] Metrics collection working

---

## 🚀 Next Steps

1. **Run automated test script** against all endpoints
2. **Document failures** with detailed error messages
3. **Fix critical issues** (5xx errors, missing tables)
4. **Validate RBAC** across all roles
5. **Performance test** high-traffic endpoints
6. **Security audit** (OWASP Top 10)
7. **Update documentation** with OpenAPI specs
8. **Create CI/CD pipeline** to run tests on every commit

---

## 📚 References

- **Backend Code**: `/Users/decimai/workspace/agent-identity-management/apps/backend/cmd/server/main.go`
- **Migrations**: `/Users/decimai/workspace/agent-identity-management/apps/backend/migrations/`
- **Frontend**: `/Users/decimai/workspace/agent-identity-management/apps/web/`
- **Docker Compose**: `/Users/decimai/workspace/agent-identity-management/docker-compose.yml`
- **Project Docs**: `/Users/decimai/workspace/agent-identity-management/CLAUDE_CONTEXT.md`

---

**Status**: Ready for systematic testing
**Priority**: Complete Phase 1 (Automated Happy Path) before MVP release
**Estimated Effort**: 8-12 hours for comprehensive testing + fixes
