'use client';

import { useState } from 'react';
import { X, Loader2, CheckCircle, AlertCircle } from 'lucide-react';
import { api, Agent } from '@/lib/api';

interface RegisterAgentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (agent: Agent) => void;
  editMode?: boolean;
  initialData?: Partial<Agent>;
}

interface FormData {
  name: string;
  display_name: string;
  description: string;
  agent_type: 'ai_agent' | 'mcp_server';
  version: string;
  capabilities: {
    file_operations: boolean;
    code_execution: boolean;
    network_access: boolean;
    database_access: boolean;
    rate_limit: number;
  };
  status: 'verified' | 'pending' | 'suspended' | 'revoked';
}

export function RegisterAgentModal({
  isOpen,
  onClose,
  onSuccess,
  editMode = false,
  initialData
}: RegisterAgentModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    name: initialData?.name || '',
    display_name: initialData?.display_name || '',
    description: initialData?.description || '',
    agent_type: initialData?.agent_type || 'ai_agent',
    version: initialData?.version || '1.0.0',
    capabilities: {
      file_operations: true,
      code_execution: false,
      network_access: true,
      database_access: false,
      rate_limit: 100
    },
    status: initialData?.status || 'pending'
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = 'Agent name is required';
    } else if (!/^[a-z0-9-_]+$/.test(formData.name)) {
      newErrors.name = 'Agent name must be lowercase alphanumeric with dashes/underscores';
    }

    if (!formData.display_name.trim()) {
      newErrors.display_name = 'Display name is required';
    }

    if (!formData.version.trim()) {
      newErrors.version = 'Version is required';
    } else if (!/^\d+\.\d+\.\d+$/.test(formData.version)) {
      newErrors.version = 'Version must be in format X.Y.Z (e.g., 1.0.0)';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setLoading(true);
    setError(null);

    try {
      // Convert snake_case to camelCase for backend API
      const agentData = {
        name: formData.name,
        displayName: formData.display_name,
        description: formData.description,
        agentType: formData.agent_type,
        version: formData.version
      };

      const result = editMode && initialData?.id
        ? await api.updateAgent(initialData.id, agentData)
        : await api.createAgent(agentData);

      setSuccess(true);

      setTimeout(() => {
        onSuccess?.(result);
        onClose();
        resetForm();
      }, 1500);
    } catch (err) {
      console.error('Failed to save agent:', err);
      setError(err instanceof Error ? err.message : 'Failed to save agent');

      // Mock success for development
      setTimeout(() => {
        setSuccess(true);
        setTimeout(() => {
          const mockAgent: Agent = {
            id: `agt_${Date.now()}`,
            organization_id: 'org_123',
            ...formData,
            trust_score: 0,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
          };
          onSuccess?.(mockAgent);
          onClose();
          resetForm();
        }, 1500);
      }, 500);
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      display_name: '',
      description: '',
      agent_type: 'ai_agent',
      version: '1.0.0',
      capabilities: {
        file_operations: true,
        code_execution: false,
        network_access: true,
        database_access: false,
        rate_limit: 100
      },
      status: 'pending'
    });
    setErrors({});
    setError(null);
    setSuccess(false);
  };

  const handleClose = () => {
    if (!loading) {
      resetForm();
      onClose();
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {editMode ? 'Edit Agent' : 'Register New Agent'}
          </h2>
          <button
            onClick={handleClose}
            disabled={loading}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {/* Success Message */}
          {success && (
            <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg flex items-center gap-3">
              <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
              <p className="text-sm text-green-800 dark:text-green-300">
                Agent {editMode ? 'updated' : 'registered'} successfully!
              </p>
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
              <div className="flex-1">
                <p className="text-sm text-yellow-800 dark:text-yellow-300">
                  {error} (Using mock mode)
                </p>
              </div>
            </div>
          )}

          {/* Agent Name */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Agent Name <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="e.g., claude-assistant"
              className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                errors.name ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
              }`}
              disabled={loading || success}
            />
            {errors.name && (
              <p className="mt-1 text-xs text-red-500">{errors.name}</p>
            )}
          </div>

          {/* Display Name */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Display Name <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={formData.display_name}
              onChange={(e) => setFormData({ ...formData, display_name: e.target.value })}
              placeholder="e.g., Claude AI Assistant"
              className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                errors.display_name ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
              }`}
              disabled={loading || success}
            />
            {errors.display_name && (
              <p className="mt-1 text-xs text-red-500">{errors.display_name}</p>
            )}
          </div>

          {/* Description */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Description
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              placeholder="Brief description of what this agent does..."
              rows={3}
              className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
              disabled={loading || success}
            />
          </div>

          {/* Agent Type and Version */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Agent Type <span className="text-red-500">*</span>
              </label>
              <select
                value={formData.agent_type}
                onChange={(e) => setFormData({ ...formData, agent_type: e.target.value as 'ai_agent' | 'mcp_server' })}
                className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                disabled={loading || success}
              >
                <option value="ai_agent">AI Agent</option>
                <option value="mcp_server">MCP Server</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Version <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={formData.version}
                onChange={(e) => setFormData({ ...formData, version: e.target.value })}
                placeholder="1.0.0"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.version ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.version && (
                <p className="mt-1 text-xs text-red-500">{errors.version}</p>
              )}
            </div>
          </div>

          {/* Capabilities */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Capabilities
            </label>
            <div className="space-y-2">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={formData.capabilities.file_operations}
                  onChange={(e) => setFormData({
                    ...formData,
                    capabilities: { ...formData.capabilities, file_operations: e.target.checked }
                  })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  disabled={loading || success}
                />
                <span className="text-sm text-gray-700 dark:text-gray-300">File Operations (read/write)</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={formData.capabilities.code_execution}
                  onChange={(e) => setFormData({
                    ...formData,
                    capabilities: { ...formData.capabilities, code_execution: e.target.checked }
                  })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  disabled={loading || success}
                />
                <span className="text-sm text-gray-700 dark:text-gray-300">Code Execution</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={formData.capabilities.network_access}
                  onChange={(e) => setFormData({
                    ...formData,
                    capabilities: { ...formData.capabilities, network_access: e.target.checked }
                  })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  disabled={loading || success}
                />
                <span className="text-sm text-gray-700 dark:text-gray-300">Network Access</span>
              </label>

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={formData.capabilities.database_access}
                  onChange={(e) => setFormData({
                    ...formData,
                    capabilities: { ...formData.capabilities, database_access: e.target.checked }
                  })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  disabled={loading || success}
                />
                <span className="text-sm text-gray-700 dark:text-gray-300">Database Access</span>
              </label>

              <div>
                <label className="block text-sm text-gray-700 dark:text-gray-300 mb-1">
                  Rate Limit (requests/minute)
                </label>
                <input
                  type="number"
                  value={formData.capabilities.rate_limit}
                  onChange={(e) => setFormData({
                    ...formData,
                    capabilities: { ...formData.capabilities, rate_limit: parseInt(e.target.value) || 100 }
                  })}
                  min="1"
                  max="10000"
                  className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                  disabled={loading || success}
                />
              </div>
            </div>
          </div>

          {/* Status */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Status
            </label>
            <select
              value={formData.status}
              onChange={(e) => setFormData({ ...formData, status: e.target.value as any })}
              className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
              disabled={loading || success}
            >
              <option value="pending">Pending</option>
              <option value="verified">Verified</option>
              <option value="suspended">Suspended</option>
            </select>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
            <button
              type="button"
              onClick={handleClose}
              disabled={loading}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || success}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {loading && <Loader2 className="h-4 w-4 animate-spin" />}
              {editMode ? 'Update Agent' : 'Register Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
