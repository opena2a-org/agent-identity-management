# 📦 Installation Guide

Complete installation guide for Agent Identity Management platform.

## Prerequisites

### Required Software

- **Docker** 24.0+ and **Docker Compose** 2.20+
- **Go** 1.25+ (for backend development)
- **Node.js** 20.19+ or 22.13+ and **npm** 10+ (for frontend development)
- **PostgreSQL** 16+ (or use Docker)
- **Redis** 7+ (or use Docker)

### System Requirements

**Minimum (Development):**
- CPU: 4 cores
- RAM: 8 GB
- Disk: 20 GB

**Recommended (Production):**
- CPU: 8+ cores
- RAM: 16+ GB
- Disk: 100+ GB SSD

## Quick Start (Docker Compose)

### 1. Clone Repository

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
```

### 2. Configure Environment

```bash
# Copy example environment file
cp .env.example .env

# Edit configuration
nano .env  # or use your preferred editor
```

Required environment variables:

```bash
# JWT Secret (generate with: openssl rand -hex 32)
JWT_SECRET=your-super-secret-jwt-key-min-32-chars

# Key Vault master key, encrypts agent private keys (generate with: openssl rand -base64 32)
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key

# Database (defaults work for Docker Compose)
POSTGRES_PASSWORD=postgres  # Change for production!
```

### 3. Start Services

```bash
# Start all infrastructure services
docker compose up -d

# Verify all services are running
docker compose ps
```

Expected output:
```
NAME                    STATUS              PORTS
identity-postgres       Up 30 seconds       0.0.0.0:5432->5432/tcp
identity-redis          Up 30 seconds       0.0.0.0:6379->6379/tcp
identity-elasticsearch  Up 30 seconds       0.0.0.0:9200->9200/tcp
identity-minio          Up 30 seconds       0.0.0.0:9000->9000/tcp
identity-nats           Up 30 seconds       0.0.0.0:4222->4222/tcp
```

### 4. Run Database Migrations

```bash
cd apps/backend

# Install Go dependencies
go mod download

# Run migrations
go run cmd/migrate/main.go up
```

### 5. Start Backend Server

```bash
# From apps/backend directory
go run cmd/server/main.go
```

Server will start on `http://localhost:8080`

### 6. Start Frontend

```bash
# Open new terminal
cd apps/web

# Install dependencies
npm install

# Start development server
npm run dev
```

Frontend will start on `http://localhost:3000`

### 7. Verify Installation

Open your browser:
- **Frontend**: http://localhost:3000
- **API Docs**: http://localhost:8080/docs
- **Health Check**: http://localhost:8080/health

## Development Setup

### Backend Development

```bash
cd apps/backend

# Install dependencies
go mod download

# Run tests
go test ./...

# Run with hot reload (install air first)
go install github.com/cosmtrek/air@latest
air

# Build for production
go build -o bin/server cmd/server/main.go
./bin/server
```

### Frontend Development

```bash
cd apps/web

# Install dependencies
npm install

# Run development server
npm run dev

# Run tests
npm test

# Build for production
npm run build
npm start
```

### Database Management

```bash
cd apps/backend

# Create new migration (writes migrations/<next number>_migration_name.sql)
go run cmd/migrate/main.go create migration_name

# Run migrations
go run cmd/migrate/main.go up

# Check migration status (lists applied and pending migrations, changes nothing)
go run cmd/migrate/main.go status
```

Migrations have no down scripts, so the tool has no `down` command. To reverse a
change, create a migration that undoes it and run `up`. The tool refuses a
command it does not offer, such as `down`, `force` or `drop`, with exit code 2 and
without connecting to the database.

## Production Deployment

### Docker (Recommended)

```bash
# Build the backend and dashboard images from this checkout
docker compose build

# Start the services in docker-compose.yml (.env holds the secrets it requires,
# KEYVAULT_MASTER_KEY among them; set ENVIRONMENT=production there)
docker compose up -d

# View logs
docker compose logs -f
```

