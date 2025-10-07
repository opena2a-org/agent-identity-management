# User & Admin Workflow Analysis - AIM Platform

## 🎯 Your Question
**"When open-source users download AIM and deploy it, how does the admin user get set up? How does the platform manage this?"**

This is a **critical production readiness question** that reveals gaps in the current implementation.

---

## ✅ What's Currently Implemented

### 1. **Auto-Provisioning Logic** (`auth_service.go` lines 60-117)
The platform has smart auto-provisioning that:

```go
// Lines 90-99: First user becomes admin automatically
existingUsers, err := s.userRepo.GetByOrganization(org.ID)
if err != nil {
    return nil, err
}

role := domain.RoleMember
if len(existingUsers) == 0 {
    role = domain.RoleAdmin  // 🎯 First user = Admin!
}
```

**How it works:**
1. User signs in with OAuth (Google/Microsoft/Okta)
2. System extracts email domain (e.g., `cybersecuritynp.org`)
3. System checks if organization exists for that domain
4. **If new domain → creates organization + makes first user Admin**
5. **If existing domain → adds user as Member**

### 2. **Organization Auto-Creation** (`auth_service.go` lines 74-88)
```go
if org == nil {
    // Create new organization
    org = &domain.Organization{
        Name:      emailDomain,           // e.g., "cybersecuritynp.org"
        Domain:    emailDomain,
        PlanType:  "free",
        MaxAgents: 100,
        MaxUsers:  10,
        IsActive:  true,
    }

    if err := s.orgRepo.Create(org); err != nil {
        return nil, fmt.Errorf("failed to create organization: %w", err)
    }
}
```

### 3. **Database Schema** (`001_initial_schema_fixed.sql`)
```sql
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    role VARCHAR(50) NOT NULL DEFAULT 'member',  -- admin, manager, member, viewer
    provider VARCHAR(50) NOT NULL,               -- google, microsoft, okta
    provider_id VARCHAR(255) NOT NULL,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, email),
    UNIQUE(provider, provider_id)
);
```

---

## ❌ Critical Gaps - What's Missing

### 1. **No Initial Super Admin Setup** ⚠️
**Problem:**
- When someone deploys AIM for the first time, there's **no way to bootstrap the first admin**
- OAuth requires external identity providers (Google/Microsoft)
- What if user wants local admin without OAuth?

**Enterprise users expect:**
```bash
# During initial setup
docker-compose up -d
# Script should prompt:
# "Enter initial admin email: admin@company.com"
# "Enter admin password: ********"
# "Confirm password: ********"
# ✅ Created super admin: admin@company.com
```

**Missing:**
- [ ] Seed script for initial admin creation
- [ ] Local authentication (email/password) as fallback
- [ ] Bootstrap CLI command (`aim init --admin-email=admin@company.com`)

### 2. **No Self-Service User Registration** ⚠️
**Problem:**
- Users can only login via OAuth
- No way for employees to request access
- No approval workflow for new users

**What's needed:**
```typescript
// Missing registration flow:
1. Employee visits /register
2. Fills form: email, name, department, manager
3. System creates pending user
4. Admin receives notification
5. Admin approves/rejects
6. User receives email notification
7. User can login
```

**Missing endpoints:**
- [ ] `POST /api/v1/auth/register` - Self-service registration
- [ ] `GET /api/v1/admin/pending-users` - List pending approvals
- [ ] `POST /api/v1/admin/users/:id/approve` - Approve user
- [ ] `POST /api/v1/admin/users/:id/reject` - Reject user

### 3. **No Role-Based Access Control (RBAC) Enforcement** ⚠️
**Problem:**
- Roles are stored in database but not enforced consistently
- Admin middleware exists but not used on all sensitive endpoints
- Users can potentially access resources they shouldn't

**Current middleware (`middleware/auth.go`):**
```go
// admin_middleware.go exists but not applied everywhere
func AdminMiddleware() fiber.Handler {
    return func(c fiber.Ctx) error {
        role := c.Locals("role").(string)
        if role != "admin" {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "Admin access required",
            })
        }
        return c.Next()
    }
}
```

**Missing:**
- [ ] Manager middleware (can manage agents but not users)
- [ ] Member middleware (read-only access to own resources)
- [ ] Viewer middleware (read-only access, no modifications)
- [ ] Resource-level permissions (who can create/delete agents)

