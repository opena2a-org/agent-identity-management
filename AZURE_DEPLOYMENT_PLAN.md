# ☁️ Azure Deployment Plan for AIM Demo Environment

## 🎯 Requirements

**Objective**: Deploy AIM as a 24/7 demo environment supporting:
- 100 agent registrations
- Continuous verification workload (every 24 hours per agent)
- Email notifications for all events
- Cost-optimized architecture

## 💰 Cost Analysis

### Monthly Cost Breakdown

#### Option 1: Azure Container Apps (RECOMMENDED)
| Service | Tier | Specs | Monthly Cost |
|---------|------|-------|--------------|
| **Container Apps - Backend** | Consumption | 0.5 vCPU, 1GB RAM | ~$25 |
| **Container Apps - Frontend** | Consumption | 0.25 vCPU, 0.5GB RAM | ~$15 |
| **PostgreSQL Flexible Server** | Burstable B1ms | 1 vCPU, 2GB RAM, 32GB storage | ~$25 |
| **Azure Cache for Redis** | Basic C0 | 250MB cache | ~$16 |
| **Container Registry** | Basic | 10GB storage | ~$5 |
| **Communication Services Email** | Pay-per-use | 500 emails/month (FREE) | $0 |
| **Application Insights** | Pay-per-GB | ~1GB/month | ~$2 |
| **Azure Monitor** | Basic metrics | Included with services | $0 |
| **Virtual Network** | Standard | Included with Container Apps | $0 |
| **Data Transfer** | Outbound | ~5GB/month | ~$0.50 |
| **TOTAL** | | | **~$88.50/month** |

#### Option 2: Azure App Service (Alternative)
| Service | Tier | Specs | Monthly Cost |
|---------|------|-------|--------------|
| **App Service - Backend** | B1 Basic | 1 vCPU, 1.75GB RAM | ~$55 |
| **App Service - Frontend** | B1 Basic | 1 vCPU, 1.75GB RAM | ~$55 |
| **PostgreSQL Flexible Server** | Burstable B1ms | 1 vCPU, 2GB RAM, 32GB storage | ~$25 |
| **Azure Cache for Redis** | Basic C0 | 250MB cache | ~$16 |
| **Container Registry** | Basic | 10GB storage | $5 |
| **Communication Services Email** | Pay-per-use | 500 emails/month (FREE) | $0 |
| **Application Insights** | Pay-per-GB | ~1GB/month | ~$2 |
| **TOTAL** | | | **~$158/month** |

#### Option 3: Azure Kubernetes Service (NOT Recommended for Demo)
| Service | Tier | Specs | Monthly Cost |
|---------|------|-------|--------------|
| **AKS Cluster** | Standard | 2 nodes × D2s_v3 | ~$140 |
| **Load Balancer** | Standard | Public IP | ~$20 |
| **PostgreSQL Flexible Server** | Burstable B1ms | 1 vCPU, 2GB RAM | ~$25 |
| **Azure Cache for Redis** | Basic C0 | 250MB | ~$16 |
| **Container Registry** | Basic | 10GB | $5 |
| **TOTAL** | | | **~$206/month** |

### ✅ RECOMMENDATION: Azure Container Apps (~$89/month)

**Why Container Apps?**
1. **Lowest Cost**: 44% cheaper than App Service, 57% cheaper than AKS
2. **Auto-Scaling**: Scales to zero when idle (demo periods)
3. **Managed**: No infrastructure management overhead
4. **Kubernetes-Based**: Easy migration to AKS later if needed
5. **DAPR Support**: Built-in service mesh for microservices
6. **Ingress**: Automatic HTTPS with managed certificates

## 🏗️ Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Azure Subscription                       │
│                  1b1e58e7-97db-4b08-b3d9-ee8e7867bcb9       │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┴─────────────────────┐
        │                                           │
┌───────▼────────┐                       ┌──────────▼─────────┐
│  Resource Group │                       │  Container Registry │
│   aim-demo-rg   │                       │   aimdemoregistry   │
└────────┬────────┘                       └──────────┬─────────┘
         │                                           │
         │                          ┌────────────────┘
         │                          │
