-- Migration 012: Remove Approval Requests Feature
-- This feature was removed to simplify the product and focus on verification monitoring

-- Drop the approval_requests table (formerly verifications)
DROP TABLE IF EXISTS approval_requests CASCADE;

-- Note: We're keeping verification_events table as that's the new monitoring feature
