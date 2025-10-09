# ADR 005: API Authentication & Authorization

**Status**: ✅ Accepted
**Date**: 2025-10-06
**Decision Makers**: AIM Architecture Team, Security Team
**Stakeholders**: Backend Team, Frontend Team, Enterprise Customers

---

## Context

AIM must provide secure authentication and authorization for:
1. **Web Application Users**: Enterprise employees accessing the UI
2. **API Clients**: Programmatic access to AIM API
3. **Third-Party Integrations**: Webhook receivers, external systems
4. **MCP Servers**: Agent-to-agent communication

### Security Requirements

1. **Zero Password Storage**: No plaintext or hashed passwords
2. **Enterprise SSO**: Support Google, Microsoft, Okta
3. **API Key Management**: Secure programmatic access
4. **Role-Based Access Control (RBAC)**: Four permission levels
5. **JWT Token Security**: Short-lived access tokens, long-lived refresh tokens
6. **Audit Trail**: Log all authentication attempts

---

## Decision

We will implement a **Multi-Layer Authentication Strategy**:

### Layer 1: OAuth2/OIDC for Web Users
- **Providers**: Google, Microsoft, Okta
- **Flow**: Authorization Code Flow with PKCE
- **No Passwords**: Zero password storage
- **Auto-Provisioning**: Create users on first login

### Layer 2: JWT Tokens for Session Management
- **Access Tokens**: 24-hour expiry, stateless
- **Refresh Tokens**: 7-day expiry, stored in database
- **HTTP-Only Cookies**: Prevent XSS attacks
- **Token Rotation**: New tokens on refresh

### Layer 3: API Keys for Programmatic Access
- **SHA-256 Hashing**: Keys never stored in plaintext
- **Expiration Tracking**: Configurable expiry (30, 90, 365 days)
- **Usage Monitoring**: Track last used, request count
- **Revocation**: Immediate invalidation

### Layer 4: RBAC for Authorization
- **Admin**: Full system access
- **Manager**: Manage team agents
- **Member**: View and create agents
- **Viewer**: Read-only access

---

## Implementation

### 1. OAuth2 Flow (Web Authentication)

```
User → Frontend → Backend → OAuth Provider → Backend → Frontend
  │                 │                │            │          │
  ▼                 ▼                ▼            ▼          ▼
1. Click      → 2. Generate → 3. Redirect  → 4. User   → 5. Callback
   "Login        auth URL       to provider    approves    with code
   with Google"                                           ↓
                                            6. Exchange code for tokens
                                               ↓
                                            7. Fetch user info
                                               ↓
                                            8. Create/update user
                                               ↓
                                            9. Generate JWT
                                               ↓
                                            10. Set HTTP-only cookies
                                               ↓
                                            11. Redirect to dashboard
```

**Backend Implementation**:

```go
// internal/interfaces/http/handlers/auth_handler.go
package handlers

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/application"
    "github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

type AuthHandler struct {
    authService  *application.AuthService
    oauthService *auth.OAuthService
    jwtService   *auth.JWTService
}

// Login initiates OAuth flow
func (h *AuthHandler) Login(c fiber.Ctx) error {
    provider := c.Params("provider") // "google", "microsoft", "okta"

    // 1. Generate CSRF state token
    state := uuid.New().String()

    // 2. Get authorization URL
    authURL, err := h.oauthService.GetAuthURL(provider, state)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to generate auth URL",
        })
    }

    // 3. Store state in session (CSRF protection)
    c.Cookie(&fiber.Cookie{
        Name:     "oauth_state",
        Value:    state,
        HTTPOnly: true,
        Secure:   true,
        SameSite: "Lax",
        MaxAge:   600, // 10 minutes
    })

    return c.JSON(fiber.Map{
        "redirect_url": authURL,
    })
}

// Callback handles OAuth callback
func (h *AuthHandler) Callback(c fiber.Ctx) error {
    code := c.Query("code")
    state := c.Query("state")
    provider := c.Params("provider")

    // 1. Verify CSRF state
    savedState := c.Cookies("oauth_state")
    if state != savedState {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid state parameter",
        })
    }

    // 2. Exchange code for access token
    accessToken, err := h.oauthService.ExchangeCode(c.Context(), provider, code)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Failed to exchange code",
        })
    }

    // 3. Get user info from provider
    oauthUser, err := h.oauthService.GetUserInfo(c.Context(), provider, accessToken)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Failed to get user info",
        })
    }

    // 4. Create or update user in database
    user, err := h.authService.LoginWithOAuth(c.Context(), oauthUser)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to process login",
        })
    }

    // 5. Generate JWT tokens
    accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
        user.ID.String(),
        user.OrganizationID.String(),
        user.Email,
        string(user.Role),
    )
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to generate tokens",
        })
    }

    // 6. Set HTTP-only cookies
    c.Cookie(&fiber.Cookie{
        Name:     "access_token",
        Value:    accessToken,
        HTTPOnly: true,
        Secure:   true,
        SameSite: "Lax",
        MaxAge:   86400, // 24 hours
    })

    c.Cookie(&fiber.Cookie{
        Name:     "refresh_token",
        Value:    refreshToken,
        HTTPOnly: true,
        Secure:   true,
        SameSite: "Lax",
        MaxAge:   604800, // 7 days
    })

    // 7. Redirect to frontend dashboard
    return c.Redirect().To("http://localhost:3000/dashboard?auth=success")
}
```

