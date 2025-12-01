'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Code2, Copy, CheckCircle2, Zap, Shield } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

interface SDKSetupGuideProps {
  agentId: string;
  agentName: string;
  agentType: string;
}

export function SDKSetupGuide({ agentId, agentName, agentType }: SDKSetupGuideProps) {
  const [copiedLang, setCopiedLang] = useState<string | null>(null);
  const [copiedQuick, setCopiedQuick] = useState<string | null>(null);

  const copyToClipboard = (text: string, lang: string, isQuick = false) => {
    navigator.clipboard.writeText(text);
    if (isQuick) {
      setCopiedQuick(lang);
      setTimeout(() => setCopiedQuick(null), 2000);
    } else {
      setCopiedLang(lang);
      setTimeout(() => setCopiedLang(null), 2000);
    }
  };

  // Backend API URL - port 8080, not frontend port 3000
  const apiUrl = typeof window !== 'undefined'
    ? `${window.location.protocol}//${window.location.hostname}:8080`
    : 'http://localhost:8080';

  // Zero-config registration: Register your agent with 1 line of code
  const quickStart = {
    javascript: `import { secure } from '@aim/sdk';\nconst agent = secure({ name: '${agentId}' });`,
    python: `from aim_sdk import secure\nagent = secure("${agentId}")`,
    go: `import aimsdk "github.com/opena2a/aim-sdk-go"\nclient := aimsdk.NewClient()\nreg, _ := client.Secure(ctx, aimsdk.SecureOptions{Name: "${agentId}"})`
  };

  const examples = {
    javascript: `// NO npm package available - Download SDK from AIM Dashboard
// Go to Settings → SDK Download → Download JavaScript SDK

import { AIMClient } from '@aim/sdk';

// Your Agent: ${agentName} (${agentType})
// Prerequisites: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

// Full control with AIMClient (optional)
const client = new AIMClient({
  apiUrl: '${apiUrl}',
  agentId: '${agentId}',
  privateKey: process.env.AIM_PRIVATE_KEY,
  autoDetect: {
    enabled: true,
    configPath: '~/.config/claude/mcp_config.json'
  }
});

// Auto-detect and report MCPs
const detection = await client.detectMCPs();
console.log(\`[${agentName}] Detected \${detection.mcps.length} MCPs\`);

// Verify agent actions with context
const verification = await client.verifyAction({
  action: 'read_file',
  resource: '/path/to/file.txt',
  context: { reason: 'Reading user data' }
});`,

    python: `# NO pip package available - Download SDK from AIM Dashboard
# Go to Settings → SDK Download → Download Python SDK
# Install dependencies: pip install keyring PyNaCl requests cryptography

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
)`,

    go: `go get github.com/opena2a/aim-sdk-go

import (
    "context"
    "fmt"
    "os"
    aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
    ctx := context.Background()

    // Your Agent: ${agentName} (${agentType})
    // Prerequisites: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

    // Full control with NewClient (optional)
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:     "${apiUrl}",
        AgentID:    "${agentId}",
        PrivateKey: os.Getenv("AIM_PRIVATE_KEY"),
        AutoDetect: aimsdk.AutoDetectConfig{
            Enabled:    true,
            ConfigPath: "~/.config/claude/mcp_config.json",
        },
    })

    // Auto-detect and report MCPs
    detection, _ := client.DetectMCPs(ctx)
    fmt.Printf("[${agentName}] Detected %d MCPs\\n", len(detection.MCPs))

    // Verify agent actions with context
    verification, _ := client.VerifyAction(ctx, aimsdk.ActionRequest{
        Action:   "api_call",
        Resource: "external-api.com/endpoint",
        Context:  map[string]interface{}{"reason": "Fetching data"},
    })
}`,
  };

  return (
    <div className="space-y-6">
      {/* Hero Section: 1 Line of Code */}
      <Card className="border-2 border-primary/20 bg-gradient-to-br from-primary/5 to-transparent">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Zap className="h-6 w-6 text-primary" />
            <CardTitle className="text-2xl">Secure Your Agent with 1 Line of Code</CardTitle>
          </div>
          <CardDescription className="text-base">
            Secure by default with zero-configuration setup
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="javascript" className="w-full">
            <TabsList className="grid w-full grid-cols-3 mb-4">
              <TabsTrigger value="javascript">JavaScript</TabsTrigger>
              <TabsTrigger value="python">Python</TabsTrigger>
              <TabsTrigger value="go">Go</TabsTrigger>
            </TabsList>

            {Object.entries(quickStart).map(([lang, code]) => (
              <TabsContent key={lang} value={lang} className="space-y-4">
                <div className="relative">
                  <pre className="bg-black text-green-400 p-6 rounded-lg text-base font-mono overflow-x-auto border-2 border-primary/30">
                    <code>{code}</code>
                  </pre>
                  <Button
                    size="sm"
                    className="absolute top-3 right-3 bg-primary hover:bg-primary/90"
                    onClick={() => copyToClipboard(code, lang, true)}
                  >
                    {copiedQuick === lang ? (
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

                <div className="p-4 bg-green-50 dark:bg-green-950/20 border border-green-200 dark:border-green-800 rounded-lg">
                  <p className="text-sm text-green-900 dark:text-green-100 font-medium mb-2">
                    ✨ That's it! Your agent is now secure.
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
              </TabsContent>
            ))}
          </Tabs>
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
          <Tabs defaultValue="javascript" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="javascript">JavaScript</TabsTrigger>
              <TabsTrigger value="python">Python</TabsTrigger>
              <TabsTrigger value="go">Go</TabsTrigger>
            </TabsList>

            {Object.entries(examples).map(([lang, code]) => (
              <TabsContent key={lang} value={lang} className="space-y-4">
                <div className="relative">
                  <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
                    <code>{code}</code>
                  </pre>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="absolute top-2 right-2"
                    onClick={() => copyToClipboard(code, lang)}
                  >
                    {copiedLang === lang ? (
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
              </TabsContent>
            ))}
          </Tabs>

          <div className="mt-6 p-4 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
            <p className="text-sm text-blue-900 dark:text-blue-100 space-y-2">
              <strong>💡 Quick Start:</strong>
              <br />
              1. Create agent in AIM dashboard → Get agent ID and Ed25519 private key
              <br />
              2. Set environment variable: <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">export AIM_PRIVATE_KEY="your-private-key"</code>
              <br />
              3. Add 1 line of code to your agent (see above)
              <br />
              4. Done! View real-time security analytics in the AIM dashboard
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
