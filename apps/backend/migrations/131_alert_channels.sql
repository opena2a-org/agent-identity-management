-- Migration 131: alert channels.
--
-- An alert channel sends an organization's alerts to one destination in that
-- service's published format: a Slack incoming webhook, or a PagerDuty Events
-- API v2 integration. The server sends a channel each alert whose severity
-- ranks at or above the channel's min_severity (info < warning < high <
-- critical).
--
-- config holds the destination: {"webhookUrl": ...} for slack and
-- {"routingKey": ...} for pagerduty. It is the credential that posts to the
-- destination, so no answer, log line or audit row carries it; answers carry
-- each value masked to its last four characters.
--
-- last_delivery_status is 'delivered' or 'failed' after the channel's latest
-- delivery, last_delivery_code the HTTP status of its final attempt (NULL
-- after a network error), and last_delivery_at when it ended.
--
-- The kind check is named so a later migration can widen it. Idempotent.

CREATE TABLE IF NOT EXISTS alert_channels (
    id                   UUID        PRIMARY KEY,
    organization_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                 TEXT        NOT NULL,
    kind                 TEXT        NOT NULL,
    config               JSONB       NOT NULL,
    min_severity         TEXT        NOT NULL CHECK (min_severity IN ('info', 'warning', 'high', 'critical')),
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,
    last_delivery_status TEXT,
    last_delivery_code   INTEGER,
    last_delivery_at     TIMESTAMPTZ,
    created_by           UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT alert_channels_kind_check CHECK (kind IN ('slack', 'pagerduty')),
    CONSTRAINT alert_channels_organization_id_name_key UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_alert_channels_organization_id
    ON alert_channels (organization_id);

COMMENT ON TABLE alert_channels IS
    'Destinations an organization''s alerts are sent to (Slack incoming webhooks, PagerDuty Events API v2); config holds the destination credential';
