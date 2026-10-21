-- Migration 141: privileged access management records belong to an organization.
--
-- The /api/v1/pam routes read and write emergency declarations and
-- certification campaigns of the caller's organization only, so each row names
-- its organization. A row written before this migration names none and is read
-- by no route. A pending approval belongs to the organization of its agent; it
-- now also names the user who requested it, whom the routes refuse as its
-- approver.

ALTER TABLE emergency_declarations
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE certification_campaigns
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE pending_approvals
    ADD COLUMN IF NOT EXISTS requested_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_emergency_declarations_organization
    ON emergency_declarations (organization_id, declared_at DESC);

CREATE INDEX IF NOT EXISTS idx_certification_campaigns_organization
    ON certification_campaigns (organization_id, starts_at DESC);
