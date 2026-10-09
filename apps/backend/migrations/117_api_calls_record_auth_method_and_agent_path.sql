-- Record how each API call authenticated, and how a route's agent parameter
-- compared with the calling agent.
--
-- Routes under /agents/:id that an agent credential can reach either bind :id
-- to the calling agent (a request for another agent's ID is refused 403) or
-- are held: they still serve such requests while the effect of refusing them
-- is measured. The measurement is, per held route, the count of requests made
-- with an agent credential whose path ID was not the caller's own. api_calls
-- could not answer it: it stored the concrete path but not the route that
-- matched, nor whether the caller was an agent credential or a user session.
--
-- auth_method        how the caller authenticated: ed25519, mldsa, hybrid,
--                    api_key, atc or service. NULL for a user session.
-- agent_path_route   the matched route, "METHOD /api/v1/.../:id", on a route
--                    that binds or holds its agent parameter. NULL elsewhere.
-- agent_path_outcome self, other, no_principal or fault: how the parameter
--                    compared with the calling agent. On a bound route other
--                    was refused; on a held route it was served.
--
-- No CHECK constraint: the row is written after the response, and a value
-- outside a fixed list must not cost the whole record.
--
-- scripts/agent_path_binding_report.sql reads these columns.

ALTER TABLE api_calls
    ADD COLUMN IF NOT EXISTS auth_method TEXT,
    ADD COLUMN IF NOT EXISTS agent_path_route TEXT,
    ADD COLUMN IF NOT EXISTS agent_path_outcome TEXT;

-- The report filters on the route and a time window; only rows on bound or
-- held routes carry one, so a partial index stays small.
CREATE INDEX IF NOT EXISTS idx_api_calls_agent_path_route_time
    ON api_calls (agent_path_route, called_at DESC)
    WHERE agent_path_route IS NOT NULL;

COMMENT ON COLUMN api_calls.auth_method IS
    'How the caller authenticated: ed25519, mldsa, hybrid, api_key, atc or service. NULL for a user session.';
COMMENT ON COLUMN api_calls.agent_path_route IS
    'Matched route (METHOD and path template) on a route that binds or holds its agent parameter; NULL elsewhere.';
COMMENT ON COLUMN api_calls.agent_path_outcome IS
    'self, other, no_principal or fault: the route''s agent parameter compared with the calling agent.';
