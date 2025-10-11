# AIM Security Best Practices

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Authentication & Authorization](#authentication--authorization)
3. [Cryptographic Verification](#cryptographic-verification)
4. [Data Protection](#data-protection)
5. [API Security](#api-security)
6. [Network Security](#network-security)
7. [Monitoring & Detection](#monitoring--detection)
8. [Incident Response](#incident-response)
9. [Compliance & Audit](#compliance--audit)
10. [Security Checklist](#security-checklist)

---

## Overview

AIM (Agent Identity Management) implements a **security-first architecture** designed to protect AI agent identities and prevent unauthorized access. This document outlines the security measures, best practices, and guidelines for deploying and maintaining a secure AIM installation.

### Security Principles

1. **Zero Trust Architecture**: Never trust, always verify
2. **Defense in Depth**: Multiple layers of security controls
3. **Least Privilege**: Users and agents get minimum necessary access
4. **Cryptographic Verification**: Ed25519 digital signatures for identity proof
5. **Comprehensive Audit**: Every action is logged immutably
6. **Proactive Monitoring**: Real-time threat detection and alerting

---

## Authentication & Authorization

### Multi-Layer Authentication

AIM implements a four-layer security model:

```
Layer 1: Network Security (Nginx rate limiting, firewall rules)
Layer 2: Authentication (JWT tokens, OAuth/OIDC, API keys)
Layer 3: Authorization (RBAC - Role-Based Access Control)
Layer 4: Resource-Level (Ownership checks, organization isolation)
```

### JWT Token Management

**Token Lifecycle**:
- Access tokens expire after 24 hours
- Refresh tokens expire after 7 days
- Automatic token rotation on refresh
- Token revocation support for compromised tokens

**Token Security**:
```typescript
// Frontend - Token storage in localStorage
localStorage.setItem('auth_token', token);

// Backend - Token validation
func validateJWT(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(jwtSecret), nil
    })

    if err != nil || !token.Valid {
        return nil, errors.New("invalid token")
    }

    claims := token.Claims.(*Claims)

    // Check expiration
    if claims.ExpiresAt.Time.Before(time.Now()) {
        return nil, errors.New("token expired")
    }

    return claims, nil
}
```

### OAuth 2.0 / OIDC Integration

Supported providers:
- **Google OAuth**: Enterprise SSO with G Suite
- **Microsoft Azure AD**: OIDC with multi-tenant support
- **Okta**: SAML 2.0 + OIDC for enterprises

**OAuth Flow**:
```
1. User clicks "Sign in with Google"
2. Redirect to provider authorization endpoint
3. User authenticates with provider
4. Provider redirects back with authorization code
5. Backend exchanges code for access token
6. Backend creates JWT token for user
7. Frontend stores JWT token
8. Subsequent requests use JWT token
```

### API Key Authentication

For programmatic access (SDK, CLI, scripts):

**API Key Properties**:
- SHA-256 hashed before storage (never stored in plaintext)
- Optional expiration dates
- Scoped permissions (read-only, read-write, admin)
- Usage tracking and rate limiting
- Revocable at any time

**API Key Generation**:
```go
// Generate secure random API key
func generateAPIKey() (string, error) {
    b := make([]byte, 32)
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}

// Hash API key before storage
func hashAPIKey(apiKey string) string {
    hash := sha256.Sum256([]byte(apiKey))
    return hex.EncodeToString(hash[:])
}
```

**Usage**:
```bash
# Using API key in requests
curl -H "X-API-Key: your-api-key-here" \
  http://localhost:8080/api/v1/agents
```

### Role-Based Access Control (RBAC)

**Role Hierarchy**:

| Role | Permissions | Use Case |
|------|------------|----------|
| **Admin** | Full system access, user management, all endpoints | System administrators |
| **Manager** | Monitoring, security, alerts, compliance reports | Security teams |
| **Member** | Agent/MCP management, API keys, SDK access | Developers |
| **Viewer** | Read-only access to agents and dashboards | Auditors, observers |

**Permission Matrix**:

| Action | Admin | Manager | Member | Viewer |
|--------|-------|---------|--------|--------|
| Create Agent | ✅ | ✅ | ✅ | ❌ |
| View Agent | ✅ | ✅ | ✅ | ✅ |
| Delete Agent | ✅ | ❌ | ✅ | ❌ |
| View Security Threats | ✅ | ✅ | ❌ | ❌ |
| Manage Users | ✅ | ❌ | ❌ | ❌ |
| View Audit Logs | ✅ | ✅ | ❌ | ❌ |
| Create API Keys | ✅ | ✅ | ✅ | ❌ |

**Implementation**:
```go
// Middleware for role-based authorization
func RequireRole(allowedRoles ...string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        userRole := c.Locals("user_role").(string)

        for _, role := range allowedRoles {
            if userRole == role {
                return c.Next()
            }
        }

        return c.Status(403).JSON(fiber.Map{
            "error": "insufficient permissions",
        })
    }
}

// Usage in routes
app.Delete("/api/v1/agents/:id",
    AuthMiddleware,
    RequireRole("admin", "member"),
    handlers.DeleteAgent)
```

---

## Cryptographic Verification

### Ed25519 Digital Signatures

AIM uses **Ed25519** for cryptographic identity verification:

**Why Ed25519?**
- 256-bit security level (equivalent to RSA 3072-bit)
- Fast signature generation and verification
- Small key sizes (32 bytes public, 64 bytes private)
- Immune to timing attacks
- Industry standard (used by SSH, TLS 1.3)

**Key Generation**:
```python
# Python SDK
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

# Generate new keypair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

# Serialize keys
private_bytes = private_key.private_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PrivateFormat.Raw,
    encryption_algorithm=serialization.NoEncryption()
)

public_bytes = public_key.public_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PublicFormat.Raw
)
```

**Key Storage**:
- **Private Key**: NEVER sent to server, stored in OS keyring (SDK)
- **Public Key**: Stored in database, shared openly
- **Keyring**: System keychain (macOS), Credential Manager (Windows), Secret Service (Linux)

### Challenge-Response Protocol

**Verification Flow**:

```
1. Client → Server: "I want to verify agent X"
2. Server → Client: Challenge (32 random bytes)
3. Client: Sign challenge with private key
4. Client → Server: Signature
5. Server: Verify signature with public key
6. Server: Update trust score if valid
```

**Implementation**:
```go
// Generate challenge
func generateChallenge() ([]byte, error) {
    challenge := make([]byte, 32)
    _, err := rand.Read(challenge)
    return challenge, err
}

// Verify signature
func verifySignature(publicKey, message, signature []byte) bool {
    return ed25519.Verify(publicKey, message, signature)
}
```

**Security Properties**:
- Challenge is random and unique (replay attack prevention)
- Signature proves possession of private key
- Public key verification confirms identity
- No private key ever transmitted
- Resistant to man-in-the-middle attacks

---

## Data Protection

### Data at Rest

**Database Encryption**:
- PostgreSQL encryption at rest enabled
- Encrypted filesystem (LUKS on Linux, FileVault on macOS)
- Encrypted database backups

**Sensitive Data Handling**:
```go
// API keys are ALWAYS hashed
type APIKey struct {
    ID        uuid.UUID
    KeyHash   string  // SHA-256 hash, never plaintext
    CreatedAt time.Time
}

// Never store private keys
// (They remain in OS keyring on SDK client side)
```

**Secrets Management**:
- Environment variables for configuration
- No hardcoded secrets in code
- Secrets rotation every 90 days
- Use secret management tools (HashiCorp Vault, AWS Secrets Manager)

### Data in Transit

**TLS/SSL Configuration**:
```nginx
# Nginx SSL configuration
ssl_protocols TLSv1.3 TLSv1.2;
ssl_ciphers 'ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
ssl_prefer_server_ciphers on;
ssl_session_cache shared:SSL:10m;
ssl_session_timeout 10m;

# HSTS header
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

**Certificate Management**:
- Use Let's Encrypt for automatic certificate renewal
- Certificate pinning for mobile SDKs
- Monitor certificate expiration (30-day alerts)

### Data Retention & Deletion

**Retention Policies**:
- Audit logs: 1 year
- Security threat logs: 2 years
- User data: Until account deletion
- Deleted data: Permanently erased after 30 days

**GDPR Compliance**:
- Right to access: Export user data in JSON format
- Right to erasure: Permanent deletion within 30 days
- Data portability: Export in machine-readable format

---

## API Security

### Rate Limiting

**Rate Limits by Endpoint**:

| Endpoint | Rate Limit | Burst |
|----------|-----------|-------|
| `/api/v1/auth/login` | 5 requests/minute | 10 |
| `/api/v1/agents` (POST) | 10 requests/minute | 20 |
| `/api/v1/agents` (GET) | 100 requests/minute | 200 |
| `/api/v1/audit-logs` | 50 requests/minute | 100 |

**Implementation (Redis)**:
```go
func rateLimitMiddleware(maxRequests int, window time.Duration) fiber.Handler {
    return func(c *fiber.Ctx) error {
        key := fmt.Sprintf("rate_limit:%s", c.IP())

        count, err := redisClient.Incr(c.Context(), key).Result()
        if err != nil {
            return err
        }

        if count == 1 {
            redisClient.Expire(c.Context(), key, window)
        }

        if count > int64(maxRequests) {
            return c.Status(429).JSON(fiber.Map{
                "error": "rate limit exceeded",
            })
        }

        return c.Next()
    }
}
```

### Input Validation

**Always validate and sanitize input**:

```go
// Example: Agent creation validation
type CreateAgentRequest struct {
    Name      string `json:"name" validate:"required,min=3,max=100"`
    AgentType string `json:"agentType" validate:"required,oneof=ai_agent custom_agent mcp_server"`
    PublicKey string `json:"publicKey" validate:"required,base64"`
}

func validateRequest(req interface{}) error {
    validate := validator.New()
    return validate.Struct(req)
}
```

**SQL Injection Prevention**:
```go
// ALWAYS use parameterized queries
// GOOD ✅
result := db.Exec("SELECT * FROM agents WHERE id = $1", agentID)

// BAD ❌ (vulnerable to SQL injection)
// query := fmt.Sprintf("SELECT * FROM agents WHERE id = '%s'", agentID)
// result := db.Exec(query)
```

### CORS Configuration

**Production CORS Settings**:
```go
app.Use(cors.New(cors.Config{
    AllowOrigins:     "https://yourdomain.com",
    AllowMethods:     "GET,POST,PUT,DELETE",
    AllowHeaders:     "Origin,Content-Type,Authorization",
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

### Security Headers

**Recommended HTTP headers**:
```go
app.Use(func(c *fiber.Ctx) error {
    c.Set("X-Content-Type-Options", "nosniff")
    c.Set("X-Frame-Options", "DENY")
    c.Set("X-XSS-Protection", "1; mode=block")
    c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
    c.Set("Content-Security-Policy", "default-src 'self'")
    return c.Next()
})
```

---

## Network Security

### Firewall Rules

**Recommended iptables rules**:
```bash
# Allow incoming SSH (port 22)
iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# Allow incoming HTTPS (port 443)
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# Allow incoming HTTP (port 80) - redirect to HTTPS
iptables -A INPUT -p tcp --dport 80 -j ACCEPT

# Block all other incoming traffic
iptables -A INPUT -j DROP
```

### DDoS Protection

**Nginx rate limiting**:
```nginx
# Limit connections per IP
limit_conn_zone $binary_remote_addr zone=conn_limit_per_ip:10m;
limit_conn conn_limit_per_ip 10;

# Limit requests per IP
limit_req_zone $binary_remote_addr zone=req_limit_per_ip:10m rate=5r/s;
limit_req zone=req_limit_per_ip burst=10 nodelay;
```

### Intrusion Detection

**Fail2Ban Configuration**:
```ini
[nginx-limit-req]
enabled = true
filter = nginx-limit-req
action = iptables-multiport[name=ReqLimit, port="http,https", protocol=tcp]
logpath = /var/log/nginx/error.log
findtime = 600
bantime = 7200
maxretry = 10
```

---

## Monitoring & Detection

### Real-Time Threat Detection

AIM automatically monitors and detects:

1. **Behavioral Anomalies**:
   - Unusual patterns in agent actions
   - Spike in API requests
   - Actions outside normal hours

2. **Trust Score Monitoring**:
   - Sudden drops in trust scores
   - Failed verification attempts
   - Anomalous score patterns

3. **Verification Failures**:
   - Invalid signatures
   - Expired challenges
   - Mismatched public keys

4. **Compliance Violations**:
   - Unauthorized access attempts
   - Role escalation attempts
   - Access to restricted resources

### Security Metrics

**Key Metrics to Monitor**:

| Metric | Threshold | Alert Level |
|--------|-----------|-------------|
| Failed login attempts | > 5 in 5 minutes | High |
| Trust score drop | > 20 points | Critical |
| Verification failures | > 3 consecutive | High |
| API rate limit hits | > 10 per minute | Medium |
| Unauthorized access attempts | > 1 | Critical |

**Prometheus Metrics**:
```go
var (
    failedLogins = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aim_failed_logins_total",
            Help: "Total number of failed login attempts",
        },
        []string{"ip_address"},
    )

    trustScoreChanges = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "aim_trust_score_changes",
            Help: "Trust score changes over time",
        },
        []string{"agent_id"},
    )
)
```

### Alerting

**Alert Channels**:
- Email notifications
- Slack webhooks
- PagerDuty integration
- SMS alerts (critical only)

**Alert Severities**:
- **Critical**: Immediate action required (security breach, system down)
- **High**: Urgent attention needed (trust score drop, multiple failures)
- **Medium**: Investigation recommended (rate limit hits, anomalies)
- **Low**: Informational (routine events)

---

## Incident Response

### Security Incident Workflow

```
1. Detection → Automated monitoring or manual report
2. Triage → Assess severity and impact
3. Containment → Block threat, revoke access
4. Investigation → Root cause analysis
5. Remediation → Fix vulnerability
6. Recovery → Restore normal operations
7. Post-Mortem → Document lessons learned
```

### Incident Response Playbooks

#### Playbook 1: Compromised Agent
```
1. Immediately revoke agent's access
2. Review audit logs for suspicious activity
3. Reset agent credentials (new keypair)
4. Verify all recent actions by agent
5. Notify organization admin
6. Document incident
```

#### Playbook 2: Suspicious Login Activity
```
1. Lock user account temporarily
2. Notify user via email
3. Require password reset
4. Review login history
5. Check for unauthorized API key creation
6. Re-enable account after verification
```

#### Playbook 3: Trust Score Drop
```
1. Investigate recent agent activity
2. Check verification history
3. Review connected MCP servers
4. Analyze behavior patterns
5. Determine if legitimate or threat
6. Take appropriate action (monitor, suspend, revoke)
```

### Contact Information

**Security Team**:
- Email: security@yourdomain.com
- PagerDuty: 24/7 on-call rotation
- Slack: #security-incidents (internal)

---

## Compliance & Audit

### Audit Trail

**Every action is logged**:
- User authentication (login, logout, token refresh)
- Agent operations (create, update, delete, verify)
- MCP server operations (register, verify, update)
- API key operations (create, revoke, usage)
- Security events (threats, alerts, blocks)
- Configuration changes (user roles, settings)

**Audit Log Structure**:
```json
{
  "id": "uuid",
  "timestamp": "2025-10-10T14:30:00Z",
  "user_id": "uuid",
  "action": "create",
  "resource_type": "agent",
  "resource_id": "uuid",
  "source_ip": "192.168.1.100",
  "user_agent": "Mozilla/5.0...",
  "metadata": {
    "agent_name": "production-agent-1",
    "agent_type": "ai_agent"
  }
}
```

### Compliance Standards

**SOC 2 Type II**:
- ✅ Access controls (authentication, authorization, RBAC)
- ✅ Audit logging (comprehensive immutable trail)
- ✅ Encryption (at rest and in transit)
- ✅ Change management (version control, code reviews)
- ✅ Incident response (playbooks, alerts)

**HIPAA**:
- ✅ Access controls (unique user identification, emergency access)
- ✅ Audit controls (audit logs, access reviews)
- ✅ Integrity controls (authentication, data validation)
- ✅ Transmission security (TLS 1.3, encrypted backups)

**GDPR**:
- ✅ Right to access (data export endpoints)
- ✅ Right to erasure (permanent deletion)
- ✅ Data portability (JSON export)
- ✅ Privacy by design (encryption, access controls)
- ✅ Breach notification (automated alerts)

### Compliance Reporting

**Generate Compliance Reports**:
```bash
# Access review report
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/compliance/access-review \
  -o access-review.pdf

# Audit log export
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/export?start=2025-01-01&end=2025-12-31" \
  -o audit-logs-2025.csv
```

---

## Security Checklist

### Pre-Deployment Security Checklist

- [ ] **Authentication**
  - [ ] JWT secret is strong (256-bit random)
  - [ ] Token expiration configured (24h access, 7d refresh)
  - [ ] OAuth providers configured with correct credentials
  - [ ] API key hashing enabled

- [ ] **Authorization**
  - [ ] RBAC roles configured correctly
  - [ ] All endpoints have authentication middleware
  - [ ] Resource ownership checks implemented

- [ ] **Network Security**
  - [ ] TLS 1.3 enabled with strong cipher suites
  - [ ] HTTPS-only in production
  - [ ] Firewall rules configured
  - [ ] DDoS protection enabled

- [ ] **Data Protection**
  - [ ] Database encryption at rest enabled
  - [ ] Sensitive data not logged
  - [ ] Backup encryption configured
  - [ ] Private keys never stored server-side

- [ ] **Monitoring**
  - [ ] Prometheus metrics exposed
  - [ ] Alerting rules configured
  - [ ] Log aggregation enabled (ELK/Loki)
  - [ ] Security dashboard accessible

- [ ] **Compliance**
  - [ ] Audit logging enabled
  - [ ] Data retention policies configured
  - [ ] Privacy policy updated
  - [ ] Terms of service reviewed

### Monthly Security Tasks

- [ ] Review audit logs for anomalies
- [ ] Update dependencies (npm, Go modules)
- [ ] Rotate secrets (JWT secret, OAuth keys)
- [ ] Review user access (remove inactive users)
- [ ] Security scan (Trivy, Snyk, OWASP ZAP)
- [ ] Backup verification (restore test)

### Quarterly Security Tasks

- [ ] Penetration testing
- [ ] Compliance audit (SOC 2, HIPAA)
- [ ] Disaster recovery drill
- [ ] Review incident response playbooks
- [ ] Security training for team
- [ ] Third-party security assessment

---

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [CIS Controls](https://www.cisecurity.org/controls/)
- [Ed25519 Specification](https://ed25519.cr.yp.to/)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)

---

**Maintained by**: OpenA2A Security Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026

For security concerns or to report vulnerabilities, please contact: security@yourdomain.com
