import * as path from 'path';
import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';
import { nodeRequire } from '../node-require';

/** Binaries commonly used for exfiltration, lateral movement, or exploitation */
const SUSPICIOUS_BINARIES = [
  'curl', 'wget', 'nc', 'ncat', 'nmap', 'ssh', 'scp',
  'python', 'python3', 'perl', 'ruby', 'base64',
  'socat', 'telnet', 'ftp', 'rsync',
];

/**
 * CR-001: Command complexity analysis for adversarial bash pipeline detection.
 *
 * Counts shell operators (pipes, semicolons, command substitutions, etc.)
 * to detect adversarial command chains that attempt to exceed enforcement
 * thresholds. Any command exceeding MAX_SUBCOMMANDS is treated as a threat.
 *
 * NOTE: tree-sitter-bash (v0.25.1) was evaluated for full AST parsing but
 * requires native compilation (node-gyp). This regex-based approach covers
 * the adversarial pipeline attack vector without the native dependency.
 * tree-sitter-bash remains a candidate for future upgrade if the native
 * dep becomes acceptable.
 */
const MAX_SUBCOMMANDS = 10;

/** Shell metacharacters that separate commands */
const SHELL_OPERATORS = /[|;&]|\$\(|`/g;

function analyzeCommandComplexity(command: string): { subcommandCount: number; isAdversarial: boolean } {
  const operators = command.match(SHELL_OPERATORS);
  const subcommandCount = (operators?.length ?? 0) + 1;
  return {
    subcommandCount,
    isAdversarial: subcommandCount > MAX_SUBCOMMANDS,
  };
}

/**
 * Process interceptor — hooks child_process.spawn/exec/execFile/fork to
 * intercept ALL process creation at the application level.
 *
 * Advantages over ps polling:
 * - Zero latency: events fire before the process is spawned
 * - 100% accuracy: catches every spawn, even short-lived processes
 * - No system tool dependency: works in sandboxed environments
 * - Full argument visibility: sees the exact command and args
 */
export class ProcessInterceptor implements Monitor {
  readonly type: MonitorType = 'process';
  private readonly engine: EventEngine;
  // Use require() to get the mutable CJS module (ESM namespaces are frozen)
  private readonly cpModule: Record<string, Function>;
  private originals: Record<string, Function> | null = null;
  private active = false;

  constructor(engine: EventEngine) {
    this.engine = engine;
    this.cpModule = nodeRequire('child_process');
  }

  async start(): Promise<void> {
    if (this.active) return;

    const self = this;
    const mod = this.cpModule;
    this.originals = {};
    for (const fn of ['spawn', 'spawnSync', 'exec', 'execSync', 'execFile', 'execFileSync', 'fork']) {
      this.originals[fn] = mod[fn];
    }

    const orig = this.originals;

    mod.spawn = function (...args: unknown[]) {
      self.handleSpawn(args[0] as string, Array.isArray(args[1]) ? args[1] as string[] : []);
      return orig.spawn.apply(mod, args);
    };

    mod.spawnSync = function (...args: unknown[]) {
      self.handleSpawn(args[0] as string, Array.isArray(args[1]) ? args[1] as string[] : []);
      return orig.spawnSync.apply(mod, args);
    };

    mod.exec = function (...args: unknown[]) {
      self.handleExec(args[0] as string);
      return orig.exec.apply(mod, args);
    };

    mod.execSync = function (...args: unknown[]) {
      self.handleExec(args[0] as string);
      return orig.execSync.apply(mod, args);
    };

    mod.execFile = function (...args: unknown[]) {
      self.handleSpawn(args[0] as string, Array.isArray(args[1]) ? args[1] as string[] : []);
      return orig.execFile.apply(mod, args);
    };

    mod.execFileSync = function (...args: unknown[]) {
      self.handleSpawn(args[0] as string, Array.isArray(args[1]) ? args[1] as string[] : []);
      return orig.execFileSync.apply(mod, args);
    };

    mod.fork = function (...args: unknown[]) {
      self.handleSpawn('node', [args[0] as string, ...((args[1] as string[]) ?? [])]);
      return orig.fork.apply(mod, args);
    };

    this.active = true;
  }

  async stop(): Promise<void> {
    if (!this.active || !this.originals) return;

    const mod = this.cpModule;
    for (const [name, original] of Object.entries(this.originals)) {
      mod[name] = original;
    }

    this.originals = null;
    this.active = false;
  }

  isRunning(): boolean {
    return this.active;
  }

  private handleSpawn(command: string, args: string[]): void {
    const binary = path.basename(command);
    const fullCommand = [command, ...args].join(' ');
    const isSuspicious = SUSPICIOUS_BINARIES.includes(binary);

    this.engine.emit({
      source: 'process',
      category: isSuspicious ? 'violation' : 'normal',
      severity: isSuspicious ? 'high' : 'info',
      description: isSuspicious
        ? `Intercepted suspicious binary: ${binary} — ${fullCommand.slice(0, 100)}`
        : `Intercepted process spawn: ${fullCommand.slice(0, 100)}`,
      data: {
        binary,
        command: fullCommand,
        args,
        intercepted: true,
        suspicious: isSuspicious,
      },
    });
  }

  private handleExec(command: string): void {
    const parts = command.trim().split(/\s+/);
    const binary = path.basename(parts[0]);
    const isSuspicious = SUSPICIOUS_BINARIES.includes(binary);

    // CR-001: Analyze command complexity for adversarial pipeline detection
    const complexity = analyzeCommandComplexity(command);
    const isAdversarial = complexity.isAdversarial;

    // Adversarial pipelines are always threats, regardless of binary name
    const category = isAdversarial ? 'threat' : (isSuspicious ? 'violation' : 'normal');
    const severity = isAdversarial ? 'critical' : (isSuspicious ? 'high' : 'info');

    const description = isAdversarial
      ? `Adversarial command pipeline detected: ${complexity.subcommandCount} subcommands (max ${MAX_SUBCOMMANDS})`
      : isSuspicious
        ? `Intercepted suspicious exec: ${binary} -- ${command.slice(0, 100)}`
        : `Intercepted exec: ${command.slice(0, 100)}`;

    this.engine.emit({
      source: 'process',
      category,
      severity,
      description,
      data: {
        binary,
        command,
        intercepted: true,
        suspicious: isSuspicious,
        adversarialPipeline: isAdversarial,
        subcommandCount: complexity.subcommandCount,
      },
    });
  }
}
