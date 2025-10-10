# 🤖 Intelligent Agent Capability Detection

**Status**: Design Phase
**Author**: Claude Code
**Date**: October 9, 2025
**Scope**: Detect agent capabilities beyond just MCP usage

---

## 🎯 Vision

While MCP detection tells us **what tools an agent uses**, we also need to detect **what capabilities the agent itself has**. This is crucial for:

- **Trust Scoring**: Agents with more capabilities need higher scrutiny
- **Security Assessment**: Detect risky capabilities (file system access, network calls)
- **Compliance**: Ensure agents meet regulatory requirements
- **RBAC**: Grant permissions based on detected capabilities
- **Dashboard Visualization**: Show agent "power level" and risk profile

---

## 🧠 What We Need to Detect

### 1. **Programming Environment**
- Language: Python, JavaScript/TypeScript, Go, Java, etc.
- Runtime version: Python 3.11, Node.js 18, Go 1.21, etc.
- Frameworks: LangChain, CrewAI, AutoGPT, LlamaIndex, Haystack, etc.
- Package ecosystem: pip, npm, go modules, Maven, etc.

### 2. **AI Model Usage**
- Primary model: Claude (Opus/Sonnet/Haiku), GPT-4, Llama, Mistral, etc.
- Model providers: Anthropic, OpenAI, Hugging Face, Cohere, etc.
- Model switching: Single model vs multi-model
- Batch API usage: Cost optimization indicator
- Embedding models: For RAG/vector search

### 3. **Data Access Capabilities**
- **File System**: Read/write files, directories, uploads
- **Databases**: PostgreSQL, MySQL, MongoDB, Redis, etc.
- **Vector Stores**: Pinecone, Qdrant, Weaviate, Chroma, etc.
- **APIs**: External REST APIs, GraphQL, webhooks
- **Cloud Storage**: S3, GCS, Azure Blob, etc.

### 4. **Network Capabilities**
- **HTTP/HTTPS**: Making web requests
- **WebSockets**: Real-time connections
- **SSH**: Remote server access
- **FTP/SFTP**: File transfer
- **Email**: SMTP/IMAP access

### 5. **Security-Sensitive Capabilities**
- **Code Execution**: `eval()`, `exec()`, dynamic imports
- **Shell Commands**: `os.system()`, `child_process.exec()`
- **Credential Access**: Reading env vars, keyring access
- **Cryptography**: Encryption, signing, hashing
- **Browser Automation**: Puppeteer, Playwright, Selenium

### 6. **Agent Architecture**
- **Type**: Autonomous, semi-autonomous, tool-based, reactive
- **Memory**: Short-term, long-term, episodic, semantic
- **Planning**: ReAct, Chain-of-Thought, Tree-of-Thought, etc.
- **Multi-Agent**: Single agent vs agent swarm
- **Human-in-loop**: Requires approvals vs fully autonomous

### 7. **Deployment Capabilities**
- **Environment**: Local, Docker, Kubernetes, serverless
- **Scaling**: Single instance vs distributed
- **Monitoring**: Logging, metrics, tracing
- **CI/CD**: Automated deployment pipeline

---

## 🏗️ Detection Architecture (3-Tier System)

### Tier 1: Static Analysis (<10ms)

**Package/Dependency Scanning**:
```javascript
// JavaScript: Scan package.json
{
  "dependencies": {
    "@anthropic-ai/sdk": "^0.9.0",     // → Capability: Claude API
    "langchain": "^0.0.200",           // → Capability: LangChain framework
    "pg": "^8.11.0",                   // → Capability: PostgreSQL database
    "puppeteer": "^21.0.0",            // → Capability: Browser automation
    "openai": "^4.0.0"                 // → Capability: OpenAI API
  }
}
```

**Import Statement Analysis**:
```python
# Python: Scan import statements
import anthropic                    # → Capability: Claude API
from langchain.agents import Agent  # → Capability: LangChain framework
import psycopg2                     # → Capability: PostgreSQL
from playwright.sync_api import *   # → Capability: Browser automation
```

**Config File Analysis**:
```yaml
# docker-compose.yml → Capability: Docker deployment
# Dockerfile → Capability: Container deployment
# .env → Capability: Environment-based config
```

---

### Tier 2: Runtime Detection (<0.1% CPU)

**Module Load Hooks**:
```javascript
// Hook require() to detect loaded modules
const anthropic = require('@anthropic-ai/sdk');
// → Detected: Claude API usage at runtime
```

**API Call Monitoring** (Passive):
```javascript
// Hook fetch() to detect API endpoints
fetch('https://api.anthropic.com/v1/messages', ...)
// → Detected: Claude API, specific endpoint
```

**File System Access Tracking**:
```javascript
// Hook fs.readFile/writeFile
fs.writeFile('/tmp/data.txt', data)
// → Detected: File write capability
```

**Database Connection Detection**:
```python
# Detect database connections
conn = psycopg2.connect("postgresql://...")
# → Detected: PostgreSQL database capability
```

---

### Tier 3: Deep Inspection (Opt-in)

