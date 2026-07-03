import type { EnforcementAction, EnforcementResult, ARPEvent } from '../types';

export type AlertCallback = (event: ARPEvent, result: EnforcementResult) => void | Promise<void>;

/**
 * Enforcement engine — executes actions on agent processes.
 * Supports: log, alert, pause (SIGSTOP), kill (SIGTERM → SIGKILL).
 */
export class EnforcementEngine {
  private paused = new Set<number>();
  private onAlert?: AlertCallback;

  constructor(onAlert?: AlertCallback) {
    this.onAlert = onAlert;
  }

  /** Register or replace the alert callback */
  setAlertCallback(callback: AlertCallback): void {
    this.onAlert = callback;
  }

  /** Execute an enforcement action */
  async execute(action: EnforcementAction, event: ARPEvent, targetPid?: number): Promise<EnforcementResult> {
    const pid = targetPid ?? (event.data.pid as number | undefined);

    switch (action) {
      case 'log':
        return { action, success: true, reason: 'Event logged', event };

      case 'alert': {
        const result: EnforcementResult = { action, success: true, reason: `Alert raised: ${event.description}`, event };
        if (this.onAlert) {
          try { await this.onAlert(event, result); } catch { /* callback errors don't block enforcement */ }
        }
        return result;
      }

      case 'pause':
        return this.pauseProcess(pid, event);

      case 'kill':
        return this.killProcess(pid, event);

      default:
        return { action, success: false, reason: `Unknown action: ${action}`, event };
    }
  }

  /** Resume a paused process */
  resume(pid: number): boolean {
    if (!this.paused.has(pid)) return false;
    try {
      process.kill(pid, 'SIGCONT');
      this.paused.delete(pid);
      return true;
    } catch {
      this.paused.delete(pid);
      return false;
    }
  }

  /** Get list of paused PIDs */
  getPausedPids(): number[] {
    return Array.from(this.paused);
  }

  private pauseProcess(pid: number | undefined, event: ARPEvent): EnforcementResult {
    if (!pid) {
      return { action: 'pause', success: false, reason: 'No PID to pause', event };
    }

    try {
      process.kill(pid, 'SIGSTOP');
      this.paused.add(pid);
      return {
        action: 'pause',
        targetPid: pid,
        success: true,
        reason: `Paused PID ${pid}: ${event.description}`,
        event,
      };
    } catch (err) {
      return {
        action: 'pause',
        targetPid: pid,
        success: false,
        reason: `Failed to pause PID ${pid}: ${err instanceof Error ? err.message : String(err)}`,
        event,
      };
    }
  }

  private killProcess(pid: number | undefined, event: ARPEvent): EnforcementResult {
    if (!pid) {
      return { action: 'kill', success: false, reason: 'No PID to kill', event };
    }

    try {
      // A SIGSTOPped process can't process SIGTERM — the signal stays queued
      // until SIGCONT wakes it. Resume paused targets first so graceful kill works.
      if (this.paused.has(pid)) {
        try { process.kill(pid, 'SIGCONT'); } catch { /* already gone */ }
      }

      // Graceful first
      process.kill(pid, 'SIGTERM');

      // Give 5 seconds, then force kill
      setTimeout(() => {
        try {
          process.kill(pid, 0); // Check if still alive
          process.kill(pid, 'SIGKILL');
        } catch {
          // Already dead — good
        }
      }, 5000).unref();

      this.paused.delete(pid);
      return {
        action: 'kill',
        targetPid: pid,
        success: true,
        reason: `Killed PID ${pid}: ${event.description}`,
        event,
      };
    } catch (err) {
      return {
        action: 'kill',
        targetPid: pid,
        success: false,
        reason: `Failed to kill PID ${pid}: ${err instanceof Error ? err.message : String(err)}`,
        event,
      };
    }
  }
}
