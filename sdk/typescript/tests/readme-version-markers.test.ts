import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * A version named in the README is a version a reader can install.
 *
 * README.md carried "Changed in 1.4.0." while the newest release was 1.3.1 and
 * CHANGELOG.md had no 1.4.0 heading, so a reader on 1.3.1 looked for a release
 * that did not exist. Every "Changed in X", "Added in X", "Deprecated in X",
 * "Removed in X" or "Fixed in X" marker in README.md must name a version that
 * has a `## [X]` release heading in CHANGELOG.md. A change that is not released
 * yet is described relative to the published version ("Changed after 1.3.1"),
 * never with a number that is still to come.
 */

const PACKAGE_ROOT = join(__dirname, '..');
const README = readFileSync(join(PACKAGE_ROOT, 'README.md'), 'utf-8');
const CHANGELOG = readFileSync(join(PACKAGE_ROOT, 'CHANGELOG.md'), 'utf-8');

const MARKER =
  /\b(?:changed|added|deprecated|removed|fixed) in v?(\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*)?)/gi;

/** The versions of the `## [X]` release headings, `[Unreleased]` aside. */
function releaseHeadings(changelog: string): Set<string> {
  const versions = new Set<string>();
  for (const match of changelog.matchAll(/^## \[([^\]]+)\]/gm)) {
    if (match[1] !== 'Unreleased') versions.add(match[1]);
  }
  return versions;
}

/** Each marker in `text` whose version has no release heading, as `line N: marker`. */
function unreleasedMarkers(text: string, released: Set<string>): string[] {
  const misses: string[] = [];
  text.split('\n').forEach((line, index) => {
    for (const match of line.matchAll(MARKER)) {
      if (!released.has(match[1])) misses.push(`line ${index + 1}: ${match[0]}`);
    }
  });
  return misses;
}

describe('README version markers name released versions', () => {
  const released = releaseHeadings(CHANGELOG);

  it('reads release headings from CHANGELOG.md, so the check is not vacuous', () => {
    expect(released.size).toBeGreaterThan(0);
    for (const version of released) expect(version).toMatch(/^\d+\.\d+\.\d+/);
    expect(released.has('Unreleased')).toBe(false);
  });

  it('every Changed/Added/Deprecated/Removed/Fixed in X marker in README.md has a ## [X] heading', () => {
    expect(unreleasedMarkers(README, released)).toEqual([]);
  });

  it('refuses the marker that named an unpublished version, and near misses of it', () => {
    const headings = releaseHeadings('## [Unreleased]\n\n## [1.3.1] - 2026-09-02\n');
    expect(unreleasedMarkers('> **Changed in 1.4.0.** Versions 1.0.0', headings)).toEqual([
      'line 1: Changed in 1.4.0',
    ]);
    expect(unreleasedMarkers('Added in v1.4.0', headings)).toEqual(['line 1: Added in v1.4.0']);
    expect(unreleasedMarkers('was changed in 1.4', headings)).toEqual(['line 1: changed in 1.4']);
    expect(unreleasedMarkers('Added in 1.3.1-rc1.', headings)).toEqual([
      'line 1: Added in 1.3.1-rc1',
    ]);
    expect(unreleasedMarkers('x\nDeprecated in Unreleased', headings)).toEqual([]);
  });

  it('accepts a marker that names a released version, and a change worded after it', () => {
    const headings = releaseHeadings('## [Unreleased]\n\n## [1.3.1] - 2026-09-02\n');
    expect(unreleasedMarkers('> **Changed in 1.3.1.** Text', headings)).toEqual([]);
    expect(unreleasedMarkers('> **Changed after 1.3.1.** Text', headings)).toEqual([]);
  });
});
