-- Migration 127: coalesce repeated alerts that carry the same dedupe key.
--
-- The capability check can raise a capability violation alert on every request
-- for a capability the agent was not granted, and the verification endpoint
-- accepts 100 requests a minute per principal. An agent retrying one such
-- action can therefore write up to 100 identical alerts a minute, and each one
-- would be an alert.created webhook delivery.
--
-- A producer that sets dedupe_key opts into coalescing: while an alert with the
-- same organization and dedupe_key is unacknowledged and was created inside the
-- coalescing window, a repeat increments occurrence_count and moves
-- last_seen_at instead of inserting a row, and no webhook fires. Acknowledging
-- the alert ends the coalescing, so the next repeat is a new alert. The
-- capability check does not set dedupe_key on the alerts it creates yet, so its
-- alerts are not coalesced.
--
-- Existing rows keep dedupe_key NULL and never coalesce. occurrence_count is 1
-- for every row that was inserted, which is what each existing row recorded.
--
-- This file was first committed as 126_coalesce_repeated_alerts.sql, beside
-- 126_remove_row_of_renumbered_drop_password_reset_expires.sql. Both runners
-- key a migration by its full file name, so a database migrated from that tree
-- applies this file once more, where every statement is IF NOT EXISTS and
-- changes nothing; the last statement removes the row of the old name. On a
-- database that never recorded it, that statement deletes nothing.

ALTER TABLE alerts ADD COLUMN IF NOT EXISTS dedupe_key TEXT;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS occurrence_count INTEGER NOT NULL DEFAULT 1;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- The lookup before every coalescing insert: newest open alert for one key in
-- one organization since a point in time.
CREATE INDEX IF NOT EXISTS idx_alerts_org_dedupe_key_created_at
    ON alerts (organization_id, dedupe_key, created_at DESC)
    WHERE dedupe_key IS NOT NULL;

DELETE FROM schema_migrations WHERE version = '126_coalesce_repeated_alerts.sql';
