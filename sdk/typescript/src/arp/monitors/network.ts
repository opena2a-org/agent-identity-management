import { execSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';

interface Connection {
  protocol: string;
  localAddr: string;
  localPort: number;
  remoteAddr: string;
  remotePort: number;
  state: string;
  pid?: number;
}

/** Known exfiltration/suspicious destinations */
const SUSPICIOUS_HOSTS = [
  'webhook.site', 'requestbin', 'ngrok.io', 'pipedream.net',
  'hookbin.com', 'burpcollaborator', 'interact.sh', 'oastify.com',
  'pastebin.com', 'transfer.sh',
];

/**
 * Network monitor — tracks outbound connections using lsof/ss.
 * Detects connections to suspicious hosts, new outbound destinations,
 * and unexpected port usage.
 */
export class NetworkMonitor implements Monitor {
  readonly type: MonitorType = 'network';
  private timer?: ReturnType<typeof setInterval>;
  private readonly engine: EventEngine;
  private readonly intervalMs: number;
  private readonly allowedHosts: Set<string>;
  private knownDestinations = new Set<string>();
  /** Sliding window of connection timestamps for burst detection */
  private connectionTimestamps: number[] = [];
  /** Rolling average of connections per minute (exponential moving average) */
  private rollingAvgPerMinute = 0;
  /** Number of completed windows used for the rolling average */
  private windowCount = 0;
  /** Configurable burst threshold multiplier (default: 3x rolling average) */
  private readonly burstThreshold: number;
  /** Sliding window duration in ms (1 minute) */
  private readonly burstWindowMs = 60000;

  constructor(engine: EventEngine, intervalMs: number = 10000, allowedHosts?: string[], burstThreshold: number = 3) {
    this.engine = engine;
    this.intervalMs = intervalMs;
    this.allowedHosts = new Set(allowedHosts ?? []);
    this.burstThreshold = burstThreshold;
  }

  async start(): Promise<void> {
    // Initial snapshot
    const connections = this.getConnections();
    for (const conn of connections) {
      this.knownDestinations.add(`${conn.remoteAddr}:${conn.remotePort}`);
    }

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
      const connections = this.getConnections();

      for (const conn of connections) {
        const dest = `${conn.remoteAddr}:${conn.remotePort}`;

        // Check for suspicious hosts
        const isSuspicious = SUSPICIOUS_HOSTS.some((h) =>
          conn.remoteAddr.includes(h)
        );

        if (isSuspicious) {
          this.engine.emit({
            source: 'network',
            category: 'threat',
            severity: 'critical',
            description: `Connection to suspicious host: ${dest}`,
            data: {
              remoteAddr: conn.remoteAddr,
              remotePort: conn.remotePort,
              protocol: conn.protocol,
              pid: conn.pid,
            },
          });
          continue;
        }

        // Check for new outbound destinations
        if (!this.knownDestinations.has(dest)) {
          const isAllowed = this.allowedHosts.size === 0 ||
            this.allowedHosts.has(conn.remoteAddr) ||
            Array.from(this.allowedHosts).some((h) =>
              conn.remoteAddr === h || conn.remoteAddr.endsWith('.' + h)
            );

          this.engine.emit({
            source: 'network',
            category: isAllowed ? 'normal' : 'anomaly',
            severity: isAllowed ? 'info' : 'medium',
            description: `New outbound connection: ${dest}`,
            data: {
              remoteAddr: conn.remoteAddr,
              remotePort: conn.remotePort,
              protocol: conn.protocol,
              pid: conn.pid,
              allowed: isAllowed,
            },
          });

          this.knownDestinations.add(dest);
        }
      }
      // Burst detection: track connection count in sliding window
      this.checkConnectionBurst(connections.length);
    } catch {
      // lsof/ss failed — skip
    }
  }

  /**
   * Track connections per minute and detect bursts.
   * Emits a 'violation' event if current window exceeds burstThreshold * rolling average.
   */
  private checkConnectionBurst(connectionCount: number): void {
    const now = Date.now();

    // Add current connections to the timestamp window
    for (let i = 0; i < connectionCount; i++) {
      this.connectionTimestamps.push(now);
    }

    // Trim timestamps outside the 1-minute window
    const cutoff = now - this.burstWindowMs;
    this.connectionTimestamps = this.connectionTimestamps.filter((t) => t >= cutoff);

    const currentWindowCount = this.connectionTimestamps.length;

    // Update rolling average (exponential moving average, alpha = 0.2)
    if (this.windowCount === 0) {
      this.rollingAvgPerMinute = currentWindowCount;
      this.windowCount = 1;
      return; // First window — no baseline to compare against
    }

    const alpha = 0.2;
    const previousAvg = this.rollingAvgPerMinute;
    this.rollingAvgPerMinute = alpha * currentWindowCount + (1 - alpha) * previousAvg;
    this.windowCount++;

    // Need at least 3 windows before burst detection activates
    if (this.windowCount < 3) return;

    const threshold = previousAvg * this.burstThreshold;
    if (threshold > 0 && currentWindowCount > threshold) {
      this.engine.emit({
        source: 'network',
        category: 'violation',
        severity: 'high',
        description: `Connection burst detected: ${currentWindowCount} connections in last 60s (threshold: ${Math.round(threshold)}, avg: ${Math.round(previousAvg)})`,
        data: {
          currentCount: currentWindowCount,
          rollingAverage: Math.round(previousAvg),
          threshold: Math.round(threshold),
          burstMultiplier: this.burstThreshold,
        },
      });
    }
  }

  private getConnections(): Connection[] {
    const platform = os.platform();

    if (platform === 'darwin') {
      // macOS: try lsof first, fall back to netstat
      const lsof = this.parseLsof();
      if (lsof.length > 0) return lsof;
      return this.parseNetstat();
    }

    // Linux: try ss first, fall back to /proc/net/tcp, then netstat
    const ss = this.parseSs();
    if (ss.length > 0) return ss;
    const proc = this.parseProcNetTcp();
    if (proc.length > 0) return proc;
    return this.parseNetstat();
  }

  private parseLsof(): Connection[] {
    const connections: Connection[] = [];
    try {
      const output = execSync(
        'lsof -i -n -P 2>/dev/null | grep ESTABLISHED',
        { encoding: 'utf-8', timeout: 5000 },
      );

      for (const line of output.trim().split('\n')) {
        if (!line) continue;
        const parts = line.split(/\s+/);
        if (parts.length < 9) continue;

        const nameField = parts[8] ?? '';
        const match = nameField.match(/(\S+):(\d+)->(\S+):(\d+)/);
        if (!match) continue;

        connections.push({
          protocol: parts[7]?.includes('TCP') ? 'tcp' : 'udp',
          localAddr: match[1],
          localPort: parseInt(match[2]),
          remoteAddr: match[3],
          remotePort: parseInt(match[4]),
          state: 'ESTABLISHED',
          pid: parseInt(parts[1]) || undefined,
        });
      }
    } catch {
      // lsof not available or no connections
    }
    return connections;
  }

  private parseSs(): Connection[] {
    const connections: Connection[] = [];
    try {
      const output = execSync(
        'ss -tpn state established 2>/dev/null',
        { encoding: 'utf-8', timeout: 5000 },
      );

      const lines = output.trim().split('\n');
      // Under the `state established` filter ss omits the State column,
      // shifting every field left by one; the header line says which shape
      // this run produced. Without root, ss also omits the trailing
      // `users:(...)` process field, so data lines may end at the peer column.
      const localIdx = lines[0].startsWith('State') ? 3 : 2;

      for (const line of lines.slice(1)) {
        if (!line) continue;
        const parts = line.split(/\s+/);
        if (parts.length < localIdx + 2) continue;

        const local = parts[localIdx]?.split(':') ?? [];
        const remote = parts[localIdx + 1]?.split(':') ?? [];

        connections.push({
          protocol: 'tcp',
          localAddr: local.slice(0, -1).join(':'),
          localPort: parseInt(local[local.length - 1] ?? '0'),
          remoteAddr: remote.slice(0, -1).join(':'),
          remotePort: parseInt(remote[remote.length - 1] ?? '0'),
          state: 'ESTABLISHED',
        });
      }
    } catch {
      // ss not available
    }
    return connections;
  }

  /** Parse /proc/net/tcp (Linux) — no external tools required */
  private parseProcNetTcp(): Connection[] {
    const connections: Connection[] = [];
    try {
      const content = fs.readFileSync('/proc/net/tcp', 'utf-8');
      for (const line of content.trim().split('\n').slice(1)) {
        const parts = line.trim().split(/\s+/);
        if (parts.length < 4) continue;

        const state = parseInt(parts[3], 16);
        if (state !== 1) continue; // 01 = ESTABLISHED

        const local = this.parseHexAddr(parts[1]);
        const remote = this.parseHexAddr(parts[2]);
        if (!local || !remote) continue;

        connections.push({
          protocol: 'tcp',
          localAddr: local.addr,
          localPort: local.port,
          remoteAddr: remote.addr,
          remotePort: remote.port,
          state: 'ESTABLISHED',
        });
      }
    } catch {
      // /proc/net/tcp not available (macOS, containers without /proc)
    }
    return connections;
  }

  /** Parse hex address:port from /proc/net/tcp (e.g., "0100007F:0050") */
  private parseHexAddr(hexPair: string): { addr: string; port: number } | null {
    const [hexAddr, hexPort] = hexPair.split(':');
    if (!hexAddr || !hexPort) return null;

    const port = parseInt(hexPort, 16);
    const addrNum = parseInt(hexAddr, 16);
    // /proc/net/tcp stores addresses in little-endian
    const addr = [
      addrNum & 0xff,
      (addrNum >> 8) & 0xff,
      (addrNum >> 16) & 0xff,
      (addrNum >> 24) & 0xff,
    ].join('.');

    return { addr, port };
  }

  /** Parse netstat output — available on most systems as last resort */
  private parseNetstat(): Connection[] {
    const connections: Connection[] = [];
    try {
      const output = execSync(
        'netstat -an 2>/dev/null | grep ESTABLISHED',
        { encoding: 'utf-8', timeout: 5000 },
      );

      for (const line of output.trim().split('\n')) {
        if (!line) continue;
        const parts = line.trim().split(/\s+/);
        if (parts.length < 5) continue;

        const proto = parts[0];
        if (!proto.startsWith('tcp')) continue;

        const localParts = this.splitAddrPort(parts[3]);
        const remoteParts = this.splitAddrPort(parts[4]);
        if (!localParts || !remoteParts) continue;

        connections.push({
          protocol: 'tcp',
          localAddr: localParts.addr,
          localPort: localParts.port,
          remoteAddr: remoteParts.addr,
          remotePort: remoteParts.port,
          state: 'ESTABLISHED',
        });
      }
    } catch {
      // netstat not available
    }
    return connections;
  }

  /** Split "addr.port" or "addr:port" into components */
  private splitAddrPort(value: string): { addr: string; port: number } | null {
    // Try colon separator first (Linux netstat, IPv6)
    const colonIdx = value.lastIndexOf(':');
    if (colonIdx > 0) {
      const port = parseInt(value.slice(colonIdx + 1));
      if (!isNaN(port)) return { addr: value.slice(0, colonIdx), port };
    }
    // macOS netstat uses dot separator: "127.0.0.1.8080"
    const dotIdx = value.lastIndexOf('.');
    if (dotIdx > 0) {
      const port = parseInt(value.slice(dotIdx + 1));
      if (!isNaN(port)) return { addr: value.slice(0, dotIdx), port };
    }
    return null;
  }
}
