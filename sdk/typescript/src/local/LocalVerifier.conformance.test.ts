import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import type { Atx } from '@opena2a/atx-verify';
import { LocalVerifier } from './LocalVerifier';

// Replays the ATX conformance fixtures vendored for the Java SDK (verbatim copies
// of atx-conformance/fixtures/, pinned by CI) through LocalVerifier. They are read
// in place so the two SDKs are tested against one copy.
const FIXTURE_DIR = join(__dirname, '..', '..', '..', 'java', 'src', 'test', 'resources', 'atx-fixtures');

interface Fixture {
  file: string;
  /** The credential exactly as it appears in the fixture, duplicate members included. */
  rawAtx: string;
  atx: Atx;
  verifierState: {
    clockRfc3339: string;
    trustedIssuers: string[];
    publicKeys: Array<{ algorithm: string; publicKeyHex: string; keyId?: string }>;
    crl?: { entries: Array<{ agentId: string; reason?: string }> };
  };
  expected: { verifyResult: 'ACCEPT' | 'REJECT'; rejectCategory?: string; reasonContains?: string };
}

function loadFixtures(): Fixture[] {
  return readdirSync(FIXTURE_DIR)
    .filter((file) => file.endsWith('.json'))
    .sort()
    .map((file) => {
      const text = readFileSync(join(FIXTURE_DIR, file), 'utf8');
      return { file, rawAtx: rawAtxText(text), ...JSON.parse(text) };
    });
}

function verifierFor(f: Fixture): LocalVerifier {
  const vs = f.verifierState;
  return new LocalVerifier({
    trustedIssuers: vs.trustedIssuers,
    publicKeys: vs.publicKeys.map(({ algorithm, publicKeyHex, keyId }) => ({ algorithm, publicKeyHex, keyId })),
    crl: vs.crl,
    now: () => new Date(vs.clockRfc3339),
  });
}

/**
 * Returns the verbatim text of the fixture's top-level `atx` member. Parsing the
 * fixture would collapse the duplicate members the strict-parse fixtures exist to
 * carry, so the credential is sliced out of the raw text instead.
 */
function rawAtxText(text: string): string {
  let depth = 0;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === '"') {
      const end = endOfString(text, i);
      if (depth === 1) {
        const colon = skipSpace(text, end);
        if (text[colon] === ':' && JSON.parse(text.slice(i, end)) === 'atx') {
          const start = skipSpace(text, colon + 1);
          return text.slice(start, endOfContainer(text, start));
        }
      }
      i = end - 1;
    } else if (ch === '{' || ch === '[') {
      depth++;
    } else if (ch === '}' || ch === ']') {
      depth--;
    }
  }
  throw new Error('fixture has no top-level atx member');
}

/** Index just past the closing quote of the string that opens at `start`. */
function endOfString(text: string, start: number): number {
  for (let i = start + 1; i < text.length; i++) {
    if (text[i] === '\\') i++;
    else if (text[i] === '"') return i + 1;
  }
  throw new Error('unterminated string in fixture');
}

/** Index just past the bracket that closes the container opening at `start`. */
function endOfContainer(text: string, start: number): number {
  let depth = 0;
  for (let i = start; i < text.length; i++) {
    const ch = text[i];
    if (ch === '"') {
      i = endOfString(text, i) - 1;
    } else if (ch === '{' || ch === '[') {
      depth++;
    } else if (ch === '}' || ch === ']') {
      depth--;
      if (depth === 0) return i + 1;
    }
  }
  throw new Error('unterminated container in fixture');
}

function skipSpace(text: string, i: number): number {
  while (/\s/.test(text[i] ?? '')) i++;
  return i;
}

const fixtures = loadFixtures();
// The reference verifiers report a strict-parse rejection as PARSE_ERROR; the SDK's
// RejectCategory union has no such member and reports MALFORMED, its structural-parse
// category (the Java SDK's conformance test pins the same mapping).
const strictParse = fixtures.filter((f) => f.expected.rejectCategory === 'PARSE_ERROR');
const others = fixtures.filter((f) => f.expected.rejectCategory !== 'PARSE_ERROR');

describe('LocalVerifier raw-text entry against the vendored ATX conformance fixtures', () => {
  it('finds the vendored fixtures, including the strict-parse ones', () => {
    expect(fixtures.length).toBeGreaterThan(0);
    expect(strictParse.length).toBeGreaterThan(0);
  });

  it.each(strictParse)('$file: verifyCredential(raw text) rejects as MALFORMED', async (f) => {
    const result = await verifierFor(f).verifyCredential(f.rawAtx);

    expect(f.expected.verifyResult).toBe('REJECT');
    expect(result.valid).toBe(false);
    expect(result.rejectCategory).toBe('MALFORMED');
    expect(result.reason).toContain(f.expected.reasonContains ?? 'duplicate');
  });

  it.each(strictParse)('$file: authorize(raw text) denies', async (f) => {
    const res = await verifierFor(f).authorize(f.rawAtx, { action: 'billing:inquiry' });

    expect(res.verified).toBe(false);
    expect(res.actionAllowed).toBe(false);
    expect(res.rejectCategory).toBe('MALFORMED');
    expect(res.denialReason).toContain(f.expected.reasonContains ?? 'duplicate');
  });

  // Once the strict parse passes, the raw-text entry reads the same fields the
  // parsed-object entry does, so every other fixture gets the same verdict either way.
  it.each(others)('$file: raw text and bytes get the parsed-object verdict', async (f) => {
    expect(JSON.parse(f.rawAtx)).toEqual(f.atx);
    const verifier = verifierFor(f);

    const fromObject = await verifier.verifyCredential(f.atx);
    const fromText = await verifier.verifyCredential(f.rawAtx);
    const fromBytes = await verifier.verifyCredential(new TextEncoder().encode(f.rawAtx));

    expect(fromText).toEqual(fromObject);
    expect(fromBytes).toEqual(fromObject);
  });
});
