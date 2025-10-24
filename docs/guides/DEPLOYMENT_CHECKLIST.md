# Deployment Verification Checklist

## Pre-Deployment Requirements

### Code Quality
- [ ] All Go code compiles without errors
- [ ] All TypeScript code compiles without errors
- [ ] No linter errors in backend
- [ ] No linter errors in frontend
- [ ] All TODO comments addressed or documented
- [ ] No hardcoded secrets or credentials
- [ ] Environment variables properly documented

### Testing
- [ ] All unit tests pass (`go test ./...`)
- [ ] All integration tests pass
- [ ] Frontend E2E tests executed and passed
- [ ] Manual testing completed (see MANUAL_TESTING_GUIDE.md)
- [ ] Performance benchmarks met (p95 < 100ms)
- [ ] Load testing completed (10,000 concurrent users)
- [ ] Security scan completed (no critical vulnerabilities)

### Documentation
- [ ] README.md is comprehensive and up-to-date
- [ ] API documentation generated (OpenAPI/Swagger)
- [ ] SETUP_GUIDE.md created
- [ ] MANUAL_TESTING_GUIDE.md created
- [ ] INTEGRATION_TEST_PLAN.md created
- [ ] Architecture diagrams included
- [ ] Environment variables documented

### Security
- [ ] OAuth credentials configured (Google, Microsoft, Okta)
- [ ] JWT secret is strong (32+ characters)
- [ ] Database credentials are strong
- [ ] All secrets stored in environment variables (not hardcoded)
- [ ] TLS/HTTPS configured for production
- [ ] API keys hashed with SHA-256
- [ ] Input validation on all endpoints
- [ ] SQL injection prevention verified
- [ ] CORS configured correctly
- [ ] Rate limiting enabled

---

## Local Development Deployment

### Step 1: Environment Setup
```bash
cd /Users/decimai/workspace/agent-identity-management

# Verify .env files exist
ls -la apps/backend/.env
ls -la apps/web/.env.local

# If missing, copy from examples
cp apps/backend/.env.example apps/backend/.env
cp apps/web/.env.local.example apps/web/.env.local

# Edit .env files with your credentials
```

#### Backend .env Required Variables
```bash
APP_PORT=8080
ENVIRONMENT=development

POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=identity

REDIS_HOST=localhost
REDIS_PORT=6379

JWT_SECRET=<32-char-random-string>

GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback/google

# Optional: Microsoft OAuth
# MICROSOFT_CLIENT_ID=
# MICROSOFT_CLIENT_SECRET=
# MICROSOFT_REDIRECT_URL=

# Optional: Okta OAuth
# OKTA_CLIENT_ID=
# OKTA_CLIENT_SECRET=
# OKTA_DOMAIN=
# OKTA_REDIRECT_URL=
```

#### Frontend .env.local Required Variables
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

**Checklist:**
- [ ] Backend .env file created and configured
- [ ] Frontend .env.local file created and configured
- [ ] OAuth credentials obtained from providers
- [ ] JWT secret generated (use `openssl rand -base64 32`)

### Step 2: Start Infrastructure Services
```bash
# Start PostgreSQL and Redis
docker compose up -d postgres redis

# Wait for services to be ready (30 seconds)
sleep 30

# Verify services are running
docker ps | grep -E 'postgres|redis'

# Expected output: Two running containers
```

**Checklist:**
- [ ] PostgreSQL container running
- [ ] Redis container running
- [ ] No error logs in `docker logs aim-postgres`
- [ ] No error logs in `docker logs aim-redis`

### Step 3: Database Setup
```bash
cd apps/backend

# Run migrations
go run cmd/migrate/main.go up

# Verify tables created
docker exec -it aim-postgres psql -U postgres -d identity -c "\dt"

# Expected output: List of tables (users, organizations, agents, api_keys, etc.)
```

**Checklist:**
- [ ] Migrations executed successfully
- [ ] All tables created
- [ ] No migration errors
- [ ] Database schema matches domain models

### Step 4: Start Backend Server
```bash
cd apps/backend

# Start server
go run cmd/server/main.go

# Expected output:
# - Server starting on :8080
# - Database connected
# - Redis connected
# - Routes registered
```

