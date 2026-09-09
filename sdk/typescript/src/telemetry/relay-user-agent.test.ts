/**
 * AIM-12 item 7 (relay half): the causal-denial relay sent a bare
 * `User-Agent: OpenA2A-AIM-SDK-Relay` with no version, so the registry could
 * not attribute an uploaded indicator to the SDK build that produced it —
 * the same defect the frozen `OpenA2A-ARP/0.2.0` header was (arp/index.ts).
 */
import { it, expect, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { CorrelatedRelay } from './relay';
import { writeCorrelatedRecord } from './local-writer';
import { buildCorrelatedRecord } from './correlated-record';
import { SDK_VERSION } from '../version';

const tmpDirs: string[] = [];

afterEach(() => {
  for (const dir of tmpDirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

it('AIM-12.AC3 item 7: the relay User-Agent carries the SDK version', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cde-relay-ua-'));
  tmpDirs.push(dir);
  writeCorrelatedRecord(
    buildCorrelatedRecord({
      correlationId: 'cde_000000000_aabbccddeeff',
      agentId: 'agent-1',
      enforcement: {
        decision: 'deny',
        outcome: 'DENY_INTENT',
        capability: 'net:connect',
        resource: 'https://example.invalid/x',
        deniedReason: 'blocked',
        occurredAt: '2026-06-06T00:00:01.000Z',
        source: 'aim-pdp',
      },
      detection: {
        injectionDetected: true,
        attackClass: 'injection',
        detector: 'guard',
        detectedAt: '2026-06-06T00:00:01.000Z',
      },
      intent: { intentClass: 'exfiltration', confidence: 0.7, blocked: true, source: 'intent' },
    }),
    dir,
  );

  let headers: Record<string, string> | undefined;
  const fetchImpl = vi.fn(async (_url: string, init: RequestInit) => {
    headers = init.headers as Record<string, string>;
    return {
      ok: true,
      status: 201,
      json: async () => ({}),
      text: async () => '',
    } as unknown as Response;
  });
  const relay = new CorrelatedRelay({
    enabled: true,
    dataDir: dir,
    fetchImpl: fetchImpl as unknown as typeof fetch,
  });
  const res = await relay.flushOnce();

  expect(res.uploaded).toBe(1);
  expect(headers).toBeDefined();
  expect(headers!['User-Agent']).toBe(`OpenA2A-AIM-SDK-Relay/${SDK_VERSION}`);
});
