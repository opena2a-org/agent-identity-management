import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { MCPCapability, DetectionResult } from './capability-detection';

/**
 * Configuration for intelligent MCP detection
 */
export interface IntelligentDetectionConfig {
  // Detection level
  level?: 'minimal' | 'standard' | 'deep';

  // Tier 1 options (static analysis)
  scanPackages?: boolean;
  scanImports?: boolean;
  scanConfigFiles?: boolean;

  // Tier 2 options (runtime hooks)
  hookModuleLoads?: boolean;
  hookChildProcesses?: boolean;
  hookWebSockets?: boolean;

  // Tier 3 options (deep inspection - requires explicit opt-in)
  enableASTAnalysis?: boolean;
  enableDeepDependencyTree?: boolean;
  enableNetworkMonitoring?: boolean;

  // Performance options
  cacheTimeout?: number;
  watchForChanges?: boolean;
  maxDetectionTimeMs?: number;
}

/**
 * Performance metrics for detection
 */
export interface PerformanceMetrics {
  detectionTimeMs: number;
  tier1TimeMs: number;
  tier2TimeMs: number;
  tier3TimeMs?: number;
  cpuOverheadPercent: number;
  memoryUsageMb: number;
  cacheHitRate: number;
  mcpsDetected: number;
}

/**
 * Enhanced detection result with performance metrics
 */
export interface IntelligentDetectionResult extends DetectionResult {
  performanceMetrics: PerformanceMetrics;
  detectionConfig: IntelligentDetectionConfig;
}

/**
 * Known MCP package patterns for fast lookup
 */
const KNOWN_MCP_PACKAGES = new Set([
  '@modelcontextprotocol/sdk',
  '@modelcontextprotocol/server-filesystem',
  '@modelcontextprotocol/server-github',
  '@modelcontextprotocol/server-brave-search',
  '@modelcontextprotocol/server-memory',
  'mcp-server-sqlite',
  'mcp-server-postgres',
  'mcp-server-puppeteer',
  'sequential-thinking-mcp',
]);

/**
 * Cache for detected MCPs
 */
interface DetectionCache {
  mcps: MCPCapability[];
  detectedAt: number;
  ttl: number;
}

let detectionCache: DetectionCache | null = null;

/**
 * Default configuration (Tier 1 + Tier 2 - recommended)
 */
const DEFAULT_CONFIG: IntelligentDetectionConfig = {
  level: 'standard',

  // Tier 1 (always enabled)
  scanPackages: true,
  scanImports: true,
  scanConfigFiles: true,

  // Tier 2 (enabled in standard mode)
  hookModuleLoads: true,
  hookChildProcesses: true,
  hookWebSockets: true,

  // Tier 3 (disabled by default)
  enableASTAnalysis: false,
  enableDeepDependencyTree: false,
  enableNetworkMonitoring: false,

  // Performance
  cacheTimeout: 300000, // 5 minutes
  watchForChanges: true,
  maxDetectionTimeMs: 100,
};

/**
 * Intelligent MCP auto-detection with performance monitoring
 */
