-- Migration 116: onboarding telemetry
--
-- One row per onboarding step an organization takes: the dashboard showing
-- the first-run screen, an SDK tab being picked, a bootstrap token being
-- minted or exchanged, the first agent being registered, and the screen being
-- completed or skipped. A row holds the organization, the event, the tab for
-- tab_selected, and the time. It holds no user, email address, client
-- address, user agent or free text; event and tab are closed sets.
--
-- first_agent_registered is recorded at most once per organization, stamped
-- with the earliest agent's created_at. The INSERT at the end backfills it
-- for every organization that already has an agent, so the funnel starts
-- with the history the agents table already holds.

CREATE TABLE IF NOT EXISTS onboarding_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event VARCHAR(32) NOT NULL CHECK (event IN (
        'onboarding_viewed',
        'tab_selected',
        'token_minted',
        'token_exchanged',
        'first_agent_registered',
        'onboarding_completed',
        'onboarding_skipped'
    )),
    tab VARCHAR(16) CHECK (tab IN ('python', 'typescript', 'java', 'go', 'cli', 'mcp', 'claude', 'cursor')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT onboarding_events_tab_only_on_tab_selected CHECK ((event = 'tab_selected') = (tab IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_onboarding_events_org_time ON onboarding_events(organization_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_onboarding_events_event_time ON onboarding_events(event, occurred_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_onboarding_events_one_first_agent
    ON onboarding_events(organization_id)
    WHERE event = 'first_agent_registered';

INSERT INTO onboarding_events (organization_id, event, occurred_at)
SELECT organization_id, 'first_agent_registered', MIN(created_at)
FROM agents
GROUP BY organization_id
ON CONFLICT (organization_id) WHERE event = 'first_agent_registered' DO NOTHING;
