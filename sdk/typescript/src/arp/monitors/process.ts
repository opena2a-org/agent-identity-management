import { spawnSync } from 'child_process';
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

/** Arguments of the one `ps` call that reads the process table. `command=` is
 *  last because it holds spaces. Valid for both macOS and Linux (procps) `ps`. */
const LISTING_ARGS = ['-ax', '-o', 'pid=,ppid=,user=,%cpu=,%mem=,command='];

/** The command line `ps` prints in the row for the listing process itself. */
const LISTING_COMMAND = ['ps', ...LISTING_ARGS].join(' ');

/** One row of the listing: pid, ppid, user, %cpu, %mem, then the command. */
const LISTING_ROW = /^\s*(\d+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+?)\s*$/;

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
    const table = this.readProcessTable();
    this.knownPids = new Set(table ? this.getDescendants(table).map((info) => info.pid) : []);

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
      const table = this.readProcessTable();
      if (!table) return; // ps command failed — skip this cycle
      const current = this.getDescendants(table);
      const currentSet = new Set(current.map((info) => info.pid));

      // Detect new child processes
      for (const info of current) {
        const pid = info.pid;
        if (!this.knownPids.has(pid)) {
          this.engine.emit({
            source: 'process',
            category: 'normal',
            severity: 'info',
            description: `New child process: PID ${pid} — ${info.command.slice(0, 100)}`,
            data: { pid, command: info.command, user: info.user, cpu: info.cpu, mem: info.mem },
          });
        }
      }

      // Detect terminated processes: only pids present at start() or announced above
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
      for (const info of current) {
        const pid = info.pid;

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
      // skip this cycle
    }
  }

  /** Read the whole process table with one `ps` call, run without a shell, and
   *  parse it into rows. That `ps` is a child of the agent while it prints, so
   *  its own row is in the listing: it is left out, matched by the pid
   *  spawnSync reports, the agent as parent and its exact command line.
   *  Returns null when `ps` fails. */
  private readProcessTable(): ProcessInfo[] | null {
    const result = spawnSync('ps', LISTING_ARGS, {
      encoding: 'utf-8',
      timeout: 5000,
      // The command column makes the listing large; the 1 MiB default cuts it off on a busy host.
      maxBuffer: 64 * 1024 * 1024,
    });
    if (result.error || result.status !== 0 || typeof result.stdout !== 'string') return null;

    const rows: ProcessInfo[] = [];
    for (const line of result.stdout.split('\n')) {
      const match = LISTING_ROW.exec(line);
      if (!match) continue;
      const row: ProcessInfo = {
        pid: parseInt(match[1]),
        ppid: parseInt(match[2]),
        user: match[3],
        cpu: parseFloat(match[4]),
        mem: parseFloat(match[5]),
        command: match[6],
      };
      if (row.pid === result.pid && row.ppid === this.agentPid && row.command === LISTING_COMMAND) continue;
      rows.push(row);
    }
    return rows;
  }

  /** Walk the process tree in `table` to find all descendants of the agent, breadth first. */
  private getDescendants(table: ProcessInfo[]): ProcessInfo[] {
    if (!this.agentPid) return [];
    const childMap = new Map<number, ProcessInfo[]>();
    for (const row of table) {
      if (!childMap.has(row.ppid)) childMap.set(row.ppid, []);
      childMap.get(row.ppid)!.push(row);
    }

    const result: ProcessInfo[] = [];
    const queue = [this.agentPid];
    while (queue.length > 0) {
      const current = queue.shift()!;
      for (const child of childMap.get(current) ?? []) {
        result.push(child);
        queue.push(child.pid);
      }
    }
    return result;
  }
}
