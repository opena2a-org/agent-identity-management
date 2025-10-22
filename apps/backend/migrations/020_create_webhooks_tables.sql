-- Migration: Create Webhooks and Webhook Deliveries tables
-- Created: 2025-10-22
-- Description: Sprint 4 - Webhooks Management System

-- Create webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}', -- Array of webhook event types
    secret VARCHAR(255) NOT NULL, -- For signature verification
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_triggered TIMESTAMPTZ,
    failure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    CONSTRAINT webhooks_name_unique_per_org UNIQUE (organization_id, name),
    CONSTRAINT webhooks_url_check CHECK (url ~* '^https?://.*'),
    CONSTRAINT webhooks_events_not_empty CHECK (array_length(events, 1) > 0)
);

-- Create webhook_deliveries table
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(100) NOT NULL, -- agent.created, trust_score.changed, etc.
    payload TEXT NOT NULL, -- JSON payload sent
    status_code INTEGER,
    response_body TEXT,
    success BOOLEAN NOT NULL DEFAULT false,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_webhooks_organization_id ON webhooks(organization_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_is_active ON webhooks(is_active);
CREATE INDEX IF NOT EXISTS idx_webhooks_created_at ON webhooks(created_at);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event ON webhook_deliveries(event);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_success ON webhook_deliveries(success);

-- Add comments for documentation
COMMENT ON TABLE webhooks IS 'Webhook subscriptions for event notifications (Sprint 4)';
COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery attempts and results (Sprint 4)';

COMMENT ON COLUMN webhooks.events IS 'Array of webhook events: agent.created, agent.verified, agent.suspended, trust_score.changed, alert.created, compliance.violation';
COMMENT ON COLUMN webhooks.secret IS 'Secret key for HMAC signature verification of webhook payloads';
COMMENT ON COLUMN webhooks.failure_count IS 'Cumulative count of failed delivery attempts';
COMMENT ON COLUMN webhook_deliveries.attempt_count IS 'Number of delivery attempts for this event';
