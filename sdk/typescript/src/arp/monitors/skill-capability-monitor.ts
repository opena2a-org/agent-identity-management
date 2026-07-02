/**
 * Skill Capability Monitor
 *
 * Tracks runtime behavior of skills and compares against declared capabilities.
 * Generates findings when skills exceed their declared permissions:
 * - SKILL-013: Permission overreach (action not declared in capabilities)
 * - SKILL-014: Undeclared network access (network call not listed in permissions)
 */

import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';

// --- Types ---

export interface DeclaredCapabilities {
  /** Skill name */
  name: string;
  /** Declared filesystem paths the skill is allowed to access */
  fileAccess: string[];
  /** Declared network hosts/URLs the skill is allowed to contact */
  networkAccess: string[];
  /** Declared tools the skill is allowed to invoke */
  tools: string[];
  /** Declared credential scopes the skill may use */
  credentials: string[];
}

export interface ObservedBehavior {
  /** Filesystem paths accessed at runtime */
  fileAccesses: Array<{ path: string; operation: string; timestamp: string }>;
  /** Network calls made at runtime */
  networkCalls: Array<{ host: string; port?: number; protocol?: string; timestamp: string }>;
  /** Tools invoked at runtime */
  toolUses: Array<{ tool: string; timestamp: string }>;
  /** Credentials accessed at runtime */
  credentialAccesses: Array<{ scope: string; timestamp: string }>;
}

export interface CapabilityViolation {
  /** Finding ID (SKILL-013 or SKILL-014) */
  id: string;
  /** Type of violation */
  type: 'permission-overreach' | 'undeclared-network';
  /** Description of the violation */
  description: string;
  /** What was observed */
  observed: string;
  /** Timestamp of the violation */
  timestamp: string;
}

// --- Capability Parsing ---

/**
 * Parse declared capabilities from SKILL.md content.
 * Extracts permissions, network access, tools, and credential scopes from YAML frontmatter.
 */
