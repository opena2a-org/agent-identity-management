// CJS consumer smoke: require() the built CJS entry and exercise the local
// verification path end-to-end. Proves an ESM-only @opena2a/atx-verify dep loads
// from the CJS build (the dynamic import() must resolve natively on this Node).
const crypto = require('node:crypto');
const assert = require('node:assert');
const { AIMClient, ActionDeniedError } = require('../dist/index.js');

const ISSUER_DID = 'did:opena2a:issuer-1';

(async () => {
  // canonicalPayloadV11 lives in the ESM-only package; pull it via dynamic import.
  const { canonicalPayloadV11 } = await import('@opena2a/atx-verify');

  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const spki = publicKey.export({ type: 'spki', format: 'der' });
  const rawHex = spki.subarray(spki.length - 32).toString('hex');

  const atx = {
    atcVersion: '1.1',
    agentId: 'agent-cjs',
    agentDid: 'did:opena2a:agent-cjs',
    version: '1.0.0',
    contentHash: 'sha256:cjs',
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

  let denied = false;
  try {
    await client.verifyActionLocally({ action: 'file:delete' });
  } catch (e) {
    denied = e instanceof ActionDeniedError;
  }
  assert.ok(denied, 'expected ActionDeniedError for file:delete');

  console.log('CJS consumer smoke OK');
})().catch((e) => {
  console.error('CJS consumer smoke FAILED:', e);
  process.exit(1);
});
