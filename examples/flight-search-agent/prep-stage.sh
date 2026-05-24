#!/bin/bash
# Pre-stage setup. Run ONCE before the demo (or after each reset-demo.sh).
# Leaves the system in this state:
#   - org enforcement_mode = monitoring (so registration succeeds)
#   - agent flight-search-agent = verified, trust 0.92, capabilities bound
#   - local SDK credentials cache populated with a fresh refresh token
#
# On stage:
#   python flight_agent.py        # uses cached creds (Using existing credentials)
#   flightagent> search NYC       # Beats 1-3 ALLOW
#   flightagent> inject data-exfil  # Beat 4 - still allowed in monitoring
#   # (operator clicks Strict Mode in dashboard - Beat 5)
#   flightagent> inject data-exfil  # Beat 6 - now DENIED

set -e
cd "$(dirname "$0")"

# Per-run scratch dir under $TMPDIR (defaults to /tmp). Avoids hardcoded
# /tmp paths so two operators on the same host can't collide and so an
# attacker cannot pre-create a symlink at a known path to redirect writes
# (CWE-377).
WORKDIR=$(mktemp -d -t aim-prep-stage-XXXXXX)
trap 'rm -rf "$WORKDIR"' EXIT

./reset-demo.sh

echo "Minting a fresh SDK refresh token (the existing one may have been rotated)..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/public/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@opena2a.org","password":"AIM2025!Secure"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['accessToken'])")
if [ -z "$TOKEN" ]; then
  echo "ERROR: admin login failed. Check backend health on localhost:8080."
  exit 1
fi
curl -s -o "$WORKDIR/aim-sdk-fresh.zip" \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/sdk/download?sdk=python"
unzip -q "$WORKDIR/aim-sdk-fresh.zip" -d "$WORKDIR/aim-sdk-fresh"
cp "$WORKDIR/aim-sdk-fresh/aim-sdk-python/.aim/sdk_credentials.json" ~/.aim/sdk_credentials.json
# The bundled-SDK-side copy is optional — the SDK falls back to the home-dir
# location. Only sync the local aim-sdk-python copy if that bundle exists.
if [ -d aim-sdk-python/.aim ]; then
  cp "$WORKDIR/aim-sdk-fresh/aim-sdk-python/.aim/sdk_credentials.json" aim-sdk-python/.aim/sdk_credentials.json
fi
rm -f ~/.aim/sdk_credentials.encrypted ~/.aim/temp_validation_creds.encrypted

echo "Registering agent (register-only, no search — keeps audit log clean for stage)..."
source .venv/bin/activate
python3 -c "
import sys, os
sys.path.insert(0, os.path.join(os.getcwd(), '../../sdk/python'))
sys.path.insert(0, os.path.join(os.getcwd(), 'aim-sdk-python'))
from flight_agent import FlightAgent
agent = FlightAgent()
if not agent.client or not agent.agent_id:
    print('ERROR: agent failed to register')
    sys.exit(1)
print(f'Registered agent_id={agent.agent_id}')
" > "$WORKDIR/register.log" 2>&1 || {
  echo "ERROR: registration failed. Last 20 lines:"
  tail -20 "$WORKDIR/register.log"
  exit 1
}

echo "Promoting agent to verified, trust 0.92, syncing both trust stores..."
docker exec aim-postgres psql -U postgres -d identity -c \
  "UPDATE agents SET status='verified', trust_score=0.92, verified_at=NOW(), last_active=NOW() WHERE name='flight-search-agent';
   UPDATE trust_scores SET score=0.92, last_calculated=NOW() WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');
   SELECT 'agents.trust_score' AS source, trust_score::text AS value FROM agents WHERE name='flight-search-agent'
   UNION ALL SELECT 'trust_scores.score', score::text FROM trust_scores WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');"

python3 -c "
import json, pathlib
p = pathlib.Path.home() / '.aim' / 'agents' / 'flight-search-agent.json'
d = json.loads(p.read_text())
d['status'] = 'verified'
d['trust_score'] = 0.92
p.write_text(json.dumps(d, indent=2))
print('Cached creds updated: status=verified, trust_score=0.92')
"

