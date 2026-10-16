# 🚀 AIM Deployment Guide

Complete guide for deploying AIM (Agent Identity Management) in development, staging, and production environments.

> **Note on the hosted service.** This repository is the open-source source of truth for AIM and this guide targets **self-hosters**. The hosted service at `aim.opena2a.org` deploys from a separate private mirror (`opena2a-org/aim-cloud`) that layers cloud-only features (Stripe billing, Google OAuth) on top of this repo via a one-directional sync. If you're here to deploy your own AIM instance, continue below. If you're looking for how the hosted service works, the mirror's `aim-cloud/.github/workflows/deploy.yml` is the canonical pipeline.

---

## 📋 Table of Contents

1. [Prerequisites](#-prerequisites)
2. [Quick Start](#-quick-start)
3. [Deployment Modes](#-deployment-modes)
4. [Environment Configuration](#-environment-configuration)
5. [Production Deployment](#-production-deployment)
6. [Kubernetes Deployment](#-kubernetes-deployment)
7. [Monitoring & Observability](#-monitoring--observability)
8. [Troubleshooting](#-troubleshooting)

---

## ✅ Prerequisites

### Required Software

#### Development Mode
- **Docker** 20.10+ ([Install](https://docs.docker.com/get-docker/))
- **Docker Compose** 2.0+ (included with Docker Desktop)
- **Go** 1.25+ ([Install](https://golang.org/dl/))
- **Node.js** 20.19+ or 22.13+ ([Install](https://nodejs.org/))
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
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# 2. Generate local secrets into .env (compose does not start without them)
./scripts/gen-dev-secrets.sh > .env

# 3. Start PostgreSQL and Redis
docker compose up -d postgres redis

# 4. Start backend (in new terminal)
cd apps/backend
export $(grep -v '^#' ../../.env | xargs)
go run cmd/server/main.go

# 5. Start frontend (in new terminal)
cd apps/web
npm install
npm run dev

# 6. Open browser
open http://localhost:3000
```

### Production Deployment (Docker Compose)

```bash
# 1. Clone repository
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# 2. Generate secrets into .env, then set ENVIRONMENT=production and your
#    production values in it (see Environment Configuration). The generated
#    KEYVAULT_MASTER_KEY must keep the same value across restarts.
./scripts/gen-dev-secrets.sh > .env

# 3. Build the backend and dashboard images from this checkout and start
#    every service in docker-compose.yml
docker compose up -d --build

# 4. Create the first administrator once the backend is healthy
docker compose run --rm bootstrap

# 5. Access application
open http://localhost:3000
```

---

## 🔀 Deployment Modes

### Development Mode

**Use Case**: Local development, testing, debugging

```bash
./scripts/gen-dev-secrets.sh > .env
docker compose up -d postgres redis
```

**What it does:**
- ✅ Writes the secrets docker compose requires to `.env`
- ✅ Starts PostgreSQL and Redis
- ❌ Does NOT start backend/frontend containers (run manually)

The backend applies the database migrations when it starts.

**Services Started:**
- PostgreSQL (port 5432)
- Redis (port 6379)

`docker-compose.yml` also defines Elasticsearch, MinIO, NATS, Prometheus, Grafana, Loki and Promtail. Start any of them by name, for example `docker compose up -d grafana`.

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
# .env holds the secrets compose requires, KEYVAULT_MASTER_KEY among them
# (./scripts/gen-dev-secrets.sh > .env writes them); set ENVIRONMENT=production
# in it before you start
docker compose up -d --build
```

**What it does:**
- ✅ Starts all infrastructure services
- ✅ Builds backend Docker image
- ✅ Builds frontend Docker image
- ✅ Starts backend container (port 8080)
- ✅ Starts frontend container (port 3000) once the backend's health check passes

The backend applies the database migrations when it starts. Create the first administrator with `docker compose run --rm bootstrap` once the backend is healthy.

**All Services Running:**
- Frontend (port 3000)
- Backend (port 8080)
- All infrastructure services (PostgreSQL, Redis, etc.)

### Clean Mode

**Use Case**: Reset deployment, remove all containers and volumes

```bash
docker compose down -v
```

**What it does:**
- ❌ Stops all Docker containers
- ❌ Removes all containers
- ❌ Removes the volumes declared in `docker-compose.yml` (⚠️ **DATA LOSS!**)

---

## ⚙️ Environment Configuration

### .env File Structure

`./scripts/gen-dev-secrets.sh > .env` writes the secrets docker compose requires. Here's a complete reference:

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
JWT_ACCESS_TTL=2h            # Access token lifetime
JWT_REFRESH_TTL=168h         # A refresh token unused this long expires; each refresh issues a new one
JWT_SESSION_MAX_AGE=8h       # Longest a sign-in lasts, however often it is refreshed
SDK_TOKEN_FAMILY_MAX_AGE=8760h  # Longest a downloaded SDK's credential lasts from its download, however often it is refreshed

# Key Vault master key: base64-encoded 32 bytes that encrypt agent private keys.
# Required in every environment except development. Generate it once with
# `openssl rand -base64 32` and keep the same value across restarts.
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key

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

`KEYVAULT_MASTER_KEY` is required in every environment except `ENVIRONMENT=development`. Agent private keys are encrypted under it, and each of the server's Ed25519 signing keys that is not provisioned is derived from it (see [Server Signing Keys](#server-signing-keys)). Generate it once with `openssl rand -base64 32`, store it with your other secrets, and keep the same value across restarts and upgrades. With `ENVIRONMENT=production` the server refuses to start without it. In development, a server started without it generates a new key at each start, and anything encrypted under that key cannot be decrypted after a restart. Setting `ENVIRONMENT=development` does not fix a missing key: set the key.

### Secrets Management

**⚠️ NEVER commit `.env` to version control!**

#### Development
- Use auto-generated `.env` file
- Store secrets in `.env` locally

#### Production
- **Option 1**: Environment variables (recommended)
  ```bash
  export JWT_SECRET="$(openssl rand -hex 32)"
  export KEYVAULT_MASTER_KEY="your-stored-key"  # created once with: openssl rand -base64 32
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
    --from-literal=keyvault-master-key="$(openssl rand -base64 32)" \
    --from-literal=database-url=postgresql://...
  ```

- **Option 4**: HashiCorp Vault (advanced)
  ```bash
  vault kv put secret/aim/production \
    jwt_secret=your-secret \
    keyvault_master_key="$(openssl rand -base64 32)" \
    database_url=postgresql://...
  ```

### Server Signing Keys

The backend signs with two Ed25519 keys, one per purpose. A signature made for one purpose does not verify for another.

| Purpose | Variable | Signs | Public key |
|---------|----------|-------|------------|
| `card-attestation` | `AIM_SIGNING_KEY_CARD_ATTESTATION` | A2A agent card attestations | Published at `/.well-known/jwks.json` |
| `atc-issuer` | `AIM_SIGNING_KEY_ATC_ISSUER` | ATCs this server trusts under `ATC_ISSUER_URI` | Published at `/.well-known/jwks.json` |

Each variable holds a base64-encoded 32-byte Ed25519 seed. When a variable is unset, that key is derived from `KEYVAULT_MASTER_KEY` with HKDF-SHA256, so an existing deployment upgrades without new configuration.

**In production, provision both keys.** A derived key changes whenever `KEYVAULT_MASTER_KEY` is rotated, and anyone who holds the master key can recompute it. A provisioned key is independent of the master key and can be rotated on its own:

```bash
# Generate each seed where it will be stored (secret store, Kubernetes secret, Vault).
# Use a different seed for each purpose: the server refuses to start if two purposes share a key.
export AIM_SIGNING_KEY_CARD_ATTESTATION="$(openssl rand -base64 32)"
export AIM_SIGNING_KEY_ATC_ISSUER="$(openssl rand -base64 32)"
```

At startup the backend logs one line per key with its purpose, key ID (`kid`, the hex SHA-256 of the public key) and source (`provisioned` or `derived`). It never logs key material. A malformed value stops startup with an error that names the variable. With `ENVIRONMENT=production`, a derived key also logs which variable to set.

Verify what is published:

```bash
curl -s http://localhost:8080/.well-known/jwks.json
```

The response is a JSON Web Key Set with the active card-attestation and ATC-issuer keys. Each key carries `kid`, `purpose` and `status` (`active` or `retired`); an active key also carries `source`. Card attestations record the `kid` of the key that signed them as `attestationKeyId`. Third parties verify a served card's attestation with this key set alone; the procedure and a conformance vector are in [`docs/specs/card-attestation-v2.md`](specs/card-attestation-v2.md).

**Rotating a key.** Set the variable to a new seed and list the old public key in `<VARIABLE>_RETIRED` (comma-separated). Retired keys still verify and stay in the key set with `status: "retired"` until you remove them. Copy the old key's `x` value from `/.well-known/jwks.json` before you restart.

Remove a retired key once nothing it signed is still valid: card attestations expire after `A2A_ATTESTATION_VALIDITY_HOURS` (24 hours by default).

**Rotating `KEYVAULT_MASTER_KEY` moves every derived key.** Each key whose variable is unset gets a new key and a new `kid` at the next start, and what the old key signed stops verifying. Before rotating the master key, copy the `x` value of the active `card-attestation` and `atc-issuer` keys from `/.well-known/jwks.json` into `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED` and `AIM_SIGNING_KEY_ATC_ISSUER_RETIRED`. Provisioning both keys keeps them fixed when the master key rotates.

### SIEM Forwarding

The backend forwards every audit log entry it writes to a Splunk HTTP Event Collector (HEC) once both `SIEM_SPLUNK_HEC_URL` and `SIEM_SPLUNK_HEC_TOKEN` are set. One collector receives the entries of every organization on the server.

| Variable | Value |
|----------|-------|
| `SIEM_SPLUNK_HEC_URL` | The collector's event endpoint, such as `https://splunk.example.com:8088/services/collector/event`. It must use `https`; `http` is accepted only for a loopback host. |
| `SIEM_SPLUNK_HEC_TOKEN` | The HEC token, sent as `Authorization: Splunk <token>`. |
| `SIEM_SPLUNK_INDEX` | Optional. Unset, events go to the token's default index. |
| `SIEM_MIN_SEVERITY` | Optional. `info` (the default), `low`, `medium`, `high` or `critical`. Entries below it are not sent. |

One of the pair set without the other, an `http` URL to a host that is not loopback, or an unknown severity stops startup with an error that names the variable. With forwarding on, startup logs `siem: forwarding audit log entries at severity <severity> and above to splunk`.

Each entry is one HEC event with sourcetype `opena2a:aim` and source `opena2a-aim`. Its `event` is:

```json
{
  "eventType": "aim.audit.verification_refused",
  "timestamp": "2026-10-11T18:00:00Z",
  "agentId": "<agent id, when an agent acted>",
  "severity": "medium",
  "source": "opena2a-aim",
  "details": {
    "auditLogId": "<audit log entry id>",
    "organizationId": "<organization id>",
    "action": "verification_refused",
    "actorType": "agent",
    "resourceType": "agent",
    "resourceId": "<resource id>",
    "userId": "<user id, when recorded>",
    "ipAddress": "<when recorded>",
    "userAgent": "<when recorded>",
    "metadata": {"<key>": "<the entry's metadata, when it has any>"}
  }
}
```

`eventType` is `aim.audit.` followed by the entry's action. `severity` is `high` for `honeytoken_triggered` and `refresh_token_reuse`, `medium` for `verification_refused`, `credential_mint_refused` and `refresh_session_revoked`, and `info` for every other action.

Delivery runs in the background, so a request that writes an entry never waits for the collector. Entries wait in a queue of 1,000 and are sent in batches of up to 100, at least every 5 seconds. A batch is tried up to 3 times, 1 second and then 2 seconds apart, when the collector cannot be reached or answers 408, 429 or 5xx. Any other refusal, such as a token the collector does not accept, is not retried. Each batch that is not delivered is logged with the collector's answer. When the queue is full, the entry is not forwarded, and a warning is logged for the first such entry and every thousandth after it; the entry is still in the audit log. At shutdown the queued entries are sent within 2 seconds, and the backend logs how many were sent, failed and dropped.

To check delivery, search Splunk for `sourcetype="opena2a:aim"`.

Microsoft Sentinel is not supported: Microsoft ended support for the Azure Monitor HTTP Data Collector API, which the source tree's Sentinel adapter targets, on 14 September 2026. A Sentinel adapter for the Logs Ingestion API is planned.

### Database Migrations and the Application Role

`aim-server migrate` applies the pending migrations and exits without starting the server. It reads only the `POSTGRES_*` variables and is meant to run as the role that owns the schema. A missing `POSTGRES_HOST`, `POSTGRES_USER` or `POSTGRES_DB` stops it with one line that names each missing variable.

With `POSTGRES_APP_USER` and `POSTGRES_APP_PASSWORD` set, it first creates or updates that role for the server to connect as: a login role that is not a superuser, has no BYPASSRLS and cannot create objects in schema `public`, with read and write access to the tables and sequences the migrations create. Two effects reach beyond that role:

- **CREATE on schema `public` is revoked from `PUBLIC` as well as from the application role.** PostgreSQL before 15 grants CREATE on schema `public` to `PUBLIC`, and every role holds what `PUBLIC` holds, so a revoke from the application role alone would leave it able to create tables. On such a server every role that has no grant of its own loses CREATE on `public`; grant it back to a role that needs it (`GRANT CREATE ON SCHEMA public TO <role>`). PostgreSQL 15 and later does not grant it to `PUBLIC`, and the revoke changes nothing there. The command logs the revoke after provisioning.
- **The password travels in the statement.** The role's password is set with `CREATE ROLE ... PASSWORD '<literal>'` or `ALTER ROLE ... PASSWORD '<literal>'`. With `log_statement` set to `ddl` or `all`, PostgreSQL writes that statement, password included, to its server log. Run the command while statement logging is below `ddl`, or set the password again afterwards from a client that sends a SCRAM verifier instead of the password, such as `\password <role>` in `psql`.

**Upgrading across the agent private key format change.** A release that stores agent private keys in format v2 converts the stored v1 keys once at startup and records completion in `schema_migrations` as `keyvault/agent-private-key-v2`; later starts skip the conversion. An instance of the earlier release that is still running keeps writing v1 keys, and a key written after completion is never converted and is refused on the request path until that agent's credentials are rotated. Stop every instance of the earlier release before the first instance of the new release starts.

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
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Create .env with the secrets docker compose requires
./scripts/gen-dev-secrets.sh > .env
nano .env  # Edit with production values
```

#### 3. Deploy

```bash
# Build the images from this checkout and start every service
docker compose up -d --build

# Create the first administrator once the backend is healthy
docker compose run --rm bootstrap

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
```

The repository ships no reverse-proxy configuration. Run a TLS-terminating reverse proxy (nginx, Caddy, Traefik or a cloud load balancer) in front of the dashboard on port 3000 and the API on port 8080, using the certificate from the previous command.

---

## ☸️ Kubernetes Deployment

### Prerequisites

- **Kubernetes cluster** (EKS, GKE, AKS, or self-hosted)
- **kubectl** configured
- **Helm** 3+ (optional)

### Deploy with kubectl

This repository ships no Kubernetes manifests. The steps below apply manifests you write yourself, kept in a directory shown here as `<manifests>`: a Deployment and a Service for each of the `ghcr.io/opena2a-org/aim-server` and `ghcr.io/opena2a-org/aim-dashboard` images, and an Ingress.

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

# Or deploy from your own manifests
kubectl apply -f <manifests>/postgres.yaml
kubectl apply -f <manifests>/redis.yaml
```

#### 4. Deploy AIM

```bash
# Apply your Kubernetes manifests
kubectl apply -f <manifests>/backend-deployment.yaml
kubectl apply -f <manifests>/frontend-deployment.yaml
kubectl apply -f <manifests>/ingress.yaml

# Check status
kubectl get pods -n aim-production
kubectl get services -n aim-production
kubectl get ingress -n aim-production
```

#### 5. Configure Ingress

```yaml
# <manifests>/ingress.yaml
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
# <manifests>/hpa.yaml
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
# Clean everything and start fresh (removes the volumes: all data is lost)
docker compose down -v
rm .env
./scripts/gen-dev-secrets.sh > .env
docker compose up -d postgres redis
```

---

## Install from this repository

> The `docker pull` command shown in the box on the `aim-dashboard` and `aim-server` package pages names a `sha256-<hex>` tag that holds the build's SLSA provenance attestation, not the image; with Docker 29.2 that pull fails with `unsupported media type application/vnd.oci.empty.v1+json` and installs nothing (measured 2026-09-24). Install through the Docker steps below; they pin and verify nothing.

### Docker

The quickstart script is fetched from `main`. On a first run it downloads [`docker-compose.quickstart.yml`](../docker-compose.quickstart.yml), also from `main`, and starts `aim-server` and `aim-dashboard` at their `latest` tags. `latest` moves only when a platform release is published; merges to `main` do not move it. Until the first release after this change, `latest` names the last build of `main` published before this change landed, whose version label reads `edge`. PostgreSQL (the TimescaleDB image) and Redis start at floating upstream tags. Nothing on this path is pinned or verified: read [the script](../scripts/quickstart.sh) and the compose file it fetches before running it. Published security advisories are on the repository's [security advisories page](https://github.com/opena2a-org/agent-identity-management/security/advisories).

```bash
curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
bash quickstart.sh
```

Brings up `aim-server`, `aim-dashboard`, PostgreSQL, and Redis. Dashboard at `localhost:3000`, API at `localhost:8080`. Login credentials print at the end of the run.

The host ports and the container names come from the environment, with the values below as defaults. Set a variable before `bash quickstart.sh` (or in the `.env` next to `docker-compose.quickstart.yml`) to run AIM beside another service on the same port, or to run a second AIM stack on one host. A first run records the values it ran with in the install's `.env`, so a later `docker compose` in that directory keeps them. A run over an existing install starts the stack with the variables set for that run and leaves that `.env` as it is, so a later `docker compose` in that directory reads the values `.env` holds; to keep a new value, change it in that `.env`.

| Variable | Default | Sets |
|---|---|---|
| `AIM_BACKEND_PORT` | `8080` | host port of the API (`aim-backend`) |
| `AIM_FRONTEND_PORT` | `3000` | host port of the dashboard (`aim-frontend`) |
| `AIM_POSTGRES_PORT` | `5432` | host port of PostgreSQL (`aim-postgres`), bound to `127.0.0.1` |
| `AIM_REDIS_PORT` | `6379` | host port of Redis (`aim-redis`), bound to `127.0.0.1` |
| `AIM_PROJECT` | `aim` | prefix of the four container names |

A second stack beside the default one, in its own directory and Compose project, with its own container names and ports:

```bash
AIM_DIR=aim2 AIM_PROJECT=aim2 AIM_BACKEND_PORT=8081 AIM_FRONTEND_PORT=3001 AIM_POSTGRES_PORT=5433 AIM_REDIS_PORT=6380 bash quickstart.sh
```

Compose names the volumes and the network after its project (`COMPOSE_PROJECT_NAME`, or the install directory's name), so two stacks also need two projects; `AIM_DIR` gives each its own directory and project.

Production deployment (Azure, GCP, AWS): [infrastructure/DEPLOYMENT.md](../infrastructure/DEPLOYMENT.md).

Merges to `main` no longer publish images: only a push of a `platform-vX.Y.Z` tag does, and it publishes `X.Y.Z`, `X.Y`, `X` and `latest` for both images on both registries. The `edge` tag is frozen at the last build of `main` published before this change and will be removed; to follow `main`, build from source (next section). This change does not move `1`, `1.0` or `1.0.0`. A running image is identified by its digest and its revision label (`org.opencontainers.image.revision`), not by the tag it was pulled by.

New accounts wait for an administrator's approval. To bootstrap the first administrator, set `AIM_PLATFORM_ADMINS` (comma-separated emails) before starting the backend: accounts on that list are approved automatically and approve everyone else from the dashboard's admin area. Until a listed address has registered, or some administrator exists, other sign-ups are refused with an error that says so; nothing is queued. The backend logs how the variable was read at startup, so a mistyped entry is visible there.

The `aim-server` and `aim-dashboard` image tags `1.23`, `1.23.0`, `1.5` and `1.5.0`, and the `aim-server` tags `1.22` and `1.22.0`, are withdrawn: their numbers sort above `1.0.0`, but no platform release has used them. If a pull of one of them fails with `manifest unknown`, it is because the tag was removed from the registry. The images behind them bundle components with critical-rated vulnerabilities, described in [GHSA-rqgr-f9m6-xphr](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-rqgr-f9m6-xphr) for the dashboard and [GHSA-fxfm-hhhw-3mgf](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-fxfm-hhhw-3mgf) for the server. The dashboard images are also affected by [GHSA-638x-gwq7-xfh7](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-638x-gwq7-xfh7), rated high. Each of these advisories lists the images it affects and shows how to check a deployment.

### From source

Prerequisites: Docker, Go 1.25+, Node.js 20.19+ or 22.13+, Python 3.11+.

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Generate local-dev secrets (one-time setup)
./scripts/gen-dev-secrets.sh > .env

# Minimal dev stack
docker compose up -d postgres redis backend frontend

# Health check
curl -fsS localhost:8080/health

# Python SDK editable for examples
pip install -e sdk/python

# Try the flight-search-agent demo
cd examples/flight-search-agent && python3 flight_agent.py
```

The full `docker-compose.yml` also brings up Elasticsearch, MinIO, NATS, Prometheus, Grafana, and Loki. Skip those services with the minimal command above.

`docker-compose.yml` takes `AIM_PROJECT` (`aim`, the prefix of every container name) and the quick-start's `AIM_BACKEND_PORT` (`8080`), `AIM_FRONTEND_PORT` (`3000`), `AIM_POSTGRES_PORT` (`5432`) and `AIM_REDIS_PORT` (`6379`) with the same defaults, plus one host port per other service: `AIM_ELASTICSEARCH_PORT` (`9200`), `AIM_ELASTICSEARCH_TRANSPORT_PORT` (`9300`), `AIM_MINIO_PORT` (`9000`), `AIM_MINIO_CONSOLE_PORT` (`9001`), `AIM_NATS_PORT` (`4222`), `AIM_NATS_MONITOR_PORT` (`8222`), `AIM_PROMETHEUS_PORT` (`9090`), `AIM_GRAFANA_PORT` (`3003`), `AIM_LOKI_PORT` (`3100`). Set them in `.env` to run the dev stack beside a quick-start or another checkout on one host; each stack also needs its own Compose project (`COMPOSE_PROJECT_NAME`, or the checkout directory's name).

## Observability

The backend emits OpenTelemetry traces, metrics, and logs. A hermetic demo stack lives at [`apps/backend/deployments/otel-demo/`](../apps/backend/deployments/otel-demo/): OpenTelemetry Collector, Tempo, Prometheus, Loki, Grafana.

```bash
cd apps/backend/deployments/otel-demo
docker compose up -d
./smoke-backend.sh
```

Every `fga.authorize` decision lands as a parent span with one child span per FGA step it ran: up to five on `POST /api/v1/agents/:id/authorize` (up to four when the policy's risk level is LOW), and one, `fga.capability_check`, when the agent has no FGA policy for the capability or the decision is an SDK verification's. 9 SemConv attributes ride the parent: `agent.id`, `agent.public_key.algorithm`, `agent.trust_score`, `agent.drift_score`, `agent.scan_verdict`, `agent.capability`, `fga.step`, `fga.outcome`, `fga.denied_by`. The attribute set is proposed to the OpenTelemetry Semantic Conventions WG. Full design notes: [`apps/backend/docs/OBSERVABILITY.md`](../apps/backend/docs/OBSERVABILITY.md).

---

## 📞 Support

- **Documentation**: https://opena2a.org/docs
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discord**: https://discord.gg/opena2a
- **Email**: info@opena2a.org

---

**🛡️ Happy Deploying with AIM!**
