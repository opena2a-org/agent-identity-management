# Intelligent MCP Detection - Implementation Plan

## Overview
Build intelligent MCP detection system with 3 modules (SCAN, DEPS, RTMN) that operates in "ghost mode" - zero noticeable performance impact on agents.

---

## Architecture

### Module Structure
```
apps/backend/internal/
├── modules/
│   ├── scan/           # Code Scanner module
│   │   ├── scanner.go              # Main scanner interface
│   │   ├── javascript_scanner.go   # JS/TS AST parser
│   │   ├── python_scanner.go       # Python AST parser
│   │   ├── go_scanner.go           # Go AST parser
│   │   └── cache.go                # Scan result caching
│   │
│   ├── deps/           # Dependency Analyzer module
│   │   ├── analyzer.go             # Main analyzer interface
│   │   ├── npm_analyzer.go         # package.json parser
│   │   ├── pip_analyzer.go         # requirements.txt parser
│   │   ├── go_mod_analyzer.go      # go.mod parser
│   │   └── cache.go                # Dependency cache
│   │
│   └── rtmn/           # Runtime Monitor module
│       ├── monitor.go              # Main monitor interface
│       ├── process_watcher.go      # Process monitoring
│       ├── network_watcher.go      # Network connection tracking
│       ├── sampler.go              # Statistical sampling
│       └── resource_limiter.go     # CPU/memory limits
│
└── application/
    └── mcp_detection/
        ├── orchestrator.go         # Coordinates all 3 modules
        ├── config_parser.go        # Config file parser (Core)
        └── result_merger.go        # Deduplicates and merges results
```

### Database Schema Addition
```sql
-- Detection results cache
CREATE TABLE mcp_detection_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    detection_method VARCHAR(50) NOT NULL, -- 'scan', 'deps', 'rtmn', 'config'
    file_hash VARCHAR(64) NOT NULL,        -- Git commit hash or file hash
    mcp_servers JSONB NOT NULL,            -- Array of detected MCPs
    confidence_score DECIMAL(5,2) NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,       -- TTL for cache

    UNIQUE(agent_id, detection_method, file_hash),
    INDEX idx_detection_cache_lookup (agent_id, detection_method, expires_at)
);

-- Detection performance metrics
CREATE TABLE mcp_detection_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    detection_method VARCHAR(50) NOT NULL,
    execution_time_ms INTEGER NOT NULL,
    cpu_usage_percent DECIMAL(5,2),
    memory_usage_mb INTEGER,
    success BOOLEAN NOT NULL,
    error_message TEXT,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    INDEX idx_detection_metrics_perf (detection_method, detected_at)
);
```

---

## Phase 1: Core Foundation (Config Parser + Caching)

### 1.1 Implement Cache Layer
**File**: `apps/backend/internal/modules/cache/detection_cache.go`

```go
package cache

type DetectionCache interface {
    Get(agentID uuid.UUID, method string, fileHash string) (*CachedResult, error)
    Set(agentID uuid.UUID, method string, fileHash string, result *DetectionResult, ttl time.Duration) error
    Invalidate(agentID uuid.UUID, method string) error
}

type CachedResult struct {
    MCPServers      []string
    ConfidenceScore float64
    DetectedAt      time.Time
}
```

**Requirements**:
- Redis-backed cache with PostgreSQL fallback
- TTL support (configurable per method)
- Cache invalidation by agent ID or detection method
- Atomic operations for concurrent access

### 1.2 Implement Config Parser (Core)
**File**: `apps/backend/internal/application/mcp_detection/config_parser.go`

**Features**:
- Parse Claude Desktop config (`claude_desktop_config.json`)
- Parse agent-specific configs (`.mcprc`, `mcp.config.json`)
- 85% confidence score
- Fast execution (<100ms)
- No external dependencies

**Cache Strategy**:
- Cache key: `config:<agent_id>:<file_hash>`
- TTL: 24 hours
- Invalidate on file change (watch file modification time)

---

## Phase 2: SCAN Module (Code Scanner)

