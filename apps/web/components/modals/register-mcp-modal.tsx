'use client';

import { useState } from 'react';
import { X, Loader2, CheckCircle, AlertCircle } from 'lucide-react';
import { api } from '@/lib/api';

interface RegisterMCPModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (server: any) => void;
  editMode?: boolean;
  initialData?: any;
}

interface FormData {
  name: string;
  description: string;
  url: string;
  version: string;
  public_key: string;
  verification_url: string;
  capabilities: string[];
}

// Common MCP capability options
const MCP_CAPABILITY_OPTIONS = [
  'tools',
  'prompts',
  'resources',
  'sampling',
  'logging',
  'pagination'
];

export function RegisterMCPModal({
  isOpen,
  onClose,
  onSuccess,
  editMode = false,
  initialData
}: RegisterMCPModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    name: initialData?.name || '',
    description: initialData?.description || '',
    url: initialData?.url || '',
    version: initialData?.version || '1.0.0',
    public_key: initialData?.public_key || '',
    verification_url: initialData?.verification_url || '',
    capabilities: initialData?.capabilities || []
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validateURL = (url: string): boolean => {
    try {
      new URL(url);
      return true;
    } catch {
      return false;
    }
  };

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = 'Server name is required';
    }

    if (!formData.url.trim()) {
      newErrors.url = 'Server URL is required';
    } else if (!validateURL(formData.url)) {
      newErrors.url = 'Please enter a valid URL (e.g., https://example.com)';
    }

    // Validate version format if provided
    if (formData.version && !/^\d+\.\d+\.\d+$/.test(formData.version)) {
      newErrors.version = 'Version must be in format X.Y.Z (e.g., 1.0.0)';
    }

    // Validate verification_url if provided
    if (formData.verification_url && !validateURL(formData.verification_url)) {
      newErrors.verification_url = 'Must be a valid URL';
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
      // Convert to backend format (camelCase)
      const serverData: any = {
        name: formData.name,
        description: formData.description,
        url: formData.url
      };

      // Add optional fields only if they have values
      if (formData.version) {
        serverData.version = formData.version;
      }
      if (formData.public_key) {
        serverData.publicKey = formData.public_key;
      }
      if (formData.verification_url) {
        serverData.verificationUrl = formData.verification_url;
      }
      if (formData.capabilities.length > 0) {
        serverData.capabilities = formData.capabilities;
      }

      const result = editMode && initialData?.id
        ? await api.updateMCPServer(initialData.id, serverData)
        : await api.createMCPServer(serverData);

      setSuccess(true);

      setTimeout(() => {
        onSuccess?.(result);
        onClose();
        resetForm();
      }, 1500);
    } catch (err) {
      console.error('Failed to save MCP server:', err);
      setError(err instanceof Error ? err.message : 'Failed to save MCP server');
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      url: '',
      version: '1.0.0',
      public_key: '',
      verification_url: '',
      capabilities: []
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

  const toggleCapability = (capability: string) => {
    setFormData(prev => ({
      ...prev,
      capabilities: prev.capabilities.includes(capability)
        ? prev.capabilities.filter(c => c !== capability)
        : [...prev.capabilities, capability]
    }));
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {editMode ? 'Edit MCP Server' : 'Register MCP Server'}
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
          {success && (
            <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg flex items-center gap-3">
              <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
              <p className="text-sm text-green-800 dark:text-green-300">
                MCP Server {editMode ? 'updated' : 'registered'} successfully!
              </p>
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
              <div className="flex-1">
                <p className="text-sm text-red-800 dark:text-red-300">
                  {error}
                </p>
              </div>
            </div>
          )}

          {/* Basic Information */}
          <div className="space-y-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Basic Information
            </h3>

            {/* Server Name */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Server Name <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="e.g., filesystem-mcp or github-mcp"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.name ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.name && (
                <p className="mt-1 text-xs text-red-500">{errors.name}</p>
              )}
            </div>

            {/* Server URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Server URL <span className="text-red-500">*</span>
              </label>
              <input
                type="url"
                value={formData.url}
                onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                placeholder="https://mcp.example.com"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.url ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.url && (
                <p className="mt-1 text-xs text-red-500">{errors.url}</p>
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
                placeholder="Brief description of what this MCP server provides..."
                rows={3}
                className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                disabled={loading || success}
              />
            </div>

            {/* Version */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Version
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
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Must be in format X.Y.Z (e.g., 1.0.0)
              </p>
            </div>
          </div>

          {/* Security Configuration */}
          <div className="space-y-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Security Configuration (Optional)
            </h3>

            {/* Info Box - Automatic Security */}
            <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                  <CheckCircle className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h4 className="text-sm font-medium text-blue-900 dark:text-blue-100">
                    Automatic Key Generation
                  </h4>
                  <p className="mt-1 text-xs text-blue-700 dark:text-blue-300">
                    If you don't provide a public key, AIM will automatically generate Ed25519 cryptographic keys for you.
                    You can also provide your own public key if you've already generated one.
                  </p>
                </div>
              </div>
            </div>

            {/* Public Key */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Public Key
              </label>
              <textarea
                value={formData.public_key}
                onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
                placeholder="Base64-encoded Ed25519 public key (optional - leave empty for automatic generation)"
                rows={3}
                className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 font-mono text-xs"
                disabled={loading || success}
              />
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Leave empty to let AIM generate keys automatically
              </p>
            </div>

            {/* Verification URL */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Verification URL
              </label>
              <input
                type="url"
                value={formData.verification_url}
                onChange={(e) => setFormData({ ...formData, verification_url: e.target.value })}
                placeholder="https://mcp.example.com/.well-known/aim-verification"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.verification_url ? 'border-red-500' : 'border-gray-200 dark:border-gray-700'
                }`}
                disabled={loading || success}
              />
              {errors.verification_url && (
                <p className="mt-1 text-xs text-red-500">{errors.verification_url}</p>
              )}
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                URL endpoint for cryptographic challenge-response verification
              </p>
            </div>
          </div>

          {/* Capabilities */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Capabilities
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
              Select the MCP capabilities this server supports based on the Model Context Protocol specification.
            </p>
            <div className="grid grid-cols-2 gap-2">
              {MCP_CAPABILITY_OPTIONS.map(capability => (
                <label key={capability} className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 dark:hover:bg-gray-800">
                  <input
                    type="checkbox"
                    checked={formData.capabilities.includes(capability)}
                    onChange={() => toggleCapability(capability)}
                    className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    disabled={loading || success}
                  />
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {capability.charAt(0).toUpperCase() + capability.slice(1)}
                  </span>
                </label>
              ))}
            </div>
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
              {editMode ? 'Update Server' : 'Register Server'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
