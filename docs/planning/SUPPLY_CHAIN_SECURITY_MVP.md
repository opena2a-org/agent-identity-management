# 🔒 Supply Chain Security for AI Agents - MVP Approach

**Goal**: Lightweight supply chain security that doesn't overwhelm MVP but provides real value.

---

## The Supply Chain Security Problem

### What is AI Agent Supply Chain?
```
Developer → Framework (LangChain/CrewAI) → MCP Server → External Tools → Data Access
    ↓              ↓                          ↓              ↓             ↓
  Can we        Does it have              Is it          Are they      Is the data
  trust         vulnerabilities?          verified?      malicious?    secure?
  them?
```

### Real-World Scenarios
1. **Compromised MCP Server**: Developer installs MCP server from GitHub, but it was backdoored
2. **Malicious Framework Plugin**: LangChain plugin claims to do X, actually exfiltrates data
3. **Dependency Confusion**: npm/PyPI package with same name as internal tool
4. **Typosquatting**: `langchan` instead of `langchain`
5. **Outdated Dependencies**: Agent uses vulnerable version of library

---

## MVP Supply Chain Features (Lightweight!)

### Feature 1: SDK Integrity Verification (2-3 hours) ✅ EASY WIN

**Problem**: How do we know the SDK downloaded from AIM wasn't tampered with?

**Solution**: SHA-256 checksum verification

```python
# When downloading SDK
response = requests.get(f"{aim_url}/api/v1/agents/{id}/sdk?lang=python")

# Backend adds checksum header
# X-AIM-SDK-Checksum: sha256:abc123def456...

# SDK installer verifies
downloaded_checksum = response.headers.get("X-AIM-SDK-Checksum")
actual_checksum = hashlib.sha256(response.content).hexdigest()

if f"sha256:{actual_checksum}" != downloaded_checksum:
    raise IntegrityError("SDK checksum mismatch - possible tampering detected!")
```

**Backend changes:**
```go
// In agent_handler.go DownloadSDK()
func (h *AgentHandler) DownloadSDK(c fiber.Ctx) error {
    // ... generate SDK ...

    // Calculate checksum
    checksum := sha256.Sum256(sdkBytes)
    checksumHex := hex.EncodeToString(checksum[:])

    // Add header
    c.Set("X-AIM-SDK-Checksum", fmt.Sprintf("sha256:%s", checksumHex))

    // Log for audit
    h.auditService.LogAction(ctx, orgID, userID, "sdk_download", "agent", agentID, {
        "checksum": checksumHex,
        "language": language,
    })

    return c.Send(sdkBytes)
}
```

**Value**: Detects MITM attacks, corrupted downloads, tampering

---

### Feature 2: Package Version Tracking (3-4 hours) ✅ MEDIUM EFFORT

**Problem**: Which versions of dependencies are agents using? Are they vulnerable?

**Solution**: Track SDK and framework versions in agent metadata

```python
# When auto-registering or on first verification
import sys
import importlib.metadata

def get_installed_packages():
    """Get relevant AI framework versions."""
    frameworks = {
        "langchain": None,
        "crewai": None,
        "openai": None,
        "anthropic": None,
        "aim-sdk": None
    }

    for package in frameworks.keys():
        try:
            frameworks[package] = importlib.metadata.version(package)
        except importlib.metadata.PackageNotFoundError:
            pass

    return frameworks

# Include in metadata
client = AIMClient.auto_register(
    name="my-agent",
    metadata={
        "packages": get_installed_packages(),
        "python_version": sys.version,
        "platform": sys.platform
    }
)
```

**Backend changes:**
```go
// Add to agents table
ALTER TABLE agents
ADD COLUMN dependency_snapshot JSONB;

// Store in CreateAgentWithAutoKeys
agent.DependencySnapshot = req.Metadata["packages"]
```

**Dashboard view:**
```
Agent: customer-service-bot
Dependencies:
  ✅ langchain: 0.2.5 (latest)
  ⚠️  openai: 0.28.0 (outdated - 1.0.0 available)
  ❌ requests: 2.25.0 (VULNERABLE - CVE-2023-xxxxx)
```

**Value**:
- Know which agents need updates
- Identify vulnerable dependencies
- Track adoption of new versions

---

### Feature 3: MCP Server Registry (4-5 hours) 📋 MEDIUM EFFORT

**Problem**: How do we know an MCP server is trustworthy?

**Solution**: Curated registry + community voting

```python
# MCP server registration
from aim_sdk.integrations.mcp import register_mcp_server

register_mcp_server(
    name="filesystem-mcp",
    source="github.com/anthropics/mcp-filesystem",
    version="1.0.0",
    checksum="sha256:abc123...",
    description="Official Anthropic filesystem MCP server"
)
```