**Checklist:**
- [ ] Backend server starts without errors
- [ ] Health endpoint responds: `curl http://localhost:8080/api/v1/health`
- [ ] No panic or fatal errors in logs
- [ ] All routes registered successfully

### Step 5: Start Frontend Dev Server
```bash
# In a new terminal
cd apps/web

# Install dependencies (if not done)
npm install

# Start dev server
npm run dev

# Expected output:
# - Server starting on http://localhost:3000
# - Ready in Xms
```

**Checklist:**
- [ ] Frontend server starts without errors
- [ ] Can access http://localhost:3000 in browser
- [ ] No compilation errors
- [ ] Hot reload working

### Step 6: Smoke Test
```bash
# Test backend health
curl http://localhost:8080/api/v1/health

# Test OAuth endpoints (should redirect)
curl -I http://localhost:8080/api/v1/auth/login/google

# Test protected endpoint (should return 401)
curl http://localhost:8080/api/v1/agents
```

**Checklist:**
- [ ] Health check returns 200 OK
- [ ] OAuth endpoints return 302 redirect
- [ ] Protected endpoints return 401 unauthorized
- [ ] Frontend landing page loads
- [ ] No console errors in browser

---

## Production Deployment (Docker Compose)

### Step 1: Build Production Images
```bash
# Build backend image
docker build -f apps/backend/Dockerfile -t aim-backend:latest .

# Build frontend image
docker build -f apps/web/Dockerfile -t aim-frontend:latest .
```

**Checklist:**
- [ ] Backend image builds successfully
- [ ] Frontend image builds successfully
- [ ] Images are under 500MB each
- [ ] No build errors

### Step 2: Configure Production Environment
```bash
# Create production .env file
cp apps/backend/.env.example apps/backend/.env.production

# Update with production values:
# - Strong database password
# - Production JWT secret (64+ chars)
# - Real OAuth credentials
# - ENVIRONMENT=production
# - Enable TLS
```

**Checklist:**
- [ ] Production .env configured
- [ ] Strong passwords (20+ characters)
- [ ] JWT secret is 64+ characters
- [ ] OAuth credentials are production-ready
- [ ] Database connection string uses production host

### Step 3: Start Production Stack
```bash
# Start all services
docker compose -f docker-compose.prod.yml up -d

# Wait for services
sleep 30

# Check status
docker compose ps
```

**Checklist:**
- [ ] All containers running
- [ ] PostgreSQL healthy
- [ ] Redis healthy
- [ ] Backend healthy
- [ ] Frontend healthy
- [ ] Nginx/reverse proxy healthy (if used)

### Step 4: Run Production Migrations
```bash
# Run migrations in production
docker compose exec backend /app/migrate up
```

**Checklist:**
- [ ] Migrations completed
- [ ] No errors
- [ ] Database schema up to date

### Step 5: Production Smoke Test
```bash
# Test health endpoint
curl https://your-domain.com/api/v1/health

# Test landing page
curl -I https://your-domain.com

# Test OAuth flow
curl -I https://your-domain.com/api/v1/auth/login/google
```

**Checklist:**
- [ ] HTTPS working
- [ ] SSL certificate valid
- [ ] Health check returns 200
- [ ] Landing page loads
- [ ] OAuth redirects to Google
- [ ] No errors in production logs

---

## Kubernetes Deployment

### Prerequisites
- [ ] Kubernetes cluster available (EKS, GKE, AKS, or local)
- [ ] `kubectl` configured
- [ ] Docker images pushed to registry
- [ ] Kubernetes manifests created
- [ ] Secrets configured

### Step 1: Create Namespace
```bash
kubectl create namespace aim-prod
kubectl config set-context --current --namespace=aim-prod
```

### Step 2: Create Secrets
```bash
# Create database secret
kubectl create secret generic postgres-secret \
  --from-literal=username=postgres \
  --from-literal=password=<strong-password> \
  --from-literal=database=identity

# Create JWT secret
kubectl create secret generic jwt-secret \
  --from-literal=secret=<64-char-random-string>

# Create OAuth secrets
kubectl create secret generic oauth-secrets \
  --from-literal=google-client-id=<your-id> \
  --from-literal=google-client-secret=<your-secret>
```

**Checklist:**
- [ ] All secrets created
- [ ] No secrets in Git or ConfigMaps
- [ ] Secrets encrypted at rest

