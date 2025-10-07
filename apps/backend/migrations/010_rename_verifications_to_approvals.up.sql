-- Rename existing verifications table to approval_requests (manual workflow)
ALTER TABLE verifications RENAME TO approval_requests;

-- Rename indexes
ALTER INDEX idx_verifications_organization_id RENAME TO idx_approval_requests_organization_id;
ALTER INDEX idx_verifications_agent_id RENAME TO idx_approval_requests_agent_id;
ALTER INDEX idx_verifications_status RENAME TO idx_approval_requests_status;
ALTER INDEX idx_verifications_created_at RENAME TO idx_approval_requests_created_at;

-- Update comments
COMMENT ON TABLE approval_requests IS 'Agent approval requests for permission workflows';
COMMENT ON COLUMN approval_requests.status IS 'Approval status: approved, denied, pending';
COMMENT ON COLUMN approval_requests.action IS 'Action being requested (e.g., File Access Request, API Key Generation)';