### 4. **No User Invitation System** ⚠️
**Problem:**
- Admins can't invite specific users to join
- No controlled onboarding process
- Users must discover the platform themselves

**What's needed:**
```go
// Missing invitation flow:
POST /api/v1/admin/users/invite
{
  "email": "john@company.com",
  "role": "member",
  "expires_in_days": 7
}

// Returns invitation link:
{
  "invitation_url": "https://aim.company.com/accept-invite?token=abc123",
  "expires_at": "2025-10-13T00:00:00Z"
}
```

**Missing tables:**
```sql
-- User invitations
CREATE TABLE IF NOT EXISTS user_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    invited_by UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, email)
);
```

### 5. **No Audit Trail for Admin Actions** ⚠️
**Problem:**
- No record of who created/modified/deleted users
- Can't track role changes
- No compliance reporting (SOC 2, HIPAA, GDPR require this)

**What's needed:**
```sql
-- Admin action audit log
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    admin_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(100) NOT NULL,  -- create_user, update_role, delete_user
    target_user_id UUID REFERENCES users(id),
    old_value TEXT,  -- JSON of old state
    new_value TEXT,  -- JSON of new state
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 6. **No Multi-Tenancy Isolation** ⚠️
**Problem:**
- Organizations exist but isolation not enforced everywhere
- Users might see resources from other organizations
- Security vulnerability for multi-tenant SaaS

**What's needed:**
- [ ] Middleware to inject organization_id filter on all queries
- [ ] Database-level row-level security (RLS) policies
- [ ] API endpoint isolation tests

---

## 🚀 Recommended Implementation Plan

### Phase 1: Bootstrap & Initial Admin (Critical - Week 1)
**Priority: HIGHEST**

1. **Create seed script** (`apps/backend/scripts/seed_admin.go`)
```go
// Run during first deployment
func SeedInitialAdmin() {
    email := os.Getenv("INITIAL_ADMIN_EMAIL")
    password := os.Getenv("INITIAL_ADMIN_PASSWORD")

    // Create default organization
    org := &domain.Organization{
        Name:      "Default Organization",
        Domain:    "localhost",
        PlanType:  "free",
        MaxAgents: 100,
        MaxUsers:  10,
        IsActive:  true,
    }

    // Create super admin user
    user := &domain.User{
        OrganizationID: org.ID,
        Email:          email,
        Name:           "Super Admin",
        Role:           domain.RoleAdmin,
        Provider:       "local",
        ProviderID:     "super-admin",
    }

    // Hash password and store
    // ...
}
```

2. **Add local authentication**
- [ ] Password hashing (bcrypt)
- [ ] Login endpoint for local users
- [ ] Password reset flow

3. **Add bootstrap CLI command**
```bash
go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \
  --admin-password=SecurePassword123! \
  --org-name="My Company"
```

### Phase 2: User Management UI (Week 2)
**Priority: HIGH**

1. **Self-Service Registration**
- [ ] `/register` page in frontend
- [ ] Registration form with validation
- [ ] Email verification flow
- [ ] Admin approval queue

2. **User Invitation System**
- [ ] Invitation creation UI in admin panel
- [ ] Email templates for invitations
- [ ] Accept invitation page
- [ ] Invitation expiration handling

3. **Admin User Management**
- [ ] Enhanced user list with filters (pending, active, suspended)
- [ ] Bulk actions (approve multiple, delete multiple)
- [ ] User detail modal
- [ ] Activity history per user

### Phase 3: RBAC Enforcement (Week 3)
**Priority: HIGH**

1. **Middleware Stack**
```go
// Protect all admin routes
admin.Use(middleware.AuthMiddleware(jwtService))
admin.Use(middleware.AdminMiddleware())

// Protect manager routes
manager.Use(middleware.AuthMiddleware(jwtService))
manager.Use(middleware.ManagerOrAdminMiddleware())

// Protect member routes
member.Use(middleware.AuthMiddleware(jwtService))
member.Use(middleware.MemberOrHigherMiddleware())
```

2. **Permission System**
```go
type Permission string

