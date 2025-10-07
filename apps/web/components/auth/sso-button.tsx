'use client'

import { Chrome, Shield } from 'lucide-react'
import { useState } from 'react'

type OAuthProvider = 'google' | 'microsoft' | 'okta'

interface SSOButtonProps {
  provider: OAuthProvider
  onClick?: () => void
  disabled?: boolean
  loading?: boolean
}

const providerConfig = {
  google: {
    name: 'Google',
    icon: Chrome,
    bgColor: 'bg-white hover:bg-gray-50',
    textColor: 'text-gray-900',
    borderColor: 'border-gray-300',
  },
  microsoft: {
    name: 'Microsoft',
    icon: Shield,
    bgColor: 'bg-[#2F2F2F] hover:bg-[#1F1F1F]',
    textColor: 'text-white',
    borderColor: 'border-gray-600',
  },
  okta: {
    name: 'Okta',
    icon: Shield,
    bgColor: 'bg-[#007DC1] hover:bg-[#005A8F]',
    textColor: 'text-white',
    borderColor: 'border-[#007DC1]',
  },
}

export function SSOButton({ provider, onClick, disabled = false, loading = false }: SSOButtonProps) {
  const [isLoading, setIsLoading] = useState(loading)
  const config = providerConfig[provider]
  const Icon = config.icon

  const handleClick = () => {
    if (disabled || isLoading) return

    if (onClick) {
      onClick()
    } else {
      // Default behavior: redirect to backend OAuth endpoint
      setIsLoading(true)
      window.location.href = `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v1/oauth/${provider}/login`
    }
  }

  return (
    <button
      onClick={handleClick}
      disabled={disabled || isLoading}
      className={`
        flex items-center justify-center gap-3 w-full px-6 py-3 rounded-lg
        border ${config.borderColor} ${config.bgColor} ${config.textColor}
        font-medium transition-all duration-200
        disabled:opacity-50 disabled:cursor-not-allowed
        focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2
        ${isLoading ? 'cursor-wait' : 'cursor-pointer'}
      `}
    >
      {isLoading ? (
        <>
          <div className="w-5 h-5 border-2 border-current border-t-transparent rounded-full animate-spin" />
          <span>Redirecting...</span>
        </>
      ) : (
        <>
          <Icon className="w-5 h-5" />
          <span>Sign up with {config.name}</span>
        </>
      )}
    </button>
  )
}
