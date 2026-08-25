'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Code2, Copy, CheckCircle2, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useClipboard } from '@/hooks/use-clipboard';

interface SDKSetupGuideProps {
  agentId: string;
  agentName: string;
  agentType: string;
}

export function SDKSetupGuide({ agentId, agentName, agentType }: SDKSetupGuideProps) {
  // Use separate clipboard hooks for each copy button to track state independently
  // This hook handles error handling, timeout cleanup, and fallback for older browsers
  const quickClipboard = useClipboard();
  const advancedClipboard = useClipboard();

  // Backend API URL - dynamic based on deployment
  // In production: same domain (reverse proxy handles routing)
  // In development: localhost:8080
  const apiUrl = typeof window !== 'undefined'
    ? window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
      ? `${window.location.protocol}//${window.location.hostname}:8080`
      : `${window.location.protocol}//${window.location.host}`
    : 'http://localhost:8080';

  // Zero-config registration (Python only)
  const quickStartCode = `from aim_sdk import secure
agent = secure("${agentId}")`;

  // Advanced Python example
  const advancedCode = `# pip install aim-sdk

from aim_sdk import AIMClient
import os

# Your Agent: ${agentName} (${agentType})
# Prerequisites: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

# Full control with AIMClient (optional)
client = AIMClient(
    api_url="${apiUrl}",
    agent_id="${agentId}",
    private_key=os.getenv("AIM_PRIVATE_KEY"),
    auto_detect={
        "enabled": True,
        "config_path": "~/.config/claude/mcp_config.json"
    }
)

# Auto-detect and report MCPs
detection = client.detect_mcps()
print(f"[${agentName}] Detected {len(detection['mcps'])} MCPs")

# Verify agent actions with context
verification = client.verify_action(
    action="database_read",
    resource="users_table",
    context={"reason": "Fetching analytics"}
)`;

  return (
    <div className="space-y-6">
      {/* Hero Section */}
      <Card className="border-2 border-primary/20 bg-gradient-to-br from-primary/5 to-transparent">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Zap className="h-6 w-6 text-primary" />
            <CardTitle className="text-2xl">Secure Your Agent</CardTitle>
          </div>
          <CardDescription className="text-base">
            Secure by default with zero-configuration setup using the Python SDK
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
              onClick={() => quickClipboard.copy(quickStartCode)}
            >
              {quickClipboard.copied ? (
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
              That's it! Your agent is now secure.
            </p>
            <p className="text-sm text-green-800 dark:text-green-200">
              Automatically enabled:
            </p>
            <ul className="text-sm text-green-800 dark:text-green-200 list-disc list-inside ml-2 mt-1 space-y-1">
              <li><strong>Ed25519 cryptographic signing</strong> on every request</li>
              <li><strong>Auto-MCP detection</strong> from Claude Desktop config</li>
              <li><strong>Real-time trust scoring</strong> and behavior analytics</li>
              <li><strong>Audit logging</strong> and compliance reporting</li>
              <li><strong>Anomaly detection</strong> and security alerts</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Advanced Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Code2 className="h-5 w-5 text-primary" />
            <CardTitle>Advanced: Full Client Control</CardTitle>
          </div>
          <CardDescription>
            Need more control? Use the full AIMClient for custom configurations
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
              onClick={() => advancedClipboard.copy(advancedCode)}
            >
              {advancedClipboard.copied ? (
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
              <strong>Quick Start:</strong>
              <br />
              1. Create agent in AIM dashboard → Get agent ID and Ed25519 private key
              <br />
              2. Set environment variable: <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">export AIM_PRIVATE_KEY="your-private-key"</code>
              <br />
              3. Add the code snippet to your agent (see above)
              <br />
              4. Done! View real-time security analytics in the AIM dashboard
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
