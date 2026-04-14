# Brief: Pin Azure Container Apps deploys to digest + unique revision suffix

## Problem

`deploy-azure.yml` uses `azure/container-apps-deploy-action@v2` with
`imageToDeploy: ghcr.io/opena2a-org/aim-server:latest`. Azure Container Apps
treats the image reference as equal across deploys when the tag string is the
same — so pushing a new `:latest` does not guarantee the container app pulls
the new digest. The ARM revision is created, but it can re-use the cached
manifest.

**Incident:** 2026-04-14, 15 minutes lost on a PR-A deploy that the workflow
reported as "success" while the container kept running the previous revision.
Discovered only by inspecting `revisions[].createdTime` vs workflow run time.
See memory `feedback_azure_image_tag_deploy_silent_noop.md`.

## Decision

**[CHIEF-CA] DECISION:** Pin every deploy to a sha256 digest resolved at
deploy time, and assign each revision a unique suffix derived from
`github.sha` + `github.run_number`.

**RATIONALE:** Digest references are content-addressed — Azure cannot cache
a stale image when the target digest changes. A unique revision suffix makes
every deploy observable (`az containerapp revision list` shows a new row)
and rollback-able.

**ALTERNATIVES REJECTED:**
- Keep the deploy-action, add a follow-up `az containerapp revision restart`
  — treats the symptom, not the cause; still cache-prone on the first update.
- Emit the digest as an artifact from `docker-publish.yml` and consume it
  — works but couples two workflows via artifact plumbing. Resolving the
  digest at deploy time from the tag is simpler and self-contained.
- Deploy by SHA tag (`:<long-sha>`) via `docker/metadata-action type=sha` —
  tag-addressed, not content-addressed. Tag still mutable in principle; and
  the deploy workflow would need the source SHA from the completed
  Docker Publish run, which is not trivial across workflow_run triggers.

**ESCALATION:** none. This follows the documented CSR threat guidance and
does not change the deploy surface, credentials, or trust model.

## Design

Replace the two `azure/container-apps-deploy-action@v2` steps with raw
`az containerapp update` calls, preceded by a digest-resolution step:

1. **GHCR login** — `docker/login-action@v3` with `${{ github.token }}` so
   `docker buildx imagetools inspect` can read the `:latest` manifest.
2. **Resolve digests** — for each of `aim-server` and `aim-dashboard`,
   `docker buildx imagetools inspect <image>:<tag> --format '{{.Manifest.Digest}}'`
   and write the result to `$GITHUB_OUTPUT`.
3. **Deploy backend** — `az containerapp update --image <image>@<digest>
   --revision-suffix sha-<7-char-sha>-<run-number>`.
4. **Deploy frontend** — same.

Suffix format: `sha-${SHORT_SHA}-${GITHUB_RUN_NUMBER}`. Lowercase hex +
numeric; satisfies the revision-suffix charset (`[a-z0-9]([-a-z0-9]*[a-z0-9])?`).
The run number makes manual redeploys of the same SHA unique.

## What stays the same

- Trigger: `workflow_run` on successful Docker Publish.
- Azure Login step, Wait-for-health, Verify deployment, Summary — unchanged.
- `workflow_dispatch` inputs still accept `latest` or a specific tag; the
  resolve step handles either.

## Test plan

- Dry-run: push to a throwaway branch with a trivial backend change, verify
  workflow passes and backend revision has the new suffix + new digest.
- Confirm `az containerapp revision list -n aim-prod-backend -g aim-production-rg`
  shows the new revision as Active within ~60s of workflow completion.
- Confirm `curl https://aim.oa2a.org/health` still returns 200.

## Out of scope

- Digest verification via cosign (separate hardening, tracked as follow-up).
- Extending the pattern to `ca-aim-backend-prod` (different product, different
  session owns that repo).
