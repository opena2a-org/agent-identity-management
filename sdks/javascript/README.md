# @aim/sdk - JavaScript/TypeScript SDK

AIM SDK for automatic MCP detection in AI agents.

## Installation

```bash
npm install @aim/sdk
```

## Quick Start

```javascript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: 'https://aim.yourcompany.com',
  apiKey: process.env.AIM_API_KEY,
  agentId: 'your-agent-id',
  autoDetect: true, // Enable auto-detection
});

// SDK will automatically detect and report MCP usage!
```

## Features

- ✅ Automatic MCP detection from imports
- ✅ Automatic reporting to AIM API
- ✅ Zero-config operation
- ✅ TypeScript support
- ✅ Capability auto-detection
- ✅ Claude Desktop config parsing

## API

### `new AIMClient(config)`

Create a new AIM client.

**Config Options:**
- `apiUrl` (required): AIM API URL
- `apiKey` (required): Your AIM API key
- `agentId` (required): Your agent ID
- `autoDetect` (optional): Enable auto-detection (default: true)
- `detectionMethods` (optional): Detection methods to use (default: ['import', 'connection'])
- `reportInterval` (optional): Report interval in milliseconds (default: 10000)

### `client.detect()`

Manually trigger detection (returns array of detected MCPs).

```javascript
const detections = await aim.detect();
console.log('Detected MCPs:', detections);
```

### `client.reportMCP(name)`

Manually report a specific MCP usage.

```javascript
await aim.reportMCP('filesystem');
```

### `client.destroy()`

Clean up resources (stop detectors, clear intervals).

```javascript
aim.destroy();
```

## Utility Functions

### `autoDetectCapabilities()`

Automatically detect agent capabilities from loaded modules.

```javascript
import { autoDetectCapabilities } from '@aim/sdk';

const capabilities = autoDetectCapabilities();
console.log('Capabilities:', capabilities);
// ['make_api_calls', 'read_files', 'write_files', 'access_database']
```

### `autoDetectMCPs()`

Detect MCP servers from Claude Desktop configuration.

```javascript
import { autoDetectMCPs } from '@aim/sdk';

const mcps = autoDetectMCPs();
console.log('MCPs from config:', mcps);
// [{ mcpServer: 'filesystem', detectionMethod: 'claude_config', confidence: 100, ... }]
```

## Example Usage

```javascript
import { AIMClient, autoDetectCapabilities, autoDetectMCPs } from '@aim/sdk';

async function main() {
  // Initialize AIM client
  const aim = new AIMClient({
    apiUrl: 'http://localhost:8080',
    apiKey: process.env.AIM_API_KEY,
    agentId: process.env.AIM_AGENT_ID,
  });

  // Auto-detect capabilities
  const capabilities = autoDetectCapabilities();
  console.log('Auto-detected capabilities:', capabilities);

  // Auto-detect MCPs from Claude config
  const mcps = autoDetectMCPs();
  console.log('MCPs from Claude config:', mcps);

  // SDK will automatically report detections every 10 seconds
  console.log('AIM SDK is now monitoring MCP usage...');

  // Clean up on exit
  process.on('SIGINT', () => {
    console.log('Cleaning up...');
    aim.destroy();
    process.exit(0);
  });
}

main().catch(console.error);
```

## TypeScript Support

The SDK is written in TypeScript and includes full type definitions.

```typescript
import { AIMClient, AIMClientConfig, DetectedMCP } from '@aim/sdk';

const config: AIMClientConfig = {
  apiUrl: 'http://localhost:8080',
  apiKey: 'aim_key_123',
  agentId: 'agent-uuid',
  autoDetect: true,
};

const client = new AIMClient(config);
```

## How It Works

1. **Import Detection**: The SDK hooks into Node.js's `require()` to detect when MCP packages are imported
2. **Automatic Reporting**: Detections are reported to the AIM API every 10 seconds (configurable)
3. **Deduplication**: MCPs are only reported once per 60 seconds to avoid spam
4. **Silent Failures**: Network failures don't break your agent - errors are logged but ignored

## Performance

- **Initialization**: <50ms
- **Memory Usage**: <10MB
- **CPU Overhead**: <0.1% (imperceptible)
- **Network**: 1 API call every 10 seconds (only if new detections)

## License

MIT

## Support

For issues and questions, please visit:
https://github.com/opena2a-org/agent-identity-management/issues
