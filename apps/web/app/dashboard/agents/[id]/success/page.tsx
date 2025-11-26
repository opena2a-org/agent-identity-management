'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { CheckCircle, Download, Copy, Check, ArrowRight, Book, Github } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import { AuthGuard } from '@/components/auth-guard';

interface Agent {
  id: string;
  name: string;
  displayName: string;
  description: string;
  publicKey?: string;
  agentType: string;
  status: string;
  createdAt: string;
}

export default function AgentSuccessPage() {
  const params = useParams();
  const router = useRouter();
  const agentId = params.id as string;

  const [agent, setAgent] = useState<Agent | null>(null);
  const [loading, setLoading] = useState(true);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [downloadingSDK, setDownloadingSDK] = useState<string | null>(null);

  useEffect(() => {
    const fetchAgent = async () => {
      try {
        const data = await api.getAgent(agentId);
        setAgent(data);
      } catch (error) {
        console.error('Failed to fetch agent:', error);
      } finally {
        setLoading(false);
      }
    };

    if (agentId) {
      fetchAgent();
    }
  }, [agentId]);

  const copyToClipboard = async (text: string, field: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedField(field);
      setTimeout(() => setCopiedField(null), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  };

  const downloadSDK = async (language: 'python' | 'nodejs' | 'go') => {
    setDownloadingSDK(language);
    try {
      // Get auth token from API client
      const token = api.getToken();
      if (!token) {
        throw new Error('Not authenticated');
      }

      // Create download URL
      const baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
      const url = `${baseURL}/api/v1/agents/${agentId}/sdk?lang=${language}`;

      // Fetch the SDK file
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Download failed: ${response.statusText}`);
      }

      // Get filename from Content-Disposition header or use default
      const contentDisposition = response.headers.get('Content-Disposition');
      let filename = `aim-sdk-${agent?.name}-${language}.zip`;
      if (contentDisposition) {
        const matches = /filename=([^;]+)/.exec(contentDisposition);
        if (matches && matches[1]) {
          filename = matches[1].replace(/['"]/g, '');
        }
      }

      // Download the file
      const blob = await response.blob();
      const downloadUrl = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(downloadUrl);

    } catch (error) {
      console.error('Failed to download SDK:', error);
      alert(`Failed to download ${language.toUpperCase()} SDK. Please try again.`);
    } finally {
      setDownloadingSDK(null);
    }
  };

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto mt-12">
        <div className="flex items-center justify-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
        </div>
      </div>
    );
  }

  if (!agent) {
    return (
      <div className="max-w-4xl mx-auto mt-12">
        <Card>
          <CardContent className="pt-6">
            <p className="text-center text-gray-600">Agent not found</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <AuthGuard>
      <div className="max-w-4xl mx-auto space-y-6 pb-12">
      {/* Success Header */}
      <div className="text-center pt-8 pb-4">
        <div className="flex justify-center mb-4">
          <div className="bg-green-100 p-4 rounded-full">
            <CheckCircle className="h-16 w-16 text-green-600" />
          </div>
        </div>
        <h1 className="text-3xl font-bold text-gray-900 mb-2">
          Agent Registered Successfully!
        </h1>
        <p className="text-gray-600 max-w-2xl mx-auto">
          Your agent <span className="font-semibold">{agent.displayName}</span> has been registered with AIM.
          Download the SDK to start building with automatic identity verification.
        </p>
      </div>

      {/* Agent Details Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CheckCircle className="h-5 w-5 text-green-600" />
            Agent Details
          </CardTitle>
          <CardDescription>Your agent has been created with these credentials</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Agent ID */}
          <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
            <div className="flex-1">
              <p className="text-sm font-medium text-gray-700">Agent ID</p>
              <p className="text-sm text-gray-600 font-mono break-all">{agent.id}</p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => copyToClipboard(agent.id, 'agent_id')}
            >
              {copiedField === 'agent_id' ? (
                <Check className="h-4 w-4 text-green-600" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>

          {/* Agent Name */}
          <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
            <div className="flex-1">
              <p className="text-sm font-medium text-gray-700">Agent Name</p>
              <p className="text-sm text-gray-600">{agent.name}</p>
            </div>
          </div>

          {/* Public Key */}
          {agent.publicKey && (
            <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
              <div className="flex-1">
                <p className="text-sm font-medium text-gray-700">Public Key (Ed25519)</p>
                <p className="text-sm text-gray-600 font-mono break-all truncate max-w-[500px]">
                  {agent.publicKey}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => copyToClipboard(agent.publicKey!, 'public_key')}
              >
                {copiedField === 'public_key' ? (
                  <Check className="h-4 w-4 text-green-600" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
          )}

          {/* Status */}
          <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
            <div className="flex-1">
              <p className="text-sm font-medium text-gray-700">Status</p>
              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                {agent.status}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* SDK Download Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Download className="h-5 w-5" />
            Download SDK
          </CardTitle>
          <CardDescription>
            Get started with the pre-configured SDK for your preferred language
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 gap-4">
            {/* Python SDK - Production Ready */}
            <div className="border-2 border-blue-500 rounded-lg p-6 bg-gradient-to-br from-blue-50 to-white">
              <div className="flex flex-col h-full">
                <div className="mb-4">
                  <div className="flex items-center gap-3 mb-3">
                    <div className="h-12 w-12 bg-blue-100 rounded-lg flex items-center justify-center">
                      <Download className="h-6 w-6 text-blue-600" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-xl mb-1">Python SDK</h3>
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                        Production Ready
                      </span>
                    </div>
                  </div>
                  <p className="text-sm text-gray-700 mb-3">
                    Official production-ready SDK with Ed25519 cryptographic signing, OAuth integration,
                    automatic MCP detection, and secure keyring storage.
                  </p>
                  <div className="space-y-2 text-sm text-gray-600">
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600" />
                      <span>Ed25519 cryptographic signing</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600" />
                      <span>OAuth/OIDC integration (Google, Microsoft, Okta)</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600" />
                      <span>Automatic MCP detection</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600" />
                      <span>Secure keyring credential storage</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-green-600" />
                      <span>100% test coverage</span>
                    </div>
                  </div>
                </div>
                <div className="mt-auto">
                  <Button
                    className="w-full bg-blue-600 hover:bg-blue-700"
                    onClick={() => downloadSDK('python')}
                    disabled={downloadingSDK !== null}
                  >
                    {downloadingSDK === 'python' ? (
                      <>
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                        Downloading...
                      </>
                    ) : (
                      <>
                        <Download className="h-4 w-4 mr-2" />
                        Download Python SDK
                      </>
                    )}
                  </Button>
                </div>
              </div>
            </div>

            {/* Future SDKs Note */}
            <div className="border border-gray-200 rounded-lg p-4 bg-gray-50">
              <p className="text-sm text-gray-600 mb-2">
                <strong>Future Releases:</strong> Go and JavaScript/TypeScript SDKs are planned for Q1-Q2 2026.
              </p>
              <p className="text-xs text-gray-500">
                The Python SDK provides complete feature parity and is production-ready for all use cases.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Quick Start Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Book className="h-5 w-5" />
            Quick Start Guide
          </CardTitle>
          <CardDescription>Get up and running in 3 steps</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {/* Step 1 */}
            <div className="flex gap-4">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center font-semibold">
                1
              </div>
              <div>
                <h4 className="font-semibold mb-1">Download and Extract SDK</h4>
                <p className="text-sm text-gray-600">
                  Download the Python SDK above and extract the ZIP file to your project directory
                </p>
                <pre className="mt-2 p-3 bg-gray-50 rounded text-xs overflow-x-auto">
                  <code>unzip aim-sdk-{agent.name}-python.zip</code>
                </pre>
              </div>
            </div>

            {/* Step 2 */}
            <div className="flex gap-4">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center font-semibold">
                2
              </div>
              <div>
                <h4 className="font-semibold mb-1">Install SDK</h4>
                <p className="text-sm text-gray-600">
                  Install the SDK and its dependencies
                </p>
                <pre className="mt-2 p-3 bg-gray-50 rounded text-xs overflow-x-auto">
                  <code>pip install -e .</code>
                </pre>
              </div>
            </div>

            {/* Step 3 */}
            <div className="flex gap-4">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center font-semibold">
                3
              </div>
              <div>
                <h4 className="font-semibold mb-1">Run Example</h4>
                <p className="text-sm text-gray-600">
                  Test the automatic verification with the included example
                </p>
                <pre className="mt-2 p-3 bg-gray-50 rounded text-xs overflow-x-auto">
                  <code>python example.py</code>
                </pre>
              </div>
            </div>
          </div>

          {/* Example Code */}
          <div className="mt-6">
            <h4 className="font-semibold mb-2">Example Usage</h4>
            <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg text-xs overflow-x-auto">
              <code>{`from aim_sdk import AIMClient
from aim_sdk.config import AGENT_ID, PUBLIC_KEY, PRIVATE_KEY, AIM_URL

# Initialize client with embedded credentials
client = AIMClient(
    agent_id=AGENT_ID,
    public_key=PUBLIC_KEY,
    private_key=PRIVATE_KEY,
    aim_url=AIM_URL
)

# Automatic verification with decorator
@client.perform_action("read_database", resource="users_table")
def get_users():
    # Your agent code here
    return database.query("SELECT * FROM users")

# Just call the function - verification happens automatically!
users = get_users()`}</code>
            </pre>
          </div>
        </CardContent>
      </Card>

      {/* Security Notice */}
      <div className="bg-yellow-50 border-l-4 border-yellow-400 p-4 rounded">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg className="h-5 w-5 text-yellow-400" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
            </svg>
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-yellow-800">Security Notice</h3>
            <div className="mt-2 text-sm text-yellow-700">
              <p>
                The downloaded SDK contains your agent's <strong>private key</strong>. Never commit this file to version control
                or share it publicly. Keep it secure and regenerate keys immediately if compromised.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Action Buttons */}
      <div className="flex gap-4 justify-center pt-6">
        <Button
          variant="outline"
          onClick={() => router.push('/dashboard/agents')}
        >
          View All Agents
        </Button>
        <Button
          onClick={() => window.open('https://opena2a.org/docs/aim', '_blank')}
        >
          <Book className="h-4 w-4 mr-2" />
          View Documentation
        </Button>
        <Button
          onClick={() => router.push('/dashboard')}
        >
          Go to Dashboard
          <ArrowRight className="h-4 w-4 ml-2" />
        </Button>
      </div>
    </div>
    </AuthGuard>
  );
}
