# Changelog reconciliation for platform 1.1.0

This table checks the `## [Unreleased]` section of `CHANGELOG.md` against every
change merged to `main` since the `platform-v1.0.0` tag. It decides, for each
pull request, whether the change needs a changelog entry, and where that entry is.

## Decision rule

A change needs an entry when it changes user-visible behaviour, a route, a flag,
a migration, a default or an SDK surface, fixes a security issue, or removes
something. CI, tests, internal refactors, comment-only edits and documentation
typos do not need one.

Scope: `CHANGELOG.md` records the platform (backend and dashboard), as its
preamble says. A change confined to one SDK under `sdk/` ships under that SDK's
own `sdk-*` tag and is recorded in that SDK's changelog, so its row reads "not
needed here", unless it moves a dependency off a vulnerable version: this file
records those in its Security section, as it does for jackson 2.18.11.

## How to read the table

- PR: every distinct `#N` in the subject of a commit in the range. A subject that
  names two numbers gives each its own row.
- Commit: the commit or commits whose subject carries the number.
- Subject: the commit subject. Advisory identifiers and em dashes in a subject are
  left out or replaced here, and a subject whose wording a documentation check of
  this repository refuses in a tracked guide is paraphrased and marked so.
- Entry: the line of `CHANGELOG.md` that cites the number (the `### ` heading when
  one cites it, otherwise the bullet), or the line of the entry the same change
  added when that entry does not cite the number. Line numbers refer to
  `CHANGELOG.md` in the commit that carries this version of the table.
- Decision: `present`, `added by this reconciliation`, or `not needed` with the
  reason.

Range: `platform-v1.0.0..origin/main`. Rows: 412, one per distinct pull request number in the commit subjects. `[Unreleased]` spans CHANGELOG.md lines 12-8074 as of this table.

| Decision | Rows |
| --- | --- |
| added by this reconciliation | 4 |
| not needed | 121 |
| present | 287 |

## Pull requests

