import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The README section that registers a first agent opens with a command that
 * runs on the published package.
 *
 * "Register a first agent from the terminal" led with
 * `npx @opena2a/aim-sdk init`, and the published 1.3.1 answers that command
 * with `Unknown command: init`: `init` is in the source build only. While
 * `init` is unreleased:
 *
 *   R1  the first fenced block of the section runs `init` from a source build
 *       (`node dist/arp/cli/aim-arp.js init`) and does not contain
 *       `npx @opena2a/aim-sdk init`;
 *   R2  the section carries the sentence that states when the npx form runs,
 *       on a line that opens with "Once a release after X is published", and X
 *       is the newest `## [X]` release heading in CHANGELOG.md, never a version
 *       still to come;
 *   R3  every line of README.md that contains `npx @opena2a/aim-sdk init` is on
 *       or after that sentence's line.
 *
 * Whether `init` is released is read from CHANGELOG.md, from its heading lines
 * only, never from prose. A `###` heading names `init` as a command when it
 * carries `init` in backticks or `aim-sdk init` / `aim-arp init`. `init` is
 * unreleased when the first such heading sits under `## [Unreleased]` and no
 * such heading sits under a `## [x.y.z]` release heading. Once a release
 * heading's section names it, R1 to R3 lift, so the section can lead with the
 * npx form again without an edit to this test.
 */

const PACKAGE_ROOT = join(__dirname, '..');
const README = readFileSync(join(PACKAGE_ROOT, 'README.md'), 'utf-8');
const CHANGELOG = readFileSync(join(PACKAGE_ROOT, 'CHANGELOG.md'), 'utf-8');

const SECTION_HEADING = '### Register a first agent from the terminal';
const NPX_INIT = /npx @opena2a\/aim-sdk init\b/;
const SOURCE_INIT = /node dist\/arp\/cli\/aim-arp\.js init\b/;
const CONDITION = /^Once a release after (\d+\.\d+\.\d+) is published\b/;

const INIT_HEADING = /^###\s.*(?:`init`|\baim-(?:sdk|arp) init\b)/;
const UNRELEASED_HEADING = /^## \[Unreleased\]/;
const RELEASE_HEADING = /^## \[(\d+\.\d+\.\d+[^\]]*)\]/;
const ANY_SECTION_HEADING = /^## /;
const FENCE = /^```/;
const README_HEADING = /^#{1,3} /;

type InitState = 'unreleased' | 'released' | 'not named';

/** Whether CHANGELOG.md names `init` in a release heading's section, read from heading lines only. */
function initReleaseState(changelog: string): InitState {
  let section: 'unreleased' | 'release' | 'other' = 'other';
  let first: 'unreleased' | 'release' | 'other' | undefined;
  let namedInRelease = false;
  for (const line of changelog.split('\n')) {
    if (UNRELEASED_HEADING.test(line)) section = 'unreleased';
    else if (RELEASE_HEADING.test(line)) section = 'release';
    else if (ANY_SECTION_HEADING.test(line)) section = 'other';
    else if (INIT_HEADING.test(line)) {
      first ??= section;
      if (section === 'release') namedInRelease = true;
    }
  }
  if (namedInRelease) return 'released';
  return first === 'unreleased' ? 'unreleased' : 'not named';
}

/** The version of the first `## [x.y.z]` heading, the newest release in Keep a Changelog order. */
function newestRelease(changelog: string): string | undefined {
  for (const line of changelog.split('\n')) {
    const match = RELEASE_HEADING.exec(line);
    if (match) return match[1];
  }
  return undefined;
}

/** Each R1 to R3 miss in `readme`, as `line N: what`, for a newest release of `newest`. */
function quickStartMisses(readme: string, newest: string): string[] {
  const lines = readme.split('\n');
  const start = lines.indexOf(SECTION_HEADING);
  if (start === -1) return [`no "${SECTION_HEADING}" heading`];

  let inFence = false;
  let firstBlockLine: number | undefined;
  let firstBlockClosed = false;
  const firstBlockBody: string[] = [];
  let conditionLine: number | undefined;
  let conditionVersion: string | undefined;
  for (let i = start + 1; i < lines.length; i++) {
    const line = lines[i];
    if (FENCE.test(line)) {
      if (!inFence && firstBlockLine === undefined) firstBlockLine = i + 1;
      else if (inFence && firstBlockLine !== undefined) firstBlockClosed = true;
      inFence = !inFence;
      continue;
    }
    if (inFence) {
      if (!firstBlockClosed) firstBlockBody.push(line);
      continue;
    }
    if (README_HEADING.test(line)) break;
    const condition = CONDITION.exec(line);
    if (condition && conditionLine === undefined) {
      conditionLine = i + 1;
      conditionVersion = condition[1];
    }
  }

  const misses: string[] = [];
  if (firstBlockLine === undefined) {
    misses.push(`line ${start + 1}: the section has no fenced block`);
  } else {
    const body = firstBlockBody.join('\n');
    if (NPX_INIT.test(body)) {
      misses.push(`line ${firstBlockLine}: the first fenced block runs npx @opena2a/aim-sdk init`);
    }
    if (!SOURCE_INIT.test(body)) {
      misses.push(
        `line ${firstBlockLine}: the first fenced block does not run node dist/arp/cli/aim-arp.js init`,
      );
    }
  }
  if (conditionLine === undefined) {
    misses.push(
      `line ${start + 1}: the section has no "Once a release after ${newest} is published" sentence`,
    );
  } else if (conditionVersion !== newest) {
    misses.push(
      `line ${conditionLine}: the release condition names ${conditionVersion}, not the newest release ${newest}`,
    );
  }
  lines.forEach((line, index) => {
    if (NPX_INIT.test(line) && (conditionLine === undefined || index + 1 < conditionLine)) {
      misses.push(`line ${index + 1}: npx @opena2a/aim-sdk init before the release condition`);
    }
  });
  return misses;
}

