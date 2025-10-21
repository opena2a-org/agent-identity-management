# Agent Identity Management - Authentication Summary

**Last Updated**: October 19, 2025
**Authentication Method**: Email/Password Only
**OAuth Status**: ✅ Completely Removed

---

## Current Authentication System

AIM uses **email/password authentication** with an **admin approval workflow** for all new user registrations.

### User Registration Flow

1. **User Registration** (`POST /api/v1/public/register`)
   - User provides: email, firstName, lastName, password
   - Password hashed with bcrypt (cost 12)
   - Registration request created with status = "pending"
   - User receives confirmation message

2. **Admin Review** (Admin Dashboard)
   - Admin views pending registration requests
   - Admin approves or rejects request
   - User notified of decision (email notification if configured)

3. **User Login** (`POST /api/v1/public/login`)
   - **If pending**: Returns user info with `isApproved: false`
   - **If approved**: Returns JWT tokens + user info
   - **If rejected**: Returns error message

---

## Database Schema

### user_registration_requests
```sql
- id (UUID)
- email (VARCHAR, unique for pending status)
- first_name (VARCHAR)
- last_name (VARCHAR)
- password_hash (TEXT) -- bcrypt hashed
- organization_id (UUID, nullable)
- status (ENUM: pending, approved, rejected)
- requested_at (TIMESTAMPTZ)
- reviewed_at (TIMESTAMPTZ, nullable)
- reviewed_by (UUID, nullable)
- rejection_reason (TEXT, nullable)
```

### users
```sql
- id (UUID)
- organization_id (UUID)
- email (VARCHAR)
- name (VARCHAR)
- password_hash (VARCHAR) -- bcrypt hashed
- role (VARCHAR: admin, manager, member, viewer)
- status (user_status: active, deactivated)
- force_password_change (BOOLEAN)
- created_at, updated_at
```

---

## API Endpoints

### Public (No Auth Required)
- `POST /api/v1/public/register` - Create registration request
- `POST /api/v1/public/login` - Login with email/password
- `POST /api/v1/public/change-password` - Change password (for force_password_change)
- `GET /api/v1/public/register/:requestId/status` - Check registration status

### Admin (Auth Required)
- `GET /api/v1/admin/registrations` - List pending registrations
- `POST /api/v1/admin/registrations/:id/approve` - Approve registration
- `POST /api/v1/admin/registrations/:id/reject` - Reject registration

---

## Security Features

1. **Password Requirements**
   - Minimum 8 characters
   - Must contain: uppercase, lowercase, number, special character
   - Validated on both frontend and backend

2. **Bcrypt Hashing**
   - Cost factor: 12
   - One-way encryption
   - Salt automatically generated per password

3. **JWT Tokens**
   - Access token: 15 minutes expiration
   - Refresh token: 7 days expiration
   - Stored in HTTP-only cookies

4. **Admin Approval**
   - All registrations require admin approval
   - Prevents unauthorized access
   - Audit trail of who approved each user

5. **Account Deactivation**
   - Admins can deactivate users
   - Deactivated users cannot login
   - Soft delete with audit trail

---

## Default Admin Account

```
Email: admin@aim.test
Password: admin123 (CHANGE ON FIRST LOGIN)
Role: admin
Organization: Test Organization
```

**⚠️ IMPORTANT**: The default admin has `force_password_change: true`. You MUST change the password on first login via the `/api/v1/public/change-password` endpoint.

---

## OAuth Removal

OAuth infrastructure (Google, Microsoft, Okta) was **completely removed** on October 19, 2025.

### What Was Removed
- ✅ OAuth handler and routes
- ✅ OAuth service layer
- ✅ OAuth database tables (oauth_connections)
- ✅ OAuth columns from users and registration tables
- ✅ OAuth frontend buttons (login/register pages)
- ✅ OAuth callback pages
- ✅ SSO button component

### Migration
Database migration `041_remove_oauth_infrastructure` removes all OAuth-related schema and adds password_hash support.

**Archive**: All OAuth-related documentation moved to `docs/archive/`

---

## Testing

### Registration Test
```bash
curl -X POST http://localhost:8080/api/v1/public/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "firstName": "New",
    "lastName": "User",
    "password": "SecurePass123!"
  }'
```

### Login Test (Pending Approval)
```bash
curl -X POST http://localhost:8080/api/v1/public/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "SecurePass123!"
  }'

# Response: { success: true, isApproved: false, message: "..." }
```

### Login Test (Approved User)
```bash
curl -X POST http://localhost:8080/api/v1/public/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@aim.test",
    "password": "your-new-password"
  }'

# Response: { success: true, isApproved: true, accessToken: "...", refreshToken: "...", user: {...} }
```

---

## Environment Variables

```bash
# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=identity

# JWT
JWT_SECRET=your-secret-key-here
JWT_ACCESS_TOKEN_EXPIRATION=15m
JWT_REFRESH_TOKEN_EXPIRATION=7d

# Server
PORT=8080
FRONTEND_URL=http://localhost:3000

# Optional: Email Notifications
EMAIL_FROM_ADDRESS=noreply@aim.example.com
SENDGRID_API_KEY=your-sendgrid-key (if using SendGrid)
```

---

## Next Steps

1. **Change Default Admin Password**
   ```bash
   curl -X POST http://localhost:8080/api/v1/public/change-password \
     -H "Content-Type: application/json" \
     -d '{
       "email": "admin@aim.test",
       "oldPassword": "admin123",
       "newPassword": "YourNewSecurePassword123!"
     }'
   ```

2. **Configure Email Notifications**
   - Set `EMAIL_FROM_ADDRESS` environment variable
   - Configure SendGrid or SMTP settings
   - Users will receive approval/rejection notifications

3. **Test Complete Flow**
   - Register a new user
   - Login with pending account (should show "not approved")
   - Approve via admin dashboard
   - Login again (should receive tokens and redirect to dashboard)

---

## Documentation

- **API Documentation**: `docs/api/README.md`
- **Development Guide**: `CLAUDE_CONTEXT.md`
- **Architecture**: `PROJECT_OVERVIEW.md`
- **OAuth Archive**: `docs/archive/` (historical reference only)

---

**Authentication System**: Production Ready ✅
**OAuth**: Completely Removed ✅
**Security**: Enterprise Grade ✅
