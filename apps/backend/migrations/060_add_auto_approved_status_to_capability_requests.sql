-- Migration: Add auto-approved status to capability requests
-- This allows capability requests to be auto-approved when organization is in monitoring mode

-- Drop the old constraint and add a new one that includes 'auto-approved'
ALTER TABLE capability_requests DROP CONSTRAINT IF EXISTS capability_requests_status_check;

ALTER TABLE capability_requests
ADD CONSTRAINT capability_requests_status_check
CHECK (status IN ('pending', 'approved', 'auto-approved', 'rejected'));
