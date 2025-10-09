# 👥 AIM User Roles & Dashboard Architecture

**Date**: October 6, 2025
**Status**: Design Document - Ready for Implementation

---

## 🎯 Role-Based Access Control (RBAC)

AIM supports **4 user roles** with distinct capabilities and dashboard views:

### 1. **Admin** (Full Control)
**Persona**: IT Director, Security Administrator
**Scope**: Organization-wide management

**Capabilities**:
- ✅ Manage ALL agents in organization (view, create, edit, delete, verify)
- ✅ Manage ALL MCP servers in organization
- ✅ Manage ALL users (approve/reject registrations, change roles, deactivate)
- ✅ View ALL verifications organization-wide
- ✅ Manage security threats and incidents
- ✅ View ALL audit logs
- ✅ Configure organization settings
- ✅ Manage API keys for all agents
- ✅ Export compliance reports

**Dashboard Access**:
- `/dashboard` - Organization-wide metrics
- `/dashboard/agents` - ALL agents (organization-wide)
- `/dashboard/mcp` - ALL MCP servers (organization-wide)
- `/dashboard/verifications` - ALL verifications
- `/dashboard/security` - Security dashboard
- `/dashboard/api-keys` - ALL API keys (organization-wide)
- `/dashboard/admin/users` - User management ⭐️
- `/dashboard/admin/alerts` - System alerts
- `/dashboard/admin/audit-logs` - Complete audit trail

---

### 2. **Manager** (Team Lead)
**Persona**: DevOps Manager, Engineering Manager
**Scope**: Team-level management

**Capabilities**:
- ✅ Manage own agents + team agents
- ✅ Manage own MCP servers + team MCP servers
- ❌ Cannot manage other teams' agents/MCPs
- ✅ View team verifications
- ✅ View security threats affecting team
- ✅ Approve/reject agent registrations from team members
- ❌ Cannot manage users
- ❌ Cannot view organization-wide audit logs

**Dashboard Access**:
- `/dashboard` - Team metrics (filtered)
- `/dashboard/agents` - Team agents only
- `/dashboard/mcp` - Team MCP servers only
- `/dashboard/verifications` - Team verifications
- `/dashboard/security` - Team security events
- `/dashboard/api-keys` - Team API keys
- ❌ No `/dashboard/admin/*` access

---

### 3. **Member** (Standard User)
**Persona**: Developer, Data Analyst, AI Engineer
**Scope**: Personal workspace

**Capabilities**:
- ✅ Manage own agents only
- ✅ Manage own MCP servers only
- ✅ View own verifications only
- ✅ View security events affecting own agents
- ✅ Create API keys for own agents
- ❌ Cannot see other users' agents/MCPs
- ❌ Cannot manage users
- ❌ Cannot approve registrations

**Dashboard Access**:
- `/dashboard` - Personal metrics
- `/dashboard/my-agents` - Own agents only
- `/dashboard/my-mcp` - Own MCP servers only
- `/dashboard/my-verifications` - Own verifications
- `/dashboard/my-api-keys` - Own API keys
- ❌ No `/dashboard/admin/*` access
- ❌ No organization-wide views

---

### 4. **Viewer** (Read-Only)
**Persona**: Auditor, Compliance Officer, Stakeholder
**Scope**: Read-only across organization

**Capabilities**:
- ✅ View ALL agents (read-only)
- ✅ View ALL MCP servers (read-only)
- ✅ View ALL verifications (read-only)
- ✅ View security dashboard (read-only)
- ✅ Export compliance reports
- ❌ Cannot create/edit/delete anything
- ❌ Cannot manage users
- ❌ Cannot trigger verifications

**Dashboard Access**:
- `/dashboard` - Organization metrics (read-only)
- `/dashboard/agents` - ALL agents (read-only)
- `/dashboard/mcp` - ALL MCP servers (read-only)
- `/dashboard/verifications` - ALL verifications (read-only)
- `/dashboard/security` - Security dashboard (read-only)
- ❌ No edit/delete buttons visible
- ❌ No `/dashboard/admin/*` access

---

## 🚪 User Registration & Onboarding

### Registration Methods

AIM supports **3 registration methods**:

#### Method 1: SSO (Single Sign-On) ⭐️ **RECOMMENDED**
**Providers**: Google, Microsoft/Azure AD, Okta

**Flow**:
1. User clicks "Sign in with Google/Microsoft/Okta"
2. OAuth2/OIDC redirect to provider
3. User authenticates with provider
4. Provider returns to AIM with user profile
5. **NEW**: User registration goes to **Pending** status
6. **NEW**: Admin receives notification
7. **NEW**: Admin reviews and approves/rejects
8. Upon approval: User gets access with assigned role (default: Member)

