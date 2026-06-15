// ESM consumer smoke: import the built ESM entry and exercise the local
// verification path end-to-end against the ESM-only @opena2a/atx-verify dep.
import crypto from 'node:crypto';
import assert from 'node:assert';
import { AIMClient } from '../dist/index.mjs';
import { canonicalPayloadV11 } from '@opena2a/atx-verify';

const ISSUER_DID = 'did:opena2a:issuer-1';

const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
const spki = publicKey.export({ type: 'spki', format: 'der' });
const rawHex = spki.subarray(spki.length - 32).toString('hex');

const atx = {
  atcVersion: '1.1',
  agentId: 'agent-mjs',
  agentDid: 'did:opena2a:agent-mjs',
  version: '1.0.0',
  contentHash: 'sha256:mjs',
  issuerDid: ISSUER_DID,
  trustLevel: 3,
  trustScore: 90,
  issuedAt: '2026-06-01T00:00:00Z',
  expiresAt: '2026-06-30T00:00:00Z',
  capabilities: ['file:read'],
  signatures: [],
};
atx.signatures = [
  {
    algorithm: 'Ed25519',
    keyId: 'k1',
    value: crypto.sign(null, canonicalPayloadV11(atx), privateKey).toString('base64'),
  },
];

const client = new AIMClient({
  localVerification: {
    trustedIssuers: [ISSUER_DID],
    publicKeys: [{ algorithm: 'Ed25519', publicKeyHex: rawHex }],
    now: () => new Date('2026-06-15T00:00:00Z'),
  },
});
client.setLocalCredential(atx);

const allowed = await client.verifyActionLocally({ action: 'file:read' });
assert.strictEqual(allowed.actionAllowed, true, 'expected file:read allowed');
assert.strictEqual(allowed.verified, true, 'expected verified');

console.log('ESM consumer smoke OK');
