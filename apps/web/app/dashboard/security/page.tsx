'use client';

import { useState, useEffect } from 'react';
import {
  Shield,
  AlertTriangle,
  Activity,
  TrendingUp,
  AlertOctagon,
  Eye,
  XCircle,
  CheckCircle,
  Loader2,
  AlertCircle
} from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar, Cell } from 'recharts';
import { api } from '@/lib/api';
import ThreatDetailModal from '@/components/modals/threat-detail-modal';
import { formatDateTime } from '@/lib/date-utils';

interface SecurityThreat {
  id: string;
  target_id: string;  // Agent or MCP server ID
  target_name?: string;  // Agent or MCP server name
  threat_type: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  description: string;
  is_blocked: boolean;
  created_at: string;
  // Additional fields for enhanced detail view
  source_ip?: string;
  metadata?: Record<string, any>;
}

interface SecurityIncident {
  id: string;
  title: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  status: 'open' | 'investigating' | 'resolved';
  created_at: string;
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

function SeverityBadge({ severity }: { severity: string }) {
  const getSeverityStyles = (severity: string) => {
    switch (severity) {
      case 'critical':
        return 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300';
      case 'high':
        return 'bg-orange-100 dark:bg-orange-900/30 text-orange-800 dark:text-orange-300';
      case 'medium':
        return 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300';
      case 'low':
        return 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getSeverityStyles(severity)}`}>
      {severity}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const getStatusStyles = (status: string) => {
    switch (status) {
      case 'active':
      case 'open':
        return 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300';
      case 'mitigated':
      case 'investigating':
        return 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300';
      case 'resolved':
        return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300';
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
        <p className="text-sm text-gray-500 dark:text-gray-400">Loading security data...</p>
      </div>
    </div>
  );
}

function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
  const is403 = message.includes('403');

  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center px-4">
        <Shield className={`h-16 w-16 ${is403 ? 'text-amber-500' : 'text-red-500'}`} />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            {is403 ? 'Access Restricted' : 'Failed to Load Security Data'}
          </h3>
          {is403 ? (
            <div className="space-y-3">
              <p className="text-base text-gray-600 dark:text-gray-400">
                Security monitoring is only available to <strong>Admin</strong> and <strong>Manager</strong> roles.
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-500">
                To view security threats, incidents, and metrics, please contact your organization administrator to upgrade your account permissions.
              </p>
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">{message}</p>
          )}
        </div>
        {!is403 && (
          <button
            onClick={onRetry}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            Retry
          </button>
        )}
      </div>
    </div>
  );
}

export default function SecurityPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [threats, setThreats] = useState<SecurityThreat[]>([]);
  const [incidents, setIncidents] = useState<SecurityIncident[]>([]);
  const [metrics, setMetrics] = useState<any>(null);

  // Modal states
  const [selectedThreat, setSelectedThreat] = useState<SecurityThreat | null>(null);
  const [showThreatModal, setShowThreatModal] = useState(false);

  const fetchSecurityData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [threatsData, incidentsData, metricsData] = await Promise.all([
        api.getSecurityThreats(),
        api.getSecurityIncidents(),
        api.getSecurityMetrics()
      ]);
      setThreats(threatsData.threats || []);
      setIncidents(incidentsData.incidents || []);
      setMetrics(metricsData);
    } catch (err) {
      console.error('Failed to fetch security data:', err);
      setError(err instanceof Error ? err.message : 'An unknown error occurred');
      // No mock data fallback - show error instead
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSecurityData();
  }, []);

  // Handler for viewing threat details
  const handleViewThreat = (threat: SecurityThreat) => {
    setSelectedThreat(threat);
    setShowThreatModal(true);
  };

  if (loading) {
    return <LoadingSpinner />;
  }

  if (error && !metrics) {
    return <ErrorDisplay message={error} onRetry={fetchSecurityData} />;
  }

  const stats = [
    {
      name: 'Total Threats',
      value: metrics?.total_threats?.toLocaleString() || '0',
      change: '+15.2%',
      changeType: 'negative',
      icon: AlertTriangle,
    },
    {
      name: 'Active Threats',
      value: metrics?.active_threats?.toLocaleString() || '0',
      change: '-12.5%',
      changeType: 'positive',
      icon: AlertOctagon,
    },
    {
      name: 'Critical Incidents',
      value: incidents.filter(i => i.severity === 'critical').length.toLocaleString(),
      change: '+5.1%',
      changeType: 'negative',
      icon: XCircle,
    },
    {
      name: 'Anomalies Detected',
      value: metrics?.total_anomalies?.toLocaleString() || '0',
      change: '+8.3%',
      changeType: 'negative',
      icon: Activity,
    },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Security Dashboard</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Monitor security threats, incidents, and anomalies across all agents.
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

      {/* Charts */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Threat Trend */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Threat Trend (30 Days)</h3>
            <TrendingUp className="h-5 w-5 text-gray-400" />
          </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={metrics?.threat_trend || []}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                <XAxis
                  dataKey="date"
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
                <Line type="monotone" dataKey="count" stroke="#ef4444" strokeWidth={2} name="Threats" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Severity Distribution */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Severity Distribution</h3>
            <Shield className="h-5 w-5 text-gray-400" />
          </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={metrics?.severity_distribution || []}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                <XAxis
                  dataKey="severity"
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
                <Bar
                  dataKey="count"
                  name="Count"
                  radius={[4, 4, 0, 0]}
                >
                  {(metrics?.severity_distribution || []).map((entry: any, index: number) => {
                    const colors: Record<string, string> = {
                      'Critical': '#dc2626',
                      'High': '#ea580c',
                      'Medium': '#f59e0b',
                      'Low': '#3b82f6'
                    };
                    return <Cell key={`cell-${index}`} fill={colors[entry.severity] || '#3b82f6'} />;
                  })}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* Recent Threats */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Recent Threats</h3>
            <AlertTriangle className="h-5 w-5 text-gray-400" />
          </div>
        </div>
        <div className="overflow-hidden">
          <table className="w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[30%]">
                  Threat Type
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[15%]">
                  Agent
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[12%]">
                  Severity
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[12%]">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[18%]">
                  Detected At
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[8%]">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {threats.slice(0, 5).map((threat) => (
                <tr key={threat.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-4 py-3">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100 break-words">
                      {threat.threat_type}
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400 line-clamp-2">
                      {threat.description}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="text-sm text-gray-900 dark:text-gray-100 truncate" title={threat.target_name || threat.target_id}>
                      {threat.target_name || `ID: ${threat.target_id.substring(0, 8)}...`}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <SeverityBadge severity={threat.severity} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <StatusBadge status={threat.is_blocked ? 'resolved' : 'active'} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {formatDateTime(threat.created_at)}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <button
                      onClick={() => handleViewThreat(threat)}
                      className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                      title="View threat details"
                    >
                      <Eye className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Recent Incidents */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Security Incidents</h3>
            <AlertOctagon className="h-5 w-5 text-gray-400" />
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Title
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Severity
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Created At
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {incidents.map((incident) => (
                <tr key={incident.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {incident.title}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <SeverityBadge severity={incident.severity} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={incident.status} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {formatDateTime(incident.created_at)}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Modals */}
      <ThreatDetailModal
        isOpen={showThreatModal}
        onClose={() => {
          setShowThreatModal(false);
          setSelectedThreat(null);
        }}
        threat={selectedThreat}
      />
    </div>
  );
}
