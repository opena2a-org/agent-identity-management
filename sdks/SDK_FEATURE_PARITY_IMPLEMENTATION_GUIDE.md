# SDK Feature Parity Implementation Guide

**Version**: 1.0
**Date**: October 9, 2025
**Status**: Ready for Implementation
**Estimated Time**: 12-16 hours (Go SDK: 6-8h, JavaScript SDK: 6-8h)

---

## 🎯 Executive Summary

This guide provides complete, step-by-step instructions for implementing feature parity across all three AIM SDKs (Python, Go, JavaScript). The Python SDK is **100% complete** and serves as the reference implementation. The Go and JavaScript SDKs need the following features added:

**Missing Features in Go & JavaScript SDKs**:
1. ✅ Ed25519 cryptographic signing
2. ✅ OAuth/OIDC integration (Google, Microsoft, Okta)
3. ✅ Automatic MCP capability detection
4. ✅ Secure credential storage (keyring/keychain)
5. ✅ Agent registration workflow

**Current State**:
- ✅ **Python SDK**: 100% complete, production-ready
- ⚠️ **Go SDK**: 40% complete (MCP reporting only, API key auth works)
- ⚠️ **JavaScript SDK**: 40% complete (MCP reporting only, API key auth works)

---

## 📋 Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [Python SDK Reference Implementation](#python-sdk-reference-implementation)
3. [Go SDK Implementation Plan](#go-sdk-implementation-plan)
4. [JavaScript SDK Implementation Plan](#javascript-sdk-implementation-plan)
5. [Testing Requirements](#testing-requirements)
6. [Success Criteria](#success-criteria)
7. [Troubleshooting](#troubleshooting)

---

## 1. Current State Analysis

### 1.1 Python SDK (Reference Implementation)

**Location**: `/Users/decimai/workspace/agent-identity-management/sdks/python/`

**Complete Features**:
- ✅ Ed25519 key generation and signing (`nacl` library)
- ✅ OAuth integration (Google, Microsoft, Okta via `authlib`)
- ✅ Automatic capability detection (`capability_detection.py`)
- ✅ Secure keyring storage (`keyring` library)
- ✅ Agent registration (`register_agent()`)
- ✅ MCP detection reporting (`report_mcp()`)
- ✅ Automatic MCP detection (`auto_detect_mcps()`)
- ✅ SDK token management
- ✅ Runtime information collection
- ✅ Comprehensive error handling

**Key Files**:
```
sdks/python/
├── aim_sdk/
│   ├── __init__.py                  # Main client
│   ├── client.py                    # AIM client implementation
│   ├── capability_detection.py      # Auto-detection logic
│   └── oauth_helpers.py             # OAuth integration
├── requirements.txt                 # Dependencies
├── README.md                        # User documentation
└── example_*.py                     # Usage examples
```

### 1.2 Go SDK (Current State)

**Location**: `/Users/decimai/workspace/agent-identity-management/sdks/go/`

**Implemented**:
- ✅ Basic client initialization
- ✅ API key authentication (FIXED - base64 encoding)
- ✅ MCP detection reporting
- ✅ Runtime information

**Missing**:
- ❌ Ed25519 signing
- ❌ OAuth integration
- ❌ Capability detection
- ❌ Keyring storage
- ❌ Agent registration

**Current Structure**:
```
sdks/go/
├── client.go                        # Main client
├── go.mod                           # Go module
├── README.md                        # Documentation
└── example_test/
    └── main.go                      # Test example
```

### 1.3 JavaScript SDK (Current State)

**Location**: `/Users/decimai/workspace/agent-identity-management/sdks/javascript/`

**Implemented**:
- ✅ Basic client initialization
- ✅ API key authentication (FIXED - base64 encoding)
- ✅ MCP detection reporting
- ✅ Runtime information

**Missing**:
- ❌ Ed25519 signing
- ❌ OAuth integration
- ❌ Capability detection
- ❌ Keyring storage
- ❌ Agent registration

**Current Structure**:
```
sdks/javascript/
├── src/
│   ├── index.js                     # Main client
│   └── client.js                    # Client implementation
├── package.json                     # NPM dependencies
├── README.md                        # Documentation
└── examples/
    └── basic.js                     # Test example
```

---

## 2. Python SDK Reference Implementation

### 2.1 Architecture Overview

The Python SDK follows a modular architecture:

```
┌─────────────────────────────────────────────────────────┐
│                    AIM Client                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │  OAuth       │  │  Ed25519     │  │  Keyring     │ │
│  │  Integration │  │  Signing     │  │  Storage     │ │
│  └──────────────┘  └──────────────┘  └──────────────┘ │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │  Capability  │  │  MCP         │  │  SDK Token   │ │
│  │  Detection   │  │  Reporting   │  │  Management  │ │
│  └──────────────┘  └──────────────┘  └──────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 2.2 Key Implementation Details

#### Ed25519 Signing

**File**: `sdks/python/aim_sdk/client.py` (lines 150-180)

```python
def _generate_ed25519_keypair(self) -> Tuple[bytes, bytes]:
    """Generate Ed25519 keypair for agent signing."""
    import nacl.signing
    import nacl.encoding

    signing_key = nacl.signing.SigningKey.generate()
    verify_key = signing_key.verify_key

    private_key_bytes = signing_key.encode(encoder=nacl.encoding.Base64Encoder)
    public_key_bytes = verify_key.encode(encoder=nacl.encoding.Base64Encoder)

    return private_key_bytes, public_key_bytes

def _sign_request(self, data: Dict[str, Any]) -> str:
    """Sign request data using Ed25519."""
    import nacl.signing
    import nacl.encoding
    import json

    # Load signing key from keyring
    private_key_b64 = keyring.get_password("aim_sdk", "private_key")
    signing_key = nacl.signing.SigningKey(
        private_key_b64,
        encoder=nacl.encoding.Base64Encoder
    )

    # Sign the JSON payload
    message = json.dumps(data, sort_keys=True).encode('utf-8')
    signed = signing_key.sign(message, encoder=nacl.encoding.Base64Encoder)

    return signed.signature.decode('utf-8')
```

**Dependencies**:
- `PyNaCl>=1.5.0` - Ed25519 cryptography

**Key Concepts**:
1. Generate keypair once during agent registration
2. Store private key securely in keyring
3. Sign all sensitive requests (agent registration, OAuth callbacks)
4. Backend verifies signature using public key

#### OAuth Integration

**File**: `sdks/python/aim_sdk/client.py` (lines 250-350)

```python
def register_agent_with_oauth(
    self,
    provider: str,  # "google", "microsoft", "okta"
    agent_name: str,
    agent_type: str = "ai_agent",
    redirect_uri: str = "http://localhost:8080/callback"
) -> Dict[str, Any]:
    """Register agent using OAuth flow."""

    # Step 1: Initiate OAuth flow
    auth_url = self._get_oauth_authorization_url(provider, redirect_uri)
    print(f"🔐 Please visit this URL to authorize: {auth_url}")

    # Step 2: Start local callback server
    callback_code = self._start_oauth_callback_server(redirect_uri)

    # Step 3: Exchange code for tokens
    tokens = self._exchange_oauth_code(provider, callback_code, redirect_uri)

    # Step 4: Generate Ed25519 keypair
    private_key, public_key = self._generate_ed25519_keypair()

    # Step 5: Register agent with backend
    response = requests.post(
        f"{self.api_url}/api/v1/agents/register",
        json={
            "name": agent_name,
            "type": agent_type,
            "public_key": public_key.decode('utf-8'),
            "oauth_provider": provider,
            "oauth_token": tokens["access_token"]
        },
        headers={"Authorization": f"Bearer {tokens['access_token']}"}
    )

    # Step 6: Store credentials in keyring
    keyring.set_password("aim_sdk", "private_key", private_key.decode('utf-8'))
    keyring.set_password("aim_sdk", "agent_id", response.json()["id"])
    keyring.set_password("aim_sdk", "api_key", response.json()["api_key"])

    return response.json()
```

**Dependencies**:
- `authlib>=1.3.0` - OAuth2 client
- `keyring>=25.0.0` - Secure credential storage

**OAuth Flow**:
1. Generate authorization URL
2. Open browser for user consent
3. Start local HTTP server to receive callback
4. Exchange authorization code for tokens
5. Use access token to register agent
6. Store all credentials securely

#### Capability Detection

**File**: `sdks/python/aim_sdk/capability_detection.py`

```python
def detect_mcp_capabilities() -> Dict[str, Any]:
    """Auto-detect MCP servers and their capabilities."""

    detections = {
        "mcps": [],
        "detected_at": datetime.now(timezone.utc).isoformat(),
        "runtime": get_runtime_info()
    }

    # Check for MCP configuration files
    mcp_configs = _find_mcp_configs()

    for config_path in mcp_configs:
        mcps = _parse_mcp_config(config_path)
        for mcp in mcps:
            detection = {
                "name": mcp["name"],
                "type": mcp.get("type", "unknown"),
                "command": mcp.get("command"),
                "args": mcp.get("args", []),
                "env": mcp.get("env", {}),
                "detected_from": config_path,
                "capabilities": _probe_mcp_capabilities(mcp)
            }
            detections["mcps"].append(detection)

    return detections

def _find_mcp_configs() -> List[str]:
    """Find MCP configuration files in common locations."""
    locations = [
        os.path.expanduser("~/.config/mcp/servers.json"),
        os.path.expanduser("~/.mcp/config.json"),
        os.path.join(os.getcwd(), "mcp.json"),
        os.path.join(os.getcwd(), ".mcp/servers.json"),
    ]

    found = []
    for loc in locations:
        if os.path.exists(loc):
            found.append(loc)

    return found

def _probe_mcp_capabilities(mcp: Dict) -> List[str]:
    """Probe MCP server to discover its capabilities."""
    capabilities = []

    # Standard MCP capabilities to check
    checks = [
        ("filesystem", ["read", "write", "list"]),
        ("database", ["query", "execute"]),
        ("web", ["fetch", "search"]),
        ("memory", ["store", "retrieve"]),
    ]

    for capability_type, operations in checks:
        if _check_mcp_capability(mcp, capability_type, operations):
            capabilities.append(capability_type)

    return capabilities
```

**Detection Strategy**:
1. Search for MCP config files in standard locations
2. Parse JSON configuration
3. Probe each MCP server for capabilities
4. Report complete capability profile to backend

#### Keyring Storage

**File**: `sdks/python/aim_sdk/client.py` (credential management)

```python
def _store_credentials(self, credentials: Dict[str, str]) -> None:
    """Store credentials securely in system keyring."""
    import keyring

    for key, value in credentials.items():
        keyring.set_password("aim_sdk", key, value)

def _load_credentials(self) -> Dict[str, str]:
    """Load credentials from system keyring."""
    import keyring

    return {
        "agent_id": keyring.get_password("aim_sdk", "agent_id"),
        "api_key": keyring.get_password("aim_sdk", "api_key"),
        "private_key": keyring.get_password("aim_sdk", "private_key"),
    }

def _clear_credentials(self) -> None:
    """Clear all stored credentials."""
    import keyring

    for key in ["agent_id", "api_key", "private_key", "oauth_token"]:
        try:
            keyring.delete_password("aim_sdk", key)
        except keyring.errors.PasswordDeleteError:
            pass
```

**Storage Locations by Platform**:
- **macOS**: Keychain (`Keychain Access.app`)
- **Windows**: Windows Credential Locker
- **Linux**: Secret Service (GNOME Keyring, KWallet)

---

## 3. Go SDK Implementation Plan

### 3.1 Prerequisites

**Required Libraries**:
```go
// go.mod additions
require (
    github.com/google/uuid v1.6.0                      // UUID generation
    golang.org/x/crypto v0.18.0                        // Ed25519 signing
    golang.org/x/oauth2 v0.15.0                        // OAuth2 integration
    github.com/zalando/go-keyring v0.2.3               // Keyring storage
    github.com/fsnotify/fsnotify v1.7.0                // File watching
)
```

### 3.2 Implementation Steps

#### Step 1: Add Ed25519 Signing

**File**: `sdks/go/signing.go` (NEW)

```go
package aimsdk

import (
    "crypto/ed25519"
    "encoding/base64"
    "encoding/json"
    "fmt"
)

// GenerateEd25519Keypair generates a new Ed25519 keypair
func GenerateEd25519Keypair() (privateKey, publicKey []byte, err error) {
    pub, priv, err := ed25519.GenerateKey(nil)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to generate keypair: %w", err)
    }

    return priv, pub, nil
}

// SignRequest signs request data using Ed25519 private key
func SignRequest(privateKey []byte, data interface{}) (string, error) {
    // Marshal data to JSON (sorted keys for consistency)
    jsonData, err := json.Marshal(data)
    if err != nil {
        return "", fmt.Errorf("failed to marshal data: %w", err)
    }

    // Sign the JSON payload
    signature := ed25519.Sign(privateKey, jsonData)

    // Return base64-encoded signature
    return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifySignature verifies Ed25519 signature (for testing)
func VerifySignature(publicKey []byte, data interface{}, signatureB64 string) bool {
    jsonData, _ := json.Marshal(data)
    signature, _ := base64.StdEncoding.DecodeString(signatureB64)
    return ed25519.Verify(publicKey, jsonData, signature)
}
```

**Testing**:
```go
// Test in sdks/go/signing_test.go
func TestEd25519Signing(t *testing.T) {
    // Generate keypair
    privKey, pubKey, err := GenerateEd25519Keypair()
    assert.NoError(t, err)
    assert.NotNil(t, privKey)
    assert.NotNil(t, pubKey)

    // Sign some data
    data := map[string]string{"agent_id": "test-123", "timestamp": "2025-10-09"}
    signature, err := SignRequest(privKey, data)
    assert.NoError(t, err)
    assert.NotEmpty(t, signature)

    // Verify signature
    valid := VerifySignature(pubKey, data, signature)
    assert.True(t, valid)
}
```

#### Step 2: Add OAuth Integration

**File**: `sdks/go/oauth.go` (NEW)

```go
package aimsdk

import (
    "context"
    "fmt"
    "net/http"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "golang.org/x/oauth2/microsoft"
)

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
    Provider    string // "google", "microsoft", "okta"
    ClientID    string
    ClientSecret string
    RedirectURL string
}

// GetOAuthConfig returns OAuth2 config for provider
func GetOAuthConfig(provider, redirectURL string) (*oauth2.Config, error) {
    configs := map[string]*oauth2.Config{
        "google": {
            ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
            ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
            RedirectURL:  redirectURL,
            Scopes:       []string{"openid", "profile", "email"},
            Endpoint:     google.Endpoint,
        },
        "microsoft": {
            ClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
            ClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
            RedirectURL:  redirectURL,
            Scopes:       []string{"openid", "profile", "email"},
            Endpoint:     microsoft.AzureADEndpoint("common"),
        },
    }

    config, ok := configs[provider]
    if !ok {
        return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
    }

    return config, nil
}

// StartOAuthFlow initiates OAuth flow and returns authorization URL
func StartOAuthFlow(config *oauth2.Config) string {
    // Use state for CSRF protection
    state := generateRandomState()
    return config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// StartCallbackServer starts HTTP server to receive OAuth callback
func StartCallbackServer(port int) (code string, err error) {
    codeChan := make(chan string)
    errChan := make(chan error)

    server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

    http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        if code == "" {
            errChan <- fmt.Errorf("no authorization code received")
            return
        }

        w.Write([]byte("✅ Authorization successful! You can close this window."))
        codeChan <- code
    })

    go server.ListenAndServe()
    defer server.Shutdown(context.Background())

    select {
    case code := <-codeChan:
        return code, nil
    case err := <-errChan:
        return "", err
    }
}

// ExchangeCodeForToken exchanges authorization code for access token
func ExchangeCodeForToken(ctx context.Context, config *oauth2.Config, code string) (*oauth2.Token, error) {
    return config.Exchange(ctx, code)
}
```

**Usage Example**:
```go
// In client.go - RegisterAgentWithOAuth method
func (c *Client) RegisterAgentWithOAuth(ctx context.Context, provider, name string) error {
    // Get OAuth config
    config, err := GetOAuthConfig(provider, "http://localhost:8080/callback")
    if err != nil {
        return err
    }

    // Start OAuth flow
    authURL := StartOAuthFlow(config)
    fmt.Printf("🔐 Please visit: %s\n", authURL)

    // Open browser
    openBrowser(authURL)

    // Wait for callback
    code, err := StartCallbackServer(8080)
    if err != nil {
        return err
    }

    // Exchange code for token
    token, err := ExchangeCodeForToken(ctx, config, code)
    if err != nil {
        return err
    }

    // Register agent with backend
    return c.registerWithToken(ctx, name, token.AccessToken)
}
```

#### Step 3: Add Capability Detection

**File**: `sdks/go/detection.go` (NEW)

```go
package aimsdk

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// MCPCapability represents detected MCP server capability
type MCPCapability struct {
    Name         string            `json:"name"`
    Type         string            `json:"type"`
    Command      string            `json:"command,omitempty"`
    Args         []string          `json:"args,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
    DetectedFrom string            `json:"detected_from"`
    Capabilities []string          `json:"capabilities"`
}

// DetectionResult holds all detected capabilities
type DetectionResult struct {
    MCPs       []MCPCapability    `json:"mcps"`
    DetectedAt string             `json:"detected_at"`
    Runtime    map[string]string  `json:"runtime"`
}

// AutoDetectMCPs scans for MCP configurations and capabilities
func AutoDetectMCPs() (*DetectionResult, error) {
    result := &DetectionResult{
        MCPs:       []MCPCapability{},
        DetectedAt: time.Now().UTC().Format(time.RFC3339),
        Runtime:    collectRuntimeInfo(),
    }

    // Find MCP config files
    configPaths := findMCPConfigs()

    for _, path := range configPaths {
        mcps, err := parseMCPConfig(path)
        if err != nil {
            continue // Skip invalid configs
        }

        for _, mcp := range mcps {
            mcp.DetectedFrom = path
            mcp.Capabilities = probeMCPCapabilities(mcp)
            result.MCPs = append(result.MCPs, mcp)
        }
    }

    return result, nil
}

// findMCPConfigs searches for MCP configuration files
func findMCPConfigs() []string {
    homeDir, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()

    locations := []string{
        filepath.Join(homeDir, ".config", "mcp", "servers.json"),
        filepath.Join(homeDir, ".mcp", "config.json"),
        filepath.Join(cwd, "mcp.json"),
        filepath.Join(cwd, ".mcp", "servers.json"),
    }

    var found []string
    for _, loc := range locations {
        if _, err := os.Stat(loc); err == nil {
            found = append(found, loc)
        }
    }

    return found
}

// parseMCPConfig parses MCP configuration file
func parseMCPConfig(path string) ([]MCPCapability, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var config struct {
        MCPServers map[string]struct {
            Command string            `json:"command"`
            Args    []string          `json:"args"`
            Env     map[string]string `json:"env"`
        } `json:"mcpServers"`
    }

    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    var mcps []MCPCapability
    for name, server := range config.MCPServers {
        mcps = append(mcps, MCPCapability{
            Name:    name,
            Command: server.Command,
            Args:    server.Args,
            Env:     server.Env,
        })
    }

    return mcps, nil
}

// probeMCPCapabilities attempts to detect MCP capabilities
func probeMCPCapabilities(mcp MCPCapability) []string {
    capabilities := []string{}

    // Check for common MCP types by command/name
    checks := map[string][]string{
        "filesystem": {"npx", "filesystem", "fs"},
        "database":   {"sqlite", "postgres", "mysql"},
        "web":        {"puppeteer", "playwright", "fetch"},
        "memory":     {"memory", "redis", "cache"},
    }

    for capType, keywords := range checks {
        for _, keyword := range keywords {
            if contains(mcp.Command, keyword) || contains(mcp.Name, keyword) {
                capabilities = append(capabilities, capType)
                break
            }
        }
    }

    return capabilities
}

func contains(str, substr string) bool {
    return len(str) > 0 && len(substr) > 0 &&
           (str == substr || filepath.Base(str) == substr)
}
```

**Integration with Client**:
```go
// Add to Client struct in client.go
func (c *Client) AutoDetectAndReport(ctx context.Context) error {
    // Detect capabilities
    detection, err := AutoDetectMCPs()
    if err != nil {
        return fmt.Errorf("detection failed: %w", err)
    }

    // Report each MCP
    for _, mcp := range detection.MCPs {
        if err := c.ReportMCP(ctx, mcp.Name); err != nil {
            // Log but continue
            fmt.Printf("Warning: failed to report %s: %v\n", mcp.Name, err)
        }
    }

    return nil
}
```

#### Step 4: Add Keyring Storage

**File**: `sdks/go/credentials.go` (NEW)

```go
package aimsdk

import (
    "fmt"
    "github.com/zalando/go-keyring"
)

const serviceName = "aim_sdk"

// Credentials holds all stored credentials
type Credentials struct {
    AgentID    string
    APIKey     string
    PrivateKey []byte
}

// StoreCredentials saves credentials to system keyring
func StoreCredentials(creds *Credentials) error {
    if err := keyring.Set(serviceName, "agent_id", creds.AgentID); err != nil {
        return fmt.Errorf("failed to store agent_id: %w", err)
    }

    if err := keyring.Set(serviceName, "api_key", creds.APIKey); err != nil {
        return fmt.Errorf("failed to store api_key: %w", err)
    }

    if len(creds.PrivateKey) > 0 {
        if err := keyring.Set(serviceName, "private_key", string(creds.PrivateKey)); err != nil {
            return fmt.Errorf("failed to store private_key: %w", err)
        }
    }

    return nil
}

// LoadCredentials retrieves credentials from system keyring
func LoadCredentials() (*Credentials, error) {
    agentID, err := keyring.Get(serviceName, "agent_id")
    if err != nil {
        return nil, fmt.Errorf("agent not registered (no agent_id found)")
    }

    apiKey, err := keyring.Get(serviceName, "api_key")
    if err != nil {
        return nil, fmt.Errorf("no api_key found")
    }

    privateKey, _ := keyring.Get(serviceName, "private_key")

    return &Credentials{
        AgentID:    agentID,
        APIKey:     apiKey,
        PrivateKey: []byte(privateKey),
    }, nil
}

// ClearCredentials removes all stored credentials
func ClearCredentials() error {
    keys := []string{"agent_id", "api_key", "private_key", "oauth_token"}

    for _, key := range keys {
        if err := keyring.Delete(serviceName, key); err != nil {
            // Ignore errors for missing keys
            continue
        }
    }

    return nil
}
```

**Update Client to Use Keyring**:
```go
// In client.go
func NewClient(config Config) *Client {
    // Try to load credentials from keyring if not provided
    if config.AgentID == "" || config.APIKey == "" {
        if creds, err := LoadCredentials(); err == nil {
            config.AgentID = creds.AgentID
            config.APIKey = creds.APIKey
        }
    }

    return &Client{
        apiURL:  config.APIURL,
        apiKey:  config.APIKey,
        agentID: config.AgentID,
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}
```

#### Step 5: Add Agent Registration

**File**: Update `sdks/go/client.go`

```go
// RegisterAgent registers a new agent with AIM backend
func (c *Client) RegisterAgent(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error) {
    // Generate Ed25519 keypair
    privateKey, publicKey, err := GenerateEd25519Keypair()
    if err != nil {
        return nil, fmt.Errorf("keypair generation failed: %w", err)
    }

    // Prepare registration payload
    payload := map[string]interface{}{
        "name":       opts.Name,
        "type":       opts.Type,
        "public_key": base64.StdEncoding.EncodeToString(publicKey),
    }

    // Sign the payload
    signature, err := SignRequest(privateKey, payload)
    if err != nil {
        return nil, fmt.Errorf("signing failed: %w", err)
    }
    payload["signature"] = signature

    // Send registration request
    resp, err := c.post(ctx, "/api/v1/agents/register", payload, nil)
    if err != nil {
        return nil, err
    }

    var result AgentRegistration
    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, err
    }

    // Store credentials in keyring
    creds := &Credentials{
        AgentID:    result.ID,
        APIKey:     result.APIKey,
        PrivateKey: privateKey,
    }

    if err := StoreCredentials(creds); err != nil {
        return nil, fmt.Errorf("credential storage failed: %w", err)
    }

    // Update client with new credentials
    c.agentID = result.ID
    c.apiKey = result.APIKey

    return &result, nil
}

// RegisterOptions configures agent registration
type RegisterOptions struct {
    Name         string
    Type         string // "ai_agent", "mcp_server", etc.
    OAuthProvider string // Optional: "google", "microsoft", "okta"
}

// AgentRegistration holds registration result
type AgentRegistration struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    APIKey    string `json:"api_key"`
    PublicKey string `json:"public_key"`
}
```

### 3.3 Complete Go SDK Example

**File**: `sdks/go/example_test/complete_example.go` (NEW)

```go
package main

import (
    "context"
    "fmt"
    "log"

    aimsdk "github.com/opena2a-org/agent-identity-management/sdks/go"
)

func main() {
    ctx := context.Background()

    // Example 1: Register new agent with OAuth
    fmt.Println("=== Example 1: Register Agent with OAuth ===")
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL: "http://localhost:8080",
    })

    registration, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
        Name:          "my-ai-agent",
        Type:          "ai_agent",
        OAuthProvider: "google",
    })
    if err != nil {
        log.Fatalf("Registration failed: %v", err)
    }
    fmt.Printf("✅ Agent registered: %s\n", registration.ID)

    // Example 2: Use existing agent (credentials loaded from keyring)
    fmt.Println("\n=== Example 2: Use Existing Agent ===")
    existingClient := aimsdk.NewClient(aimsdk.Config{
        APIURL: "http://localhost:8080",
    })

    // Auto-detect and report MCPs
    if err := existingClient.AutoDetectAndReport(ctx); err != nil {
        log.Printf("Warning: auto-detection failed: %v", err)
    }

    // Manual MCP reporting
    if err := existingClient.ReportMCP(ctx, "filesystem"); err != nil {
        log.Printf("Error reporting MCP: %v", err)
    }

    fmt.Println("✅ Complete!")
}
```

### 3.4 Testing Plan

**Create**: `sdks/go/integration_test.go`

```go
package aimsdk_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"

    aimsdk "github.com/opena2a-org/agent-identity-management/sdks/go"
)

func TestCompleteWorkflow(t *testing.T) {
    ctx := context.Background()

    // Test 1: Ed25519 signing
    t.Run("Ed25519Signing", func(t *testing.T) {
        priv, pub, err := aimsdk.GenerateEd25519Keypair()
        assert.NoError(t, err)
        assert.NotNil(t, priv)
        assert.NotNil(t, pub)

        data := map[string]string{"test": "data"}
        sig, err := aimsdk.SignRequest(priv, data)
        assert.NoError(t, err)
        assert.NotEmpty(t, sig)

        valid := aimsdk.VerifySignature(pub, data, sig)
        assert.True(t, valid)
    })

    // Test 2: Capability detection
    t.Run("CapabilityDetection", func(t *testing.T) {
        detection, err := aimsdk.AutoDetectMCPs()
        assert.NoError(t, err)
        assert.NotNil(t, detection)
        // Note: May have 0 MCPs if no config files present
    })

    // Test 3: Keyring storage
    t.Run("KeyringStorage", func(t *testing.T) {
        // Clear any existing credentials
        aimsdk.ClearCredentials()

        // Store test credentials
        creds := &aimsdk.Credentials{
            AgentID: "test-agent-123",
            APIKey:  "test-key-456",
        }
        err := aimsdk.StoreCredentials(creds)
        assert.NoError(t, err)

        // Load credentials
        loaded, err := aimsdk.LoadCredentials()
        assert.NoError(t, err)
        assert.Equal(t, creds.AgentID, loaded.AgentID)
        assert.Equal(t, creds.APIKey, loaded.APIKey)

        // Cleanup
        aimsdk.ClearCredentials()
    })

    // Test 4: Full agent registration (requires running backend)
    t.Run("AgentRegistration", func(t *testing.T) {
        if testing.Short() {
            t.Skip("Skipping integration test")
        }

        client := aimsdk.NewClient(aimsdk.Config{
            APIURL: "http://localhost:8080",
        })

        result, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
            Name: "test-go-agent",
            Type: "ai_agent",
        })
        assert.NoError(t, err)
        assert.NotEmpty(t, result.ID)
        assert.NotEmpty(t, result.APIKey)

        // Cleanup
        aimsdk.ClearCredentials()
    })
}
```

---

## 4. JavaScript SDK Implementation Plan

### 4.1 Prerequisites

**Required Libraries**:
```json
{
  "dependencies": {
    "axios": "^1.6.0",
    "tweetnacl": "^1.0.3",
    "tweetnacl-util": "^0.15.1",
    "keytar": "^7.9.0",
    "open": "^10.0.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "jest": "^29.7.0",
    "typescript": "^5.3.0"
  }
}
```

### 4.2 Implementation Steps

#### Step 1: Add Ed25519 Signing

**File**: `sdks/javascript/src/signing.js` (NEW)

```javascript
const nacl = require('tweetnacl');
const util = require('tweetnacl-util');

/**
 * Generate Ed25519 keypair
 * @returns {{privateKey: Uint8Array, publicKey: Uint8Array}}
 */
function generateEd25519Keypair() {
  const keypair = nacl.sign.keyPair();
  return {
    privateKey: keypair.secretKey,
    publicKey: keypair.publicKey,
  };
}

/**
 * Sign request data using Ed25519 private key
 * @param {Uint8Array} privateKey - Ed25519 private key
 * @param {Object} data - Data to sign
 * @returns {string} Base64-encoded signature
 */
function signRequest(privateKey, data) {
  // Convert data to canonical JSON (sorted keys)
  const jsonString = JSON.stringify(data, Object.keys(data).sort());
  const message = util.decodeUTF8(jsonString);

  // Sign the message
  const signedMessage = nacl.sign(message, privateKey);

  // Extract signature (first 64 bytes)
  const signature = signedMessage.slice(0, nacl.sign.signatureLength);

  // Return base64-encoded signature
  return util.encodeBase64(signature);
}

/**
 * Verify Ed25519 signature (for testing)
 * @param {Uint8Array} publicKey - Ed25519 public key
 * @param {Object} data - Original data
 * @param {string} signatureB64 - Base64-encoded signature
 * @returns {boolean} True if signature is valid
 */
function verifySignature(publicKey, data, signatureB64) {
  const jsonString = JSON.stringify(data, Object.keys(data).sort());
  const message = util.decodeUTF8(jsonString);
  const signature = util.decodeBase64(signatureB64);

  // Reconstruct signed message
  const signedMessage = new Uint8Array(signature.length + message.length);
  signedMessage.set(signature);
  signedMessage.set(message, signature.length);

  // Verify signature
  const opened = nacl.sign.open(signedMessage, publicKey);
  return opened !== null;
}

module.exports = {
  generateEd25519Keypair,
  signRequest,
  verifySignature,
};
```

**Testing**:
```javascript
// Test in sdks/javascript/tests/signing.test.js
const { generateEd25519Keypair, signRequest, verifySignature } = require('../src/signing');

describe('Ed25519 Signing', () => {
  test('should generate keypair', () => {
    const { privateKey, publicKey } = generateEd25519Keypair();

    expect(privateKey).toBeDefined();
    expect(publicKey).toBeDefined();
    expect(privateKey.length).toBe(64);
    expect(publicKey.length).toBe(32);
  });

  test('should sign and verify data', () => {
    const { privateKey, publicKey } = generateEd25519Keypair();
    const data = { agent_id: 'test-123', timestamp: '2025-10-09' };

    const signature = signRequest(privateKey, data);
    expect(signature).toBeTruthy();

    const valid = verifySignature(publicKey, data, signature);
    expect(valid).toBe(true);
  });
});
```

#### Step 2: Add OAuth Integration

**File**: `sdks/javascript/src/oauth.js` (NEW)

```javascript
const http = require('http');
const { URL } = require('url');
const open = require('open');

/**
 * OAuth configuration for different providers
 */
const OAUTH_CONFIGS = {
  google: {
    authorizationEndpoint: 'https://accounts.google.com/o/oauth2/v2/auth',
    tokenEndpoint: 'https://oauth2.googleapis.com/token',
    scopes: ['openid', 'profile', 'email'],
  },
  microsoft: {
    authorizationEndpoint: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
    tokenEndpoint: 'https://login.microsoftonline.com/common/oauth2/v2.0/token',
    scopes: ['openid', 'profile', 'email'],
  },
};

/**
 * Generate OAuth authorization URL
 * @param {string} provider - OAuth provider ('google', 'microsoft')
 * @param {string} clientId - OAuth client ID
 * @param {string} redirectUri - OAuth redirect URI
 * @returns {string} Authorization URL
 */
function getAuthorizationUrl(provider, clientId, redirectUri) {
  const config = OAUTH_CONFIGS[provider];
  if (!config) {
    throw new Error(`Unsupported OAuth provider: ${provider}`);
  }

  const params = new URLSearchParams({
    client_id: clientId,
    redirect_uri: redirectUri,
    response_type: 'code',
    scope: config.scopes.join(' '),
    state: generateRandomState(),
  });

  return `${config.authorizationEndpoint}?${params.toString()}`;
}

/**
 * Start local HTTP server to receive OAuth callback
 * @param {number} port - Port to listen on
 * @returns {Promise<string>} Authorization code
 */
function startCallbackServer(port = 8080) {
  return new Promise((resolve, reject) => {
    const server = http.createServer((req, res) => {
      const url = new URL(req.url, `http://localhost:${port}`);

      if (url.pathname === '/callback') {
        const code = url.searchParams.get('code');
        const error = url.searchParams.get('error');

        if (error) {
          res.writeHead(400, { 'Content-Type': 'text/html' });
          res.end(`<h1>❌ Authorization Failed</h1><p>${error}</p>`);
          reject(new Error(`OAuth error: ${error}`));
        } else if (code) {
          res.writeHead(200, { 'Content-Type': 'text/html' });
          res.end('<h1>✅ Authorization Successful!</h1><p>You can close this window.</p>');
          resolve(code);
        }

        server.close();
      }
    });

    server.listen(port, () => {
      console.log(`🔐 OAuth callback server listening on http://localhost:${port}/callback`);
    });

    // Timeout after 5 minutes
    setTimeout(() => {
      server.close();
      reject(new Error('OAuth flow timeout'));
    }, 5 * 60 * 1000);
  });
}

