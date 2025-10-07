'use client';

import { useState, useEffect } from 'react';
import {
  Server,
  CheckCircle2,
  XCircle,
  Clock,
  Plus,
  Shield,
  Edit,
  Trash2,
  Loader2,
  AlertCircle,
  Globe,
  Eye
} from 'lucide-react';
import { api } from '@/lib/api';
import { RegisterMCPModal } from '@/components/modals/register-mcp-modal';
import { MCPDetailModal } from '@/components/modals/mcp-detail-modal';
import { formatDateTime } from '@/lib/date-utils';

interface MCPServer {
  id: string;
  name: string;
  url: string;
  description?: string;
  status: 'active' | 'inactive';
  public_key?: string;
  key_type?: string;
  last_verified_at?: string;
  created_at: string;
  trust_score?: number;
  capability_count?: number;
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
      case 'active':
        return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300';
      case 'inactive':
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
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
        <p className="text-sm text-gray-500 dark:text-gray-400">Loading MCP servers...</p>
      </div>
    </div>
  );
}

function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center">
        <AlertCircle className="h-12 w-12 text-red-500" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Failed to Load MCP Servers</h3>
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

export default function MCPServersPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([]);

  // Modal state
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedMCP, setSelectedMCP] = useState<MCPServer | null>(null);
  const [editingMCP, setEditingMCP] = useState<MCPServer | null>(null);

  const fetchMCPServers = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listMCPServers();
      // Backend returns "servers" not "mcp_servers"
      setMcpServers(data.servers || data.mcp_servers || []);
    } catch (err) {
      console.error('Failed to fetch MCP servers:', err);
      setError(err instanceof Error ? err.message : 'An unknown error occurred');
      // For development, use mock data as fallback
      setMcpServers(getMockMCPServers());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMCPServers();
  }, []);

  // Mock data fallback for development
  const getMockMCPServers = (): MCPServer[] => [
    {
      id: 'mcp_001',
      name: 'File Operations Server',
      url: 'https://mcp.example.com/filesystem',
      description: 'Provides secure file system operations and management',
      status: 'active',
      public_key: '-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----',
      key_type: 'RSA-2048',
      last_verified_at: '2025-01-20T14:30:00Z',
      created_at: '2025-01-15T10:00:00Z',
      trust_score: 95.0,
      capability_count: 8
    },
    {
      id: 'mcp_002',
      name: 'Database Connector',
      url: 'https://mcp.example.com/database',
      description: 'Database access and query execution service',
      status: 'active',
      public_key: '-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----',
      key_type: 'Ed25519',
      last_verified_at: '2025-01-20T12:15:00Z',
      created_at: '2025-01-16T11:30:00Z',
      trust_score: 88.5,
      capability_count: 12
    },
    {
      id: 'mcp_003',
      name: 'Cloud Storage Gateway',
      url: 'https://mcp.example.com/cloud-storage',
      status: 'active',
      last_verified_at: '2025-01-20T16:00:00Z',
      created_at: '2025-01-17T09:00:00Z',
      trust_score: 92.0,
      capability_count: 6
    },
    {
      id: 'mcp_004',
      name: 'API Integration Hub',
      url: 'https://mcp.example.com/api-hub',
      status: 'inactive',
      created_at: '2025-01-19T15:45:00Z',
      trust_score: 45.0,
      capability_count: 0
    },
    {
      id: 'mcp_005',
      name: 'Analytics Service',
      url: 'https://mcp.example.com/analytics',
      status: 'active',
      last_verified_at: '2025-01-20T10:30:00Z',
      created_at: '2025-01-18T14:20:00Z',
      trust_score: 78.5,
      capability_count: 15
    },
    {
      id: 'mcp_006',
      name: 'Legacy Systems Bridge',
      url: 'https://mcp.example.com/legacy',
      status: 'inactive',
      last_verified_at: '2025-01-19T08:00:00Z',
      created_at: '2025-01-12T16:00:00Z',
      trust_score: 35.0,
      capability_count: 4
    }
  ];

  // Calculate stats
  const stats = {
    total: mcpServers.length,
    active: mcpServers.filter(s => s.status === 'active').length,
    avgTrustScore: mcpServers.reduce((sum, s) => sum + (s.trust_score || 0), 0) / mcpServers.length,
    lastActivity: mcpServers
      .filter(s => s.last_verified_at)
      .sort((a, b) => new Date(b.last_verified_at!).getTime() - new Date(a.last_verified_at!).getTime())[0]?.last_verified_at
  };

  const statCards = [
    {
      name: 'Total MCP Servers',
      value: stats.total.toLocaleString(),
      change: '+15.3%',
      changeType: 'positive',
      icon: Server,
    },
    {
      name: 'Active Servers',
      value: stats.active.toLocaleString(),
      change: '+8.7%',
      changeType: 'positive',
      icon: CheckCircle2,
    },
    {
      name: 'Avg Trust Score',
      value: stats.avgTrustScore.toFixed(1),
      change: stats.avgTrustScore >= 75 ? '+5.2%' : '-2.1%',
      changeType: stats.avgTrustScore >= 75 ? 'positive' : 'negative',
      icon: Shield,
    },
    {
      name: 'Last Activity',
      value: stats.lastActivity ? formatRelativeTime(stats.lastActivity) : 'N/A',
      icon: Clock,
    },
  ];

  function formatRelativeTime(dateString: string): string {
    const now = new Date();
    const date = new Date(dateString);
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  }


  // Handlers
  const handleServerCreated = (newServer: any) => {
    setMcpServers([newServer, ...mcpServers]);
    setShowRegisterModal(false);
  };

  const handleViewMCP = (mcp: MCPServer) => {
    setSelectedMCP(mcp);
    setShowDetailModal(true);
  };

  const handleEditMCP = (mcp: MCPServer) => {
    setEditingMCP(mcp);
    setShowDetailModal(false);
    setShowRegisterModal(true);
  };

  const handleDeleteMCP = async (mcp: MCPServer) => {
    if (!confirm(`Are you sure you want to delete ${mcp.name}?`)) return;

    try {
      await api.deleteMCPServer(mcp.id);
      setMcpServers(mcpServers.filter(s => s.id !== mcp.id));
      setShowDetailModal(false);
    } catch (err) {
      console.error('Failed to delete MCP server:', err);
      alert('Failed to delete MCP server');
    }
  };


  if (loading) {
    return <LoadingSpinner />;
  }

  if (error && mcpServers.length === 0) {
    return <ErrorDisplay message={error} onRetry={fetchMCPServers} />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">MCP Servers</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage Model Context Protocol (MCP) servers and their cryptographic verification status.
          </p>
          {error && (
            <div className="mt-2 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
              <p className="text-sm text-yellow-800 dark:text-yellow-300">
                ⚠️ Using mock data - API connection failed: {error}
              </p>
            </div>
          )}
        </div>
        <button
          onClick={() => {
            setEditingMCP(null);
            setShowRegisterModal(true);
          }}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          <Plus className="h-4 w-4" />
          Register MCP Server
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* MCP Servers Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  URL
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Trust Score
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Activity
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {mcpServers.map((server) => (
                <tr key={server.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="flex-shrink-0 h-8 w-8 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                        <Server className="h-4 w-4 text-purple-600 dark:text-purple-400" />
                      </div>
                      <div className="ml-3">
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {server.name}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400" title={server.id}>
                          {server.id.substring(0, 8)}...
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center text-sm text-gray-900 dark:text-gray-100">
                      <Globe className="h-3 w-3 mr-1 text-gray-400 flex-shrink-0" />
                      <a
                        href={server.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="truncate max-w-[200px] hover:text-blue-600 dark:hover:text-blue-400 hover:underline transition-colors text-xs"
                        title={server.url}
                      >
                        {server.url}
                      </a>
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <StatusBadge status={server.status} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className={`text-sm font-semibold ${
                      (server.trust_score || 0) >= 80 ? 'text-green-600 dark:text-green-400' :
                      (server.trust_score || 0) >= 60 ? 'text-yellow-600 dark:text-yellow-400' :
                      (server.trust_score || 0) >= 40 ? 'text-orange-600 dark:text-orange-400' :
                      'text-red-600 dark:text-red-400'
                    }`}>
                      {(server.trust_score || 0).toFixed(1)}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {server.last_verified_at ? formatDateTime(server.last_verified_at) : 'Never'}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleViewMCP(server)}
                        className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                        title="View details"
                      >
                        <Eye className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => handleEditMCP(server)}
                        className="p-1 text-gray-400 hover:text-yellow-600 dark:hover:text-yellow-400 transition-colors"
                        title="Edit"
                      >
                        <Edit className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => handleDeleteMCP(server)}
                        className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                        title="Delete"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {mcpServers.length === 0 && (
          <div className="text-center py-12">
            <Server className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">No MCP servers registered</h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Get started by registering your first MCP server.
            </p>
            <button
              onClick={() => setShowRegisterModal(true)}
              className="mt-4 inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              <Plus className="h-4 w-4" />
              Register MCP Server
            </button>
          </div>
        )}
      </div>

      {/* Info Card */}
      <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            <Shield className="h-6 w-6 text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-blue-900 dark:text-blue-100">About MCP Server Verification</h3>
            <p className="mt-2 text-sm text-blue-700 dark:text-blue-300">
              Model Context Protocol (MCP) servers must be verified before they can interact with AI agents.
              Cryptographic verification uses public key infrastructure to ensure servers meet security standards
              and operate within defined boundaries. Regular re-verification is recommended to maintain trust scores.
            </p>
          </div>
        </div>
      </div>

      {/* Modals */}
      <RegisterMCPModal
        isOpen={showRegisterModal}
        onClose={() => {
          setShowRegisterModal(false);
          setEditingMCP(null);
        }}
        onSuccess={handleServerCreated}
        editMode={!!editingMCP}
        initialData={editingMCP}
      />

      <MCPDetailModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedMCP(null);
        }}
        mcp={selectedMCP}
        onEdit={handleEditMCP}
        onDelete={handleDeleteMCP}
      />
    </div>
  );
}