**Database:**
```sql
CREATE TABLE mcp_registry (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    source VARCHAR(500) NOT NULL,  -- GitHub URL
    version VARCHAR(50) NOT NULL,
    checksum VARCHAR(128),
    verified_by_aim BOOLEAN DEFAULT FALSE,
    community_votes INT DEFAULT 0,
    downloads INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

**CLI verification:**
```bash
# Check if MCP server is in registry
aim verify-mcp github.com/anthropics/mcp-filesystem

# Output:
# ✅ Verified MCP Server
#    Name: filesystem-mcp
#    Source: github.com/anthropics/mcp-filesystem
#    Version: 1.0.0
#    Checksum: sha256:abc123...
#    Verified by AIM: Yes
#    Community Votes: 1,234
#    Downloads: 5,678
```

**Value**:
- Developers know which MCP servers are trusted
- Community can flag malicious servers
- AIM team can verify official servers

---

### Feature 4: Audit Trail for Dependency Changes (2-3 hours) ✅ EASY WIN

**Problem**: When did agent dependencies change? Who updated them?

**Solution**: Log all dependency updates

```python
# When agent updates SDK or dependencies
client = AIMClient.auto_register("my-agent", force_refresh=True)

# SDK detects changes
old_packages = load_from_file("~/.aim/credentials/my-agent.json")["packages"]
new_packages = get_installed_packages()

changes = detect_changes(old_packages, new_packages)

if changes:
    # Log to AIM
    client.log_dependency_change(
        old_versions=old_packages,
        new_versions=new_packages,
        changes=changes
    )
```

**Audit log entry:**
```json
{
  "event": "dependency_change",
  "agent_id": "uuid",
  "timestamp": "2025-10-07T...",
  "changes": [
    {
      "package": "langchain",
      "old_version": "0.2.4",
      "new_version": "0.2.5",
      "change_type": "upgrade"
    },
    {
      "package": "requests",
      "old_version": null,
      "new_version": "2.31.0",
      "change_type": "added"
    }
  ]
}
```

**Value**:
- Track when vulnerable dependencies were added
- Investigate security incidents
- Compliance audits (SOC 2, HIPAA)

---

### Feature 5: Vulnerability Scanning (Future - Post MVP)

**NOT in MVP, but architecture supports it:**

```python
# Future feature
def check_vulnerabilities(packages: dict) -> list:
    """Check against OSV database or Snyk API."""
    vulnerabilities = []

    for package, version in packages.items():
        vulns = osv_api.check(package, version)
        if vulns:
            vulnerabilities.extend(vulns)

    return vulnerabilities

# Would add to dashboard
# ❌ CRITICAL: Agent 'customer-bot' has 2 high-severity vulnerabilities
#    - CVE-2023-12345: SQL Injection in sqlalchemy < 2.0.0
#    - CVE-2023-67890: RCE in jinja2 < 3.1.0
```

---

## MVP Implementation Priority

### Phase 1: Quick Wins (6-8 hours total)
1. **SDK Integrity Verification** (2-3 hours) - Add checksum header
2. **Package Version Tracking** (3-4 hours) - Store dependency snapshot
3. **Audit Trail** (2-3 hours) - Log dependency changes

### Phase 2: Medium Effort (4-5 hours)
4. **MCP Registry** (4-5 hours) - Basic registry with verification

### Post-MVP (Future)
5. Vulnerability scanning integration (OSV, Snyk)
6. Automated dependency updates
7. SBOM (Software Bill of Materials) generation
8. Supply chain attack detection (ML-based)

---

## Integration with Existing Features

### Works with Auto-Registration
```python
# Automatically captures package versions on registration
client = AIMClient.auto_register(
    name="my-agent",
    # SDK automatically detects and sends:
    # - Python version
    # - Installed packages (langchain, openai, etc.)
    # - Platform info
)
```

### Works with Challenge-Response
```python
# Challenge-response includes supply chain metadata
challenge = {
    "nonce": "...",
    "agent_metadata": {
        "packages": get_installed_packages(),
        "last_update": "2025-10-07T..."
    }
}
```

### Works with Framework Integrations
```python
# LangChain integration auto-reports version
from aim_sdk.integrations.langchain import AIMCallbackHandler

# Automatically includes langchain version in audit logs
callback = AIMCallbackHandler()
```

---

## Dashboard UI (Simple MVP View)

### Agent Details Page
```
Agent: customer-service-bot
Status: Verified
Trust Score: 75%

Supply Chain Security:
  ✅ SDK Integrity: Verified
  ✅ Dependencies: Up to date
  ⚠️  MCP Servers: 1 unverified

Dependencies (5):
  ✅ aim-sdk: 1.0.0 (latest)
  ✅ langchain: 0.2.5 (latest)
  ✅ openai: 1.0.0 (latest)
  ⚠️  requests: 2.28.0 (update available: 2.31.0)
  ⚠️  pydantic: 1.10.0 (update available: 2.5.0)