const (
    PermCreateAgent    Permission = "create:agent"
    PermDeleteAgent    Permission = "delete:agent"
    PermManageUsers    Permission = "manage:users"
    PermViewAuditLogs  Permission = "view:audit-logs"
    // ... etc
)

var RolePermissions = map[domain.UserRole][]Permission{
    domain.RoleAdmin: {
        PermCreateAgent, PermDeleteAgent, PermManageUsers, PermViewAuditLogs,
    },
    domain.RoleManager: {
        PermCreateAgent, PermDeleteAgent,
    },
    domain.RoleMember: {
        PermCreateAgent,
    },
    domain.RoleViewer: {},
}
```

### Phase 4: Audit & Compliance (Week 4)
**Priority: MEDIUM**

1. **Audit Logging**
- [ ] Middleware to log all admin actions
- [ ] Audit log viewer in admin panel
- [ ] Export audit logs (CSV, JSON)
- [ ] Retention policy configuration

2. **Compliance Reports**
- [ ] User access review report
- [ ] Permission changes report
- [ ] Data retention compliance
- [ ] GDPR data export

---

## 🎬 Deployment Scenarios

### Scenario 1: Fresh Installation (No Users)
```bash
# 1. Deploy with Docker Compose
docker-compose up -d

# 2. Run bootstrap script
docker-compose exec backend go run cmd/bootstrap/main.go \
  --admin-email=admin@company.com \
  --admin-password=SecurePassword123!

# 3. Admin logs in and invites users
# 4. Users accept invitations and complete registration
```

### Scenario 2: Existing OAuth Setup
```bash
# 1. Deploy with OAuth configured
GOOGLE_CLIENT_ID=xxx \
GOOGLE_CLIENT_SECRET=yyy \
docker-compose up -d

# 2. First employee from company.com signs in via Google
# → Becomes admin automatically (first user rule)

# 3. Admin invites other employees or enables self-registration
```

### Scenario 3: Enterprise On-Premise
```bash
# 1. Deploy with custom organization
docker-compose exec backend go run cmd/bootstrap/main.go \
  --admin-email=admin@enterprise.local \
  --admin-password=*** \
  --org-name="Enterprise Corp" \
  --org-domain="enterprise.local" \
  --max-users=1000 \
  --max-agents=5000

# 2. Configure LDAP/SAML integration
# 3. Import existing users from directory
# 4. Assign roles based on AD groups
```

---

## 📋 Action Items Summary

### Immediate (Next Session)
- [ ] Create bootstrap script for initial admin
- [ ] Add local authentication (email/password)
- [ ] Document deployment process with admin setup

### Short Term (This Week)
- [ ] Build user invitation system
- [ ] Add pending user approval workflow
- [ ] Enhance RBAC middleware enforcement

### Medium Term (Next Week)
- [ ] Implement audit logging
- [ ] Add compliance reports
- [ ] Multi-tenancy isolation tests

### Long Term (Future)
- [ ] LDAP/SAML integration
- [ ] Custom permission system
- [ ] User import/export tools

---

## 🔒 Security Considerations

1. **Password Requirements**
   - Minimum 12 characters
   - Mix of uppercase, lowercase, numbers, symbols
   - Not in common password list
   - Password rotation policy (90 days)

2. **Account Lockout**
   - Lock after 5 failed attempts
   - 30-minute cooldown
   - Email notification on lockout

3. **Session Management**
   - JWT expiration: 24 hours
   - Refresh token expiration: 30 days
   - Ability to revoke all sessions

4. **Multi-Factor Authentication (MFA)**
   - TOTP support (Google Authenticator)
   - SMS backup (Twilio)
   - Recovery codes

---

## 💡 Conclusion

**Current State:**
- ✅ Auto-provisioning works for OAuth users
- ✅ First user becomes admin automatically
- ✅ Basic role system exists

**Critical Gaps:**
- ❌ No bootstrap for initial admin
- ❌ No local authentication fallback
- ❌ No user invitation system
- ❌ RBAC not fully enforced
- ❌ No audit trail

**Recommendation:**
Implement Phase 1 (Bootstrap & Initial Admin) immediately before deploying to production. This is a **blocking issue** for enterprise adoption.

---

**Last Updated:** October 6, 2025
**Status:** Analysis Complete - Ready for Implementation
