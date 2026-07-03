import { execSync } from 'child_process';
import * as os from 'os';
import * as path from 'path';
import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';

interface ProcessInfo {
  pid: number;
  ppid: number;
  user: string;
  command: string;
  cpu: number;
  mem: number;
}

/** Binaries commonly used for exfiltration, lateral movement, or exploitation */
const SUSPICIOUS_BINARIES = [
  'curl', 'wget', 'nc', 'ncat', 'nmap', 'ssh', 'scp',
  'python', 'python3', 'perl', 'ruby', 'base64',
  'socat', 'telnet', 'ftp', 'rsync',
];

/**
 * Process monitor — tracks agent lifecycle, child processes, and resource usage.
 * Uses `ps` polling (cross-platform, no root required).
 */
export class ProcessMonitor implements Monitor {
  readonly type: MonitorType = 'process';
  private timer?: ReturnType<typeof setInterval>;
  private readonly engine: EventEngine;
  private readonly intervalMs: number;
  private knownPids = new Set<number>();
  private agentPid?: number;

  constructor(engine: EventEngine, intervalMs: number = 5000) {
    this.engine = engine;
    this.intervalMs = intervalMs;
  }

  async start(): Promise<void> {
    this.agentPid = process.pid; // Monitor children of the current (agent) process
    this.knownPids = new Set(this.getDescendantPids(this.agentPid));

    this.timer = setInterval(() => this.poll(), this.intervalMs);
    if (this.timer.unref) this.timer.unref();
  }

  async stop(): Promise<void> {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  isRunning(): boolean {
    return this.timer !== undefined;
  }

  private poll(): void {
    try {
      const currentPids = this.getDescendantPids(this.agentPid);
      const currentSet = new Set(currentPids);

      // Detect new child processes
      for (const pid of currentPids) {
        if (!this.knownPids.has(pid)) {
          const info = this.getProcessInfo(pid);
          if (info) {
            this.engine.emit({
              source: 'process',
              category: 'normal',
              severity: 'info',
              description: `New child process: PID ${pid} — ${info.command.slice(0, 100)}`,
              data: { pid, command: info.command, user: info.user, cpu: info.cpu, mem: info.mem },
            });
          }
        }
      }

      // Detect terminated processes
      for (const pid of this.knownPids) {
        if (!currentSet.has(pid)) {
          this.engine.emit({
            source: 'process',
            category: 'normal',
            severity: 'info',
            description: `Child process terminated: PID ${pid}`,
            data: { pid, action: 'terminated' },
          });
        }
      }

      // Check for suspicious processes (binaries, high CPU, unexpected users)
      for (const pid of currentPids) {
        const info = this.getProcessInfo(pid);
        if (!info) continue;

        // Suspicious binary detection
        const binaryName = path.basename(info.command.split(/\s+/)[0]);
        if (SUSPICIOUS_BINARIES.includes(binaryName)) {
          this.engine.emit({
            source: 'process',
            category: 'violation',
            severity: 'high',
            description: `Suspicious binary executed: ${binaryName} (PID ${pid})`,
            data: { pid, binary: binaryName, command: info.command, user: info.user },
          });
        }

        // High CPU for extended period
        if (info.cpu > 90) {
          this.engine.emit({
            source: 'process',
            category: 'anomaly',
            severity: 'medium',
            description: `High CPU usage: PID ${pid} at ${info.cpu}% — ${info.command.slice(0, 60)}`,
            data: { pid, cpu: info.cpu, command: info.command },
          });
        }

        // Running as different user
        if (info.user === 'root' && os.userInfo().username !== 'root') {
          this.engine.emit({
            source: 'process',
            category: 'violation',
            severity: 'high',
            description: `Child process running as root: PID ${pid} — ${info.command.slice(0, 60)}`,
            data: { pid, user: info.user, command: info.command },
          });
        }
      }

      this.knownPids = currentSet;
    } catch {
      // ps command failed — skip this cycle
    }
  }

  /** Walk the full process tree to find all descendants of parentPid.
   *  Uses `ps -ax -o pid=,ppid=` which works on both macOS and Linux. */
  private getDescendantPids(parentPid?: number): number[] {
    if (!parentPid) return [];
    try {
      const output = execSync('ps -ax -o pid=,ppid=', { encoding: 'utf-8', timeout: 5000 });
      const childMap = new Map<number, number[]>();

      for (const line of output.trim().split('\n')) {
        const parts = line.trim().split(/\s+/);
        const pid = parseInt(parts[0]);
        const ppid = parseInt(parts[1]);
        if (isNaN(pid) || isNaN(ppid)) continue;
        if (!childMap.has(ppid)) childMap.set(ppid, []);
        childMap.get(ppid)!.push(pid);
      }

      // BFS from parentPid
      const result: number[] = [];
      const queue = [parentPid];
      while (queue.length > 0) {
        const current = queue.shift()!;
        for (const child of childMap.get(current) ?? []) {
          result.push(child);
          queue.push(child);
        }
      }

      return result;
    } catch {
      return [];
    }
  }

  private getProcessInfo(pid: number): ProcessInfo | null {
    try {
      const output = execSync(
        `ps -o pid=,ppid=,user=,%cpu=,%mem=,command= -p ${pid}`,
        { encoding: 'utf-8', timeout: 5000 },
      );
      const line = output.trim();
      if (!line) return null;

      const parts = line.trim().split(/\s+/);
      if (parts.length < 6) return null;

      return {
        pid: parseInt(parts[0]),
        ppid: parseInt(parts[1]),
        user: parts[2],
        cpu: parseFloat(parts[3]),
        mem: parseFloat(parts[4]),
        command: parts.slice(5).join(' '),
      };
    } catch {
      return null;
    }
  }
}
