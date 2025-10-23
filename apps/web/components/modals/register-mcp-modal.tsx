"use client";

import { useState, useEffect } from "react";
import { X, Loader2, CheckCircle, AlertCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { extractErrorMessage, ERROR_MESSAGES } from "@/lib/error-utils";

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
  "tools",
  "prompts",
  "resources",
  "sampling",
  "logging",
  "pagination",
];

export function RegisterMCPModal({
  isOpen,
  onClose,
  onSuccess,
  editMode = false,
  initialData,
}: RegisterMCPModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    name: "",
    description: "",
    url: "",
    version: "1.0.0",
    public_key: "",
    verification_url: "",
    capabilities: [],
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Update form data when initialData or editMode changes
  useEffect(() => {
    if (isOpen && editMode && initialData) {
      setFormData({
        name: initialData.name || "",
        description: initialData.description || "",
        url: initialData.url || "",
        version: initialData.version || "1.0.0",
        public_key: initialData.public_key || "",
        verification_url: initialData.verification_url || "",
        capabilities: initialData.capabilities || [],
      });
    } else if (isOpen && !editMode) {
      // Reset form for new MCP server
      setFormData({
        name: "",
        description: "",
        url: "",
        version: "1.0.0",
        public_key: "",
        verification_url: "",
        capabilities: [],
      });
    }
  }, [isOpen, editMode, initialData]);

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
      newErrors.name = "Server name is required";
    }

    if (!formData.url.trim()) {
      newErrors.url = "Server URL is required";
    } else if (!validateURL(formData.url)) {
      newErrors.url = "Please enter a valid URL (e.g., https://example.com)";
    }

    // Validate version format if provided
    if (formData.version && !/^\d+\.\d+\.\d+$/.test(formData.version)) {
      newErrors.version = "Version must be in format X.Y.Z (e.g., 1.0.0)";
    }

    // Validate verification_url if provided
    if (formData.verification_url && !validateURL(formData.verification_url)) {
      newErrors.verification_url = "Must be a valid URL";
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
        url: formData.url,
      };

      // Add optional fields only if they have values
      if (formData.version) {
        serverData.version = formData.version;
      }
      if (formData.public_key) {
        serverData.public_key = formData.public_key;  // Backend expects snake_case
      }
      if (formData.verification_url) {
        serverData.verification_url = formData.verification_url;  // Backend expects snake_case
      }
      if (formData.capabilities.length > 0) {
        serverData.capabilities = formData.capabilities;
      }

      const result =
        editMode && initialData?.id
          ? await api.updateMCPServer(initialData.id, serverData)
          : await api.createMCPServer(serverData);

      setSuccess(true);

      // Show success toast
      toast.success("MCP Server Registered Successfully", {
        description: `${formData.name} has been registered and is ready to use.`,
      });

      setTimeout(() => {
        onSuccess?.(result);
        onClose();
        resetForm();
      }, 1500);
    } catch (err) {
      console.error("Failed to save MCP server:", err);

      // Extract error message using utility function
      const errorMessage = extractErrorMessage(
        err,
        ERROR_MESSAGES.MCP_SERVER_SAVE
      );

      // Log the full error for debugging
      console.log("Error details:", { err, errorMessage });

      setError(errorMessage);

      // Show toast notification with the backend error message
      toast.error("MCP Server Registration Failed", {
        description: errorMessage,
        action: {
          label: "Retry",
          onClick: () => handleSubmit(new Event("submit") as any),
        },
      });
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setFormData({
      name: "",
      description: "",
      url: "",
      version: "1.0.0",
      public_key: "",
      verification_url: "",
      capabilities: [],
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

  // Check if form has been modified
  const isFormDirty = () => {
    // If server is already created successfully, no need to confirm
    if (success) return false;

    // Check if any field has been filled out
    return (
      formData.name.trim() !== "" ||
      formData.description.trim() !== "" ||
      formData.url.trim() !== "" ||
      formData.version !== "1.0.0" ||
      formData.public_key.trim() !== "" ||
      formData.verification_url.trim() !== "" ||
      formData.capabilities.length > 0
    );
  };

  // Handle click on overlay (outside modal)
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
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

  const toggleCapability = (capability: string) => {
    setFormData((prev) => ({
      ...prev,
      capabilities: prev.capabilities.includes(capability)
        ? prev.capabilities.filter((c) => c !== capability)
        : [...prev.capabilities, capability],
    }));
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {editMode ? "Edit MCP Server" : "Register MCP Server"}
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
                MCP Server {editMode ? "updated" : "registered"} successfully!
              </p>
            </div>
          )}

          {/* Error Message */}
          {/* {error && (
            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
              <div className="flex-1">
                <p className="text-sm text-red-800 dark:text-red-300">
                  {error}
                </p>
              </div>
            </div>
          )} */}

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
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="e.g., filesystem-mcp or github-mcp"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.name
                    ? "border-red-500"
                    : "border-gray-200 dark:border-gray-700"
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
                onChange={(e) =>
                  setFormData({ ...formData, url: e.target.value })
                }
                placeholder="https://mcp.example.com"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.url
                    ? "border-red-500"
                    : "border-gray-200 dark:border-gray-700"
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
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
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
                onChange={(e) =>
                  setFormData({ ...formData, version: e.target.value })
                }
                placeholder="1.0.0"
                className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                  errors.version
                    ? "border-red-500"
                    : "border-gray-200 dark:border-gray-700"
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
                    Automatic Key Generation & Verification
                  </h4>
                  <p className="mt-1 text-xs text-blue-700 dark:text-blue-300">
                    AIM will automatically generate Ed25519 cryptographic keys
                    and detect capabilities from your MCP server. You can
                    optionally provide your own public key if you've already
                    generated one.
                  </p>
                </div>
              </div>
            </div>

            {/* Public Key */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Public Key (Optional)
              </label>
              <textarea
                value={formData.public_key}
                onChange={(e) =>
                  setFormData({ ...formData, public_key: e.target.value })
                }
                placeholder="Base64-encoded Ed25519 public key (leave empty for automatic generation)"
                rows={3}
                className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 font-mono text-xs"
                disabled={loading || success}
              />
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                If empty, AIM generates secure Ed25519 keys automatically
              </p>
            </div>
          </div>

          {/* Capabilities Auto-Detection Notice */}
          <div className="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0">
                <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
              </div>
              <div>
                <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  Capabilities Auto-Detected
                </h4>
                <p className="mt-1 text-xs text-gray-600 dark:text-gray-400">
                  After registration, AIM will automatically detect and register
                  all capabilities (tools, resources, prompts) that your MCP
                  server exposes. No manual configuration needed.
                </p>
              </div>
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
              {editMode ? "Update Server" : "Register Server"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
