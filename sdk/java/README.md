# AIM Java SDK

Agent Identity Management SDK for Java. Provides cryptographic identity verification, capability-based access control, and security logging for AI agents.

## Quick Start

```java
import org.opena2a.aim.client.AIMClient;

// Register an agent with cryptographic identity
AIMClient agent = AIMClient.secure("my-agent");

// Ed25519 signatures, trust scoring, and audit trail are configured automatically
```

### With Capabilities

```java
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import java.util.Arrays;

AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call", "email:send"),
    AgentType.CUSTOM
);
```

## Installation

### Step 1: Download SDK from Dashboard

1. Log in to AIM at http://localhost:3000 (or your AIM instance)
2. Go to **Settings → SDK Download**
3. Click **"Download SDK"** → Includes pre-configured credentials

### Step 2: Add Maven Dependency

Add the AIM SDK to your `pom.xml`:

```xml
<dependency>
    <groupId>org.opena2a</groupId>
    <artifactId>aim-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

Or for Gradle:

```groovy
implementation 'org.opena2a:aim-sdk:1.0.0'
```

### Step 3: Configure Credentials

Place your downloaded credentials at `~/.aim/sdk_credentials.json`:

```json
{
    "aimUrl": "http://localhost:8080",
    "clientId": "your-client-id",
    "clientSecret": "your-client-secret",
    "organizationId": "your-org-id"
}
```

Or set environment variables:

```bash
export AIM_URL=http://localhost:8080
export AIM_CLIENT_ID=your-client-id
export AIM_CLIENT_SECRET=your-client-secret
export AIM_ORG_ID=your-org-id
```

## Features

| Feature | Description |
|---------|-------------|
| Cryptographic Identity | Ed25519 signatures on every action |
| Trust Scoring | Risk assessment for agent operations |
| Audit Trail | Structured security event logging |
| Action Verification | Capability-based access control |
| AspectJ Integration | Declarative security with `@SecureAction` annotations |

## Usage Examples

### Basic Agent Registration

```java
import org.opena2a.aim.client.AIMClient;

// Register agent with auto-detected credentials
AIMClient agent = AIMClient.secure("my-agent");

// The agent is now registered and ready to use
System.out.println("Agent registered: " + agent.getAgentName());
```

### Verify Capabilities Before Actions

```java
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.client.RiskLevel;

// Verify before performing sensitive actions
VerificationResult result = agent.verifyCapability("db:read", "users_table");

if (result.isVerified()) {
    // Safe to proceed
    User user = userRepository.findById(userId);
} else {
    // Handle denial
    System.err.println("Access denied: " + result.getStatus());
}
```

### Automatic Verification with performAction

```java
// Execute action with automatic verification
User user = agent.performAction("db:read", "users_table", () -> {
    return userRepository.findById(userId);
});

// With explicit risk level
PaymentResult payment = agent.performAction(
    "payment:process",
    "stripe_api",
    RiskLevel.HIGH,
    () -> paymentService.process(request)
);
```

### Using @SecureAction Annotation (AspectJ)

For declarative security, use the `@SecureAction` annotation with AspectJ:

```java
import org.opena2a.aim.annotations.SecureAction;
import org.opena2a.aim.client.RiskLevel;

public class UserService {

    @SecureAction(capability = "db:read", resource = "users_table")
    public User getUserById(String id) {
        return userRepository.findById(id);
    }

    @SecureAction(
        capability = "payment:process",
        resource = "stripe",
        riskLevel = RiskLevel.HIGH
    )
    public PaymentResult processPayment(PaymentRequest request) {
        return paymentService.process(request);
    }

    @SecureAction(
        capability = "db:delete",
        resource = "users_table",
        riskLevel = RiskLevel.CRITICAL,
        jitAccess = true,
        jitTimeout = 300
    )
    public void deleteUser(String userId) {
        // Requires admin approval before execution
        userRepository.deleteById(userId);
    }
}
```

**Setup for AspectJ:**

```java
import org.opena2a.aim.annotations.SecureActionAspect;

// Configure the aspect with your AIM client
SecureActionAspect.setClient(agent);
```

For Spring Boot applications, enable AspectJ auto-proxy:

```java
@Configuration
@EnableAspectJAutoProxy
public class SecurityConfig {

    @Bean
    public AIMClient aimClient() {
        return AIMClient.secure("my-spring-agent");
    }

    @PostConstruct
    public void configureAspect() {
        SecureActionAspect.setClient(aimClient());
    }
}
```

### MCP Server Integration

```java
import org.opena2a.aim.integrations.mcp.MCPIntegration;
import org.opena2a.aim.integrations.mcp.MCPServerInfo;
import org.opena2a.aim.integrations.mcp.AttestationResult;

// Register an MCP server
MCPServerInfo server = MCPIntegration.registerServer(
    agent,
    "filesystem-mcp",
    "http://localhost:3000",
    publicKeyBase64,
    Arrays.asList("read_file", "write_file", "list_directory")
);

System.out.println("MCP Server registered: " + server.getId());

// Attest to the server's capabilities
AttestationResult attestation = MCPIntegration.attestServer(
    agent,
    server.getId(),
    "http://localhost:3000",
    "filesystem-mcp",
    Arrays.asList("read_file", "write_file")
);

System.out.println("Attestation confidence: " + attestation.getConfidenceScore() + "%");

// Record tool usage for supply chain analytics
MCPIntegration.recordToolUsage(agent, server.getId(), "read_file");
```

### Request Additional Capabilities

```java
import java.util.Map;

// Request a new capability (requires admin approval)
Map<String, Object> result = agent.requestCapability(
    "db:delete",
    "Need to implement user account deletion feature"
);