export async function intelligentAutoDetectMCPs(
  config?: IntelligentDetectionConfig
): Promise<IntelligentDetectionResult> {
  const startTime = Date.now();
  const cfg = { ...DEFAULT_CONFIG, ...config };

  // Apply level presets
  if (cfg.level === 'minimal') {
    cfg.hookModuleLoads = false;
    cfg.hookChildProcesses = false;
    cfg.hookWebSockets = false;
  } else if (cfg.level === 'deep') {
    cfg.enableASTAnalysis = true;
    cfg.enableDeepDependencyTree = true;
    // Network monitoring still requires explicit consent
  }

  // Check cache first
  if (detectionCache && isCacheValid(detectionCache, cfg.cacheTimeout!)) {
    return {
      mcps: detectionCache.mcps,
      detectedAt: new Date(detectionCache.detectedAt).toISOString(),
      runtime: collectRuntimeInfo(),
      performanceMetrics: {
        detectionTimeMs: Date.now() - startTime,
        tier1TimeMs: 0,
        tier2TimeMs: 0,
        cpuOverheadPercent: 0,
        memoryUsageMb: getMemoryUsage(),
        cacheHitRate: 1.0,
        mcpsDetected: detectionCache.mcps.length,
      },
      detectionConfig: cfg,
    };
  }

  const mcps: MCPCapability[] = [];
  let tier1Time = 0;
  let tier2Time = 0;
  let tier3Time = 0;

  // === TIER 1: Static Detection ===
  const tier1Start = Date.now();

  // 1. Scan package.json for MCP packages
  if (cfg.scanPackages) {
    const packageMCPs = await detectFromPackageJson();
    mcps.push(...packageMCPs);
  }

  // 2. Scan import statements
  if (cfg.scanImports) {
    const importMCPs = await detectFromImports();
    mcps.push(...importMCPs);
  }

  // 3. Scan config files (backward compatibility)
  if (cfg.scanConfigFiles) {
    const configMCPs = await detectFromConfigFiles();
    mcps.push(...configMCPs);
  }

  tier1Time = Date.now() - tier1Start;

  // === TIER 2: Runtime Hooks ===
  if (cfg.level !== 'minimal') {
    const tier2Start = Date.now();

    if (cfg.hookModuleLoads) {
      setupModuleLoadHook(mcps);
    }

    if (cfg.hookChildProcesses) {
      setupChildProcessHook(mcps);
    }

    if (cfg.hookWebSockets) {
      setupWebSocketHook(mcps);
    }

    tier2Time = Date.now() - tier2Start;
  }

  // === TIER 3: Deep Inspection (opt-in only) ===
  if (cfg.level === 'deep') {
    const tier3Start = Date.now();

    if (cfg.enableASTAnalysis) {
      console.warn('[AIM SDK] AST analysis enabled - may add ~50ms per file');
      // TODO: Implement AST parsing
    }

    if (cfg.enableDeepDependencyTree) {
      console.warn('[AIM SDK] Deep dependency analysis enabled - may add ~500ms');
      // TODO: Implement deep dependency tree
    }

    if (cfg.enableNetworkMonitoring) {
      console.warn(
        '[AIM SDK] ⚠️  Network monitoring enabled - This may add 2-5% CPU overhead. ' +
        'Make sure you have user consent for traffic monitoring.'
      );
      // TODO: Implement network traffic monitoring
    }

    tier3Time = Date.now() - tier3Start;
  }

  // Deduplicate MCPs by name
  const uniqueMCPs = deduplicateMCPs(mcps);

  // Update cache
  detectionCache = {
    mcps: uniqueMCPs,
    detectedAt: Date.now(),
    ttl: cfg.cacheTimeout!,
  };

  // Calculate performance metrics
  const totalTime = Date.now() - startTime;
  const metrics: PerformanceMetrics = {
    detectionTimeMs: totalTime,
    tier1TimeMs: tier1Time,
    tier2TimeMs: tier2Time,
    tier3TimeMs: tier3Time > 0 ? tier3Time : undefined,
    cpuOverheadPercent: estimateCPUOverhead(cfg),
    memoryUsageMb: getMemoryUsage(),
    cacheHitRate: 0.0,
    mcpsDetected: uniqueMCPs.length,
  };

  // Warn if detection is slow
  if (totalTime > cfg.maxDetectionTimeMs!) {
    console.warn(
      `[AIM SDK] MCP detection took ${totalTime}ms (expected <${cfg.maxDetectionTimeMs}ms). ` +
      `Consider using 'minimal' mode for faster startup.`
    );
  }

  return {
    mcps: uniqueMCPs,
    detectedAt: new Date().toISOString(),
    runtime: collectRuntimeInfo(),
    performanceMetrics: metrics,
    detectionConfig: cfg,
  };
}

/**
 * Tier 1: Detect MCPs from package.json dependencies
 */
async function detectFromPackageJson(): Promise<MCPCapability[]> {
  const mcps: MCPCapability[] = [];

  try {
    const cwd = process.cwd();
    const packagePath = path.join(cwd, 'package.json');

    if (!fs.existsSync(packagePath)) {
      return mcps;
    }

    const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf-8'));
    const allDeps = {
      ...packageJson.dependencies,
      ...packageJson.devDependencies,
    };

    // Fast O(n) scan for MCP packages
    for (const [name, version] of Object.entries(allDeps)) {
      if (isMCPPackage(name)) {
        mcps.push({
          name: name,
          type: 'package',
          command: 'node',
          args: ['-e', `require('${name}')`],
          detectedFrom: packagePath,
          capabilities: inferCapabilitiesFromPackageName(name),
        });
      }
    }
  } catch (error) {
    // Silently fail - package.json might not exist
  }

  return mcps;
}

/**
 * Tier 1: Detect MCPs from import statements in source files
 */
