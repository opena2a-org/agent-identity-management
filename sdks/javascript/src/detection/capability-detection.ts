import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

export interface MCPCapability {
  name: string;
  type: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  detectedFrom: string;
  capabilities: string[];
}

export interface DetectionResult {
  mcps: MCPCapability[];
  detectedAt: string;
  runtime: Record<string, string>;
}

/**
 * Auto-detect MCP servers and their capabilities
 * Searches for MCP configuration files in standard locations
 */
export async function autoDetectMCPs(): Promise<DetectionResult> {
  const result: DetectionResult = {
    mcps: [],
    detectedAt: new Date().toISOString(),
    runtime: collectRuntimeInfo(),
  };

  // Find MCP config files in standard locations
  const configPaths = findMCPConfigs();

  for (const configPath of configPaths) {
    try {
      const mcps = await parseMCPConfig(configPath);

      for (const mcp of mcps) {
        mcp.detectedFrom = configPath;
        mcp.capabilities = probeMCPCapabilities(mcp);
        result.mcps.push(mcp);
      }
    } catch (error) {
      // Log warning but continue with other configs
      console.warn(`Warning: Failed to parse ${configPath}: ${error}`);
    }
  }

  return result;
}

/**
 * Find MCP configuration files in standard locations
 */
function findMCPConfigs(): string[] {
  const homeDir = os.homedir();
  const cwd = process.cwd();

  const locations = [
    path.join(homeDir, '.config', 'mcp', 'servers.json'),
    path.join(homeDir, '.mcp', 'config.json'),
    path.join(homeDir, '.config', 'claude', 'mcp', 'servers.json'),
    path.join(cwd, 'mcp.json'),
    path.join(cwd, '.mcp', 'servers.json'),
    path.join(cwd, 'mcp', 'servers.json'),
  ];

  return locations.filter((loc) => fs.existsSync(loc));
}

/**
 * Parse MCP configuration file
 */
async function parseMCPConfig(configPath: string): Promise<MCPCapability[]> {
  const content = fs.readFileSync(configPath, 'utf-8');
  const config = JSON.parse(content);

  const mcps: MCPCapability[] = [];

  // Handle both formats: {mcpServers: {...}} and direct array
  const servers = config.mcpServers || config;

  for (const [name, server] of Object.entries(servers) as [string, any][]) {
    mcps.push({
      name,
      type: server.type || 'unknown',
      command: server.command,
      args: server.args || [],
      env: server.env || {},
      detectedFrom: '',
      capabilities: [],
    });
  }

  return mcps;
}

/**
 * Probe MCP server to detect capabilities based on command and name patterns
 */
function probeMCPCapabilities(mcp: MCPCapability): string[] {
  const capabilities: string[] = [];

  const checks: Record<string, string[]> = {
    filesystem: ['npx', 'filesystem', 'fs', '@modelcontextprotocol/server-filesystem', 'file'],
    database: ['sqlite', 'postgres', 'postgresql', 'mysql', 'mongodb', 'db'],
    web: ['puppeteer', 'playwright', 'fetch', 'browser', 'http'],
    memory: ['memory', 'redis', 'cache', 'qdrant', 'vector'],
    github: ['github', 'git'],
    sequential: ['sequential', 'thinking'],
    brave: ['brave', 'search'],
  };

  const commandLower = (mcp.command || '').toLowerCase();
  const nameLower = mcp.name.toLowerCase();

  for (const [capType, keywords] of Object.entries(checks)) {
    for (const keyword of keywords) {
      if (commandLower.includes(keyword) || nameLower.includes(keyword)) {
        capabilities.push(capType);
        break;
      }
    }
  }

  // Check args for additional hints
  if (mcp.args) {
    for (const arg of mcp.args) {
      const argLower = arg.toLowerCase();
      for (const [capType, keywords] of Object.entries(checks)) {
        for (const keyword of keywords) {
          if (argLower.includes(keyword) && !capabilities.includes(capType)) {
            capabilities.push(capType);
            break;
          }
        }
      }
    }
  }

  return capabilities;
}

/**
 * Collect runtime information about Node.js environment
 */
function collectRuntimeInfo(): Record<string, string> {
  return {
    runtime: 'node',
    node_version: process.version,
    platform: process.platform,
    arch: process.arch,
    num_cpus: os.cpus().length.toString(),
  };
}
