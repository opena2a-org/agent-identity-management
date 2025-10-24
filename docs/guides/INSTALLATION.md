# 📦 Installation Guide

Complete installation guide for Agent Identity Management platform.

## Prerequisites

### Required Software

- **Docker** 24.0+ and **Docker Compose** 2.20+
- **Go** 1.21+ (for backend development)
- **Node.js** 18+ and **pnpm** 8+ (for frontend development)
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
git clone https://github.com/opena2a/identity.git
cd identity
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
# OAuth Providers (at least one required)
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_secret

# OR Microsoft
MICROSOFT_CLIENT_ID=your_microsoft_client_id
MICROSOFT_CLIENT_SECRET=your_microsoft_secret

# OR Okta
OKTA_CLIENT_ID=your_okta_client_id
OKTA_CLIENT_SECRET=your_okta_secret
OKTA_DOMAIN=your-domain.okta.com

# JWT Secret (generate with: openssl rand -hex 32)
JWT_SECRET=your-super-secret-jwt-key-min-32-chars

# Database (defaults work for Docker Compose)
POSTGRES_PASSWORD=postgres  # Change for production!
```

### 3. Start Services

```bash
# Start all infrastructure services
docker-compose up -d

# Verify all services are running
docker-compose ps
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
pnpm install

# Start development server
pnpm dev
```

Frontend will start on `http://localhost:3000`

### 7. Verify Installation

Open your browser:
- **Frontend**: http://localhost:3000
- **API Docs**: http://localhost:8080/docs
- **Health Check**: http://localhost:8080/health

## OAuth Provider Setup

### Google OAuth

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create new project or select existing
3. Enable Google+ API
4. Create OAuth 2.0 credentials:
   - Application type: Web application
   - Authorized redirect URIs: `http://localhost:8080/api/v1/auth/callback/google`
5. Copy Client ID and Client Secret to `.env`

### Microsoft OAuth

1. Go to [Azure Portal](https://portal.azure.com)
2. Navigate to Azure Active Directory → App registrations
3. Create new registration:
   - Name: Agent Identity Management
   - Redirect URI: `http://localhost:8080/api/v1/auth/callback/microsoft`
4. Copy Application (client) ID
5. Create client secret in Certificates & secrets
6. Copy values to `.env`

### Okta OAuth

1. Go to [Okta Developer Console](https://developer.okta.com)
2. Create new app integration
3. Select OIDC - Web Application
4. Add redirect URI: `http://localhost:8080/api/v1/auth/callback/okta`
5. Copy Client ID, Client Secret, and Okta domain to `.env`

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
pnpm install

# Run development server
pnpm dev

# Run tests
pnpm test

# Build for production
pnpm build
pnpm start
```

### Database Management

```bash
cd apps/backend

# Create new migration
go run cmd/migrate/main.go create migration_name

# Run migrations
go run cmd/migrate/main.go up

# Rollback last migration
go run cmd/migrate/main.go down 1

# Check migration status
go run cmd/migrate/main.go status
```

## Production Deployment

### Docker (Recommended)

```bash
# Build images
docker-compose -f docker-compose.prod.yml build

# Start services
docker-compose -f docker-compose.prod.yml up -d

# View logs
docker-compose -f docker-compose.prod.yml logs -f
```

### Kubernetes

```bash
# Apply namespace
kubectl create namespace identity

# Apply secrets
kubectl create secret generic identity-secrets \
  --from-env-file=.env.production \
  -n identity

# Deploy PostgreSQL
kubectl apply -f infrastructure/k8s/postgres.yaml

# Deploy Redis
kubectl apply -f infrastructure/k8s/redis.yaml

# Deploy backend
kubectl apply -f infrastructure/k8s/backend.yaml

# Deploy frontend
kubectl apply -f infrastructure/k8s/frontend.yaml

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
LOG_LEVEL=info
ENABLE_CORS=true
DATABASE_SSL_MODE=require
```

**Production** (`.env.production`):
```bash
ENVIRONMENT=production
LOG_LEVEL=warn
ENABLE_CORS=false
DATABASE_SSL_MODE=require
REDIS_TLS_ENABLED=true
```

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

```bash
# Start Prometheus
docker run -d -p 9090:9090 \
  -v $(pwd)/infrastructure/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

### Grafana

```bash
# Start Grafana
docker run -d -p 3001:3000 \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  grafana/grafana

# Import dashboards from infrastructure/grafana/
```

## Troubleshooting

### Database Connection Issues

```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# View PostgreSQL logs
docker-compose logs postgres

# Test connection
psql -h localhost -U postgres -d identity
```

### Redis Connection Issues

```bash
# Check if Redis is running
docker-compose ps redis

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

### OAuth Redirect Issues

1. Verify redirect URI matches exactly in provider console
2. Check frontend URL in environment:
   ```bash
   FRONTEND_URL=http://localhost:3000
   ```
3. Ensure callback URL is correct:
   ```bash
   # For local development
   http://localhost:8080/api/v1/auth/callback/google

   # For production
   https://your-domain.com/api/v1/auth/callback/google
   ```

### Migration Failures

```bash
# Check current migration version
go run cmd/migrate/main.go status

# Force to specific version
go run cmd/migrate/main.go force 001

# Rebuild schema (caution: deletes all data)
go run cmd/migrate/main.go drop
go run cmd/migrate/main.go up
```

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

1. **Create Admin User**: Sign in with OAuth provider
2. **Register First Agent**: Use the UI or API
3. **Generate API Keys**: Create keys for agent authentication
4. **Configure Alerts**: Set up email notifications
5. **Review Audit Logs**: Familiarize yourself with the audit trail

## Getting Help

- **Documentation**: https://docs.opena2a.org
- **GitHub Issues**: https://github.com/opena2a/identity/issues
- **Discord**: https://discord.gg/opena2a
- **Email**: info@opena2a.org
