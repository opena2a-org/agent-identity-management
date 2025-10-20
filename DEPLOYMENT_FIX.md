# Production Deployment Fix - API URL Auto-Detection

## Problem
Frontend was calling `http://localhost:8080` instead of production backend URL when deployed to Azure Container Apps, causing CORS errors and failed logins.

## Root Cause
Next.js bakes `process.env.NEXT_PUBLIC_API_URL` into the client-side JavaScript bundle **at build time**. Even though the code runs in the browser, the environment variable has the value from when the Docker image was built (localhost), not the actual deployment environment.

## Solution
Prioritize runtime hostname detection OVER baked-in environment variables.

### Implementation (apps/web/lib/api.ts)

```typescript
"use client";

const getApiUrl = (): string => {
  if (typeof window === 'undefined') {
    throw new Error('getApiUrl() MUST be called in browser context only');
  }

  // 1. Check for runtime config (future: injected by server)
  if ((window as any).__RUNTIME_CONFIG__?.apiUrl) {
    return (window as any).__RUNTIME_CONFIG__.apiUrl;
  }

  // 2. Auto-detect from hostname (PRIMARY method for environment-agnostic deployment)
  // ⚠️ IMPORTANT: Do this BEFORE checking process.env!
  const { protocol, hostname } = window.location;
  if (hostname.includes('aim-frontend')) {
    const backendHost = hostname.replace('aim-frontend', 'aim-backend');
    return `${protocol}//${backendHost}`;
  }

  // 3. Fallback to localhost (local development only)
  // Note: process.env.NEXT_PUBLIC_API_URL is baked at build time and can't be trusted
  return "http://localhost:8080";
};

// Lazy singleton with Proxy pattern
let _apiInstance: APIClient | null = null;

function getAPIClient(): APIClient {
  if (!_apiInstance) {
    console.log('[API] Creating APIClient instance for the first time');
    _apiInstance = new APIClient();
  }
  return _apiInstance;
}

// Export Proxy that creates real instance on first property access
export const api = new Proxy({} as APIClient, {
  get(target, prop) {
    const instance = getAPIClient();
    const value = (instance as any)[prop];
    if (typeof value === 'function') {
      return value.bind(instance);
    }
    return value;
  },
  set(target, prop, value) {
    const instance = getAPIClient();
    (instance as any)[prop] = value;
    return true;
  }
});
```

## Key Principles

1. **Never trust `process.env.NEXT_PUBLIC_API_URL` in production** - it's baked at build time
2. **Use `'use client'` directive** - ensures module is client-side only
3. **Lazy initialization with Proxy** - APIClient constructor only runs in browser
4. **Hostname pattern matching** - primary method for environment detection
5. **Build with `--no-cache`** - when deploying critical fixes to avoid Docker layer caching

## Docker Build Commands

```bash
# Login to ACR
az acr login --name aimdemoregistry

# Build and push (use --no-cache for critical fixes)
docker buildx build \
  --no-cache \
  --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.frontend \
  -t aimdemoregistry.azurecr.io/aim-frontend:latest \
  --push .

# Update Container App
az containerapp update \
  --name aim-frontend \
  --resource-group aim-demo-rg \
  --image aimdemoregistry.azurecr.io/aim-frontend:latest
```

## Verification

After deployment, check browser console for:
```
[API] Creating APIClient instance for the first time
[API] Auto-detected URL from hostname: https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io
```

Network requests should show:
```
https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io/api/v1/...
```

## Benefits

✅ **Environment-agnostic deployment** - Same Docker image works across Azure, AWS, GCP, self-hosted
✅ **No configuration needed** - Auto-detects backend from frontend hostname
✅ **Zero downtime** - No environment variables to configure
✅ **Developer-friendly** - Falls back to localhost for local development

## Commits
- `447f1e4` - Force client-side only API URL detection with 'use client' directive
- `a54fd08` - Use Proxy pattern for truly lazy APIClient initialization
- `49a2457` - Prioritize hostname detection over baked-in env vars (FINAL FIX)

## Date Fixed
October 20, 2025
