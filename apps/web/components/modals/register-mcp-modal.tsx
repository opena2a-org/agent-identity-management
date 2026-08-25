"use client";

import { useState, useEffect, useRef } from "react";
import { X, Loader2, CheckCircle, AlertCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { extractErrorMessage, ERROR_MESSAGES } from "@/lib/error-utils";
import { LoadingOverlay } from "@/components/ui/loading-overlay";

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
}

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

  const createEmptyFormData = (): FormData => ({
    name: "",
    description: "",
    url: "",
    version: "1.0.0",
    public_key: "",
    verification_url: "",
  });

  const [formData, setFormData] = useState<FormData>(createEmptyFormData());
  const [initialFormData, setInitialFormData] = useState<FormData>(createEmptyFormData());
  const [errors, setErrors] = useState<Record<string, string>>({});
  const nameRef = useRef<HTMLInputElement | null>(null);
  const urlRef = useRef<HTMLInputElement | null>(null);
  const versionRef = useRef<HTMLInputElement | null>(null);
  const errorBannerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (error && errorBannerRef.current) {
      requestAnimationFrame(() => {
        errorBannerRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      });
    }
  }, [error]);

  // Update form data when initialData or editMode changes
  useEffect(() => {
    if (isOpen && editMode && initialData) {
      const mapped: FormData = {
        name: initialData.name || "",
        description: initialData.description || "",
        url: initialData.url || "",
        version: initialData.version || "1.0.0",
        public_key: initialData.publicKey || "",
        verification_url: initialData.verificationUrl || "",
      };
      setFormData(mapped);
      setInitialFormData(mapped);
    } else if (isOpen && !editMode) {
      const empty = createEmptyFormData();
      setFormData(empty);
      setInitialFormData(empty);
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
    if (Object.keys(newErrors).length > 0) {
      requestAnimationFrame(() => {
        if (newErrors.name && nameRef.current) {
          nameRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          nameRef.current.focus();
        } else if (newErrors.url && urlRef.current) {
          urlRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          urlRef.current.focus();
        } else if (newErrors.version && versionRef.current) {
          versionRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          versionRef.current.focus();
        }
      });
    }
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
        serverData.publicKey = formData.public_key;  // Backend expects camelCase
      }
      if (formData.verification_url) {
        serverData.verificationUrl = formData.verification_url;  // Backend expects camelCase
      }
      // Note: Capabilities are auto-detected by the backend from /.well-known/mcp/capabilities

      const result =
        editMode && initialData?.id
          ? await api.updateMCPServer(initialData.id, serverData)
          : await api.createMCPServer(serverData);

      setSuccess(true);

      // Show success toast
      toast.success(
        editMode ? "MCP server updated successfully" : "MCP server registered successfully",
        {
          description: editMode
            ? `${formData.name} has been updated.`
            : `${formData.name} has been registered and is ready to use.`,
        }
      );

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
      toast.error("MCP server registration failed", {
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
    const empty = createEmptyFormData();
    setFormData(empty);
    setInitialFormData(empty);
    setErrors({});
    setError(null);
    setSuccess(false);
  };

  const isFormDirty = () =>
    !success && JSON.stringify(formData) !== JSON.stringify(initialFormData);

  const handleClose = () => {
    if (loading) return;

    if (isFormDirty()) {
      const confirmed = confirm(
        "You have unsaved changes. Are you sure you want to close without saving?"
      );
      if (!confirmed) {
        return;
      }
    }

    resetForm();
    onClose();
  };

  // Handle click on overlay (outside modal)
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(29,29,31,0.45)] backdrop-blur-sm"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="glass-chrome max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-divider">
          <h2 className="text-xl font-semibold text-ink">
            {editMode ? "Edit MCP server" : "Register MCP server"}
          </h2>
          <button
            onClick={handleClose}
            disabled={loading}
            className="text-ink-tertiary hover:text-ink transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="relative min-h-[400px] p-6 space-y-6">
          <LoadingOverlay
            show={loading || success}
            label={
              loading
                ? editMode
                  ? "Updating server..."
                  : "Registering server..."
                : "Processing..."
            }
          />
          {/* Success Message */}
          {success && (
            <div className="p-4 bg-success-fill border border-success-border rounded-inset flex items-center gap-3">
              <CheckCircle className="h-5 w-5 text-success-text" />
              <p className="text-sm text-success-text">
                MCP server {editMode ? "updated" : "registered"} successfully
              </p>
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div
              ref={errorBannerRef}
              className="glass-alert p-4 flex items-center gap-3"
            >
              <AlertCircle className="h-5 w-5 text-danger-text" />
              <div className="flex-1">
                <p className="text-sm text-danger-text">
                  {error}
                </p>
              </div>
            </div>
          )}

          {/* Basic Information */}
          <div className="space-y-4">
            <h3 className="text-overline">
              Basic information
            </h3>

            {/* Server Name */}
            <div>
              <label className="block text-sm font-medium text-ink-body mb-1">
                Server name <span className="text-danger-text">*</span>
              </label>
              <input
                ref={nameRef}
                type="text"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="e.g., filesystem-mcp or github-mcp"
                className={`w-full px-3 py-2 bg-glass-inset border rounded-inset focus:outline-none focus:ring-2 focus:ring-ring text-ink placeholder:text-ink-tertiary ${errors.name
                  ? "border-danger"
                  : "border-stroke"
                  }`}
                disabled={loading || success}
              />
              {errors.name && (
                <p className="mt-1 text-xs text-danger-text">{errors.name}</p>
              )}
            </div>

            {/* Server URL */}
            <div>
              <label className="block text-sm font-medium text-ink-body mb-1">
                Server URL <span className="text-danger-text">*</span>
              </label>
              <input
                ref={urlRef}
                type="url"
                value={formData.url}
                onChange={(e) =>
                  setFormData({ ...formData, url: e.target.value })
                }
                placeholder="https://mcp.example.com"
                className={`w-full px-3 py-2 bg-glass-inset border rounded-inset focus:outline-none focus:ring-2 focus:ring-ring text-ink placeholder:text-ink-tertiary ${errors.url
                  ? "border-danger"
                  : "border-stroke"
                  }`}
                disabled={loading || success}
              />
              {errors.url && (
                <p className="mt-1 text-xs text-danger-text">{errors.url}</p>
              )}
            </div>

            {/* Description */}
            <div>
              <label className="block text-sm font-medium text-ink-body mb-1">
                Description
              </label>
              <textarea
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                placeholder="Brief description of what this MCP server provides..."
                rows={3}
                className="w-full px-3 py-2 bg-glass-inset border border-stroke rounded-inset focus:outline-none focus:ring-2 focus:ring-ring text-ink placeholder:text-ink-tertiary"
                disabled={loading || success}
              />
            </div>

            {/* Version */}
            <div>
              <label className="block text-sm font-medium text-ink-body mb-1">
                Version
              </label>
              <input
                ref={versionRef}
                type="text"
                value={formData.version}
                onChange={(e) =>
                  setFormData({ ...formData, version: e.target.value })
                }
                placeholder="1.0.0"
                className={`w-full px-3 py-2 bg-glass-inset border rounded-inset focus:outline-none focus:ring-2 focus:ring-ring text-ink placeholder:text-ink-tertiary ${errors.version
                  ? "border-danger"
                  : "border-stroke"
                  }`}
                disabled={loading || success}
              />
              {errors.version && (
                <p className="mt-1 text-xs text-danger-text">{errors.version}</p>
              )}
              <p className="mt-1 text-xs text-ink-tertiary">
                Must be in format X.Y.Z (e.g., 1.0.0)
              </p>
            </div>
          </div>

          {/* Security Configuration */}
          <div className="space-y-4">
            <h3 className="text-overline">
              Security configuration (optional)
            </h3>

            {/* Info Box - Automatic Security */}
            <div className="bg-brand-soft border border-brand-soft rounded-inset p-4">
              <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                  <CheckCircle className="h-5 w-5 text-brand-text" />
                </div>
                <div>
                  <h4 className="text-sm font-medium text-ink">
                    Automatic key generation and verification
                  </h4>
                  <p className="mt-1 text-xs text-ink-body">
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
              <label className="block text-sm font-medium text-ink-body mb-1">
                Public key (optional)
              </label>
              <textarea
                value={formData.public_key}
                onChange={(e) =>
                  setFormData({ ...formData, public_key: e.target.value })
                }
                placeholder="Base64-encoded Ed25519 public key (leave empty for automatic generation)"
                rows={3}
                className="w-full px-3 py-2 bg-glass-inset border border-stroke rounded-inset focus:outline-none focus:ring-2 focus:ring-ring text-ink placeholder:text-ink-tertiary font-mono text-xs"
                disabled={loading || success}
              />
              <p className="mt-1 text-xs text-ink-tertiary">
                If empty, AIM generates Ed25519 keys automatically
              </p>
            </div>
          </div>

          {/* Auto-Detection Info */}
          <div className="space-y-4">
            <h3 className="text-overline">
              MCP capabilities
            </h3>

            <div className="bg-success-fill border border-success-border rounded-inset p-4">
              <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                  <CheckCircle className="h-5 w-5 text-success-text" />
                </div>
                <div>
                  <h4 className="text-sm font-medium text-ink">
                    Automatic capability detection
                  </h4>
                  <p className="mt-1 text-xs text-ink-body">
                    AIM will automatically discover capabilities from your MCP server's{" "}
                    <code className="bg-glass-inset-gray text-ink px-1 py-0.5 rounded">
                      /.well-known/mcp/capabilities
                    </code>{" "}
                    endpoint. No manual configuration needed.
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 pt-4 border-t border-divider">
            <button
              type="button"
              onClick={handleClose}
              disabled={loading}
              className="px-4 py-2 text-sm font-medium text-ink-body hover:bg-glass-inset-gray rounded-pill transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || (editMode && !success && !isFormDirty())}
              className="px-4 py-2 text-sm font-medium rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {loading && <Loader2 className="h-4 w-4 animate-spin" />}
              {editMode ? "Update server" : "Register server"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
