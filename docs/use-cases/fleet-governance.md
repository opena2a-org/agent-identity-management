# Use Case: Manage Identity Across Multiple Agents

**Time:** 30 minutes
**Prerequisites:** Docker, Docker Compose

## Problem

You have a team running multiple AI agents across different machines. Each agent needs its own identity, but you need centralized audit logging, policy management, and service tokens for agents -- not isolated local files on each developer's laptop.

## Step 1: Deploy AIM Server

Pull the images:

```bash
docker pull opena2a/aim-server
docker pull opena2a/aim-dashboard
```

In an empty directory, generate the server's secrets into a `.env` file. Docker Compose reads `.env` from the directory it runs in:

```bash
cat > .env <<EOF
JWT_SECRET=$(openssl rand -hex 32)
KEYVAULT_MASTER_KEY=$(openssl rand -base64 32)
POSTGRES_PASSWORD=$(openssl rand -hex 16)
EOF
```

`JWT_SECRET` signs the access tokens the server issues. The server refuses to start when it is shorter than 32 characters. `KEYVAULT_MASTER_KEY` encrypts the agent private keys the server stores in the database. Keep `.env` for as long as you keep the database volume: a server started with a different key cannot decrypt the keys stored under the old one.

Create `docker-compose.yml` in the same directory:

```yaml
services:
  aim-server:
    image: opena2a/aim-server:latest
    ports:
      - "8080:8080"
    environment:
      - POSTGRES_HOST=db
      - POSTGRES_USER=aim
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD in .env}
      - POSTGRES_DB=aim
      - REDIS_HOST=redis
      - JWT_SECRET=${JWT_SECRET:?Set JWT_SECRET in .env}
      - KEYVAULT_MASTER_KEY=${KEYVAULT_MASTER_KEY:?Set KEYVAULT_MASTER_KEY in .env}
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_started

  aim-dashboard:
    image: opena2a/aim-dashboard:latest
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
    depends_on:
      aim-server:
        condition: service_started

  db:
    image: postgres:16
    environment:
      - POSTGRES_USER=aim
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD in .env}
      - POSTGRES_DB=aim
    volumes:
      - aim-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U aim -d aim"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7-alpine

volumes:
  aim-data:
```

The server connects to the database once, when it starts, so it waits for the `db` healthcheck. It keeps its list of revoked tokens in Redis. Without Redis it still runs, but revoking a token, by logging out or otherwise, has no effect before the token expires.

Start the stack:

```bash
docker compose up -d
```

The server applies its database migrations when it starts. Verify it is running:

```bash
curl http://localhost:8080/health
```

Expected output:

```json
{"service":"agent-identity-management","status":"healthy","time":"2026-03-16T14:00:00.123456789Z"}
```

