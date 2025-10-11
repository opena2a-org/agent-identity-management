# AIM SDK for JavaScript/TypeScript

**Agent Identity Management SDK** - Enterprise-grade identity and capability management for AI agents.

## 🚀 Features

- ✅ **Ed25519 Cryptographic Signing** - Secure agent identity verification
- ✅ **OAuth/OIDC Integration** - Enterprise SSO (Google, Microsoft, Okta)
- ✅ **Automatic MCP Detection** - Discover MCP servers from configs
- ✅ **Secure Credential Storage** - System keyring integration (Keychain/Credential Locker)
- ✅ **Agent Registration** - Complete onboarding workflow
- ✅ **Manual MCP Reporting** - Report MCP usage to AIM backend
- ✅ **TypeScript Support** - Full type safety with TypeScript
- ✅ **Zero-Config Operation** - Automatic detection and reporting
- ✅ **Claude Desktop Integration** - Parse Claude Desktop configs

## 📦 Installation

```bash
npm install @aim/sdk
# or
yarn add @aim/sdk
# or
pnpm add @aim/sdk
```

## 🎯 Quick Start

### Option 1: Register a New Agent

```typescript
import { AIMClient } from '@aim/sdk';

async function main() {
  const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
  });

  // Register new agent (generates Ed25519 keypair)
  const registration = await client.registerAgent({
    name: 'my-js-agent',
    type: 'ai_agent',
    description: 'My first JavaScript agent',
  });

  console.log(`✅ Agent registered: ${registration.id}`);
  console.log('   Credentials stored in system keyring');
}

main();
```

### Option 2: Use Existing Agent

```typescript
import { AIMClient } from '@aim/sdk';

async function main() {
  // Load client from system keyring
  const client = await AIMClient.fromKeyring('http://localhost:8080');

  // Auto-detect and report MCPs
  await client.autoDetectAndReport();

  console.log('✅ MCPs reported successfully');
}

main();
```

## 📚 Core Features

### 1. Ed25519 Cryptographic Signing

Secure agent identity verification using Ed25519 digital signatures.

```typescript
import {
  generateEd25519Keypair,
  signRequest,
  verifySignature,
  encodePublicKey,
  encodePrivateKey,
} from '@aim/sdk';

// Generate new keypair
const { privateKey, publicKey } = generateEd25519Keypair();

// Sign data
const data = {
  agent_id: 'agent-123',
  timestamp: new Date().toISOString(),
};
const signature = signRequest(privateKey, data);

// Verify signature
const valid = verifySignature(publicKey, data, signature);
console.log(`Signature valid: ${valid}`);

// Encode keys for storage
const publicKeyB64 = encodePublicKey(publicKey);
const privateKeyB64 = encodePrivateKey(privateKey);
```

### 2. OAuth/OIDC Integration

Enterprise SSO authentication with Google, Microsoft, and Okta.

```typescript
import { AIMClient, OAuthProvider } from '@aim/sdk';

const client = new AIMClient({
  apiUrl: 'http://localhost:8080',
});

// Register agent with OAuth
const registration = await client.registerAgentWithOAuth({
  name: 'oauth-agent',
  type: 'ai_agent',
  oauthProvider: OAuthProvider.Google,
  redirectUrl: 'http://localhost:8080/callback',
});

console.log(`✅ Registered with OAuth: ${registration.id}`);
```

**Supported Providers:**
- `OAuthProvider.Google` - Google (accounts.google.com)
- `OAuthProvider.Microsoft` - Microsoft (login.microsoftonline.com)
- `OAuthProvider.Okta` - Okta (custom domain)

**OAuth Flow:**
1. SDK generates authorization URL
2. Opens browser for user consent
3. Starts local callback server (port 8080)
4. Receives authorization code
5. Exchanges code for access token
6. Registers agent with token

### 3. Automatic MCP Detection

Discover MCP servers from configuration files.

```typescript
import { autoDetectMCPs } from '@aim/sdk';

// Auto-detect MCPs
const detection = await autoDetectMCPs();

console.log(`Found ${detection.mcps.length} MCP(s):`);
detection.mcps.forEach((mcp) => {
  console.log(`  - ${mcp.name} (${mcp.capabilities.join(', ')})`);
});

// Detection includes runtime info
console.log(`Runtime: ${detection.runtime.runtime}`);
console.log(`Node Version: ${detection.runtime.node_version}`);
console.log(`Platform: ${detection.runtime.platform}`);
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

```typescript
import {
  storeCredentials,
  loadCredentials,
  hasCredentials,
  clearCredentials,
} from '@aim/sdk';

// Store credentials
await storeCredentials({
  agentId: 'agent-123',
  apiKey: 'aim_key_456',
  privateKey: privateKey,
});

// Load credentials
const creds = await loadCredentials();
if (creds) {
  console.log(`Agent ID: ${creds.agentId}`);
}

// Check if credentials exist
const exists = await hasCredentials();
console.log(`Credentials exist: ${exists}`);

