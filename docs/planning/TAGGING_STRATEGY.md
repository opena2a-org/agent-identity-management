# 🏷️ AIM Tagging & Asset Classification Strategy

**Date**: October 8, 2025
**Status**: Design Phase - Premium Feature
**Target**: Enterprise Edition

---

## 🎯 Executive Summary

Implement a comprehensive tagging and asset classification system to help organizations:
- **Discover assets** quickly (e.g., "Show me all filesystem MCP servers")
- **Enforce policies** (e.g., "All database MCPs must be tagged 'production' or 'development'")
- **Generate reports** (e.g., "How many API integration agents do we have?")
- **Improve security** (e.g., "Which MCP servers have access to sensitive data?")
- **Track compliance** (e.g., "Show all HIPAA-compliant agents")

---

## 📊 Business Value (Premium Feature Justification)

### For Enterprise Customers
1. **Asset Discovery**:
   - **Agents**: "Show me all AI agents that process customer data"
   - **MCPs**: "Find all filesystem MCP servers in production"
2. **Compliance Reporting**:
   - **Agents**: "List all agents with PII access for SOC 2 audit"
   - **MCPs**: "Show all database MCPs that are HIPAA-compliant"
3. **Cost Allocation**:
   - **Agents**: "Which department owns these AI agents?" (chargeback by cost-center)
   - **MCPs**: "What's the infrastructure cost for marketing team's agents?"
4. **Risk Management**:
   - **Cross-asset**: "Alert me if any production-tagged agent communicates with a dev-tagged MCP"
   - **Agent-specific**: "Which agents have high-risk capabilities and are in production?"
5. **Governance**:
   - **Policy**: "Require all agents AND MCPs to have environment, owner, and cost-center tags"
   - **Validation**: "Prevent deploying agents without required compliance tags"

### Why Agents Need Tags Even More Than MCPs

**Agents are MORE critical to tag because**:
1. **Dynamic Behavior**: Agents make decisions and take actions (higher risk than static MCPs)
2. **Data Access**: Agents often access sensitive data across multiple MCPs
3. **Compliance**: Regulators care about "WHO accessed the data" (agents) more than "HOW" (MCPs)
4. **Cost**: Agents consume AI tokens/compute (need cost attribution)
5. **Scale**: Organizations may have 100s of agents vs dozens of MCPs

### Premium Tier Differentiation
| Feature | Community (Free) | Pro | Enterprise |
|---------|------------------|-----|------------|
| Basic Tags (3 tags max) | ✅ | ✅ | ✅ |
| Custom Tags (unlimited) | ❌ | ✅ | ✅ |
| Required Tags (policy enforcement) | ❌ | ❌ | ✅ |
| Tag-based RBAC | ❌ | ❌ | ✅ |
| Tag Taxonomies/Hierarchies | ❌ | ❌ | ✅ |
| Tag Compliance Reports | ❌ | ✅ | ✅ |
| Tag Auto-suggestions | ❌ | ✅ | ✅ |

---

## 🏗️ Tag Categories (Proposed Taxonomy)

### 1. **Resource Type Tags** (Auto-detected from MCP metadata)
**Purpose**: Categorize what the MCP server/agent does

**MCP Server Types**:
- `filesystem` - File operations (read, write, list)
- `database` - Database connections (PostgreSQL, MySQL, MongoDB)
- `api` - External API integrations (REST, GraphQL, SOAP)
- `cloud` - Cloud provider services (AWS, Azure, GCP)
- `security` - Security tools (vault, secrets manager)
- `communication` - Messaging, email, notifications
- `analytics` - Data processing, reporting
- `ai-ml` - AI/ML model services
- `monitoring` - Observability, logging, metrics
- `git` - Version control operations

**Agent Types** (Critical for compliance and risk management):
- `ai-agent` - General purpose AI agent (default)
- `workflow-agent` - Orchestration and workflow automation
- `data-agent` - Data processing and transformation
- `integration-agent` - System integration (connects multiple services)
- `monitoring-agent` - System monitoring and observability
- `security-agent` - Security scanning and validation
- `customer-facing-agent` - Direct customer interaction (chatbots, support)
- `internal-agent` - Internal operations only
- `autonomous-agent` - Fully autonomous decision-making
- `supervised-agent` - Human-in-the-loop required

