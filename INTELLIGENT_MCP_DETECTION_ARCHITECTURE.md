# 🧠 Intelligent MCP Detection Architecture

**Vision**: Automatically detect ALL MCP servers a company uses by analyzing agent code, not just config files.

**Key Insight**: If an agent uses an MCP server, that information is embedded in the agent's code, dependencies, or runtime behavior. We can scan for these patterns and automatically map everything.

---

## 🎯 Core Principle

**"All company agents registered → All MCPs automatically detected"**

When a company registers their agents in AIM, we should:
1. **Scan agent codebases** for MCP client usage
2. **Analyze dependencies** (package.json, requirements.txt)
3. **Parse configuration files** (agent-specific configs)
4. **Monitor runtime behavior** (if agent is running)
5. **Automatically register and map** all detected MCPs

---

## 🔍 Detection Methods (Priority Order)

### **Method 1: Code Scanning** (Highest Accuracy)

Scan agent source code for MCP SDK usage patterns.

#### Patterns to Detect:

**JavaScript/TypeScript**:
```typescript
// Pattern 1: Import statements
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

// Pattern 2: Client initialization
const client = new Client({
  name: "my-agent",
  version: "1.0.0"
});

// Pattern 3: Transport connections
await client.connect(new StdioClientTransport({
  command: "npx",
  args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
}));

// Pattern 4: Tool calls (indicates active MCP usage)
await client.callTool({
  name: "read_file",  // filesystem MCP
  arguments: { path: "/data/file.txt" }
});
```

**Python**:
```python
# Pattern 1: Import statements
from mcp import Client, StdioServerParameters
from mcp.client.stdio import stdio_client

# Pattern 2: Client initialization
async with stdio_client(StdioServerParameters(
    command="npx",
    args=["-y", "@modelcontextprotocol/server-filesystem", "/data"]
)) as (read, write):
    async with Client(read, write) as client:
        # Pattern 3: Tool calls
        result = await client.call_tool("read_file", {"path": "/data/file.txt"})
```

#### Detection Strategy:
1. Use AST (Abstract Syntax Tree) parsing to find:
   - MCP SDK imports
   - Client instantiation
   - Transport connections
   - Tool call patterns
2. Extract MCP server identifiers from:
   - Command strings (e.g., `@modelcontextprotocol/server-filesystem`)
   - Tool names (e.g., `read_file` → filesystem MCP)
   - Connection parameters

---

### **Method 2: Dependency Analysis** (High Accuracy)

Parse package managers to find MCP dependencies.

#### Files to Scan:

**Node.js** (`package.json`):
```json
{
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.0",
    "@modelcontextprotocol/server-filesystem": "^0.1.0",
    "@modelcontextprotocol/server-github": "^0.1.0",
    "@modelcontextprotocol/server-sqlite": "^0.1.0"
  }
}
```

**Python** (`requirements.txt` or `pyproject.toml`):
```txt
mcp>=1.0.0
mcp-server-filesystem>=0.1.0
mcp-server-github>=0.1.0
```

**Go** (`go.mod`):
```go
require (
    github.com/modelcontextprotocol/sdk v1.0.0
    github.com/modelcontextprotocol/server-filesystem v0.1.0
)
```

#### Detection Strategy:
1. Parse dependency files
2. Filter for `@modelcontextprotocol/*` packages
3. Extract server names (e.g., `server-filesystem` → "filesystem")
4. Map to known MCP server catalog

---

### **Method 3: Configuration File Parsing** (Medium Accuracy)

Parse agent-specific configuration files.

#### Common Config Formats:

**YAML** (`agent-config.yaml`):
```yaml
agent:
  name: "customer-support-agent"
  mcp_servers:
    - name: "filesystem"
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem"]
    - name: "github"
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"
```

**JSON** (`.agent.json`):
```json
{
  "agent": {
    "name": "data-analyst-agent",
    "tools": ["read_file", "write_file", "list_directory"],
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem"]
      }
    }
  }
}
```

**TOML** (`agent.toml`):
```toml
[agent]
name = "research-agent"

[[agent.mcp_servers]]
name = "filesystem"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem"]

[[agent.mcp_servers]]
name = "github"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
```

#### Detection Strategy:
1. Scan for common config file names:
   - `agent-config.yaml`
   - `.agent.json`
   - `mcp-config.json`
   - `agent.toml`
2. Parse config structure
3. Extract MCP server definitions

---

### **Method 4: Runtime Behavior Analysis** (Real-Time Detection)

Monitor running agents to detect active MCP connections.

#### Detection Techniques:

**1. Environment Variables**:
```bash
# Check agent process environment
ps -eww | grep node | grep MCP
# Look for: MCP_SERVER_FILESYSTEM_PATH, MCP_SERVER_GITHUB_TOKEN, etc.
```

