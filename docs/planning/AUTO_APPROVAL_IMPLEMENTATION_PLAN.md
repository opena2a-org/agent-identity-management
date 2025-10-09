# Auto-Approval vs Manual Approval Implementation Plan

## Problem Statement

Currently, AIM automatically provisions all SSO users without admin oversight. We need two approval modes:

### Mode 1: Auto-Approval (SSO Trust Model)
- **Use Case**: Enterprise trusts their SSO provider (Google Workspace, Microsoft Entra ID, Okta)
- **Behavior**: If user authenticates via SSO, they're automatically granted access
- **Rationale**: If they're in the SSO directory, they're a valid employee

### Mode 2: Manual Approval (Explicit Control)
- **Use Case**: Enterprises want granular control over who uses AIM
- **Behavior**: New SSO logins create pending registration requests requiring admin approval
- **Rationale**: Additional security layer for sensitive systems

## Current Issues

1. ✅ **Auto-provision is working** (lines 60-122 in auth_service.go)
2. ❌ **No organization settings** to toggle between modes
3. ❌ **Admin can't see all users** in their organization
4. ❌ **No approval workflow** for manual mode
5. ❌ **First user is auto-admin** (good), but subsequent users need visibility

## Implementation Plan

### Phase 1: Database Schema Updates

#### 1.1 Add Organization Settings
```sql
-- Migration: 014_organization_settings.up.sql
ALTER TABLE organizations
ADD COLUMN auto_approve_sso BOOLEAN DEFAULT TRUE,
ADD COLUMN settings JSONB DEFAULT '{}'::jsonb;

COMMENT ON COLUMN organizations.auto_approve_sso IS
'When TRUE, SSO users are automatically approved. When FALSE, admins must manually approve.';
```

#### 1.2 Add User Pending Status
```sql
-- User status enum
CREATE TYPE user_status AS ENUM ('pending', 'active', 'suspended', 'deactivated');

ALTER TABLE users
ADD COLUMN status user_status DEFAULT 'active',
ADD COLUMN approved_by UUID REFERENCES users(id),
ADD COLUMN approved_at TIMESTAMPTZ;

-- Index for pending users query
CREATE INDEX idx_users_org_status ON users(organization_id, status);
```

### Phase 2: Backend Updates

#### 2.1 Update Domain Models

**apps/backend/internal/domain/organization.go**:
```go
type Organization struct {
    // ... existing fields ...
    AutoApproveSSO bool                   `json:"auto_approve_sso"`
    Settings       map[string]interface{} `json:"settings"`
}
```

**apps/backend/internal/domain/user.go**:
```go
type UserStatus string

const (
    UserStatusPending     UserStatus = "pending"
    UserStatusActive      UserStatus = "active"
    UserStatusSuspended   UserStatus = "suspended"
    UserStatusDeactivated UserStatus = "deactivated"
)

type User struct {
    // ... existing fields ...
    Status     UserStatus  `json:"status"`
    ApprovedBy *uuid.UUID  `json:"approved_by,omitempty"`
    ApprovedAt *time.Time  `json:"approved_at,omitempty"`
}
```

#### 2.2 Update Auth Service Logic

**apps/backend/internal/application/auth_service.go**:
```go
func (s *AuthService) autoProvisionUser(ctx context.Context, oauthUser *auth.OAuthUser) (*domain.User, error) {
    // ... existing organization lookup ...

    // Check organization approval mode
    userStatus := domain.UserStatusActive
    if org != nil && !org.AutoApproveSSO {
        userStatus = domain.UserStatusPending
    }

    // First user is always admin and active
    existingUsers, err := s.userRepo.GetByOrganization(org.ID)
    if err != nil {
        return nil, err
    }

    role := domain.RoleMember
    if len(existingUsers) == 0 {
        role = domain.RoleAdmin
        userStatus = domain.UserStatusActive // First user always active
    }

    user := &domain.User{
        OrganizationID: org.ID,
        Email:          oauthUser.Email,
        Name:           oauthUser.Name,
        Role:           role,
        Status:         userStatus,
        Provider:       oauthUser.Provider,
        ProviderID:     oauthUser.ID,
    }

    if err := s.userRepo.Create(user); err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }

    return user, nil
}
```