**AST Parsing for Code Patterns**:
```python
# Detect dangerous patterns
exec(user_input)           # → CRITICAL: Code execution risk
os.system(command)         # → HIGH: Shell command execution
eval(expression)           # → CRITICAL: Dynamic code evaluation
```

**Behavioral Analysis**:
```javascript
// Detect agent behavior patterns
- Number of API calls per minute
- File system access patterns
- Network request destinations
- Resource consumption (CPU, memory)
```

---

## 📊 Capability Taxonomy

### Capability Risk Levels

**LOW RISK** (Trust Score Impact: +0 to -5):
- Reading static files
- HTTP GET requests to known APIs
- Basic logging
- Environment variable reading (non-sensitive)

**MEDIUM RISK** (Trust Score Impact: -5 to -15):
- Writing files
- Database queries
- POST/PUT requests to APIs
- Browser automation (read-only)
- SSH connections (read-only)

**HIGH RISK** (Trust Score Impact: -15 to -30):
- Dynamic code execution (`eval`, `exec`)
- Shell command execution
- Credential access (keyring, env vars)
- File system modifications (beyond temp files)
- Network scanning/port probing

**CRITICAL RISK** (Trust Score Impact: -30 to -50):
- Unrestricted shell access
- Root/admin privilege escalation
- Cryptographic key generation (without user consent)
- Mass data exfiltration patterns
- Self-modification (code that modifies itself)

---

## 🎛️ Configuration API

### Basic Usage (Auto-detect everything)
```typescript
import { detectAgentCapabilities } from '@aim/sdk';

const capabilities = await detectAgentCapabilities();
// Returns: {
//   language: 'javascript',
//   runtime: 'node-v18.17.0',
//   frameworks: ['langchain', 'express'],
//   aiModels: ['claude-3-opus', 'claude-3-sonnet'],
//   capabilities: {
//     fileSystem: { read: true, write: true, level: 'MEDIUM' },
//     database: { types: ['postgresql', 'redis'], level: 'MEDIUM' },
//     network: { http: true, websocket: true, level: 'LOW' },
//     codeExecution: { eval: false, exec: false, level: 'LOW' },
//     browserAutomation: { enabled: true, level: 'MEDIUM' }
//   },
//   riskScore: 35,  // 0-100, lower is safer
//   trustImpact: -12  // Impact on trust score
// }
```

### Advanced Configuration
```typescript
const capabilities = await detectAgentCapabilities({
  level: 'deep',  // Tier 3 deep inspection
  scanBehavior: true,  // Monitor runtime behavior
  trackAPICalls: true,  // Log API endpoints
  detectDangerousPatterns: true,  // AST parsing for risky code

  // Privacy options
  anonymizeEndpoints: true,  // Hide sensitive URLs
  excludeSecrets: true  // Don't report env var values
});
```

---

## 🔍 Example Detection Results

### Example 1: LangChain Agent with Claude
```typescript
{
  agentId: 'agent-123',
  detectedAt: '2025-10-09T22:00:00Z',

  environment: {
    language: 'python',
    version: '3.11.4',
    frameworks: ['langchain', 'fastapi'],
    packageManager: 'pip'
  },

  aiModels: [
    {
      provider: 'anthropic',
      models: ['claude-3-opus-20240229', 'claude-3-sonnet-20240229'],
      usage: 'primary',
      batchAPI: false
    }
  ],

  capabilities: {
    fileSystem: {
      read: true,
      write: true,
      paths: ['/tmp/*', './data/*'],
      riskLevel: 'MEDIUM'
    },
    database: {
      types: ['postgresql', 'redis'],
      operations: ['read', 'write'],
      riskLevel: 'MEDIUM'
    },
    network: {
      http: true,
      websocket: true,
      endpoints: [
        'api.anthropic.com',
        'api.example.com'
      ],
      riskLevel: 'LOW'
    },
    codeExecution: {
      eval: false,
      exec: false,
      dynamicImports: true,
      riskLevel: 'LOW'
    }
  },

  architecture: {
    type: 'autonomous',
    memory: ['short-term', 'long-term'],
    planning: 'ReAct',
    humanInLoop: false
  },

  riskAssessment: {
    overallRiskScore: 35,  // 0-100
    riskLevel: 'MEDIUM',
    trustScoreImpact: -12,
    recommendations: [
      'Enable human-in-loop for database writes',
      'Restrict file system access to /tmp only',
      'Add rate limiting for API calls'
    ]
  }
}
```

### Example 2: Dangerous Agent (HIGH RISK)
```typescript
{
  agentId: 'agent-456',
  detectedAt: '2025-10-09T22:00:00Z',

  capabilities: {
    codeExecution: {
      eval: true,        // 🚨 CRITICAL
      exec: true,        // 🚨 CRITICAL
      shellCommands: true,  // 🚨 HIGH RISK
      riskLevel: 'CRITICAL'
    },
    credentials: {
      keyringAccess: true,  // 🚨 HIGH RISK
      envVarAccess: true,
      riskLevel: 'HIGH'
    }
  },

  riskAssessment: {
    overallRiskScore: 85,  // 🚨 VERY HIGH RISK
    riskLevel: 'CRITICAL',
    trustScoreImpact: -45,
    alerts: [
      {
        severity: 'CRITICAL',
        capability: 'code_execution',
        message: 'Agent uses eval() and exec() - CODE INJECTION RISK',
        recommendation: 'Disable dynamic code execution or sandbox agent'
      },
      {
        severity: 'HIGH',
        capability: 'credential_access',
        message: 'Agent accesses system keyring',
        recommendation: 'Restrict keyring access or require user approval'
      }
    ]
  }
}
```

