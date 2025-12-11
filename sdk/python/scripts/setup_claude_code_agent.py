#!/usr/bin/env python3
"""
Setup Claude Code Agent with MCP Auto-Detection

This script:
1. Registers a Claude Code agent with AIM
2. Auto-detects MCPs from Claude Desktop config
3. Creates attestations connecting the agent to all detected MCPs
4. Generates sample security events and compliance data

Usage:
    python setup_claude_code_agent.py
"""

import os
import sys
import json
import time
from pathlib import Path
from datetime import datetime, timezone

# Add the SDK to path for development
SDK_PATH = Path(__file__).parent.parent
sys.path.insert(0, str(SDK_PATH))

from aim_sdk import (
    register_agent,
    AgentType,
    MCPDetector,
    auto_detect_mcps,
)
from aim_sdk.detection import get_mcp_server_config
from aim_sdk.integrations.mcp.registration import (
    register_mcp_server,
    attest_mcp_server,
    use_mcp_tool,
    list_mcp_servers,
)
from aim_sdk.console import console


# Configuration - Update these values
AIM_URL = "http://localhost:8080"  # Your AIM backend URL
API_KEY = None  # Optional: Use API key instead of SDK credentials

# Agent details
AGENT_NAME = "claude-code-agent"
AGENT_DISPLAY_NAME = "Claude Code Agent"
AGENT_DESCRIPTION = "Claude Code CLI instance with MCP integrations for development"


def print_header(title: str):
    """Print a section header."""
    print()
    print("=" * 60)
    print(f"  {title}")
    print("=" * 60)
    print()


def detect_mcps():
    """Detect all MCPs from Claude Desktop config."""
    print_header("MCP Auto-Detection")

    detector = MCPDetector()
    detections = detector.detect_from_claude_config()

    if not detections:
        console.warning("No MCPs found in Claude Desktop config")
        return []

    console.success(f"Found {len(detections)} MCP servers in Claude Desktop config:")
    for d in detections:
        print(f"  • {d['mcpServer']}")
        if d.get('details', {}).get('command'):
            cmd = d['details']['command']
            args = ' '.join(d['details'].get('args', [])[:3])
            print(f"    └─ {cmd} {args}...")

    return detections


def register_claude_code_agent():
    """Register the Claude Code agent with AIM."""
    print_header("Agent Registration")

    # Get MCP server names from detections
    mcp_detections = detect_mcps()
    mcp_names = [d['mcpServer'] for d in mcp_detections]

    try:
        # Register or load existing agent
        agent = register_agent(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            display_name=AGENT_DISPLAY_NAME,
            description=AGENT_DESCRIPTION,
            agent_type=AgentType.CLAUDE,
            version="1.0.0",
            repository_url="https://github.com/anthropics/claude-code",
            mcp_servers=mcp_names,
            capabilities=[
                "file:read",
                "file:write",
                "code:execute",
                "git:commit",
                "git:push",
                "api:call",
                "db:query",
                "network:fetch",
            ],
            tags=["development", "cli", "mcp-enabled"],
            metadata={
                "model": "claude-opus-4-5-20251101",
                "platform": "macos",
                "cli_version": "1.0.0",
            },
            auto_detect=True,
            force_new=False,  # Reuse existing if available
        )

        console.success(f"Agent registered/loaded: {agent.agent_id}")
        return agent, mcp_detections

    except Exception as e:
        console.error(f"Failed to register agent: {e}")
        raise


