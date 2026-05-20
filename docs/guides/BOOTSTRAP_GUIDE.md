# 🚀 AIM Bootstrap Guide - Initial Admin Setup

This guide explains how to set up the initial administrator account for Agent Identity Management (AIM) during first deployment.

---

## 📋 Prerequisites

1. **PostgreSQL 16+** running
2. **Database created** (e.g., `agent_identity`)
3. **Environment variables** configured

```bash
# Required environment variables
export DATABASE_URL="postgresql://user:password@localhost:5432/agent_identity?sslmode=disable"
```

---

## 🎯 Quick Start (Recommended)

### Step 1: Run Database Migrations

```bash
# Navigate to backend directory
cd apps/backend

# Run migrations
go run cmd/migrate/main.go up
```

Expected output:
```
✅ Applied migration: 001_initial_schema_fixed.sql
✅ Applied migration: 002_performance_indexes.sql
✅ Applied migration: 003_local_authentication.up.sql
```

### Step 2: Run Bootstrap Command

```bash
# Bootstrap with required parameters
go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \
  --admin-password="SecurePassword123!" \
  --org-name="My Company"
```

### Alternative: `--default` mode for the canonical OpenA2A admin

If you want the default `admin@opena2a.org` org and admin (the supported path
for evaluation deployments), use `--default`. Bootstrap will fill in the
canonical fields and generate a random password if you don't supply one,
printing the credentials to stdout once.

```bash
# Random password generated, printed once to stdout:
go run cmd/bootstrap/main.go --default

# Or with an operator-chosen password:
go run cmd/bootstrap/main.go --default --admin-password='OperatorChoiceP@ss1'
```

`--default` is idempotent: if the bootstrap_completed marker is already set, the
command exits successfully without changing the existing admin. This makes it
safe to wire into deployment scripts that may run on every container start.

Inside the docker-compose image (post-B2), the binary is shipped as
`/app/aim-bootstrap`, so the equivalent invocation is:

```bash
docker compose run --rm aim-backend /app/aim-bootstrap --default
```

Capture the printed password from the command output before the container is
removed. The admin user is forced to change the password at first login.

Expected output:
```
 █████╗ ██╗███╗   ███╗    ██████╗  ██████╗  ██████╗ ████████╗███████╗████████╗██████╗  █████╗ ██████╗
██╔══██╗██║████╗ ████║    ██╔══██╗██╔═══██╗██╔═══██╗╚══██╔══╝██╔════╝╚══██╔══╝██╔══██╗██╔══██╗██╔══██╗
███████║██║██╔████╔██║    ██████╔╝██║   ██║██║   ██║   ██║   ███████╗   ██║   ██████╔╝███████║██████╔╝
██╔══██║██║██║╚██╔╝██║    ██╔══██╗██║   ██║██║   ██║   ██║   ╚════██║   ██║   ██╔══██╗██╔══██║██╔═══╝
██║  ██║██║██║ ╚═╝ ██║    ██████╔╝╚██████╔╝╚██████╔╝   ██║   ███████║   ██║   ██║  ██║██║  ██║██║
╚═╝  ╚═╝╚═╝╚═╝     ╚═╝    ╚═════╝  ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝

Agent Identity Management - Initial Setup

📊 Connecting to database...

📋 Bootstrap Configuration:
   • Admin Email:    admin@company.com
   • Admin Name:     System Administrator
   • Organization:   My Company
   • Domain:         localhost
   • Max Users:      100
   • Max Agents:     1000

⚠️  This will create the initial admin user and organization. Continue? (yes/no): yes

🚀 Starting bootstrap process...
1️⃣  Checking organization...
   Creating organization 'My Company'...
   ✓ Organization created
2️⃣  Hashing password...
   ✓ Password hashed
3️⃣  Creating admin user...
   ✓ Admin user created (ID: 83018b76-39b0-4dea-bc1b-67c53bb03fc7)
4️⃣  Updating system configuration...
   ✓ System configuration updated

✅ Bootstrap completed successfully!

🔐 Admin Credentials:
   Email:    admin@company.com
   Password: SecurePassword123!

🌐 You can now log in at: http://localhost:3000/login

⚠️  IMPORTANT: Please change the admin password after first login!
```

### Step 3: Start Backend Server

```bash
# Start the API server
go run cmd/server/main.go
```

### Step 4: Login

1. Navigate to `http://localhost:3000/login`
2. Enter admin credentials
3. Change password immediately

---

## 🔧 Advanced Options

### Full Parameter List

```bash
go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \      # Required: Admin email
  --admin-password="Password123!" \       # Required: Secure password
  --admin-name="John Doe" \               # Optional: Admin display name
  --org-name="ACME Corp" \                # Required: Organization name
  --org-domain="acme.com" \               # Optional: Organization domain
  --max-users=500 \                       # Optional: Max users (default: 100)
  --max-agents=5000 \                     # Optional: Max agents (default: 1000)
  --database-url="postgresql://..." \     # Optional: DB URL (uses DATABASE_URL env)
  --yes                                   # Optional: Skip confirmation prompts
```

### Docker Deployment

```bash
# 1. Run bootstrap inside Docker container
docker compose exec backend go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \
  --admin-password="SecurePassword123!" \
  --org-name="My Company" \
  --yes

# 2. Alternative: Run before starting services
docker compose run --rm backend go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \
  --admin-password="SecurePassword123!" \
  --org-name="My Company" \
  --yes
```

### Environment Variable Configuration