---

## 🔐 Security & Privacy

**Privacy Principles**:
1. **No data exfiltration**: All detection happens locally
2. **Anonymize sensitive data**: Hide API keys, credentials, URLs
3. **User consent**: Tier 3 detection requires explicit opt-in
4. **Minimal logging**: Only log capability metadata, not actual data

**Security Principles**:
1. **Read-only scanning**: No modification of agent code
2. **Sandboxed analysis**: Isolate detection from agent execution
3. **Fail-safe**: If detection fails, agent continues normally
4. **No interference**: Detection doesn't affect agent behavior

---

## 📈 Integration with Trust Scoring

```typescript
// Trust score calculation with agent capabilities
const trustScore = calculateTrustScore({
  baseScore: 80,

  // MCP usage (-0 to -20)
  mcpServers: ['filesystem', 'github', 'memory'],
  mcpImpact: -8,

  // Agent capabilities (-0 to -50)
  capabilities: {
    fileSystem: -5,
    database: -5,
    codeExecution: -25,  // 🚨 Heavy penalty
    credentials: -10
  },
  capabilityImpact: -45,

  // Verification (+5 to +15)
  verification: {
    ed25519Signed: true,
    challengeResponse: true
  },
  verificationBonus: +12,

  finalScore: 80 - 8 - 45 + 12 = 39  // MEDIUM-LOW trust
});
```

---

## 🎯 Use Cases

### 1. Dashboard Visualization
```
Agent: my-autonomous-agent
Trust Score: 42/100 (MEDIUM)

Capabilities Detected:
✅ Python 3.11 with LangChain
✅ Claude Opus (primary), GPT-4 (fallback)
⚠️  PostgreSQL database access (read/write)
⚠️  File system access (/tmp/*, ./data/*)
🚨 Shell command execution (HIGH RISK)

Recommendations:
1. Enable human-in-loop for shell commands
2. Restrict database writes to approved tables
3. Add rate limiting for API calls
```

### 2. Security Alerts
```
🚨 CRITICAL ALERT: agent-456

Detected dangerous capability:
  - eval() usage detected in agent code
  - Risk: Code injection vulnerability
  - Trust score impact: -30

Action Required:
  [ ] Disable dynamic code execution
  [ ] Sandbox agent in isolated environment
  [ ] Add code review workflow
```

### 3. Compliance Reporting
```
Compliance Check: GDPR Data Processing

Agent: data-processor-agent
Capabilities:
  ✅ Data encryption (AES-256)
  ✅ Secure credential storage (keyring)
  ⚠️  Database access (PostgreSQL)
  ⚠️  File system writes (./exports/*)

GDPR Compliance: PARTIAL
Issues:
  - Data retention policy not detected
  - Data deletion capability not confirmed

Recommendation: Add data lifecycle management
```

---

## 🚀 Implementation Plan

### Phase 1: JavaScript SDK (Priority 1)
- [ ] Design capability detection API
- [ ] Implement Tier 1 static analysis
- [ ] Implement Tier 2 runtime hooks
- [ ] Add risk scoring algorithm
- [ ] Write comprehensive tests

### Phase 2: Go SDK (Priority 2)
- [ ] Port JavaScript implementation to Go
- [ ] Adapt for Go-specific patterns
- [ ] Add Go module analysis
- [ ] Write comprehensive tests

### Phase 3: Python SDK (Priority 3)
- [ ] Port to Python SDK
- [ ] Add Python-specific detection (decorators, etc.)
- [ ] Import hook integration
- [ ] Write comprehensive tests

### Phase 4: Frontend Integration (Priority 4)
- [ ] Add capability visualization to dashboard
- [ ] Add risk score display
- [ ] Add security alerts UI
- [ ] Add compliance reporting

---

## 📝 Next Steps

1. **Finalize Architecture** (This document)
2. **Implement JavaScript SDK** (Tier 1 + Tier 2)
3. **Add to Trust Score Calculation**
4. **Update Frontend Dashboard**
5. **Write Documentation**

---

**Key Insight**: Agent capability detection is **equally important** as MCP detection because it gives us the full picture:

- **MCP Detection** = What tools the agent uses
- **Capability Detection** = What the agent itself can do

Together, they provide **complete visibility** into agent behavior and risk profile.

---

**Status**: 📋 Design Complete, Ready for Implementation
**Priority**: 🔥 HIGH (Equal to MCP detection)
**Impact**: 🎯 Critical for trust scoring, security, and compliance

---

**Built by**: Claude Code (World's Best Architect 🏗️)
**Date**: October 9, 2025
