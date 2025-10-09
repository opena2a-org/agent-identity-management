# Prompt for New Claude Code Session

Copy and paste this entire prompt into a new Claude Code session to implement intelligent MCP detection:

---

## Context

You are working on **AIM (Agent Identity Management)**, an open-source enterprise platform for managing AI agent and MCP (Model Context Protocol) server identities. The codebase is located at:

```
/Users/decimai/workspace/agent-identity-management/
```

### Tech Stack
- **Backend**: Go (Fiber v3 framework)
- **Frontend**: Next.js 15 (App Router), TypeScript, Shadcn/ui
- **Database**: PostgreSQL 16
- **Cache**: Redis 7

### Current State
The platform currently has:
- ✅ Agent registration & management
- ✅ MCP server registration & management
- ✅ Agent-MCP relationship tracking (`talks_to` field)
- ✅ Basic auto-detection (Claude Desktop config parsing only)
- ✅ Trust scoring, audit logging, authentication

### What We're Building
We need to build an **intelligent MCP detection system** that automatically discovers which MCP servers an agent uses by:
1. **Scanning agent code** (JavaScript/TypeScript/Python/Go)
2. **Analyzing dependencies** (package.json, requirements.txt, go.mod)
3. **Parsing config files** (agent-specific configs, not just Claude Desktop)
4. **Monitoring runtime** (optional, opt-in for high-security environments)

### Critical Requirements

**Performance is CRITICAL**:
- Must operate in "ghost mode" - zero noticeable impact on agents
- Target: <5s detection time, <5% CPU, <100MB memory
- Never block agent startup or execution
- All detection must run asynchronously in background
- Aggressive caching with intelligent invalidation
- Resource limits strictly enforced

**Architecture**:
- 3 new modules: **SCAN** (code scanner), **DEPS** (dependency analyzer), **RTMN** (runtime monitor)
- Each module is independent and can be enabled/disabled
- Results are merged and deduplicated by orchestrator
- Confidence scoring: 95% (scan), 90% (deps), 85% (config), 100% (runtime)

---

## Task: Implement Intelligent MCP Detection

### Step 1: Read the Implementation Plan
First, read the complete implementation plan:

```
/Users/decimai/workspace/agent-identity-management/INTELLIGENT_MCP_DETECTION_IMPLEMENTATION_PLAN.md
```

This document contains:
- Detailed architecture
- Database schema changes
- Module-by-module implementation details
- API endpoint specifications
- UI component updates
- Performance targets and monitoring
- Testing strategy

### Step 2: Understand Existing Code Structure

The backend follows clean architecture:
```
apps/backend/
├── cmd/server/main.go              # Entry point, route registration
├── internal/
│   ├── domain/                     # Business entities (Agent, MCPServer)
│   ├── application/                # Use cases and services
│   ├── infrastructure/             # External dependencies (DB, cache)
│   └── interfaces/http/handlers/   # HTTP request handlers
├── migrations/                     # Database migrations
└── tests/                          # Integration tests
```

Key existing files to reference:
- `internal/domain/agent.go` - Agent entity
- `internal/domain/mcp_server.go` - MCP server entity
- `internal/application/agent_service.go` - Agent business logic
- `internal/interfaces/http/handlers/agent_handler.go` - Agent HTTP endpoints

### Step 3: Implementation Phases

Implement in this order:

#### Phase 1: Foundation (Start Here)
1. **Create database migrations** for detection cache and metrics tables
2. **Implement cache layer** (`internal/modules/cache/detection_cache.go`)
3. **Enhance config parser** (`internal/application/mcp_detection/config_parser.go`)
4. Test Phase 1 before moving on

#### Phase 2: SCAN Module
1. **Create scanner interface** (`internal/modules/scan/scanner.go`)
2. **Implement JavaScript/TypeScript scanner** using `github.com/evanw/esbuild`
3. **Implement Python scanner** (shell out to Python AST)
4. **Implement Go scanner** (use native `go/parser`)
5. **Add caching to scanners**
6. Test each scanner independently

#### Phase 3: DEPS Module
1. **Create analyzer interface** (`internal/modules/deps/analyzer.go`)
2. **Implement NPM analyzer** (parse package.json, package-lock.json)
3. **Implement Pip analyzer** (parse requirements.txt, Pipfile)
4. **Implement Go module analyzer** (parse go.mod, go.sum)
5. **Add caching to analyzers**
6. Test each analyzer independently

