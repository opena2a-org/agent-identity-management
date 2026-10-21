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
# HOST is the address the API listens on. Unset, it is 127.0.0.1 outside a
# container and 0.0.0.0 in one; the image built from
# infrastructure/docker/Dockerfile.backend sets HOST=0.0.0.0. Set HOST=0.0.0.0
# where other machines connect to the API directly.
# HOST=127.0.0.1
APP_PORT=8080
ENVIRONMENT=production  # development | production | testing

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_ACCESS_TTL=2h            # Access token lifetime
JWT_REFRESH_TTL=168h         # A refresh token unused this long expires; each refresh issues a new one
JWT_SESSION_MAX_AGE=8h       # Longest a sign-in lasts, however often it is refreshed
SDK_TOKEN_FAMILY_MAX_AGE=8760h  # Longest a downloaded SDK's credential lasts from its download, however often it is refreshed

# Key Vault master key: base64-encoded 32 bytes that encrypt agent private keys.
# Required in every environment except development and test. Generate it once
# with `openssl rand -base64 32` and keep the same value across restarts.
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key

# CORS Configuration
ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

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

`KEYVAULT_MASTER_KEY` is required in every environment except `ENVIRONMENT=development` and `ENVIRONMENT=test`. Agent private keys are encrypted under it, and each of the server's Ed25519 signing keys that is not provisioned is derived from it (see [Server Signing Keys](#server-signing-keys)). Generate it once with `openssl rand -base64 32`, store it with your other secrets, and keep the same value across restarts and upgrades. To replace it, follow [Rotating `KEYVAULT_MASTER_KEY`](#rotating-keyvault_master_key). Without it, the server refuses to start unless `ENVIRONMENT` is exactly `development` or `test`: unset, empty, `production`, `staging`, `Development` and `testing` all refuse. With `ENVIRONMENT=development` or `ENVIRONMENT=test`, a server started without it generates a new key at each start, and anything encrypted under that key, stored agent private keys included, cannot be decrypted after a restart. `ENVIRONMENT=test` is for the repository's own tests and also raises both rate limits tenfold, so never set it in a deployment. Setting `ENVIRONMENT=development` or `ENVIRONMENT=test` does not fix a missing key: set the key.

### Rotating `KEYVAULT_MASTER_KEY`

The server encrypts each agent private key it stores under a data key of its own, and stores the data key encrypted under a master key; the stored key carries in its header the storage key id of that master key. A key stored in format v2, before data keys, is encrypted under the master key itself and carries its id the same way. The server holds a ring of master keys: it decrypts under any id in the ring, encrypts only under the current id, and refuses to decrypt a key stored under an id outside the ring (`unknown storage key id <id>`).

| Variable | Holds |
|----------|-------|
| `KEYVAULT_MASTER_KEY` | The current master key. The data key of every private key the server stores, or moves to the current id, is encrypted under it. |
| `KEYVAULT_MASTER_KEY_ID` | The storage key id of `KEYVAULT_MASTER_KEY`, a number from 1 to 255. Unset, it is 1, the id of every key a release without the ring stored in format v2. |
| `KEYVAULT_MASTER_KEY_<id>` | Another master key in the ring under that id, for example `KEYVAULT_MASTER_KEY_1`. The server decrypts under it and never encrypts under it. |

The server refuses to start when `KEYVAULT_MASTER_KEY_ID` is not a number from 1 to 255, when a `KEYVAULT_MASTER_KEY_<id>` names the current id, when a `KEYVAULT_MASTER_KEY_<id>` is set without `KEYVAULT_MASTER_KEY`, when a key in the ring is malformed (for a `KEYVAULT_MASTER_KEY_<id>`, the error names that variable), and when two ids hold the same key.

To rotate from id 1 to id 2:

1. Generate the new key with `openssl rand -base64 32` and store it beside the current one. Keep the current key.
2. With more than one instance, first set `KEYVAULT_MASTER_KEY_2` to the new key on every instance and restart them, leaving `KEYVAULT_MASTER_KEY` and `KEYVAULT_MASTER_KEY_ID` as they are. Every instance can then read keys stored under id 2 before any instance stores one. A single instance can skip this step.
3. Make the new key current: set `KEYVAULT_MASTER_KEY` to the new key, `KEYVAULT_MASTER_KEY_ID=2` and `KEYVAULT_MASTER_KEY_1` to the old key, remove `KEYVAULT_MASTER_KEY_2`, and restart. Derived signing keys come from `KEYVAULT_MASTER_KEY`, so this step moves them: read [Server Signing Keys](#server-signing-keys) first.
4. At each start with more than one key in the ring, after the agent private key format conversion and before the server accepts requests, a sweep moves each stored private key under another id in the ring to the current id: it encrypts the key's data key under the current master key and leaves the encrypted private key as it is, and it encrypts a key in format v2 again in format v3, under a data key of its own. Each row is rewritten on its own, so a sweep that stops part way continues at the next start. It logs one line of counts and no agent IDs or key material:

   ```text
   Agent private key sweep to storage key 2: alreadyCurrent=0 rewritten=3 failures=0 notV2=0 remaining=1:0
   ```

   `remaining` gives, for each other id in the ring and for each id outside the ring that a stored key was found under, how many stored keys are still under that id. `failures` counts keys under an older id in the ring that were not moved: unreadable for their agent row, changed since the sweep read them, or not written. They stay in `remaining` and are read again at the next start. `notV2` counts stored values with no storage key id.
5. Remove `KEYVAULT_MASTER_KEY_1` once a start made after every instance runs the configuration of step 3 logs `remaining=1:0`. Disable the old key in your secret store rather than deleting it, and keep it until every database backup taken before the sweep has expired: restoring one of those backups needs it.

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

### Email

The backend sends email (the welcome, account-approved and password-reset messages, and the deliveries of `email` alert channels, see [`docs/API.md`](API.md) § Alert channels) through one provider, chosen by `EMAIL_PROVIDER`:

| `EMAIL_PROVIDER` | Sends through |
|------------------|---------------|
| `azure` (the default when `EMAIL_PROVIDER` is unset) | Azure Communication Services Email |
| `smtp` | An SMTP server, with STARTTLS |
| `console` | Nothing: each message is printed to the backend's standard output. For development only. |

Any other value is refused at startup.

Variables every provider reads:

| Variable | Default | Meaning |
|----------|---------|---------|
| `EMAIL_FROM_ADDRESS` | none, required | The sender address. |
| `EMAIL_FROM_NAME` | `Agent Identity Management` | The sender's display name. |
| `EMAIL_TEMPLATES_DIR` | unset | A directory of templates that replace the built-in ones. A directory that does not load logs a warning, and the built-in templates are used. |
| `EMAIL_RATE_LIMIT_PER_MINUTE` | `60` | Read, but no provider applies it. |

`azure` reads:

| Variable | Default | Meaning |
|----------|---------|---------|
| `AZURE_EMAIL_CONNECTION_STRING` | none, required | The resource's connection string, `endpoint=https://...;accesskey=...`. |
| `AZURE_EMAIL_POLLING_ENABLED` | `false` | Read, but not used. |

`smtp` reads:

| Variable | Default | Meaning |
|----------|---------|---------|
| `SMTP_HOST` | none, required | The SMTP server's host name. The server's certificate is checked against this name. |
| `SMTP_PORT` | `587` | The port to connect to. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | unset | Sent with `AUTH PLAIN` after STARTTLS. Authentication is used only when both are set. |
| `SMTP_TLS_ENABLED` | `true` | Use STARTTLS (see below). |
| `SMTP_MAX_CONNECTIONS` | `10` | Read, but not used. |

With `SMTP_TLS_ENABLED=true`, the backend connects to `SMTP_HOST` on `SMTP_PORT` in clear text, sends `EHLO` and then `STARTTLS`, and verifies the server's certificate against the system's trusted roots before it sends `AUTH` or any message. When the server offers no STARTTLS, or the certificate does not verify, the backend refuses to send: neither the credentials nor the message leave in clear text. Implicit TLS (SMTPS, port 465), where the connection starts with a TLS handshake, is not supported. Every message carries a `Message-ID` of the form `<32 hex characters@domain of EMAIL_FROM_ADDRESS>`.

`SMTP_TLS_ENABLED=false` is meant for a relay on the same host or network. The message is then sent with Go's `net/smtp` `SendMail`, which still uses STARTTLS when the server offers it, and sends credentials only over TLS or to `localhost`, `127.0.0.1` or `::1`.

At startup the backend builds the provider and checks it once:

- `smtp` connects, runs STARTTLS when `SMTP_TLS_ENABLED` is true, and authenticates when a username and password are set.
- `azure` checks that the connection string has an `https://` endpoint and an access key. It does not contact Azure, so a wrong access key shows only when a message is sent.
- `console` always passes.

When building or checking the provider fails, for example because `EMAIL_FROM_ADDRESS` is unset or the SMTP server cannot be reached, the backend logs `Email service initialization failed: <reason>` and `AIM will continue without email notifications`, and keeps running without email. On success it logs `Email service initialized (provider: <provider>, from: <address>)`. Check which one happened:

```bash
curl -s http://localhost:8080/api/v1/status
```

`services.email` is `healthy` when the email service started and `unavailable` when the server runs without email. Because `azure` is the default, a server started with no email variables runs without email and reports `unavailable`. Set `EMAIL_PROVIDER` and that provider's variables, then restart the backend to turn email on.

### SIEM Forwarding

The backend forwards security events to a Splunk HTTP Event Collector (HEC) once both `SIEM_SPLUNK_HEC_URL` and `SIEM_SPLUNK_HEC_TOKEN` are set, and to Microsoft Sentinel through the Azure Monitor Logs Ingestion API once the six required `SIEM_SENTINEL_*` variables are set (see Microsoft Sentinel below). With both configured, each event goes to both. One collector receives the events of every organization on the server. Four kinds of event are forwarded:

| Event | `eventType` | `severity` |
|-------|-------------|------------|
| Every audit log entry the backend writes | `aim.audit.<action>` | `high`, `medium` or `info`, by action (see below) |
| Every password sign-in that `POST /api/v1/public/login` or `POST /api/v1/auth/login/local` answers 401, and each one they answer 403 because the user's organization requires sign-in through its identity provider or 500 because that requirement could not be read | `aim.auth.signin_failed` | `low` |
| Every FGA decision of `POST /api/v1/agents/:id/authorize` that does not allow the request | `aim.fga.denied` | `medium` |
| Every alert the backend raises | `aim.alert.created` | `info`, `medium`, `high` or `critical`, by the alert's severity (see below) |

A failed sign-in and an FGA denial write no audit log entry, so each is forwarded as an event of its own. An allowed FGA decision is not forwarded. An alert is forwarded when its row is written, whichever part of the backend raised it; a repeat that is counted on an alert still open (the alert's `occurrenceCount`) is not forwarded again.

The audit log entries include a verification refused because the agent's status does not let it act (`aim.audit.verification_refused`) and each secrets management request that succeeds, written in the caller's organization under the caller's user id, with the namespace's id as `resourceId` and the namespace's backend as the only metadata (`{"backendType": "aim_native"}`); the credential is never in the entry:

| Request | `eventType` | `resourceType` |
|---------|-------------|----------------|
| `POST /api/v1/secrets/namespaces` | `aim.audit.create` | `secret_namespace` |
| `POST /api/v1/secrets/namespaces/:id/credentials` | `aim.audit.create` | `secret_credential` |
| `POST /api/v1/secrets/namespaces/:id/rotate` | `aim.audit.update` | `secret_credential` |
| `DELETE /api/v1/secrets/namespaces/:id` | `aim.audit.delete` | `secret_namespace` |

A secrets request that is refused or fails writes no entry.

| Variable | Value |
|----------|-------|
| `SIEM_SPLUNK_HEC_URL` | The collector's event endpoint, such as `https://splunk.example.com:8088/services/collector/event`. It must use `https`; `http` is accepted only for a loopback host. |
| `SIEM_SPLUNK_HEC_TOKEN` | The HEC token, sent as `Authorization: Splunk <token>`. |
| `SIEM_SPLUNK_INDEX` | Optional. Unset, events go to the token's default index. |
| `SIEM_MIN_SEVERITY` | Optional. `info` (the default), `low`, `medium`, `high` or `critical`. Events below it are not sent: at `medium`, failed sign-ins are not sent. |

One of the pair set without the other, an `http` URL to a host that is not loopback, or an unknown severity stops startup with an error that names the variable. With forwarding on, startup logs `siem: forwarding audit log entries at severity <severity> and above to <adapters>`, where `<adapters>` is `splunk`, `sentinel` or `splunk, sentinel`, then `siem: forwarding alerts (aim.alert.created)` and `siem: forwarding failed sign-ins (aim.auth.signin_failed) and FGA denials (aim.fga.denied)`.

Each event is one HEC event with sourcetype `opena2a:aim` and source `opena2a-aim`. The `event` of an audit log entry is:

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
    "resourceType": "agent_action",
    "resourceId": "<resource id>",
    "userId": "<user id, when recorded>",
    "ipAddress": "<when recorded>",
    "userAgent": "<when recorded>",
    "metadata": {"<key>": "<the entry's metadata, when it has any>"}
  }
}
```

`eventType` is `aim.audit.` followed by the entry's action. `severity` is `high` for `honeytoken_triggered` and `refresh_token_reuse`, `medium` for `verification_refused`, `credential_mint_refused` and `refresh_session_revoked`, and `info` for every other action.

The `event` of a failed sign-in is:

```json
{
  "eventType": "aim.auth.signin_failed",
  "timestamp": "2026-10-16T18:00:00Z",
  "severity": "low",
  "source": "opena2a-aim",
  "details": {
    "method": "password",
    "reason": "password_mismatch",
    "route": "/api/v1/auth/login/local",
    "ipAddress": "<the caller's address>",
    "userAgent": "<when sent>",
    "userId": "<only when the submitted email matched a user>",
    "organizationId": "<only when the submitted email matched a user>"
  }
}
```

The event never carries the submitted email, the password or a token, and the caller's answer is unchanged: an unknown email and a wrong password are still answered alike. `reason` names the check that refused the sign-in, and Answer is the status the route answered:

| Route | Answer | `reason` | Check |
|-------|--------|----------|-------|
| `POST /api/v1/public/login` | 401 | `account_deactivated` | The user is deactivated or deleted. |
| `POST /api/v1/public/login` | 401 | `registration_rejected` | The email and password match a rejected registration request. |
| `POST /api/v1/public/login` | 401 | `email_or_password_mismatch` | No user or registration request has the email and password. |
| `POST /api/v1/public/login` | 403 | `provider_sign_in_required` | The password is the user's, the user is not an administrator, and the user's organization requires sign-in through its identity provider. |
| `POST /api/v1/public/login` | 500 | `sign_in_refused` | The password is the user's, the user is not an administrator, and whether the user's organization requires sign-in through its identity provider could not be read. |
| `POST /api/v1/auth/login/local` | 401 | `account_locked` | The email is locked out after repeated failures. |
| `POST /api/v1/auth/login/local` | 401 | `unknown_email` | No user with the email was read. |
| `POST /api/v1/auth/login/local` | 401 | `account_deactivated` | The user is deactivated or deleted. |
| `POST /api/v1/auth/login/local` | 401 | `password_not_set` | The user has no password. |
| `POST /api/v1/auth/login/local` | 401 | `password_mismatch` | The password is not the user's. |
| `POST /api/v1/auth/login/local` | 403 | `provider_sign_in_required` | The password is the user's, the user is not an administrator, and the user's organization requires sign-in through its identity provider. |
| `POST /api/v1/auth/login/local` | 500 | `sign_in_refused` | The password is the user's, the user is not an administrator, and whether the user's organization requires sign-in through its identity provider could not be read. |

A request answered 400, without an email or a password or with a body that does not parse, is not forwarded.

The `event` of an FGA denial is:

```json
{
  "eventType": "aim.fga.denied",
  "timestamp": "2026-10-16T18:00:00Z",
  "agentId": "<agent id>",
  "capability": "<the requested capability>",
  "outcome": "DENY",
  "severity": "medium",
  "source": "opena2a-aim",
  "details": {
    "organizationId": "<the agent's organization>",
    "deniedBy": "capability_check",
    "deniedReason": "Agent does not have the requested capability"
  }
}
```

`outcome` is `DENY`, `DENY_ATTRIBUTE`, `DENY_CONTEXT` or `DENY_CHAIN`, as in the authorize answer, or `ERROR` when the engine failed and the route answered 500; `deniedReason` then holds the error.

The `event` of an alert is:

```json
{
  "eventType": "aim.alert.created",
  "timestamp": "2026-10-16T18:00:00Z",
  "agentId": "<agent id, when the alert names an agent>",
  "severity": "high",
  "source": "opena2a-aim",
  "details": {
    "alertId": "<alert id>",
    "organizationId": "<organization id>",
    "alertType": "security_breach",
    "resourceType": "<when the alert has one, such as agent>",
    "resourceId": "<when the alert has one>"
  }
}
```

`severity` is `info`, `medium`, `high` or `critical` for the alert severities `info`, `warning`, `high` and `critical`. `agentId` is the alert's resource when that resource is an agent, or the agent the alert's metadata names, as a honeytoken alert's does. The event does not carry the alert's title, description, source address or metadata; read them with `GET /api/v1/admin/alerts` or in the dashboard.

Delivery runs in the background, so a request never waits for the collector. Events wait in a queue of 1,000 and are sent in batches of up to 100, at least every 5 seconds. A batch is tried up to 3 times, 1 second and then 2 seconds apart, when the collector cannot be reached or answers 408, 429 or 5xx. Any other refusal, such as a token the collector does not accept, is not retried. Each batch that is not delivered is logged with the collector's answer. When the queue is full, the event is not forwarded, and a warning is logged for the first such event and every thousandth after it; an audit log entry is still in the audit log. At shutdown the queued events are sent within 2 seconds, and the backend logs how many were sent, failed and dropped.

Forwarding loses events within a stated bound and keeps no copy of its own. A batch refused on its third attempt, about 3 seconds after the first, is counted failed and is not sent again; a batch refused in a way that is not retried is counted failed at its first attempt. An event that finds 1,000 events already queued is not queued. Nothing is written to disk to send later, and HEC indexer acknowledgment is not requested: an answer below 400 counts as delivered. Where the record of a lost event survives depends on its kind:

| Event | Record outside the SIEM |
|-------|-------------------------|
| Audit log entry (`aim.audit.<action>`) | The entry stays in the audit log: it is forwarded only after it is written. |
| Alert (`aim.alert.created`) | The alert stays in the `alerts` table: it is forwarded only after it is written. |
| Failed sign-in (`aim.auth.signin_failed`) | On `POST /api/v1/auth/login/local`, the reasons `unknown_email`, `account_deactivated` and `password_mismatch` also write an `auth_failures` row, which the lockout count reads and a later successful `POST /api/v1/auth/login/local` or `POST /api/v1/auth/reauth` with that email deletes; a successful `POST /api/v1/public/login` does not delete it. The reasons `account_locked`, `password_not_set`, `provider_sign_in_required` and `sign_in_refused`, and every refused `POST /api/v1/public/login`, are not stored by AIM outside the SIEM stream. |
| FGA denial (`aim.fga.denied`) | When an FGA policy covers the agent and capability, the denial also writes an `access_attestations` row with its outcome. A denial without a policy, and an `ERROR` outcome, are not stored by AIM outside the SIEM stream. |

The loss shows in three places:

- `GET /health/ready` carries `dependencies.siem`, with `"required": false`. It is `notConfigured` without a SIEM. It is `unavailable`, with `"reason": "dependency check failed"`, while the most recent batch to the collector was not delivered after its attempts, or an event was dropped after the last delivered batch; the response then has `"degraded": true` and is still answered 200, because the SIEM is optional. It is `ok` otherwise, including before the first batch. The entry reads the backend's record of its own deliveries and drops and sends nothing to the collector, so it changes only when a batch is sent or an event is dropped: the next delivered batch returns it to `ok`. Check it with `curl -s http://localhost:8080/health/ready`.
- `/metrics` carries three counters, in events, while a SIEM is configured: `aim_siem_events_sent_total{adapter="splunk"}`, `aim_siem_events_failed_total{adapter="splunk"}` and `aim_siem_events_dropped_total`, with a second `sent` and `failed` pair labelled `adapter="sentinel"` for Microsoft Sentinel. Without a SIEM there are no `aim_siem_` series. Check them on the metrics listener's default address with `curl -s http://127.0.0.1:9464/metrics | grep '^aim_siem_'`; with `METRICS_AUTH_TOKEN` set, add `-H "Authorization: Bearer $METRICS_AUTH_TOKEN"`.
- Each batch that is not delivered logs `ERROR SIEM delivery failed adapter=splunk events=<count>` (or `adapter=sentinel`), followed by the collector's answer.