**Why Agent Type Tags Matter**:
- **Risk scoring**: `autonomous-agent` + `production` + `pii-access` = HIGH RISK
- **Compliance**: "Show all `customer-facing-agent` with data access for audit"
- **Governance**: "Only `supervised-agent` can access financial data"

**How to Auto-Detect**:
```typescript
// Option 1: From MCP manifest
{
  "name": "filesystem-mcp",
  "capabilities": ["read_file", "write_file", "list_directory"],
  "tags": {
    "resource_type": "filesystem" // ✅ MCP self-declares
  }
}

// Option 2: Smart detection from capabilities
function detectResourceType(capabilities: string[]): string[] {
  const tags = [];

  if (capabilities.some(c => c.includes('file') || c.includes('directory'))) {
    tags.push('filesystem');
  }

  if (capabilities.some(c => c.includes('database') || c.includes('sql'))) {
    tags.push('database');
  }

  // ... more detection logic

  return tags;
}

// Option 3: User selects during registration (with suggestions)
// UI shows: "We detected this might be a 'filesystem' MCP. Confirm or select:"
```

### 2. **Environment Tags** (User-defined, REQUIRED in Enterprise)
**Purpose**: Separate production from non-production assets

- `production` - Live production environment
- `staging` - Pre-production staging
- `development` - Developer environments
- `testing` - QA/test environments
- `sandbox` - Experimental/sandbox

**CRITICAL for Agents**: Environment tags prevent agents from accidentally:
- Using production data in development
- Deploying untested agents to production
- Mixing test and prod API keys

**Enterprise Policy Examples**:
```yaml
# Example 1: Require environment tag on ALL assets
required_tags:
  - environment

# Example 2: Prevent cross-environment communication
policies:
  - name: "No prod-dev communication"
    rule: "agents with tag 'production' cannot talk to MCPs with tag 'development'"
    severity: "critical"
    action: "block" # Prevent registration if violated

# Example 3: Agent-specific policies
  - name: "Prod agents must be supervised"
    rule: "agents with tag 'production' must also have tag 'agent-type:supervised'"
    severity: "high"
    action: "warn"

# Example 4: Data access restrictions
  - name: "PII agents require approval"
    rule: "agents with tag 'data-classification:pii' require manual approval"
    severity: "critical"
    action: "block"
```

**Real-World Scenario**:
```
❌ BLOCKED: Agent "CustomerDataProcessor" registration failed
   Reason: Production agent with PII access missing required tags
   Missing: owner, cost-center, compliance:soc2
   Action: Add required tags or demote to development environment
```

### 3. **Ownership Tags** (Enterprise feature)
**Purpose**: Track who owns/maintains assets

- `owner:team-platform` - Platform team owns this
- `owner:team-ml` - ML team owns this
- `cost-center:engineering` - Charge to engineering budget
- `department:sales` - Sales department asset

### 4. **Security/Compliance Tags** (Premium feature)
**Purpose**: Track data sensitivity and compliance requirements

- `data-classification:public` - Public data only
- `data-classification:internal` - Internal data
- `data-classification:confidential` - Confidential data
- `data-classification:pii` - Contains personally identifiable information
- `compliance:soc2` - SOC 2 compliant
- `compliance:hipaa` - HIPAA compliant
- `compliance:gdpr` - GDPR compliant

### 5. **Business Context Tags** (Custom)
**Purpose**: Organization-specific categorization

- `project:customer-portal` - Part of customer portal project
- `service:billing` - Billing service
- `criticality:high` - Business-critical asset
- `region:us-east` - Geographical region

---

## 🗄️ Database Schema

### Tags Table
```sql
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL, -- 'resource_type', 'environment', 'ownership', 'security', 'custom'
    description TEXT,
    color VARCHAR(7), -- Hex color for UI (e.g., '#3B82F6')
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),

    UNIQUE(organization_id, key, value)
);

CREATE INDEX idx_tags_org_category ON tags(organization_id, category);
CREATE INDEX idx_tags_key_value ON tags(key, value);
```

### Agent Tags (Many-to-Many)
```sql
CREATE TABLE agent_tags (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (agent_id, tag_id)
);

CREATE INDEX idx_agent_tags_agent ON agent_tags(agent_id);
CREATE INDEX idx_agent_tags_tag ON agent_tags(tag_id);
```