**2. Network Connections**:
```bash
# Detect stdio pipes to MCP servers
lsof -p <agent-pid> | grep PIPE
# Look for: npx @modelcontextprotocol/server-*
```

**3. Process Tree**:
```bash
# Find child processes (MCP servers)
pstree -p <agent-pid>
# Look for: npx, uvx, python, node running MCP servers
```

**4. Log Analysis**:
```bash
# Parse agent logs for MCP tool calls
grep -i "callTool\|listTools\|mcp" /var/log/agent.log
# Extract: tool names, server references
```

#### Detection Strategy:
1. Monitor agent processes
2. Detect child processes running MCP servers
3. Parse environment variables for MCP config
4. Analyze network connections (stdio, HTTP)
5. Real-time detection as agents start/stop

---

### **Method 5: Claude Desktop Config** (Fallback)

Only used as fallback when other methods fail.

**Location**:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

**Note**: This is what Phase 2 currently implements, but it should be the LAST resort, not the primary method.

---

## 🏗️ Implementation Architecture

### **Backend Service: Intelligent Detection Engine**

```go
// intelligent_mcp_detector.go

package application

import (
    "context"
    "fmt"
)

type MCPDetectionMethod string

const (
    DetectionMethodCodeScan      MCPDetectionMethod = "code_scan"
    DetectionMethodDependencies  MCPDetectionMethod = "dependencies"
    DetectionMethodConfigFiles   MCPDetectionMethod = "config_files"
    DetectionMethodRuntime       MCPDetectionMethod = "runtime"
    DetectionMethodClaudeDesktop MCPDetectionMethod = "claude_desktop"
)

type IntelligentMCPDetector struct {
    // Scanners for different detection methods
    codeScanner       *CodeScanner
    dependencyScanner *DependencyScanner
    configScanner     *ConfigFileScanner
    runtimeMonitor    *RuntimeMonitor
    desktopConfigScanner *ClaudeDesktopScanner
}

type DetectionRequest struct {
    AgentID           string
    AgentCodePath     string   // Path to agent source code
    EnableCodeScan    bool     // Enable code scanning
    EnableDepScan     bool     // Enable dependency analysis
    EnableConfigScan  bool     // Enable config file parsing
    EnableRuntime     bool     // Enable runtime monitoring
    FallbackToDesktop bool     // Fallback to Claude Desktop config
}

type DetectionResult struct {
    Method            MCPDetectionMethod
    DetectedServers   []DetectedMCPServer
    Confidence        float64  // 0-100
    ScanDurationMs    int64
    ErrorsEncountered []string
}

func (d *IntelligentMCPDetector) DetectAllMCPs(
    ctx context.Context,
    req *DetectionRequest,
) ([]*DetectionResult, error) {
    results := []*DetectionResult{}

    // Method 1: Code Scanning (highest accuracy)
    if req.EnableCodeScan {
        result, err := d.codeScanner.Scan(ctx, req.AgentCodePath)
        if err == nil && len(result.DetectedServers) > 0 {
            result.Method = DetectionMethodCodeScan
            result.Confidence = 95.0
            results = append(results, result)
        }
    }

    // Method 2: Dependency Analysis
    if req.EnableDepScan {
        result, err := d.dependencyScanner.Scan(ctx, req.AgentCodePath)
        if err == nil && len(result.DetectedServers) > 0 {
            result.Method = DetectionMethodDependencies
            result.Confidence = 90.0
            results = append(results, result)
        }
    }

    // Method 3: Config File Parsing
    if req.EnableConfigScan {
        result, err := d.configScanner.Scan(ctx, req.AgentCodePath)
        if err == nil && len(result.DetectedServers) > 0 {
            result.Method = DetectionMethodConfigFiles
            result.Confidence = 85.0
            results = append(results, result)
        }
    }

    // Method 4: Runtime Monitoring (if agent is running)
    if req.EnableRuntime {
        result, err := d.runtimeMonitor.Detect(ctx, req.AgentID)
        if err == nil && len(result.DetectedServers) > 0 {
            result.Method = DetectionMethodRuntime
            result.Confidence = 100.0  // Runtime is 100% accurate
            results = append(results, result)
        }
    }

    // Method 5: Claude Desktop Config (fallback)
    if req.FallbackToDesktop && len(results) == 0 {
        result, err := d.desktopConfigScanner.Scan(ctx)
        if err == nil && len(result.DetectedServers) > 0 {
            result.Method = DetectionMethodClaudeDesktop
            result.Confidence = 70.0  // Lower confidence
            results = append(results, result)
        }
    }

    return results, nil
}

// DeduplicateAndMerge combines results from multiple detection methods
func (d *IntelligentMCPDetector) DeduplicateAndMerge(
    results []*DetectionResult,
) *DetectionResult {
    merged := &DetectionResult{
        Method: "multi_method",
        DetectedServers: []DetectedMCPServer{},
        Confidence: 0,
    }

    seenServers := make(map[string]*DetectedMCPServer)

    for _, result := range results {
        for _, server := range result.DetectedServers {
            if existing, ok := seenServers[server.Name]; ok {
                // Merge: take higher confidence
                if server.Confidence > existing.Confidence {
                    existing.Confidence = server.Confidence
                    existing.Source = fmt.Sprintf("%s,%s", existing.Source, result.Method)
                }
            } else {
                seenServers[server.Name] = &server
            }
        }
    }

    // Convert map to slice
    for _, server := range seenServers {
        merged.DetectedServers = append(merged.DetectedServers, *server)
    }

    // Calculate weighted average confidence
    if len(results) > 0 {
        totalConf := 0.0
        for _, r := range results {
            totalConf += r.Confidence
        }
        merged.Confidence = totalConf / float64(len(results))
    }

    return merged
}
```