### Kubernetes

This repository ships no Kubernetes manifests. The commands below apply manifests you write yourself, kept in a directory shown here as `<manifests>`.

```bash
# Apply namespace
kubectl create namespace identity

# Apply secrets
kubectl create secret generic identity-secrets \
  --from-env-file=.env.production \
  -n identity

# Deploy PostgreSQL
kubectl apply -f <manifests>/postgres.yaml

# Deploy Redis
kubectl apply -f <manifests>/redis.yaml

# Deploy backend
kubectl apply -f <manifests>/backend.yaml

# Deploy frontend
kubectl apply -f <manifests>/frontend.yaml

# Check status
kubectl get pods -n identity
```

### Environment-Specific Configs

**Development** (`.env`):
```bash
ENVIRONMENT=development
LOG_LEVEL=debug
ENABLE_CORS=true
```

**Staging** (`.env.staging`):
```bash
ENVIRONMENT=staging
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key  # openssl rand -base64 32
LOG_LEVEL=info
ENABLE_CORS=true
DATABASE_SSL_MODE=require
```

**Production** (`.env.production`):
```bash
ENVIRONMENT=production
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key  # openssl rand -base64 32
LOG_LEVEL=warn
ENABLE_CORS=false
DATABASE_SSL_MODE=require
REDIS_TLS_ENABLED=true
```

`KEYVAULT_MASTER_KEY` is required in every environment except `ENVIRONMENT=development`. Agent private keys are encrypted under it, so generate a separate key for each environment once and keep the same value across restarts and upgrades. With `ENVIRONMENT=production` the server refuses to start without it. In development, a server started without it generates a new key at each start, and anything encrypted under that key cannot be decrypted after a restart. Setting `ENVIRONMENT=development` does not fix a missing key: set the key.

## SSL/TLS Setup

### Development (Self-Signed)

```bash
# Generate self-signed certificate
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Update .env
TLS_ENABLED=true
TLS_CERT_FILE=./cert.pem
TLS_KEY_FILE=./key.pem
```

### Production (Let's Encrypt)

```bash
# Install certbot
sudo apt-get install certbot

# Get certificate
sudo certbot certonly --standalone -d your-domain.com

# Certificates will be in /etc/letsencrypt/live/your-domain.com/
```

## Database Backups

### Automated Backups

```bash
# Create backup script
cat > backup.sh <<'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/identity"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

# Backup PostgreSQL
docker exec identity-postgres pg_dump -U postgres identity > \
  $BACKUP_DIR/identity_$TIMESTAMP.sql

# Compress
gzip $BACKUP_DIR/identity_$TIMESTAMP.sql

# Delete backups older than 30 days
find $BACKUP_DIR -name "*.sql.gz" -mtime +30 -delete
EOF

chmod +x backup.sh

# Add to crontab (daily at 2 AM)
(crontab -l 2>/dev/null; echo "0 2 * * * /path/to/backup.sh") | crontab -
```

### Manual Backup

```bash
# Backup
docker exec identity-postgres pg_dump -U postgres identity > backup.sql

# Restore
cat backup.sql | docker exec -i identity-postgres psql -U postgres identity
```

## Monitoring Setup

### Prometheus

The backend serves `GET /metrics` on its own listener, not on the API port
(8080), and the dashboard does not proxy it. Two settings control it:

- `METRICS_LISTEN_ADDR` (default `127.0.0.1:9464`): the listener's address. On
  a loopback address it serves without a token, so a Prometheus on the same
  host scrapes `http://127.0.0.1:9464/metrics`.
- `METRICS_AUTH_TOKEN` (32 characters or more, `openssl rand -hex 32`): required
  for any address that is not loopback, where the backend otherwise refuses to
  start. When set, the API port also serves `GET /metrics`, and both require
  `Authorization: Bearer <token>`. Keep a non-loopback listener reachable only
  from the scraper's network as well.

