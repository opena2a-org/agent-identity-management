/**
 * Can ARP attribute a runtime event to the installed package version whose code
 * made the call? A package is installed under a temporary node_modules/, its
 * package.json beside its index.js, and it makes calls that ARP's interceptors
 * patch.
 *
 * As ARP is built, these tests record the answer as no:
 * - for each of the four calls in CALLS, the event the interceptor emits names
 *   neither the package nor its version;
 * - AgentRuntimeProtection calls the GTIN forwarder from a handler registered
 *   after the one that awaits the intelligence layer, and for a spawnSync the
 *   package makes, no frame of the package is on the stack there; the GTIN
 *   payload carries the unattributed sentinel and an empty packageVersion.
 *
 * The call site is on the stack inside the interception: the engine calls its
 * first handler while the patched call is still running, so for each of the
 * four calls a stack captured there has a frame in the package's own file, and
 * the package.json at the root of that package's directory gives its name and
 * version.
 *
 * These tests must never reach the production registry. Every request funnels
 * through a guarding fetch stub that only forwards to 127.0.0.1; anything else
 * is answered locally and recorded. Every connection the package makes goes to
 * a server on 127.0.0.1, and the process it spawns names a path that does not
 * exist, so nothing runs.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createServer, type Server } from 'http';
import { mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'fs';
import { createRequire } from 'module';
import { tmpdir } from 'os';
import { join } from 'path';
import { AgentRuntimeProtection } from '../index';
import { EventEngine } from '../engine/event-engine';
import { ProcessInterceptor } from '../interceptors/process';
import { FilesystemInterceptor } from '../interceptors/filesystem';
import { NetworkInterceptor } from '../interceptors/network';
import { GTINForwarder } from './forwarder';
import { GTIN_UNATTRIBUTED_PACKAGE_NAME } from './gtin';
import type { ARPEvent, Monitor } from '../types';

const PROBE_NAME = 'arp-callsite-probe';
const PROBE_VERSION = '9.8.7';
const AGENT_NAME = 'call-site-measurement-agent';

/** The installed package: each export makes one call an ARP interceptor patches. */
const PROBE_SOURCE = `'use strict';
const childProcess = require('child_process');
const fs = require('fs');
const http = require('http');
const net = require('net');
const path = require('path');

// 'curl' is on the process interceptor's list of suspicious binaries.
exports.spawn = function spawn() {
  return childProcess.spawnSync('/nonexistent-arp-probe-dir/curl', ['--version']);
};

exports.readEnv = function readEnv(dir) {
  try {
    fs.readFileSync(path.join(dir, '.env'));
  } catch (_) {
    // the file does not exist
  }
};

exports.netConnect = function netConnect(port) {
  const socket = net.connect({ host: '127.0.0.1', port });
  socket.on('error', () => {});
  return socket;
};

exports.httpRequest = function httpRequest(port) {
  const req = http.request({ host: '127.0.0.1', port, path: '/' });
  req.on('error', () => {});
  req.end();
  return req;
};
`;

interface Probe {
  spawn(): unknown;
  readEnv(dir: string): void;
  netConnect(port: number): { destroy(): void };
  httpRequest(port: number): { destroy(): void };
}

/** Every variable that moves the GTIN destination or the opt-in/opt-out decision. */
const ENV_KEYS = [
  'OPENA2A_HOME',
  'OPENA2A_REGISTRY_URL',
  'OPENA2A_TELEMETRY',
  'OPENA2A_TELEMETRY_OPTOUT',
  'ARP_TELEMETRY_DISABLED',
  'AIM_TELEMETRY',
] as const;

const NO_MONITORS = {
  process: { enabled: false },
  network: { enabled: false },
  filesystem: { enabled: false },
};

let server: Server;
let port: number;
let loopbackBase: string;
let received: Array<{ method: string | undefined; url: string | undefined; body: string }>;
let attempted: string[];
let root: string;
let probeDir: string;
let probe: Probe;
const savedEnv: Record<string, string | undefined> = {};
const realFetch = globalThis.fetch;