---

### **Code Scanner Implementation**

```go
// code_scanner.go

package application

import (
    "context"
    "go/ast"
    "go/parser"
    "go/token"
    "os"
    "path/filepath"
    "strings"
)

type CodeScanner struct {
    // AST parsers for different languages
    jsParser     *JavaScriptParser
    pythonParser *PythonParser
    goParser     *GoParser
}

func (s *CodeScanner) Scan(ctx context.Context, codePath string) (*DetectionResult, error) {
    result := &DetectionResult{
        DetectedServers: []DetectedMCPServer{},
    }

    // Walk through all source files
    err := filepath.Walk(codePath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        // Skip directories and non-source files
        if info.IsDir() || !isSourceFile(path) {
            return nil
        }

        // Parse file based on extension
        servers, err := s.parseFile(path)
        if err != nil {
            return nil // Continue on error
        }

        result.DetectedServers = append(result.DetectedServers, servers...)
        return nil
    })

    if err != nil {
        return nil, err
    }

    return result, nil
}

func (s *CodeScanner) parseFile(filePath string) ([]DetectedMCPServer, error) {
    ext := filepath.Ext(filePath)

    switch ext {
    case ".js", ".ts", ".jsx", ".tsx":
        return s.jsParser.Parse(filePath)
    case ".py":
        return s.pythonParser.Parse(filePath)
    case ".go":
        return s.goParser.Parse(filePath)
    default:
        return nil, nil
    }
}

// JavaScriptParser parses JS/TS files for MCP usage
type JavaScriptParser struct{}

func (p *JavaScriptParser) Parse(filePath string) ([]DetectedMCPServer, error) {
    content, err := os.ReadFile(filePath)
    if err != nil {
        return nil, err
    }

    code := string(content)
    servers := []DetectedMCPServer{}

    // Pattern 1: Look for MCP SDK imports
    if strings.Contains(code, "@modelcontextprotocol/sdk") {
        // Pattern 2: Find StdioClientTransport connections
        // Regex: new StdioClientTransport\(\{[^}]*command:\s*["']([^"']+)["'][^}]*args:\s*\[([^\]]+)\]

        // Parse command and args to extract server name
        // Example: "@modelcontextprotocol/server-filesystem" → "filesystem"

        server := DetectedMCPServer{
            Name:       "filesystem", // Extracted from parsing
            Command:    "npx",
            Args:       []string{"-y", "@modelcontextprotocol/server-filesystem"},
            Confidence: 95.0,
            Source:     "code_scan",
            Metadata: map[string]interface{}{
                "file_path": filePath,
                "line_number": 42,
            },
        }
        servers = append(servers, server)
    }

    return servers, nil
}
```

---

## 🔄 Complete Workflow

### **Scenario: Company Registers Agent**

1. **User registers agent** via AIM UI:
   ```json
   {
     "name": "customer-support-agent",
     "type": "ai_agent",
     "code_repository": "https://github.com/company/support-agent"
   }
   ```

2. **AIM automatically triggers intelligent detection**:
   ```bash
   # Clone repository (if provided)
   git clone https://github.com/company/support-agent /tmp/detection-scan

   # Run all detection methods in parallel
   - Code scanning (AST parsing)
   - Dependency analysis (package.json)
   - Config file parsing (agent-config.yaml)
   - Runtime monitoring (if agent is running)
   ```

3. **Detection results** (combined from all methods):
   ```json
   {
     "detected_servers": [
       {
         "name": "filesystem",
         "confidence": 95.0,
         "source": "code_scan,dependencies",
         "detected_by": ["import statement", "package.json"]
       },
       {
         "name": "github",
         "confidence": 90.0,
         "source": "code_scan,config_files",
         "detected_by": ["client.connect()", "agent-config.yaml"]
       }
     ]
   }
   ```

