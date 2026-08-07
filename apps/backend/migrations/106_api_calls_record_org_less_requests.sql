-- Record unauthenticated requests in api_calls.
--
-- `api_calls.organization_id` was `NOT NULL`, and `logAPICall` returned early whenever the
-- request carried no organization. Between them, no request to an unauthenticated route
-- was recorded — or could be. That is not a gap in what we searched; it is a gap in what
-- could ever exist.
--
-- It became load-bearing in the 2026 verification-event exposure: the affected route was
-- unauthenticated, so `api_calls` holds zero rows for it BY CONSTRUCTION, and the honest
-- statement about whether it was ever exploited is "no record could exist" rather than
-- "no evidence of exploitation". Container logs that carried the request path expired
-- under 30-day retention before anyone asked. Every unauthenticated route in the service
-- is in that position today.
--
-- Making the column nullable is the smallest change that closes it: the same table, the
-- same middleware, the same indexes, with the org simply absent where there is no org.
-- The alternative — a separate table for org-less calls — splits the one place an
-- investigator looks into two, and the next investigation forgets the second.

ALTER TABLE api_calls ALTER COLUMN organization_id DROP NOT NULL;

-- Existing indexes lead with organization_id, so they do not serve a lookup that filters
-- only on endpoint and time — which is exactly the shape of "was this route ever called".
-- Without this, the query that answers an exposure question does a sequential scan over
-- every API call ever recorded.
CREATE INDEX IF NOT EXISTS idx_api_calls_endpoint_time
    ON api_calls (endpoint, called_at DESC);

-- Partial index for the org-less rows this migration makes possible, so
-- "which unauthenticated routes were called, and when" stays cheap as the table grows.
CREATE INDEX IF NOT EXISTS idx_api_calls_anonymous_time
    ON api_calls (called_at DESC)
    WHERE organization_id IS NULL;

COMMENT ON COLUMN api_calls.organization_id IS
    'Caller organization, or NULL for an unauthenticated request. NULL is meaningful: it '
    'is the only way an unauthenticated route appears here at all. Do not restore NOT NULL '
    'without replacing the forensic channel it removes.';
