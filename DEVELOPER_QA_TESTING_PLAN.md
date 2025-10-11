# 🔍 Developer QA Testing Plan - Agent Identity Management (AIM)

**Version**: 1.1
**Date**: October 11, 2025
**Purpose**: Complete quality assurance testing of all AIM features (backend and frontend)
**Estimated Time**: 31 hours

## 🆕 What's New in v1.1

This update includes testing scenarios for newly implemented features:

### New Backend Endpoints (10 endpoints)
- **Capability Requests**: 5 endpoints for requesting, listing, approving, and rejecting capability expansion requests
- **Security Policies**: 5 endpoints for managing configurable security policies with enforcement modes

### New Frontend Pages (2 pages)
- **Admin Capability Requests Page** (`/dashboard/admin/capability-requests`): Admin interface for reviewing and approving capability expansion requests
- **Admin Security Policies Page** (`/dashboard/admin/security-policies`): Admin interface for configuring security policy enforcement modes

### Enhanced Features
- **Agent Capabilities Display**: Enhanced agent detail page with comprehensive capability visualization (risk levels, actions, granted by)
- **Trust Score Auto-Grant**: Automatic capability granting based on trust score threshold (≥ 0.30)
- **Security Policy Enforcement**: Configurable enforcement modes (Alert Only, Block & Alert, Allow)
- **Capability Request Workflow**: Complete end-to-end approval workflow from request to grant

### New Integration Tests (3 flows)
- Capability Request Approval Flow (15 steps)
- Trust Score Auto-Grant Flow (9 steps)
- Security Policy Enforcement Flow (15 steps)

