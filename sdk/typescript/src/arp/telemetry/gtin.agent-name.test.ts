/**
 * The GTIN payload AgentRuntimeProtection sends does not carry the agent name
 * it is configured with. The agent name is the operator's own label for the
 * agent, not a public package name, so the payload's `packageName` is the
 * unattributed sentinel, the same label the causal-denial relay sends when the
 * integrator declares no package.
 *
 * These tests must never reach the production registry. Every request funnels
 * through a guarding fetch stub that only forwards to 127.0.0.1; anything else
 * is answered locally and recorded.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createServer, type Server } from 'http';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { AgentRuntimeProtection } from '../index';
import { loadConfig } from '../config/loader';
import { GTIN_UNATTRIBUTED_PACKAGE_NAME } from './gtin';
import { SENTINEL_PACKAGE_NAME } from '../../telemetry/relay';

const AGENT_NAME = 'acme-payroll-agent-7f3';
const CHECKOUT_DIR = 'acme-payroll-checkout-7f3';

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
let loopbackBase: string;
let received: Array<{ method: string | undefined; url: string | undefined; body: string }>;
let attempted: string[];
let home: string;
const savedEnv: Record<string, string | undefined> = {};
const realFetch = globalThis.fetch;

beforeEach(async () => {
  received = [];
  attempted = [];

  for (const key of ENV_KEYS) {
    savedEnv[key] = process.env[key];
    delete process.env[key];
  }
  // The sensor salt, the install-date marker and the opt-out marker all live
  // here, so the test neither reads nor writes the real ~/.opena2a.
  home = mkdtempSync(join(tmpdir(), 'arp-gtin-agent-name-'));
  process.env.OPENA2A_HOME = home;

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
  const port = typeof address === 'object' && address ? address.port : 0;
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
  await new Promise<void>((resolve) => server.close(() => resolve()));
  rmSync(home, { recursive: true, force: true });
});

/** Emit one anomalous event, then stop(), which flushes the GTIN queue and waits for each POST. */
async function sendOneAnomaly(arp: AgentRuntimeProtection): Promise<void> {
  await arp.start();
  await arp.getEngine().emit({
    source: 'network',
    category: 'anomaly',
    severity: 'medium',
    description: 'Outbound connection to a host outside the allow list',
    data: {},
  });
  await arp.stop();
}

/** The GTIN POST bodies the loopback registry received; fails when there are none. */
function gtinBodies(): string[] {
  const posts = received.filter((r) => r.url === '/api/v1/telemetry/runtime');
  expect(posts.length).toBeGreaterThan(0);
  for (const post of posts) expect(post.method).toBe('POST');
  for (const url of attempted) expect(url.startsWith('http://127.0.0.1:')).toBe(true);
  return posts.map((p) => p.body);
}

describe('the GTIN payload AgentRuntimeProtection sends', () => {
  it('carries the unattributed sentinel as packageName and not the configured agent name', async () => {
    const arp = new AgentRuntimeProtection({
      agentName: AGENT_NAME,
      dataDir: join(home, 'arp'),
      monitors: NO_MONITORS,
      // No rule can match, so the event only reaches the event handlers.
      rules: [],
      gtin: { enabled: true, registryUrl: loopbackBase },
    });

    await sendOneAnomaly(arp);

    for (const body of gtinBodies()) {
      expect(JSON.parse(body).packageName).toBe(SENTINEL_PACKAGE_NAME);
      expect(body).not.toContain(AGENT_NAME);
    }
  });

  it('does not carry the working-directory name a config file without agentName is given', async () => {
    const configPath = join(home, 'arp.json');
    writeFileSync(
      configPath,
      JSON.stringify({
        dataDir: join(home, 'arp'),
        monitors: NO_MONITORS,
        rules: [],
        gtin: { enabled: true, registryUrl: loopbackBase },
      }),
    );

    const cwd = vi.spyOn(process, 'cwd').mockReturnValue(join(home, CHECKOUT_DIR));
    let arp: AgentRuntimeProtection;
    try {
      // The precondition: the loader names the unnamed agent after the directory.
      expect(loadConfig(configPath).agentName).toBe(CHECKOUT_DIR);
      arp = new AgentRuntimeProtection(configPath);
    } finally {
      cwd.mockRestore();
    }

    await sendOneAnomaly(arp);

    for (const body of gtinBodies()) {
      expect(JSON.parse(body).packageName).toBe(SENTINEL_PACKAGE_NAME);
      expect(body).not.toContain(CHECKOUT_DIR);
    }
  });

  it('uses the same sentinel label as the causal-denial relay', () => {
    expect(GTIN_UNATTRIBUTED_PACKAGE_NAME).toBe(SENTINEL_PACKAGE_NAME);
  });
});