### MCP Server Tags (Many-to-Many)
```sql
CREATE TABLE mcp_server_tags (
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (mcp_server_id, tag_id)
);

CREATE INDEX idx_mcp_server_tags_mcp ON mcp_server_tags(mcp_server_id);
CREATE INDEX idx_mcp_server_tags_tag ON mcp_server_tags(tag_id);
```

### Tag Policies (Enterprise)
```sql
CREATE TABLE tag_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    required_tags JSONB NOT NULL DEFAULT '[]', -- e.g., ["environment", "owner"]
    validation_rules JSONB, -- Custom validation rules
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(organization_id, name)
);
```

---

## 🎨 UI/UX Design

### 1. Agent Registration (Tag Selection - MORE CRITICAL)
```
┌─────────────────────────────────────────────────┐
│ Register AI Agent                               │
├─────────────────────────────────────────────────┤
│ Name: CustomerSupportBot                        │
│ Type: AI Agent                                  │
│                                                  │
│ 🏷️ Tags (* = Required by your organization)    │
│                                                  │
│ Agent Type: * [customer-facing-agent ▼]        │
│   ⚠️ Customer-facing agents require extra       │
│      compliance validation                      │
│                                                  │
│ Environment: * [production ▼]                   │
│   🔒 Production requires: owner, cost-center    │
│                                                  │
│ Data Classification: * [pii ▼]                  │
│   ⚠️ PII access requires compliance tags        │
│                                                  │
│ Owner: * [team-support ▼]                       │
│ Cost Center: * [support-ops ▼]                  │
│ Compliance: [soc2 ▼] [hipaa ▼]                 │
│                                                  │
│ + Add custom tag                                │
│                                                  │
│ Current tags:                                   │
│ [customer-facing-agent] [production] [pii]      │
│ [team-support] [support-ops] [soc2] [hipaa]    │
│                                                  │
│ ✅ All required tags present                    │
│ ⚠️ Recommendation: Add tag "region:us-east"    │
│                                                  │
│           [Cancel]  [Register Agent]            │
└─────────────────────────────────────────────────┘
```

### 2. MCP Server Registration (Tag Selection)
```
┌─────────────────────────────────────────────────┐
│ Register MCP Server                             │
├─────────────────────────────────────────────────┤
│ Name: Filesystem MCP Server                     │
│ URL: http://localhost:3100                      │
│                                                  │
│ 🏷️ Tags (Recommended)                           │
│                                                  │
│ Resource Type: [filesystem ▼]                   │
│   Suggestions based on capabilities:            │
│   ✓ filesystem (from: read_file, write_file)   │
│                                                  │
│ Environment: * [production ▼]                   │
│   Required by your organization                 │
│                                                  │
│ Owner: [team-platform ▼]                        │
│                                                  │
│ + Add custom tag                                │
│                                                  │
│ Current tags:                                   │
│ [filesystem] [production] [team-platform]       │
│                                                  │
│           [Cancel]  [Register Server]           │
└─────────────────────────────────────────────────┘
```

### 3. Agent Detail Modal with Tags
```
┌─────────────────────────────────────────────────┐
│ CustomerSupportBot                              │
│ abc123-456-789                                  │
├─────────────────────────────────────────────────┤
│ Status      Trust Score      Type               │
│ Verified    85.0             AI Agent           │
│                                                  │
│ 🏷️ Tags                                          │
│ [customer-facing-agent] [production] [pii]      │
│ [team-support] [support-ops] [soc2] [hipaa]    │
│                                                  │
│ ⚠️ Risk Assessment: HIGH                        │
│ Reasons:                                        │
│ • Production environment                        │
│ • PII data access                               │
│ • Customer-facing (external exposure)           │
│                                                  │
│ 🔒 Compliance Status: ✅ COMPLIANT              │
│ • SOC 2 certified                               │
│ • HIPAA compliant                               │
│ • Required tags present                         │
│                                                  │
│ Capabilities                                    │
│ [customer_query] [ticket_creation] [pii_lookup] │
│                                                  │
│ Authorized MCP Servers (based on tags)          │
│ [CustomerDB] [TicketingSystem]                  │
│ (Only production MCPs with matching compliance) │
│                                                  │
│ ...                                             │
└─────────────────────────────────────────────────┘
```

