'use client'

import { useSearchParams } from 'next/navigation'
import { Shield, CheckCircle, Mail, Clock } from 'lucide-react'
import Link from 'next/link'
import { Suspense } from 'react'

function RegistrationPendingContent() {
  const searchParams = useSearchParams()
  const requestId = searchParams.get('request_id')
  const supportEmail = process.env.NEXT_PUBLIC_SUPPORT_EMAIL || 'info@opena2a.org'

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 via-white to-purple-50 flex items-center justify-center p-4">
      <div className="w-full max-w-2xl">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="flex justify-center mb-4">
            <div className="w-16 h-16 bg-gradient-to-br from-blue-600 to-purple-600 rounded-2xl flex items-center justify-center shadow-lg">
              <Shield className="w-10 h-10 text-white" />
            </div>
          </div>
          <h1 className="text-3xl font-bold text-gray-900">
            AIM - Agent Identity Management
          </h1>
        </div>

        {/* Success Card */}
        <div className="bg-white rounded-2xl shadow-xl border border-gray-200 p-8">
          {/* Success Icon */}
          <div className="flex justify-center mb-6">
            <div className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center">
              <CheckCircle className="w-12 h-12 text-green-600" />
            </div>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold text-gray-900 text-center mb-4">
            Registration Submitted Successfully!
          </h2>

          {/* Description */}
          <p className="text-gray-600 text-center mb-8">
            Your account request has been submitted and is now pending administrator approval.
          </p>

          {/* Request ID */}
          {requestId && (
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-6">
              <p className="text-sm text-gray-600 mb-1">Request ID:</p>
              <p className="font-mono text-sm text-gray-900 break-all">
                {requestId}
              </p>
            </div>
          )}

          {/* Next Steps */}
          <div className="space-y-4 mb-8">
            <h3 className="font-semibold text-gray-900 text-lg">What happens next?</h3>

            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-blue-100 rounded-full flex items-center justify-center flex-shrink-0 mt-1">
                <Clock className="w-4 h-4 text-blue-600" />
              </div>
              <div>
                <h4 className="font-medium text-gray-900">Administrator Review</h4>
                <p className="text-sm text-gray-600">
                  An administrator will review your registration request. This typically takes 1-2 business days.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-purple-100 rounded-full flex items-center justify-center flex-shrink-0 mt-1">
                <Mail className="w-4 h-4 text-purple-600" />
              </div>
              <div>
                <h4 className="font-medium text-gray-900">Email Notification</h4>
                <p className="text-sm text-gray-600">
                  You'll receive an email notification once your account has been approved or if additional information is needed.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-green-100 rounded-full flex items-center justify-center flex-shrink-0 mt-1">
                <CheckCircle className="w-4 h-4 text-green-600" />
              </div>
              <div>
                <h4 className="font-medium text-gray-900">Access Granted</h4>
                <p className="text-sm text-gray-600">
                  Once approved, you'll be able to sign in and start using AIM to manage your AI agents and MCP servers.
                </p>
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex flex-col gap-3">
            <Link
              href="/auth/login"
              className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-3 px-6 rounded-lg transition-colors text-center"
            >
              Go to Sign In
            </Link>

            <a
              href={`mailto:${supportEmail}?subject=AIM Account Registration - Urgent`}
              className="w-full border border-gray-300 hover:bg-gray-50 text-gray-700 font-medium py-3 px-6 rounded-lg transition-colors text-center"
            >
              Contact Administrator
            </a>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-6 text-center text-sm text-gray-500">
          Need help?{' '}
          <a href="/support" className="text-blue-600 hover:underline">
            Contact Support
          </a>
        </div>
      </div>
    </div>
  )
}

export default function RegistrationPendingPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin" />
      </div>
    }>
      <RegistrationPendingContent />
    </Suspense>
  )
}
