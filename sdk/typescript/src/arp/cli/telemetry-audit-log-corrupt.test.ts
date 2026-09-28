/**
 * #416: a telemetry audit log holding only unparseable lines made
 * `aim-arp telemetry log` print "No telemetry has been sent yet" (exit 0) and
 * `telemetry status` print "Sent: 0". The audit log is the surface the
 * disclosure tells users to trust, so a corrupt or unreadable log must be
 * reported as such, never as an empty one. A JSON line without the writer's
 * fields used to throw a raw TypeError (`padEnd`) from the renderer.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { chmodSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { telemetryLog, telemetryStatus } from './telemetry';
import { readAuditLog, readAuditRecords } from '../telemetry/signature/audit-log';

let home: string;
let savedHome: string | undefined;
let lines: string[];
let savedExitCode: typeof process.exitCode;

const SENT = JSON.stringify({
  ts: '2026-09-27T00:00:00.000Z',
  phase: 'sent',
  endpoint: 'https://registry.example/api/v1/telemetry/signatures',
  body: '{}',
  behavioralHash: 'h',
  techniqueId: 'T-0001',
  severity: 'high',
  outcome: 'blocked',
});

function writeLog(content: string): string {
  mkdirSync(home, { recursive: true });
  const p = join(home, 'telemetry-audit.log');
  writeFileSync(p, content);
  return p;
}

beforeEach(() => {
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-audit-')), 'opena2a-home');
  savedHome = process.env.OPENA2A_HOME;
  process.env.OPENA2A_HOME = home;
  savedExitCode = process.exitCode;
  lines = [];
  vi.spyOn(console, 'log').mockImplementation((...a: unknown[]) => {
    lines.push(a.join(' '));
  });
});

afterEach(() => {
  if (savedHome === undefined) delete process.env.OPENA2A_HOME;
  else process.env.OPENA2A_HOME = savedHome;
  process.exitCode = savedExitCode;
  vi.restoreAllMocks();
  rmSync(join(home, '..'), { recursive: true, force: true });
});

describe('readAuditLog', () => {
  it('counts unparseable and schema-less lines instead of dropping them silently', async () => {
    writeLog(`not-json garbage line\n{"half":\n${SENT}\n{"phase":"sent"}\n`);
    const read = await readAuditLog(100);
    expect(read.records).toHaveLength(1);
    expect(read.unparseable).toBe(3);
    expect(read.readError).toBeUndefined();
    // The long-standing export keeps its shape: records only.
    expect(await readAuditRecords(100)).toHaveLength(1);
  });

  it('reports a log that exists but cannot be read', async () => {
    const p = writeLog(`${SENT}\n`);
    chmodSync(p, 0o000);
    try {
      const read = await readAuditLog(100);
      // Running as root can still read a 000 file; only assert when denied.
      if (read.records.length === 0) expect(read.readError).toBeTruthy();
    } finally {
      chmodSync(p, 0o600);
    }
  });
});

describe('telemetry log on a corrupt audit log (#416)', () => {
  it("does not claim nothing was sent when every line is unparseable (the issue's repro)", async () => {
    writeLog('not-json garbage line\n{"half":\n');
    await telemetryLog();
    const out = lines.join('\n');
    expect(out).not.toContain('No telemetry has been sent yet');
    expect(out).toContain('2 line(s) could not be read as audit records; showing 0 record(s)');
    expect(out).toContain('this does not mean nothing was sent');
  });

  it('renders the readable records and notes the rest, without a TypeError', async () => {
    writeLog(`${SENT}\n{"phase":"sent"}\n`);
    await expect(telemetryLog()).resolves.toBeUndefined();
    const out = lines.join('\n');
    expect(out).toContain('1 line(s) could not be read as audit records; showing 1 record(s)');
    expect(out).toContain('T-0001');
  });

  it('reports an unreadable log and exits 1 instead of showing an empty one', async () => {
    const p = writeLog(`${SENT}\n`);
    chmodSync(p, 0o000);
    try {
      if ((await readAuditLog(1)).readError === undefined) return; // root can read a 000 file
      lines.length = 0;
      await telemetryLog();
      const out = lines.join('\n');
      expect(out).toContain('Could not read the audit log');
      expect(out).not.toContain('No telemetry has been sent yet');
      expect(process.exitCode).toBe(1);
      lines.length = 0;
      await telemetryStatus(undefined);
      expect(lines.join('\n')).toContain('Sent:         unknown (audit log could not be read');
    } finally {
      chmodSync(p, 0o600);
    }
  });

  it('still says nothing was sent for a log that does not exist', async () => {
    await telemetryLog();
    expect(lines.join('\n')).toContain('No telemetry has been sent yet');
  });
});

describe('telemetry status on a corrupt audit log (#416)', () => {
  it('names the unreadable lines next to the counts', async () => {
    writeLog('not-json garbage line\n{"half":\n');
    await telemetryStatus(undefined);
    const out = lines.join('\n');
    expect(out).toContain('Sent:         0');
    expect(out).toContain('Unreadable:   2 audit log line(s) could not be read');
  });

  it('prints no unreadable line for a clean log', async () => {
    writeLog(`${SENT}\n`);
    await telemetryStatus(undefined);
    const out = lines.join('\n');
    expect(out).toContain('Sent:         1');
    expect(out).not.toContain('Unreadable:');
  });
});
