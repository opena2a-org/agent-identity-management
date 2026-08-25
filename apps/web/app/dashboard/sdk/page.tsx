'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Download, Code, Terminal, CheckCircle, AlertCircle, Copy, Check } from 'lucide-react'
import { api } from '@/lib/api'
import { AuthGuard } from "@/components/auth-guard";

type SDKLanguage = 'python' | 'java' | 'go' | 'javascript'

const pythonSampleCode = `from aim_sdk import secure, AgentType

# ══════════════════════════════════════════════════════════════════════════
# AGENT REGISTRATION - Only Requires Agent's Name & Capabilities
# ══════════════════════════════════════════════════════════════════════════
agent = secure(
    "my-ai-assistant",
    agent_type=AgentType.LANGCHAIN,  # CREWAI, AUTOGEN, GPT, CLAUDE, etc.
    capabilities=["db:read", "api:call"],
    mcp_servers=["filesystem"],
    version="1.0.0", # Note: version defaults to "1.0.0" if undeclared
    description="Customer support AI agent",
    tags=["production", "customer-facing", "gpt-4", "support-team"],
    metadata={
        "model": "gpt-4",
        "department": "support"
    }
)

# ══════════════════════════════════════════════════════════════════════════
# TRACK ACTIONS & RISKS - Log all agent activities in AIM
# -----------------------------------------------------------
# AIM verifies agent has db:read and approve or reject action
@agent.perform_action(capability="db:read")  # auto: low risk
def get_customer(customer_id: str):
    return {"id": customer_id, "name": "Jane Doe"}
result = get_customer("cust-123")
print(f"Result: {result}")

# -----------------------------------------------------------
# @agent.perform_action(capability="text:generate") # auto: low risk
# def generate_response(prompt: str):
#     return llm.generate(prompt)

# @agent.perform_action(capability="sentiment:analyze") # auto: low risk
# def analyze_sentiment(text: str):
#     return {"sentiment": "positive", "score": 0.92}

# Critical action - requires admin approval (JIT Access)
# @agent.perform_action(capability="customer:delete", jit_access=True)
# def delete_customer(customer_id: str):
#     return {"deleted": customer_id}  # Waits for admin approval!

# Run your functions - AIM verifies and tracks everything!
# response = generate_response("Hello, how can I help?")
# print(f"Response: {response}")

# ══════════════════════════════════════════════════════════════════════════
# CAPABILITY REQUEST - Request New Capabilities
# ══════════════════════════════════════════════════════════════════════════

# Request capabilities that need admin approval
# agent.request_capability(
#     "db:admin",
#     reason="Need admin access for migration",
#     metadata={
#         "use_case": "quarterly migration",
#         "expires": "2025-01-01"}
# )`

const javaSampleCode = `import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import org.opena2a.aim.client.RiskLevel;
import org.opena2a.aim.annotations.SecureAction;

import java.util.Arrays;
import java.util.Map;

// ══════════════════════════════════════════════════════════════════════════
// AGENT REGISTRATION - Only Requires Agent's Name & Capabilities
// ══════════════════════════════════════════════════════════════════════════
AIMClient agent = AIMClient.builder("my-ai-assistant")
    .agentType(AgentType.LANGCHAIN)  // CREWAI, AUTOGEN, OPENAI, ANTHROPIC, CUSTOM
    .capabilities(Arrays.asList("db:read", "api:call"))
    .talksTo(Arrays.asList("filesystem"))  // MCP servers
    .description("Customer support AI agent")
    .tags(Arrays.asList("production", "customer-facing", "gpt-4", "support-team"))
    .metadata(Map.of(
        "model", "gpt-4",
        "department", "support"
    ))
    .build();

// ══════════════════════════════════════════════════════════════════════════
// TRACK ACTIONS & RISKS - Using performAction for functional style
// ══════════════════════════════════════════════════════════════════════════
// AIM verifies agent has db:read and approves or rejects action
String result = agent.performAction("db:read", "users_table", () -> {
    return userRepository.findById("cust-123").toString();
});
System.out.println("Result: " + result);

// With explicit risk level for sensitive operations
PaymentResult payment = agent.performAction(
    "payment:process",
    "stripe_api",
    RiskLevel.HIGH,
    () -> paymentService.process(request)
);

// ══════════════════════════════════════════════════════════════════════════
// OR USE @SecureAction ANNOTATION - Declarative security with AspectJ
// ══════════════════════════════════════════════════════════════════════════
public class CustomerService {

    @SecureAction(capability = "db:read", resource = "customers")
    public Customer getCustomer(String id) {
        return customerRepository.findById(id);
    }

    @SecureAction(
        capability = "customer:delete",
        resource = "customers",
        riskLevel = RiskLevel.CRITICAL,
        jitAccess = true  // Requires admin approval!
    )
    public void deleteCustomer(String id) {
        customerRepository.deleteById(id);
    }
}

// ══════════════════════════════════════════════════════════════════════════
// CAPABILITY REQUEST - Request New Capabilities
// ══════════════════════════════════════════════════════════════════════════
// Map<String, Object> result = agent.requestCapability(
//     "db:admin",
//     "Need admin access for migration"
// );`

