#!/usr/bin/env python3
"""
Test MCP Server with Cryptographic Verification
================================================

A production-ready MCP (Model Context Protocol) server that implements:
- Ed25519 cryptographic verification
- Standard MCP protocol endpoints
- Auto-detectable capabilities (tools, resources, prompts)
- Challenge-response authentication

This server is perfect for:
- Testing AIM's MCP verification flow
- Demonstrating cryptographic trust
- Learning MCP protocol implementation
- Development and testing

Features:
- ✅ Ed25519 cryptographic signing
- ✅ Standard /.well-known/mcp/capabilities endpoint
- ✅ 3 tools (echo, calculate, timestamp)
- ✅ 2 resources (server status, config)
- ✅ 1 prompt (greeting)
- ✅ Automatic capability detection by AIM

Usage:
    python3 test-mcp-server.py

The server will display its public key. Use this to register
the MCP server in the AIM dashboard.

Registration in AIM Dashboard:
    1. Go to: http://localhost:3000/dashboard/mcp
    2. Click "Register MCP Server"
    3. Name: test-mcp-local
    4. URL: http://localhost:5555
    5. Public Key: (copy from server output)
    6. Click "Save" then "Verify"
    7. Capabilities will auto-detect!

Security:
    This server generates a NEW Ed25519 key pair on each restart.
    You must update the public key in AIM dashboard after restart.
"""

from flask import Flask, request, jsonify
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder
import json
import sys

app = Flask(__name__)

# Generate Ed25519 key pair for this MCP server
signing_key = SigningKey.generate()
verify_key = signing_key.verify_key

# Store these for registration
PUBLIC_KEY = verify_key.encode(encoder=Base64Encoder).decode('utf-8')
PRIVATE_KEY = signing_key.encode(encoder=Base64Encoder).decode('utf-8')

print("=" * 70)
print("🔐 TEST MCP SERVER - Cryptographic Verification Enabled")
print("=" * 70)
print("")
print("📋 Server Details:")
print(f"   URL: http://localhost:5151/mcp")
print(f"   Verification Endpoint: http://localhost:5151/mcp/.well-known/mcp/verify")
print(f"   Capabilities Endpoint: http://localhost:5151/mcp/capabilities")
print("")
print("🔑 Cryptographic Keys (Ed25519):")
print(f"   Public Key:  {PUBLIC_KEY}")
print(f"   Private Key: {PRIVATE_KEY[:20]}... (saved)")
print("")
print("🛠️  Auto-Detectable Capabilities:")
print("   Tools:")
print("     • echo - Echo back any input text")
print("     • calculate - Perform basic math calculations")
print("     • timestamp - Get current server timestamp")
print("   Resources:")
print("     • server://status - Server health and statistics")
print("     • server://config - Server configuration")
print("   Prompts:")
print("     • greeting - Generate friendly greeting messages")
print("")
print("=" * 70)
print("")
print("✅ REGISTER IN AIM DASHBOARD:")
print("")
print("   Name: test-mcp-local")
print("   URL: http://localhost:5151/mcp")
print(f"   Public Key: {PUBLIC_KEY}")
print("   Description: Local test MCP server with crypto verification")
print("")
print("Then click 'Verify' - capabilities will be auto-detected!")
print("")
print("=" * 70)
print("")


@app.route('/mcp/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        'status': 'healthy',
        'server': 'test-mcp-local',
        'verification_supported': True,
        'public_key': PUBLIC_KEY
    })


@app.route('/mcp/.well-known/mcp/verify', methods=['POST'])
def verify():
    """
    Cryptographic verification endpoint
    
    AIM sends:
        POST /.well-known/mcp/verify
        { "challenge": "random-base64-string", "server_id": "uuid" }
    
    MCP responds:
        { "signed_challenge": "base64-signature" }
    
    AIM verifies signature using public key
    """
    try:
        data = request.get_json()
        challenge = data.get('challenge')
        server_id = data.get('server_id')
        
        if not challenge:
            return jsonify({'error': 'challenge required'}), 400
        
        print(f"📨 Received verification challenge:")
        print(f"   Challenge (base64): {challenge}")
        print(f"   Challenge length: {len(challenge)}")
        print(f"   Server ID: {server_id}")
        print("")
        
        # Sign the challenge with our private key
        # The challenge is a base64-encoded string that we need to sign AS IS (as bytes)
        challenge_bytes = challenge.encode('utf-8')
        
        print(f"🔐 Signing challenge...")
        print(f"   Challenge bytes length: {len(challenge_bytes)}")
        print(f"   Public key: {PUBLIC_KEY}")
        print("")
        
        # Use PyNaCl to sign
        signed = signing_key.sign(challenge_bytes)
        # Extract only the 64-byte signature (first 64 bytes)
        signature_raw = signed[:64]
        
        # Base64 encode for transmission
        import base64
        signature = base64.b64encode(signature_raw).decode('utf-8')
        
        print(f"✅ Signature generated:")
        print(f"   Signature (base64): {signature}")
        print(f"   Signature length: {len(signature)}")
        print("")
        print("📤 Sending signed challenge back to AIM")
        print("")
        sys.stdout.flush()
        
        return jsonify({
            'signed_challenge': signature,
            'public_key': PUBLIC_KEY,
            'algorithm': 'ed25519'
        })
        
    except Exception as e:
        print(f"❌ Error: {e}")
        return jsonify({'error': str(e)}), 500