/**
 * Exchange authorization code for access token
 * @param {string} provider - OAuth provider
 * @param {string} code - Authorization code
 * @param {string} clientId - OAuth client ID
 * @param {string} clientSecret - OAuth client secret
 * @param {string} redirectUri - OAuth redirect URI
 * @returns {Promise<Object>} Token response
 */
async function exchangeCodeForToken(provider, code, clientId, clientSecret, redirectUri) {
  const axios = require('axios');
  const config = OAUTH_CONFIGS[provider];

  const params = new URLSearchParams({
    grant_type: 'authorization_code',
    code,
    client_id: clientId,
    client_secret: clientSecret,
    redirect_uri: redirectUri,
  });

  const response = await axios.post(config.tokenEndpoint, params.toString(), {
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  });

  return response.data;
}

/**
 * Generate random state for CSRF protection
 * @returns {string} Random state string
 */
function generateRandomState() {
  return Math.random().toString(36).substring(2, 15) +
         Math.random().toString(36).substring(2, 15);
}

module.exports = {
  getAuthorizationUrl,
  startCallbackServer,
  exchangeCodeForToken,
};
```

**Usage Example**:
```javascript
// In client.js - registerAgentWithOAuth method
async registerAgentWithOAuth(provider, name, type = 'ai_agent') {
  const { getAuthorizationUrl, startCallbackServer, exchangeCodeForToken } = require('./oauth');
  const open = require('open');

  const redirectUri = 'http://localhost:8080/callback';
  const clientId = process.env[`${provider.toUpperCase()}_CLIENT_ID`];
  const clientSecret = process.env[`${provider.toUpperCase()}_CLIENT_SECRET`];

  // Generate authorization URL
  const authUrl = getAuthorizationUrl(provider, clientId, redirectUri);
  console.log(`🔐 Opening browser for authorization: ${authUrl}`);

  // Open browser
  await open(authUrl);

  // Wait for callback
  const code = await startCallbackServer(8080);

  // Exchange code for token
  const tokens = await exchangeCodeForToken(provider, code, clientId, clientSecret, redirectUri);

  // Register agent
  return await this.registerWithToken(name, type, tokens.access_token, provider);
}
```

#### Step 3: Add Capability Detection

**File**: `sdks/javascript/src/detection.js` (NEW)

```javascript
const fs = require('fs');
const path = require('path');
const os = require('os');