// Clear all credentials
await clearCredentials();
```

**Platform Support:**
- **macOS**: Keychain Access
- **Windows**: Credential Locker
- **Linux**: Secret Service (GNOME Keyring, KWallet)

### 5. Agent Registration

Complete agent onboarding workflow.

```typescript
// Basic registration (Ed25519 only)
const registration = await client.registerAgent({
  name: 'my-agent',
  type: 'ai_agent',
  description: 'My AI agent',
});

// OAuth registration
const registration = await client.registerAgentWithOAuth({
  name: 'oauth-agent',
  type: 'ai_agent',
  oauthProvider: OAuthProvider.Google,
  redirectUrl: 'http://localhost:8080/callback',
});
```

**Registration Flow:**
1. Generate Ed25519 keypair
2. Create payload (name, type, public_key)
3. Sign payload with private key
4. Send registration request to backend
5. Receive agent_id and api_key
6. Store all credentials in system keyring
7. Update client with new credentials

### 6. Capability Management - Auto-Grant Workflow 🔒

#### Initial Registration: Auto-Grant (No Approval Needed!)

When you register an agent, **capabilities are automatically granted** - no admin approval required!

```typescript
import { AIMClient } from '@aim/sdk';

async function registerAgent() {
  const client = new AIMClient({ apiUrl: 'http://localhost:8080' });

  // Capabilities detected and AUTO-GRANTED immediately
  const registration = await client.registerAgent({
    name: 'my-agent',
    type: 'ai_agent',
    description: 'My AI agent',
  });

  // ✅ Capabilities: Auto-detected from your code
  // ✅ Granted: Automatically during registration
  // ✅ Ready to use: Perform actions immediately!
}
```

**This is a game-changer**: Users can start using agents immediately without waiting for admin approval.

#### Capability Updates: Admin Approval Required

If you need to add NEW capabilities after registration, admins must approve:

```typescript
// Request new capability (requires admin approval)
const request = await client.capabilities.request({
  capability_type: 'delete_email',
  reason: 'Need to clean up spam automatically',
});

console.log(`Request created: ${request.id}`);
console.log(`Status: ${request.status}`);  // "pending"

// Admin reviews and approves via dashboard
// Once approved, capability is automatically granted
```

**Why this workflow?**
- **Fast onboarding**: Users start immediately
- **Security**: Admins review capability expansions
- **Scalability**: No bottleneck for thousands of agents

#### How Enforcement Works

AIM enforces capabilities using a **single source of truth**:

```typescript
// ✅ ENFORCEMENT: Only GRANTED capabilities are enforced
// - agent.capabilities (array) = DECLARED (reference only)
// - agent_capabilities (table) = GRANTED (enforcement)

// This action requires "read_email" capability
const result = await client.verifyAction({
  action_type: 'read_email',
  resource: 'inbox'
});

// ✅ Allowed if "read_email" was GRANTED
// ❌ Denied if "read_email" not granted (even if declared)
```

**Security Benefits**:
- Prevents CVE-2025-32711 (EchoLeak) attacks
- Admin control over capability expansion
- Full audit trail (who granted what, when)

#### Alternative: Delete and Re-register

Don't want to wait for admin approval? Delete your agent and re-register with updated capabilities:

```typescript
// Delete existing agent
await client.agents.delete(agentId);

// Re-register with updated capabilities
const registration = await client.registerAgent({
  name: 'my-agent',
  type: 'ai_agent',
  capabilities: ['read_email', 'send_email', 'delete_email']  // ✅ All auto-granted
});
```

**Trade-off**: Loses historical trust score and audit logs.

### 7. MCP Reporting

Report MCP usage to AIM backend.

```typescript
// Manual reporting
await client.reportMCP('filesystem');