#### Phase 4: RTMN Module
1. **Create monitor interface** (`internal/modules/rtmn/monitor.go`)
2. **Implement process watcher** using `github.com/shirou/gopsutil`
3. **Implement network watcher**
4. **Add resource limiter** (CPU/memory caps)
5. **Implement statistical sampling**
6. Test monitoring with sample agents

#### Phase 5: Orchestration
1. **Create orchestrator** (`internal/application/mcp_detection/orchestrator.go`)
2. **Implement concurrent method execution** with timeouts
3. **Create result merger** with deduplication logic
4. **Add confidence score calculation**
5. Test end-to-end detection flow

#### Phase 6: API Integration
1. **Create new endpoint**: `POST /api/v1/agents/:id/mcp-servers/detect-intelligent`
2. **Update existing endpoint**: `POST /api/v1/agents/:id/mcp-servers/detect` (add intelligent mode)
3. **Add handler logic** in `internal/interfaces/http/handlers/agent_handler.go`
4. Test API endpoints with Postman/curl

#### Phase 7: UI Updates
1. **Create detection method badges** component
2. **Update AutoDetectButton** modal to show multiple methods
3. **Enhance MCPServerList** to display confidence scores and detection methods
4. **Add performance metrics display**
5. Test UI in browser with chrome-devtools MCP

#### Phase 8: Performance Monitoring
1. **Implement metrics collection**
2. **Create performance dashboard** (admin only)
3. **Add auto-tuning logic**
4. Load test with 100+ concurrent detections

#### Phase 9: Testing
1. **Write unit tests** for each module
2. **Write integration tests** for orchestrator
3. **Create test fixtures** (sample agents in JS, Python, Go)
4. **Run performance benchmarks**
5. Ensure 100% test coverage for critical paths

#### Phase 10: Documentation
1. **User documentation** (how to use intelligent detection)
2. **Developer documentation** (architecture, adding new scanners)
3. **Update README.md**
4. **Add configuration guide**

---

## Important Guidelines

### Naming Conventions (CRITICAL)
- **Backend**: Go uses PascalCase for structs, camelCase for JSON tags
- **Frontend**: TypeScript uses camelCase for everything
- **Database**: PostgreSQL uses snake_case
- **MUST MATCH EXACTLY** across all layers (see CLAUDE.md for details)

### Code Quality
- Every feature must have tests (unit + integration)
- Use existing patterns from the codebase
- Follow clean architecture principles
- Add error handling and logging
- Use TODO comments for future improvements

### Performance
- Profile each module after implementation
- Measure execution time, CPU, memory
- Cache aggressively, invalidate intelligently
- Never block agent startup or execution
- Add timeouts to prevent hanging

### Security
- No code sent to external APIs (all local)
- Validate all file paths (prevent directory traversal)
- Limit file sizes (max 5MB per file)
- Resource limits strictly enforced
- Sanitize all user inputs

---

## Key Files to Create

### Backend (Go)
```
apps/backend/
├── internal/
│   ├── modules/
│   │   ├── cache/
│   │   │   └── detection_cache.go
│   │   ├── scan/
│   │   │   ├── scanner.go
│   │   │   ├── javascript_scanner.go
│   │   │   ├── python_scanner.go
│   │   │   └── go_scanner.go
│   │   ├── deps/
│   │   │   ├── analyzer.go
│   │   │   ├── npm_analyzer.go
│   │   │   ├── pip_analyzer.go
│   │   │   └── go_mod_analyzer.go
│   │   └── rtmn/
│   │       ├── monitor.go
│   │       ├── process_watcher.go
│   │       ├── network_watcher.go
│   │       └── resource_limiter.go
│   └── application/
│       └── mcp_detection/
│           ├── orchestrator.go
│           ├── config_parser.go
│           └── result_merger.go
└── migrations/
    ├── 029_create_detection_cache_tables.up.sql
    └── 029_create_detection_cache_tables.down.sql
```

### Frontend (TypeScript/React)
```
apps/web/
├── components/
│   └── agents/
│       ├── detection-method-badge.tsx (new)
│       ├── auto-detect-button.tsx (update)
│       └── mcp-server-list.tsx (update)
└── app/dashboard/admin/
    └── performance/
        └── page.tsx (new)
```

---

## Testing Strategy

### Unit Tests
```bash
# Test each module independently
go test ./internal/modules/scan/...
go test ./internal/modules/deps/...
go test ./internal/modules/rtmn/...
```

### Integration Tests
```bash
# Test end-to-end flow
go test ./internal/application/mcp_detection/...
```

