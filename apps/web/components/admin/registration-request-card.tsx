'use client'

import { useState } from 'react'
import { CheckCircle, XCircle, User, Mail, Shield, Calendar, AlertCircle } from 'lucide-react'
import { api } from '@/lib/api'

interface RegistrationRequest {
  id: string
  email: string
  firstName: string
  lastName: string
  // Absent for email/password registrations (the backend list query does not
  // populate OAuth columns), so always guard when rendering
  oauthProvider?: 'google' | 'microsoft' | 'okta' | 'local'
  oauthUserId?: string
  status: 'pending' | 'approved' | 'rejected'
  requestedAt: string
  reviewedAt?: string
  reviewedBy?: string
  rejectionReason?: string
  profilePictureUrl?: string
  oauthEmailVerified: boolean
  metadata?: {
    signupProfile?: {
      role?: string
      primaryUseCase?: string
      referralSource?: string
    }
  }
}

interface RegistrationRequestCardProps {
  request: RegistrationRequest
  onApproved?: () => void
  onRejected?: () => void
}

const providerColors = {
  google: 'bg-blue-100 text-blue-700',
  microsoft: 'bg-gray-800 text-white',
  okta: 'bg-blue-600 text-white',
  local: 'bg-gray-100 text-gray-700',
}

// Human-readable labels for the signup questionnaire slugs
// (vocabulary defined in backend domain/signup_profile.go)
const signupProfileLabels: Record<string, string> = {
  developer: 'Developer',
  'security-engineer': 'Security engineer',
  'founder-or-exec': 'Founder or executive',
  'student-or-researcher': 'Student or researcher',
  'securing-production-agents': 'Securing production agents',
  'evaluating-for-team': 'Evaluating for team',
  'research-or-learning': 'Research or learning',
  'personal-project': 'Personal project',
  github: 'GitHub',
  search: 'Search engine',
  'social-media': 'Social media',
  'colleague-or-friend': 'Colleague or friend',
  'blog-or-article': 'Blog or article',
  other: 'Other',
}

const formatProfileValue = (value?: string) =>
  value ? (signupProfileLabels[value] ?? value) : null