// Auto-detect and report all MCPs
await client.autoDetectAndReport();
```

## 🔧 API Reference

### Client Configuration

```typescript
interface AIMClientConfig {
  apiUrl: string;           // AIM API URL (required)
  apiKey?: string;          // API key (optional, loaded from keyring if empty)
  agentId?: string;         // Agent ID (optional, loaded from keyring if empty)
  autoDetect?: boolean;     // Enable auto-detection (default: true)
  reportInterval?: number;  // Report interval in ms (default: 10000)
}
```

### Registration Options

```typescript
interface RegisterOptions {
  name: string;             // Agent name (required)
  type: 'ai_agent' | 'human_agent';  // Agent type (required)
  description?: string;     // Agent description (optional)
  oauthProvider?: OAuthProvider;     // OAuth provider (optional)
  redirectUrl?: string;     // OAuth redirect URL (optional, default: http://localhost:8080/callback)
}
```

### Client Methods

**`new AIMClient(config: AIMClientConfig)`**
- Create new AIM client

**`static async fromKeyring(apiUrl: string): Promise<AIMClient>`**
- Load client from stored credentials

**`async registerAgent(opts: RegisterOptions): Promise<AgentRegistration>`**
- Register new agent with Ed25519 signing

**`async registerAgentWithOAuth(opts: RegisterOptions): Promise<AgentRegistration>`**
- Register agent with OAuth/OIDC

**`async reportMCP(name: string): Promise<void>`**
- Manually report MCP usage

**`async autoDetectAndReport(): Promise<void>`**
- Auto-detect and report all MCPs

**`destroy(): void`**
- Clean up resources

### Credential Functions

**`async storeCredentials(creds: Credentials): Promise<void>`**
- Store credentials in system keyring

**`async loadCredentials(): Promise<Credentials | null>`**
- Load credentials from system keyring

**`async hasCredentials(): Promise<boolean>`**
- Check if credentials exist

**`async clearCredentials(): Promise<void>`**
- Clear all credentials

### Signing Functions

**`generateEd25519Keypair(): { privateKey: Uint8Array, publicKey: Uint8Array }`**
- Generate new Ed25519 keypair

**`signRequest(privateKey: Uint8Array, data: Record<string, any>): string`**
- Sign data with private key

**`verifySignature(publicKey: Uint8Array, data: Record<string, any>, signature: string): boolean`**
- Verify signed data

**`encodePublicKey(publicKey: Uint8Array): string`**
- Encode public key to base64

**`decodePublicKey(encoded: string): Uint8Array`**
- Decode base64 public key

**`encodePrivateKey(privateKey: Uint8Array): string`**
- Encode private key to base64

**`decodePrivateKey(encoded: string): Uint8Array`**
- Decode base64 private key

### Detection Functions

**`async autoDetectMCPs(): Promise<Detection>`**
- Auto-detect MCP servers from configs

**`autoDetectCapabilities(): string[]`**
- Auto-detect agent capabilities (legacy)

## 📖 Complete Example

See [`examples/complete-example.ts`](./examples/complete-example.ts) for a comprehensive example demonstrating all SDK features.

```bash
# Run the complete example
npm run example

# Run specific example
npm run example 1  # Register agent
npm run example 2  # OAuth registration
npm run example 3  # Use existing agent
npm run example 4  # Auto-detect MCPs
npm run example 5  # Report MCPs
npm run example 6  # Manual reporting
npm run example 7  # Clear credentials
```

## 🧪 Testing

```bash
# Run all tests
npm test

# Run with coverage
npm test -- --coverage

# Run specific test file
npm test signing.test.ts
```

**Test Coverage:**
- ✅ Ed25519 signing (11 test cases)
- ✅ MCP detection (8 test cases)
- ✅ Integration tests (10+ test cases)
- ✅ Keypair generation
- ✅ Signature verification
- ✅ Key encoding/decoding
- ✅ Credential management
- ✅ Complete lifecycle workflow

## 🔒 Security

- **Ed25519**: Industry-standard elliptic curve signatures (via tweetnacl)
- **System Keyring**: Never store credentials in plaintext (via keytar)
- **OAuth PKCE**: CSRF protection via state parameter
- **Canonical JSON**: Deterministic signing with sorted keys
- **Base64 Encoding**: Safe key transmission

## ⚡ Performance

- **Initialization**: <50ms
- **Memory Usage**: <10MB
- **CPU Overhead**: <0.1% (imperceptible)
- **Network**: 1 API call per manual report or periodic interval
- **Signing**: <1ms per operation

## 🐛 Troubleshooting

### "No credentials found"
```typescript
// Register a new agent first
const registration = await client.registerAgent({
  name: 'my-agent',
  type: 'ai_agent',
});
```

### "Failed to access keyring"
- **macOS**: Grant Keychain Access permission
- **Windows**: Ensure Credential Locker is enabled
- **Linux**: Install gnome-keyring or kwallet

### "OAuth callback timeout"
- Check that port 8080 is available
- Ensure browser opens automatically
- Verify redirect URL matches OAuth config

### TypeScript Errors
```bash
# Ensure TypeScript is installed
npm install --save-dev typescript @types/node

# Check tsconfig.json includes SDK types
{
  "compilerOptions": {
    "moduleResolution": "node",
    "esModuleInterop": true
  }
}
```

## 📝 TypeScript Support

The SDK is written in TypeScript and includes full type definitions.

```typescript
import type {
  AIMClient,
  AIMClientConfig,
  RegisterOptions,
  AgentRegistration,
  DetectedMCP,
  Detection,
  Credentials,
} from '@aim/sdk';
```

## 🔄 Migration from AIVF SDK

If you're migrating from the old AIVF SDK:

```typescript
// Old AIVF SDK
import { AIVFClient } from '@aivf/sdk';
const client = new AIVFClient({ apiKey, agentId });

// New AIM SDK
import { AIMClient } from '@aim/sdk';
const client = new AIMClient({ apiUrl, apiKey, agentId });
```

**Breaking Changes:**
- `apiUrl` is now required (was optional)
- `autoDetect` is now `true` by default (was `false`)
- MCP detection now includes capability probing
- Credentials are now stored in system keyring (not environment variables)

## 📝 License

MIT

## 🤝 Support

For issues and questions:
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Documentation**: https://docs.opena2a.org/aim-sdk-js

## 🔗 Related SDKs

- [Python SDK](../python/) - Python/asyncio implementation
- [Go SDK](../go/) - Go implementation

---

**Version**: 1.0.0
**Node.js Version**: 16+
**Status**: Production Ready ✅
