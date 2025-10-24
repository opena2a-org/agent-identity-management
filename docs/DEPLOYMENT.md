# 🚀 AIM Deployment Guide

Complete guide for deploying AIM (Agent Identity Management) in development, staging, and production environments.

---

## 📋 Table of Contents

1. [Prerequisites](#-prerequisites)
2. [Quick Start](#-quick-start)
3. [Deployment Modes](#-deployment-modes)
4. [Environment Configuration](#-environment-configuration)
5. [OAuth Setup](#-oauth-setup)
6. [Production Deployment](#-production-deployment)
7. [Kubernetes Deployment](#-kubernetes-deployment)
8. [Monitoring & Observability](#-monitoring--observability)
9. [Troubleshooting](#-troubleshooting)

---

## ✅ Prerequisites

### Required Software

#### Development Mode
- **Docker** 20.10+ ([Install](https://docs.docker.com/get-docker/))
- **Docker Compose** 2.0+ (included with Docker Desktop)
- **Go** 1.23+ ([Install](https://golang.org/dl/))
- **Node.js** 22+ ([Install](https://nodejs.org/))
- **Python** 3.8+ ([Install](https://python.org/))

#### Production Mode
- **Docker** 20.10+ ([Install](https://docs.docker.com/get-docker/))
- **Docker Compose** 2.0+ (included with Docker Desktop)
- **Kubernetes** 1.28+ (optional, for K8s deployment)
- **kubectl** ([Install](https://kubernetes.io/docs/tasks/tools/))

### System Requirements

#### Minimum (Development)
- **CPU**: 4 cores
- **RAM**: 8 GB
- **Disk**: 20 GB free space

#### Recommended (Production)
- **CPU**: 8+ cores
- **RAM**: 16+ GB
- **Disk**: 100+ GB SSD

---

## 🚀 Quick Start

### Development Deployment (Simplest)

```bash
# 1. Clone repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# 2. Deploy infrastructure only
./deploy.sh development

# 3. Start backend (in new terminal)
cd apps/backend
export $(grep -v '^#' ../../.env | xargs)
go run cmd/server/main.go

# 4. Start frontend (in new terminal)
cd apps/web
npm install
npm run dev

# 5. Open browser
open http://localhost:3000
```

### Production Deployment (Docker Compose)

```bash
# 1. Clone repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# 2. Deploy everything (infrastructure + backend + frontend)
./deploy.sh production

# 3. Access application
open http://localhost:3000
```

---

## 🔀 Deployment Modes

### Development Mode

**Use Case**: Local development, testing, debugging

```bash
./deploy.sh development
```

**What it does:**
- ✅ Starts infrastructure services (PostgreSQL, Redis, Elasticsearch, etc.)
- ✅ Creates `.env` file with development settings
- ✅ Runs database migrations
- ❌ Does NOT start backend/frontend containers (run manually)

**Services Started:**
- PostgreSQL (port 5432)
- Redis (port 6379)
- Elasticsearch (port 9200)
- MinIO (ports 9000, 9001)
- NATS (port 4222)
- Prometheus (port 9090)
- Grafana (port 3003)
- Loki (port 3100)

**Backend & Frontend:**
```bash
# Terminal 1: Backend
cd apps/backend
export $(grep -v '^#' ../../.env | xargs)
go run cmd/server/main.go

# Terminal 2: Frontend
cd apps/web
npm install
npm run dev
```

### Production Mode

**Use Case**: Production deployment with Docker Compose

```bash
./deploy.sh production
```

**What it does:**
- ✅ Starts all infrastructure services
- ✅ Builds backend Docker image
- ✅ Builds frontend Docker image
- ✅ Starts backend container (port 8080)
- ✅ Starts frontend container (port 3000)
- ✅ Creates `.env` file with production settings
- ✅ Runs database migrations
- ✅ Performs health checks

**All Services Running:**
- Frontend (port 3000)
- Backend (port 8080)
- All infrastructure services (PostgreSQL, Redis, etc.)

### Clean Mode

**Use Case**: Reset deployment, remove all containers and volumes

```bash
./deploy.sh clean
```

**What it does:**
- ❌ Stops all Docker containers
- ❌ Removes all containers
- ❌ Removes all volumes (⚠️ **DATA LOSS!**)

---

## ⚙️ Environment Configuration

### .env File Structure

The deployment script automatically creates a `.env` file. Here's a complete reference:

```bash
################################################################################
# Application Settings
################################################################################

# Server configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
ENVIRONMENT=production  # development | production | testing

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_EXPIRATION=24h

# CORS Configuration
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

################################################################################
# Database Configuration
################################################################################

# PostgreSQL
DATABASE_URL=postgresql://postgres:your_password@postgres:5432/identity?sslmode=require
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=identity

# Redis
REDIS_URL=redis://redis:6379/0

# Elasticsearch
ELASTICSEARCH_URL=http://elasticsearch:9200

################################################################################
# Object Storage (MinIO)
################################################################################

MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=your_minio_access_key
MINIO_SECRET_KEY=your_minio_secret_key
MINIO_USE_SSL=true  # Enable in production
MINIO_BUCKET=aim-storage

################################################################################
# Message Queue (NATS)
################################################################################

NATS_URL=nats://nats:4222

################################################################################
# OAuth Providers
################################################################################

# Google OAuth
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback

# Microsoft OAuth
MICROSOFT_CLIENT_ID=your-microsoft-client-id
MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret
MICROSOFT_REDIRECT_URL=https://yourdomain.com/auth/microsoft/callback
MICROSOFT_TENANT_ID=common  # or your-tenant-id

# Okta OAuth
OKTA_CLIENT_ID=your-okta-client-id
OKTA_CLIENT_SECRET=your-okta-client-secret
OKTA_REDIRECT_URL=https://yourdomain.com/auth/okta/callback
OKTA_DOMAIN=your-domain.okta.com

################################################################################
# Monitoring & Observability
################################################################################

# Prometheus
PROMETHEUS_ENABLED=true

# Grafana
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=change-me-in-production

# Loki
LOKI_ENABLED=true

################################################################################
# Security Settings
################################################################################

# API Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=100

# Trust Score Thresholds
TRUST_SCORE_MIN_LOW=50.0
TRUST_SCORE_MIN_MEDIUM=70.0
TRUST_SCORE_MIN_HIGH=85.0

################################################################################
# Feature Flags
################################################################################

ENABLE_AUDIT_LOGGING=true
ENABLE_WEBHOOKS=true
ENABLE_ALERTS=true
ENABLE_COMPLIANCE_REPORTS=true
```

### Secrets Management

**⚠️ NEVER commit `.env` to version control!**

#### Development
- Use auto-generated `.env` file
- Store secrets in `.env` locally

#### Production
- **Option 1**: Environment variables (recommended)
  ```bash
  export JWT_SECRET="$(openssl rand -hex 32)"
  export DATABASE_URL="postgresql://..."
  ```

- **Option 2**: Docker secrets
  ```bash
  echo "my-jwt-secret" | docker secret create jwt_secret -
  ```

- **Option 3**: Kubernetes secrets
  ```bash
  kubectl create secret generic aim-secrets \
    --from-literal=jwt-secret=your-secret \
    --from-literal=database-url=postgresql://...
  ```

- **Option 4**: HashiCorp Vault (enterprise)
  ```bash
  vault kv put secret/aim/production \
    jwt_secret=your-secret \
    database_url=postgresql://...
  ```

---

## 🔐 OAuth Setup

Complete setup guides for Google, Microsoft, and Okta SSO integration.

### Google OAuth Setup

#### 1. Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click **Create Project**
3. Enter project name: `aim-production`
4. Click **Create**

#### 2. Enable APIs

1. Navigate to **APIs & Services > Library**
2. Search for **Google+ API**
3. Click **Enable**

#### 3. Configure OAuth Consent Screen

1. Navigate to **APIs & Services > OAuth consent screen**
2. Select **External** (for public access) or **Internal** (for workspace)
3. Fill in application details:
   - **App name**: `AIM - Agent Identity Management`
   - **User support email**: `support@yourdomain.com`
   - **Developer contact email**: `dev@yourdomain.com`
4. Add scopes:
   - `email`
   - `profile`
   - `openid`
5. Click **Save and Continue**

#### 4. Create OAuth Credentials

1. Navigate to **APIs & Services > Credentials**
2. Click **Create Credentials > OAuth client ID**
3. Select **Application type**: Web application
4. Enter **Name**: `AIM Production`
5. Add **Authorized redirect URIs**:
   - Development: `http://localhost:8080/auth/google/callback`
   - Production: `https://yourdomain.com/auth/google/callback`
6. Click **Create**
7. Copy **Client ID** and **Client Secret**

#### 5. Update .env File

```bash
GOOGLE_CLIENT_ID=123456789-abcdefghijklmnop.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-abcdefghijklmnopqrstuvwx
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback
```

#### 6. Test Integration

```bash
# Visit your AIM instance
open https://yourdomain.com

# Click "Sign in with Google"
# Should redirect to Google OAuth consent screen
# After approval, redirects back to AIM
```

---

### Microsoft OAuth Setup

#### 1. Register Application in Azure AD

1. Go to [Azure Portal](https://portal.azure.com/)
2. Navigate to **Azure Active Directory**
3. Click **App registrations > New registration**
4. Fill in details:
   - **Name**: `AIM - Agent Identity Management`
   - **Supported account types**:
     - **Single tenant** (your org only)
     - **Multitenant** (any Azure AD org)
     - **Multitenant + personal** (anyone with Microsoft account)
   - **Redirect URI**:
     - Platform: **Web**
     - URI: `https://yourdomain.com/auth/microsoft/callback`
5. Click **Register**

#### 2. Create Client Secret

1. In your app, navigate to **Certificates & secrets**
2. Click **New client secret**
3. Enter **Description**: `AIM Production Secret`
4. Select **Expires**: 24 months (or custom)
5. Click **Add**
6. **Copy the secret value immediately** (shown only once!)

#### 3. Configure API Permissions

1. Navigate to **API permissions**
2. Click **Add a permission > Microsoft Graph**
3. Select **Delegated permissions**
4. Add permissions:
   - `User.Read`
   - `email`
   - `openid`
   - `profile`
5. Click **Add permissions**
6. Click **Grant admin consent** (if you're admin)

#### 4. Get Tenant ID

1. Navigate to **Overview**
2. Copy **Directory (tenant) ID**

#### 5. Update .env File

```bash
MICROSOFT_CLIENT_ID=12345678-1234-1234-1234-123456789abc
MICROSOFT_CLIENT_SECRET=abcdefghijklmnopqrstuvwxyz123456
MICROSOFT_REDIRECT_URL=https://yourdomain.com/auth/microsoft/callback
MICROSOFT_TENANT_ID=87654321-4321-4321-4321-cba987654321
# Or use "common" for multi-tenant
```

#### 6. Test Integration

```bash
# Visit your AIM instance
open https://yourdomain.com

# Click "Sign in with Microsoft"
# Should redirect to Microsoft login
# After approval, redirects back to AIM
```

---

### Okta OAuth Setup

#### 1. Create Okta Account

1. Go to [Okta Developer](https://developer.okta.com/)
2. Click **Sign Up** (free developer account)
3. Verify your email
4. Access your Okta domain: `https://dev-123456.okta.com`

#### 2. Create Application Integration

1. Log in to Okta Admin Console
2. Navigate to **Applications > Applications**
3. Click **Create App Integration**
4. Select **OIDC - OpenID Connect**
5. Select **Web Application**
6. Click **Next**

#### 3. Configure Application

1. Fill in details:
   - **App integration name**: `AIM - Agent Identity Management`
   - **Logo** (optional): Upload your logo
   - **Grant type**: Authorization Code
   - **Sign-in redirect URIs**:
     - Development: `http://localhost:8080/auth/okta/callback`
     - Production: `https://yourdomain.com/auth/okta/callback`
   - **Sign-out redirect URIs**:
     - Development: `http://localhost:3000`
     - Production: `https://yourdomain.com`
2. **Assignments**:
   - Select **Allow everyone in your organization to access**
   - Or **Limit access to selected groups**
3. Click **Save**

#### 4. Get Client Credentials

1. In your app, go to **General** tab
2. Copy **Client ID**
3. Copy **Client Secret**

#### 5. Get Okta Domain

1. Look at your Okta admin URL
2. Your domain is the subdomain: `dev-123456.okta.com`

#### 6. Update .env File

```bash
OKTA_CLIENT_ID=0oa1234567890abcdef
OKTA_CLIENT_SECRET=abcdefghijklmnopqrstuvwxyz1234567890ABCD
OKTA_REDIRECT_URL=https://yourdomain.com/auth/okta/callback
OKTA_DOMAIN=dev-123456.okta.com
```

#### 7. Configure Authorization Server (Optional)

For custom scopes and claims:

1. Navigate to **Security > API**
2. Select **default** authorization server
3. Click **Scopes** tab
4. Add custom scopes if needed

#### 8. Test Integration

```bash
# Visit your AIM instance
open https://yourdomain.com

# Click "Sign in with Okta"
# Should redirect to Okta login
# After approval, redirects back to AIM
```

---

## 🏭 Production Deployment

### Docker Compose (Simple)

#### 1. Prepare Server

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

#### 2. Clone & Configure

```bash
# Clone repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# Create production .env
cp .env.example .env
nano .env  # Edit with production values
```

#### 3. Deploy

```bash
# Deploy in production mode
./deploy.sh production

# Check status
docker compose ps

# View logs
docker compose logs -f
```

#### 4. Enable HTTPS (Let's Encrypt)

```bash
# Install Certbot
sudo apt install certbot

# Get certificate
sudo certbot certonly --standalone -d yourdomain.com

# Configure Nginx (create docker-compose.override.yml)
cat > docker-compose.override.yml << EOF
version: '3.9'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./infrastructure/nginx/nginx.conf:/etc/nginx/nginx.conf
      - /etc/letsencrypt:/etc/letsencrypt:ro
    depends_on:
      - backend
      - frontend
    networks:
      - aim-network
EOF

# Restart
docker compose down
docker compose up -d
```

---

## ☸️ Kubernetes Deployment

### Prerequisites

- **Kubernetes cluster** (EKS, GKE, AKS, or self-hosted)
- **kubectl** configured
- **Helm** 3+ (optional)

### Deploy with kubectl

#### 1. Create Namespace

```bash
kubectl create namespace aim-production
```

#### 2. Create Secrets

```bash
# Create from .env file
kubectl create secret generic aim-secrets \
  --from-env-file=.env \
  --namespace=aim-production

# Or create manually
kubectl create secret generic aim-secrets \
  --from-literal=jwt-secret="$(openssl rand -hex 32)" \
  --from-literal=database-url="postgresql://..." \
  --namespace=aim-production
```

#### 3. Deploy Infrastructure

```bash
# PostgreSQL (using Bitnami chart)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install postgres bitnami/postgresql \
  --namespace aim-production \
  --set auth.postgresPassword=your_password \
  --set primary.persistence.size=100Gi

# Redis
helm install redis bitnami/redis \
  --namespace aim-production \
  --set auth.enabled=false

# Or deploy from manifests
kubectl apply -f infrastructure/k8s/postgres.yaml
kubectl apply -f infrastructure/k8s/redis.yaml
kubectl apply -f infrastructure/k8s/elasticsearch.yaml
```

#### 4. Deploy AIM

```bash
# Apply Kubernetes manifests
kubectl apply -f infrastructure/k8s/backend-deployment.yaml
kubectl apply -f infrastructure/k8s/frontend-deployment.yaml
kubectl apply -f infrastructure/k8s/ingress.yaml

# Check status
kubectl get pods -n aim-production
kubectl get services -n aim-production
kubectl get ingress -n aim-production
```

#### 5. Configure Ingress

```yaml
# infrastructure/k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: aim-ingress
  namespace: aim-production
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - yourdomain.com
    secretName: aim-tls
  rules:
  - host: yourdomain.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: aim-backend
            port:
              number: 8080
      - path: /
        pathType: Prefix
        backend:
          service:
            name: aim-frontend
            port:
              number: 3000
```

### Horizontal Pod Autoscaling

```yaml
# infrastructure/k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: aim-backend-hpa
  namespace: aim-production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: aim-backend
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## 📊 Monitoring & Observability

### Accessing Dashboards

After deployment, access monitoring tools:

- **Grafana**: http://localhost:3003 (admin/admin)
- **Prometheus**: http://localhost:9090
- **MinIO Console**: http://localhost:9001

### Grafana Setup

1. Open http://localhost:3003
2. Login with `admin` / `admin`
3. Add Prometheus data source:
   - URL: `http://prometheus:9090`
4. Add Loki data source:
   - URL: `http://loki:3100`
5. Import AIM dashboards from `infrastructure/monitoring/grafana/dashboards/`

### Key Metrics to Monitor

- **Backend API**:
  - Request rate (requests/sec)
  - Response time (p50, p95, p99)
  - Error rate (%)
  - Active connections

- **Database**:
  - Connection pool usage
  - Query latency
  - Cache hit rate (Redis)

- **Infrastructure**:
  - CPU usage
  - Memory usage
  - Disk I/O
  - Network I/O

---

## 🐛 Troubleshooting

### Common Issues

#### Backend won't start

```bash
# Check logs
docker compose logs backend

# Common causes:
# 1. Database not ready
docker compose logs postgres

# 2. Missing environment variables
cat .env | grep -E "JWT_SECRET|DATABASE_URL"

# 3. Port already in use
lsof -i :8080
```

#### Frontend build fails

```bash
# Check Node.js version
node --version  # Should be 22+

# Clear cache and reinstall
cd apps/web
rm -rf node_modules package-lock.json .next
npm install
npm run build
```

#### Database connection errors

```bash
# Test PostgreSQL connectivity
docker compose exec postgres psql -U postgres -d identity -c "SELECT 1"

# Check DATABASE_URL format
# Correct: postgresql://user:pass@host:port/dbname?sslmode=disable
# Wrong: postgres://... (use postgresql://)
```

#### OAuth redirect errors

```bash
# Verify redirect URLs match EXACTLY
# Google Console: http://localhost:8080/auth/google/callback
# .env file: GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Check OAuth credentials
echo $GOOGLE_CLIENT_ID
echo $GOOGLE_CLIENT_SECRET
```

### Health Checks

```bash
# Backend API
curl http://localhost:8080/health

# PostgreSQL
docker compose exec postgres pg_isready -U postgres

# Redis
docker compose exec redis redis-cli ping

# Elasticsearch
curl http://localhost:9200/_cluster/health
```

### Reset Deployment

```bash
# Clean everything and start fresh
./deploy.sh clean
rm .env
./deploy.sh development
```

---

## 📞 Support

- **Documentation**: https://docs.opena2a.org
- **GitHub Issues**: https://github.com/opena2a/agent-identity-management/issues
- **Discord**: https://discord.gg/opena2a
- **Email**: info@opena2a.org

---

**🛡️ Happy Deploying with AIM!**