**Self-Service Fields** (user fills during first login):
- First Name ✅
- Last Name ✅
- Job Title (optional)
- Department (optional)
- Reason for access (required for approval)

**Admin Approval Fields**:
- Role assignment (Admin, Manager, Member, Viewer)
- Team assignment (optional)
- Access restrictions (optional)
- Approval notes

---

#### Method 2: Basic Auth (Username/Password) 🔐 **FOR NON-SSO COMPANIES**
**Use Case**: Small companies, air-gapped environments, no SSO provider

**Registration Flow**:
1. User navigates to `/register`
2. User fills registration form:
   - Email address (required, becomes username)
   - Password (min 12 chars, complexity rules)
   - First Name (required)
   - Last Name (required)
   - Organization (auto-detected or selected)
   - Job Title (optional)
   - Department (optional)
   - Reason for access (required)
3. System sends verification email
4. User clicks verification link
5. **Status**: Pending (awaiting admin approval)
6. Admin receives notification
7. Admin reviews profile and approves/rejects
8. Upon approval: User can log in with assigned role

**Password Requirements**:
- Minimum 12 characters
- At least 1 uppercase letter
- At least 1 lowercase letter
- At least 1 number
- At least 1 special character
- Not in common password list
- Password expiry: 90 days

**Login Flow**:
1. User navigates to `/login`
2. Enters email + password
3. Optional: MFA/2FA code (if enabled)
4. System validates credentials
5. JWT token issued
6. Redirect to dashboard

---

#### Method 3: Admin-Created Accounts 👤 **FOR MANAGED ACCESS**
**Use Case**: Enterprise with strict access control

**Flow**:
1. Admin navigates to `/dashboard/admin/users`
2. Clicks "Create User" button
3. Fills form:
   - Email (required)
   - First Name (required)
   - Last Name (required)
   - Role (required)
   - Team (optional)
   - Send invitation email? (checkbox)
4. If invitation email enabled:
   - User receives email with temporary password
   - User logs in and must change password
5. If invitation email disabled:
   - Admin manually provides credentials
   - User logs in with admin-provided password

---

## 📊 Dashboard Views by Role

### Admin Dashboard (`/dashboard`)

**Metrics Cards** (Top Row):
- Total Users (organization-wide)
- Total Agents (organization-wide)
- Total Verifications (last 24h)
- Pending User Registrations ⭐️ **NEW** (requires action)

**Charts**:
- Verification Trend (24h, organization-wide)
- Agent Activity by Team
- **NEW**: User Registration Trend (last 30 days)
- **NEW**: Active Users by Role

**Tables**:
- Recent Verifications (organization-wide)
- Pending User Approvals ⭐️ **NEW** (action required)
- Active Security Threats

**Quick Actions**:
- Approve/Reject Pending Users
- View All Agents
- View Security Dashboard
- Export Compliance Report

---

### Manager Dashboard (`/dashboard`)

**Metrics Cards** (Top Row):
- Team Members
- Team Agents
- Team Verifications (last 24h)
- Team Success Rate

**Charts**:
- Team Verification Trend (24h)
- Agent Distribution by Team Member
- Team Performance vs Organization Average

**Tables**:
- Recent Team Verifications
- Team Agents
- Team MCP Servers

**Quick Actions**:
- View Team Agents
- View Team Verifications
- Register New Agent

---

### Member Dashboard (`/dashboard`)

**Metrics Cards** (Top Row):
- My Agents
- My MCP Servers
- My Verifications (last 24h)
- My Success Rate

**Charts**:
- My Verification Trend (7 days)
- My Agent Activity
- My Top Agents by Usage

**Tables**:
- My Recent Verifications
- My Agents
- My API Keys

**Quick Actions**:
- Register New Agent
- Register MCP Server
- Create API Key
- View My Verifications

---

### Viewer Dashboard (`/dashboard`)

**Metrics Cards** (Top Row):
- Total Agents (org-wide, read-only)
- Total Verifications (24h, read-only)
- Security Alerts (read-only)
- Compliance Score (read-only)

**Charts**:
- Organization Verification Trend
- Compliance Metrics
- Security Posture

**Tables**:
- Recent Verifications (read-only)
- All Agents (read-only)
- Security Events (read-only)

**Quick Actions**:
- Export Compliance Report
- View Security Dashboard (read-only)
- View Audit Logs (read-only)

---

## 🔒 Permission Matrix

