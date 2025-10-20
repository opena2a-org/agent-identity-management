# Protocol Detection Strategy

**Date**: October 19, 2025
**Purpose**: Define how AIM detects and identifies different authentication/verification protocols

---

## Supported Protocols

AIM supports 6 verification protocols:

1. **MCP** (Model Context Protocol)
2. **A2A** (Agent-to-Agent)
3. **ACP** (Agent Communication Protocol)
4. **DID** (Decentralized Identity)
5. **OAuth** (OAuth 2.0 / OIDC)
6. **SAML** (Security Assertion Markup Language)

---

## Detection Methods (Current Implementation)

### 1. MCP (Model Context Protocol) Detection

**How to Detect**:
- Check for MCP server registration in user's Claude Desktop config
- Look for MCP-specific headers in requests
- Detect MCP JSON-RPC 2.0 message format

**Implementation**:
```go
// Backend detection
func detectMCP(req *http.Request) bool {
    // Check for MCP-specific headers
    if req.Header.Get("X-MCP-Version") != "" {
        return true
    }

    // Check for JSON-RPC 2.0 format in body
    var body map[string]interface{}
    json.NewDecoder(req.Body).Decode(&body)

    if body["jsonrpc"] == "2.0" && body["method"] != nil {
        return true
    }

    return false
}
```

**Auto-Detection** (Python SDK):
```python
# SDK auto-detects MCP servers from Claude config
from aim_sdk import secure

# Automatically scans ~/.config/claude/config.json for MCP servers
agent = secure("my-agent")
```

**When Used**:
- Agent connects to MCP server
- MCP server requests verification
- MCP capabilities are checked

---

### 2. A2A (Agent-to-Agent) Detection

**How to Detect**:
- Both parties are registered agents in AIM
- Request includes both `agent_id` (initiator) and `target_agent_id`
- Uses Ed25519 mutual authentication

**Implementation**:
```go
func detectA2A(req *VerificationRequest) bool {
    // Both initiator and target are agents
    return req.InitiatorType == "agent" && req.TargetAgentID != nil
}
```

**When Used**:
- Agent-to-agent API calls
- Agent delegation
- Agent collaboration workflows

**Example Flow**:
```
Agent A (initiator) → Verifies with AIM → Calls Agent B (target)
                      ↓
            Protocol: A2A detected
```

---

### 3. ACP (Agent Communication Protocol) Detection

**How to Detect**:
- Standardized agent communication headers
- ACP message format (JSON with specific schema)
- ACP version in request

**Implementation**:
```go
func detectACP(req *http.Request) bool {
    // Check for ACP headers
    if req.Header.Get("X-ACP-Version") != "" {
        return true
    }

    // Check for ACP message schema
    var msg ACPMessage
    if err := json.NewDecoder(req.Body).Decode(&msg); err == nil {
        return msg.Protocol == "ACP" && msg.Version != ""
    }

    return false
}

type ACPMessage struct {
    Protocol string `json:"protocol"` // "ACP"
    Version  string `json:"version"`  // "1.0"
    Sender   string `json:"sender"`   // Agent ID
    Receiver string `json:"receiver"` // Target ID
    Action   string `json:"action"`   // Action type
    Payload  interface{} `json:"payload"`
}
```

**When Used**:
- Standardized agent communication
- Multi-agent systems
- Agent orchestration platforms

---

### 4. DID (Decentralized Identity) Detection

**How to Detect**:
- DID identifier format: `did:method:identifier`
- DID Document verification
- W3C DID standard compliance

**Implementation**:
```go
func detectDID(req *VerificationRequest) bool {
    // Check if identifier is a DID
    if strings.HasPrefix(req.Identifier, "did:") {
        return true
    }

    // Validate DID format (did:method:identifier)
    parts := strings.Split(req.Identifier, ":")
    return len(parts) >= 3 && parts[0] == "did"
}
```

**Supported DID Methods**:
- `did:web` - Web-based DIDs
- `did:key` - Key-based DIDs (Ed25519)
- `did:peer` - Peer-to-peer DIDs
- Custom methods as needed

**When Used**:
- Decentralized agent identity
- Web3 integrations
- Self-sovereign identity

**Example**:
```
Agent DID: did:key:z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH
```

---

### 5. OAuth (OAuth 2.0 / OIDC) Detection

**How to Detect**:
- OAuth 2.0 bearer token in `Authorization` header
- OIDC ID token (JWT)
- OAuth provider metadata

**Implementation**:
```go
func detectOAuth(req *http.Request) bool {
    authHeader := req.Header.Get("Authorization")

    // Check for Bearer token
    if strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimPrefix(authHeader, "Bearer ")

        // Validate JWT structure (OAuth/OIDC tokens are JWTs)
        parts := strings.Split(token, ".")
        if len(parts) == 3 {
            return true
        }
    }

    return false
}
```