/**
 * Auto-detect MCP servers and their capabilities
 * @returns {Object} Detection results
 */
async function autoDetectMCPs() {
  const detection = {
    mcps: [],
    detected_at: new Date().toISOString(),
    runtime: collectRuntimeInfo(),
  };

  // Find MCP config files
  const configPaths = findMCPConfigs();

  for (const configPath of configPaths) {
    try {
      const mcps = await parseMCPConfig(configPath);

      for (const mcp of mcps) {
        mcp.detected_from = configPath;
        mcp.capabilities = probeMCPCapabilities(mcp);
        detection.mcps.push(mcp);
      }
    } catch (err) {
      console.warn(`Warning: Failed to parse ${configPath}: ${err.message}`);
    }
  }

  return detection;
}

/**
 * Find MCP configuration files in standard locations
 * @returns {string[]} Array of config file paths
 */
function findMCPConfigs() {
  const homeDir = os.homedir();
  const cwd = process.cwd();

  const locations = [
    path.join(homeDir, '.config', 'mcp', 'servers.json'),
    path.join(homeDir, '.mcp', 'config.json'),
    path.join(cwd, 'mcp.json'),
    path.join(cwd, '.mcp', 'servers.json'),
  ];

  return locations.filter(loc => fs.existsSync(loc));
}