| Feature | Admin | Manager | Member | Viewer |
|---------|-------|---------|--------|--------|
| **User Management** |
| View all users | ✅ | ❌ | ❌ | ❌ |
| Approve/reject users | ✅ | ❌ | ❌ | ❌ |
| Change user roles | ✅ | ❌ | ❌ | ❌ |
| Deactivate users | ✅ | ❌ | ❌ | ❌ |
| Create users | ✅ | ❌ | ❌ | ❌ |
| **Agent Management** |
| View all agents (org) | ✅ | ❌ | ❌ | ✅ |
| View team agents | ✅ | ✅ | ❌ | ✅ |
| View own agents | ✅ | ✅ | ✅ | ✅ |
| Create agent | ✅ | ✅ | ✅ | ❌ |
| Edit any agent | ✅ | Team only | Own only | ❌ |
| Delete any agent | ✅ | Team only | Own only | ❌ |
| Verify agent | ✅ | ✅ | ✅ | ❌ |
| **MCP Server Management** |
| View all MCP (org) | ✅ | ❌ | ❌ | ✅ |
| View team MCP | ✅ | ✅ | ❌ | ✅ |
| View own MCP | ✅ | ✅ | ✅ | ✅ |
| Create MCP server | ✅ | ✅ | ✅ | ❌ |
| Edit MCP server | ✅ | Team only | Own only | ❌ |
| Delete MCP server | ✅ | Team only | Own only | ❌ |
| Verify MCP server | ✅ | ✅ | ✅ | ❌ |
| **Verifications** |
| View all verifications | ✅ | ❌ | ❌ | ✅ |
| View team verifications | ✅ | ✅ | ❌ | ✅ |
| View own verifications | ✅ | ✅ | ✅ | ✅ |
| **Security** |
| View security dashboard | ✅ | Team only | Own only | ✅ |
| Mitigate threats | ✅ | Team only | ❌ | ❌ |
| Resolve incidents | ✅ | Team only | ❌ | ❌ |
| **API Keys** |
| View all API keys | ✅ | ❌ | ❌ | ✅ |
| View team API keys | ✅ | ✅ | ❌ | ✅ |
| View own API keys | ✅ | ✅ | ✅ | ✅ |
| Create API key | ✅ | ✅ | ✅ | ❌ |
| Revoke any API key | ✅ | Team only | Own only | ❌ |
| **Admin Functions** |
| View audit logs | ✅ | ❌ | ❌ | ✅ |
| Export compliance | ✅ | ❌ | ❌ | ✅ |
| Manage alerts | ✅ | ❌ | ❌ | ❌ |
| Configure org settings | ✅ | ❌ | ❌ | ❌ |

---

## 🎨 UI Changes Required

### 1. User Registration Page (`/register`)
**NEW PAGE** - For Basic Auth registration

Components needed:
- Registration form
- Email verification flow
- Password strength meter
- Terms of Service checkbox
- Privacy Policy link

---

### 2. Login Page (`/login`)
**NEW PAGE** - For Basic Auth login

Components needed:
- Email/Password form
- "Forgot Password" link
- SSO buttons (Google, Microsoft, Okta)
- "Don't have an account? Register" link

---

### 3. Pending User Approvals (`/dashboard/admin/users/pending`)
**NEW PAGE** - Admin-only

Components needed:
- Table of pending registrations
- User detail modal showing:
  - Profile information
  - Registration reason
  - Registration date
  - IP address / location
- Approve/Reject buttons with role assignment
- Bulk approve/reject functionality
- Email notification toggle

---

### 4. Dashboard Customization by Role
**MODIFY EXISTING** - `/dashboard/page.tsx`

Changes needed:
- Detect user role from context
- Show different metrics based on role
- Filter data by role permissions
- Add "Pending Approvals" card for admins

---

### 5. Sidebar Navigation by Role
**MODIFY EXISTING** - `/components/sidebar.tsx`

Changes needed:
- Show/hide menu items based on role:
  - Admin: ALL items visible
  - Manager: No `/admin/*` items
  - Member: Only "My *" items
  - Viewer: All items but read-only

---

### 6. Role Badge in Sidebar
**MODIFY EXISTING** - `/components/sidebar.tsx`

Add user profile section showing:
- User name
- Email
- **Role badge** (Admin, Manager, Member, Viewer)
- Logout button

---

## 📋 Updated User Workflows

### Workflow 1: New User Self-Registration (SSO)
**Persona**: New Employee - Emily Chen
**Time**: 3-5 minutes