To check delivery, search Splunk for `sourcetype="opena2a:aim"`.

#### Microsoft Sentinel

The backend sends events to Microsoft Sentinel through the Azure Monitor Logs Ingestion API, as records of a data collection rule (DCR) stream that the rule writes to a Log Analytics table. It does not use the HTTP Data Collector API.

| Variable | Value |
|----------|-------|
| `SIEM_SENTINEL_ENDPOINT` | The logs ingestion endpoint the records are posted to: `{endpoint}` in `POST {endpoint}/dataCollectionRules/...`. It must use `https`; `http` is accepted only for a loopback host. |
| `SIEM_SENTINEL_DCR_IMMUTABLE_ID` | The data collection rule's `immutableId` property, the id the ingestion URL names the rule by. |
| `SIEM_SENTINEL_STREAM_NAME` | The stream the rule declares for these records, such as `Custom-AIMSecurityEvents`. |
| `SIEM_SENTINEL_TENANT_ID` | The Microsoft Entra tenant id of the application the backend signs in as. |
| `SIEM_SENTINEL_CLIENT_ID` | The application (client) id. |
| `SIEM_SENTINEL_CLIENT_SECRET` | A client secret of the application. It is sent only in the token request, and no error or log line carries it or a token. |
| `SIEM_SENTINEL_AUTHORITY_HOST` | Optional. The host the token is requested from; `https://login.microsoftonline.com` when unset. It must use `https`; `http` is accepted only for a loopback host. The token is always requested for `https://monitor.azure.com`. |

