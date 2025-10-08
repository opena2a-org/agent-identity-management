'use client'

import { useState } from 'react'
import { Download, Code, Terminal, CheckCircle, AlertCircle, Lock, Shield } from 'lucide-react'
import Link from 'next/link'

export default function SDKDownloadPage() {
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const handleDownload = async () => {
    try {
      setDownloading(true)
      setError(null)
      setSuccess(false)

      const response = await fetch('http://localhost:8080/api/v1/sdk/download', {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to download SDK')
      }

      // Create blob and trigger download
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'aim-sdk-python.zip'
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

      {/* Download Card */}
      <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden mb-8">
        <div className="p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="h-12 w-12 bg-blue-100 rounded-lg flex items-center justify-center">
              <Code className="h-6 w-6 text-blue-600" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900">Python SDK</h2>
              <p className="text-sm text-gray-500">Pre-configured with your credentials</p>
            </div>
          </div>

          <p className="text-gray-700 mb-6">
            This SDK is already configured with your identity. Just download, install, and start
            registering agents. No API keys or configuration needed!
          </p>

          <button
            onClick={handleDownload}
            disabled={downloading}
            className="w-full bg-blue-600 text-white px-6 py-3 rounded-lg font-medium hover:bg-blue-700 disabled:bg-blue-400 disabled:cursor-not-allowed flex items-center justify-center gap-2 transition-colors"
          >
            <Download className="h-5 w-5" />
            {downloading ? 'Downloading...' : 'Download SDK'}
          </button>
        </div>

        <div className="bg-gray-50 px-6 py-4 border-t border-gray-200">
          <div className="flex items-center gap-2 text-sm text-gray-600">
            <CheckCircle className="h-4 w-4 text-green-600" />
            <span>Credentials valid for 90 days</span>
          </div>
          <div className="flex items-center gap-2 text-sm text-gray-600 mt-2">
            <CheckCircle className="h-4 w-4 text-green-600" />
            <span>Auto-authentication included</span>
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

      {/* Setup Instructions */}
      <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
        <div className="p-6">
          <div className="flex items-center gap-2 mb-4">
            <Terminal className="h-5 w-5 text-gray-700" />
            <h3 className="text-lg font-semibold text-gray-900">Quick Start</h3>
          </div>

          <div className="space-y-6">
            <div>
              <h4 className="font-medium text-gray-900 mb-2">1. Extract & Install</h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
                <code className="text-sm text-green-400 font-mono">
                  unzip aim-sdk-python.zip<br />
                  cd aim-sdk-python<br />
                  pip install -e .
                </code>
              </div>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">2. Register Your First Agent</h4>
              <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
                <code className="text-sm text-green-400 font-mono">
                  from aim_sdk import register_agent<br />
                  <br />
                  # Zero configuration - credentials already embedded!<br />
                  agent = register_agent(<br />
                  &nbsp;&nbsp;&nbsp;&nbsp;name="my-awesome-agent",<br />
                  &nbsp;&nbsp;&nbsp;&nbsp;display_name="My Awesome Agent",<br />
                  &nbsp;&nbsp;&nbsp;&nbsp;description="Does amazing things",<br />
                  &nbsp;&nbsp;&nbsp;&nbsp;agent_type="ai_agent"<br />
                  )<br />
                  <br />
                  print(f"Agent ID: {'{'}agent.agent_id{'}'}")<br />
                  print(f"Dashboard: {'{'}agent.aim_url{'}'}/dashboard/agents")
                </code>
              </div>
            </div>

            <div>
              <h4 className="font-medium text-gray-900 mb-2">3. View in Dashboard</h4>
              <p className="text-gray-700 mb-3">
                Your agent will appear in the dashboard automatically, linked to your account.
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
            Your credentials are embedded. Just install and use!
          </p>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="h-10 w-10 bg-blue-100 rounded-lg flex items-center justify-center mb-3">
            <Code className="h-5 w-5 text-blue-600" />
          </div>
          <h4 className="font-medium text-gray-900 mb-1">Auto-Auth</h4>
          <p className="text-sm text-gray-600">
            SDK automatically refreshes tokens when needed.
          </p>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="h-10 w-10 bg-purple-100 rounded-lg flex items-center justify-center mb-3">
            <Terminal className="h-5 w-5 text-purple-600" />
          </div>
          <h4 className="font-medium text-gray-900 mb-1">One-Line</h4>
          <p className="text-sm text-gray-600">
            Register agents with a single function call.
          </p>
        </div>
      </div>
    </div>
  )
}
