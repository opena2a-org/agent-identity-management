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
import { runAimArp } from './aim-arp-main';
import { readAuditLog, readAuditRecords } from '../telemetry/signature/audit-log';

const canDropPermissions = typeof process.getuid === 'function' && process.getuid() !== 0;

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
    await expect(telemetryLog()).resolves.toBe(0);
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
      // The exit code is the return value; the handler leaves process.exitCode
      // to the bin entry, which sets it from that value.
      expect(await telemetryLog()).toBe(1);
      const out = lines.join('\n');
      expect(out).toContain('Could not read the audit log');
      expect(out).not.toContain('No telemetry has been sent yet');
      expect(process.exitCode).toBe(savedExitCode);
      lines.length = 0;
      expect(await telemetryStatus(undefined)).toBe(1);
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

describe.skipIf(!canDropPermissions)('the aim-arp bin exits 1 when the audit log cannot be read (#755)', () => {
  it('telemetry log and telemetry status return 1 through the dispatch the bin entry calls', async () => {
    const p = writeLog(`${SENT}\n`);
    chmodSync(p, 0o000);
    try {
      expect(await runAimArp(['telemetry', 'log'])).toBe(1);
      expect(lines.join('\n')).toContain('Could not read the audit log (EACCES), so what was sent cannot be shown.');
      lines.length = 0;
      expect(await runAimArp(['telemetry', 'status'])).toBe(1);
      expect(lines.join('\n')).toContain('Sent:         unknown (audit log could not be read: EACCES)');
    } finally {
      chmodSync(p, 0o600);
    }
  });

  it('telemetry log and telemetry status still return 0 on a readable log', async () => {
    writeLog(`${SENT}\n`);
    expect(await runAimArp(['telemetry', 'log'])).toBe(0);
    expect(await runAimArp(['telemetry', 'status'])).toBe(0);
  });
});

/** One audit record, numbered so a test can tell which records were shown. */
function record(n: number, phase: string): string {
  return JSON.stringify({ ...JSON.parse(SENT), ts: `2026-10-01T00:00:0${n}.000Z`, phase, techniqueId: `T-000${n}` });
}

/** Five records, then a line that is not JSON and a JSON line with an unknown phase. */
function writeSevenLineLog(): void {
  writeLog(
    [
      record(1, 'queued'),
      record(2, 'sent'),
      record(3, 'sent'),
      record(4, 'failed'),
      record(5, 'dropped'),
      'this line is not json',
      record(6, 'exploded'),
    ].join('\n') + '\n',
  );
}

describe('telemetry log N shows the last N records, not the records among the last N lines (#755)', () => {
  it('log 2 shows the last two records when the last two lines are not records', async () => {
    writeSevenLineLog();
    expect(await runAimArp(['telemetry', 'log', '2'])).toBe(0);
    const out = lines.join('\n');
    expect(out).toContain('2 line(s) could not be read as audit records; showing 2 record(s).');
    expect(out).toContain('Last 2 record(s)');
    expect(out).toContain('T-0004');
    expect(out).toContain('T-0005');
    expect(out).not.toContain('T-0003');
    expect(out).not.toContain('T-0006');
    expect(out).not.toContain('No readable records');
  });

  it('log 1 shows the last record', async () => {
    writeSevenLineLog();
    expect(await runAimArp(['telemetry', 'log', '1'])).toBe(0);
    const out = lines.join('\n');
    expect(out).toContain('2 line(s) could not be read as audit records; showing 1 record(s).');
    expect(out).toContain('T-0005');
    expect(out).not.toContain('T-0004');
  });

  it('log with no count shows every record of a log holding fewer than 20', async () => {
    writeSevenLineLog();
    expect(await runAimArp(['telemetry', 'log'])).toBe(0);
    const out = lines.join('\n');
    expect(out).toContain('2 line(s) could not be read as audit records; showing 5 record(s).');
    for (const id of ['T-0001', 'T-0002', 'T-0003', 'T-0004', 'T-0005']) expect(out).toContain(id);
  });

  it('readAuditLog counts the unreadable lines after the oldest record it returns', async () => {
    writeLog(`not json\n${record(1, 'sent')}\n${record(2, 'sent')}\nalso not json\n`);
    const two = await readAuditLog(2);
    expect(two.records.map((r) => r.techniqueId)).toEqual(['T-0001', 'T-0002']);
    expect(two.unparseable).toBe(1);
    // Fewer records than asked for: every line is read.
    const three = await readAuditLog(3);
    expect(three.records.map((r) => r.techniqueId)).toEqual(['T-0001', 'T-0002']);
    expect(three.unparseable).toBe(2);
    expect((await readAuditRecords(1)).map((r) => r.techniqueId)).toEqual(['T-0002']);
    expect(await readAuditRecords(0)).toEqual([]);
  });

  it('status counts the same seven-line log as it did before', async () => {
    writeSevenLineLog();
    expect(await runAimArp(['telemetry', 'status'])).toBe(0);
    const out = lines.join('\n');
    expect(out).toContain('Sent:         2');
    expect(out).toContain('Failed:       1');
    expect(out).toContain('Dropped:      1');
    expect(out).toContain('Unreadable:   2 audit log line(s) could not be read');
  });
});