**Total Endpoints**: 70+ (was 60+)
**Total Admin Pages**: 6 (was 4)
**Estimated Testing Time**: 31 hours (was 26 hours)

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Test Environment Setup](#test-environment-setup)
3. [Backend API Testing](#backend-api-testing)
4. [Frontend Testing](#frontend-testing)
5. [Integration Testing](#integration-testing)
6. [Security Testing](#security-testing)
7. [Performance Testing](#performance-testing)
8. [Test Reporting](#test-reporting)
9. [Bug Report Template](#bug-report-template)

---

## 📊 Overview

### Objectives
- **Verify all 70+ backend endpoints** are functional and return correct responses (including new capability requests & security policies)
- **Test all frontend pages** for UI/UX, data display, and user interactions (including 2 new admin pages)
- **Validate integration** between frontend, backend, and database
- **Ensure security** measures are properly implemented
- **Check performance** meets targets (<100ms API response, <2s page load)
- **Validate new features**: Capability request approval workflow, security policies enforcement, trust score auto-grant

### Scope
- ✅ All REST API endpoints (`/api/v1/*`)
- ✅ All frontend pages (`/dashboard/*`, `/auth/*`, `/admin/*`)
- ✅ Authentication flows (local, OAuth)
- ✅ RBAC (Role-Based Access Control)
- ✅ Data persistence and consistency
- ✅ Error handling and validation
- ✅ Security measures (JWT, HTTPS, rate limiting)
- ✅ Performance benchmarks

### Test Deliverables
1. **Test execution report** (Excel/Google Sheets)
2. **Bug reports** (GitHub Issues)
3. **Screenshots/videos** of critical bugs
4. **Performance metrics** (API response times, page load times)
5. **Security audit findings**

---

## 🛠️ Test Environment Setup

### Prerequisites
```bash
# Required tools
- Docker 20.10+
- Docker Compose 2.0+
- Node.js 22+
- Go 1.23+
- Python 3.8+
- Postman or Insomnia (API testing)
- Chrome DevTools
- Browser: Chrome/Firefox (latest)
```

### 1. Clone Repository
```bash
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management
```

### 2. Deploy Infrastructure
```bash
# Start PostgreSQL, Redis, and other services
./deploy.sh development

# Wait for services to be healthy (check with docker ps)
docker ps
```

### 3. Start Backend
```bash
cd apps/backend
cp .env.example .env

# Update .env with correct values
# Ensure DATABASE_URL points to localhost:5432
# Ensure REDIS_URL points to localhost:6379

# Run backend
go run cmd/server/main.go

# Backend should start on http://localhost:8080
```

### 4. Start Frontend
```bash
cd apps/web
npm install
npm run dev

# Frontend should start on http://localhost:3000
```

### 5. Verify Services
```bash
# Check backend health
curl http://localhost:8080/health

# Check backend readiness
curl http://localhost:8080/health/ready

# Check frontend
open http://localhost:3000
```

### 6. Create Test Users

**Admin User**:
```bash
# Register via UI at http://localhost:3000/auth/register
Email: admin@test.com
Password: Admin123!@#
First Name: Admin
Last Name: User

# Manually set role to ADMIN in database
docker exec -it aim-postgres psql -U aim -d aim_db -c "UPDATE users SET role = 'ADMIN', approved = true WHERE email = 'admin@test.com';"
```

**Manager User**:
```bash
Email: manager@test.com
Password: Manager123!@#

# Set role to MANAGER
docker exec -it aim-postgres psql -U aim -d aim_db -c "UPDATE users SET role = 'MANAGER', approved = true WHERE email = 'manager@test.com';"
```

**Member User**:
```bash
Email: member@test.com
Password: Member123!@#

# Set role to MEMBER
docker exec -it aim-postgres psql -U aim -d aim_db -c "UPDATE users SET role = 'MEMBER', approved = true WHERE email = 'member@test.com';"
```

**Viewer User**:
```bash
Email: viewer@test.com
Password: Viewer123!@#

# Set role to VIEWER
docker exec -it aim-postgres psql -U aim -d aim_db -c "UPDATE users SET role = 'VIEWER', approved = true WHERE email = 'viewer@test.com';"
```

---

## 🔌 Backend API Testing

### Testing Methodology
1. **Use Postman/Insomnia** to create a collection
2. **Test each endpoint** individually with valid and invalid inputs
3. **Verify response codes** (200, 201, 400, 401, 403, 404, 500)
4. **Check response structure** matches expected format
5. **Validate data persistence** in database

### Authentication Endpoints

#### 1.1 Local Login
```http
POST http://localhost:8080/api/v1/auth/login/local
Content-Type: application/json

{
  "email": "admin@test.com",
  "password": "Admin123!@#"
}
```
**Expected**: 200 OK with JWT token
**Test Cases**:
- ✅ Valid credentials → 200 OK
- ✅ Invalid email → 401 Unauthorized
- ✅ Invalid password → 401 Unauthorized
- ✅ Missing fields → 400 Bad Request
- ✅ SQL injection attempt → 400 Bad Request

#### 1.2 Get Current User
```http
GET http://localhost:8080/api/v1/auth/me
Authorization: Bearer {token}
```
**Expected**: 200 OK with user details
**Test Cases**:
- ✅ Valid token → 200 OK
- ✅ Expired token → 401 Unauthorized
- ✅ Invalid token → 401 Unauthorized
- ✅ Missing token → 401 Unauthorized

#### 1.3 Change Password
```http
POST http://localhost:8080/api/v1/auth/change-password
Authorization: Bearer {token}
Content-Type: application/json

{
  "currentPassword": "Admin123!@#",
  "newPassword": "NewAdmin123!@#"
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid passwords → 200 OK
- ✅ Wrong current password → 400 Bad Request
- ✅ Weak new password → 400 Bad Request
- ✅ Same as current password → 400 Bad Request

#### 1.4 Logout
```http
POST http://localhost:8080/api/v1/auth/logout
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid token → 200 OK
- ✅ Token invalidated after logout → 401 on next request

#### 1.5 Refresh Token
```http
POST http://localhost:8080/api/v1/auth/refresh
Content-Type: application/json

{
  "refreshToken": "{refresh_token}"
}
```
**Expected**: 200 OK with new access token
**Test Cases**:
- ✅ Valid refresh token → 200 OK
- ✅ Expired refresh token → 401 Unauthorized
- ✅ Invalid refresh token → 401 Unauthorized

---

### Agent Endpoints

#### 2.1 List Agents
```http
GET http://localhost:8080/api/v1/agents
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of agents
**Test Cases**:
- ✅ Returns all agents for organization
- ✅ Pagination works (if implemented)
- ✅ Filtering works (if implemented)
- ✅ Sorting works (if implemented)

#### 2.2 Create Agent
```http
POST http://localhost:8080/api/v1/agents
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "test-agent-001",
  "displayName": "Test Agent 001",
  "description": "Agent for QA testing",
  "agentType": "ai_agent",
  "version": "1.0.0",
  "status": "active"
}
```
**Expected**: 201 Created with agent details
**Test Cases**:
- ✅ Valid data → 201 Created
- ✅ Duplicate name → 409 Conflict
- ✅ Missing required fields → 400 Bad Request
- ✅ Invalid agent type → 400 Bad Request
- ✅ MEMBER role can create → 201
- ✅ VIEWER role cannot create → 403 Forbidden

#### 2.3 Get Agent Details
```http
GET http://localhost:8080/api/v1/agents/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK with agent details
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ Invalid UUID format → 400 Bad Request
- ✅ Agent from different org → 404 Not Found (security)

#### 2.4 Update Agent
```http
PUT http://localhost:8080/api/v1/agents/{id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "displayName": "Updated Test Agent",
  "description": "Updated description",
  "status": "active"
}
```
**Expected**: 200 OK with updated agent
**Test Cases**:
- ✅ Valid data → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ Invalid fields → 400 Bad Request
- ✅ MEMBER role can update → 200 OK
- ✅ VIEWER role cannot update → 403 Forbidden

#### 2.5 Delete Agent
```http
DELETE http://localhost:8080/api/v1/agents/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ MANAGER role can delete → 200 OK
- ✅ MEMBER role cannot delete → 403 Forbidden
- ✅ Agent data properly cleaned up in DB

#### 2.6 Verify Agent
```http
POST http://localhost:8080/api/v1/agents/{id}/verify
Authorization: Bearer {token}
Content-Type: application/json

{
  "publicKey": "{ed25519_public_key}"
}
```
**Expected**: 200 OK with verification result
**Test Cases**:
- ✅ Valid public key → 200 OK
- ✅ Invalid public key format → 400 Bad Request
- ✅ Non-existent agent → 404 Not Found
- ✅ MANAGER role can verify → 200 OK
- ✅ MEMBER role cannot verify → 403 Forbidden

#### 2.7 Verify Action
```http
POST http://localhost:8080/api/v1/agents/{id}/verify-action
Authorization: Bearer {token}
Content-Type: application/json

{
  "action": "read_database",
  "resource": "users_table",
  "context": {
    "user_id": "12345",
    "reason": "User data query"
  }
}
```
**Expected**: 200 OK with audit_id
**Test Cases**:
- ✅ Valid action → 200 OK with audit_id
- ✅ Invalid action → 400 Bad Request
- ✅ Missing required fields → 400 Bad Request
- ✅ Agent not verified → 403 Forbidden
- ✅ Action logged in audit_logs table

#### 2.8 Log Action Result
```http
POST http://localhost:8080/api/v1/agents/{id}/log-action/{audit_id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "status": "success",
  "result": {
    "rows_returned": 1,
    "execution_time_ms": 45
  }
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid audit_id → 200 OK
- ✅ Invalid audit_id → 404 Not Found
- ✅ Status updated in audit_logs

#### 2.9 Get Agent Credentials
```http
GET http://localhost:8080/api/v1/agents/{id}/credentials
Authorization: Bearer {token}
```
**Expected**: 200 OK with Ed25519 keys
**Test Cases**:
- ✅ Valid agent → 200 OK
- ✅ Returns public and private keys
- ✅ Keys are valid Ed25519 format
- ✅ VIEWER role can access → 200 OK

#### 2.10 Download SDK
```http
GET http://localhost:8080/api/v1/agents/{id}/sdk?language=python
Authorization: Bearer {token}
```
**Expected**: 200 OK with zip file
**Test Cases**:
- ✅ Python SDK → 200 OK with .zip
- ✅ Node.js SDK → 200 OK with .zip
- ✅ Go SDK → 200 OK with .zip
- ✅ Invalid language → 400 Bad Request
- ✅ SDK contains credentials
- ✅ SDK is properly formatted

#### 2.11 Get Agent MCP Servers
```http
GET http://localhost:8080/api/v1/agents/{id}/mcp-servers
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of MCP servers
**Test Cases**:
- ✅ Returns all MCP servers agent talks to
- ✅ Empty array if no relationships
- ✅ Contains correct MCP server details

#### 2.12 Add MCP Servers to Agent
```http
PUT http://localhost:8080/api/v1/agents/{id}/mcp-servers
Authorization: Bearer {token}
Content-Type: application/json

{
  "mcpServerIds": [
    "mcp-server-id-1",
    "mcp-server-id-2"
  ]
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid MCP server IDs → 200 OK
- ✅ Non-existent MCP server → 404 Not Found
- ✅ Duplicate relationship → 409 Conflict
- ✅ MEMBER role can add → 200 OK

#### 2.13 Remove MCP Server from Agent
```http
DELETE http://localhost:8080/api/v1/agents/{id}/mcp-servers/{mcp_id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid relationship → 200 OK
- ✅ Non-existent relationship → 404 Not Found
- ✅ MEMBER role can remove → 200 OK

#### 2.14 Detect and Map MCP Servers
```http
POST http://localhost:8080/api/v1/agents/{id}/mcp-servers/detect
Authorization: Bearer {token}
Content-Type: application/json

{
  "configPath": "~/.config/claude/mcp_config.json"
}
```
**Expected**: 200 OK with detected MCPs
**Test Cases**:
- ✅ Valid config path → 200 OK
- ✅ Invalid path → 404 Not Found
- ✅ Malformed config → 400 Bad Request
- ✅ Auto-registers new MCPs

#### 2.15 Get Agent Tags
```http
GET http://localhost:8080/api/v1/agents/{id}/tags
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of tags
**Test Cases**:
- ✅ Returns all tags for agent
- ✅ Empty array if no tags

#### 2.16 Add Tags to Agent
```http
POST http://localhost:8080/api/v1/agents/{id}/tags
Authorization: Bearer {token}
Content-Type: application/json

{
  "tagIds": ["tag-id-1", "tag-id-2"]
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid tag IDs → 200 OK
- ✅ Non-existent tag → 404 Not Found
- ✅ Duplicate tag → 409 Conflict
- ✅ MEMBER role can add → 200 OK

#### 2.17 Remove Tag from Agent
```http
DELETE http://localhost:8080/api/v1/agents/{id}/tags/{tagId}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid tag relationship → 200 OK
- ✅ Non-existent relationship → 404 Not Found

#### 2.18 Get Agent Capabilities
```http
GET http://localhost:8080/api/v1/agents/{id}/capabilities
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of capabilities
**Test Cases**:
- ✅ Returns all capabilities granted to agent
- ✅ Includes risk level, actions allowed
- ✅ Empty array if no capabilities

#### 2.19 Grant Capability to Agent
```http
POST http://localhost:8080/api/v1/agents/{id}/capabilities
Authorization: Bearer {token}
Content-Type: application/json

{
  "action": "read_database",
  "resourceType": "database",
  "resourceId": "*",
  "riskLevel": "medium",
  "requiresApproval": false
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid capability → 201 Created
- ✅ Duplicate capability → 409 Conflict
- ✅ Invalid risk level → 400 Bad Request
- ✅ MANAGER role can grant → 201 Created
- ✅ MEMBER role cannot grant → 403 Forbidden

#### 2.20 Revoke Capability from Agent
```http
DELETE http://localhost:8080/api/v1/agents/{id}/capabilities/{capabilityId}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid capability → 200 OK
- ✅ Non-existent capability → 404 Not Found
- ✅ MANAGER role can revoke → 200 OK

#### 2.21 Get Agent Violations
```http
GET http://localhost:8080/api/v1/agents/{id}/violations
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of violations
**Test Cases**:
- ✅ Returns all capability violations
- ✅ Includes violation details (timestamp, action, reason)
- ✅ Empty array if no violations

---

### MCP Server Endpoints

#### 3.1 List MCP Servers
```http
GET http://localhost:8080/api/v1/mcp-servers
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of MCP servers
**Test Cases**:
- ✅ Returns all MCP servers for organization
- ✅ Pagination works
- ✅ Filtering works

#### 3.2 Create MCP Server
```http
POST http://localhost:8080/api/v1/mcp-servers
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "test-mcp-server",
  "displayName": "Test MCP Server",
  "description": "MCP server for testing",
  "endpoint": "http://localhost:3000",
  "version": "1.0.0"
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid data → 201 Created
- ✅ Duplicate name → 409 Conflict
- ✅ Missing required fields → 400 Bad Request
- ✅ Invalid endpoint URL → 400 Bad Request
- ✅ MEMBER role can create → 201 Created

#### 3.3 Get MCP Server
```http
GET http://localhost:8080/api/v1/mcp-servers/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK with MCP server details
**Test Cases**:
- ✅ Valid MCP server ID → 200 OK
- ✅ Non-existent ID → 404 Not Found

#### 3.4 Update MCP Server
```http
PUT http://localhost:8080/api/v1/mcp-servers/{id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "displayName": "Updated MCP Server",
  "description": "Updated description"
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid data → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ MEMBER role can update → 200 OK

#### 3.5 Delete MCP Server
```http
DELETE http://localhost:8080/api/v1/mcp-servers/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid MCP server ID → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ MANAGER role can delete → 200 OK
- ✅ MEMBER role cannot delete → 403 Forbidden

#### 3.6 Verify MCP Server
```http
POST http://localhost:8080/api/v1/mcp-servers/{id}/verify
Authorization: Bearer {token}
```
**Expected**: 200 OK with verification result
**Test Cases**:
- ✅ Valid MCP server → 200 OK
- ✅ Verification status updated
- ✅ MANAGER role can verify → 200 OK

#### 3.7 Add Public Key to MCP Server
```http
POST http://localhost:8080/api/v1/mcp-servers/{id}/keys
Authorization: Bearer {token}
Content-Type: application/json

{
  "publicKey": "{ed25519_public_key}"
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid public key → 201 Created
- ✅ Invalid key format → 400 Bad Request
- ✅ MEMBER role can add → 201 Created

#### 3.8 Get Verification Status
```http
GET http://localhost:8080/api/v1/mcp-servers/{id}/verification-status
Authorization: Bearer {token}
```
**Expected**: 200 OK with status
**Test Cases**:
- ✅ Returns last verification attempt
- ✅ Includes success/failure reason
- ✅ Shows timestamp

#### 3.9 Get MCP Server Capabilities
```http
GET http://localhost:8080/api/v1/mcp-servers/{id}/capabilities
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of capabilities
**Test Cases**:
- ✅ Returns detected capabilities
- ✅ Includes tool names and descriptions
- ✅ Empty array if none detected

#### 3.10 Get MCP Server Agents
```http
GET http://localhost:8080/api/v1/mcp-servers/{id}/agents
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of agents
**Test Cases**:
- ✅ Returns all agents talking to this MCP
- ✅ Empty array if no relationships
- ✅ Includes agent details

#### 3.11 Verify MCP Action
```http
POST http://localhost:8080/api/v1/mcp-servers/{id}/verify-action
Authorization: Bearer {token}
Content-Type: application/json

{
  "action": "read_file",
  "resource": "/home/user/document.txt"
}
```
**Expected**: 200 OK with audit_id
**Test Cases**:
- ✅ Valid action → 200 OK
- ✅ Invalid action → 400 Bad Request
- ✅ MCP not verified → 403 Forbidden

---

### API Key Endpoints

#### 4.1 List API Keys
```http
GET http://localhost:8080/api/v1/api-keys
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of API keys
**Test Cases**:
- ✅ Returns all API keys for user's org
- ✅ Hashed keys shown (not plain text)
- ✅ Shows expiration dates
- ✅ Shows usage count

#### 4.2 Create API Key
```http
POST http://localhost:8080/api/v1/api-keys
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "Test API Key",
  "description": "For QA testing",
  "expiresAt": "2026-12-31T23:59:59Z"
}
```
**Expected**: 201 Created with plain text key
**Test Cases**:
- ✅ Valid data → 201 Created
- ✅ Returns plain text key (only once)
- ✅ Key is SHA-256 hashed in DB
- ✅ Missing name → 400 Bad Request
- ✅ Past expiration date → 400 Bad Request
- ✅ MEMBER role can create → 201 Created

#### 4.3 Disable API Key
```http
PATCH http://localhost:8080/api/v1/api-keys/{id}/disable
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid API key → 200 OK
- ✅ Key disabled in DB
- ✅ Cannot authenticate with disabled key
- ✅ MEMBER role can disable → 200 OK

#### 4.4 Delete API Key
```http
DELETE http://localhost:8080/api/v1/api-keys/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid API key → 200 OK
- ✅ Non-existent key → 404 Not Found
- ✅ Key deleted from DB
- ✅ MEMBER role can delete → 200 OK

---

### Trust Score Endpoints

#### 5.1 Calculate Trust Score
```http
POST http://localhost:8080/api/v1/trust-score/calculate/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK with calculated score
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Score between 0-100
- ✅ Score saved to DB
- ✅ Auto-grant triggered if score ≥ 0.30 (30%)
- ✅ Capabilities automatically granted based on trust level
- ✅ Auto-granted capabilities have granted_by = NULL
- ✅ Initial capabilities include:
  - file:read (if trust ≥ 0.30)
  - database:read (if trust ≥ 0.30)
  - api:call (if trust ≥ 0.30)
- ✅ Higher trust scores unlock more capabilities
- ✅ MANAGER role can calculate → 200 OK
- ✅ MEMBER role cannot calculate → 403 Forbidden

#### 5.2 Get Trust Score
```http
GET http://localhost:8080/api/v1/trust-score/agents/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK with current score
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Returns current score
- ✅ Includes calculation timestamp
- ✅ All roles can view → 200 OK

#### 5.3 Get Trust Score History
```http
GET http://localhost:8080/api/v1/trust-score/agents/{id}/history
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of historical scores
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Returns time-series data
- ✅ Sorted by timestamp DESC
- ✅ Empty array if no history

#### 5.4 Get Trust Score Trends
```http
GET http://localhost:8080/api/v1/trust-score/trends
Authorization: Bearer {token}
```
**Expected**: 200 OK with trend analysis
**Test Cases**:
- ✅ Returns org-wide trends
- ✅ Includes average, min, max scores
- ✅ Time-based aggregation

---

### Admin Endpoints

#### 6.1 List Users
```http
GET http://localhost:8080/api/v1/admin/users
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of users
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Non-admin role → 403 Forbidden
- ✅ Returns all users in org
- ✅ Includes role, status, email

#### 6.2 Get Pending Users
```http
GET http://localhost:8080/api/v1/admin/users/pending
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of pending users
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns only approved=false users
- ✅ Empty array if no pending

#### 6.3 Approve User
```http
POST http://localhost:8080/api/v1/admin/users/{id}/approve
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid pending user → 200 OK
- ✅ User approved in DB
- ✅ User can now log in
- ✅ Non-pending user → 400 Bad Request
- ✅ ADMIN role only → 403 for others

#### 6.4 Reject User
```http
POST http://localhost:8080/api/v1/admin/users/{id}/reject
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid pending user → 200 OK
- ✅ User marked as rejected
- ✅ User cannot log in
- ✅ ADMIN role only → 403 for others

#### 6.5 Update User Role
```http
PUT http://localhost:8080/api/v1/admin/users/{id}/role
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "role": "MANAGER"
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid role → 200 OK
- ✅ Invalid role → 400 Bad Request
- ✅ Role updated in DB
- ✅ User permissions changed immediately
- ✅ ADMIN role only → 403 for others

#### 6.6 Deactivate User
```http
DELETE http://localhost:8080/api/v1/admin/users/{id}
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid user → 200 OK
- ✅ User deactivated (not deleted)
- ✅ User cannot log in
- ✅ Cannot deactivate self → 400 Bad Request
- ✅ ADMIN role only → 403 for others

#### 6.7 Get Organization Settings
```http
GET http://localhost:8080/api/v1/admin/organization/settings
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with settings
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns org settings
- ✅ Includes feature flags

#### 6.8 Update Organization Settings
```http
PUT http://localhost:8080/api/v1/admin/organization/settings
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "allowSelfRegistration": true,
  "requireApproval": true
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid settings → 200 OK
- ✅ Settings updated in DB
- ✅ Changes take effect immediately
- ✅ ADMIN role only → 403 for others

#### 6.9 Get Audit Logs
```http
GET http://localhost:8080/api/v1/admin/audit-logs?limit=50
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of audit logs
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns recent logs
- ✅ Pagination works
- ✅ Filtering by user/agent works

#### 6.10 Get Alerts
```http
GET http://localhost:8080/api/v1/admin/alerts
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of alerts
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns all alerts
- ✅ Includes severity, status
- ✅ Sorted by timestamp DESC

#### 6.11 Acknowledge Alert
```http
POST http://localhost:8080/api/v1/admin/alerts/{id}/acknowledge
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid alert → 200 OK
- ✅ Alert marked as acknowledged
- ✅ Timestamp recorded
- ✅ ADMIN role only → 403 for others

#### 6.12 Resolve Alert
```http
POST http://localhost:8080/api/v1/admin/alerts/{id}/resolve
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "resolution": "False positive - normal behavior"
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid alert → 200 OK
- ✅ Alert marked as resolved
- ✅ Resolution notes saved
- ✅ ADMIN role only → 403 for others

#### 6.13 Approve Drift
```http
POST http://localhost:8080/api/v1/admin/alerts/{id}/approve-drift
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid drift alert → 200 OK
- ✅ Drift approved and baseline updated
- ✅ Future similar behavior allowed
- ✅ ADMIN role only → 403 for others

#### 6.14 Get Dashboard Stats
```http
GET http://localhost:8080/api/v1/admin/dashboard/stats
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with statistics
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns total_agents, verified_agents, etc.
- ✅ Includes recent activity
- ✅ Performance metrics

#### 6.15 List Capability Requests
```http
GET http://localhost:8080/api/v1/admin/capability-requests
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of capability requests
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns all capability requests for organization
- ✅ Includes agent details, requester, reviewer info
- ✅ Filter by status works (pending, approved, rejected)
- ✅ Filter by agent_id works
- ✅ Pagination works (limit, offset)
- ✅ Non-admin role → 403 Forbidden

#### 6.16 Get Capability Request
```http
GET http://localhost:8080/api/v1/admin/capability-requests/{id}
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with capability request details
**Test Cases**:
- ✅ Valid request ID → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ Includes full details (agent, capability type, reason, status)
- ✅ ADMIN role only → 403 for others

#### 6.17 Approve Capability Request
```http
POST http://localhost:8080/api/v1/admin/capability-requests/{id}/approve
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid pending request → 200 OK
- ✅ Request status updated to approved
- ✅ Reviewer information recorded
- ✅ Capability automatically granted to agent
- ✅ Capability appears in agent_capabilities table
- ✅ Already approved request → 400 Bad Request
- ✅ Non-existent request → 404 Not Found
- ✅ ADMIN role only → 403 for others

#### 6.18 Reject Capability Request
```http
POST http://localhost:8080/api/v1/admin/capability-requests/{id}/reject
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid pending request → 200 OK
- ✅ Request status updated to rejected
- ✅ Reviewer information recorded
- ✅ Capability NOT granted to agent
- ✅ Already rejected request → 400 Bad Request
- ✅ ADMIN role only → 403 for others

#### 6.19 Create Capability Request
```http
POST http://localhost:8080/api/v1/capability-requests
Authorization: Bearer {token}
Content-Type: application/json

{
  "agentId": "agent-uuid",
  "capabilityType": "database:write",
  "reason": "Need to update user records for analytics"
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid data → 201 Created
- ✅ Request created with pending status
- ✅ Requester set to current user
- ✅ Duplicate pending request → 409 Conflict
- ✅ Capability already granted → 409 Conflict
- ✅ Invalid agent ID → 404 Not Found
- ✅ Missing required fields → 400 Bad Request
- ✅ All authenticated users can create

#### 6.20 List Security Policies
```http
GET http://localhost:8080/api/v1/admin/security-policies
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with array of security policies
**Test Cases**:
- ✅ ADMIN role → 200 OK
- ✅ Returns all policies for organization
- ✅ Includes policy details (name, description, enforcement mode, priority)
- ✅ Shows enabled/disabled status
- ✅ Non-admin role → 403 Forbidden

#### 6.21 Get Security Policy
```http
GET http://localhost:8080/api/v1/admin/security-policies/{id}
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with policy details
**Test Cases**:
- ✅ Valid policy ID → 200 OK
- ✅ Non-existent ID → 404 Not Found
- ✅ Includes configuration details
- ✅ ADMIN role only → 403 for others

#### 6.22 Update Security Policy
```http
PUT http://localhost:8080/api/v1/admin/security-policies/{id}
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "enforcementMode": "block_and_alert",
  "isEnabled": true,
  "priority": 100
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid data → 200 OK
- ✅ Policy updated in database
- ✅ Valid enforcement modes: alert_only, block_and_alert, allow
- ✅ Invalid enforcement mode → 400 Bad Request
- ✅ Priority validation (1-1000) works
- ✅ Changes take effect immediately
- ✅ ADMIN role only → 403 for others

#### 6.23 Enable/Disable Security Policy
```http
PATCH http://localhost:8080/api/v1/admin/security-policies/{id}/toggle
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "isEnabled": false
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid policy → 200 OK
- ✅ Policy enabled/disabled in database
- ✅ Disabled policies not enforced
- ✅ ADMIN role only → 403 for others

#### 6.24 Create Security Policy
```http
POST http://localhost:8080/api/v1/admin/security-policies
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "High Risk Action Monitoring",
  "description": "Monitor high-risk capability usage",
  "policyType": "capability_violation",
  "enforcementMode": "alert_only",
  "priority": 95
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid data → 201 Created
- ✅ Default values applied (isEnabled: true)
- ✅ Duplicate name → 409 Conflict
- ✅ Invalid policy type → 400 Bad Request
- ✅ ADMIN role only → 403 for others

---

### Compliance Endpoints

#### 7.1 Generate Compliance Report
```http
POST http://localhost:8080/api/v1/compliance/reports/generate
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "framework": "SOC2",
  "startDate": "2025-01-01",
  "endDate": "2025-12-31"
}
```
**Expected**: 200 OK with report
**Test Cases**:
- ✅ Valid framework → 200 OK
- ✅ Invalid framework → 400 Bad Request
- ✅ Report includes all required sections
- ✅ ADMIN role only → 403 for others

#### 7.2 Get Compliance Status
```http
GET http://localhost:8080/api/v1/compliance/status
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with status
**Test Cases**:
- ✅ Returns compliance score
- ✅ Lists active frameworks
- ✅ Shows violations count

#### 7.3 Get Compliance Metrics
```http
GET http://localhost:8080/api/v1/compliance/metrics
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with metrics
**Test Cases**:
- ✅ Returns time-series compliance data
- ✅ Includes trend analysis

#### 7.4 Export Audit Log
```http
GET http://localhost:8080/api/v1/compliance/audit-log/export
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with CSV file
**Test Cases**:
- ✅ Returns CSV format
- ✅ Includes all audit log fields
- ✅ Date range filtering works

#### 7.5 Get Access Review
```http
GET http://localhost:8080/api/v1/compliance/access-review
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with access review data
**Test Cases**:
- ✅ Lists all users and permissions
- ✅ Shows last access timestamp
- ✅ Identifies stale accounts

#### 7.6 Get Data Retention
```http
GET http://localhost:8080/api/v1/compliance/data-retention
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with retention info
**Test Cases**:
- ✅ Shows retention policies
- ✅ Lists data to be deleted
- ✅ Shows storage usage

#### 7.7 Run Compliance Check
```http
POST http://localhost:8080/api/v1/compliance/check
Authorization: Bearer {admin_token}
```
**Expected**: 200 OK with check results
**Test Cases**:
- ✅ Runs all compliance checks
- ✅ Returns pass/fail status
- ✅ Lists violations

---

### Security Endpoints

#### 8.1 Get Threats
```http
GET http://localhost:8080/api/v1/security/threats
Authorization: Bearer {manager_token}
```
**Expected**: 200 OK with array of threats
**Test Cases**:
- ✅ MANAGER role → 200 OK
- ✅ Returns detected threats
- ✅ Includes severity, timestamp
- ✅ Empty array if no threats

#### 8.2 Get Anomalies
```http
GET http://localhost:8080/api/v1/security/anomalies
Authorization: Bearer {manager_token}
```
**Expected**: 200 OK with array of anomalies
**Test Cases**:
- ✅ MANAGER role → 200 OK
- ✅ Returns behavioral anomalies
- ✅ Includes agent ID, description

#### 8.3 Get Security Metrics
```http
GET http://localhost:8080/api/v1/security/metrics
Authorization: Bearer {manager_token}
```
**Expected**: 200 OK with metrics
**Test Cases**:
- ✅ Returns security score
- ✅ Includes threat count
- ✅ Time-series data

#### 8.4 Run Security Scan
```http
GET http://localhost:8080/api/v1/security/scan/{id}
Authorization: Bearer {manager_token}
```
**Expected**: 200 OK with scan results
**Test Cases**:
- ✅ Valid agent ID → 200 OK
- ✅ Scan completes successfully
- ✅ Returns vulnerabilities found

#### 8.5 Get Incidents
```http
GET http://localhost:8080/api/v1/security/incidents
Authorization: Bearer {manager_token}
```
**Expected**: 200 OK with array of incidents
**Test Cases**:
- ✅ Returns all security incidents
- ✅ Includes status (open/resolved)
- ✅ Sorted by severity

#### 8.6 Resolve Incident
```http
POST http://localhost:8080/api/v1/security/incidents/{id}/resolve
Authorization: Bearer {manager_token}
Content-Type: application/json

{
  "resolution": "Patched vulnerability"
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid incident → 200 OK
- ✅ Incident marked as resolved
- ✅ Resolution notes saved

---

### Analytics Endpoints

#### 9.1 Get Dashboard Stats
```http
GET http://localhost:8080/api/v1/analytics/dashboard
Authorization: Bearer {token}
```
**Expected**: 200 OK with stats
**Test Cases**:
- ✅ All roles can access → 200 OK
- ✅ Returns total agents, verifications, etc.
- ✅ Includes recent activity

#### 9.2 Get Usage Statistics
```http
GET http://localhost:8080/api/v1/analytics/usage
Authorization: Bearer {token}
```
**Expected**: 200 OK with usage data
**Test Cases**:
- ✅ Returns API call counts
- ✅ Time-series data
- ✅ Agent activity breakdown

#### 9.3 Get Trust Score Trends
```http
GET http://localhost:8080/api/v1/analytics/trends
Authorization: Bearer {token}
```
**Expected**: 200 OK with trends
**Test Cases**:
- ✅ Returns org-wide trust trends
- ✅ Time-series data
- ✅ Average, min, max scores

#### 9.4 Generate Report
```http
GET http://localhost:8080/api/v1/analytics/reports/generate
Authorization: Bearer {token}
```
**Expected**: 200 OK with report
**Test Cases**:
- ✅ Generates custom report
- ✅ Includes selected metrics

#### 9.5 Get Agent Activity
```http
GET http://localhost:8080/api/v1/analytics/agents/activity
Authorization: Bearer {token}
```
**Expected**: 200 OK with activity data
**Test Cases**:
- ✅ Returns agent activity timeline
- ✅ Includes action counts
- ✅ Time-based aggregation

---

### Webhook Endpoints

#### 10.1 Create Webhook
```http
POST http://localhost:8080/api/v1/webhooks
Authorization: Bearer {token}
Content-Type: application/json

{
  "url": "https://webhook.site/unique-id",
  "events": ["agent.created", "agent.verified"],
  "secret": "webhook_secret_key"
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid webhook → 201 Created
- ✅ Invalid URL → 400 Bad Request
- ✅ Empty events → 400 Bad Request
- ✅ MEMBER role can create → 201 Created

#### 10.2 List Webhooks
```http
GET http://localhost:8080/api/v1/webhooks
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of webhooks
**Test Cases**:
- ✅ Returns all webhooks for org
- ✅ Includes status (active/inactive)

#### 10.3 Get Webhook
```http
GET http://localhost:8080/api/v1/webhooks/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK with webhook details
**Test Cases**:
- ✅ Valid webhook ID → 200 OK
- ✅ Non-existent ID → 404 Not Found

#### 10.4 Delete Webhook
```http
DELETE http://localhost:8080/api/v1/webhooks/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid webhook → 200 OK
- ✅ Webhook deleted from DB
- ✅ MEMBER role can delete → 200 OK

#### 10.5 Test Webhook
```http
POST http://localhost:8080/api/v1/webhooks/{id}/test
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid webhook → 200 OK
- ✅ Test event sent successfully
- ✅ Returns delivery status

---

### Tag Endpoints

#### 11.1 Get Tags
```http
GET http://localhost:8080/api/v1/tags
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of tags
**Test Cases**:
- ✅ Returns all tags for org
- ✅ Includes usage count

#### 11.2 Create Tag
```http
POST http://localhost:8080/api/v1/tags
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "production",
  "color": "#FF5733"
}
```
**Expected**: 201 Created
**Test Cases**:
- ✅ Valid tag → 201 Created
- ✅ Duplicate name → 409 Conflict
- ✅ Invalid color → 400 Bad Request
- ✅ MEMBER role can create → 201 Created

#### 11.3 Delete Tag
```http
DELETE http://localhost:8080/api/v1/tags/{id}
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid tag → 200 OK
- ✅ Tag removed from all entities
- ✅ MANAGER role can delete → 200 OK

---

### Detection Endpoints

#### 12.1 Report Detection
```http
POST http://localhost:8080/api/v1/detection/agents/{id}/report
Authorization: Bearer {token}
Content-Type: application/json

{
  "mcpConfig": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}
```
**Expected**: 200 OK with detected MCPs
**Test Cases**:
- ✅ Valid config → 200 OK
- ✅ Auto-registers new MCPs
- ✅ Returns detected capabilities

#### 12.2 Get Detection Status
```http
GET http://localhost:8080/api/v1/detection/agents/{id}/status
Authorization: Bearer {token}
```
**Expected**: 200 OK with status
**Test Cases**:
- ✅ Valid agent → 200 OK
- ✅ Returns detection timestamp
- ✅ Shows detected MCP count

#### 12.3 Report Capabilities
```http
POST http://localhost:8080/api/v1/detection/agents/{id}/capabilities/report
Authorization: Bearer {token}
Content-Type: application/json

{
  "capabilities": [
    {
      "action": "read_file",
      "riskLevel": "low"
    }
  ]
}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid capabilities → 200 OK
- ✅ Capabilities saved to DB

---

### SDK Token Endpoints

#### 13.1 List User Tokens
```http
GET http://localhost:8080/api/v1/users/me/sdk-tokens
Authorization: Bearer {token}
```
**Expected**: 200 OK with array of tokens
**Test Cases**:
- ✅ Returns all tokens for current user
- ✅ Includes last used timestamp
- ✅ Shows status (active/revoked)

#### 13.2 Get Active Token Count
```http
GET http://localhost:8080/api/v1/users/me/sdk-tokens/count
Authorization: Bearer {token}
```
**Expected**: 200 OK with count
**Test Cases**:
- ✅ Returns active token count
- ✅ Excludes revoked tokens

#### 13.3 Revoke Token
```http
POST http://localhost:8080/api/v1/users/me/sdk-tokens/{id}/revoke
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ Valid token → 200 OK
- ✅ Token marked as revoked
- ✅ Cannot use revoked token

#### 13.4 Revoke All Tokens
```http
POST http://localhost:8080/api/v1/users/me/sdk-tokens/revoke-all
Authorization: Bearer {token}
```
**Expected**: 200 OK
**Test Cases**:
- ✅ All tokens revoked
- ✅ Current session unaffected
- ✅ Returns count of revoked tokens

---

### Public Endpoints

#### 14.1 Public Agent Registration
```http
POST http://localhost:8080/api/v1/public/agents/register
Content-Type: application/json

{
  "name": "public-agent-001",
  "displayName": "Public Agent 001",
  "agentType": "ai_agent"
}
```
**Expected**: 201 Created with credentials
**Test Cases**:
- ✅ No authentication required → 201 Created
- ✅ Auto-generates Ed25519 keys
- ✅ Returns public and private keys
- ✅ Agent created with verified status

---

## 🎨 Frontend Testing

### Testing Methodology
1. **Use Chrome DevTools** to inspect network requests
2. **Test all user interactions** (clicks, forms, navigation)
3. **Verify data display** matches backend responses
4. **Check error handling** (network errors, validation errors)
5. **Test responsive design** (mobile, tablet, desktop)
6. **Validate accessibility** (keyboard navigation, screen readers)

---

### Authentication Pages

#### 15.1 Login Page (`/auth/login`)
**URL**: http://localhost:3000/auth/login

**Test Cases**:
- ✅ Page loads without errors
- ✅ Email and password fields visible
- ✅ Login button enabled/disabled appropriately
- ✅ OAuth buttons visible (Google, Microsoft, Okta)
- ✅ Valid credentials → redirect to dashboard
- ✅ Invalid credentials → error message displayed
- ✅ Empty fields → validation error
- ✅ "Register" link navigates to registration page
- ✅ "Forgot password" link works (if implemented)
- ✅ Password visibility toggle works

#### 15.2 Register Page (`/auth/register`)
**URL**: http://localhost:3000/auth/register

**Test Cases**:
- ✅ Page loads without errors
- ✅ All form fields visible (email, password, firstName, lastName)
- ✅ Password strength indicator works
- ✅ Password confirmation validation works
- ✅ Email format validation works
- ✅ Valid data → redirect to pending approval page
- ✅ Duplicate email → error message
- ✅ Weak password → validation error
- ✅ "Already have account" link navigates to login

#### 15.3 Registration Pending Page (`/auth/registration-pending`)
**URL**: http://localhost:3000/auth/registration-pending

**Test Cases**:
- ✅ Page loads without errors
- ✅ Message displayed explaining approval process
- ✅ "Back to login" link works

#### 15.4 OAuth Callback Page (`/auth/callback`)
**URL**: http://localhost:3000/auth/callback

**Test Cases**:
- ✅ Handles OAuth callback correctly
- ✅ Extracts token from URL
- ✅ Redirects to dashboard on success
- ✅ Shows error on OAuth failure

---

### Dashboard Pages

#### 16.1 Main Dashboard (`/dashboard`)
**URL**: http://localhost:3000/dashboard

**Test Cases**:
- ✅ Page loads without errors
- ✅ Statistics cards display correctly:
  - Total Agents
  - Verified Agents
  - Trust Score Average
  - Recent Activity Count
- ✅ Recent activity list displays
- ✅ Trust score chart renders
- ✅ Agent status breakdown chart renders
- ✅ Navigation sidebar visible
- ✅ User menu in header works
- ✅ Data refreshes on page load
- ✅ Loading states shown while fetching
- ✅ Error states handled gracefully

#### 16.2 Agents List Page (`/dashboard/agents`)
**URL**: http://localhost:3000/dashboard/agents

**Test Cases**:
- ✅ Page loads without errors
- ✅ Agents list displays in table format
- ✅ Table columns: Name, Type, Status, Trust Score, Last Verified
- ✅ "Create Agent" button visible (for MEMBER+)
- ✅ Search/filter functionality works
- ✅ Pagination works (if implemented)
- ✅ Sorting by columns works
- ✅ Click on agent row → navigates to agent detail
- ✅ Empty state shown when no agents
- ✅ Loading skeleton shown while fetching

#### 16.3 Create Agent Page (`/dashboard/agents/new`)
**URL**: http://localhost:3000/dashboard/agents/new

**Test Cases**:
- ✅ Page loads without errors
- ✅ Form fields visible:
  - Name (required)
  - Display Name
  - Description
  - Agent Type (dropdown)
  - Version
- ✅ Agent type dropdown populated
- ✅ Form validation works (required fields)
- ✅ "Create Agent" button disabled until valid
- ✅ Valid submission → redirect to agent detail
- ✅ Error handling (duplicate name, etc.)
- ✅ "Cancel" button navigates back
- ✅ MEMBER role can access
- ✅ VIEWER role redirected to 403 page

#### 16.4 Agent Detail Page (`/dashboard/agents/[id]`)
**URL**: http://localhost:3000/dashboard/agents/{agent-id}

**Test Cases**:
- ✅ Page loads without errors
- ✅ Agent details displayed:
  - Name, Display Name, Description
  - Agent Type, Version, Status
  - Trust Score (with badge)
  - Created At, Updated At
  - Last Verified At
- ✅ "Edit" button visible (for MEMBER+)
- ✅ "Delete" button visible (for MANAGER+)
- ✅ "Verify Agent" button visible (for MANAGER+)
- ✅ Trust score history chart renders
- ✅ Recent activity table shows agent actions
- ✅ Capabilities section displays correctly:
  - Lists all granted capabilities
  - Shows capability type (database:read, api:external_call, etc.)
  - Displays risk level badges (Low, Medium, High, Critical)
  - Risk level colors match severity:
    - Low: green
    - Medium: yellow
    - High: orange
    - Critical: red
  - Shows granted by (user who granted)
  - Shows granted at (timestamp)
  - Shows actions allowed for each capability
  - Empty state when no capabilities granted
  - "Request Capability" button works (opens modal/form)
- ✅ Auto-grant indicator shown for trust score ≥ 0.30
- ✅ MCP servers section lists related MCPs
- ✅ Tags displayed with colors
- ✅ "Download SDK" button works
- ✅ "Get Credentials" button works
- ✅ Edit modal/page works
- ✅ Delete confirmation modal works
- ✅ Verify agent flow works

#### 16.5 Agent Success Page (`/dashboard/agents/[id]/success`)
**URL**: http://localhost:3000/dashboard/agents/{agent-id}/success

**Test Cases**:
- ✅ Page loads without errors
- ✅ Success message displayed
- ✅ Agent credentials shown (public/private key)
- ✅ "Download SDK" button works
- ✅ "Copy credentials" button works
- ✅ Warning about saving credentials shown
- ✅ "Go to agent" button navigates to detail page

#### 16.6 MCP Servers List Page (`/dashboard/mcp`)
**URL**: http://localhost:3000/dashboard/mcp

**Test Cases**:
- ✅ Page loads without errors
- ✅ MCP servers list displays in table
- ✅ Table columns: Name, Endpoint, Status, Verified
- ✅ "Register MCP Server" button visible
- ✅ Search/filter works
- ✅ Click on MCP row → navigates to detail
- ✅ Empty state shown when no MCPs
- ✅ Loading state shown while fetching

#### 16.7 MCP Server Detail Page (`/dashboard/mcp/[id]`)
**URL**: http://localhost:3000/dashboard/mcp/{mcp-id}

**Test Cases**:
- ✅ Page loads without errors
- ✅ MCP server details displayed:
  - Name, Display Name, Description
  - Endpoint, Version, Status
  - Verification Status
- ✅ Capabilities section lists detected capabilities
- ✅ Agents section lists agents talking to this MCP
- ✅ "Edit" button visible (for MEMBER+)
- ✅ "Delete" button visible (for MANAGER+)
- ✅ "Verify" button visible (for MANAGER+)
- ✅ Tags displayed with colors

#### 16.8 API Keys Page (`/dashboard/api-keys`)
**URL**: http://localhost:3000/dashboard/api-keys

**Test Cases**:
- ✅ Page loads without errors
- ✅ API keys list displays
- ✅ Table columns: Name, Key (hashed), Status, Expires At
- ✅ "Create API Key" button visible
- ✅ Create modal/page works
- ✅ API key displayed only once after creation
- ✅ "Copy key" button works
- ✅ "Disable" button works
- ✅ "Delete" button works with confirmation
- ✅ Empty state shown when no keys

#### 16.9 SDK Tokens Page (`/dashboard/sdk-tokens`)
**URL**: http://localhost:3000/dashboard/sdk-tokens

**Test Cases**:
- ✅ Page loads without errors
- ✅ SDK tokens list displays
- ✅ Table columns: Agent, Token (partial), Last Used, Status
- ✅ "Revoke" button works
- ✅ "Revoke All" button works with confirmation
- ✅ Active token count badge displayed
- ✅ Empty state shown when no tokens

#### 16.10 SDK Download Page (`/dashboard/sdk`)
**URL**: http://localhost:3000/dashboard/sdk

**Test Cases**:
- ✅ Page loads without errors
- ✅ SDK language tabs work (Python, Node.js, Go)
- ✅ "Download SDK" button works for each language
- ✅ Installation instructions displayed
- ✅ Code examples shown
- ✅ Integration guides linked

#### 16.11 Monitoring Page (`/dashboard/monitoring`)
**URL**: http://localhost:3000/dashboard/monitoring

**Test Cases**:
- ✅ Page loads without errors
- ✅ Real-time verification events displayed
- ✅ Events update automatically (polling/websocket)
- ✅ Event details shown (agent, action, timestamp, status)
- ✅ Filter by agent works
- ✅ Filter by status (success/failure) works
- ✅ Event timeline chart renders

#### 16.12 Security Dashboard (`/dashboard/security`)
**URL**: http://localhost:3000/dashboard/security

**Test Cases**:
- ✅ Page loads without errors (MANAGER+ only)
- ✅ Security metrics displayed:
  - Threat count
  - Anomaly count
  - Incident count
  - Security score
- ✅ Threats table displays
- ✅ Anomalies table displays
- ✅ Incidents table displays
- ✅ Security trend chart renders
- ✅ Filter by severity works
- ✅ "Resolve incident" button works
- ✅ VIEWER role redirected to 403 page

---

### Admin Pages

#### 17.1 Admin Dashboard (`/dashboard/admin`)
**URL**: http://localhost:3000/dashboard/admin

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Admin statistics displayed:
  - Total users
  - Pending registrations
  - Total agents
  - Active alerts
- ✅ Quick actions cards visible
- ✅ Recent admin activity displayed
- ✅ Non-admin role redirected to 403 page

#### 17.2 Admin Users Page (`/dashboard/admin/users`)
**URL**: http://localhost:3000/dashboard/admin/users

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Users list displays in table
- ✅ Table columns: Name, Email, Role, Status, Last Login
- ✅ "Change role" dropdown works
- ✅ "Deactivate" button works with confirmation
- ✅ Filter by role works
- ✅ Filter by status (active/inactive) works
- ✅ Search by name/email works
- ✅ Pagination works

#### 17.3 Admin Registrations Page (`/dashboard/admin/registrations`)
**URL**: http://localhost:3000/dashboard/admin/registrations

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Pending registrations list displays
- ✅ Table columns: Name, Email, Registered At
- ✅ "Approve" button works
- ✅ "Reject" button works with confirmation
- ✅ User notified after approval/rejection
- ✅ Empty state shown when no pending
- ✅ Count badge shows pending count

#### 17.4 Admin Alerts Page (`/dashboard/admin/alerts`)
**URL**: http://localhost:3000/dashboard/admin/alerts

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Alerts list displays
- ✅ Table columns: Title, Severity, Agent, Status, Created At
- ✅ "Acknowledge" button works
- ✅ "Resolve" button opens modal
- ✅ Resolve modal with notes field works
- ✅ "Approve drift" button works (for drift alerts)
- ✅ Filter by severity works
- ✅ Filter by status (open/acknowledged/resolved) works
- ✅ Severity badges colored correctly

#### 17.5 Admin Capability Requests Page (`/dashboard/admin/capability-requests`)
**URL**: http://localhost:3000/dashboard/admin/capability-requests

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Statistics cards display:
  - Total Requests
  - Pending Review
  - Approved
  - Rejected
- ✅ Search/filter functionality works
- ✅ Filter by status works (all, pending, approved, rejected)
- ✅ Search by agent name/capability type works
- ✅ Requests list displays with correct data:
  - Agent display name and name
  - Capability type badge
  - Reason text
  - Requested by (user email)
  - Requested at (timestamp)
- ✅ Status badges colored correctly:
  - Pending: yellow
  - Approved: green
  - Rejected: red
- ✅ "Approve" button visible only for pending requests
- ✅ "Reject" button visible only for pending requests
- ✅ Approve flow works:
  - Click approve → API call succeeds
  - Success alert shown
  - List refreshes automatically
  - Request status updated to approved
  - Reviewer info displayed
- ✅ Reject flow works:
  - Confirmation dialog shown
  - Click confirm → API call succeeds
  - Request status updated to rejected
  - Reviewer info displayed
- ✅ Empty state shown when no requests match filter
- ✅ Info banner explains auto-grant architecture
- ✅ Non-admin role redirected to 403 page

#### 17.6 Admin Security Policies Page (`/dashboard/admin/security-policies`)
**URL**: http://localhost:3000/dashboard/admin/security-policies

**Test Cases**:
- ✅ Page loads without errors (ADMIN only)
- ✅ Security policies list displays
- ✅ Table columns: Name, Description, Type, Enforcement Mode, Priority, Status
- ✅ Enforcement mode selector works:
  - Alert Only (monitor only)
  - Block & Alert (enforce and notify)
  - Allow (disabled)
- ✅ Enforcement mode changes save correctly
- ✅ Visual indicators for enforcement modes:
  - Alert Only: blue/info
  - Block & Alert: red/warning
  - Allow: gray/muted
- ✅ Warning banner shows when blocking mode enabled
- ✅ Enable/disable toggle works
- ✅ Priority displayed correctly (1-1000)
- ✅ Policy type badges displayed:
  - Capability Violation
  - Low Trust Score
  - Unusual Activity
- ✅ Default policies present for all organizations:
  - Capability Violation Detection
  - Low Trust Score Monitoring
  - Unusual Activity Detection
- ✅ Empty state handled (though should not occur with defaults)
- ✅ Non-admin role redirected to 403 page

---

### Common UI Components

#### 18.1 Navigation Sidebar
**Test Cases**:
- ✅ Sidebar visible on all dashboard pages
- ✅ Current page highlighted
- ✅ Navigation links work
- ✅ Role-based menu items shown:
  - Admin menu only for ADMIN
  - Security menu only for MANAGER+
- ✅ Admin submenu includes:
  - Users
  - Registrations
  - Alerts
  - Capability Requests (NEW)
  - Security Policies (NEW)
- ✅ Capability Requests link navigates to /dashboard/admin/capability-requests
- ✅ Security Policies link navigates to /dashboard/admin/security-policies
- ✅ Collapse/expand button works
- ✅ Sidebar responsive on mobile

#### 18.2 Header/TopBar
**Test Cases**:
- ✅ Header visible on all dashboard pages
- ✅ User avatar/name displayed
- ✅ User menu dropdown works
- ✅ "Profile" link navigates (if implemented)
- ✅ "Change password" link navigates
- ✅ "Logout" button works
- ✅ Notification bell shows count (if implemented)

#### 18.3 Data Tables
**Test Cases**:
- ✅ Tables render correctly
- ✅ Column headers visible
- ✅ Sorting works (click column header)
- ✅ Pagination works (if enabled)
- ✅ Row actions (edit, delete) work
- ✅ Row click navigates to detail (where applicable)
- ✅ Empty state shown when no data
- ✅ Loading skeleton shown while fetching

#### 18.4 Forms
**Test Cases**:
- ✅ Form fields render correctly
- ✅ Field validation works (required, format, etc.)
- ✅ Error messages displayed below fields
- ✅ Submit button disabled until valid
- ✅ Submit button shows loading state
- ✅ Cancel button navigates back
- ✅ Success message after submission
- ✅ Error handling (network errors, etc.)

#### 18.5 Modals/Dialogs
**Test Cases**:
- ✅ Modal opens correctly
- ✅ Modal overlay blocks background interaction
- ✅ "X" close button works
- ✅ "Cancel" button closes modal
- ✅ "Confirm" button performs action
- ✅ Modal closes after action
- ✅ ESC key closes modal
- ✅ Click outside closes modal (where appropriate)

#### 18.6 Toast Notifications
**Test Cases**:
- ✅ Success toasts shown (green)
- ✅ Error toasts shown (red)
- ✅ Info toasts shown (blue)
- ✅ Warning toasts shown (yellow)
- ✅ Toasts auto-dismiss after 3-5 seconds
- ✅ Multiple toasts stack correctly
- ✅ Close button works

---

## 🔗 Integration Testing

### End-to-End User Flows

#### 19.1 User Registration & Login Flow
**Steps**:
1. Navigate to registration page
2. Fill out registration form
3. Submit registration
4. Verify redirect to pending page
5. Admin approves user
6. User logs in with credentials
7. Verify redirect to dashboard

**Expected**:
- ✅ User created in database
- ✅ User cannot login until approved
- ✅ After approval, user can login
- ✅ JWT token generated and stored
- ✅ Dashboard loads with user data

#### 19.2 Agent Creation & Verification Flow
**Steps**:
1. Login as MEMBER user
2. Navigate to agents page
3. Click "Create Agent"
4. Fill out agent creation form
5. Submit form
6. Verify redirect to success page
7. Copy credentials
8. Download SDK
9. Manager verifies agent
10. Check trust score calculated

**Expected**:
- ✅ Agent created in database
- ✅ Ed25519 keys generated automatically
- ✅ Agent appears in agents list
- ✅ SDK downloaded with embedded credentials
- ✅ Verification updates status
- ✅ Trust score calculated

#### 19.3 MCP Server Registration & Detection Flow
**Steps**:
1. Login as MEMBER user
2. Navigate to MCP servers page
3. Create new MCP server manually
4. Navigate to agent detail
5. Click "Detect MCPs"
6. Upload MCP config file
7. Verify auto-detected MCPs listed
8. Check capabilities detected
9. Verify agent-MCP relationship created

**Expected**:
- ✅ MCP server created in database
- ✅ Config file parsed correctly
- ✅ New MCPs auto-registered
- ✅ Capabilities detected and saved
- ✅ Agent-MCP relationship established

#### 19.4 API Key Creation & Usage Flow
**Steps**:
1. Login as MEMBER user
2. Navigate to API keys page
3. Click "Create API Key"
4. Fill out form (name, expiration)
5. Submit form
6. Copy plain text key (shown once)
7. Use key in API request (header: X-API-Key)
8. Verify API authentication works
9. Disable key
10. Verify API authentication fails

**Expected**:
- ✅ API key created in database
- ✅ Key hashed with SHA-256
- ✅ Plain text key shown once
- ✅ API request authenticated successfully
- ✅ Disabled key rejected

#### 19.5 Trust Score Calculation Flow
**Steps**:
1. Create new agent
2. Verify agent
3. Perform some actions (verify-action)
4. Manager calculates trust score
5. Check score displayed in UI
6. View trust score history
7. Verify trend chart

**Expected**:
- ✅ Trust score calculated (0-100)
- ✅ Score saved to database
- ✅ Score displayed on agent detail page
- ✅ History tracked over time
- ✅ Chart renders correctly

#### 19.6 Security Alert Flow
**Steps**:
1. Trigger security event (suspicious action)
2. Check alert created in database
3. Admin views alerts page
4. Alert visible with correct severity
5. Admin acknowledges alert
6. Admin resolves alert with notes
7. Verify alert status updated

**Expected**:
- ✅ Alert created automatically
- ✅ Severity calculated correctly
- ✅ Alert appears in admin dashboard
- ✅ Acknowledgement recorded
- ✅ Resolution notes saved
- ✅ Status updated to resolved

#### 19.7 Compliance Report Generation Flow
**Steps**:
1. Login as ADMIN
2. Navigate to compliance page
3. Select framework (SOC2, HIPAA, GDPR)
4. Select date range
5. Click "Generate Report"
6. Wait for report generation
7. View report in browser
8. Export report to CSV

**Expected**:
- ✅ Report generated successfully
- ✅ All required sections included
- ✅ Data accurate and up-to-date
- ✅ CSV export works
- ✅ Report downloadable

#### 19.8 Webhook Creation & Testing Flow
**Steps**:
1. Login as MEMBER user
2. Navigate to webhooks page
3. Create new webhook (URL: https://webhook.site)
4. Select events to subscribe
5. Save webhook
6. Click "Test webhook"
7. Check webhook.site for test event
8. Trigger real event (e.g., create agent)
9. Verify webhook called

**Expected**:
- ✅ Webhook created in database
- ✅ Test event sent successfully
- ✅ Real event triggers webhook
- ✅ Webhook payload correctly formatted
- ✅ Webhook signature included

#### 19.9 Capability Request Approval Flow
**Steps**:
1. Login as MEMBER user
2. Navigate to agent detail page
3. Click "Request Capability" button
4. Fill out capability request form:
   - Select capability type (database:write)
   - Enter reason (business justification)
5. Submit request
6. Verify request created with pending status
7. Logout and login as ADMIN
8. Navigate to capability requests page
9. Verify request appears in pending list
10. Click "Approve" button
11. Verify success message
12. Check request status updated to approved
13. Navigate to agent detail page
14. Verify capability now appears in capabilities section
15. Check agent_capabilities table has new entry

**Expected**:
- ✅ Request created with status: pending
- ✅ Requester recorded correctly
- ✅ Admin can see pending request
- ✅ Approval updates status to approved
- ✅ Reviewer information recorded
- ✅ Capability automatically granted to agent
- ✅ Capability visible on agent detail page
- ✅ Database tables updated correctly (capability_requests, agent_capabilities)

#### 19.10 Trust Score Auto-Grant Flow
**Steps**:
1. Create new agent
2. Agent automatically verified with initial capabilities
3. Calculate trust score (should be ≥ 0.30 for new verified agent)
4. Verify capabilities auto-granted:
   - Check agent_capabilities table
   - Verify capabilities based on trust score threshold
5. Login to dashboard
6. Navigate to agent detail page
7. Verify auto-granted capabilities displayed
8. Check granted_by is NULL (auto-granted)
9. Verify trust score badge shows ≥ 0.30

**Expected**:
- ✅ New agent trust score calculated automatically
- ✅ Trust score ≥ 0.30 triggers auto-grant
- ✅ Initial capabilities granted without manual approval
- ✅ Auto-granted capabilities have granted_by = NULL
- ✅ Agent can immediately use auto-granted capabilities
- ✅ UI shows auto-grant indicator
- ✅ Additional capabilities require approval (request flow)

#### 19.11 Security Policy Enforcement Flow
**Steps**:
1. Login as ADMIN
2. Navigate to security policies page
3. Verify 3 default policies exist:
   - Capability Violation Detection
   - Low Trust Score Monitoring
   - Unusual Activity Detection
4. Select "Capability Violation Detection" policy
5. Change enforcement mode to "Block & Alert"
6. Verify warning banner appears
7. Save changes
8. Trigger capability violation:
   - Agent attempts action without granted capability
   - Or agent attempts high-risk action
9. Verify action blocked (if enforcement mode = block_and_alert)
10. Check security alert created
11. Navigate to alerts page
12. Verify violation alert visible
13. Change enforcement back to "Alert Only"
14. Trigger same violation
15. Verify action allowed but alert created

**Expected**:
- ✅ Default policies created for all organizations
- ✅ Enforcement mode changes save correctly
- ✅ Block & Alert mode prevents unauthorized actions
- ✅ Alert Only mode allows actions but creates alerts
- ✅ Security alerts created with correct severity
- ✅ Policy priority determines execution order
- ✅ Disabled policies not enforced

---

## 🔒 Security Testing

### Authentication & Authorization

#### 20.1 JWT Token Security
**Test Cases**:
- ✅ Token contains user ID, email, role
- ✅ Token signed with secret key
- ✅ Token expires after configured time
- ✅ Expired token rejected (401)
- ✅ Invalid signature rejected (401)
- ✅ Token refresh works correctly
- ✅ Token rotation implemented

#### 20.2 Role-Based Access Control (RBAC)
**Test Cases**:
- ✅ VIEWER cannot create/edit/delete
- ✅ MEMBER can create agents/keys
- ✅ MEMBER cannot delete agents
- ✅ MANAGER can verify/delete agents
- ✅ ADMIN can manage users/org settings
- ✅ 403 Forbidden for unauthorized actions
- ✅ UI hides unauthorized actions

#### 20.3 API Key Security
**Test Cases**:
- ✅ API keys hashed with SHA-256
- ✅ Plain text key never stored
- ✅ Plain text key shown only once
- ✅ API key authentication works
- ✅ Disabled API key rejected
- ✅ Expired API key rejected
- ✅ Rate limiting applied to API key requests

#### 20.4 Password Security
**Test Cases**:
- ✅ Passwords hashed with bcrypt
- ✅ Minimum password strength enforced
- ✅ Password confirmation required
- ✅ Change password requires current password
- ✅ Passwords never logged or exposed
- ✅ Password reset flow secure (if implemented)

#### 20.5 SQL Injection Prevention
**Test Cases**:
- ✅ All queries use parameterized statements
- ✅ User input sanitized
- ✅ No raw SQL with user input
- ✅ Test with common SQL injection payloads:
  - `' OR '1'='1`
  - `'; DROP TABLE users; --`
  - `1' UNION SELECT * FROM users--`
- ✅ All inputs rejected or escaped

#### 20.6 Cross-Site Scripting (XSS) Prevention
**Test Cases**:
- ✅ User input sanitized before display
- ✅ HTML special characters escaped
- ✅ Test with XSS payloads:
  - `<script>alert('XSS')</script>`
  - `<img src=x onerror=alert('XSS')>`
  - `javascript:alert('XSS')`
- ✅ All payloads rendered as text, not executed

#### 20.7 Cross-Site Request Forgery (CSRF) Prevention
**Test Cases**:
- ✅ CSRF tokens implemented (if using cookies)
- ✅ SameSite cookie attribute set
- ✅ Referer header validation
- ✅ JWT in Authorization header (not cookies)

#### 20.8 Rate Limiting
**Test Cases**:
- ✅ Rate limiting applied to auth endpoints
- ✅ Rate limiting applied to API endpoints
- ✅ Rate limit headers included in response:
  - X-RateLimit-Limit
  - X-RateLimit-Remaining
  - X-RateLimit-Reset
- ✅ 429 Too Many Requests after limit
- ✅ Rate limit resets after window

#### 20.9 CORS Configuration
**Test Cases**:
- ✅ CORS enabled for frontend origin
- ✅ Only allowed origins accepted
- ✅ Credentials allowed for authenticated requests
- ✅ Unauthorized origins rejected
- ✅ Preflight requests handled correctly

#### 20.10 HTTPS/TLS (Production)
**Test Cases**:
- ✅ HTTPS enforced in production
- ✅ HTTP redirected to HTTPS
- ✅ TLS certificate valid
- ✅ TLS 1.2+ used
- ✅ Secure ciphers only

---

## ⚡ Performance Testing

### API Response Times

#### 21.1 Endpoint Performance Targets
**Target**: < 100ms p95 latency

**Test Cases**:
- ✅ GET /api/v1/agents → < 100ms
- ✅ POST /api/v1/agents → < 150ms
- ✅ GET /api/v1/agents/{id} → < 50ms
- ✅ POST /api/v1/auth/login → < 200ms
- ✅ GET /api/v1/analytics/dashboard → < 200ms
- ✅ POST /api/v1/agents/{id}/verify-action → < 100ms

**Tools**:
- Apache Bench: `ab -n 1000 -c 10 http://localhost:8080/api/v1/agents`
- K6 Load Testing
- Postman Collection Runner

#### 21.2 Database Query Performance
**Test Cases**:
- ✅ All queries have proper indexes
- ✅ N+1 query problems avoided
- ✅ Query execution time < 50ms
- ✅ Database connection pooling works
- ✅ Long-running queries identified and optimized

**Tools**:
- PostgreSQL `EXPLAIN ANALYZE`
- Database monitoring dashboard

#### 21.3 Frontend Page Load Times
**Target**: < 2s initial load, < 1s navigation

**Test Cases**:
- ✅ Dashboard page loads < 2s
- ✅ Agents list page loads < 2s
- ✅ Agent detail page loads < 1.5s
- ✅ Navigation between pages < 1s
- ✅ Code splitting implemented
- ✅ Assets minified and gzipped
- ✅ Images optimized

**Tools**:
- Chrome DevTools Lighthouse
- WebPageTest
- Network throttling testing

#### 21.4 Concurrent User Load Testing
**Test Cases**:
- ✅ 10 concurrent users → no degradation
- ✅ 50 concurrent users → < 200ms p95
- ✅ 100 concurrent users → < 500ms p95
- ✅ No memory leaks under load
- ✅ No connection pool exhaustion
- ✅ Graceful degradation at high load

**Tools**:
- K6 load testing scripts
- Artillery.io
- Locust (Python)

---

## 📝 Test Reporting

### Test Execution Report Format

**Use Google Sheets or Excel with the following columns**:

| Test ID | Feature | Test Case | Priority | Status | Expected | Actual | Notes | Severity | Assignee |
|---------|---------|-----------|----------|--------|----------|--------|-------|----------|----------|
| TC001 | Auth | Login with valid credentials | High | PASS | 200 OK with token | 200 OK with token | - | - | - |
| TC002 | Auth | Login with invalid password | High | FAIL | 401 Unauthorized | 500 Internal Error | Server error on wrong password | Critical | Backend |
| TC003 | Agents | Create agent as MEMBER | High | PASS | 201 Created | 201 Created | - | - | - |
| TC004 | Agents | Delete agent as VIEWER | Medium | FAIL | 403 Forbidden | 200 OK (deleted) | RBAC not enforced | High | Backend |

**Status Values**:
- PASS: Test passed as expected
- FAIL: Test failed
- BLOCKED: Cannot test due to blocker
- SKIP: Test skipped intentionally
- N/A: Not applicable

**Priority Values**:
- Critical: Must fix before launch
- High: Should fix before launch
- Medium: Fix if time permits
- Low: Nice to have

**Severity Values** (for bugs):
- Critical: Application crash, data loss, security vulnerability
- High: Major functionality broken, workaround exists
- Medium: Minor functionality broken, UI issue
- Low: Cosmetic issue, typo

---

## 🐛 Bug Report Template

### GitHub Issue Format

```markdown
## 🐛 Bug Report

### Description
Clear description of the bug

### Steps to Reproduce
1. Step 1
2. Step 2
3. Step 3

### Expected Behavior
What should happen

### Actual Behavior
What actually happens

### Screenshots/Videos
Attach screenshots or screen recordings

### Environment
- Browser: Chrome 120.0
- OS: macOS 14.0
- Backend Version: v1.0.0
- Frontend Version: v1.0.0

### API Request/Response (if applicable)
```http
POST http://localhost:8080/api/v1/agents
Authorization: Bearer {token}

{
  "name": "test-agent"
}
```

Response:
```json
{
  "error": "Internal server error"
}
```

### Console Errors (if applicable)
```
TypeError: Cannot read properties of undefined (reading 'name')
  at AgentList.tsx:45
```

### Database State (if applicable)
```sql
SELECT * FROM agents WHERE id = 'agent-id';
-- Shows agent with invalid status
```

### Severity
- [ ] Critical
- [ ] High
- [ ] Medium
- [ ] Low

### Priority
- [ ] Must fix
- [ ] Should fix
- [ ] Nice to have

### Assignee
@backend-team or @frontend-team
```

---

## 📊 Final Deliverables

### 1. Test Execution Report (Excel/Google Sheets)
- All test cases with PASS/FAIL status
- Bug count by severity
- Overall test coverage %
- Completion timeline

### 2. Bug Reports (GitHub Issues)
- One issue per bug
- Detailed reproduction steps
- Screenshots/videos attached
- Labeled with severity/priority

### 3. Performance Report (PDF/Markdown)
- API response times (p50, p95, p99)
- Page load times
- Database query performance
- Load testing results
- Recommendations for optimization

### 4. Security Audit Report (PDF/Markdown)
- Authentication/authorization findings
- OWASP Top 10 compliance
- Vulnerability scan results
- Recommendations for hardening

### 5. Summary Video (Optional)
- 5-10 minute walkthrough
- Demo of critical bugs
- Recommendations for fixes
- Overall assessment

---

## 📅 Estimated Timeline

| Phase | Duration | Tasks |
|-------|----------|-------|
| Setup | 2 hours | Environment setup, test data creation |
| Backend Testing | 10 hours | Test all 70+ API endpoints (including new capability requests & security policies) |
| Frontend Testing | 8 hours | Test all dashboard pages (including new admin pages) |
| Integration Testing | 4 hours | End-to-end user flows (including capability request & security policy flows) |
| Security Testing | 2 hours | Auth, RBAC, injection tests |
| Performance Testing | 2 hours | Load testing, profiling |
| Reporting | 3 hours | Document findings, create issues |
| **Total** | **31 hours** | **Complete QA cycle** |

---

## ✅ Success Criteria

### Acceptance Criteria
- ✅ All critical bugs fixed
- ✅ 90%+ test cases passing
- ✅ API response times < 100ms p95
- ✅ Page load times < 2s
- ✅ No security vulnerabilities (OWASP Top 10)
- ✅ RBAC properly enforced
- ✅ All user flows working end-to-end

### Definition of Done
- All test cases executed
- All bugs documented in GitHub
- Test report submitted
- Performance report submitted
- Security audit report submitted
- Recommendations provided

---

## 🤝 Communication & Support

### Questions During Testing
- Create GitHub Discussion for questions
- Tag @opena2a/developers for technical questions
- Tag @opena2a/product for feature clarification

### Bug Priority Escalation
- Critical bugs: Report immediately in Discord/Slack
- High bugs: Create GitHub issue within 24 hours
- Medium/Low bugs: Batch report at end of day

---

## 📚 Additional Resources

- **API Documentation**: http://localhost:8080/swagger
- **Frontend Repo**: https://github.com/opena2a/agent-identity-management
- **Backend Repo**: https://github.com/opena2a/agent-identity-management
- **Project Wiki**: https://github.com/opena2a/agent-identity-management/wiki
- **Architecture Docs**: `/docs/ARCHITECTURE.md`

---

**Last Updated**: October 11, 2025
**Version**: 1.1
**Maintainer**: OpenA2A Team (hello@opena2a.org)

## 📝 Changelog

### v1.1 (October 11, 2025)
- ✅ Added 10 new backend endpoint tests (Capability Requests & Security Policies)
- ✅ Added 2 new frontend page tests (Admin Capability Requests & Security Policies)
- ✅ Added 3 new integration test flows (Capability Request, Trust Score Auto-Grant, Security Policy Enforcement)
- ✅ Enhanced agent capabilities display testing with risk levels and auto-grant indicators
- ✅ Updated trust score testing to include auto-grant functionality
- ✅ Updated sidebar navigation testing to include new admin menu items
- ✅ Increased total testing time estimate from 26 to 31 hours
- ✅ Updated endpoint count from 60+ to 70+

### v1.0 (October 10, 2025)
- Initial release with comprehensive testing plan for all core features
