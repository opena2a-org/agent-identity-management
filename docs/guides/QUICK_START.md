# Quick Start Guide - Agent Identity Management

Get AIM running locally in under 5 minutes.

---

## Option 1: One-Line Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh | bash
```

This script handles everything: pulls images, generates secrets, starts services, runs migrations, and prints login credentials. Dashboard opens at [localhost:3000](http://localhost:3000), API at [localhost:8080](http://localhost:8080).

---

## Option 2: Build from Source

### Prerequisites

- Docker Desktop running
- Go 1.23+ installed
- Node.js 18+ installed

### Step 1: Clone and Start Infrastructure

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Start PostgreSQL (TimescaleDB), Redis, and other services
docker compose up -d postgres redis
```

Wait for services to be healthy:

```bash
docker compose ps
```

### Step 2: Start the Backend

```bash
cd apps/backend

# Copy the example environment file
cp .env.example .env

# Build and run (migrations run automatically on startup)
go build -o aim-server ./cmd/server && ./aim-server
```

The backend starts on port 8080. Migrations are applied automatically on first run.

**Verify:**

```bash
curl http://localhost:8080/health
# Expected: {"status":"healthy","service":"agent-identity-management","timestamp":"..."}
```

### Step 3: Start the Dashboard (Optional)

```bash
cd apps/web
npm install
npm run dev
```

Dashboard opens at [localhost:3000](http://localhost:3000).

### Step 4: Log In

The default admin account is created automatically:

- **Email:** `admin@opena2a.org`
- **Password:** `AIM2025!Secure` (you must change this on first login)

Log in via the dashboard or via API:

```bash
curl -X POST http://localhost:8080/api/v1/public/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@opena2a.org","password":"AIM2025!Secure"}'
```

The response includes an `accessToken` for authenticated API calls.

---

## Verify the Installation

### Health Check

```bash
curl http://localhost:8080/health
```

### A2A Agent Discovery

```bash
curl http://localhost:8080/.well-known/agent.json
```

### API Protection (Should Return 401)

```bash
curl http://localhost:8080/api/v1/agents
```

### Create an Agent (Authenticated)

```bash
TOKEN="<your-access-token-from-login>"

curl -X POST http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-agent",
    "type": "langchain",
    "capabilities": ["db:read", "api:call"]
  }'
```

---

## Running Tests

### Unit Tests

```bash
cd apps/backend
go test ./... -short
```

### Integration Tests

Integration tests require a running backend:

```bash
cd apps/backend
ENVIRONMENT=test go test ./tests/integration/... -v
```

---

## Common Issues

### Port Already in Use

```bash
# Check what's using the port
lsof -i :8080
lsof -i :3000

# Kill the process if needed
kill <PID>
```

### Database Connection Failed

```bash
# Check if PostgreSQL is running
docker compose ps postgres

# Restart if needed
docker compose restart postgres
```

### Backend Won't Start

Check that required environment variables are set. The backend needs at minimum:
- `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_DB` (database connection)
- `JWT_SECRET` (authentication)

See `.env.example` for all available configuration options.

---

## What's Running

| Service | Address | Description |
|---------|---------|-------------|
| PostgreSQL | localhost:5432 | TimescaleDB (PG16) |
| Redis | localhost:6379 | Cache and session storage |
| Backend | localhost:8080 | Go API server (Fiber v3) |
| Dashboard | localhost:3000 | Next.js web interface |

---

## Next Steps

- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) -- Secure your first agent with the Python or Java SDK
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) -- Connect and attest MCP servers
- [Post-Quantum Cryptography](PQC.md) -- Enable ML-DSA signatures
- [Deployment Guide](../../infrastructure/DEPLOYMENT.md) -- Production deployment on AWS, Azure, GCP, Kubernetes

---

## Getting Help

- [Documentation](https://opena2a.org/docs)
- [Discord](https://discord.gg/uRZa3KXgEn)
- [GitHub Issues](https://github.com/opena2a-org/agent-identity-management/issues)