beforeEach(async () => {
  received = [];
  attempted = [];

  for (const key of ENV_KEYS) {
    savedEnv[key] = process.env[key];
    delete process.env[key];
  }
  // The GTIN sensor salt and install-date marker live here, so the test neither
  // reads nor writes the real ~/.opena2a. Real path, because Node's require
  // names a module by its real path in stack frames.
  root = realpathSync(mkdtempSync(join(tmpdir(), 'arp-gtin-call-site-')));
  process.env.OPENA2A_HOME = join(root, 'home');

  probeDir = join(root, 'node_modules', PROBE_NAME);
  mkdirSync(probeDir, { recursive: true });
  writeFileSync(
    join(probeDir, 'package.json'),
    JSON.stringify({ name: PROBE_NAME, version: PROBE_VERSION, main: 'index.js' }),
  );
  writeFileSync(join(probeDir, 'index.js'), PROBE_SOURCE);
  // Node's own require loads it, so its frames carry the file's real path.
  probe = createRequire(join(root, 'loader.js'))(probeDir) as Probe;

  server = createServer((req, res) => {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      received.push({ method: req.method, url: req.url, body });
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ eventId: 'evt-loopback-1' }));
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  port = typeof address === 'object' && address ? address.port : 0;
  loopbackBase = `http://127.0.0.1:${port}`;

  vi.stubGlobal('fetch', ((input: Parameters<typeof fetch>[0], init?: RequestInit) => {
    const url = String(input);
    attempted.push(url);
    if (url.startsWith('http://127.0.0.1:')) return realFetch(input, init);
    return Promise.resolve(new Response('{}', { status: 599 }));
  }) as typeof fetch);
});

afterEach(async () => {
  vi.unstubAllGlobals();
  for (const key of ENV_KEYS) {
    if (savedEnv[key] === undefined) delete process.env[key];
    else process.env[key] = savedEnv[key];
  }
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
  rmSync(root, { recursive: true, force: true });
});

/**
 * A stack trace captured with Error.stackTraceLimit raised to 64. Node 26.12
 * starts with a limit of 10, and under that limit the stack captured in an
 * engine handler during an http.request call stopped before the calling
 * package's frame.
 */
function deepStack(): string {
  const saved = Error.stackTraceLimit;
  Error.stackTraceLimit = 64;
  try {
    return new Error('call site').stack ?? '';
  } finally {
    Error.stackTraceLimit = saved;
  }
}

/**
 * The package that owns the innermost frame whose file is inside a
 * node_modules directory: the name and version in the package.json at the root
 * of that package's directory. Null when no frame is in node_modules.
 */
function callingPackage(stack: string): { name: string; version: string } | null {
  const marker = '/node_modules/';
  for (const line of stack.split('\n').slice(1)) {
    const file = /\(?(\/[^()]+?):\d+:\d+\)?$/.exec(line.trim())?.[1];
    if (!file) continue;
    const at = file.lastIndexOf(marker);
    if (at < 0) continue;
    const segments = file.slice(at + marker.length).split('/');
    const packageDir = segments[0].startsWith('@') ? `${segments[0]}/${segments[1]}` : segments[0];
    const manifest = JSON.parse(
      readFileSync(join(file.slice(0, at), 'node_modules', packageDir, 'package.json'), 'utf8'),
    ) as { name: string; version: string };
    return { name: manifest.name, version: manifest.version };
  }
  return null;
}

interface InterceptedCall {
  label: string;
  source: ARPEvent['source'];
  interceptor: (engine: EventEngine) => Monitor;
  call: () => { destroy(): void } | void;
}

