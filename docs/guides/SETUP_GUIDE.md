# Agent Identity Management - Setup Guide

This guide will help you get the Agent Identity Management platform running locally.

## Prerequisites

- **Go 1.25+** - [Install Go](https://go.dev/doc/install)
- **Node.js 20.19+ or 22.13+** - [Install Node.js](https://nodejs.org/)
- **Docker & Docker Compose** - [Install Docker](https://docs.docker.com/get-docker/)
- **Git** - For cloning the repository

## Quick Start

### 1. Clone the Repository

```bash
git clone <repository-url>
cd agent-identity-management
```

### 2. Start Infrastructure Services

Start PostgreSQL and Redis using Docker Compose:

```bash
docker compose up -d postgres redis
```

Wait for services to be healthy:

```bash
docker compose ps
```

You should see both `aim-postgres` and `aim-redis` with status `healthy`.

### 3. Configure Backend

The backend `.env` file has been created with development defaults:

```bash
cd apps/backend
cat .env
```

### 4. Run Database Migrations

```bash
cd apps/backend
go run cmd/migrate/main.go up
```

This will create all necessary database tables and indexes.

### 5. Start Backend Server

```bash
cd apps/backend
go run cmd/server/main.go
```

The backend will start on `http://localhost:8080`

### 6. Start Frontend

In a new terminal:

```bash
cd apps/web
npm install
npm run dev
```

The frontend will start on `http://localhost:3000`

## Environment Variables

### Backend (.env)

Located at `apps/backend/.env`:

```bash
# Required - Change in production!
JWT_SECRET=dev-secret-key-for-local-development-change-in-production-32chars-min

# Database (matches Docker Compose)
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=identity

# Redis (matches Docker Compose)
REDIS_HOST=localhost
REDIS_PORT=6379
```

### Frontend (.env.local)

Located at `apps/web/.env.local`:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Database Schema

The migrations will create the following tables:

- `organizations` - Multi-tenant organizations
- `users` - Platform users
- `agents` - Registered AI agents and MCP servers
- `api_keys` - API keys for agent authentication
- `trust_scores` - Historical trust score calculations
- `audit_logs` - Comprehensive audit trail (TimescaleDB hypertable)
- `alerts` - System alerts and notifications

## Testing the Installation

### 1. Check Backend Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"service":"agent-identity-management","status":"healthy","time":"2026-03-16T14:00:00.123456789Z"}
```

`time` is the server's clock in UTC when it answered.

### 2. Test Database Connection

```bash
docker exec -it aim-postgres psql -U postgres -d identity -c "\\dt"
```

You should see all the database tables listed.

### 3. Test Redis Connection

```bash
docker exec -it aim-redis redis-cli ping
```

Expected response: `PONG`

### 4. Access Frontend

Open browser to `http://localhost:3000`

You should see the sign-in page, which asks for an email and password.

## Common Issues

### Port Already in Use

If ports 5432, 6379, 8080, or 3000 are already in use:

**PostgreSQL (5432)**:
```bash
# Check what's using the port
lsof -i :5432
# Kill the process or change POSTGRES_PORT in docker-compose.yml and .env
```

**Redis (6379)**:
```bash
# Check what's using the port
lsof -i :6379
# Kill the process or change REDIS_PORT
```

### Database Connection Errors

1. Ensure Docker containers are running:
   ```bash
   docker compose ps
   ```

2. Check container logs:
   ```bash
   docker compose logs postgres
   ```

3. Verify credentials match between `.env` and `docker-compose.yml`

### Migration Errors

If migrations fail:

1. Reset database:
   ```bash
   docker compose down -v  # Warning: deletes all data
   docker compose up -d postgres redis
   ```

2. Run migrations again:
   ```bash
   cd apps/backend
   go run cmd/migrate/main.go up
   ```

## Development Workflow

### Backend Development

```bash
cd apps/backend

# Run tests
go test ./...

# Build
go build -o bin/server ./cmd/server

# Run with hot reload (install air first)
go install github.com/air-verse/air@latest
air
```

### Frontend Development

```bash
cd apps/web

# Run tests
npm test

# Type check
npm run type-check

# Lint
npm run lint

# Build for production
npm run build
```

## API Documentation

Once the backend is running, API endpoints are available at:

- **Base URL**: `http://localhost:8080/api/v1`
- **Health**: `GET /health`
- **Auth**: `POST /auth/login/local` (email and password)
- **Agents**: `GET/POST/PUT/DELETE /agents`
- **API Keys**: `GET/POST /api-keys`
- **Trust Scores**: `POST /trust-scores/:agentId/calculate`
- **Admin**: `GET /admin/users`, `/admin/audit-logs`, `/admin/alerts`
- **Compliance**: `POST /compliance/reports`, `GET /compliance/status`

## Next Steps

1. **Create an Administrator** - Create the first administrator as [BOOTSTRAP_GUIDE.md](./BOOTSTRAP_GUIDE.md) describes, then sign in with its email and password
2. **Create Test Data** - Register some agents through the UI
3. **Explore Features** - Try trust score calculation, audit logs, compliance reports
4. **Read Documentation** - Check PROJECT_STATUS.md and PROGRESS_SUMMARY.md
5. **Customize** - Modify trust score factors, add custom alerts, etc.

## Production Deployment

For production deployment:

1. Generate strong JWT secret (32+ characters)
2. Set `KEYVAULT_MASTER_KEY` to a key from `openssl rand -base64 32` and keep the same value across restarts. Agent private keys are encrypted under it, and the server refuses to start in production without it. Setting `ENVIRONMENT=development` does not fix a missing key.
3. Enable PostgreSQL SSL mode
4. Set strong database passwords
5. Configure Redis password
6. Set `ENVIRONMENT=production`
7. Use proper secrets management (AWS Secrets Manager, Vault, etc.)
8. Enable rate limiting
9. Configure monitoring and logging
10. Set up backups for PostgreSQL

## Support

For issues or questions:
- Check PROGRESS_SUMMARY.md for project status
- Review BACKEND_COMPILATION_FIXES_COMPLETE.md for backend details
- Open an issue in the repository

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Frontend (Next.js)                   │
│                    http://localhost:3000                 │
└────────────────────┬────────────────────────────────────┘
                     │ HTTP/REST API
┌────────────────────▼────────────────────────────────────┐
│                  Backend (Go/Fiber)                      │
│                 http://localhost:8080                    │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Handlers → Services → Repositories              │  │
│  └──────────────────────────────────────────────────┘  │
└──────┬──────────────────────────┬──────────────────────┘
       │                           │
┌──────▼─────────┐        ┌───────▼──────┐
│   PostgreSQL   │        │    Redis     │
│   (TimescaleDB)│        │   (Cache)    │
│   port: 5432   │        │  port: 6379  │
└────────────────┘        └──────────────┘
```

## License

[Your License Here]
