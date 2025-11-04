/**
 * Role-Based Access Control (RBAC) Permissions
 *
 * Defines what each role can access in the AIM platform.
 * Roles: admin, manager, member, viewer
 */

export type UserRole = "admin" | "manager" | "member" | "viewer";

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
export function hasPermission(
  userRole: UserRole | undefined,
  allowedRoles: UserRole[]
): boolean {
  if (!userRole) return false;
  return allowedRoles.includes(userRole);
}

/**
 * Get role display information
 */
export function getRoleInfo(role: UserRole) {
  const roleMap = {
    admin: {
      label: "Administrator",
      color: "text-purple-600 dark:text-purple-400",
      bgColor: "bg-purple-100 dark:bg-purple-900/20",
    },
    manager: {
      label: "Manager",
      color: "text-blue-600 dark:text-blue-400",
      bgColor: "bg-blue-100 dark:bg-blue-900/20",
    },
    member: {
      label: "Member",
      color: "text-green-600 dark:text-green-400",
      bgColor: "bg-green-100 dark:bg-green-900/20",
    },
    viewer: {
      label: "Viewer",
      color: "text-gray-600 dark:text-gray-400",
      bgColor: "bg-gray-100 dark:bg-gray-900/20",
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
    .map((section) => {
      const filteredItems = section.items.filter((item) =>
        hasPermission(userRole, item.roles)
      );

      // If section has no accessible items, exclude it
      if (filteredItems.length === 0) return null;

      return {
        ...section,
        items: filteredItems,
      };
    })
    .filter((section) => section !== null) as NavSection[];
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

    // Route-level permissions
    canViewAdmin: false,
    canViewAudit: false,
    canViewCapabilityRequests: false,
    canViewSecurity: false,
    canViewMonitoring: false,
    canViewAnalytics: false,
  };

  if (!userRole) return permissions;

  // Viewer: Limited read-only access
  if (userRole === "viewer") {
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
      canViewAdmin: false,
      canViewAudit: false,
      canViewCapabilityRequests: false,
      canViewSecurity: false,
      canViewMonitoring: false,
      canViewAnalytics: false,
    };
  }

  // Member: Can view their own agents and MCP servers
  if (userRole === "member") {
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
      canViewAdmin: false,
      canViewAudit: false,
      canViewCapabilityRequests: false,
      canViewSecurity: false,
      canViewMonitoring: false,
      canViewAnalytics: false,
    };
  }

  // Manager: Can view team-level stats and alerts
  if (userRole === "manager") {
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
      canViewAdmin: false,
      canViewAudit: true,
      canViewCapabilityRequests: true,
      canViewSecurity: true,
      canViewMonitoring: true,
      canViewAnalytics: true,
    };
  }

  // Admin: Full access to all stats
  if (userRole === "admin") {
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
      canViewAdmin: true,
      canViewAudit: true,
      canViewCapabilityRequests: true,
      canViewSecurity: true,
      canViewMonitoring: true,
      canViewAnalytics: true,
    };
  }

  return permissions;
}

/**
 * Agent permissions by role
 *
 * VIEWER: Cannot create/edit/delete
 * MEMBER: Can create agents/keys, cannot delete agents
 * MANAGER: Can verify/delete agents
 * ADMIN: Full access
 */
export function getAgentPermissions(userRole: UserRole | undefined) {
  const permissions = {
    canCreateAgent: false,
    canEditAgent: false,
    canDeleteAgent: false,
    canVerifyAgent: false,
    canViewAgent: false,
    canCreateAPIKey: false,
    canDeleteAPIKey: false,
    canDownloadSDK: false,
    canManageMCPServers: false,
  };

  if (!userRole) return permissions;

  // Viewer: Read-only access
  if (userRole === "viewer") {
    return {
      ...permissions,
      canViewAgent: true,
    };
  }

  // Member: Can create/edit agents and keys, but cannot delete agents
  if (userRole === "member") {
    return {
      ...permissions,
      canViewAgent: true,
      canCreateAgent: true,
      canEditAgent: true,
      canDeleteAgent: false, // Members cannot delete agents
      canVerifyAgent: false,
      canCreateAPIKey: true,
      canDeleteAPIKey: true,
      canDownloadSDK: true,
      canManageMCPServers: true,
    };
  }

  // Manager: Can verify and delete agents
  if (userRole === "manager") {
    return {
      ...permissions,
      canViewAgent: true,
      canCreateAgent: true,
      canEditAgent: true,
      canDeleteAgent: true, // Managers can delete agents
      canVerifyAgent: true, // Managers can verify agents
      canCreateAPIKey: true,
      canDeleteAPIKey: true,
      canDownloadSDK: true,
      canManageMCPServers: true,
    };
  }

  // Admin: Full access
  if (userRole === "admin") {
    return {
      canViewAgent: true,
      canCreateAgent: true,
      canEditAgent: true,
      canDeleteAgent: true,
      canVerifyAgent: true,
      canCreateAPIKey: true,
      canDeleteAPIKey: true,
      canDownloadSDK: true,
      canManageMCPServers: true,
    };
  }

  return permissions;
}

/**
 * MCP Server permissions by role
 */
export function getMCPPermissions(userRole: UserRole | undefined) {
  const permissions = {
    canCreateMCPServer: false,
    canEditMCPServer: false,
    canDeleteMCPServer: false,
    canViewMCPServer: false,
    canRegisterMCPServer: false,
  };

  if (!userRole) return permissions;

  // Viewer: Read-only access
  if (userRole === "viewer") {
    return {
      ...permissions,
      canViewMCPServer: true,
    };
  }

  // Member: Can create/edit MCP servers
  if (userRole === "member") {
    return {
      ...permissions,
      canViewMCPServer: true,
      canCreateMCPServer: true,
      canEditMCPServer: true,
      canDeleteMCPServer: false,
      canRegisterMCPServer: true,
    };
  }

  // Manager: Can delete MCP servers
  if (userRole === "manager") {
    return {
      canViewMCPServer: true,
      canCreateMCPServer: true,
      canEditMCPServer: true,
      canDeleteMCPServer: true,
      canRegisterMCPServer: true,
    };
  }

  // Admin: Full access
  if (userRole === "admin") {
    return {
      canViewMCPServer: true,
      canCreateMCPServer: true,
      canEditMCPServer: true,
      canDeleteMCPServer: true,
      canRegisterMCPServer: true,
    };
  }

  return permissions;
}
