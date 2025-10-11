# AIM SDK Integration Guide

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Python SDK](#python-sdk)
3. [Go SDK](#go-sdk)
4. [JavaScript/TypeScript SDK](#javascripttypescript-sdk)
5. [Framework Integrations](#framework-integrations)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

---

## Overview

AIM provides SDKs in **Python, Go, and JavaScript/TypeScript** for seamless integration with your AI agent applications. All SDKs provide:

- **One-Line Registration**: Register agents with a single function call
- **Automatic Key Management**: Ed25519 keypair generation and secure storage
- **Cryptographic Verification**: Challenge-response authentication
- **Trust Score Tracking**: Monitor agent trustworthiness
- **MCP Integration**: Connect to Model Context Protocol servers
- **Action Verification**: Verify actions before execution

### SDK Feature Comparison

| Feature | Python SDK | Go SDK | JS/TS SDK |
|---------|-----------|--------|-----------|
| Agent Registration | ✅ | ✅ | ✅ |
| Key Generation (Ed25519) | ✅ | ✅ | ✅ |
| Keyring Storage | ✅ | ✅ | ✅ |
| Cryptographic Verification | ✅ | ✅ | ✅ |
| MCP Auto-Detection | ✅ | ⏳ | ⏳ |
| Action Verification | ✅ | ✅ | ✅ |
| CrewAI Integration | ✅ | ❌ | ❌ |
| LangChain Integration | ✅ | ✅ | ✅ |
| Type Safety | ✅ | ✅ | ✅ |

---

## Python SDK

### Installation

**Option 1: From Source** (Development)

```bash
cd agent-identity-management/sdks/python
pip install -e .
```

**Option 2: From AIM Dashboard** (Production)

1. Navigate to http://localhost:3000/dashboard/sdk
2. Click "Download SDK" → "Python"
3. Extract and install:

```bash
unzip aim-sdk-python.zip
cd aim-sdk-python
pip install .
```

**Option 3: From PyPI** (When published)

```bash
pip install aim-sdk
```

### Quick Start

**1. Register Agent (One-Line)**

```python
from aim_sdk import register_agent

# Register agent with automatic key generation
agent = register_agent(
    name="my-agent",
    aim_url="http://localhost:8080",
    email="user@example.com",
    password="YourPassword123!"
)

print(f"Agent registered: {agent.id}")
print(f"Trust Score: {agent.trust_score}/100")
```

**2. Load Existing Agent**

```python
from aim_sdk import AIMClient

# Load from stored credentials (keyring)
agent = AIMClient.load_from_credentials("my-agent")

# Or initialize manually
agent = AIMClient(
    api_url="http://localhost:8080",
    agent_id="your-agent-id",
    private_key=b"your-private-key"
)
```

**3. Verify Agent**

```python
# Perform cryptographic verification
result = await agent.verify()

print(f"Verification Status: {result['status']}")
print(f"Trust Score: {result['trust_score']}/100")
```

### Action Verification

**Decorator-Based Verification**:

```python
from aim_sdk import AIMClient

agent = AIMClient.load_from_credentials("my-agent")

# Verify action before execution
@agent.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    # Your code here
    return database.query(f"SELECT * FROM users WHERE id = {user_id}")

# Action is verified, logged, and executed
user = get_user_data(12345)
```

**Explicit Verification**:

```python
# Verify action explicitly
verification = await agent.verify_action(
    action="modify_file",
    resource="config.json",
    metadata={"reason": "update API key"}
)

if verification.approved:
    # Perform action
    with open("config.json", "w") as f:
        f.write(new_config)
else:
    print(f"Action blocked: {verification.reason}")
```

### MCP Integration

**Auto-Detect MCP Servers**:

```python
from aim_sdk import AIMClient

agent = AIMClient.load_from_credentials("my-agent")

# Auto-detect MCPs from Claude config
detected_mcps = await agent.detect_mcp_servers()

print(f"Found {len(detected_mcps)} MCP servers")
for mcp in detected_mcps:
    print(f"- {mcp['name']}: {mcp['command']}")

# Register detected MCPs
await agent.register_detected_mcps(detected_mcps)
```

**Manual MCP Registration**:

```python
# Register MCP server
mcp = await agent.register_mcp_server(
    name="filesystem-mcp",
    url="http://localhost:3100",
    capabilities=["read", "write", "search"]
)

# Connect agent to MCP
await agent.connect_to_mcp(mcp['id'])

print(f"Connected to MCP: {mcp['name']}")
```

### Framework Integrations

#### CrewAI

```python
from crewai import Agent, Task, Crew
from aim_sdk.integrations.crewai import AIMCrewWrapper

# Create CrewAI crew
researcher = Agent(
    role='Researcher',
    goal='Research AI safety topics',
    backstory='Expert in AI safety research',
    tools=[search_tool, scrape_tool]
)

writer = Agent(
    role='Writer',
    goal='Write comprehensive reports',
    backstory='Technical writer with AI expertise',
    tools=[write_tool]
)

crew = Crew(
    agents=[researcher, writer],
    tasks=[research_task, writing_task],
    verbose=True
)

# Wrap crew with AIM verification
aim_agent = AIMClient.load_from_credentials("crewai-agent")
verified_crew = AIMCrewWrapper(
    crew=crew,
    aim_agent=aim_agent,
    risk_level="medium"
)

# All agent actions are verified and logged
result = verified_crew.kickoff(inputs={"topic": "AI alignment"})
```

#### LangChain

```python
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain.tools import Tool
from aim_sdk.integrations.langchain import AIMAgentExecutor

# Create LangChain agent
tools = [
    Tool(name="Search", func=search_function),
    Tool(name="Database", func=db_function)
]

agent = create_openai_functions_agent(llm, tools, prompt)

# Wrap with AIM verification
aim_agent = AIMClient.load_from_credentials("langchain-agent")
verified_agent = AIMAgentExecutor(
    agent=agent,
    tools=tools,
    aim_agent=aim_agent,
    verbose=True
)

# Every tool use is verified before execution
result = verified_agent.run("Find recent AI research papers and summarize")
```

### Async/Await Support

```python
import asyncio
from aim_sdk import AIMClient

async def main():
    agent = AIMClient(
        api_url="http://localhost:8080",
        email="user@example.com",
        password="YourPassword123!"
    )

    # Async registration
    await agent.register_agent("my-async-agent")

    # Async verification
    result = await agent.verify()

    # Async MCP detection
    mcps = await agent.detect_mcp_servers()

    print(f"Trust Score: {result['trust_score']}")

if __name__ == "__main__":
    asyncio.run(main())
```

### Configuration

**Environment Variables**:

```bash
# .env file
AIM_API_URL=http://localhost:8080
AIM_EMAIL=user@example.com
AIM_PASSWORD=YourPassword123!
AIM_AGENT_NAME=my-agent
```

**Python Code**:

```python
import os
from aim_sdk import AIMClient

# Load from environment variables
agent = AIMClient(
    api_url=os.getenv("AIM_API_URL"),
    email=os.getenv("AIM_EMAIL"),
    password=os.getenv("AIM_PASSWORD")
)

# Or use config file
from aim_sdk import load_config

config = load_config("aim_config.yaml")
agent = AIMClient(**config)
```

---

## Go SDK

### Installation

```bash
go get github.com/opena2a/agent-identity-management/sdks/go
```

### Quick Start

**1. Register Agent**

```go
package main

import (
    "context"
    "fmt"
    "log"

    aim "github.com/opena2a/agent-identity-management/sdks/go"
)

func main() {
    // Initialize client
    client, err := aim.NewClient(
        "http://localhost:8080",
        "user@example.com",
        "YourPassword123!",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Register agent
    agent, err := client.RegisterAgent(context.Background(), &aim.RegisterAgentRequest{
        Name:        "my-go-agent",
        AgentType:   "ai_agent",
        Description: "Go-based AI agent",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Agent registered: %s\n", agent.ID)
    fmt.Printf("Trust Score: %.2f/100\n", agent.TrustScore)
}
```

**2. Load Existing Agent**

```go
// Load from keyring
agent, err := aim.LoadFromCredentials("my-go-agent")
if err != nil {
    log.Fatal(err)
}

// Or initialize manually
client := &aim.Client{
    APIURL:     "http://localhost:8080",
    AgentID:    "your-agent-id",
    PrivateKey: privateKeyBytes,
}
```

**3. Verify Agent**

```go
// Perform cryptographic verification
result, err := client.Verify(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Verification Status: %s\n", result.Status)
fmt.Printf("Trust Score: %.2f/100\n", result.TrustScore)
```

### Action Verification

```go
// Verify action before execution
verification, err := client.VerifyAction(context.Background(), &aim.VerifyActionRequest{
    Action:   "read_file",
    Resource: "config.json",
    Metadata: map[string]interface{}{
        "reason": "configuration update",
    },
})
if err != nil {
    log.Fatal(err)
}

if verification.Approved {
    // Perform action
    data, err := os.ReadFile("config.json")
    // ...
} else {
    fmt.Printf("Action blocked: %s\n", verification.Reason)
}
```

### MCP Integration

```go
// Register MCP server
mcp, err := client.RegisterMCPServer(context.Background(), &aim.RegisterMCPServerRequest{
    Name:         "filesystem-mcp",
    URL:          "http://localhost:3100",
    Capabilities: []string{"read", "write", "search"},
})
if err != nil {
    log.Fatal(err)
}

// Connect agent to MCP
err = client.ConnectToMCP(context.Background(), mcp.ID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Connected to MCP: %s\n", mcp.Name)
```

### LangChain Go Integration

```go
import (
    langchain "github.com/tmc/langchaingo"
    aim "github.com/opena2a/agent-identity-management/sdks/go"
)

// Wrap LangChain agent with AIM
aimAgent, err := aim.LoadFromCredentials("langchain-go-agent")
if err != nil {
    log.Fatal(err)
}

verifiedAgent := aim.WrapLangChainAgent(
    langchainAgent,
    aimAgent,
    aim.WithRiskLevel("medium"),
)

// Every tool use is verified
result, err := verifiedAgent.Run(context.Background(), "Summarize recent AI papers")
```

---

## JavaScript/TypeScript SDK

### Installation

```bash
npm install @aim/sdk
# or
yarn add @aim/sdk
```

### Quick Start

**1. Register Agent (TypeScript)**

```typescript
import { AIMClient } from '@aim/sdk';

async function main() {
  // Initialize client
  const client = new AIMClient({
    apiUrl: 'http://localhost:8080',
    email: 'user@example.com',
    password: 'YourPassword123!',
  });

  // Register agent
  const agent = await client.registerAgent({
    name: 'my-js-agent',
    agentType: 'ai_agent',
    description: 'JavaScript-based AI agent',
  });

  console.log(`Agent registered: ${agent.id}`);
  console.log(`Trust Score: ${agent.trustScore}/100`);
}

main();
```

**2. Load Existing Agent**

```typescript
// Load from keyring
const agent = await AIMClient.loadFromCredentials('my-js-agent');

// Or initialize manually
const client = new AIMClient({
  apiUrl: 'http://localhost:8080',
  agentId: 'your-agent-id',
  privateKey: privateKeyBuffer,
});
```

**3. Verify Agent**

```typescript
// Perform cryptographic verification
const result = await client.verify();

console.log(`Verification Status: ${result.status}`);
console.log(`Trust Score: ${result.trustScore}/100`);
```

### Action Verification

```typescript
// Verify action before execution
const verification = await client.verifyAction({
  action: 'write_file',
  resource: 'output.txt',
  metadata: {
    reason: 'save analysis results',
  },
});

if (verification.approved) {
  // Perform action
  await fs.writeFile('output.txt', data);
} else {
  console.log(`Action blocked: ${verification.reason}`);
}
```

### MCP Integration

```typescript
// Register MCP server
const mcp = await client.registerMCPServer({
  name: 'database-mcp',
  url: 'http://localhost:3100',
  capabilities: ['query', 'execute'],
});

// Connect agent to MCP
await client.connectToMCP(mcp.id);

console.log(`Connected to MCP: ${mcp.name}`);
```

### React Integration

```typescript
import { useAIMAgent } from '@aim/sdk/react';

function MyComponent() {
  const { agent, verify, trustScore, loading } = useAIMAgent('my-agent');

  const handleAction = async () => {
    const result = await verify();
    if (result.approved) {
      // Perform action
    }
  };

  return (
    <div>
      <h1>Agent: {agent?.name}</h1>
      <p>Trust Score: {trustScore}/100</p>
      <button onClick={handleAction} disabled={loading}>
        Perform Verified Action
      </button>
    </div>
  );
}
```

### LangChain.js Integration

```typescript
import { ChatOpenAI } from 'langchain/chat_models/openai';
import { initializeAgentExecutorWithOptions } from 'langchain/agents';
import { AIMAgentExecutor } from '@aim/sdk/langchain';

// Create LangChain agent
const llm = new ChatOpenAI({ temperature: 0 });
const tools = [searchTool, calculatorTool];

const executor = await initializeAgentExecutorWithOptions(tools, llm, {
  agentType: 'openai-functions',
});

// Wrap with AIM verification
const aimAgent = await AIMClient.loadFromCredentials('langchain-js-agent');
const verifiedExecutor = new AIMAgentExecutor(executor, aimAgent);

// Every tool use is verified
const result = await verifiedExecutor.call({
  input: 'What is the weather in San Francisco?',
});
```

---

## Framework Integrations

### CrewAI (Python)

**Installation**:
```bash
pip install aim-sdk[crewai]
```

**Integration**:
```python
from crewai import Agent, Task, Crew
from aim_sdk.integrations.crewai import AIMCrewWrapper

# Create crew
crew = Crew(agents=[agent1, agent2], tasks=[task1, task2])

# Wrap with AIM
aim_agent = AIMClient.load_from_credentials("crewai-agent")
verified_crew = AIMCrewWrapper(crew, aim_agent, risk_level="medium")

# Run with verification
result = verified_crew.kickoff(inputs={"topic": "AI safety"})
```

### LangChain (Python/Go/JS)

**Python**:
```python
from aim_sdk.integrations.langchain import AIMAgentExecutor

verified_agent = AIMAgentExecutor(agent, tools, aim_agent)
```

**Go**:
```go
verifiedAgent := aim.WrapLangChainAgent(langchainAgent, aimAgent)
```

**JavaScript**:
```typescript
const verifiedExecutor = new AIMAgentExecutor(executor, aimAgent);
```

### AutoGen (Python)

```python
from aim_sdk.integrations.autogen import AIMAutoGenWrapper

# Wrap AutoGen agent
verified_agent = AIMAutoGenWrapper(autogen_agent, aim_agent)

# All actions verified
result = verified_agent.initiate_chat(message="Analyze data")
```

### Microsoft Copilot Studio

```python
from aim_sdk.integrations.microsoft import AIMCopilotWrapper

# Wrap Copilot agent
verified_copilot = AIMCopilotWrapper(copilot_agent, aim_agent)

# Verified actions
result = verified_copilot.execute_action("summarize_report")
```

---

## Best Practices

### 1. Credential Management

**✅ DO**:
- Store credentials in OS keyring (automatic with SDKs)
- Use environment variables for configuration
- Rotate API keys every 90 days
- Use separate agents for dev/staging/prod

**❌ DON'T**:
- Hardcode credentials in code
- Commit `.env` files to git
- Share credentials between environments
- Store private keys in databases

### 2. Error Handling

**Python**:
```python
from aim_sdk import AIMClient, AIMError

try:
    agent = AIMClient.load_from_credentials("my-agent")
    result = await agent.verify()
except AIMError as e:
    print(f"AIM Error: {e.message}")
    print(f"Error Code: {e.code}")
    # Handle error appropriately
```

**Go**:
```go
agent, err := aim.LoadFromCredentials("my-agent")
if err != nil {
    log.Printf("Failed to load agent: %v", err)
    return
}
```

**TypeScript**:
```typescript
try {
  const agent = await AIMClient.loadFromCredentials('my-agent');
  const result = await agent.verify();
} catch (error) {
  if (error instanceof AIMError) {
    console.error(`AIM Error: ${error.message}`);
    console.error(`Error Code: ${error.code}`);
  }
}
```

### 3. Logging & Monitoring

**Enable SDK Logging**:

**Python**:
```python
import logging
from aim_sdk import AIMClient

# Enable DEBUG logging
logging.basicConfig(level=logging.DEBUG)
aim_logger = logging.getLogger('aim_sdk')
aim_logger.setLevel(logging.DEBUG)
```

**Go**:
```go
import "log"

client, err := aim.NewClient(
    apiURL,
    email,
    password,
    aim.WithLogger(log.Default()),
)
```

**TypeScript**:
```typescript
const client = new AIMClient({
  apiUrl: 'http://localhost:8080',
  logLevel: 'debug',
});
```

### 4. Trust Score Monitoring

```python
# Check trust score before critical operations
agent = AIMClient.load_from_credentials("my-agent")

if agent.trust_score < 75:
    print("Warning: Low trust score. Verification recommended.")
    await agent.verify()

if agent.trust_score >= 85:
    # Proceed with critical operation
    perform_critical_action()
else:
    print("Error: Trust score too low for critical operations")
```

### 5. Graceful Degradation

```python
# Implement fallback if AIM is unavailable
try:
    verification = await agent.verify_action("read_file", "data.csv")
    if verification.approved:
        data = read_file("data.csv")
except AIMError as e:
    if e.code == "SERVICE_UNAVAILABLE":
        print("Warning: AIM unavailable, proceeding without verification")
        data = read_file("data.csv")  # Fallback
    else:
        raise  # Re-raise other errors
```

---

## Troubleshooting

### Common Issues

**Problem**: `KeyringError: Failed to store credentials`

**Solution**:
```bash
# macOS: Grant Keychain access
security unlock-keychain

# Linux: Install keyring backend
sudo apt-get install gnome-keyring

# Windows: No action needed (Credential Manager works out of box)
```

**Problem**: `ConnectionError: Failed to connect to AIM server`

**Solution**:
```python
# Check AIM server is running
import requests
try:
    response = requests.get("http://localhost:8080/health")
    print(f"Server status: {response.json()}")
except requests.ConnectionError:
    print("AIM server not running. Start with: docker compose up -d")
```

**Problem**: `VerificationError: Signature verification failed`

**Solution**:
```python
# Re-register agent with new keypair
from aim_sdk import AIMClient

client = AIMClient(
    api_url="http://localhost:8080",
    email="user@example.com",
    password="YourPassword123!"
)

# Delete old agent
await client.delete_agent("my-agent")

# Register new agent
agent = await client.register_agent("my-agent")
```

**Problem**: `TokenExpiredError: JWT token expired`

**Solution**:
```python
# SDK automatically refreshes tokens, but if manual refresh needed:
await agent.refresh_token()
```

### Debugging Tips

**1. Enable Verbose Logging**

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

**2. Test Connection**

```python
from aim_sdk import AIMClient

client = AIMClient(api_url="http://localhost:8080")
health = await client.health_check()
print(f"Server status: {health['status']}")
```

**3. Verify Keyring Access**

```python
import keyring

# Test keyring
keyring.set_password("aim_test", "test_user", "test_password")
password = keyring.get_password("aim_test", "test_user")
print(f"Keyring works: {password == 'test_password'}")
```

---

## References

- [Python SDK README](../sdks/python/README.md)
- [Go SDK README](../sdks/go/README.md)
- [JavaScript SDK README](../sdks/javascript/README.md)
- [API Documentation](API.md)
- [Quick Start Guide](QUICK_START.md)

---

**Maintained by**: OpenA2A SDK Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026

For SDK support, contact: sdk-support@opena2a.org
