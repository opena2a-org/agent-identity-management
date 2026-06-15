# AIM Development Roadmap

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

### Unusual Activity Detection ✅
**Priority**: Medium
**Status**: ✅ IMPLEMENTED

**What Was Implemented**:
- ✅ `BehaviorAnalysisService` with per-agent baseline learning
- ✅ `AgentBehaviorBaseline` domain model with velocity/capability/resource tracking
- ✅ `behavior_baselines` and `behavioral_anomalies` tables (migration 055)
- ✅ Statistical anomaly detection using sigma thresholds (2σ to 5σ)
- ✅ `EvaluateUnusualActivity()` in SecurityPolicyService
- ✅ Velocity spike detection (calls per hour vs baseline)
- ✅ New capability/resource access detection
- ✅ Risk-weighted activity scoring
- ✅ Alert creation for detected anomalies
- ✅ Configurable blocking via policy enforcement

**Anomaly Types Detected**:
- `velocity_spike` - Sudden increase in activity rate
- `new_capability` - First-time use of a capability
- `new_resource` - Access to never-before-seen resource
- `pattern_break` - Unusual action sequence
- `risk_spike` - Sudden increase in risk-weighted score

**Key Files**:
- `apps/backend/internal/domain/behavior_baseline.go`
- `apps/backend/internal/application/behavior_analysis_service.go`
- `apps/backend/internal/infrastructure/repository/behavior_baseline_repository.go`
- `apps/backend/migrations/055_add_agent_behavior_baselines.sql`

**Use Case**: Detect compromised agents or malicious behavior

---

### Data Exfiltration Detection ✅
**Priority**: Medium
**Status**: ✅ IMPLEMENTED

**What Was Implemented**:
- ✅ `data_transfers` table for individual transfer records
- ✅ `data_transfer_aggregates` table for hourly rollups
- ✅ `DataTransferRepository` with cumulative size tracking
- ✅ Size-based detection in `EvaluateDataExfiltration()`
- ✅ Pattern-based detection (fetch_external_url, bulk_export, etc.)
- ✅ `AlertDataExfiltration` alert type
- ✅ Configurable thresholds (default: 100MB per hour)
- ✅ Destination IP/domain logging
- ✅ Policy-based enforcement (block/alert/allow)

**Configuration** (via policy rules):
- `data_threshold_mb`: Transfer size threshold (default: 100 MB)
- `time_window_mins`: Time window for threshold (default: 60 minutes)
- `patterns`: Suspicious action patterns to detect

**Key Files**:
- `apps/backend/internal/domain/data_transfer.go`
- `apps/backend/internal/infrastructure/repository/data_transfer_repository.go`
- `apps/backend/migrations/063_add_data_transfer_tracking.sql`
- `apps/backend/internal/application/security_policy_service.go` (updated)

**Use Case**: Prevent data breaches and insider threats

---

## Deployment & Infrastructure

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

## SDK Security & Stability Fixes

The following issues were identified during a comprehensive code review (December 2025) and are tracked for resolution.

### Python SDK Issues

#### ~~Memory Leak in LangChain Callback Handler~~ ✅ FIXED
**Priority**: Medium
**Status**: Fixed (December 2025)
**File**: `sdk/python/aim_sdk/integrations/langchain/callback.py`

**Issue**: The `_active_tools` dictionary stores tool invocation data keyed by `run_id`. If `on_tool_end` or `on_tool_error` is never called for a tool (e.g., due to exception or unexpected flow), entries accumulate and are never cleaned up.

**Fix**: Added TTL-based cleanup mechanism with 1-hour expiry and max 1000 entries limit.

---

#### ~~Silent Fallback to Unencrypted Storage~~ ✅ FIXED
**Priority**: Medium
**Status**: Fixed (December 2025)
**File**: `sdk/python/aim_sdk/oauth.py`

**Issue**: When secure storage initialization fails (e.g., keyring unavailable), the code silently falls back to storing credentials in plaintext without warning the user.

**Fix**:
- Added prominent `UserWarning` when falling back to plaintext storage
- Added `allow_plaintext_fallback` parameter to `OAuthTokenManager`:
  - `True` (default): Falls back with warning
  - `False`: Raises `RuntimeError` requiring secure storage

