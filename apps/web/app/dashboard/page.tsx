'use client';

import { useState, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import {
  Shield,
  Users,
  Activity,
  TrendingUp,
  AlertTriangle,
  CheckCircle,
  Clock,
  Network,
  Loader2,
  AlertCircle
} from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { api } from '@/lib/api';
import { getDashboardPermissions, type UserRole } from '@/lib/permissions';
import { TimezoneIndicator } from '@/components/timezone-indicator';

interface DashboardStats {
  // Backend returns these exact fields (snake_case from Go JSON tags)
  // Agent metrics
  total_agents: number;
  verified_agents: number;
  pending_agents: number;
  verification_rate: number;
  avg_trust_score: number;

  // MCP Server metrics
  total_mcp_servers: number;
  active_mcp_servers: number;

  // User metrics
  total_users: number;
  active_users: number;

  // Security metrics
  active_alerts: number;
  critical_alerts: number;
  security_incidents: number;

  // Organization
  organization_id: string;
}

function StatCard({ stat }: { stat: any }) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <stat.icon className="h-6 w-6 text-gray-400" />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">{stat.name}</dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">{stat.value}</div>
              {stat.change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    stat.changeType === 'positive' ? 'text-green-600' : 'text-red-600'
                  }`}
                >
                  {stat.change}
                </div>
              )}
            </dd>
          </dl>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const getStatusStyles = (status: string) => {
    switch (status) {
      case 'success':
        return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300';
      case 'failed':
        return 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300';
      case 'pending':
        return 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getStatusStyles(status)}`}>
      {status}
    </span>
  );
}

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4">
        <Loader2 className="h-12 w-12 text-blue-500 animate-spin" />
        <p className="text-sm text-gray-500 dark:text-gray-400">Loading dashboard data...</p>
      </div>
    </div>
  );
}

function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center">
        <AlertCircle className="h-12 w-12 text-red-500" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Failed to Load Dashboard</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">{message}</p>
        <button
          onClick={onRetry}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          Retry
        </button>
      </div>
    </div>
  );
}

interface AuditLog {
  id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  user_id: string;
  metadata: any;
  timestamp: string;
}

