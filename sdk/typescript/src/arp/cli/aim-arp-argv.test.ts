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
import { cliVersion } from '../../build-version';

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
  it('bare `help` prints usage and exits 0', async () => {
    const code = await runAimArp(['help']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain('USAGE');
    expect(errLines.join('\n')).toBe('');
  });

  it('bare `version` prints the version and exits 0', async () => {
    const code = await runAimArp(['version']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain(`aim-arp v${SDK_VERSION}`);
    expect(errLines.join('\n')).toBe('');
  });

  it('`telemetry --version` answers with the version, not an unknown-subcommand error', async () => {
    const code = await runAimArp(['telemetry', '--version']);
    expect(code).toBe(0);
    expect(logLines.join('\n')).toContain(`aim-arp v${SDK_VERSION}`);
    expect(errLines.join('\n')).not.toContain('Unknown telemetry subcommand');
  });

  it('`-V` prints the version and exits 0, as `aim-sdk -V` does (#755)', async () => {
    expect(await runAimArp(['-V'])).toBe(0);
    expect(logLines).toEqual([`aim-arp v${cliVersion()}`]);
    expect(errLines).toEqual([]);
  });

  it('`telemetry -V` answers with the version, as `telemetry --version` does (#755)', async () => {
    expect(await runAimArp(['telemetry', '-V'])).toBe(0);
    expect(logLines).toEqual([`aim-arp v${cliVersion()}`]);
    expect(errLines).toEqual([]);
  });

  it('an unknown bare word is still an error (the fix adds words, not leniency)', async () => {
    const code = await runAimArp(['frobnicate']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('frobnicate');
  });
});

describe('a build the release job did not make says so in each version line', () => {
  // Under vitest the tsup build constant is not defined, so this is a source build.
  const line = `aim-arp v${cliVersion()}`;

  it('the qualifier follows the version', () => {
    expect(line).toBe(
      `aim-arp v${SDK_VERSION}+source (built from source; not the published ${SDK_VERSION})`,
    );
  });

  it('`--version` prints the qualified version', async () => {
    expect(await runAimArp(['--version'])).toBe(0);
    expect(logLines).toEqual([line]);
  });

  it('`telemetry --version` prints the qualified version', async () => {
    expect(await runAimArp(['telemetry', '--version'])).toBe(0);
    expect(logLines).toEqual([line]);
  });

  it('the help header prints the qualified version and no em dash', async () => {
    expect(await runAimArp(['--help'])).toBe(0);
    const header = logLines.join('\n').split('\n').slice(0, 3);
    expect(header).toEqual(['', `  ${line}`, '  OpenA2A AIM SDK CLI']);
    expect(header.join('\n')).not.toContain('—');
  });
});

describe('`telemetry log` validates its count argument instead of silently defaulting', () => {
  it('`log abc` is an error naming the argument, not a silent 20', async () => {
    const code = await runAimArp(['telemetry', 'log', 'abc']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('abc');
    expect(logLines.join('\n')).not.toContain('Telemetry audit log');
  });

  it('`log 0` is an error (the count must be a positive integer)', async () => {
    const code = await runAimArp(['telemetry', 'log', '0']);
    expect(code).toBe(1);
    expect(errLines.join('\n')).toContain('0');
  });

  it('`log 5 5` is an error — extra arguments are refused, matching the option side', async () => {
    const code = await runAimArp(['telemetry', 'log', '5', '5']);
    expect(code).toBe(1);
    expect(logLines.join('\n')).not.toContain('Telemetry audit log');
  });

  it('a stray positional on a no-argument subcommand is an error too', async () => {
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

describe('an argument aim-arp does not know is named on stderr with a pointer to --help (#852)', () => {
  it('an unknown option is called an option, and nothing is printed on stdout', async () => {
    expect(await runAimArp(['--frob'])).toBe(1);
    expect(errLines).toEqual(['Unknown option: --frob', 'Run: aim-arp --help']);
    expect(logLines).toEqual([]);
  });

  it('an unknown bare word is called a command, and nothing is printed on stdout', async () => {
    expect(await runAimArp(['frobnicate'])).toBe(1);
    expect(errLines).toEqual(['Unknown command: frobnicate', 'Run: aim-arp --help']);
    expect(logLines).toEqual([]);
  });

  it('a bootstrap token given as the argument is echoed redacted', async () => {
    const fake = 'aim_ob_FAKE';
    expect(await runAimArp([fake])).toBe(1);
    expect(await runAimArp([`--paste=${fake}`])).toBe(1);
    expect(errLines).toEqual([
      'Unknown command: aim_ob_[REDACTED]',
      'Run: aim-arp --help',
      'Unknown option: --paste=aim_ob_[REDACTED]',
      'Run: aim-arp --help',
    ]);
    expect(errLines.join('\n')).not.toContain(fake);
    expect(logLines).toEqual([]);
  });
});

describe('the telemetry text has no capitals for emphasis and no em dash (#852)', () => {
  // Words in capitals that are not emphasis: the channel state, and names.
  const NOT_EMPHASIS = new Set(['ON', 'OFF', 'AIM', 'ARP', 'GTIN', 'IP']);

  /** Everything printed so far, with the random test home replaced: its path is not copy. */
  function printed(): string {
    return [...logLines, ...errLines].join('\n').split(home).join('<home>');
  }

  function expectPlainCopy(text: string): void {
    const emphasis = [...text.matchAll(/\b[A-Z]{2,}\b/g)].map((m) => m[0]).filter((w) => !NOT_EMPHASIS.has(w));
    const emDashes = text.split('—').length - 1;
    expect({ emphasis, emDashes }).toEqual({ emphasis: [], emDashes: 0 });
  }

  it('telemetry --help, telemetry disclosure and telemetry opt-out', async () => {
    for (const sub of ['--help', 'disclosure', 'opt-out']) {
      expect(await runAimArp(['telemetry', sub]), sub).toBe(0);
    }
    const text = printed();
    expectPlainCopy(text);
    // Each of the three was printed: an empty capture would pass the check above.
    expect(text).toContain('opt-out       Disable all OpenA2A telemetry');
    expect(text).toContain('The shape of an anomalous behavior');
    expect(text).toContain('What is shared: the structural shape of an anomalous behavior');
    expect(text).toContain('OpenA2A telemetry disabled (signature, GTIN and fleet-gradient channels).');
  });

  it('telemetry opt-in while an environment variable keeps telemetry off', async () => {
    const saved = process.env.OPENA2A_TELEMETRY_OPTOUT;
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    try {
      expect(await runAimArp(['telemetry', 'opt-in'])).toBe(0);
    } finally {
      if (saved === undefined) delete process.env.OPENA2A_TELEMETRY_OPTOUT;
      else process.env.OPENA2A_TELEMETRY_OPTOUT = saved;
    }
    const text = printed();
    expectPlainCopy(text);
    expect(text).toContain('Note: telemetry is still disabled by an env var or config');
  });
});