### Performance Tests
```bash
# Benchmark detection speed
go test -bench=. ./internal/modules/...

# Load test
k6 run tests/load/detection_load_test.js
```

### Manual Testing
1. Create sample agents in `test/fixtures/`:
   - JavaScript agent with MCP imports
   - Python agent with mcp package
   - Go agent with MCP SDK
2. Run detection on each sample agent
3. Verify correct MCPs detected with confidence scores
4. Check performance metrics (should be <5s)

---

## Success Criteria

Before marking this task complete, verify:

- [ ] All 4 detection methods implemented (config, scan, deps, rtmn)
- [ ] Cache layer working (Redis + PostgreSQL fallback)
- [ ] API endpoints functional (test with curl/Postman)
- [ ] UI shows detection methods, confidence scores, performance metrics
- [ ] Performance targets met (<5s, <5% CPU, <100MB memory)
- [ ] Tests passing (unit + integration)
- [ ] Documentation updated (README, user guide, developer guide)
- [ ] No breaking changes to existing features
- [ ] Chrome DevTools MCP testing shows no console errors

---

## Example: How to Test End-to-End

### 1. Create Test Agent
```bash
mkdir -p test/fixtures/js-agent
cd test/fixtures/js-agent
npm init -y
npm install @modelcontextprotocol/sdk
```

Create `test/fixtures/js-agent/src/index.ts`:
```typescript
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

const transport = new StdioClientTransport({
  command: 'npx',
  args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'],
});

const client = new Client({ name: 'test-agent', version: '1.0.0' }, { capabilities: {} });
await client.connect(transport);
```

### 2. Register Test Agent
```bash
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test JS Agent",
    "type": "ai_agent",
    "description": "Test agent for MCP detection"
  }'
```

### 3. Run Intelligent Detection
```bash
curl -X POST http://localhost:8080/api/v1/agents/{agent_id}/mcp-servers/detect-intelligent \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "methods": ["scan", "deps", "config"],
    "agentPath": "/path/to/test/fixtures/js-agent",
    "autoRegister": false,
    "timeout": 30000
  }'
```

### 4. Verify Response
Should return:
```json
{
  "mcpServers": [
    {
      "name": "filesystem",
      "confidenceScore": 95.5,
      "detectedBy": ["scan", "deps"],
      "details": { ... }
    }
  ],
  "executionTimeMs": 2341,
  "methodsUsed": [
    { "method": "scan", "success": true, "executionTimeMs": 1823, "serversFound": 1 },
    { "method": "deps", "success": true, "executionTimeMs": 156, "serversFound": 1 }
  ]
}
```

---

## Common Pitfalls to Avoid

1. **Don't skip caching** - It's critical for performance
2. **Don't block goroutines** - Use context.WithTimeout everywhere
3. **Don't scan node_modules/** - Exclude it in .gitignore-style patterns
4. **Don't forget error handling** - Return errors, don't panic
5. **Don't hardcode paths** - Use environment variables
6. **Test with Chrome DevTools MCP** - Catch frontend bugs early
7. **Profile before optimizing** - Measure first, optimize later

---

## Resources

### Implementation Plan (Read First)
```
/Users/decimai/workspace/agent-identity-management/INTELLIGENT_MCP_DETECTION_IMPLEMENTATION_PLAN.md
```

### Project Documentation
```
/Users/decimai/workspace/agent-identity-management/CLAUDE.md
/Users/decimai/workspace/agent-identity-management/CLAUDE_CONTEXT.md
/Users/decimai/workspace/agent-identity-management/README.md
```

### Go Libraries to Use
- **esbuild**: `github.com/evanw/esbuild` (JS/TS AST parsing)
- **gopsutil**: `github.com/shirou/gopsutil` (process monitoring)
- **fiber**: `github.com/gofiber/fiber/v3` (HTTP framework)
- **pgx**: `github.com/jackc/pgx/v5` (PostgreSQL driver)
- **redis**: `github.com/redis/go-redis/v9` (Redis client)

### Questions?
- Check existing code patterns in the codebase
- Read CLAUDE.md for naming conventions
- Profile if performance is slow
- Ask for clarification if requirements unclear

---

## Ready to Start?

Begin with **Phase 1: Foundation** (database migrations + cache layer). Once that's working, move to Phase 2 (SCAN module).

Take your time, test thoroughly, and follow the "ghost mode" performance philosophy - AIM should be invisible to users.

Good luck! 🚀