async function detectFromImports(): Promise<MCPCapability[]> {
  const mcps: MCPCapability[] = [];

  try {
    const cwd = process.cwd();
    const entryPoints = ['index.js', 'index.ts', 'src/index.js', 'src/index.ts', 'main.js'];

    for (const entry of entryPoints) {
      const filePath = path.join(cwd, entry);

      if (!fs.existsSync(filePath)) {
        continue;
      }

      const content = fs.readFileSync(filePath, 'utf-8');

      // Regex to match import/require statements
      const importRegex = /(?:import|require)\s*\(?['"]([^'"]+)['"]\)?/g;
      let match;

      while ((match = importRegex.exec(content)) !== null) {
        const packageName = match[1];

        if (isMCPPackage(packageName)) {
          mcps.push({
            name: packageName,
            type: 'import',
            command: 'node',
            args: [],
            detectedFrom: filePath,
            capabilities: inferCapabilitiesFromPackageName(packageName),
          });
        }
      }
    }
  } catch (error) {
    // Silently fail
  }

  return mcps;
}

/**
 * Tier 1: Detect MCPs from config files (backward compatibility)
 */
async function detectFromConfigFiles(): Promise<MCPCapability[]> {
  const mcps: MCPCapability[] = [];
  const configPaths = findMCPConfigs();

  for (const configPath of configPaths) {
    try {
      const content = fs.readFileSync(configPath, 'utf-8');
      const config = JSON.parse(content);
      const servers = config.mcpServers || config;

      for (const [name, server] of Object.entries(servers) as [string, any][]) {
        mcps.push({
          name,
          type: server.type || 'config',
          command: server.command,
          args: server.args || [],
          env: server.env || {},
          detectedFrom: configPath,
          capabilities: probeMCPCapabilities({ name, command: server.command, args: server.args }),
        });
      }
    } catch (error) {
      // Continue with other config files
    }
  }

  return mcps;
}

/**
 * Tier 2: Setup module load hook to detect MCP package loading at runtime
 */
function setupModuleLoadHook(mcps: MCPCapability[]): void {
  // Hook require() to detect MCP loads
  const Module = require('module');
  const originalRequire = Module.prototype.require;

  Module.prototype.require = function (id: string) {
    const module = originalRequire.apply(this, arguments);

    // Fast path: only check if MCP-related (~0.001ms overhead)
    if (isMCPPackage(id)) {
      const existing = mcps.find((m) => m.name === id);
      if (!existing) {
        mcps.push({
          name: id,
          type: 'runtime',
          command: 'node',
          args: [],
          detectedFrom: 'runtime-hook',
          capabilities: inferCapabilitiesFromPackageName(id),
        });
      }
    }

    return module;
  };
}

/**
 * Tier 2: Setup child process hook to detect MCP server spawns
 */
function setupChildProcessHook(mcps: MCPCapability[]): void {
  const childProcess = require('child_process');
  const originalSpawn = childProcess.spawn;

  childProcess.spawn = function (command: string, args?: string[]) {
    // Check if spawning an MCP server (~0.01ms overhead)
    if (isMCPServerCommand(command, args || [])) {
      const existing = mcps.find((m) => m.command === command);
      if (!existing) {
        mcps.push({
          name: command,
          type: 'process',
          command,
          args: args || [],
          detectedFrom: 'spawn-hook',
          capabilities: probeMCPCapabilities({ name: command, command, args: args || [] }),
        });
      }
    }

    return originalSpawn.apply(this, arguments);
  };
}

/**
 * Tier 2: Setup WebSocket hook to detect MCP server connections
 */
function setupWebSocketHook(mcps: MCPCapability[]): void {
  if (typeof WebSocket === 'undefined') {
    return; // WebSocket not available in this environment
  }

  const OriginalWebSocket = WebSocket;

  (global as any).WebSocket = function (url: string, protocols?: string | string[]) {
    // Check if connecting to MCP server (~0.01ms overhead)
    if (isMCPServerURL(url)) {
      const serverName = extractServerNameFromURL(url);
      const existing = mcps.find((m) => m.name === serverName);

      if (!existing) {
        mcps.push({
          name: serverName,
          type: 'websocket',
          command: 'websocket',
          args: [url],
          detectedFrom: 'websocket-hook',
          capabilities: ['network'],
        });
      }
    }

    return new OriginalWebSocket(url, protocols);
  };
}

/**
 * Check if package name is a known MCP package
 */
