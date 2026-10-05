import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * docs/DEMO_SCRIPT_ECHOLEAK.md tells the presenter how to log in to the
 * dashboard before the demo. The admin password is set per install: it is
 * the DEFAULT_ADMIN_PASSWORD value from .env, or a random password that
 * `aim-bootstrap --default` generates and prints once. The login step used
 * to also print the fixed password that stacks seeded before bootstrap
 * existed were given, so a reader copying the script carried that value
 * into new installs and recordings.
 *
 * These tests refuse a demo script that contains that fixed value anywhere,
 * and require the login step to show a placeholder and say where the
 * per-install password comes from.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DOC_PATH = join(REPO_ROOT, 'docs', 'DEMO_SCRIPT_ECHOLEAK.md');

// Assembled at run time so this file is not itself a match when the tree is
// searched for the value.
const LEGACY_DEFAULT_PASSWORD = ['AIM', '2025', '!', 'Secure'].join('');

const doc = readFileSync(DOC_PATH, 'utf8');

function loginStep(text: string): string {
  const lines = text.split('\n').filter((line) => /^\s*\*\s*Log in:/.test(line));
  expect(lines, 'demo script has exactly one "Log in:" step').toHaveLength(1);
  return lines[0];
}

describe('EchoLeak demo script admin password', () => {
  it('does not contain the fixed password of legacy seeded stacks', () => {
    const hits = doc
      .split('\n')
      .map((line, i) => ({ line: i + 1, text: line }))
      .filter(({ text }) => text.includes(LEGACY_DEFAULT_PASSWORD))
      .map(({ line }) => `docs/DEMO_SCRIPT_ECHOLEAK.md:${line}`);
    expect(hits).toEqual([]);
  });

  it('shows a placeholder for the password in the login step', () => {
    const step = loginStep(doc);
    expect(step).toContain('`admin@opena2a.org`');
    expect(step).toContain('`<admin-password>`');
  });

  it('says the password is set per install and names both sources', () => {
    const step = loginStep(doc);
    expect(step).toContain('`DEFAULT_ADMIN_PASSWORD`');
    expect(step).toContain('`aim-bootstrap --default`');
  });
});
