'use client'

import { useState } from 'react'
import { Download, Code, Terminal, CheckCircle, AlertCircle, Copy, Check } from 'lucide-react'
import { api } from '@/lib/api'
import { AuthGuard } from "@/components/auth-guard";

type SDKLanguage = 'python' | 'go' | 'javascript'

const sampleCode = `from aim_sdk import secure, AgentType

# ══════════════════════════════════════════════════════════════════════════
# AGENT REGISTRATION - Only Requires Agent's Name & Capabilities
# ══════════════════════════════════════════════════════════════════════════
agent = secure(
    "my-ai-assistant",
    agent_type=AgentType.LANGCHAIN,  # CREWAI, AUTOGEN, GPT, CLAUDE, etc.
    capabilities=["db:read", "api:call"],
    version="1.0.0",  # Note: version defaults to "1.0.0" if undeclared
    description="Customer support AI agent",
    tags=["production", "customer-facing", "gpt-4", "support-team"],
    metadata={
        "model": "gpt-4",
        "department": "support"
    },
    mcp_servers=["github", "filesystem"],  # Remove to use mcp auto detect
)

# ══════════════════════════════════════════════════════════════════════════
# TRACK ACTIONS & RISKS - Log all agent activities in AIM
# ------------------------------------------------------------------------------
# AIM verifies agent has db:read and approve or reject action
@agent.perform_action(capability="db:read")  # auto: low risk
def get_customer(customer_id: str):
    return {"id": customer_id, "name": "Jane Doe"}
# ------------------------------------------------------------------------------
# @agent.perform_action(capability="text:generate")  # auto: low risk
# def generate_response(prompt: str):
#     return llm.generate(prompt)

# @agent.perform_action(capability="sentiment:analyze")  # auto: low risk
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
#         "expires": "2025-01-01"
#     }
# )`