`docker-compose.yml` runs Prometheus with
`infrastructure/monitoring/prometheus.yml`, which scrapes `backend:9464` over the
compose network with the token from `METRICS_AUTH_TOKEN` (`.env`). Port 9464 is
not published to the host.

To run Prometheus on its own against a backend on the same host, write a scrape
job to `prometheus-aim.yml`:

```yaml
scrape_configs:
  - job_name: 'aim-backend'
    static_configs:
      - targets: ['127.0.0.1:9464']
```

and start Prometheus on the host network, so it reaches the loopback listener
(Linux; Docker Desktop needs host networking enabled):

```bash
docker run -d --network host \
  -v $(pwd)/prometheus-aim.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

For a scraper on another host, set `METRICS_LISTEN_ADDR` to an address it can
reach and `METRICS_AUTH_TOKEN` on the backend, and add to its scrape job:

```yaml
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/aim-metrics-token
```

### Grafana

```bash
# Start Grafana
docker run -d -p 3001:3000 \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  grafana/grafana

# Import dashboards from infrastructure/monitoring/grafana/dashboards/
```

## Troubleshooting

### Database Connection Issues

```bash
# Check if PostgreSQL is running
docker compose ps postgres

# View PostgreSQL logs
docker compose logs postgres

# Test connection
psql -h localhost -U postgres -d identity
```

### Redis Connection Issues

```bash
# Check if Redis is running
docker compose ps redis

# Test connection
redis-cli ping
# Expected: PONG
```

### Port Conflicts

```bash
# Check what's using port 8080
lsof -i :8080

# Kill process using port
kill -9 $(lsof -t -i:8080)
```

### Migration Failures

```bash
# List applied and pending migrations
go run cmd/migrate/main.go status

# Fix the failing migration file, then apply what is still pending
go run cmd/migrate/main.go up

# Rebuild the Docker Compose development database (caution: deletes all data;
# stop the backend first so nothing holds a connection)
docker compose exec postgres dropdb -U postgres identity
docker compose exec postgres createdb -U postgres identity
go run cmd/migrate/main.go up
```

Each migration runs in its own transaction and is recorded in
`schema_migrations` only when it commits, so a migration that fails leaves no
row behind and `up` runs it again.

## Performance Tuning

### PostgreSQL

Edit `postgresql.conf`:
```conf
# Connections
max_connections = 100
shared_buffers = 2GB
effective_cache_size = 6GB

# Write ahead log
wal_buffers = 16MB
checkpoint_completion_target = 0.9

# Query planning
random_page_cost = 1.1
effective_io_concurrency = 200
```

### Redis

Edit `redis.conf`:
```conf
# Memory
maxmemory 4gb
maxmemory-policy allkeys-lru

# Persistence
save 900 1
save 300 10
save 60 10000
```

### Backend

```bash
# Increase Go max processes
export GOMAXPROCS=8

# Increase file descriptors
ulimit -n 65536
```

## Security Hardening

### Firewall Rules

```bash
# Allow only necessary ports
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22    # SSH
sudo ufw allow 80    # HTTP
sudo ufw allow 443   # HTTPS
sudo ufw enable
```

### Fail2Ban

```bash
# Install fail2ban
sudo apt-get install fail2ban

# Configure for your application
sudo cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
sudo nano /etc/fail2ban/jail.local

# Restart
sudo systemctl restart fail2ban
```

## Next Steps

After installation:

1. **Create Admin User**: Create the first administrator as [BOOTSTRAP_GUIDE.md](./BOOTSTRAP_GUIDE.md) describes, then sign in with its email and password
2. **Register First Agent**: Use the UI or API
3. **Generate API Keys**: Create keys for agent authentication
4. **Configure Alerts**: Set up email notifications
5. **Review Audit Logs**: Familiarize yourself with the audit trail

## Getting Help

- **Documentation**: https://opena2a.org/docs
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discord**: https://discord.gg/opena2a
- **Email**: info@opena2a.org
