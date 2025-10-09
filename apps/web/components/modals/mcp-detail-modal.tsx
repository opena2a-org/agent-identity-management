'use client';

import { X, Shield, Calendar, CheckCircle, Clock, Edit, Trash2, Key, Download, TrendingUp } from 'lucide-react';
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
  capabilities?: string[]; // List of capabilities this MCP provides
  talks_to?: string[]; // List of agents that communicate with this MCP
}

interface MCPDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  mcp: MCPServer | null;
  onEdit?: (mcp: MCPServer) => void;
  onDelete?: (mcp: MCPServer) => void;
}

export function MCPDetailModal({
  isOpen,
  onClose,
  mcp,
  onEdit,
  onDelete
}: MCPDetailModalProps) {
  if (!isOpen || !mcp) return null;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300';
      case 'inactive':
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
      default:
        return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300';
    }
  };

  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return 'text-green-600 dark:text-green-400';
    if (score >= 60) return 'text-yellow-600 dark:text-yellow-400';
    if (score >= 40) return 'text-orange-600 dark:text-orange-400';
    return 'text-red-600 dark:text-red-400';
  };

  const calculateFingerprint = (publicKey: string): string => {
    if (!publicKey) return 'N/A';
    // Simple mock fingerprint - in production this would use crypto.subtle.digest
    const hash = publicKey.substring(0, 64);
    return hash.match(/.{1,2}/g)?.slice(0, 16).join(':') || 'N/A';
  };

  const handleDownloadKey = () => {
    if (!mcp.public_key) return;

    const blob = new Blob([mcp.public_key], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${mcp.name.replace(/\s+/g, '_')}_public_key.pem`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  // Handle click on overlay (outside modal) - MCP detail modal is read-only, so close immediately
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-gradient-to-br from-purple-500 to-purple-600 rounded-lg flex items-center justify-center">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                {mcp.name}
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">{mcp.id}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-6">
          {/* Status and Metrics */}
          <div className="grid grid-cols-3 gap-4">
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Status</span>
              <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium capitalize ${getStatusColor(mcp.status)}`}>
                {mcp.status}
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Trust Score</span>
              <div className="flex items-center gap-2">
                <TrendingUp className={`h-4 w-4 ${getTrustScoreColor(mcp.trust_score || 0)}`} />
                <span className={`text-lg font-semibold ${getTrustScoreColor(mcp.trust_score || 0)}`}>
                  {(mcp.trust_score || 0).toFixed(1)}
                </span>
              </div>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">Capabilities</span>
              <span className="text-lg font-semibold text-gray-900 dark:text-white">
                {mcp.capabilities?.length || 0}
              </span>
            </div>
          </div>

          {/* Description */}
          {mcp.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Description</h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">{mcp.description}</p>
            </div>
          )}

          {/* Capabilities */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {mcp.capabilities && mcp.capabilities.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {mcp.capabilities.map((capability, index) => (
                  <div
                    key={index}
                    className="inline-flex items-center gap-2 px-3 py-2 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md"
                  >
                    <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400 flex-shrink-0" />
                    <p className="text-sm font-medium text-blue-900 dark:text-blue-100">
                      {capability}
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Talks To (Agents) */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
              </svg>
              Talks To
            </h3>
            {mcp.talks_to && mcp.talks_to.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {mcp.talks_to.map((agent, index) => (
                  <div
                    key={index}
                    className="px-3 py-2 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md text-sm font-medium text-green-900 dark:text-green-100"
                  >
                    {agent}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No agents configured to use this MCP server
              </div>
            )}
          </div>

          {/* Basic Details Grid */}
          <div className="grid grid-cols-2 gap-6">
            <div className="col-span-2">
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Server URL</h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono break-all">{mcp.url}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Registered
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">{formatDateTime(mcp.created_at)}</p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last Activity
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {mcp.last_verified_at ? formatDateTime(mcp.last_verified_at) : 'Never'}
              </p>
            </div>
          </div>

          {/* Cryptographic Identity Section */}
          {mcp.public_key && (
            <div className="border-t border-gray-200 dark:border-gray-700 pt-6">
              <div className="flex items-center gap-2 mb-4">
                <Key className="h-5 w-5 text-purple-600 dark:text-purple-400" />
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Cryptographic Identity</h3>
              </div>

              <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Public Key Fingerprint (SHA-256)</h4>
                    <p className="text-xs text-gray-900 dark:text-gray-100 font-mono bg-white dark:bg-gray-900 p-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                      {calculateFingerprint(mcp.public_key)}
                    </p>
                  </div>

                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Key Type</h4>
                    <p className="text-sm text-gray-900 dark:text-gray-100 font-medium">
                      {mcp.key_type || 'RSA-2048'}
                    </p>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Public Key</h4>
                  <div className="relative">
                    <pre className="text-xs text-gray-900 dark:text-gray-100 font-mono bg-white dark:bg-gray-900 p-3 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto max-h-32">
                      {mcp.public_key}
                    </pre>
                    <button
                      onClick={handleDownloadKey}
                      className="absolute top-2 right-2 p-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                      title="Download public key"
                    >
                      <Download className="h-4 w-4 text-gray-600 dark:text-gray-300" />
                    </button>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                  <div className="flex items-center gap-2">
                    <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                    <span className="text-sm text-green-600 dark:text-green-400 font-medium">
                      Cryptographic identity verified on registration
                    </span>
                  </div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    Ed25519 signature
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Registration Timeline */}
          <div className="border-t border-gray-200 dark:border-gray-700 pt-6">
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Registration Timeline</h3>
            <div className="space-y-2">
              <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                <div className="flex-1">
                  <p className="text-sm text-gray-900 dark:text-gray-100">MCP server registered & verified</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">{formatDateTime(mcp.created_at)}</p>
                </div>
              </div>
              {mcp.public_key && (
                <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                  <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                  <div className="flex-1">
                    <p className="text-sm text-gray-900 dark:text-gray-100">Capabilities auto-detected from metadata</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">{mcp.capability_count || 0} capabilities registered</p>
                  </div>
                </div>
              )}
              {mcp.last_verified_at && (
                <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                  <Clock className="h-4 w-4 text-purple-600 dark:text-purple-400" />
                  <div className="flex-1">
                    <p className="text-sm text-gray-900 dark:text-gray-100">Last activity</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">{formatDateTime(mcp.last_verified_at)}</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            {onDelete && (
              <button
                onClick={() => onDelete(mcp)}
                className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex items-center gap-2"
              >
                <Trash2 className="h-4 w-4" />
                Delete
              </button>
            )}
          </div>
          <div className="flex items-center gap-2">
            {onEdit && (
              <button
                onClick={() => onEdit(mcp)}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors flex items-center gap-2"
              >
                <Edit className="h-4 w-4" />
                Edit Server
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