export default function SDKDownloadPage() {
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [selectedSDK, setSelectedSDK] = useState<SDKLanguage>('python')
  const [selectedCodeTab, setSelectedCodeTab] = useState<'python' | 'java'>('python')
  const [copied, setCopied] = useState(false)

  const currentSampleCode = selectedCodeTab === 'python' ? pythonSampleCode : javaSampleCode

  const handleCopy = async () => {
    await navigator.clipboard.writeText(currentSampleCode)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleDownload = async (sdk: SDKLanguage) => {
    try {
      setDownloading(true)
      setError(null)
      setSuccess(false)
      setSelectedSDK(sdk)

      // Use API client with automatic token refresh on 401
      const { blob, filename } = await api.downloadSDK(sdk)

      // Create blob and trigger download with versioned filename from backend
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)

      setSuccess(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to download SDK')
    } finally {
      setDownloading(false)
    }
  }

  return (
    <AuthGuard>
      <div className="container mx-auto py-8 px-4 max-w-4xl">
        <div className="mb-8">
        <h1 className="text-3xl font-bold text-ink mb-2">
          Open source agent security
        </h1>
        <p className="text-ink-secondary text-lg">
          Offline install path: download the SDK package for machines without registry access.
          The supported path is <code className="font-mono text-base">pip install aim-sdk</code>.
        </p>
      </div>

      {/* Success message */}
      {success && (
        <div className="mb-6 p-4 bg-success-fill border border-success-border rounded-card-sm flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-success-text mt-0.5 flex-shrink-0" />
          <div>
            <p className="font-medium text-success-text">SDK downloaded</p>
            <p className="text-sm text-ink-body mt-1">
              Follow the setup instructions below to get started.
            </p>
          </div>
        </div>
      )}

      {/* Error message */}
      {error && (
        <div className="glass-alert mb-6 p-4 flex items-start gap-3">
          <AlertCircle className="h-5 w-5 text-danger-text mt-0.5 flex-shrink-0" />
          <div>
            <p className="font-medium text-danger-text">Download failed</p>
            <p className="text-sm text-ink-body mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* SDK Cards - Python and Java */}
      <div className="mb-8 grid md:grid-cols-2 gap-6 max-w-4xl mx-auto">
        {/* Python SDK - Stable */}
        <div className="glass overflow-hidden">
          <div className="p-6">
            <div className="flex items-center gap-4 mb-4">
              <div className="h-14 w-14 bg-brand rounded-inset flex items-center justify-center shadow-glow">
                <Code className="h-7 w-7 text-white" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-ink">Python SDK</h2>
                <div className="flex items-center gap-2 mt-1">
                  <span className="inline-flex items-center px-2 py-0.5 rounded-pill text-xs font-medium bg-success-fill border border-success-border text-success-text">
                    Stable
                  </span>
                </div>
              </div>
            </div>

            <p className="text-sm text-ink-body mb-4">
              Official Python client with Ed25519 cryptographic verification, OAuth integration,
              automatic MCP detection, and secure keyring storage.
            </p>

            <button
              onClick={() => handleDownload('python')}
              disabled={downloading && selectedSDK === 'python'}
              className="w-full rounded-pill bg-brand text-white shadow-glow px-4 py-2.5 font-medium hover:bg-brand-hover disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-sm"
            >
              <Download className="h-4 w-4" />
              {downloading && selectedSDK === 'python' ? 'Downloading...' : 'Download Python SDK'}
            </button>
          </div>
        </div>

        {/* Java SDK - New! */}
        <div className="glass overflow-hidden">
          <div className="p-6">
            <div className="flex items-center gap-4 mb-4">
              <div className="h-14 w-14 bg-brand rounded-inset flex items-center justify-center shadow-glow">
                <Code className="h-7 w-7 text-white" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-ink">Java SDK</h2>
                <div className="flex items-center gap-2 mt-1">
                  <span className="inline-flex items-center px-2 py-0.5 rounded-pill text-xs font-medium bg-warning-fill border border-warning-border text-warning-text">
                    New
                  </span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium bg-glass-inset-gray text-ink-body">
                    v1.0.0
                  </span>
                </div>
              </div>
            </div>

            <p className="text-sm text-ink-body mb-4">
              Java client with Maven support, AspectJ annotations, OkHttp,
              BouncyCastle cryptography, and Spring Boot integration.
            </p>

            <button
              onClick={() => handleDownload('java')}
              disabled={downloading && selectedSDK === 'java'}
              className="w-full rounded-pill bg-brand text-white shadow-glow px-4 py-2.5 font-medium hover:bg-brand-hover disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-sm"
            >
              <Download className="h-4 w-4" />
              {downloading && selectedSDK === 'java' ? 'Downloading...' : 'Download Java SDK'}
            </button>
          </div>
        </div>
      </div>

      {/* Setup Instructions */}
      <div className="glass overflow-hidden">
        <div className="p-6">
          <div className="flex items-center gap-2 mb-4">
            <Terminal className="h-5 w-5 text-ink-body" />
            <h3 className="text-lg font-semibold text-ink">Quick start</h3>
          </div>

          {/* Language Tabs - Applies to all steps */}
          <div className="flex gap-2 mb-4">
            <button
              onClick={() => setSelectedCodeTab('python')}
              className={`px-4 py-2 rounded-pill text-sm font-medium transition-colors ${
                selectedCodeTab === 'python'
                  ? 'bg-brand text-white shadow-glow'
                  : 'bg-glass-inset-gray text-ink-body hover:bg-track'
              }`}
            >
              Python
            </button>
            <button
              onClick={() => setSelectedCodeTab('java')}
              className={`px-4 py-2 rounded-pill text-sm font-medium transition-colors ${
                selectedCodeTab === 'java'
                  ? 'bg-brand text-white shadow-glow'
                  : 'bg-glass-inset-gray text-ink-body hover:bg-track'
              }`}
            >
              Java
            </button>
          </div>

          <div className="space-y-6">
            <div>
              <h4 className="font-medium text-ink mb-2">1. Extract and install the SDK</h4>
              <div className="glass-contrast p-4 overflow-x-auto">
                {selectedCodeTab === 'python' ? (
                  <code className="text-sm text-ink-code font-mono">
                    cd ~/projects  # or any folder you prefer<br />
                    unzip ~/Downloads/aim-sdk-python.zip<br />
                    cd aim-sdk-python<br />
                    pip install -e .
                  </code>
                ) : (
                  <code className="text-sm text-ink-code font-mono">
                    cd ~/projects  # or any folder you prefer<br />
                    unzip ~/Downloads/aim-sdk-java.zip<br />
                    cd aim-sdk-java<br />
                    mvn install
                  </code>
                )}
              </div>
            </div>

            <div className="bg-success-fill border border-success-border rounded-card-sm p-4">
              <h4 className="font-semibold text-ink mb-2">
                2. Run the demo agent
              </h4>
              <div className="glass-contrast p-4 overflow-x-auto mb-2">
                {selectedCodeTab === 'python' ? (
                  <code className="text-sm text-ink-code font-mono">
                    python demo_agent.py
                  </code>
                ) : (
                  <code className="text-sm text-ink-code font-mono">
                    mvn exec:java -Dexec.mainClass="org.opena2a.aim.demo.DemoAgent"
                  </code>
                )}
              </div>
              <p className="text-sm text-ink-body mb-2">
                The demo agent includes interactive actions you can trigger. Open your{' '}
                <Link href="/dashboard/agents" className="underline font-medium text-brand-text">agents dashboard</Link>{' '}
                side-by-side and watch it update as you perform actions.
              </p>
              <p className="text-sm text-ink-secondary">
                <strong>Actions included:</strong> Weather checks, product searches, user lookups, notifications, and more - each with different risk levels so you can see how AIM monitors them differently.
              </p>
            </div>

            <div>
              <h4 className="font-medium text-ink mb-2">3. Build your own agent</h4>
              <div className="glass-contrast relative p-4 overflow-x-auto mb-2">
                <button
                  onClick={handleCopy}
                  className="absolute top-3 right-3 p-2 rounded-md bg-glass-code hover:bg-glass-inset-gray text-ink-inverse-secondary hover:text-ink-inverse transition-colors"
                  title="Copy to clipboard"
                >
                  {copied ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
                </button>
                <pre className="text-sm text-ink-code font-mono whitespace-pre overflow-x-auto">
                  {currentSampleCode}
                </pre>
              </div>
              <p className="text-sm text-ink-secondary flex items-start gap-2">
                <CheckCircle className="h-4 w-4 text-success-text mt-0.5 flex-shrink-0" />
                <span>
                  {selectedCodeTab === 'python'
                    ? 'Click the copy button above or use the demo_agent.py file as a starting point.'
                    : 'Add the SDK to your Maven pom.xml and use the examples as a starting point.'}
                </span>
              </p>
            </div>

            <div>
              <h4 className="font-medium text-ink mb-2">4. View security analytics</h4>
              <p className="text-ink-body mb-3">
                Monitor your agent&apos;s security posture, trust score, MCP connections, and behavior analytics.
              </p>
              <Link
                href="/dashboard/agents"
                className="inline-flex items-center gap-2 text-brand-text hover:underline font-medium"
              >
                View agents dashboard →
              </Link>
            </div>
          </div>
        </div>
      </div>

    </div>
    </AuthGuard>
  );
}