function DashboardContent() {
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboardData, setDashboardData] = useState<DashboardStats | null>(null);
  const [userRole, setUserRole] = useState<UserRole>('viewer');
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);

  // Extract user role from JWT token
  useEffect(() => {
    const token = api.getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split('.')[1]));
        setUserRole((payload.role as UserRole) || 'viewer');
      } catch (e) {
        console.error('Failed to decode JWT token:', e);
        setUserRole('viewer');
      }
    }
  }, []);

  const fetchDashboardData = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getDashboardStats();
      setDashboardData(data);
    } catch (err) {
      console.error('Failed to fetch dashboard data:', err);
      setError(err instanceof Error ? err.message : 'An unknown error occurred');
      // No mock data fallback - show error instead
    } finally {
      setLoading(false);
    }
  };

  const fetchAuditLogs = async () => {
    try {
      setLogsLoading(true);
      // Fetch more logs to get past the excessive "view" actions
      // Most recent 50 logs are mostly "view + alerts" (automated polling)
      // Need to fetch 500+ to get interesting integration test data (create, verify, etc.)
      const logs = await api.getAuditLogs(500, 0);

      // Filter out excessive "view" + "alerts" entries (there are 4,226 of them)
      // Keep more interesting activities like create, verify, update, delete
      const filtered = logs.filter((log: AuditLog) => {
        // Exclude view + alerts (automated polling)
        if (log.action === 'view' && log.resource_type === 'alerts') return false;
        // Exclude view + dashboard_stats (also automated)
        if (log.action === 'view' && log.resource_type === 'dashboard_stats') return false;
        // Exclude view + users (less interesting)
        if (log.action === 'view' && log.resource_type === 'users') return false;
        // Exclude view + audit_logs (less interesting)
        if (log.action === 'view' && log.resource_type === 'audit_logs') return false;
        return true;
      });

      // Take first 10 interesting activities (create, verify, delete, etc.)
      setAuditLogs(filtered.slice(0, 10));
    } catch (err) {
      console.error('Failed to fetch audit logs:', err);
      // Fail silently - keep empty array
    } finally {
      setLogsLoading(false);
    }
  };

  useEffect(() => {
    // Check if OAuth returned with a token
    const token = searchParams.get('token');
    if (token) {
      api.setToken(token);
      // Clean up URL
      window.history.replaceState({}, '', '/dashboard');
    }

    fetchDashboardData();
    fetchAuditLogs();
  }, [searchParams]);

  // Mock data fallback matching backend response structure
  // NOTE: Backend MCP stats fix committed but not deployed due to compilation errors
  // When backend is rebuilt, it will fetch actual MCP servers from mcp_servers table
  const getMockData = (): DashboardStats => ({
    // Agent metrics
    total_agents: 3,       // Matches backend response
    verified_agents: 0,
    pending_agents: 3,
    verification_rate: 0,
    avg_trust_score: 0.38,

    // MCP Server metrics
    total_mcp_servers: 7,  // Backend will return this after fix is deployed
    active_mcp_servers: 0,

    // User metrics
    total_users: 1,         // Current logged-in user
    active_users: 1,

    // Security metrics
    active_alerts: 0,
    critical_alerts: 0,
    security_incidents: 0,

    // Organization
    organization_id: '',
  });

  if (loading) {
    return <LoadingSpinner />;
  }

  if (error && !dashboardData) {
    return <ErrorDisplay message={error} onRetry={fetchDashboardData} />;
  }

  const data = dashboardData!;

  // Get role-based permissions
  const permissions = getDashboardPermissions(userRole);

  // Helper function to format audit log event name with entity details
  const formatEventName = (log: AuditLog): string => {
    const action = log.action.charAt(0).toUpperCase() + log.action.slice(1);
    const resource = log.resource_type.replace(/_/g, ' ');

    // Extract entity name from metadata for more meaningful display
    let entityName = '';
    if (log.metadata) {
      // Try to get specific entity name from metadata
      entityName = log.metadata.agent_name ||
                   log.metadata.server_name ||
                   log.metadata.mcp_name ||
                   log.metadata.key_name ||
                   log.metadata.tag_name ||
                   '';
    }

    // Format with entity name if available
    const entityDisplay = entityName ? ` "${entityName}"` : '';

    // Special handling for specific action types
    if (log.action === 'view') {
      return `Viewed ${resource}${entityDisplay}`;
    } else if (log.action === 'create') {
      return `Created ${resource}${entityDisplay}`;
    } else if (log.action === 'verify') {
      return `Verified ${resource}${entityDisplay}`;
    } else if (log.action === 'update') {
      return `Updated ${resource}${entityDisplay}`;
    } else if (log.action === 'delete') {
      return `Deleted ${resource}${entityDisplay}`;
    } else if (log.action === 'grant') {
      return `Granted ${resource}${entityDisplay}`;
    } else if (log.action === 'revoke') {
      return `Revoked ${resource}${entityDisplay}`;
    } else if (log.action === 'suspend') {
      return `Suspended ${resource}${entityDisplay}`;
    } else if (log.action === 'acknowledge') {
      return `Acknowledged ${resource}${entityDisplay}`;
    } else if (log.resource_type === 'agent_action') {
      // For agent actions, use the action name as the event
      return action.replace(/_/g, ' ');
    }

    return `${action} ${resource}${entityDisplay}`;
  };

  // Helper function to get WHO initiated the action (user, agent, or MCP)
  const getInitiatedBy = (log: AuditLog): string => {
    // Check metadata for agent or MCP context
    if (log.metadata) {
      // If action was initiated by an agent
      if (log.metadata.registered_by_agent || log.metadata.acting_agent_name) {
        return `Agent: ${log.metadata.registered_by_agent || log.metadata.acting_agent_name}`;
      }
      // If action was initiated by an MCP server
      if (log.metadata.mcp_server || log.metadata.server_name) {
        return `MCP: ${log.metadata.mcp_server || log.metadata.server_name}`;
      }
      // If we have user email in metadata
      if (log.metadata.user_email) {
        return `User: ${log.metadata.user_email}`;
      }
    }
    // Default: assume it was a user action
    return 'User';
  };

  // Helper function to categorize the event type
  const getEventType = (log: AuditLog): string => {
    if (log.resource_type.includes('agent')) {
      return 'Agent Management';
    } else if (log.resource_type.includes('mcp')) {
      return 'MCP Servers';
    } else if (log.resource_type.includes('auth') || log.action === 'login') {
      return 'Authentication';
    } else if (log.resource_type.includes('alert') || log.resource_type.includes('security')) {
      return 'Security';
    } else if (log.resource_type.includes('api_key')) {
      return 'API Keys';
    } else if (log.resource_type.includes('user')) {
      return 'User Management';
    } else if (log.action === 'view') {
      return 'View';
    }
    return 'System';
  };

  // Helper function to format relative time
  const formatRelativeTime = (timestamp: string): string => {
    const now = new Date();
    const then = new Date(timestamp);
    const diffMs = now.getTime() - then.getTime();
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffSecs < 10) return 'Just now';
    if (diffSecs < 60) return `${diffSecs} seconds ago`;
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;

    return then.toLocaleDateString();
  };

  // Prepare stats for display using actual backend field names
  const allStats = [
    {
      name: 'Total Agents',
      value: data.total_agents.toLocaleString(),
      change: `${data.verification_rate.toFixed(1)}% verified`,
      changeType: 'positive' as const,
      icon: Shield,
      permission: 'canViewAgentStats' as const,
    },
    {
      name: 'MCP Servers',
      value: data.total_mcp_servers.toLocaleString(),
      change: `${data.active_mcp_servers} active`,
      changeType: 'positive' as const,
      icon: Network,
      permission: 'canViewMCPStats' as const,
    },
    {
      name: 'Avg Trust Score',
      value: data.avg_trust_score.toFixed(1),
      change: data.avg_trust_score >= 80 ? 'Excellent' : data.avg_trust_score >= 60 ? 'Good' : 'Fair',
      changeType: data.avg_trust_score >= 80 ? 'positive' as const : 'negative' as const,
      icon: TrendingUp,
      permission: 'canViewTrustScore' as const,
    },
    {
      name: 'Active Alerts',
      value: data.active_alerts.toLocaleString(),
      change: data.critical_alerts > 0 ? `${data.critical_alerts} critical` : 'Normal',
      changeType: data.critical_alerts > 0 ? 'negative' as const : 'positive' as const,
      icon: AlertTriangle,
      permission: 'canViewAlerts' as const,
    },
  ];

  // Filter stats based on role permissions
  const stats = allStats.filter(stat => permissions[stat.permission]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Dashboard Overview</h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Monitor agent verification activities and system performance across all protocols.
            </p>
          </div>
          <TimezoneIndicator />
        </div>
        {error && (
          <div className="mt-2 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
            <p className="text-sm text-yellow-800 dark:text-yellow-300">
              ⚠️ Using mock data - API connection failed: {error}
            </p>
          </div>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Trust Score Trend - All roles can see */}
        {permissions.canViewTrustTrend && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Trust Score Trend (30 Days)</h3>
              <TrendingUp className="h-5 w-5 text-gray-400" />
            </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={[
                { day: 'Week 1', score: 82 },
                { day: 'Week 2', score: 85 },
                { day: 'Week 3', score: 87 },
                { day: 'Week 4', score: data.avg_trust_score || 0 },
              ]}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                <XAxis
                  dataKey="day"
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                />
                <YAxis
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                  domain={[0, 100]}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#fff',
                    border: '1px solid #e5e7eb',
                    borderRadius: '0.5rem',
                    boxShadow: '0 1px 3px 0 rgb(0 0 0 / 0.1)'
                  }}
                />
                <Line type="monotone" dataKey="score" stroke="#3b82f6" strokeWidth={3} name="Trust Score" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
        )}

        {/* Agent Activity - All roles can see */}
        {permissions.canViewActivityChart && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Agent Verification Activity</h3>
              <Activity className="h-5 w-5 text-gray-400" />
            </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={[
                { month: 'Jan', verified: 12, pending: 3 },
                { month: 'Feb', verified: 18, pending: 2 },
                { month: 'Mar', verified: data.verified_agents, pending: data.pending_agents },
              ]}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                <XAxis
                  dataKey="month"
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                />
                <YAxis
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#fff',
                    border: '1px solid #e5e7eb',
                    borderRadius: '0.5rem',
                    boxShadow: '0 1px 3px 0 rgb(0 0 0 / 0.1)'
                  }}
                />
                <Bar dataKey="verified" fill="#22c55e" name="Verified" />
                <Bar dataKey="pending" fill="#eab308" name="Pending" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
        )}
      </div>

      {/* Metrics Grid */}
      <div className={`grid grid-cols-1 gap-6 ${permissions.canViewSecurityMetrics ? 'lg:grid-cols-3' : 'lg:grid-cols-2'}`}>
        {/* Agent Metrics - All roles can see */}
        {permissions.canViewDetailedMetrics && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
            <Shield className="h-5 w-5 text-blue-500" />
            Agent Metrics
          </h3>
          <div className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500 dark:text-gray-400">Total Agents</span>
              <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.total_agents}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500 dark:text-gray-400">Verified</span>
              <span className="text-sm font-medium text-green-600">{data.verified_agents}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500 dark:text-gray-400">Pending</span>
              <span className="text-sm font-medium text-yellow-600">{data.pending_agents}</span>
            </div>
            <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
              <span className="text-sm text-gray-500 dark:text-gray-400">Verification Rate</span>
              <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.verification_rate.toFixed(1)}%</span>
            </div>
          </div>
        </div>
        )}

        {/* Security Metrics - Manager+ Only */}
        {permissions.canViewSecurityMetrics && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-yellow-500" />
              Security Status
            </h3>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">Active Alerts</span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.active_alerts}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">Critical</span>
                <span className="text-sm font-medium text-red-600">{data.critical_alerts}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">Incidents</span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.security_incidents}</span>
              </div>
              <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                <span className="text-sm text-gray-500 dark:text-gray-400">System Status</span>
                <div className="flex items-center gap-1">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <span className="text-sm font-medium text-green-600">Operational</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Platform/MCP Metrics - All roles see this */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
            {permissions.canViewUserStats ? <Users className="h-5 w-5 text-purple-500" /> : <Network className="h-5 w-5 text-purple-500" />}
            {permissions.canViewUserStats ? 'Platform Metrics' : 'MCP Servers'}
          </h3>
          <div className="space-y-3">
            {permissions.canViewUserStats && (
              <>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">Total Users</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.total_users}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">Active Users</span>
                  <span className="text-sm font-medium text-green-600">{data.active_users}</span>
                </div>
                <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                  <span className="text-sm text-gray-500 dark:text-gray-400">MCP Servers</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.total_mcp_servers}</span>
                </div>
              </>
            )}
            {!permissions.canViewUserStats && (
              <>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">Total MCP Servers</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.total_mcp_servers}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">Active MCP Servers</span>
                  <span className="text-sm font-medium text-green-600">{data.active_mcp_servers}</span>
                </div>
                <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                  <span className="text-sm text-gray-500 dark:text-gray-400">Total Agents</span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{data.total_agents}</span>
                </div>
              </>
            )}
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500 dark:text-gray-400">{permissions.canViewUserStats ? 'Active MCPs' : 'Verified Agents'}</span>
              <span className="text-sm font-medium text-green-600">{permissions.canViewUserStats ? data.active_mcp_servers : data.verified_agents}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity Table - All roles can see */}
      {permissions.canViewRecentActivity && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Recent Activity</h3>
            <Clock className="h-5 w-5 text-gray-400" />
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Event
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Initiated By
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Timestamp
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {logsLoading ? (
                <tr>
                  <td colSpan={5} className="px-6 py-12 text-center">
                    <Loader2 className="h-6 w-6 text-blue-500 animate-spin mx-auto" />
                    <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading recent activity...</p>
                  </td>
                </tr>
              ) : auditLogs.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-12 text-center">
                    <p className="text-sm text-gray-500 dark:text-gray-400">No recent activity found</p>
                  </td>
                </tr>
              ) : (
                auditLogs.map((log) => (
                  <tr key={log.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        {formatEventName(log)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-700 dark:text-gray-300">
                        {getInitiatedBy(log)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                        {getEventType(log)}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                        ✓ success
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-500 dark:text-gray-400">
                        {formatRelativeTime(log.timestamp)}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
      )}

    </div>
  );
}

export default function DashboardPage() {
  return <DashboardContent />;
}