export function RegistrationRequestCard({ request, onApproved, onRejected }: RegistrationRequestCardProps) {
  const [isApproving, setIsApproving] = useState(false)
  const [isRejecting, setIsRejecting] = useState(false)
  const [showRejectModal, setShowRejectModal] = useState(false)
  const [rejectionReason, setRejectionReason] = useState('')
  const [error, setError] = useState<string | null>(null)

  const fullName = [request.firstName, request.lastName].filter(Boolean).join(' ') || 'Unknown'

  const signupProfile = request.metadata?.signupProfile
  const profileEntries = [
    { label: 'Role', value: formatProfileValue(signupProfile?.role) },
    { label: 'Use case', value: formatProfileValue(signupProfile?.primaryUseCase) },
    { label: 'Heard via', value: formatProfileValue(signupProfile?.referralSource) },
  ].filter((entry) => entry.value)

  const handleApprove = async () => {
    setIsApproving(true)
    setError(null)

    try {
      await api.approveRegistration(request.id)
      onApproved?.()
    } catch (err: any) {
      setError(err.message || 'Failed to approve registration')
      setIsApproving(false)
    }
  }

  const handleReject = async () => {
    if (!rejectionReason.trim()) {
      setError('Rejection reason is required')
      return
    }

    setIsRejecting(true)
    setError(null)

    try {
      await api.rejectRegistration(request.id, rejectionReason)
      setShowRejectModal(false)
      onRejected?.()
    } catch (err: any) {
      setError(err.message || 'Failed to reject registration')
      setIsRejecting(false)
    }
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <>
      <div className="bg-white border border-gray-200 rounded-lg p-6 hover:shadow-md transition-shadow">
        <div className="flex items-start gap-4">
          {/* Profile Picture */}
          <div className="flex-shrink-0">
            {request.profilePictureUrl ? (
              <img
                src={request.profilePictureUrl}
                alt={fullName}
                className="w-16 h-16 rounded-full object-cover"
              />
            ) : (
              <div className="w-16 h-16 rounded-full bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center">
                <User className="w-8 h-8 text-white" />
              </div>
            )}
          </div>

          {/* Content */}
          <div className="flex-grow min-w-0">
            {/* Name and Provider */}
            <div className="flex items-start justify-between gap-4 mb-3">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 mb-1">
                  {fullName}
                </h3>
                <div className="flex items-center gap-2 text-sm text-gray-600">
                  <Mail className="w-4 h-4" />
                  <span className="truncate">{request.email}</span>
                </div>
              </div>

              <span className={`px-3 py-1 rounded-full text-xs font-medium ${providerColors[request.oauthProvider ?? 'local']}`}>
                {(request.oauthProvider ?? 'local').toUpperCase()}
              </span>
            </div>

            {/* Metadata */}
            <div className="grid grid-cols-2 gap-3 mb-4 text-sm">
              <div className="flex items-center gap-2 text-gray-600">
                <Calendar className="w-4 h-4" />
                <span>Requested {formatDate(request.requestedAt)}</span>
              </div>

              <div className="flex items-center gap-2">
                {request.oauthEmailVerified ? (
                  <>
                    <CheckCircle className="w-4 h-4 text-green-600" />
                    <span className="text-green-700 font-medium">Email Verified</span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="w-4 h-4 text-amber-600" />
                    <span className="text-amber-700">Email Not Verified</span>
                  </>
                )}
              </div>
            </div>

            {/* Signup Profile Answers */}
            {profileEntries.length > 0 && (
              <div className="flex flex-wrap gap-2 mb-4">
                {profileEntries.map((entry) => (
                  <span
                    key={entry.label}
                    className="inline-flex items-center gap-1.5 px-3 py-1 bg-gray-50 border border-gray-200 rounded-full text-xs"
                  >
                    <span className="text-gray-500">{entry.label}:</span>
                    <span className="font-medium text-gray-800">{entry.value}</span>
                  </span>
                ))}
              </div>
            )}

            {/* Error Message */}
            {error && (
              <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-800">
                {error}
              </div>
            )}

            {/* Action Buttons */}
            {request.status === 'pending' && (
              <div className="flex gap-3">
                <button
                  onClick={handleApprove}
                  disabled={isApproving || isRejecting}
                  className="flex-1 flex items-center justify-center gap-2 bg-green-600 hover:bg-green-700 text-white font-medium py-2 px-4 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isApproving ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                      <span>Approving...</span>
                    </>
                  ) : (
                    <>
                      <CheckCircle className="w-4 h-4" />
                      <span>Approve</span>
                    </>
                  )}
                </button>

                <button
                  onClick={() => setShowRejectModal(true)}
                  disabled={isApproving || isRejecting}
                  className="flex-1 flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-medium py-2 px-4 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <XCircle className="w-4 h-4" />
                  <span>Reject</span>
                </button>
              </div>
            )}

            {/* Status Badge for Reviewed Requests */}
            {request.status !== 'pending' && (
              <div className={`inline-flex items-center gap-2 px-4 py-2 rounded-lg ${
                request.status === 'approved' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
              }`}>
                {request.status === 'approved' ? (
                  <CheckCircle className="w-4 h-4" />
                ) : (
                  <XCircle className="w-4 h-4" />
                )}
                <span className="font-medium capitalize">{request.status}</span>
                {request.reviewedAt && (
                  <span className="text-sm">on {formatDate(request.reviewedAt)}</span>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Rejection Modal */}
      {showRejectModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg max-w-md w-full p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">
              Reject Registration Request
            </h3>

            <p className="text-sm text-gray-600 mb-4">
              Please provide a reason for rejecting {fullName}'s registration request.
              This will be sent to the user via email.
            </p>

            <textarea
              value={rejectionReason}
              onChange={(e) => setRejectionReason(e.target.value)}
              placeholder="e.g., Email address does not match company domain"
              rows={4}
              className="w-full border border-gray-300 rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-red-500 focus:border-transparent"
            />

            {error && (
              <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-800">
                {error}
              </div>
            )}

            <div className="mt-6 flex gap-3">
              <button
                onClick={() => {
                  setShowRejectModal(false)
                  setRejectionReason('')
                  setError(null)
                }}
                disabled={isRejecting}
                className="flex-1 border border-gray-300 hover:bg-gray-50 text-gray-700 font-medium py-2 px-4 rounded-lg transition-colors"
              >
                Cancel
              </button>

              <button
                onClick={handleReject}
                disabled={isRejecting || !rejectionReason.trim()}
                className="flex-1 bg-red-600 hover:bg-red-700 text-white font-medium py-2 px-4 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isRejecting ? (
                  <span className="flex items-center justify-center gap-2">
                    <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    Rejecting...
                  </span>
                ) : (
                  'Reject Request'
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
