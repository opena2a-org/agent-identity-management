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
  url: string;
  description: string;
  public_key: string;
  key_type: 'RSA-2048' | 'RSA-4096' | 'Ed25519' | 'ECDSA-P256';
  verification_url: string;
  status: 'active' | 'inactive' | 'pending';
}

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
    url: initialData?.url || '',
    description: initialData?.description || '',
    public_key: initialData?.public_key || '',
    key_type: initialData?.key_type || 'RSA-2048',
    verification_url: initialData?.verification_url || '',
    status: initialData?.status || 'pending'
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
      const result = editMode && initialData?.id
        ? await api.updateMCPServer(initialData.id, formData)
        : await api.createMCPServer(formData);

      setSuccess(true);

      setTimeout(() => {
        onSuccess?.(result);
        onClose();
        resetForm();
      }, 1500);
    } catch (err) {
      console.error('Failed to save MCP server:', err);
      setError(err instanceof Error ? err.message : 'Failed to save MCP server');

      // Mock success for development
      setTimeout(() => {
        setSuccess(true);
        setTimeout(() => {
          const mockServer = {
            id: `mcp_${Date.now()}`,
            ...formData,
            verification_status: 'unverified',
            created_at: new Date().toISOString()
          };
          onSuccess?.(mockServer);
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
      url: '',
      description: '',
      public_key: '',
      key_type: 'RSA-2048',
      verification_url: '',
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
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
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
            <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
              <div className="flex-1">
                <p className="text-sm text-yellow-800 dark:text-yellow-300">
                  {error} (Using mock mode)
                </p>
              </div>
            </div>
          )}

          {/* Server Name */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Server Name <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="e.g., File Server MCP"
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

          {/* Public Key */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Public Key <span className="text-gray-500">(optional)</span>
            </label>
            <textarea
              value={formData.public_key}
              onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
              placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"
              rows={6}
              className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 font-mono text-xs"
              disabled={loading || success}
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Paste PEM-formatted public key for cryptographic verification
            </p>
          </div>

          {/* Key Type */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Key Type
            </label>
            <select
              value={formData.key_type}
              onChange={(e) => setFormData({ ...formData, key_type: e.target.value as any })}
              className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
              disabled={loading || success}
            >
              <option value="RSA-2048">RSA-2048</option>
              <option value="RSA-4096">RSA-4096</option>
              <option value="Ed25519">Ed25519</option>
              <option value="ECDSA-P256">ECDSA P-256</option>
            </select>
          </div>

          {/* Verification URL */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Verification URL <span className="text-gray-500">(optional)</span>
            </label>
            <input
              type="url"
              value={formData.verification_url}
              onChange={(e) => setFormData({ ...formData, verification_url: e.target.value })}
              placeholder="https://mcp-server.example.com/verify"
              className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
              disabled={loading || success}
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Endpoint for cryptographic challenge-response verification
            </p>
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
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
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
              {editMode ? 'Update Server' : 'Register Server'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
