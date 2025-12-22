# Java SDK Guide - Complete Reference

The complete guide to the AIM Java SDK.

## Installation

### Download Pre-Configured SDK

The AIM SDK comes with your embedded credentials - zero configuration required.

**Steps to Download**:
1. **Login** to AIM Dashboard (http://localhost:3000)
   - Email: `admin@opena2a.org`
   - Password: `AIM2025!Secure`
2. **Navigate** to Settings → SDK Download
3. **Click** "Download Java SDK" button
4. **Extract** the downloaded ZIP file to your project directory

### Add Maven Dependency

Add to your `pom.xml`:

```xml
<dependency>
    <groupId>org.opena2a</groupId>
    <artifactId>aim-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

Or for Gradle (`build.gradle`):

```groovy
implementation 'org.opena2a:aim-sdk:1.0.0'
```

### Requirements

- Java 17+
- Maven 3.6+ or Gradle 7+

### Dependencies

The SDK includes:
- **OkHttp 4.12** - HTTP client
- **Jackson 2.16** - JSON processing
- **BouncyCastle 1.77** - Ed25519 cryptography
- **AspectJ 1.9.21** - AOP for annotations
- **SLF4J 2.0** - Logging

---

## Quick Start (30 Seconds)

```java
import org.opena2a.aim.client.AIMClient;

// ONE LINE - Secure your agent!
AIMClient agent = AIMClient.secure("my-agent");

// That's it! Your agent is now secure.
```

---

## Core Functions

### `secure()` - The Main Entry Point

Register and secure an agent with one line.

```java
// Minimal - just the name
AIMClient agent = AIMClient.secure("my-agent");

// With capabilities
AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call")
);

// With agent type
AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call"),
    AgentType.LANGCHAIN
);

// With MCP servers
AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call"),
    AgentType.LANGCHAIN,
    Arrays.asList("filesystem", "github")  // talksTo
);

// Full configuration with tags and metadata
AIMClient agent = AIMClient.secure(
    "my-agent",                              // agentName
    Arrays.asList("db:read", "api:call"),    // capabilities
    AgentType.LANGCHAIN,                     // agentType
    Arrays.asList("filesystem"),             // talksTo
    "Customer support AI agent",             // description
    Arrays.asList("production", "gpt-4"),    // tags
    Map.of("model", "gpt-4", "team", "support")  // metadata
);
```

**Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `agentName` | String | Yes | Unique agent identifier |
| `capabilities` | List<String> | No | Capabilities to grant |
| `agentType` | AgentType | No | Agent type enum |
| `talksTo` | List<String> | No | MCP servers the agent uses |
| `description` | String | No | Human-readable description |
| `tags` | List<String> | No | Tags for categorization |
| `metadata` | Map<String, Object> | No | Custom key-value metadata |

---

## Supported Agent Types

```java
import org.opena2a.aim.client.AgentType;

// Available types
AgentType.LANGCHAIN   // "langchain"
AgentType.CREWAI      // "crewai"
AgentType.AUTOGEN     // "autogen"
AgentType.OPENAI      // "gpt"
AgentType.ANTHROPIC   // "claude"
AgentType.CUSTOM      // "custom"
AgentType.UNKNOWN     // "unknown"
```

---

## AIMClient Methods

### Agent Properties

```java
// Access agent information
String agentId = agent.getAgentId();
String agentName = agent.getAgentName();
String aimUrl = agent.getAimUrl();
```

### Verify Capability

Verify a capability before executing an action.

```java
import org.opena2a.aim.client.VerificationResult;
import org.opena2a.aim.client.RiskLevel;

// Basic verification
VerificationResult result = agent.verifyCapability("db:read", "users_table");

if (result.isVerified()) {
    // Safe to proceed
    User user = userRepository.findById(userId);
} else {
    System.err.println("Access denied: " + result.getStatus());
}

// With risk level
VerificationResult result = agent.verifyCapability(
    "payment:process",
    "stripe_api",
    RiskLevel.HIGH,
    Map.of("amount", 1000)
);
```

### Perform Action (Recommended)

Execute an action with automatic verification and logging.

```java
// Basic usage with lambda
User user = agent.performAction("db:read", "users_table", () -> {
    return userRepository.findById(userId);
});

// With risk level
PaymentResult payment = agent.performAction(
    "payment:process",
    "stripe_api",
    RiskLevel.HIGH,
    () -> paymentService.process(request)
);
```

### Get Agent Details

```java
Map<String, Object> details = agent.getAgentDetails();

System.out.println("Agent ID: " + details.get("id"));
System.out.println("Trust Score: " + details.get("trustScore"));
System.out.println("Status: " + details.get("status"));
System.out.println("Capabilities: " + details.get("capabilities"));
```

### Request Capability

Request a new capability (requires admin approval).

```java
Map<String, Object> result = agent.requestCapability(
    "db:delete",
    "Need to implement user account deletion feature"
);

String status = (String) result.get("status");
if ("pending".equals(status)) {
    System.out.println("Request submitted - awaiting admin approval");
}
```

### Register Capability

Explicitly declare a capability.

```java
agent.registerCapability(
    "api:weather",
    "Fetch weather data from external API",
    "low"
);
```

---

## Risk Levels

```java
import org.opena2a.aim.client.RiskLevel;

RiskLevel.LOW       // Read operations, public data
RiskLevel.MEDIUM    // Write operations, data modification
RiskLevel.HIGH      // Sensitive operations
RiskLevel.CRITICAL  // Destructive operations, requires approval
```

---

## AspectJ Annotations

For declarative security, use the `@SecureAction` annotation with AspectJ.

### Basic Usage

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

### Spring Boot Configuration

```java
import org.opena2a.aim.annotations.SecureActionAspect;
import org.opena2a.aim.client.AIMClient;

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