---

#### ~~JWT Token Decoding Without Signature Verification~~ ✅ FIXED
**Priority**: High
**Status**: Fixed (December 2025)
**File**: `sdk/python/aim_sdk/oauth.py`

**Issue**: JWT tokens are decoded by splitting and base64-decoding the payload directly without cryptographic signature verification.

**Fix**:
- Added PyJWT>=2.8.0 as a dependency
- Created `decode_jwt_claims()` helper function using PyJWT for proper JWT parsing
- PyJWT validates JWT structure (3 parts, valid base64, valid JSON payload)
- Added issuer validation check (warns on unexpected issuers)
- Replaced all manual base64 decoding with PyJWT

**Note**: Full signature verification is not performed locally because:
- Server uses HS256 (symmetric key) - sharing the secret would defeat the purpose
- Tokens are always verified server-side when used for API calls
- Local parsing is only for housekeeping (reading expiry, token ID)

---

### Java SDK Issues

#### ~~HTTP Connection Pool Not Properly Closed~~ ✅ FIXED
**Priority**: Medium
**Status**: Fixed
**File**: `sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java`

**Issue**: The `close()` method shuts down the executor service but does not evict connections from the OkHttpClient connection pool.

**Fix**: Fixed - `evictAll()` is called for both httpClient and authClient connection pools.

---

#### ~~Race Condition in Key Rotation~~ ✅ FIXED
**Priority**: High
**Status**: Fixed (December 2025)
**File**: `sdk/java/src/main/java/org/opena2a/aim/security/SecureCredentialStorage.java`

**Issue**: The `rotateKey()` method is not synchronized. Concurrent calls could corrupt credentials or the key file.

**Fix**: Added `synchronized` keyword to `rotateKey()` method to prevent concurrent execution.

---

#### ~~Master Key Security Concerns~~ ✅ PARTIALLY FIXED
**Priority**: High
**Status**: Partially Fixed (December 2025)
**File**: `sdk/java/src/main/java/org/opena2a/aim/security/SecureCredentialStorage.java`

**Issue**:
1. Master encryption key stored in plain file (`~/.aim/secure/.keystore`)
2. Key is never zeroed from memory after use
3. No integrity check on the key file

**Fixes Applied**:
- ✅ `deriveKey()` now zeros PBEKeySpec password and intermediate key bytes after use
- ✅ Added `clearMasterKey()` method to securely zero master key from memory
- ✅ Auto-registers JVM shutdown hook to clear key on exit
- ✅ `rotateKey()` already zeros old key after rotation

**Still TODO**:
- OS keychain integration (macOS Keychain, Windows Credential Manager)
- HMAC integrity check for key file

---

#### ~~No Retry Logic in Client Credentials OAuth Flow~~ ✅ FIXED
**Priority**: Medium
**Status**: Already Fixed
**File**: `sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java`

**Issue**: The `authenticateWithClientCredentials()` method has no retry logic for transient network failures.

**Fix**: Fixed - method uses `httpClient` which has retry interceptor with exponential backoff.

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

### ~~API Rate Limiting~~ ✅ IMPLEMENTED
**Priority**: Medium
**Status**: Implemented (December 2025)

**Features Implemented**:
- ✅ Per-user rate limits (authenticated users)
- ✅ Per-organization rate limits (via callback)
- ✅ Configurable via environment variables:
  - `RATE_LIMIT_DEFAULT_MAX` (default: 100)
  - `RATE_LIMIT_DEFAULT_WINDOW_SEC` (default: 60)
  - `RATE_LIMIT_STRICT_MAX` (default: 10)
  - `RATE_LIMIT_STRICT_WINDOW_SEC` (default: 60)
- ✅ Rate limit headers in responses:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `X-RateLimit-Reset`
  - `Retry-After`
- ✅ Redis-based distributed rate limiting
- ✅ Graceful fallback to in-memory when Redis unavailable
- ✅ Strict rate limiting for sensitive endpoints (login, registration)

**Files**:
- `apps/backend/internal/interfaces/http/middleware/redis_rate_limit.go`
- `apps/backend/internal/infrastructure/cache/redis.go`

**Use Case**: Prevent abuse and ensure fair usage

