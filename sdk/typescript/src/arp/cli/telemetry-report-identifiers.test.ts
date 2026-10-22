/**
 * The texts that list the identifiers a signature telemetry report carries
 * name each one the report carries.
 *
 * buildSignedSubmission puts the sensor's sensorId and publicKey and the org
 * pseudonym into each report beside its structural fields (wire.ts), and the
 * emitter POSTs that body as built. Four texts list those identifiers: the
 * README's structural signature telemetry paragraph, disclosureText() (what
 * the first-run notice and `aim-arp telemetry disclosure` print), the footer
 * of `aim-arp telemetry --help`, and the footer `aim-arp telemetry log` prints
 * under the audited payloads. Each named the sensor id and the org pseudonym
 * and left out the public key, and the log footer called those two "the only
 * identifiers" under a payload that carried the key.
 *
 * The identifiers come from real reports, not from a list: a field is a
 * per-install identifier when two reports one install builds in one UTC month
 * carry the same value in it and a report a second install builds carries a
 * different value.
 */

import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import { mkdtempSync, readFileSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { buildSignedSubmission, SIGNATURE_INGEST_PATH } from '../telemetry/signature/wire';
import { appendAuditRecord, queuedRecord } from '../telemetry/signature/audit-log';
import { disclosureText } from '../telemetry/signature/disclosure';
import type { RedactedSignal } from '../telemetry/signature/redaction';
import { telemetryHelpText, telemetryLog } from './telemetry';

/** How the texts name each per-install identifier, by report field. */
const NAMED_AS: Record<string, RegExp> = {
  sensorId: /\bsensor id\b/i,
  publicKey: /\bpublic key\b/i,
  orgPseudonym: /\borg pseudonym\b/i,
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

const harvest: RedactedSignal = {
  tacticId: 'credential-harvest',
  techniqueId: 'ATM-T2001',
  actionClass: 'credential_access',
  targetClass: 'credential',
  sequencePattern: '',
  outcomeClass: 'detected',
  severity: 'medium',
};

/** The variables that decide the identity a report carries. */
const IDENTITY_ENV = ['OPENA2A_HOME', 'OPENA2A_ORG_ID', 'OPENA2A_SENSOR_ID'] as const;
const saved: Partial<Record<(typeof IDENTITY_ENV)[number], string>> = {};
let root: string;
let installA: string;

/** Two reports install A builds in July 2026, and one install B builds. */
let a1: Record<string, unknown>;
let a2: Record<string, unknown>;
let b1: Record<string, unknown>;

beforeAll(async () => {
  for (const name of IDENTITY_ENV) {
    saved[name] = process.env[name];
    delete process.env[name];
  }
  // No identity is pinned in the environment, so each install's own files under
  // its home decide every identifier, as they do on an install that sets none.
  root = mkdtempSync(join(tmpdir(), 'arp-identifiers-'));
  installA = join(root, 'install-a');

  process.env.OPENA2A_HOME = installA;
  const first = await buildSignedSubmission(exfiltration, { now: new Date('2026-07-01T00:00:00Z') });
  const second = await buildSignedSubmission(harvest, { count: 3, now: new Date('2026-07-31T23:59:59Z') });
  // The log test reads this record back: the first report, audited as sent.
  const audited = await appendAuditRecord({
    ...queuedRecord(
      first.request,
      first.body,
      `https://registry.example${SIGNATURE_INGEST_PATH}`,
      '2026-07-01T00:00:01.000Z',
    ),
    phase: 'sent',
  });
  expect(audited).toBe(true);

  process.env.OPENA2A_HOME = join(root, 'install-b');
  const other = await buildSignedSubmission(exfiltration, { now: new Date('2026-07-01T00:00:00Z') });

  a1 = JSON.parse(first.body) as Record<string, unknown>;
  a2 = JSON.parse(second.body) as Record<string, unknown>;
  b1 = JSON.parse(other.body) as Record<string, unknown>;
});

afterAll(() => {
  for (const name of IDENTITY_ENV) {
    const value = saved[name];
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  rmSync(root, { recursive: true, force: true });
});

/** Fields with one value across install A's reports and another in install B's. */
function perInstallIdentifiers(): string[] {
  return Object.keys(a1)
    .filter((field) => a1[field] === a2[field] && a1[field] !== b1[field])
    .sort();
}

/** Collapse line breaks and indentation, so a name wrapped across lines reads as one phrase. */
function flat(text: string): string {
  return text.replace(/\s+/g, ' ');
}

function readmeParagraph(): string {
  const readme = readFileSync(join(__dirname, '..', '..', '..', 'README.md'), 'utf-8');
  const start = readme.indexOf('**Structural signature telemetry (off by default, opt-in).**');
  const end = readme.indexOf('Turn it on with either of:', start);
  expect(start).toBeGreaterThanOrEqual(0);
  expect(end).toBeGreaterThan(start);
  return readme.slice(start, end);
}

/** What `aim-arp telemetry log` prints for install A's audit log, split at its one payload line. */
async function readLog(): Promise<{ payload: string; footer: string }> {
  const lines: string[] = [];
  const spy = vi.spyOn(console, 'log').mockImplementation((...args: unknown[]) => {
    lines.push(args.join(' '));
  });
  try {
    process.env.OPENA2A_HOME = installA;
    expect(await telemetryLog()).toBe(0);
  } finally {
    spy.mockRestore();
  }
  const at = lines.findIndex((line) => line.trimStart().startsWith('payload:'));
  expect(at).toBeGreaterThanOrEqual(0);
  return { payload: lines[at], footer: lines.slice(at + 1).join('\n') };
}

describe('the per-install identifiers a signature telemetry report carries', () => {
  it('are the fields whose names this test looks for in the texts', () => {
    expect(perInstallIdentifiers()).toEqual(Object.keys(NAMED_AS).sort());
  });

  it('are all in the payload `aim-arp telemetry log` prints above its footer', async () => {
    const { payload } = await readLog();
    const printed = JSON.parse(payload.trim().slice('payload:'.length)) as Record<string, unknown>;
    for (const field of perInstallIdentifiers()) {
      expect(printed[field], field).toBe(a1[field]);
    }
  });
});

const TEXTS: Array<[string, () => string | Promise<string>]> = [
  ['the README paragraph on structural signature telemetry', readmeParagraph],
  ['disclosureText(), the first-run notice and `aim-arp telemetry disclosure`', () => disclosureText()],
  ['the `aim-arp telemetry --help` text', () => telemetryHelpText()],
  ['the footer `aim-arp telemetry log` prints under the payloads', async () => (await readLog()).footer],
];

describe.each(TEXTS)('%s', (_label, read) => {
  it.each(Object.keys(NAMED_AS))('names the %s field every report carries', async (field) => {
    expect(flat(await read())).toMatch(NAMED_AS[field]);
  });
});