#### 2.3 Add Admin Service Methods

**apps/backend/internal/application/admin_service.go** (new or update existing):
```go
// GetAllUsers returns all users in admin's organization
func (s *AdminService) GetAllUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
    return s.userRepo.GetByOrganization(adminOrgID)
}

// GetPendingUsers returns users awaiting approval
func (s *AdminService) GetPendingUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
    return s.userRepo.GetByOrganizationAndStatus(adminOrgID, domain.UserStatusPending)
}

// ApproveUser approves a pending user
func (s *AdminService) ApproveUser(ctx context.Context, userID, adminID uuid.UUID) error {
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return err
    }

    if user.Status != domain.UserStatusPending {
        return fmt.Errorf("user is not pending approval")
    }

    now := time.Now()
    user.Status = domain.UserStatusActive
    user.ApprovedBy = &adminID
    user.ApprovedAt = &now

    return s.userRepo.Update(user)
}

// RejectUser rejects a pending user
func (s *AdminService) RejectUser(ctx context.Context, userID, adminID uuid.UUID, reason string) error {
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return err
    }

    // Log rejection reason in audit log
    // Then delete or deactivate user
    return s.userRepo.Delete(userID)
}

// UpdateOrganizationSettings updates organization settings
func (s *AdminService) UpdateOrganizationSettings(
    ctx context.Context,
    orgID uuid.UUID,
    autoApproveSSO bool,
) error {
    org, err := s.orgRepo.GetByID(orgID)
    if err != nil {
        return err
    }

    org.AutoApproveSSO = autoApproveSSO
    return s.orgRepo.Update(org)
}
```

### Phase 3: API Endpoints

#### 3.1 New Admin Endpoints

**GET /api/v1/admin/users** - List all users in organization
```json
{
  "users": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "name": "John Doe",
      "role": "member",
      "status": "active",
      "provider": "google",
      "created_at": "2025-10-07T...",
      "approved_by": "admin-uuid",
      "approved_at": "2025-10-07T..."
    }
  ]
}
```

**GET /api/v1/admin/users/pending** - List pending users
**POST /api/v1/admin/users/:id/approve** - Approve user
**POST /api/v1/admin/users/:id/reject** - Reject user
**PUT /api/v1/admin/users/:id/role** - Update user role
**DELETE /api/v1/admin/users/:id** - Deactivate user

**GET /api/v1/admin/organization/settings** - Get org settings
**PUT /api/v1/admin/organization/settings** - Update org settings
```json
{
  "auto_approve_sso": true
}
```

### Phase 4: Frontend Updates

#### 4.1 Admin Users Page Enhancement

**apps/web/app/dashboard/admin/users/page.tsx**:

```typescript
interface User {
  id: string;
  email: string;
  name: string;
  role: 'admin' | 'member' | 'viewer';
  status: 'pending' | 'active' | 'suspended' | 'deactivated';
  provider: string;
  created_at: string;
  approved_by?: string;
  approved_at?: string;
}

// Features:
// - Table showing ALL users (not just pending)
// - Filter by status (All, Pending, Active, Suspended)
// - Approve/Reject buttons for pending users
// - Role change dropdown for active users
// - Deactivate button for active users
// - Bulk actions (Approve All, Reject All)
```

#### 4.2 Organization Settings Page

**apps/web/app/dashboard/admin/organization/page.tsx** (new):

```typescript
// Settings page with:
// - Auto-Approve SSO toggle
// - If OFF, show warning: "New SSO users will require manual approval"
// - If ON, show info: "New SSO users are automatically granted access"
```

#### 4.3 Auth Flow Updates

