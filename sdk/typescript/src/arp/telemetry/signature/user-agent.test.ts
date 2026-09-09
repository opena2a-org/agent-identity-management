/**
 * AIM-12 item 7 (enroll/purge half): every ARP telemetry request must identify
 * its build. The signature emitter sends `User-Agent: OpenA2A-ARP/<VERSION>`,
 * but enroll and purge sent no User-Agent at all, so the registry logged the
 * runtime's default ("node") for exactly the two requests that manage a
 * sensor's identity and its right-to-delete.
 *
 * Injected fetch, temp OPENA2A_HOME — nothing leaves the process.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { enrollSensor } from './enroll';
import { purgeRemoteSignatures } from './purge';
import { VERSION } from '../../index';

let home: string;
let savedHome: string | undefined;

beforeEach(() => {
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-ua-')), 'opena2a-home');
  savedHome = process.env.OPENA2A_HOME;
  process.env.OPENA2A_HOME = home;
});

afterEach(() => {
  if (savedHome === undefined) delete process.env.OPENA2A_HOME;
  else process.env.OPENA2A_HOME = savedHome;
  vi.restoreAllMocks();
  rmSync(join(home, '..'), { recursive: true, force: true });
});

function captureHeaders(): {
  headers: () => Record<string, string> | undefined;
  fetchImpl: typeof fetch;
} {
  let captured: Record<string, string> | undefined;
  const fetchImpl = vi.fn(async (_url: unknown, init?: RequestInit) => {
    captured = init?.headers as Record<string, string>;
    return {
      ok: true,
      status: 200,
      json: async () => ({}),
      text: async () => '',
    } as unknown as Response;
  }) as unknown as typeof fetch;
  return { headers: () => captured, fetchImpl };
}

describe('enroll and purge identify their build like the emitter does', () => {
  it('AIM-12.AC3 item 7: enrollSensor sends User-Agent OpenA2A-ARP/<VERSION>', async () => {
    const { headers, fetchImpl } = captureHeaders();
    await enrollSensor(undefined, { fetchImpl });
    expect(headers()).toBeDefined();
    expect(headers()!['User-Agent']).toBe(`OpenA2A-ARP/${VERSION}`);
  });

  it('AIM-12.AC3 item 7: purgeRemoteSignatures sends User-Agent OpenA2A-ARP/<VERSION>', async () => {
    const { headers, fetchImpl } = captureHeaders();
    await purgeRemoteSignatures(undefined, { fetchImpl });
    expect(headers()).toBeDefined();
    expect(headers()!['User-Agent']).toBe(`OpenA2A-ARP/${VERSION}`);
  });
});
