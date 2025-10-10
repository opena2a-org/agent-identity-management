# Go SDK Enterprise Readiness Implementation Plan

**Goal**: Achieve full feature parity with Python SDK for enterprise production use
**Timeline**: 6-8 hours
**Status**: PLANNING → IMPLEMENTATION

---

## 📊 Current State vs Target

### Current (40% Complete)
- ✅ Basic HTTP client
- ✅ API key authentication
- ✅ MCP detection reporting
- ✅ `RegisterMCP()` method
- ✅ `ReportSDKIntegration()` method

### Target (100% Enterprise Ready)
- ✅ **Ed25519 cryptographic signing** (security)
- ✅ **Secure credential storage** (production)
- ✅ **Agent registration workflow** (onboarding)
- ✅ **Message signing for verification** (compliance)
- ⏳ OAuth integration (Phase 2 - optional for MVP)
- ⏳ Capability auto-detection (Phase 2)

---

## 🏗️ Implementation Phases

### **Phase 1: Core Security (PRIORITY 1)** ⚡
**Time**: 2-3 hours
**Blocking**: Required for enterprise use

#### 1.1 Ed25519 Signing Support
**Files to modify**:
- `sdks/go/signing.go` (NEW or update existing)
- `sdks/go/client.go` (add signing methods)
- `sdks/go/types.go` (add key types)

**Implementation**:
```go
// Use Go's crypto/ed25519 package
import "crypto/ed25519"

type Client struct {
    // Existing fields...
    signingKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
}

// New methods
func (c *Client) SignMessage(message string) (string, error)
func (c *Client) VerifyAction(actionType, resource string, context map[string]interface{}) (*VerificationResult, error)
```

**Key Features**:
- Generate Ed25519 keypairs
- Sign messages (base64 encoding)
- Verify public/private key pairs
- Support both 32-byte and 64-byte private keys (compatibility with backend)

---

#### 1.2 Secure Credential Storage
**Files modified**:
- `sdks/go/credentials.go` (UPDATED)

**Implementation**:
```go
type Credentials struct {
    AgentID    string
    APIKey     string
    PrivateKey ed25519.PrivateKey
}

func StoreCredentials(creds *Credentials) error  // System keyring
func LoadCredentials() (*Credentials, error)     // System keyring
func ClearCredentials() error                     // System keyring
```

**Security** (OS-Level Keyring):
- ✅ **macOS**: Keychain
- ✅ **Linux**: Secret Service / gnome-keyring
- ✅ **Windows**: Credential Locker
- ✅ **No plaintext files** - credentials encrypted by OS
- ✅ **No JSON storage** - keyring is more secure
- ✅ Uses `github.com/zalando/go-keyring` package

---

#### 1.3 Agent Registration Workflow
**Files to modify**:
- `sdks/go/registration.go` (update existing)
- `sdks/go/client.go` (add RegisterAgent function)

**Implementation**:
```go
type RegistrationOptions struct {
    Name              string
    APIURL            string
    APIKey            string
    DisplayName       string
    Description       string
    AgentType         string
    Version           string
    RepositoryURL     string
    DocumentationURL  string
    OrganizationDomain string
    TalksTo           []string
    Capabilities      []string
    AutoDetect        bool
    ForceNew          bool
}

func RegisterAgent(opts RegistrationOptions) (*Client, error)
```

**Features**:
- Check for existing credentials (reuse if found)
- Generate Ed25519 keypair client-side
- Call backend registration API
- Save credentials securely
- Return ready-to-use Client instance

---

### **Phase 2: Advanced Features (PRIORITY 2)** 🚀
**Time**: 3-4 hours
**Optional for MVP, Critical for Enterprise**

#### 2.1 OAuth Integration
**Files to create**:
- `sdks/go/oauth.go` (NEW)

**Implementation**:
```go
type OAuthTokenManager struct {
    credentialsPath string
    accessToken     string
    refreshToken    string
    expiresAt       time.Time
}

func NewOAuthTokenManager(credentialsPath string) *OAuthTokenManager
func (m *OAuthTokenManager) GetAccessToken() (string, error)
func (m *OAuthTokenManager) RefreshToken() error
```

---

#### 2.2 Capability Auto-Detection
**Files to create**:
- `sdks/go/capability_detection.go` (NEW)

**Implementation**:
```go
func AutoDetectCapabilities() ([]string, error)
func AutoDetectMCPs() ([]MCPDetection, error)
```

**Detection Methods**:
- Parse `go.mod` for MCP dependencies
- Check runtime environment
- Detect framework integrations

---

### **Phase 3: Testing & Verification** ✅
**Time**: 1-2 hours

