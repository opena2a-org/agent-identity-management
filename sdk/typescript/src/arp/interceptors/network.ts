import * as net from 'net';
import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';

/** Known exfiltration/suspicious destinations */
const SUSPICIOUS_HOSTS = [
  'webhook.site', 'requestbin', 'ngrok.io', 'pipedream.net',
  'hookbin.com', 'burpcollaborator', 'interact.sh', 'oastify.com',
  'pastebin.com', 'transfer.sh',
];

/**
 * Network interceptor — hooks net.Socket.prototype.connect to intercept
 * ALL outbound TCP connections at the application level.
 *
 * Advantages over lsof/ss polling:
 * - Zero latency: events fire before the connection is made
 * - 100% accuracy: no missed connections between poll intervals
 * - No system tool dependency: works in sandboxed/container environments
 * - Covers all Node.js networking (http, https, fetch, net.connect)
 */
export class NetworkInterceptor implements Monitor {
  readonly type: MonitorType = 'network';
  private readonly engine: EventEngine;
  private readonly allowedHosts: Set<string>;
  private originalConnect: typeof net.Socket.prototype.connect | null = null;
  private active = false;

  constructor(engine: EventEngine, allowedHosts?: string[]) {
    this.engine = engine;
    this.allowedHosts = new Set(allowedHosts ?? []);
  }

  async start(): Promise<void> {
    if (this.active) return;

    this.originalConnect = net.Socket.prototype.connect;
    const self = this;

    // Patch net.Socket.prototype.connect to intercept all TCP connections
    net.Socket.prototype.connect = function (this: net.Socket, ...args: unknown[]) {
      const parsed = self.parseConnectArgs(args);
      if (parsed) {
        self.handleConnection(parsed.host, parsed.port);
      }
      return self.originalConnect!.apply(this, args as Parameters<typeof net.Socket.prototype.connect>);
    } as typeof net.Socket.prototype.connect;

    this.active = true;
  }

  async stop(): Promise<void> {
    if (!this.active || !this.originalConnect) return;
    net.Socket.prototype.connect = this.originalConnect;
    this.originalConnect = null;
    this.active = false;
  }

  isRunning(): boolean {
    return this.active;
  }

  private parseConnectArgs(args: unknown[]): { host: string; port: number } | null {
    if (args.length === 0) return null;

    let first = args[0];

    // Node.js internals normalize args as [options, callback] array
    if (Array.isArray(first)) {
      first = first[0];
    }

    if (typeof first === 'object' && first !== null) {
      const opts = first as Record<string, unknown>;
      const port = opts.port as number | undefined;
      const host = (opts.host as string) ?? '127.0.0.1';
      if (typeof port === 'number') {
        return { host, port };
      }
    } else if (typeof first === 'number') {
      const host = typeof args[1] === 'string' ? args[1] : '127.0.0.1';
      return { host, port: first };
    }

    return null;
  }

  private handleConnection(host: string, port: number): void {
    const dest = `${host}:${port}`;

    // Check for suspicious hosts
    const isSuspicious = SUSPICIOUS_HOSTS.some((h) =>
      host.includes(h)
    );

    if (isSuspicious) {
      this.engine.emit({
        source: 'network',
        category: 'threat',
        severity: 'critical',
        description: `Intercepted connection to suspicious host: ${dest}`,
        data: {
          remoteAddr: host,
          remotePort: port,
          intercepted: true,
        },
      });
      return;
    }

    // Check allowed hosts
    const isAllowed = this.allowedHosts.size === 0 ||
      this.allowedHosts.has(host) ||
      Array.from(this.allowedHosts).some((h) =>
        host === h || host.endsWith('.' + h)
      );

    this.engine.emit({
      source: 'network',
      category: isAllowed ? 'normal' : 'anomaly',
      severity: isAllowed ? 'info' : 'medium',
      description: `Intercepted outbound connection: ${dest}`,
      data: {
        remoteAddr: host,
        remotePort: port,
        allowed: isAllowed,
        intercepted: true,
      },
    });
  }
}
