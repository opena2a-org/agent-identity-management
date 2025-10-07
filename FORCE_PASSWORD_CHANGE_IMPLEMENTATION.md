# Force Password Change Implementation

## Overview
Implemented a complete forced password change system for AIM to ensure users created via bootstrap must change their default password on first login for security.

## Changes Made

### 1. Database Schema (`004_force_password_change` migration)
- Added `force_password_change` BOOLEAN column to `users` table
- Created index on the column for efficient queries
- Default value: `FALSE`
- Set to `TRUE` when admin is created via bootstrap

### 2. Domain Model Updates
**File**: `apps/backend/internal/domain/user.go`
- Added `ForcePasswordChange bool` field to User struct
- Field is exposed in JSON responses as `force_password_change`

### 3. Bootstrap Script Updates
**File**: `apps/backend/cmd/bootstrap/main.go`
- Modified user creation query to include `force_password_change` column
- Set `force_password_change = TRUE` for initial admin user
- Added comment explaining the security requirement

### 4. Backend Authentication Service
**File**: `apps/backend/internal/application/auth_service.go`
- Added `ChangePassword()` method with following features:
  - Verifies current password
  - Validates new password strength (12+ chars, uppercase, lowercase, digits, special chars)
  - Ensures new password differs from current
  - Hashes new password with bcrypt
  - Clears `force_password_change` flag upon successful change
  - Only available for local auth users (not OAuth)

### 5. Backend HTTP Handler
**File**: `apps/backend/internal/interfaces/http/handlers/auth_handler.go`
- Added `ChangePassword()` handler for POST `/api/v1/auth/change-password`
- Requires authentication (JWT middleware)
- Returns clear error messages for validation failures
- Updated `LocalLogin()` response to include `force_password_change` flag

### 6. Repository Updates
**File**: `apps/backend/internal/infrastructure/repository/user_repository.go`
- Updated `GetByEmail()` SELECT query to include `force_password_change`
- Updated Scan() to read the new field
- Ensures field is properly populated when fetching users

### 7. Route Registration
**File**: `apps/backend/cmd/server/main.go`
- Added route: `POST /api/v1/auth/change-password` (requires authentication)

### 8. Frontend Login Flow
**File**: `apps/web/app/login/page.tsx`
- After successful login, checks `user.force_password_change` flag
- Redirects to `/change-password` if flag is `true`
- Otherwise redirects to `/dashboard` as normal

### 9. Frontend Password Change Page
**File**: `apps/web/app/change-password/page.tsx` (NEW)
- Complete password change UI with:
  - Current password input
  - New password input with strength requirements
  - Password confirmation input
  - Show/hide password toggles
  - Real-time password requirement validation
  - Visual feedback for each requirement (checkmarks)
  - Error handling with clear messages
  - Success state with automatic redirect
  - Updates localStorage to clear `force_password_change` flag
  - Prevents access for users without the flag set

## Password Requirements
Users must create passwords with:
- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character (`!@#$%^&*()_+-=[]{}; ':"\\|,.<>/?`)

## Security Features
1. **Forced Password Change**: Users created via bootstrap MUST change password before accessing system
2. **Strong Password Policy**: Enforced both client and server-side
3. **Current Password Verification**: Users must prove they know current password
4. **Password Reuse Prevention**: New password must differ from current
5. **Automatic Flag Clearing**: `force_password_change` cleared after successful change
6. **Redirect Protection**: Users can't bypass password change page

## User Flow

### First Login (Bootstrap User)
1. User logs in with default password
2. Backend returns `force_password_change: true`
3. Frontend redirects to `/change-password`
4. User must change password before accessing dashboard
5. After successful change, redirected to dashboard

### Subsequent Logins
1. User logs in with new password
2. Backend returns `force_password_change: false`
3. Frontend redirects directly to dashboard

## Testing
Tested end-to-end:
1. ✅ Database migration applied successfully
2. ✅ Bootstrap creates admin with `force_password_change = true`
3. ✅ Local login endpoint returns correct flag
4. ✅ Password change endpoint validates and updates correctly
5. ✅ Frontend redirects to change password page
6. ✅ Password change UI shows requirements and validates input
7. ✅ Successful password change clears flag and redirects to dashboard

## API Endpoints

### POST /api/v1/auth/change-password
**Authentication**: Required (Bearer token)

**Request Body**:
```json
{
  "current_password": "AdminPassword123!",
  "new_password": "NewSecurePassword123!"
}
```

**Response (Success - 200)**:
```json
{
  "message": "Password changed successfully"
}
```

**Response (Error - 400)**:
```json
{
  "error": "Current password is incorrect"
}
```

**Possible Errors**:
- `"Current password is incorrect"`
- `"Password must be at least 12 characters long"`
- `"Password must contain at least one uppercase letter"`
- `"Password must contain at least one lowercase letter"`
- `"Password must contain at least one digit"`
- `"Password must contain at least one special character"`
- `"New password must be different from current password"`
- `"Password change not available for OAuth users"`

## Files Modified
1. `apps/backend/migrations/004_force_password_change.up.sql` (NEW)
2. `apps/backend/migrations/004_force_password_change.down.sql` (NEW)
3. `apps/backend/internal/domain/user.go`
4. `apps/backend/cmd/bootstrap/main.go`
5. `apps/backend/internal/application/auth_service.go`
6. `apps/backend/internal/interfaces/http/handlers/auth_handler.go`
7. `apps/backend/internal/infrastructure/repository/user_repository.go`
8. `apps/backend/cmd/server/main.go`
9. `apps/web/app/login/page.tsx`
10. `apps/web/app/change-password/page.tsx` (NEW)

## Production Deployment Notes
1. Run migration `004_force_password_change.up.sql` before deploying
2. Existing users will have `force_password_change = FALSE` (no impact)
3. New bootstrap installs will require password change
4. Consider setting `force_password_change = TRUE` for existing admin users if needed
5. Password policy is enforced server-side (client validation is UX enhancement only)

## Future Enhancements (Not Implemented)
- Password expiration policy (force change every N days)
- Password history (prevent reuse of last N passwords)
- Account lockout after failed password change attempts
- Email notification when password is changed
- Admin ability to force password reset for specific users
- Password complexity scoring/meter
