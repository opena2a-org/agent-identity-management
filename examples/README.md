# AIM Integration Examples

This directory contains real-world examples demonstrating how to integrate AI agents and MCP servers with the Agent Identity Management (AIM) platform.

## 📁 Available Examples

### 1. Flight Search Agent (`flight-search-agent/`)

A complete AI agent that searches flights and demonstrates:
- ✅ Auto-registration with AIM using `secure("agent-name")`
- ✅ Auto-detection of capabilities and MCPs
- ✅ Ed25519 cryptographic signatures
- ✅ Action verification workflow
- ✅ Activity logging and audit trail
- ✅ Trust scoring through verified actions

**Use Case**: Production AI agent performing real searches with security verification

**Tech Stack**: Python 3.11+, AIM SDK

[View Documentation →](./flight-search-agent/README.md)

### 2. LangChain CRUD Agent (`langchain-crud-agent/`)

A LangChain-based agent that performs CRUD operations on a todo list:
- ✅ LangChain integration with custom tools
- ✅ Secured CRUD operations (Create, Read, Update, Delete)
- ✅ AIM SDK `perform_action` decorator
- ✅ Real-time trust scoring
- ✅ Security alerts for dangerous operations

**Use Case**: LangChain agents with enterprise security and compliance

**Tech Stack**: Python 3.11+, LangChain, Google Gemini, AIM SDK

[View Code →](./langchain-crud-agent/langchain_crud_agent.py)

### 3. MCP Server Demo (`mcp-server-demo/`)

Model Context Protocol (MCP) server registration example:
- ✅ MCP server registration with AIM
- ✅ Cryptographic attestation
- ✅ Capability declarations
- ✅ Server verification workflow

**Use Case**: Registering and securing MCP servers

**Tech Stack**: Python 3.11+, MCP Protocol, AIM SDK

[View Code →](./mcp-server-demo/mcp-server.py)

## 🚀 Quick Start

### Prerequisites

1. **AIM Platform Running**
   ```bash
   # Start AIM backend and frontend
   docker-compose up -d
   ```

2. **Download SDK**
   - Navigate to http://localhost:3000/dashboard/settings
   - Click "Download SDK"
   - Extract to your project or add to PYTHONPATH

### Running an Example

```bash
# Navigate to any example
cd examples/flight-search-agent/

# Install dependencies
pip install -r requirements.txt

# Run the agent
python3 flight_agent.py
```

## 📚 Integration Patterns

### Pattern 1: Simple Auto-Registration
```python
from aim_sdk import secure

# One-line registration
client = secure("my-agent-name")

# Agent is now registered and verified
```

### Pattern 2: Action Verification
```python
# Request verification before performing action
verification = client.verify_action(
    action_type="search_flights",
    action_details={"destination": "NYC", "risk_level": "low"}
)

if verification.approved:
    # Perform the action
    results = search_flights("NYC")

    # Log the result
    client.log_action_result(
        action_type="search_flights",
        success=True,
        metadata={"flights_found": len(results)}
    )
```

### Pattern 3: LangChain Decorator
```python
from aim_sdk import perform_action

class TodoTool(BaseTool):
    @perform_action(action_type="create_todo")
    def _run(self, task: str):
        # Action is automatically verified by AIM
        return add_todo(task)
```

## 🎯 Use Cases by Example

| Use Case | Example | Key Features |
|----------|---------|--------------|
| **Production AI Search** | Flight Search Agent | Auto-detection, verification, audit trail |
| **LangChain Integration** | CRUD Agent | Decorator pattern, tool security |
| **MCP Server Registration** | MCP Server Demo | Server attestation, capability management |
| **Enterprise Compliance** | All examples | Audit logs, trust scoring, RBAC |

## 📖 Documentation

### Core Concepts
- **Agent Registration**: [View Docs](../docs/agent-registration.md)
- **Action Verification**: [View Docs](../docs/verification-workflow.md)
- **Trust Scoring**: [View Docs](../docs/trust-scoring.md)
- **MCP Integration**: [View Docs](../docs/mcp-integration.md)

### SDK Documentation
- **Python SDK**: [View SDK Docs](../sdk/python/README.md)
- **API Reference**: [View API Docs](../docs/api-reference.md)

## 🛠️ Development

### Project Structure
```
examples/
├── flight-search-agent/     # Complete flight search example
│   ├── flight_agent.py      # Main agent code
│   ├── README.md            # Comprehensive docs
│   └── requirements.txt
├── langchain-crud-agent/    # LangChain integration
│   ├── langchain_crud_agent.py
│   └── requirements.txt
├── mcp-server-demo/         # MCP server example
│   ├── mcp-server.py
│   └── requirements.txt
└── README.md                # This file
```

### Adding New Examples

1. Create a new directory under `examples/`
2. Include a README.md explaining the use case
3. Add requirements.txt with dependencies
4. Update this README with your example

## 🔧 Troubleshooting

### Authentication Errors

If you see "Authentication failed" errors, your SDK credentials may have expired:

```bash
# Option 1: Download fresh SDK from dashboard
open http://localhost:3000/dashboard/settings

# Option 2: Use automated QA test (flight-search-agent only)
cd examples/flight-search-agent/
./quick_qa_test.sh
```

### Empty Dashboard Tabs

This is expected if:
1. Agent hasn't performed any actions yet
2. Credentials have expired (see above)

**Solution**: Get fresh credentials and run the agent

### Connection Refused

Ensure AIM platform is running:

```bash
# Check if backend is running
curl http://localhost:8080/api/v1/health

# Check if frontend is running
curl http://localhost:3000
```

## 🎉 Success Metrics

After running an example successfully, you should see:

- ✅ Agent registered in AIM dashboard
- ✅ Capabilities auto-detected
- ✅ Trust score calculated
- ✅ Status: Verified
- ✅ Activity logs populated
- ✅ Dashboard tabs showing data

## 💡 Next Steps

1. **Explore Examples**: Try each example to understand different integration patterns
2. **Customize**: Modify examples for your specific use case
3. **Build Your Agent**: Use these examples as templates
4. **Join Community**: Share your integration stories

## 📞 Support

- **GitHub Issues**: [Report bugs](https://github.com/opena2a-org/agent-identity-management/issues)
- **Discussions**: [Ask questions](https://github.com/opena2a-org/agent-identity-management/discussions)
- **Documentation**: [View full docs](../docs/)

---

**Last Updated**: January 2025
