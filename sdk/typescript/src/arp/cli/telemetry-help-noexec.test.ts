/**
 * `aim-arp telemetry <sub> --help` must answer the question, never run the
 * command. Before this guarantee existed, `opt-out --help` performed a real
 * opt-out — the marker was written by the question — `opt-in --help` removed
 * one, and on a machine that had sent, `purge --help` would have fired the
 * right-to-delete from a help query (found in the 1.3.0 release test,
 * 2026-08-24). The dispatcher read only the subcommand and never looked at
 * the args, so unknown options were silently ignored too.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { existsSync, mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { runTelemetrySubcommand, telemetryStatus } from './telemetry';

let home: string;
let savedHome: string | undefined;
let logLines: string[];
let errLines: string[];

const marker = () => join(home, 'telemetry-optout');

beforeEach(() => {
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-help-')), 'opena2a-home');
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
  if (savedHome === undefined) delete process.env.OPENA2A_HOME;
  else process.env.OPENA2A_HOME = savedHome;
  vi.restoreAllMocks();
  rmSync(join(home, '..'), { recursive: true, force: true });
});

describe('--help on a subcommand prints help and executes nothing', () => {
  it('opt-out --help writes no marker and exits 0', async () => {
    const code = await runTelemetrySubcommand('opt-out', ['--help']);
    expect(code).toBe(0);
    expect(existsSync(marker())).toBe(false);
    expect(existsSync(home)).toBe(false);
    expect(logLines.join('\n')).not.toContain('DISABLED');
  });

  it('opt-out -h writes no marker and exits 0', async () => {
    const code = await runTelemetrySubcommand('opt-out', ['-h']);
    expect(code).toBe(0);
    expect(existsSync(marker())).toBe(false);
  });

  it('opt-in --help leaves an existing marker in place', async () => {
    await runTelemetrySubcommand('opt-out', ['--no-purge']);
    expect(existsSync(marker())).toBe(true);
    const code = await runTelemetrySubcommand('opt-in', ['--help']);
    expect(code).toBe(0);
    expect(existsSync(marker())).toBe(true);
  });

  it('status --help shows help, not status', async () => {
    const code = await runTelemetrySubcommand('status', ['--help']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).not.toContain('State:');
  });

  it('purge --help does not run purge', async () => {
    const code = await runTelemetrySubcommand('purge', ['--help']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).not.toContain('nothing to purge');
  });

  it('--help wins even alongside a recognised option', async () => {
    const code = await runTelemetrySubcommand('opt-out', ['--no-purge', '--help']);
    expect(code).toBe(0);
    expect(existsSync(marker())).toBe(false);
  });
});

describe('unrecognised options are an error, not a silent no-op', () => {
  it('status --bogus exits 1 and names the option', async () => {
    const code = await runTelemetrySubcommand('status', ['--bogus']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('--bogus');
    expect(logLines.join('\n')).not.toContain('State:');
  });

  it('log -5 exits 1 and names the option', async () => {
    const code = await runTelemetrySubcommand('log', ['-5']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('-5');
  });

  it('opt-out --no-purge still executes (the recognised option keeps working)', async () => {
    const code = await runTelemetrySubcommand('opt-out', ['--no-purge']);
    expect(code).toBe(0);
    expect(existsSync(marker())).toBe(true);
  });
});

describe('env-var opt-outs are attributed to the variable actually set', () => {
  const VARS = ['OPENA2A_TELEMETRY', 'OPENA2A_TELEMETRY_OPTOUT', 'ARP_TELEMETRY_DISABLED'] as const;
  const saved: Record<string, string | undefined> = {};

  beforeEach(() => {
    for (const v of VARS) {
      saved[v] = process.env[v];
      delete process.env[v];
    }
  });

  afterEach(() => {
    for (const v of VARS) {
      if (saved[v] === undefined) delete process.env[v];
      else process.env[v] = saved[v];
    }
  });

  it('OPENA2A_TELEMETRY_OPTOUT is named with a clearing instruction', async () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    await telemetryStatus(undefined);
    expect(logLines.join('\n')).toContain('OPENA2A_TELEMETRY_OPTOUT; unset it to clear');
  });

  it('ARP_TELEMETRY_DISABLED is named with a clearing instruction', async () => {
    process.env.ARP_TELEMETRY_DISABLED = '1';
    await telemetryStatus(undefined);
    expect(logLines.join('\n')).toContain('ARP_TELEMETRY_DISABLED; unset it to clear');
  });
});