### Step 3: Deploy Infrastructure
```bash
# Deploy PostgreSQL StatefulSet
kubectl apply -f infrastructure/k8s/postgres.yaml

# Deploy Redis Deployment
kubectl apply -f infrastructure/k8s/redis.yaml

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=postgres --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis --timeout=300s
```

**Checklist:**
- [ ] PostgreSQL pod running
- [ ] Redis pod running
- [ ] PVCs created and bound
- [ ] Services created

### Step 4: Run Migrations
```bash
# Create migration job
kubectl apply -f infrastructure/k8s/migration-job.yaml

# Wait for completion
kubectl wait --for=condition=complete job/migration --timeout=300s

# Check logs
kubectl logs job/migration
```

**Checklist:**
- [ ] Migration job completed
- [ ] All tables created
- [ ] No errors in logs

### Step 5: Deploy Backend
```bash
# Deploy backend
kubectl apply -f infrastructure/k8s/backend-deployment.yaml

# Wait for rollout
kubectl rollout status deployment/backend

# Check pods
kubectl get pods -l app=backend
```

**Checklist:**
- [ ] Backend pods running (2+ replicas)
- [ ] All pods ready
- [ ] Health checks passing
- [ ] Service created

### Step 6: Deploy Frontend
```bash
# Deploy frontend
kubectl apply -f infrastructure/k8s/frontend-deployment.yaml

# Wait for rollout
kubectl rollout status deployment/frontend

# Check pods
kubectl get pods -l app=frontend
```

**Checklist:**
- [ ] Frontend pods running (2+ replicas)
- [ ] All pods ready
- [ ] Health checks passing
- [ ] Service created

### Step 7: Deploy Ingress
```bash
# Deploy ingress
kubectl apply -f infrastructure/k8s/ingress.yaml

# Wait for external IP
kubectl get ingress -w
```

**Checklist:**
- [ ] Ingress created
- [ ] External IP assigned
- [ ] TLS certificate issued (cert-manager)
- [ ] DNS configured

### Step 8: Production Verification
```bash
# Test health endpoint
curl https://your-domain.com/api/v1/health

# Test landing page
curl -I https://your-domain.com

# Check all pods
kubectl get pods

# Check logs
kubectl logs -l app=backend --tail=100
kubectl logs -l app=frontend --tail=100
```

**Checklist:**
- [ ] All pods running
- [ ] HTTPS working
- [ ] Health checks passing
- [ ] Landing page loads
- [ ] No errors in logs
- [ ] Monitoring enabled

---

## Post-Deployment Verification

### Functional Testing
- [ ] Can log in with Google OAuth
- [ ] Can create an agent
- [ ] Can generate API key
- [ ] Can view trust score
- [ ] Admin can access admin panel
- [ ] Audit logs are created
- [ ] Alerts are generated

### Performance Testing
- [ ] API response time < 100ms (p95)
- [ ] Frontend FCP < 1s
- [ ] Frontend TTI < 2s
- [ ] Database queries < 50ms
- [ ] Cache hit rate > 80%

### Security Testing
- [ ] HTTPS enforced
- [ ] OAuth flow secure
- [ ] JWT tokens expire correctly
- [ ] API keys are hashed
- [ ] SQL injection prevented
- [ ] XSS protection enabled
- [ ] CSRF protection enabled
- [ ] Rate limiting working

### Monitoring
- [ ] Prometheus metrics exposed
- [ ] Grafana dashboards configured
- [ ] Loki logging configured
- [ ] Alerts configured
- [ ] Health checks configured
- [ ] Uptime monitoring configured

### Backup & Recovery
- [ ] Database backups automated
- [ ] Backup restoration tested
- [ ] Disaster recovery plan documented
- [ ] RTO/RPO defined

---

## Rollback Procedure

If deployment fails:

### Docker Compose Rollback
```bash
# Stop current deployment
docker compose down

# Restore previous .env
cp .env.backup .env

# Start previous version
docker compose up -d

# Verify health
curl http://localhost:8080/api/v1/health
```

### Kubernetes Rollback
```bash
# Rollback backend deployment
kubectl rollout undo deployment/backend

# Rollback frontend deployment
kubectl rollout undo deployment/frontend

# Verify rollback
kubectl rollout status deployment/backend
kubectl rollout status deployment/frontend

# Check health
curl https://your-domain.com/api/v1/health
```

