-- Migration: Add source_ip column to alerts table
-- This captures the IP address that triggered the alert for security tracking

-- Add source_ip column to alerts table
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS source_ip VARCHAR(45);

-- Create index for faster lookups by source IP
CREATE INDEX IF NOT EXISTS idx_alerts_source_ip ON alerts(source_ip);

-- Add comment explaining the column
COMMENT ON COLUMN alerts.source_ip IS 'IP address that triggered the alert (IPv4 or IPv6)';
