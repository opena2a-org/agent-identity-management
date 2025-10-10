'use client'

import { useState } from 'react'
import { Download, Code, Terminal, CheckCircle, AlertCircle, Lock, Shield } from 'lucide-react'
import Link from 'next/link'
import { api } from '@/lib/api'

type SDKLanguage = 'python' | 'go' | 'javascript'

export default function SDKDownloadPage() {
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [selectedSDK, setSelectedSDK] = useState<SDKLanguage>('python')

  const handleDownload = async (sdk: SDKLanguage) => {
    try {
      setDownloading(true)
      setError(null)
      setSuccess(false)
      setSelectedSDK(sdk)

      if (sdk === 'python') {
        // Use API client with automatic token refresh on 401
        const blob = await api.downloadSDK()

        // Create blob and trigger download
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = 'aim-sdk-python.zip'
        document.body.appendChild(a)
        a.click()
        window.URL.revokeObjectURL(url)
        document.body.removeChild(a)

        setSuccess(true)
      } else {
        // For Go and JavaScript, download from GitHub releases
        const repoUrl = 'https://github.com/opena2a-org/agent-identity-management'
        const sdkPath = sdk === 'go' ? 'sdks/go' : 'sdks/javascript'
        window.open(`${repoUrl}/tree/main/${sdkPath}`, '_blank')
        setSuccess(true)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to download SDK')
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div className="container mx-auto py-8 px-4 max-w-4xl">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-2">Download SDK</h1>
        <p className="text-gray-600">
          Get started with AIM SDK in seconds. Zero configuration required!
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

      {/* SDK Cards */}
      <div className="grid md:grid-cols-3 gap-6 mb-8">
        {/* Python SDK - Full Featured */}
        <div className="bg-white border-2 border-blue-500 rounded-lg shadow-sm overflow-hidden">
          <div className="bg-blue-50 px-4 py-2 border-b border-blue-200">
            <span className="text-xs font-semibold text-blue-700">✨ RECOMMENDED</span>
          </div>
          <div className="p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="h-12 w-12 bg-blue-100 rounded-lg flex items-center justify-center">
                <Code className="h-6 w-6 text-blue-600" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-gray-900">Python SDK</h2>
                <p className="text-xs text-gray-500">Full featured</p>
              </div>
            </div>

            <p className="text-sm text-gray-700 mb-4 h-12">
              Zero config, OAuth, auto-detection, Ed25519 signing, keyring support.
            </p>

            <button
              onClick={() => handleDownload('python')}
              disabled={downloading && selectedSDK === 'python'}
              className="w-full bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 disabled:bg-blue-400 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-sm"
            >
              <Download className="h-4 w-4" />
              {downloading && selectedSDK === 'python' ? 'Downloading...' : 'Download SDK'}
            </button>
          </div>

          <div className="bg-gray-50 px-4 py-3 border-t border-gray-200 space-y-1">
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <CheckCircle className="h-3 w-3 text-green-600 flex-shrink-0" />
              <span>OAuth auto-configured</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <CheckCircle className="h-3 w-3 text-green-600 flex-shrink-0" />
              <span>Auto-detect MCPs & capabilities</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <CheckCircle className="h-3 w-3 text-green-600 flex-shrink-0" />
              <span>Ed25519 crypto signing</span>
            </div>
          </div>
        </div>

        {/* Go SDK - Feature Parity Coming */}
        <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
          <div className="bg-orange-50 px-4 py-2 border-b border-orange-200">
            <span className="text-xs font-semibold text-orange-700">🚧 BASIC (Feature parity soon)</span>
          </div>
          <div className="p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="h-12 w-12 bg-cyan-100 rounded-lg flex items-center justify-center">
                <Code className="h-6 w-6 text-cyan-600" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-gray-900">Go SDK</h2>
                <p className="text-xs text-gray-500">API key mode only</p>
              </div>
            </div>

            <p className="text-sm text-gray-700 mb-4 h-12">
              Manual setup required. Full feature parity (OAuth, auto-detection) coming soon!
            </p>

            <button
              onClick={() => handleDownload('go')}
              disabled={downloading && selectedSDK === 'go'}
              className="w-full bg-cyan-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-cyan-700 disabled:bg-cyan-400 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-sm"
            >
              <Download className="h-4 w-4" />
              {downloading && selectedSDK === 'go' ? 'Opening...' : 'View on GitHub →'}
            </button>
          </div>

          <div className="bg-gray-50 px-4 py-3 border-t border-gray-200 space-y-1">
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <CheckCircle className="h-3 w-3 text-green-600 flex-shrink-0" />
              <span>API key authentication</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <AlertCircle className="h-3 w-3 text-orange-500 flex-shrink-0" />
              <span>Manual MCP reporting</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <AlertCircle className="h-3 w-3 text-orange-500 flex-shrink-0" />
              <span>No OAuth (yet)</span>
            </div>
          </div>
        </div>

        {/* JavaScript SDK - Feature Parity Coming */}
        <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
          <div className="bg-orange-50 px-4 py-2 border-b border-orange-200">
            <span className="text-xs font-semibold text-orange-700">🚧 BASIC (Feature parity soon)</span>
          </div>
          <div className="p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="h-12 w-12 bg-yellow-100 rounded-lg flex items-center justify-center">
                <Code className="h-6 w-6 text-yellow-600" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-gray-900">JavaScript SDK</h2>
                <p className="text-xs text-gray-500">API key mode only</p>
              </div>
            </div>

            <p className="text-sm text-gray-700 mb-4 h-12">
              Manual setup required. Full feature parity (OAuth, auto-detection) coming soon!
            </p>

            <button
              onClick={() => handleDownload('javascript')}
              disabled={downloading && selectedSDK === 'javascript'}
              className="w-full bg-yellow-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-yellow-700 disabled:bg-yellow-400 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors text-sm"
            >
              <Download className="h-4 w-4" />
              {downloading && selectedSDK === 'javascript' ? 'Opening...' : 'View on GitHub →'}
            </button>
          </div>

          <div className="bg-gray-50 px-4 py-3 border-t border-gray-200 space-y-1">
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <CheckCircle className="h-3 w-3 text-green-600 flex-shrink-0" />
              <span>API key authentication</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <AlertCircle className="h-3 w-3 text-orange-500 flex-shrink-0" />
              <span>Manual MCP reporting</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <AlertCircle className="h-3 w-3 text-orange-500 flex-shrink-0" />
              <span>No OAuth (yet)</span>
            </div>
          </div>
        </div>
      </div>

      {/* Security Notice */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-8 flex items-start gap-3">
        <Shield className="h-5 w-5 text-blue-600 mt-0.5 flex-shrink-0" />
        <div className="flex-1">
          <p className="font-medium text-blue-900">Security Best Practices</p>
          <p className="text-sm text-blue-700 mt-1">
            Each SDK download generates a unique authentication token. You can monitor and revoke tokens anytime.
          </p>
          <Link
            href="/dashboard/sdk-tokens"
            className="inline-flex items-center gap-2 text-sm text-blue-600 hover:text-blue-800 font-medium mt-2"
          >
            <Lock className="h-4 w-4" />
            Manage SDK Tokens →
          </Link>
        </div>
      </div>

      {/* Auto-Detection Features */}
      <div className="bg-gradient-to-r from-blue-50 to-purple-50 border border-blue-200 rounded-lg p-5 mb-8">
        <div className="flex items-start gap-3">
          <CheckCircle className="h-6 w-6 text-blue-600 mt-0.5 flex-shrink-0" />
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">🚀 Zero-Config Auto-Detection</h3>
            <p className="text-gray-700 mb-3">
              The SDK automatically detects <strong>everything</strong> - no manual configuration needed!
            </p>
            <div className="space-y-2 text-sm text-gray-700">
              <div className="flex items-start gap-2">
                <CheckCircle className="h-4 w-4 mt-0.5 flex-shrink-0 text-green-600" />
                <div>
                  <strong>Capabilities</strong> - Auto-detected from imports, decorators, and config files
                </div>
              </div>
              <div className="flex items-start gap-2">
                <CheckCircle className="h-4 w-4 mt-0.5 flex-shrink-0 text-green-600" />
                <div>
                  <strong>MCP Servers</strong> - Auto-detected from Claude Desktop config and Python imports
                </div>
              </div>
              <div className="flex items-start gap-2">
                <CheckCircle className="h-4 w-4 mt-0.5 flex-shrink-0 text-green-600" />
                <div>
                  <strong>Security Packages</strong> - Cryptography and keyring auto-install with dependencies
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Setup Instructions */}
      <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
        <div className="p-6">
          <div className="flex items-center gap-2 mb-4">
            <Terminal className="h-5 w-5 text-gray-700" />
            <h3 className="text-lg font-semibold text-gray-900">Quick Start</h3>
          </div>

          <div className="space-y-6">
            <div>
              <h4 className="font-medium text-gray-900 mb-2">1. Extract & Install SDK</h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto mb-2">
                <code className="text-sm text-green-400 font-mono">
                  unzip aim-sdk-python.zip<br />
                  cd aim-sdk-python<br />
                  pip install -e .
                </code>
              </div>
              <p className="text-sm text-gray-600 flex items-start gap-2">
                <CheckCircle className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
                <span>All security dependencies (cryptography, keyring) auto-install automatically!</span>
              </p>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">2. Register Your First Agent - ONE LINE!</h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto mb-2">
                <code className="text-sm text-green-400 font-mono">
                  from aim_sdk import register_agent<br />
                  <br />
                  # ONE LINE - Everything auto-detected! 🚀<br />
                  agent = register_agent("my-awesome-agent")<br />
                  <br />
                  # ✅ Credentials: Auto-loaded from SDK<br />
                  # ✅ Capabilities: Auto-detected from imports<br />
                  # ✅ MCP Servers: Auto-detected from Claude config<br />
                  # ✅ Verification: Auto-completed via challenge-response<br />
                  <br />
                  print(f"Agent ID: {'{'}agent.agent_id{'}'}")<br />
                  print(f"Trust Score: {'{'}agent.trust_score{'}'}")
                </code>
              </div>
              <p className="text-sm text-gray-600 flex items-start gap-2">
                <AlertCircle className="h-4 w-4 text-blue-500 mt-0.5 flex-shrink-0" />
                <span>That&apos;s it! One line. Zero configuration. The &quot;Stripe Moment&quot; for AI agents.</span>
              </p>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">3. View in Dashboard</h4>
              <p className="text-gray-700 mb-3">
                Your agent appears automatically with full trust score, capabilities, and MCP server connections.
              </p>
              <a
                href="/dashboard/agents"
                className="inline-flex items-center gap-2 text-blue-600 hover:text-blue-700 font-medium"
              >
                View Agents Dashboard →
              </a>
            </div>
          </div>
        </div>
      </div>

      {/* Features */}
      <div className="mt-8 grid md:grid-cols-3 gap-4">
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="h-10 w-10 bg-green-100 rounded-lg flex items-center justify-center mb-3">
            <CheckCircle className="h-5 w-5 text-green-600" />
          </div>
          <h4 className="font-medium text-gray-900 mb-1">Zero Config</h4>
          <p className="text-sm text-gray-600">
            Credentials embedded. Dependencies auto-install. One line to register!
          </p>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="h-10 w-10 bg-blue-100 rounded-lg flex items-center justify-center mb-3">
            <Code className="h-5 w-5 text-blue-600" />
          </div>
          <h4 className="font-medium text-gray-900 mb-1">Auto-Detection</h4>
          <p className="text-sm text-gray-600">
            Capabilities and MCP servers detected automatically from your code.
          </p>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="h-10 w-10 bg-purple-100 rounded-lg flex items-center justify-center mb-3">
            <Shield className="h-5 w-5 text-purple-600" />
          </div>
          <h4 className="font-medium text-gray-900 mb-1">Auto-Verification</h4>
          <p className="text-sm text-gray-600">
            Challenge-response verification completes automatically with auto-approval.
          </p>
        </div>
      </div>
    </div>
  )
}
