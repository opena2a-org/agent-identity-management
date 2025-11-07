# AIM SDK Documentation

This directory contains comprehensive integration guides and configuration documentation for the AIM Python SDK.

## 📚 Integration Guides

### Framework Integrations

- **[LANGCHAIN_INTEGRATION.md](./LANGCHAIN_INTEGRATION.md)** - Complete guide for integrating AIM with LangChain agents
  - LCEL agent integration
  - Chain-based agent integration
  - Tool registration and verification
  - Real-time capability authorization

- **[CREWAI_INTEGRATION.md](./CREWAI_INTEGRATION.md)** - Integration guide for CrewAI agents
  - Agent decorator usage
  - Task and crew integration
  - Multi-agent coordination with AIM
  - Security and trust scoring

- **[MCP_INTEGRATION.md](./MCP_INTEGRATION.md)** - Model Context Protocol (MCP) server integration
  - MCP server registration
  - Cryptographic verification
  - Server lifecycle management
  - Real-time verification

- **[MICROSOFT_COPILOT_INTEGRATION.md](./MICROSOFT_COPILOT_INTEGRATION.md)** - Microsoft Copilot Studio integration
  - Power Platform connector setup
  - Copilot skill registration
  - Authentication configuration
  - Runtime verification

## ⚙️ Configuration

- **[ENV_CONFIG.md](./ENV_CONFIG.md)** - Environment variables and configuration options
  - Authentication settings
  - API endpoint configuration
  - Logging and debugging
  - Security options

- **[VERSIONING.md](./VERSIONING.md)** - SDK versioning strategy and release process
  - Semantic versioning explained
  - VERSION file usage
  - Compatibility guarantees
  - Upgrade guides
  - Support policy

## 🚀 Quick Start

For basic usage and getting started, see the main [README.md](../README.md) in the SDK root directory.

For working code examples, check the [examples/](../examples/) directory.

## 📖 Additional Resources

- **[Main SDK README](../README.md)** - Installation, quick start, and basic usage
- **[Examples](../examples/)** - Working code examples
- **[Tests](../tests/)** - Comprehensive test suite demonstrating SDK functionality

## 🤝 Contributing

When adding new integration guides:
1. Follow the existing document structure
2. Include working code examples
3. Document all configuration options
4. Add troubleshooting section
5. Update this README with a link to your guide

---

**Last Updated**: October 2024