When none of the six required variables is set, Sentinel is not configured, even with `SIEM_SENTINEL_AUTHORITY_HOST` set. When some but not all are set, startup stops with an error that names each missing variable and quotes no value.

To create the application, register it in Microsoft Entra ID, add a client secret, and give it permission to send data to the rule. Microsoft's [Logs Ingestion API overview](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/logs-ingestion-api-overview) covers the rule, the endpoint and the role assignment the application needs.

For a batch, the backend asks for a token with the client credentials grant, `POST <authority host>/<tenant id>/oauth2/v2.0/token` with scope `https://monitor.azure.com/.default`, and keeps it until 60 seconds before its `expires_in` runs out, so one token serves many batches. It posts the batch as one JSON array of records to `<endpoint>/dataCollectionRules/<DCR immutableId>/streams/<stream name>?api-version=2023-01-01` with `Authorization: Bearer <token>`. Any 2xx answer counts as delivered. A token request or upload that cannot reach its host, or that is answered 408, 429 or 5xx, is retried as described above. Any other answer is not retried, such as a refused token request (`invalid_client` for a wrong client secret), a 2xx token answer without a bearer `access_token` or a positive `expires_in`, or an upload answered 400, 401 or 403. After a 401 or 403 from the upload, the next batch asks for a new token. A redirect is not followed, because the token request carries the client secret.

