-- Migration 115: onboarding bootstrap tokens
--
-- A bootstrap token lets a signed-in dashboard user hand a one-line install
-- command to a terminal: the SDK exchanges the token for one agent
-- registration in the user's organization. The token is single use, expires
-- 15 minutes after it is minted, and carries one scope, agents:register.
--
-- Only a SHA-256 hash of the token is stored. display_prefix holds the first
-- 8 characters of the token's secret part so the dashboard can tell tokens
-- apart without holding the secret. The plaintext is returned once, by the
-- mint call, and never again.
--
-- At most one unused, unrevoked token exists per (organization, user): the
-- partial unique index backs the rule that minting a new token revokes the
-- caller's previous one, including under concurrent mints.

CREATE TABLE IF NOT EXISTS bootstrap_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    display_prefix VARCHAR(8) NOT NULL,
    scope VARCHAR(64) NOT NULL DEFAULT 'agents:register' CHECK (scope = 'agents:register'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    CONSTRAINT bootstrap_tokens_expiry_after_creation CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bootstrap_tokens_token_hash ON bootstrap_tokens(token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bootstrap_tokens_one_open_per_user
    ON bootstrap_tokens(organization_id, created_by)
    WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_bootstrap_tokens_organization_id ON bootstrap_tokens(organization_id);
