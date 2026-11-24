import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';
import { scanText, PATTERN_SETS, MAX_SCAN_TOTAL, type ScanResult } from '../patterns/ai-threats';
import { warnNotStarted } from './not-started';
import { emitTruncation } from './truncation';

/**
 * Prompt interceptor -- scans user messages and LLM responses for
 * injection, jailbreak, data exfiltration, and output leak patterns.
 *
 * Operates at the AI/semantic layer (L0 regex detection).
 * Designed to be called by the HTTP proxy or directly via SDK integration.
 */
export class PromptInterceptor implements Monitor {
  readonly type: MonitorType = 'prompt';
  private readonly engine: EventEngine;
  private active = false;
  /** Whether the not-started warning has been printed since the last start(). */
  private warnedNotStarted = false;

  constructor(engine: EventEngine) {
    this.engine = engine;
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
    warnNotStarted('PromptInterceptor', method);
  }

  /**
   * Scan user/agent input for prompt injection, jailbreak,
   * data exfiltration, and context manipulation attempts.
   */
  scanInput(content: string): ScanResult {
    if (!this.active) {
      this.warnIfNotStarted('scanInput');
      return { detected: false, matches: [], scanned: false, scope: { set: 'input', patternIds: [] } };
    }
    const result = scanText(content, PATTERN_SETS.inputPatterns, 'input');

    if (result.detected) {
      for (const match of result.matches) {
        this.engine.emit({
          source: 'prompt',
          category: 'threat',
          severity: match.pattern.severity,
          description: `[${match.pattern.id}] ${match.pattern.description}`,
          data: {
            patternId: match.pattern.id,
            patternCategory: match.pattern.category,
            matchedText: match.matchedText,
            direction: 'input',
            contentLength: content.length,
          },
        });
      }
    }

    if (result.truncated) {
      emitTruncation(this.engine, 'prompt', 'PromptInterceptor.scanInput', {
        reason: 'truncated',
        matchedLength: MAX_SCAN_TOTAL,
        receivedLength: content.length,
        unit: 'characters',
      });
    }

    return result;
  }

  /**
   * Scan LLM output for leaked secrets, PII, and system prompt disclosure.
   */
  scanOutput(content: string): ScanResult {
    if (!this.active) {
      this.warnIfNotStarted('scanOutput');
      return { detected: false, matches: [], scanned: false, scope: { set: 'output', patternIds: [] } };
    }
    const result = scanText(content, PATTERN_SETS.outputPatterns, 'output');

    if (result.detected) {
      for (const match of result.matches) {
        this.engine.emit({
          source: 'prompt',
          category: 'threat',
          severity: match.pattern.severity,
          description: `[${match.pattern.id}] ${match.pattern.description}`,
          data: {
            patternId: match.pattern.id,
            patternCategory: match.pattern.category,
            matchedText: match.matchedText,
            direction: 'output',
            contentLength: content.length,
          },
        });
      }
    }

    if (result.truncated) {
      emitTruncation(this.engine, 'prompt', 'PromptInterceptor.scanOutput', {
        reason: 'truncated',
        matchedLength: MAX_SCAN_TOTAL,
        receivedLength: content.length,
        unit: 'characters',
      });
    }

    return result;
  }
}
