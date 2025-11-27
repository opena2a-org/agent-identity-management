"use client";

import { useState } from "react";
import {
  X,
  Loader2,
  CheckCircle,
  AlertCircle,
  Copy,
  Check,
  Eye,
  EyeOff,
} from "lucide-react";
import { api, Agent } from "@/lib/api";

interface CreateAPIKeyModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (apiKey: any) => void;
  agents: Agent[];
}

interface FormData {
  name: string;
  agentId: string;
  expiresIn: string;
}

export function CreateAPIKeyModal({
  isOpen,
  onClose,
  onSuccess,
  agents,
}: CreateAPIKeyModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [showKey, setShowKey] = useState(true);

  const [formData, setFormData] = useState<FormData>({
    name: "",
    agentId: "",
    expiresIn: "90",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = "API key name is required";
    }

    if (!formData.agentId) {
      newErrors.agentId = "Please select an agent";
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
      const result = await api.createAPIKey(formData.agentId, formData.name);
      console.log("API Key creation result:", result);

      if (!result.apiKey) {
        console.error("No API key in response:", result);
        throw new Error("API key not returned from server");
      }

      setApiKey(result.apiKey);
      setSuccess(true);
      onSuccess?.(result);
    } catch (err) {
      console.error("Failed to create API key:", err);
      setError(err instanceof Error ? err.message : "Failed to create API key");
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = async () => {
    if (apiKey) {
      await navigator.clipboard.writeText(apiKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const resetForm = () => {
    setFormData({
      name: "",
      agentId: "",
      expiresIn: "90",
    });
    setErrors({});
    setError(null);
    setSuccess(false);
    setApiKey(null);
    setCopied(false);
    setShowKey(true);
  };

  const handleClose = () => {
    if (!loading) {
      resetForm();
      onClose();
    }
  };

  // Check if form has been modified
  const isFormDirty = () => {
    // If API key is already created, no confirmation needed
    if (success) return false;
    return formData.name.trim() !== "" || formData.agentId !== "";
  };

  // Handle click on overlay (outside modal)
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      // Don't allow closing if API key is shown (user must copy it first)
      if (success && apiKey) {
        return;
      }

      if (isFormDirty()) {
        if (
          confirm(
            "You have unsaved changes. Are you sure you want to close without saving?"
          )
        ) {
          handleClose();
        }
      } else {
        handleClose();
      }
    }
  };

  const getExpirationDate = () => {
    const days = parseInt(formData.expiresIn);
    if (days === 0) return "Never";
    const date = new Date();
    date.setDate(date.getDate() + days);
    return date.toLocaleDateString("en-US", {
      month: "long",
      day: "numeric",
      year: "numeric",
    });
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {success ? "API Key Created Successfully" : "Create API Key"}
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
        <div className="p-6">
          {/* Show API Key (only after creation) */}
          {success && apiKey && (
            <div className="space-y-4">
              <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                <div className="flex items-start gap-3">
                  <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400 flex-shrink-0 mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-green-800 dark:text-green-300">
                      API Key Created Successfully
                    </p>
                    <p className="text-xs text-green-700 dark:text-green-400 mt-1">
                      Make sure to copy your API key now. You won't be able to
                      see it again!
                    </p>
                  </div>
                </div>
              </div>

              {/* Critical Warning */}
              <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border-2 border-yellow-400 dark:border-yellow-600 rounded-lg">
                <div className="flex items-start gap-3">
                  <AlertCircle className="h-5 w-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0 mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-bold text-yellow-900 dark:text-yellow-300">
                      ⚠️ IMPORTANT: Copy the Full API Key Now!
                    </p>
                    <p className="text-xs text-yellow-800 dark:text-yellow-400 mt-1">
                      <strong className="block mt-1">
                        You must copy the entire key below - it will never be
                        shown again!
                      </strong>
                    </p>
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Your Complete API Key ({apiKey.length} characters)
                </label>
                <div className="relative">
                  <div className="flex items-center gap-2 p-3 bg-gray-100 dark:bg-gray-800 border-2 border-blue-500 dark:border-blue-400 rounded-lg font-mono text-sm">
                    <code className="flex-1 overflow-x-auto break-all text-gray-900 dark:text-gray-100">
                      {showKey ? apiKey : "•".repeat(apiKey.length)}
                    </code>
                    <button
                      onClick={() => setShowKey(!showKey)}
                      className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors flex-shrink-0"
                      title={showKey ? "Hide key" : "Show key"}
                    >
                      {showKey ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                    <button
                      onClick={copyToClipboard}
                      className="flex items-center gap-1 px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors flex-shrink-0"
                    >
                      {copied ? (
                        <>
                          <Check className="h-4 w-4" />
                          <span className="text-xs font-bold">Copied!</span>
                        </>
                      ) : (
                        <>
                          <Copy className="h-4 w-4" />
                          <span className="text-xs font-bold">
                            Copy Full Key
                          </span>
                        </>
                      )}
                    </button>
                  </div>
                </div>
                <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  💡 Tip: Save this key in a secure location (e.g., environment
                  variables, password manager)
                </p>
              </div>

              <div className="flex items-center justify-end pt-4 border-t border-gray-200 dark:border-gray-700">
                <button
                  onClick={handleClose}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors"
                >
                  Done
                </button>
              </div>
            </div>
          )}

          {/* Show Form (only before creation) */}
          {!success && (
            <form onSubmit={handleSubmit} className="space-y-4">
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

              {/* Key Name */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Key Name <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) =>
                    setFormData({ ...formData, name: e.target.value })
                  }
                  placeholder="e.g., Production API Key"
                  className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                    errors.name
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                  }`}
                  disabled={loading}
                />
                {errors.name && (
                  <p className="mt-1 text-xs text-red-500">{errors.name}</p>
                )}
              </div>

              {/* Agent Selection */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Agent <span className="text-red-500">*</span>
                </label>
                <select
                  value={formData.agentId}
                  onChange={(e) =>
                    setFormData({ ...formData, agentId: e.target.value })
                  }
                  className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                    errors.agentId
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                  }`}
                  disabled={loading}
                >
                  <option value="">Select an agent...</option>
                  {agents.map((agent) => (
                    <option key={agent.id} value={agent.id}>
                      {agent.displayName} ({agent.name})
                    </option>
                  ))}
                </select>
                {errors.agentId && (
                  <p className="mt-1 text-xs text-red-500">{errors.agentId}</p>
                )}
              </div>

              {/* Expiration */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Expiration
                </label>
                <select
                  value={formData.expiresIn}
                  onChange={(e) =>
                    setFormData({ ...formData, expiresIn: e.target.value })
                  }
                  className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                  disabled={loading}
                >
                  <option value="30">30 days</option>
                  <option value="90">90 days</option>
                  <option value="180">180 days</option>
                  <option value="365">1 year</option>
                  <option value="0">Never</option>
                </select>
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  Expires on: {getExpirationDate()}
                </p>
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
                  disabled={loading}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                >
                  {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                  Create API Key
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
