# Backend Compilation Fixes Needed

## Summary
Based on analysis of handlers and services, here are all the compilation fixes needed to get the backend compiling.

## ✅ Already Fixed

1. **domain/audit_log.go** - Added missing audit action constants
   - AuditActionCreate, AuditActionUpdate, AuditActionDelete, AuditActionVerify
   - AuditActionView, AuditActionRevoke, AuditActionCalculate
   - AuditActionAcknowledge, AuditActionResolve
   - AuditActionGenerate, AuditActionExport, AuditActionCheck

2. **domain/alert.go** - Added alert severity constants
   - AlertSeverityInfo, AlertSeverityWarning, AlertSeverityCritical

3. **application/audit_service.go** - Added GetAuditLogs method
   - Takes filtering parameters
   - Returns ([]*domain.AuditLog, int, error)

4. **application/alert_service.go** - Fixed and updated
   - Removed duplicate GetAlerts/AcknowledgeAlert methods
   - Added GetAlerts(ctx, orgID, severity, status, limit, offset) ([]*domain.Alert, int, error)
   - Added AcknowledgeAlert(ctx, alertID, orgID, userID) error
   - Added ResolveAlert(ctx, alertID, orgID, userID, resolution) error
   - Fixed constructor to match main.go (removed apiKeyRepo parameter)

5. **application/auth_service.go** - Fixed constructor and added methods
   - Fixed NewAuthService to take only (userRepo, orgRepo)
   - Added LoginWithOAuth(ctx, oauthUser) (*domain.User, error)
   - Added GetUserByID(ctx, userID) (*domain.User, error)
   - Added GetUsersByOrganization(ctx, orgID) ([]*domain.User, error)
   - Added UpdateUserRole(ctx, userID, orgID, role, adminID) (*domain.User, error)
   - Added DeactivateUser(ctx, userID, orgID, adminID) error
   - Removed methods that used oauthService/jwtService fields

## 🔧 Fixes Still Needed

### 1. ComplianceService Constructor and Methods

**File**: `internal/application/compliance_service.go`

**Issues**:
- Constructor signature doesn't match main.go call
  - Current: NewCompliance Service(agentRepo, apiKeyRepo, auditRepo, alertRepo)
  - Expected in main.go: NewComplianceService(auditRepo, agentRepo, userRepo)

**Methods Missing**:
- `GenerateComplianceReport(ctx, orgID, reportType, startDate, endDate) (interface{}, error)`
- `GetComplianceStatus(ctx, orgID) (interface{}, error)`
- `GetComplianceMetrics(ctx, orgID, startDate, endDate, interval) (interface{}, error)`
- `ExportAuditLog(ctx, orgID, startDate, endDate, format) (string, error)`
- `GetAccessReview(ctx, orgID) (interface{}, error)`
- `GetDataRetentionStatus(ctx, orgID) (interface{}, error)`
- `RunComplianceCheck(ctx, orgID, checkType) (interface{}, error)`

### 2. TrustCalculator Methods

**File**: `internal/application/trust_calculator.go`

**Methods Potentially Missing**:
- `GetLatestTrustScore(ctx, agentID) (*domain.TrustScore, error)`
- `GetTrustScoreHistory(ctx, agentID, limit) ([]*domain.TrustScore, error)`

### 3. User Domain Model

**File**: `internal/domain/user.go`

**Check for**:
- `OrganizationName` field (used in AuthHandler.Me())
- May need to add this field or change handler to lookup organization separately

### 4. Alert Service Constructor

**File**: Already fixed but verify main.go matches
- Should be: `NewAlertService(alertRepo, agentRepo)`
- NOT: `NewAlertService(alertRepo, apiKeyRepo, agentRepo)`

### 5. Missing Fields in Alert Service

**File**: `internal/application/alert_service.go`

**Issue**:
- Struct still has `apiKeyRepo` field but constructor doesn't set it
- `CheckAPIKeyExpiry` method uses `s.apiKeyRepo` which will be nil
- Either remove CheckAPIKeyExpiry or fix the constructor

## 🎯 Strategy

1. Fix ComplianceService completely (constructor + all methods)
2. Verify TrustCalculator has all needed methods
3. Check User domain model
4. Remove apiKeyRepo from AlertService struct entirely
5. Try compilation
6. Fix any remaining import/type errors
7. Create stub implementations for any complex methods if needed

## 📋 After Compilation Fixes

Once backend compiles, we need to:
1. Create .env file
2. Start Docker Compose
3. Run migrations
4. Test server startup
5. Move to frontend fixes
6. Then integration testing

## Notes

- Many service methods can have simple stub implementations that return empty data
- The goal is to get it compiling and starting, not perfect functionality
- We can iterate on actual implementation during testing phase
- Focus on getting the types and interfaces correct first
