/**
 * Role-Based Access Control (RBAC) Permissions
 *
 * Defines what each role can access in the AIM platform.
 * Roles: admin, manager, member, viewer
 */

export type UserRole = 'admin' | 'manager' | 'member' | 'viewer';

export interface NavItem {
  name: string;
  href: string;
  icon: any;
  roles: UserRole[];
  badge?: number;
}

export interface NavSection {
  title?: string;
  items: NavItem[];
}

/**
 * Check if a user role has permission to access a specific route
 */
export function hasPermission(userRole: UserRole | undefined, allowedRoles: UserRole[]): boolean {
  if (!userRole) return false;
  return allowedRoles.includes(userRole);
}

/**
 * Get role display information
 */
export function getRoleInfo(role: UserRole) {
  const roleMap = {
    admin: {
      label: 'Administrator',
      color: 'text-purple-600 dark:text-purple-400',
      bgColor: 'bg-purple-100 dark:bg-purple-900/20',
    },
    manager: {
      label: 'Manager',
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-100 dark:bg-blue-900/20',
    },
    member: {
      label: 'Member',
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-100 dark:bg-green-900/20',
    },
    viewer: {
      label: 'Viewer',
      color: 'text-gray-600 dark:text-gray-400',
      bgColor: 'bg-gray-100 dark:bg-gray-900/20',
    },
  };

  return roleMap[role] || roleMap.viewer;
}

/**
 * Filter navigation items based on user role
 */
export function filterNavigationByRole(
  navigation: NavSection[],
  userRole: UserRole | undefined
): NavSection[] {
  if (!userRole) return [];

  return navigation
    .map(section => {
      const filteredItems = section.items.filter(item =>
        hasPermission(userRole, item.roles)
      );

      // If section has no accessible items, exclude it
      if (filteredItems.length === 0) return null;

      return {
        ...section,
        items: filteredItems,
      };
    })
    .filter(section => section !== null) as NavSection[];
}

/**
 * Dashboard permissions by role
 */
export function getDashboardPermissions(userRole: UserRole | undefined) {
  const permissions = {
    // Stat cards visibility
    canViewAgentStats: false,
    canViewMCPStats: false,
    canViewTrustScore: false,
    canViewAlerts: false,
    canViewUserStats: false,
    canViewSecurityMetrics: false,

    // Chart visibility
    canViewTrustTrend: false,
    canViewActivityChart: false,

    // Table visibility
    canViewRecentActivity: false,
    canViewDetailedMetrics: false,
  };

  if (!userRole) return permissions;

  // Viewer: Limited read-only access
  if (userRole === 'viewer') {
    return {
      canViewAgentStats: true,
      canViewMCPStats: true,
      canViewTrustScore: true,
      canViewAlerts: false,
      canViewUserStats: false,
      canViewSecurityMetrics: false,
      canViewTrustTrend: true,
      canViewActivityChart: true,
      canViewRecentActivity: true,
      canViewDetailedMetrics: false,
    };
  }

  // Member: Can view their own agents and MCP servers
  if (userRole === 'member') {
    return {
      canViewAgentStats: true,
      canViewMCPStats: true,
      canViewTrustScore: true,
      canViewAlerts: false,
      canViewUserStats: false,
      canViewSecurityMetrics: false,
      canViewTrustTrend: true,
      canViewActivityChart: true,
      canViewRecentActivity: true,
      canViewDetailedMetrics: true,
    };
  }

  // Manager: Can view team-level stats and alerts
  if (userRole === 'manager') {
    return {
      canViewAgentStats: true,
      canViewMCPStats: true,
      canViewTrustScore: true,
      canViewAlerts: true,
      canViewUserStats: true,
      canViewSecurityMetrics: true,
      canViewTrustTrend: true,
      canViewActivityChart: true,
      canViewRecentActivity: true,
      canViewDetailedMetrics: true,
    };
  }

  // Admin: Full access to all stats
  if (userRole === 'admin') {
    return {
      canViewAgentStats: true,
      canViewMCPStats: true,
      canViewTrustScore: true,
      canViewAlerts: true,
      canViewUserStats: true,
      canViewSecurityMetrics: true,
      canViewTrustTrend: true,
      canViewActivityChart: true,
      canViewRecentActivity: true,
      canViewDetailedMetrics: true,
    };
  }

  return permissions;
}
