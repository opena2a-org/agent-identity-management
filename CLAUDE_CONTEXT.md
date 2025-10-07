# Claude Context - Agent Identity Management (AIM)

**Last Updated**: October 6, 2025
**Project**: Agent Identity Management (AIM)
**Status**: Active Development

---

## 🚨 CRITICAL: Port Configuration Standard

**NEVER CHANGE THESE PORTS - THEY ARE FIXED**

### Development Environment Ports

```
Frontend (Next.js): http://localhost:3000
Backend (Go/Fiber):  http://localhost:8080
PostgreSQL:          localhost:5432
Redis:               localhost:6379
```

### Why This Matters

- **CORS Configuration**: Backend CORS is configured ONLY for `http://localhost:3000`
- **OAuth Redirects**: Google OAuth callback URLs are registered for port 3000
- **Token Storage**: Frontend expects to run on port 3000 for localStorage access
- **Consistency**: Changing ports breaks authentication, API calls, and OAuth flow

### If You Ever See Different Ports

❌ **WRONG**: `localhost:3001`, `localhost:3002`, `localhost:3003`
✅ **CORRECT**: `localhost:3000` (frontend), `localhost:8080` (backend)

**Action Required**: Stop everything and fix the port configuration immediately.

---

## 🔑 Authentication & Security Standards

### Token Storage

- **Key Name**: `aim_token` (stored in localStorage)
- **Never Use**: `aivf_token` (old project, deprecated)
- **Format**: JWT (JSON Web Token)
- **Location**: Browser localStorage, NOT cookies or sessionStorage

### Authentication Flow

1. User visits `/` → Landing page
2. Clicks "Sign In" → Redirects to `/login`
3. Clicks "Continue with Google" → Backend generates OAuth URL
4. User authorizes → Google redirects to `/auth/callback?code=...`
5. Callback exchanges code → Stores JWT as `aim_token`
6. Redirects to `/dashboard` → User authenticated

---

## 🚫 Project Naming Standards

### NEVER Use "AIVF" in New Code

**Background**: AIM was born from an older project called AIVF. All new code must use "AIM" branding.

❌ **Forbidden**:
- Variable names: `aivf_token`, `aivfConfig`, `AIVF_API_KEY`
- File names: `aivf-utils.ts`, `aivf.config.json`
- UI Text: "AIVF Dashboard"

✅ **Correct**:
- Variable names: `aim_token`, `aimConfig`, `AIM_API_KEY`
- File names: `aim-utils.ts`, `aim.config.json`
- UI Text: "AIM Dashboard", "Agent Identity Management"

---

**This document is the single source of truth for AIM development standards.**
