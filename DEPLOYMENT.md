# 🚀 AIM Production Deployment Guide

Complete guide for deploying Agent Identity Management (AIM) to Azure Container Apps.

## 📋 Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Default Credentials](#default-credentials)
- [Configuration](#configuration)
- [Deployment Steps](#deployment-steps)
- [Post-Deployment](#post-deployment)
- [Troubleshooting](#troubleshooting)
- [Cleanup](#cleanup)

---

## Prerequisites

### Required Tools
- **Azure CLI** (`az`) - [Install Guide](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli)
- **Docker Desktop** - [Download](https://www.docker.com/products/docker-desktop/)
- **Go 1.23+** - [Download](https://go.dev/dl/)
- **Azure Subscription** with contributor access

### Azure Login
```bash
# Login to Azure
az login

# Set subscription (if you have multiple)
az account set --subscription "Your Subscription Name"

# Verify you're logged in
az account show
```

---

## Quick Start

### Option 1: Default Deployment (Recommended)
```bash
# Clone the repository
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Run deployment script with defaults
./deploy-azure-production.sh
```

**Default Configuration:**
- **Admin Email**: `admin@opena2a.org`
- **Admin Password**: `Admin2025!Secure`
- **Organization**: `OpenA2A`
- **Domain**: `opena2a.org`
- **Region**: `canadacentral`

### Option 2: Custom Configuration
```bash
# Set custom configuration via environment variables
export ADMIN_EMAIL="admin@yourcompany.com"
export ADMIN_PASSWORD="YourSecurePassword123!"
export ADMIN_NAME="Your Name"
export ORG_NAME="Your Company"
export ORG_DOMAIN="yourcompany.com"

# Run deployment
./deploy-azure-production.sh
```

---

## Default Credentials

### 🔐 Admin Login
After deployment completes, you can log in with:

```
Email:    admin@opena2a.org
Password: Admin2025!Secure
```

**⚠️ IMPORTANT**:
- You MUST change the default password on first login
- The system will force a password change for security
- Never use default credentials in production!

### 🗄️ Database Credentials
```
Host:     aim-prod-db-[timestamp].postgres.database.azure.com
Port:     5432
Database: identity
User:     aimadmin
Password: AIM2025!Secure
```

---

## Configuration

### Environment Variables

The deployment script accepts the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ADMIN_EMAIL` | `admin@opena2a.org` | Admin user email |
| `ADMIN_PASSWORD` | `Admin2025!Secure` | Admin user password (min 8 chars, must include uppercase, lowercase, number) |
| `ADMIN_NAME` | `System Administrator` | Admin display name |
| `ORG_NAME` | `OpenA2A` | Organization name |
| `ORG_DOMAIN` | `opena2a.org` | Organization domain |

### Azure Resources Created

The deployment creates the following Azure resources:

1. **Resource Group**: `aim-production-rg`
2. **Container Registry**: `aimprodregistry`
3. **PostgreSQL Server**: `aim-prod-db-[timestamp]`
4. **Redis Cache**: `aim-prod-redis-[timestamp]`
5. **Container Apps Environment**: `aim-prod-env`
6. **Backend Container App**: `aim-backend`
7. **Frontend Container App**: `aim-frontend`

### Pricing Estimate

Approximate monthly costs in Canada Central region:

| Resource | SKU | Monthly Cost (USD) |
|----------|-----|-------------------|
| Container Apps | 2x 0.5 vCPU, 1GB RAM | ~$15 |
| PostgreSQL | B1ms (1 vCore, 2GB) | ~$25 |
| Redis | Basic C0 (250MB) | ~$16 |
| Container Registry | Basic | ~$5 |
| **Total** | | **~$61/month** |

---

## Deployment Steps

The deployment script performs these steps automatically:

### 1. Create Resource Group
Creates the `aim-production-rg` resource group in Canada Central.

### 2. Create Container Registry
Sets up Azure Container Registry (ACR) for storing Docker images.

### 3. Build Docker Images
Builds backend and frontend images with versioned tags:
- Backend: `aimprodregistry.azurecr.io/aim-backend:[version]`
- Frontend: `aimprodregistry.azurecr.io/aim-frontend:[version]`

### 4. Create PostgreSQL Database
Provisions a PostgreSQL 16 Flexible Server with:
- SSL required
- Public access (for initial setup)
- 32GB storage
- Automatic backups enabled

### 5. Create Redis Cache
Sets up Redis 7.0 for session management and caching:
- Basic tier (250MB)
- SSL required (port 6380)

### 6. Create Container Apps Environment
Provisions the managed environment for container apps.

### 7. Deploy Backend
Deploys the Go/Fiber backend with:
- Automatic database migrations
- Environment-specific configuration
- Health checks enabled
- Auto-scaling (1-3 replicas)

### 8. Deploy Frontend
Deploys the Next.js frontend with:
- Server-side rendering
- API URL auto-detection
- Static asset optimization
- Auto-scaling (1-3 replicas)

### 9. Bootstrap Admin User
Creates the initial admin user and organization using the provided credentials.

---

## Post-Deployment

### 1. Verify Deployment
```bash
# Check backend health
curl https://aim-backend.[region].azurecontainerapps.io/health

# Expected response:
# {"service":"agent-identity-management","status":"healthy","time":"..."}
```

### 2. Access the Application
1. Open your browser to the frontend URL (displayed at end of deployment)
2. Click "Sign In"
3. Enter the admin credentials
4. Change your password when prompted

### 3. Configure OAuth Providers (Optional)

To enable Google/Microsoft sign-in:

```bash
# Update backend environment variables
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --set-env-vars \
    "GOOGLE_CLIENT_ID=your-google-client-id" \
    "GOOGLE_CLIENT_SECRET=your-google-client-secret" \
    "MICROSOFT_CLIENT_ID=your-microsoft-client-id" \
    "MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret"
```

### 4. Enable Email Notifications (Optional)

```bash
# Update backend with email configuration
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --set-env-vars \
    "EMAIL_FROM_ADDRESS=noreply@yourcompany.com" \
    "EMAIL_FROM_NAME=AIM Notifications" \
    "SMTP_HOST=smtp.sendgrid.net" \
    "SMTP_PORT=587" \
    "SMTP_USER=apikey" \
    "SMTP_PASSWORD=your-sendgrid-api-key"
```

### 5. Configure Custom Domain (Optional)

```bash
# Add custom domain to frontend
az containerapp hostname add \
  --name aim-frontend \
  --resource-group aim-production-rg \
  --hostname app.yourcompany.com

# Add custom domain to backend
az containerapp hostname add \
  --name aim-backend \
  --resource-group aim-production-rg \
  --hostname api.yourcompany.com
```

---

## Troubleshooting

### Backend Not Starting

**Check logs:**
```bash
az containerapp logs show \
  --name aim-backend \
  --resource-group aim-production-rg \
  --tail 50
```

**Common issues:**
- **Database connection failed**: Check PostgreSQL firewall rules
- **JWT_SECRET not set**: Verify environment variables
- **CORS panic**: Check `ALLOWED_ORIGINS` matches frontend URL

### Frontend 502/503 Errors

**Check logs:**
```bash
az containerapp logs show \
  --name aim-frontend \
  --resource-group aim-production-rg \
  --tail 50
```

**Common issues:**
- **API URL not set**: Verify `NEXT_PUBLIC_API_URL` environment variable
- **Image pull failed**: Check ACR credentials
- **Out of memory**: Increase memory limit to 2Gi

### Database Connection Issues

**Test connection:**
```bash
# Get database URL from deployment
DB_HOST=$(az postgres flexible-server show \
  --name aim-prod-db-[timestamp] \
  --resource-group aim-production-rg \
  --query fullyQualifiedDomainName -o tsv)

# Test connection
psql "postgresql://aimadmin:AIM2025!Secure@$DB_HOST:5432/identity?sslmode=require"
```

### Redis Connection Timeout

Redis may be blocked by Azure network security groups:

**Option 1: Disable Redis (AIM works without it)**
```bash
# Redis is optional - AIM will continue without caching
# Just ignore the timeout warning in logs
```

**Option 2: Configure network access**
```bash
# Allow Container Apps to access Redis
# (Requires VNet integration - advanced setup)
```

---

## Monitoring

### View Application Logs
```bash
# Backend logs (real-time)
az containerapp logs show \
  --name aim-backend \
  --resource-group aim-production-rg \
  --follow

# Frontend logs
az containerapp logs show \
  --name aim-frontend \
  --resource-group aim-production-rg \
  --follow
```

### Check Resource Status
```bash
# List all resources
az resource list \
  --resource-group aim-production-rg \
  --output table

# Check container app status
az containerapp show \
  --name aim-backend \
  --resource-group aim-production-rg \
  --query "properties.runningStatus"
```

### Database Metrics
```bash
# View PostgreSQL metrics
az postgres flexible-server list \
  --resource-group aim-production-rg \
  --output table
```

---

## Updating the Application

### Update Backend
```bash
# Build new image
VERSION=$(date +%Y%m%d-%H%M%S)
docker buildx build --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  -t aimprodregistry.azurecr.io/aim-backend:$VERSION \
  --push .

# Deploy new version
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --image aimprodregistry.azurecr.io/aim-backend:$VERSION
```

### Update Frontend
```bash
# Build new image
VERSION=$(date +%Y%m%d-%H%M%S)
docker buildx build --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.frontend \
  --build-arg NEXT_PUBLIC_API_URL=https://[your-backend-url] \
  -t aimprodregistry.azurecr.io/aim-frontend:$VERSION \
  --push .

# Deploy new version
az containerapp update \
  --name aim-frontend \
  --resource-group aim-production-rg \
  --image aimprodregistry.azurecr.io/aim-frontend:$VERSION
```

---

## Cleanup

### Delete All Resources
```bash
# WARNING: This deletes EVERYTHING including database!
az group delete \
  --name aim-production-rg \
  --yes \
  --no-wait
```

### Delete Specific Resources
```bash
# Delete only container apps (keep database)
az containerapp delete --name aim-backend --resource-group aim-production-rg --yes
az containerapp delete --name aim-frontend --resource-group aim-production-rg --yes

# Delete database
az postgres flexible-server delete \
  --name aim-prod-db-[timestamp] \
  --resource-group aim-production-rg \
  --yes
```

---

## Security Best Practices

### 1. Change Default Passwords
- Change admin password immediately after first login
- Use strong, unique passwords (min 12 characters)
- Enable MFA when available

### 2. Restrict Database Access
```bash
# Remove public access after deployment
az postgres flexible-server firewall-rule delete \
  --name AllowAll \
  --resource-group aim-production-rg \
  --server-name aim-prod-db-[timestamp]

# Add specific IP ranges only
az postgres flexible-server firewall-rule create \
  --resource-group aim-production-rg \
  --server-name aim-prod-db-[timestamp] \
  --name "AllowContainerApps" \
  --start-ip-address [container-app-ip] \
  --end-ip-address [container-app-ip]
```

### 3. Enable HTTPS Only
```bash
# Container Apps use HTTPS by default
# Redirect HTTP to HTTPS (automatic)
```

### 4. Rotate Secrets Regularly
```bash
# Rotate JWT secret (forces re-authentication)
NEW_JWT_SECRET=$(openssl rand -base64 32)

az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --set-env-vars "JWT_SECRET=$NEW_JWT_SECRET"
```

### 5. Enable Audit Logging
AIM automatically logs all administrative actions to the `audit_logs` table.

```sql
-- View recent audit logs
SELECT * FROM audit_logs
ORDER BY timestamp DESC
LIMIT 100;
```

---

## Support

### Getting Help
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Documentation**: https://github.com/opena2a-org/agent-identity-management
- **Community**: Join our Discord server

### Common Questions

**Q: How do I backup the database?**
```bash
az postgres flexible-server backup create \
  --resource-group aim-production-rg \
  --name aim-prod-db-[timestamp] \
  --backup-name manual-backup-$(date +%Y%m%d)
```

**Q: How do I scale the application?**
```bash
# Increase max replicas
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --max-replicas 10

# Increase resources
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --cpu 1.0 \
  --memory 2Gi
```

**Q: How do I enable debug logging?**
```bash
az containerapp update \
  --name aim-backend \
  --resource-group aim-production-rg \
  --set-env-vars "LOG_LEVEL=debug"
```

---

## License

MIT License - See LICENSE file for details

---

**Last Updated**: October 20, 2025
**Version**: 1.0.0
**Deployment Script**: `deploy-azure-production.sh`