export function parseDeclaredCapabilities(skillMd: string): DeclaredCapabilities {
  const capabilities: DeclaredCapabilities = {
    name: '',
    fileAccess: [],
    networkAccess: [],
    tools: [],
    credentials: [],
  };

  // Extract frontmatter
  const frontmatterMatch = skillMd.match(/^---\s*\n([\s\S]*?)\n---/);
  if (!frontmatterMatch) return capabilities;

  const frontmatter = frontmatterMatch[1];

  // Parse name
  const nameMatch = frontmatter.match(/^name:\s*(.+)$/m);
  if (nameMatch) {
    capabilities.name = nameMatch[1].trim().replace(/^["']|["']$/g, '');
  }

  // Parse permissions/capabilities sections (try primary field name, fall back to alternate)
  const fileAccess = parseYamlList(frontmatter, 'file_access');
  capabilities.fileAccess = fileAccess.length > 0 ? fileAccess : parseYamlList(frontmatter, 'filesystem');
  const networkAccess = parseYamlList(frontmatter, 'network_access');
  capabilities.networkAccess = networkAccess.length > 0 ? networkAccess : parseYamlList(frontmatter, 'network');
  capabilities.tools = parseYamlList(frontmatter, 'tools');
  const credentials = parseYamlList(frontmatter, 'credentials');
  capabilities.credentials = credentials.length > 0 ? credentials : parseYamlList(frontmatter, 'credential_scopes');

  return capabilities;
}

/**
 * Parse a simple YAML list from frontmatter.
 */
function parseYamlList(frontmatter: string, field: string): string[] {
  // Inline format: field: [a, b, c]
  const inlineMatch = frontmatter.match(new RegExp(`^${field}:\\s*\\[([^\\]]*)]`, 'm'));
  if (inlineMatch) {
    return inlineMatch[1]
      .split(',')
      .map((item) => item.trim().replace(/^["']|["']$/g, ''))
      .filter((item) => item.length > 0);
  }

  // Block format
  const blockMatch = frontmatter.match(new RegExp(`^${field}:\\s*\\n((?:\\s+-\\s+.+\\n?)*)`, 'm'));
  if (blockMatch) {
    return blockMatch[1]
      .split('\n')
      .map((line) => {
        const itemMatch = line.match(/^\s+-\s+(.+)/);
        return itemMatch ? itemMatch[1].trim().replace(/^["']|["']$/g, '') : '';
      })
      .filter((item) => item.length > 0);
  }

  return [];
}

// --- Monitor ---

/**
 * Create a skill capability monitor that tracks runtime behavior
 * and compares it against declared capabilities.
 */
export function createCapabilityMonitor(declared: DeclaredCapabilities): SkillCapabilityMonitor {
  return new SkillCapabilityMonitor(declared);
}

/**
 * Skill Capability Monitor
 *
 * Records runtime actions and detects capability violations.
 * Can operate standalone or integrated with the ARP event engine.
 */
export class SkillCapabilityMonitor implements Monitor {
  readonly type: MonitorType = 'skill';
  private readonly declared: DeclaredCapabilities;
  private readonly observed: ObservedBehavior;
  private readonly violations: CapabilityViolation[] = [];
  private engine: EventEngine | null = null;
  private running = false;

  constructor(declared: DeclaredCapabilities, engine?: EventEngine) {
    this.declared = declared;
    this.engine = engine ?? null;
    this.observed = {
      fileAccesses: [],
      networkCalls: [],
      toolUses: [],
      credentialAccesses: [],
    };
  }

  async start(): Promise<void> {
    this.running = true;
  }

  async stop(): Promise<void> {
    this.running = false;
  }

  isRunning(): boolean {
    return this.running;
  }

  /**
   * Record a file access event and check against declared capabilities.
   */
  recordFileAccess(filePath: string, operation: string = 'read'): void {
    const timestamp = new Date().toISOString();
    this.observed.fileAccesses.push({ path: filePath, operation, timestamp });

    const isAllowed = this.isFileAccessAllowed(filePath);
    if (!isAllowed) {
      const violation: CapabilityViolation = {
        id: 'SKILL-013',
        type: 'permission-overreach',
        description: `Skill "${this.declared.name}" accessed file "${filePath}" (${operation}) which is not in declared file_access permissions`,
        observed: filePath,
        timestamp,
      };
      this.violations.push(violation);
      this.emitViolation(violation);
    }
  }

  /**
   * Record a network call event and check against declared capabilities.
   */
  recordNetworkCall(host: string, port?: number, protocol?: string): void {
    const timestamp = new Date().toISOString();
    this.observed.networkCalls.push({ host, port, protocol, timestamp });

    const isAllowed = this.isNetworkAccessAllowed(host);
    if (!isAllowed) {
      const violation: CapabilityViolation = {
        id: 'SKILL-014',
        type: 'undeclared-network',
        description: `Skill "${this.declared.name}" made undeclared network call to "${host}${port ? ':' + port : ''}"`,
        observed: host,
        timestamp,
      };
      this.violations.push(violation);
      this.emitViolation(violation);
    }
  }

  /**
   * Record a tool invocation and check against declared capabilities.
   */
  recordToolUse(tool: string): void {
    const timestamp = new Date().toISOString();
    this.observed.toolUses.push({ tool, timestamp });

    const isAllowed = this.declared.tools.some((t) =>
      t === tool || t === '*'
    );
    if (!isAllowed) {
      const violation: CapabilityViolation = {
        id: 'SKILL-013',
        type: 'permission-overreach',
        description: `Skill "${this.declared.name}" invoked tool "${tool}" which is not in declared tools`,
        observed: tool,
        timestamp,
      };
      this.violations.push(violation);
      this.emitViolation(violation);
    }
  }

  /**
   * Record a credential access and check against declared capabilities.
   */
  recordCredentialAccess(scope: string): void {
    const timestamp = new Date().toISOString();
    this.observed.credentialAccesses.push({ scope, timestamp });

    const isAllowed = this.declared.credentials.some((c) =>
      c === scope || c === '*'
    );
    if (!isAllowed) {
      const violation: CapabilityViolation = {
        id: 'SKILL-013',
        type: 'permission-overreach',
        description: `Skill "${this.declared.name}" accessed credential scope "${scope}" which is not in declared credentials`,
        observed: scope,
        timestamp,
      };
      this.violations.push(violation);
      this.emitViolation(violation);
    }
  }

  /**
   * Get all recorded violations as findings.
   */
  getViolations(): CapabilityViolation[] {
    return [...this.violations];
  }

  /**
   * Get observed behavior summary.
   */
  getObserved(): ObservedBehavior {
    return {
      fileAccesses: [...this.observed.fileAccesses],
      networkCalls: [...this.observed.networkCalls],
      toolUses: [...this.observed.toolUses],
      credentialAccesses: [...this.observed.credentialAccesses],
    };
  }

  /**
   * Reset all recorded observations and violations.
   */
  reset(): void {
    this.observed.fileAccesses.length = 0;
    this.observed.networkCalls.length = 0;
    this.observed.toolUses.length = 0;
    this.observed.credentialAccesses.length = 0;
    this.violations.length = 0;
  }

  // --- Private Helpers ---

  private isFileAccessAllowed(filePath: string): boolean {
    if (this.declared.fileAccess.length === 0) return false;

    return this.declared.fileAccess.some((allowed) => {
      if (allowed === '*') return true;
      // Support glob-like prefix matching
      if (allowed.endsWith('/*') || allowed.endsWith('/**')) {
        const prefix = allowed.replace(/\/\*+$/, '');
        return filePath.startsWith(prefix);
      }
      return filePath === allowed || filePath.startsWith(allowed + '/');
    });
  }

  private isNetworkAccessAllowed(host: string): boolean {
    if (this.declared.networkAccess.length === 0) return false;

    return this.declared.networkAccess.some((allowed) => {
      if (allowed === '*') return true;
      // Exact match or subdomain match
      return host === allowed || host.endsWith('.' + allowed);
    });
  }

  private emitViolation(violation: CapabilityViolation): void {
    if (!this.engine) return;

    const severity = violation.id === 'SKILL-014' ? 'high' : 'medium';
    const category = violation.id === 'SKILL-014' ? 'violation' : 'anomaly';

    this.engine.emit({
      source: 'skill',
      category,
      severity,
      description: violation.description,
      data: {
        violationId: violation.id,
        violationType: violation.type,
        skillName: this.declared.name,
        observed: violation.observed,
      },
    });
  }
}
