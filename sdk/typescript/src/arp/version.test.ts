import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'fs';
import { join } from 'path';
import { VERSION as ARP_VERSION } from './index';
import { VERSION as ROOT_VERSION } from '../index';

/**
 * `@opena2a/aim-sdk/arp` exports `VERSION`, and so does `@opena2a/aim-sdk`.
 * Same export name, two subpaths of one npm package. They used to disagree:
 * the root read the package version while ARP read a hand-maintained `0.2.0`
 * that no release ever bumped, so 1.0.0 through 1.2.0 all reported the same
 * ARP version.
 *
 * The value is not decoration. Both telemetry channels put it on the wire as
 * `User-Agent: OpenA2A-ARP/<VERSION>`, which is the only build signal the
 * registry records for a submission, so a frozen literal makes every ingested
 * signature look like it came from the same sensor build.
 *
 * `src/version.test.ts` holds the root export to package.json. These hold the
 * ARP export to the same line, and hold the wire header to the constant rather
 * than to a literal someone typed.
 */
describe('the version ARP reports is the version npm published', () => {
  const pkg = JSON.parse(
    readFileSync(join(__dirname, '..', '..', 'package.json'), 'utf-8'),
  ) as { version: string };

  it('the ARP subpath export equals package.json version', () => {
    expect(ARP_VERSION).toBe(pkg.version);
  });

  it('and the root subpath export agrees with it', () => {
    // One package cannot have two version lines; the two subpaths are how it
    // grew one.
    expect(ARP_VERSION).toBe(ROOT_VERSION);
  });

  it('and package.json actually carries a version, so this is not vacuous', () => {
    // Guards the oracle: a silently-undefined read would make the assertions
    // above compare undefined to undefined.
    expect(pkg.version).toMatch(/^\d+\.\d+\.\d+/);
  });
});

describe('the ARP User-Agent is built from that constant, not from a literal', () => {
  /** Every `.ts` under src/, excluding tests -- the class, not the two known sites. */
  function sourceFiles(dir: string, found: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name);
      if (entry.isDirectory()) {
        sourceFiles(full, found);
      } else if (entry.name.endsWith('.ts') && !entry.name.endsWith('.test.ts')) {
        found.push(full);
      }
    }
    return found;
  }

  /**
   * Code lines only. Docstrings quote the frozen `OpenA2A-ARP/0.2.0` on purpose
   * -- this file and `arp/index.ts` both explain the defect by naming it -- and
   * a guard that cannot tell prose from a header would force the explanation
   * out. Comment detection is line-granular: a whole-line `//`, `/*` or ` *`.
   * A header hidden inside a multi-line template on a line that opens with `*`
   * would slip through, which is not a shape a refactor produces.
   */
  function codeOf(file: string): string {
    return readFileSync(file, 'utf-8')
      .split('\n')
      .filter((line) => {
        const t = line.trimStart();
        return !t.startsWith('//') && !t.startsWith('*') && !t.startsWith('/*');
      })
      .join('\n');
  }

  const src = join(__dirname, '..');
  const files = sourceFiles(src);

  it('scans a non-empty set of source files, so this is not vacuous', () => {
    expect(files.length).toBeGreaterThan(50);
  });

  it('finds the header in code, so the pattern still matches how it is written', () => {
    const withHeader = files.filter((f) => codeOf(f).includes('OpenA2A-ARP/'));
    // Both telemetry channels send it: the GTIN channel and the signature
    // emitter. If a refactor renames the product token, or the comment filter
    // starts eating code, this goes red rather than silently guarding nothing.
    expect(withHeader.length).toBeGreaterThanOrEqual(2);
  });

  it('never hardcodes a version after the product token', () => {
    // Matches `OpenA2A-ARP/0`..`OpenA2A-ARP/9`; the legitimate form is
    // `OpenA2A-ARP/${VERSION}`, which starts with `$`.
    const offenders = files
      .filter((f) => /OpenA2A-ARP\/\d/.test(codeOf(f)))
      .map((f) => f.slice(src.length + 1));

    expect(offenders).toEqual([]);
  });
});
