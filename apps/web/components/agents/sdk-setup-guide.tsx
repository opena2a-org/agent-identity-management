'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Code2, Copy, CheckCircle2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

interface SDKSetupGuideProps {
  agentId: string;
  agentName: string;
  agentType: string;
}

export function SDKSetupGuide({ agentId, agentName, agentType }: SDKSetupGuideProps) {
  const [copiedLang, setCopiedLang] = useState<string | null>(null);

  const copyToClipboard = (text: string, lang: string) => {
    navigator.clipboard.writeText(text);
    setCopiedLang(lang);
    setTimeout(() => setCopiedLang(null), 2000);
  };

  // Backend API URL - port 8080, not frontend port 3000
  const apiUrl = typeof window !== 'undefined'
    ? `${window.location.protocol}//${window.location.hostname}:8080`
    : 'http://localhost:8080';

  const examples = {
    javascript: `npm install @aim/sdk

import { AIMClient } from '@aim/sdk';

// Your Agent: ${agentName} (${agentType})
// Prerequisites: Get Ed25519 private key from agent creation success page
// Set environment variable: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

// Initialize client with Ed25519 authentication
const client = new AIMClient({
  apiUrl: '${apiUrl}',
  agentId: '${agentId}',  // Your agent ID (pre-filled)
  privateKey: process.env.AIM_PRIVATE_KEY,  // Ed25519 private key (64 hex chars)
  autoDetect: {
    enabled: true,
    configPath: '~/.config/claude/mcp_config.json'  // Claude Desktop MCP config
  }
});

// Auto-detect and report MCPs from Claude Desktop config
const detection = await client.detectMCPs();
console.log(\`Detected \${detection.mcps.length} MCPs\`);

// Verify agent action (with Ed25519 signature)
const verification = await client.verifyAction({
  action: 'read_file',
  resource: '/path/to/file.txt',
  context: { reason: 'Reading user data for ${agentName}' }
});

// ✅ Ed25519 signing for all requests
// ✅ Auto-MCP detection from Claude config
// ✅ Real-time reporting to dashboard`,

    python: `pip install aim-sdk

from aim_sdk import AIMClient
import os

# Your Agent: ${agentName} (${agentType})
# Prerequisites: Get Ed25519 private key from agent creation success page
# Set environment variable: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

# Initialize client with Ed25519 authentication
client = AIMClient(
    api_url="${apiUrl}",
    agent_id="${agentId}",  # Your agent ID (pre-filled)
    private_key=os.getenv("AIM_PRIVATE_KEY"),  # Ed25519 private key (64 hex chars)
    auto_detect={
        "enabled": True,
        "config_path": "~/.config/claude/mcp_config.json"  # Claude Desktop MCP config
    }
)

# Auto-detect and report MCPs from Claude Desktop config
detection = client.detect_mcps()
print(f"[${agentName}] Detected {len(detection['mcps'])} MCPs")

# Verify agent action (with Ed25519 signature)
verification = client.verify_action(
    action="database_read",
    resource="users_table",
    context={"reason": "Fetching user analytics for ${agentName}"}
)

# ✅ Ed25519 signing for all requests
# ✅ Auto-MCP detection from Claude config
# ✅ Real-time reporting to dashboard`,

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
    // Prerequisites: Get Ed25519 private key from agent creation success page
    // Set environment variable: export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

    // Initialize client with Ed25519 authentication
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:     "${apiUrl}",
        AgentID:    "${agentId}",  // Your agent ID (pre-filled)
        PrivateKey: os.Getenv("AIM_PRIVATE_KEY"),  // Ed25519 private key (64 hex chars)
        AutoDetect: aimsdk.AutoDetectConfig{
            Enabled:    true,
            ConfigPath: "~/.config/claude/mcp_config.json",  // Claude Desktop MCP config
        },
    })

    // Auto-detect and report MCPs from Claude Desktop config
    detection, _ := client.DetectMCPs(ctx)
    fmt.Printf("[${agentName}] Detected %d MCPs\\n", len(detection.MCPs))

    // Verify agent action (with Ed25519 signature)
    verification, _ := client.VerifyAction(ctx, aimsdk.ActionRequest{
        Action:   "api_call",
        Resource: "external-api.com/endpoint",
        Context:  map[string]interface{}{"reason": "Fetching external data for ${agentName}"},
    })

    // ✅ Ed25519 signing for all requests
    // ✅ Auto-MCP detection from Claude config
    // ✅ Real-time reporting to dashboard
}`,
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Code2 className="h-5 w-5 text-primary" />
          <CardTitle>Auto-Detect MCPs with AIM SDK</CardTitle>
        </div>
        <CardDescription>
          Install the SDK in your agent to automatically detect and report MCP usage
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

              <div className="text-sm text-muted-foreground space-y-1">
                <p className="font-medium">✅ All SDKs include:</p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li><strong>Ed25519 Authentication:</strong> All requests cryptographically signed with private key</li>
                  <li><strong>Auto-MCP Detection:</strong> Detects MCPs from Claude Desktop config (~/.config/claude/mcp_config.json)</li>
                  <li><strong>Real-time Reporting:</strong> Automatically reports detected MCPs to AIM dashboard</li>
                  <li><strong>Action Verification:</strong> Verify agent actions with context and audit trail</li>
                  <li><strong>Capability Detection:</strong> Auto-detect agent capabilities from code imports</li>
                  <li><strong>Trust Score Integration:</strong> Automatic trust score calculation based on behavior</li>
                </ul>
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
            3. Install SDK and initialize with agent ID + private key
            <br />
            4. MCPs auto-detected from Claude Desktop config and reported to dashboard in real-time!
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
