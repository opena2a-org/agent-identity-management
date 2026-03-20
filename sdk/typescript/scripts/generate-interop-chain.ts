/**
 * Generate a delegation chain for the cross-engine interop test.
 * https://github.com/kanoniv/agent-auth/issues/2
 *
 * Chain: Human -> Coordinator (full scope) -> Researcher (narrowed to search + memory.read)
 *
 * Usage: npx tsx scripts/generate-interop-chain.ts
 */

import {
  generateKeyPair,
  createDelegation,
  verifyDelegationChain,
  exportDelegationChain,
  publicKeyToDidKey,
  delegationSignablePayload,
  toBase64url,
} from '../src/crypto/delegation';
import { generateKeyPair as genKP } from '../src/crypto/ed25519';

async function main() {
  // Generate three key pairs: human (root), coordinator, researcher
  const human = await genKP();
  const coordinator = await genKP();
  const researcher = await genKP();

  const now = new Date();
  const oneWeek = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);
  const oneDay = new Date(now.getTime() + 24 * 60 * 60 * 1000);

  // Delegation 1: Human -> Coordinator (full scope)
  const d1 = await createDelegation({
    delegatorKeyPair: human,
    delegatePublicKey: coordinator.publicKey,
    scopes: ['search', 'memory.read', 'memory.write', 'resolve', 'delegate'],
    createdAt: now.toISOString(),
    expiresAt: oneWeek.toISOString(),
  });

  // Delegation 2: Coordinator -> Researcher (narrowed scope)
  const d2 = await createDelegation({
    delegatorKeyPair: coordinator,
    delegatePublicKey: researcher.publicKey,
    scopes: ['search', 'memory.read'],
    createdAt: now.toISOString(),
    expiresAt: oneDay.toISOString(),
    parentDelegation: 'del-1',
  });

  // Verify our own chain before posting
  const result = await verifyDelegationChain([d1, d2]);
  if (!result.valid) {
    console.error('Chain verification FAILED:', JSON.stringify(result.results, null, 2));
    process.exit(1);
  }

  // Export in interop format
  const exported = exportDelegationChain(
    [d1, d2],
    ['root_to_coordinator', 'coordinator_to_researcher']
  );

  console.log(JSON.stringify(exported, null, 2));

  // Also print verification instructions
  console.log('\n---\n');
  console.log('### How to verify (Python)\n');
  console.log('```python');
  console.log('import json, base64');
  console.log('from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey\n');

  // Delegation 1 verification
  const d1Payload = new TextDecoder().decode(delegationSignablePayload(d1));
  console.log('# Delegation 1: root -> coordinator');
  console.log(`pub_bytes = base64.urlsafe_b64decode("${d1.publicKey}==")`);
  console.log('pub_key = Ed25519PublicKey.from_public_bytes(pub_bytes)');
  console.log(`payload = ${JSON.stringify(d1Payload)}.encode()`);
  console.log(`sig = base64.urlsafe_b64decode("${d1.signature}==")`);
  console.log('pub_key.verify(sig, payload)');
  console.log('print("Delegation 1 verified.")\n');

  // Delegation 2 verification
  const d2Payload = new TextDecoder().decode(delegationSignablePayload(d2));
  console.log('# Delegation 2: coordinator -> researcher');
  console.log(`pub_bytes = base64.urlsafe_b64decode("${d2.publicKey}==")`);
  console.log('pub_key = Ed25519PublicKey.from_public_bytes(pub_bytes)');
  console.log(`payload = ${JSON.stringify(d2Payload)}.encode()`);
  console.log(`sig = base64.urlsafe_b64decode("${d2.signature}==")`);
  console.log('pub_key.verify(sig, payload)');
  console.log('print("Delegation 2 verified.")');
  console.log('```\n');

  console.log('Scope narrowing: delegation_2 scopes ["search", "memory.read"] are a strict subset of delegation_1 scopes.');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
