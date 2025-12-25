# AIM SDK for TypeScript/Node.js

Official TypeScript SDK for Agent Identity Management (AIM) - secure identity verification for AI agents.

## Installation

```bash
npm install @opena2a/aim-sdk
# or
yarn add @opena2a/aim-sdk
# or
pnpm add @opena2a/aim-sdk
```

## Quick Start

```typescript
import { AIMClient, AgentType } from '@opena2a/aim-sdk';

// Create client
const client = new AIMClient({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
});

// Register an agent
const agent = await client.registerAgent({
  name: 'my-ai-agent',
  displayName: 'My AI Agent',
  agentType: AgentType.LANGCHAIN,
  capabilities: ['file:read', 'api:call'],
});

// Verify an action before execution
const result = await client.verifyAction({
  action: 'file:read',
  resource: '/data/config.json',
});

console.log(`Action allowed: ${result.actionAllowed}`);
console.log(`Trust score: ${result.trustScore}`);
```

## Features

- **Full TypeScript Support**: Complete type definitions for all APIs
- **Ed25519 Signatures**: Cryptographic signing for secure verification
- **OAuth 2.0**: Automatic token management with client credentials flow
- **Express Middleware**: Easy integration with Express.js applications
- **Fastify Plugin**: First-class support for Fastify applications
- **Automatic Retries**: Built-in retry logic with exponential backoff

## Express Integration

```typescript
import express from 'express';
import { createAIMMiddleware, verifyAction } from '@opena2a/aim-sdk/express';

const app = express();

// Add AIM middleware globally
app.use(createAIMMiddleware({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
}));

// Verify specific actions on routes
app.post('/api/data',
  verifyAction('data:write'),
  (req, res) => {
    // Action has been verified
    res.json({ success: true });
  }
);

// Access AIM context in handlers
app.get('/api/profile', (req, res) => {
  const { agentId, trustScore } = req.aim ?? {};
  res.json({ agentId, trustScore });
});
```

## Fastify Integration

```typescript
import Fastify from 'fastify';
import { aimPlugin, verifyAction } from '@opena2a/aim-sdk/fastify';

const fastify = Fastify();

// Register the AIM plugin
await fastify.register(aimPlugin, {
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
});

// Verify actions with preHandler hook
fastify.post('/api/data', {
  preHandler: verifyAction('data:write'),
}, async (request, reply) => {
  return { success: true };
});
```

## Configuration

### Environment Variables

The SDK automatically reads from these environment variables:

| Variable | Description |
|----------|-------------|
| `AIM_BASE_URL` | AIM server base URL |
| `AIM_API_KEY` | API key for authentication |
| `AIM_ORGANIZATION_ID` | Organization ID |
| `AIM_AGENT_ID` | Pre-registered agent ID |
| `AIM_PRIVATE_KEY` | Ed25519 private key (base64) |
| `AIM_PUBLIC_KEY` | Ed25519 public key (base64) |
| `AIM_DEBUG` | Enable debug logging (`true`/`false`) |

### Client Options

```typescript
const client = new AIMClient({
  baseUrl: 'https://aim.example.com',  // AIM server URL
  apiKey: 'your-api-key',              // API key
  organizationId: 'org-uuid',          // Organization ID
  autoRegister: true,                  // Auto-register if not registered
  timeout: 30000,                      // Request timeout in ms
  debug: false,                        // Debug logging
  headers: {},                         // Custom headers
});
```

## Agent Types

The SDK supports various agent types:

```typescript
import { AgentType } from '@opena2a/aim-sdk';

// LLM Providers
AgentType.CLAUDE
AgentType.GPT
AgentType.GEMINI

// Frameworks
AgentType.LANGCHAIN
AgentType.CREWAI
AgentType.AUTOGEN

// Assistants
AgentType.COPILOT
AgentType.ASSISTANT
```

## Error Handling

```typescript
import {
  ActionDeniedError,
  AuthenticationError,
  RateLimitError,
} from '@opena2a/aim-sdk';

try {
  await client.verifyAction({ action: 'file:delete' });
} catch (error) {
  if (error instanceof ActionDeniedError) {
    console.log(`Denied: ${error.reason}`);
    console.log(`Trust score: ${error.trustScore}`);
  } else if (error instanceof RateLimitError) {
    console.log(`Retry after: ${error.retryAfter} seconds`);
  }
}
```

## Credential Management

```typescript
import {
  loadCredentialsFromFile,
  saveCredentialsToFile,
} from '@opena2a/aim-sdk';

// Save credentials after registration
const credentials = client.getCredentials();
await saveCredentialsToFile(credentials, '.aim/credentials.json');

// Load credentials on startup
const saved = await loadCredentialsFromFile('.aim/credentials.json');
client.setCredentials(saved);
```

## API Reference

### AIMClient

- `registerAgent(options)` - Register a new agent
- `verifyAction(options)` - Verify an action
- `getAgent()` - Get current agent info
- `updateAgent(updates)` - Update agent metadata
- `reportCapabilities(capabilities)` - Report agent capabilities
- `getTrustScore()` - Get current trust score
- `getCredentials()` - Get stored credentials
- `setCredentials(credentials)` - Set credentials

### Types

See [src/types/index.ts](./src/types/index.ts) for complete type definitions.

## License

Apache-2.0

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.