// The section as it stood when the published 1.3.1 lacked `init` and the npx form led it.
const NPX_FIRST = [
  '## Quick Start',
  '',
  SECTION_HEADING,
  '',
  'The onboarding screen in the dashboard generates a bootstrap token:',
  '',
  '```bash',
  'AIM_BOOTSTRAP_TOKEN=<token> npx @opena2a/aim-sdk init --url https://aim.example.com',
  '```',
  '',
  'Run `npx @opena2a/aim-sdk init --help` for every option.',
  '',
  '`init` is not in 1.3.1: there `npx @opena2a/aim-sdk init` answers `Unknown command: init`.',
  '',
  '```bash',
  'AIM_BOOTSTRAP_TOKEN=<token> node dist/arp/cli/aim-arp.js init --url https://aim.example.com',
  '```',
  '',
  '## Features',
].join('\n');

// The same section with the source build first and the npx form after the release condition.
const SOURCE_FIRST = [
  '## Quick Start',
  '',
  SECTION_HEADING,
  '',
  '`init` is not in 1.3.1, the published version.',
  '',
  '```bash',
  'npm ci && npm run build',
  'AIM_BOOTSTRAP_TOKEN=<token> node dist/arp/cli/aim-arp.js init --url https://aim.example.com',
  '```',
  '',
  '```',
  'Agent "my-first-agent" is registered.',
  '```',
  '',
  'Run `node dist/arp/cli/aim-arp.js init --help` for every option.',
  '',
  'Once a release after 1.3.1 is published, the same command runs from npm:',
  '',
  '```bash',
  'AIM_BOOTSTRAP_TOKEN=<token> npx @opena2a/aim-sdk init --url https://aim.example.com',
  '```',
  '',
  '## Features',
].join('\n');

describe('the README quick start for init opens with a command the published package runs', () => {
  it('reads the state of init from CHANGELOG.md headings, so the check is not vacuous', () => {
    expect(initReleaseState(CHANGELOG)).not.toBe('not named');
    expect(newestRelease(CHANGELOG)).toMatch(/^\d+\.\d+\.\d+/);
  });

  it('while init is unreleased, README.md opens the section with the source build and puts the npx form after the release condition', () => {
    const newest = newestRelease(CHANGELOG) ?? '';
    const misses =
      initReleaseState(CHANGELOG) === 'unreleased' ? quickStartMisses(README, newest) : [];
    expect(misses).toEqual([]);
  });

  it('decides that init is unreleased from heading lines under [Unreleased], not from prose', () => {
    expect(
      initReleaseState(
        [
          '## [Unreleased]',
          '',
          '### Fixed — `init` checks the agent ID the server answers with',
          '',
          '## [0.0.1] - 2026-01-01',
          '',
          '### Added',
          '',
          '- prose that names `init` and `npx @opena2a/aim-sdk init` is not a heading',
        ].join('\n'),
      ),
    ).toBe('unreleased');
    expect(initReleaseState('## [Unreleased]\n\n### Added\n\n- `init` in prose only\n')).toBe(
      'not named',
    );
    expect(initReleaseState('## [Unreleased]\n\n### Fixed — initial values\n')).toBe('not named');
  });

  it('decides that init is released once a release heading section names it', () => {
    const released = [
      '## [Unreleased]',
      '',
      '## [0.0.2] - 2026-01-02',
      '',
      '### Fixed — `init` checks the agent ID the server answers with',
      '',
      '## [0.0.1] - 2026-01-01',
    ].join('\n');
    expect(initReleaseState(released)).toBe('released');
    expect(
      initReleaseState(released.replace('## [Unreleased]', '## [Unreleased]\n\n### Fixed — `init` again')),
    ).toBe('released');
    expect(initReleaseState('## [0.0.1]\n\n### Added — `aim-sdk init` registers an agent\n')).toBe(
      'released',
    );
  });

  it('refuses the npx-first section, and an npx line or a later version ahead of the condition', () => {
    expect(quickStartMisses(NPX_FIRST, '1.3.1')).toEqual([
      'line 7: the first fenced block runs npx @opena2a/aim-sdk init',
      'line 7: the first fenced block does not run node dist/arp/cli/aim-arp.js init',
      'line 3: the section has no "Once a release after 1.3.1 is published" sentence',
      'line 8: npx @opena2a/aim-sdk init before the release condition',
      'line 11: npx @opena2a/aim-sdk init before the release condition',
      'line 13: npx @opena2a/aim-sdk init before the release condition',
    ]);
    expect(
      quickStartMisses(
        SOURCE_FIRST.replace('the published version.', 'see `npx @opena2a/aim-sdk init --help`.'),
        '1.3.1',
      ),
    ).toEqual(['line 5: npx @opena2a/aim-sdk init before the release condition']);
    expect(
      quickStartMisses(SOURCE_FIRST.replace('after 1.3.1 is', 'after 1.3.0 is'), '1.3.1'),
    ).toEqual(['line 18: the release condition names 1.3.0, not the newest release 1.3.1']);
    expect(quickStartMisses('# AIM SDK\n', '1.3.1')).toEqual([`no "${SECTION_HEADING}" heading`]);
  });

  it('accepts the source-first section', () => {
    expect(quickStartMisses(SOURCE_FIRST, '1.3.1')).toEqual([]);
  });
});
