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
  const [selectedLanguage, setSelectedLanguage] = useState<'python' | 'java'>('python');

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

  const downloadSDK = async (language: 'python' | 'java' | 'nodejs' | 'go') => {
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
          <div className="animate-spin rounded-pill h-12 w-12 border-b-2 border-brand"></div>
        </div>
      </div>
    );
  }

  if (!agent) {
    return (
      <div className="max-w-4xl mx-auto mt-12">
        <Card>
          <CardContent className="pt-6">
            <p className="text-center text-ink-secondary">Agent not found</p>
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
          <div className="bg-success-fill p-4 rounded-pill">
            <CheckCircle className="h-16 w-16 text-success-text" />
          </div>
        </div>
        <h1 className="text-3xl font-bold text-ink mb-2">
          Agent registered successfully
        </h1>
        <p className="text-ink-secondary max-w-2xl mx-auto">
          Your agent <span className="font-semibold">{agent.displayName}</span> has been registered with AIM.
          Download the SDK to start building with automatic identity verification.
        </p>
      </div>

      {/* Agent Details Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CheckCircle className="h-5 w-5 text-success-text" />
            Agent details
          </CardTitle>
          <CardDescription>Your agent has been created with these credentials</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Agent ID */}
          <div className="flex items-center justify-between p-3 bg-glass-inset-gray rounded-inset">
            <div className="flex-1">
              <p className="text-sm font-medium text-ink-body">Agent ID</p>
              <p className="text-sm text-ink-secondary font-mono break-all">{agent.id}</p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => copyToClipboard(agent.id, 'agent_id')}
            >
              {copiedField === 'agent_id' ? (
                <Check className="h-4 w-4 text-success-text" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>

          {/* Agent Name */}
          <div className="flex items-center justify-between p-3 bg-glass-inset-gray rounded-inset">
            <div className="flex-1">
              <p className="text-sm font-medium text-ink-body">Agent name</p>
              <p className="text-sm text-ink-secondary">{agent.name}</p>
            </div>
          </div>

          {/* Public Key */}
          {agent.publicKey && (
            <div className="flex items-center justify-between p-3 bg-glass-inset-gray rounded-inset">
              <div className="flex-1">
                <p className="text-sm font-medium text-ink-body">Public key (Ed25519)</p>
                <p className="text-sm text-ink-secondary font-mono break-all truncate max-w-[500px]">
                  {agent.publicKey}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => copyToClipboard(agent.publicKey!, 'public_key')}
              >
                {copiedField === 'public_key' ? (
                  <Check className="h-4 w-4 text-success-text" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
          )}

          {/* Status */}
          <div className="flex items-center justify-between p-3 bg-glass-inset-gray rounded-inset">
            <div className="flex-1">
              <p className="text-sm font-medium text-ink-body">Status</p>
              <span className="inline-flex items-center px-2.5 py-0.5 rounded-pill border border-warning-border bg-warning-fill text-xs font-medium text-warning-text">
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
            <div className="rounded-card border border-stroke bg-glass-inset p-6">
              <div className="flex flex-col h-full">
                <div className="mb-4">
                  <div className="flex items-center gap-3 mb-3">
                    <div className="h-12 w-12 bg-brand rounded-inset shadow-glow flex items-center justify-center">
                      <Download className="h-6 w-6 text-ink-inverse" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-xl mb-1">Python SDK</h3>
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-pill border border-success-border bg-success-fill text-xs font-medium text-success-text">
                        Production ready
                      </span>
                    </div>
                  </div>
                  <p className="text-sm text-ink-body mb-3">
                    Official production-ready SDK with Ed25519 cryptographic signing, OAuth integration,
                    automatic MCP detection, and secure keyring storage.
                  </p>
                  <div className="space-y-2 text-sm text-ink-secondary">
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>Ed25519 cryptographic signing</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>OAuth/OIDC integration (Google, Microsoft, Okta)</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>Automatic MCP detection</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>Secure keyring credential storage</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>100% test coverage</span>
                    </div>
                  </div>
                </div>
                <div className="mt-auto">
                  <Button
                    className="w-full"
                    onClick={() => downloadSDK('python')}
                    disabled={downloadingSDK !== null}
                  >
                    {downloadingSDK === 'python' ? (
                      <>
                        <div className="animate-spin rounded-pill h-4 w-4 border-b-2 border-current mr-2"></div>
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

            {/* Java SDK - Production Ready */}
            <div className="rounded-card border border-stroke bg-glass-inset p-6">
              <div className="flex flex-col h-full">
                <div className="mb-4">
                  <div className="flex items-center gap-3 mb-3">
                    <div className="h-12 w-12 bg-brand rounded-inset shadow-glow flex items-center justify-center">
                      <Download className="h-6 w-6 text-ink-inverse" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-xl mb-1">Java SDK</h3>
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-pill border border-success-border bg-success-fill text-xs font-medium text-success-text">
                        Production ready
                      </span>
                    </div>
                  </div>
                  <p className="text-sm text-ink-body mb-3">
                    Java SDK with Ed25519 cryptographic signing, OAuth integration,
                    AspectJ annotations, and Spring Boot support.
                  </p>
                  <div className="space-y-2 text-sm text-ink-secondary">
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>Ed25519 cryptographic signing</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>OAuth client credentials flow</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>@SecureAction AspectJ annotations</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>Spring Boot integration</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-success-text" />
                      <span>MCP server attestation</span>
                    </div>
                  </div>
                </div>
                <div className="mt-auto">
                  <Button
                    className="w-full"
                    onClick={() => downloadSDK('java')}
                    disabled={downloadingSDK !== null}
                  >
                    {downloadingSDK === 'java' ? (
                      <>
                        <div className="animate-spin rounded-pill h-4 w-4 border-b-2 border-current mr-2"></div>
                        Downloading...
                      </>
                    ) : (
                      <>
                        <Download className="h-4 w-4 mr-2" />
                        Download Java SDK
                      </>
                    )}
                  </Button>
                </div>
              </div>
            </div>

            {/* Future SDKs Note */}
            <div className="rounded-inset border border-divider bg-glass-inset-gray p-4">
              <p className="text-sm text-ink-body mb-2">
                <strong>Future releases:</strong> Go and JavaScript/TypeScript SDKs are planned for Q1-Q2 2026.
              </p>
              <p className="text-xs text-ink-tertiary">
                Python and Java SDKs provide complete feature parity and are production-ready for all use cases.
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
            Quick start guide
          </CardTitle>
          <CardDescription>Get up and running in 3 steps</CardDescription>
        </CardHeader>
        <CardContent>
          {/* Language Tabs */}
          <div className="flex gap-2 mb-6">
            <Button
              variant={selectedLanguage === 'python' ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedLanguage('python')}
            >
              Python
            </Button>
            <Button
              variant={selectedLanguage === 'java' ? 'default' : 'outline'}
              size="sm"
              onClick={() => setSelectedLanguage('java')}
            >
              Java
            </Button>
          </div>

          {selectedLanguage === 'python' ? (
            <div className="space-y-4">
              {/* Python Step 1 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-bold">
                  1
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Install the SDK</h4>
                  <p className="text-sm text-ink-secondary">
                    From PyPI. For machines without registry access, download the package above and use the offline install on the SDK page.
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>pip install aim-sdk</code>
                  </pre>
                </div>
              </div>
              {/* Python Step 2 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-bold">
                  2
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Sign in once</h4>
                  <p className="text-sm text-ink-secondary">
                    Links your machine to this account; no API key needed
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>aim-sdk login</code>
                  </pre>
                </div>
              </div>
              {/* Python Step 3 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-bold">
                  3
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Connect this agent</h4>
                  <p className="text-sm text-ink-secondary">
                    Use this agent&apos;s own identifier so the SDK attaches to it instead of registering a new one
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>{`from aim_sdk import secure

agent = secure("${agent.id}")`}</code>
                  </pre>
                </div>
              </div>

              {/* Python Example Code */}
              <div className="mt-6">
                <h4 className="font-semibold mb-2">Example usage</h4>
                <pre className="glass-contrast p-4 rounded-inset font-mono text-xs text-ink-code overflow-x-auto">
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
            </div>
          ) : (
            <div className="space-y-4">
              {/* Java Step 1 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-semibold">
                  1
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Download and extract SDK</h4>
                  <p className="text-sm text-ink-secondary">
                    Download the Java SDK above and extract the ZIP file to your project directory
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>unzip aim-sdk-{agent.name}-java.zip</code>
                  </pre>
                </div>
              </div>

              {/* Java Step 2 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-semibold">
                  2
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Add to your project</h4>
                  <p className="text-sm text-ink-secondary">
                    Add the SDK to your Maven or Gradle project
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>{`<!-- Maven -->
mvn install:install-file -Dfile=aim-sdk.jar \\
  -DgroupId=org.opena2a -DartifactId=aim-sdk \\
  -Dversion=1.0.0 -Dpackaging=jar`}</code>
                  </pre>
                </div>
              </div>

              {/* Java Step 3 */}
              <div className="flex gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-pill bg-brand-soft text-brand-text flex items-center justify-center font-semibold">
                  3
                </div>
                <div>
                  <h4 className="font-semibold mb-1">Run example</h4>
                  <p className="text-sm text-ink-secondary">
                    Compile and run the included example
                  </p>
                  <pre className="mt-2 p-3 rounded-inset-sm bg-glass-inset-gray font-mono text-xs text-ink-body overflow-x-auto">
                    <code>mvn compile exec:java -Dexec.mainClass="BasicExample"</code>
                  </pre>
                </div>
              </div>

              {/* Java Example Code */}
              <div className="mt-6">
                <h4 className="font-semibold mb-2">Example usage</h4>
                <pre className="glass-contrast p-4 rounded-inset font-mono text-xs text-ink-code overflow-x-auto">
                  <code>{`import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import java.util.Arrays;

public class MyAgent {
    public static void main(String[] args) {
        // One-line secure registration
        AIMClient agent = AIMClient.secure(
            "my-agent",
            Arrays.asList("db:read", "api:call"),
            AgentType.CUSTOM
        );

        // Execute with automatic verification
        String result = agent.performAction(
            "db:read",
            "users_table",
            () -> {
                // Your agent code here
                return database.query("SELECT * FROM users");
            }
        );

        System.out.println("Result: " + result);
        agent.close();
    }
}`}</code>
                </pre>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Security Notice */}
      <div className="rounded-card border border-warning-border bg-warning-fill p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg className="h-5 w-5 text-warning" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
            </svg>
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-warning-text">Security notice</h3>
            <div className="mt-2 text-sm text-ink-body">
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
          View all agents
        </Button>
        <Button
          onClick={() => window.open('https://opena2a.org/docs', '_blank')}
        >
          <Book className="h-4 w-4 mr-2" />
          View documentation
        </Button>
        <Button
          onClick={() => router.push('/dashboard')}
        >
          Go to dashboard
          <ArrowRight className="h-4 w-4 ml-2" />
        </Button>
      </div>
    </div>
    </AuthGuard>
  );
}
