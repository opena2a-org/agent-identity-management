'use client'

import { useState } from 'react'
import { AlertCircle, Calendar, CheckCircle, Mail, XCircle } from 'lucide-react'
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

  const provider = request.oauthProvider ?? 'local'
  const providerLabel = provider === 'local' ? 'Email and password' : provider.charAt(0).toUpperCase() + provider.slice(1)

  return (
    <>
      <div className="glass p-5">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            {request.profilePictureUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={request.profilePictureUrl} alt="" className="h-12 w-12 rounded-full object-cover" />
            ) : (
              <span className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#a78bfa] to-[#6366f1] text-sm font-bold text-white" aria-hidden="true">
                {fullName.slice(0, 1).toUpperCase()}
              </span>
            )}
          </div>

          <div className="min-w-0 flex-grow">
            <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="truncate text-[15px] font-bold tracking-[-0.02em] text-ink">{fullName}</h3>
                <p className="flex items-center gap-1.5 text-xs text-ink-secondary">
                  <Mail className="h-3.5 w-3.5 text-ink-tertiary" aria-hidden="true" />
                  <span className="truncate">{request.email}</span>
                </p>
              </div>
              <span className="inline-flex rounded-pill border border-glass-inset-border bg-glass-inset-gray px-2.5 py-0.5 text-2xs font-bold text-ink-body">
                {providerLabel}
              </span>
            </div>

            <div className="mb-3 flex flex-wrap gap-x-5 gap-y-1.5 text-xs">
              <span className="inline-flex items-center gap-1.5 text-ink-secondary">
                <Calendar className="h-3.5 w-3.5 text-ink-tertiary" aria-hidden="true" />
                Requested {formatDate(request.requestedAt)}
              </span>
              {request.oauthEmailVerified ? (
                <span className="inline-flex items-center gap-1.5 font-semibold text-success-text">
                  <CheckCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  Email verified by the provider
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 font-semibold text-warning-text">
                  <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  Email not verified
                </span>
              )}
            </div>

            {profileEntries.length > 0 && (
              <div className="mb-3 flex flex-wrap gap-2">
                {profileEntries.map((entry) => (
                  <span key={entry.label} className="inline-flex items-center gap-1.5 rounded-pill bg-glass-inset-gray px-2.5 py-1 text-2xs">
                    <span className="text-ink-tertiary">{entry.label}</span>
                    <span className="font-bold text-ink">{entry.value}</span>
                  </span>
                ))}
              </div>
            )}

            {error && (
              <div className="mb-3 rounded-inset border border-danger-border bg-danger-fill p-3 text-xs font-semibold text-danger-text" role="alert">
                {error}
              </div>
            )}

            {request.status === 'pending' && (
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={handleApprove}
                  disabled={isApproving || isRejecting}
                  className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-pill bg-brand px-4 text-xs font-bold text-white shadow-glow hover:bg-brand-hover disabled:opacity-50"
                >
                  {isApproving ? (
                    <>
                      <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white" aria-hidden="true" />
                      Approving...
                    </>
                  ) : (
                    <>
                      <CheckCircle className="h-4 w-4" aria-hidden="true" />
                      Approve
                    </>
                  )}
                </button>
                <button
                  type="button"
                  onClick={() => setShowRejectModal(true)}
                  disabled={isApproving || isRejecting}
                  className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-pill border border-danger-border bg-danger-fill px-4 text-xs font-bold text-danger-text hover:brightness-95 disabled:opacity-50"
                >
                  <XCircle className="h-4 w-4" aria-hidden="true" />
                  Reject
                </button>
              </div>
            )}

            {request.status !== 'pending' && (
              <div className={`inline-flex items-center gap-2 rounded-pill border px-3 py-1 text-xs font-bold ${
                request.status === 'approved' ? 'border-success-border bg-success-fill text-success-text' : 'border-danger-border bg-danger-fill text-danger-text'
              }`}>
                {request.status === 'approved' ? <CheckCircle className="h-3.5 w-3.5" aria-hidden="true" /> : <XCircle className="h-3.5 w-3.5" aria-hidden="true" />}
                <span className="capitalize">{request.status}</span>
                {request.reviewedAt && <span className="font-medium opacity-80">on {formatDate(request.reviewedAt)}</span>}
              </div>
            )}
          </div>
        </div>
      </div>

      {showRejectModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="reject-title">
          <div className="glass-chrome w-full max-w-md p-6">
            <h3 id="reject-title" className="text-[17px] font-bold tracking-[-0.02em] text-ink">Reject this request</h3>
            <p className="mt-1 text-xs text-ink-secondary">
              Give a reason for rejecting {fullName}. It is stored with the request for other administrators.
            </p>
            <label htmlFor="reject-reason" className="sr-only">Rejection reason</label>
            <textarea
              id="reject-reason"
              value={rejectionReason}
              onChange={(e) => setRejectionReason(e.target.value)}
              placeholder="e.g. Email address is not on the company domain"
              rows={4}
              className="mt-4 w-full rounded-inset border border-stroke bg-glass-inset p-3 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {error && (
              <div className="mt-3 rounded-inset border border-danger-border bg-danger-fill p-3 text-xs font-semibold text-danger-text" role="alert">
                {error}
              </div>
            )}
            <div className="mt-5 flex gap-3">
              <button
                type="button"
                onClick={() => {
                  setShowRejectModal(false)
                  setRejectionReason('')
                  setError(null)
                }}
                disabled={isRejecting}
                className="inline-flex h-10 flex-1 items-center justify-center rounded-pill border border-stroke bg-glass text-sm font-bold text-ink hover:bg-glass-inset"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleReject}
                disabled={isRejecting || !rejectionReason.trim()}
                className="inline-flex h-10 flex-1 items-center justify-center gap-2 rounded-pill bg-danger text-sm font-bold text-white hover:brightness-95 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {isRejecting ? (
                  <>
                    <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white" aria-hidden="true" />
                    Rejecting...
                  </>
                ) : (
                  'Reject request'
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
