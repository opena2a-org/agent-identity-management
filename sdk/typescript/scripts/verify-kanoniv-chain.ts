/**
 * Verify Kanoniv's delegation chain from the interop thread.
 * This proves AIM can cross-verify chains from other engines.
 *
 * Chain from: https://github.com/kanoniv/agent-auth/issues/2
 * Kanoniv uses base64url encoding with Python's json.dumps(sort_keys=True) canonical form.
 *
 * Usage: npx tsx scripts/verify-kanoniv-chain.ts
 */

import * as ed from '@noble/ed25519';
import { fromBase64url } from '../src/crypto/delegation';

/**
 * Reproduce Python's json.dumps(obj, sort_keys=True) output.
 * Python defaults to separators=(', ', ': ') — spaces after structural comma and colon only.
 */
function pythonJsonDumps(obj: Record<string, unknown>): string {
  const sortedKeys = Object.keys(obj).sort();
  const parts: string[] = [];
  for (const key of sortedKeys) {
    const val = obj[key];
    parts.push(`${JSON.stringify(key)}: ${pythonSerializeValue(val)}`);
  }
  return `{${parts.join(', ')}}`;
}

function pythonSerializeValue(val: unknown): string {
  if (Array.isArray(val)) {
    return `[${val.map((v) => pythonSerializeValue(v)).join(', ')}]`;
  }
  if (val !== null && typeof val === 'object') {
    return pythonJsonDumps(val as Record<string, unknown>);
  }
  return JSON.stringify(val);
}

// Kanoniv's delegation chains from the interop thread
// Note: Kanoniv uses base64url with padding (=), we handle both

function fromKanonivBase64(str: string): Uint8Array {
  // Kanoniv uses base64 with url-unsafe chars in some places
  // Their public_key uses _ which is base64url, their signature uses - and _
  let b64 = str.replace(/-/g, '+').replace(/_/g, '/');
  while (b64.length % 4 !== 0) b64 += '=';
  return new Uint8Array(Buffer.from(b64, 'base64'));
}

async function main() {
  console.log('=== Verifying Kanoniv delegation chains ===\n');

  // Delegation 1: Human -> Coordinator
  const d1 = {
    delegator: 'did:key:z6MkiUeH31224B5RxNFWsg4zVoQ4udGhEoTnvka9fKUK6MzR',
    delegate: 'did:key:z6Mkw7QgaQRzrookrBMqXieZKYG8DoT8KjUJkFhDYjMZyEEK',
    scopes: ['search', 'memory.read', 'memory.write', 'resolve', 'delegate'],
    created_at: '2026-03-18T20:24:36.000Z',
    expires_at: '2026-03-25T20:24:36.000Z',
  };

  const d1Sig = fromKanonivBase64('o1TcsSfrSwdnbDhrNOsMoPaPHKDnjynJas4upOWmwrWAPpwBxOw0TJ44fsVzDiJtKUA4fM8DotcLv95ra6azAw==');
  const d1PubKey = fromKanonivBase64('O8l3UUd1AyylJd8AYG_PMq_3yx1y18JRCUJSqJYFbxo=');

  // Kanoniv canonical form: Python json.dumps(sort_keys=True)
  // Python default separators are (', ', ': ') — spaces after comma and colon
  // We must reproduce this exactly without breaking colons inside string values
  const d1PayloadBytes = new TextEncoder().encode(pythonJsonDumps(d1));

  console.log('Delegation 1 payload:', new TextDecoder().decode(d1PayloadBytes).slice(0, 80) + '...');

  let d1Valid: boolean;
  try {
    d1Valid = await ed.verifyAsync(d1Sig, d1PayloadBytes, d1PubKey);
  } catch {
    d1Valid = false;
  }

  console.log(`Delegation 1 (Human -> Coordinator): ${d1Valid ? 'VERIFIED' : 'FAILED'}`);

  // Delegation 2: Coordinator -> Researcher
  const d2 = {
    created_at: '2026-03-18T20:24:36.000Z',
    delegate: 'did:key:z6MkiEVa7rFTL9RKreitJXH6qqLv6Gqdrc9TjBcyS8aW7FAU',
    delegator: 'did:key:z6Mkw7QgaQRzrookrBMqXieZKYG8DoT8KjUJkFhDYjMZyEEK',
    expires_at: '2026-03-19T20:24:36.000Z',
    parent_delegation: 'del-1',
    scopes: ['search', 'memory.read'],
  };

  const d2Sig = fromKanonivBase64('g4EKkWQGXyNU9dUW4pRyBzQTKbADTUEuZZ_iJuvlG4dNBx3wQUYZ74QXUR5a6I16gEb9MdN85WVt06SfBRQ8DQ==');
  const d2PubKey = fromKanonivBase64('94DUBnHhZl6javmYK67Os4l-gSpU8wJyBVV_ZkXv4Rg=');

  let d2Valid: boolean;
  try {
    d2Valid = await ed.verifyAsync(d2Sig, new TextEncoder().encode(pythonJsonDumps(d2)), d2PubKey);
  } catch {
    d2Valid = false;
  }

  console.log(`Delegation 2 (Coordinator -> Researcher): ${d2Valid ? 'VERIFIED' : 'FAILED'}`);

  // Scope narrowing check
  const parentScopes = new Set(d1.scopes);
  const childScopesValid = d2.scopes.every((s) => parentScopes.has(s));
  console.log(`\nScope narrowing: ${childScopesValid ? 'VALID' : 'INVALID'}`);
  console.log(`  Parent scopes: ${d1.scopes.join(', ')}`);
  console.log(`  Child scopes:  ${d2.scopes.join(', ')}`);

  // Chain linkage check
  const chainLinked = d2.delegator === d1.delegate;
  console.log(`Chain linkage: ${chainLinked ? 'VALID' : 'BROKEN'}`);

  const allPassed = d1Valid && d2Valid && childScopesValid && chainLinked;
  console.log(`\n=== Overall: ${allPassed ? 'ALL CHECKS PASSED' : 'SOME CHECKS FAILED'} ===`);
  process.exit(allPassed ? 0 : 1);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