**Rollback Checklist:**
- [ ] Previous version restored
- [ ] Health checks passing
- [ ] No errors in logs
- [ ] Users can access application
- [ ] Data integrity verified

---

## Success Criteria

Deployment is successful when:

### Infrastructure
- ✅ All services running
- ✅ Database migrations applied
- ✅ No container restarts
- ✅ Health checks passing

### Functionality
- ✅ OAuth login working
- ✅ Agent registration working
- ✅ API key generation working
- ✅ Trust scoring working
- ✅ Admin panel accessible
- ✅ Audit logs captured

### Performance
- ✅ API p95 < 100ms
- ✅ Frontend FCP < 1s
- ✅ No timeout errors
- ✅ Cache working

### Security
- ✅ HTTPS enabled
- ✅ Secrets not exposed
- ✅ Authentication required
- ✅ Authorization enforced
- ✅ Rate limiting active

### Monitoring
- ✅ Metrics collected
- ✅ Logs aggregated
- ✅ Alerts configured
- ✅ Dashboards available

---

## Maintenance Tasks

### Daily
- [ ] Check application logs for errors
- [ ] Monitor performance metrics
- [ ] Review security alerts
- [ ] Check backup status

### Weekly
- [ ] Review and rotate logs
- [ ] Update dependencies (security patches)
- [ ] Review and close resolved alerts
- [ ] Performance optimization review

### Monthly
- [ ] Review and update documentation
- [ ] Conduct security scan
- [ ] Review and optimize database
- [ ] Disaster recovery drill
- [ ] Dependency updates (minor versions)

---

## Support & Troubleshooting

### Common Issues

**Issue: Backend won't start**
- Check logs: `docker logs aim-backend` or `kubectl logs -l app=backend`
- Verify database connection string
- Check environment variables
- Verify migrations completed

**Issue: Frontend won't connect to backend**
- Verify NEXT_PUBLIC_API_URL is correct
- Check CORS configuration
- Verify backend is accessible
- Check network/firewall rules

**Issue: OAuth login fails**
- Verify OAuth credentials
- Check redirect URLs match exactly
- Verify callback URL is accessible
- Check OAuth provider settings

**Issue: Database connection fails**
- Verify PostgreSQL is running
- Check database credentials
- Verify connection string format
- Check network connectivity

**Issue: High latency**
- Check database query performance
- Verify cache is working
- Check network latency
- Review slow query logs

### Getting Help
- Check logs first
- Review documentation
- Search GitHub issues
- Join Discord community
- Contact support: info@opena2a.org

---

## Production Readiness Score

Calculate your production readiness:

### Infrastructure (20 points)
- [ ] All services containerized (5 pts)
- [ ] Docker Compose working (5 pts)
- [ ] Kubernetes manifests ready (5 pts)
- [ ] Health checks configured (5 pts)

### Code Quality (20 points)
- [ ] No compilation errors (5 pts)
- [ ] No linter errors (5 pts)
- [ ] Code coverage > 80% (5 pts)
- [ ] No critical security vulnerabilities (5 pts)

### Testing (20 points)
- [ ] Unit tests passing (5 pts)
- [ ] Integration tests passing (5 pts)
- [ ] E2E tests passing (5 pts)
- [ ] Load testing completed (5 pts)

### Documentation (15 points)
- [ ] README comprehensive (5 pts)
- [ ] API docs complete (5 pts)
- [ ] Deployment guide complete (5 pts)

### Security (15 points)
- [ ] No hardcoded secrets (5 pts)
- [ ] Authentication working (5 pts)
- [ ] HTTPS enabled (5 pts)

### Monitoring (10 points)
- [ ] Metrics exposed (5 pts)
- [ ] Logging configured (5 pts)

**Total Score: ____ / 100**

- **90-100**: Production ready! 🚀
- **75-89**: Almost there, minor fixes needed
- **60-74**: Some work needed before launch
- **Below 60**: Not ready for production

---

**Deployment Date**: _______________
**Deployed By**: _______________
**Version**: _______________
**Environment**: _______________
**Status**: ✅ Success / ❌ Failed / ⏳ In Progress