echo "Clearing registration-time noise from all activity surfaces (clean stage start)..."
docker exec aim-postgres psql -U postgres -d identity -c "
DELETE FROM audit_logs           WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');
DELETE FROM alerts               WHERE resource_id=(SELECT id FROM agents WHERE name='flight-search-agent');
DELETE FROM capability_violations WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');
DELETE FROM verification_events  WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');
DELETE FROM trust_score_history  WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent');
-- Seed one baseline trust history entry so the API returns a non-null array.
-- previous_score=NULL avoids a misleading 42 percent jump on the timeline.
INSERT INTO trust_score_history (agent_id, organization_id, trust_score, previous_score, change_reason, recorded_at)
SELECT id, organization_id, 0.92, NULL, 'agent_registered', NOW() - INTERVAL '2 minutes'
FROM agents WHERE name='flight-search-agent';
" > /dev/null
echo "Activity feed reset: 0 audit logs, 0 alerts, 0 violations, 0 verification events, 0 trust history."

echo "Attaching MCP servers (github, slack, google-calendar) for Beat 2 richness..."
docker exec aim-postgres psql -U postgres -d identity -c "
WITH org AS (SELECT 'a0000000-0000-0000-0000-000000000001'::uuid AS id),
     usr AS (SELECT 'a0000000-0000-0000-0000-000000000002'::uuid AS id),
     gh  AS (INSERT INTO mcp_servers (organization_id, name, description, url, version, status, is_verified, capabilities, created_by, last_verified_at)
             SELECT org.id, 'github', 'GitHub repository operations (read PRs, issues, code search)', 'https://github.com/mcp', '1.0.0', 'verified', true, '[\"get_repository\",\"search_repositories\",\"list_branches\",\"get_file_contents\",\"list_pull_requests\",\"get_pull_request\",\"create_issue\",\"list_issues\",\"search_code\",\"list_commits\"]'::jsonb, usr.id, NOW()
             FROM org, usr
             ON CONFLICT (organization_id, url) DO UPDATE SET name=EXCLUDED.name, capabilities=EXCLUDED.capabilities, description=EXCLUDED.description, version=EXCLUDED.version, status=EXCLUDED.status, is_verified=EXCLUDED.is_verified, last_verified_at=EXCLUDED.last_verified_at, updated_at=NOW() RETURNING id),
     sl  AS (INSERT INTO mcp_servers (organization_id, name, description, url, version, status, is_verified, capabilities, created_by, last_verified_at)
             SELECT org.id, 'slack', 'Slack notifications and channel messages', 'https://slack.com/mcp', '1.2.0', 'verified', true, '[\"chat.postMessage\",\"channels.list\",\"conversations.history\",\"users.list\",\"files.upload\",\"reactions.add\"]'::jsonb, usr.id, NOW()
             FROM org, usr
             ON CONFLICT (organization_id, url) DO UPDATE SET name=EXCLUDED.name, capabilities=EXCLUDED.capabilities, description=EXCLUDED.description, version=EXCLUDED.version, status=EXCLUDED.status, is_verified=EXCLUDED.is_verified, last_verified_at=EXCLUDED.last_verified_at, updated_at=NOW() RETURNING id),
     gc  AS (INSERT INTO mcp_servers (organization_id, name, description, url, version, status, is_verified, capabilities, created_by, last_verified_at)
             SELECT org.id, 'google-calendar', 'Google Calendar events and booking', 'https://calendar.google.com/mcp', '0.9.1', 'verified', true, '[\"events.list\",\"events.insert\",\"events.update\",\"calendars.list\",\"freebusy.query\"]'::jsonb, usr.id, NOW()
             FROM org, usr
             ON CONFLICT (organization_id, url) DO UPDATE SET name=EXCLUDED.name, capabilities=EXCLUDED.capabilities, description=EXCLUDED.description, version=EXCLUDED.version, status=EXCLUDED.status, is_verified=EXCLUDED.is_verified, last_verified_at=EXCLUDED.last_verified_at, updated_at=NOW() RETURNING id)
UPDATE agents
SET talks_to = jsonb_build_array((SELECT id::text FROM gh), (SELECT id::text FROM sl), (SELECT id::text FROM gc))
WHERE name='flight-search-agent';
" > /dev/null

