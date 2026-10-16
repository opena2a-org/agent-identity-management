import type { EnforcementAction, EnforcementExecutor, EnforcementResult, ARPEvent } from '../types';

export type AlertCallback = (event: ARPEvent, result: EnforcementResult) => void | Promise<void>;

/**
 * The PID a pause or kill may signal: a positive safe integer. `process.kill`
 * reads 0 as the caller's process group and -1 as every process the user may
 * signal, so anything else is treated as no target.
 */
function signalTarget(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0 ? value : undefined;
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/**
 * Enforcement engine — executes actions on agent processes.
 * Supports: log, alert, pause (SIGSTOP), kill (SIGTERM → SIGKILL).
 */
export class EnforcementEngine implements EnforcementExecutor {
  private paused = new Set<number>();
  private onAlert?: AlertCallback;

  constructor(onAlert?: AlertCallback) {
    this.onAlert = onAlert;
  }

  /** Register or replace the alert callback */
  setAlertCallback(callback: AlertCallback): void {
    this.onAlert = callback;
  }

  /**
   * Execute an enforcement action and return its outcome. A pause or kill
   * signals `targetPid`, or else `event.data.pid`, only when that value is a
   * positive integer; otherwise it returns `outcome: 'no-target'`.
   */
  async execute(action: EnforcementAction, event: ARPEvent, targetPid?: number): Promise<EnforcementResult> {
    const pid = signalTarget(targetPid ?? event.data.pid);

    switch (action) {
      case 'log':
        return { action, success: true, outcome: 'executed', reason: 'Event logged', event };

      case 'alert': {
        const reason = `Alert raised: ${event.description}`;
        const result: EnforcementResult = { action, success: true, outcome: 'executed', reason, event };
        if (this.onAlert) {
          // A throwing callback does not stop enforcement; it makes the alert failed.
          try {
            await this.onAlert(event, result);
          } catch (err) {
            return {
              action,
              success: false,
              outcome: 'failed',
              reason: `${reason}; the alert callback threw: ${errorMessage(err)}`,
              event,
            };
          }
        }
        return result;
      }

      case 'pause':
        return this.pauseProcess(pid, event);

      case 'kill':
        return this.killProcess(pid, event);

      default:
        return { action, success: false, outcome: 'not-executed', reason: `Unknown action: ${action}`, event };
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
    if (pid === undefined) {
      return { action: 'pause', success: false, outcome: 'no-target', reason: 'No PID to pause', event };
    }

    try {
      process.kill(pid, 'SIGSTOP');
      this.paused.add(pid);
      return {
        action: 'pause',
        targetPid: pid,
        success: true,
        outcome: 'executed',
        reason: `Paused PID ${pid}: ${event.description}`,
        event,
      };
    } catch (err) {
      return {
        action: 'pause',
        targetPid: pid,
        success: false,
        outcome: 'failed',
        reason: `Failed to pause PID ${pid}: ${errorMessage(err)}`,
        event,
      };
    }
  }

  private killProcess(pid: number | undefined, event: ARPEvent): EnforcementResult {
    if (pid === undefined) {
      return { action: 'kill', success: false, outcome: 'no-target', reason: 'No PID to kill', event };
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
        outcome: 'executed',
        reason: `Killed PID ${pid}: ${event.description}`,
        event,
      };
    } catch (err) {
      return {
        action: 'kill',
        targetPid: pid,
        success: false,
        outcome: 'failed',
        reason: `Failed to kill PID ${pid}: ${errorMessage(err)}`,
        event,
      };
    }
  }
}
