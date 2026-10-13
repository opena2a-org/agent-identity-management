# Verifying packages

## Verifying the `@opena2a/aim-core` package

Version 0.2.0 was built by the `release.yml` workflow in `opena2a-org/opena2a` at tag `aim-core-v0.2.0` and published through npm Trusted Publishing with SLSA v1 provenance; versions 0.1.0 to 0.1.2 carry no provenance. To check the registry's 0.2.0 tarball against that provenance, run these in an empty directory (needs cosign, jq and openssl):

```bash
npm pack @opena2a/aim-core@0.2.0
curl -s https://registry.npmjs.org/-/npm/v1/attestations/@opena2a%2faim-core@0.2.0 \
  | jq '.attestations[] | select(.predicateType=="https://slsa.dev/provenance/v1") | .bundle' > aim-core.sigstore.json
cosign verify-blob-attestation --bundle aim-core.sigstore.json --new-bundle-format --type slsaprovenance1 \
  --certificate-identity https://github.com/opena2a-org/opena2a/.github/workflows/release.yml@refs/tags/aim-core-v0.2.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --digestAlg sha512 --digest "$(openssl dgst -sha512 -r opena2a-aim-core-0.2.0.tgz | cut -d' ' -f1)"
# Prints "Verified OK" (cosign v3.0.4, 2026-09-24); a changed tarball byte or a different signer exits non-zero.
```

Identity files (`~/.opena2a/aim-core/identity.json`) are written `mode 0600`. OAuth tokens live in the OS keychain by default. `~/.opena2a/auth.json` stores metadata only.