**Supported Providers**:
- Google OAuth 2.0
- Microsoft Azure AD / Entra ID
- Okta
- Auth0
- Custom OAuth providers

**When Used**:
- User authentication
- SSO (Single Sign-On)
- Enterprise identity federation

---

### 6. SAML (Security Assertion Markup Language) Detection

**How to Detect**:
- SAML assertion XML format
- SAML response in POST body
- SAML metadata

**Implementation**:
```go
func detectSAML(req *http.Request) bool {
    // Check for SAML assertion in form data
    if req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
        samlResponse := req.FormValue("SAMLResponse")
        if samlResponse != "" {
            // Decode base64 and check for XML
            decoded, err := base64.StdEncoding.DecodeString(samlResponse)
            if err == nil && strings.Contains(string(decoded), "samlp:Response") {
                return true
            }
        }
    }

    return false
}
```

**When Used**:
- Enterprise SSO
- SAML 2.0 federation
- Corporate identity providers (Azure AD, Okta SAML)

---

## Protocol Detection Priority

When multiple protocols could apply, use this priority order:

1. **Explicit Protocol Header** - If client specifies protocol, use that
2. **DID** - Check for `did:` prefix first
3. **SAML** - Check for SAML assertion
4. **OAuth** - Check for Bearer token
5. **MCP** - Check for MCP headers/format
6. **ACP** - Check for ACP message format
7. **A2A** - Check if both parties are agents
8. **Default** - Fall back to basic authentication

---

## Current vs. Future Detection

### ✅ Currently Implemented (Phase 1 - v1.0)
- **MCP**: Auto-detection from Claude config ✅
- **OAuth**: Google, Microsoft, Okta integration ✅
- **A2A**: Agent-to-agent verification ✅
- **Basic protocol field**: Stored in verification events ✅

### 🚧 Planned for Future (Phase 2 - v1.1)
- **ACP**: Full ACP message parsing ⏳
- **DID**: W3C DID resolution ⏳
- **SAML**: Enterprise SAML federation ⏳
- **Auto-detection middleware**: Automatic protocol detection ⏳

---

## Adding Protocol Detection to Verification Flow

### Backend Integration

**Location**: `apps/backend/internal/application/verification_event_service.go`

```go
// DetectProtocol automatically determines the protocol from request context
func (s *VerificationEventService) DetectProtocol(ctx context.Context, req *http.Request) domain.VerificationProtocol {
    // 1. Check explicit protocol header
    if protocol := req.Header.Get("X-Protocol"); protocol != "" {
        return domain.VerificationProtocol(protocol)
    }

    // 2. DID detection
    identifier := ctx.Value("identifier").(string)
    if strings.HasPrefix(identifier, "did:") {
        return domain.VerificationProtocolDID
    }

    // 3. SAML detection
    if detectSAML(req) {
        return domain.VerificationProtocolSAML
    }

    // 4. OAuth detection
    if detectOAuth(req) {
        return domain.VerificationProtocolOAuth
    }

    // 5. MCP detection
    if detectMCP(req) {
        return domain.VerificationProtocolMCP
    }

    // 6. ACP detection
    if detectACP(req) {
        return domain.VerificationProtocolACP
    }

    // 7. A2A detection
    if ctx.Value("initiator_type") == "agent" && ctx.Value("target_agent_id") != nil {
        return domain.VerificationProtocolA2A
    }

    // Default to A2A for basic agent verification
    return domain.VerificationProtocolA2A
}
```

---

## Testing Protocol Detection

Use the test script to verify protocol detection:

```bash
# Run protocol detection tests
python test_protocol_detection.py

# Expected output:
# ✅ MCP protocol detected
# ✅ OAuth protocol detected
# ✅ A2A protocol detected
# ✅ DID protocol detected
# ✅ ACP protocol detected
# ✅ SAML protocol detected
```

---

## Metrics and Analytics

All protocol usage is tracked in the monitoring dashboard:

- **Protocol Distribution**: Shows breakdown of protocols used
- **Protocol Success Rate**: Success rate by protocol
- **Protocol Latency**: Average verification time per protocol
- **Protocol Trends**: Usage patterns over time

**Dashboard Location**: `/dashboard/monitoring`

---

## References

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [W3C DID Core](https://www.w3.org/TR/did-core/)
- [OAuth 2.0 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [SAML 2.0 Specification](https://docs.oasis-open.org/security/saml/v2.0/)

---

**Last Updated**: October 19, 2025
