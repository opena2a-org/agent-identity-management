'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Code2, Copy, CheckCircle2, Shield, Terminal } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

interface MCPSetupGuideProps {
  mcpServerId: string;
  mcpServerName: string;
  mcpServerUrl: string;
}

export function MCPSetupGuide({ mcpServerId, mcpServerName, mcpServerUrl }: MCPSetupGuideProps) {
  const [copiedQuick, setCopiedQuick] = useState(false);
  const [copiedAdvanced, setCopiedAdvanced] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  const copyToClipboard = (text: string, type: 'quick' | 'advanced' | 'curl') => {
    navigator.clipboard.writeText(text);
    if (type === 'quick') {
      setCopiedQuick(true);
      setTimeout(() => setCopiedQuick(false), 2000);
    } else if (type === 'advanced') {
      setCopiedAdvanced(true);
      setTimeout(() => setCopiedAdvanced(false), 2000);
    } else {
      setCopiedCurl(true);
      setTimeout(() => setCopiedCurl(false), 2000);
    }
  };

  // Backend API URL - dynamic based on deployment
  // In production: same domain (reverse proxy handles routing)
  // In development: localhost:8080
  const apiUrl = typeof window !== 'undefined'
    ? window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
      ? `${window.location.protocol}//${window.location.hostname}:8080`
      : `${window.location.protocol}//${window.location.host}`
    : 'http://localhost:8080';

  // Quick attestation example (Python only - the only SDK available)
  const quickStartCode = `from aim_sdk import secure
agent = secure("my-agent")

# Attest to MCP server
agent.attest_mcp("${mcpServerName}", "${mcpServerUrl}")`;

  // cURL example for verification (READ-only operations)
  const curlExample = `# cURL can READ MCP data, but CANNOT create attestations
# Attestations require Ed25519 signing (use Python SDK instead)

# Use an Agent API Key (Dashboard → API Keys → Create Key)
API_KEY="<your-agent-api-key>"

# Get MCP server details
curl -X GET "${apiUrl}/api/v1/mcp-servers/${mcpServerId}" \\
  -H "Authorization: Bearer $API_KEY"

# List existing attestations for this MCP
curl -X GET "${apiUrl}/api/v1/mcp-servers/${mcpServerId}/attestations" \\
  -H "Authorization: Bearer $API_KEY"`;

  // Advanced Python example
  const advancedCode = `# Full MCP attestation with capability verification
from aim_sdk import AIMClient
import os

# Initialize client with your agent credentials
client = AIMClient(
    api_url="${apiUrl}",
    agent_id="YOUR_AGENT_ID",
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# MCP Server: ${mcpServerName}
# URL: ${mcpServerUrl}

# Create cryptographically-signed attestation
attestation = client.attest_mcp_server(
    mcp_server_id="${mcpServerId}",
    mcp_server_url="${mcpServerUrl}",
    capabilities=["tools", "resources"],  # Verified capabilities
    health_check=True  # Run connectivity check
)

print(f"Attestation created: {attestation['id']}")
print(f"Signature: {attestation['signature'][:32]}...")
print(f"Confidence score: {attestation['confidence_score']}%")`;

  return (
    <div className="space-y-6">
      {/* cURL Verification */}
      <Card className="border border-amber-200 dark:border-amber-800 bg-amber-50/50 dark:bg-amber-950/20">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-2">
            <Terminal className="h-5 w-5 text-amber-600" />
            <CardTitle className="text-lg">Query via cURL (Read-Only)</CardTitle>
          </div>
          <CardDescription>
            View MCP details & attestations — requires an <a href="/dashboard/api-keys" className="text-amber-700 dark:text-amber-400 underline hover:no-underline">Agent API Key</a>. Creating attestations requires the SDK.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <pre className="bg-gray-900 text-green-400 p-4 rounded-lg text-sm font-mono overflow-x-auto">
              <code>{curlExample}</code>
            </pre>
            <Button
              size="sm"
              variant="secondary"
              className="absolute top-2 right-2"
              onClick={() => copyToClipboard(curlExample, 'curl')}
            >
              {copiedCurl ? (
                <>
                  <CheckCircle2 className="h-4 w-4 mr-1 text-green-500" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-1" />
                  Copy
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Hero Section: Attestation */}
      <Card className="border-2 border-primary/20 bg-gradient-to-br from-primary/5 to-transparent">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Shield className="h-6 w-6 text-primary" />
            <CardTitle className="text-2xl">Attest to This MCP Server</CardTitle>
          </div>
          <CardDescription className="text-base">
            Create a cryptographically-signed attestation from your verified agent using the Python SDK
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <div className="absolute top-3 left-3 px-2 py-1 bg-blue-600 text-white text-xs font-medium rounded">
              Python
            </div>
            <pre className="bg-black text-green-400 p-6 pt-10 rounded-lg text-base font-mono overflow-x-auto border-2 border-primary/30">
              <code>{quickStartCode}</code>
            </pre>
            <Button
              size="sm"
              className="absolute top-3 right-3 bg-primary hover:bg-primary/90"
              onClick={() => copyToClipboard(quickStartCode, 'quick')}
            >
              {copiedQuick ? (
                <>
                  <CheckCircle2 className="h-4 w-4 mr-1" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-1" />
                  Copy
                </>
              )}
            </Button>
          </div>

          <div className="mt-4 p-4 bg-green-50 dark:bg-green-950/20 border border-green-200 dark:border-green-800 rounded-lg">
            <p className="text-sm text-green-900 dark:text-green-100 font-medium mb-2">
              What happens when you attest:
            </p>
            <ul className="text-sm text-green-800 dark:text-green-200 list-disc list-inside ml-2 mt-1 space-y-1">
              <li><strong>Ed25519 signature</strong> creates cryptographic proof</li>
              <li><strong>Health check</strong> verifies MCP server connectivity</li>
              <li><strong>Capability detection</strong> discovers tools & resources</li>
              <li><strong>Confidence score</strong> increases with each attestation</li>
              <li><strong>Trust chain</strong> links agent to MCP server</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Advanced Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Code2 className="h-5 w-5 text-primary" />
            <CardTitle>Advanced: Full Attestation Control</CardTitle>
          </div>
          <CardDescription>
            Need more control? Use the full attestation API with capability verification
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <div className="absolute top-2 left-2 px-2 py-1 bg-blue-600 text-white text-xs font-medium rounded">
              Python
            </div>
            <pre className="bg-muted p-4 pt-10 rounded-lg text-sm overflow-x-auto">
              <code>{advancedCode}</code>
            </pre>
            <Button
              size="sm"
              variant="ghost"
              className="absolute top-2 right-2"
              onClick={() => copyToClipboard(advancedCode, 'advanced')}
            >
              {copiedAdvanced ? (
                <>
                  <CheckCircle2 className="h-4 w-4 mr-1 text-green-500" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-1" />
                  Copy
                </>
              )}
            </Button>
          </div>

          <div className="mt-6 p-4 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
            <p className="text-sm text-blue-900 dark:text-blue-100 space-y-2">
              <strong>How MCP Attestation Works:</strong>
              <br />
              1. Your agent connects to this MCP server at <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">{mcpServerUrl}</code>
              <br />
              2. SDK verifies connectivity and discovers capabilities
              <br />
              3. Agent signs attestation with Ed25519 private key
              <br />
              4. AIM verifies signature and updates confidence score
              <br />
              5. Other agents can trust this MCP server based on attestations
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
