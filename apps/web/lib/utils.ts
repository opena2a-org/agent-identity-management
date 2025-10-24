import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(date: string | Date): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(d)
}

export function formatTrustScore(score: number): string {
  return (score * 100).toFixed(1) + '%'
}

export function getTrustScoreColor(score: number): string {
  if (score >= 0.8) return 'text-green-600'
  if (score >= 0.6) return 'text-blue-600'
  if (score >= 0.4) return 'text-yellow-600'
  return 'text-red-600'
}

export function getTrustScoreBadgeColor(score: number): string {
  if (score >= 0.8) return 'bg-green-100 text-green-800'
  if (score >= 0.6) return 'bg-blue-100 text-blue-800'
  if (score >= 0.4) return 'bg-yellow-100 text-yellow-800'
  return 'bg-red-100 text-red-800'
}
