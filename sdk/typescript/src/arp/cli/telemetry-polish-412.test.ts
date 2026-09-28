/**
 * Issue #412: aim-arp CLI polish from the 1.3.0 walkthroughs. Items already
 * fixed on main before this change (positional validation, `log` count
 * validation, `telemetry --version`, `aim-arp help`, footer attribution of the
 * env opt-outs) keep their own tests; this file pins the rest.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { runTelemetrySubcommand, telemetryHelpText } from './telemetry';
import { registerAction } from './register-dispatch';
import { isOptedOut, optOutMarkerExists } from '../telemetry/signature';

let root: string;
let home: string;
let savedHome: string | undefined;
let logLines: string[];
let errLines: string[];

const out = () => logLines.join('\n');
const err = () => errLines.join('\n');
const canDropPermissions = typeof process.getuid === 'function' && process.getuid() !== 0;

beforeEach(() => {
  root = mkdtempSync(join(tmpdir(), 'aim-arp-412-'));
  home = join(root, 'opena2a-home');
  savedHome = process.env.OPENA2A_HOME;
  process.env.OPENA2A_HOME = home;
  logLines = [];
  errLines = [];
  vi.spyOn(console, 'log').mockImplementation((...a: unknown[]) => {
    logLines.push(a.join(' '));
  });
  vi.spyOn(console, 'error').mockImplementation((...a: unknown[]) => {
    errLines.push(a.join(' '));
  });
});

afterEach(() => {
  if (existsSync(home)) chmodSync(home, 0o755);
  if (savedHome === undefined) delete process.env.OPENA2A_HOME;
  else process.env.OPENA2A_HOME = savedHome;
  vi.restoreAllMocks();
  rmSync(root, { recursive: true, force: true });
});

function writeAuditLog(lines: string[], trailing = ''): void {
  mkdirSync(home, { recursive: true });
  writeFileSync(join(home, 'telemetry-audit.log'), lines.join('\n') + '\n' + trailing);
}

const GOOD = JSON.stringify({
  ts: '2026-09-01T00:00:00Z', phase: 'sent', techniqueId: 'T1', severity: 'high',
  outcome: 'blocked', body: '{}', endpoint: 'x', behavioralHash: 'h',
});

describe('log survives a corrupt audit log (P2)', () => {
  it('skips a record missing fields and a torn final line, renders the rest, and says so', async () => {
    writeAuditLog([GOOD, JSON.stringify({ ts: '2026-09-02T00:00:00Z', phase: 'sent' })], '{"ts":"2026-09-03T0');
    const code = await runTelemetrySubcommand('log', []);
    expect(code).toBe(0);
    expect(out()).toContain('T1');
    expect(out()).toContain('Last 1 record(s)');
    expect(out()).toContain('2 line(s) could not be read as audit records; showing 1 record(s)');
    expect(err()).toBe('');
  });

  it('a log with only corrupt lines is not reported as nothing sent', async () => {
    writeAuditLog(['not json']);
    expect(await runTelemetrySubcommand('log', [])).toBe(0);
    expect(out()).not.toContain('No telemetry has been sent yet');
    expect(out()).toContain('1 line(s) could not be read as audit records; showing 0 record(s)');
    expect(out()).toContain('this does not mean nothing was sent');
  });
});

describe.skipIf(!canDropPermissions)('an unreadable state directory is reported, never shown as defaults (item 2)', () => {
  it('status says the state is unknown instead of "OFF (not turned on)" when an opt-out marker is hidden', async () => {
    mkdirSync(home, { recursive: true });
    writeFileSync(join(home, 'telemetry-optout'), 'x');
    chmodSync(home, 0o000);

    const code = await runTelemetrySubcommand('status', []);
    expect(code).toBe(1);
    expect(err()).toContain('Cannot read the telemetry state directory');
    expect(err()).toContain('unknown, not off');
    expect(out()).not.toContain('not turned on');
  });

  it('log says it cannot read instead of "No telemetry has been sent yet"', async () => {
    writeAuditLog([GOOD]);
    chmodSync(home, 0o000);
    expect(await runTelemetrySubcommand('log', [])).toBe(1);
    expect(out()).not.toContain('No telemetry has been sent yet');
  });

  it('an unreadable home counts as opted out: consent that cannot be read is not consent', () => {
    mkdirSync(home, { recursive: true });
    chmodSync(home, 0o000);
    expect(optOutMarkerExists()).toBe(true);
    expect(isOptedOut()).toBe(true);
  });

  it('opt-out into a read-only home names the failure, says it did not take effect, and gives the env route', async () => {
    mkdirSync(home, { recursive: true });
    chmodSync(home, 0o555);
    const code = await runTelemetrySubcommand('opt-out', ['--no-purge']);
    expect(code).toBe(1);
    expect(err()).toContain('could not write');
    expect(err()).toContain('EACCES');
    expect(err()).toContain('did NOT take effect');
    expect(err()).toContain('OPENA2A_TELEMETRY=off');
    expect(err()).not.toMatch(/^Error: /m);
  });
});

it('a missing home is not an error and not an opt-out', () => {
  expect(optOutMarkerExists()).toBe(false);
  expect(isOptedOut()).toBe(false);
});

describe('help (items 5 and 6)', () => {
  it('opt-out --help prints opt-out\'s own usage and flag, executes nothing', async () => {
    expect(await runTelemetrySubcommand('opt-out', ['--help'])).toBe(0);
    expect(out()).toContain('aim-arp telemetry opt-out [--no-purge]');
    expect(out()).toContain('--no-purge    Keep already-sent signatures');
    expect(out()).not.toContain('log [N]');
    expect(existsSync(join(home, 'telemetry-optout'))).toBe(false);
  });

  it('log --help documents N', async () => {
    expect(await runTelemetrySubcommand('log', ['-h'])).toBe(0);
    expect(out()).toContain('aim-arp telemetry log [N]');
    expect(out()).toContain('positive integer');
  });

  it('status --help says it takes no arguments', async () => {
    expect(await runTelemetrySubcommand('status', ['--help'])).toBe(0);
    expect(out()).toContain('Takes no arguments.');
  });

  it('the group help no longer promises an install-time disclosure', () => {
    expect(telemetryHelpText()).not.toContain('install-time');
    expect(telemetryHelpText()).toContain('disclosure    Print the telemetry disclosure');
  });
});

describe('an option before its subcommand (item 7)', () => {
  it('names the order that works', async () => {
    expect(await runTelemetrySubcommand('--no-purge', ['opt-out'])).toBe(1);
    expect(err()).toContain('Options go after the subcommand: aim-arp telemetry opt-out --no-purge');
    expect(existsSync(join(home, 'telemetry-optout'))).toBe(false);
  });
});

describe('layout (item 3) and the local-install footer', () => {
  it('disclosure uses the two-space indent and a box-drawing rule', async () => {
    expect(await runTelemetrySubcommand('disclosure', [])).toBe(0);
    const lines = out().split('\n').filter((l) => l.length > 0);
    expect(lines.every((l) => l.startsWith('  '))).toBe(true);
    expect(out()).toContain('  ─────');
    expect(out()).not.toMatch(/^\s*-{10,}\s*$/m);
  });

  it('status names the npx form once for a bin that is not on PATH', async () => {
    expect(await runTelemetrySubcommand('status', [])).toBe(0);
    expect(out()).toContain('npx @opena2a/aim-sdk telemetry <subcommand>');
  });
});

describe('internal register guard has a test (seam extracted from index.ts)', () => {
  it('a help request is help, never a run', () => {
    expect(registerAction(['--help'])).toEqual({ kind: 'help' });
    expect(registerAction(['-h', '--bogus'])).toEqual({ kind: 'help' });
  });

  it('an unknown option or a stray argument is an error, never a run', () => {
    expect(registerAction(['--bogus'])).toEqual({ kind: 'error', message: 'Unknown option for register: --bogus' });
    expect(registerAction(['now'])).toEqual({ kind: 'error', message: 'Unexpected argument for register: now' });
  });

  it('no arguments runs', () => {
    expect(registerAction([])).toEqual({ kind: 'run' });
  });
});
