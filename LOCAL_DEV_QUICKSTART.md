# 🚀 AIM Local Development - Quick Start

## Prerequisites
- Docker & Docker Compose installed
- Ports available: 3000, 8080, 5432, 6379, 9200, 9000, 4222, 9090, 3003, 3100

## Start AIM Locally (Full Stack)

```bash
# Navigate to project root
cd /Users/decimai/workspace/agent-identity-management

# Start all services (backend + frontend + infrastructure)
docker-compose up -d

# Watch logs (optional)
docker-compose logs -f backend frontend
```

## Access Points

### Application
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health

### Default Login Credentials
- **Email**: `admin@opena2a.org`
- **Password**: `AIM2025!Secure`

### Infrastructure Services
- **PostgreSQL**: localhost:5432 (postgres/postgres/identity)
- **Redis**: localhost:6379
- **Elasticsearch**: http://localhost:9200
- **MinIO Console**: http://localhost:9001 (aim_minio_user/aim_minio_password_dev)
- **NATS**: localhost:4222
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3003 (admin/admin)
- **Loki**: http://localhost:3100

## Alternative: Run Frontend Only (Development Mode)

If backend is already running or you want faster frontend reloads:

```bash
# Start only infrastructure services
docker-compose up -d postgres redis elasticsearch minio nats

# Navigate to frontend directory
cd apps/web

# Install dependencies (if needed)
npm install

# Run frontend in dev mode
npm run dev
```

Then access frontend at http://localhost:3000

## Testing Sprint 1 (Tags Management)

### Pages to Test
1. **Tags Management**: http://localhost:3000/dashboard/tags
   - Create a new tag
   - Edit existing tag
   - Delete tag
   - Search and filter tags

2. **Agent Tags**: http://localhost:3000/dashboard/agents/[id]
   - Click on any agent
   - Go to "Tags" tab
   - Add tags to agent
   - Remove tags from agent
   - Check tag suggestions

3. **MCP Tags**: http://localhost:3000/dashboard/mcp/[id]
   - Click on any MCP server
   - Go to "Tags" tab
   - Add tags to MCP
   - Remove tags from MCP
   - Check tag suggestions

### Chrome DevTools Verification
1. Open Chrome DevTools (F12)
2. Go to Network tab
3. Filter by "Fetch/XHR"
4. Perform actions and verify these endpoints are called:
   - `POST /api/v1/tags` - Create tag
   - `GET /api/v1/tags` - List tags
   - `PUT /api/v1/tags/:id` - Update tag (if implemented)
   - `DELETE /api/v1/tags/:id` - Delete tag
   - `POST /api/v1/agents/:id/tags` - Add tags to agent
   - `DELETE /api/v1/agents/:id/tags/:tagId` - Remove tag from agent
   - `GET /api/v1/agents/:id/tags` - Get agent tags
   - `GET /api/v1/agents/:id/tags/suggestions` - Get tag suggestions
   - `POST /api/v1/mcp-servers/:id/tags` - Add tags to MCP
   - `DELETE /api/v1/mcp-servers/:id/tags/:tagId` - Remove tag from MCP
   - `GET /api/v1/mcp-servers/:id/tags/suggestions` - Get MCP tag suggestions

## Stop Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (clean slate)
docker-compose down -v
```

## Troubleshooting

### Frontend can't connect to backend
- Check backend is running: `docker-compose ps backend`
- Check backend health: `curl http://localhost:8080/health`
- Check backend logs: `docker-compose logs backend`

### Database connection issues
- Check postgres is running: `docker-compose ps postgres`
- Check postgres logs: `docker-compose logs postgres`
- Verify database exists: `docker-compose exec postgres psql -U postgres -d identity -c "\dt"`

### Port already in use
```bash
# Find process using port (e.g., 3000)
lsof -i :3000

# Kill process if needed
kill -9 <PID>
```

### Fresh start
```bash
# Complete reset
docker-compose down -v
docker-compose up -d --build
```

## Database Migrations

Migrations run automatically when backend starts. If you need to run manually:

```bash
# Connect to postgres
docker-compose exec postgres psql -U postgres -d identity

# Check migrations table
\dt migrations

# View applied migrations
SELECT * FROM schema_migrations;
```

## Next Steps

After testing Sprint 1:
- Sprint 2: Agent Lifecycle Management (suspend, reactivate, rotate credentials, trust score)
- Sprint 3: Advanced Analytics (trends, usage stats, activity timeline)
- Sprint 4: Webhooks System (create, list, test, delete webhooks)
- Sprint 5: Compliance Details (access review, data retention, alert resolution)

---

**Last Updated**: October 22, 2025 (Sprint 1 Complete)
