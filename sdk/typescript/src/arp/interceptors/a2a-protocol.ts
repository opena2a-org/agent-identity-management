import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';
import { scanText, PATTERN_SETS, type ScanResult } from '../patterns/ai-threats';

/**
 * A2A Protocol interceptor -- scans agent-to-agent messages for
 * identity spoofing, delegation abuse, and embedded prompt injection.
 *
 * Enforces a trusted agent list and validates message content.
 */
export class A2AProtocolInterceptor implements Monitor {
  readonly type: MonitorType = 'a2a-protocol';
  private readonly engine: EventEngine;
  private readonly trustedAgents: Set<string>;
  private active = false;

  constructor(engine: EventEngine, trustedAgents?: string[]) {
    this.engine = engine;
    this.trustedAgents = new Set(trustedAgents ?? []);
  }

  async start(): Promise<void> {
    this.active = true;
  }

  async stop(): Promise<void> {
    this.active = false;
  }

  isRunning(): boolean {
    return this.active;
  }

  /**
   * Scan an A2A message for identity spoofing, delegation abuse,
   * and embedded prompt injection.
   *
   * @param from - Sending agent identifier
   * @param to - Receiving agent identifier
   * @param content - Message content
   */
  scanMessage(from: string, to: string, content: string): ScanResult {
    if (!this.active) return { detected: false, matches: [] };
    const allMatches: ScanResult['matches'] = [];

    // Check trusted agent list
    if (this.trustedAgents.size > 0 && !this.trustedAgents.has(from)) {
      this.engine.emit({
        source: 'a2a-protocol',
        category: 'violation',
        severity: 'high',
        description: `A2A message from untrusted agent: ${from}`,
        data: {
          from,
          to,
          reason: 'untrusted-agent',
          trustedAgents: Array.from(this.trustedAgents),
        },
      });

      allMatches.push({
        pattern: {
          id: 'A2A-TRUST',
          category: 'a2a-attack',
          description: 'Message from untrusted agent',
          pattern: /./,
          severity: 'high',
        },
        matchedText: from,
      });
    }

    // Scan for A2A-specific attack patterns
    const a2aResult = scanText(content, PATTERN_SETS.a2aPatterns);
    if (a2aResult.detected) {
      for (const match of a2aResult.matches) {
        this.engine.emit({
          source: 'a2a-protocol',
          category: 'threat',
          severity: match.pattern.severity,
          description: `[${match.pattern.id}] ${match.pattern.description} from ${from} to ${to}`,
          data: {
            patternId: match.pattern.id,
            patternCategory: match.pattern.category,
            from,
            to,
            matchedText: match.matchedText,
          },
        });
      }
      allMatches.push(...a2aResult.matches);
    }

    // Also scan for prompt injection embedded in A2A messages
    const injectionResult = scanText(content, PATTERN_SETS.promptInjection);
    if (injectionResult.detected) {
      for (const match of injectionResult.matches) {
        this.engine.emit({
          source: 'a2a-protocol',
          category: 'threat',
          severity: match.pattern.severity,
          description: `[${match.pattern.id}] Prompt injection in A2A message from ${from} to ${to}`,
          data: {
            patternId: match.pattern.id,
            patternCategory: 'prompt-injection-via-a2a',
            from,
            to,
            matchedText: match.matchedText,
          },
        });
      }
      allMatches.push(...injectionResult.matches);
    }

    return {
      detected: allMatches.length > 0,
      matches: allMatches,
    };
  }
}