String status = (String) result.get("status");
if ("pending".equals(status)) {
    System.out.println("Request submitted - awaiting admin approval");
} else if ("approved".equals(status)) {
    System.out.println("Capability granted!");
}
```

## Risk Levels

| Risk Level | Approval Required? | When to Use |
|------------|-------------------|-------------|
| `LOW` | No | Read operations, safe actions |
| `MEDIUM` | No | Write operations, data modification |
| `HIGH` | No (unless JIT) | Sensitive operations |
| `CRITICAL` | Yes with JIT | Destructive operations, requires approval |

## JIT (Just-In-Time) Access

For critical operations requiring human oversight:

```java
@SecureAction(
    capability = "payment:refund",
    resource = "stripe",
    riskLevel = RiskLevel.CRITICAL,
    jitAccess = true,
    jitTimeout = 300  // 5 minutes
)
public RefundResult processRefund(String orderId, BigDecimal amount) {
    // This method is BLOCKED until admin approves in the AIM dashboard
    return stripe.refund(orderId, amount);
}
```

With `jitAccess = true`:
- Pauses execution and creates approval request in AIM dashboard
- Notifies admin with action details
- Waits for admin decision (approve/reject)
- Times out after `jitTimeout` seconds if no decision

## Exception Handling

The SDK provides specific exceptions for different error conditions:

```java
import org.opena2a.aim.exceptions.*;

try {
    agent.performAction("db:delete", "users", () -> deleteAllUsers());
} catch (ActionDeniedException e) {
    System.err.println("Action denied: " + e.getCapability() + " on " + e.getResource());
} catch (VerificationException e) {
    System.err.println("Verification failed: " + e.getMessage());
} catch (AuthenticationException e) {
    System.err.println("Authentication failed: " + e.getMessage());
} catch (CredentialException e) {
    System.err.println("Credential error: " + e.getMessage());
} catch (AIMException e) {
    System.err.println("AIM error: " + e.getMessage());
}
```

## Builder Pattern

For more control, use the builder pattern:

```java
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;

AIMClient agent = new AIMClient.Builder()
    .agentName("my-enterprise-agent")
    .aimUrl("https://aim.mycompany.com")
    .clientId("client-id")
    .clientSecret("client-secret")
    .organizationId("org-id")
    .agentType(AgentType.LANGCHAIN)
    .addCapability("db:read")
    .addCapability("db:write")
    .addCapability("api:call")
    .build();
```

## Credential Management

Credentials are loaded in the following order:

1. **Environment Variables** (highest priority)
   - `AIM_URL`
   - `AIM_CLIENT_ID`
   - `AIM_CLIENT_SECRET`
   - `AIM_ORG_ID`

2. **Home Directory** - `~/.aim/sdk_credentials.json`

3. **Local Directory** - `.aim/credentials.json`

4. **Current Directory** - `sdk_credentials.json`

### Programmatic Credential Management

```java
import org.opena2a.aim.credentials.CredentialManager;
import java.util.Map;
import java.nio.file.Path;

// Load credentials
Map<String, String> creds = CredentialManager.loadSdkCredentials();

// Save credentials
Map<String, String> newCreds = Map.of(
    "aimUrl", "https://aim.mycompany.com",
    "clientId", "your-client-id",
    "clientSecret", "your-client-secret",
    "organizationId", "your-org-id"
);
CredentialManager.saveSdkCredentials(newCreds, Path.of("~/.aim/sdk_credentials.json"));

// Agent-specific credentials
Map<String, String> agentCreds = CredentialManager.loadAgentCredentials("my-agent");
CredentialManager.saveAgentCredentials("my-agent", agentCreds);
```

## SDK Structure

```
sdk/java/
├── src/
│   └── main/
│       └── java/
│           └── org/opena2a/aim/
│               ├── client/           # Core client classes
│               │   ├── AIMClient.java
│               │   ├── AgentType.java
│               │   ├── RiskLevel.java
│               │   └── VerificationResult.java
│               ├── annotations/      # AspectJ annotations
│               │   ├── SecureAction.java
│               │   └── SecureActionAspect.java
│               ├── credentials/      # Credential management
│               │   └── CredentialManager.java
│               ├── integrations/     # Framework integrations
│               │   └── mcp/          # MCP integration
│               │       ├── MCPIntegration.java
│               │       ├── MCPServerInfo.java
│               │       └── AttestationResult.java
│               └── exceptions/       # Custom exceptions
│                   ├── AIMException.java
│                   ├── ActionDeniedException.java
│                   ├── AuthenticationException.java
│                   ├── CredentialException.java
│                   └── VerificationException.java
├── pom.xml               # Maven configuration
└── README.md             # This file
```

## Dependencies

The SDK uses the following libraries:

| Dependency | Purpose |
|------------|---------|
| OkHttp 4.12 | HTTP client |
| Jackson 2.16 | JSON processing |
| BouncyCastle 1.79 | Ed25519 cryptography |
| AspectJ 1.9.21 | AOP for @SecureAction |
| SLF4J 2.0 | Logging |

## Requirements

- Java 17 or higher
- Maven 3.6+ or Gradle 7+

## Versioning

The SDK follows [Semantic Versioning 2.0.0](https://semver.org/):

```
1.0.0
│ │ │
│ │ └─── PATCH: Bug fixes
│ └───── MINOR: New features (backward-compatible)
└─────── MAJOR: Breaking changes
```

**Current Version**: 1.0.0

## License

Apache License 2.0 (Apache-2.0) - See [LICENSE](../../LICENSE) for details
