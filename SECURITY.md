# Security Policy

> **AIM 1.0.** Every Roadmap-to-1.0 gate criterion in [HARDENING.md](HARDENING.md) is met as of 2026-05-28. The capabilities described in this document reflect shipped behavior; items still tracked as "Partial" or "Roadmap" in the Enterprise Compliance section below are flagged inline so consumers can plan around the actual enforcement boundary. Semver is honored from 1.0 forward. Response times for security reports are in the reporting flow below.

## Reporting Security Vulnerabilities

The Agent Identity Management (AIM) team takes security seriously. We appreciate the security research community's efforts in responsibly disclosing vulnerabilities.

### Please Report Security Issues Responsibly

**DO NOT** open public GitHub issues for security vulnerabilities.

Instead, please report security vulnerabilities by emailing:

**info@opena2a.org**

### What to Include in Your Report

To help us assess and address the vulnerability quickly, please include:

- **Description**: A clear description of the vulnerability
- **Impact**: The potential impact if exploited
- **Reproduction Steps**: Detailed steps to reproduce the issue
- **Proof of Concept**: Code or screenshots demonstrating the vulnerability (if applicable)
- **Suggested Fix**: If you have ideas on how to fix it (optional)
- **Your Contact Information**: So we can follow up with questions

### What to Expect

1. **Acknowledgment**: We will acknowledge receipt of your report within 48 hours
2. **Assessment**: We will assess the vulnerability and determine its severity
3. **Updates**: We will keep you informed of our progress
4. **Fix**: We will work on a fix and coordinate disclosure timing with you
5. **Credit**: We will credit you in our security advisories (unless you prefer to remain anonymous)

### Disclosure Timeline

- **Day 0**: Vulnerability reported to info@opena2a.org
- **Day 1-2**: Acknowledgment sent to reporter
- **Day 3-7**: Assessment and severity determination
- **Day 7-30**: Development and testing of fix
- **Day 30-90**: Coordinated public disclosure after fix is deployed

We ask that you:
- Give us reasonable time to fix the vulnerability before public disclosure
- Make a good faith effort to avoid privacy violations, data destruction, and service disruption
- Do not exploit the vulnerability beyond what is necessary to demonstrate it

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Security Features

AIM includes the following security features:

### Authentication & Authorization
- **JWT-based Authentication**: Secure token-based authentication
- **Bcrypt Password Hashing**: Industry-standard password protection
- **Role-Based Access Control (RBAC)**: Granular permission management
- **OAuth/OIDC Support**: SSO integration

### Cryptographic Security
- **Ed25519 Key Pairs**: Modern elliptic curve cryptography for agent identity (RFC 8032)
- **AES-256-GCM**: Authenticated encryption for stored private keys (NIST SP 800-38D)
- **SHA-256 API Key Hashing**: Secure API key storage
- **bcrypt (cost=12)**: Password hashing per OWASP guidelines
- **TLS 1.2+**: Encrypted data in transit (TLS 1.3 recommended)

#### Cryptographic Standards Reference

| Component | Algorithm | Key Size | Standard |
|-----------|-----------|----------|----------|
| Agent Signatures | Ed25519 | 256-bit | RFC 8032 |
| JWT Signing | HMAC-SHA256 | 256-bit | RFC 7519 |
| Key Encryption | AES-256-GCM | 256-bit | NIST SP 800-38D |
| Password Hashing | bcrypt | cost=12 | OWASP |
| API Key Hashing | SHA-256 | 256-bit | FIPS 180-4 |

### Application Security
- **Input Validation**: Comprehensive request validation
- **SQL Injection Prevention**: Parameterized queries throughout
- **XSS Protection**: Content Security Policy and output encoding
- **CSRF Protection**: Token-based CSRF prevention
- **Rate Limiting**: API request throttling
- **Audit Logging**: Comprehensive security event logging

### Infrastructure Security
- **Environment Variables**: No hardcoded secrets
- **Docker Security**: Non-root containers, minimal base images
- **Database Encryption**: Encrypted connections required
- **Secret Management**: Secure credential handling

## Security Best Practices

### For Deployment