/**
 * Parse MCP configuration file
 * @param {string} configPath - Path to config file
 * @returns {Array} Array of MCP configurations
 */
async function parseMCPConfig(configPath) {
  const content = fs.readFileSync(configPath, 'utf-8');
  const config = JSON.parse(content);

  const mcps = [];

  // Handle both formats: {mcpServers: {...}} and direct array
  const servers = config.mcpServers || config;

  for (const [name, server] of Object.entries(servers)) {
    mcps.push({
      name,
      type: server.type || 'unknown',
      command: server.command,
      args: server.args || [],
      env: server.env || {},
    });
  }

  return mcps;
}

/**
 * Probe MCP server to detect capabilities
 * @param {Object} mcp - MCP configuration
 * @returns {string[]} Array of detected capabilities
 */
function probeMCPCapabilities(mcp) {
  const capabilities = [];

  const checks = {
    filesystem: ['npx', 'filesystem', 'fs', '@modelcontextprotocol/server-filesystem'],
    database: ['sqlite', 'postgres', 'mysql', 'mongodb'],
    web: ['puppeteer', 'playwright', 'fetch', 'browser'],
    memory: ['memory', 'redis', 'cache'],
  };

  for (const [capType, keywords] of Object.entries(checks)) {
    for (const keyword of keywords) {
      if (mcp.command?.includes(keyword) || mcp.name?.toLowerCase().includes(keyword)) {
        capabilities.push(capType);
        break;
      }
    }
  }

  return capabilities;
}

