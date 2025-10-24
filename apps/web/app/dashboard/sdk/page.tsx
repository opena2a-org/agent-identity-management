'use client'

import { useState } from 'react'
import { Download, Code, Terminal, CheckCircle, AlertCircle, Lock, Shield } from 'lucide-react'
import Link from 'next/link'
import { api } from '@/lib/api'
import { AuthGuard } from "@/components/auth-guard";

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

      // Use API client with automatic token refresh on 401
      const blob = await api.downloadSDK(sdk)

      // Create blob and trigger download
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `aim-sdk-${sdk}.zip`
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
          Enterprise-Grade Agent Security
        </h1>
        <p className="text-gray-600 dark:text-gray-400 text-lg">
          Secure your agents with 1 line of code. Zero configuration required.
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
        {/* Python SDK - Production Ready */}
        <div className="bg-white border-2 border-blue-500 rounded-lg shadow-lg overflow-hidden max-w-2xl mx-auto">
          <div className="p-8">
            <div className="flex items-center gap-4 mb-6">
              <div className="h-16 w-16 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center shadow-lg">
                <Code className="h-8 w-8 text-white" />
              </div>
              <div>
                <h2 className="text-2xl font-bold text-gray-900">Python SDK</h2>
                <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-800 mt-1">
                  ✅ Production Ready
                </span>
              </div>
            </div>

            <p className="text-base text-gray-700 mb-6">
              Official production-ready Python client for agent identity management with Ed25519 cryptographic
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

          <div className="bg-gradient-to-br from-gray-50 to-blue-50 px-6 py-5 border-t border-gray-200">
            <h3 className="text-sm font-semibold text-gray-700 mb-3">Features Included:</h3>
            <div className="grid md:grid-cols-2 gap-3">
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>OAuth/OIDC auto-configured</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>Auto-detect MCPs & capabilities</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>Ed25519 crypto signing</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>System keyring integration</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>LangChain & CrewAI support</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-gray-700">
                <CheckCircle className="h-4 w-4 text-green-600 flex-shrink-0" />
                <span>100% test coverage</span>
              </div>
            </div>
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
              complete feature parity and is production-ready for all use cases today.
            </p>
          </div>
        </div>
      </div>

      {/* Security Notice */}
      <div className="bg-gradient-to-br from-primary/5 to-transparent border-2 border-primary/20 rounded-lg p-6 mb-8 flex items-start gap-3">
        <Shield className="h-6 w-6 text-primary mt-0.5 flex-shrink-0" />
        <div className="flex-1">
          <p className="font-semibold text-gray-900 dark:text-white text-lg">Enterprise-Grade Security, Developer-Friendly UX</p>
          <p className="text-sm text-gray-700 dark:text-gray-300 mt-2">
            AIM SDK uses Ed25519 cryptographic signing for authentication - more secure than API keys.
            Each agent gets a unique private key, and you can monitor and revoke access anytime.
          </p>
          <Link
            href="/dashboard/agents"
            className="inline-flex items-center gap-2 text-sm text-primary hover:text-primary/80 font-medium mt-3"
          >
            <Lock className="h-4 w-4" />
            View Agent Security Dashboard →
          </Link>
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
              <h4 className="font-medium text-gray-900 mb-2">2. Register Your Agent - ONE LINE!</h4>
              <div className="bg-black rounded-lg p-4 overflow-x-auto mb-2 border-2 border-primary/30">
                <code className="text-sm text-green-400 font-mono">
                  from aim_sdk import secure<br />
                  <br />
                  # ONE LINE - Enterprise security enabled! 🚀<br />
                  agent = secure(&quot;your-agent-name&quot;)<br />
                  <br />
                  # ✨ That&apos;s it! Your agent is now secure.<br />
                  <br />
                  # Automatically enabled:<br />
                  # ✅ Ed25519 cryptographic signing on every request<br />
                  # ✅ Auto-MCP detection from Claude Desktop config<br />
                  # ✅ Real-time trust scoring and behavior analytics<br />
                  # ✅ Audit logging and compliance reporting<br />
                  # ✅ Anomaly detection and security alerts
                </code>
              </div>
              <p className="text-sm text-gray-600 dark:text-gray-400 flex items-start gap-2">
                <CheckCircle className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
                <span>That&apos;s it! One line. Zero configuration. Enterprise-grade security.</span>
              </p>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">3. View Real-Time Security Analytics</h4>
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
