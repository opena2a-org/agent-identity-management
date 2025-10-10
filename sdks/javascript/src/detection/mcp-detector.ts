import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

interface MCPDetection {
  mcpServer: string;
  detectionMethod: 'claude_config' | 'import';
  confidence: number;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
}

/**
 * Auto-detect MCP servers from Claude Desktop config
 */
export function autoDetectMCPs(): MCPDetection[] {
  const detections: MCPDetection[] = [];

  // Try to read Claude Desktop config
  const configPath = path.join(
    os.homedir(),
    'Library',
    'Application Support',
    'Claude',
    'claude_desktop_config.json'
  );

  // Also try alternate path
  const altConfigPath = path.join(
    os.homedir(),
    '.claude',
    'claude_desktop_config.json'
  );

  const pathsToTry = [configPath, altConfigPath];

  for (const configFile of pathsToTry) {
    if (fs.existsSync(configFile)) {
      try {
        const configContent = fs.readFileSync(configFile, 'utf-8');
        const config = JSON.parse(configContent);

        if (config.mcpServers && typeof config.mcpServers === 'object') {
          Object.entries(config.mcpServers).forEach(([name, serverConfig]: [string, any]) => {
            detections.push({
              mcpServer: name,
              detectionMethod: 'claude_config',
              confidence: 100,
              command: serverConfig.command,
              args: serverConfig.args,
              env: serverConfig.env,
            });
          });
        }
        break; // Found config, no need to try other paths
      } catch (error) {
        console.error('[AIM SDK] Failed to parse Claude Desktop config:', error);
      }
    }
  }

  return detections;
}
