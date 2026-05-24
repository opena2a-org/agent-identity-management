#!/bin/bash
# Clean state between flight-search-agent demo iterations.
# Wipes the agent + local cache, and always leaves the org in MONITORING
# mode (registration only works in monitoring; strict is enabled on stage).

set -e

rm -f ~/.aim/agents/flight-search-agent.json
docker exec aim-postgres psql -U postgres -d identity -c \
  "DELETE FROM agents WHERE name = 'flight-search-agent';" > /dev/null
docker exec aim-postgres psql -U postgres -d identity -c \
  "UPDATE organizations SET enforcement_mode='monitoring' WHERE id='a0000000-0000-0000-0000-000000000001';" > /dev/null

echo "Demo reset complete (mode: monitoring, agent: wiped)."
echo "Next: ./prep-stage.sh to register + verify the agent before stage."
