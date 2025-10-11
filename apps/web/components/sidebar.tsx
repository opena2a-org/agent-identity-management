'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  Home,
  Shield,
  AlertTriangle,
  CheckCircle,
  Server,
  Key,
  Users,
  Bell,
  LogOut,
  ChevronLeft,
  Menu,
  X,
  Activity,
  Download,
  Lock,
} from 'lucide-react';
import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import { filterNavigationByRole, type UserRole, type NavSection } from '@/lib/permissions';

// ✅ Navigation with role-based access control
const navigationBase: NavSection[] = [
  {
    items: [
      // Everyone can see Dashboard and Agents
      {
        name: 'Dashboard',
        href: '/dashboard',
        icon: Home,
        roles: ['admin', 'manager', 'member', 'viewer'],
      },
      {
        name: 'Agents',
        href: '/dashboard/agents',
        icon: Shield,
        roles: ['admin', 'manager', 'member', 'viewer'],
      },
      // Member+ can access MCP Servers and API Keys
      {
        name: 'MCP Servers',
        href: '/dashboard/mcp',
        icon: Server,
        roles: ['admin', 'manager', 'member'],
      },
      {
        name: 'API Keys',
        href: '/dashboard/api-keys',
        icon: Key,
        roles: ['admin', 'manager', 'member'],
      },
      {
        name: 'Download SDK',
        href: '/dashboard/sdk',
        icon: Download,
        roles: ['admin', 'manager', 'member'],
      },
      {
        name: 'SDK Tokens',
        href: '/dashboard/sdk-tokens',
        icon: Lock,
        roles: ['admin', 'manager', 'member'],
      },
      // Manager+ can access monitoring and security
      {
        name: 'Activity Monitoring',
        href: '/dashboard/monitoring',
        icon: Activity,
        roles: ['admin', 'manager'],
      },
      {
        name: 'Security',
        href: '/dashboard/security',
        icon: AlertTriangle,
        roles: ['admin', 'manager'],
      },
    ],
  },
  {
    title: 'Administration',
    items: [
      // Admin-only access to user management and audit logs
      {
        name: 'Users',
        href: '/dashboard/admin/users',
        icon: Users,
        roles: ['admin'],
      },
      {
        name: 'Alerts',
        href: '/dashboard/admin/alerts',
        icon: Bell,
        roles: ['admin', 'manager'], // Managers can view alerts
      },
    ],
  },
];

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [user, setUser] = useState<{ email: string; display_name?: string; role?: UserRole } | null>(null);
  const [alertCount, setAlertCount] = useState<number>(0);
  const [navigation, setNavigation] = useState<NavSection[]>(navigationBase);

  useEffect(() => {
    // Fetch current user
    const fetchUser = async () => {
      try {
        const userData = await api.getCurrentUser();
        setUser(userData);
      } catch (error) {
        // Silently handle errors - don't throw to UI
        console.log('API call failed, using token fallback');

        // Fallback: decode user info from JWT token
        const token = api.getToken();
        if (token) {
          try {
            const payload = JSON.parse(atob(token.split('.')[1]));

            // Check if token is expired
            const now = Math.floor(Date.now() / 1000);
            if (payload.exp && payload.exp < now) {
              // Token expired - clear and redirect
              api.clearToken();
              setTimeout(() => router.push('/login'), 0);
              return;
            }

            setUser({
              email: payload.email || '',
              display_name: payload.email?.split('@')[0] || 'User',
              role: (payload.role as UserRole) || 'viewer'
            });
          } catch (e) {
            console.log('Token invalid, redirecting to login');
            api.clearToken();
            setTimeout(() => router.push('/login'), 0);
          }
        } else {
          // No token at all - redirect to login
          setTimeout(() => router.push('/login'), 0);
        }
      }
    };
    fetchUser();
  }, [router]);

  // ✅ Filter navigation based on user role using permissions system
  useEffect(() => {
    if (!user?.role) return;

    const filteredNav = filterNavigationByRole(navigationBase, user.role);
    setNavigation(filteredNav);
  }, [user?.role]);

  useEffect(() => {
    // Fetch alert count
    const fetchAlertCount = async () => {
      try {
        const count = await api.getUnacknowledgedAlertCount();
        setAlertCount(count);

        // Update navigation with alert badge (only if user can access alerts)
        setNavigation(prev => prev.map(section => ({
          ...section,
          items: section.items.map(item =>
            item.name === 'Alerts' && count > 0
              ? { ...item, badge: count }
              : item
          )
        })));
      } catch (error) {
        console.log('Failed to fetch alert count:', error);
      }
    };

    // Only fetch alerts if user has permission (not a viewer)
    if (user?.role && user.role !== 'viewer') {
      fetchAlertCount();
      // Refresh alert count every 30 seconds
      const interval = setInterval(fetchAlertCount, 30000);
      return () => clearInterval(interval);
    }
  }, [user?.role]);

  const handleLogout = async () => {
    try {
      await api.logout();
      router.push('/login');
    } catch (error) {
      console.error('Logout failed:', error);
      // Force logout even if API call fails
      api.clearToken();
      router.push('/login');
    }
  };

  const isActive = (href: string) => {
    if (href === '/dashboard') {
      return pathname === '/dashboard';
    }
    // Exact match OR starts with href followed by '/' (to avoid partial matches like /dashboard/sdk matching /dashboard/sdk-tokens)
    return pathname === href || pathname.startsWith(href + '/');
  };

  const SidebarContent = () => (
    <>
      {/* Logo */}
      <div className="flex items-center justify-between px-4 py-4 border-b border-gray-200 dark:border-gray-700">
        <Link href="/dashboard" className="flex items-center gap-3">
          <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center">
            <Shield className="h-5 w-5 text-white" />
          </div>
          {!collapsed && (
            <div className="flex flex-col">
              <span className="text-lg font-bold text-gray-900 dark:text-white">AIM</span>
              <span className="text-xs text-gray-500 dark:text-gray-400">Agent Identity</span>
            </div>
          )}
        </Link>
        {!collapsed && (
          <button
            onClick={() => setCollapsed(true)}
            className="lg:flex hidden p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
        {navigation.map((section, idx) => (
          <div key={idx} className="space-y-1">
            {section.title && !collapsed && (
              <h3 className="px-3 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                {section.title}
              </h3>
            )}
            <div className="space-y-1">
              {section.items.map((item) => {
                const active = isActive(item.href);
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    onClick={() => setMobileOpen(false)}
                    className={`
                      flex items-center gap-3 px-3 py-2 rounded-lg transition-all
                      ${active
                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
                      }
                      ${collapsed ? 'justify-center' : ''}
                    `}
                    title={collapsed ? item.name : undefined}
                  >
                    <item.icon className={`h-5 w-5 flex-shrink-0 ${active ? 'text-blue-600 dark:text-blue-400' : ''}`} />
                    {!collapsed && (
                      <>
                        <span className="flex-1 font-medium">{item.name}</span>
                        {item.badge && (
                          <span className="inline-flex items-center justify-center px-2 py-0.5 text-xs font-bold text-white bg-red-500 rounded-full">
                            {item.badge}
                          </span>
                        )}
                      </>
                    )}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* User Profile */}
      <div className="border-t border-gray-200 dark:border-gray-700 p-4">
        <div className={`flex items-center gap-3 ${collapsed ? 'justify-center' : ''}`}>
          <div className="w-9 h-9 bg-gradient-to-br from-purple-500 to-pink-500 rounded-full flex items-center justify-center text-white font-semibold">
            {user?.email?.[0]?.toUpperCase() || 'U'}
          </div>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                {user?.display_name || 'User Account'}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                {user?.email || 'Loading...'}
              </p>
            </div>
          )}
        </div>
        {!collapsed && (
          <button
            onClick={handleLogout}
            className="mt-3 w-full flex items-center gap-2 px-3 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
          >
            <LogOut className="h-4 w-4" />
            <span>Logout</span>
          </button>
        )}
        {collapsed && (
          <button
            onClick={() => setCollapsed(false)}
            className="mt-3 w-full flex items-center justify-center p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <Menu className="h-5 w-5" />
          </button>
        )}
      </div>
    </>
  );

  return (
    <>
      {/* Mobile Menu Button */}
      <button
        onClick={() => setMobileOpen(!mobileOpen)}
        className="lg:hidden fixed top-4 left-4 z-50 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg"
      >
        {mobileOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
      </button>

      {/* Mobile Overlay */}
      {mobileOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-black/50 z-40"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Desktop Sidebar */}
      <aside
        className={`
          hidden lg:flex flex-col bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700
          transition-all duration-300 ease-in-out
          ${collapsed ? 'w-20' : 'w-64'}
        `}
      >
        <SidebarContent />
      </aside>

      {/* Mobile Sidebar */}
      <aside
        className={`
          lg:hidden fixed top-0 left-0 bottom-0 z-40 w-64 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700
          transform transition-transform duration-300 ease-in-out
          ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <SidebarContent />
      </aside>
    </>
  );
}
