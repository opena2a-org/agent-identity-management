/**
 * `aim-arp telemetry status` shows each per-install identifier a signature
 * telemetry report carries, with the value the report carries.
 *
 * buildSignedSubmission puts the sensor's sensorId and publicKey and the org
 * pseudonym into each report (wire.ts), and the emitter POSTs that body as
 * built. Status printed a line for the sensor id and one for the org
 * pseudonym, and none for the public key.
 *
 * Status is a read command: it shows the public key without creating a sensor
 * key, as it shows the other two without creating them.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { buildSignedSubmission } from '../telemetry/signature/wire';
import { peekSensorPublicKey } from '../telemetry/signature/sensor-identity';
import { SENSOR_KEY_FILE } from '../telemetry/signature/paths';
import type { RedactedSignal } from '../telemetry/signature/redaction';
import { runTelemetrySubcommand } from './telemetry';

/** The status line that shows each per-install identifier, by report field. */
const STATUS_LINE: Record<string, RegExp> = {
  sensorId: /^ {2}Sensor id: +(\S+)/,
  publicKey: /^ {2}Public key: +(\S+)/,
  orgPseudonym: /^ {2}Org pseudonym: +(\S+)/,
};

const exfiltration: RedactedSignal = {
  tacticId: 'exfiltration',
  techniqueId: 'ATM-T8002',
  actionClass: 'network_egress',
  targetClass: 'network',
  sequencePattern: '',
  outcomeClass: 'blocked',
  severity: 'high',
};

/** The variables that decide the identity a report carries. */
const IDENTITY_ENV = ['OPENA2A_HOME', 'OPENA2A_ORG_ID', 'OPENA2A_SENSOR_ID'] as const;
const saved: Partial<Record<(typeof IDENTITY_ENV)[number], string>> = {};
let home: string;

beforeEach(() => {
  for (const name of IDENTITY_ENV) {
    saved[name] = process.env[name];
    delete process.env[name];
  }
  // No identity is pinned in the environment, so the files under this home
  // decide every identifier, as they do on an install that sets none.
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-status-ids-')), 'opena2a-home');
  process.env.OPENA2A_HOME = home;
  // One clock for the report and for status: the org pseudonym changes with
  // the UTC month, and a run across a month boundary would compare two months.
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(new Date('2026-07-15T12:00:00Z'));
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  for (const name of IDENTITY_ENV) {
    const value = saved[name];
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  rmSync(join(home, '..'), { recursive: true, force: true });
});

/** The lines `aim-arp telemetry status` prints, through the dispatch the bin calls. */
async function status(): Promise<string[]> {
  const lines: string[] = [];
  vi.spyOn(console, 'log').mockImplementation((...args: unknown[]) => {
    lines.push(...args.join(' ').split('\n'));
  });
  expect(await runTelemetrySubcommand('status', [])).toBe(0);
  return lines;
}

describe('aim-arp telemetry status', () => {
  it('shows the sensorId, publicKey and orgPseudonym a report built on this install carries', async () => {
    const built = await buildSignedSubmission(exfiltration);
    const report = JSON.parse(built.body) as Record<string, unknown>;
    const lines = await status();
    for (const [field, line] of Object.entries(STATUS_LINE)) {
      const shown = lines.map((l) => line.exec(l)?.[1]).filter((v) => v !== undefined);
      expect(shown, field).toEqual([report[field]]);
    }
  });

  it('says no public key exists yet on a home with no sensor key, and creates none', async () => {
    const lines = await status();
    expect(lines).toContain('  Public key:   none yet (created on first send)');
    expect(existsSync(home)).toBe(false);
  });
});

describe('peekSensorPublicKey', () => {
  it('returns the publicKey of the next report built from the same key file', async () => {
    const before = await buildSignedSubmission(exfiltration);
    const peeked = await peekSensorPublicKey();
    const after = await buildSignedSubmission(exfiltration);
    expect(peeked).toBe(before.request.publicKey);
    expect(peeked).toBe(after.request.publicKey);
  });

  it('returns null for a key file that holds no 32-byte key, and leaves the file as it was', async () => {
    mkdirSync(home, { recursive: true });
    const keyFile = join(home, SENSOR_KEY_FILE);
    writeFileSync(keyFile, 'not-a-key', { mode: 0o600 });
    expect(await peekSensorPublicKey()).toBeNull();
    expect(readFileSync(keyFile, 'utf8')).toBe('not-a-key');
  });
});
