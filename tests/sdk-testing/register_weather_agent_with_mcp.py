#!/usr/bin/env python3
"""
Real-World Example: Weather Agent with Public MCP Server

This demonstrates registering a weather agent that uses the official
Open-Meteo Weather API MCP Server from the Model Context Protocol repository.

Based on: https://github.com/modelcontextprotocol/servers/tree/main/src/weather
"""

import sys
import os

# Add the Python SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../sample-agent-python/aim-sdk-python'))

from aim_sdk import secure

# Configuration
API_KEY = os.getenv('AIM_API_KEY', 'aim_live_rVqDQwzk-p9KRfxZZM3ocnidPcSqf9yhxVZkjs_CVXc=')
AIM_URL = os.getenv('AIM_API_URL', 'http://localhost:8080')

def main():
    print("🌤️  Weather Agent with MCP Server Registration")
    print("=" * 70)
    print("")
    
    # Register the weather agent
    print("1️⃣  Registering weather agent...")
    
    agent = secure(
        "weather-agent",
        aim_url=AIM_URL,
        api_key=API_KEY
    )
    
    print(f"   ✅ Agent registered: {agent.agent_id}")
    print(f"   📝 Agent name: weather-agent")
    print("")
    
    # Report MCP detection
    print("2️⃣  Reporting MCP Server usage...")
    print("")
    
    # The agent uses the official weather MCP server
    mcp_data = {
        "name": "weather-mcp-server",
        "description": "Open-Meteo Weather API MCP Server - provides real-time weather data",
        "url": "https://github.com/modelcontextprotocol/servers/tree/main/src/weather",
        "version": "1.0.0",
        "public_key": "weather-mcp-public-key-placeholder",  # Would be actual public key
        "capabilities": ["tools", "resources"],  # MCP capabilities: tools = functions, resources = data
        "verification_url": "https://github.com/modelcontextprotocol/servers"
    }
    
    # Report to AIM using detection endpoint
    import requests
    from datetime import datetime
    
    detection_payload = {
        "detections": [
            {
                "mcpServer": mcp_data["name"],
                "detectionMethod": "config_file",
                "confidence": 100,
                "sdkVersion": "1.0.0",
                "timestamp": datetime.utcnow().isoformat() + 'Z',
                "details": {
                    "url": mcp_data["url"],
                    "description": mcp_data["description"],
                    "verification_url": mcp_data["verification_url"]
                }
            }
        ]
    }
    
    response = requests.post(
        f"{AIM_URL}/api/v1/sdk-api/agents/{agent.agent_id}/detection/report",
        json=detection_payload,
        headers={"X-API-Key": API_KEY}
    )
    
    if response.status_code == 200:
        result = response.json()
        print(f"   ✅ MCP detection reported successfully")
        print(f"   📊 Detections processed: {result.get('detectionsProcessed', 0)}")
    else:
        print(f"   ❌ Failed to report detection: {response.status_code}")
        print(f"   {response.text}")
    
    print("")
    print("=" * 70)
    print("✅ Weather Agent Setup Complete!")
    print("")
    print("📝 Next Steps:")
    print("")
    print("1. View agent in dashboard:")
    print(f"   http://localhost:3000/dashboard/agents/{agent.agent_id}")
    print("")
    print("2. See detected MCP:")
    print("   → Agent Details → MCPs Tab (should show 1 MCP)")
    print("")
    print("3. Register MCP globally in dashboard:")
    print("   → Go to http://localhost:3000/dashboard/mcp")
    print("   → Click 'Register MCP Server'")
    print("   → Fill in:")
    print(f"      Name: {mcp_data['name']}")
    print(f"      URL: {mcp_data['url']}")
    print(f"      Description: {mcp_data['description']}")
    print(f"      Capabilities: {', '.join(mcp_data['capabilities'])}")
    print("")
    print("4. Compare:")
    print("   • Agent page: Shows agent uses weather-mcp-server")
    print("   • Sidebar: Shows weather-mcp-server registered globally")
    print("")

if __name__ == '__main__':
    main()