---

## MCP Server Integration

### Register MCP Server

```java
Map<String, Object> result = agent.registerMcp(
    "filesystem-mcp",     // server ID
    "sdk_registration",   // detection method
    0.85                  // confidence score
);

System.out.println("Server ID: " + result.get("id"));
```

### List MCP Servers

```java
List<Map<String, Object>> servers = agent.listMcpServers(20);

for (Map<String, Object> server : servers) {
    System.out.println(server.get("name") + ": " + server.get("status"));
}
```

### Attest MCP Server

```java
Map<String, Object> attestation = agent.attestMcp(
    serverId,
    "http://localhost:3001",
    "filesystem-mcp",
    Arrays.asList("read_file", "write_file"),
    45.0  // confidence threshold
);

System.out.println("Attestation ID: " + attestation.get("id"));
System.out.println("Confidence: " + attestation.get("confidenceScore"));
```

### Record Tool Usage

```java
agent.useMcpTool(
    serverId,
    "read_file",
    "http://localhost:3001",
    "filesystem-mcp"
);
```

---

## Builder Pattern

For more control, use the builder pattern:

```java
AIMClient agent = new AIMClient.Builder()
    .agentName("my-enterprise-agent")
    .aimUrl("https://aim.yourcompany.com")
    .clientId("client-id")
    .clientSecret("client-secret")
    .organizationId("org-id")
    .agentType(AgentType.LANGCHAIN)
    .addCapability("db:read")
    .addCapability("db:write")
    .addCapability("api:call")
    .tags(Arrays.asList("production", "team-a"))
    .metadata(Map.of("owner", "john@example.com"))
    .build();
```

---

## Credential Management

### Credential Lookup Order

Credentials are loaded in this order:

1. **Environment Variables** (highest priority)
   - `AIM_URL`
   - `AIM_REFRESH_TOKEN`
   - `AIM_SDK_TOKEN_ID`
   - `AIM_CLIENT_ID` / `AIM_CLIENT_SECRET` (legacy)

2. **Home Directory** - `~/.aim/sdk_credentials.json`

3. **Local Directory** - `.aim/sdk_credentials.json`

4. **Classpath** - Bundled in JAR

### Programmatic Credential Management

```java
import org.opena2a.aim.credentials.CredentialManager;

// Load SDK credentials
Map<String, String> creds = CredentialManager.loadSdkCredentials();

// Save SDK credentials
Map<String, String> newCreds = Map.of(
    "aimUrl", "https://aim.mycompany.com",
    "refreshToken", "your-refresh-token",
    "sdkTokenId", "your-token-id"
);
CredentialManager.saveSdkCredentials(newCreds, Path.of("~/.aim/sdk_credentials.json"));

// Agent-specific credentials
Map<String, String> agentCreds = CredentialManager.loadAgentCredentials("my-agent");
CredentialManager.saveAgentCredentials("my-agent", agentCreds);
```

---

## Exception Handling

### Exception Hierarchy

```java
import org.opena2a.aim.exceptions.*;

// Base exception
AIMException

// Specific exceptions
ActionDeniedException     // 403 - Capability not allowed
AuthenticationException   // 401 - Auth failed
VerificationException     // Verification failed
CredentialException       // Credential issues
ConfigurationException    // Configuration issues
```

### Example Error Handling

```java
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

---

## Complete Example

```java
import org.opena2a.aim.client.*;
import org.opena2a.aim.annotations.SecureAction;
import java.util.*;

public class WeatherAgent {

    private final AIMClient agent;

    public WeatherAgent() {
        // Register agent with AIM
        this.agent = AIMClient.secure(
            "weather-agent",
            Arrays.asList("api:weather", "api:forecast"),
            AgentType.CUSTOM,
            null,
            "Weather information agent",
            Arrays.asList("production", "public-api"),
            Map.of("service", "openweathermap")
        );
    }

    public Map<String, Object> getWeather(String city) {
        return agent.performAction("api:weather", "openweathermap", RiskLevel.LOW, () -> {
            // Call weather API
            return Map.of(
                "city", city,
                "temperature", 72,
                "condition", "Sunny"
            );
        });
    }

    public static void main(String[] args) {
        WeatherAgent weatherAgent = new WeatherAgent();

        Map<String, Object> weather = weatherAgent.getWeather("San Francisco");
        System.out.println("Weather: " + weather);

        // Check trust score
        Map<String, Object> details = weatherAgent.agent.getAgentDetails();
        System.out.println("Trust Score: " + details.get("trustScore"));
    }
}
```

---

## Troubleshooting

### Issue: "Authentication failed"

**Error**: `AuthenticationException: Invalid credentials`

**Solution**:
1. Check SDK credentials in `~/.aim/sdk_credentials.json`
2. Re-download SDK from dashboard
3. Verify AIM backend is running

### Issue: "Connection refused"

**Error**: `AIMException: Connection refused`

**Solution**:
1. Check AIM backend: `curl http://localhost:8080/health`
2. Verify `AIM_URL` environment variable
3. Check firewall settings

### Issue: "ActionDeniedException"

**Error**: `ActionDeniedException: Capability 'db:delete' not allowed`

**Solution**:
1. Register the capability: `agent.registerCapability("db:delete", "...")`
2. Check enforcement mode (MONITORING vs STRICT)
3. Request capability approval from admin

---

## Next Steps

- **[Python SDK Guide →](./python.md)** - Python SDK reference
- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography
- **[Trust Scoring Guide →](./trust-scoring.md)** - 8-factor trust algorithm

---

<div align="center">

[Back to Home](../../README.md) | [SDK Documentation](./index.md) | [Get Help](https://discord.gg/opena2a)

</div>