### 2.1 Scanner Interface
**File**: `apps/backend/internal/modules/scan/scanner.go`

```go
package scan

type Scanner interface {
    Scan(ctx context.Context, agentPath string) (*ScanResult, error)
    GetSupportedLanguages() []string
}

type ScanResult struct {
    MCPServers      []MCPServerDetection
    ConfidenceScore float64
    ExecutionTimeMS int64
    FilesScanned    int
}

type MCPServerDetection struct {
    Name            string
    FilePath        string
    LineNumber      int
    DetectionType   string // "import", "client_init", "tool_call"
    CodeSnippet     string
}
```

### 2.2 JavaScript/TypeScript Scanner
**File**: `apps/backend/internal/modules/scan/javascript_scanner.go`

**Implementation**:
- Use `github.com/evanw/esbuild` for AST parsing (fast, production-ready)
- Scan for:
  - `import { ... } from '@modelcontextprotocol/sdk'`
  - `new MCPClient(...)`
  - `client.connectToServer(...)`
- Incremental scanning (git diff only)
- 95% confidence score

**Performance Optimizations**:
- Parse only `.js`, `.ts`, `.jsx`, `.tsx` files
- Skip `node_modules/`, `.git/`, `dist/`, `build/`
- Parallel file processing (worker pool)
- Max 5MB file size limit (skip large files)
- Timeout: 5 seconds per file

### 2.3 Python Scanner
**File**: `apps/backend/internal/modules/scan/python_scanner.go`

**Implementation**:
- Use `github.com/google/go-python-ast` or shell out to `python -m ast`
- Scan for:
  - `import mcp` or `from mcp import ...`
  - `MCPClient(...)` instantiation
  - `client.connect_to_server(...)`

### 2.4 Go Scanner
**File**: `apps/backend/internal/modules/scan/go_scanner.go`

**Implementation**:
- Use Go's native `go/parser` and `go/ast` packages
- Scan for MCP SDK imports

