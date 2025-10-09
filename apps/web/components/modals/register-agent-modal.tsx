'use client';

import { useState } from 'react';
import { X, Loader2, CheckCircle, AlertCircle, Plus, Trash2, Download, ShieldAlert } from 'lucide-react';
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
  certificate_url: string;
  repository_url: string;
  documentation_url: string;
  talks_to: string[];  // MCP server IDs/names
  capabilities: string[];  // Capability strings
}

// Common capability options
const CAPABILITY_OPTIONS = [
  'read_files',
  'write_files',
  'execute_code',
  'network_access',
  'database_access',
  'api_calls',
  'user_interaction',
  'data_processing'
];

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
  const [createdAgent, setCreatedAgent] = useState<Agent | null>(null);
  const [downloadingSDK, setDownloadingSDK] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    name: initialData?.name || '',
    display_name: initialData?.display_name || '',
    description: initialData?.description || '',
    agent_type: initialData?.agent_type || 'ai_agent',
    version: initialData?.version || '1.0.0',
    certificate_url: '',
    repository_url: '',
    documentation_url: '',
    talks_to: [],
    capabilities: []
  });

  const [newMcpServer, setNewMcpServer] = useState('');
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

    // Validate URLs if provided
    const urlPattern = /^https?:\/\/.+/;
    if (formData.certificate_url && !urlPattern.test(formData.certificate_url)) {
      newErrors.certificate_url = 'Must be a valid HTTP(S) URL';
    }
    if (formData.repository_url && !urlPattern.test(formData.repository_url)) {
      newErrors.repository_url = 'Must be a valid HTTP(S) URL';
    }
    if (formData.documentation_url && !urlPattern.test(formData.documentation_url)) {
      newErrors.documentation_url = 'Must be a valid HTTP(S) URL';
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
      const agentData: any = {
        name: formData.name,
        displayName: formData.display_name,
        description: formData.description,
        agentType: formData.agent_type,
        version: formData.version
      };

      // Add optional fields only if they have values
      if (formData.certificate_url) {
        agentData.certificateUrl = formData.certificate_url;
      }
      if (formData.repository_url) {
        agentData.repositoryUrl = formData.repository_url;
      }
      if (formData.documentation_url) {
        agentData.documentationUrl = formData.documentation_url;
      }
      if (formData.talks_to.length > 0) {
        agentData.talksTo = formData.talks_to;
      }
      if (formData.capabilities.length > 0) {
        agentData.capabilities = formData.capabilities;
      }

      const result = editMode && initialData?.id
        ? await api.updateAgent(initialData.id, agentData)
        : await api.createAgent(agentData);

      setSuccess(true);
      setCreatedAgent(result);

      // Don't auto-close for new registrations - let user download SDK first
      if (editMode) {
        setTimeout(() => {
          onSuccess?.(result);
          onClose();
          resetForm();
        }, 1500);
      }
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
            name: formData.name,
            display_name: formData.display_name,
            description: formData.description,
            agent_type: formData.agent_type,
            version: formData.version,
            status: 'pending',
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

  const downloadSDK = async () => {
    if (!createdAgent) return;

    setDownloadingSDK(true);
    try {
      const token = localStorage.getItem('auth_token');
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v1/agents/${createdAgent.id}/sdk?lang=python`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to download SDK');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `aim-sdk-${createdAgent.name}-python.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);

      // After successful download, close modal
      setTimeout(() => {
        onSuccess?.(createdAgent);
        onClose();
        resetForm();
      }, 1000);
    } catch (err) {
      console.error('Failed to download SDK:', err);
      alert('Failed to download SDK. Please try again from the agent details page.');
    } finally {
      setDownloadingSDK(false);
    }
  };

  const handleSkipSDK = () => {
    if (createdAgent) {
      onSuccess?.(createdAgent);
      onClose();
      resetForm();
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      display_name: '',
      description: '',
      agent_type: 'ai_agent',
      version: '1.0.0',
      certificate_url: '',
      repository_url: '',
      documentation_url: '',
      talks_to: [],
      capabilities: []
    });
    setNewMcpServer('');
    setErrors({});
    setError(null);
    setSuccess(false);
    setCreatedAgent(null);
    setDownloadingSDK(false);
  };

  const handleClose = () => {
    if (!loading) {
      resetForm();
      onClose();
    }
  };

  const toggleCapability = (capability: string) => {
    setFormData(prev => ({
      ...prev,
      capabilities: prev.capabilities.includes(capability)
        ? prev.capabilities.filter(c => c !== capability)
        : [...prev.capabilities, capability]
    }));
  };

  const addMcpServer = () => {
    const trimmed = newMcpServer.trim();
    if (trimmed && !formData.talks_to.includes(trimmed)) {
      setFormData(prev => ({
        ...prev,
        talks_to: [...prev.talks_to, trimmed]
      }));
      setNewMcpServer('');
    }
  };

  const removeMcpServer = (server: string) => {
    setFormData(prev => ({
      ...prev,
      talks_to: prev.talks_to.filter(s => s !== server)
    }));
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
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
        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {/* Success Message */}
          {success && !editMode && createdAgent && (
            <div className="space-y-4">
              {/* Success Banner */}
              <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg flex items-center gap-3">
                <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
                <p className="text-sm text-green-800 dark:text-green-300">
                  Agent registered successfully! Cryptographic keys generated automatically.
                </p>
              </div>

              {/* Security Warning + SDK Download */}
              <div className="p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg space-y-4">
                <div className="flex items-start gap-3">
                  <Download className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                  <div className="flex-1">
                    <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                      Download Python SDK
                    </h4>
                    <p className="text-xs text-blue-800 dark:text-blue-200 mb-3">
                      Get started immediately with automatic identity verification. The SDK includes your agent's cryptographic keys for seamless authentication.
                    </p>
                    <button
                      onClick={downloadSDK}
                      disabled={downloadingSDK}
                      className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                    >
                      {downloadingSDK ? (
                        <>
                          <Loader2 className="h-4 w-4 animate-spin" />
                          Downloading...
                        </>
                      ) : (
                        <>
                          <Download className="h-4 w-4" />
                          Download SDK (.zip)
                        </>
                      )}
                    </button>
                  </div>
                </div>

                {/* Security Warning */}
                <div className="flex items-start gap-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded">
                  <ShieldAlert className="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5 flex-shrink-0" />
                  <div className="flex-1">
                    <h5 className="text-xs font-semibold text-red-900 dark:text-red-100 mb-1">
                      ⚠️ Security Notice: Contains Private Key
                    </h5>
                    <ul className="text-xs text-red-800 dark:text-red-200 space-y-1">
                      <li>• This SDK contains your agent's <strong>private cryptographic key</strong></li>
                      <li>• <strong>Never</strong> commit this SDK to version control (Git, GitHub, etc.)</li>
                      <li>• <strong>Never</strong> share this SDK publicly or with untrusted parties</li>
                      <li>• Store it securely and use environment variables in production</li>
                      <li>• Regenerate keys immediately if compromised</li>
                    </ul>
                  </div>
                </div>

                {/* Skip Option */}
                <div className="text-center">
                  <button
                    onClick={handleSkipSDK}
                    className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                  >
                    Skip for now (you can download SDK later from agent details)
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Edit Mode Success Message */}
          {success && editMode && (
            <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg flex items-center gap-3">
              <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
              <p className="text-sm text-green-800 dark:text-green-300">
                Agent updated successfully!
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

          {/* Hide form fields when showing SDK download */}
          {!(success && !editMode) && (
          <>
          {/* Basic Information */}
          <div className="space-y-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Basic Information
            </h3>

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
          </div>

          {/* Additional Resources */}
          <div className="space-y-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Additional Resources (Optional)
            </h3>

            {/* Certificate URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Certificate URL
              </label>
              <input
                type="url"
                value={formData.certificate_url}
                onChange={(e) => setFormData({ ...formData, certificate_url: e.target.value })}
                placeholder="https://example.com/certs/agent-cert.pem"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.certificate_url ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.certificate_url && (
                <p className="mt-1 text-xs text-red-500">{errors.certificate_url}</p>
              )}
            </div>

            {/* Repository URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Repository URL
              </label>
              <input
                type="url"
                value={formData.repository_url}
                onChange={(e) => setFormData({ ...formData, repository_url: e.target.value })}
                placeholder="https://github.com/yourusername/your-agent"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.repository_url ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.repository_url && (
                <p className="mt-1 text-xs text-red-500">{errors.repository_url}</p>
              )}
            </div>

            {/* Documentation URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Documentation URL
              </label>
              <input
                type="url"
                value={formData.documentation_url}
                onChange={(e) => setFormData({ ...formData, documentation_url: e.target.value })}
                placeholder="https://docs.example.com/agents/your-agent"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.documentation_url ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.documentation_url && (
                <p className="mt-1 text-xs text-red-500">{errors.documentation_url}</p>
              )}
            </div>
          </div>

          {/* Capabilities */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Capabilities
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
              Select the capabilities this agent has. These define what actions the agent can perform.
            </p>
            <div className="grid grid-cols-2 gap-2">
              {CAPABILITY_OPTIONS.map(capability => (
                <label key={capability} className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 dark:hover:bg-gray-800">
                  <input
                    type="checkbox"
                    checked={formData.capabilities.includes(capability)}
                    onChange={() => toggleCapability(capability)}
                    className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    disabled={loading || success}
                  />
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {capability.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())}
                  </span>
                </label>
              ))}
            </div>
          </div>

          {/* MCP Servers Communication */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              MCP Servers (Talks To)
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
              List the MCP servers this agent communicates with. This helps track dependencies and enforce security policies.
            </p>

            {/* Add MCP Server Input */}
            <div className="flex gap-2 mb-3">
              <input
                type="text"
                value={newMcpServer}
                onChange={(e) => setNewMcpServer(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), addMcpServer())}
                placeholder="e.g., filesystem-mcp or github-mcp"
                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                disabled={loading || success}
              />
              <button
                type="button"
                onClick={addMcpServer}
                disabled={!newMcpServer.trim() || loading || success}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                <Plus className="h-4 w-4" />
                Add
              </button>
            </div>

            {/* MCP Servers List */}
            {formData.talks_to.length > 0 && (
              <div className="space-y-2">
                {formData.talks_to.map(server => (
                  <div key={server} className="flex items-center justify-between p-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded">
                    <span className="text-sm text-gray-700 dark:text-gray-300">{server}</span>
                    <button
                      type="button"
                      onClick={() => removeMcpServer(server)}
                      disabled={loading || success}
                      className="text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
          </>
          )}

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
