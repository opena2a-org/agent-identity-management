import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';
import { scanText, PATTERN_SETS, type ScanResult } from '../patterns/ai-threats';
import { warnNotStarted } from './not-started';

/**
 * MCP Protocol interceptor -- scans MCP tool calls for path traversal,
 * command injection, and SSRF exploitation patterns.
 *
 * Enforces tool allowlists and validates parameters at the protocol level.
 */
export class MCPProtocolInterceptor implements Monitor {
  readonly type: MonitorType = 'mcp-protocol';
  private readonly engine: EventEngine;
  private readonly allowedTools: Set<string>;
  private active = false;
  /** Whether the not-started warning has been printed since the last start(). */
  private warnedNotStarted = false;

  constructor(engine: EventEngine, allowedTools?: string[]) {
    this.engine = engine;
    this.allowedTools = new Set(allowedTools ?? []);
  }

  async start(): Promise<void> {
    this.active = true;
    this.warnedNotStarted = false;
  }

  async stop(): Promise<void> {
    this.active = false;
  }

  isRunning(): boolean {
    return this.active;
  }

  /** Print the not-started warning once until the next start(). */
  private warnIfNotStarted(method: string): void {
    if (this.warnedNotStarted) return;
    this.warnedNotStarted = true;
    warnNotStarted('MCPProtocolInterceptor', method);
  }

  /**
   * Scan an MCP tool call for exploitation patterns.
   * Checks tool name against allowlist and scans all parameter values.
   */
  scanToolCall(toolName: string, args: Record<string, unknown>): ScanResult {
    if (!this.active) {
      this.warnIfNotStarted('scanToolCall');
      return { detected: false, matches: [], scanned: false, scope: { set: 'mcp', patternIds: [] } };
    }
    const checkedIds = this.allowedTools.size > 0 ? ['MCP-ALLOWLIST'] : [];
    // Check tool allowlist
    if (this.allowedTools.size > 0 && !this.allowedTools.has(toolName)) {
      this.engine.emit({
        source: 'mcp-protocol',
        category: 'violation',
        severity: 'high',
        description: `MCP tool not in allowlist: ${toolName}`,
        data: {
          toolName,
          reason: 'not-in-allowlist',
          allowedTools: Array.from(this.allowedTools),
        },
      });

      return {
        detected: true,
        matches: [{
          pattern: {
            id: 'MCP-ALLOWLIST',
            category: 'mcp-exploitation',
            description: 'Tool not in allowlist',
            pattern: /./,
            severity: 'high',
          },
          matchedText: toolName,
        }],
        // The allowlist check ran; the arguments were not scanned, so no MCP-* pattern id is listed
        scanned: true,
        scope: { set: 'mcp', patternIds: checkedIds },
      };
    }

    // Scan each top-level argument on its own, so that one long argument
    // cannot use up the scan total of the others, and text from two
    // arguments is never joined into one match.
    const matches: ScanResult['matches'] = [];
    let truncated = false;
    for (const [argument, value] of Object.entries(args)) {
      const argResult = scanText(flattenValue(value), PATTERN_SETS.mcpPatterns, 'mcp');
      truncated ||= Boolean(argResult.truncated);
      for (const match of argResult.matches) {
        matches.push({ ...match, argument });
      }
    }

    for (const match of matches) {
      this.engine.emit({
        source: 'mcp-protocol',
        category: 'threat',
        severity: match.pattern.severity,
        description: `[${match.pattern.id}] ${match.pattern.description} in tool "${toolName}"`,
        data: {
          patternId: match.pattern.id,
          patternCategory: match.pattern.category,
          toolName,
          argument: match.argument,
          matchedText: match.matchedText,
          args: sanitizeArgs(args),
        },
      });
    }

    return {
      detected: matches.length > 0,
      matches,
      truncated,
      scanned: true,
      scope: { set: 'mcp', patternIds: [...checkedIds, ...PATTERN_SETS.mcpPatterns.map((p) => p.id)] },
    };
  }
}

/** Flatten one argument value into a string for scanning */
function flattenValue(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (typeof value === 'object' && value !== null) return flattenArgs({ value });
  return '';
}

/** Recursively flatten object values into a single string for scanning */
function flattenArgs(obj: Record<string, unknown>, depth = 0): string {
  if (depth > 10) return '';
  const parts: string[] = [];

  for (const value of Object.values(obj)) {
    if (typeof value === 'string') {
      parts.push(value);
    } else if (typeof value === 'number' || typeof value === 'boolean') {
      parts.push(String(value));
    } else if (Array.isArray(value)) {
      for (const item of value) {
        if (typeof item === 'string') {
          parts.push(item);
        } else if (typeof item === 'object' && item !== null) {
          parts.push(flattenArgs(item as Record<string, unknown>, depth + 1));
        }
      }
    } else if (typeof value === 'object' && value !== null) {
      parts.push(flattenArgs(value as Record<string, unknown>, depth + 1));
    }
  }

  return parts.join('\n');
}

/** Sanitize args for logging (truncate long strings) */
function sanitizeArgs(args: Record<string, unknown>): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(args)) {
    if (typeof value === 'string' && value.length > 200) {
      result[key] = value.slice(0, 200) + '...[truncated]';
    } else {
      result[key] = value;
    }
  }
  return result;
}
