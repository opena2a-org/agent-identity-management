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

# Set branch protection on main
gh api -X PUT "repos/$REPO/branches/main/protection" \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Secret Detection",
      "Dependency Audit"
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
echo "  - Required checks: Secret Detection, Dependency Audit"
echo "  - Strict status checks (branch must be up-to-date)"
echo "  - Linear history enforced (squash merge)"
echo "  - Force push and branch deletion blocked"
echo "  - Admins included (no bypass)"
echo ""
echo "Done. To add more required checks later:"
echo "  gh api -X PATCH repos/$REPO/branches/main/protection/required_status_checks \\"
echo "    -f 'contexts[]=Secret Detection' -f 'contexts[]=Dependency Audit' -f 'contexts[]=Backend (Go)'"