def register_and_attest_mcps(agent, mcp_detections):
    """Register MCP servers and create attestations."""
    print_header("MCP Registration & Attestation")

    if not mcp_detections:
        console.warning("No MCPs to register")
        return

    # First, check what MCPs are already registered
    try:
        existing_mcps = list_mcp_servers(agent)
        existing_names = {mcp.get('name', '').lower() for mcp in existing_mcps}
        console.info(f"Found {len(existing_mcps)} existing MCP servers in AIM")
    except Exception as e:
        console.warning(f"Could not list existing MCPs: {e}")
        existing_mcps = []
        existing_names = set()

    registered_count = 0
    attested_count = 0

    for detection in mcp_detections:
        server_name = detection['mcpServer']
        details = detection.get('details', {})
        command = details.get('command', '')
        args = details.get('args', [])

        # Build MCP URL from command + args
        mcp_url = f"{command} {' '.join(args)}" if args else command

        console.info(f"\nProcessing: {server_name}")

        # Check if already registered
        if server_name.lower() in existing_names:
            console.info(f"  └─ Already registered, looking up server ID...")

            # Find the server ID
            server_id = None
            for mcp in existing_mcps:
                if mcp.get('name', '').lower() == server_name.lower():
                    server_id = mcp.get('id')
                    break

            if server_id:
                # Create attestation for existing server
                try:
                    console.info(f"  └─ Creating attestation...")
                    attest_response = attest_mcp_server(
                        aim_client=agent,
                        server_id=server_id,
                        mcp_url=mcp_url,
                        mcp_name=server_name,
                        auto_discover=True,  # Discover capabilities automatically
                        discovery_timeout=15.0,
                    )
                    attested_count += 1
                    confidence = attest_response.get('mcpConfidenceScore', 0)
                    console.success(f"  └─ Attestation created (confidence: {confidence}%)")
                except Exception as e:
                    console.warning(f"  └─ Attestation failed: {e}")
            continue

        # Register new MCP server
        try:
            # Generate a public key for the MCP server
            from nacl.signing import SigningKey
            import base64

            signing_key = SigningKey.generate()
            public_key_bytes = signing_key.verify_key.encode()
            public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

            # Determine capabilities based on server name
            capabilities = ["tools"]  # Default capability
            if "filesystem" in server_name.lower() or "file" in server_name.lower():
                capabilities.extend(["resources"])
            if "github" in server_name.lower() or "git" in server_name.lower():
                capabilities.extend(["resources"])
            if "memory" in server_name.lower():
                capabilities.extend(["resources"])

            console.info(f"  └─ Registering MCP server...")

            register_response = register_mcp_server(
                aim_client=agent,
                server_name=server_name,
                server_url=mcp_url,
                public_key=public_key_b64,
                capabilities=capabilities,
                description=f"Auto-detected MCP server from Claude Desktop config",
                version="1.0.0",
            )

            registered_count += 1
            server_id = register_response.get('id')
            console.success(f"  └─ Registered with ID: {server_id}")

            # Create attestation for newly registered server
            if server_id:
                try:
                    time.sleep(0.5)  # Small delay to ensure server is ready
                    console.info(f"  └─ Creating attestation...")
                    attest_response = attest_mcp_server(
                        aim_client=agent,
                        server_id=server_id,
                        mcp_url=mcp_url,
                        mcp_name=server_name,
                        auto_discover=True,
                        discovery_timeout=15.0,
                    )
                    attested_count += 1
                    confidence = attest_response.get('mcpConfidenceScore', 0)
                    console.success(f"  └─ Attestation created (confidence: {confidence}%)")
                except Exception as e:
                    console.warning(f"  └─ Attestation failed: {e}")

        except Exception as e:
            console.error(f"  └─ Registration failed: {e}")

    print_header("Summary")
    console.success(f"Registered {registered_count} new MCP servers")
    console.success(f"Created {attested_count} attestations")


def simulate_mcp_usage(agent, mcp_detections):
    """Simulate MCP tool usage to generate connection data."""
    print_header("Simulating MCP Tool Usage")

    # Get existing MCPs to get their IDs
    try:
        existing_mcps = list_mcp_servers(agent)
        mcp_id_map = {mcp.get('name', '').lower(): mcp.get('id') for mcp in existing_mcps}
    except Exception as e:
        console.warning(f"Could not list MCPs: {e}")
        return

    # Define some simulated tool usages
    tool_usages = [
        ("filesystem", "read_file", "/tmp/test.txt"),
        ("filesystem", "write_file", "/tmp/output.txt"),
        ("github", "search_repositories", "langchain"),
        ("github", "create_issue", "bug report"),
        ("memory", "create_entities", "project notes"),
        ("browserbase", "browserbase_stagehand_navigate", "https://example.com"),
        ("docker-mcp", "list-containers", ""),
        ("aws-docs", "search_documentation", "lambda"),
    ]

    usage_count = 0
    for server_name, tool_name, resource in tool_usages:
        server_id = mcp_id_map.get(server_name.lower())
        if not server_id:
            continue

        config = get_mcp_server_config(server_name)
        if not config:
            continue

        mcp_url = f"{config.get('command', '')} {' '.join(config.get('args', []))}"

        try:
            console.info(f"Using {server_name}:{tool_name}...")
            response = use_mcp_tool(
                aim_client=agent,
                server_id=server_id,
                tool_name=tool_name,
                mcp_url=mcp_url,
                mcp_name=server_name,
                auto_attest=True,
            )
            usage_count += 1
            console.success(f"  └─ Recorded usage")
        except Exception as e:
            console.warning(f"  └─ Failed: {e}")

    console.success(f"Recorded {usage_count} tool usages")