`time` is the server's clock in UTC when it answered.

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
      "result": "allowed",
      "timestamp": "2026-03-16T14:00:00Z",
      "hash": "sha256:f8a1..."
    },
    {
      "id": "evt_d4e5f6",
      "agentId": "aim_2d8f5a3c",
      "agentName": "code-reviewer",
      "action": "identity:create",
      "target": "code-reviewer",
      "result": "allowed",
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

## Step 4: Service Tokens for Agents

AIM Server has an OAuth 2.0 token endpoint for machine-to-machine authentication. An agent exchanges an assertion signed with its own Ed25519 key for an access token, using the JWT-bearer grant (RFC 7523). The endpoint needs no identity provider.

Set the URL that agents use to reach the server in the `aim-server` environment. The token endpoint compares the audience of each assertion with it:

```yaml
environment:
  - AIM_BASE_URL=http://localhost:8080
```

The assertion is a JWT (`alg` `EdDSA`) that the agent signs with its Ed25519 private key. The server checks the signature against the public key registered for the agent and requires three claims:

| Claim | Value |
|-------|-------|
| `sub` | The agent's ID on the server, a UUID. The same value as `client_id` |
| `aud` | The value of `AIM_BASE_URL` |
| `exp` | A time at most five minutes ahead |

`OAuthTokenManager` in [`sdk/typescript/src/auth/oauth.ts`](../../sdk/typescript/src/auth/oauth.ts) builds this assertion.

Request a token with the agent's ID in `$AGENT_ID` and the signed assertion in `$ASSERTION`:

```bash
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -d "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" \
  -d "client_id=$AGENT_ID" \
  -d "client_assertion=$ASSERTION"
```

Expected output:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

`expires_in` is the lifetime of the token in seconds: two hours unless `JWT_ACCESS_TTL` is set.

### Verifying the token

The access token is a JWT signed with HS256 under the server's `JWT_SECRET`. HS256 is symmetric: the key that verifies a token is the key that signs one. The server therefore publishes no JWK Set for it, and no public key verifies the token. The public keys the server does publish are Ed25519 keys for agent card attestations and ATCs, and none of them verifies an access token.

AIM Server is the verifier. The agent sends the token as `Authorization: Bearer <access_token>` on the server's `/api/v1/agents` routes, and on each request the server checks the signature, the expiry, whether the token has been revoked, and whether the agent is still allowed to authenticate. A token issued to an agent that is later suspended or revoked stops working at that point.

Two things follow:

- Keep `JWT_SECRET` on the server. A service that holds it to verify tokens can also issue them, for any user or agent.
- Do not hand the access token to another service as proof of the agent's identity. It is a bearer credential for AIM Server, and a service that receives it can use it there as the agent.

## Step 5: Fleet Overview via Dashboard

Open [http://localhost:3000](http://localhost:3000) in your browser. The dashboard shows:

- **Agent inventory** -- all registered agents with trust scores
- **Audit timeline** -- real-time event stream across all agents
- **Policy status** -- which agents have policies loaded, any violations
- **Trust trends** -- score history per agent over time

## Option B: SDK Approach

If you prefer to manage fleet identities programmatically instead of through the CLI, use the TypeScript or Python SDKs with the `server` parameter.

### TypeScript

```bash
npm install @opena2a/aim-core
```

```typescript
import { AIMCore } from '@opena2a/aim-core';

// Connect to your AIM Server
const dataProcessor = new AIMCore({
  agentName: 'data-processor',
  serverUrl: 'http://localhost:8080'
});

const codeReviewer = new AIMCore({
  agentName: 'code-reviewer',
  serverUrl: 'http://localhost:8080'
});

// Create identities (registered on the server)
const dpIdentity = dataProcessor.getIdentity();
console.log('Data Processor:', dpIdentity.agentId);

const crIdentity = codeReviewer.getIdentity();
console.log('Code Reviewer:', crIdentity.agentId);

// Save policies per agent
dataProcessor.savePolicy({
  version: '1.0',
  defaultAction: 'deny',
  rules: [
    { capability: 'db:read', action: 'allow' },
    { capability: 'api:call', action: 'allow' },
    { capability: 'db:delete', action: 'deny' },
    { capability: 'file:execute', action: 'deny' }
  ]
});

codeReviewer.savePolicy({
  version: '1.0',
  defaultAction: 'deny',
  rules: [
    { capability: 'file:read', action: 'allow' },
    { capability: 'api:call', action: 'allow' },
    { capability: 'db:write', action: 'deny' },
    { capability: 'db:delete', action: 'deny' }
  ]
});

// Log events (sent to the central server)
dataProcessor.logEvent({
  action: 'db:read',
  target: 'customers',
  result: 'allowed',
  plugin: 'data-processor'
});

// Calculate trust (server-side with history)
const dpTrust = dataProcessor.calculateTrust();
console.log(`Data Processor Trust: ${dpTrust.overall}`);

const crTrust = codeReviewer.calculateTrust();
console.log(`Code Reviewer Trust: ${crTrust.overall}`);
```

Expected output:

```
Data Processor: aim_9c4b2e1f
Code Reviewer: aim_2d8f5a3c
Data Processor Trust: 0.72
Code Reviewer Trust: 0.68
```

### Python

```bash
pip install -e sdk/python/
```

```python
from aim_sdk import secure, register_agent, AIMClient, AgentType

# One-line registration (recommended)
data_processor = secure(
    name="data-processor",
    capabilities=["db:read", "api:call"],
    agent_type=AgentType.LANGCHAIN,
    aim_url="http://localhost:8080",
    api_key="your-api-key"
)

code_reviewer = secure(
    name="code-reviewer",
    capabilities=["file:read", "api:call"],
    agent_type=AgentType.CLAUDE,
    aim_url="http://localhost:8080",
    api_key="your-api-key"
)

print(f"Data Processor: {data_processor.agent_id}")
print(f"Code Reviewer: {code_reviewer.agent_id}")

# Manual client for existing agents
client = AIMClient(
    agent_id="aim_9c4b2e1f",
    aim_url="http://localhost:8080",
    api_key="your-api-key"
)
```

Expected output:

```
Data Processor: aim_9c4b2e1f
Code Reviewer: aim_2d8f5a3c
```

### Java

The Java SDK is built from source (`cd sdk/java && mvn install`), which places this dependency in your local Maven repository:

```xml
<dependency>
    <groupId>org.opena2a</groupId>
    <artifactId>aim-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

```java
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import java.util.List;

// Builder pattern for manual client
AIMClient client = AIMClient.builder()
    .baseUrl("http://localhost:8080")
    .apiKey("your-api-key")
    .build();

// One-line registration
var dataProcessor = AIMClient.secure("data-processor",
    List.of("db:read", "api:call"),
    AgentType.LANGCHAIN);

var codeReviewer = AIMClient.secure("code-reviewer",
    List.of("file:read", "api:call"),
    AgentType.CLAUDE);

System.out.println("Data Processor: " + dataProcessor.getAgentId());
System.out.println("Code Reviewer: " + codeReviewer.getAgentId());
```

Expected output:

```
Data Processor: aim_9c4b2e1f
Code Reviewer: aim_2d8f5a3c
```

When the `serverUrl` (TypeScript), `aim_url` (Python), or `baseUrl` (Java) parameter is set, identities and audit events are routed through the AIM Server and stored in the central PostgreSQL database. The SDKs handle authentication and local key caching automatically.

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
| Service tokens | Not available | OAuth 2.0 token endpoint |

## What You Now Have

- Centralized identity management for all agents in your organization
- PostgreSQL-backed audit log queryable via REST API
- OAuth 2.0 token endpoint for machine-to-machine authentication
- A dashboard for monitoring trust scores and audit events across the fleet

## Next Steps

- [Enforce capabilities](enforce-capabilities.md) -- define policies for each agent in the fleet
- [Embed in my app](embed-in-my-app.md) -- connect your custom agents to the server programmatically
- [Deployment guide](../../infrastructure/DEPLOYMENT.md) -- production deployment on AWS, Azure, GCP, and Kubernetes