| PR | Commit | Subject | Entry | Decision |
| --- | --- | --- | --- | --- |
| #131 | `9f569b00`, `69716f12`, `fd45e375` | FGA Step 5 no longer denies HIGH-risk requests on the intent classifier; it runs async and is only recorded (#131) | line 7871 | present (cites #323, carried by the same commit) |
| #269 | `b1bc019d` | docs: add "See it work" A/B demo GIF, link to DVAA RAGBot-AIM (#269) | none | not needed: documentation only, no behaviour change |
| #270 | `d383d451` | docs(sdk): richer `aim-sdk login` output + install-first README quickstart (#270) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #272 | `83853ff1` | docs(security): correct credential-storage section (plaintext 0600, not encrypted) (#272) | none | not needed: documentation only, no behaviour change |
| #273 | `f5651ed7` | chore(sdk): aim-sdk 1.23.0 (provisional AAP grant surface, dashboard-styled login pages, packaging fixes) (#273) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #274 | `dfe4ac89` | Wire MCP manifest drift detection into capability and attestation flows (#274) | line 7859 | present |
| #277 | `018dc16d` | Causal-denial telemetry: correlation envelope + AIMClient wiring (TS SDK) (#277) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #278 | `b8f41f30` | feat(telemetry): SDK→Registry relay for anonymized causal-denial indicators (#278) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #279 | `54c95381` | feat(telemetry): harden joiner, bounded buffers + deferred default-sink write (#279) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #280 | `42c56f3b` | fix(telemetry): align auto-joiner write dir with the relay read dir (#280) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #281 | `3b294f40` | feat(python-sdk): causal-denial telemetry parity with the TypeScript SDK (#281) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #282 | `ceeaa720` | fix(backend): return 400 not 500 for agent registration with invalid org/user (#282) | line 7979 | present (entry added by the same change; it does not cite the number) |
| #283 | `59f9addc` | test(sdk): guard test_sdk_verification stray script under __main__ (#283) | none | not needed: tests only |
| #284 | `aa893da3` | ci: restore Sync to AIM Cloud workflow with token preflight (#284) | none | not needed: CI only |
| #285 | `fa4ff8a7` | chore: remove dead sync-to-frontend script (#285) | none | not needed: CI and repository tooling only |
| #286 | `3cbb44f5` | fix(sdk-cli): stop doubling the 'Token exchange failed:' prefix (#286) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #287 | `4621a28a` | release(sdk-py): aim-sdk 1.24.1 (#287) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #288 | `e9f29e04` | fix(sdk): name the real credential source path; guard login EOF (#288) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #289 | `70821be0` | feat(agents): add optional declaredPurpose to agent identity (#289) | line 7850 | present |
| #290 | `1bbfb393` | ci: retry docker registry logins to survive transient 500s (#290) | none | not needed: CI only |
| #291 | `07e5630f` | feat(registration): capture optional signup profile questions (#291) | line 7875 | present |
| #293 | `ebd049cb` | feat(capabilities): honeytoken capabilities, alert + audit on verification (#293) (#316) | line 7856 | present (cites #316, carried by the same commit) |
| #294 | `56d0b0be` | fix(java-sdk): bump bouncycastle 1.79 -> 1.84 (advisory id omitted) (#294) | line 7747 | present |
| #295 | `ccb86823` | perf(agents): eliminate 1+2N query pattern in agent list, add server-side pagination (#295) | line 7961 | present |
| #296 | `60862582` | ci(sync): bump actions/checkout v4 -> v5 (#296) | none | not needed: CI only |
| #297 | `b2243491` | perf: eliminate per-row query N+1 in MCP servers + pending verifications lists (#297) | line 7961 | present |
| #298 | `f2462ba4` | perf(web): progressively load the A2A dashboard page (#298) | line 7962 | present |
| #299 | `c69182ee` | feat(ts-sdk): local ATX credential verification path (#299) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #300 | `02e055b9` | feat(java-sdk): local ATX credential verification (atx-verify extraction step 5) (#300) | line 7879 | present |
| #301 | `ea766088` | sync-to-cloud: respect .sync-protect in sync_migrations() (#301) | none | not needed: CI and repository tooling only |
| #302 | `bc5c036b` | docs(python-sdk): cross-link aicomply as content-compliance companion (#302) | none | not needed: documentation only, no behaviour change |
| #303 | `aae8bfba` | Run vitest once in sdk/typescript test script (#303) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #304 | `162f4469` | Rename dashboard heading to 'Security Overview' (#304) | none | not needed: the heading it set is no longer on the page at main |
| #305 | `b449b77c` | Harden auth: block SDK tokens as bearer, idle/absolute session timeout, org name in /auth/me (#305) | line 7808 | present |
| #307 | `5faf9019` | Server-side token revocation on logout + opt-in CORS preview origins (#307) | line 7840 | present |
| #308 | `af523bb3` | Separate access vs refresh tokens with a typ claim (grace rollout) (#308) | line 7808 | present |
| #309 | `fb2efbaa` | Retire empty-typ grace on the access path (keep it for refresh) (#309) | line 7809 | present |
| #311 | `75308bd2` | Add SDK isolation attestation ingest and surface the isolation factor (#311) | line 7865 | present |
| #312 | `542bd258` | Add Agent Trust Credential issuance delegating to the Registry (#312) | line 7863 | present |
| #313 | `91c4abf9` | fix(mcp-drift): atomic well-known recompute + operator alert on drift (#313) | line 7860 | present |
| #314 | `35bca2e8` | fix(trust): MCP drift alerts now affect the connected agent's trust score (#314) (#321) | line 7993 | present (cites #321, carried by the same commit) |
| #315 | `86f77173` | Rename ATC client to ATCClient to avoid cloud-build collision (#315) | none | not needed: internal rename of a client type |
| #316 | `ebd049cb` | feat(capabilities): honeytoken capabilities, alert + audit on verification (#293) (#316) | line 7856 | present |
| #317 | `fc0a23c8` | feat(sdk): async-refreshed revocation cache + Java verify benchmark (#317) (#318) | line 7880 | present (cites #318, carried by the same commit) |
| #318 | `fc0a23c8` | feat(sdk): async-refreshed revocation cache + Java verify benchmark (#317) (#318) | line 7880 | present |
| #319 | `6615a1cd` | Fix AIM Cloud sync: move raiseHoneytokenAlert to a synced file (#319) | none | not needed: moves code between files for the hosted build's sync, no behaviour change |
| #320 | `8bfedaae` | Fix AIM Cloud sync: move HasCapabilityNoAlert to a synced file (#320) | none | not needed: moves code between files for the hosted build's sync, no behaviour change |
| #321 | `35bca2e8` | fix(trust): MCP drift alerts now affect the connected agent's trust score (#314) (#321) | line 7993 | present |
| #322 | `fd45e375` | feat(fga): add Step 5 intent-check observability metrics (#131) (#322) | line 7869 | present |
| #323 | `69716f12` | feat(fga): consume daemon classify/abstain signal in Step 5 intent check (#131) (#323) | line 7871 | present |
| #324 | `9cbf6032` | feat(fga): emit gen_ai.agent.* authorization attributes on the fga.authorize span (#324) | line 7872 | present |
| #325 | `26d2ace1` | ci: make Go linting actually run (golangci-lint v2 + action v8) (#325) | none | not needed: CI lint configuration |
| #326 | `2af4e6ee` | ci: make Go security linters blocking; fix authenticated config-path file read (#326) | line 7805 | present |
| #327 | `5e2cd815` | ci: make Go Lint (security) always report so it can be a required check (#327) | none | not needed: CI only |
| #328 | `0b57e502` | ci: add fan-in CI Gate so the full test suite can be a required check (#328) | none | not needed: CI and repository tooling only |
| #329 | `740efab3` | Add ARP runtime-protection module at @opena2a/aim-sdk/arp (#329) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #330 | `f2a01c19` | AIP §6.1 composition rule with anti-gaming ceiling + atx-conformance CI gate (#330) | line 7942 | present |
| #331 | `808bc6ae` | fix(sdk-ts): make the Fastify plugin work through fastify.register + close packaging gaps (#331) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #332 | `39b7c34f` | Persist execution_isolation and excluded_factors on trust scores (#332) | line 7943 | present |
| #333 | `888a16c4` | Bump pinned atx-conformance ref to 18-fixture suite; vendor degenerate declaredPurpose fixtures (#333) | none | not needed: CI only |
| #336 | `02dc2a14` | Merge pull request #336 from opena2a-org/fix/aim-sdk-101-batch-arp-livefire | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #337 | `d74080d2` | ATX: strict-parse the credential (fold-aware duplicate rejection) in the Java SDK; bump conformance ref to 20 fixtures (#337) | line 7766 | present |
| #338 | `e29705e7` | sdk-ts: bump @opena2a/atx-verify to 0.3.0 (1.0.1 -> 1.0.2) (#338) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #339 | `0cac02b3` | chore: point published contact email to info@opena2a.org (#339) | none | not needed: contact address in a code comment and the documentation |
| #340 | `c09ce5cd` | fix(deps): bump jackson-databind and x/crypto/x/net for HIGH CVEs (#340) | line 7747 | present |
| #341 | `1e0d0b96` | feat(auth): allow machine API keys to register agents via a narrow gate (#341) | line 7853 | present |
| #342 | `0a54fed4` | harden delegation verification: enforce signed temporal window (#342) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #343 | `aff134f6` | auth: gate agent credential + SDK download routes to member (JWT) access (#343) | line 7802 | present |
| #344 | `4dc241f5` | docs: document the delegation-chain API in the SDK README (#344) | none | not needed: documentation only, no behaviour change |
| #345 | `63dd7ebf` | harden delegation chains: enforce child expiry <= parent expiry (#345) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #347 | `8a2a0185` | auth: stop OAuth service tokens from traversing human role gates (#347) | line 7799 | present |
| #349 | `27a35e3c` | metrics: optional bearer-token auth for the /metrics endpoint (#349) | line 6165 | present |
| #350 | `b0f9bbbd` | trust: calculate MCP trust scores instead of asserting them (#350) | line 7949 | present |
| #351 | `36b897d4` | test: repair the integration fixtures against the current schema (#351) | none | not needed: tests only |
| #352 | `8ea7badf` | ci: run the Go integration tests (#352) | none | not needed: CI only |
| #353 | `a1947fa8` | auth: enforce agent revocation on the signature and API-key paths (#353) | line 7794 | present |
| #354 | `24e91e68` | mcp: stop the repository Update from writing the trust-score cache (#354) | line 7996 | present |
| #355 | `3a66faab`, `819a828c`, `55c7700c` | MCP security policies: the minTrustScore rescale is migration 114, so number 112 names one migration (#355) | line 6972 | present |
| #356 | `22ed95fd` | mcp: stop presenting MCP security policies as enforced (#356) | line 7953 | present |
| #358 | `54705efb` | backend: scope the last reviewed-pending handlers to the requesting organization (#358) (#544) | line 7446 | present |
| #360 | `95244430` | Fix the revocation list returning no revocations to any caller (#360) | line 7635 | present (entry added by the same change; it does not cite the number) |
| #362 | `91570c14` | db: convert all naked TIMESTAMP columns to TIMESTAMPTZ (#362) | line 8020 | present (entry added by the same change; it does not cite the number) |
| #363 | `249c31dd` | Close six cross-tenant defects and the blind spot that hid them (#363) | line 7783 | present |
| #364 | `c9578276` | Scope the A2A consent and trust-score list endpoints to the caller's organization (#364) | line 7658 | present (entry added by the same change; it does not cite the number) |
| #365 | `0cc72ce0` | Scope the A2A agent-card list to the caller's organization (#365) | line 7644 | present (entry added by the same change; it does not cite the number) |
| #366 | `07297bf5` | File an issue when the cloud sync skips, instead of only warning (#366) | none | not needed: CI only |
| #367 | `8ea0cebf` | Make the PR review gate fail closed instead of approving (#367) | none | not needed: CI only |
| #368 | `6b4a94df` | Fix the invalid expression that has been rejecting the sync workflow since 8d79696 (#368) | none | not needed: CI only |
| #369 | `02a46c69` | Fix having to sign in twice, caused by a login that looked like a token refresh (#369) | line 7844 | present |
| #371 | `d4f815ef` | Stop claiming security audits and compliance we have not had (#371) | none | not needed: wording of claims in documentation and page copy; the same change reworded this file's support line |
| #372 | `27fecee8` | Authenticate the SDK verification write routes, and stop /result writing the decision (#372) | line 7778 | present |
| #373 | `a7f243d7` | Honour .sync-protect on every sync path, not most of them (#373) | none | not needed: CI and repository tooling only |
| #374 | `0b1f8760` | Raise the js-yaml floor to the patched version, and unstale the lockfile (#374) | line 23 | added by this reconciliation |
| #375 | `b78db564` | Gate the SDK's shipped dependencies, and stop a green check that cannot fail (#375) | none | not needed: CI only |
| #381 | `2980a027` | Correct the AAP spec version in the SDK, and stop dead-ending on the broker 404 (#381) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #384 | `d2190d0c` | Sanitise server-sent text before it can reach a terminal (#384) (#404) | SDK changelogs | present (recorded in `sdk/typescript/CHANGELOG.md` and `sdk/python/CHANGELOG.md`, which cite #384) |
| #385 | `46c1b599` | Give verification a typed decision, and make a denial behave like one (#385) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #386 | `7edef266` | Block a denied action in every enforcement mode (#386) | SDK changelog | present (recorded in `sdk/python/CHANGELOG.md`; the dashboard gains one sentence that explains it) |
| #387 | `a7a95312` | Pin the enforcement matrix so a denial cannot silently start executing again (#387) | none | not needed: tests only |
| #388 | `288c21c5` | Catch a server-side denial in the Express and Fastify middlewares (#388) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #392 | `8ffe3537` | Record the two carried defects in the 2.0.0 CHANGELOG (#392) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #393 | `6190f813` | Merge pull request #393 from opena2a-org/fix/arp-signature-telemetry-opt-in | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #394 | `ad9f0226` | Merge pull request #394 from opena2a-org/fix/sdk-py-trust-score-and-console-glyphs | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #395 | `1279338b` | Merge pull request #395 from opena2a-org/fix/ci-sdk-reaches-the-backend-image | none | not needed: CI only |
| #396 | `279c962f` | Merge pull request #396 from opena2a-org/fix/sdk-ts-version-single-source | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #399 | `27d0500a` | Merge pull request #399 from opena2a-org/fix/container-scan-follows-the-dockerfile | none | not needed: CI and repository tooling only |
| #401 | `8c7c16ab` | Make ARP report the version npm published (#401) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #402 | `d466eb95` | ci(pr-review): share the model call; fix the prior-review fetch and the `-` filename hole (#402) | none | not needed: CI only |
| #403 | `f18e88f6` | ci: update claude-review action pin (#403) | none | not needed: CI only |
| #404 | `d2190d0c` | Sanitise server-sent text before it can reach a terminal (#384) (#404) | SDK changelogs | present (recorded in `sdk/typescript/CHANGELOG.md` and `sdk/python/CHANGELOG.md`, which cite #384 from the same commit) |
| #405 | `f0ebb6b5` | Bump golang.org/x/net to v0.56.0 (advisory id omitted) (#405) | line 7752 | present |
| #406 | `b39e4589` | Ship the telemetry consent CLI as the aim-arp bin (#406) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #407 | `bc1e5866` | Cut aim-sdk 2.0.1 (#407) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #411 | `fd31532f` | Attribute an opt-out to the source actually in effect, and stop claiming actions that did not happen (#411) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #413 | `bcac70d5` | A help request answers with help, never by running the command (#413) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #414 | `ed1290da` | Date 1.3.0 by its tag; restore canonical Apache-2.0 text (#414) | none | not needed: an SDK changelog date and the license text |
| #415 | `6a20b601` | Merge pull request #415 from opena2a-org/fix/pypi-license-field | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #419 | `4ed0a560` | Add the Glasshouse foundation: tokens, theme switch, shell and edge gate (#419) | line 7964 | present |
| #420 | `ac178fff` | Rebuild the overview as a Glasshouse home with Developer, Security and Executive lenses (#420) | line 7965 | present |
| #421 | `b764a14b` | Move the auth pages and the registration queue onto Glasshouse, with claims measured against the backend (#421) | line 7966 | present |
| #422 | `8b321ed8` | Move the agents, SDK and developer pages onto Glasshouse and teach the pip path (#422) | line 7966 | present |
| #423 | `ef9c1cbb` | Move the security surfaces onto Glasshouse with distinct severities and honest wording (#423) | line 7966 | present |
| #424 | `2753b66b` | Move the MCP and A2A pages onto Glasshouse and escape the ABOM print export (#424) | line 7966 | present |
| #425 | `e7f1d525` | Refuse a registration nobody in the deployment could approve (#425) | line 7989 | present |
| #426 | `c9941b19` | Report the allowlist truthfully in the refusal log and pin the 503 mapping at the routes (#426) | line 7990 | present |
| #427 | `5bd8e2ec` | Make the dashboard readable in light theme and group the agent detail tabs (#427) | line 7964 | present |
| #428 | `34fa311f` | Live first-check-in listener, whole-number metrics, opaque overlays, and the shield mark (#428) | line 7967 | present |
| #429 | `169ffcfb` | Serve an app icon: the web app shipped no favicon, so browsers logged intermittent console 404s for /favicon.ico (#429) | line 7968 | present |
| #430 | `b035d8ed` | Consolidate navigation to seven top-level entries with hub tabs (#430) | line 7967 | present |
| #431 | `40587e6e` | Add aim-sdk demo: one command to a live dashboard (#431) | line 7877 | present |
| #432 | `a1f428de` | Refresh mints role and email from the user record, never from the token (#432) | line 7643 | present |
| #433 | `ae0d2a5c` | Make the mobile nav drawer and remaining menus opaque overlay surfaces (#433) | line 7968 | present |
| #434 | `90dcb178` | Record the refresh/recovery token fix in the changelog (advisory id omitted) (#434) | line 7637 | present (entry added by the same change; it does not cite the number) |
| #435 | `e191eadc` | Rename review check to 'Automated code review' (#435) | none | not needed: CI and repository tooling only |
| #436 | `c8ad876e` | Drop the previous-review replay from the review gate (#436) | none | not needed: CI only |
| #437 | `70a64d82` | Require an out-of-fleet approval for changes to the merge gate's own config (#437) | none | not needed: CI only |
| #438 | `eb68b89e` | Document the gate-file authorship rule (#438) | none | not needed: CI only |
| #439 | `09bef205` | Document gate-file change control in the security policy (#439) | none | not needed: documentation only, no behaviour change |
| #440 | `630bacbc` | Add the gate-file approval check (#440) | none | not needed: CI only |
| #442 | `3c95f6ea` | Python SDK: TS-parity fixtures and drift guard for the ARP telemetry seam (#442) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #443 | `972964d9` | Encode gate verdicts by state class: action_required while awaiting approval (#443) | none | not needed: CI only |
| #444 | `ac01cfdf` | Trust factor 9: cap unverified isolation attestations, expire stale ones (#444) | line 7884 | present (entry added by the same change; it does not cite the number) |
| #445 | `7506cc4c` | Backend: register the SDK isolation-attestation path, keep /isolation as deprecated alias (#445) | line 7915 | present (entry added by the same change; it does not cite the number) |
| #446 | `0afe2ebf` | AIM-04: fix NetworkMonitor ss parser column shift under the state filter (#446) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #447 | `2fa3c064` | release: run the SDK test suite and typecheck before publishing to npm (#447) | none | not needed: CI only |
| #448 | `e389a3da` | Release @opena2a/aim-sdk 1.3.1 (#448) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #451 | `cff33cac` | SDK README: the Express and Fastify examples need registered-agent credentials; 1.3.1 known issues (#451) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #452 | `5405500a` | ci: run the TypeScript SDK suite in a job the CI Gate needs (AIM-05) (#452) | none | not needed: CI only |
| #453 | `6deb5cb3` | feat: one enforcement deadline over the whole SDK verification call (AIM-06) (#453) | none | not needed: confined to `sdk/python` and `sdk/typescript`, which keeps its own changelog and release (scope) |
| #454 | `a94f7032` | feat: FGA contextRules.onUnavailable, fail closed by default (AIM-08) (#454) | line 7828 | present |
| #455 | `d176590a` | feat: GET /health/ready body contract, commit stamping, vendored schema (AIM-07) (#455) | line 7958 | present |
| #456 | `627a804f` | ARP L2 sends nothing to a vendor unless it was configured to (#456) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #457 | `e9a33632` | Report L2's real state, and stop withheld descriptors from reconstructing values (#457) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #458 | `d29a2eb4` | Merge pull request #458 from opena2a-org/hotfix/sdk-py-2.0.2 | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #461 | `45f17ae9` | Run the Python SDK test suite in CI and in the publish job before the build (#461) | none | not needed: CI only |
| #462 | `0c3fa80b` | Java SDK: an untrusted issuerChain authority cannot sign for a trusted issuer (#462) | line 7763 | present |
| #464 | `f2aa4012` | aim-sdk (Python): MCP detection payloads report the installed package version (#464) (#470) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #465 | `442389cf` | ci: pin atx-conformance at the 21-fixture suite and vendor the untrusted-chain-authority fixture (#465) | none | not needed: CI only |
| #466 | `28434a0f` | Sweep internal role tokens and machine-local paths from the public surface (#466) | none | not needed: wording of code comments and documentation, no behaviour change |
| #467 | `b7764bca` | fix(sdk-ts): close eight open 1.3.1 release-test P2 findings (AIM-11) (#467) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #468 | `4404f023` | sdk/typescript: disposition all sixteen 1.3.1 release-test P3 findings (AIM-12) (#468) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #469 | `d3b9bfe9` | ci(sdk): walk per-assertion vitest statuses in the publish/CI zero-skip gate (#469) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #470 | `f2aa4012` | aim-sdk (Python): MCP detection payloads report the installed package version (#464) (#470) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #475 | `d93a53b9` | sdk-python: drop the stale capability_detection.__version__ (#475) (#547) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #476 | `909ef0a1` | ci(sdk): match --allow-skips entries exactly against the SDK-root-relative path (AIM-18) (#476) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #477 | `063feec6` | docs(sdk): name the two skipIf groups by file in the publish gate doc (AIM-20) (#477) | none | not needed: documentation only, no behaviour change |
| #478 | `af0a25be` | test(sdk-python): read the newest release section, not the first heading, in the version changelog check (#478) | none | not needed: tests only |
| #479 | `c28f4aad` | AIM-15: replace U+2713/U+2717 console marks with [OK]/[FAIL] across the Python SDK (#479) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #482 | `dfc5719f` | ci(sdk): bind every --allow-skips entry to a per-file skip bound in the publish/CI gate (AIM-19) (#482) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #484 | `6f86614b` | AIM-14: rebase the five P2 findings fix onto main (CHANGELOG merged) (#484) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #486 | `27dc545b` | AIM-16: rebase the twelve P3 findings fix onto main (CHANGELOG merged) (#486) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #487 | `ce68f10e` | AIM-21: the three P2 findings and the seven P3 findings from the aim-sdk 2.0.3 release test (#487) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #488 | `c4424351` | Make the upstream tests and the cloud sync layout-agnostic (#488) | none | not needed: CI and repository tooling only |
| #489 | `a796109a` | Converge the name of the fga_policies onUnavailable constraint (#489) | line 7999 | present |
| #490 | `fa571523` | Harden the public did:aip resolver: no pending-agent oracle, rate limited, rightmost untrusted hop (#490) | line 7814 | present |
| #491 | `efe28e11` | Bump next, sharp, postcss, x/crypto, grpc, go-jose and x/text; turn the self-hosted image optimizer off (#491) | line 17 | present |
| #492 | `1ea0dff8` | Retire empty-typ grace on the refresh path (P1-phase-2) (#492) | line 7809 | present |
| #493 | `dcb7f5bf` | Pin apps/web to a committed lockfile: add apps/web/package-lock.json (v3, seeded from the root lockfile, next 16.3.5 exact) and re-sync the root lockfile's apps/web entry (QGF-224, Control B, Sequencing 1) (#493) | line 14 | added by this reconciliation |
| #494 | `fcfa9c60` | Replace the Maven Central coordinate with the source build across the Java SDK docs (QGF-225) (#494) | none | not needed: documentation; the same change rewrote this file's preamble, lines 7-10 |
| #495 | `1b9ffa4c` | Replace npm lockfile hunks in the PR review input with a predicate-backed placeholder and enforce its report (QGF-226) (#495) | none | not needed: CI and repository tooling only |
| #496 | `3a65737c` | Pin both frontend Dockerfile bases and root the aim-dashboard runtime stage on alpine (QGF-230) (#496) | line 7754 | present |
| #497 | `ed45a6b5` | License Compliance: the Go license verdict is the checker exit status behind a pinned go-licenses script with retry (QGF-251) (#497) | none | not needed: CI license check and its inputs |
| #498 | `4c4e0706` | Backend: remove the community-intelligence push that posted into a Registry route that does not exist, and pin the Registry's route table (#498) | line 7977 | present |
| #499 | `993256b3` | A2A: an agent with no task data is unscored, the trust-score PUT refuses, routing stops defaulting to 0.5 (#499) | line 7622 | present (entry added by the same change; it does not cite the number) |
| #500 | `2a749df5` | sdk-java: bouncycastle 1.84 -> 1.85 (advisory id omitted) critical, (advisory id omitted) high) (#500) | line 7748 | present |
| #501 | `f651fc57` | README: the quick start shows a measured denied action, and the demo leads with AIM (#501) | none | not needed: documentation only, no behaviour change |
| #502 | `ed73d900` | A trust score computed from no allowed action is not evaluable: no low-trust block, no suspension (#502) | line 7985 | present |
| #503 | `26227da9` | sdk-python: api-key mode registers through POST /api/v1/agents with X-API-Key and a local keypair (#503) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #504 | `30ca4857` | aim-sdk login completes through the device grant the backend serves; the dashboard gains the consent page (#504) | line 7847 | present |
| #505 | `5909cc0d` | The dashboard no longer takes a bearer from the URL query as the session (#505) | line 7823 | present |
| #506 | `4c4ad1e3` | The device token endpoint mints no pair for an account that can no longer hold a session (#506) | line 7817 | present |
| #509 | `c7d2d340` | Re-pin the aim-dashboard runtime base to alpine 3.21.8 (advisory id omitted) in openssl 3.3.7-r0) (#509) | line 7755 | present |
| #511 | `61731aa3` | examples: clear six high/critical advisories in the Java demo and install the Python examples from exact locks (#511) | line 7750 | present |
| #514 | `a28d78cc` | Demo videos are rendered from the README's Quick start on a throwaway stack of released images: the recording instrument under docs/demo (#514) | none | not needed: documentation tooling |
| #515 | `7d04cd1d` | A revoked session cannot download an SDK, approve a device sign-in or recover an SDK credential with its still-valid access token (#515) | line 7495 | present (entry added by the same change; it does not cite the number) |
| #516 | `bcb367cd` | Retiring a refresh token is atomic on a set-if-absent store, and the presentation that loses the race is a reuse (#516) | line 7510 | present (entry added by the same change; it does not cite the number) |
| #517 | `e377fffa` | Logouts are recorded in the audit log: the row comes from the presented token's claims (#517) | line 7521 | present (entry added by the same change; it does not cite the number) |
| #518 | `f0ffcf55` | A new dashboard session never keeps the previous account's refresh token (#518) | line 7541 | present (entry added by the same change; it does not cite the number) |
| #519 | `bfa4fb7b` | The SDK credential recovery route mints only for the account that owns the revoked token (#519) | line 7487 | present (entry added by the same change; it does not cite the number) |
| #520 | `dc5c993f` | A password reset link carries only the token; a failed reset mail is logged without the recipient's address (#520) | line 7548 | present (entry added by the same change; it does not cite the number) |
| #521 | `d82b1eba` | A sign-in ends a fixed time after it began, however often it is refreshed (#521) | line 7558 | present (entry added by the same change; it does not cite the number) |
| #523 | `850d4ee6` | The dashboard keeps its sign-in in one place: the session store is the only source of truth and the route gate moves into the dashboard shell (#523) | line 7529 | present (entry added by the same change; it does not cite the number) |
| #524 | `993adce2` | Note the withdrawn 1.23 and 1.5 image tags in the Docker install section (#524) | none | not needed: documentation only, no behaviour change |
| #525 | `e9be8194` | Describe what the Docker quickstart fetches, and scope the npm provenance check to @opena2a/aim-core (#525) | none | not needed: documentation and a script comment |
| #526 | `cd83c3aa` | Sign-in follows only a same-origin return path: decode once, refuse control characters, resolve against a fixed origin (#526) | line 7820 | present |
| #527 | `c629fe4f` | The API accepts a sign-in only from the Authorization header (#527) | line 7480 | present (entry added by the same change; it does not cite the number) |
| #528 | `a7607bb5` | README: point readers from the GHCR package page to the Docker steps and the advisory's verify command (#528) | none | not needed: documentation only, no behaviour change |
| #529 | `fbf3494e` | Sign-in sets no session cookie, and sign-out takes the refresh token from the body only (#529) | line 7467 | present (entry added by the same change; it does not cite the number) |
| #530 | `8f040490` | README: the GHCR pointer says the Docker steps pin and verify nothing, and scopes cosign to the build the advisory names (#530) | none | not needed: documentation only, no behaviour change |
| #531 | `b69f3ebb` | README: the aim-core verification step checks the registry tarball against its SLSA provenance with cosign (#531) | none | not needed: documentation only, no behaviour change |
| #532 | `ef8befe1` | README: the Docker section links the image advisories and names no advisory build to run (#532) | none | not needed: documentation only, no behaviour change |
| #533 | `7584d418` | A reused refresh token is recorded as the same client or a different one (#533) | line 7448 | present (entry added by the same change; it does not cite the number) |
| #534 | `a0990d7b` | Record capability verifications as capability checks, not identity (#534) | line 7423 | present (entry added by the same change; it does not cite the number) |
| #535 | `fd0e4906` | sdk-ts: entry points share one copy of the client and error classes (#535) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #536 | `6cad88a4` | Python SDK: every entry point reports a blocked action before raising (#536) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #537 | `dc6bfc23` | docs: cite public PRs instead of private planning records and local paths (#537) | none | not needed: code comments |
| #540 | `c94cf505` | sdk-python: console panels, trust-score range, User-Agent versions, legacy logout (#540) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #541 | `7190a0f1` | aim-arp telemetry: report a corrupt or unreadable audit log instead of an empty one (#541) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #543 | `d9daccdc` | Key-expiry enforcement suspends expired agents without touching their key material (#543) | line 7939 | present |
| #544 | `54705efb` | backend: scope the last reviewed-pending handlers to the requesting organization (#358) (#544) | line 7446 | present (cites #358, carried by the same commit) |
| #547 | `d93a53b9` | sdk-python: drop the stale capability_detection.__version__ (#475) (#547) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #548 | `4f7265ba` | backend: an API-call analytics row keeps the strings of its own request (#548) | line 7618 | present (entry added by the same change; it does not cite the number) |
| #550 | `e7dff3eb` | Verify ML-DSA-65, assert the conformance row set, and re-vendor the 23-fixture suite (#550) | line 7758 | present |
| #552 | `3fa3d84e` | Stop tracking backup sibling files and ignore them (#552) | none | not needed: stops tracking backup copies that no route served, and ignores them |
| #553 | `3ab8285e` | Python SDK: aim-sdk commands import only what they use (#553) | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| #554 | `815b78c1` | python sdk: refresh-rejected banner gives the fix for the stored credential (#554) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #556 | `94d993d3` | verification-events: refuse writes until outcomes are server-derived (#556) | line 7772 | present |
| #559 | `e4f07656` | verification-events: agent ownership check before any use, real collaborators in the refusal tests, DELETE contract corrected (#559) | line 7772 | present |
| #560 | `c2a8ac5e` | sdk/java: bump jackson to 2.18.11 (#560) | line 6753 | present |
| #561 | `3dd37aea` | Retire the sync-to-cloud push workflow and add a parsed-YAML guard that keeps it retired (#561) | none | not needed: CI only |
| #562 | `135b65e9` | Give every gate-file path a code owner (#562) | none | not needed: CI only |
| #564 | `f2a0deab` | docker-publish.yml: read-only verify builds on main and dispatch, publish only from a platform-v tag push with a release guard (QGF-273) (#564) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #565 | `0e58129d` | trust_calculator.go: factor 1, 2, 5 and 7 comments say what the code computes, held by aim25_comments_test.go (AIM-25) (#565) | none | not needed: code comments |
| #566 | `1c05049d` | README: the quick start states the monitoring-mode default and the strict-mode step its refusal needs (#566) | none | not needed: documentation only, no behaviour change |
| #567 | `d60017a4` | otel-demo smoke scripts run in their own compose project and remove only what they started (#567) | none | not needed: demo smoke tooling |
| #568 | `156e3553` | Batch: land 7 open pull requests and 3 related fixes, one commit each (#568) | line 14 | added by this reconciliation |
| #572 | `d15e3b52` | Remove 16 backup-suffixed files under the admin dashboard directory (#572) | none | not needed: removes backup copies that no route served |
| #573 | `786c31e1` | Automated code review: a planner comment and test name give #573 and #575 their real file and line counts | none | not needed: named in the subject of a review-tooling fix, which changes CI only |
| #575 | `786c31e1` | Automated code review: a planner comment and test name give #573 and #575 their real file and line counts | none | not needed: named in the subject of a review-tooling fix, which changes CI only |
| #579 | `d7ff3905` | The web TypeScript build cache is no longer tracked or committed (#579) | none | not needed: stops tracking a build cache file |
| #581 | `cba1cb69` | Credentials links scroll to their section and mark the key or token they name (#581) | line 6914 | present (entry added by the same change; it does not cite the number) |
| #583 | `ccd836b6` | Agent page fits phone widths, names the Java SDK's credential source and survives a malformed trust-score answer (#583) | line 6941 | present |
| #585 | `c86017ec`, `02d75a40` | The shutdown s1_refusals line states how long its period lasted, MCP rescoring retries after a server fails to score, four sign-in pages render text at 14px, and the demo render keeps its secrets in a mktemp directory (#585) | line 3449 | present (entry added by the same change; it does not cite the number) |
| #587 | `16e34ea0`, `31e93214` | The docker-publish workflow-shape test refuses build cache inputs on the verify builds and a QEMU step without cache-image: false (#587) | none | not needed: CI and repository tooling only |
| #590 | `c094efe3` | The header account menu is tested at a phone viewport too short for it, proving it caps its height above the bottom tab bar and scrolls (#590) | none | not needed: tests only |
| #591 | `96b57be1` | The phone tab bar shows an admin's tabs when the profile request is refused (#591) | line 92 | added by this reconciliation |
| #592 | `b21cb57b` | The end of a dashboard page clears the bottom tab bar's raised Secure button on tablet-width screens (#592) | line 6904 | present (entry added by the same change; it does not cite the number) |
| #593 | `2b7a054a`, `97724d10` | MCP security policies match host names in any case, report unreadable rules, and refuse a floor off the trust scale (#593) | line 3602 | present |
| #595 | `035eb6b7` | onboarding: events are capped per organization, a token mint is retried only after a concurrent mint, sign-out clears the viewed marker, and init links to the dashboard the server reports (#595) | line 6296 | present (entry added by the same change; it does not cite the number) |
| #599 | `2de8ab10`, `898b1228`, `9745cd02` | The agent page's Connections and Security tabs and the admin security policies page fit a 375px screen, and an agent without a capability report is answered 204 instead of 404 (#599) | line 3201 | present (entry added by the same change; it does not cite the number) |
| #603 | `255d7de4` | Java SDK token recovery sends the key the server reads, and the FGA docs, migration number and changelogs are corrected (#603) | line 5848 | present (entry added by the same change; it does not cite the number) |
| #604 | `669cb82a` | Java SDK listAgents URL-encodes the status filter and its other query parameters (#604) | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| #606 | `b9e4af98`, `ab19b7ac` | An agent's activity lists an audit record whose address or user agent is NULL (#606) | line 628 | present |
| #607 | `f5a06cad` | The fleet governance guide's deployment step creates an account to sign in with (#607) | line 5995 | present (entry added by the same change; it does not cite the number) |
| #608 | `1f5c7a7c` | Every changelog heading separates the change type from its summary with an em dash (#608) | line 5961 | present (entry added by the same change; it does not cite the number) |
| #611 | `6e2f79d9` | The 1280x800 menus test waits for the dashboard shell, so it no longer fails when the overlay check runs before the sidebar renders (#611) | none | not needed: tests only |
| #612 | `bc2f4e30`, `982a86a5` | A logged-out SDK token is refused at recovery with or without a revocation store, and report_capabilities, the agents directory test and ruff findings in the Python SDK are fixed (#612) | line 3500 | present |
| #613 | `2fed3245`, `1eac5305` | PQCAgentMiddleware admits an agent in hybrid mode only by a request signed with both Ed25519 and ML-DSA, Ed25519AgentMiddleware refuses it, and refusing an unrecognised agent key costs a signature verification (#613) | line 3436 | present (entry added by the same change; it does not cite the number) |
| #615 | `38554e10` | The web typecheck generates next-env.d.ts before tsc reads it, and the changelog records the untracked file and the pattern the logo split matched (#615) | line 5877 | present (entry added by the same change; it does not cite the number) |
| #617 | `009d0030`, `74c9fc62`, `46b2b93f`, `4b74c22a` | The server refuses the signing seeds the card attestation conformance vector publishes, aim-server check-agent-keys names the agents whose stored key does not open, and three SDK request bodies are shared fixtures (#617) | line 421 | present |
| #619 | `564d972c` | The third-suite refusal for an issued trust credential cites the ATX credential schema and section 14 (#619) | line 5968 | present (rewords the existing entry it changes) |
| #620 | `36c02a05` | The next-env.d.ts ignore check passes only on a rule in the repo-root .gitignore (#620) | none | not needed: tests only |
| #621 | `572114c3` | LocalVerifier refuses a malformed ML-DSA-65 trust anchor at construction instead of rejecting every hybrid credential (#621) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #623 | `0906da09` | SDK requests carry the members the MCP routes read, and MCP server creation requires a name and a URL (#623) | line 3510 | present |
| #625 | `42f8a819` | The Java SDK's token recovery carries its access token and registerMcp adds to the agent's list, with migration, docs and test config follow-ups (#625) | line 5779 | present |
| #627 | `c895a26c`, `28ec35ff` | The SDK's install line is pip install aim-sdk, and the SDK README's Requirements name PyJWT and the optional rich extra (#627) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #629 | `24f56f9b` | Java SDK docs show registerMcp adding a server to the agent's list and printing what AIM answers (#629) | none | not needed: documentation only, no behaviour change |
| #630 | `48f713ad` | The Java SDK sends no token recovery request once its access token has expired (#630) | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| #631 | `69f83359` | The TypeScript SDK's npm run lint covers scripts/ and tests/ as well as src/ (#631) | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| #633 | `88da9fe8` | Alerts show their repeat count, and concurrent repeats share one alert (#633) | line 5752 | present |
| #635 | `1c54ccb1` | The agent API lists capabilities as the credential does, and A2A verification refuses suspended agents (#635) | line 5613 | present |
| #637 | `736769b2` | Repeated capability-check alerts are counted on one alert, and other follow-ups (#637) | line 5724 | present |
| #639 | `fe60dd59` | A key expiry sweep that cannot mint one organization's trace keeps the earlier agents' errors and goes on (#639) | none | not needed: changes the key expiry sweep's recorded path, which runs only with a transition recorder; the server sets none |
| #641 | `b94e4e33` | A transition change given its own agent's opening event id is refused, and the opening and replay tests pin the record-type and first-record conditions (#641) | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| #643 | `a820a51d` | An A2A request whose nonce cannot be recorded does not verify as valid, and other follow-ups (#643) | line 5596 | present |
| #645 | `948c47b4` | An A2A request whose nonce cannot be stored is answered 400, a failed agent read during A2A verification is answered 500, and a transition change given another agent's opening event id is refused (#645) | line 5577 | present |
| #648 | `79c0ec21` | Filing a pending capability request no longer reads every agent of the organization (#648) | line 5562 | present |
| #649 | `064119d0` | A request that POST /api/v1/a2a/verify refuses with 400 is answered with valid: false, as the 401 is (#649) | line 5554 | present |
| #652 | `fb77bff6`, `cf1e988d`, `d1c29ca7` | A2A policy evaluation reads the policies of the caller's organization, and both SDKs send the A2A security settings under the names the server reads (#652) | line 3486 | present |
| #654 | `cf1e988d` | POST /api/v1/a2a/verify carries the shared rate limiter, the test that it is mounted once reads the server's routes, and the #652 changelog heading covers only requests that reach the routes (#654) | line 5429 | present |
| #655 | `f700513f` | The Quick start says a pending agent's call runs on a new organization and is refused on a strict-mode one (#655) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #656 | `40d79275`, `22f07c4f` | The SDK verification route decides with the FGA engine's five steps, and an agent with no FGA policy is decided under a default policy (#656) | line 493 | present |
| #657 | `a4515938`, `39b32e7a`, `e486152b` | The server mounts /api/v1/pam routes for capability tiers, approvals of SUPER_PRIVILEGED capabilities, break-glass declarations and certification campaigns of an organization (#657) | line 398 | present |
| #658 | `f309470a` | An SDK verification of a granted capability is denied in strict mode when the agent's trust score is below the minimum of the capability's definition (#658) | line 3532 | present |
| #659 | `2b5941aa` | The README says when the server recalculates the trust score, and that a verification request does not (#659) | none | not needed: documentation only, no behaviour change |
| #660 | `ea6955fc` | The setup, installation and contributing guides name Go 1.25+, Node.js 20.19+ or 22.13+ and npm (#660) | line 5504 | present |
| #661 | `24efde24` | The Quick start's sample output shows the lines this SDK prints, and says when the token-rotation line appears (#661) | none | not needed: documentation only, no behaviour change |
| #662 | `97f9a81f` | The TypeScript SDK's verifyAction posts to the route the server serves for action verification (#662) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #663 | `c1257b6a` | The pages that install the Python SDK give pip install aim-sdk, the README's install command (#663) | line 5487 | present |
| #664 | `b085ce04` | The Python SDK's requirements.txt and documented install commands list PyJWT, so login and secure() run (#664) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #665 | `dbbc70b3` | The documented secure() calls no longer pass private_key, which secure() does not take; a keypair you hold goes to AIMClient (#665) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #666 | `b93fe732`, `8446c2bd`, `49c1c58c` | The Python SDK reads the counts and the average of an agent's user ratings, from a route that serves an agent's own credential for its own ID only (#666) | line 291 | present |
| #667 | `60c693c1` | The guides no longer describe a provider sign-in, which no route serves (#667) (subject paraphrased) | line 5467 | present |
| #668 | `9b0dcc4c` | migrate status lists migrations without changing the database, create writes a new file, and down, force and drop are refused (#668) | line 5373 | present |
| #669 | `6459a0c6` | The guides clone from and link to github.com/opena2a-org/agent-identity-management, and a lint rejects a link to the opena2a account, which does not exist (#669) | line 5266 | present |
| #670 | `02bf3d44` | The setup, deployment and troubleshooting guides check the server's health at /health, the path it mounts (#670) | line 5187 | present |
| #671 | `6ace805d` | The API reference pages, tutorials and guides name only routes the server registers (#671) | line 5349 | present |
| #672 | `69b4b85f`, `3ac73bdf` | The server delivers agent, trust score, alert and API key events to subscribed webhooks, with limits on retries, pending deliveries and attempts in flight (#672) | line 3393 | present |
| #673 | `7f8934cc` | The guides' docker commands name the services, containers and network the compose file defines (#673) | line 5158 | present |
| #674 | `5a42d1be` | The deployment, installation and troubleshooting pages name only files the repository carries (#674) | line 5318 | present |
| #675 | `1cec3ad0` | The guides list the nine trust score factors with their weights and give 59% as a new pending agent's starting score (#675) | line 5223 | present |
| #676 | `91088f03` | The tutorials, the deployment reference, the backend README and the SDK test plan give no count the repository does not bear out (#676) | line 5136 | present |
| #677 | `f35db69b` | SECURITY.md describes the CSRF, SDK credential, configuration and personal-data controls the code has (#677) | line 5299 | present |
| #678 | `05eb8268` | The security guides name the SDK and agent credential files, link the SDK tokens list at /dashboard/credentials, give 2 hours as the access token lifetime and say a reused refresh token ends its family and writes an audit record (#678) | line 5198 | present |
| #679 | `da77555d`, `a31fbb85` | The server grants delegations between agents and verifies a delegation chain, applying each delegation's trust attenuation factor and floor (#679) | line 3632 | present |
| #680 | `695e5985` | The TypeScript examples in the PQC, A2A and direct API guides import names the SDK exports (#680) | line 5281 | present |
| #681 | `dd71d0b0` | The guides, the roadmap and aim-sdk login say a call for a capability the agent was not granted is refused only in strict mode, and name where an administrator turns strict mode on (#681) | line 3554 | present |
| #683 | `e2548969`, `9864ec1d` | Dashboard paths with a percent-escape that does not decode are answered with the not-found page, and the readability sweep answers the onboarding metrics (#683) | line 605 | present |
| #685 | `324d85b1` | aim-sdk init reads the bootstrap token from AIM_BOOTSTRAP_TOKEN only, GET /api/v1/status reports no oauth feature, and the README names both verification routes (#685) | line 5455 | present |
| #686 | `9a929ca4` | The provider sign-in docs test reads the tracked file list from git instead of skipping local files by name (#686) | line 5089 | present |
| #687 | `69949c7a` | The backend image's license manifest takes the gopkg.in/yaml.v3 URL from a committed file, not a lookup during the build (#687) | line 5097 | present |
| #688 | `eed6d38b`, `159374d3`, `7560de39`, `3a32c5a1` | A check holds the root and apps/web package.json versions to the newest CHANGELOG release and the platform-v tag (#688) | line 568 | present |
| #690 | `efcb2bfe`, `8272d89d`, `094b69e0` | test: the TypeScript SDK tests run the token literal lint and its tests on every pull request (#690) | line 5011 | present |
| #691 | `93b43997` | Secret Detection runs hackmyagent 0.33.2, a pinned version, instead of whatever npm serves as latest (#691) | none | not needed: CI and repository tooling only |
| #693 | `5671ab6a` | The startup cache line says Redis is disabled when REDIS_HOST is empty, and the scratch-repository docs test passes under a global git excludes file (#693) | line 5043 | present |
| #695 | `9d829c80`, `74a3b230` | Backend print and log lines carry no pictographic marks, the TypeScript SDK's test titles open with what they check, the contributing guide says what the SDK tests need, and the SDK download drops its unserved SDK branches (#695) | line 3174 | present |
| #697 | `7f383b1e` | A database password with a single quote or a backslash reaches PostgreSQL unchanged, the startup summary names a Redis it could not reach, the token rotation guide quotes the line the SDK prints, and the SDK route census reads every mounted route (#697) | line 4816 | present |
| #699 | `8fdf739f` | The phone menu is a modal dialog, the Secure tab's link covers its raised icon, and colour regressions fail unit tests (#699) | line 4778 | present (entry added by the same change; it does not cite the number) |
| #701 | `473c698b` | The compose bootstrap passes a database password with a quote or a backslash unchanged, the revocation startup line names an unreachable Redis, the token guides say what the backend does, four A2A SDK methods and the JIT approval link reach real routes, and the list smoke runs no docker exec (#701) | line 4705 | present |
| #702 | `bcbd3978` | The docker publish release-only and license compliance tests name their scratch directories and stub helpers after what they serve (#702) | none | not needed: tests only |
| #705 | `af2d6664` | The tab bar test catches every spelling of a raised icon and a browser test measures the tabs, .hmaignore documents expires:, and a web test makes the log check the scan skips (#705) | line 4794 | present (entry added by the same change; it does not cite the number) |
| #707 | `d8b11575`, `a61457b0`, `d717a0cd` | The TypeScript SDK's typecheck script type-checks the two integration test files it left out (#707) | none | not needed: SDK typecheck tooling |
| #709 | `5d12750e` | The server reads PGPASSWORD or the password file when POSTGRES_PASSWORD is empty, aim-bootstrap names a driver variable it refuses, GET /api/v1/agents applies the status and agent type filters, and the Python SDK's log_capability_result sends no request (#709) | line 4637 | present |
| #711 | `ac121c4e` | The connection-string login tests record each login before the recording server refuses it (#711) | none | not needed: tests only |
| #714 | `51637c91` | The scratch-name guard parses the SDK tests, reads the test files under src too, and the telemetry tests there name their scratch directories after the test (#714) | none | not needed: tests only |
| #716 | `3988b3c1`, `e5a7828e`, `47432ed5` | The Java SDK's logCapabilityResult sends nothing to the withdrawn result route and listAgents documents the four agent statuses, with eight more AIMClient request and answer fixes (#716) | line 4595 | present |
| #718 | `1dad1181` | GET /api/v1/tags/search answers 400 for a q or category that is not UTF-8 text or holds a NUL character, the Python SDK sends the MCP server name of its cached-capability lookup as one query value, the password file notes say which modes the driver reads, the agent handler is gofmt-formatted, and the Python SDK tests import only what they use (#718) | line 4575 | present |
| #720 | `9f4db3aa` | POST /api/v1/agents/:id/pqc-key answers the hybrid mode it stored, four more handlers answer 400 for a query value that is not UTF-8 text or holds a NUL character, every Go file of the handler package is gofmt-formatted, and the token refresh comment says the lease does not rule out two refreshes (#720) | line 4480 | present |
| #722 | `b10352c3` | The pending user list gives lastLoginAt in UTC, the release smoke names its PostgreSQL image by digest, and the quick-start and SDK lock docs say what a rerun and Linux do (#722) | line 4356 | present |
| #724 | `a6481184` | The prerequisite-version test reads README.md for stale Go, Node.js and pnpm statements (#724) | none | not needed: tests only |
| #726 | `16485200` | The Python SDK tests run against an empty temporary home instead of the stored credentials of the machine (#726) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #728 | `66d8626f` | Two more IPv6 ranges are refused, and mail and approval log lines name ids (#728) | line 4130 | present (entry added by the same change; it does not cite the number) |
| #730 | `b0674171` | The Detect MCP servers dialog says its command can replace the agent's key pair (#730) | line 3763 | present (entry added by the same change; it does not cite the number) |
| #731 | `c02b4300` | The dialog Close button is a 44x44 px target (#731) | line 3752 | present |
| #733 | `5ff256a8` | CHANGELOG drops the sentence saying the Java A2A demo reads AIM_URL only (#733) | none | not needed: corrects the wording of an existing entry |
| #735 | `9c9f4771` | The Python SDK download's docs point at the test suite in the repository (#735) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #738 | `647ba0f7`, `21c35cc2` | A revoked session's request that the session middleware refuses at a credential route writes the credential_mint_refused audit row and SECURITY line (#738) | line 448 | present |
| #740 | `17ab6864`, `76a243aa`, `8021f566` | fix(sdk/python): aim-sdk demo links to the dashboard address the server states, as aim-sdk init does (#740) | line 585 | present |
| #742 | `327556fd` | The dashboard's API reference lists the fields POST /api/v1/api-keys reads and returns (#742) | line 3122 | present |
| #744 | `c679aff5` | Reading a user's or an organization's SDK token rows gives the read's error when it fails after the first row (#744) | line 3137 | present |
| #747 | `4592f0ee` | aim-bootstrap prints its step lines the way the bootstrap guide shows them (#747) | line 2718 | present |
| #748 | `f770c709` | POST /api/v1/a2a/verify reports a request from an agent in hybrid mode signed with Ed25519 alone invalid (#748) | line 2729 | present |
| #749 | `170a6201` | Registering an agent or an MCP server applies only tags of the caller's organization (#749) | line 2744 | present |
| #751 | `3a3074d1` | Audit rows keep their request's user agent, CI runs the executable lint, and the secrets client encodes its query (#751) | line 2360 | present |
| #753 | `56500793`, `f367970c` | The two-tab token refresh e2e spec sends its refresh and its closing GET /api/v1/auth/me again after a 429 from the /auth rate limit, so it passes on its first attempt when the rest of the suite has spent the limit (#753) | line 2568 | present |
| #755 | `2f5fb4ce`, `ff4e9168` | aim-sdk demo names no mode the server has not reported, status reports an unreadable credentials file and the saved agents, init refuses --token and a path as a name and says whether the token was used, logout removes the lock file, and aim-arp -V prints the version (#755) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #756 | `3ceb123d` | The ARP runtime twin caps its home-directory event and escalation logs at 50 MiB each and keeps the five newest rotated copies (#756) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #757 | `cefa3387` | The API server listens on 127.0.0.1 outside a container unless HOST is set (#757) | line 2385 | present |
| #758 | `476092be` | The CyberArk CCP client sends a Query that names several properties, and a value that holds a space (#758) | line 2408 | present |
| #759 | `c694174a` | A test fails when internal/config reads the environment other than through lookupEnv (#759) | none | not needed: tests only |
| #761 | `65ff36ce` | The SDK integration scripts pass aim-bootstrap and the server the DATABASE_URL user, password and database name decoded (#761) | line 2288 | present |
| #762 | `ca83992d` | ARP log rotation follows a symbolic link and keeps five rotated copies beside other files, telemetryLog refuses 0, a bodiless 200 to an alert channel test is a server error, and the webhook dialogs fit a 375px screen (#762) | line 2302 | present |
| #764 | `98c35f7b` | The backend integration tests that need a running backend are skipped unless BACKEND_URL names one (#764) | line 2314 | present |
| #765 | `e7944e64` | GET /api/v1/secrets/audit refuses a since, limit or offset it cannot read (#765) | line 2343 | present |
| #767 | `58c4bbd5` | A Sentinel token answer without a bearer token is not retried (#767) | line 2852 | present |
| #769 | `f275f7b7` | The A2A policy evaluation test looks for another organization's trust scores outside the answer's evaluatedAt time (#769) | none | not needed: tests only |
| #771 | `849e938f` | The Python SDK's get_audit_log and list_namespaces encode their query values, so a since with a positive UTC offset reaches the server as passed (#771) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #772 | `97c1076b` | The Python SDK's A2AClient.list_skills() requests GET /api/v1/a2a/agents/{agentId}/skills with the client's own agent ID, a route the server registers (#772) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #773 | `ab9e66cf` | The Python SDK's A2AClient.revoke_attestation raises without a request, because the server mounts no route that revokes an A2A skill attestation (#773) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #775 | `2a78fbe9` | The MCP server page lays out within a 375px phone viewport and its Integration tab opens on a tap (#775) | line 2024 | present (entry added by the same change; it does not cite the number) |
| #776 | `3e67cf94` | The register-agent dialog's Copy button sits in a row above the verification command instead of over its first line (#776) | line 2051 | present |
| #778 | `80808c01` | ATC verification spec section 5 describes the signed revocation list the client reads at the discovery document's endpoints.revocations (#778) | none | not needed: documentation only, no behaviour change |
| #780 | `e9bc83d4` | The register-agent dialog's Copy key button stays inside the API key card at a 375px viewport (#780) | line 1789 | present |
| #781 | `71d9aebd` | The API endpoint summary names the four dependencies GET /health/ready reports (#781) | line 1804 | present |
| #783 | `2051f6d4` | The revocation list timeout test's stalling authority aborts its answer once the client has left, instead of completing a list body that stops (#783) | none | not needed: tests only |
| #784 | `f8949a4a` | A revocation list refresh whose answer ends after the read's deadline is refused as timeout, however the answer's body ends (#784) | line 1654 | present |
| #785 | `5fc3b8f6` | ATC verification spec section 5.1 says the list request carries If-None-Match only at the same address and under the same keys (#785) | none | not needed: documentation only, no behaviour change |
| #786 | `bbaecad0` | ATC spec section 5 names GET /metrics and GET /health/ready as reading the revocation list held, and says no request the backend answers fetches it (#786) | none | not needed: documentation only, no behaviour change |
| #788 | `69028438` | The quick start generates an administrator password the backend accepts, and stops when the backend refuses the seed (#788) | line 1668 | present |
| #789 | `427d75e5` | The README's Quick start says the PyPI release does not yet print a sign-in code, and links a self-hosted install from the dashboard (#789) | line 863 | present |
| #790 | `e4fca7de` | POST /api/v1/agents answers 400 naming the members a request leaves out, and the dashboard's API reference example registers an agent (#790) | line 1695 | present |
| #791 | `3fa14c04` | The deployment guide's From source section creates the first administrator and says how to sign in (#791) | line 1720 | present |
| #792 | `c2618a14`, `0a6cfc4f`, `76398269` | The per-agent SDK download keeps a port such as 18080 in the server URL it embeds, and the trust calculator's comments say how a factor with no data counts (#792) | line 271 | present |
| #795 | `29324d54` | docs/API.md's POST /api/v1/agents request sends agentType and registers an agent, and its agent answers show agentType (#795) | line 1486 | present |
| #797 | `fdec4158` | The Java SDK's AgentTypeServerTypesTest reads the server's agent types from recognizedAgentTypes (#797) | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| #798 | `c66e53b9` | ARP's budget status reports the current hour's L2 calls (#798) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #799 | `c5ed27fd` | The SDK install guides name the dashboard's SDK & docs page, its download buttons and the archive name the server sends (#799) | line 1574 | present |
| #801 | `6a6eb689` | ARP applies a batched L2 answer to each event of the batch (#801) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #802 | `010a56e7` | ARP queues a batched event only while the L2 budget can pay for its batch (#802) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #803 | `64f67c51` | The e2e global setup derives a 24-character intermediate password, so a seeded admin password of 64 to 72 bytes completes the required change (#803) | none | not needed: tests only |
| #805 | `5406b691` | ARP logs a batched event again with its L2 verdict, three more guides name the SDK page, and the docs unzip the newest SDK download and replace ~/.aim/sdk_credentials.json (#805) | line 1600 | present |
| #811 | `54cf9519` | The account deletion purge removes the OpenID Connect sign-in configuration of an organization it deletes (#811) | line 892 | present (entry added by the same change; it does not cite the number) |
| #812 | `6157b78f` | The deployment guide's SIEM section names the failed sign-ins answered 403 and 500 and their reasons provider_sign_in_required and sign_in_refused (#812) | line 1129 | present |
| #817 | `79cf9409` | The agent-request-v2 payload test sends a member name spelled with a JSON escape, and fails when one of its cells repeats another's input (#817) | none | not needed: tests only |
| #818 | `33ea64b7` | The handler tests' password routes hash a changed password at bcrypt's minimum cost, so the password change tests finish within their timeout under -race (#818) | none | not needed: tests hash at bcrypt's minimum cost; the server's cost is unchanged |
| #825 | `b1a5325f`, `f2bf6008` | The detection report routes take a report an agent signed with ML-DSA or with both keys (#825) | line 433 | present |
| #826 | `af7a0d89` | The API reference says that a verification, a reactivation and an agent delete write an audit log entry (#826) | line 382 | present |
| #828 | `9e8a9f87` | The Python SDK's quick-start refresh test passes after the perform_action claims tests in the same pytest session (#828) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #829 | `a9f65c50` | scripts/lint-token-literals.mjs exits 0 on the tracked tree: the test values behind its six findings in three backend tests are replaced with values it does not report (#829) | line 367 | present |
| #830 | `1ce0c85d` | Secrets namespace creation refuses a name that resolve refuses to sign, in the server and in the Python and TypeScript SDKs (#830) | line 351 | present |
| #831 | `99ba0a52` | The Python SDK's secrets.resolve() checks, signs and sends the text a str subclass holds, so a (str, Enum) namespace or operation is signed as it is sent (#831) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #832 | `abe4ab15` | The TypeScript and Python SDKs refuse a secrets namespace ID that is not a UUID before they send a request, and secrets resolve refuses a namespace or operation that holds a vertical bar, a carriage return or a line feed (#832) | line 333 | present |
| #834 | `46cca5df` | A negative offset on the agent audit log route is read as 0 instead of answering 500 (#834) | line 319 | present |
| #835 | `0fa1ecda` | The ARP runtime twin divides its noised fleet gradient by its L2 norm when that norm is above 1.0 (#835) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #836 | `6fa1116e` | The TypeScript SDK's engines field admits only the Node.js releases that load its CommonJS build (#836) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #838 | `bbc835e9` | scripts/lint-public-surface.mjs exits 0 on the tracked tree: the dialog test's mocked read error names a path that is not a home directory (#838) | line 308 | present |
| #840 | `1bd88dbf` | The Python SDK's secrets.create_namespace() checks and sends the text a str subclass holds, so it refuses a (str, Enum) namespace that resolve() refuses to sign (#840) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #841 | `0dfc1533` | A negative offset on the MCP server audit log route is read as 0 instead of answering 500 (#841) | line 262 | present |
| #844 | `4ce1ca5b` | A negative offset on the agent alert, agent violation, admin alert and A2A violation routes is read as 0 instead of answering 500 (#844) | line 249 | present |
| #846 | `891205a1` | A negative limit on the agent alert, agent violation, admin alert and A2A violation routes is read as the route's default page size instead of answering 500 (#846) | line 216 | present |
| #847 | `37d1d195` | A limit below 1 on GET /api/v1/admin/alerts is read as the default page size of 100 instead of answering 500 (#847) | line 201 | present |
| #849 | `8e2c7ca9` | fix: a limit of 0 on the agent alert, agent violation and A2A violation routes is read as the route's default page size instead of answering an empty page (#849) | line 187 | present |
| #852 | `29d40689` | fix(aim-arp): an unknown argument is named on stderr with a pointer to --help, init refuses a name whose credentials file name is over 255 bytes, and the telemetry text drops emphasis capitals and em dashes (#852) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #853 | `ff17ef49` | fix(arp): the proxy listens on 127.0.0.1 unless its config sets a host (#853) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #854 | `85f79340` | fix(backend): a failed FGA policy or capability read is answered with a fixed reason, not the database error (#854) | line 163 | present |
| #855 | `bc83e143` | fix(backend): the agent alert, agent violation, admin alert and A2A violation routes cap limit and offset (#855) | line 144 | present |
| #857 | `1b4d86a9` | feat(arp): getStatus() reports whether L2 and the runtime twin run (#857) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #859 | `88b03d3c`, `04879c56` | fix(sdk/python): secure() refuses an agent file's server address with a user name or password before any request (#859) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| #860 | `943eca84` | fix(sdk): ~/.aim is created 0700, and an existing ~/.aim or ~/.aim/agents loses only its group and other bits, with a line naming both modes (#860) | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #861 | `c659aa5c` | fix(sdk/arp): loadConfig names the file and the parser's message when a config does not parse (#861) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #862 | `79756fe5` | fix(arp): signatureTelemetry: { enabled: false } in the ARP config stops the runtime twin's fleet gradients (#862) | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| #866 | `c4b3ab15` | test(sdk/java): find the recovery route's sign-in check when its label is the middleware constant (#866) | none | not needed: tests only |

