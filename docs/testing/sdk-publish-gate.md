# TypeScript SDK publish gate: zero-skip refusal and its exclusions

The npm publish job (`.github/workflows/release.yml`, `publish-npm`) and the
per-PR SDK test job (`.github/workflows/ci.yml`, `sdk-tests`) run the same
gate: `npx vitest run` writes a JSON report, and the committed checker
`sdk/typescript/scripts/check-vitest-report.mjs` judges it.

The checker walks **every** `testResults[].assertionResults[].status` in the
report and refuses (non-zero exit, which fails the publish or the PR gate)
when anything failed, was skipped, pending or todo, or when nothing ran at
all. It prints the per-status counts (`passed/failed/skipped/pending/todo`)
before its verdict, so every gate log carries them. It deliberately ignores
the report's aggregate counters: the vitest JSON reporter counts a
`skipIf`-skipped case in `numPassedTests` and leaves
`numPendingTests`/`numTodoTests` at 0, so an aggregate-only check reports
"0 skipped" while the default reporter shows dozens of skips — measured on
the 1.3.1 release commit as 1158 passed / 36 skipped in the terminal against
1194 total / 1194 passed / 0 pending in the JSON aggregate. A workflow-shape
test (`sdk/typescript/tests/check-vitest-report.test.ts`) pins both
workflows to the checker and refuses any return of an aggregate expression.

## Exclusion statement (release contract)

The gate cannot execute every test in the suite on a hosted runner. The
exclusions are explicit, file-scoped, and named in the workflow step as
`--allow-skips=` flags; the checker tolerates status `skipped` **only**
inside the named files. This section states exactly which cases those are
and why.

### Excluded: the 36 environment-gated integration cases

| File | Cases |
|---|---|
| `sdk/typescript/src/a2a/A2AClient.integration.test.ts` | 21 |
| `sdk/typescript/src/client/AIMClient.integration.test.ts` | 13 |
| `sdk/typescript/src/auth/oauth.integration.test.ts` | 2 |

Every case is an `it.skipIf(<predicate>)` case gated on one or both of two
runner probes: `backendAvailable` (a live AIM backend answering at
`AIM_BASE_URL`, default `http://localhost:8080`) and `credentialsAvailable`
(API credentials found in the environment or on disk). Not every case
gates on both. The population splits into two groups by predicate form:

| File | Both predicates | One predicate | Total |
|---|---|---|---|
| `sdk/typescript/src/a2a/A2AClient.integration.test.ts` | 19 | 2 (`!backendAvailable`) | 21 |
| `sdk/typescript/src/client/AIMClient.integration.test.ts` | 10 | 3 (2 on `!backendAvailable` and 1 on `!credentialsAvailable`) | 13 |
| `sdk/typescript/src/auth/oauth.integration.test.ts` | 0 | 2 (`!backendAvailable`) | 2 |
| Total | 29 | 7 | 36 |

A both-predicate case is `it.skipIf(!backendAvailable || !credentialsAvailable)`
and skips when either the backend or the credentials are missing. A
single-predicate case skips when only that one condition is missing on
the runner: the six `it.skipIf(!backendAvailable)` cases execute against a
reachable backend even without credentials, and the one
`it.skipIf(!credentialsAvailable)` case executes with credentials configured
even without a backend. The hosted runner provides neither probe: there is
no backend service in the publish or sdk-tests jobs, and provisioning the
full platform (backend, database, seeded credentials) inside them is out
of scope for this gate. These 36 cases are therefore **excluded from the
publish gate** via the workflow's allowlist. A committed test
(`sdk/typescript/tests/publish-gate-doc-skipif-groups.test.ts`) derives
both groups from the three files and refuses a drift between the table
above and what the files carry.

Compensating controls:

- The exclusion is bounded and reviewable: the allowlist lives in the
  workflow step itself and in committed test assertions that pin it to
  exactly these three files. Growing it requires a workflow diff and a test
  change. A skip in **any other file** — including a new `.skip`, `.todo`
  or an environment probe added elsewhere — still fails the publish.
- The excluded population stays visible: the checker prints the skipped
  count in every gate log, so the log line (`skipped=36` today) shows any
  growth inside the allowlisted files.
- The cases remain runnable on demand: point `AIM_BASE_URL` at a reachable
  backend (for example the docker-compose quickstart stack) with
  credentials configured and run `npx vitest run` in `sdk/typescript`; the
  same 36 cases then execute instead of skipping.

### Provided (not excluded): the curl-gated signature block

`sdk/typescript/src/arp/telemetry/signature/manual-curl.test.ts` gates one
describe block on `describe.skipIf(!hasCurl())`. The gate's runners
(`ubuntu-latest`) ship curl, so **the dependency is provided and the block
executes in both gates** — it is not on the allowlist. On any runner
without curl the checker refuses (a skip outside the allowlist) rather than
silently passing, which is the intended fail-closed behavior.
