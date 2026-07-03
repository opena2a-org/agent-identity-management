import * as fs from 'fs';
import * as path from 'path';
import type { ARPEvent, EnforcementResult } from '../types';

const EVENT_LOG = 'events.jsonl';
const ENFORCEMENT_LOG = 'enforcement.jsonl';
const MAX_LOG_SIZE = 50 * 1024 * 1024; // 50MB

/**
 * Local JSONL logger — append-only event and enforcement logs.
 * Follows the aim-core audit.jsonl pattern.
 */
export class LocalLogger {
  private readonly dataDir: string;

  constructor(dataDir: string) {
    this.dataDir = dataDir;
    fs.mkdirSync(dataDir, { recursive: true });
  }

  /** Log an ARP event */
  logEvent(event: ARPEvent): void {
    this.appendLog(EVENT_LOG, event);
  }

  /** Log an enforcement action */
  logEnforcement(result: EnforcementResult): void {
    const entry = {
      timestamp: new Date().toISOString(),
      action: result.action,
      targetPid: result.targetPid,
      success: result.success,
      reason: result.reason,
      eventId: result.event.id,
      eventSource: result.event.source,
      eventSeverity: result.event.severity,
    };
    this.appendLog(ENFORCEMENT_LOG, entry);
  }

  /** Read recent events */
  readEvents(limit?: number): ARPEvent[] {
    return this.readLog<ARPEvent>(EVENT_LOG, limit);
  }

  /** Read recent enforcement actions */
  readEnforcements(limit?: number): EnforcementResult[] {
    return this.readLog<EnforcementResult>(ENFORCEMENT_LOG, limit);
  }

  /** Tail the event log (returns last N lines as JSON) */
  tail(n: number = 20): ARPEvent[] {
    return this.readLog<ARPEvent>(EVENT_LOG, n);
  }

  private appendLog(filename: string, data: unknown): void {
    const filePath = path.join(this.dataDir, filename);

    // Rotate if needed
    try {
      const stat = fs.statSync(filePath);
      if (stat.size > MAX_LOG_SIZE) {
        const rotatedPath = `${filePath}.${Date.now()}`;
        fs.renameSync(filePath, rotatedPath);
      }
    } catch {
      // File doesn't exist yet
    }

    const line = JSON.stringify(data) + '\n';
    fs.appendFileSync(filePath, line, 'utf-8');
  }

  private readLog<T>(filename: string, limit?: number): T[] {
    const filePath = path.join(this.dataDir, filename);
    if (!fs.existsSync(filePath)) return [];

    try {
      const content = fs.readFileSync(filePath, 'utf-8');
      const lines = content.trim().split('\n').filter(Boolean);
      const entries = lines.map((l) => JSON.parse(l) as T);

      if (limit && limit > 0) {
        return entries.slice(-limit);
      }
      return entries;
    } catch {
      return [];
    }
  }
}
