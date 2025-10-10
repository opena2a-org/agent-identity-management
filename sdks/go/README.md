# AIM SDK for Go

**Agent Identity Management SDK** - Enterprise-grade identity and capability management for AI agents.

## 🚀 Features

- ✅ **Ed25519 Cryptographic Signing** - Secure agent identity verification
- ✅ **OAuth/OIDC Integration** - Enterprise SSO (Google, Microsoft, Okta)
- ✅ **Automatic MCP Detection** - Discover MCP servers from configs
- ✅ **Secure Credential Storage** - System keyring integration (Keychain/Credential Locker)
- ✅ **Agent Registration** - Complete onboarding workflow
- ✅ **Manual MCP Reporting** - Report MCP usage to AIM backend
- ✅ **Type-Safe API** - Full Go type safety
- ✅ **Context Support** - Native context.Context integration
- ✅ **Graceful Shutdown** - Clean resource cleanup

## 📦 Installation

```bash
go get github.com/opena2a/aim-sdk-go
```

## 🎯 Quick Start

### Option 1: Register a New Agent

```go
package main

import (
    "context"
    "fmt"
    "log"

    aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
    ctx := context.Background()

    // Create client without credentials
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL: "http://localhost:8080",
    })

    // Register new agent (generates Ed25519 keypair)
    registration, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
        Name: "my-go-agent",
        Type: "ai_agent",
        Description: "My first Go agent",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("✅ Agent registered: %s\n", registration.ID)
    fmt.Printf("   Credentials stored in system keyring\n")
}
```

### Option 2: Use Existing Agent

```go
package main

import (
    "context"
    "log"

    aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
    // Load credentials from system keyring
    creds, err := aimsdk.LoadCredentials()
    if err != nil {
        log.Fatal("No credentials found. Please register first.")
    }

    // Create client with stored credentials
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:  "http://localhost:8080",
        AgentID: creds.AgentID,
        APIKey:  creds.APIKey,
    })
    defer client.Close()

    // Auto-detect and report MCPs
    aimsdk.AutoDetectAndReport(client)
}
```

## 📚 Core Features

### 1. Ed25519 Cryptographic Signing

Secure agent identity verification using Ed25519 digital signatures.

```go
import aimsdk "github.com/opena2a/aim-sdk-go"

// Generate new keypair
privateKey, publicKey, err := aimsdk.GenerateEd25519Keypair()
if err != nil {
    log.Fatal(err)
}

// Sign data
data := map[string]interface{}{
    "agent_id": "agent-123",
    "timestamp": time.Now().Format(time.RFC3339),
}
signature, err := aimsdk.SignRequest(privateKey, data)

// Verify signature
valid := aimsdk.VerifySignature(publicKey, data, signature)
fmt.Printf("Signature valid: %v\n", valid)

// Encode keys for storage
publicKeyB64 := aimsdk.EncodePublicKey(publicKey)
privateKeyB64 := aimsdk.EncodePrivateKey(privateKey)
```

### 2. OAuth/OIDC Integration

Enterprise SSO authentication with Google, Microsoft, and Okta.

```go
// Register agent with OAuth
registration, err := client.RegisterAgentWithOAuth(ctx, aimsdk.RegisterOptions{
    Name:          "oauth-agent",
    Type:          "ai_agent",
    OAuthProvider: aimsdk.OAuthProviderGoogle,
    RedirectURL:   "http://localhost:8080/callback",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("✅ Registered with OAuth: %s\n", registration.ID)
```

**Supported Providers:**
- `OAuthProviderGoogle` - Google (accounts.google.com)
- `OAuthProviderMicrosoft` - Microsoft (login.microsoftonline.com)
- `OAuthProviderOkta` - Okta (custom domain)

**OAuth Flow:**
1. SDK generates authorization URL
2. Opens browser for user consent
3. Starts local callback server (port 8080)
4. Receives authorization code
5. Exchanges code for access token
6. Registers agent with token

### 3. Automatic MCP Detection

Discover MCP servers from configuration files.

```go
// Auto-detect MCPs
detection, err := aimsdk.AutoDetectMCPs()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d MCP(s):\n", len(detection.MCPs))
for _, mcp := range detection.MCPs {
    fmt.Printf("  - %s (%s)\n", mcp.Name, strings.Join(mcp.Capabilities, ", "))
}

// Auto-detect and report in one step
aimsdk.AutoDetectAndReport(client)
```

**Detection Locations:**
- `~/.config/mcp/servers.json`
- `~/.mcp/config.json`
- `~/.config/claude/mcp/servers.json`
- `./mcp.json`
- `./.mcp/servers.json`

**Detected Capabilities:**
- `filesystem` - File operations
- `database` - SQL/NoSQL databases (sqlite, postgres, mongodb)
- `web` - Browser automation
- `memory` - Vector/cache storage
- `github` - GitHub integration
- `sequential` - Sequential thinking
- `brave` - Brave search

### 4. Secure Credential Storage

System keyring integration for secure credential management.

```go
// Store credentials
err := aimsdk.StoreCredentials(&aimsdk.Credentials{
    AgentID:    "agent-123",
    APIKey:     "aim_key_456",
    PrivateKey: privateKey,
})

// Load credentials
creds, err := aimsdk.LoadCredentials()
if err != nil {
    log.Fatal("No credentials found")
}

// Check if credentials exist
exists := aimsdk.HasCredentials()
fmt.Printf("Credentials exist: %v\n", exists)

// Clear all credentials
err = aimsdk.ClearCredentials()
```

**Platform Support:**
- **macOS**: Keychain Access
- **Windows**: Credential Locker
- **Linux**: Secret Service (GNOME Keyring, KWallet)

