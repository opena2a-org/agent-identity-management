# AIM Java SDK

Cryptographic identity, capability authorization, and audit trails for Java AI agents. Apache 2.0.

[![Maven Central](https://img.shields.io/maven-central/v/org.opena2a/aim-sdk.svg)](https://search.maven.org/artifact/org.opena2a/aim-sdk)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)

Part of [Agent Identity Management (AIM)](../../README.md). Managed hosting at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started); self-host via the [main README](../../README.md#install-aim-self-hosted).

## Quick start

```java
import org.opena2a.aim.client.AIMClient;

AIMClient agent = AIMClient.secure("my-first-agent");

User user = agent.performAction("db:read", "users_table", () ->
    userRepository.findById(userId)
);
```

`secure()` generates an Ed25519 keypair, registers the agent, and stores credentials at `~/.aim/`. `performAction()` signs every invocation, runs it through 5-step Fine-Grained Authorization, and records the outcome in the audit log.

For declarative security, use `@SecureAction` with AspectJ:

```java
import org.opena2a.aim.annotations.SecureAction;
import org.opena2a.aim.client.RiskLevel;

public class UserService {
    @SecureAction(capability = "db:read", resource = "users_table")
    public User getUserById(String id) {
        return userRepository.findById(id);
    }

    @SecureAction(capability = "payment:process", riskLevel = RiskLevel.HIGH)
    public PaymentResult processPayment(PaymentRequest request) {
        return paymentService.process(request);
    }
}
```

## Installation

### Maven

```xml
<dependency>
    <groupId>org.opena2a</groupId>
    <artifactId>aim-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'org.opena2a:aim-sdk:1.0.0'
```

### Configure credentials

The SDK reads credentials from environment variables first, then `~/.aim/sdk_credentials.json`.

```bash
export AIM_URL=https://aim.opena2a.org
export AIM_CLIENT_ID=your-client-id
export AIM_CLIENT_SECRET=your-client-secret
export AIM_ORG_ID=your-org-id
```

Or place a credentials file at `~/.aim/sdk_credentials.json` (mode 0600):

```json
{
    "aimUrl": "https://aim.opena2a.org",
    "clientId": "your-client-id",
    "clientSecret": "your-client-secret",
    "organizationId": "your-org-id"
}
```

To download pre-configured credentials: log in to the AIM dashboard, go to **Settings → SDK Download**, copy the JSON into `~/.aim/sdk_credentials.json`.

## API reference

### Registration

```java
// Auto-detected agent type
AIMClient agent = AIMClient.secure("my-agent");

// With declared capabilities and explicit agent type
AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call", "email:send"),
    AgentType.CUSTOM
);
```

### Verifying capabilities

```java
// Pre-check without executing
VerificationResult result = agent.verifyCapability("db:read", "users_table");
if (result.isVerified()) {
    User user = userRepository.findById(userId);
} else {
    System.err.println("Denied: " + result.getStatus());
}

// Execute with automatic verification
User user = agent.performAction("db:read", "users_table", () ->
    userRepository.findById(userId)
);

// Explicit risk level
PaymentResult payment = agent.performAction(
    "payment:process", "stripe_api", RiskLevel.HIGH,
    () -> paymentService.process(request)
);
```

### `@SecureAction` annotation (AspectJ)

Wire the aspect once at startup:

```java
import org.opena2a.aim.annotations.SecureActionAspect;

AIMClient agent = AIMClient.secure("my-agent");
SecureActionAspect.setClient(agent);
```

Then annotate methods:

```java
@SecureAction(
    capability = "db:delete",
    resource = "users_table",
    riskLevel = RiskLevel.CRITICAL,
    jitAccess = true,
    jitTimeout = 300
)
public void deleteUser(String userId) {
    userRepository.deleteById(userId);
}
```

For Spring Boot:

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

### Risk levels

| Level | Approval | When to use |
|---|---|---|
| `LOW` | None | Read operations, safe actions |
| `MEDIUM` | None | Write operations, data modification |
| `HIGH` | None unless `jitAccess` | Sensitive operations |
| `CRITICAL` | `jitAccess = true` | Destructive operations |

### JIT access

`jitAccess = true` pauses execution and creates an approval request in the AIM dashboard. The method returns only after a human approves, or throws `JITAccessTimeoutException` after `jitTimeout` seconds.

### MCP server integration

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

// Attest server capabilities
AttestationResult attestation = MCPIntegration.attestServer(
    agent, server.getId(), "http://localhost:3000",
    "filesystem-mcp",
    Arrays.asList("read_file", "write_file")
);

// Record tool usage for supply-chain analytics
MCPIntegration.recordToolUsage(agent, server.getId(), "read_file");
```

### Request additional capabilities

```java
Map<String, Object> result = agent.requestCapability(
    "db:delete",
    "Need to implement user account deletion feature"
);

String status = (String) result.get("status");
if ("pending".equals(status)) {
    // Awaiting admin approval in the dashboard
}
```

### Builder pattern

For full control over configuration:

```java
AIMClient agent = new AIMClient.Builder()
    .agentName("my-enterprise-agent")
    .aimUrl("https://aim.mycompany.com")
    .clientId("client-id")
    .clientSecret("client-secret")
    .organizationId("org-id")
    .agentType(AgentType.LANGCHAIN)
    .addCapability("db:read")
    .addCapability("db:write")
    .build();
```

## Exception handling

```java
import org.opena2a.aim.exceptions.*;

try {
    agent.performAction("db:delete", "users", () -> deleteAllUsers());
} catch (ActionDeniedException e) {
    // Capability denied — capability + resource available via getters
} catch (VerificationException e) {
    // Signature or trust-score check failed
} catch (AuthenticationException e) {
    // Credentials missing or invalid
} catch (CredentialException e) {
    // Cannot load credentials from disk
} catch (AIMException e) {
    // Any other AIM error
}
```

## Credential management

The SDK loads credentials in this order:

1. Environment variables (`AIM_URL`, `AIM_CLIENT_ID`, `AIM_CLIENT_SECRET`, `AIM_ORG_ID`)
2. `~/.aim/sdk_credentials.json`
3. `./.aim/credentials.json` (project-local)
4. `./sdk_credentials.json` (current directory)

Programmatic access:

```java
import org.opena2a.aim.credentials.CredentialManager;

Map<String, String> creds = CredentialManager.loadSdkCredentials();
Map<String, String> agentCreds = CredentialManager.loadAgentCredentials("my-agent");
```

Credential files are written with mode 0600. Agent private keys are returned **once** at registration; the SDK saves them to `~/.aim/agents/<id>.json`.

## Dependencies

| Library | Purpose |
|---|---|
| OkHttp 4.12 | HTTP client |
| Jackson 2.16 | JSON processing |
| BouncyCastle 1.79 | Ed25519 cryptography |
| AspectJ 1.9.21 | AOP for `@SecureAction` |
| SLF4J 2.0 | Logging |

## Requirements

- Java 17 or higher
- Maven 3.6+ or Gradle 7+

## Versioning

[Semantic Versioning 2.0.0](https://semver.org/). Current: 1.0.0. SDK 1.x.x is compatible with backend 1.x.x.

## Related

- [Python SDK](../python/README.md) — same API shape, decorator-based
- [TypeScript SDK](../typescript/README.md) — local-or-server mode
- [opena2a CLI](https://github.com/opena2a-org/opena2a) — codebase auditing, credential migration, runtime monitoring
- [AIM backend](../../README.md) — server, dashboard, deployment

## License

Apache-2.0. See [LICENSE](../../LICENSE).