docker exec aim-postgres psql -U postgres -d identity -c "
INSERT INTO agent_mcp_connections (agent_id, mcp_server_id, connection_type, first_connected_at, last_attested_at, attestation_count, is_active)
SELECT a.id, m.id, 'user_registered', NOW() - INTERVAL '3 days', NOW() - INTERVAL '5 minutes', 12, true
FROM agents a, mcp_servers m
WHERE a.name='flight-search-agent' AND m.name IN ('github','slack','google-calendar') AND m.organization_id='a0000000-0000-0000-0000-000000000001'
ON CONFLICT (agent_id, mcp_server_id) DO UPDATE SET is_active=true, last_attested_at=EXCLUDED.last_attested_at, attestation_count=EXCLUDED.attestation_count;
" > /dev/null

mcp_count=$(docker exec aim-postgres psql -U postgres -d identity -t -c "SELECT count(*) FROM agent_mcp_connections WHERE agent_id=(SELECT id FROM agents WHERE name='flight-search-agent') AND is_active=true;" | tr -d ' ')
echo "MCP servers attached: $mcp_count (github, slack, google-calendar)."

echo "Populating MCP capabilities, attestations, and trust scores..."
docker exec aim-postgres psql -U postgres -d identity -c "
DELETE FROM mcp_server_capabilities WHERE mcp_server_id IN (SELECT id FROM mcp_servers WHERE organization_id='a0000000-0000-0000-0000-000000000001' AND name IN ('github','slack','google-calendar'));
DELETE FROM mcp_trust_scores       WHERE mcp_server_id IN (SELECT id FROM mcp_servers WHERE organization_id='a0000000-0000-0000-0000-000000000001' AND name IN ('github','slack','google-calendar'));
" > /dev/null
docker exec aim-postgres psql -U postgres -d identity -c "
WITH gh AS (SELECT id FROM mcp_servers WHERE name='github' AND organization_id='a0000000-0000-0000-0000-000000000001'),
     sl AS (SELECT id FROM mcp_servers WHERE name='slack' AND organization_id='a0000000-0000-0000-0000-000000000001'),
     gc AS (SELECT id FROM mcp_servers WHERE name='google-calendar' AND organization_id='a0000000-0000-0000-0000-000000000001')

-- GitHub capabilities (10 tools)
INSERT INTO mcp_server_capabilities (mcp_server_id, name, capability_type, description, last_verified_at)
SELECT gh.id, name, 'tool', description, NOW() FROM gh, (VALUES
  ('get_repository',     'Fetch metadata for a single repository'),
  ('search_repositories','Search public and private repositories by query string'),
  ('list_branches',      'List branches in a repository'),
  ('get_file_contents',  'Read the contents of a file at a given path'),
  ('list_pull_requests', 'List pull requests with filters'),
  ('get_pull_request',   'Fetch a pull request and its review state'),
  ('create_issue',       'Open a new issue in a repository'),
  ('list_issues',        'List issues with filters'),
  ('search_code',        'Full-text search across code in indexed repositories'),
  ('list_commits',       'List commits on a branch or path')
) AS t(name, description);
" > /dev/null

docker exec aim-postgres psql -U postgres -d identity -c "
WITH sl AS (SELECT id FROM mcp_servers WHERE name='slack' AND organization_id='a0000000-0000-0000-0000-000000000001')
INSERT INTO mcp_server_capabilities (mcp_server_id, name, capability_type, description, last_verified_at)
SELECT sl.id, name, 'tool', description, NOW() FROM sl, (VALUES
  ('chat.postMessage',     'Post a message to a Slack channel or DM'),
  ('channels.list',        'List public channels in the workspace'),
  ('conversations.history','Fetch message history for a channel'),
  ('users.list',           'List workspace members'),
  ('files.upload',         'Upload a file to a channel or DM'),
  ('reactions.add',        'Add an emoji reaction to a message')
) AS t(name, description);
" > /dev/null

