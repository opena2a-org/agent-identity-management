'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { api } from '@/lib/api'
import { Loader2, CheckCircle2, XCircle } from 'lucide-react'

export default function AuthCallbackPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [error, setError] = useState<string>('')

  useEffect(() => {
    const handleCallback = async () => {
      try {
        // Get the authorization code from URL
        const code = searchParams.get('code')
        const state = searchParams.get('state')
        const error = searchParams.get('error')

        if (error) {
          throw new Error(error || 'OAuth authorization failed')
        }

        if (!code) {
          throw new Error('No authorization code received')
        }

        // Exchange code for token via backend
        const response = await fetch(
          `http://localhost:8080/api/v1/auth/callback/google?code=${code}&state=${state || ''}`
        )

        if (!response.ok) {
          const data = await response.json()
          throw new Error(data.message || 'Authentication failed')
        }

        const data = await response.json()

        // Store the token
        if (data.token || data.access_token) {
          const token = data.token || data.access_token
          api.setToken(token)

          setStatus('success')

          // Redirect to dashboard after short delay
          setTimeout(() => {
            router.push('/dashboard')
          }, 1500)
        } else {
          throw new Error('No token received from server')
        }
      } catch (err) {
        console.error('Auth callback error:', err)
        setStatus('error')
        setError(err instanceof Error ? err.message : 'Authentication failed')

        // Redirect to login after delay
        setTimeout(() => {
          router.push('/login')
        }, 3000)
      }
    }

    handleCallback()
  }, [router, searchParams])

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-slate-800/50 backdrop-blur-sm border border-slate-700 rounded-2xl p-8 shadow-2xl text-center">
        {status === 'loading' && (
          <>
            <Loader2 className="w-16 h-16 text-blue-500 mx-auto mb-4 animate-spin" />
            <h2 className="text-2xl font-bold text-white mb-2">
              Completing sign in...
            </h2>
            <p className="text-slate-400">
              Please wait while we verify your credentials
            </p>
          </>
        )}

        {status === 'success' && (
          <>
            <CheckCircle2 className="w-16 h-16 text-green-500 mx-auto mb-4" />
            <h2 className="text-2xl font-bold text-white mb-2">
              Sign in successful!
            </h2>
            <p className="text-slate-400">
              Redirecting to dashboard...
            </p>
          </>
        )}

        {status === 'error' && (
          <>
            <XCircle className="w-16 h-16 text-red-500 mx-auto mb-4" />
            <h2 className="text-2xl font-bold text-white mb-2">
              Authentication failed
            </h2>
            <p className="text-slate-400 mb-4">{error}</p>
            <p className="text-slate-500 text-sm">
              Redirecting to login page...
            </p>
          </>
        )}
      </div>
    </div>
  )
}
