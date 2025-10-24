import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { api } from '@/lib/api'

export function useAuth(required = true) {
  const router = useRouter()
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const checkAuth = () => {
      const token = api.getToken()

      if (!token && required) {
        // No token and auth is required - redirect to login
        router.push('/login')
        return
      }

      if (token) {
        setIsAuthenticated(true)
      }

      setIsLoading(false)
    }

    checkAuth()
  }, [router, required])

  return { isAuthenticated, isLoading }
}