def register_multiple_agents():
    """Register multiple agents of different types to populate the dashboard."""
    print_header("Multi-Agent Registration")

    # Define agents with different types
    agents_to_create = [
        {
            "name": "claude-code-agent",
            "display_name": "Claude Code Agent",
            "description": "Claude Code CLI instance for development",
            "agent_type": AgentType.CLAUDE,
            "version": "1.0.0",
            "capabilities": ["file:read", "file:write", "code:execute", "git:commit"],
            "tags": ["development", "cli", "primary"],
            "metadata": {"model": "claude-opus-4-5-20251101", "platform": "macos"},
        },
        {
            "name": "langchain-data-processor",
            "display_name": "LangChain Data Processor",
            "description": "LangChain agent for processing and analyzing data",
            "agent_type": AgentType.LANGCHAIN,
            "version": "2.1.0",
            "capabilities": ["data:read", "data:write", "api:call", "db:query"],
            "tags": ["production", "data-processing"],
            "metadata": {"model": "gpt-4-turbo", "framework_version": "0.1.0"},
        },
        {
            "name": "crewai-research-team",
            "display_name": "CrewAI Research Team",
            "description": "Multi-agent crew for autonomous research tasks",
            "agent_type": AgentType.CREWAI,
            "version": "1.5.0",
            "capabilities": ["web:scrape", "file:read", "api:call", "data:analyze"],
            "tags": ["research", "autonomous"],
            "metadata": {"crew_size": "3", "model": "gpt-4"},
        },
        {
            "name": "github-copilot-assistant",
            "display_name": "GitHub Copilot Assistant",
            "description": "Code completion and suggestion assistant",
            "agent_type": AgentType.COPILOT,
            "version": "1.0.0",
            "capabilities": ["code:suggest", "code:complete", "code:explain"],
            "tags": ["ide", "code-assistant"],
            "metadata": {"ide": "vscode", "model": "copilot"},
        },
        {
            "name": "autogpt-task-runner",
            "display_name": "AutoGPT Task Runner",
            "description": "Autonomous agent for complex multi-step tasks",
            "agent_type": AgentType.AUTOGPT,
            "version": "0.5.0",
            "capabilities": ["file:read", "file:write", "web:browse", "code:execute", "api:call"],
            "tags": ["autonomous", "experimental"],
            "metadata": {"model": "gpt-4", "memory_type": "pinecone"},
        },
    ]

    registered_agents = []
    mcp_detections = None

    for agent_config in agents_to_create:
        console.info(f"\nRegistering {agent_config['display_name']}...")

        # Get MCP names from config (only for first agent)
        if mcp_detections is None:
            mcp_detections = detect_mcps()

        mcp_names = [d['mcpServer'] for d in mcp_detections] if mcp_detections else []

        try:
            agent = register_agent(
                name=agent_config["name"],
                aim_url=AIM_URL,
                api_key=API_KEY,
                display_name=agent_config["display_name"],
                description=agent_config["description"],
                agent_type=agent_config["agent_type"],
                version=agent_config["version"],
                mcp_servers=mcp_names,
                capabilities=agent_config["capabilities"],
                tags=agent_config.get("tags"),
                metadata=agent_config.get("metadata"),
                auto_detect=False,
                force_new=False,
            )
            registered_agents.append((agent, agent_config))
            console.success(f"  Registered: {agent.agent_id}")
        except Exception as e:
            console.error(f"  Failed: {e}")

    return registered_agents, mcp_detections


def main():
    """Main entry point."""
    print_header("AIM Agent Setup Tool")
    print(f"AIM URL: {AIM_URL}")
    print()

    try:
        # Step 1: Register multiple agents with different types
        registered_agents, mcp_detections = register_multiple_agents()

        if not registered_agents:
            console.error("No agents were registered")
            sys.exit(1)

        # Step 2: Register and attest MCPs using the first agent
        primary_agent = registered_agents[0][0]
        register_and_attest_mcps(primary_agent, mcp_detections)

        # Step 3: Simulate MCP usage for all agents
        for agent, config in registered_agents:
            console.info(f"\nSimulating usage for {config['display_name']}...")
            simulate_mcp_usage(agent, mcp_detections)

        print_header("Setup Complete!")
        print(f"Registered {len(registered_agents)} agents:")
        for agent, config in registered_agents:
            print(f"  • {config['display_name']} ({config['agent_type']}) - {agent.agent_id}")

        print(f"\nView agents in the dashboard:")
        print(f"  {AIM_URL}/dashboard/agents")
        print(f"\nView MCP supply chain:")
        print(f"  {AIM_URL}/dashboard/mcp/supply-chain")

    except KeyboardInterrupt:
        console.warning("\nSetup cancelled by user")
        sys.exit(1)
    except Exception as e:
        console.error(f"\nSetup failed: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
