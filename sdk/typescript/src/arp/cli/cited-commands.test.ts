/**
 * #400: the install-time disclosure once told users to run `arp telemetry log`
 * — a command no installed package registered. A comment asking humans to keep
 * docs and commands in sync is not a mechanism; this test is the mechanism,
 * the same shape as src/version.test.ts pinning SDK_VERSION to package.json.
 *
 * It walks every `aim-arp telemetry <sub>` citation in the shipped sources and
 * asserts each cited subcommand is registered by the shipped bin, that nothing
 * cites the held `register` subcommand, and that the dead `arp telemetry`
 * invocation never reappears anywhere under src/.
 */

import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'fs';
import { join, relative } from 'path';
import { disclosureText } from '../telemetry/signature';
import { TELEMETRY_SUBCOMMANDS, telemetryHelpText, SHIPPED_INVOCATION } from './telemetry';

const SRC_ROOT = join(__dirname, '..', '..');

function walkTsFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walkTsFiles(p));
    else if (name.endsWith('.ts') && !name.endsWith('.test.ts')) out.push(p);
  }
  return out;
}

/** Every `aim-arp telemetry <word>` citation in the given text. */
function citedSubcommands(text: string): string[] {
  const cites = [...text.matchAll(/aim-arp telemetry ([a-z-]+)/g)].map((m) => m[1]);
  // `npx @opena2a/aim-sdk telemetry <word>` resolves to the same bin.
  cites.push(...[...text.matchAll(/@opena2a\/aim-sdk telemetry ([a-z-]+)/g)].map((m) => m[1]));
  return cites;
}

const NON_SUBCOMMAND_TOKENS = new Set(['--help', '-h']);

describe('every cited telemetry command is one the shipped bin registers', () => {
  const files = walkTsFiles(SRC_ROOT);

  it('walks a non-trivial source tree (guard: the walker itself works)', () => {
    expect(files.length).toBeGreaterThan(20);
    expect(files.some((f) => f.endsWith('disclosure.ts'))).toBe(true);
  });

  for (const file of walkTsFiles(SRC_ROOT)) {
    const text = readFileSync(file, 'utf8');
    const cites = citedSubcommands(text).filter((c) => !NON_SUBCOMMAND_TOKENS.has(c));
    if (cites.length === 0) continue;
    const rel = relative(SRC_ROOT, file);

    it(`${rel} cites only registered subcommands`, () => {
      for (const cite of cites) {
        expect(TELEMETRY_SUBCOMMANDS, `${rel} cites unregistered '${cite}'`).toContain(cite);
      }
    });
  }

  it('the rendered disclosure text cites only registered subcommands', () => {
    for (const cite of citedSubcommands(disclosureText())) {
      expect(TELEMETRY_SUBCOMMANDS).toContain(cite as (typeof TELEMETRY_SUBCOMMANDS)[number]);
    }
    // The disclosure must actually point at the CLI (an empty cite list would
    // make the loop above pass vacuously).
    expect(citedSubcommands(disclosureText()).length).toBeGreaterThanOrEqual(2);
  });

  it('the telemetry help text documents exactly the registered subcommands', () => {
    const help = telemetryHelpText();
    for (const sub of TELEMETRY_SUBCOMMANDS) {
      expect(help).toContain(sub);
    }
    expect(help).not.toContain('register');
  });
});

describe('the held register subcommand stays out of the shipped surface', () => {
  it('no shipped string cites register via the shipped invocation', () => {
    for (const file of walkTsFiles(SRC_ROOT)) {
      const cites = citedSubcommands(readFileSync(file, 'utf8'));
      expect(cites, `${relative(SRC_ROOT, file)} cites the held 'register'`).not.toContain(
        'register',
      );
    }
  });

  it('register is not a registered subcommand of the shipped bin', () => {
    expect(TELEMETRY_SUBCOMMANDS).not.toContain('register' as never);
  });
});

describe('the dead invocation never reappears', () => {
  it("no source file cites bare `arp telemetry` (the command #400 caught)", () => {
    for (const file of walkTsFiles(SRC_ROOT)) {
      const text = readFileSync(file, 'utf8');
      // `arp` as its own word: excludes aim-arp (hyphen before) and arp-guard
      // (hyphen after the token boundary is fine — 'arp-guard telemetry' does
      // not match because the char after 'arp' is '-').
      const bare = text.match(/(?<![\w-])arp telemetry/);
      expect(bare, `${relative(SRC_ROOT, file)} cites dead 'arp telemetry'`).toBeNull();
    }
  });

  it('the shipped invocation constant is the aim-arp form', () => {
    expect(SHIPPED_INVOCATION).toBe('aim-arp telemetry');
  });
});
