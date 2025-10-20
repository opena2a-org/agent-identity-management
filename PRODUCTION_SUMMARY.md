# 🎉 AIM Production Deployment - Quick Reference

## ✅ Current Production Deployment

### 🌐 Live URLs
- **Frontend**: https://aim-frontend.yellowbush-08563eac.canadacentral.azurecontainerapps.io
- **Backend**: https://aim-backend.yellowbush-08563eac.canadacentral.azurecontainerapps.io
- **Health Check**: https://aim-backend.yellowbush-08563eac.canadacentral.azurecontainerapps.io/health

### 🔐 Current Admin Credentials
```
Email:    admin@opena2a.org
Password: Admin2025!Secure
```

**⚠️ IMPORTANT**: Change this password immediately after logging in!

---

## 🚀 For Future Deployments

### Quick Deploy (Using Defaults)
```bash
# Clone repo
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Run deployment script
./deploy-azure-production.sh
```

This will deploy with:
- Admin: `admin@opena2a.org` / `Admin2025!Secure`
- Organization: `OpenA2A` (`opena2a.org`)
- Region: `canadacentral`

### Custom Deploy
```bash
# Set your custom configuration
export ADMIN_EMAIL="admin@yourcompany.com"
export ADMIN_PASSWORD="YourSecurePassword123!"
export ORG_NAME="Your Company"
export ORG_DOMAIN="yourcompany.com"

# Deploy
./deploy-azure-production.sh
```

---

## 📋 What Gets Deployed

### Azure Resources Created
1. **Resource Group**: `aim-production-rg` (Canada Central)
2. **Container Registry**: `aimprodregistry.azurecr.io`
3. **PostgreSQL 16**: `aim-prod-db-[timestamp]` (with SSL)
4. **Redis 7.0**: `aim-prod-redis-[timestamp]` (optional caching)
5. **Backend App**: Go/Fiber API (239 endpoints)
6. **Frontend App**: Next.js SSR application

### Database Setup
- **Automatic Migrations**: Runs on backend startup
- **Clean Schema**: Only 2 migration files
  - `001_initial_schema.sql` (core tables)
  - `002_fix_alerts_schema.up.sql` (alerts fixes)
- **Admin Bootstrap**: Creates default admin user and organization

### Features Included
✅ User authentication (local + OAuth ready)
✅ Organization management (multi-tenant)
✅ Agent registration and verification
✅ API key management (SHA-256 hashed)
✅ Trust scoring (8-factor ML algorithm)
✅ Audit logging (all admin actions)
✅ Security alerts and notifications
✅ Compliance reporting (SOC 2, HIPAA, GDPR)

---

## 🔧 Common Tasks

### View Logs
```bash
# Backend logs
az containerapp logs show --name aim-backend --resource-group aim-production-rg --follow

# Frontend logs
az containerapp logs show --name aim-frontend --resource-group aim-production-rg --follow
```

### Check Health
```bash
# Backend health check
curl https://aim-backend.yellowbush-08563eac.canadacentral.azurecontainerapps.io/health

# Expected: {"service":"agent-identity-management","status":"healthy","time":"..."}
```

### Update Application
```bash
# Backend update
VERSION=$(date +%Y%m%d-%H%M%S)
docker buildx build --platform linux/amd64 --no-cache \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  -t aimprodregistry.azurecr.io/aim-backend:$VERSION --push .

az containerapp update --name aim-backend --resource-group aim-production-rg \
  --image aimprodregistry.azurecr.io/aim-backend:$VERSION
```

### Delete Everything
```bash
# WARNING: Deletes EVERYTHING including database!
az group delete --name aim-production-rg --yes --no-wait
```

---

## 💰 Cost Estimate

Approximate monthly cost in Canada Central:

| Resource | Monthly Cost |
|----------|-------------|
| Container Apps (2x) | ~$15 |
| PostgreSQL B1ms | ~$25 |
| Redis Basic C0 | ~$16 |
| Container Registry | ~$5 |
| **Total** | **~$61/month** |

---

## 📚 Full Documentation

- **Deployment Guide**: `DEPLOYMENT.md` (comprehensive guide)
- **Migration Docs**: `AUTOMATIC_MIGRATIONS.md` (database migrations)
- **Deployment Script**: `deploy-azure-production.sh` (automated setup)

---

## 🐛 Troubleshooting

### Backend Not Starting
```bash
# Check logs for errors
az containerapp logs show --name aim-backend --resource-group aim-production-rg --tail 100

# Common issues:
# - Database connection: Check firewall rules
# - CORS panic: Verify ALLOWED_ORIGINS matches frontend URL
# - JWT_SECRET missing: Check environment variables
```

### Frontend 502/503
```bash
# Check if backend is healthy first
curl https://aim-backend.[...].azurecontainerapps.io/health

# Check frontend logs
az containerapp logs show --name aim-frontend --resource-group aim-production-rg --tail 100
```

### Database Connection Timeout
```bash
# Test database connection
psql "postgresql://aimadmin:AIM2025!Secure@[db-host]:5432/identity?sslmode=require"

# If fails: Check PostgreSQL firewall rules
az postgres flexible-server firewall-rule list \
  --resource-group aim-production-rg \
  --name aim-prod-db-[timestamp]
```

---

## 🔒 Security Checklist

After deployment:
- [ ] Change default admin password
- [ ] Review CORS configuration
- [ ] Restrict PostgreSQL firewall (remove 0.0.0.0/0)
- [ ] Enable email notifications
- [ ] Configure OAuth providers (Google, Microsoft)
- [ ] Review audit logs regularly
- [ ] Set up monitoring alerts
- [ ] Configure custom domain + SSL
- [ ] Enable database backups
- [ ] Rotate JWT secret regularly

---

## 📞 Support

- **GitHub**: https://github.com/opena2a-org/agent-identity-management
- **Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Documentation**: See `DEPLOYMENT.md`

---

**Last Deployed**: October 20, 2025
**Deployment Status**: ✅ Successful
**Migration Status**: ✅ All migrations applied
**Health Status**: ✅ Healthy
