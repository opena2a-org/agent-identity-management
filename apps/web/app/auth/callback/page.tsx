import { Suspense } from 'react'
import { AuthCallbackContent } from './auth-callback-content'
import { Loader2 } from 'lucide-react'

export default function AuthCallbackPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-slate-800/50 backdrop-blur-sm border border-slate-700 rounded-2xl p-8 shadow-2xl text-center">
          <Loader2 className="w-16 h-16 text-blue-500 mx-auto mb-4 animate-spin" />
          <h2 className="text-2xl font-bold text-white mb-2">
            Loading...
          </h2>
          <p className="text-slate-400">
            Please wait
          </p>
        </div>
      </div>
    }>
      <AuthCallbackContent />
    </Suspense>
  )
}