function isMCPPackage(name: string): boolean {
  if (KNOWN_MCP_PACKAGES.has(name)) {
    return true;
  }

  // Pattern matching for MCP packages
  const mcpPatterns = [
    /^@modelcontextprotocol\//,
    /^mcp-server-/,
    /-mcp$/,
    /sequential-thinking/,
  ];

  return mcpPatterns.some((pattern) => pattern.test(name));
}

/**
 * Check if command is spawning an MCP server
 */
function isMCPServerCommand(command: string, args: string[]): boolean {
  const commandLower = command.toLowerCase();
  const argsLower = args.map((a) => a.toLowerCase()).join(' ');

  const patterns = [
    'mcp-server',
    '@modelcontextprotocol',
    'sequential-thinking',
    'filesystem-server',
    'github-server',
  ];

  return patterns.some((pattern) => commandLower.includes(pattern) || argsLower.includes(pattern));
}

/**
 * Check if URL is an MCP server WebSocket connection
 */
function isMCPServerURL(url: string): boolean {
  const urlLower = url.toLowerCase();
  return urlLower.includes('mcp') || urlLower.includes('model-context-protocol');
}

/**
 * Extract server name from WebSocket URL
 */
function extractServerNameFromURL(url: string): string {
  try {
    const parsed = new URL(url);
    return parsed.hostname.split('.')[0];
  } catch {
    return 'unknown-server';
  }
}

/**
 * Infer capabilities from package name
 */
function inferCapabilitiesFromPackageName(name: string): string[] {
  const capabilities: string[] = [];
  const nameLower = name.toLowerCase();

  const patterns: Record<string, string[]> = {
    filesystem: ['filesystem', 'fs', 'file'],
    database: ['sqlite', 'postgres', 'mysql', 'mongodb', 'db'],
    web: ['puppeteer', 'playwright', 'browser'],
    memory: ['memory', 'redis', 'cache', 'qdrant'],
    github: ['github', 'git'],
    sequential: ['sequential', 'thinking'],
    brave: ['brave', 'search'],
  };

  for (const [cap, keywords] of Object.entries(patterns)) {
    if (keywords.some((k) => nameLower.includes(k))) {
      capabilities.push(cap);
    }
  }

  return capabilities;
}

/**
 * Probe MCP capabilities from command and args
 */
function probeMCPCapabilities(mcp: { name: string; command: string; args: string[] }): string[] {
  const capabilities: string[] = [];

  const checks: Record<string, string[]> = {
    filesystem: ['filesystem', 'fs', '@modelcontextprotocol/server-filesystem'],
    database: ['sqlite', 'postgres', 'mongodb'],
    github: ['github', 'git'],
    sequential: ['sequential', 'thinking'],
  };

  const combined = `${mcp.name} ${mcp.command} ${mcp.args.join(' ')}`.toLowerCase();

  for (const [cap, keywords] of Object.entries(checks)) {
    if (keywords.some((k) => combined.includes(k))) {
      capabilities.push(cap);
    }
  }

  return capabilities;
}

/**
 * Find MCP config files in standard locations
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
 * Deduplicate MCPs by name
 */
function deduplicateMCPs(mcps: MCPCapability[]): MCPCapability[] {
  const seen = new Set<string>();
  const unique: MCPCapability[] = [];

  for (const mcp of mcps) {
    if (!seen.has(mcp.name)) {
      seen.add(mcp.name);
      unique.push(mcp);
    }
  }

  return unique;
}

/**
 * Check if cache is still valid
 */
function isCacheValid(cache: DetectionCache, ttl: number): boolean {
  return Date.now() - cache.detectedAt < ttl;
}

/**
 * Estimate CPU overhead based on configuration
 */
function estimateCPUOverhead(config: IntelligentDetectionConfig): number {
  let overhead = 0;

  if (config.hookModuleLoads) overhead += 0.03;
  if (config.hookChildProcesses) overhead += 0.03;
  if (config.hookWebSockets) overhead += 0.04;
  if (config.enableASTAnalysis) overhead += 0.5;
  if (config.enableDeepDependencyTree) overhead += 1.0;
  if (config.enableNetworkMonitoring) overhead += 3.0;

  return overhead;
}

/**
 * Get current memory usage in MB
 */
function getMemoryUsage(): number {
  const usage = process.memoryUsage();
  return Math.round((usage.heapUsed / 1024 / 1024) * 100) / 100;
}

/**
 * Collect runtime information
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

/**
 * Invalidate detection cache
 */
export function invalidateDetectionCache(): void {
  detectionCache = null;
}
