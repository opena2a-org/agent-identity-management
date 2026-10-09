-- Agent path binding report: per route that binds or holds its agent
-- parameter, how agent credentials used it over the last 14 days.
--
--   psql "$DATABASE_URL" -f scripts/agent_path_binding_report.sql
--
-- A held route is listed in cmd/server/agent_binding_routes_test.go
-- (agentBindingHeld). Its row here releases it:
--   other_id = 0     bind it (register it through bindAgentRoutes).
--   other_id > 0     the third query names the callers; either admit the route
--                    as an exception naming that call site, or bind it with a
--                    dated migration for those callers.
-- On a bound route, other_id counts refusals (403, agent_path_mismatch).
--
-- Read the first query before the others. A count is complete only when
-- window_complete is true: the columns were added by migration 117, so a
-- window that starts before the first recorded row undercounts. A held route
-- with no row in the second query was not called in the window.

-- 1. Coverage: when the records start.
SELECT MIN(called_at)                                AS instrumented_since,
       MIN(called_at) <= NOW() - INTERVAL '14 days' AS window_complete,
       COUNT(*)                                      AS requests_recorded
FROM api_calls
WHERE agent_path_route IS NOT NULL;

-- 2. Per route: who called it, and how many agent calls named another ID.
SELECT agent_path_route                                                            AS route,
       COUNT(*)                                                                    AS requests,
       COUNT(*) FILTER (WHERE agent_path_outcome = 'no_principal')                 AS user_session,
       COUNT(*) FILTER (WHERE agent_path_outcome = 'self')                         AS own_id,
       COUNT(*) FILTER (WHERE agent_path_outcome = 'other')                        AS other_id,
       COUNT(*) FILTER (WHERE agent_path_outcome = 'fault')                        AS fault,
       COUNT(DISTINCT agent_id) FILTER (WHERE agent_path_outcome = 'other')        AS other_id_agents,
       COUNT(DISTINCT organization_id) FILTER (WHERE agent_path_outcome = 'other') AS other_id_organizations
FROM api_calls
WHERE agent_path_route IS NOT NULL
  AND called_at >= NOW() - INTERVAL '14 days'
GROUP BY agent_path_route
ORDER BY other_id DESC, agent_path_route;

-- 3. The callers behind each non-zero other_id: the call site an exception
--    must name, or the callers a binding's migration must reach.
SELECT agent_path_route AS route,
       auth_method,
       user_agent,
       status_code,
       COUNT(*)         AS requests
FROM api_calls
WHERE agent_path_outcome = 'other'
  AND called_at >= NOW() - INTERVAL '14 days'
GROUP BY agent_path_route, auth_method, user_agent, status_code
ORDER BY agent_path_route, requests DESC;
