import { describe, it, expect } from 'vitest';
import { existsSync, readdirSync, readFileSync, statSync } from 'fs';
import { dirname, join, relative, resolve } from 'path';

/**
 * docs/sdk/authentication.md used to list "No Known Vulnerabilities"
 * among Ed25519's strengths and say that, unlike RSA, it withstands
 * quantum attacks. Ed25519 is an elliptic-curve scheme: a large enough
 * quantum computer running Shor's algorithm recovers the private key
 * from the public key, exactly as it does for RSA. AIM's
 * quantum-resistant option is ML-DSA, documented in docs/guides/PQC.md.
 *
 * These tests refuse any doc that calls a signature scheme immune to
 * attack by a quantum computer, and require the authentication guide
 * to say Ed25519 is not quantum-resistant and to link to a PQC guide
 * that exists.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DOCS_DIR = join(REPO_ROOT, 'docs');
const AUTH_DOC = join(DOCS_DIR, 'sdk', 'authentication.md');

function markdownFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules') continue;
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...markdownFiles(path));
    else if (name.endsWith('.md')) out.push(path);
  }
  return out;
}

describe('docs on Ed25519 and quantum attacks', () => {
  it('no doc calls a scheme quantum-immune', () => {
    const offending: string[] = [];
    for (const file of markdownFiles(DOCS_DIR)) {
      readFileSync(file, 'utf8')
        .split('\n')
        .forEach((line, i) => {
          if (/immune\s+to\s+quantum/i.test(line)) {
            offending.push(`${relative(REPO_ROOT, file)}:${i + 1}: ${line.trim()}`);
          }
        });
    }
    expect(offending).toEqual([]);
  });

  it('the authentication guide says Ed25519 is not quantum-resistant', () => {
    const doc = readFileSync(AUTH_DOC, 'utf8');
    expect(doc).toMatch(/Ed25519[^.\n]*not resistant to quantum attacks/);
  });

  it('the authentication guide links to a PQC guide that exists', () => {
    const doc = readFileSync(AUTH_DOC, 'utf8');
    const link = doc.match(/\]\(([^)]*PQC\.md)\)/);
    expect(link).not.toBeNull();
    expect(existsSync(resolve(dirname(AUTH_DOC), link![1]))).toBe(true);
  });
});
