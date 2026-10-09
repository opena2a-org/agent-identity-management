#!/usr/bin/env node
// Verifies an AIM agent card attestation (format opena2a-aim/card-attestation/v2) from the
// served card and the server's JWK Set, following card-attestation-v2.md. Node.js 18 or
// later; no dependencies.
//
//   node verify-card-attestation.mjs card.json jwks.json
//   node verify-card-attestation.mjs --vector card-attestation-v2-vector.json
//
// The first form prints the result and exits 0 only when the signature verifies and the
// attestation has not expired. The second runs every case of the conformance vector.

import { createHash, createPublicKey, verify } from 'node:crypto';
import { readFileSync } from 'node:fs';

const LABEL = 'opena2a-aim/card-attestation/v2';
const TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$/;
const HEX64 = /^[0-9a-f]{64}$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const BASE64 = /^[A-Za-z0-9+/]{86}==$/;

// UTC normalisation: whole seconds, written YYYY-MM-DDTHH:MM:SSZ.
function utc(value) {
  if (typeof value !== 'string' || !TIMESTAMP.test(value)) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString().replace('.000Z', 'Z');
}

export function verifyCardAttestation(agentId, attestation, jwks) {
  if (attestation?.format !== LABEL || attestation.alg !== 'EdDSA') return 'unsupported-format';

  const issuedAt = utc(attestation.issuedAt);
  const expiresAt = utc(attestation.expiresAt);
  if (!issuedAt || !expiresAt || !HEX64.test(attestation.cardHash ?? '') || !UUID.test(agentId ?? '') ||
      attestation.issuer !== 'aim-server' || !BASE64.test(attestation.signature ?? '')) {
    return 'malformed';
  }

  const jwk = (jwks?.keys ?? []).find((k) =>
    k.kid === attestation.keyId && k.purpose === 'card-attestation' && k.kty === 'OKP' && k.crv === 'Ed25519');
  if (!jwk) return 'unknown-key';

  // Go's JSON form of the payload: these members, in this order, no whitespace.
  const payload = `{"cardHash":"${attestation.cardHash}","agentId":"${agentId}","issuer":"${attestation.issuer}",` +
    `"issuedAt":"${issuedAt}","expiresAt":"${expiresAt}"}`;
  const digest = createHash('sha256').update(`${LABEL}\n${payload}`, 'utf8').digest();

  const key = createPublicKey({ key: { kty: 'OKP', crv: 'Ed25519', x: jwk.x }, format: 'jwk' });
  const signature = Buffer.from(attestation.signature, 'base64');
  return verify(null, digest, key, signature) ? 'verified' : 'signature-invalid';
}

const args = process.argv.slice(2);
if (args[0] === '--vector' && args.length === 2) {
  const vector = JSON.parse(readFileSync(args[1], 'utf8'));
  let failed = 0;
  for (const c of vector.cases) {
    const result = verifyCardAttestation(c.agentId, c.attestation, vector.jwks);
    const ok = result === c.result;
    if (!ok) failed++;
    console.log(`${ok ? 'ok  ' : 'FAIL'} ${c.name}: ${result}${ok ? '' : ` (expected ${c.result})`}`);
  }
  process.exit(failed === 0 ? 0 : 1);
} else if (args.length === 2) {
  const card = JSON.parse(readFileSync(args[0], 'utf8'));
  const jwks = JSON.parse(readFileSync(args[1], 'utf8'));
  const attestation = card.aim?.attestation;
  const result = verifyCardAttestation(card.aim?.agentId, attestation, jwks);
  console.log(`signature: ${result}`);
  if (result !== 'verified') process.exit(1);
  const expired = new Date(attestation.expiresAt) <= new Date();
  console.log(`expiresAt: ${utc(attestation.expiresAt)}${expired ? ' (expired)' : ''}`);
  console.log(`cardHash: ${attestation.cardHash}`);
  process.exit(expired ? 1 : 0);
} else {
  console.error('usage: node verify-card-attestation.mjs card.json jwks.json');
  console.error('       node verify-card-attestation.mjs --vector card-attestation-v2-vector.json');
  process.exit(2);
}