```bash
# .env file
DATABASE_URL=postgresql://user:password@localhost:5432/agent_identity
INITIAL_ADMIN_EMAIL=admin@company.com
INITIAL_ADMIN_PASSWORD=SecurePassword123!
INITIAL_ORG_NAME=My Company

# Run bootstrap using env vars
go run cmd/bootstrap/main.go \
  --admin-email=$INITIAL_ADMIN_EMAIL \
  --admin-password=$INITIAL_ADMIN_PASSWORD \
  --org-name="$INITIAL_ORG_NAME"
```

---

## 🔐 Password Requirements

The bootstrap script enforces strong password requirements:

- **Minimum 12 characters**
- **At least 1 uppercase letter** (A-Z)
- **At least 1 lowercase letter** (a-z)
- **At least 1 number** (0-9)
- **At least 1 special character** (!@#$%^&*()_+-=[]{};"'\\|,.<>/?)

### Valid Password Examples:
```
✅ MySecurePass123!
✅ Admin@Company2025
✅ P@ssw0rd!Security
```

### Invalid Password Examples:
```
❌ password123        (no uppercase, no special char)
❌ PASSWORD123!       (no lowercase)
❌ MyPassword!        (no number)
❌ Short1!            (too short)
```

---

## 🧪 Testing Bootstrap

### 1. Check if Bootstrap Completed

```sql
-- Connect to database
psql -U user -d agent_identity

-- Check system config
SELECT * FROM system_config WHERE key = 'bootstrap_completed';
-- Should return: bootstrap_completed | true

-- Check admin user exists
SELECT id, email, name, role, provider, email_verified
FROM users
WHERE role = 'admin' AND provider = 'local';
```

### 2. Test Local Login

```bash
# Using curl
curl -X POST http://localhost:8080/api/v1/auth/login/local \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@company.com",
    "password": "SecurePassword123!"
  }'

# Expected response:
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "user": {
    "id": "83018b76-39b0-4dea-bc1b-67c53bb03fc7",
    "email": "admin@company.com",
    "name": "System Administrator",
    "role": "admin",
    "organization_id": "9a72f03a-0fb2-4352-bdd3-1f930ef6051d"
  }
}
```

---

## 🔄 Re-running Bootstrap

If you need to create additional admin users or re-bootstrap:

```bash
# Bootstrap will prompt for confirmation if already run
go run cmd/bootstrap/main.go \
  --admin-email=admin2@company.com \
  --admin-password="AnotherSecure123!" \
  --org-name="My Company"

# Output:
⚠️  System already bootstrapped!
Do you want to create another admin user? (yes/no): yes
```

---

## 🚨 Troubleshooting

### Error: "password must be at least 12 characters long"
**Solution:** Use a stronger password meeting all requirements

### Error: "failed to connect to database"
**Solution:** Verify `DATABASE_URL` is correct and PostgreSQL is running
```bash
# Test connection
psql "$DATABASE_URL" -c "SELECT version();"
```

### Error: "migration 003_local_authentication not found"
**Solution:** Run migrations first
```bash
go run cmd/migrate/main.go up
```

### Error: "user already exists"
**Solution:** Bootstrap creates admin with conflict handling. Check existing user:
```sql
SELECT * FROM users WHERE email = 'admin@company.com';
```

---

## 📊 What Gets Created

### 1. Organization
```sql
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active)
VALUES (
  '9a72f03a-0fb2-4352-bdd3-1f930ef6051d',
  'My Company',
  'localhost',
  'enterprise',
  1000,
  100,
  true
);
```

### 2. Admin User
```sql
INSERT INTO users (
  id, organization_id, email, name, role, provider, provider_id,
  password_hash, email_verified, created_at, updated_at
) VALUES (
  '83018b76-39b0-4dea-bc1b-67c53bb03fc7',
  '9a72f03a-0fb2-4352-bdd3-1f930ef6051d',
  'admin@company.com',
  'System Administrator',
  'admin',
  'local',
  'local-83018b76-39b0-4dea-bc1b-67c53bb03fc7',
  '$2a$12$...',  -- bcrypt hash
  true,
  NOW(),
  NOW()
);
```

### 3. System Config
```sql
INSERT INTO system_config (key, value, description)
VALUES ('bootstrap_completed', 'true', 'Initial admin bootstrap completed');
```

---

## 🔒 Security Best Practices

1. **Change Default Password Immediately**
   - First action after login should be password change

2. **Use Strong Passwords**
   - Use password manager to generate secure passwords
   - Never reuse passwords across systems

3. **Enable MFA** (when available)
   - Add additional security layer for admin accounts

4. **Limit Admin Accounts**
   - Only create admin users for authorized personnel
   - Use manager/member roles for regular users

5. **Audit Admin Actions**
   - Review audit logs regularly
   - Monitor for suspicious activity

---

## 📝 Next Steps

After bootstrap:

1. ✅ **Login** with admin credentials
2. ✅ **Change password** in user settings
3. ✅ **Configure OAuth** (optional) for team logins
4. ✅ **Invite users** to join organization
5. ✅ **Register agents** and MCP servers
6. ✅ **Configure security policies**

---

## 🆘 Support

If you encounter issues:

1. Check logs: `tail -f /tmp/aim-backend-enhanced.log`
2. Verify database: `psql $DATABASE_URL -c "\dt"`
3. Review documentation: `USER_ADMIN_WORKFLOW_ANALYSIS.md`
4. Open issue: https://github.com/opena2a/agent-identity-management/issues

---

**Last Updated:** October 6, 2025
**AIM Version:** 1.0.0
**Bootstrap Script:** v1.0.0
