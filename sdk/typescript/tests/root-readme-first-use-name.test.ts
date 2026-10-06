import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The root README names the platform in full where the name first appears.
 *
 * Readers arriving from standards documents meet several similar acronyms
 * next to a bare "AIM", so the first README line that uses the word carries
 * the name in the form the OpenA2A specifications use:
 * "OpenA2A AIM (Agent Identity Management)". The README opened with
 * "# Agent Identity Management (AIM)", which expanded the acronym but in a
 * different form and without the OpenA2A qualifier.
 *
 * These cells run in the SDK suite because that job runs on every pull
 * request, so an edit that drops the expansion fails before merge.
 *
 *   C1  the first README line containing the whole word AIM contains the
 *       exact phrase;
 *   C2  the pre-fix heading and near misses, kept as literals, are refused,
 *       and the accepted forms pass, through the same predicate as C1.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const readme = readFileSync(join(REPO_ROOT, 'README.md'), 'utf-8');

const FULL_NAME = 'OpenA2A AIM (Agent Identity Management)';
const WHOLE_WORD_AIM = /\bAIM\b/;

function firstUse(text: string): { line: number; text: string } | null {
  const lines = text.split(/\r?\n/);
  const index = lines.findIndex((line) => WHOLE_WORD_AIM.test(line));
  return index === -1 ? null : { line: index + 1, text: lines[index] };
}

function firstUseIsExpanded(text: string): boolean {
  const first = firstUse(text);
  return first !== null && first.text.includes(FULL_NAME);
}

describe('root README first use of the name', () => {
  it('C1: the first line using AIM reads "OpenA2A AIM (Agent Identity Management)"', () => {
    const first = firstUse(readme);
    expect(first, 'README.md has no line containing the word AIM').not.toBeNull();
    expect(
      first!.text,
      `README.md:${first!.line} is the first use of AIM and must contain "${FULL_NAME}"`,
    ).toContain(FULL_NAME);
  });

  it('C2: the pre-fix heading and near misses are refused', () => {
    const refused = [
      '# Agent Identity Management (AIM)\n',
      '# AIM\n',
      '# OpenA2A AIM\n',
      '# AIM (Agent Identity Management)\n',
      '# OpenA2A AIM (Agent Identity Management System)\n',
      `# Agent Identity Management\n\nAIM issues identities.\n\n${FULL_NAME}\n`,
    ];
    for (const text of refused) {
      expect(firstUseIsExpanded(text), JSON.stringify(text)).toBe(false);
    }
  });

  it('C2: the expanded first use passes, and later bare uses are allowed', () => {
    const accepted = [
      `# ${FULL_NAME}\n\nAIM issues identities.\n`,
      `# aim-sdk\n\nPart of ${FULL_NAME}. AIM_URL selects the AIM server.\n`,
    ];
    for (const text of accepted) {
      expect(firstUseIsExpanded(text), JSON.stringify(text)).toBe(true);
    }
  });
});
