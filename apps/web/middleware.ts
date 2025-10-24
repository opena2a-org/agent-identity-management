import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

// Define role-based route protection
const ROUTE_PERMISSIONS: Record<string, string[]> = {
  '/dashboard/admin': ['admin'],
  '/dashboard/admin/users': ['admin'],
  '/dashboard/admin/alerts': ['admin', 'manager'],
  '/dashboard/admin/audit': ['admin', 'manager'],
  '/dashboard/admin/security-policies': ['admin'],
  '/dashboard/admin/capability-requests': ['admin'],
  '/dashboard/security': ['admin', 'manager', 'member'],
  '/dashboard/monitoring': ['admin', 'manager', 'member'],
  '/dashboard/analytics': ['admin', 'manager', 'member'],
};

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Public routes that don't require authentication
  const publicRoutes = ['/login', '/auth/callback', '/auth/login', '/auth/register', '/auth/registration-pending']
  const isPublicRoute = publicRoutes.some(route => pathname.startsWith(route))

  // If accessing a public route, allow it
  if (isPublicRoute) {
    return NextResponse.next()
  }

  // Check auth token in cookies
  const token = request.cookies.get('access_token')?.value;
  
  if (!token) {
    // No token, redirect to login
    const loginUrl = new URL('/auth/login', request.url);
    loginUrl.searchParams.set('returnUrl', pathname);
    return NextResponse.redirect(loginUrl);
  }

  try {
    // Decode JWT to get role (basic check, real validation happens server-side)
    const payload = JSON.parse(atob(token.split('.')[1]));
    const userRole = payload?.role;

    // Normalize "pending" role to "viewer"
    const normalizedRole = userRole === 'pending' ? 'viewer' : userRole;

    // Check if current route requires specific role
    for (const [route, allowedRoles] of Object.entries(ROUTE_PERMISSIONS)) {
      if (pathname.startsWith(route)) {
        if (!allowedRoles.includes(normalizedRole)) {
          // User doesn't have required role, redirect to forbidden page
          return NextResponse.redirect(new URL('/dashboard/forbidden', request.url));
        }
      }
    }
  } catch (error) {
    // Token invalid, redirect to login
    const loginUrl = new URL('/auth/login', request.url);
    loginUrl.searchParams.set('returnUrl', pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     */
    '/((?!_next/static|_next/image|favicon.ico).*)',
  ],
}