**apps/web/middleware.ts**:
```typescript
// After successful OAuth callback:
// - If user.status === 'pending', redirect to /auth/pending-approval
// - If user.status === 'active', redirect to /dashboard
```

**apps/web/app/auth/pending-approval/page.tsx** (new):
```tsx
// Shows message:
// "Your account is pending admin approval. You'll receive an email once approved."
```

### Phase 5: Migration Path

#### 5.1 For Existing Installations
```sql
-- Set auto_approve_sso = TRUE for existing organizations
-- This maintains backward compatibility
UPDATE organizations SET auto_approve_sso = TRUE;

-- Set all existing users to 'active' status
UPDATE users SET status = 'active';
```

#### 5.2 For New Installations
- First user in organization: auto-approved, role=admin
- Subsequent users: follow organization's auto_approve_sso setting

## User Experience Flows

### Flow A: Auto-Approval Mode (Default)
```
1. User authenticates via SSO (Google/Microsoft/Okta)
2. System checks: organization.auto_approve_sso = TRUE
3. System creates user with status = 'active'
4. User redirected to /dashboard
5. Admin sees new user in /admin/users (already active)
```

### Flow B: Manual Approval Mode
```
1. User authenticates via SSO
2. System checks: organization.auto_approve_sso = FALSE
3. System creates user with status = 'pending'
4. User redirected to /auth/pending-approval
5. Admin receives notification (email/dashboard badge)
6. Admin navigates to /admin/users
7. Admin sees pending user with Approve/Reject buttons
8. Admin approves → user.status = 'active', email sent
9. User can now login and access /dashboard
```

### Flow C: Admin Configuration
```
1. Admin navigates to /dashboard/admin/organization
2. Admin toggles "Auto-Approve SSO Users"
3. System updates organization.auto_approve_sso
4. Setting applies to all future SSO logins
```

## Implementation Priority

### Must Have (This Session)
1. ✅ Database migration for organization.auto_approve_sso
2. ✅ Database migration for user.status
3. ✅ Update auth service to respect auto_approve_sso
4. ✅ Add admin API endpoints for user management
5. ✅ Update /admin/users page to show all users
6. ✅ Add approve/reject buttons for pending users

### Should Have (Soon)
1. Organization settings page
2. Pending approval page for users
3. Email notifications for approval/rejection
4. Bulk approval actions

### Nice to Have (Future)
1. Custom approval workflows
2. Role-based approval (different approvers for different roles)
3. Time-limited approvals
4. Approval history audit trail

## Testing Scenarios

### Test 1: Auto-Approval Mode
1. Create new organization
2. First user logs in with Google SSO → becomes admin, active
3. Second user logs in with Google SSO → becomes member, active
4. Both users visible in /admin/users

### Test 2: Manual Approval Mode
1. Admin disables auto_approve_sso
2. New user logs in with Okta SSO → status = pending
3. User sees "Pending approval" message
4. Admin sees user in /admin/users with "Pending" badge
5. Admin clicks Approve
6. User receives email notification
7. User logs in again → redirected to /dashboard

### Test 3: Mixed Mode Transition
1. Organization starts with auto_approve_sso = TRUE
2. 10 users already active
3. Admin disables auto_approve_sso
4. New user logs in → pending
5. Existing 10 users remain active

## Security Considerations

1. **First User Exception**: Always auto-approve first user (otherwise no one can approve)
2. **Admin-Only Actions**: Only admins can approve/reject/change roles
3. **Audit Trail**: Log all approval/rejection decisions
4. **Email Verification**: Optional layer on top of SSO
5. **Rate Limiting**: Prevent registration spam

## Database Impact

- **organizations table**: +2 columns
- **users table**: +3 columns, +1 index
- **Backward compatible**: Existing users unaffected
- **Performance**: Index on (organization_id, status) for fast pending queries

---

**Implementation Status**: ⏳ READY TO BUILD
**Estimated Time**: 2-3 hours
**Breaking Changes**: None (backward compatible)
