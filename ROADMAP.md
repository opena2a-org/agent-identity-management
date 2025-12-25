# 🗺️ AIM Development Roadmap

**Last Updated**: December 24, 2025

This document tracks future enhancements and features that are deferred from the current development cycle.

---

## ✅ Recently Completed

### Trust Score Policy Enforcement (December 2025)
- Automatic enforcement when trust scores fall below thresholds
- Score < 70%: Warning alert created (agent continues to operate)
- Score < 50%: Critical alert created AND agent automatically suspended
- Deduplication prevents duplicate alerts within 1 hour
- Integrated into all trust score update paths

### Java SDK (December 2025)
- Full Java SDK with Maven/Gradle support
- AspectJ `@SecureAction` annotations for declarative security
- Spring Boot integration
- OkHttp HTTP client with BouncyCastle Ed25519 cryptography
- Tags and metadata support
- Complete feature parity with Python SDK

### Agent Tags & Metadata (December 2025)
- Tags for categorization (auto-created if they don't exist)
- Custom metadata as key-value pairs
- Both Python and Java SDK support

### Supply Chain Analytics Enhancements (December 2025)
- ABOM (Agent Bill of Materials) tab
- Capability drift alerts with pagination
- Enhanced threat detection modal with tabs and smart recommendations

---

## 🛡️ Security Policy Enforcement (Post-MVP)

### Current Status (MVP)
✅ **Capability Violation Detection** - FULLY IMPLEMENTED
- Actively enforced on all agent API calls
- Creates security alerts for violations
- Blocks unauthorized actions in real-time
- Prevents EchoLeak-style attacks (CVE-2025-32711)

### Planned Security Policy Enforcers

---

### ~~Trust Score Policy Enforcement~~ ✅ IMPLEMENTED
**Priority**: High
**Status**: Implemented (December 2025)

Trust scores are now actively enforced with automatic policy evaluation:

**Enforcement Rules**:
- Score < 70%: Warning alert created (agent continues to operate)
- Score < 50%: Critical alert created AND agent automatically suspended

**Implementation**:
- Added `EvaluateTrustScoreOnUpdate()` method to `SecurityPolicyService`
- Called after every trust score update (recalculation, manual update, violation impact)
- Creates alerts with deduplication (prevents duplicate alerts within 1 hour)
- Automatically suspends agents with critical trust scores
- Integrated into `RecalculateTrustScore()`, `UpdateTrustScore()`, and capability violation handling

**Files Modified**:
- `apps/backend/internal/application/security_policy_service.go`
- `apps/backend/internal/application/agent_service.go`
- `apps/backend/cmd/server/main.go`

---

### Failed Authentication Monitoring ✅
**Priority**: High
**Status**: ✅ IMPLEMENTED
**Estimated Effort**: 4 hours

**What Was Implemented**:
- ✅ `auth_failures` table to track failed login attempts (email, IP, user agent, metadata)
- ✅ `auth_lockouts` table to track account lockouts with automatic unlock scheduling
- ✅ `AuthFailureRepository` with full CRUD + lockout management
- ✅ `LoginWithPasswordExtended()` in AuthService with lockout checking and failure recording
- ✅ `EvaluateAuthFailures()` in SecurityPolicyService for alert creation
- ✅ Automatic account lockout after 5 failed attempts within 15 minutes
- ✅ 30-minute lockout duration (auto-unlock)
- ✅ Warning alerts after 3 failures, high-severity alerts on lockout
- ✅ Admin unlock capability via `UnlockAccount()` method
- ✅ Duplicate alert prevention (cooldown periods)

**Policy Configuration** (default):
- `max_attempts`: 5 failed attempts before lockout
- `time_window`: 15 minutes for counting attempts
- `lockout_duration`: 30 minutes
- `alert_threshold`: 3 failures before warning alert

**Key Files**:
- `apps/backend/internal/domain/auth_failure.go`
- `apps/backend/internal/infrastructure/repository/auth_failure_repository.go`
- `apps/backend/migrations/062_add_auth_failures_monitoring.sql`
- `apps/backend/internal/application/auth_service.go` (updated)
- `apps/backend/internal/application/security_policy_service.go` (updated)

**Use Case**: Prevent brute force attacks and credential stuffing

---

### Unusual Activity Detection
**Priority**: Medium
**Status**: Planned (Phase 3)
**Estimated Effort**: 6 hours

**Current**: Agent API calls are logged but no anomaly detection
**Needed**: Real-time detection of unusual behavior patterns

**Policy to Enforce**:
- Unusual Activity Monitoring (api_rate_threshold: 1000, time_window: 1h, check_off_hours: true)

**Implementation**:
- Add middleware to track API call rates
- Implement baseline behavior profiling per agent
- Add `EvaluateUnusualActivity()` method
- Detect API rate spikes
- Detect off-hours access patterns
- Create alerts for anomalies
- Optional blocking of suspicious activity

**Use Case**: Detect compromised agents or malicious behavior

---

### Data Exfiltration Detection
**Priority**: Medium
**Status**: Planned (Phase 3)
**Estimated Effort**: 8 hours

**Current**: No tracking of data transfers or response sizes
**Needed**: Detection and prevention of large-scale data exfiltration

**Policy to Enforce**:
- Data Exfiltration Detection (data_threshold_mb: 100, time_window: 1h)

**Implementation**:
- Add response size tracking middleware
- Track cumulative data transfer per agent
- Add `EvaluateDataExfiltration()` method
- Detect large data transfers
- Detect unusual download patterns
- Create alerts for suspicious transfers
- Optional blocking of exfiltration attempts
- Log destination IPs/domains

**Use Case**: Prevent data breaches and insider threats

---

### Phase Timeline

**Phase 1** (Post-MVP, Week 1):
- Trust Score Policy Enforcement
- Documentation and testing

**Phase 2** (Post-MVP, Week 2):
- Failed Authentication Monitoring
- Integration testing

**Phase 3** (Post-MVP, Weeks 3-4):
- Unusual Activity Detection
- Data Exfiltration Detection
- End-to-end security testing
- Performance optimization

**Total Estimated Effort**: 20 hours across 4 weeks

---

## 📦 Deployment & Infrastructure

### Docker Compose for Production
**Priority**: Medium
**Status**: Deferred

Create a complete `docker-compose.yml` for single-command deployment:
- PostgreSQL database with persistent volumes
- Redis cache
- Backend service
- Frontend service
- Auto-initialization on first run
- Environment variable configuration
- Health checks and restart policies

**Use Case**: Local production-like deployments and testing

---

### GitHub Actions CI/CD Workflow
**Priority**: Medium
**Status**: Deferred

Automate Docker image builds and deployments:
- Build backend and frontend images on push to main
- Push images to Azure Container Registry
- Run tests before building
- Automated deployment to Azure Container Apps
- Multi-stage builds for optimization
- Security scanning with Trivy

**Use Case**: Automated deployments on git push

---

### One-Command Deployment Testing
**Priority**: Medium
**Status**: Deferred

End-to-end testing of simplified deployment:
- Test `docker compose up` deployment
- Verify auto-initialization works
- Validate all services start correctly
- Test database migrations apply automatically
- Verify admin user creation
- Check default security policies seeded

**Use Case**: Ensuring deployment reliability

---

## 🐛 SDK Security & Stability Fixes

The following issues were identified during a comprehensive code review (December 2025) and are tracked for resolution.

### Python SDK Issues

#### ~~Memory Leak in LangChain Callback Handler~~ ✅ FIXED
**Priority**: Medium
**Status**: Fixed (December 2025)
**File**: `sdk/python/aim_sdk/integrations/langchain/callback.py`

**Issue**: The `_active_tools` dictionary stores tool invocation data keyed by `run_id`. If `on_tool_end` or `on_tool_error` is never called for a tool (e.g., due to exception or unexpected flow), entries accumulate and are never cleaned up.

**Fix**: Added TTL-based cleanup mechanism with 1-hour expiry and max 1000 entries limit.

---

#### Silent Fallback to Unencrypted Storage
**Priority**: Medium
**Status**: Identified
**File**: `sdk/python/aim_sdk/secure_storage.py`

**Issue**: When secure storage initialization fails (e.g., keyring unavailable), the code silently falls back to storing credentials in plaintext without warning the user.

**Recommendation**: Make fallback behavior configurable and emit a prominent warning when using unencrypted storage.

---

#### JWT Token Decoding Without Signature Verification
**Priority**: High
**Status**: Identified
**File**: `sdk/python/aim_sdk/oauth.py`

**Issue**: JWT tokens are decoded by splitting and base64-decoding the payload directly without cryptographic signature verification.

**Recommendation**: Use a proper JWT library (like `PyJWT`) with signature verification before trusting claims.

---

### Java SDK Issues

#### ~~HTTP Connection Pool Not Properly Closed~~ ✅ FIXED
**Priority**: Medium
**Status**: Already Fixed
**File**: `sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java`

**Issue**: The `close()` method shuts down the executor service but does not evict connections from the OkHttpClient connection pool.

**Fix**: Already fixed - `evictAll()` is called for both httpClient and authClient connection pools.

---

#### ~~Race Condition in Key Rotation~~ ✅ FIXED
**Priority**: High
**Status**: Fixed (December 2025)
**File**: `sdk/java/src/main/java/org/opena2a/aim/security/SecureCredentialStorage.java`

**Issue**: The `rotateKey()` method is not synchronized. Concurrent calls could corrupt credentials or the key file.

**Fix**: Added `synchronized` keyword to `rotateKey()` method to prevent concurrent execution.

---

#### Master Key Security Concerns
**Priority**: High
**Status**: Identified
**File**: `sdk/java/src/main/java/org/opena2a/aim/security/SecureCredentialStorage.java`
**Lines**: 132-153

**Issue**:
1. Master encryption key stored in plain file (`~/.aim/secure/.keystore`)
2. Key is never zeroed from memory after use
3. No integrity check on the key file

**Recommendation**:
- Zero out key material after use with `Arrays.fill()`
- Consider OS keychain integration (macOS Keychain, Windows Credential Manager)
- Add HMAC integrity check for key file

---

#### ~~No Retry Logic in Client Credentials OAuth Flow~~ ✅ FIXED
**Priority**: Medium
**Status**: Already Fixed
**File**: `sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java`

**Issue**: The `authenticateWithClientCredentials()` method has no retry logic for transient network failures.

**Fix**: Already fixed - method uses `httpClient` which has retry interceptor with exponential backoff.

---

#### ~~Missing Agent Name Length Validation~~ ✅ FIXED
**Priority**: Low
**Status**: Fixed (December 2025)
**File**: `sdk/java/src/main/java/org/opena2a/aim/credentials/CredentialManager.java`

**Issue**: Agent names are sanitized for filesystem safety but there's no maximum length validation, which could exceed filesystem path limits.

**Fix**: Added 200-character maximum length check with clear error message.

---

## 🔐 Security Enhancements

### Advanced RBAC System
**Priority**: High
**Status**: Planned

Implement fine-grained role-based access control:
- Custom role definitions
- Permission-based access control
- Role inheritance
- Organization-level and resource-level permissions
- Audit trail for role changes

**Use Case**: Organizations with complex permission requirements

---

### Multi-Factor Authentication (MFA)
**Priority**: High
**Status**: Planned

Add MFA support for enhanced security:
- TOTP (Time-based One-Time Password)
- SMS-based verification
- Backup codes
- Recovery mechanisms
- Enforced MFA for admin accounts

**Use Case**: Compliance requirements (SOC 2, HIPAA)

---

### API Rate Limiting
**Priority**: Medium
**Status**: Planned

Implement rate limiting for API endpoints:
- Per-user rate limits
- Per-organization rate limits
- Configurable limits in settings
- Rate limit headers in responses
- Redis-based distributed rate limiting

**Use Case**: Prevent abuse and ensure fair usage

---

## 📊 Features & Enhancements

### Advanced Analytics Dashboard
**Priority**: Medium
**Status**: Planned

Enhanced analytics and insights:
- Trust score trends over time
- Agent usage patterns
- Security incident heatmaps
- Compliance reporting
- Exportable reports (PDF, CSV)

**Use Case**: Security teams and compliance auditors

---

### Webhook Integration System
**Priority**: Medium
**Status**: Planned

Allow external systems to receive AIM events:
- Configurable webhook endpoints
- Event filtering
- Retry logic for failed deliveries
- Webhook signature verification
- Event replay capabilities

**Use Case**: Integration with SIEM, Slack, PagerDuty, etc.

---

### CLI Tool for Automation
**Priority**: Low
**Status**: Planned

Command-line tool for AIM operations:
- Agent registration via CLI
- API key generation
- Bulk operations
- Configuration management
- Scripting support

**Use Case**: DevOps automation and CI/CD pipelines

---

### GraphQL API
**Priority**: Low
**Status**: Planned

Add GraphQL endpoint alongside REST API:
- Flexible querying
- Reduced over-fetching
- Real-time subscriptions
- Schema introspection
- GraphQL Playground

**Use Case**: Frontend flexibility and efficiency

---

## 🧪 Testing & Quality

### Integration Test Suite
**Priority**: High
**Status**: Planned

Comprehensive integration tests:
- API endpoint tests
- Database integration tests
- Authentication flow tests
- Authorization tests
- Error handling tests

**Use Case**: Regression prevention and quality assurance

---

### Load Testing Framework
**Priority**: Medium
**Status**: Planned

Performance testing infrastructure:
- k6 load testing scripts
- Stress testing scenarios
- Performance benchmarks
- Scalability testing
- Results visualization

**Use Case**: Ensuring performance at scale

---

### E2E Frontend Tests
**Priority**: Medium
**Status**: Planned

End-to-end UI testing:
- Playwright test suite
- Critical user journey tests
- Cross-browser testing
- Visual regression testing
- Automated screenshot comparisons

**Use Case**: Frontend quality assurance

---

## 🔌 Framework Integrations

### TypeScript/Node.js SDK
**Priority**: High
**Status**: Planned (Q1 2026)
**Estimated Effort**: 2 weeks

TypeScript SDK for Node.js applications:
- Full TypeScript support with type definitions
- async/await API design
- npm package distribution
- Express/Fastify middleware integration
- Feature parity with Python and Java SDKs

**Use Case**: Node.js applications, serverless functions, TypeScript projects

---

### GitHub Copilot Integration
**Priority**: Medium
**Status**: Planned (Q1 2026)
**Estimated Effort**: 2 weeks

Integrate AIM with GitHub Copilot for VS Code:
- Copilot agent capability detection
- Code suggestion security verification
- Trust-based code review workflows
- Audit trail for AI-generated code
- Integration with GitHub Copilot Business/Enterprise

**Implementation**:
- VS Code extension integration
- Copilot API hooks for code suggestions
- Security policy enforcement for generated code
- Dashboard visibility for Copilot usage patterns

**Use Case**: Organizations using GitHub Copilot who need visibility and control over AI-generated code

---

## 📖 Documentation

### API Documentation Portal
**Priority**: High
**Status**: Planned

Interactive API documentation:
- Swagger/OpenAPI spec
- Interactive API explorer
- Code examples in multiple languages
- Authentication guide
- Rate limiting documentation

**Use Case**: Developer onboarding and API adoption

---

### Video Tutorials
**Priority**: Low
**Status**: Planned

Video guides for common tasks:
- Getting started with AIM
- Registering your first agent
- Configuring security policies
- Integrating with SSO
- Troubleshooting common issues

**Use Case**: User education and onboarding

---

## 🚀 Deployment History

### Completed Deployments
- **December 22, 2025**: Java SDK and documentation update
  - Java SDK with Maven/Gradle support
  - Tags and metadata support in both SDKs
  - Supply Chain Analytics enhancements
  - Updated README with dual-language examples
  - Migrated to Next.js 16

- **October 20, 2025**: Auto-initialization feature deployed
  - Complete database schema
  - Default seed data
  - Automatic admin user creation
  - Super admin protection
  - Users page fixes
  - Organization settings fixes

---

## 📝 Notes

- Items in this roadmap are not prioritized in any particular order within their priority level
- Priorities may change based on user feedback and business needs
- Completed items will be moved to the "Completed Deployments" section
- New items can be added by creating a PR to update this file

---

**Questions or Suggestions?**
Open an issue on GitHub: https://github.com/opena2a-org/agent-identity-management/issues
