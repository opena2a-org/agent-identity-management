"use client";

import { useState, useEffect, useRef } from "react";
import {
  X,
  Loader2,
  CheckCircle,
  AlertCircle,
  Plus,
  Trash2,
  Download,
  ShieldAlert,
  Code,
  Package,
  Copy,
  Eye,
  EyeOff,
  ExternalLink,
} from "lucide-react";
import { api, Agent } from "@/lib/api";
import { downloadSDK as downloadAgentSDK } from "@/lib/agent-sdk";
import { toast } from "sonner";
import { LoadingOverlay } from "@/components/ui/loading-overlay";

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
  agent_type: "ai_agent" | "mcp_server";
  version: string;
  certificate_url: string;
  repository_url: string;
  documentation_url: string;
  talks_to: string[]; // MCP server IDs/names
  capabilities: string[]; // Capability strings
}

// Common capability options
const CAPABILITY_OPTIONS = [
  "read_files",
  "write_files",
  "execute_code",
  "network_access",
  "database_access",
  "api_calls",
  "user_interaction",
  "data_processing",
];

export function RegisterAgentModal({
  isOpen,
  onClose,
  onSuccess,
  editMode = false,
  initialData,
}: RegisterAgentModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [createdAgent, setCreatedAgent] = useState<Agent | null>(null);
  const [downloadingSDK, setDownloadingSDK] = useState(false);
  const [integrationMethod, setIntegrationMethod] = useState<
    "sdk" | "manual" | null
  >(null);
  const [showPrivateKey, setShowPrivateKey] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [agentKeys, setAgentKeys] = useState<{
    publicKey: string;
    privateKey: string;
  } | null>(null);
  const [loadingKeys, setLoadingKeys] = useState(false);
  const createEmptyFormData = (): FormData => ({
    name: "",
    display_name: "",
    description: "",
    agent_type: "ai_agent",
    version: "1.0.0",
    certificate_url: "",
    repository_url: "",
    documentation_url: "",
    talks_to: [],
    capabilities: [],
  });
  const [formData, setFormData] = useState<FormData>(createEmptyFormData());

  const nameRef = useRef<HTMLInputElement | null>(null);
  const displayNameRef = useRef<HTMLInputElement | null>(null);
  const descriptionRef = useRef<HTMLTextAreaElement | null>(null);
  const versionRef = useRef<HTMLInputElement | null>(null);
  const certificateUrlRef = useRef<HTMLInputElement | null>(null);
  const repositoryUrlRef = useRef<HTMLInputElement | null>(null);
  const documentationUrlRef = useRef<HTMLInputElement | null>(null);
  const [initialFormData, setInitialFormData] = useState<FormData>(
    createEmptyFormData()
  );
  const errorBannerRef = useRef<HTMLDivElement | null>(null);
  const [newMcpServer, setNewMcpServer] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (error && errorBannerRef.current) {
      requestAnimationFrame(() => {
        errorBannerRef.current?.scrollIntoView({
          behavior: "smooth",
          block: "center",
        });
      });
    }
  }, [error]);

  // Update form data when initialData or editMode changes
  useEffect(() => {
    if (isOpen && editMode && initialData) {
      const mapped: FormData = {
        name: initialData.name || "",
        display_name: initialData.display_name || "",
        description: initialData.description || "",
        agent_type: initialData.agent_type || "ai_agent",
        version: initialData.version || "1.0.0",
        certificate_url: (initialData as any).certificate_url || "",
        repository_url: (initialData as any).repository_url || "",
        documentation_url: (initialData as any).documentation_url || "",
        talks_to: (initialData as any).talks_to || [],
        capabilities: (initialData as any).capabilities || [],
      };
      setFormData(mapped);
      setInitialFormData(mapped);
    } else if (isOpen && !editMode) {
      // Reset form for new agent
      const empty = createEmptyFormData();
      setFormData(empty);
      setInitialFormData(empty);
    }
  }, [isOpen, editMode, initialData]);

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = "Agent name is required";
    } else if (!/^[a-z0-9-_]+$/.test(formData.name)) {
      newErrors.name =
        "Agent name must be lowercase alphanumeric with dashes/underscores";
    }

    if (!formData.display_name.trim()) {
      newErrors.display_name = "Display name is required";
    }

    if (!formData.version.trim()) {
      newErrors.version = "Version is required";
    } else if (!/^\d+\.\d+\.\d+$/.test(formData.version)) {
      newErrors.version = "Version must be in format X.Y.Z (e.g., 1.0.0)";
    }

    // Validate URLs if provided
    const urlPattern = /^https?:\/\/.+/;
    if (
      formData.certificate_url &&
      !urlPattern.test(formData.certificate_url)
    ) {
      newErrors.certificate_url = "Must be a valid HTTP(S) URL";
    }
    if (formData.repository_url && !urlPattern.test(formData.repository_url)) {
      newErrors.repository_url = "Must be a valid HTTP(S) URL";
    }
    if (
      formData.documentation_url &&
      !urlPattern.test(formData.documentation_url)
    ) {
      newErrors.documentation_url = "Must be a valid HTTP(S) URL";
    }

    setErrors(newErrors);
    if (Object.keys(newErrors).length > 0) {
      requestAnimationFrame(() => {
        if (newErrors.name && nameRef.current) {
          nameRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          nameRef.current.focus();
        } else if (newErrors.display_name && displayNameRef.current) {
          displayNameRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          displayNameRef.current.focus();
        } else if (newErrors.description && descriptionRef.current) {
          descriptionRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          descriptionRef.current.focus();
        } else if (newErrors.version && versionRef.current) {
          versionRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          versionRef.current.focus();
        } else if (newErrors.certificate_url && certificateUrlRef.current) {
          certificateUrlRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          certificateUrlRef.current.focus();
        } else if (newErrors.repository_url && repositoryUrlRef.current) {
          repositoryUrlRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          repositoryUrlRef.current.focus();
        } else if (newErrors.documentation_url && documentationUrlRef.current) {
          documentationUrlRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          documentationUrlRef.current.focus();
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
      // Send snake_case to match backend JSON tags
      const agentData: any = {
        name: formData.name,
        display_name: formData.display_name,
        description: formData.description,
        agent_type: formData.agent_type,
        version: formData.version,
      };

      // Add optional fields only if they have values
      if (formData.certificate_url) {
        agentData.certificate_url = formData.certificate_url;
      }
      if (formData.repository_url) {
        agentData.repository_url = formData.repository_url;
      }
      if (formData.documentation_url) {
        agentData.documentation_url = formData.documentation_url;
      }
      if (formData.talks_to.length > 0) {
        agentData.talks_to = formData.talks_to;
      }
      agentData.capabilities = formData.capabilities;

      const result =
        editMode && initialData?.id
          ? await api.updateAgent(initialData.id, agentData)
          : await api.createAgent(agentData);

      setSuccess(true);
      setCreatedAgent(result);

      if (editMode) {
        toast.success("Agent updated successfully", {
          description: `${result.display_name || result.name} has been updated.`,
        });
        setTimeout(() => {
          onSuccess?.(result);
          onClose();
          resetForm();
        }, 1200);
      } else {
        toast.success("Agent created successfully", {
          description: `${result.display_name || result.name} is ready to integrate.`,
        });
      }
    } catch (err) {
      console.error("Failed to save agent:", err);
      setError(err instanceof Error ? err.message : "Failed to save agent");
    } finally {
      setLoading(false);
    }
  };

  const downloadSDK = async () => {
    if (!createdAgent) return;

    setDownloadingSDK(true);
    try {
      await downloadAgentSDK(createdAgent.id, createdAgent.name, 'python');

      // After successful download, close modal
      setTimeout(() => {
        onSuccess?.(createdAgent);
        onClose();
        resetForm();
      }, 1000);
    } catch (err) {
      console.error("Failed to download SDK:", err);
      alert(
        "Failed to download SDK. Please try again from the agent details page."
      );
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

  const fetchAgentKeys = async () => {
    if (!createdAgent) return;

    setLoadingKeys(true);
    try {
      const token = api.getToken();

      // Get runtime-detected API URL from api client's baseURL
      const apiBaseURL = (api as any).baseURL;

      const response = await fetch(
        `${apiBaseURL}/api/v1/agents/${createdAgent.id}/credentials`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!response.ok) {
        throw new Error("Failed to fetch agent keys");
      }

      const data = await response.json();
      setAgentKeys({
        publicKey: data.publicKey,
        privateKey: data.privateKey,
      });
    } catch (err) {
      console.error("Failed to fetch agent keys:", err);
      alert(
        "Failed to fetch credentials. Please try again from the agent details page."
      );
      setIntegrationMethod(null); // Reset to selection screen
    } finally {
      setLoadingKeys(false);
    }
  };

  const copyToClipboard = async (text: string, field: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedField(field);
      setTimeout(() => setCopiedField(null), 2000);
    } catch (err) {
      console.error("Failed to copy to clipboard:", err);
      alert("Failed to copy to clipboard");
    }
  };

  const handleManualIntegration = () => {
    setIntegrationMethod("manual");
    fetchAgentKeys();
  };

  const resetForm = () => {
    const empty = createEmptyFormData();
    setFormData(empty);
    setInitialFormData(empty);
    setNewMcpServer("");
    setErrors({});
    setError(null);
    setSuccess(false);
    setCreatedAgent(null);
    setDownloadingSDK(false);
    setIntegrationMethod(null);
    setShowPrivateKey(false);
    setCopiedField(null);
    setAgentKeys(null);
    setLoadingKeys(false);
  };

  const handleClose = () => {
    if (!loading) {
      resetForm();
      onClose();
    }
  };

  // Check if form has been modified
  const isFormDirty = () => {
    // If agent is already created successfully, no need to confirm
    if (success) return false;

    // Check if any field has been filled out
    return JSON.stringify(formData) !== JSON.stringify(initialFormData);
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

  const addMcpServer = () => {
    const trimmed = newMcpServer.trim();
    if (trimmed && !formData.talks_to.includes(trimmed)) {
      setFormData((prev) => ({
        ...prev,
        talks_to: [...prev.talks_to, trimmed],
      }));
      setNewMcpServer("");
    }
  };

  const removeMcpServer = (server: string) => {
    setFormData((prev) => ({
      ...prev,
      talks_to: prev.talks_to.filter((s) => s !== server),
    }));
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center p-4 bg-black/50"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {editMode ? "Edit Agent" : "Register New Agent"}
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
        <form onSubmit={handleSubmit} className="relative min-h-[400px] p-6 space-y-6">
          <LoadingOverlay
            show={loading || (editMode && success)}
            label={
              loading
                ? editMode
                  ? "Updating agent..."
                  : "Registering agent..."
                : "Processing..."
            }
          />
          {/* Success Message */}
          {success && !editMode && createdAgent && (
            <div className="space-y-4">
              {/* Success Banner */}
              <div className="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg flex items-center gap-3">
                <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400" />
                <p className="text-sm text-green-800 dark:text-green-300">
                  Agent registered successfully! Cryptographic keys generated
                  automatically.
                </p>
              </div>

              {/* Integration Method Selection - Show if no method chosen yet */}
              {!integrationMethod && (
                <div className="space-y-3">
                  <h4 className="text-sm font-semibold text-gray-900 dark:text-white">
                    🎯 Choose Your Integration Method
                  </h4>

                  {/* SDK Integration Option */}
                  <button
                    onClick={() => setIntegrationMethod("sdk")}
                    className="w-full p-4 border-2 border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20 rounded-lg hover:border-blue-300 dark:hover:border-blue-700 transition-colors text-left"
                  >
                    <div className="flex items-start gap-3">
                      <Package className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
                      <div className="flex-1">
                        <h5 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                          📦 SDK Integration (Recommended)
                        </h5>
                        <p className="text-xs text-blue-800 dark:text-blue-200">
                          Download ready-to-use SDK for <strong>Python</strong>.
                          Includes cryptographic keys and automatic
                          verification.
                        </p>
                      </div>
                    </div>
                  </button>

                  {/* Manual Integration Option */}
                  <button
                    onClick={handleManualIntegration}
                    className="w-full p-4 border-2 border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors text-left"
                  >
                    <div className="flex items-start gap-3">
                      <Code className="h-5 w-5 text-gray-600 dark:text-gray-400 mt-0.5 flex-shrink-0" />
                      <div className="flex-1">
                        <h5 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1">
                          🔧 Manual Integration
                        </h5>
                        <p className="text-xs text-gray-700 dark:text-gray-300">
                          Use <strong>any programming language</strong> (Rust,
                          Ruby, PHP, Java, etc.). Get your credentials and API
                          documentation.
                        </p>
                      </div>
                    </div>
                  </button>
                </div>
              )}

              {/* SDK Download Section - Show if SDK method chosen */}
              {integrationMethod === "sdk" && (
                <div className="p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg space-y-4">
                  <div className="flex items-start gap-3">
                    <Download className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                    <div className="flex-1">
                      <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                        Download Python SDK
                      </h4>
                      <p className="text-xs text-blue-800 dark:text-blue-200 mb-3">
                        Get started immediately with automatic identity
                        verification. The SDK includes your agent's
                        cryptographic keys for seamless authentication.
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
                        <li>
                          • This SDK contains your agent's{" "}
                          <strong>private cryptographic key</strong>
                        </li>
                        <li>
                          • <strong>Never</strong> commit this SDK to version
                          control (Git, GitHub, etc.)
                        </li>
                        <li>
                          • <strong>Never</strong> share this SDK publicly or
                          with untrusted parties
                        </li>
                        <li>
                          • Store it securely and use environment variables in
                          production
                        </li>
                        <li>• Regenerate keys immediately if compromised</li>
                      </ul>
                    </div>
                  </div>

                  {/* Change Method */}
                  <div className="text-center">
                    <button
                      onClick={() => setIntegrationMethod(null)}
                      className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                    >
                      ← Choose a different integration method
                    </button>
                  </div>
                </div>
              )}

              {/* Manual Integration Section - Show if manual method chosen */}
              {integrationMethod === "manual" && (
                <div className="p-4 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg space-y-4">
                  <div className="flex items-start gap-3">
                    <Code className="h-5 w-5 text-gray-600 dark:text-gray-400 mt-0.5" />
                    <div className="flex-1">
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1">
                        🔑 Agent Credentials & API Access
                      </h4>
                      <p className="text-xs text-gray-700 dark:text-gray-300 mb-4">
                        Use these credentials to integrate AIM with any
                        programming language. Keep your private key secure.
                      </p>

                      {loadingKeys ? (
                        <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                          <Loader2 className="h-4 w-4 animate-spin" />
                          Loading credentials...
                        </div>
                      ) : agentKeys ? (
                        <div className="space-y-3">
                          {/* Agent ID */}
                          <div>
                            <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                              Agent ID
                            </label>
                            <div className="flex gap-2">
                              <input
                                type="text"
                                value={createdAgent.id}
                                readOnly
                                className="flex-1 px-3 py-2 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded text-xs font-mono"
                              />
                              <button
                                onClick={() =>
                                  copyToClipboard(createdAgent.id, "agent_id")
                                }
                                className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                              >
                                {copiedField === "agent_id" ? (
                                  <CheckCircle className="h-4 w-4 text-green-600" />
                                ) : (
                                  <Copy className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                )}
                              </button>
                            </div>
                          </div>

                          {/* Public Key */}
                          <div>
                            <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                              Public Key (Ed25519)
                            </label>
                            <div className="flex gap-2">
                              <input
                                type="text"
                                value={agentKeys.publicKey}
                                readOnly
                                className="flex-1 px-3 py-2 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded text-xs font-mono"
                              />
                              <button
                                onClick={() =>
                                  copyToClipboard(
                                    agentKeys.publicKey,
                                    "public_key"
                                  )
                                }
                                className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                              >
                                {copiedField === "public_key" ? (
                                  <CheckCircle className="h-4 w-4 text-green-600" />
                                ) : (
                                  <Copy className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                )}
                              </button>
                            </div>
                          </div>

                          {/* Private Key */}
                          <div>
                            <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                              Private Key (Ed25519) - ⚠️ Keep Secret!
                            </label>
                            <div className="flex gap-2">
                              <input
                                type={showPrivateKey ? "text" : "password"}
                                value={agentKeys.privateKey}
                                readOnly
                                className="flex-1 px-3 py-2 bg-white dark:bg-gray-900 border border-red-200 dark:border-red-800 rounded text-xs font-mono"
                              />
                              <button
                                onClick={() =>
                                  setShowPrivateKey(!showPrivateKey)
                                }
                                className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                              >
                                {showPrivateKey ? (
                                  <EyeOff className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                ) : (
                                  <Eye className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                )}
                              </button>
                              <button
                                onClick={() =>
                                  copyToClipboard(
                                    agentKeys.privateKey,
                                    "private_key"
                                  )
                                }
                                className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                              >
                                {copiedField === "private_key" ? (
                                  <CheckCircle className="h-4 w-4 text-green-600" />
                                ) : (
                                  <Copy className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                )}
                              </button>
                            </div>
                            <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                              Never commit this to version control or share
                              publicly
                            </p>
                          </div>

                          {/* API Documentation Link */}
                          <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
                            <a
                              href="https://docs.aim.dev/api/authentication"
                              target="_blank"
                              rel="noopener noreferrer"
                              className="inline-flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
                            >
                              <ExternalLink className="h-4 w-4" />
                              View Full API Documentation →
                            </a>
                          </div>
                        </div>
                      ) : (
                        <p className="text-sm text-red-600 dark:text-red-400">
                          Failed to load credentials. Please try again from the
                          agent details page.
                        </p>
                      )}
                    </div>
                  </div>

                  {/* Change Method */}
                  <div className="text-center">
                    <button
                      onClick={() => setIntegrationMethod(null)}
                      className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                    >
                      ← Choose a different integration method
                    </button>
                  </div>
                </div>
              )}

              {/* Skip/Close Option - Show for both methods */}
              {integrationMethod && (
                <div className="text-center">
                  <button
                    onClick={handleSkipSDK}
                    className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                  >
                    {integrationMethod === "sdk"
                      ? "Skip for now (you can download SDK later from agent details)"
                      : "Done (you can access credentials later from agent details)"}
                  </button>
                </div>
              )}
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div
              ref={errorBannerRef}
              className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg flex items-center gap-3"
            >
              <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
              <div className="flex-1">
                <p className="text-sm text-red-800 dark:text-red-300">
                  {error}
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
                    ref={nameRef}
                    type="text"
                    value={formData.name}
                    onChange={(e) =>
                      setFormData({ ...formData, name: e.target.value })
                    }
                    placeholder="e.g., claude-assistant"
                    className={`w-full px-3 py-2 border rounded-lg focus:outline-none text-gray-900 dark:text-gray-100 ${
                      // Styling for normal/active state
                      loading || success || editMode
                        ? "bg-gray-200 dark:bg-gray-700 cursor-not-allowed border-gray-300 dark:border-gray-600 focus:ring-0"
                        : "bg-gray-50 dark:bg-gray-800 focus:ring-2 focus:ring-blue-500 border-gray-200 dark:border-gray-700"
                      } ${
                      // Styling for error state (overrides normal/active styling if present)
                      errors.name
                        ? "border-red-500"
                        : ""
                      }`}
                    disabled={loading || success || editMode}
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
                    ref={displayNameRef}
                    type="text"
                    value={formData.display_name}
                    onChange={(e) =>
                      setFormData({ ...formData, display_name: e.target.value })
                    }
                    placeholder="e.g., Claude AI Assistant"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.display_name
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.display_name && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.display_name}
                    </p>
                  )}
                </div>

                {/* Description */}
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Description
                  </label>
                  <textarea
                    ref={descriptionRef}
                    value={formData.description}
                    onChange={(e) =>
                      setFormData({ ...formData, description: e.target.value })
                    }
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
                      onChange={(e) =>
                        setFormData({
                          ...formData,
                          agent_type: e.target.value as
                            | "ai_agent"
                            | "mcp_server",
                        })
                      }
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
                      ref={versionRef}
                      type="text"
                      value={formData.version}
                      onChange={(e) =>
                        setFormData({ ...formData, version: e.target.value })
                      }
                      placeholder="1.0.0"
                      className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.version
                        ? "border-red-500"
                        : "border-gray-200 dark:border-gray-700"
                        }`}
                      disabled={loading || success}
                    />
                    {errors.version && (
                      <p className="mt-1 text-xs text-red-500">
                        {errors.version}
                      </p>
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
                    ref={certificateUrlRef}
                    type="url"
                    value={formData.certificate_url}
                    onChange={(e) =>
                      setFormData({
                        ...formData,
                        certificate_url: e.target.value,
                      })
                    }
                    placeholder="https://example.com/certs/agent-cert.pem"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.certificate_url
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.certificate_url && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.certificate_url}
                    </p>
                  )}
                </div>

                {/* Repository URL */}
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Repository URL
                  </label>
                  <input
                    ref={repositoryUrlRef}
                    type="url"
                    value={formData.repository_url}
                    onChange={(e) =>
                      setFormData({
                        ...formData,
                        repository_url: e.target.value,
                      })
                    }
                    placeholder="https://github.com/yourusername/your-agent"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.repository_url
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.repository_url && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.repository_url}
                    </p>
                  )}
                </div>

                {/* Documentation URL */}
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Documentation URL
                  </label>
                  <input
                    ref={documentationUrlRef}
                    type="url"
                    value={formData.documentation_url}
                    onChange={(e) =>
                      setFormData({
                        ...formData,
                        documentation_url: e.target.value,
                      })
                    }
                    placeholder="https://docs.example.com/agents/your-agent"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.documentation_url
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.documentation_url && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.documentation_url}
                    </p>
                  )}
                </div>
              </div>

              {/* Capabilities */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Capabilities
                </label>
                <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                  Select the capabilities this agent has. These define what
                  actions the agent can perform.
                </p>
                <div className="grid grid-cols-2 gap-2">
                  {CAPABILITY_OPTIONS.map((capability) => (
                    <label
                      key={capability}
                      className="flex items-center gap-2 p-2 rounded hover:bg-gray-50 dark:hover:bg-gray-800"
                    >
                      <input
                        type="checkbox"
                        checked={formData.capabilities.includes(capability)}
                        onChange={() => toggleCapability(capability)}
                        className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        disabled={loading || success}
                      />
                      <span className="text-sm text-gray-700 dark:text-gray-300">
                        {capability
                          .replace(/_/g, " ")
                          .replace(/\b\w/g, (l) => l.toUpperCase())}
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
                  List the MCP servers this agent communicates with. This helps
                  track dependencies and enforce security policies.
                </p>

                {/* Add MCP Server Input */}
                <div className="flex gap-2 mb-3">
                  <input
                    type="text"
                    value={newMcpServer}
                    onChange={(e) => setNewMcpServer(e.target.value)}
                    onKeyPress={(e) =>
                      e.key === "Enter" && (e.preventDefault(), addMcpServer())
                    }
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
                    {formData.talks_to.map((server) => (
                      <div
                        key={server}
                        className="flex items-center justify-between p-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded"
                      >
                        <span className="text-sm text-gray-700 dark:text-gray-300">
                          {server}
                        </span>
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
            {success && !editMode ? (
              // Show Done button after successful registration
              <button
                type="button"
                onClick={handleSkipSDK}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors flex items-center gap-2"
              >
                <CheckCircle className="h-4 w-4" />
                Done
              </button>
            ) : (
              // Show normal form buttons
              <>
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
                  disabled={loading || success || JSON.stringify(formData) === JSON.stringify(initialFormData)}
                  className=
                  "px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                >
                  {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                  {editMode ? "Update Agent" : "Register Agent"}
                </button>
              </>
            )}
          </div>
        </form>
      </div>
    </div>
  );
}
