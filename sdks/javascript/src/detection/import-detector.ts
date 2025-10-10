import Module from 'module';
import { DetectionMethod, DetectedMCP } from '../types';

export class ImportDetector implements DetectionMethod {
  private detectedMCPs: Set<string> = new Set();
  private originalRequire?: typeof Module.prototype.require;

  start(): void {
    this.hookRequire();
  }

  stop(): void {
    if (this.originalRequire) {
      Module.prototype.require = this.originalRequire;
    }
  }

  private hookRequire(): void {
    const self = this;
    this.originalRequire = Module.prototype.require;

    // TypeScript typing workaround
    const moduleProto = Module.prototype as any;

    moduleProto.require = function (this: any, id: string) {
      // Detect MCP packages
      if (id.startsWith('@modelcontextprotocol/')) {
        // Extract MCP server name
        // Example: @modelcontextprotocol/server-filesystem → filesystem
        const mcpName = id
          .replace('@modelcontextprotocol/server-', '')
          .replace('@modelcontextprotocol/sdk', 'sdk-core');

        if (mcpName !== 'sdk-core') {
          self.detectedMCPs.add(mcpName);
        }
      }

      // Also detect MCP SDK usage patterns
      if (id.includes('mcp-sdk') || id.includes('mcp/')) {
        const parts = id.split('/');
        const serverName = parts[parts.length - 1];
        if (serverName && serverName !== 'mcp-sdk') {
          self.detectedMCPs.add(serverName);
        }
      }

      // Call original require
      return self.originalRequire!.apply(this, arguments as any);
    };
  }

  getDetections(): DetectedMCP[] {
    return Array.from(this.detectedMCPs).map(name => ({
      name,
      detectionMethod: 'sdk_import',
      confidenceScore: 95.0,
      details: { source: 'import_hook' },
    }));
  }
}