export default function SDKDownloadPage() {
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [selectedSDK, setSelectedSDK] = useState<SDKLanguage>('python')
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(sampleCode)
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
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
          Open Source Agent Security
        </h1>
        <p className="text-gray-600 dark:text-gray-400 text-lg">
          Secure your agents with minimal code. Zero configuration required.
        </p>
      </div>

      {/* Success message */}
      {success && (
        <div className="mb-6 p-4 bg-green-50 border border-green-200 rounded-lg flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-green-600 mt-0.5 flex-shrink-0" />
          <div>
            <p className="font-medium text-green-900">SDK downloaded successfully!</p>
            <p className="text-sm text-green-700 mt-1">
              Follow the setup instructions below to get started.
            </p>
          </div>
        </div>
      )}

      {/* Error message */}
      {error && (
        <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-lg flex items-start gap-3">
          <AlertCircle className="h-5 w-5 text-red-600 mt-0.5 flex-shrink-0" />
          <div>
            <p className="font-medium text-red-900">Download failed</p>
            <p className="text-sm text-red-700 mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* SDK Card - Python Only */}
      <div className="mb-8">
        {/* Python SDK - Stable */}
        <div className="bg-white border-2 border-blue-500 rounded-lg shadow-lg overflow-hidden max-w-2xl mx-auto">
          <div className="p-8">
            <div className="flex items-center gap-4 mb-6">
              <div className="h-16 w-16 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center shadow-lg">
                <Code className="h-8 w-8 text-white" />
              </div>
              <div>
                <h2 className="text-2xl font-bold text-gray-900">Python SDK</h2>
                <div className="flex items-center gap-2 mt-1">
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-800">
                    ✅ Stable Release
                  </span>
                  <span className="inline-flex items-center px-2 py-1 rounded text-sm font-mono font-medium bg-gray-100 text-gray-700">
                    v1.7.0
                  </span>
                </div>
              </div>
            </div>

            <p className="text-base text-gray-700 mb-6">
              Official Python client for agent identity management with Ed25519 cryptographic
              verification, OAuth integration, automatic MCP detection, and secure keyring storage.
            </p>

            <button
              onClick={() => handleDownload('python')}
              disabled={downloading && selectedSDK === 'python'}
              className="w-full bg-blue-600 text-white px-6 py-3 rounded-lg font-medium hover:bg-blue-700 disabled:bg-blue-400 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-base shadow-md"
            >
              <Download className="h-5 w-5" />
              {downloading && selectedSDK === 'python' ? 'Downloading...' : 'Download Python SDK'}
            </button>
          </div>
        </div>

        {/* Future SDKs Notice */}
        <div className="mt-6 max-w-2xl mx-auto">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <p className="text-sm text-blue-900 mb-2">
              <strong>🚀 Future SDK Releases:</strong>
            </p>
            <p className="text-sm text-blue-800">
              Go and JavaScript/TypeScript SDKs are planned for Q1-Q2 2026. The Python SDK provides
              complete feature parity and is ready for all use cases today.
            </p>
          </div>
        </div>
      </div>

      {/* Setup Instructions */}
      <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
        <div className="p-6">
          <div className="flex items-center gap-2 mb-4">
            <Terminal className="h-5 w-5 text-gray-700" />
            <h3 className="text-lg font-semibold text-gray-900">Quick Start - See Results in 60 Seconds!</h3>
          </div>

          <div className="space-y-6">
            <div>
              <h4 className="font-medium text-gray-900 mb-2">1. Extract SDK to Your Projects Folder</h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto mb-2">
                <code className="text-sm text-green-400 font-mono">
                  # Recommended: Extract to your home directory or projects folder<br />
                  cd ~/projects  # or any folder you prefer<br />
                  unzip ~/Downloads/aim-sdk-python.zip<br />
                  cd aim-sdk-python<br />
                  pip install -e .
                </code>
              </div>
              <div className="bg-blue-50 border border-blue-200 rounded-lg p-3 mt-2">
                <p className="text-sm text-blue-800">
                  <strong>Where to extract?</strong> Anywhere you keep your projects! Common locations:
                </p>
                <ul className="text-sm text-blue-700 mt-1 ml-4 list-disc">
                  <li><code>~/projects/aim-sdk-python</code></li>
                  <li><code>~/dev/aim-sdk-python</code></li>
                  <li><code>~/Desktop/aim-sdk-python</code> (for quick testing)</li>
                </ul>
              </div>
            </div>

            <div className="bg-gradient-to-r from-green-50 to-emerald-50 border-2 border-green-300 rounded-lg p-4">
              <h4 className="font-semibold text-green-900 mb-2 flex items-center gap-2">
                <span className="bg-green-600 text-white px-2 py-0.5 rounded text-xs">NEW</span>
                2. Run the Demo Agent - Watch Dashboard Update Live!
              </h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto mb-2">
                <code className="text-sm text-green-400 font-mono">
                  python demo_agent.py
                </code>
              </div>
              <p className="text-sm text-green-800 mb-2">
                The demo agent includes interactive actions you can trigger. Open your{' '}
                <a href="/dashboard/agents" className="underline font-medium">Agents Dashboard</a>{' '}
                side-by-side and watch it update in real-time as you perform actions!
              </p>
              <p className="text-sm text-green-700">
                <strong>Actions included:</strong> Weather checks, product searches, user lookups, notifications, and more - each with different risk levels so you can see how AIM monitors them differently.
              </p>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">3. Build Your Own Agent</h4>
              <div className="relative bg-black rounded-lg p-4 overflow-x-auto mb-2 border-2 border-primary/30">
                <button
                  onClick={handleCopy}
                  className="absolute top-3 right-3 p-2 rounded-md bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white transition-colors"
                  title="Copy to clipboard"
                >
                  {copied ? <Check className="h-4 w-4 text-green-400" /> : <Copy className="h-4 w-4" />}
                </button>
                <pre className="text-sm text-green-400 font-mono whitespace-pre overflow-x-auto">
                  {sampleCode}
                </pre>
              </div>
              <p className="text-sm text-gray-600 dark:text-gray-400 flex items-start gap-2">
                <CheckCircle className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
                <span>Click the copy button above or use the demo_agent.py file as a starting point!</span>
              </p>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">4. View Real-Time Security Analytics</h4>
              <p className="text-gray-700 dark:text-gray-300 mb-3">
                Monitor your agent&apos;s security posture, trust score, MCP connections, and behavior analytics in real-time.
              </p>
              <a
                href="/dashboard/agents"
                className="inline-flex items-center gap-2 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 font-medium"
              >
                View Agents Dashboard →
              </a>
            </div>
          </div>
        </div>
      </div>

    </div>
    </AuthGuard>
  );
}