### 4. MCP Server Detail Modal with Tags
```
┌─────────────────────────────────────────────────┐
│ Filesystem MCP Server                           │
│ 51bd4b5b-87a6-426f-ab14-9b490d7226e4           │
├─────────────────────────────────────────────────┤
│ Status      Trust Score      Capabilities       │
│ Verified    75.0             3                  │
│                                                  │
│ 🏷️ Tags                                          │
│ [filesystem] [production] [team-platform]       │
│                                                  │
│ Capabilities                                    │
│ [read_file] [write_file] [list_directory]      │
│                                                  │
│ ...                                             │
└─────────────────────────────────────────────────┘
```

### 3. Filter by Tags (Dashboard)
```
┌─────────────────────────────────────────────────┐
│ MCP Servers Dashboard                           │
├─────────────────────────────────────────────────┤
│ Filters:                                        │
│ [All Tags ▼] [Environment: production ▼]       │
│ [Resource Type: filesystem ▼]                   │
│                                                  │
│ Active Filters:                                 │
│ [production ×] [filesystem ×] Clear all         │
│                                                  │
│ ┌─────────────────────────────────────────────┐ │
│ │ Filesystem MCP Server                       │ │
│ │ [filesystem] [production] [team-platform]   │ │
│ │ Trust Score: 75.0 | Capabilities: 3         │ │
│ └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### 4. Tag Management (Enterprise Admin)
```
┌─────────────────────────────────────────────────┐
│ Tag Management                                  │
├─────────────────────────────────────────────────┤
│ [+ Create Tag]  [📊 Tag Usage Report]          │
│                                                  │
│ Resource Type Tags (System-managed)             │
│ ┌───────────┬─────────┬──────────────────────┐ │
│ │ Tag       │ Assets  │ Description          │ │
│ ├───────────┼─────────┼──────────────────────┤ │
│ │filesystem │ 12 MCPs │ File system ops      │ │
│ │database   │ 8 MCPs  │ Database connections │ │
│ └───────────┴─────────┴──────────────────────┘ │
│                                                  │
│ Environment Tags (Required)                     │
│ ┌──────────────┬─────────┬───────────────────┐ │
│ │ Tag          │ Assets  │ Policy            │ │
│ ├──────────────┼─────────┼───────────────────┤ │
│ │production    │ 45      │ Required on all   │ │
│ │staging       │ 23      │ Optional          │ │
│ └──────────────┴─────────┴───────────────────┘ │
└─────────────────────────────────────────────────┘
```

---

## 🔌 API Endpoints (New)

### Tag Management
```typescript
// Create tag
POST /api/v1/organizations/{orgId}/tags
{
  "key": "environment",
  "value": "production",
  "category": "environment",
  "description": "Production environment",
  "color": "#22C55E"
}

// List all tags for organization
GET /api/v1/organizations/{orgId}/tags
GET /api/v1/organizations/{orgId}/tags?category=environment
GET /api/v1/organizations/{orgId}/tags?key=environment

// Delete tag (only if not in use)
DELETE /api/v1/organizations/{orgId}/tags/{tagId}
```

### Apply Tags to Assets
```typescript
// Add tags to MCP server
POST /api/v1/mcp-servers/{mcpId}/tags
{
  "tag_ids": ["uuid1", "uuid2", "uuid3"]
}

// Remove tag from MCP server
DELETE /api/v1/mcp-servers/{mcpId}/tags/{tagId}

// Get MCP servers by tag
GET /api/v1/mcp-servers?tags=filesystem,production
GET /api/v1/mcp-servers?tags=environment:production

// Add tags to agent
POST /api/v1/agents/{agentId}/tags
DELETE /api/v1/agents/{agentId}/tags/{tagId}
```

### Tag Policies (Enterprise)
```typescript
// Create tag policy (require certain tags)
POST /api/v1/organizations/{orgId}/tag-policies
{
  "name": "Require Environment Tag",
  "description": "All agents and MCPs must have an environment tag",
  "required_tags": ["environment"],
  "applies_to": ["agents", "mcp_servers"]
}