const CALLS: InterceptedCall[] = [
  {
    label: 'child_process.spawnSync',
    source: 'process',
    interceptor: (engine) => new ProcessInterceptor(engine),
    call: () => {
      probe.spawn();
    },
  },
  {
    label: 'fs.readFileSync',
    source: 'filesystem',
    interceptor: (engine) => new FilesystemInterceptor(engine),
    call: () => probe.readEnv(root),
  },
  {
    label: 'net.connect',
    source: 'network',
    interceptor: (engine) => new NetworkInterceptor(engine, ['allowed.invalid']),
    call: () => probe.netConnect(port),
  },
  {
    label: 'http.request',
    source: 'network',
    interceptor: (engine) => new NetworkInterceptor(engine, ['allowed.invalid']),
    call: () => probe.httpRequest(port),
  },
];

describe('attributing an intercepted call to the installed package that made it', () => {
  it.each(CALLS)(
    '$label: the event names neither the package nor its version, and a stack captured in the first handler resolves to both',
    async ({ source, interceptor, call }) => {
      const engine = new EventEngine({ agentName: AGENT_NAME, rules: [] });
      const seen: Array<{ event: ARPEvent; stack: string }> = [];
      engine.onEvent((event) => {
        seen.push({ event, stack: deepStack() });
      });

      const monitor = interceptor(engine);
      await monitor.start();
      let handle: { destroy(): void } | void;
      let duringCall: Array<{ event: ARPEvent; stack: string }>;
      try {
        handle = call();
        // Only what the first handler saw before the patched call returned.
        duringCall = seen.slice();
      } finally {
        await monitor.stop();
      }
      if (handle) handle.destroy();

      const intercepted = duringCall.filter(
        (s) => s.event.source === source && s.event.data.intercepted === true,
      );
      expect(intercepted.length).toBeGreaterThan(0);
      const { event, stack } = intercepted[0];

      const eventJson = JSON.stringify(event);
      expect(eventJson).not.toContain(PROBE_NAME);
      expect(eventJson).not.toContain(PROBE_VERSION);

      expect(stack).toContain(join(probeDir, 'index.js'));
      expect(callingPackage(stack)).toEqual({ name: PROBE_NAME, version: PROBE_VERSION });
    },
  );

  it('runs the GTIN forwarder with no frame of the calling package on the stack, and the payload carries the unattributed sentinel', async () => {
    const forwarderStacks: string[] = [];
    const original = GTINForwarder.prototype.onEvent;
    GTINForwarder.prototype.onEvent = function (this: GTINForwarder, event: ARPEvent): void {
      forwarderStacks.push(deepStack());
      original.call(this, event);
    };

    try {
      const arp = new AgentRuntimeProtection({
        agentName: AGENT_NAME,
        dataDir: join(root, 'arp'),
        monitors: NO_MONITORS,
        interceptors: { process: { enabled: true } },
        // No rule can match, so the event only reaches the event handlers.
        rules: [],
        gtin: { enabled: true, registryUrl: loopbackBase },
      });
      await arp.start();
      try {
        probe.spawn();
        await vi.waitFor(() => expect(forwarderStacks.length).toBeGreaterThan(0));
      } finally {
        // stop() restores child_process, then flushes the GTIN queue and waits for each POST.
        await arp.stop();
      }
    } finally {
      GTINForwarder.prototype.onEvent = original;
    }

    for (const stack of forwarderStacks) {
      expect(stack).not.toContain(probeDir);
      expect(callingPackage(stack)?.name).not.toBe(PROBE_NAME);
    }

    const posts = received.filter((r) => r.url === '/api/v1/telemetry/runtime');
    expect(posts.length).toBeGreaterThan(0);
    for (const url of attempted) expect(url.startsWith('http://127.0.0.1:')).toBe(true);
    for (const post of posts) {
      expect(post.method).toBe('POST');
      const payload = JSON.parse(post.body) as { packageName: string; packageVersion: string };
      expect(payload.packageName).toBe(GTIN_UNATTRIBUTED_PACKAGE_NAME);
      expect(payload.packageVersion).toBe('');
      expect(post.body).not.toContain(PROBE_NAME);
      expect(post.body).not.toContain(PROBE_VERSION);
    }
  });
});