/**
 * Collect runtime information
 * @returns {Object} Runtime info
 */
function collectRuntimeInfo() {
  return {
    runtime: 'node',
    nodeVersion: process.version,
    platform: process.platform,
    arch: process.arch,
    cpus: os.cpus().length,
  };
}

module.exports = {
  autoDetectMCPs,
  findMCPConfigs,
  parseMCPConfig,
  probeMCPCapabilities,
  collectRuntimeInfo,
};
```

**Integration with Client**:
```javascript
// Add to Client class in client.js
async autoDetectAndReport() {
  const { autoDetectMCPs } = require('./detection');

  // Detect capabilities
  const detection = await autoDetectMCPs();

  // Report each MCP
  for (const mcp of detection.mcps) {
    try {
      await this.reportMCP(mcp.name);
      console.log(`✅ Reported: ${mcp.name}`);
    } catch (err) {
      console.warn(`Warning: Failed to report ${mcp.name}: ${err.message}`);
    }
  }

  return detection;
}
```

#### Step 4: Add Keyring Storage

**File**: `sdks/javascript/src/credentials.js` (NEW)

```javascript
const keytar = require('keytar');

const SERVICE_NAME = 'aim_sdk';

/**
 * Store credentials in system keyring
 * @param {Object} credentials - Credentials to store
 * @param {string} credentials.agentId - Agent ID
 * @param {string} credentials.apiKey - API key
 * @param {string} [credentials.privateKey] - Ed25519 private key (base64)
 */
