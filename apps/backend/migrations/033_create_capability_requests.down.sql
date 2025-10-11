-- Drop capability_requests table and related objects

DROP TRIGGER IF EXISTS trigger_update_capability_requests_updated_at ON capability_requests;
DROP FUNCTION IF EXISTS update_capability_requests_updated_at();
DROP TABLE IF EXISTS capability_requests CASCADE;
