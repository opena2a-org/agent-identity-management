-- Revert: Rename approval_requests back to verifications
ALTER TABLE approval_requests RENAME TO verifications;

-- Revert: Rename indexes
ALTER INDEX idx_approval_requests_organization_id RENAME TO idx_verifications_organization_id;
ALTER INDEX idx_approval_requests_agent_id RENAME TO idx_verifications_agent_id;
ALTER INDEX idx_approval_requests_status RENAME TO idx_verifications_status;
ALTER INDEX idx_approval_requests_created_at RENAME TO idx_verifications_created_at;

-- Revert: Update comments
COMMENT ON TABLE verifications IS 'Agent verification requests and approval workflows';
COMMENT ON COLUMN verifications.status IS 'Verification status: approved, denied, pending';