// Validate asset against policies
POST /api/v1/organizations/{orgId}/validate-tags
{
  "asset_type": "mcp_server",
  "tags": ["filesystem", "production"]
}
// Response: { "valid": true, "missing_required_tags": [] }
```

### Tag Analytics (Premium)
```typescript
// Tag usage report
GET /api/v1/organizations/{orgId}/tag-analytics
// Response:
{
  "total_tags": 45,
  "tags_by_category": {
    "resource_type": 12,
    "environment": 4,
    "ownership": 15,
    "security": 8,
    "custom": 6
  },
  "top_tags": [
    { "key": "environment", "value": "production", "asset_count": 45 },
    { "key": "resource_type", "value": "filesystem", "asset_count": 12 }
  ],
  "untagged_assets": {
    "agents": 3,
    "mcp_servers": 1
  }
}

// Find untagged assets
GET /api/v1/organizations/{orgId}/untagged-assets
```

---

## 🤖 Auto-Detection Strategy

### 1. From MCP Server Manifest (Preferred)
Many MCP servers have a manifest or metadata file:
```json
// mcp-manifest.json
{
  "name": "filesystem-mcp",
  "version": "1.0.0",
  "tags": {
    "resource_type": "filesystem",
    "capabilities": ["read", "write", "list"]
  },
  "tools": [
    {
      "name": "read_file",
      "description": "Read file contents",
      "inputSchema": {...}
    }
  ]
}
```

**Implementation**:
```go
// During MCP registration, fetch manifest
func RegisterMCP(req RegisterMCPRequest) (*MCPServer, error) {
    // Fetch MCP manifest (if available)
    manifest := fetchMCPManifest(req.URL)

    // Auto-detect tags from manifest
    autoTags := []string{}
    if manifest.Tags.ResourceType != "" {
        autoTags = append(autoTags, manifest.Tags.ResourceType)
    }

    // Or detect from capabilities
    if len(manifest.Tools) > 0 {
        detectedType := detectResourceTypeFromTools(manifest.Tools)
        autoTags = append(autoTags, detectedType)
    }

    // Suggest to user (don't auto-apply without confirmation)
    return &MCPServer{
        SuggestedTags: autoTags,
        // ...
    }
}
```

### 2. From Capability Analysis (Smart Detection)
```typescript
function detectResourceType(capabilities: string[]): string[] {
  const detectionRules = {
    filesystem: ['read_file', 'write_file', 'list_directory', 'delete_file'],
    database: ['query', 'execute_sql', 'transaction', 'connection'],
    api: ['http_request', 'rest_call', 'graphql_query'],
    cloud: ['aws_', 'azure_', 'gcp_', 's3_', 'lambda_'],
    security: ['encrypt', 'decrypt', 'sign', 'verify', 'vault'],
  };

  const tags = [];

  for (const [tag, keywords] of Object.entries(detectionRules)) {
    const matches = capabilities.filter(cap =>
      keywords.some(keyword => cap.toLowerCase().includes(keyword))
    );

    if (matches.length > 0) {
      tags.push(tag);
    }
  }

  return tags;
}

// Example:
const capabilities = ['read_file', 'write_file', 'list_directory'];
const detected = detectResourceType(capabilities);
// Returns: ['filesystem']
```

### 3. From URL/Name Pattern Matching
```typescript
function detectFromName(name: string, url: string): string[] {
  const tags = [];

  const patterns = {
    filesystem: /file.*system|fs|storage/i,
    database: /postgres|mysql|mongo|database|db/i,
    api: /api|rest|graphql|http/i,
    git: /git|github|gitlab/i,
  };

  for (const [tag, pattern] of Object.entries(patterns)) {
    if (pattern.test(name) || pattern.test(url)) {
      tags.push(tag);
    }
  }

  return tags;
}

