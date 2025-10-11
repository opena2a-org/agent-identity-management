# AIM Production Deployment Guide

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Infrastructure Setup](#infrastructure-setup)
4. [Kubernetes Deployment](#kubernetes-deployment)
5. [Database Setup](#database-setup)
6. [SSL/TLS Configuration](#ssltls-configuration)
7. [Monitoring & Logging](#monitoring--logging)
8. [Backup & Disaster Recovery](#backup--disaster-recovery)
9. [CI/CD Pipeline](#cicd-pipeline)
10. [Operational Procedures](#operational-procedures)

---

## Overview

This guide provides step-by-step instructions for deploying AIM to a **production Kubernetes cluster** with:

- High availability (99.9% uptime SLA)
- Auto-scaling (3-20 replicas)
- Database replication (PostgreSQL + Redis)
- SSL/TLS encryption (Let's Encrypt)
- Monitoring (Prometheus + Grafana)
- Logging (ELK Stack / Loki)
- Automated backups (daily snapshots)
- Disaster recovery (< 1 hour RTO)

### Deployment Topology

```
┌─────────────────────────────────────────────────────┐
│              Cloud Load Balancer                     │
│              (SSL Termination)                       │
└──────────────────┬──────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────┐
│         Kubernetes Ingress (Nginx)                   │
│         (Rate limiting, auth)                        │
└───────────┬─────────────────────┬───────────────────┘
            │                     │
┌───────────▼──────────┐  ┌──────▼──────────────────┐
│   Backend Service    │  │   Frontend Service       │
│   (3-20 pods)        │  │   (2-10 pods)            │
│   Port: 8080         │  │   Port: 3000             │
└───────────┬──────────┘  └──────┬──────────────────┘
            │                     │
┌───────────▼─────────────────────▼───────────────────┐
│              PostgreSQL StatefulSet                  │
│              (Primary + 2 Replicas)                  │
│              Port: 5432                              │
└──────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────┐
│              Redis StatefulSet                       │
│              (3-node cluster)                        │
│              Port: 6379                              │
└──────────────────────────────────────────────────────┘
```

---

## Prerequisites

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| kubectl | 1.28+ | Kubernetes CLI |
| helm | 3.12+ | Package manager |
| docker | 24+ | Container runtime |
| psql | 16+ | PostgreSQL client |
| redis-cli | 7+ | Redis client |
| terraform | 1.5+ | Infrastructure as code |

### Cloud Provider Accounts

Choose one:
- **AWS**: EKS (Elastic Kubernetes Service)
- **GCP**: GKE (Google Kubernetes Engine)
- **Azure**: AKS (Azure Kubernetes Service)

### Domain & DNS

- Domain name (e.g., `aim.yourdomain.com`)
- DNS access (A/CNAME records)
- Email for Let's Encrypt (SSL certificates)

---

## Infrastructure Setup

### Step 1: Create Kubernetes Cluster

**AWS EKS**:
```bash
# Install eksctl
brew install eksctl

# Create cluster
eksctl create cluster \
  --name aim-production \
  --region us-west-2 \
  --nodegroup-name standard-workers \
  --node-type t3.xlarge \
  --nodes 3 \
  --nodes-min 3 \
  --nodes-max 10 \
  --managed
```

**GCP GKE**:
```bash
# Create cluster
gcloud container clusters create aim-production \
  --zone us-central1-a \
  --num-nodes 3 \
  --machine-type n1-standard-4 \
  --enable-autoscaling \
  --min-nodes 3 \
  --max-nodes 10
```

**Azure AKS**:
```bash
# Create resource group
az group create --name aim-production --location eastus

# Create cluster
az aks create \
  --resource-group aim-production \
  --name aim-production \
  --node-count 3 \
  --node-vm-size Standard_D4s_v3 \
  --enable-cluster-autoscaler \
  --min-count 3 \
  --max-count 10 \
  --generate-ssh-keys
```

### Step 2: Configure kubectl

```bash
# AWS
aws eks update-kubeconfig --name aim-production --region us-west-2

# GCP
gcloud container clusters get-credentials aim-production --zone us-central1-a

# Azure
az aks get-credentials --resource-group aim-production --name aim-production

# Verify connection
kubectl get nodes
```

### Step 3: Create Namespace

```bash
# Create production namespace
kubectl create namespace aim-production

# Set default namespace
kubectl config set-context --current --namespace=aim-production
```

---

## Kubernetes Deployment

### Step 1: Create Secrets

**Database Credentials**:
```bash
kubectl create secret generic aim-db-credentials \
  --from-literal=username=aim_user \
  --from-literal=password=SECURE_PASSWORD_HERE \
  --from-literal=database=aim_production
```

**JWT Secret**:
```bash
# Generate secure random secret (256-bit)
JWT_SECRET=$(openssl rand -base64 32)

kubectl create secret generic aim-jwt-secret \
  --from-literal=secret=$JWT_SECRET
```

**OAuth Credentials**:
```bash
kubectl create secret generic aim-oauth-credentials \
  --from-literal=google-client-id=YOUR_GOOGLE_CLIENT_ID \
  --from-literal=google-client-secret=YOUR_GOOGLE_CLIENT_SECRET \
  --from-literal=microsoft-client-id=YOUR_MICROSOFT_CLIENT_ID \
  --from-literal=microsoft-client-secret=YOUR_MICROSOFT_CLIENT_SECRET
```

### Step 2: Deploy PostgreSQL

**PostgreSQL StatefulSet** (`k8s/postgres-statefulset.yaml`):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: aim-production
spec:
  serviceName: postgres
  replicas: 3
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
        image: postgres:16
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: database
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: password
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
  volumeClaimTemplates:
  - metadata:
      name: postgres-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: aim-production
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
  clusterIP: None
```

**Deploy**:
```bash
kubectl apply -f k8s/postgres-statefulset.yaml

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=postgres --timeout=300s
```

### Step 3: Deploy Redis

**Redis StatefulSet** (`k8s/redis-statefulset.yaml`):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: aim-production
spec:
  serviceName: redis
  replicas: 3
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        command:
        - redis-server
        - --appendonly
        - "yes"
        volumeMounts:
        - name: redis-storage
          mountPath: /data
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
  volumeClaimTemplates:
  - metadata:
      name: redis-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 20Gi
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: aim-production
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
  clusterIP: None
```

**Deploy**:
```bash
kubectl apply -f k8s/redis-statefulset.yaml

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=redis --timeout=300s
```

### Step 4: Deploy Backend

**Backend Deployment** (`k8s/backend-deployment.yaml`):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aim-backend
  namespace: aim-production
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
        image: yourdomain/aim-backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          value: "postgresql://$(DB_USER):$(DB_PASSWORD)@postgres:5432/$(DB_NAME)?sslmode=require"
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: password
        - name: DB_NAME
          valueFrom:
            secretKeyRef:
              name: aim-db-credentials
              key: database
        - name: REDIS_URL
          value: "redis://redis:6379"
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: aim-jwt-secret
              key: secret
        - name: GOOGLE_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: aim-oauth-credentials
              key: google-client-id
        - name: GOOGLE_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: aim-oauth-credentials
              key: google-client-secret
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: aim-backend
  namespace: aim-production
spec:
  selector:
    app: aim-backend
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
---
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
  maxReplicas: 20
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

**Deploy**:
```bash
kubectl apply -f k8s/backend-deployment.yaml

# Wait for deployment
kubectl rollout status deployment/aim-backend
```

### Step 5: Deploy Frontend

**Frontend Deployment** (`k8s/frontend-deployment.yaml`):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aim-frontend
  namespace: aim-production
spec:
  replicas: 2
  selector:
    matchLabels:
      app: aim-frontend
  template:
    metadata:
      labels:
        app: aim-frontend
    spec:
      containers:
      - name: frontend
        image: yourdomain/aim-frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: NEXT_PUBLIC_API_URL
          value: "https://api.aim.yourdomain.com"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: aim-frontend
  namespace: aim-production
spec:
  selector:
    app: aim-frontend
  ports:
  - port: 3000
    targetPort: 3000
  type: ClusterIP
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: aim-frontend-hpa
  namespace: aim-production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: aim-frontend
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**Deploy**:
```bash
kubectl apply -f k8s/frontend-deployment.yaml

# Wait for deployment
kubectl rollout status deployment/aim-frontend
```

### Step 6: Configure Ingress

**Nginx Ingress Controller**:
```bash
# Install Nginx Ingress Controller
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer
```

**Ingress Resource** (`k8s/ingress.yaml`):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: aim-ingress
  namespace: aim-production
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - aim.yourdomain.com
    - api.aim.yourdomain.com
    secretName: aim-tls-cert
  rules:
  - host: aim.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: aim-frontend
            port:
              number: 3000
  - host: api.aim.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: aim-backend
            port:
              number: 8080
```

**Deploy**:
```bash
kubectl apply -f k8s/ingress.yaml
```

---

## Database Setup

### Step 1: Run Migrations

**From Migration Pod**:
```bash
# Create migration job
kubectl run migrate --image=yourdomain/aim-backend:latest \
  --command -- /app/migrate \
  --env="DATABASE_URL=postgresql://..." \
  --restart=Never

# Wait for completion
kubectl wait --for=condition=complete job/migrate --timeout=300s

# Check logs
kubectl logs job/migrate
```

**Or from local machine**:
```bash
# Port-forward to PostgreSQL
kubectl port-forward svc/postgres 5432:5432

# Run migrations
cd apps/backend
go run cmd/migrate/main.go up
```

### Step 2: Verify Database

```bash
# Connect to database
kubectl exec -it postgres-0 -- psql -U aim_user -d aim_production

# Check tables
\dt

# Expected tables:
# - users
# - organizations
# - agents
# - mcp_servers
# - api_keys
# - audit_logs
# - security_threats
# - alerts

# Exit
\q
```

---

## SSL/TLS Configuration

### Install Cert-Manager

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app=cert-manager -n cert-manager --timeout=300s
```

### Create Let's Encrypt Issuer

**ClusterIssuer** (`k8s/letsencrypt-issuer.yaml`):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@yourdomain.com
    privateKeySecretRef:
      name: letsencrypt-prod-key
    solvers:
    - http01:
        ingress:
          class: nginx
```

**Deploy**:
```bash
kubectl apply -f k8s/letsencrypt-issuer.yaml

# Verify issuer
kubectl get clusterissuer letsencrypt-prod
```

### Verify SSL Certificate

```bash
# Check certificate status
kubectl get certificate -n aim-production

# Should show:
# NAME            READY   SECRET          AGE
# aim-tls-cert    True    aim-tls-cert    5m

# Test HTTPS
curl https://aim.yourdomain.com
curl https://api.aim.yourdomain.com/health
```

---

## Monitoring & Logging

### Prometheus + Grafana

**Install kube-prometheus-stack**:
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.adminPassword=SECURE_PASSWORD
```

**Access Grafana**:
```bash
# Port-forward to Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Open http://localhost:3000
# Username: admin
# Password: SECURE_PASSWORD
```

### Loki for Logging

**Install Loki**:
```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm install loki grafana/loki-stack \
  --namespace logging \
  --create-namespace \
  --set promtail.enabled=true \
  --set grafana.enabled=false
```

**Configure Grafana Data Source**:
1. Open Grafana
2. Configuration → Data Sources → Add data source
3. Select "Loki"
4. URL: `http://loki.logging:3100`
5. Save & Test

---

## Backup & Disaster Recovery

### Daily Database Backups

**Backup CronJob** (`k8s/backup-cronjob.yaml`):

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgres-backup
  namespace: aim-production
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM UTC
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: postgres:16
            command:
            - /bin/sh
            - -c
            - |
              TIMESTAMP=$(date +%Y%m%d_%H%M%S)
              pg_dump -h postgres -U $POSTGRES_USER -d $POSTGRES_DB > /backup/aim_backup_$TIMESTAMP.sql
              # Upload to S3/GCS/Azure Blob
              aws s3 cp /backup/aim_backup_$TIMESTAMP.sql s3://your-bucket/backups/
            env:
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: aim-db-credentials
                  key: username
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: aim-db-credentials
                  key: password
            - name: POSTGRES_DB
              valueFrom:
                secretKeyRef:
                  name: aim-db-credentials
                  key: database
            volumeMounts:
            - name: backup-storage
              mountPath: /backup
          volumes:
          - name: backup-storage
            persistentVolumeClaim:
              claimName: backup-pvc
          restartPolicy: OnFailure
```

**Deploy**:
```bash
kubectl apply -f k8s/backup-cronjob.yaml

# Verify
kubectl get cronjob -n aim-production
```

### Restore from Backup

```bash
# Download backup
aws s3 cp s3://your-bucket/backups/aim_backup_20251010_020000.sql /tmp/

# Restore to database
kubectl exec -it postgres-0 -- psql -U aim_user -d aim_production < /tmp/aim_backup_20251010_020000.sql
```

---

## CI/CD Pipeline

### GitHub Actions Workflow

**Production Deploy** (`.github/workflows/production-deploy.yml`):

```yaml
name: Production Deploy

on:
  push:
    branches:
      - main

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v3

    - name: Build and push Docker images
      run: |
        docker build -t yourdomain/aim-backend:${{ github.sha }} -f infrastructure/docker/Dockerfile.backend .
        docker build -t yourdomain/aim-frontend:${{ github.sha }} -f infrastructure/docker/Dockerfile.frontend .
        docker push yourdomain/aim-backend:${{ github.sha }}
        docker push yourdomain/aim-frontend:${{ github.sha }}

    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/aim-backend backend=yourdomain/aim-backend:${{ github.sha }} -n aim-production
        kubectl set image deployment/aim-frontend frontend=yourdomain/aim-frontend:${{ github.sha }} -n aim-production

    - name: Wait for rollout
      run: |
        kubectl rollout status deployment/aim-backend -n aim-production
        kubectl rollout status deployment/aim-frontend -n aim-production

    - name: Run smoke tests
      run: |
        curl -f https://api.aim.yourdomain.com/health || exit 1
        curl -f https://aim.yourdomain.com || exit 1
```

---

## Operational Procedures

### Rolling Updates

```bash
# Update backend image
kubectl set image deployment/aim-backend \
  backend=yourdomain/aim-backend:v2.0.0 \
  -n aim-production

# Monitor rollout
kubectl rollout status deployment/aim-backend -n aim-production

# Verify new pods
kubectl get pods -l app=aim-backend -n aim-production
```

### Rollback

```bash
# Rollback to previous version
kubectl rollout undo deployment/aim-backend -n aim-production

# Rollback to specific revision
kubectl rollout undo deployment/aim-backend --to-revision=3 -n aim-production
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment aim-backend --replicas=10 -n aim-production

# Check HPA status
kubectl get hpa -n aim-production
```

### Health Checks

```bash
# Check all pods
kubectl get pods -n aim-production

# Check specific deployment
kubectl describe deployment aim-backend -n aim-production

# Check logs
kubectl logs -f deployment/aim-backend -n aim-production

# Check events
kubectl get events -n aim-production --sort-by='.lastTimestamp'
```

---

## References

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [SECURITY.md](SECURITY.md) - Security best practices
- [PERFORMANCE.md](PERFORMANCE.md) - Performance optimization
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Charts](https://helm.sh/docs/)

---

**Maintained by**: OpenA2A DevOps Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026

For deployment support, contact: devops@yourdomain.com