**Steps**:
1. Navigate to https://aim.company.com
2. Click "Sign in with Google"
3. Authenticate with Google
4. **NEW**: Fill self-registration form:
   - First Name: Emily
   - Last Name: Chen
   - Job Title: Software Engineer
   - Department: Engineering
   - Reason: "Need to manage AI agents for my development work"
5. Submit registration
6. See message: "Your registration is pending approval. You'll receive an email when approved."
7. **Admin receives notification**
8. **Admin reviews and approves** with role: Member
9. **Emily receives approval email**
10. Emily logs in and sees Member dashboard

---

### Workflow 2: New User Self-Registration (Basic Auth)
**Persona**: New Employee - Marco Rodriguez
**Time**: 5-7 minutes

**Steps**:
1. Navigate to https://aim.company.com/register
2. Fill registration form:
   - Email: marco@company.com
   - Password: SecureP@ssw0rd123
   - Confirm Password: SecureP@ssw0rd123
   - First Name: Marco
   - Last Name: Rodriguez
   - Job Title: DevOps Engineer
   - Department: Infrastructure
   - Reason: "Managing MCP servers for infrastructure automation"
3. Submit registration
4. Receive verification email
5. Click verification link
6. See message: "Email verified. Registration pending approval."
7. **Admin receives notification**
8. **Admin approves** with role: Manager
9. **Marco receives approval email**
10. Marco logs in at /login with email + password
11. Marco sees Manager dashboard

---

### Workflow 3: Admin Approves Pending Registration
**Persona**: Admin - Sarah Johnson
**Time**: 2-3 minutes per user

**Steps**:
1. Log in to AIM dashboard
2. See notification: "3 pending user registrations"
3. Navigate to `/dashboard/admin/users/pending`
4. See table of pending users
5. Click "Review" on Emily Chen's registration
6. Modal opens showing:
   - Name: Emily Chen
   - Email: emily@company.com
   - Job Title: Software Engineer
   - Department: Engineering
   - Reason: "Need to manage AI agents for my development work"
   - Registered: 2 hours ago
   - IP: 192.168.1.100
7. Admin selects role: "Member"
8. Admin clicks "Approve"
9. System sends approval email to Emily
10. Emily removed from pending list
11. Emily can now log in

---

### Workflow 4: Admin Rejects Suspicious Registration
**Persona**: Admin - Sarah Johnson
**Time**: 1-2 minutes

**Steps**:
1. Review pending registration for "Unknown User"
2. See suspicious details:
   - Email: random123@tempmail.com
   - Reason: "just testing"
   - IP: Foreign location
3. Click "Reject" button
4. Add rejection reason: "Suspicious registration from temporary email"
5. Confirm rejection
6. System sends rejection email (optional)
7. User registration deleted

---

### Workflow 5: Member Views Own Dashboard
**Persona**: Member - Emily Chen
**Time**: 30 seconds

**Steps**:
1. Log in to AIM
2. See personalized dashboard:
   - "Welcome back, Emily!"
   - Metrics: My Agents: 3, My Verifications: 47, Success Rate: 98%
3. See only own agents in tables
4. Sidebar shows only: Dashboard, My Agents, My MCP, My Verifications, My API Keys
5. No admin menu visible

---

### Workflow 6: Admin Views Organization Dashboard
**Persona**: Admin - Sarah Johnson
**Time**: 30 seconds

**Steps**:
1. Log in to AIM
2. See organization-wide dashboard:
   - Total Users: 25
   - Total Agents: 78
   - Pending Registrations: 2 (requires action)
3. See all agents from all users
4. Sidebar shows ALL menu items including Admin section
5. Can navigate to any page

---

## 🔧 Implementation Priority

### Phase 1: Core RBAC (2 hours)
1. Add role-based sidebar filtering
2. Add role badge to user profile section
3. Filter dashboard data by role
4. Add permission checks to all pages

### Phase 2: Registration & Login (3 hours)
1. Create `/register` page (Basic Auth)
2. Create `/login` page
3. Create email verification flow
4. Add SSO buttons to login page

### Phase 3: Pending User Approvals (2 hours)
1. Create `/dashboard/admin/users/pending` page
2. Create user review modal
3. Add approve/reject functionality
4. Add email notifications

### Phase 4: Dashboard Customization (1 hour)
1. Customize dashboard by role
2. Add "Pending Approvals" card for admins
3. Filter metrics by role
4. Update charts to respect role permissions

**Total Estimated Time**: 8 hours

---

**Last Updated**: October 6, 2025
**Status**: ✅ **READY FOR IMPLEMENTATION**
**Next Step**: Integrate into Phase 4 implementation

🎯 **This completes the AIM role-based architecture design!**
