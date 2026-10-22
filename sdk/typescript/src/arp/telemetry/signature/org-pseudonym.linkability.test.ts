/**
 * The org pseudonym is not the only identifier a report carries.
 *
 * buildSignedSubmission puts the sensor's sensorId and publicKey into every
 * report beside the org pseudonym, and the emitter POSTs the body it builds
 * (emitter.ts). Two reports one install builds on either side of a UTC month
 * boundary carry different orgPseudonym values and the same sensorId and
 * publicKey, so the two reports link by either value. The doc comment at the
 * top of org-pseudonym.ts has to say so, and must not describe the monthly
 * rotation as an end to long-term tracking.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdtempSync, readFileSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { buildSignedSubmission } from './wire';
import type { RedactedSignal } from './redaction';

const signal: RedactedSignal = {
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

beforeAll(() => {
  for (const name of IDENTITY_ENV) {
    saved[name] = process.env[name];
    delete process.env[name];
  }
  // No identity is pinned in the environment, so the install's own files under
  // this home decide every identifier, as they do on an install that sets none.
  home = mkdtempSync(join(tmpdir(), 'arp-pseudonym-'));
  process.env.OPENA2A_HOME = home;
});

afterAll(() => {
  for (const name of IDENTITY_ENV) {
    const value = saved[name];
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  rmSync(home, { recursive: true, force: true });
});

describe('two reports one install builds in consecutive UTC months', () => {
  it('carry different orgPseudonym values and the same sensorId and publicKey', async () => {
    const june = await buildSignedSubmission(signal, { now: new Date('2026-06-30T23:59:59Z') });
    const july = await buildSignedSubmission(signal, { now: new Date('2026-07-01T00:00:00Z') });

    expect(july.request.orgPseudonym).not.toBe(june.request.orgPseudonym);
    expect(july.request.sensorId).toBe(june.request.sensorId);
    expect(july.request.publicKey).toBe(june.request.publicKey);

    // The body the emitter POSTs carries both values beside the pseudonym.
    for (const built of [june, july]) {
      const posted = JSON.parse(built.body) as Record<string, unknown>;
      expect(posted.orgPseudonym).toBe(built.request.orgPseudonym);
      expect(posted.sensorId).toBe(june.request.sensorId);
      expect(posted.publicKey).toBe(june.request.publicKey);
    }
  });
});

describe('the doc comment at the top of org-pseudonym.ts', () => {
  const source = readFileSync(join(__dirname, 'org-pseudonym.ts'), 'utf-8');
  const header = source.slice(0, source.indexOf('*/') + 2);

  it('is the comment the module opens with', () => {
    expect(header.startsWith('/**')).toBe(true);
  });

  it('names the sensorId and publicKey every report carries beside the pseudonym', () => {
    expect(header).toContain('sensorId');
    expect(header).toContain('publicKey');
  });

  it('does not say the monthly rotation ends long-term tracking', () => {
    expect(header).not.toMatch(/no long-term tracking/i);
  });
});