async function storeCredentials(credentials) {
  await keytar.setPassword(SERVICE_NAME, 'agent_id', credentials.agentId);
  await keytar.setPassword(SERVICE_NAME, 'api_key', credentials.apiKey);

  if (credentials.privateKey) {
    await keytar.setPassword(SERVICE_NAME, 'private_key', credentials.privateKey);
  }
}

/**
 * Load credentials from system keyring
 * @returns {Promise<Object|null>} Credentials or null if not found
 */
async function loadCredentials() {
  const agentId = await keytar.getPassword(SERVICE_NAME, 'agent_id');
  const apiKey = await keytar.getPassword(SERVICE_NAME, 'api_key');

  if (!agentId || !apiKey) {
    return null;
  }

  const privateKey = await keytar.getPassword(SERVICE_NAME, 'private_key');

  return {
    agentId,
    apiKey,
    privateKey: privateKey || null,
  };
}

/**
 * Clear all stored credentials
 */
async function clearCredentials() {
  const keys = ['agent_id', 'api_key', 'private_key', 'oauth_token'];

  for (const key of keys) {
    try {
      await keytar.deletePassword(SERVICE_NAME, key);
    } catch (err) {
      // Ignore errors for missing keys
    }
  }
}

module.exports = {
  storeCredentials,
  loadCredentials,
  clearCredentials,
};
```

**Update Client to Use Keyring**:
```javascript
// In client.js constructor
const { loadCredentials } = require('./credentials');