[View Full Dependency Graph] [Check for Updates]
```

### Organization Dashboard
```
Supply Chain Overview:
  📦 Total Agents: 47
  ✅ Up to date: 38 (81%)
  ⚠️  Updates available: 7 (15%)
  ❌ Vulnerable: 2 (4%)

Vulnerable Agents:
  1. email-processor (CVE-2023-12345 - High)
  2. data-analyzer (CVE-2023-67890 - Medium)

[View Details] [Auto-Update]
```

---

## API Endpoints (Minimal for MVP)

### 1. Get Agent Dependencies
```http
GET /api/v1/agents/{id}/dependencies

Response:
{
  "agent_id": "uuid",
  "packages": {
    "langchain": "0.2.5",
    "openai": "1.0.0"
  },
  "last_updated": "2025-10-07T...",
  "updates_available": [
    {
      "package": "requests",
      "current": "2.28.0",
      "latest": "2.31.0"
    }
  ]
}
```

### 2. Verify MCP Server
```http
GET /api/v1/mcp/verify?source=github.com/org/repo

Response:
{
  "verified": true,
  "name": "filesystem-mcp",
  "verified_by_aim": true,
  "community_votes": 1234,
  "checksum": "sha256:abc123..."
}
```

---

## Benefits vs Effort

| Feature | Effort | Value | MVP? |
|---------|--------|-------|------|
| SDK Checksum | 2-3h | High | ✅ YES |
| Package Tracking | 3-4h | High | ✅ YES |
| Audit Trail | 2-3h | Medium | ✅ YES |
| MCP Registry | 4-5h | Medium | ✅ YES |
| Vulnerability Scanning | 8-12h | High | ❌ POST-MVP |
| Auto-Updates | 12-16h | Medium | ❌ POST-MVP |
| SBOM Generation | 6-8h | Low | ❌ POST-MVP |

**Total MVP effort**: 11-15 hours (spread across Phase 1 implementation)

---

## Framework Examples to Support

### LangChain
- Track langchain version
- Track LangChain plugins
- Verify official LangChain tools

### CrewAI
- Track crewai version
- Track crew dependencies
- Verify crew tool sources

### Microsoft Copilot
- Track copilot SDK version
- Verify copilot extensions
- Track API usage

### AutoGPT
- Track autogpt version
- Verify AutoGPT plugins
- Track plugin sources

---

## Testing Supply Chain Features

### Test 1: SDK Checksum Verification
```python
# Download SDK normally - should pass
response = requests.get(f"{url}/api/v1/agents/{id}/sdk")
checksum = response.headers.get("X-AIM-SDK-Checksum")
assert checksum.startswith("sha256:")

# Tamper with response - should fail
tampered = response.content + b"malicious"
with pytest.raises(IntegrityError):
    verify_sdk_integrity(tampered, checksum)
```

### Test 2: Package Version Tracking
```python
# Register with packages
client = AIMClient.auto_register("test", metadata={
    "packages": {"langchain": "0.2.5"}
})

# Verify stored in database
agent = get_agent(client.agent_id)
assert agent.dependency_snapshot["langchain"] == "0.2.5"
```

### Test 3: MCP Registry Lookup
```bash
# Verify known MCP server
aim verify-mcp github.com/anthropics/mcp-filesystem
# Expected: ✅ Verified

# Verify unknown MCP server
aim verify-mcp github.com/malicious/backdoor-mcp
# Expected: ⚠️  Not in registry
```

---

## Documentation Updates

### README Addition
```markdown
## Supply Chain Security

AIM helps secure your AI agent supply chain:

- **SDK Integrity**: Verify SDK downloads with SHA-256 checksums
- **Dependency Tracking**: Know what packages your agents use
- **MCP Registry**: Verify MCP servers before using them
- **Audit Trail**: Track all dependency changes

Example:
```python
# Automatically tracks dependencies
client = AIMClient.auto_register("my-agent")

# Verify MCP server
aim verify-mcp github.com/anthropics/mcp-filesystem
```

---

## Summary: Supply Chain Security MVP

**What we're building (MVP):**
1. ✅ SDK checksum verification (2-3 hours)
2. ✅ Package version tracking (3-4 hours)
3. ✅ Dependency change audit trail (2-3 hours)
4. ✅ Basic MCP registry (4-5 hours)

**Total time**: 11-15 hours

**What we're NOT building (yet):**
- ❌ Automated vulnerability scanning
- ❌ Auto-updates
- ❌ SBOM generation
- ❌ ML-based attack detection

**Why this MVP approach works:**
- Lightweight enough to not delay core features
- Valuable enough to differentiate from competitors
- Foundation for advanced features later
- Addresses real security concerns

**Atomic Habits principle**: Make secure supply chain management EASY (automatic tracking) and OBVIOUS (clear dashboard warnings).

---

**Add to Phase 1 implementation as you go - don't make it a separate phase!**