Each record has these columns. Declare them in the rule's stream declaration, and give the destination table the same columns:

| Column | Type | Value |
|--------|------|-------|
| `TimeGenerated` | `datetime` | The event's time, RFC 3339 in UTC. |
| `EventType` | `string` | The event's `eventType`, such as `aim.auth.signin_failed`. |
| `Severity` | `string` | The event's `severity`. |
| `AgentId` | `string` | The event's `agentId`; empty when the event names no agent. |
| `Capability` | `string` | The event's `capability`; empty when it has none. |
| `Outcome` | `string` | The event's `outcome`; empty when it has none. |
| `Source` | `string` | `opena2a-aim`. |
| `Details` | `dynamic` | The event's `details`, an object; `{}` when it has none. |

```json
[
  {"name": "TimeGenerated", "type": "datetime"},
  {"name": "EventType", "type": "string"},
  {"name": "Severity", "type": "string"},
  {"name": "AgentId", "type": "string"},
  {"name": "Capability", "type": "string"},
  {"name": "Outcome", "type": "string"},
  {"name": "Source", "type": "string"},
  {"name": "Details", "type": "dynamic"}
]
```

A refused token shows where a refused Splunk token does: `dependencies.siem` in `GET /health/ready` is `unavailable` and the response has `"degraded": true`, `aim_siem_events_failed_total{adapter="sentinel"}` counts the batch's events and `aim_siem_events_sent_total{adapter="sentinel"}` does not move, and the backend logs `ERROR SIEM delivery failed adapter=sentinel events=<count> error="sentinel token request returned <status>: invalid_client: <description>"`.

To check delivery, query the destination table in Log Analytics. For a table named `AIMSecurityEvents_CL`:

```kusto
AIMSecurityEvents_CL
| where TimeGenerated > ago(1h)
| project TimeGenerated, EventType, Severity, AgentId, Details
```

### CyberArk Central Credential Provider

The backend reads agent credentials from a CyberArk vault through the Central Credential Provider (CCP) web service once `CYBERARK_CCP_URL`, `CYBERARK_CCP_APP_ID` and `CYBERARK_CCP_SAFE` are set. A secrets namespace created with `"backendType": "cyberark_ccp"` then reads its credential from CyberArk; the backend stores and rotates nothing there.

| Variable | Value |
|----------|-------|
| `CYBERARK_CCP_URL` | The `https` base URL of the CCP web service, such as `https://ccp.example.com`, with no user information or query. The backend sends `GET <url>/AIMWebService/api/Accounts`. |
| `CYBERARK_CCP_APP_ID` | The application ID the backend is registered under in CyberArk, sent as `AppID`. |
| `CYBERARK_CCP_SAFE` | The safe that holds the namespaces' accounts, sent as `Safe`. |
| `CYBERARK_CCP_CERT_PATH` | Optional. A PEM client certificate the backend presents to CCP. Set it together with `CYBERARK_CCP_KEY_PATH`, or neither. |
| `CYBERARK_CCP_KEY_PATH` | Optional. The PEM private key of that certificate. |
| `CYBERARK_CCP_CA_CERT_PATH` | Optional. A PEM bundle that verifies the CCP server's certificate. Unset, the system roots verify it. |

With the backend registered, startup logs `CyberArk CCP backend registered at <url> for safe "<safe>"`. When the application ID or the safe is not set, the URL is not an `https` URL with a host or carries user information or a query, the application ID or the safe holds `+`, `&`, `%` or `;`, only one of the certificate pair is set, or a certificate, key or CA file cannot be read or holds no PEM the backend can use, startup logs `WARNING: CyberArk CCP backend configured but failed to initialize: <reason>` and the server runs without the backend.

To give an agent a credential from CyberArk:

1. Create a namespace for the agent with `POST /api/v1/secrets/namespaces`, sending `"backendType": "cyberark_ccp"`. The answer carries the namespace's `id`.
2. In the safe, vault the credential as an account named `aim-secrets-<id>`, and let the application ID retrieve it.
3. The agent that owns the namespace resolves it with `POST /api/v1/secrets/resolve`. The backend sends CCP one request naming `AppID`, `Safe` and, as `Object`, the account name, and the answer carries the account's content encrypted to the agent's key.

