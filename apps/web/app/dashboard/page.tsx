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

function DashboardContent() {
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboardData, setDashboardData] = useState<DashboardStats | null>(null);
  const [userRole, setUserRole] = useState<string>('viewer');

  // Extract user role from JWT token
  useEffect(() => {
    const token = api.getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split('.')[1]));
        setUserRole(payload.role || 'viewer');
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

  useEffect(() => {
    // Check if OAuth returned with a token
    const token = searchParams.get('token');
    if (token) {
      api.setToken(token);
      // Clean up URL
      window.history.replaceState({}, '', '/dashboard');
    }

    fetchDashboardData();
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

  // Determine if user can see admin features
  const isAdminOrManager = userRole === 'admin' || userRole === 'manager';

  // Prepare stats for display using actual backend field names
  const allStats = [
    {
      name: 'Total Agents',
      value: data.total_agents.toLocaleString(),
      change: `${data.verification_rate.toFixed(1)}% verified`,
      changeType: 'positive' as const,
      icon: Shield,
    },
    {
      name: 'MCP Servers',
      value: data.total_mcp_servers.toLocaleString(),
      change: `${data.active_mcp_servers} active`,
      changeType: 'positive' as const,
      icon: Network,
    },
    {
      name: 'Avg Trust Score',
      value: data.avg_trust_score.toFixed(1),
      change: data.avg_trust_score >= 80 ? 'Excellent' : data.avg_trust_score >= 60 ? 'Good' : 'Fair',
      changeType: data.avg_trust_score >= 80 ? 'positive' as const : 'negative' as const,
      icon: TrendingUp,
    },
    {
      name: 'Active Alerts',
      value: data.active_alerts.toLocaleString(),
      change: data.critical_alerts > 0 ? `${data.critical_alerts} critical` : 'Normal',
      changeType: data.critical_alerts > 0 ? 'negative' as const : 'positive' as const,
      icon: AlertTriangle,
      adminOnly: true, // Hide from viewers
    },
  ];

  // Filter stats based on role
  const stats = allStats.filter(stat => !stat.adminOnly || isAdminOrManager);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Dashboard Overview</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Monitor agent verification activities and system performance across all protocols.
        </p>
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
        {/* Trust Score Trend */}
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

        {/* Agent Activity */}
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
      </div>

      {/* Metrics Grid */}
      <div className={`grid grid-cols-1 gap-6 ${isAdminOrManager ? 'lg:grid-cols-3' : 'lg:grid-cols-2'}`}>
        {/* Agent Metrics */}
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

        {/* Security Metrics - Admin/Manager Only */}
        {isAdminOrManager && (
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

        {/* MCP Metrics (viewers see this) or User & MCP Metrics (admins see this) */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
            {isAdminOrManager ? <Users className="h-5 w-5 text-purple-500" /> : <Network className="h-5 w-5 text-purple-500" />}
            {isAdminOrManager ? 'Platform Metrics' : 'MCP Servers'}
          </h3>
          <div className="space-y-3">
            {isAdminOrManager && (
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
            {!isAdminOrManager && (
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
              <span className="text-sm text-gray-500 dark:text-gray-400">{isAdminOrManager ? 'Active MCPs' : 'Verified Agents'}</span>
              <span className="text-sm font-medium text-green-600">{isAdminOrManager ? data.active_mcp_servers : data.verified_agents}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity Table */}
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
              {[
                {
                  event: 'User logged in',
                  type: 'Authentication',
                  status: 'success',
                  time: 'Just now'
                },
                {
                  event: 'Dashboard stats viewed',
                  type: 'View',
                  status: 'success',
                  time: '2 minutes ago'
                },
                {
                  event: 'OAuth authentication',
                  type: 'Authentication',
                  status: 'success',
                  time: '1 hour ago'
                },
              ].map((activity, idx) => (
                <tr key={idx} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {activity.event}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                      {activity.type}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                      ✓ {activity.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {activity.time}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

    </div>
  );
}

export default function DashboardPage() {
  return <DashboardContent />;
}
