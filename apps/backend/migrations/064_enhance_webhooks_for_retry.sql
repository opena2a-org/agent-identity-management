-- Migration: Enhance Webhooks for Retry Logic and Better Tracking
-- Created: 2025-12-24
-- Description: Add delivery settings, status tracking, and retry fields

-- Add new columns to webhooks table
ALTER TABLE webhooks
ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active',
ADD COLUMN IF NOT EXISTS description TEXT,
ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER DEFAULT 30,
ADD COLUMN IF NOT EXISTS max_retries INTEGER DEFAULT 3,
ADD COLUMN IF NOT EXISTS retry_delay_seconds INTEGER DEFAULT 60,
ADD COLUMN IF NOT EXISTS last_delivery_status VARCHAR(20),
ADD COLUMN IF NOT EXISTS success_count BIGINT DEFAULT 0,
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Add new columns to webhook_deliveries table
ALTER TABLE webhook_deliveries
ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'pending',
ADD COLUMN IF NOT EXISTS error_message TEXT,
ADD COLUMN IF NOT EXISTS duration_ms BIGINT,
ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

-- Backfill organization_id for existing deliveries
UPDATE webhook_deliveries wd
SET organization_id = w.organization_id
FROM webhooks w
WHERE wd.webhook_id = w.id AND wd.organization_id IS NULL;

-- Backfill status for existing deliveries based on success field
UPDATE webhook_deliveries
SET status = CASE
    WHEN success = true THEN 'success'
    ELSE 'failed'
END
WHERE status IS NULL OR status = 'pending';

-- Create indexes for retry queries
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status = 'retrying';
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_organization_id ON webhook_deliveries(organization_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_deleted_at ON webhooks(deleted_at) WHERE deleted_at IS NULL;

-- Add comments
COMMENT ON COLUMN webhooks.status IS 'Webhook status: active, paused, disabled';
COMMENT ON COLUMN webhooks.timeout_seconds IS 'HTTP request timeout in seconds';
COMMENT ON COLUMN webhooks.max_retries IS 'Maximum number of retry attempts';
COMMENT ON COLUMN webhooks.retry_delay_seconds IS 'Initial delay between retries (exponential backoff)';
COMMENT ON COLUMN webhooks.success_count IS 'Total successful deliveries';
COMMENT ON COLUMN webhooks.deleted_at IS 'Soft delete timestamp';

COMMENT ON COLUMN webhook_deliveries.status IS 'Delivery status: pending, success, failed, retrying, abandoned';
COMMENT ON COLUMN webhook_deliveries.error_message IS 'Error message if delivery failed';
COMMENT ON COLUMN webhook_deliveries.duration_ms IS 'Request duration in milliseconds';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'When to retry (for retrying status)';
COMMENT ON COLUMN webhook_deliveries.completed_at IS 'When delivery was completed (success or abandoned)';
