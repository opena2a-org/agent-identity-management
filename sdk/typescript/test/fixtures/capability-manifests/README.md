# Capability Manifest Fixtures

Generated deterministically by `src/arp/crypto/manifest-loader.test.ts`. Do not hand-edit. To refresh, rerun `npx vitest src/arp/crypto/manifest-loader.test.ts` and commit the resulting diff.

Each file exercises one rejection path through the loader:

- `happy.yaml` -- valid signed manifest, should load
- `tampered-payload.yaml` -- agentId changed post-signing
- `tampered-ed-signature.yaml` -- one byte flipped in ed25519 half
- `tampered-mldsa-signature.yaml` -- one byte flipped in ml-dsa half
- `missing-signature.yaml` -- signature block stripped
- `wrong-algorithm.yaml` -- alg claims ml-dsa-44 instead of ml-dsa-65
- `expired.yaml` -- signed manifest with expiresAt in 2020
- `bad-base64-key.yaml` -- ed25519 public key is not valid base64
