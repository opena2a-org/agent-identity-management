-- Fix alerts table schema to match expected structure
-- This migration adds missing columns if they don't exist

-- Add is_acknowledged column if missing
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'alerts' AND column_name = 'is_acknowledged'
    ) THEN
        ALTER TABLE alerts ADD COLUMN is_acknowledged BOOLEAN NOT NULL DEFAULT FALSE;
    END IF;
END $$;

-- Add acknowledged_by column if missing
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'alerts' AND column_name = 'acknowledged_by'
    ) THEN
        ALTER TABLE alerts ADD COLUMN acknowledged_by UUID REFERENCES users(id);
    END IF;
END $$;

-- Add acknowledged_at column if missing
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'alerts' AND column_name = 'acknowledged_at'
    ) THEN
        ALTER TABLE alerts ADD COLUMN acknowledged_at TIMESTAMP;
    END IF;
END $$;

-- Create indexes if they don't exist
CREATE INDEX IF NOT EXISTS idx_alerts_is_acknowledged ON alerts(is_acknowledged);
CREATE INDEX IF NOT EXISTS idx_alerts_organization_id ON alerts(organization_id);
CREATE INDEX IF NOT EXISTS idx_alerts_alert_type ON alerts(alert_type);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at DESC);