┌────────▼────────────────────────────────────────────────────┐
│              Container Apps Environment                      │
│                   aim-demo-env                              │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐        ┌─────────────────┐            │
│  │  Backend API    │        │  Frontend UI    │            │
│  │  (Go + Fiber)   │◄───────┤  (Next.js)      │            │
│  │  0.5 vCPU       │        │  0.25 vCPU      │            │
│  │  1 GB RAM       │        │  0.5 GB RAM     │            │
│  │  Min: 1 replica │        │  Min: 1 replica │            │
│  │  Max: 3 replicas│        │  Max: 2 replicas│            │
│  └────────┬────────┘        └─────────────────┘            │
│           │                                                  │
└───────────┼──────────────────────────────────────────────────┘
            │
    ┌───────┼──────────────────────────┐
    │       │                          │
┌───▼───────▼─────┐    ┌──────────────▼──────────┐
│ PostgreSQL      │    │  Azure Cache for Redis  │
│ Flexible Server │    │  (Session Store)        │
│ Burstable B1ms  │    │  Basic C0 (250MB)       │
│ 1 vCPU, 2GB RAM │    └─────────────────────────┘
│ 32GB Storage    │
└─────────────────┘
         │
    ┌────┴────┐
    │         │
┌───▼─────────▼─────────────┐
│ Azure Communication       │
│ Services Email            │
│ (SMTP + Email API)        │
└───────────────────────────┘

┌────────────────────────────┐
│  Application Insights      │
│  (Monitoring & Logs)       │
└────────────────────────────┘
```

## 📊 Capacity Planning for 100 Agents

### Backend Workload Analysis

**Assumptions**:
- 100 agents total
- Each agent verifies every 24 hours
- Average verification takes 2 seconds
- Peak load: 20 concurrent verifications

**Calculations**:
```
Daily verifications: 100 agents × 1 verification = 100 verifications
Verification load: 100 verifications × 2 seconds = 200 seconds/day = 3.3 minutes/day
Peak concurrent: 20 verifications × 2 seconds = 40 seconds of CPU time

Memory per verification: ~50MB
Peak memory: 20 × 50MB = 1GB

Recommended Backend:
- Min replicas: 1
- Max replicas: 3
- CPU: 0.5 vCPU (enough for 20 concurrent verifications)
- RAM: 1GB (handles peak load with headroom)
```

### Database Sizing

**Storage Requirements**:
```
Tables:
- users: 100 rows × 1KB = 100KB
- agents: 100 rows × 2KB = 200KB
- api_keys: 100 rows × 512B = 50KB
- verification_history: 100 agents × 30 days × 500B = 1.5MB
- trust_scores: 100 agents × 30 days × 200B = 600KB
- audit_logs: ~10,000 entries × 1KB = 10MB
- alerts: ~1,000 entries × 500B = 500KB

Total: ~13MB of data (32GB storage is massive headroom)
```

**Connection Pool**:
```
Max concurrent connections: 20
Backend replicas: 3 × 10 connections = 30 connections
PostgreSQL max_connections: 100 (default)
```

### Redis Cache Sizing

**Cache Keys**:
```
- Session tokens: 100 users × 2KB = 200KB
- Rate limit counters: 100 agents × 1KB = 100KB
- Verification queue: 20 items × 5KB = 100KB

Total: ~400KB (250MB Basic C0 is more than enough)
```

### Email Volume

**Monthly Email Count**:
```
Registration emails: 100 agents × 1 = 100 emails
Verification reminders: 100 agents × 30 days × 0.1 (10% get reminders) = 300 emails
Alert notifications: 100 agents × 2 alerts/month = 200 emails
User approvals: 10 new users × 1 = 10 emails

Total: ~610 emails/month

Azure ACS Email Cost:
- First 500 emails: FREE
- Next 110 emails: 110 × $0.0001 = $0.011

Monthly Email Cost: ~$0.01 (negligible)
```

## 🚀 Deployment Steps

### Prerequisites
1. Azure CLI installed
2. Docker installed
3. GitHub repository access
4. Azure subscription: `1b1e58e7-97db-4b08-b3d9-ee8e7867bcb9`

### Step 1: Login to Azure
```bash
az login
az account set --subscription 1b1e58e7-97db-4b08-b3d9-ee8e7867bcb9
```

### Step 2: Create Resource Group
```bash
az group create \
  --name aim-demo-rg \
  --location eastus2
