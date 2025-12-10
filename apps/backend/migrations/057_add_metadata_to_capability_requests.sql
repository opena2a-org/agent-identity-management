-- Add metadata column to capability_requests table
-- This column stores optional metadata provided with capability requests
ALTER TABLE capability_requests
ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- Add comment for documentation
COMMENT ON COLUMN capability_requests.metadata IS 'Optional metadata provided with the capability request (e.g., use case details, dependencies)';

-- Create index for efficient JSONB queries
CREATE INDEX IF NOT EXISTS idx_capability_requests_metadata ON capability_requests USING gin(metadata);