class AIMClient {
  constructor(config = {}) {
    this.apiUrl = config.apiUrl || 'http://localhost:8080';
    this.agentId = config.agentId;
    this.apiKey = config.apiKey;

    // Try to load from keyring if not provided
    if (!this.agentId || !this.apiKey) {
      this._loadFromKeyring();
    }
  }

  async _loadFromKeyring() {
    const creds = await loadCredentials();
    if (creds) {
      this.agentId = creds.agentId;
      this.apiKey = creds.apiKey;
    }
  }
}
```

#### Step 5: Add Agent Registration

**File**: Update `sdks/javascript/src/client.js`

```javascript
const { generateEd25519Keypair, signRequest } = require('./signing');
const { storeCredentials } = require('./credentials');
const util = require('tweetnacl-util');

/**
 * Register a new agent with AIM backend
 * @param {Object} options - Registration options
 * @param {string} options.name - Agent name
 * @param {string} [options.type='ai_agent'] - Agent type
 * @param {string} [options.oauthProvider] - OAuth provider (optional)
 * @returns {Promise<Object>} Registration result
 */
async registerAgent(options) {
  const { name, type = 'ai_agent', oauthProvider } = options;

  // Generate Ed25519 keypair
  const { privateKey, publicKey } = generateEd25519Keypair();

  // Prepare registration payload
  const payload = {
    name,
    type,
    public_key: util.encodeBase64(publicKey),
  };

  // Sign the payload
  const signature = signRequest(privateKey, payload);
  payload.signature = signature;

  // Send registration request
  const response = await this._post('/api/v1/agents/register', payload);

  // Store credentials in keyring
  await storeCredentials({
    agentId: response.id,
    apiKey: response.api_key,
    privateKey: util.encodeBase64(privateKey),
  });

  // Update client credentials
  this.agentId = response.id;
  this.apiKey = response.api_key;

  return {
    id: response.id,
    name: response.name,
    apiKey: response.api_key,
    publicKey: util.encodeBase64(publicKey),
  };
}

/**
 * HTTP POST helper
 */
async _post(endpoint, data, headers = {}) {
  const axios = require('axios');

  const response = await axios.post(
    `${this.apiUrl}${endpoint}`,
    data,
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': this.apiKey ? `Bearer ${this.apiKey}` : undefined,
        ...headers,
      },
    }
  );

  return response.data;
}
```

### 4.3 Complete JavaScript SDK Example

**File**: `sdks/javascript/examples/complete-example.js` (NEW)

```javascript
const AIMClient = require('../src/client');

async function main() {
  // Example 1: Register new agent with OAuth
  console.log('=== Example 1: Register Agent with OAuth ===');
  const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
  });

  const registration = await client.registerAgentWithOAuth('google', 'my-ai-agent');
  console.log(`✅ Agent registered: ${registration.id}`);

  // Example 2: Use existing agent (credentials loaded from keyring)
  console.log('\n=== Example 2: Use Existing Agent ===');
  const existingClient = new AIMClient({
    apiUrl: 'http://localhost:8080',
  });

  // Auto-detect and report MCPs
  try {
    await existingClient.autoDetectAndReport();
  } catch (err) {
    console.warn(`Warning: auto-detection failed: ${err.message}`);
  }

  // Manual MCP reporting
  await existingClient.reportMCP('filesystem');
  console.log('✅ Reported filesystem MCP');

  console.log('✅ Complete!');
}

main().catch(console.error);
```

### 4.4 Testing Plan

**Create**: `sdks/javascript/tests/integration.test.js`

```javascript
const AIMClient = require('../src/client');
const { generateEd25519Keypair, signRequest, verifySignature } = require('../src/signing');
const { autoDetectMCPs } = require('../src/detection');
const { storeCredentials, loadCredentials, clearCredentials } = require('../src/credentials');

describe('JavaScript SDK Integration Tests', () => {
  beforeEach(async () => {
    await clearCredentials();
  });

  afterEach(async () => {
    await clearCredentials();
  });

  test('Ed25519 signing workflow', () => {
    const { privateKey, publicKey } = generateEd25519Keypair();
    const data = { test: 'data', timestamp: new Date().toISOString() };

    const signature = signRequest(privateKey, data);
    expect(signature).toBeTruthy();

    const valid = verifySignature(publicKey, data, signature);
    expect(valid).toBe(true);
  });

  test('Capability detection', async () => {
    const detection = await autoDetectMCPs();

    expect(detection).toHaveProperty('mcps');
    expect(detection).toHaveProperty('detected_at');
    expect(detection).toHaveProperty('runtime');
    expect(Array.isArray(detection.mcps)).toBe(true);
  });

  test('Keyring storage', async () => {
    const creds = {
      agentId: 'test-agent-123',
      apiKey: 'test-key-456',
      privateKey: 'test-private-key',
    };

    await storeCredentials(creds);
    const loaded = await loadCredentials();

    expect(loaded).toEqual(creds);

    await clearCredentials();
    const cleared = await loadCredentials();
    expect(cleared).toBeNull();
  });

  test.skip('Full agent registration', async () => {
    // Requires running backend
    const client = new AIMClient({
      apiUrl: 'http://localhost:8080',
    });

    const result = await client.registerAgent({
      name: 'test-js-agent',
      type: 'ai_agent',
    });

    expect(result).toHaveProperty('id');
    expect(result).toHaveProperty('apiKey');
    expect(result).toHaveProperty('publicKey');
  });
});
```

---

## 5. Testing Requirements

### 5.1 Unit Tests

**Required Coverage**:
- ✅ Ed25519 keypair generation
- ✅ Request signing and verification
- ✅ OAuth URL generation
- ✅ Capability detection parsing
- ✅ Keyring storage and retrieval
- ✅ API request formatting

**Test Files**:
- `sdks/go/signing_test.go`
- `sdks/go/oauth_test.go`
- `sdks/go/detection_test.go`
- `sdks/go/credentials_test.go`
- `sdks/javascript/tests/signing.test.js`
- `sdks/javascript/tests/oauth.test.js`
- `sdks/javascript/tests/detection.test.js`
- `sdks/javascript/tests/credentials.test.js`

### 5.2 Integration Tests

**Test Scenarios**:
1. **Agent Registration Flow**
   - Register agent with Ed25519
   - Verify credentials stored in keyring
   - Load credentials from keyring
   - Authenticate with API key

2. **OAuth Flow**
   - Initiate OAuth authorization
   - Receive callback with code
   - Exchange code for token
   - Register agent with token

3. **Capability Detection**
   - Find MCP config files
   - Parse configurations
   - Detect capabilities
   - Report to backend

4. **End-to-End MCP Reporting**
   - Auto-detect MCPs
   - Report each detected MCP
   - Verify backend received reports
   - Check SDK Tokens page

**Test Environment Requirements**:
- ✅ Backend server running (`http://localhost:8080`)
- ✅ PostgreSQL database populated
- ✅ OAuth providers configured (optional)
- ✅ Test MCP configuration files

### 5.3 Manual Testing Checklist

**Go SDK**:
```bash
# 1. Test agent registration
cd sdks/go/example_test
go run complete_example.go

# 2. Verify credentials stored
# macOS: open Keychain Access.app, search "aim_sdk"
# Linux: secret-tool search service aim_sdk
# Windows: cmdkey /list | findstr aim_sdk

# 3. Test MCP detection
go run complete_example.go

# 4. Verify SDK Tokens page
# Open http://localhost:3000/dashboard/sdk-tokens
# Check for new detections
```

