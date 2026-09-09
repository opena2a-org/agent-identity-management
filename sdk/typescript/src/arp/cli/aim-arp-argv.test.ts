/**
 * AIM-12 items 1 and 3 — the shipped `aim-arp` bin's argv surface.
 *
 * Item 1: the release test typed the bare words `help` and `version` (the
 * spelling every modern CLI accepts) and got "Unknown command" + exit 1; and
 * `aim-arp telemetry --version` was answered with "Unknown telemetry
 * subcommand" instead of the version.
 *
 * Item 3: `telemetry log abc` silently showed 20 records (parseInt || 20),
 * `telemetry log 5 5` silently ignored the extra argument, while `log -1` was
 * refused as an unknown option — an asymmetry where garbage on the left of the
 * dash errors and garbage on the right is swallowed.
 *
 * Same no-exec pattern as telemetry-help-noexec.test.ts: import the dispatch
 * the thin bin entry calls, drive it with argv arrays, spy the console. No
 * subprocess, no network.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { runAimArp } from './aim-arp-main';
import { SDK_VERSION } from '../../version';

let home: string;
let savedHome: string | undefined;
let logLines: string[];
let errLines: string[];

beforeEach(() => {
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-argv-')), 'opena2a-home');
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

describe('bare help/version words work like their flag spellings', () => {
  it('AIM-12.AC3 item 1: bare `help` prints usage and exits 0', async () => {
    const code = await runAimArp(['help']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain('USAGE');
    expect(errLines.join('\n')).toBe('');
  });

  it('AIM-12.AC3 item 1: bare `version` prints the version and exits 0', async () => {
    const code = await runAimArp(['version']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain(`aim-arp v${SDK_VERSION}`);
    expect(errLines.join('\n')).toBe('');
  });

  it('AIM-12.AC3 item 1: `telemetry --version` answers with the version, not an unknown-subcommand error', async () => {
    const code = await runAimArp(['telemetry', '--version']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain(`aim-arp v${SDK_VERSION}`);
    expect(errLines.join('\n')).not.toContain('Unknown telemetry subcommand');
  });

  it('an unknown bare word is still an error (the fix adds words, not leniency)', async () => {
    const code = await runAimArp(['frobnicate']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('frobnicate');
  });
});

describe('`telemetry log` validates its count argument instead of silently defaulting', () => {
  it('AIM-12.AC3 item 3: `log abc` is an error naming the argument, not a silent 20', async () => {
    const code = await runAimArp(['telemetry', 'log', 'abc']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('abc');
    expect(logLines.join('\n')).not.toContain('Telemetry audit log');
  });

  it('AIM-12.AC3 item 3: `log 0` is an error (the count must be a positive integer)', async () => {
    const code = await runAimArp(['telemetry', 'log', '0']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('0');
  });

  it('AIM-12.AC3 item 3: `log 5 5` is an error — extra arguments are refused, matching the option side', async () => {
    const code = await runAimArp(['telemetry', 'log', '5', '5']);
    expect(code).toBe(1);
    expect(logLines.join('\n')).not.toContain('Telemetry audit log');
  });

  it('AIM-12.AC3 item 3: a stray positional on a no-argument subcommand is an error too', async () => {
    const code = await runAimArp(['telemetry', 'status', 'extra']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('extra');
    expect(logLines.join('\n')).not.toContain('State:');
  });

  it('`log 10` still works (validation refuses garbage, not the feature)', async () => {
    const code = await runAimArp(['telemetry', 'log', '10']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain('Telemetry audit log');
  });

  it('`log` with no count still defaults to 20 records', async () => {
    const code = await runAimArp(['telemetry', 'log']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain('Telemetry audit log');
  });
});