```

### Step 3: Create Container Registry
```bash
az acr create \
  --name aimdemoregistry \
  --resource-group aim-demo-rg \
  --sku Basic \
  --admin-enabled true
```

### Step 4: Create PostgreSQL Flexible Server
```bash
az postgres flexible-server create \
  --name aim-demo-db \
  --resource-group aim-demo-rg \
  --location eastus2 \
  --admin-user aimadmin \
  --admin-password 'AIM$ecure2025!' \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --storage-size 32 \
  --version 16 \
  --public-access 0.0.0.0 \
  --high-availability Disabled
```

### Step 5: Create Redis Cache
```bash
az redis create \
  --name aim-demo-redis \
  --resource-group aim-demo-rg \
  --location eastus2 \
  --sku Basic \
  --vm-size C0
```

### Step 6: Create Communication Services
```bash
az communication create \
  --name aim-demo-email \
  --resource-group aim-demo-rg \
  --location global \
  --data-location UnitedStates

# Get connection string
az communication list-key \
  --name aim-demo-email \
  --resource-group aim-demo-rg
```

### Step 7: Create Container Apps Environment
```bash
az containerapp env create \
  --name aim-demo-env \
  --resource-group aim-demo-rg \
  --location eastus2 \
  --enable-workload-profiles false
```

### Step 8: Build and Push Docker Images
```bash
# Login to ACR
az acr login --name aimdemoregistry

# Build backend
docker build \
  -f infrastructure/docker/Dockerfile.backend \
  -t aimdemoregistry.azurecr.io/aim-backend:latest \
  .

# Build frontend
docker build \
  -f infrastructure/docker/Dockerfile.frontend \
  -t aimdemoregistry.azurecr.io/aim-frontend:latest \
  ./apps/web

# Push images
docker push aimdemoregistry.azurecr.io/aim-backend:latest
docker push aimdemoregistry.azurecr.io/aim-frontend:latest
```

### Step 9: Deploy Backend Container App
```bash
az containerapp create \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --environment aim-demo-env \
  --image aimdemoregistry.azurecr.io/aim-backend:latest \
  --registry-server aimdemoregistry.azurecr.io \
  --registry-username aimdemoregistry \
  --registry-password $(az acr credential show --name aimdemoregistry --query passwords[0].value -o tsv) \
  --target-port 8080 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 3 \
  --cpu 0.5 \
  --memory 1Gi \
  --env-vars \
    DATABASE_URL=secretref:database-url \
    REDIS_URL=secretref:redis-url \
    JWT_SECRET=secretref:jwt-secret \
    AZURE_EMAIL_CONNECTION_STRING=secretref:email-connection \
    EMAIL_FROM_ADDRESS=noreply@aim-demo.com \
    EMAIL_PROVIDER=azure
```

### Step 10: Deploy Frontend Container App
```bash
az containerapp create \
  --name aim-frontend \
  --resource-group aim-demo-rg \
  --environment aim-demo-env \
  --image aimdemoregistry.azurecr.io/aim-frontend:latest \
  --registry-server aimdemoregistry.azurecr.io \
  --registry-username aimdemoregistry \
  --registry-password $(az acr credential show --name aimdemoregistry --query passwords[0].value -o tsv) \
  --target-port 3000 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 2 \
  --cpu 0.25 \
  --memory 0.5Gi \
  --env-vars \
    NEXT_PUBLIC_API_URL=https://aim-backend.proudsand-12345.eastus2.azurecontainerapps.io
```

### Step 11: Configure Secrets
```bash
# Database URL
az containerapp secret set \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --secrets database-url="postgresql://aimadmin:AIM\$ecure2025!@aim-demo-db.postgres.database.azure.com:5432/aim?sslmode=require"

# Redis URL
REDIS_KEY=$(az redis list-keys --name aim-demo-redis --resource-group aim-demo-rg --query primaryKey -o tsv)
az containerapp secret set \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --secrets redis-url="redis://:${REDIS_KEY}@aim-demo-redis.redis.cache.windows.net:6380?ssl=true"