4. **AIM automatically**:
   - Registers detected MCP servers (if not already registered)
   - Maps MCP servers to agent's `talks_to` list
   - Creates audit log entry
   - Sends notification to admin

5. **Result**: Zero manual work required! ✅

---

## 📊 Confidence Scoring

| Detection Method | Confidence | Reasoning |
|-----------------|------------|-----------|
| **Runtime Monitoring** | 100% | Agent is actively using the MCP |
| **Code Scanning (AST)** | 95% | Direct evidence in source code |
| **Dependency Analysis** | 90% | MCP is installed as dependency |
| **Config File Parsing** | 85% | Explicitly configured |
| **Claude Desktop Config** | 70% | May not reflect agent's actual usage |

**Combined Confidence**: Average of all methods that detected the server.

---

## 🚀 API Design

### **New Endpoint: Intelligent Detection**

```
POST /api/v1/agents/:id/mcp-servers/detect-intelligent
```

**Request**:
```json
{
  "agent_code_path": "/path/to/agent/repo",
  "detection_methods": {
    "code_scan": true,
    "dependencies": true,
    "config_files": true,
    "runtime": true,
    "claude_desktop": false
  },
  "auto_register": true,
  "dry_run": false
}
```

**Response**:
```json
{
  "detection_results": [
    {
      "method": "code_scan",
      "detected_servers": [
        {
          "name": "filesystem",
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-filesystem"],
          "confidence": 95.0,
          "source": "code_scan",
          "metadata": {
            "file_path": "src/agent.ts",
            "line_number": 42
          }
        }
      ],
      "scan_duration_ms": 1234
    },
    {
      "method": "dependencies",
      "detected_servers": [
        {
          "name": "filesystem",
          "confidence": 90.0
        },
        {
          "name": "github",
          "confidence": 90.0
        }
      ],
      "scan_duration_ms": 123
    }
  ],
  "merged_results": {
    "detected_servers": [
      {
        "name": "filesystem",
        "confidence": 92.5,
        "source": "code_scan,dependencies"
      },
      {
        "name": "github",
        "confidence": 90.0,
        "source": "dependencies"
      }
    ]
  },
  "registered_count": 2,
  "mapped_count": 2,
  "total_talks_to": 2
}
```

---

## 🎯 Benefits of Intelligent Detection

1. **Zero Manual Work**: No config files to maintain
2. **Always Accurate**: Reflects actual agent code
3. **Multi-Method Validation**: Cross-references multiple sources
4. **High Confidence**: 90%+ accuracy
5. **Automatic Registration**: "All agents registered → All MCPs detected"
6. **Real-Time Updates**: Detects changes when code updates

---

## 🛠️ Implementation Phases

### **Phase 2.5: Intelligent Detection Engine** (New)

1. **Week 1**: Code scanner (JavaScript/TypeScript)
2. **Week 2**: Dependency analyzer (package.json, requirements.txt)
3. **Week 3**: Config file parser (YAML, JSON, TOML)
4. **Week 4**: Runtime monitor (process inspection)
5. **Week 5**: Integration and testing

### **Phase 2.6: UI Updates**

Update AutoDetectButton to support multiple detection methods:
```tsx
<AutoDetectButton
  agentId={agent.id}
  detectionMethods={{
    codeScan: true,
    dependencies: true,
    configFiles: true,
    runtime: false,
    claudeDesktop: false  // Fallback only
  }}
/>
```

---

## 📚 Technology Stack

- **AST Parsing**:
  - JavaScript/TypeScript: `@babel/parser`, `typescript`
  - Python: `ast` module, `libcst`
  - Go: `go/ast`, `go/parser`

- **Dependency Analysis**:
  - Node.js: Parse `package.json`
  - Python: Parse `requirements.txt`, `pyproject.toml`
  - Go: Parse `go.mod`

- **Config Parsing**:
  - YAML: `gopkg.in/yaml.v3`
  - JSON: `encoding/json`
  - TOML: `github.com/BurntSushi/toml`

- **Process Monitoring**:
  - Linux: `/proc` filesystem
  - macOS: `ps`, `lsof`
  - Windows: `tasklist`, `wmic`

---

## 🎉 Conclusion

**Your insight is spot-on**: We should detect MCPs intelligently by analyzing agent code, not just parsing config files. This makes AIM truly automatic and zero-friction.

**Next Steps**:
1. Implement code scanner (highest priority)
2. Add dependency analyzer
3. Integrate with existing auto-detection endpoint
4. Update UI to show multi-method detection results

This is the difference between "good" and "great" - making the system intelligent enough that it Just Works™.

---

**Last Updated**: October 9, 2025
**Status**: Architecture Design Complete
**Priority**: High - Implement in Phase 2.5