#### 3.1 Unit Tests
```go
// sdks/go/signing_test.go
func TestSignMessage(t *testing.T)
func TestVerifyKeys(t *testing.T)

// sdks/go/credentials_test.go
func TestSaveLoadCredentials(t *testing.T)
func TestSecurePermissions(t *testing.T)

// sdks/go/registration_test.go
func TestRegisterAgent(t *testing.T)
```

#### 3.2 Integration Tests
```go
// sdks/go/integration_test.go
func TestFullRegistrationWorkflow(t *testing.T)
func TestSignedVerification(t *testing.T)
```

---

## 📁 File Structure After Implementation

```
sdks/go/
├── client.go                      # Main client (UPDATED)
├── types.go                       # Type definitions (UPDATED)
├── signing.go                     # Ed25519 signing (NEW/UPDATED)
├── credentials.go                 # Credential storage (UPDATED)
├── registration.go                # Agent registration (UPDATED)
├── oauth.go                       # OAuth support (NEW - Phase 2)
├── capability_detection.go        # Auto-detection (NEW - Phase 2)
├── reporter.go                    # API reporter (EXISTING)
├── detection.go                   # MCP detection (EXISTING)
├── intelligent_detection.go       # Smart detection (EXISTING)
│
├── signing_test.go                # Signing tests (NEW)
├── credentials_test.go            # Credential tests (NEW)
├── registration_test.go           # Registration tests (NEW)
├── integration_test.go            # E2E tests (NEW)
│
├── go.mod                         # Dependencies
├── go.sum                         # Checksums
└── README.md                      # Documentation
```

---

## 🔧 Go Dependencies Needed

```go
// go.mod additions
require (
    golang.org/x/crypto v0.17.0  // Ed25519 signing (crypto/ed25519)
    // Note: crypto/ed25519 is in stdlib since Go 1.13, but x/crypto has extras
)
```

---

## 🎯 Success Criteria

### Phase 1 Complete When:
- [x] Ed25519 signing works (sign + verify) ✅
- [x] Credentials save/load securely via OS keyring ✅
- [x] `RegisterAgent()` function creates working client ✅
- [x] Client can sign verification requests ✅
- [x] All unit tests pass (8/8 signing tests) ✅
- [x] Build successful with no errors ✅

**Status**: ✅ **PHASE 1 COMPLETE** (October 10, 2025)

### Phase 2 Complete When:
- [ ] OAuth token management works
- [ ] Auto-detection finds MCP dependencies
- [ ] Integration tests pass

### Enterprise Ready When:
- [ ] Python SDK feature parity (core features)
- [ ] Comprehensive documentation
- [ ] Example code works
- [ ] Security review passed

---

## 🚀 Execution Order

### Immediate (Now):
1. ✅ Implement Ed25519 signing
2. ✅ Implement credential storage
3. ✅ Implement agent registration
4. ✅ Add unit tests
5. ✅ Verify end-to-end

### Next Session:
1. OAuth integration
2. Capability detection
3. Integration tests
4. Documentation update

---

## 💡 Design Decisions

### Why Start with Ed25519?
- **Security first**: Core requirement for enterprise
- **Compliance**: Required for cryptographic verification
- **Blocking**: Other features depend on this

### Why Defer OAuth to Phase 2?
- **API key works**: Functional alternative exists
- **Time**: OAuth is complex, needs thorough testing
- **Priority**: Ed25519 + Registration more critical

### Why Defer Auto-Detection?
- **Manual works**: Users can specify capabilities
- **Complexity**: Go module parsing is non-trivial
- **ROI**: Lower impact than core security features

### Why Use Keyring Instead of JSON Files?
- **Security First**: OS-level encryption vs plaintext files
- **Enterprise Standard**: macOS Keychain, Windows Credential Locker are industry standard
- **No Attack Surface**: JSON files can be stolen, keyring is protected by OS
- **Decision**: Rejected JSON file storage - keyring only ✅

---

## 🎓 Reference Implementation

**Python SDK** is the gold standard:
- `sdks/python/aim_sdk/client.py` - Lines 84-146 (Ed25519)
- `sdks/python/aim_sdk/client.py` - Lines 682-745 (Credentials)
- `sdks/python/aim_sdk/client.py` - Lines 748-1140 (Registration)

**Port these patterns to idiomatic Go**:
- Use `crypto/ed25519` instead of PyNaCl
- Use `os.FileMode(0600)` for secure permissions
- Use `encoding/base64` for key encoding
- Follow Go error handling conventions

---

**Starting Phase 1 implementation NOW** 🚀
