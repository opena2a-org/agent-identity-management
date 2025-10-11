# AIM Quick Start Guide

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Estimated Time**: 15 minutes

---

## 🚀 Get Started in 3 Steps

This guide will have you running AIM locally in **under 15 minutes**.

### What You'll Build
- ✅ Full AIM system running locally
- ✅ Dashboard accessible at `http://localhost:3000`
- ✅ API accessible at `http://localhost:8080`
- ✅ First AI agent registered and verified
- ✅ MCP server connected

---

## Prerequisites

### Required Software

| Tool | Version | Installation |
|------|---------|--------------|
| **Docker** | 24+ | [Download](https://docs.docker.com/get-docker/) |
| **Docker Compose** | 2.20+ | Included with Docker Desktop |
| **Git** | 2.40+ | [Download](https://git-scm.com/downloads) |

**Optional** (for SDK testing):
- **Python** 3.10+ ([Download](https://www.python.org/downloads/))
- **Go** 1.23+ ([Download](https://go.dev/dl/))

### System Requirements

- **OS**: macOS, Linux, or Windows (with WSL2)
- **RAM**: 8GB minimum (16GB recommended)
- **Disk**: 10GB free space

---

## Step 1: Clone and Start (5 minutes)

### 1.1 Clone Repository

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
```

### 1.2 Start Services

```bash
# Start all services with Docker Compose
docker compose up -d

# Wait for services to be healthy (30-60 seconds)
docker compose ps
```

**Expected Output**:
```
NAME                STATUS              PORTS
aim-backend         Up (healthy)        0.0.0.0:8080->8080/tcp
aim-frontend        Up (healthy)        0.0.0.0:3000->3000/tcp
aim-postgres        Up (healthy)        5432/tcp
aim-redis           Up (healthy)        6379/tcp
```

### 1.3 Verify Services

```bash
# Check backend health
curl http://localhost:8080/health
# Expected: {"status":"healthy"}

# Check frontend
curl http://localhost:3000
# Expected: HTML response
```

**Troubleshooting**:
```bash
# If services aren't starting, check logs
docker compose logs backend
docker compose logs frontend

# Restart if needed
docker compose restart

# Full reset
docker compose down -v
docker compose up -d
```

---

## Step 2: Create Your Account (3 minutes)

### 2.1 Access Dashboard

Open your browser to **http://localhost:3000**

You'll see the AIM login page:

```
┌─────────────────────────────────┐
│          🛡️  AIM                │
│   Agent Identity Management     │
│                                 │
│  ┌──────────────────────────┐  │
│  │ Email                     │  │
│  └──────────────────────────┘  │
│  ┌──────────────────────────┐  │
│  │ Password                  │  │
│  └──────────────────────────┘  │
│                                 │
│  [ Sign In ]  [ Sign Up ]       │
└─────────────────────────────────┘
```

### 2.2 Register Account

1. Click **"Sign Up"**
2. Fill in registration form:
   - **Email**: your-email@example.com
   - **Password**: SecurePassword123! (min 12 characters)
   - **Display Name**: Your Name
3. Click **"Create Account"**

**You'll be automatically logged in** and redirected to the dashboard.

### 2.3 Explore Dashboard

You're now viewing the main dashboard:

```
Dashboard Overview
├─ 📊 Statistics (0 agents, 0 MCPs, 0 verifications)
├─ 📈 Trust Score Trend (empty chart)
├─ 🚨 Security Alerts (no threats)
└─ 📝 Recent Activity (account created)
```

**Next**: Let's register your first AI agent!

---

## Step 3: Register Your First Agent (7 minutes)

### Option A: Using Python SDK (Recommended)

**3.1 Install SDK**

```bash
# Clone SDK (if not already in repo)
cd agent-identity-management/sdks/python

# Install
pip install -e .
```

**3.2 Create Agent Script**

Create `my_first_agent.py`:

```python
#!/usr/bin/env python3
from aim_sdk import AIMClient
import asyncio

async def main():
    # Initialize AIM client
    client = AIMClient(
        api_url="http://localhost:8080",
        email="your-email@example.com",
        password="SecurePassword123!"
    )

    # Register agent
    print("Registering agent...")
    agent = await client.register_agent(
        name="my-first-agent",
        agent_type="ai_agent",
        description="My first AI agent with AIM"
    )

    print(f"✅ Agent registered!")
    print(f"   ID: {agent['id']}")
    print(f"   Name: {agent['name']}")
    print(f"   Trust Score: {agent['trust_score']}/100")

    # Verify agent (cryptographic proof)
    print("\nVerifying agent...")
    result = await client.verify_agent(agent['id'])

    print(f"✅ Agent verified!")
    print(f"   New Trust Score: {result['trust_score']}/100")
    print(f"   Status: {result['status']}")

if __name__ == "__main__":
    asyncio.run(main())
```

**3.3 Run Script**

```bash
python my_first_agent.py
```

**Expected Output**:
```
Registering agent...
✅ Agent registered!
   ID: 7f3a8b4c-5d6e-4f7g-8h9i-0j1k2l3m4n5o
   Name: my-first-agent
   Trust Score: 50/100

Verifying agent...
✅ Agent verified!
   New Trust Score: 55/100
   Status: verified
```

### Option B: Using Dashboard UI

**3.1 Navigate to Agents Page**

1. In the dashboard, click **"Agents"** in the sidebar
2. Click **"+ Register Agent"** button

**3.2 Fill Registration Form**

```
┌─────────────────────────────────┐
│   Register New Agent            │
│                                 │
│  Name:                          │
│  [my-first-agent]               │
│                                 │
│  Type:                          │
│  [ AI Agent ▼ ]                 │
│                                 │
│  Description:                   │
│  [My first agent with AIM]      │
│                                 │
│  [ Cancel ]  [ Register ]       │
└─────────────────────────────────┘
```

3. Click **"Register"**

**3.3 View Agent Details**

You'll see your new agent in the agents list:

```
my-first-agent
├─ Trust Score: 50/100
├─ Type: AI Agent
├─ Status: Active
├─ Created: Just now
└─ Verifications: 0
```

Click on the agent to see full details.

### Option C: Using API Directly

```bash
# 1. Login to get token
TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "your-email@example.com",
    "password": "SecurePassword123!"
  }' | jq -r '.token')

# 2. Register agent
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-first-agent",
    "agent_type": "ai_agent",
    "description": "My first AI agent"
  }'

# Expected response:
# {
#   "id": "uuid",
#   "name": "my-first-agent",
#   "trust_score": 50,
#   "status": "active"
# }
```

---

## 🎉 Success! What's Next?

### You've Completed Quick Start!

You now have:
- ✅ AIM running locally
- ✅ User account created
- ✅ First agent registered and verified
- ✅ Trust score tracking enabled

### Recommended Next Steps

**1. Connect MCP Servers** (5 minutes)

Register a Model Context Protocol server:

```python
from aim_sdk import AIMClient

client = AIMClient(api_url="http://localhost:8080", ...)

# Register MCP server
mcp = await client.register_mcp_server(
    name="filesystem-mcp",
    url="http://localhost:3100",
    capabilities=["read", "write", "search"]
)

# Connect agent to MCP
await client.connect_agent_to_mcp(agent_id, mcp['id'])
```

**2. Explore Security Features** (10 minutes)

- Navigate to **Dashboard → Security**
- View security threat dashboard
- Set up trust score alerts
- Review audit logs in **Monitoring**

**3. Test Trust Scoring** (10 minutes)

```python
# Trigger verification failures to see trust score drop
await client.verify_agent(agent_id)  # Success: +2-5 points
await client.verify_agent("invalid-id")  # Failure: -5-10 points

# View trust score history
history = await client.get_trust_score_history(agent_id)
```

**4. Set Up API Keys** (5 minutes)

- Navigate to **Dashboard → API Keys**
- Click **"Generate API Key"**
- Use API key for programmatic access

```bash
# Use API key instead of JWT token
curl -H "X-API-Key: your-api-key" \
  http://localhost:8080/api/v1/agents
```

**5. Try Advanced Features**

- **Multi-Agent Management**: Register multiple agents
- **Compliance Reports**: Generate audit reports
- **Custom Alerts**: Set up Slack/email notifications
- **SDK Integrations**: Try Go or JavaScript SDKs

---

## 📚 Additional Resources

### Documentation

- **[README.md](../README.md)** - Project overview and setup
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture deep-dive
- **[SECURITY.md](SECURITY.md)** - Security best practices
- **[TRUST_SCORING.md](TRUST_SCORING.md)** - Trust score algorithm explained
- **[API Documentation](API.md)** - Complete API reference

### SDK Guides

- **[Python SDK](../sdks/python/README.md)** - Python integration guide
- **[Go SDK](../sdks/go/README.md)** - Go integration guide
- **[JavaScript SDK](../sdks/javascript/README.md)** - JS/TS integration guide

### Video Tutorials

- **Getting Started** (10 min) - [Watch on YouTube](#)
- **Trust Scoring Explained** (15 min) - [Watch on YouTube](#)
- **Production Deployment** (30 min) - [Watch on YouTube](#)

---

## ❓ Troubleshooting

### Common Issues

**Problem**: Docker services not starting

```bash
# Check Docker is running
docker --version

# Check disk space
df -h

# Reset Docker
docker compose down -v
docker system prune -a
docker compose up -d
```

**Problem**: Port 8080 or 3000 already in use

```bash
# Find process using port
lsof -i :8080
lsof -i :3000

# Kill process
kill -9 <PID>

# Or change ports in docker-compose.yml
```

**Problem**: Cannot connect to backend

```bash
# Check backend logs
docker compose logs backend

# Check database connection
docker compose exec backend /app/healthcheck

# Restart backend
docker compose restart backend
```

**Problem**: Frontend shows "Network Error"

```bash
# Check NEXT_PUBLIC_API_URL in apps/web/.env.local
# Should be: NEXT_PUBLIC_API_URL=http://localhost:8080

# Rebuild frontend
docker compose build frontend
docker compose up -d frontend
```

### Get Help

- **GitHub Issues**: [Report a bug](https://github.com/opena2a-org/agent-identity-management/issues)
- **Discord Community**: [Join our Discord](#)
- **Email Support**: support@opena2a.org

---

## 🎓 Learning Path

### Beginner Track (2 hours)

1. ✅ Complete Quick Start (15 min)
2. Register 5 agents (15 min)
3. Connect MCP servers (15 min)
4. Explore dashboard features (30 min)
5. Generate first compliance report (15 min)
6. Set up API key authentication (10 min)
7. Review audit logs (10 min)

### Intermediate Track (4 hours)

1. Integrate Python SDK into your project (1 hour)
2. Implement trust score monitoring (30 min)
3. Set up security alerts (30 min)
4. Create custom agents (1 hour)
5. Test MCP auto-detection (30 min)
6. Deploy to staging environment (30 min)

### Advanced Track (8 hours)

1. Production Kubernetes deployment (2 hours)
2. Set up monitoring (Prometheus/Grafana) (1 hour)
3. Configure SSO (OAuth/OIDC) (1 hour)
4. Implement compliance reporting (1 hour)
5. Perform load testing (1 hour)
6. Security hardening (1 hour)
7. Disaster recovery testing (1 hour)

---

## 🏆 Achievement Unlocked!

**🎖️ Quick Start Champion**

You've successfully completed the AIM Quick Start guide!

**What you've learned**:
- Setting up AIM locally with Docker
- Creating user accounts
- Registering AI agents
- Cryptographic verification
- Trust score basics
- Dashboard navigation

**Share your achievement**:
- Tweet with #AIMAchievement
- Join our Discord community
- Star the GitHub repo ⭐

---

**Next**: [SDK Integration Guide](SDK_INTEGRATION_GUIDE.md) or [Production Deployment](PRODUCTION.md)

**Need Help?** Check [Troubleshooting](#troubleshooting) or [open an issue](https://github.com/opena2a-org/agent-identity-management/issues).

---

**Maintained by**: OpenA2A Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026
