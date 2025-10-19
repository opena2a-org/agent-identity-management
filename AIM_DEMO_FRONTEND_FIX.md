# 🔧 AIM Demo Frontend Environment Variable Fix

**Date**: October 19, 2025, 4:16 AM UTC
**Status**: ✅ **FRONTEND CONFIGURATION FIXED** - Backend connectivity issue remains

---

## 🎯 Issue Identified

The AIM Demo frontend was unable to connect to the backend because the `NEXT_PUBLIC_API_URL` environment variable was not set correctly during the Docker build process.

### Root Cause
- **Problem**: Frontend was hardcoded to connect to `http://localhost:8000/api/agents`
- **Why**: Next.js requires build-time environment variables (NEXT_PUBLIC_*) to be available during `npm run build`
- **Impact**: Frontend deployed successfully but couldn't communicate with deployed backend

### Evidence
Browser network requests showed:
```
❌ http://localhost:8000/api/agents - ERR_CONNECTION_REFUSED
```

---

## ✅ Fix Applied

### 1. Updated Dockerfile with Build Arguments

**File**: `/Users/decimai/workspace/aim-test/demo-frontend/Dockerfile`

**Changes**:
```dockerfile
# Added build arguments for Next.js environment variables
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_AIM_PLATFORM_URL

# Set environment variables for build
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_AIM_PLATFORM_URL=${NEXT_PUBLIC_AIM_PLATFORM_URL}
```

### 2. Rebuilt with Correct URLs

**Build Command**:
```bash
az acr build \
  --registry aimdemoregistry \
  --image demo-frontend:latest \
  --file demo-frontend/Dockerfile \
  --platform linux/amd64 \
  --build-arg NEXT_PUBLIC_API_URL=https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io \
  --build-arg NEXT_PUBLIC_AIM_PLATFORM_URL=https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io \
  demo-frontend
```

**Build Result**:
- ✅ Build ID: chg
- ✅ Image: aimdemoregistry.azurecr.io/demo-frontend:latest
- ✅ Digest: sha256:b14242c4010a2fda6cd2d9b322711085198348697ca05da9b8f144a749c9dd58
- ✅ Static pages: 8/8 generated successfully

### 3. Tagged and Deployed New Version

To force Container Apps to pull the new image:
```bash
# Tagged as v2 to ensure new deployment
docker tag aimdemoregistry.azurecr.io/demo-frontend:latest aimdemoregistry.azurecr.io/demo-frontend:v2
docker push aimdemoregistry.azurecr.io/demo-frontend:v2

# Updated Container App to use v2 tag
az containerapp update \
  --name aim-demo-frontend-app \
  --resource-group aim-demo-rg \
  --image aimdemoregistry.azurecr.io/demo-frontend:v2
```

**Deployment Result**:
- ✅ New revision created: `aim-demo-frontend-app--0000001`
- ✅ Traffic: 100% to new revision
- ✅ Status: Running

---

## 🎉 Verification Results

### Frontend Configuration - ✅ FIXED
Browser network requests now show:
```
✅ https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/api/agents - PENDING
```

**Before Fix**: `http://localhost:8000/api/agents` (ERR_CONNECTION_REFUSED)
**After Fix**: Correct production backend URL being called

### Page Status
- ✅ Homepage: Loading correctly
- ✅ Navigation: Working
- ⏳ /agents page: Shows "Loading agents..." (making API call to correct URL)

---

## ⚠️ Remaining Issue: Backend Performance

### Current Observation
- **Frontend**: ✅ Correctly configured and calling backend
- **Backend**: ⏳ API requests taking very long time or timing out
- **Impact**: Agents page stuck on "Loading agents..."

### Next Steps Required
1. **Investigate backend performance**:
   - Check backend Container App status
   - Review backend logs for errors
   - Verify backend can access database/dependencies
   - Check if backend is actually running and healthy

2. **Test backend directly**:
   ```bash
   curl https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/api/agents
   ```

3. **Possible backend issues**:
   - Database connection timeout
   - Missing environment variables
   - Application startup failure
   - Resource constraints (CPU/memory)

---

## 📊 Current Deployment State

### Frontend Container App
```yaml
Name: aim-demo-frontend-app
Image: aimdemoregistry.azurecr.io/demo-frontend:v2
URL: https://aim-demo-frontend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/
Revision: aim-demo-frontend-app--0000001
Status: ✅ Running
Environment Variables:
  - NEXT_PUBLIC_API_URL: https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io
  - NEXT_PUBLIC_AIM_PLATFORM_URL: https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io
```

### Backend Container App
```yaml
Name: aim-demo-backend-app
Image: aimdemoregistry.azurecr.io/demo-backend:latest
URL: https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/
Status: ⚠️ Running but slow/unresponsive
Health: ❓ Unknown - requests timing out
```

---

## 🔑 Key Lessons Learned

### 1. Next.js Environment Variables
- **Build-time variables** (NEXT_PUBLIC_*) must be available during `npm run build`
- Cannot be injected at runtime for static-generated pages
- Must use Docker `ARG` and `ENV` to pass values during build

### 2. Container Apps Image Updates
- Using `:latest` tag doesn't automatically pull new image
- Must either:
  - Use versioned tags (`:v1`, `:v2`, etc.)
  - Force restart/redeploy after pushing to `:latest`

### 3. Dockerfile Best Practices for Next.js
```dockerfile
# Declare build arguments
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_AIM_PLATFORM_URL

# Set as environment variables for build process
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_AIM_PLATFORM_URL=${NEXT_PUBLIC_AIM_PLATFORM_URL}

# Then run build
RUN npm run build
```

---

## 🎯 Summary

### What Was Fixed ✅
1. Frontend Dockerfile updated with build arguments
2. Frontend rebuilt with correct backend URLs baked into the bundle
3. New frontend image tagged and deployed (v2)
4. Frontend now correctly calls production backend API

### What Needs Attention ⚠️
1. Backend API performance/availability issue
2. Backend logs need to be checked for errors
3. Verify backend database connectivity
4. Test backend endpoints directly

### Success Criteria Met
- ✅ Frontend builds without TypeScript errors
- ✅ Frontend deployment successful
- ✅ Frontend calls correct backend URL
- ⏳ End-to-end functionality (blocked by backend issue)

---

## 📞 Testing URLs

### Frontend (Working)
- **Homepage**: https://aim-demo-frontend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/
- **Agents Page**: https://aim-demo-frontend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/agents
- **Chat Page**: https://aim-demo-frontend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/chat

### Backend (Needs Investigation)
- **Base URL**: https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/
- **Agents API**: https://aim-demo-backend-app.wittydesert-756d026f.eastus2.azurecontainerapps.io/api/agents

---

**Last Updated**: October 19, 2025, 4:16 AM UTC
**Fixed By**: Claude (AI Assistant)
**Method**: Docker build arguments + ACR rebuild + versioned deployment