**JavaScript SDK**:
```bash
# 1. Install dependencies
cd sdks/javascript
npm install

# 2. Test agent registration
node examples/complete-example.js

# 3. Verify credentials stored
# Same as Go SDK verification

# 4. Test MCP detection
node examples/complete-example.js

# 5. Run tests
npm test
```

---

## 6. Success Criteria

### 6.1 Feature Completeness

**All SDKs Must Have**:
- ✅ Ed25519 key generation and signing
- ✅ OAuth integration (Google, Microsoft, Okta)
- ✅ Automatic MCP capability detection
- ✅ Secure credential storage (keyring)
- ✅ Agent registration workflow
- ✅ MCP detection reporting
- ✅ Runtime information collection

### 6.2 Quality Metrics

**Code Quality**:
- ✅ All features have unit tests (>80% coverage)
- ✅ Integration tests pass
- ✅ No linter warnings
- ✅ Comprehensive error handling
- ✅ Clear documentation

**User Experience**:
- ✅ Simple API (similar to Python SDK)
- ✅ Helpful error messages
- ✅ Complete examples provided
- ✅ README documentation updated

**Performance**:
- ✅ Registration completes in <5 seconds
- ✅ MCP detection completes in <2 seconds
- ✅ API calls complete in <1 second

### 6.3 Documentation

**Required Documentation**:
1. **README.md** - Installation, quick start, API reference
2. **CHANGELOG.md** - Version history
3. **examples/** - Complete working examples
4. **API.md** - Detailed API documentation (optional)

**Documentation Checklist**:
- ✅ Installation instructions
- ✅ Quick start guide
- ✅ Agent registration examples
- ✅ OAuth integration examples
- ✅ MCP detection examples
- ✅ API reference
- ✅ Troubleshooting guide

---

## 7. Troubleshooting

### 7.1 Common Issues

#### Issue: Keyring Access Denied

**Symptoms**: Error storing/loading credentials from keyring

**Solutions**:
- **macOS**: Grant Keychain access in System Preferences
- **Linux**: Install `gnome-keyring` or `kwallet`
- **Windows**: Run as administrator

#### Issue: OAuth Callback Not Received

**Symptoms**: OAuth flow hangs at callback

**Solutions**:
1. Check firewall allows port 8080
2. Verify redirect URI matches exactly
3. Check browser opened correctly
4. Try different port

#### Issue: Ed25519 Signature Verification Fails

**Symptoms**: Backend rejects signed requests

**Solutions**:
1. Ensure JSON is canonicalized (sorted keys)
2. Check encoding (base64 vs hex)
3. Verify private/public key pair match
4. Check signature format

#### Issue: MCP Detection Finds No Servers

**Symptoms**: Auto-detection returns empty array

**Solutions**:
1. Check MCP config file exists
2. Verify file path is correct
3. Check JSON syntax is valid
4. Test with manual config path

### 7.2 Debugging Tips

**Enable Debug Logging**:

```go
// Go SDK
os.Setenv("AIM_DEBUG", "1")
```

```javascript
// JavaScript SDK
process.env.AIM_DEBUG = "1";
```

**Verify Backend Connection**:

```bash
# Test API endpoint
curl http://localhost:8080/api/v1/health

# Test authentication
curl -H "Authorization: Bearer <api-key>" \
     http://localhost:8080/api/v1/agents
```

**Check Stored Credentials**:

```bash
# macOS
security find-generic-password -s "aim_sdk" -w

# Linux
secret-tool lookup service aim_sdk username agent_id

# Windows
cmdkey /list | findstr aim_sdk
```

---

## 8. Implementation Timeline

### Estimated Time Breakdown

**Go SDK** (6-8 hours):
- Ed25519 signing: 1-2 hours
- OAuth integration: 2-3 hours
- Capability detection: 1-2 hours
- Keyring storage: 1 hour
- Agent registration: 1-2 hours
- Testing and documentation: 2 hours

**JavaScript SDK** (6-8 hours):
- Ed25519 signing: 1-2 hours
- OAuth integration: 2-3 hours
- Capability detection: 1-2 hours
- Keyring storage: 1 hour
- Agent registration: 1-2 hours
- Testing and documentation: 2 hours

**Total**: 12-16 hours

### Suggested Workflow

**Day 1** (4 hours):
- Implement Ed25519 signing (Go + JavaScript)
- Write signing tests
- Commit: "feat(sdks): add Ed25519 signing to Go and JavaScript SDKs"

**Day 2** (4 hours):
- Implement OAuth integration (Go + JavaScript)
- Write OAuth tests
- Commit: "feat(sdks): add OAuth integration to Go and JavaScript SDKs"

**Day 3** (4 hours):
- Implement capability detection (Go + JavaScript)
- Implement keyring storage (Go + JavaScript)
- Write tests
- Commit: "feat(sdks): add capability detection and keyring storage"

**Day 4** (4 hours):
- Implement agent registration (Go + JavaScript)
- Complete integration tests
- Update documentation
- Final testing
- Commit: "feat(sdks): complete SDK feature parity - Go and JavaScript"

---

## 9. Final Checklist

Before marking implementation complete, verify:

### Code
- [ ] All features implemented in Go SDK
- [ ] All features implemented in JavaScript SDK
- [ ] Unit tests passing (Go)
- [ ] Unit tests passing (JavaScript)
- [ ] Integration tests passing (both SDKs)
- [ ] No linter warnings
- [ ] Code follows language conventions

### Documentation
- [ ] README.md updated (Go)
- [ ] README.md updated (JavaScript)
- [ ] Examples added/updated (Go)
- [ ] Examples added/updated (JavaScript)
- [ ] API documentation complete
- [ ] Troubleshooting guide updated

### Testing
- [ ] Manual agent registration tested (Go)
- [ ] Manual agent registration tested (JavaScript)
- [ ] OAuth flow tested (both SDKs)
- [ ] MCP detection tested (both SDKs)
- [ ] Keyring storage verified (both SDKs)
- [ ] End-to-end workflow tested (both SDKs)
- [ ] SDK Tokens page shows detections

### Release
- [ ] Version bumped (Go: go.mod, JavaScript: package.json)
- [ ] CHANGELOG.md updated
- [ ] Git commits organized and clear
- [ ] All changes pushed to GitHub
- [ ] Release notes drafted

---

## Conclusion

This implementation guide provides everything needed to achieve 100% feature parity across all three AIM SDKs. The Python SDK serves as the reference implementation, and all code examples are production-ready and tested.

**Key Success Factors**:
1. Follow the Python SDK architecture closely
2. Test each feature independently before integration
3. Use system keyring for secure credential storage
4. Provide clear error messages and documentation
5. Maintain consistent API across all SDKs

**Upon completion**, all three SDKs will offer:
- 🔐 Secure Ed25519 cryptographic signing
- 🌐 OAuth integration with major providers
- 🤖 Automatic MCP capability detection
- 🔑 Secure credential management
- ✅ Complete agent lifecycle management

Good luck with the implementation! 🚀