### 2.5 Caching Strategy
**Cache Key**: `scan:<agent_id>:<git_commit_hash>`
**TTL**: 7 days (code doesn't change often)
**Invalidation**: On git commit (webhook or file watcher)

---

## Phase 3: DEPS Module (Dependency Analyzer)

### 3.1 Analyzer Interface
**File**: `apps/backend/internal/modules/deps/analyzer.go`

```go
package deps

type Analyzer interface {
    Analyze(ctx context.Context, manifestPath string) (*AnalysisResult, error)
    GetSupportedManifests() []string
}

type AnalysisResult struct {
    MCPDependencies []MCPDependency
    ConfidenceScore float64
    ExecutionTimeMS int64
}

type MCPDependency struct {
    Name            string
    Version         string
    ManifestFile    string
    DependencyType  string // "direct", "transitive"
    IsVerified      bool   // Future: SBOM/attestation verification
}
```

### 3.2 NPM Analyzer
**File**: `apps/backend/internal/modules/deps/npm_analyzer.go`

**Implementation**:
- Parse `package.json` and `package-lock.json`
- Look for `@modelcontextprotocol/*` packages
- Check both `dependencies` and `devDependencies`
- 90% confidence score

**Performance**:
- JSON parsing only (no npm install)
- Cache based on `package-lock.json` hash
- <100ms execution time

### 3.3 Pip Analyzer
**File**: `apps/backend/internal/modules/deps/pip_analyzer.go`

**Implementation**:
- Parse `requirements.txt`, `Pipfile`, `pyproject.toml`
- Look for `mcp` or `mcp-*` packages

### 3.4 Go Module Analyzer
**File**: `apps/backend/internal/modules/deps/go_mod_analyzer.go`

**Implementation**:
- Parse `go.mod` and `go.sum`
- Look for MCP SDK packages

### 3.5 Caching Strategy
**Cache Key**: `deps:<agent_id>:<manifest_file_hash>`
**TTL**: 7 days
**Invalidation**: On package.json/requirements.txt/go.mod change

---

## Phase 4: RTMN Module (Runtime Monitor)

### 4.1 Monitor Interface
**File**: `apps/backend/internal/modules/rtmn/monitor.go`

```go
package rtmn

type Monitor interface {
    Start(ctx context.Context, agentID uuid.UUID) error
    Stop(agentID uuid.UUID) error
    GetDetections(agentID uuid.UUID) (*MonitoringResult, error)
}

type MonitoringResult struct {
    MCPConnections  []MCPConnection
    ConfidenceScore float64
    SamplingRate    float64
}

type MCPConnection struct {
    MCPServerName   string
    ProcessID       int
    RemoteAddress   string
    Port            int
    ConnectionType  string // "stdio", "sse", "websocket"
    FirstSeenAt     time.Time
}
```

### 4.2 Process Watcher
**File**: `apps/backend/internal/modules/rtmn/process_watcher.go`

**Implementation**:
- Use `github.com/shirou/gopsutil` for process monitoring
- Detect MCP server processes by:
  - Command line arguments containing "mcp"
  - Parent-child process relationships
  - stdio/sse transport detection
- 100% confidence (if detected at runtime, it's real)

**Performance**:
- Statistical sampling (1% of agent calls by default)
- Low-priority goroutine (nice +19)
- CPU limit: 2% of single core
- Memory limit: 50MB

### 4.3 Network Watcher
**File**: `apps/backend/internal/modules/rtmn/network_watcher.go`

**Implementation**:
- Monitor network connections from agent process
- Detect MCP SSE/WebSocket connections
- Use `/proc/net/tcp` on Linux, `netstat` on macOS

### 4.4 Resource Limiter
**File**: `apps/backend/internal/modules/rtmn/resource_limiter.go`

**Implementation**:
- Enforce CPU/memory limits using cgroups (Linux) or `setrlimit` (macOS)
- Auto-disable monitoring if limits exceeded
- Graceful degradation

### 4.5 Caching Strategy
**Cache Key**: `rtmn:<agent_id>:<hour>` (hourly aggregation)
**TTL**: 1 hour
**Invalidation**: Automatic (time-based)

---

## Phase 5: Detection Orchestrator

### 5.1 Orchestrator
**File**: `apps/backend/internal/application/mcp_detection/orchestrator.go`

```go
package mcp_detection

type Orchestrator struct {
    configParser ConfigParser
    scanner      scan.Scanner
    analyzer     deps.Analyzer
    monitor      rtmn.Monitor
    cache        cache.DetectionCache
}

func (o *Orchestrator) DetectAll(ctx context.Context, req DetectionRequest) (*DetectionResponse, error) {
    // Run all methods in parallel with timeout
    // Merge and deduplicate results
    // Return unified response
}

type DetectionRequest struct {
    AgentID         uuid.UUID
    AgentPath       string
    EnabledMethods  []string // ["config", "scan", "deps", "rtmn"]
    Timeout         time.Duration
}

type DetectionResponse struct {
    MCPServers      []DetectedMCPServer
    ExecutionTimeMS int64
    MethodsUsed     []MethodResult
}

type DetectedMCPServer struct {
    Name            string
    ConfidenceScore float64
    DetectedBy      []string // ["scan", "deps"]
    Details         map[string]interface{}
}

type MethodResult struct {
    Method          string
    Success         bool
    ExecutionTimeMS int64
    ServersFound    int
    ErrorMessage    string
}
```

**Implementation Strategy**:
- Run all 4 methods concurrently (goroutines)
- Set timeout for each method (5s scan, 1s deps, 2s config, 10s rtmn)
- If method times out, log warning and continue
- Merge results with deduplication
- Calculate weighted confidence scores

### 5.2 Result Merger
**File**: `apps/backend/internal/application/mcp_detection/result_merger.go`

**Deduplication Logic**:
```
If same MCP detected by multiple methods:
- Boost confidence score (max 99%)
- Combine details from all methods
- Mark as "verified" (detected by 2+ methods)

Confidence Calculation:
- Single method: Use method's base confidence
- Two methods: Average + 10% bonus
- Three+ methods: Average + 20% bonus (cap at 99%)
```

---

## Phase 6: API Endpoints

### 6.1 New Endpoint
**Route**: `POST /api/v1/agents/:id/mcp-servers/detect-intelligent`

**Request Body**:
```json
{
  "methods": ["config", "scan", "deps", "rtmn"],
  "agentPath": "/path/to/agent/code",
  "autoRegister": false,
  "timeout": 30000
}
```

**Response**:
```json
{
  "mcpServers": [
    {
      "name": "filesystem",
      "confidenceScore": 95.5,
      "detectedBy": ["scan", "deps"],
      "details": {
        "scan": {
          "filePath": "src/index.ts",
          "lineNumber": 42,
          "codeSnippet": "import { FilesystemClient } from '@modelcontextprotocol/filesystem'"
        },
        "deps": {
          "version": "1.2.3",
          "dependencyType": "direct"
        }
      }
    }
  ],
  "executionTimeMs": 2341,
  "methodsUsed": [
    {
      "method": "scan",
      "success": true,
      "executionTimeMs": 1823,
      "serversFound": 3
    },
    {
      "method": "deps",
      "success": true,
      "executionTimeMs": 156,
      "serversFound": 2
    }
  ],
  "cached": false
}
```

### 6.2 Update Existing Auto-Detect Endpoint
**Route**: `POST /api/v1/agents/:id/mcp-servers/detect`

**Changes**:
- Keep backward compatibility (still supports Claude Desktop config)
- Add new `intelligent: true` option to enable all 4 methods
- Default to config-only for now (breaking changes later)

---

## Phase 7: UI Updates

### 7.1 Detection Method Badges
**File**: `apps/web/components/agents/detection-method-badge.tsx`

**UI Components**:
- Badge for each detection method with icon
- Color coding:
  - SCAN: Blue (code icon)
  - DEPS: Green (package icon)
  - RTMN: Orange (activity icon)
  - CONFIG: Gray (file icon)
- Confidence score tooltip
- Click to see details

### 7.2 Enhanced Auto-Detect Modal
**File**: `apps/web/components/agents/auto-detect-button.tsx`

**Updates**:
- Checkbox toggles for each detection method
- Real-time progress (show which method is running)
- Performance metrics display (execution time per method)
- Grouped results by confidence score
- Conflict resolution UI (if methods disagree)

### 7.3 Detection Results Table
**Component**: Enhanced MCPServerList

**New Columns**:
- Confidence Score (progress bar)
- Detected By (method badges)
- Last Detected (timestamp)
- Details (expandable row)

---

## Phase 8: Performance Monitoring

### 8.1 Metrics Collection
**File**: `apps/backend/internal/modules/metrics/detection_metrics.go`

**Metrics to Track**:
- Execution time per method
- CPU usage during detection
- Memory usage during detection
- Cache hit rate
- Success/failure rates
- Timeout occurrences

### 8.2 Performance Dashboard
**File**: `apps/web/app/dashboard/admin/performance/page.tsx`

**Dashboard Widgets**:
- Average detection time by method
- Resource usage graphs (CPU/memory over time)
- Cache effectiveness (hit rate, storage size)
- Error rate by method
- Slowest agents (top 10)

### 8.3 Auto-Tuning
**File**: `apps/backend/internal/modules/rtmn/auto_tuner.go`

**Smart Defaults**:
- If detection consistently times out, reduce timeout
- If cache hit rate is low, increase TTL
- If CPU usage is high, reduce sampling rate
- If errors are frequent, disable problematic method

---

## Phase 9: Testing

### 9.1 Unit Tests
- Each scanner implementation (JS, Python, Go)
- Each analyzer implementation (NPM, Pip, Go)
- Cache layer (Redis + PostgreSQL)
- Result merger (deduplication logic)

### 9.2 Integration Tests
- End-to-end detection flow
- Concurrent detections (race conditions)
- Cache invalidation scenarios
- Timeout handling
- Resource limit enforcement

### 9.3 Performance Tests
- Benchmark each method (<5s target)
- Load test (1000 concurrent detections)
- Memory leak detection (long-running monitor)
- CPU usage profiling

### 9.4 Test Agent Fixtures
Create sample agent codebases for testing:
- `test/fixtures/js-agent/` - JavaScript agent using @modelcontextprotocol/sdk
- `test/fixtures/py-agent/` - Python agent using mcp library
- `test/fixtures/go-agent/` - Go agent using MCP SDK

---

## Phase 10: Documentation

### 10.1 User Documentation
**File**: `docs/features/intelligent-mcp-detection.md`

**Content**:
- Overview of 4 detection methods
- When to use each method
- Performance considerations
- Configuration options
- Troubleshooting guide

### 10.2 Developer Documentation
**File**: `docs/development/detection-architecture.md`

**Content**:
- System architecture diagram
- Module interaction flow
- Cache strategy explanation
- Performance optimization techniques
- Adding new language scanners

---

## Performance Targets

### Execution Time
- **Config Parser**: <100ms
- **SCAN Module**: <3s (per agent)
- **DEPS Module**: <500ms
- **RTMN Module**: <10s (initial scan), then <1% overhead
- **Total (all methods)**: <5s for 95% of agents

### Resource Usage
- **CPU**: Max 5% of single core
- **Memory**: Max 100MB RSS
- **Disk I/O**: Read-only, no writes during detection
- **Network**: No external API calls (all local)

### Reliability
- **Cache Hit Rate**: >80% for repeat detections
- **Success Rate**: >95% (graceful degradation on failure)
- **Timeout Rate**: <5% (auto-tune if exceeded)

---

## Configuration

### Environment Variables
```bash
# Detection Configuration
AIM_DETECTION_ENABLED_METHODS=scan,deps,config  # rtmn opt-in only
AIM_DETECTION_TIMEOUT_MS=30000
AIM_DETECTION_CACHE_TTL_HOURS=24

# Performance Limits
AIM_DETECTION_MAX_CPU_PERCENT=5
AIM_DETECTION_MAX_MEMORY_MB=100
AIM_DETECTION_MAX_FILE_SIZE_MB=5

# RTMN Specific
AIM_RTMN_ENABLED=false
AIM_RTMN_SAMPLING_RATE=0.01  # 1%
AIM_RTMN_CPU_LIMIT_PERCENT=2

# Cache Configuration
AIM_CACHE_BACKEND=redis  # redis or postgres
AIM_CACHE_REDIS_URL=redis://localhost:6379
```

### Database Migrations
```sql
-- Migration: 029_create_detection_cache_tables.up.sql
CREATE TABLE mcp_detection_cache (...);
CREATE TABLE mcp_detection_metrics (...);
CREATE INDEX idx_detection_cache_lookup ON mcp_detection_cache(agent_id, detection_method, expires_at);
CREATE INDEX idx_detection_metrics_perf ON mcp_detection_metrics(detection_method, detected_at);
```

---

## Rollout Strategy

### Phase 1: Internal Testing (Week 1)
- Deploy to dev environment
- Test with internal agents
- Collect performance metrics
- Fix bugs and optimize

### Phase 2: Beta (Week 2)
- Enable for 10% of users (feature flag)
- Monitor performance dashboard
- Collect user feedback
- Auto-disable if performance degrades

### Phase 3: Gradual Rollout (Week 3)
- 25% of users
- 50% of users
- 100% of users
- Monitor at each stage

### Phase 4: Default Enable (Week 4)
- Make intelligent detection default
- Keep config-only as fallback
- Update documentation

---

## Success Metrics

### Technical Metrics
- ✅ 95%+ detection accuracy (vs manual verification)
- ✅ <5s p95 detection time
- ✅ <5% CPU usage
- ✅ <100MB memory usage
- ✅ >80% cache hit rate
- ✅ <5% timeout rate

### Business Metrics
- ✅ 80%+ of agents use intelligent detection
- ✅ 50%+ reduction in manual MCP registration
- ✅ 90%+ user satisfaction score
- ✅ Zero production incidents related to performance

---

## Future Enhancements

### Phase 11: Advanced Features (Post-MVP)
- **Machine Learning**: Train model to predict MCP usage patterns
- **Anomaly Detection**: Alert on unexpected MCP connections
- **Supply Chain Verification**: SBOM/SLSA attestation for detected dependencies
- **IDE Integration**: VS Code extension for real-time detection
- **CI/CD Integration**: GitHub Action for pre-deployment detection
- **Multi-Language Support**: Add support for Rust, Ruby, Java, C#

---

## Risk Mitigation

### Risk 1: Performance Impact
**Mitigation**:
- Async background processing (never block agent)
- Resource limits enforced (CPU/memory caps)
- Auto-disable if limits exceeded
- Cache aggressively

### Risk 2: False Positives
**Mitigation**:
- Confidence scoring (show user how certain we are)
- Multi-method verification (2+ methods = higher confidence)
- User feedback mechanism (report false positives)
- Machine learning refinement over time

### Risk 3: Privacy Concerns
**Mitigation**:
- No code sent to external APIs
- All scanning is local
- No telemetry without user consent
- Open source (users can verify)

### Risk 4: Maintenance Burden
**Mitigation**:
- Modular architecture (easy to disable/replace scanners)
- Comprehensive test suite
- Performance monitoring alerts
- Auto-tuning reduces manual intervention

---

## Appendix: Example Detections

### Example 1: JavaScript Agent
**Agent Code** (`src/index.ts`):
```typescript
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

const transport = new StdioClientTransport({
  command: 'npx',
  args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'],
});

const client = new Client({
  name: 'my-agent',
  version: '1.0.0',
}, {
  capabilities: {},
});

await client.connect(transport);
```

**Detection Results**:
```json
{
  "mcpServers": [
    {
      "name": "filesystem",
      "confidenceScore": 98.5,
      "detectedBy": ["scan", "deps"],
      "details": {
        "scan": {
          "filePath": "src/index.ts",
          "lineNumber": 1,
          "detectionType": "import",
          "codeSnippet": "import { Client } from '@modelcontextprotocol/sdk/client/index.js'"
        },
        "deps": {
          "version": "0.1.0",
          "dependencyType": "direct",
          "manifestFile": "package.json"
        }
      }
    }
  ]
}
```

### Example 2: Python Agent
**Agent Code** (`main.py`):
```python
from mcp.client import Client
from mcp.client.stdio import StdioServerParameters, stdio_client

async def main():
    server_params = StdioServerParameters(
        command="uvx",
        args=["mcp-server-sqlite", "--db-path", "/tmp/test.db"],
    )

    async with stdio_client(server_params) as (read, write):
        async with Client(read, write) as client:
            await client.initialize()
```

**Detection Results**:
```json
{
  "mcpServers": [
    {
      "name": "sqlite",
      "confidenceScore": 95.0,
      "detectedBy": ["scan", "deps"],
      "details": {
        "scan": {
          "filePath": "main.py",
          "lineNumber": 1,
          "detectionType": "import",
          "codeSnippet": "from mcp.client import Client"
        },
        "deps": {
          "version": "1.0.0",
          "dependencyType": "direct",
          "manifestFile": "requirements.txt"
        }
      }
    }
  ]
}
```

---

## Conclusion

This implementation plan delivers:
1. ✅ **Intelligent Detection**: 4 methods with 90%+ accuracy
2. ✅ **Ghost Mode Performance**: <5s, <5% CPU, <100MB memory
3. ✅ **Modular Architecture**: Easy to extend with new languages/methods
4. ✅ **Production Ready**: Caching, resource limits, monitoring, auto-tuning
5. ✅ **User Friendly**: Clear UI, confidence scores, detailed results

The key innovation is the **"ghost mode"** design - AIM does its job without anyone noticing it's there.
