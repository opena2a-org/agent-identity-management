import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';
import { scanText, PATTERN_SETS, type ScanResult } from '../patterns/ai-threats';

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

  constructor(engine: EventEngine) {
    this.engine = engine;
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
   * Scan user/agent input for prompt injection, jailbreak,
   * data exfiltration, and context manipulation attempts.
   */
  scanInput(content: string): ScanResult {
    if (!this.active) return { detected: false, matches: [] };
    const result = scanText(content, PATTERN_SETS.inputPatterns);

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

    return result;
  }

  /**
   * Scan LLM output for leaked secrets, PII, and system prompt disclosure.
   */
  scanOutput(content: string): ScanResult {
    if (!this.active) return { detected: false, matches: [] };
    const result = scanText(content, PATTERN_SETS.outputPatterns);

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

    return result;
  }
}
