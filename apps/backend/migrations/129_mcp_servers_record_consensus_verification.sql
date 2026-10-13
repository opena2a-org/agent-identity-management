-- Migration 129: record which MCP servers multi-agent consensus verified.
--
-- A server reaches verified status in three ways: SDK registration (verified when an agent
-- registers it), challenge-response verification (POST /api/v1/mcp-servers/:id/verify), and
-- multi-agent consensus over its attestations. Only the last one rests on what the attesting
-- agents report, so only that one is withdrawn when their attestations stop agreeing. The
-- verification_method column cannot tell them apart: neither registration nor any of the three
-- verifications writes it, so every row holds the column default.
--
-- consensus_verified_at is set when consensus verifies a server and cleared when the server is
-- verified by challenge-response or loses its verified status.

ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS consensus_verified_at TIMESTAMPTZ;

COMMENT ON COLUMN mcp_servers.consensus_verified_at IS
    'When multi-agent consensus verified the server. NULL when the server is not verified or its '
    'verification came from SDK registration or challenge-response verification. A server with a '
    'value loses its verified status when its attesting agents stop agreeing.';

-- Servers verified before this column existed. A verified server is marked as verified by
-- consensus when all of these hold:
--   - no agent registered it (SDK registration verifies a server when it is created);
--   - an agent attestation was recorded at or before its last verification time, so it was
--     verified after agents attested it, as consensus requires; a server verified at creation
--     has no attestation that old;
--   - it has no successful MCP identity verification event, which challenge-response
--     verification records.
-- A verified server that fails any of these is not marked and keeps its status. The backend
-- rechecks the marked servers at start and withdraws the verification of any whose attestations
-- disagree.
UPDATE mcp_servers s
SET consensus_verified_at = s.last_verified_at
WHERE s.status = 'verified'
  AND s.consensus_verified_at IS NULL
  AND s.registered_by_agent IS NULL
  AND s.last_verified_at IS NOT NULL
  AND EXISTS (
      SELECT 1
      FROM mcp_attestations a
      WHERE a.mcp_server_id = s.id
        AND a.agent_id IS NOT NULL
        AND a.signature <> 'manual-attestation'
        AND a.created_at <= s.last_verified_at
  )
  AND NOT EXISTS (
      SELECT 1
      FROM verification_events e
      WHERE e.mcp_server_id = s.id
        AND e.protocol = 'MCP'
        AND e.verification_type = 'identity'
        AND e.status = 'success'
  );