1. **Always use HTTPS** in production
2. **Keep dependencies updated** regularly
3. **Use strong passwords** for database and admin accounts
4. **Enable audit logging** for compliance
5. **Configure proper CORS** policies
6. **Use secrets management** solutions (not .env files in production)
7. **Regular security updates** - apply patches promptly

### For Development

1. **Never commit secrets** to version control
2. **Use .env.example** as template, never commit .env
3. **Run security scanners** before commits
4. **Review dependencies** for known vulnerabilities
5. **Follow least privilege** principle
6. **Validate all inputs** from users
7. **Test authentication** and authorization flows

## Security audits

AIM has not had a third-party security assessment. No SOC 2, HIPAA or GDPR assessment and no
external penetration test has been performed on this project. If one is performed, this section
will name the assessor and the date.

What runs automatically today is dependency and secret scanning, in
[.github/workflows/security.yml](.github/workflows/security.yml). Each run's result is on that
workflow's page under the repository's Actions tab.

Changes reach `main` through pull requests, and the checks and approvals recorded on each pull
request are the record of what ran for that change. Read the reviewer on a given pull request
rather than inferring one from this document.

## Enterprise Compliance

AIM ships at 1.0 with the gate criteria above closed. This section maps each compliance claim to the code that backs it today, and flags claims that are partial or aspirational so consumers can plan around the actual enforcement boundary. Items marked **Partial** or **Roadmap** are tracked openly in [HARDENING.md](HARDENING.md) and continue post-1.0.

### SOC 2 Type II Alignment

| Control Area | Status | Implementation today | Notes |
|---|---|---|---|
| CC6.1 Logical Access | Enforced | RBAC, JWT authentication, API key scoping | |
| CC6.3 Privileged Access | Partial | `RiskLevel` constants on capabilities (`apps/backend/internal/domain/capability.go` Low/Medium/High/Critical) gate approval requirements via `CapabilityRequestService` | The SDK registration path can still grant capabilities directly without routing through the approval workflow. See HARDENING.md "Capability lifecycle." |
| CC6.6 System Boundaries | Enforced | Network segmentation, CORS, rate limiting | |
| CC6.7 Data Classification | Partial | Credential encryption (Fernet + system keyring on the SDK side; AES-256-GCM at rest on the backend) | "Log sanitization" is per-event truncation of token IDs, not a tested PII redaction pipeline — treat as best-effort. |
| CC6.8 Data Retention | Partial | `ComplianceService.GetDataRetentionStatus` exposes the configured retention policy (`apps/backend/internal/application/compliance_service.go:458`) | No background job currently enforces the policy via row-level deletion; values are reported, not yet swept. |
| CC7.1 Configuration Management | Partial | Environment-variable-driven config | Several required secrets currently fall back to weak defaults when unset. Removing those defaults is HARDENING.md "Bootstrap secrets and configuration." |
| CC7.2 Change Management | Enforced | Git history; `audit_logs` table records every privileged action | |

### Frameworks beyond SOC 2

The product narrative also references ISO 27001, NIST SP 800-53, FedRAMP, EU AI Act, and the OWASP Agentic Top 10. The honest status of those mappings is:

| Framework / control | Status | What is true today |
|---|---|---|
| ISO 27001 A.9.2.3 (Privileged Access Management) | Partial | JIT capability grants exist (`apps/backend/internal/application/capability_request_service.go`, with `RiskLevel`-gated approval and TTLs). In **strict** enforcement mode, high-risk requests require admin approval. In **monitoring** mode (the default), the same requests auto-approve and emit an audit row — useful for visibility, not equivalent to PAM. The A.9.2.3 framing is accurate only when the org is in strict mode AND when the SDK registration path also routes through the approval workflow (HARDENING.md "Capability lifecycle"). |
| NIST SP 800-53 AC-6 (Least Privilege) | Partial | Two authorization paths exist with different scope handling. The legacy `VerifyCapability` matcher (`apps/backend/internal/application/agent_service.go:1369`) — which the SDK `verify_capability` call hits — does exact-match + wildcard-prefix on the capability type only; it does not read `capability_scope` JSONB or the `resource` argument. The FGA engine `Authorize` path (`apps/backend/internal/application/fga_engine.go:316`) does enforce attribute-level scoping at Step 2 via the separate `fga_policies` table, but also does not read `agent_capabilities.capability_scope`. Operators who set `capability_scope` on a grant should treat it as decorative until unification lands; the workaround is name-encoded granularity (`db:read:users.name`). Full reasoning in [docs/specs/capability-scope-v1.md](docs/specs/capability-scope-v1.md). Tracked in #130. |
| FedRAMP AC-2 / AU-9 (Account Management / Audit Protection) | Roadmap | The `audit_logs` table records actions and is access-controlled, but is not append-only at the database layer and is not cryptographically signed. AU-9's "protection of audit information" requirement is partially met by RBAC; it is not met by a tamper-evident signing scheme. We do not claim FedRAMP authorization. |
| EU AI Act Article 14 (Human Oversight) | Partial | Approval gates exist for capability requests, and in **strict** enforcement mode high-risk operations require human review before grant. The Article 14 framing is accurate only in that mode; in monitoring mode the human-oversight gate is observational, not blocking. |
| OWASP Agentic Top 10 | Roadmap | We are working with the OASB community on a per-scenario mapping document. The current AIM enforcement primitives (capability authorization, trust scoring, FGA, audit trail) cover the agentic-identity portion of the threat space; a published one-to-one OWASP-to-AIM mapping is not yet in this repo. |

If you are evaluating AIM against a specific certification, please read [HARDENING.md](HARDENING.md) alongside this section. We do not claim certification under any of the frameworks above today; we describe where AIM's primitives align and where they do not.

### GDPR Compliance

- **Data Minimization**: Only essential data collected
- **Right to Erasure**: Agent and credential deletion supported
- **Encryption**: All PII encrypted at rest and in transit
- **Audit Trail**: Complete logging of data access
- **No PII in Logs**: Token IDs and sensitive data truncated/hashed

### Security Logging (SOC/SIEM Integration)

AIM provides JSON Lines format security logs compatible with:
- Splunk
- ELK Stack
- Datadog
- Sumo Logic
- AWS CloudWatch

```python
# Enable security logging
from aim_sdk import configure_security_logging
configure_security_logging()
```

Environment variables:
- `AIM_SECURITY_LOG_FILE`: Path to security log file
- `AIM_SECURITY_LOG_LEVEL`: DEBUG, INFO, WARNING, ERROR, CRITICAL
- `AIM_SECURITY_LOG_STDOUT`: Include stdout logging (true/false)

## Known Security Considerations

### Multi-Tenancy
- Organizations are strictly isolated at the database level
- API keys are scoped to specific organizations
- Users cannot access resources outside their organization

### API Security
- All API endpoints require authentication
- Rate limiting prevents abuse
- Input validation prevents injection attacks
- Comprehensive audit logging for compliance

### Trust Scoring
- Trust scores use multiple factors to prevent gaming
- Historical data prevents sudden score manipulation
- ML models are trained on verified data

## Security Updates

Security updates are released as soon as fixes are available. Subscribe to:
- **GitHub Security Advisories**: For critical vulnerabilities
- **GitHub Releases**: For all security updates
- **Mailing List**: info@opena2a.org

## Vulnerability Disclosure Policy

We follow industry best practices for coordinated vulnerability disclosure:

1. **Private Disclosure**: Report to info@opena2a.org
2. **Assessment**: We evaluate and respond within 48 hours
3. **Fix Development**: We develop and test the fix
4. **Coordinated Release**: We coordinate public disclosure with reporter
5. **Public Advisory**: We publish security advisory after fix deployment

## Bug Bounty Program

We do not currently have a formal bug bounty program, but we:
- **Acknowledge** all valid security reports
- **Credit** researchers in security advisories
- **Fast-track** security fixes
- May consider **rewards** for critical vulnerabilities on a case-by-case basis

## Contact

- **Security Issues**: info@opena2a.org
- **General Security Questions**: Discuss in GitHub Discussions
- **Emergency Contact**: For critical vulnerabilities, mark email as URGENT

## Legal

We will not pursue legal action against researchers who:
- Follow this disclosure policy
- Act in good faith
- Do not violate privacy or destroy data
- Do not disrupt our services

Thank you for helping keep AIM and our users safe!