# JWT Secret
az containerapp secret set \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --secrets jwt-secret="$(openssl rand -base64 32)"

# Email Connection String
EMAIL_CONN=$(az communication list-key --name aim-demo-email --resource-group aim-demo-rg --query primaryConnectionString -o tsv)
az containerapp secret set \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --secrets email-connection="${EMAIL_CONN}"
```

### Step 12: Run Database Migrations
```bash
# Get backend FQDN
BACKEND_URL=$(az containerapp show --name aim-backend --resource-group aim-demo-rg --query properties.configuration.ingress.fqdn -o tsv)

# SSH into container and run migrations
az containerapp exec \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --command "/app/migrate up"
```

### Step 13: Create Bootstrap Admin User
```bash
az containerapp exec \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --command "/app/bootstrap create-admin --email admin@aim-demo.com --name 'Admin User'"
```

## 🎛️ Infrastructure as Code (Bicep)

Create `infrastructure/azure/main.bicep`:

```bicep
// See full Bicep template in infrastructure/azure/ directory
param location string = 'eastus2'
param environmentName string = 'aim-demo'

// Container Registry
module acr 'modules/acr.bicep' = {
  name: 'container-registry'
  params: {
    name: '${environmentName}registry'
    location: location
  }
}

// PostgreSQL
module postgres 'modules/postgres.bicep' = {
  name: 'postgresql-database'
  params: {
    serverName: '${environmentName}-db'
    location: location
  }
}

// Container Apps
module containerApps 'modules/containerapps.bicep' = {
  name: 'container-apps'
  params: {
    environmentName: '${environmentName}-env'
    location: location
  }
}
```

Deploy with:
```bash
az deployment group create \
  --resource-group aim-demo-rg \
  --template-file infrastructure/azure/main.bicep \
  --parameters environmentName=aim-demo
```

## 📊 Monitoring & Alerts

### Application Insights Queries

**Request Rate**:
```kusto
requests
| where timestamp > ago(1h)
| summarize count() by bin(timestamp, 5m)
| render timechart
```

**Failed Requests**:
```kusto
requests
| where success == false
| summarize count() by resultCode, bin(timestamp, 1h)
```

**Email Send Success Rate**:
```kusto
customEvents
| where name == "EmailSent"
| summarize success = countif(customDimensions.Status == "success"),
            failed = countif(customDimensions.Status == "failed")
| extend successRate = (success * 100.0) / (success + failed)
```

### Cost Alerts

```bash
az monitor action-group create \
  --name aim-cost-alerts \
  --resource-group aim-demo-rg \
  --short-name cost-alert \
  --email-receiver name=admin email=admin@aim-demo.com

az monitor metrics alert create \
  --name monthly-cost-alert \
  --resource-group aim-demo-rg \
  --scopes /subscriptions/1b1e58e7-97db-4b08-b3d9-ee8e7867bcb9 \
  --condition "total cost > 100" \
  --window-size 30d \
  --evaluation-frequency 1d \
  --action aim-cost-alerts
```

## 🔒 Security Hardening

1. **Enable HTTPS Only**: Container Apps default HTTPS
2. **Managed Identities**: Use for service-to-service auth
3. **Key Vault Integration**: Store secrets in Azure Key Vault
4. **Network Isolation**: Use Virtual Network integration
5. **WAF**: Add Azure Front Door with WAF rules

## 🎯 Success Criteria

- ✅ Total monthly cost < $100
- ✅ API response time < 100ms (p95)
- ✅ Email delivery rate > 99%
- ✅ Database connections < 30 concurrent
- ✅ Redis cache hit rate > 80%
- ✅ Zero-downtime deployments
- ✅ Auto-scaling working correctly

## 📚 Next Steps

1. Implement email integration (see `AZURE_EMAIL_INTEGRATION.md`)
2. Deploy infrastructure using Bicep templates
3. Configure CI/CD pipeline with GitHub Actions
4. Setup monitoring dashboards
5. Load test with 100 agents
6. Document operational runbook