## Commits with no pull request number

Commits whose subject carries no `#N` and that change code, listed by commit. 124 more change only tests, CI, documentation or this changelog, or are merge commits, and are not listed.

| Commit | Subject | Entry | Decision |
| --- | --- | --- | --- |
| `9cc1dbc4` | feat(backend): GET /health/ready reports the Container Apps revision the server runs in | line 126 | present (entry added by the same change; it does not cite the number) |
| `37ac41ad` | fix(deps): move source-map-js to 1.2.2 and baseline-browser-mapping to 2.11.31 in both lockfiles | none | not needed: transitive lockfile versions, no behaviour change |
| `fe4aa903` | fix(arp): aim-arp telemetry status shows the sensor's public key each report carries | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `3f8b1e9b` | fix: the telemetry disclosure, the README and the aim-arp telemetry help and log footers name the sensor's public key each report carries | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `bb61bc66` | docs: the org pseudonym's doc comment names the sensorId and publicKey every report also carries | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `cad5283f` | fix: the GTIN event ARP sends carries _unattributed as its package name in place of the agent name | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `cc7bb4fc` | feat: ARP reports a failed L2 call once, with its endpoint and error, and counts it in the budget status | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `19811e80` | fix: aim-sdk --version names a source build, and the SDK READMEs say what 2.0.3 lacks and what init needs from the server | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `97bc3c9f` | fix(sdk/python): aim-sdk demo --cleanup sends the stored sign-in token only to the server it signed in to and refuses a --url naming another | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `58cf6797` | feat(record): the verifier reports a chain that held checkpoints name and the deployment no longer holds as removed without record, unless exactly one tombstone accounts for it | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `94f904b9` | feat(record): a signed checkpoint names a chain's head outside its records, and the verifier reports records removed from the end and a record replaced after the checkpoint was taken | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `0d76f304` | fix(sdk/arp): a YAML config that does not parse fails with js-yaml's reason and position, not an excerpt of the file | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `8c8378a4` | fix(deps): move next to 16.4.2 and the sharp override to 0.35.5 (advisory id omitted) | line 14 | added by this reconciliation |
| `812e14c9` | feat(record): the store's writer appends a key rotation that moves the chain to its next key, and a correction of a record of the organization's chain | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `700df789` | feat(record): a correction record names an earlier record of its chain by event_id, and the verifier fails one that names no earlier record | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `b72c7c23` | feat(record): a key rotation record names a chain's next signing key, and the verifier fails a record the retired key signs after it | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `ba68754a` | fix: /health/ready reports the dependencies the service needs to answer; SIEM delivery is read on /metrics | line 230 | present (entry added by the same change; it does not cite the number) |
| `9290cc1c` | The README's Quick start has a committed terminal recording that docs/demo/record-quickstart.sh re-renders, with a demo-sidecar/2 sidecar, a narration file and a text version | none | not needed: README recording and its demo tooling |
| `e948d42e` | AIM_PUBLIC_ORIGIN holds the audience an agent-request-v2 statement must name, and the server logs it at startup | line 767 | present (entry added by the same change; it does not cite the number) |
| `d6e66523` | The Java SDK has an agent-request-v2 signer, not public, that reproduces six of the eight accepted conformance vectors byte for byte | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `a4b4e560` | The TypeScript SDK has an unexported agent-request-v2 signer that reproduces six of the eight accepted conformance vectors byte for byte | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `9a23d5ae` | The SDKs refuse to sign a secrets resolve whose namespace begins with DSSEv1, and the TypeScript SDK refuses a line break or \| in its fields | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `91d2b63e` | /metrics counts the agent-request-v2 requests the server refuses, by reason and calling SDK | line 789 | present (entry added by the same change; it does not cite the number) |
| `3b88c8ab` | /metrics counts the requests each agent signing scheme authenticates, by scheme and calling SDK | line 811 | present (entry added by the same change; it does not cite the number) |
| `c7c2c7bb` | The backend can authenticate a request signed as an agent-request-v2 statement, which no route uses yet | line 833 | present (entry added by the same change; it does not cite the number) |
| `eaac6460` | A user whose organization requires sign-in through its OpenID Connect provider confirms the deletion of their account with their password | line 1113 | present (entry added by the same change; it does not cite the number) |
| `494d633b` | An administrator sets up the organization's OpenID Connect sign-in in the dashboard | line 1081 | present (entry added by the same change; it does not cite the number) |
| `b341676c` | An organization can require its users to sign in through its OpenID Connect provider | line 1059 | present (entry added by the same change; it does not cite the number) |
| `c52ec441` | The dashboard signs a user in through the organization's OpenID Connect provider | line 1036 | present (entry added by the same change; it does not cite the number) |
| `ae1c7fb9` | An organization's OpenID Connect sign-in can create the user at the first sign-in | line 1018 | present (entry added by the same change; it does not cite the number) |
| `9ce73f4f` | An organization's users can sign in through the organization's OpenID Connect provider | line 992 | present (entry added by the same change; it does not cite the number) |
| `9e00ccb4` | The account deletion purge deletes an organization once it has deleted the organization's last user | line 1268 | present (entry added by the same change; it does not cite the number) |
| `d8ccc5cb` | A signed-in user can ask for the deletion of their account from the dashboard's account menu | line 1323 | present (entry added by the same change; it does not cite the number) |
| `9e13059a` | A signed-in user can ask for the deletion of their own account, which is deactivated at once and deleted 29 days later | line 1300 | present (entry added by the same change; it does not cite the number) |
| `ac945264` | An agent's registration, update, verification, suspension, reactivation, revocation and MCP server changes send trust_score.changed for the scores they store | line 903 | present (entry added by the same change; it does not cite the number) |
| `95afe664` | Trust score calculations, capability grants and revocations, and drift penalties send trust_score.changed for the scores they store | line 927 | present (entry added by the same change; it does not cite the number) |
| `f8013c9f` | HTTP request metrics label a request a route served with that route's path, so the values a caller puts in a path add no series | line 947 | present (entry added by the same change; it does not cite the number) |
| `409fd31d` | The server runs a capability drift monitor every 15 minutes and re-evaluates trust scores every hour | line 970 | present (entry added by the same change; it does not cite the number) |
| `7ba598f3` | Deleting an admin removes the admin's email from the admin access reviews kept as compliance evidence | line 1345 | present (entry added by the same change; it does not cite the number) |
| `f8471cc5` | Deleting a user deletes the user's registration requests and the copies of the user's email and name in audit rows | line 1366 | present (entry added by the same change; it does not cite the number) |
| `7e93d715` | Deleting a user clears the copies of the user's name and email that agents and MCP servers hold | line 1401 | present (entry added by the same change; it does not cite the number) |
| `1ad10cf0` | A user who created an agent, an API key, an MCP server or another row the organization keeps can be deleted | line 1426 | present (entry added by the same change; it does not cite the number) |
| `f9b8cacb` | Each agent private key the server stores is encrypted under a data key of its own, stored encrypted under the master key | line 1461 | present (entry added by the same change; it does not cite the number) |
| `0212af7e` | The deployment guide says the quickstart server runs no Registry bridge and names the variables that turn it on, and a test holds the bridge to REGISTRY_BRIDGE_ENABLED=true | line 1171 | present (entry added by the same change; it does not cite the number) |
| `52c4443b` | GET /api/v1/verification-events with agent_id answers 500 when the read fails and reads an offset that is not an integer as 0, and six backend files carry no ineffassign or staticcheck finding | line 1146 | present (entry added by the same change; it does not cite the number) |
| `0d858dee` | The FGA engine holds no Registry ASC address until FGA_REGISTRY_ASC_URL sets one | line 1161 | present (entry added by the same change; it does not cite the number) |
| `a784b762` | OAuthUser carries the email-verified claim of Google's and Okta's user info, and a missing claim reads as not verified | line 1253 | present (entry added by the same change; it does not cite the number) |
| `d9109147` | The Python SDK's import-cost tests run the aim-sdk probe in its temporary home, so they copy no credentials from a .aim folder above the checkout | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `71d3b418` | A request that ends in a returned error is recorded in api_calls with the status the client received | line 1240 | present (entry added by the same change; it does not cite the number) |
| `a68afeea` | The server's ADMIN_PASSWORD seed adds the default security policies a fresh database did not receive | line 1217 | present (entry added by the same change; it does not cite the number) |
| `6eb4a8a7` | aim-bootstrap --default adds the default security policies a fresh database did not receive | line 1195 | present (entry added by the same change; it does not cite the number) |
| `759263f5` | The SDK tabs link pages that continue the hosted flow, the TypeScript tab says where its API key comes from, and the Python example declares db:read | line 1631 | present (entry added by the same change; it does not cite the number) |
| `331b1087` | A stale copy of a user written back leaves the recorded password change in place, as the query now states and a test against PostgreSQL holds | line 1507 | present (entry added by the same change; it does not cite the number) |
| `32f9717a` | The SDK integration scripts complete the seeded admin's required password change before the tests sign in | line 1553 | present (entry added by the same change; it does not cite the number) |
| `03c6e50f` | The e2e suite completes the seeded admin's required password change before its workers sign in | none | not needed: end-to-end test setup |
| `d23aad40` | A password change or reset marks the user's SDK tokens revoked, and SDK token recovery does not restore them | line 1537 | present (entry added by the same change; it does not cite the number) |
| `2345c2be` | A password change ends every session signed in before it, and the API enforces a required password change | line 1502 | present (entry added by the same change; it does not cite the number) |
| `b2e05b39` | ARP's L2 budget and hourly call limit count the calls in flight | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `61358199` | The Python SDK raises RevocationUnknownError at AIM's 503 atc_revocation_unknown answer without resending it, and aim-sdk login names an ATC error code | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `6f6e0c82` | /metrics carries the age and the entries of the revocation list the server holds with REGISTRY_URL set | line 1891 | present (entry added by the same change; it does not cite the number) |
| `772e2406` | The revocation list refresher reads the discovery document with If-None-Match, and reads the list in full when the document's keys change | line 1909 | present (entry added by the same change; it does not cite the number) |
| `0f5a85bc` | GET /health/ready carries dependencies.revocationState: ok while the revocation list held is fit to answer from, and unavailable with one reason word otherwise | line 1928 | present (entry added by the same change; it does not cite the number) |
| `3c6917f3` | A Registry credential revocation request waiting for a retry is sent after the server starts again | line 1835 | present (entry added by the same change; it does not cite the number) |
| `0db5d049` | No Agent Trust Credential is issued for a revoked agent | line 1869 | present (entry added by the same change; it does not cite the number) |
| `fdbe426d` | With REGISTRY_URL set, the server reads the revocation list an authority's discovery document names, and ATC_CRL_ENDPOINT has no default | line 1961 | present (entry added by the same change; it does not cite the number) |
| `c453c803` | The revocation list client reads the list an authority's discovery document names, holds it only when a listed key signed it, and answers unknown when it holds none | line 1961 | present (matched by content) |
| `7191c15d` | An ATC whose agent is suspended, revoked or not stored is refused, and an ATC whose agent's status cannot be read is answered 503 with Retry-After | line 1999 | present (entry added by the same change; it does not cite the number) |
| `2d3d11b0` | The MCP server setup guide and the register-agent dialog's verification command name the API the dashboard signs in to, on a second stack and behind a reverse proxy | line 2065 | present (entry added by the same change; it does not cite the number) |
| `122a7e3b` | aim-sdk demo signs this machine in when no sign-in is stored and a person can approve the code | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `8b4abf52` | Revoking an agent asks the Registry to revoke the trust credentials it issued for the agent | line 2099 | present (entry added by the same change; it does not cite the number) |
| `ad332f24` | The aim-dashboard image takes no NEXT_PUBLIC_API_URL build argument, and its browser code holds no http://localhost:8080 when the publish workflow passes one | line 2121 | present (entry added by the same change; it does not cite the number) |
| `57ca843b` | docker-compose.yml hands the dashboard the API URL in FRONTEND_API_URL, and the deployment guide says to set it behind a reverse proxy | line 2154 | present (entry added by the same change; it does not cite the number) |
| `5f315f83` | A dashboard built with no NEXT_PUBLIC_API_URL holds no http://localhost:8080 in its browser code, and next build fails when such code does | line 2192 | present (entry added by the same change; it does not cite the number) |
| `9866e43a` | The dashboard reads NEXT_PUBLIC_API_URL from its environment when it renders a page, and the page's connect-src lists that API | line 2226 | present (entry added by the same change; it does not cite the number) |
| `397dc5f6` | On a -frontend hostname, the API documentation page and the MCP server dialog call the API the dashboard signs in to | line 2267 | present (entry added by the same change; it does not cite the number) |
| `87f4fce5` | aim-sdk login prints the sign-in lines secure() prints, and no box banner | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `88b6e096` | The Python SDK's secure() signs this machine in when it finds no SDK credentials and no api_key, at a terminal or in a notebook | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `3db8aeef` | The Python SDK's skill, registration, execution status, trust score and credential calls send only the members the server reads | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `427f357e` | The backend's .env.example and the deployment guide name APP_PORT and HOST, the variables the server reads | line 2336 | present (entry added by the same change; it does not cite the number) |
| `80cb2fa0` | PUT /a2a/cards/:id replaces the card document, and capability requests keep their metadata | line 3711 | present (entry added by the same change; it does not cite the number) |
| `043663a4` | Security events reach Microsoft Sentinel through the Logs Ingestion API, and the Data Collector adapter is removed | line 2839 | present (entry added by the same change; it does not cite the number) |
| `86e01e26` | The startup refusal for a missing KEYVAULT_MASTER_KEY names only the key | line 2707 | present (entry added by the same change; it does not cite the number) |
| `061cde76` | A dashboard test fails when a sidebar entry or hub tab links to a path no page serves | line 2688 | present (entry added by the same change; it does not cite the number) |
| `19e1fd1c` | The server prints one line at start naming its KeyVault master key's source, storage key id and identity | line 2667 | present (entry added by the same change; it does not cite the number) |
| `903ac658` | The Java SDK names a download's maximum age when the server refuses its refresh, and presents the refused token to the refresh route only | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `4de4944c` | The flight-search example's get_fresh_sdk.py prints the dashboard's sign-in and download steps instead of refreshing a stored credential | line 2651 | present (entry added by the same change; it does not cite the number) |
| `165d4a02` | The ARP config validator refuses a non-boolean intelligence.enabled, intelligence.runtimeTwin.enabled or intelligence.guardAnomaly.enabled | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `1896f85f` | The ARP process monitor reads the process table once per poll and never reports its own ps listing as a child | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `0179cbce` | ARP refuses a monitors, interceptors or aiLayer entry that is not in its documented shape, and warns about an unknown top-level key | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `39af59a3` | The TypeScript README's init section opens with the source build, and the npx form follows a release condition | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `cbdec7aa` | aim-sdk login accepts an http: address only on this machine unless --allow-http is passed, from the check init applies | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `d06c7027` | aim-sdk login opens no browser when its output is not a terminal, and prints the page and the code before it waits | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `073bbfda` | Every HTTP request emits a server span, the OTLP exporters choose TLS from the endpoint as the specification defines, and the MCP server gauge is aim_mcp_servers | line 2785 | present (entry added by the same change; it does not cite the number) |
| `143a5c1c` | SMTP delivery is proved with STARTTLS and AUTH PLAIN, every message carries a Message-ID, and the email providers and the default are documented | line 2818 | present (entry added by the same change; it does not cite the number) |
| `da493924` | An organization member adds, edits, tests and deletes alert channels from the dashboard | line 2874 | present (entry added by the same change; it does not cite the number) |
| `d5cd77cd` | An organization's alerts also reach a Microsoft Teams workflow and email addresses | line 2892 | present (entry added by the same change; it does not cite the number) |
| `3659c337` | An organization's alerts reach Slack and PagerDuty in each service's format, with a severity floor per channel | line 2930 | present (entry added by the same change; it does not cite the number) |
| `9f2584be` | A webhook signature covers the time the attempt was sent, so a receiver can refuse a replayed delivery | line 2965 | present (entry added by the same change; it does not cite the number) |
| `ec698b23` | A webhook's signing secret reaches its owner once, can be rotated, and an unknown event or field is refused | line 2989 | present (entry added by the same change; it does not cite the number) |
| `5d59de5c` | The Java SDK's helper classes run against a real backend, and SecurityLogger's output is measured against Splunk HEC's published event format | line 2595 | present (entry added by the same change; it does not cite the number) |
| `a3e772b5` | The Python SDK's LangChain, CrewAI and MCP adapters run against a real backend, which records a verification for each framework call | line 2618 | present (entry added by the same change; it does not cite the number) |
| `f63afc3f` | The fga.authorize spans carry gen_ai.agent.id, the AIP identifier of the agent AIM resolved | line 2579 | present (entry added by the same change; it does not cite the number) |
| `537ac12c` | A SIEM event copies the client address and user agent out of the request it reports | line 2425 | present (entry added by the same change; it does not cite the number) |
| `47b2a0e9` | The HashiCorp Vault round-trip integration test runs against an in-memory KV v2 server, so the integration job needs no vault binary | line 2520 | present (entry added by the same change; it does not cite the number) |
| `cfe0eaf7` | The Python SDK names a download's maximum age when the server refuses its refresh, and presents the refused token to the refresh route only | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `f7a0edc5` | A refused or unreachable SIEM shows in /health/ready and /metrics, and the loss bound is stated | line 3027 | present (entry added by the same change; it does not cite the number) |
| `2127db52` | Alerts and secrets management requests reach the SIEM | line 3051 | present (entry added by the same change; it does not cite the number) |
| `6ce4c756` | Failed sign-ins and FGA denials reach the SIEM next to the audit log entries | line 3082 | present (entry added by the same change; it does not cite the number) |
| `3ae98cb8` | The 1Password secrets backend reads back the bytes it stored, and the Azure Key Vault and GCP Secret Manager backends report a deleted secret as not found | line 2536 | present (entry added by the same change; it does not cite the number) |
| `50ff3f46` | A credential on the HashiCorp Vault secrets backend resolves to the bytes that were stored, and the Vault and AWS Secrets Manager backends are tested against a server | line 2505 | present (entry added by the same change; it does not cite the number) |
| `d77b7ca8` | The CyberArk CCP client encodes and checks its query as CyberArk publishes, authenticates with a client certificate, verifies the server and maps CCP's answers | line 2479 | present (entry added by the same change; it does not cite the number) |
| `dd83b42b` | cmd/server reads the environment only through internal/config, with one rule for empty values in config.Load, and a test pins each variable | line 2447 | present (entry added by the same change; it does not cite the number) |
| `e40f9a4e` | Every failure response the A2A handler builds comes from one mapping, and a test injects each failure on each route | line 3685 | present (entry added by the same change; it does not cite the number) |
| `7b766c7d` | The docs prose check and its tests run git without the repository-local variables a hook exports | none | not needed: CI and repository tooling only |
| `818faf4b` | Every request path the three SDK clients send is checked against the route table, and the detection handlers' @Router lines name the paths they are served on | none | not needed: a test and handler documentation comments |
| `04ad90e7` | The terminal-line sanitizers and their tests write the bidirectional formatting characters as escapes | none | not needed: confined to `sdk/python` and `sdk/typescript`, which keeps its own changelog and release (scope) |
| `412b0f7f` | aim-sdk init refuses a --url with a backslash, whitespace or a control character before it sends the bootstrap token | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `4f5f084a` | The master key that encrypts stored agent private keys can be rotated: a key ring and a startup sweep | line 2765 | present (entry added by the same change; it does not cite the number) |
| `1fb3bf6c` | The quick-start's full output shows the token rotation line as the SDK now prints it | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `bda501ba` | A source build of aim-arp says it is not the published release, and the TypeScript SDK README says init is not in 1.3.1 | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `7dad1b55` | init sends the bootstrap token over http: only to a loopback host, in the TypeScript and Python SDKs | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `151e34ca` | init checks the agent ID, organization ID and status the server answers with, and prints server text as one line | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `e78d9f46` | A release preflight asks the hosted AIM service whether it serves the routes the TypeScript SDK's first-run flows call | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `cab68505` | An ARP analyzer's result names the pattern set and the pattern ids it evaluated, and says when the analyzer did not scan | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `e515a7d0` | The ARP requireLlmConfirmation comment says the rule's action runs at once, and the TypeScript SDK README lists the default rules | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `8dc0d4fa` | ARP enforcement handlers receive the result of each action after it ran | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `72286bd2` | Integration docs state which integrations work today and which are planned | line 2754 | present (entry added by the same change; it does not cite the number) |
| `27320149` | The TypeScript SDK's storeCredential and rotateCredential send the algorithm as encryptionAlg, the member the server reads | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `cdafc363` | The SIEM event of a login, logout or refresh audit row names the client address of the request that wrote it | line 3670 | present (entry added by the same change; it does not cite the number) |
| `d0af0405` | A reused SDK-download token is recorded as the same client or a different one | line 3771 | present (entry added by the same change; it does not cite the number) |
| `90eb5ce7` | Login and logout audit rows record the client address a trusted proxy reports | line 3658 | present (entry added by the same change; it does not cite the number) |
| `1244d07d` | The TypeScript SDK's init checks that the agent's key file can be saved before it sends the single-use bootstrap token | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `73033e3d` | Every TypeScript SDK path that signs with an agent private key decodes it with one function, and a key in any other form is refused with a Fix that converts it | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `956db7da` | The guard-socket classification annotator sends the same allowlisted event data as the L2 prompt | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `d0582066` | A tracked file that starts with a compiled executable's magic bytes fails a lint, whatever its path, extension or mode | none | not needed: CI and repository tooling only |
| `38ab0598` | Docs use plain punctuation; a docs check refuses em dashes, spaced dashes and Why-this headings | none | not needed: documentation punctuation and its check |
| `c339546b` | A logout with any token of an SDK token family ends the family, with or without a revocation store | line 3109 | present (entry added by the same change; it does not cite the number) |
| `1ed9988b` | SDK credential recovery refuses a token family that has ended, decided from the family's own rows | line 3158 | present (entry added by the same change; it does not cite the number) |
| `5e0baa1d` | Creating an API key requires a live sign-in session, as an SDK download, a device sign-in approval and an SDK credential recovery do | line 3148 | present (entry added by the same change; it does not cite the number) |
| `69c7f9b2` | get_trust_score's docstring, the SDK changelog and the trust-scoring page say excludedFactors comes with a stored score as with a calculated one | none | not needed: a docstring, the SDK changelog and documentation |
| `ac0e9d9c` | The changelog and the dashboard's API reference say an MCP server addition is refused for its audit record only when the server writes that record | line 4042 | present (entry added by the same change; it does not cite the number) |
| `403cf07f` | A downloaded SDK's credential ends SDK_TOKEN_FAMILY_MAX_AGE after its download, and a credential-minting route needs a sign-in's access token | line 3311 | present (entry added by the same change; it does not cite the number) |
| `dced63a5` | POST /api/v1/auth/reauth re-authenticates a signed-in user with their password | line 3340 | present (entry added by the same change; it does not cite the number) |
| `6835ca80` | An addition of MCP servers refused because its audit record could not be written answers 503 with its reason and Retry-After, and the Add MCP servers dialog says nothing changed and when to try again | line 4038 | present (entry added by the same change; it does not cite the number) |
| `08b797a7` | sdk/python: aim-sdk init registers a first agent from a bootstrap token, keeping the private key on the machine | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `48e2cb1e` | A reactivation or verification refused because its audit record could not be written answers 503 with its reason and Retry-After, and the agent page says nothing changed and when to try again | line 4038 | present (entry added by the same change; it does not cite the number) |
| `d34e279a` | An approval refused because its audit record could not be written answers 503 with its reason and Retry-After, and the approve dialog says nothing changed and when to try again | line 4038 | present (entry added by the same change; it does not cite the number) |
| `c97c752a` | A delete refused because its audit record could not be written answers 503 with its reason and Retry-After, and the delete dialog says nothing changed and when to try again | line 4107 | present (entry added by the same change; it does not cite the number) |
| `95c85913` | The dev stack takes its host ports and container names from the environment | line 3814 | present (entry added by the same change; it does not cite the number) |
| `f3bb0062` | An MCP server loses consensus verification when its attesting agents stop agreeing | line 3829 | present (entry added by the same change; it does not cite the number) |
| `c36b6d17` | A revoked session's access token is refused on the routes behind the session middleware, not only at the credential-minting routes | line 4238 | present (entry added by the same change; it does not cite the number) |
| `5f512cfa` | Three dashboard sentences and two SDK printed lines are punctuated without an em dash | line 4294 | present (entry added by the same change; it does not cite the number) |
| `f11afc67` | The dashboard writes "MCP servers" in sentence case on every label it shows | line 3733 | present (entry added by the same change; it does not cite the number) |
| `2f24f3e6` | A verification event logged for an unknown or unreadable agent answers the same error as a foreign agent and logs the cause | line 3744 | present (entry added by the same change; it does not cite the number) |
| `feceffd4` | Stopping the action-request nonce purge job waits for the job to exit | line 3941 | present (entry added by the same change; it does not cite the number) |
| `37b72aa6` | The tenant-scoped table list's comments say it is not every tenant-owned table | none | not needed: code comments |
| `e676eaff` | The tenant-scope helpers' comments describe this repository | none | not needed: code comments |
| `af1843b5` | Webhook test deliveries send the event-delivery headers, are recorded as deliveries and log their failures, and a refused webhook URL is a 400 with a fixed reason | line 3921 | present (entry added by the same change; it does not cite the number) |
| `be25b767` | Add the tenant-scope primitive and an opt-in request-pool role, off by default | line 3890 | present (entry added by the same change; it does not cite the number) |
| `a7a50fcf` | A refused A2A card URL is a 400 with a fixed reason, and MCP verify and detect keep the cause in the server log | line 3871 | present (entry added by the same change; it does not cite the number) |
| `2d0d53e2` | The SDK download leaves out the SDK's tests and tooling and states no schedule | line 3857 | present (entry added by the same change; it does not cite the number) |
| `695f025d` | Sign-in failure and lock alerts log no email address | line 3849 | present (entry added by the same change; it does not cite the number) |
| `b15f29bf` | Audit log names the agent as the actor of a record that carries an agent id | line 4029 | present (entry added by the same change; it does not cite the number) |
| `411f76f3` | A revoked session's access token cannot create an API key, take or replace an agent's key, rotate a secret, or create or change a webhook | line 4514 | present (entry added by the same change; it does not cite the number) |
| `928b67fc` | A login access token carries its sign-in time and the API refuses one whose sign-in is past JWT_SESSION_MAX_AGE | line 3947 | present (entry added by the same change; it does not cite the number) |
| `f379eb2f` | The A2A multi-agent demo's Java program follows AIM_BACKEND_PORT and AIM_FRONTEND_PORT | line 3964 | present (entry added by the same change; it does not cite the number) |
| `3f665183` | The A2A multi-agent demo's Python scripts follow AIM_BACKEND_PORT and AIM_FRONTEND_PORT | line 3975 | present (entry added by the same change; it does not cite the number) |
| `88c92ebd` | The LangChain CRUD and MCP server demos follow AIM_BACKEND_PORT and AIM_FRONTEND_PORT | line 3987 | present (entry added by the same change; it does not cite the number) |
| `3bde80d0` | The flight-search demo's helper scripts follow AIM_BACKEND_PORT, AIM_FRONTEND_PORT and AIM_POSTGRES_PORT | line 4002 | present (entry added by the same change; it does not cite the number) |
| `f6f00896` | The flight-search demo's stage scripts reach the stack through AIM_PROJECT and AIM_BACKEND_PORT | line 4016 | present (entry added by the same change; it does not cite the number) |
| `b32de243` | Revoking an SDK token in the dashboard ends its token family, so the family's access token is refused at /sdk/download at once | line 4259 | present (entry added by the same change; it does not cite the number) |
| `47cd55e4` | Dashboard type comes from one six-step ramp, 12px captions and a 14px reading floor, and a test fails on classes off it | line 4275 | present (entry added by the same change; it does not cite the number) |
| `c8147f9f` | The agent Security tab no longer draws the Future Use Cases and Key Management Best Practices blocks | line 4225 | present (entry added by the same change; it does not cite the number) |
| `d2c3a154` | The agent page detects MCP servers from a More menu, with the command to run where the agent runs | line 4211 | present (entry added by the same change; it does not cite the number) |
| `5fd3676c` | GET /health/ready can report the commit a hosted build was synced from | line 4200 | present (entry added by the same change; it does not cite the number) |
| `0b6f08ee` | The key vault says whether the server holds the agent's private key, and its Algorithm row shows Not recorded when no algorithm is stored | line 4189 | present (entry added by the same change; it does not cite the number) |
| `573b7498` | Webhook, MCP server and agent card requests refuse eight more special-purpose address ranges | line 4161 | present (entry added by the same change; it does not cite the number) |
| `22f657b2` | Mail asks the SMTP relay not to track any message | line 4175 | present (entry added by the same change; it does not cite the number) |
| `6b15139d` | The Java SDK download's QUICKSTART.md names the version the download is named after, and backend tests list every file that states another Java SDK version than the pom | line 4345 | present (entry added by the same change; it does not cite the number) |
| `594409a4` | Java SDK: processes sharing the credentials file refresh it once, under its lock | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `acb99185` | A 4xx response states its reason instead of carrying an error's text | line 7070 | present (entry added by the same change; it does not cite the number) |
| `05ebfd9e` | Secrets resolve encrypts the credential only to the agent's registered key and answers a signed request once | line 7109 | present (entry added by the same change; it does not cite the number) |
| `0c9800ee` | .env.quickstart keeps its earlier content; the quick-start port and project variables are named in the compose file header, the README and the release smoke contract | line 4435 | present (entry added by the same change; it does not cite the number) |
| `29d88e7d` | The TypeScript SDK release smoke hands psql the admin email as a variable, and .hmaignore exempts its quoted docker exec calls from DOCKERINJ-001 | none | not needed: SDK release smoke tooling |
| `2181cd08` | GET /health/ready and GET /api/v1/status report the bare semver the image tag carries | line 4403 | present (entry added by the same change; it does not cite the number) |
| `cdebe383` | The security policies page shows the trust score a low-trust policy triggers below | line 6055 | present (entry added by the same change; it does not cite the number) |
| `9e57da12` | The Docker quick-start takes its host ports and container names from the environment, and its test script works in a directory it creates | line 4416 | present (entry added by the same change; it does not cite the number) |
| `16d2366b` | Sign-in responses give lastLoginAt in UTC, as /api/v1/auth/me does | line 4441 | present (entry added by the same change; it does not cite the number) |
| `3a901895` | The TypeScript SDK has a release smoke that calls verifyAction from the packed tarball against a real server | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `6cbe3284` | Java SDK: saves of the credentials file wait for a cross-process lock and replace the file in one step | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `70c4821c` | Turning on hybrid mode takes the manager or admin role, and the TypeScript SDK deprecates createHybridRequestHeaders | line 4538 | present (entry added by the same change; it does not cite the number) |
| `6f4baeb9` | Two dashboard tabs share one token refresh in a browser without Web Locks, and a browser test opens two tabs against the real backend | line 4551 | present (entry added by the same change; it does not cite the number) |
| `e6abab84` | The Java SDK's floating-point census fails on a read it does not list, and the ATX and MCP trust scores are tested with JSON strings | line 7737 | present (entry added by the same change; it does not cite the number) |
| `62006446` | The bootstrap guide, the changelog and the connection string comment name .pgpass in the home directory as the password file the driver reads when PGPASSFILE is unset | line 4641 | present (entry added by the same change; it does not cite the number) |
| `77dff8be` | The agent list query builds its LIMIT and OFFSET clause before the query text, so the required security lint passes | none | not needed: builds the same query in a form the security lint reads |
| `09a42f65` | The Python SDK's update_agent_card sends skills as the list of skill identifiers the server binds | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `47235fac` | The Python SDK's get_security_violations raises for another organization when the answer holds no violation | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `f5df5a9a` | aim-bootstrap reaches a database on the Unix socket directory POSTGRES_HOST names, and reports a connection string the driver refuses without quoting it | line 4711 | present (entry added by the same change; it does not cite the number) |
| `8500bc98` | Each .hmaignore exemption names the one check it suppresses, and the changelog says what the tab bar test reads | line 4793 | present (entry added by the same change; it does not cite the number) |
| `008cb875` | The A2A, supply chain and agent detail pages pass the readability sweep with populated charts | line 4871 | present (entry added by the same change; it does not cite the number) |
| `b30f7003` | Every link and button on the chart screens is at least 24px tall, and the readability sweep gates target size | line 4892 | present (entry added by the same change; it does not cite the number) |
| `1bb36a65` | The active navigation link, the overview's agent initials and the security tiles' units are readable in both themes | line 4907 | present (entry added by the same change; it does not cite the number) |
| `964a9736` | The compliance page fits a phone screen, and every card on it is a glass card with readable text | line 4925 | present (entry added by the same change; it does not cite the number) |
| `307159e5` | The compliance page draws its charts on the glass card, so their axis text is readable in the dark theme | line 4942 | present (entry added by the same change; it does not cite the number) |
| `c2748d81` | The readability sweep measures every chart's marks, text and table control in a browser | line 4955 | present (entry added by the same change; it does not cite the number) |
| `d07eb632` | Every chart offers its values as a table, and the light theme draws four series darker | line 4974 | present (entry added by the same change; it does not cite the number) |
| `e0da18d9` | The overview and security pages draw their charts through the chart layer | line 4994 | present (entry added by the same change; it does not cite the number) |
| `42dd2ea8` | The changelog and the docs link lint say the project has no repository under the opena2a account, where they said that account does not exist | line 5272 | present (entry added by the same change; it does not cite the number) |
| `c5d9c56e` | The changelog says which dashboard responses carry the Content-Security-Policy header, where it said every response does | line 5394 | present (entry added by the same change; it does not cite the number) |
| `9d90cde9` | Every dashboard page is served with a Content-Security-Policy | line 5392 | present (entry added by the same change; it does not cite the number) |
| `34eb724e` | The changelog says a GET, PUT, PATCH or DELETE on /api/v1/a2a/verify without credentials was answered 401, where it said every one was | line 5543 | present (entry added by the same change; it does not cite the number) |
| `16e896d7` | The changelog says both 400 answers of the POST /api/v1/a2a/verify handler carry valid: false, where it said every refusal does | line 5554 | present (entry added by the same change; it does not cite the number) |
| `a06794e7` | An authorized capability check stores its time and no longer rewrites the agent row | line 6446 | present (entry added by the same change; it does not cite the number) |
| `17cbd8d8` | An agent stored before records were written has its history opened by an opening_state record, written with its first change or by a sweep | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `fa867a4a` | With a transition recorder set, a key expiry sweep gives each organization its own trace id | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `4dfb8a14` | With a transition recorder set, an agent update answers an empty talks_to list as [], not null | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `9fc4633f` | record: the debt insert and the settler's read of open debts pass the security lint | none | not needed: internal change for the security lint, record path the server runs only with a transition recorder, which it does not set |
| `50ef30d3` | A trust score stored after a recorded change no longer puts back a status or key another request changed meanwhile | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `dfecc756` | Deleting an agent, generating its server key at signing, and suspending it as compromised can each write an authorization transition record | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `dc9155df` | Adding, replacing or removing an agent's MCP servers can each write an authorization transition record with its talks_to list before and after | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `8e7b1869` | Registrations and pending capability requests each write an authorization transition record, and a request's decision names it as its parent | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `b5a76b4f` | Capability request decisions, verification, key updates and the automatic suspensions can each write an authorization transition record | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `0b51084c` | Grants, revocations, suspensions, reactivations and key rotations can each write an authorization transition record with the agent's state before and after | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `1d73522e` | record: a job run gives each organization its own trace, and an organization finds its records by the trace its callers sent | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `99a41801` | record: every audit record carries a trace_id and a parent reference, and the writer refuses one without them | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `2c4cdc0c` | record: the server reports at start whether audit record chains can start, and a pre-chain writer commits without a record | line 5650 | present (entry added by the same change; it does not cite the number) |
| `20ba01ab` | the compliance report and an MCP server's audit timeline name who acted by the audit record's own rule | line 5665 | present (entry added by the same change; it does not cite the number) |
| `c97d0211` | audit records say who acted, and leave out an address or user agent that was never recorded | line 5682 | present (entry added by the same change; it does not cite the number) |
| `d16b2778` | record: aim-breakglass chain status reads an organization's chain through the same read as the admin route | line 5697 | present (entry added by the same change; it does not cite the number) |
| `3e292bc1` | record: admins can read their organization's chain state, and a refused record write's line says debt=none | line 5714 | present (entry added by the same change; it does not cite the number) |
| `901cddcc` | record: a reduction whose record cannot be appended commits with a debt row, and a settler appends the record later | line 118 | added by this reconciliation |
| `e3a1aedd` | record: a writer appends signed audit records to each organization's chain under a bounded append lock | line 118 | added by this reconciliation |
| `143310e6` | The alert list's repeat count wraps inside its alert card on a phone-width screen | line 5759 | present (entry added by the same change; it does not cite the number) |
| `e68272ed` | The changelog and API reference say only alerts with a dedupe key coalesce, and the capability check sets none yet | line 5752 | present (entry added by the same change; it does not cite the number) |
| `2ddfdefb` | The suspend, reactivate and revoke responses serve an agent's active grants as its capabilities | line 5903 | present (entry added by the same change; it does not cite the number) |
| `47ae57d7` | An agent's trust credential carries its active capability grants, not the list reported at registration | line 5885 | present (entry added by the same change; it does not cite the number) |
| `339ceba2` | Repeated capability violations are counted on one open alert | line 4949 | present (entry added by the same change; it does not cite the number) |
| `9481edd3` | The Java SDK's Maven project URL and README link go to the project page on opena2a.org | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `306f19bd` | README, npm and PyPI homepage links go to the project page on opena2a.org | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` and `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `aaba5931` | A test measures which routes the strict rate limit covers, from the route table the server mounts | none | not needed: a test; the route table it reads is unchanged |
| `89561a93` | Rate limits stay at their production values when ENVIRONMENT is development, and the server logs the limits in force at start | line 6077 | present (entry added by the same change; it does not cite the number) |
| `0a7c4945` | The server keeps one admin seed path, and its logs carry no part of the password | none | not needed: removes an initializer nothing called; the admin seed path is unchanged |
| `823bc145` | A suspended agent's refused signed action-request statement is recorded in the audit log | line 5918 | present (entry added by the same change; it does not cite the number) |
| `0be13a8f` | A suspended agent's refused verification request is recorded in the audit log | line 5935 | present (entry added by the same change; it does not cite the number) |
| `a6070387` | Terraform saved plans and local state are ignored by the repository | line 5953 | present (entry added by the same change; it does not cite the number) |
| `7d5b8bd7` | A password sign-in's audit row no longer copies the account's email and role | line 7014 | present (entry added by the same change; it does not cite the number) |
| `c4761159` | The Python SDK's register_mcp adds the given MCP server to the agent's list and no longer creates an unnamed one | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `00d07037` | The Python SDK adds tags to an existing agent by ID only and reads the tag route's 204 as success | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `7854098b` | Action verification accepts a signed statement with a nonce and a 30-second window | line 7121 | present (entry added by the same change; it does not cite the number) |
| `7f69b9e5` | An issued agent trust credential is returned only with both ATX signature suites and the agent, score and level AIM asked for | line 5963 | present (entry added by the same change; it does not cite the number) |
| `4b8a6f17` | The TypeScript SDK's offline verifier checks ML-DSA-65 signatures and returns the conformance verdict on every ATX fixture | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `b0c8011e` | An issued agent trust credential asks for trust level 2 at most, the highest one signing authority may assert | line 5981 | present (entry added by the same change; it does not cite the number) |
| `6db48764` | The stored-key rows close through defer so the security lint passes | none | not needed: closes rows through defer for the security lint, same behaviour |
| `d5f6447d` | Audit record package: canonical bytes, hash chain, signed records and a verifier that names the first tampered position | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `738688c9` | Delete agent has an end-to-end test from both entry points, and the list keeps its deleted notice in a development build | line 6404 | present (entry added by the same change; it does not cite the number) |
| `b39e0245` | Delete agent shows one dialog that says what it removes and keeps, and states a failure inside it | line 6393 | present (entry added by the same change; it does not cite the number) |
| `5c9eb184` | A stored agent private key decrypts only in the agent row it was written for | line 6005 | present (entry added by the same change; it does not cite the number) |
| `56911aa0` | Tracked files are checked for token-shaped literals, with a planted control the run must catch | line 6030 | present (entry added by the same change; it does not cite the number) |
| `6a6bc6c7` | aim-server migrate applies the migrations and can provision a role that is not a superuser | line 6993 | present (entry added by the same change; it does not cite the number) |
| `4f4c6cca` | Every sign-in records the token family it issued | line 7023 | present (entry added by the same change; it does not cite the number) |
| `d788d4c8` | Trust scoring counts only verification outcomes the server observed | line 7035 | present (entry added by the same change; it does not cite the number) |
| `481f494f` | A revoked agent can no longer be reactivated, verified or suspended | line 7051 | present (entry added by the same change; it does not cite the number) |
| `3385cce4` | A 5xx response carries a fixed line instead of the server's error text | line 7095 | present (entry added by the same change; it does not cite the number) |
| `4c9b04bb` | The Python SDK's PQC key calls send the members the server reads | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `85c1d7d6` | Expired A2A request nonces are deleted every minute without an admin call | line 7147 | present (entry added by the same change; it does not cite the number) |
| `c59f5704` | The five-minute cleanup job clears expired password reset tokens | line 7156 | present (entry added by the same change; it does not cite the number) |
| `ea038b0d` | A third party can verify an agent card attestation from the served card and the JWK Set | line 7164 | present (entry added by the same change; it does not cite the number) |
| `90c39718` | Card attestations and ATC trust each sign with their own key | line 7181 | present (entry added by the same change; it does not cite the number) |
| `bb133d34` | An authenticated request without an organization or user answers 401, not 500 | line 7275 | present (entry added by the same change; it does not cite the number) |
| `cc2bc3e3` | The logo SVG's embedded PNG is split into one more IDAT chunk | line 6047 | present (entry added by the same change; it does not cite the number) |
| `4b107dde` | Request records show how each caller authenticated and whether an agent used its own ID on the held agent routes | line 6283 | present (entry added by the same change; it does not cite the number) |
| `830763ed` | An agent credential acts only on its own agent ID on the SDK-API, detection and A2A write routes | line 6260 | present (entry added by the same change; it does not cite the number) |
| `1861e8ab` | README Quick start runs as written: self-hosted login, an agent that holds db:read, one allowed and one refused call, and a dev stack command that names real services | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `f207cf87` | backend: agent-signature authentication reads an agent's status only after the signature verifies, and answers an unknown agent, a missing key and a wrong key alike | line 6292 | present (entry added by the same change; it does not cite the number) |
| `2491816c` | A reused SDK-download token ends the chain that grew from it | line 6097 | present (entry added by the same change; it does not cite the number) |
| `68ada883` | /metrics leaves the API port for a loopback listener, needs a token off loopback, and drops per-agent labels | line 6159 | present (entry added by the same change; it does not cite the number) |
| `d4caf559` | Python SDK creates the agent key directory 0700 and each agent key file 0600 at creation | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `488d383e` | Python and Java SDKs report capabilities through the registration route and raise when one fails | line 6171 | present (entry added by the same change; it does not cite the number) |
| `cc3c0cd4` | The agent page's refused-call finding spans the activity row, so it is readable on a phone | line 6342 | present (entry added by the same change; it does not cite the number) |
| `f7659f9b` | The TypeScript SDK obtains a token with the 64-byte private key AIM issues | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `937e1619` | aim-sdk login ends at once on an unavailable AIM server, exits 75, and no longer reports the outage as a failed login | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `100a35f2` | The quickstart test builds the backend Dockerfile the published image is built from, and the alpine:latest copy is removed | line 6229 | present (entry added by the same change; it does not cite the number) |
| `54968330` | The TypeScript SDK requests the token grant the server accepts, and expires_in reports the token's real lifetime | line 6241 | present (entry added by the same change; it does not cite the number) |
| `a59e1e6f` | An issued agent trust credential carries publisherDid, buildAttestation and a 0-100 trustScore | line 7339 | present (entry added by the same change; it does not cite the number) |
| `bda55c79` | Java SDK comments say ML-DSA-65 signatures are verified, not just recorded | none | not needed: confined to `sdk/java`, which keeps its own changelog and release (scope) |
| `fb7624c1` | Refusals that carry a machine-readable reason also send it as reasonCode | line 7204 | present (entry added by the same change; it does not cite the number) |
| `f1ba800a` | A verification event refused because its agent could not be read logs the cause | line 7214 | present (entry added by the same change; it does not cite the number) |
| `aa0ed126` | A deleted webhook's name can be used again | line 7227 | present (entry added by the same change; it does not cite the number) |
| `29bc06c5` | Alert detail panel shows a user-account alert as an account, not an unknown agent | line 7235 | present (entry added by the same change; it does not cite the number) |
| `737e1566` | SDK token list returns token metadata with camelCase keys | line 7247 | present (entry added by the same change; it does not cite the number) |
| `e3a1f78b` | CLI device login polls every 10s so it fits the production rate limit | line 7255 | present (entry added by the same change; it does not cite the number) |
| `6b506700` | Server shutdown is bounded at 10s and a failed shutdown no longer skips the drain and cleanup | line 7265 | present (entry added by the same change; it does not cite the number) |
| `36aba15f` | TypeScript SDK: LocalVerifier accepts the raw credential text and rejects duplicate members as MALFORMED | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `443f631e` | /.well-known/aip identifies the provider as did:web:<provider-host> | line 7290 | present (entry added by the same change; it does not cite the number) |
| `87db6fe2` | /health/ready reports a configured Redis that failed at startup as unavailable, not notConfigured | line 7303 | present (entry added by the same change; it does not cite the number) |
| `d23d91e9` | A restart keeps a changed admin password; ADMIN_PASSWORD only seeds the first administrator | line 7311 | present (entry added by the same change; it does not cite the number) |
| `11ec6923` | Python SDK: processes sharing the credentials file refresh it once, under a lock | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `6e6b873f` | aim-sdk status --json with no stored credentials reports credentialsPath as null | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `2b5fba3e` | Dashboard route sweep: the readability instrument and its 2026-10-05 run | none | not needed: readability measurement tooling and its run |
| `e2893042` | record: audit record and checkpoint envelopes are signed and checked per payload class | none | not needed: record path the server runs only with a transition recorder, which it does not set |
| `d584fd62` | Java SDK: useMcpTool posts its usage report to the route the API registers | line 6311 | present (entry added by the same change; it does not cite the number) |
| `8c86a658` | The backend configuration drops its unread 24h and 7d token lifetime defaults | line 6323 | present (entry added by the same change; it does not cite the number) |
| `15c03cb0` | The users table drops its unread password_reset_expires column and index | line 6346 | present (entry added by the same change; it does not cite the number) |
| `b08fbdd4` | The agent page shows why a call was refused and where to grant it | line 6331 | present (entry added by the same change; it does not cite the number) |
| `a4d8e7fd` | A webhook delivery names the event it carries in X-Webhook-Event, not the webhook's first subscribed event | line 6526 | present (entry added by the same change; it does not cite the number) |
| `777214ff` | POST /api/v1/public/agents/register answers 401 to an agent's OAuth access token, which public routes no longer read as a signed-in user | line 6584 | present (entry added by the same change; it does not cite the number) |
| `afb304d8` | With AIM_BASE_URL set, the token endpoint accepts only an assertion addressed to AIM_BASE_URL, whatever host the request names | line 6565 | present (entry added by the same change; it does not cite the number) |
| `1f474c81` | TypeScript SDK: createAgentRequestHeaders refuses a URL with an empty query, which Node 20's fetch sends without its ? | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `57f748dd` | quickstart.sh stops before a first run over an earlier install's database volume | line 6364 | present (entry added by the same change; it does not cite the number) |
| `ee9f5dfa` | backend: a request-analytics row that fails to insert is counted by error class instead of dropped without a trace | line 6375 | present (entry added by the same change; it does not cite the number) |
| `c60a851d` | TypeScript SDK: createAgentRequestHeaders signs a request with the Ed25519 agent-request-v1 signature AIM verifies | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `ec862f04` | A failed agent delete no longer answers with the database's error text | line 6388 | present (entry added by the same change; it does not cite the number) |
| `dbe96179` | Deleting an agent that took part in A2A tasks, messages or consent records no longer fails | line 6379 | present (entry added by the same change; it does not cite the number) |
| `22c623da` | An MCP server is verified only when its attesting agents connected and report the same tool set | line 6416 | present (entry added by the same change; it does not cite the number) |
| `438e3342` | Agent heartbeat stores the heartbeat time and no longer rewrites the agent row | line 6436 | present (entry added by the same change; it does not cite the number) |
| `5b804e88` | The backend image ships license notices, and MPL-2.0 source, for the Go libraries built into it | line 6457 | present (entry added by the same change; it does not cite the number) |
| `919d955c` | Password sign-in records only the sign-in time and no longer rewrites the user row | line 6478 | present (entry added by the same change; it does not cite the number) |
| `07b859e9` | API refuses to start without KEYVAULT_MASTER_KEY unless ENVIRONMENT is development or test | line 6489 | present (entry added by the same change; it does not cite the number) |
| `f6970d28` | Webhook, MCP server and agent card requests follow no redirect and connect only to addresses registration admits | line 6504 | present (entry added by the same change; it does not cite the number) |
| `e875883c` | An agent's activity returns no network address, user agent or personal metadata to a non-admin | line 6535 | present (entry added by the same change; it does not cite the number) |
| `9325f5a3` | The per-agent and per-MCP-server audit logs return no network address, user agent or metadata to a non-admin | line 6550 | present (entry added by the same change; it does not cite the number) |
| `8db99ae9` | The token endpoint accepts an assertion addressed to the server's address and port when AIM_BASE_URL is unset | line 6211 | present (entry added by the same change; it does not cite the number) |
| `73d89cd4` | POST /api/v1/public/agents/register answers 401 without a user access token, and the API reference says it needs one | line 6581 | present (entry added by the same change; it does not cite the number) |
| `787b9a4d` | The Python SDK refuses a secrets namespace or operation that contains a line break or \| before signing | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `78df8bd4` | The Java SDK signs requests in the form the platform verifies, and signs its approval poll | line 6597 | present (entry added by the same change; it does not cite the number) |
| `3a3f4b9e` | MCPIntegration sends the client's access token, and attestServer refuses instead of sending an unsigned attestation | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `e8389676` | The dashboard asks for the signed-in user once per page load instead of once per component | line 6356 | present (entry added by the same change; it does not cite the number) |
| `1946e85f` | No tracked file carries a home directory path from the machine that built or wrote it | line 6614 | present (entry added by the same change; it does not cite the number) |
| `197d714b` | The agent page shows verify, suspend, reactivate and delete outcomes beside the actions instead of in browser alerts | line 6635 | present (entry added by the same change; it does not cite the number) |
| `8feda7fe` | The API reference names the password reset fields the endpoint reads | line 6649 | present (entry added by the same change; it does not cite the number) |
| `a45fea26` | MCP server routes answer 404 for another organization's server, the same as for an unknown ID | line 6669 | present (entry added by the same change; it does not cite the number) |
| `19dede17` | HTTP request metrics record the status the client received, and unknown paths share one series | line 6682 | present (entry added by the same change; it does not cite the number) |
| `79ac5bab` | Backend scripts no longer approve registrations or sign admin tokens outside the service | line 6695 | present (entry added by the same change; it does not cite the number) |
| `f22b29c3` | An empty talks_to list allows no MCP server in the agent-signed action check | line 6707 | present (entry added by the same change; it does not cite the number) |
| `691bbbcc` | An SDK refresh returns a new token only when its row was committed | line 6717 | present (entry added by the same change; it does not cite the number) |
| `6a338c2c` | An SDK refresh returns the presented token unchanged when its row cannot be revoked | line 6736 | present (entry added by the same change; it does not cite the number) |
| `c37cf2b5` | An admin or manager can turn off hybrid mode from the agent page | line 6761 | present (entry added by the same change; it does not cite the number) |
| `39f75db8` | The Java SDK refuses a JSON string where it reads a trust or confidence number | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `8fb332a2` | The server refuses a master key of 32 zero bytes | line 6771 | present (entry added by the same change; it does not cite the number) |
| `ef1f7089` | The Java SDK refuses to turn on hybrid mode and deprecates createHybridRequestHeaders | SDK changelog | present: recorded in `sdk/java/CHANGELOG.md` (the SDK's own release) |
| `d6dede9a` | The source tree no longer carries prebuilt server binaries | line 6780 | present (entry added by the same change; it does not cite the number) |
| `f2d098cb` | Password reset tokens are stored as a SHA-256 digest | line 6796 | present (entry added by the same change; it does not cite the number) |
| `1e6d2d9c` | Deployment and installation guides name KEYVAULT_MASTER_KEY as required outside development | line 6805 | present (entry added by the same change; it does not cite the number) |
| `66362c17` | The dashboard lists an organization's audit records, each with its actor | line 6819 | present (entry added by the same change; it does not cite the number) |
| `c81c48ec` | Docs sample output shows a placeholder account, and a lint keeps real addresses out of docs | line 6831 | present (entry added by the same change; it does not cite the number) |
| `b5adb616` | The source tree no longer carries a prebuilt macOS server binary | line 6780 | present (matched by content) |
| `b8ac3020` | The developer compose stack passes DEFAULT_ADMIN_PASSWORD only to a one-shot bootstrap run | line 6839 | present (entry added by the same change; it does not cite the number) |
| `4bebdf7d` | An approved device code is exchanged for one token pair | line 6854 | present (entry added by the same change; it does not cite the number) |
| `16820cd2` | A wrong password at sign-in is shown once in the sign-in card, not as an expired session | line 6865 | present (entry added by the same change; it does not cite the number) |
| `8eaa50c6` | aim-bootstrap prints the admin password only when it generated it | line 6879 | present (entry added by the same change; it does not cite the number) |
| `fd4b5c3d` | The server refuses to start with the JWT_SECRET placeholder from .env.example | line 6889 | present (entry added by the same change; it does not cite the number) |
| `d6d6b38c` | web: the first-run screen reports its view and tab switches, and platform admins see time to first agent in a panel | line 48 | added by this reconciliation |
| `a1d5fd14` | backend: AIM records onboarding steps per organization with no user data and reports time to first agent to platform admins | line 48 | added by this reconciliation |
| `fb04a839` | TypeScript SDK: npx @opena2a/aim-sdk init registers a first agent with a bootstrap token and saves its locally generated key to ~/.aim/agents | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `1aa4f3a2` | backend: a dashboard user can mint a 15-minute single-use bootstrap token that registers one agent in their organization | line 34 | added by this reconciliation |
| `6fc4318f` | Phone menus open above the bottom tab bar: the drawer ends above it and its account section is fully visible | line 86 | added by this reconciliation |
| `a01b46de` | Release: no job that runs for a tag or a publish uses a cache another run wrote, held by scripts/lint-release-path-cache.py | none | not needed: CI and repository tooling only |
| `6d07efe6` | The TypeScript SDK's A2AClient task history and skill list reach routes AIM registers, and revokeAttestation is deprecated | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `6e403531` | The server refuses a JWT presented as an agent ATC and accepts CBOR ATCs only | line 6974 | present (entry added by the same change; it does not cite the number) |
| `c8eb264a` | The flight-search example refuses a search AIM did not verify, and reports a verified search's execution | line 98 | added by this reconciliation |
| `e2d457ff` | Logging out an SDK-download token revokes its row in the SDK token list | line 6985 | present (entry added by the same change; it does not cite the number) |
| `c7bd84b3` | The backend image stamps the version and commit /health/ready reports | line 7359 | present (entry added by the same change; it does not cite the number) |
| `65f93c52` | Refused signed agent requests are counted by reason and SDK and logged once per 64-second period | line 7370 | present (entry added by the same change; it does not cite the number) |
| `e82223ef` | Two dashboard tabs no longer sign each other out when the access token expires | line 7397 | present (entry added by the same change; it does not cite the number) |
| `8bee1545` | MCP servers registered before the scale migration get a calculated trust score at first start | line 7412 | present (entry added by the same change; it does not cite the number) |
| `7377d52c` | domain: SUPER_PRIVILEGED actions are authorized only by a verified owner signature, never by a message | none | not needed: adds a domain check that no route calls at main |
| `5e72869b` | Dashboard charts render through one chart layer that takes its colours from theme tokens | line 7386 | present (entry added by the same change; it does not cite the number) |
| `398fb2f2` | A trust-score policy is evaluated at the threshold its seed sets | line 7325 | present (entry added by the same change; it does not cite the number) |
| `f0095e00` | examples/mcp-server-demo: the README documents the routes, port and registration URL the server serves | none | not needed: example documentation |
| `db2b7cb9` | An issued agent trust credential carries a contentHash that matches the ATX schema | line 7352 | present (entry added by the same change; it does not cite the number) |
| `072409f4` | Dashboard Java tab says the SDK is not on Maven Central; SDK tab names are pinned to each SDK's manifest | line 65 | added by this reconciliation |
| `c17967ea` | Sign-up, reset and device sign-in text reads at 14px or larger, and Show password is a 44px tap target | line 72 | added by this reconciliation |
| `44f25520` | Dashboard: a verified agent shows a Verified status badge, not a disabled button | line 80 | added by this reconciliation |
| `099d8272` | Dashboard: a newly registered agent's page shows its first-run steps, the old success URL redirects | line 6926 | present (matched by content) |
| `b2db8924` | Dashboard: API keys and SDK tokens share one Credentials page, the old URLs redirect | line 6914 | present (matched by content) |
| `31d1fc1e` | Dashboard: Compliance moves to /dashboard/compliance, the old URL redirects | line 6914 | present (matched by content) |
| `65af5360` | Automated code review: a pull request over the review budget is reviewed in batches or as a diff-only request, not left INCONCLUSIVE | none | not needed: CI and repository tooling only |
| `9fb417ad` | Published backend image reports the commit it was built from on /health/ready | line 7359 | present (matched by content) |
| `dda28577` | Release: SBOMs are generated in a read-only job with version-pinned tools, and only the attach job can write | none | not needed: CI and repository tooling only |
| `9c36c2fb` | Backend builds on Go 1.26.8, declared once by go.mod's toolchain directive and held by scripts/check-go-toolchain.sh | line 107 | added by this reconciliation |
| `73866974` | Publish builds and backend Dockerfiles use only digest-pinned images, held by scripts/lint-runtime-image-pins.py | line 112 | added by this reconciliation |
| `2a555dfb` | Workflows: every action reference is pinned to a commit sha, held by scripts/lint-action-pins.py | none | not needed: CI and repository tooling only |
| `4c4ebe95` | Dashboard: moved pages redirect from one table in lib/redirects.ts | none | not needed: moves the same redirects into one table |
| `bd325b0b` | check-public-wording.py: a check fails when ruled-out audit and evidence wording appears in the public documents | none | not needed: CI and repository tooling only |
| `f24be747` | Resolve js-yaml 4.3.2 in the TypeScript SDK lockfile (advisory id omitted) | line 23 | added by this reconciliation |
| `e1d1b3a2` | Cut aim-sdk 2.0.2 | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `41a0fcc9` | Declare the license in PyPI metadata and ship the canonical Apache-2.0 text | none | not needed: SDK package metadata and the license text |
| `4fb2d5c6` | The auto-instrumentation hooks record the model call they were installed to record | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| `22e08bee` | PolicyCache.check() raises until a policy document is loaded, and fails closed once one is | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| `d0a80150` | Declare the license in PyPI metadata | none | not needed: confined to `sdk/python`, which keeps its own changelog and release (scope) |
| `02361e5f` | Derive the docker paths filter from the Dockerfiles instead of beside them | none | not needed: CI and repository tooling only |
| `eed6198f` | Make the SDK report the version npm actually published | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `9c5482a9` | Stop reporting a trust score nobody measured, and drop emoji from output | SDK changelog | present: recorded in `sdk/python/CHANGELOG.md` (the SDK's own release) |
| `211fe742` | Honor OPENA2A_TELEMETRY, the opt-out the privacy policy actually documents | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `9bbc7963` | Scope the opt-out claim to the channels it reaches, and pin the whole channel set | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `fa504345` | Make the fleet-gradient channel honor the master opt-out, and stop citing internal paths from a public file | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `e79e9ef6` | Make structural signature telemetry opt-in instead of default-on | SDK changelog | present: recorded in `sdk/typescript/CHANGELOG.md` (the SDK's own release) |
| `10686228` | sdk-ts: fix localhost hint for empty AIM_BASE_URL; align harness port labels | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `556715f6` | sdk-ts: v1.0.1, bump version + README snippet clarity | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `7df4d789` | sdk-ts: batch 1.0.x deferred fixes + enable ARP live-fire harnesses | none | not needed: confined to `sdk/typescript`, which keeps its own changelog and release (scope) |
| `53be1f72` | Harden otel-demo smoke script: reach Postgres over published port | none | not needed: demo smoke tooling |
| `1a41bd2f` | Wire the 9-factor agent trust score to real data sources | line 7940 | present (matched by content) |