def get_capabilities_response():
    """Generate capabilities response - reusable for multiple endpoints"""
    return {
        'tools': [
            {
                'name': 'echo',
                'description': 'Echo back any input text',
                'inputSchema': {
                    'type': 'object',
                    'properties': {
                        'message': {
                            'type': 'string',
                            'description': 'The message to echo back'
                        }
                    },
                    'required': ['message']
                }
            },
            {
                'name': 'calculate',
                'description': 'Perform basic mathematical calculations',
                'inputSchema': {
                    'type': 'object',
                    'properties': {
                        'expression': {
                            'type': 'string',
                            'description': 'Mathematical expression to evaluate (e.g., "2 + 2")'
                        }
                    },
                    'required': ['expression']
                }
            },
            {
                'name': 'timestamp',
                'description': 'Get current server timestamp',
                'inputSchema': {
                    'type': 'object',
                    'properties': {}
                }
            }
        ],
        'resources': [
            {
                'uri': 'server://status',
                'name': 'Server Status',
                'description': 'Current server health and statistics',
                'mimeType': 'application/json'
            },
            {
                'uri': 'server://config',
                'name': 'Server Configuration',
                'description': 'Server configuration details',
                'mimeType': 'application/json'
            }
        ],
        'prompts': [
            {
                'name': 'greeting',
                'description': 'Generate a friendly greeting message',
                'arguments': [
                    {
                        'name': 'name',
                        'description': 'Name of the person to greet',
                        'required': False
                    }
                ]
            }
        ],
        'serverInfo': {
            'name': 'test-mcp-local',
            'version': '1.0.0',
            'protocolVersion': '2024-11-05'
        }
    }


@app.route('/.well-known/mcp/capabilities', methods=['GET'])
def capabilities_well_known():
    """Standard MCP protocol capabilities endpoint - Auto-detectable by AIM"""
    print("📡 Capabilities endpoint accessed (/.well-known/mcp/capabilities)")
    sys.stdout.flush()
    return jsonify(get_capabilities_response())


@app.route('/mcp/tools/echo', methods=['POST'])
def tool_echo():
    """Echo tool implementation"""
    data = request.get_json()
    message = data.get('message', '')
    print(f"📢 Echo tool called: {message}")
    sys.stdout.flush()
    return jsonify({
        'content': [
            {
                'type': 'text',
                'text': f'Echo: {message}'
            }
        ]
    })


@app.route('/mcp/tools/calculate', methods=['POST'])
def tool_calculate():
    """Calculate tool implementation"""
    data = request.get_json()
    expression = data.get('expression', '')
    print(f"🔢 Calculate tool called: {expression}")
    sys.stdout.flush()
    
    try:
        # Safe evaluation (only allow basic math)
        result = eval(expression, {"__builtins__": {}}, {})
        return jsonify({
            'content': [
                {
                    'type': 'text',
                    'text': f'{expression} = {result}'
                }
            ]
        })
    except Exception as e:
        return jsonify({
            'content': [
                {
                    'type': 'text',
                    'text': f'Error: {str(e)}'
                }
            ],
            'isError': True
        }), 400


@app.route('/mcp/tools/timestamp', methods=['POST'])
def tool_timestamp():
    """Timestamp tool implementation"""
    from datetime import datetime
    print(f"⏰ Timestamp tool called")
    sys.stdout.flush()
    
    now = datetime.utcnow()
    return jsonify({
        'content': [
            {
                'type': 'text',
                'text': f'Current UTC timestamp: {now.isoformat()}Z'
            }
        ]
    })


@app.route('/mcp/resources/server/status', methods=['GET'])
def resource_status():
    """Server status resource"""
    from datetime import datetime
    print(f"📊 Status resource accessed")
    sys.stdout.flush()
    
    return jsonify({
        'contents': [
            {
                'uri': 'server://status',
                'mimeType': 'application/json',
                'text': json.dumps({
                    'status': 'healthy',
                    'uptime': 'running',
                    'timestamp': datetime.utcnow().isoformat() + 'Z',
                    'requests_handled': 42
                })
            }
        ]
    })


@app.route('/mcp/prompts/greeting', methods=['GET'])
def prompt_greeting():
    """Greeting prompt"""
    name = request.args.get('name', 'friend')
    print(f"👋 Greeting prompt accessed for: {name}")
    sys.stdout.flush()
    
    return jsonify({
        'messages': [
            {
                'role': 'user',
                'content': {
                    'type': 'text',
                    'text': f'Generate a warm and friendly greeting for {name}'
                }
            }
        ]
    })


@app.route('/mcp/echo', methods=['POST'])
def echo():
    """Legacy echo endpoint (deprecated, use /mcp/tools/echo)"""
    data = request.get_json()
    print(f"📢 Legacy echo request: {data}")
    sys.stdout.flush()
    return jsonify({
        'echo': data,
        'server': 'test-mcp-local',
        'note': 'This endpoint is deprecated. Use /mcp/tools/echo instead.'
    })


if __name__ == '__main__':
    print("🚀 Starting test MCP server on http://localhost:5151")
    print("")
    app.run(host='0.0.0.0', port=5151, debug=False)