// Example:
detectFromName('Filesystem MCP Server', 'http://localhost:3100');
// Returns: ['filesystem']
```

---

## 🚦 Implementation Phases

### Phase 1: Foundation (MVP) - 2 weeks
**Goal**: Basic tagging functionality (Community + Pro)

- [ ] Database schema (tags, agent_tags, mcp_server_tags)
- [ ] Backend API (CRUD tags, apply/remove tags)
- [ ] Basic tag UI (add/remove tags in detail modal)
- [ ] Tag filtering in dashboard
- [ ] Auto-detection from capabilities (smart suggestions)

**Endpoints**: 8 new endpoints
**Database**: 3 new tables
**UI**: Tag chips, filter dropdowns

### Phase 2: Enterprise Features - 3 weeks
**Goal**: Required tags, policies, compliance

- [ ] Tag policies table and enforcement
- [ ] Required tags validation
- [ ] Tag-based RBAC (e.g., "User can only see production-tagged assets")
- [ ] Tag compliance reports
- [ ] Tag audit logs (who added/removed tags)

**Endpoints**: 6 new endpoints
**Database**: 1 new table (tag_policies)
**UI**: Tag policy management page

### Phase 3: Advanced Analytics - 2 weeks
**Goal**: Tag insights and recommendations

- [ ] Tag usage analytics
- [ ] Untagged asset detection
- [ ] Tag recommendations (ML-based)
- [ ] Tag hierarchies (e.g., environment > production > us-east)
- [ ] Bulk tag operations

**Endpoints**: 4 new endpoints
**UI**: Tag analytics dashboard

---

## 💰 Revenue Impact (Premium Justification)

### Enterprise Pricing Tier
**Why enterprises pay for tagging**:

1. **Compliance Requirements**: "We need to prove which agents access PII for GDPR"
2. **Cost Attribution**: "Which department owns this agent?" (chargeback)
3. **Policy Enforcement**: "All prod agents must have owner tag"
4. **Audit Trail**: "Show me who changed tags on this critical MCP"
5. **Scale**: Enterprises have 100s-1000s of agents/MCPs

### Competitive Analysis
| Competitor | Tagging Feature | Pricing |
|------------|-----------------|---------|
| AWS IAM | ✅ Tag-based policies | Included in Enterprise Support ($15K/month) |
| Azure AD | ✅ Resource tags | Included in P2 ($9/user/month) |
| Okta | ✅ Custom attributes | Enterprise only (custom pricing) |
| **AIM (Our offering)** | ✅ Advanced tagging | **Enterprise tier ($5K/month base)** |

---

## 📝 Example User Stories

### Story 1: Security Officer
**Goal**: "I need to audit all customer-facing agents that access PII data in production"

```
1. Navigate to Agents dashboard
2. Click "Advanced Filters"
3. Select filters:
   - Agent Type: "customer-facing-agent"
   - Environment: "production"
   - Data Classification: "pii"
4. System shows 12 matching agents
5. Click "Risk Assessment Report"
6. Export PDF with:
   - Agent names
   - Trust scores
   - Last activity
   - Compliance status
   - Authorized MCPs
   - Owner teams
```

**Why this matters**: SOC 2 auditors ask "Show me all systems that process customer PII"

### Story 2: DevOps Engineer
**Goal**: "I want to find all filesystem MCP servers in production"

```
1. Navigate to MCP Servers dashboard
2. Click filter dropdown
3. Select "Resource Type: filesystem"
4. Select "Environment: production"
5. View filtered list of 5 filesystem MCPs in production
```

### Story 3: Finance/Cost Allocation
**Goal**: "How much are we spending on AI agents per department?"

```
1. Navigate to Cost Analytics (Enterprise feature)
2. View "Cost by Tag" dashboard
3. Group by: "cost-center"
4. System shows:
   - support-ops: 45 agents, $12K/month in AI tokens
   - engineering: 23 agents, $8K/month
   - sales: 12 agents, $3K/month
5. Drill down into "support-ops"
6. See breakdown by agent:
   - CustomerSupportBot: $4K/month (high token usage)
   - EmailClassifier: $1K/month
7. Identify cost optimization opportunities
```

**Why this matters**: CFOs need to allocate cloud costs to departments for chargeback

### Story 4: Compliance Officer
**Goal**: "Generate quarterly compliance report for all PII-accessing agents"

```
1. Navigate to Compliance Reports
2. Select "PII Access Audit Report"
3. Date Range: Last 90 days
4. System filters agents with tag "data-classification:pii"
5. Export PDF with:
   - Agent Name
   - Owner Team
   - Last Activity
   - Trust Score
   - Environment
   - Compliance Tags (SOC2, HIPAA, GDPR)
   - Verification Events (all cryptographic proofs)
   - Access Patterns (which MCPs were accessed)
