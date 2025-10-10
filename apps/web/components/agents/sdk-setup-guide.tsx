'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Code2, Copy, CheckCircle2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

interface SDKSetupGuideProps {
  agentId: string;
  apiKey: string;
}

export function SDKSetupGuide({ agentId, apiKey }: SDKSetupGuideProps) {
  const [copiedLang, setCopiedLang] = useState<string | null>(null);

  const copyToClipboard = (text: string, lang: string) => {
    navigator.clipboard.writeText(text);
    setCopiedLang(lang);
    setTimeout(() => setCopiedLang(null), 2000);
  };

  const apiUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000';

  const examples = {
    javascript: `npm install @aim/sdk

import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: '${apiUrl}',
  apiKey: '${apiKey}',
  agentId: '${agentId}',
  autoDetect: true  // Enable auto-detection
});

// That's it! SDK will auto-detect MCP usage`,

    python: `pip install aim-sdk

from aim_sdk import register_agent

# ONE LINE - Zero configuration!
agent = register_agent(
    "${agentId.split('-')[0]}-agent",
    api_key="${apiKey}",
    aim_url="${apiUrl}"
)

# Auto-detects capabilities + MCPs automatically`,

    go: `go get github.com/opena2a/aim-sdk-go

import aimsdk "github.com/opena2a/aim-sdk-go"

func main() {
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:  "${apiUrl}",
        APIKey:  "${apiKey}",
        AgentID: "${agentId}",
    })
    defer client.Close()

    // Manually report MCP usage
    client.ReportMCP(ctx, "filesystem")
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
                <p className="font-medium">What happens automatically:</p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li>Detects MCP server usage from imports</li>
                  <li>Reports to AIM API every 10 seconds</li>
                  <li>Updates dashboard in real-time</li>
                  <li>Zero performance impact (&lt;0.1% CPU)</li>
                </ul>
              </div>
            </TabsContent>
          ))}
        </Tabs>

        <div className="mt-6 p-4 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
          <p className="text-sm text-blue-900 dark:text-blue-100">
            <strong>💡 Pro Tip:</strong> The SDK works automatically - just install it and run your agent.
            Check this dashboard to see detected MCPs appear in real-time!
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