### 2. JWT Token Management

**Token Structure**:

```go
// internal/infrastructure/auth/jwt_service.go
package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
    UserID         string `json:"user_id"`
    OrganizationID string `json:"organization_id"`
    Email          string `json:"email"`
    Role           string `json:"role"`
    jwt.RegisteredClaims
}

type JWTService struct {
    secret        []byte
    accessExpiry  time.Duration
    refreshExpiry time.Duration
}

func (s *JWTService) GenerateTokenPair(userID, orgID, email, role string) (string, string, error) {
    // Access Token (short-lived)
    accessClaims := &JWTClaims{
        UserID:         userID,
        OrganizationID: orgID,
        Email:          email,
        Role:           role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            NotBefore: jwt.NewNumericDate(time.Now()),
        },
    }

    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessString, err := accessToken.SignedString(s.secret)
    if err != nil {
        return "", "", err
    }

    // Refresh Token (long-lived)
    refreshClaims := &JWTClaims{
        UserID:         userID,
        OrganizationID: orgID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshString, err := refreshToken.SignedString(s.secret)
    if err != nil {
        return "", "", err
    }

    return accessString, refreshString, nil
}

func (s *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
        return s.secret, nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
        return claims, nil
    }

    return nil, jwt.ErrSignatureInvalid
}
```

### 3. Authentication Middleware

```go
// internal/interfaces/http/middleware/auth_middleware.go
package middleware

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

func AuthMiddleware(jwtService *auth.JWTService) fiber.Handler {
    return func(c fiber.Ctx) error {
        // 1. Get token from cookie
        tokenString := c.Cookies("access_token")
        if tokenString == "" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Missing authentication token",
            })
        }

        // 2. Validate JWT
        claims, err := jwtService.ValidateToken(tokenString)
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Invalid or expired token",
            })
        }

        // 3. Store user context
        userID, _ := uuid.Parse(claims.UserID)
        orgID, _ := uuid.Parse(claims.OrganizationID)

        c.Locals("user_id", userID)
        c.Locals("organization_id", orgID)
        c.Locals("email", claims.Email)
        c.Locals("role", claims.Role)

        return c.Next()
    }
}
```

### 4. API Key Authentication

**Database Schema**:

```sql
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    key_hash        TEXT NOT NULL UNIQUE, -- SHA-256 hash
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    last_used_at    TIMESTAMPTZ,
    usage_count     INTEGER DEFAULT 0,
    is_active       BOOLEAN DEFAULT true
);

CREATE INDEX idx_api_keys_org ON api_keys(organization_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
```

**API Key Generation**:

```go
// internal/application/apikey_service.go
package application

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
)

func (s *APIKeyService) GenerateKey(ctx context.Context, orgID, userID uuid.UUID, name string, expiryDays int) (string, error) {
    // 1. Generate random key (32 bytes = 256 bits)
    keyBytes := make([]byte, 32)
    if _, err := rand.Read(keyBytes); err != nil {
        return "", err
    }

    // 2. Encode to base64 for display
    apiKey := fmt.Sprintf("aim_%s", base64.URLEncoding.EncodeToString(keyBytes))

    // 3. Hash with SHA-256 for storage
    hash := sha256.Sum256([]byte(apiKey))
    keyHash := hex.EncodeToString(hash[:])

    // 4. Store in database
    expiresAt := time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour)

    dbKey := &entities.APIKey{
        OrganizationID: orgID,
        Name:           name,
        KeyHash:        keyHash,
        CreatedBy:      userID,
        ExpiresAt:      expiresAt,
    }

    if err := s.apiKeyRepo.Create(ctx, dbKey); err != nil {
        return "", err
    }

    // 5. Return plaintext key (shown only once)
    return apiKey, nil
}
```

**API Key Middleware**:

```go
// internal/interfaces/http/middleware/apikey_middleware.go
package middleware

import (
    "crypto/sha256"
    "encoding/hex"
    "strings"
    "github.com/gofiber/fiber/v3"
)

func APIKeyMiddleware(apiKeyService *application.APIKeyService) fiber.Handler {
    return func(c fiber.Ctx) error {
        // 1. Get API key from header
        authHeader := c.Get("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer aim_") {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Invalid API key format",
            })
        }

        apiKey := strings.TrimPrefix(authHeader, "Bearer ")

        // 2. Hash the key
        hash := sha256.Sum256([]byte(apiKey))
        keyHash := hex.EncodeToString(hash[:])

        // 3. Validate key
        key, err := apiKeyService.ValidateKey(c.Context(), keyHash)
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Invalid or expired API key",
            })
        }

        // 4. Store organization context
        c.Locals("organization_id", key.OrganizationID)
        c.Locals("api_key_id", key.ID)

        // 5. Update last_used_at and usage_count (async)
        go apiKeyService.UpdateUsage(context.Background(), key.ID)

        return c.Next()
    }
}
```

### 5. RBAC Authorization

**Permission Matrix**:

| Resource | Admin | Manager | Member | Viewer |
|----------|-------|---------|--------|--------|
| View Agents | ✅ | ✅ | ✅ | ✅ |
| Create Agent | ✅ | ✅ | ✅ | ❌ |
| Edit Agent | ✅ | ✅ | Own only | ❌ |
| Delete Agent | ✅ | ✅ | ❌ | ❌ |
| View API Keys | ✅ | ✅ | Own only | ❌ |
| Create API Key | ✅ | ✅ | ✅ | ❌ |
| Revoke API Key | ✅ | ✅ | Own only | ❌ |
| View Audit Logs | ✅ | ✅ | Own only | ❌ |
| Manage Users | ✅ | ✅ | ❌ | ❌ |
| View Dashboard | ✅ | ✅ | ✅ | ✅ |

**Authorization Middleware**:

```go
// internal/interfaces/http/middleware/rbac_middleware.go
package middleware

import (
    "github.com/gofiber/fiber/v3"
)

func RequireRole(allowedRoles ...string) fiber.Handler {
    return func(c fiber.Ctx) error {
        userRole := c.Locals("role").(string)

        for _, role := range allowedRoles {
            if userRole == role {
                return c.Next()
            }
        }

        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "Insufficient permissions",
        })
    }
}

// Usage in routes:
// app.Post("/agents", RequireRole("admin", "manager", "member"), agentHandler.CreateAgent)
// app.Delete("/agents/:id", RequireRole("admin", "manager"), agentHandler.DeleteAgent)
```

---

## Consequences

### Positive

1. **Enterprise-Ready**:
   - SSO integration meets enterprise requirements
   - No password management burden
   - Supports multiple identity providers

2. **Secure**:
   - OAuth 2.0 best practices
   - HTTP-only cookies prevent XSS
   - API keys hashed with SHA-256
   - Short-lived access tokens

3. **Flexible**:
   - Web UI authentication (OAuth)
   - Programmatic access (API keys)
   - Fine-grained permissions (RBAC)

4. **Auditable**:
   - All authentication attempts logged
   - API key usage tracked
   - Token expiry enforced

### Negative

1. **Complexity**:
   - Multiple authentication methods
   - OAuth flow requires external dependencies
   - Token refresh logic needed

2. **OAuth Dependency**:
   - Requires OAuth provider availability
   - Provider outages affect login

3. **Token Management**:
   - Refresh token rotation adds complexity
   - Token revocation requires database

### Mitigation

1. **Fallback Authentication**:
   - Support multiple OAuth providers
   - API keys work independently

2. **Token Caching**:
   - Cache valid tokens in Redis
   - Reduce database lookups

3. **Monitoring**:
   - Alert on OAuth failures
   - Track authentication success rates

---

## Security Checklist

- [x] HTTPS/TLS enforced in production
- [x] HTTP-only cookies (prevent XSS)
- [x] CSRF protection (state parameter in OAuth)
- [x] API keys hashed with SHA-256
- [x] Short-lived access tokens (24h)
- [x] Token rotation on refresh
- [x] Rate limiting on auth endpoints
- [x] Audit logging of all auth events
- [x] Password-less authentication (OAuth only)
- [x] RBAC enforcement on all endpoints

---

## References

- [OAuth 2.0 RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
- [OIDC Core Specification](https://openid.net/specs/openid-connect-core-1_0.html)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)

---

**Last Updated**: October 6, 2025
**Related ADRs**: ADR-001 (Technology Stack), ADR-003 (Multi-Tenancy)