6. Share with external auditor
```

### Story 5: Enterprise Admin
**Goal**: "Enforce tag policies - require all production agents to have owner, cost-center, and compliance tags"

```
1. Navigate to Tag Policies
2. Click "Create Policy"
3. Name: "Production Agent Requirements"
4. Condition: "environment = production AND asset_type = agent"
5. Required Tags: ["owner", "cost-center", "compliance"]
6. Enforcement: "Block registration if missing"
7. Save policy

8. Developer tries to register new agent:
   - Name: "NewDataProcessor"
   - Environment: production
   - Tags: [production] only

9. Registration BLOCKED:
   ❌ "Missing required tags for production agents:
      - owner (required)
      - cost-center (required)
      - compliance (required)"

10. Developer adds missing tags:
    - owner: team-data
    - cost-center: data-ops
    - compliance: soc2

11. Registration succeeds ✅

12. Admin dashboard shows:
    - Policy compliance: 98% (3 agents non-compliant)
    - Action: Fix 3 legacy agents
```

---

## 🎓 Documentation Needed

### User Documentation
1. **Tag Best Practices Guide**
   - How to choose tags
   - Standard tag taxonomy
   - Examples by industry

2. **Tag Policy Guide** (Enterprise)
   - How to create policies
   - Common policy patterns
   - Enforcement workflows

3. **Tag API Reference**
   - All endpoints with examples
   - SDKs for Python, JavaScript, Go

### Admin Documentation
1. **Tag Management Guide**
   - Bulk operations
   - Tag migration
   - Tag cleanup

2. **Compliance Reporting**
   - Pre-built reports
   - Custom report creation
   - Audit log analysis

---

## 🔐 Security Considerations

### Access Control
```typescript
// Only users with tag-management permission can create/delete tags
permissions: {
  'tags:create': ['admin', 'tag-manager'],
  'tags:delete': ['admin'],
  'tags:apply': ['admin', 'tag-manager', 'developer'],
  'tags:view': ['*'] // All users can view tags
}

// Tag-based RBAC (Enterprise)
// "User can only see assets tagged with their team"
user_policy: {
  visible_tag_filters: {
    owner: "team-platform" // User only sees assets owned by their team
  }
}
```

### Audit Logging
```sql
-- Track all tag changes
INSERT INTO audit_logs (
    organization_id,
    user_id,
    action,
    resource_type,
    resource_id,
    details,
    created_at
) VALUES (
    $1, $2, 'tag_applied',
    'mcp_server', $3,
    '{"tag_key": "environment", "tag_value": "production"}',
    CURRENT_TIMESTAMP
);
```

---

## ✅ Success Metrics

### Adoption Metrics
- **Tag Coverage**: % of assets with at least one tag (target: 90%+)
- **Tag Consistency**: % of assets with required tags (target: 100%)
- **Tag Usage**: Average tags per asset (target: 3-5)

### Business Metrics
- **Enterprise Conversion**: % of Pro users upgrading for tag policies (target: 20%)
- **Feature Usage**: % of enterprise customers using tag-based reports (target: 80%)
- **Support Reduction**: Reduction in "How do I find X?" support tickets (target: 30%)

---

## 🎯 Next Steps

1. **User Research** (1 week)
   - Interview 10 enterprise prospects
   - What tags do they currently use?
   - What policies do they need?

2. **Technical Design** (1 week)
   - Finalize database schema
   - API contract design
   - UI mockups

3. **MVP Development** (2 weeks)
   - Phase 1 implementation
   - Basic tagging + filtering

4. **Beta Testing** (2 weeks)
   - 5 enterprise beta customers
   - Feedback on tag taxonomy
   - Refinement

5. **GA Launch** (Week 7)
   - Release as Enterprise feature
   - Pricing: +$2K/month for tag policies

---

**Built by**: Claude Sonnet 4.5
**Stack**: Go + Fiber v3, PostgreSQL 16, Next.js 15, TypeScript
**License**: Apache 2.0
**Project**: OpenA2A Agent Identity Management

---

## 🔗 Related Documents

- **AIM_VISION.md** - Overall product strategy
- **REQUIRED_FIELDS_UPDATE_COMPLETE.md** - Capabilities implementation
- **CAPABILITY_BASED_ACCESS_CONTROL.md** - Access control foundation
- **CLAUDE.md** - Development guidelines
