import { describe, it, expect, vi } from 'vitest';
import { existsSync, readFileSync } from 'fs';
import { join } from 'path';
import { runCase } from './python-parity/run-case';

// The network monitor's lsof, ss and netstat parsers read execSync's output;
// run-case.ts sets it per case. Everything else in child_process is real.
vi.mock('child_process', async (importOriginal) => ({
  ...(await importOriginal<typeof import('child_process')>()),
  execSync: vi.fn(),
}));

/**
 * The ARP behavioral cases the Python SDK's engine port shares with this SDK.
 *
 * sdk/python/aim_sdk/arp is a port of src/arp. Each case in the shared file is
 * an input to the engine, an interceptor, a monitor, the runtime twin or the
 * coordinator, and the output this SDK gives for it. This suite checks this
 * SDK still gives that output; the Python suite
 * (sdk/python/tests/test_arp_engine_parity.py) checks the port gives the same.
 * A change to the behavior of either SDK fails one of the two until the other
 * SDK and the shared file follow it.
 */

const CASES_FILE = join(__dirname, '..', '..', '..', 'python', 'tests', 'fixtures', 'ts_arp', 'behavioral-cases.json');

interface SharedCase {
  kind: string;
  name: string;
  input: Record<string, unknown>;
  expected: unknown;
}

const cases: SharedCase[] = existsSync(CASES_FILE)
  ? (JSON.parse(readFileSync(CASES_FILE, 'utf-8')) as { cases: SharedCase[] }).cases
  : [];

describe('ARP behavior shared with the Python port', () => {
  it('reads the shared case file', () => {
    expect(existsSync(CASES_FILE)).toBe(true);
    expect(cases.length).toBeGreaterThan(100);
  });

  it.each(cases.map((c) => [`${c.kind}: ${c.name}`, c] as const))('gives the shared output for %s', async (_title, c) => {
    expect(await runCase(c.kind, c.input)).toEqual(c.expected);
  });
});