docker exec aim-postgres psql -U postgres -d identity -c "
WITH gc AS (SELECT id FROM mcp_servers WHERE name='google-calendar' AND organization_id='a0000000-0000-0000-0000-000000000001')
INSERT INTO mcp_server_capabilities (mcp_server_id, name, capability_type, description, last_verified_at)
SELECT gc.id, name, 'tool', description, NOW() FROM gc, (VALUES
  ('events.list',    'List events in a calendar within a date range'),
  ('events.insert',  'Create a new calendar event'),
  ('events.update',  'Modify an existing calendar event'),
  ('calendars.list', 'List calendars accessible to the user'),
  ('freebusy.query', 'Query free/busy times across calendars')
) AS t(name, description);
" > /dev/null

# Trust scores per MCP server.
# Honest math for just-declared MCPs with zero attestations:
#   verification: 1.0   (AIM has verified the MCP is real)
#   uptime:       0.5   (no deployment-specific uptime data yet)
#   attestations: 0.0   (literal — we haven't seeded any, and the SDK only
#                        creates attestations when the agent actually uses
#                        the MCP, which this demo flow does not exercise)
#   community:    ~0.6  (small ecosystem-recognition bonus for well-known names)
# Resulting scores are intentionally below the agent's 92% — MCPs accrue trust
# through usage history, and these have none yet on this deployment.
docker exec aim-postgres psql -U postgres -d identity -c "
INSERT INTO mcp_trust_scores (mcp_server_id, score, confidence, factors, last_calculated)
SELECT id, 0.68, 0.55, jsonb_build_object('verification', 1.0, 'uptime', 0.5, 'attestations', 0.0, 'community', 0.65), NOW()
FROM mcp_servers WHERE name='github' AND organization_id='a0000000-0000-0000-0000-000000000001';

INSERT INTO mcp_trust_scores (mcp_server_id, score, confidence, factors, last_calculated)
SELECT id, 0.60, 0.55, jsonb_build_object('verification', 1.0, 'uptime', 0.5, 'attestations', 0.0, 'community', 0.55), NOW()
FROM mcp_servers WHERE name='slack' AND organization_id='a0000000-0000-0000-0000-000000000001';

INSERT INTO mcp_trust_scores (mcp_server_id, score, confidence, factors, last_calculated)
SELECT id, 0.62, 0.55, jsonb_build_object('verification', 1.0, 'uptime', 0.5, 'attestations', 0.0, 'community', 0.58), NOW()
FROM mcp_servers WHERE name='google-calendar' AND organization_id='a0000000-0000-0000-0000-000000000001';

UPDATE mcp_servers SET trust_score=0.68 WHERE name='github' AND organization_id='a0000000-0000-0000-0000-000000000001';
UPDATE mcp_servers SET trust_score=0.60 WHERE name='slack' AND organization_id='a0000000-0000-0000-0000-000000000001';
UPDATE mcp_servers SET trust_score=0.62 WHERE name='google-calendar' AND organization_id='a0000000-0000-0000-0000-000000000001';
" > /dev/null

# Attestations intentionally NOT pre-seeded. The SDK creates attestations at
# runtime when the agent actually uses an MCP tool (signed with the agent's
# private key). Pre-seeding fake signatures would be dishonest, and the demo
# doesn't exercise an MCP tool call anyway, so "Attestations: 0" on the MCP
# detail page is the truthful state.

caps_total=$(docker exec aim-postgres psql -U postgres -d identity -t -c "SELECT count(*) FROM mcp_server_capabilities WHERE mcp_server_id IN (SELECT id FROM mcp_servers WHERE organization_id='a0000000-0000-0000-0000-000000000001' AND name IN ('github','slack','google-calendar'));" | tr -d ' ')
echo "MCP enrichment complete: $caps_total capabilities (10 github + 6 slack + 5 calendar), trust scores set (github 68%, slack 60%, calendar 62% — intentionally below the agent's 92% because MCPs have no usage history yet). Attestations left at 0 (SDK creates them at runtime)."

echo ""
echo "Stage-ready. To run the demo:"
echo "  source .venv/bin/activate"
echo "  python flight_agent.py"
echo ""
echo "Mid-demo strict-mode flip (Beat 5):"
echo "  docker exec aim-postgres psql -U postgres -d identity -c \\"
echo "    \"UPDATE organizations SET enforcement_mode='strict' WHERE id='a0000000-0000-0000-0000-000000000001';\""
echo "  (or click the Global Strict Mode toggle in the dashboard)"