---

## Features & Enhancements

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

### Webhook Integration System ✅ IMPLEMENTED
**Priority**: Medium
**Status**: Complete

Allow external systems to receive AIM events:
- ✅ Configurable webhook endpoints with CRUD API
- ✅ Event filtering (16 event types: alerts, agents, trust scores, security, API keys)
- ✅ Retry logic with exponential backoff (configurable max retries, delays)
- ✅ Webhook signature verification (HMAC-SHA256)
- ✅ Event replay capabilities
- ✅ Delivery tracking and statistics
- ✅ Soft delete support
- ✅ Integration with alert system (alert.created, alert.acknowledged webhooks)

**Implementation Details**:
- Enhanced domain model: `apps/backend/internal/domain/webhook.go`
- Service with retry logic: `apps/backend/internal/application/webhook_service.go`
- Repository implementation: `apps/backend/internal/infrastructure/repository/webhook_repository.go`
- Migration: `apps/backend/migrations/064_enhance_webhooks_for_retry.sql`

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

## Testing & Quality

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

## Framework Integrations

### TypeScript/Node.js SDK
**Priority**: High
**Status**: Planned (Q2 2026)

TypeScript SDK for Node.js applications:
- Full TypeScript support with type definitions
- async/await API design
- npm package distribution
- Express/Fastify middleware integration
- Feature parity with Python and Java SDKs

**Use Case**: Node.js applications, serverless functions, TypeScript projects

---

### Local Credential Verification in the SDKs
**Priority**: High
**Status**: TypeScript shipped; Java planned

The agent trust credential is a signed credential designed to be verified
locally: a verifier checks the signature against the issuer's cached public key in
roughly a millisecond, with the issuing node never on the verification path and
revocation handled by an asynchronously-refreshed, short-lived cached list.
Reference verifiers exist for Go and Python. This item brings that same local
verification to the agent-side SDKs so that capability checks resolve from the
signed credential's own claims without a per-action call to a central service,
keeping verification fast and horizontally scalable. Network access is reserved
for credential resolution and periodic revocation refresh, not per-action
decisions.

- **TypeScript SDK — shipped.** `@opena2a/aim-sdk` exposes a `LocalVerifier` and
  `AIMClient.verifyActionLocally`, verifying ATX credentials offline against
  cached trust anchors and authorizing from signed capabilities. It consumes the
  shared `@opena2a/atx-verify` verifier (byte-for-byte interoperable with the Go
  and Python reference verifiers), so there is a single conformance-locked
  implementation rather than a per-SDK reimplementation. The existing remote
  verification call is retained as a fallback.
- **Java SDK — shipped.** `org.opena2a.aim.atx` provides `LocalAtxVerifier` and
  `AtxCanonicalizer`, a native port of the Go reference verifier (Ed25519 over the
  v1.0 pipe / v1.1 JCS canonical payload, expiry, revocation, issuer-trust, and
  key-to-issuer binding). Byte-for-byte interoperable with the TS/Go/Python
  verifiers — gated by the pinned jcs-vectors canonical hash and the shared
  conformance fixtures.

**Use Case**: High-throughput agents that must authorize actions without a central
verification round-trip; spec-compliant offline operation.

---

### GitHub Copilot Integration
**Priority**: Medium
**Status**: Planned (Q2 2026)

Integrate AIM with GitHub Copilot for VS Code:
- Copilot agent capability detection
- Trust-based code review workflows
- Audit trail for AI-generated code
- Integration with GitHub Copilot Business/Enterprise

**Implementation**:
- Copilot API hooks for code suggestions
- Security policy enforcement for generated code
- Dashboard visibility for Copilot usage patterns

**Use Case**: Organizations using GitHub Copilot who need visibility and control over AI-generated code

---

## Documentation

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

## Deployment History

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

## Notes

- Items in this roadmap are not prioritized in any particular order within their priority level
- Priorities may change based on user feedback and business needs
- Completed items will be moved to the "Completed Deployments" section
- New items can be added by creating a PR to update this file

- Some features may be removed from roadmap later

---

**Questions or Suggestions?**
Open an issue on GitHub: https://github.com/opena2a-org/agent-identity-management/issues