### 5. Agent Registration

Complete agent onboarding workflow.

```go
// Basic registration (Ed25519 only)
registration, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
    Name:        "my-agent",
    Type:        "ai_agent",
    Description: "My AI agent",
})

// OAuth registration
registration, err := client.RegisterAgentWithOAuth(ctx, aimsdk.RegisterOptions{
    Name:          "oauth-agent",
    Type:          "ai_agent",
    OAuthProvider: aimsdk.OAuthProviderGoogle,
    RedirectURL:   "http://localhost:8080/callback",
})
```

**Registration Flow:**
1. Generate Ed25519 keypair
2. Create payload (name, type, public_key)
3. Sign payload with private key
4. Send registration request to backend
5. Receive agent_id and api_key
6. Store all credentials in system keyring
7. Update client with new credentials

### 6. MCP Reporting

Report MCP usage to AIM backend.

```go
// Manual reporting
err := client.ReportMCP(context.Background(), "filesystem")
if err != nil {
    log.Printf("Failed to report: %v", err)
}

// Get runtime info
info := client.GetRuntimeInfo()
fmt.Printf("Runtime: %+v\n", info)
// {runtime: "go", goVersion: "go1.23.0", os: "darwin", ...}
```

## 🔧 API Reference

### Client Configuration

```go
type Config struct {
    APIURL         string        // AIM API URL (required)
    APIKey         string        // API key (optional, loaded from keyring if empty)
    AgentID        string        // Agent ID (optional, loaded from keyring if empty)
    AutoDetect     bool          // Enable auto-detection (default: false)
    ReportInterval time.Duration // Report interval (default: 10s)
}
```

### Registration Options

```go
type RegisterOptions struct {
    Name          string        // Agent name (required)
    Type          string        // Agent type: "ai_agent" or "human_agent" (required)
    Description   string        // Agent description (optional)
    OAuthProvider OAuthProvider // OAuth provider (optional)
    RedirectURL   string        // OAuth redirect URL (optional, default: http://localhost:8080/callback)
}
```

### Client Methods

**`NewClient(config Config) *Client`**
- Create new AIM client

**`RegisterAgent(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error)`**
- Register new agent with Ed25519 signing

**`RegisterAgentWithOAuth(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error)`**
- Register agent with OAuth/OIDC

**`ReportMCP(ctx context.Context, name string) error`**
- Manually report MCP usage

**`GetRuntimeInfo() map[string]interface{}`**
- Get Go runtime information

**`Close()`**
- Clean up resources

### Credential Functions

**`StoreCredentials(creds *Credentials) error`**
- Store credentials in system keyring

**`LoadCredentials() (*Credentials, error)`**
- Load credentials from system keyring

**`HasCredentials() bool`**
- Check if credentials exist

**`ClearCredentials() error`**
- Clear all credentials

### Signing Functions

**`GenerateEd25519Keypair() (privateKey, publicKey, error)`**
- Generate new Ed25519 keypair

**`SignRequest(privateKey, data) (string, error)`**
- Sign data with private key

**`VerifySignature(publicKey, data, signature) bool`**
- Verify signed data

**`EncodePublicKey(publicKey) string`**
- Encode public key to base64

**`DecodePublicKey(encoded) ([]byte, error)`**
- Decode base64 public key

**`EncodePrivateKey(privateKey) string`**
- Encode private key to base64

**`DecodePrivateKey(encoded) ([]byte, error)`**
- Decode base64 private key

### Detection Functions

**`AutoDetectMCPs() (*Detection, error)`**
- Auto-detect MCP servers from configs

**`AutoDetectAndReport(client *Client)`**
- Auto-detect and report all MCPs

## 📖 Complete Example

See [`examples/complete/main.go`](./examples/complete/main.go) for a comprehensive example demonstrating all SDK features.

```bash
# Run the complete example
cd examples/complete
go run main.go
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -v -run TestSignRequest
```

**Test Coverage:**
- ✅ Ed25519 signing (9 test cases)
- ✅ Keypair generation
- ✅ Signature verification
- ✅ Key encoding/decoding
- ✅ Error handling

## 🔒 Security

- **Ed25519**: Industry-standard elliptic curve signatures
- **System Keyring**: Never store credentials in plaintext
- **OAuth PKCE**: CSRF protection via state parameter
- **Canonical JSON**: Deterministic signing with sorted keys
- **Base64 Encoding**: Safe key transmission

## ⚡ Performance

- **Initialization**: <10ms
- **Memory Usage**: <5MB
- **CPU Overhead**: <0.01% (imperceptible)
- **Network**: 1 API call per manual report or periodic interval
- **Signing**: <1ms per operation

## 🐛 Troubleshooting

### "No credentials found"
```go
// Register a new agent first
registration, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
    Name: "my-agent",
    Type: "ai_agent",
})
```

### "Failed to access keyring"
- **macOS**: Grant Keychain Access permission
- **Windows**: Ensure Credential Locker is enabled
- **Linux**: Install gnome-keyring or kwallet

### "OAuth callback timeout"
- Check that port 8080 is available
- Ensure browser opens automatically
- Verify redirect URL matches OAuth config

## 📝 License

MIT

## 🤝 Support

For issues and questions:
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Documentation**: https://docs.opena2a.org/aim-sdk-go

## 🔗 Related SDKs

- [Python SDK](../python/) - Python/asyncio implementation
- [JavaScript SDK](../javascript/) - Node.js/TypeScript implementation

---

**Version**: 1.0.0
**Go Version**: 1.21+
**Status**: Production Ready ✅
