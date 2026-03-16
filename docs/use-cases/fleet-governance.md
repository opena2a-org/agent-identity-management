# Use Case: Manage Identity Across Multiple Agents

**Time:** 30 minutes
**Prerequisites:** Docker, Docker Compose

## Problem

You have a team running multiple AI agents across different machines. Each agent needs its own identity, but you need centralized audit logging, policy management, and OIDC integration -- not isolated local files on each developer's laptop.

## Step 1: Deploy AIM Server

Pull the images:

```bash
docker pull opena2a/aim-server
docker pull opena2a/aim-dashboard
```

Create `docker-compose.yml`:

```yaml
version: "3.8"
services:
  aim-server:
    image: opena2a/aim-server:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://aim:aim@db:5432/aim
      - JWT_SECRET=change-this-to-a-random-value
    depends_on:
      - db

  aim-dashboard:
    image: opena2a/aim-dashboard:latest
    ports:
      - "3000:3000"
    environment:
      - API_URL=http://aim-server:8080

  db:
    image: postgres:16
    environment:
      - POSTGRES_USER=aim
      - POSTGRES_PASSWORD=aim
      - POSTGRES_DB=aim
    volumes:
      - aim-data:/var/lib/postgresql/data

volumes:
  aim-data:
```

Start the stack:

```bash
docker compose up -d
```

Expected output:

```
[+] Running 3/3
 - Container aim-db-1          Started
 - Container aim-aim-server-1  Started
 - Container aim-aim-dashboard-1 Started
```

Verify the server is running:

```bash
curl http://localhost:8080/health
```

Expected output:

```json
{"status": "healthy", "version": "1.0.0", "database": "connected"}
```

The dashboard is available at [http://localhost:3000](http://localhost:3000).

## Step 2: Register Agents

Register your first agent against the server:

```bash
opena2a identity create --name data-processor --server http://localhost:8080
```

Expected output:

```
Agent created:
  ID:         aim_9c4b2e1f
  Name:       data-processor
  Public Key: ed25519:k3Lm...nP7Q
  Server:     http://localhost:8080
  Stored:     ~/.opena2a/aim-core/identities/data-processor.json
```

Register a second agent:

```bash
opena2a identity create --name code-reviewer --server http://localhost:8080
```

Expected output:

```
Agent created:
  ID:         aim_2d8f5a3c
  Name:       code-reviewer
  Public Key: ed25519:r9Wj...tY2X
  Server:     http://localhost:8080
  Stored:     ~/.opena2a/aim-core/identities/code-reviewer.json
```

## Step 3: Centralized Audit Log

When agents are connected to a server, all audit events are sent to the central PostgreSQL database. Query them via the API:

```bash
curl http://localhost:8080/api/v1/audit?limit=20
```

Expected output:

```json
{
  "events": [
    {
      "id": "evt_a1b2c3",
      "agentId": "aim_9c4b2e1f",
      "agentName": "data-processor",
      "action": "identity:create",
      "target": "data-processor",
      "outcome": "allowed",
      "timestamp": "2026-03-16T14:00:00Z",
      "hash": "sha256:f8a1..."
    },
    {
      "id": "evt_d4e5f6",
      "agentId": "aim_2d8f5a3c",
      "agentName": "code-reviewer",
      "action": "identity:create",
      "target": "code-reviewer",
      "outcome": "allowed",
      "timestamp": "2026-03-16T14:01:00Z",
      "hash": "sha256:c2d4..."
    }
  ],
  "total": 2,
  "limit": 20
}
```

Filter by agent:

```bash
curl http://localhost:8080/api/v1/audit?agentId=aim_9c4b2e1f&limit=50
```

## Step 4: OIDC Integration

AIM Server includes an OAuth 2.0 / OIDC token endpoint for machine-to-machine authentication. Agents can request tokens that other services verify.

Configure your identity provider in the server environment:

```yaml
environment:
  - OIDC_ISSUER=https://auth.example.com
  - OIDC_CLIENT_ID=aim-server
  - OIDC_CLIENT_SECRET=your-client-secret
```

Request a token for an agent:

```bash
curl -X POST http://localhost:8080/api/v1/token \
  -H "Content-Type: application/json" \
  -d '{"agentId": "aim_9c4b2e1f", "scope": "db:read api:call"}'
```

Expected output:

```json
{
  "accessToken": "eyJhbGciOiJFZERTQSIs...",
  "tokenType": "Bearer",
  "expiresIn": 3600,
  "scope": "db:read api:call"
}
```

Other services verify the token against the AIM Server's JWKS endpoint at `http://localhost:8080/.well-known/jwks.json`.

## Step 5: Fleet Overview via Dashboard

Open [http://localhost:3000](http://localhost:3000) in your browser. The dashboard shows:

- **Agent inventory** -- all registered agents with trust scores
- **Audit timeline** -- real-time event stream across all agents
- **Policy status** -- which agents have policies loaded, any violations
- **Trust trends** -- score history per agent over time

## Architecture

```
Developer A                    Developer B
  |                              |
  opena2a CLI                    opena2a CLI
  |                              |
  +-------> AIM Server <---------+
              |
              PostgreSQL
              |
              AIM Dashboard (port 3000)
```

| Component | Local Mode | Server Mode |
|-----------|-----------|-------------|
| Identity storage | `~/.opena2a/aim-core/` | Server + local cache |
| Audit log | `audit.jsonl` (file) | PostgreSQL |
| Policy management | YAML files | REST API + dashboard |
| Trust scoring | Local calculation | Server-side + history |
| Multi-agent | Per-machine only | Cross-machine fleet |
| OIDC tokens | Not available | Built-in token endpoint |

## What You Now Have

- Centralized identity management for all agents in your organization
- PostgreSQL-backed audit log queryable via REST API
- OIDC token endpoint for machine-to-machine authentication
- A dashboard for monitoring trust scores and audit events across the fleet

## Next Steps

- [Enforce capabilities](enforce-capabilities.md) -- define policies for each agent in the fleet
- [Embed in my app](embed-in-my-app.md) -- connect your custom agents to the server programmatically
- [Deployment guide](../../infrastructure/DEPLOYMENT.md) -- production deployment on AWS, Azure, GCP, and Kubernetes