`POST /api/v1/secrets/namespaces/:id/credentials` and `POST /api/v1/secrets/namespaces/:id/rotate` on such a namespace are answered `409 {"error":"the namespace's backend is read-only: store and rotate its credential in the backend"}`; nothing is sent to CCP and no audit entry is written. Deleting the namespace sends nothing to CCP, so the account stays in the safe. A resolve request is answered `403 {"error":"Secret resolution refused"}` when the server runs without the backend (the secrets audit entry's deny reason is `backend not available: cyberark_ccp`), and when CCP finds no account for the namespace or answers with an error (`credential retrieval failed`).

### Database Migrations and the Application Role

`aim-server migrate` applies the pending migrations and exits without starting the server. It reads only the `POSTGRES_*` variables and is meant to run as the role that owns the schema. A missing `POSTGRES_HOST`, `POSTGRES_USER` or `POSTGRES_DB` stops it with one line that names each missing variable.

With `POSTGRES_APP_USER` and `POSTGRES_APP_PASSWORD` set, it first creates or updates that role for the server to connect as: a login role that is not a superuser, has no BYPASSRLS and cannot create objects in schema `public`, with read and write access to the tables and sequences the migrations create. Two effects reach beyond that role:

- **CREATE on schema `public` is revoked from `PUBLIC` as well as from the application role.** PostgreSQL before 15 grants CREATE on schema `public` to `PUBLIC`, and every role holds what `PUBLIC` holds, so a revoke from the application role alone would leave it able to create tables. On such a server every role that has no grant of its own loses CREATE on `public`; grant it back to a role that needs it (`GRANT CREATE ON SCHEMA public TO <role>`). PostgreSQL 15 and later does not grant it to `PUBLIC`, and the revoke changes nothing there. The command logs the revoke after provisioning.
- **The password travels in the statement.** The role's password is set with `CREATE ROLE ... PASSWORD '<literal>'` or `ALTER ROLE ... PASSWORD '<literal>'`. With `log_statement` set to `ddl` or `all`, PostgreSQL writes that statement, password included, to its server log. Run the command while statement logging is below `ddl`, or set the password again afterwards from a client that sends a SCRAM verifier instead of the password, such as `\password <role>` in `psql`.

**Upgrading across the agent private key format change.** A release that stores agent private keys bound to their agent row converts the stored v1 keys once at startup, to the format it writes, and records completion in `schema_migrations` as `keyvault/agent-private-key-v2`; later starts skip the conversion. An instance of the earlier release that is still running keeps writing v1 keys, and a key written after completion is never converted and is refused on the request path until that agent's credentials are rotated. Stop every instance of the earlier release before the first instance of the new release starts.

**Upgrading across the agent private key data key change.** A build that encrypts each agent private key under a data key of its own writes format v3, and still reads the keys stored in format v2: the startup sweep moves a v2 key under an older storage key id to format v3, and a v2 key under the current id stays as it is. A build from before that change refuses format v3, so a key it reads in v3 fails on the request path until that agent's credentials are rotated. Stop every instance of the earlier build before the first instance of the new one starts, and take a database backup first if you may roll back.

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

# Get one certificate for the dashboard's host name and the API's
sudo certbot certonly --standalone -d yourdomain.com -d api.yourdomain.com
```

The repository ships no reverse-proxy configuration. Run a TLS-terminating reverse proxy (nginx, Caddy, Traefik or a cloud load balancer) in front of the dashboard on port 3000 and the API on port 8080, using the certificate from the previous command.

`docker-compose.yml` publishes those two ports on every interface of the host, while every other port it publishes is bound to `127.0.0.1`, so the dashboard and the API also answer over plain HTTP on each address of the host. When the proxy runs on the same host, write `127.0.0.1:` before the host port in the `ports` entries of the `backend` and `frontend` services, as `docker-compose.quickstart.yml` does.

Then set `FRONTEND_API_URL` to the API's public origin, and `FRONTEND_URL` and `ALLOWED_ORIGINS` to the dashboard's, in `.env`, and run `docker compose up -d` again:

```bash
# The API URL the dashboard hands the browser
FRONTEND_API_URL=https://api.yourdomain.com
# The dashboard's origin: the base of the links the API sends, such as password-reset links
FRONTEND_URL=https://yourdomain.com
# The origins the API accepts browser requests from (CORS), comma-separated
ALLOWED_ORIGINS=https://yourdomain.com
```

`docker-compose.yml` sets `NEXT_PUBLIC_API_URL` on the dashboard's container from `FRONTEND_API_URL`. The dashboard reads that variable each time it renders a page, hands the URL to the browser and lists its origin in the page's `connect-src`. Without `FRONTEND_API_URL`, the dashboard is handed `http://localhost:8080` (the port follows `AIM_BACKEND_PORT`), which in a browser on another machine names that machine.

The dashboard has a fallback for a container with no `NEXT_PUBLIC_API_URL` in its environment, which does not apply to a stack started from `docker-compose.yml` or `docker-compose.quickstart.yml`: both hand the dashboard a URL whether or not `FRONTEND_API_URL` is set. Opened at a host name such as `aim-frontend.example.com`, a dashboard started without the variable calls the same name with `-backend` in place of the first `-frontend` (`https://aim-backend.example.com`), on the page's scheme and with no port. [Dashboard API origin and Content-Security-Policy](guides/DEPLOYMENT_CHECKLIST.md#dashboard-api-origin-and-content-security-policy) gives the order in which the dashboard picks the API's URL.

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

The quickstart passes no `REGISTRY_*` variable to `aim-server`, so the server runs no Registry bridge, the job that sends attestation data to the OpenA2A Registry. To turn it on, add `REGISTRY_BRIDGE_ENABLED=true` and a `REGISTRY_BRIDGE_SECRET` to the `backend` service's `environment` in the install's `docker-compose.quickstart.yml` and run `docker compose -f docker-compose.quickstart.yml up -d` in that directory; the bridge then runs every 6 hours. Set in the shell or in the `.env` beside that file, the two variables do not reach the server: the `backend` service is handed only the variables its `environment` lists.

The host ports and the container names come from the environment, with the values below as defaults. Set a variable before `bash quickstart.sh` (or in the `.env` next to `docker-compose.quickstart.yml`) to run AIM beside another service on the same port, or to run a second AIM stack on one host. A first run records the values it ran with in the install's `.env`, so a later `docker compose` in that directory keeps them. A run over an existing install starts the stack with the variables set for that run and leaves that `.env` as it is, so a later `docker compose` in that directory reads the values `.env` holds; to keep a new value, change it in that `.env`.

| Variable | Default | Sets |
|---|---|---|
| `AIM_BACKEND_PORT` | `8080` | host port of the API (`aim-backend`), bound to `127.0.0.1` |
| `AIM_FRONTEND_PORT` | `3000` | host port of the dashboard (`aim-frontend`), bound to `127.0.0.1` |
| `AIM_POSTGRES_PORT` | `5432` | host port of PostgreSQL (`aim-postgres`), bound to `127.0.0.1` |
| `AIM_REDIS_PORT` | `6379` | host port of Redis (`aim-redis`), bound to `127.0.0.1` |
| `AIM_PROJECT` | `aim` | prefix of the four container names |

A second stack beside the default one, in its own directory and Compose project, with its own container names and ports:

```bash
AIM_DIR=aim2 AIM_PROJECT=aim2 AIM_BACKEND_PORT=8081 AIM_FRONTEND_PORT=3001 AIM_POSTGRES_PORT=5433 AIM_REDIS_PORT=6380 bash quickstart.sh
```

Compose names the volumes and the network after its project (`COMPOSE_PROJECT_NAME`, or the install directory's name), so two stacks also need two projects; `AIM_DIR` gives each its own directory and project.

The dashboard hands the browser the API URL in `FRONTEND_API_URL`, or `http://localhost:<AIM_BACKEND_PORT>` when it is not set, so with an image that reads that URL when it renders a page (below), the second stack's dashboard calls `http://localhost:8081`. To open the dashboard from another machine or through a reverse proxy, set `FRONTEND_API_URL` to the URL the browser reaches the API at, and `FRONTEND_URL` and `ALLOWED_ORIGINS` to the dashboard's origin, as [Enable HTTPS](#4-enable-https-lets-encrypt) shows. The fallback on a `-frontend` host name described there does not apply to a stack started from this compose file. The quickstart publishes the API and the dashboard on `127.0.0.1` only, as it does PostgreSQL and Redis, so a reverse proxy on the same host reaches them there. To publish the two on every interface of the host instead, change `127.0.0.1` to `0.0.0.0` in the `ports` entries of the `backend` and `frontend` services in the install's `docker-compose.quickstart.yml` and run `docker compose -f docker-compose.quickstart.yml up -d` in that directory.

An `aim-dashboard` image built from code that reads `NEXT_PUBLIC_API_URL` only when it is built ignores `FRONTEND_API_URL`: its browser code holds the URL it was built with. On 2026-10-17, `latest` pointed to such an image, revision `3ab8285e`, published 2026-10-02 and built for `http://localhost:8080`; started with `NEXT_PUBLIC_API_URL=http://localhost:8081`, it served a sign-in page that named no `localhost:8081`. To check the image a stack runs, read the policy its sign-in page carries with the command below: on an image that reads the URL when it renders a page, it prints a `connect-src` that lists the API's origin, and on one that does not, it prints nothing.

```bash
curl -s http://localhost:3001/auth/login | grep -o '<meta http-equiv="Content-Security-Policy"[^>]*>'
```

Production deployment (Azure, GCP, AWS): [infrastructure/DEPLOYMENT.md](../infrastructure/DEPLOYMENT.md).

Merges to `main` no longer publish images: only a push of a `platform-vX.Y.Z` tag does, and it publishes `X.Y.Z`, `X.Y`, `X` and `latest` for both images on both registries. The `edge` tag is frozen at the last build of `main` published before this change and will be removed; to follow `main`, build from source (next section). This change does not move `1`, `1.0` or `1.0.0`. A running image is identified by its digest and its revision label (`org.opencontainers.image.revision`), not by the tag it was pulled by.

New accounts wait for an administrator's approval. To bootstrap the first administrator, set `AIM_PLATFORM_ADMINS` (comma-separated emails) before starting the backend: accounts on that list are approved automatically and approve everyone else from the dashboard's admin area. Until a listed address has registered, or some administrator exists, other sign-ups are refused with an error that says so; nothing is queued. The backend logs how the variable was read at startup, so a mistyped entry is visible there.

The `aim-server` and `aim-dashboard` image tags `1.23`, `1.23.0`, `1.5` and `1.5.0`, and the `aim-server` tags `1.22` and `1.22.0`, are withdrawn: their numbers sort above `1.0.0`, but no platform release has used them. Not every withdrawn tag is gone from the registries. On 2026-10-21, Docker Hub (`opena2a/aim-server` and `opena2a/aim-dashboard`) answered `MANIFEST_UNKNOWN` for `1.22`, `1.22.0`, `1.23` and `1.23.0` and still served `1.5` and `1.5.0` of both images, and the GitHub Container Registry (`ghcr.io/opena2a-org/aim-server` and `ghcr.io/opena2a-org/aim-dashboard`) still served every tag named above. Do not run an image pulled by one of these tags. The images behind them bundle components with critical-rated vulnerabilities, described in [GHSA-rqgr-f9m6-xphr](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-rqgr-f9m6-xphr) for the dashboard and [GHSA-fxfm-hhhw-3mgf](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-fxfm-hhhw-3mgf) for the server. The dashboard images are also affected by [GHSA-638x-gwq7-xfh7](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-638x-gwq7-xfh7), rated high. Each of these advisories lists the images it affects and shows how to check a deployment.

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

# Create the first administrator once the backend is healthy (a later run changes nothing)
docker compose run --rm bootstrap

# Python SDK editable for examples
pip install -e sdk/python

# Try the flight-search-agent demo
cd examples/flight-search-agent && python3 flight_agent.py
```

Sign in at the dashboard, `http://localhost:3000`, as `admin@opena2a.org` with the `DEFAULT_ADMIN_PASSWORD` value in `.env`: `scripts/gen-dev-secrets.sh` writes that variable, and the `bootstrap` service passes it to this one run and prints no password it was given. If `.env` sets no `DEFAULT_ADMIN_PASSWORD`, bootstrap generates a password and prints it once. Bootstrap marks the account for a password change, which the dashboard asks for at the first sign-in. The `bootstrap` service is behind the `bootstrap` profile, so `docker compose up` does not start it.

The full `docker-compose.yml` also brings up Elasticsearch, MinIO, NATS, Prometheus, Grafana, and Loki. Skip those services with the minimal command above.

`docker-compose.yml` takes `AIM_PROJECT` (`aim`, the prefix of every container name) and the quick-start's `AIM_BACKEND_PORT` (`8080`), `AIM_FRONTEND_PORT` (`3000`), `AIM_POSTGRES_PORT` (`5432`) and `AIM_REDIS_PORT` (`6379`) with the same defaults, plus one host port per other service: `AIM_ELASTICSEARCH_PORT` (`9200`), `AIM_ELASTICSEARCH_TRANSPORT_PORT` (`9300`), `AIM_MINIO_PORT` (`9000`), `AIM_MINIO_CONSOLE_PORT` (`9001`), `AIM_NATS_PORT` (`4222`), `AIM_NATS_MONITOR_PORT` (`8222`), `AIM_PROMETHEUS_PORT` (`9090`), `AIM_GRAFANA_PORT` (`3003`), `AIM_LOKI_PORT` (`3100`). Set them in `.env` to run the dev stack beside a quick-start or another checkout on one host; each stack also needs its own Compose project (`COMPOSE_PROJECT_NAME`, or the checkout directory's name).

## Observability

The backend emits OpenTelemetry traces, metrics, and logs. A hermetic demo stack lives at [`apps/backend/deployments/otel-demo/`](../apps/backend/deployments/otel-demo/): OpenTelemetry Collector, Tempo, Prometheus, Loki, Grafana.

```bash
cd apps/backend/deployments/otel-demo
docker compose up -d
./smoke-backend.sh
```

### Collector endpoint and TLS

The backend exports over OTLP/gRPC. It reads the collector's endpoint from `OTEL_EXPORTER_OTLP_ENDPOINT`, or for one signal from `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` or `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`, which wins where it is set. It decides TLS as the [OTLP exporter specification](https://opentelemetry.io/docs/specs/otel/protocol/exporter/) defines:

| Endpoint value | Connection |
|---|---|
| unset | `localhost:4317`, plaintext |
| the empty string | telemetry off |
| `https://collector.example.com:4317` | TLS, whatever `OTEL_EXPORTER_OTLP_INSECURE` says |
| `http://collector.example.com:4317` | plaintext, whatever `OTEL_EXPORTER_OTLP_INSECURE` says |
| `collector.example.com:4317` with `OTEL_EXPORTER_OTLP_INSECURE=true` | plaintext |
| `collector.example.com:4317` with `OTEL_EXPORTER_OTLP_INSECURE=false` | TLS |
| `collector.example.com:4317` with `OTEL_EXPORTER_OTLP_INSECURE` unset | TLS, unless the host is `localhost`, an address in 127.0.0.0/8 or `::1`, which is plaintext |

For TLS, three variables name PEM files:

- `OTEL_EXPORTER_OTLP_CERTIFICATE`: the CA certificate that verifies the collector. Unset, the system roots verify it.
- `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and `OTEL_EXPORTER_OTLP_CLIENT_KEY`: the client certificate and its key, for a collector that requires mutual TLS. Set both or neither.

Each also has a per-signal form, for example `OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE`, which wins where it is set. A file that cannot be read, a scheme other than `http` or `https`, or an `OTEL_EXPORTER_OTLP_INSECURE` value other than `true` or `false` stops telemetry: the start log says `OpenTelemetry init failed: telemetry: <variable>...` with the variable's name, and the server starts without telemetry. With telemetry on, the start log says `OpenTelemetry initialized (OTLP gRPC)`.

Give an IP address in the URL form, for example `http://10.0.0.5:4317`. With `host:port` and an IP address host, for example `10.0.0.5:4317`, the OpenTelemetry exporter library the backend uses reads the variable itself and logs `parse url ... first path segment in URL cannot contain colon` twice at start, once for traces and once for metrics. The backend still reads the value as the table says and exports. A host name in `host:port`, such as `localhost:4317`, logs no such line.

### Spans

Every HTTP request the backend serves, other than `/metrics`, emits a SERVER span named `{method} {route}` from the route the request matched, for example `POST /api/v1/public/login`. It carries the [HTTP semantic-convention](https://opentelemetry.io/docs/specs/semconv/http/http-spans/) attributes `http.request.method`, `url.path`, `url.scheme`, `http.route`, `http.response.status_code`, `server.address`, `server.port`, `network.protocol.version`, `client.address` and `user_agent.original`. A 5xx answer sets the span's status to Error and `error.type` to the status code; a 4xx leaves the status unset. A request no route matched, or one a middleware answered before its route was reached (an authentication refusal, for example), has no `http.route` and is named by its method alone. The span records no query string, no header other than `User-Agent` and no body. A span a handler starts is a child of its request's span. The backend does not continue an incoming `traceparent`: each request's span starts a trace of its own.

Every `fga.authorize` decision lands as a parent span, a child of its request's span, with one child span per FGA step it ran: up to five on `POST /api/v1/agents/:id/authorize` and on `POST /api/v1/sdk-api/verifications` (up to four when the policy's risk level is LOW), under the agent's FGA policy for the capability or the default policy when it has none; `fga.policy` on the parent says which (`stored` or `default`). 9 SemConv attributes ride the parent: `agent.id`, `agent.public_key.algorithm`, `agent.trust_score`, `agent.drift_score`, `agent.scan_verdict`, `agent.capability`, `fga.step`, `fga.outcome`, `fga.denied_by`. The attribute set is proposed to the OpenTelemetry Semantic Conventions WG. Full design notes: [`apps/backend/docs/OBSERVABILITY.md`](../apps/backend/docs/OBSERVABILITY.md).

### Prometheus metric names

The gauge of registered MCP servers is `aim_mcp_servers`. It was `aim_mcp_servers_total`, a counter's suffix on a gauge; a query or alert on the old name matches nothing and needs the new one.

`aim_http_requests_total` and `aim_http_request_duration_seconds` carry the labels `method`, `path` and `status`. `path` is the path of the route that served the request as the server registers it, group prefix included, for example `/api/v1/agents/:id/trust-score` or `/api/v1/did/*`, so the values a caller puts in a path parameter or a wildcard are not label values. A request no route handler served carries `path="unmatched"`: an unknown path, a method the path does not accept, or a request a group middleware refused, such as one without credentials under `/api/v1/agents`. `status` is the status the client received. Earlier builds labelled a request a route handler served with the path the caller sent, with each segment of digits and each 36-character segment holding four hyphens replaced by `:id`; a query or alert on such a value needs the route's path.

---

## 📞 Support

- **Documentation**: https://opena2a.org/docs
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discord**: https://discord.gg/opena2a
- **Email**: info@opena2a.org

---

**🛡️ Happy Deploying with AIM!**
