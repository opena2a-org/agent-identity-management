# AIM (Agent Identity Management) - Deployment Guide

> **Enterprise-Grade AI Agent Security Platform**

This guide provides comprehensive instructions for deploying AIM to various environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start (Local Development)](#quick-start-local-development)
- [Environment Configuration](#environment-configuration)
- [Deployment Options](#deployment-options)
  - [Docker Compose (Recommended for VPS)](#docker-compose)
  - [Azure Container Apps](#azure-container-apps)
  - [AWS ECS/Fargate](#aws-ecsfargate)
  - [Kubernetes (K8s)](#kubernetes-k8s)
- [Post-Deployment](#post-deployment)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Software

- **Docker** 24.0+ and **Docker Compose** 2.20+
- **Go** 1.23+
- **Node.js** 20+ and **npm** 10+
- **PostgreSQL** 16+ (or use Docker)
- **Redis** 7+ (optional - for caching)

### Recommended Tools

- **Azure CLI** (for Azure deployment)
- **AWS CLI** (for AWS deployment)
- **kubectl** (for Kubernetes deployment)
- **Helm** 3+ (optional, for K8s)

---

## Quick Start (Local Development)

### 1. First-Time Setup

Run the automated setup script:

```bash
./setup.sh
```

This script will:
- Check all prerequisites
- Create `.env` file from template
- Generate JWT secret and KeyVault master key
- Install backend and frontend dependencies
- Start database services

### 2. Start Development Environment

```bash
./start-dev.sh
```

This will start:
- PostgreSQL (port 5432)
- Redis (port 6379)
- Backend (port 8080)
- Frontend (port 3000)

**Access Points:**
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- API Documentation: http://localhost:8080/swagger

### 3. Stop Services

```bash
./stop-dev.sh              # Stop all services
./stop-dev.sh --keep-db    # Stop backend/frontend, keep database
```

---

## Environment Configuration

### Master Environment File

AIM uses a **single master `.env` file** at the project root for local development:

```
agent-identity-management/
├── .env                    # Master config (DO NOT commit)
├── .env.example            # Template with placeholders
└── ...
```

### Required Variables

```bash
# Application Environment
ENVIRONMENT=production
NODE_ENV=production
LOG_LEVEL=info

# Server Configuration
APP_PORT=8080
FRONTEND_URL=https://your-domain.com

# Database (PostgreSQL)
POSTGRES_HOST=your-db-host
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=<strong-password>
POSTGRES_DB=identity
POSTGRES_SSL_MODE=require

# JWT Authentication
JWT_SECRET=<generate-with-openssl-rand-hex-32>
JWT_ACCESS_TTL=24h
JWT_REFRESH_TTL=168h

# KeyVault (for agent private keys)
KEYVAULT_MASTER_KEY=<generate-with-openssl-rand-base64-32>

# Redis (Optional - improves performance)
REDIS_HOST=your-redis-host
REDIS_PORT=6379
REDIS_PASSWORD=<redis-password>

# Frontend
NEXT_PUBLIC_API_URL=https://api.your-domain.com
NEXT_PUBLIC_APP_NAME=Agent Identity Management
```

### Generating Secrets

```bash
# JWT Secret (64 character hex string)
openssl rand -hex 32

# KeyVault Master Key (base64 encoded)
openssl rand -base64 32

# Strong password
openssl rand -base64 24
```

---

## Deployment Options

### Docker Compose

**Best for:** VPS, DigitalOcean Droplets, AWS EC2, small-to-medium deployments

#### 1. Prepare Environment

```bash
# Copy environment template
cp .env.example .env

# Edit with production values
nano .env
```

#### 2. Build Images

```bash
docker compose -f docker-compose.yml build
```

#### 3. Security Scan (Optional but Recommended)

```bash
# Install Trivy
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# Scan images
trivy image aim-backend:latest
trivy image aim-frontend:latest
```

#### 4. Deploy

```bash
# Start all services
docker compose -f docker-compose.yml up -d

# Check status
docker compose ps

# View logs
docker compose logs -f backend
docker compose logs -f frontend
```

#### 5. Verify Deployment

```bash
# Backend health check
curl http://localhost:8080/health

# Frontend access
curl http://localhost:3000
```

#### 6. Setup Reverse Proxy (Nginx)

```nginx
# /etc/nginx/sites-available/aim

server {
    listen 80;
    server_name your-domain.com;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    
    # Frontend
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # Backend API
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

### Azure Container Apps

**Best for:** Enterprise deployments, Azure-first organizations, ~$70/month for 100 agents

#### 1. Prerequisites

```bash
# Install Azure CLI
brew install azure-cli  # macOS
# or visit: https://docs.microsoft.com/cli/azure/install-azure-cli

# Login
az login
```

#### 2. Create Resource Group

```bash
RESOURCE_GROUP="aim-production"
LOCATION="eastus"

az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION
```

#### 3. Create Azure Container Registry (ACR)

```bash
ACR_NAME="aimregistry"

az acr create \
  --resource-group $RESOURCE_GROUP \
  --name $ACR_NAME \
  --sku Basic \
  --admin-enabled true
```

#### 4. Build and Push Images

```bash
# Login to ACR
az acr login --name $ACR_NAME

# Build and push backend
docker build -f infrastructure/docker/Dockerfile.backend \
  -t $ACR_NAME.azurecr.io/aim-backend:latest .
docker push $ACR_NAME.azurecr.io/aim-backend:latest

# Build and push frontend
docker build -f infrastructure/docker/Dockerfile.frontend \
  -t $ACR_NAME.azurecr.io/aim-frontend:latest .
docker push $ACR_NAME.azurecr.io/aim-frontend:latest
```

#### 5. Create PostgreSQL Database

```bash
# Flexible Server (recommended)
az postgres flexible-server create \
  --resource-group $RESOURCE_GROUP \
  --name aim-db \
  --location $LOCATION \
  --admin-user aimadmin \
  --admin-password <strong-password> \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --storage-size 32 \
  --version 16

# Create database
az postgres flexible-server db create \
  --resource-group $RESOURCE_GROUP \
  --server-name aim-db \
  --database-name identity
```

#### 6. Create Redis Cache (Optional)

```bash
az redis create \
  --resource-group $RESOURCE_GROUP \
  --name aim-cache \
  --location $LOCATION \
  --sku Basic \
  --vm-size c0 \
  --enable-non-ssl-port false
```

#### 7. Create Container App Environment

```bash
ENV_NAME="aim-env"

az containerapp env create \
  --name $ENV_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION
```

#### 8. Deploy Backend Container App

```bash
# Get ACR credentials
ACR_PASSWORD=$(az acr credential show \
  --name $ACR_NAME \
  --query "passwords[0].value" -o tsv)

# Create backend container app
az containerapp create \
  --name aim-backend \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-backend:latest \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_NAME \
  --registry-password $ACR_PASSWORD \
  --target-port 8080 \
  --ingress external \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3 \
  --env-vars \
    ENVIRONMENT=production \
    APP_PORT=8080 \
    POSTGRES_HOST=aim-db.postgres.database.azure.com \
    POSTGRES_USER=aimadmin \
    POSTGRES_PASSWORD=<password> \
    POSTGRES_DB=identity \
    JWT_SECRET=<your-jwt-secret> \
    KEYVAULT_MASTER_KEY=<your-keyvault-key>
```

#### 9. Deploy Frontend Container App

```bash
# Get backend URL
BACKEND_URL=$(az containerapp show \
  --name aim-backend \
  --resource-group $RESOURCE_GROUP \
  --query "properties.configuration.ingress.fqdn" -o tsv)

# Create frontend container app
az containerapp create \
  --name aim-frontend \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-frontend:latest \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_NAME \
  --registry-password $ACR_PASSWORD \
  --target-port 3000 \
  --ingress external \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3 \
  --env-vars \
    NODE_ENV=production \
    NEXT_PUBLIC_API_URL=https://$BACKEND_URL \
    NEXT_PUBLIC_APP_NAME="Agent Identity Management"
```

#### 10. Configure Custom Domain (Optional)

```bash
# Add custom domain
az containerapp hostname add \
  --hostname your-domain.com \
  --name aim-frontend \
  --resource-group $RESOURCE_GROUP

# Bind SSL certificate (managed)
az containerapp hostname bind \
  --hostname your-domain.com \
  --name aim-frontend \
  --resource-group $RESOURCE_GROUP \
  --validation-method CNAME
```

---

### AWS ECS/Fargate

**Best for:** AWS-first organizations, auto-scaling requirements

#### 1. Prerequisites

```bash
# Install AWS CLI
brew install awscli  # macOS

# Configure AWS CLI
aws configure
```

#### 2. Create ECR Repositories

```bash
# Backend repository
aws ecr create-repository --repository-name aim-backend

# Frontend repository
aws ecr create-repository --repository-name aim-frontend

# Get login command
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin \
  <account-id>.dkr.ecr.us-east-1.amazonaws.com
```

#### 3. Build and Push Images

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGION="us-east-1"
ECR_URL="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"

# Backend
docker build -f infrastructure/docker/Dockerfile.backend \
  -t $ECR_URL/aim-backend:latest .
docker push $ECR_URL/aim-backend:latest

# Frontend
docker build -f infrastructure/docker/Dockerfile.frontend \
  -t $ECR_URL/aim-frontend:latest .
docker push $ECR_URL/aim-frontend:latest
```

#### 4. Create RDS PostgreSQL Database

```bash
aws rds create-db-instance \
  --db-instance-identifier aim-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version 16 \
  --master-username aimadmin \
  --master-user-password <strong-password> \
  --allocated-storage 20 \
  --vpc-security-group-ids <sg-id> \
  --db-subnet-group-name <subnet-group> \
  --backup-retention-period 7 \
  --publicly-accessible false
```

#### 5. Create ECS Cluster

```bash
aws ecs create-cluster --cluster-name aim-production
```

#### 6. Create Task Definitions

See `infrastructure/aws/task-definition-backend.json` and `task-definition-frontend.json` for complete examples.

#### 7. Deploy Services

```bash
# Backend service
aws ecs create-service \
  --cluster aim-production \
  --service-name aim-backend \
  --task-definition aim-backend:1 \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[<subnet-ids>],securityGroups=[<sg-id>],assignPublicIp=ENABLED}"

# Frontend service
aws ecs create-service \
  --cluster aim-production \
  --service-name aim-frontend \
  --task-definition aim-frontend:1 \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[<subnet-ids>],securityGroups=[<sg-id>],assignPublicIp=ENABLED}"
```

---

### Kubernetes (K8s)

**Best for:** Large-scale deployments, multi-cloud, advanced orchestration

#### 1. Create Namespace

```bash
kubectl create namespace aim
```

#### 2. Create Secrets

```bash
kubectl create secret generic aim-secrets \
  --from-literal=JWT_SECRET=<your-jwt-secret> \
  --from-literal=POSTGRES_PASSWORD=<db-password> \
  --from-literal=KEYVAULT_MASTER_KEY=<keyvault-key> \
  --namespace=aim
```

#### 3. Deploy PostgreSQL (StatefulSet)

```yaml
# infrastructure/k8s/postgres.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: aim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: aim
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:16-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: identity
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: aim-secrets
              key: POSTGRES_PASSWORD
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
      volumes:
      - name: postgres-storage
        persistentVolumeClaim:
          claimName: postgres-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: aim
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
```

#### 4. Deploy Backend

```yaml
# infrastructure/k8s/backend.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aim-backend
  namespace: aim
spec:
  replicas: 3
  selector:
    matchLabels:
      app: aim-backend
  template:
    metadata:
      labels:
        app: aim-backend
    spec:
      containers:
      - name: backend
        image: your-registry.io/aim-backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: ENVIRONMENT
          value: production
        - name: APP_PORT
          value: "8080"
        - name: POSTGRES_HOST
          value: postgres.aim.svc.cluster.local
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: aim-secrets
              key: POSTGRES_PASSWORD
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: aim-secrets
              key: JWT_SECRET
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: aim-backend
  namespace: aim
spec:
  selector:
    app: aim-backend
  ports:
  - port: 80
    targetPort: 8080
```

#### 5. Deploy Ingress

```yaml
# infrastructure/k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: aim-ingress
  namespace: aim
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - your-domain.com
    secretName: aim-tls
  rules:
  - host: your-domain.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: aim-backend
            port:
              number: 80
      - path: /
        pathType: Prefix
        backend:
          service:
            name: aim-frontend
            port:
              number: 80
```

#### 6. Deploy

```bash
kubectl apply -f infrastructure/k8s/
```

---

## Post-Deployment

### 1. Verify Services

```bash
# Check backend health
curl https://api.your-domain.com/health

# Expected response:
# {"status":"healthy","service":"agent-identity-management","time":"2025-10-19T..."}

# Check frontend
curl https://your-domain.com
```

### 2. Create First User

```bash
# Register admin user
curl -X POST https://api.your-domain.com/api/v1/public/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@your-domain.com",
    "password": "SecurePassword123!",
    "firstName": "Admin",
    "lastName": "User"
  }'

# Approve user (requires database access or admin panel)
```

### 3. Configure Monitoring (Optional)

#### Prometheus + Grafana

```bash
# Install Prometheus Operator (K8s)
helm repo add prometheus-community \
  https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace

# Configure ServiceMonitor for AIM
kubectl apply -f infrastructure/k8s/monitoring/servicemonitor.yaml
```

---

## Security Best Practices

### 1. Environment Variables

**✅ DO:**
- Use platform-specific secret management (Azure Key Vault, AWS Secrets Manager, K8s Secrets)
- Rotate JWT secrets regularly (every 90 days)
- Use strong, unique passwords (minimum 24 characters)

**❌ DON'T:**
- Commit `.env` files to version control
- Share secrets via email or Slack
- Reuse passwords across environments

### 2. Database Security

- Enable SSL/TLS for PostgreSQL connections (`POSTGRES_SSL_MODE=require`)
- Use private networking (no public IPs)
- Regular automated backups (daily minimum)
- Enable connection pooling (PgBouncer)

### 3. Network Security

- Use HTTPS everywhere (Let's Encrypt for free certificates)
- Enable WAF (Web Application Firewall)
- Configure rate limiting (built into AIM)
- Restrict database access to backend only

### 4. API Security

- Enable API key authentication for programmatic access
- Implement IP whitelisting for sensitive endpoints
- Monitor for anomalous behavior
- Enable audit logging (built into AIM)

### 5. Container Security

- Scan images for vulnerabilities (Trivy, Snyk)
- Run containers as non-root user
- Use distroless or minimal base images
- Keep images up to date

---

## Troubleshooting

### Backend Won't Start

**Error:** `panic: Required environment variable POSTGRES_HOST is not set`

**Solution:**
```bash
# Check .env file exists
ls -la .env

# Verify POSTGRES_HOST is set
grep POSTGRES_HOST .env

# If missing, copy from template
cp .env.example .env
# Edit with actual values
nano .env
```

### Database Connection Failed

**Error:** `failed to connect to database`

**Solution:**
```bash
# Check PostgreSQL is running
docker compose ps postgres

# Verify connection string
psql "postgresql://postgres:postgres@localhost:5432/identity?sslmode=disable"

# Check PostgreSQL logs
docker compose logs postgres
```

### Frontend Cannot Reach Backend

**Error:** `Network Error` or `CORS Error`

**Solution:**
```bash
# Verify NEXT_PUBLIC_API_URL is correct
grep NEXT_PUBLIC_API_URL .env

# Check CORS configuration in backend
grep CORS_ALLOWED_ORIGINS .env

# Test backend directly
curl http://localhost:8080/health
```

### Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using port
lsof -ti:8080

# Kill process
lsof -ti:8080 | xargs kill -9

# Or use stop script
./stop-dev.sh
```

### Redis Connection Failed

**Note:** Redis is optional - AIM will continue without caching.

```bash
# Check Redis is running
docker compose ps redis

# Test connection
redis-cli -h localhost -p 6379 ping
```

---

## Additional Resources

- **Documentation:** https://docs.opena2a.org/aim
- **GitHub Repository:** https://github.com/opena2a-org/agent-identity-management
- **Community:** https://discord.gg/opena2a
- **Security Issues:** security@opena2a.org

---

**Deployment Script Usage:**

```bash
# Quick setup (first time)
./setup.sh

# Start development
./start-dev.sh

# Deploy to production (Docker Compose)
./deploy-prod.sh --platform=docker

# Deploy to Azure
./deploy-prod.sh --platform=azure

# Stop services
./stop-dev.sh
```

---

**Last Updated:** October 19, 2025  
**Version:** 1.0.0  
**License:** Apache 2.0
