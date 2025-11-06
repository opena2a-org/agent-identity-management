# AIM SDK Examples

This directory contains working code examples demonstrating various features of the AIM Python SDK.

## 📝 Available Examples

### [example.py](./example.py)
**Basic AIM SDK Usage**

Demonstrates the fundamental SDK features:
- Agent registration and verification
- API key management
- Trust score monitoring
- Activity logging
- Error handling

**Use this when**: You're getting started with AIM and want to understand the core functionality.

```bash
# Run the example
python examples/example.py
```

---

### [example_auto_detection.py](./example_auto_detection.py)
**Automatic Framework Detection**

Demonstrates how AIM SDK automatically detects and integrates with popular AI frameworks:
- LangChain detection
- CrewAI detection
- AutoGen detection
- Custom agent patterns
- Automatic registration

**Use this when**: You want AIM to automatically detect your framework without manual configuration.

```bash
# Run the example
python examples/example_auto_detection.py
```

---

### [example_stripe_moment.py](./example_stripe_moment.py)
**Real-World Integration Example**

Demonstrates a production-like integration scenario:
- Multi-agent coordination
- Capability-based authorization
- Real-time verification
- Trust score impact on decisions
- Error recovery

**Use this when**: You want to see how AIM works in a realistic production scenario.

```bash
# Run the example
python examples/example_stripe_moment.py
```

---

## 🚀 Running Examples

### Prerequisites

1. **Install the SDK**:
   ```bash
   pip install -r ../requirements.txt
   ```

2. **Set Environment Variables**:
   ```bash
   export AIM_API_KEY="your-api-key"
   export AIM_API_URL="http://localhost:8080"  # or your AIM server URL
   ```

3. **Ensure AIM Server is Running**:
   ```bash
   # Check if AIM server is accessible
   curl http://localhost:8080/health
   ```

### Running an Example

```bash
# From the sdk/python directory
python examples/example.py

# Or with environment variables inline
AIM_API_KEY=your-key python examples/example_auto_detection.py
```

## 📚 Additional Resources

- **[Main SDK README](../README.md)** - Installation and setup
- **[Integration Guides](../docs/)** - Framework-specific integration guides
- **[Tests](../tests/)** - Comprehensive test suite with more examples

## 🐛 Troubleshooting

### Common Issues

**"Connection refused" error**:
- Ensure AIM server is running (`docker compose up` or `go run main.go`)
- Check `AIM_API_URL` environment variable is set correctly

**"Unauthorized" error**:
- Verify your `AIM_API_KEY` is valid
- Check the API key hasn't expired in AIM dashboard

**"Agent not found" error**:
- Ensure you've registered the agent first
- Check the agent ID is correct

For more help, see the [troubleshooting guide](../docs/README.md) or open an issue on GitHub.

---

**Last Updated**: October 2024
