#!/usr/bin/env bash
# Configure branch protection rules for the AIM repository.
# Requires: gh CLI authenticated with admin access.
#
# Strategy:
#   - main is protected: PRs required, reviews required, status checks enforced
#   - Feature branches auto-deleted after merge
#   - No direct pushes to main (not even admins)

set -euo pipefail

REPO="opena2a-org/agent-identity-management"

echo "Configuring branch protection for $REPO..."

# Enable auto-delete of head branches after PR merge
gh api -X PATCH "repos/$REPO" \
  -f delete_branch_on_merge=true \
  --silent

echo "✓ Auto-delete head branches after merge: enabled"

# Set branch protection on main.
#
# This PUT replaces the entire protection object — the checks list below is
# the single source of truth. If a required check is added via the API and
# not mirrored here, re-running this script silently removes it.
#
# Checks are app-pinned (app_id 15368 = GitHub Actions) rather than
# name-matched: with the deprecated bare-contexts form, ANY app or commit
# status posting the same context name would satisfy the requirement.
#
# "CI Gate" is the fan-in job in ci.yml that fails closed unless every CI
# job (backend tests + coverage, frontend, e2e, both repo lints,
# change-detection) succeeded or was legitimately path-skipped.
gh api -X PUT "repos/$REPO/branches/main/protection" \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "checks": [
      { "context": "Secret Detection", "app_id": 15368 },
      { "context": "Dependency Audit", "app_id": 15368 },
      { "context": "Claude Code Review", "app_id": 15368 },
      { "context": "Go Lint (security)", "app_id": 15368 },
      { "context": "CI Gate", "app_id": 15368 }
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true,
    "require_last_push_approval": true
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF

echo "✓ Branch protection on main:"
echo "  - PRs required (no direct push)"
echo "  - 1 approving review required"
echo "  - Stale reviews dismissed on new push"
echo "  - Last pusher cannot self-approve"
echo "  - Required checks: Secret Detection, Dependency Audit, Claude Code Review, Go Lint (security), CI Gate"
echo "  - Strict status checks (branch must be up-to-date)"
echo "  - Linear history enforced (squash merge)"
echo "  - Force push and branch deletion blocked"
echo "  - Admins included (no bypass)"
echo ""
echo "Done. To add a required check later: add it to the checks array in this"
echo "script (app-pinned, never the deprecated bare contexts[] form) and re-run."
