# AIM Infrastructure

This directory contains all infrastructure-related code for deploying and operating AIM (Agent Identity Management) across different environments.

## 📁 Directory Structure

```
infrastructure/
├── deployment/           # Cloud deployment scripts and configs
│   ├── aws/             # AWS deployment (ECS, RDS, ElastiCache)
│   ├── azure/           # Azure deployment (Container Apps, PostgreSQL)
│   ├── gcp/             # GCP deployment (Cloud Run, Cloud SQL)
│   └── README.md        # Detailed deployment guide
├── docker/              # Docker build configurations
│   ├── Dockerfile.backend    # Go backend container
│   └── Dockerfile.frontend   # Next.js frontend container
├── monitoring/          # Observability stack configs
│   ├── prometheus.yml   # Metrics collection
│   ├── grafana/         # Dashboards and datasources
│   ├── loki.yml         # Log aggregation
│   └── promtail.yml     # Log shipping
└── scripts/             # Operational utilities
    ├── deploy-azure-production.sh   # Production Azure deployment
    ├── setup-oauth.sh               # OAuth provider configuration
    └── verify-migrations.sh         # Database migration verification
```

## 🚀 Quick Start

### Local Development (Docker Compose)

```bash
# From project root
docker compose up -d
```

### Cloud Deployment

Choose your cloud provider:

- **AWS**: [deployment/aws/README.md](./deployment/aws/)
- **Azure**: [deployment/azure/README.md](./deployment/azure/)
- **GCP**: [deployment/gcp/README.md](./deployment/gcp/)

See [deployment/README.md](./deployment/README.md) for comprehensive deployment guide.

## 📦 Docker Images

### Backend (Go + Fiber v3)

```bash
# Build
docker build -f infrastructure/docker/Dockerfile.backend -t aim-backend:latest .

# Run
docker run -p 8080:8080 \
  -e DATABASE_URL=postgresql://... \
  -e JWT_SECRET=your-secret \
  aim-backend:latest
```

### Frontend (Next.js 15)

```bash
# Build
docker build -f infrastructure/docker/Dockerfile.frontend -t aim-frontend:latest .

# Run
docker run -p 3000:3000 \
  -e NEXT_PUBLIC_API_URL=http://localhost:8080 \
  aim-frontend:latest
```

## 📊 Monitoring Stack

### Starting Monitoring Services

```bash
# From project root
docker compose -f docker-compose.monitoring.yml up -d
```

**Services**:
- **Prometheus**: http://localhost:9090 (Metrics)
- **Grafana**: http://localhost:3001 (Dashboards)
- **Loki**: http://localhost:3100 (Logs)

### Grafana Dashboards

Pre-configured dashboards in `monitoring/grafana/provisioning/dashboards/`:
- AIM API Metrics
- Database Performance
- Agent Activity
- Security Alerts

## 🔧 Operational Scripts

### Azure Production Deployment

Deploy a complete production environment to Azure:

```bash
./infrastructure/scripts/deploy-azure-production.sh
```

**What it does**:
- Creates Azure Container Apps environment
- Deploys PostgreSQL with SSL
- Sets up Redis cache
- Configures ACR (Azure Container Registry)
- Deploys backend and frontend
- Generates secure credentials
- Runs database migrations
- Creates admin user

### OAuth Configuration

Set up OAuth providers (Google, Microsoft, Okta):

```bash
./infrastructure/scripts/setup-oauth.sh
```

### Migration Verification

Verify database migrations before deployment:

```bash
./infrastructure/scripts/verify-migrations.sh
```

## 🌍 Deployment Options

### Development

```bash
docker compose up -d
```

### Staging

```bash
# Azure
./infrastructure/scripts/deploy-azure-production.sh
```

### Production

See cloud-specific guides:
- [AWS Production](./deployment/aws/README.md)
- [Azure Production](./deployment/azure/README.md)
- [GCP Production](./deployment/gcp/README.md)

## 🔐 Security Best Practices

### Secrets Management

**Never commit secrets to git**. Use:
- **Local**: `.env` files (git-ignored)
- **Cloud**: Azure Key Vault, AWS Secrets Manager, GCP Secret Manager
- **CI/CD**: GitHub Secrets, GitLab CI Variables

### SSL/TLS

All deployments require HTTPS:
- **Local**: Self-signed cert (dev only)
- **Production**: Let's Encrypt or cloud-managed certificates

### Database Security

- Enable SSL/TLS for PostgreSQL connections
- Use strong passwords (32+ characters)
- Enable encryption at rest
- Configure network isolation (VPC/VNet)

## 📝 Environment Variables

### Backend

```env
DATABASE_URL=postgresql://user:password@host:5432/aim
REDIS_URL=redis://host:6379
JWT_SECRET=your-jwt-secret-here
CORS_ORIGINS=http://localhost:3000
PORT=8080
ENVIRONMENT=production
```

### Frontend

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_ENVIRONMENT=production
```

See [deployment/README.md](./deployment/README.md) for complete configuration reference.

## 🐛 Troubleshooting

### Container fails to start

```bash
# Check logs
docker logs aim-backend
docker logs aim-frontend

# Check environment variables
docker exec aim-backend env

# Verify database connectivity
docker exec aim-backend pg_isready -h db-host
```

### Database migration issues

```bash
# Verify migrations
./infrastructure/scripts/verify-migrations.sh

# Check migration status
docker exec aim-backend go run cmd/migrate/main.go status
```

### Network connectivity issues

```bash
# Test API connectivity
curl http://localhost:8080/health

# Test database connectivity
psql $DATABASE_URL -c "SELECT 1"

# Test Redis connectivity
redis-cli -u $REDIS_URL ping
```

## 🤝 Contributing

When adding new infrastructure:

1. **Deployment scripts** → `deployment/{provider}/`
2. **Docker configs** → `docker/`
3. **Monitoring configs** → `monitoring/`
4. **Operational scripts** → `scripts/`
5. Update this README with usage instructions

## 📚 Additional Resources

- [Main README](../README.md) - Project overview
- [Deployment Guide](./deployment/README.md) - Comprehensive deployment instructions
- [Docker Compose](../docker-compose.yml) - Local development setup
- [Kubernetes](../k8s/) - Kubernetes manifests

---

**Last Updated**: October 2024
