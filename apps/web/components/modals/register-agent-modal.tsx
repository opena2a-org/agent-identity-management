"use client";

import { useState, useEffect, useRef } from "react";
import {
  X,
  Loader2,
  CheckCircle,
  AlertCircle,
  Plus,
  Trash2,
  Code,
  Package,
  Copy,
  Eye,
  EyeOff,
  ExternalLink,
  Terminal,
  Key,
  ChevronDown,
  Server,
  Search,
} from "lucide-react";
import { api, Agent, AgentType, CapabilityDefinition, ListCapabilitiesResponse } from "@/lib/api";
import { toast } from "sonner";
import { LoadingOverlay } from "@/components/ui/loading-overlay";
import { useMemo, useCallback } from "react";

interface RegisterAgentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (agent: Agent) => void;
  editMode?: boolean;
  initialData?: Partial<Agent>;
}

interface FormData {
  name: string;
  displayName: string;
  description: string;
  agentType: AgentType;
  version: string;
  repositoryUrl: string;
  documentationUrl: string;
  talksTo: string[]; // MCP server IDs/names
  capabilities: string[]; // Capability strings
}

// Agent type options organized by category
const AGENT_TYPE_OPTIONS: { value: AgentType; label: string; description: string; category: string }[] = [
  // LLM Providers
  { value: "claude", label: "Claude", description: "Anthropic Claude models", category: "LLM Providers" },
  { value: "gpt", label: "GPT", description: "OpenAI GPT models", category: "LLM Providers" },
  { value: "gemini", label: "Gemini", description: "Google Gemini models", category: "LLM Providers" },
  { value: "llama", label: "Llama", description: "Meta Llama models", category: "LLM Providers" },
  { value: "mistral", label: "Mistral", description: "Mistral AI models", category: "LLM Providers" },
  { value: "cohere", label: "Cohere", description: "Cohere models", category: "LLM Providers" },
  // Frameworks
  { value: "langchain", label: "LangChain", description: "LangChain framework agents", category: "Frameworks" },
  { value: "llamaindex", label: "LlamaIndex", description: "LlamaIndex agents", category: "Frameworks" },
  { value: "langgraph", label: "LangGraph", description: "LangGraph workflow agents", category: "Frameworks" },
  { value: "crewai", label: "CrewAI", description: "CrewAI multi-agent systems", category: "Frameworks" },
  { value: "autogen", label: "AutoGen", description: "Microsoft AutoGen agents", category: "Frameworks" },
  { value: "semantic_kernel", label: "Semantic Kernel", description: "Microsoft Semantic Kernel", category: "Frameworks" },
  { value: "haystack", label: "Haystack", description: "Haystack pipeline agents", category: "Frameworks" },
  // Copilots & Assistants
  { value: "copilot", label: "Copilot", description: "GitHub Copilot, Microsoft Copilot, etc.", category: "Copilots & Assistants" },
  { value: "assistant", label: "Assistant", description: "OpenAI Assistants API, custom assistants", category: "Copilots & Assistants" },
  { value: "chatbot", label: "Chatbot", description: "Conversational chatbots", category: "Copilots & Assistants" },
  // Autonomous Agents
  { value: "autogpt", label: "AutoGPT", description: "AutoGPT autonomous agent", category: "Autonomous Agents" },
  { value: "babyagi", label: "BabyAGI", description: "BabyAGI task-driven agent", category: "Autonomous Agents" },
  // Other
  { value: "custom", label: "Custom", description: "Custom or other agent type", category: "Other" },
];

// Get unique categories in order
const AGENT_TYPE_CATEGORIES = [...new Set(AGENT_TYPE_OPTIONS.map(o => o.category))];

// Capability validation pattern (namespace:action format)
const CAPABILITY_PATTERN = /^[a-z][a-z0-9]*:[a-z][a-z0-9_]*$/;

// Risk level colors for capability badges
const RISK_LEVEL_COLORS: Record<string, string> = {
  low: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  medium: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400",
  high: "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400",
  critical: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
};

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
  // API key received from agent creation (auto-generated)
  const [createdApiKey, setCreatedApiKey] = useState<{
    key: string;
    id: string;
    name: string;
    prefix: string;
    expiresAt: string | null;
    createdAt: string;
  } | null>(null);
  // Capability definitions from API
  const [capabilityDefinitions, setCapabilityDefinitions] = useState<CapabilityDefinition[]>([]);
  const [reservedNamespaces, setReservedNamespaces] = useState<string[]>([]);
  const [loadingCapabilities, setLoadingCapabilities] = useState(false);
  const [customCapability, setCustomCapability] = useState("");
  const [customCapabilityError, setCustomCapabilityError] = useState<string | null>(null);
  const [showAdvancedKeys, setShowAdvancedKeys] = useState(false);
  const createEmptyFormData = (): FormData => ({
    name: "",
    displayName: "",
    description: "",
    agentType: "claude",
    version: "1.0.0",
    repositoryUrl: "",
    documentationUrl: "",
    talksTo: [],
    capabilities: [],
  });
  const [formData, setFormData] = useState<FormData>(createEmptyFormData());

  const nameRef = useRef<HTMLInputElement | null>(null);
  const displayNameRef = useRef<HTMLInputElement | null>(null);
  const descriptionRef = useRef<HTMLTextAreaElement | null>(null);
  const versionRef = useRef<HTMLInputElement | null>(null);
  const repositoryUrlRef = useRef<HTMLInputElement | null>(null);
  const documentationUrlRef = useRef<HTMLInputElement | null>(null);
  const [initialFormData, setInitialFormData] = useState<FormData>(
    createEmptyFormData()
  );
  const errorBannerRef = useRef<HTMLDivElement | null>(null);
  const [newMcpServer, setNewMcpServer] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  // MCP server autocomplete state
  const [mcpSuggestions, setMcpSuggestions] = useState<Array<{
    id: string;
    name: string;
    url?: string;
    status?: string;
    isDiscovered?: boolean;
    isRegistered?: boolean;
  }>>([]);
  const [loadingMcpSuggestions, setLoadingMcpSuggestions] = useState(false);
  const [showMcpDropdown, setShowMcpDropdown] = useState(false);
  const mcpInputRef = useRef<HTMLInputElement | null>(null);
  const mcpDropdownRef = useRef<HTMLDivElement | null>(null);

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

  // Fetch capability definitions when modal opens
  useEffect(() => {
    if (isOpen && capabilityDefinitions.length === 0) {
      setLoadingCapabilities(true);
      api.getCapabilityDefinitions()
        .then((response: ListCapabilitiesResponse) => {
          setCapabilityDefinitions(response.capabilities ?? []);
          setReservedNamespaces(response.reservedNamespaces ?? []);
        })
        .catch((err) => {
          console.error("Failed to fetch capability definitions:", err);
          // Fallback is handled by empty array - users can still add custom capabilities
        })
        .finally(() => {
          setLoadingCapabilities(false);
        });
    }
  }, [isOpen, capabilityDefinitions.length]);

  // Fetch MCP server suggestions when modal opens
  useEffect(() => {
    if (isOpen && mcpSuggestions.length === 0) {
      setLoadingMcpSuggestions(true);

      // Fetch both registered MCP servers and discovered MCPs
      Promise.all([
        api.listMCPServers(100, 0).catch(() => ({ mcpServers: [], total: 0 })),
        api.getDiscoveredMCPs().catch(() => ({ discovered: [], totalUnmapped: 0, totalAgents: 0, registeredServers: 0 })),
      ])
        .then(([registeredResponse, discoveredResponse]) => {
          const suggestions: Array<{
            id: string;
            name: string;
            url?: string;
            status?: string;
            isDiscovered?: boolean;
            isRegistered?: boolean;
          }> = [];

          // Add registered MCP servers
          (registeredResponse.mcpServers || []).forEach((server) => {
            suggestions.push({
              id: server.id,
              name: server.name,
              url: server.url,
              status: server.status,
              isRegistered: true,
              isDiscovered: false,
            });
          });

          // Add discovered MCPs that aren't already registered
          (discoveredResponse.discovered || []).forEach((discovered) => {
            // Check if this discovered MCP is already in the registered list
            const alreadyRegistered = (registeredResponse.mcpServers || []).some(
              (server) => server.name.toLowerCase() === discovered.name.toLowerCase() ||
                         server.url === discovered.url
            );

            if (!alreadyRegistered) {
              suggestions.push({
                id: `discovered-${discovered.name}`,
                name: discovered.name,
                url: discovered.url,
                isRegistered: false,
                isDiscovered: true,
              });
            }
          });

          setMcpSuggestions(suggestions);
        })
        .catch((err) => {
          console.error("Failed to fetch MCP server suggestions:", err);
        })
        .finally(() => {
          setLoadingMcpSuggestions(false);
        });
    }
  }, [isOpen, mcpSuggestions.length]);

  // Close MCP dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        mcpDropdownRef.current &&
        !mcpDropdownRef.current.contains(event.target as Node) &&
        mcpInputRef.current &&
        !mcpInputRef.current.contains(event.target as Node)
      ) {
        setShowMcpDropdown(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Update form data when initialData or editMode changes
  useEffect(() => {
    if (isOpen && editMode && initialData) {
      const mapped: FormData = {
        name: initialData.name || "",
        displayName: initialData.displayName || "",
        description: initialData.description || "",
        agentType: initialData.agentType || "ai_agent",
        version: initialData.version || "1.0.0",
        repositoryUrl: (initialData as any).repositoryUrl || "",
        documentationUrl: (initialData as any).documentationUrl || "",
        talksTo: (initialData as any).talksTo || [],
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

    if (!formData.displayName.trim()) {
      newErrors.displayName = "Display name is required";
    }

    if (!formData.version.trim()) {
      newErrors.version = "Version is required";
    } else if (!/^\d+\.\d+\.\d+$/.test(formData.version)) {
      newErrors.version = "Version must be in format X.Y.Z (e.g., 1.0.0)";
    }

    // Validate URLs if provided
    const urlPattern = /^https?:\/\/.+/;
    if (formData.repositoryUrl && !urlPattern.test(formData.repositoryUrl)) {
      newErrors.repositoryUrl = "Must be a valid HTTP(S) URL";
    }
    if (
      formData.documentationUrl &&
      !urlPattern.test(formData.documentationUrl)
    ) {
      newErrors.documentationUrl = "Must be a valid HTTP(S) URL";
    }

    // Require at least 1 capability
    if (formData.capabilities.length === 0) {
      newErrors.capabilities = "At least one capability is required";
    }

    setErrors(newErrors);
    if (Object.keys(newErrors).length > 0) {
      requestAnimationFrame(() => {
        if (newErrors.name && nameRef.current) {
          nameRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          nameRef.current.focus();
        } else if (newErrors.displayName && displayNameRef.current) {
          displayNameRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          displayNameRef.current.focus();
        } else if (newErrors.description && descriptionRef.current) {
          descriptionRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          descriptionRef.current.focus();
        } else if (newErrors.version && versionRef.current) {
          versionRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          versionRef.current.focus();
        } else if (newErrors.repositoryUrl && repositoryUrlRef.current) {
          repositoryUrlRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
          repositoryUrlRef.current.focus();
        } else if (newErrors.documentationUrl && documentationUrlRef.current) {
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
        displayName: formData.displayName,
        description: formData.description,
        agentType: formData.agentType,
        version: formData.version,
      };

      // Add optional fields only if they have values
      if (formData.repositoryUrl) {
        agentData.repositoryUrl = formData.repositoryUrl;
      }
      if (formData.documentationUrl) {
        agentData.documentationUrl = formData.documentationUrl;
      }
      if (formData.talksTo.length > 0) {
        agentData.talksTo = formData.talksTo;
      }
      agentData.capabilities = formData.capabilities;

      const result =
        editMode && initialData?.id
          ? await api.updateAgent(initialData.id, agentData)
          : await api.createAgent(agentData);

      setSuccess(true);
      setCreatedAgent(result);

      // ✅ Capture auto-generated API key (only available on creation, not edit)
      if (!editMode && result.apiKey) {
        setCreatedApiKey(result.apiKey);
      }

      if (editMode) {
        toast.success("Agent updated successfully", {
          description: `${result.displayName || result.name} has been updated.`,
        });
        setTimeout(() => {
          onSuccess?.(result);
          onClose();
          resetForm();
        }, 1200);
      } else {
        toast.success("Agent created successfully", {
          description: `${result.displayName || result.name} is ready to integrate.`,
        });
      }
    } catch (err) {
      console.error("Failed to save agent:", err);
      setError(err instanceof Error ? err.message : "Failed to save agent");
    } finally {
      setLoading(false);
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
    setCreatedApiKey(null);
    setIntegrationMethod(null);
    setShowPrivateKey(false);
    setCopiedField(null);
    setAgentKeys(null);
    setLoadingKeys(false);
    setCustomCapability("");
    setCustomCapabilityError(null);
    setShowAdvancedKeys(false);
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

  // Merge core capabilities from API with any custom capabilities already in form
  const mergedCapabilities = useMemo(() => {
    const coreTypes = capabilityDefinitions.map(c => c.type);
    const customCaps = formData.capabilities.filter(c => !coreTypes.includes(c));
    return [...capabilityDefinitions, ...customCaps.map(c => ({
      type: c,
      name: c.split(":").map(s => s.charAt(0).toUpperCase() + s.slice(1)).join(" "),
      description: `Custom capability: ${c}`,
      category: "custom",
      riskLevel: "medium" as const,
      capabilityType: "custom" as const,
    }))];
  }, [capabilityDefinitions, formData.capabilities]);

  const toggleCapability = (capability: string) => {
    setFormData((prev) => ({
      ...prev,
      capabilities: prev.capabilities.includes(capability)
        ? prev.capabilities.filter((c) => c !== capability)
        : [...prev.capabilities, capability],
    }));
  };

  // Validate custom capability format
  const validateCustomCapability = useCallback((value: string): string | null => {
    if (!value.trim()) return null;

    if (!CAPABILITY_PATTERN.test(value)) {
      return "Format must be 'namespace:action' (e.g., 'payment:process', 'email:send')";
    }

    const [namespace] = value.split(":");
    if (reservedNamespaces.includes(namespace)) {
      return `Namespace '${namespace}' is reserved for core capabilities`;
    }

    if (formData.capabilities.includes(value)) {
      return "This capability is already added";
    }

    return null;
  }, [reservedNamespaces, formData.capabilities]);

  // Add custom capability
  const addCustomCapability = () => {
    const trimmed = customCapability.trim().toLowerCase();
    const error = validateCustomCapability(trimmed);

    if (error) {
      setCustomCapabilityError(error);
      return;
    }

    setFormData((prev) => ({
      ...prev,
      capabilities: [...prev.capabilities, trimmed],
    }));
    setCustomCapability("");
    setCustomCapabilityError(null);
  };

  const addMcpServer = (serverName?: string) => {
    const trimmed = (serverName || newMcpServer).trim();
    if (trimmed && !formData.talksTo.includes(trimmed)) {
      setFormData((prev) => ({
        ...prev,
        talksTo: [...prev.talksTo, trimmed],
      }));
      setNewMcpServer("");
      setShowMcpDropdown(false);
    }
  };

  const removeMcpServer = (server: string) => {
    setFormData((prev) => ({
      ...prev,
      talksTo: prev.talksTo.filter((s) => s !== server),
    }));
  };

  // Filter MCP suggestions based on search input
  const filteredMcpSuggestions = useMemo(() => {
    const searchTerm = newMcpServer.toLowerCase().trim();
    return mcpSuggestions.filter((suggestion) => {
      // Don't show already selected servers
      if (formData.talksTo.includes(suggestion.name)) {
        return false;
      }
      // Filter by search term
      if (searchTerm) {
        return (
          suggestion.name.toLowerCase().includes(searchTerm) ||
          (suggestion.url && suggestion.url.toLowerCase().includes(searchTerm))
        );
      }
      return true;
    });
  }, [mcpSuggestions, newMcpServer, formData.talksTo]);

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
                  Agent registered successfully! Cryptographic keys and API key generated
                  automatically.
                </p>
              </div>

              {/* ✅ API KEY DISPLAY - Show immediately after creation */}
              {createdApiKey && (
                <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border-2 border-yellow-400 dark:border-yellow-600 rounded-lg space-y-3">
                  <div>
                    <p className="text-sm font-bold text-yellow-900 dark:text-yellow-300">
                      🔑 Your API Key (for Manual Integration)
                    </p>
                    <p className="text-xs text-yellow-800 dark:text-yellow-400 mt-1">
                      SDK users can skip this. Only needed for manual API calls. You can always create a new key later.
                    </p>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-yellow-800 dark:text-yellow-300 mb-1">
                      Your API Key
                    </label>
                    <div className="flex gap-2">
                      <input
                        type={showPrivateKey ? "text" : "password"}
                        value={createdApiKey.key}
                        readOnly
                        className="flex-1 px-3 py-2 bg-white dark:bg-gray-900 border-2 border-yellow-500 dark:border-yellow-600 rounded text-xs font-mono text-gray-900 dark:text-gray-100"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPrivateKey(!showPrivateKey)}
                        className="px-3 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                      >
                        {showPrivateKey ? (
                          <EyeOff className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                        ) : (
                          <Eye className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                        )}
                      </button>
                      <button
                        type="button"
                        onClick={() => copyToClipboard(createdApiKey.key, "api_key")}
                        className="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded transition-colors flex items-center gap-2"
                      >
                        {copiedField === "api_key" ? (
                          <>
                            <CheckCircle className="h-4 w-4" />
                            <span className="text-xs font-bold">Copied!</span>
                          </>
                        ) : (
                          <>
                            <Copy className="h-4 w-4" />
                            <span className="text-xs font-bold">Copy Key</span>
                          </>
                        )}
                      </button>
                    </div>
                    <p className="mt-1 text-xs text-yellow-700 dark:text-yellow-400">
                      Key: {createdApiKey.name} • Expires: {createdApiKey.expiresAt ? new Date(createdApiKey.expiresAt).toLocaleDateString() : "Never"}
                    </p>
                  </div>
                </div>
              )}

              {/* Integration Method Selection - Show if no method chosen yet */}
              {!integrationMethod && (
                <div className="space-y-3">
                  <h4 className="text-sm font-semibold text-gray-900 dark:text-white">
                    🎯 Choose Your Integration Method
                  </h4>

                  {/* Manual Integration Option - Primary */}
                  <button
                    onClick={handleManualIntegration}
                    className="w-full p-4 border-2 border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20 rounded-lg hover:border-blue-300 dark:hover:border-blue-700 transition-colors text-left"
                  >
                    <div className="flex items-start gap-3">
                      <Code className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
                      <div className="flex-1">
                        <h5 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                          🔧 Manual Integration (Recommended for Dashboard Agents)
                        </h5>
                        <p className="text-xs text-blue-800 dark:text-blue-200">
                          Use <strong>any programming language</strong> (Python, Rust,
                          Ruby, PHP, Java, etc.). Get your API key and credentials.
                        </p>
                      </div>
                    </div>
                  </button>

                  {/* SDK Integration Option - Secondary */}
                  <button
                    onClick={() => setIntegrationMethod("sdk")}
                    className="w-full p-4 border-2 border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors text-left"
                  >
                    <div className="flex items-start gap-3">
                      <Package className="h-5 w-5 text-gray-600 dark:text-gray-400 mt-0.5 flex-shrink-0" />
                      <div className="flex-1">
                        <h5 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1">
                          📦 Using Python SDK?
                        </h5>
                        <p className="text-xs text-gray-700 dark:text-gray-300">
                          The SDK auto-registers agents with local credentials.
                          See how to create agents directly via SDK code.
                        </p>
                      </div>
                    </div>
                  </button>
                </div>
              )}

              {/* SDK Integration Section - Show if SDK method chosen */}
              {integrationMethod === "sdk" && (
                <div className="p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg space-y-4">
                  <div className="flex items-start gap-3">
                    <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5" />
                    <div className="flex-1">
                      <h4 className="text-sm font-semibold text-amber-900 dark:text-amber-100 mb-1">
                        SDK Creates Agents Automatically
                      </h4>
                      <p className="text-xs text-amber-800 dark:text-amber-200 mb-3">
                        The Python SDK generates local cryptographic keys and auto-registers agents.
                        Dashboard-created agents use server-side keys which won't work with the SDK.
                      </p>
                    </div>
                  </div>

                  {/* Python Code Example */}
                  <div>
                    <p className="text-xs font-medium text-amber-900 dark:text-amber-100 mb-2">
                      Create your agent directly in Python code:
                    </p>
                    <div className="relative">
                      <pre className="p-3 bg-gray-900 dark:bg-black text-green-400 text-xs font-mono rounded-lg overflow-x-auto whitespace-pre-wrap">
{`from aim_sdk import secure, AgentType

# ══════════════════════════════════════════════════════════════════════════
# AGENT REGISTRATION - Only Requires Agent's Name & Capabilities
# ══════════════════════════════════════════════════════════════════════════
agent = secure(
    "my-ai-assistant",
    agent_type=AgentType.LANGCHAIN,  # CREWAI, AUTOGEN, GPT, CLAUDE, etc.
    capabilities=["db:read", "api:call"],
    mcp_servers=["filesystem"],
    version="1.0.0", # Note: version defaults to "1.0.0" if undeclared
    description="Customer support AI agent",
    tags=["production", "customer-facing", "gpt-4", "support-team"],
    metadata={
        "model": "gpt-4",
        "department": "support"
    }
)

# ══════════════════════════════════════════════════════════════════════════
# TRACK ACTIONS & RISKS - Log all agent activities in AIM
# -----------------------------------------------------------
# AIM verifies agent has db:read and approve or reject action
@agent.perform_action(capability="db:read")  # auto: low risk
def get_customer(customer_id: str):
    return {"id": customer_id, "name": "Jane Doe"}
result = get_customer("cust-123")
print(f"Result: {result}")

# -----------------------------------------------------------
# @agent.perform_action(capability="text:generate") # auto: low risk
# def generate_response(prompt: str):
#     return llm.generate(prompt)

# @agent.perform_action(capability="sentiment:analyze") # auto: low risk
# def analyze_sentiment(text: str):
#     return {"sentiment": "positive", "score": 0.92}

# Critical action - requires admin approval (JIT Access)
# @agent.perform_action(capability="customer:delete", jit_access=True)
# def delete_customer(customer_id: str):
#     return {"deleted": customer_id}  # Waits for admin approval!

# Run your functions - AIM verifies and tracks everything!
# response = generate_response("Hello, how can I help?")
# print(f"Response: {response}")

# ══════════════════════════════════════════════════════════════════════════
# CAPABILITY REQUEST - Request New Capabilities
# ══════════════════════════════════════════════════════════════════════════

# Request capabilities that need admin approval
# agent.request_capability(
#     "db:admin",
#     reason="Need admin access for migration",
#     metadata={
#         "use_case": "quarterly migration",
#         "expires": "2025-01-01"}
# )`}
                      </pre>
                      <button
                        type="button"
                        onClick={() => {
                          const pythonCode = `from aim_sdk import secure, AgentType

# ══════════════════════════════════════════════════════════════════════════
# AGENT REGISTRATION - Only Requires Agent's Name & Capabilities
# ══════════════════════════════════════════════════════════════════════════
agent = secure(
    "my-ai-assistant",
    agent_type=AgentType.LANGCHAIN,  # CREWAI, AUTOGEN, GPT, CLAUDE, etc.
    capabilities=["db:read", "api:call"],
    mcp_servers=["filesystem"],
    version="1.0.0", # Note: version defaults to "1.0.0" if undeclared
    description="Customer support AI agent",
    tags=["production", "customer-facing", "gpt-4", "support-team"],
    metadata={
        "model": "gpt-4",
        "department": "support"
    }
)

# ══════════════════════════════════════════════════════════════════════════
# TRACK ACTIONS & RISKS - Log all agent activities in AIM
# -----------------------------------------------------------
# AIM verifies agent has db:read and approve or reject action
@agent.perform_action(capability="db:read")  # auto: low risk
def get_customer(customer_id: str):
    return {"id": customer_id, "name": "Jane Doe"}
result = get_customer("cust-123")
print(f"Result: {result}")

# -----------------------------------------------------------
# @agent.perform_action(capability="text:generate") # auto: low risk
# def generate_response(prompt: str):
#     return llm.generate(prompt)

# @agent.perform_action(capability="sentiment:analyze") # auto: low risk
# def analyze_sentiment(text: str):
#     return {"sentiment": "positive", "score": 0.92}

# Critical action - requires admin approval (JIT Access)
# @agent.perform_action(capability="customer:delete", jit_access=True)
# def delete_customer(customer_id: str):
#     return {"deleted": customer_id}  # Waits for admin approval!

# Run your functions - AIM verifies and tracks everything!
# response = generate_response("Hello, how can I help?")
# print(f"Response: {response}")

# ══════════════════════════════════════════════════════════════════════════
# CAPABILITY REQUEST - Request New Capabilities
# ══════════════════════════════════════════════════════════════════════════

# Request capabilities that need admin approval
# agent.request_capability(
#     "db:admin",
#     reason="Need admin access for migration",
#     metadata={
#         "use_case": "quarterly migration",
#         "expires": "2025-01-01"}
# )`;
                          copyToClipboard(pythonCode, "python_code");
                        }}
                        className="absolute top-2 right-2 px-2 py-1 bg-gray-700 hover:bg-gray-600 text-white text-xs rounded transition-colors flex items-center gap-1"
                      >
                        {copiedField === "python_code" ? (
                          <>
                            <CheckCircle className="h-3 w-3" />
                            Copied!
                          </>
                        ) : (
                          <>
                            <Copy className="h-3 w-3" />
                            Copy
                          </>
                        )}
                      </button>
                    </div>
                  </div>

                  {/* SDK Download Link and Documentation */}
                  <div className="pt-3 border-t border-amber-200 dark:border-amber-700">
                    <div className="flex flex-wrap gap-2 mb-2">
                      <a
                        href="/dashboard/sdk"
                        className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
                      >
                        <Package className="h-4 w-4" />
                        Download Python SDK
                      </a>
                      <a
                        href="https://opena2a.org/docs/integration/python"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-2 px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-lg transition-colors"
                      >
                        <ExternalLink className="h-4 w-4" />
                        SDK Documentation
                      </a>
                    </div>
                    <p className="text-xs text-amber-700 dark:text-amber-300">
                      After downloading, run: <code className="bg-amber-100 dark:bg-amber-900 px-1 rounded">pip install -e .</code>
                    </p>
                  </div>

                  {/* Change Method */}
                  <div className="text-center pt-2">
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
                                type="button"
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

                          {/* ✅ Pre-filled Curl Command for Verification */}
                          {createdApiKey && formData.capabilities.length > 0 && (
                            <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
                              <div className="flex items-center gap-2 mb-2">
                                <Terminal className="h-4 w-4 text-green-600 dark:text-green-400" />
                                <label className="text-xs font-medium text-gray-700 dark:text-gray-300">
                                  Verify Your Agent (Ready to Copy & Paste)
                                </label>
                              </div>
                              <div className="relative">
                                <pre className="p-3 bg-gray-900 dark:bg-black text-green-400 text-xs font-mono rounded-lg overflow-x-auto whitespace-pre-wrap break-all">
{`curl -X POST "${typeof window !== 'undefined' ? window.location.origin.replace('-frontend', '-backend').replace(':3000', ':8080') : 'http://localhost:8080'}/api/v1/agents/${createdAgent.id}/verify-capability" \\
  -H "Authorization: Bearer ${createdApiKey.key}" \\
  -H "Content-Type: application/json" \\
  -d '{"capability": "${formData.capabilities[0]}", "resource": "test.resource"}'`}
                                </pre>
                                <button
                                  type="button"
                                  onClick={() => {
                                    const curlCmd = `curl -X POST "${typeof window !== 'undefined' ? window.location.origin.replace('-frontend', '-backend').replace(':3000', ':8080') : 'http://localhost:8080'}/api/v1/agents/${createdAgent.id}/verify-capability" \\\n  -H "Authorization: Bearer ${createdApiKey.key}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"capability": "${formData.capabilities[0]}", "resource": "test.resource"}'`;
                                    copyToClipboard(curlCmd, "curl_cmd");
                                  }}
                                  className="absolute top-2 right-2 px-2 py-1 bg-gray-700 hover:bg-gray-600 text-white text-xs rounded transition-colors flex items-center gap-1"
                                >
                                  {copiedField === "curl_cmd" ? (
                                    <>
                                      <CheckCircle className="h-3 w-3" />
                                      Copied!
                                    </>
                                  ) : (
                                    <>
                                      <Copy className="h-3 w-3" />
                                      Copy
                                    </>
                                  )}
                                </button>
                              </div>
                              <p className="mt-2 text-xs text-gray-600 dark:text-gray-300">
                                Run this command in your terminal to verify your agent can use the <code className="bg-blue-100 dark:bg-blue-800 text-blue-800 dark:text-blue-200 px-1.5 py-0.5 rounded font-semibold">{formData.capabilities[0]}</code> capability.
                              </p>
                            </div>
                          )}

                          {/* API Documentation Link */}
                          <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
                            <a
                              href="https://opena2a.org/docs/api/rest"
                              target="_blank"
                              rel="noopener noreferrer"
                              className="inline-flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
                            >
                              <ExternalLink className="h-4 w-4" />
                              View Full API Documentation →
                            </a>
                          </div>

                          {/* Advanced: Cryptographic Keys (Collapsible) */}
                          <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
                            <button
                              type="button"
                              onClick={() => setShowAdvancedKeys(!showAdvancedKeys)}
                              className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
                            >
                              <ChevronDown className={`h-3 w-3 transition-transform ${showAdvancedKeys ? 'rotate-180' : ''}`} />
                              Advanced: Cryptographic Keys (Ed25519)
                            </button>

                            {showAdvancedKeys && (
                              <div className="mt-3 space-y-3 p-3 bg-gray-100 dark:bg-gray-900 rounded-lg">
                                <p className="text-xs text-gray-500 dark:text-gray-400">
                                  For custom integrations with request signing. Most users don't need these.
                                </p>

                                {/* Public Key */}
                                <div>
                                  <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Public Key
                                  </label>
                                  <div className="flex gap-2">
                                    <input
                                      type="text"
                                      value={agentKeys.publicKey}
                                      readOnly
                                      className="flex-1 px-3 py-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded text-xs font-mono"
                                    />
                                    <button
                                      type="button"
                                      onClick={() => copyToClipboard(agentKeys.publicKey, "public_key")}
                                      className="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 rounded transition-colors"
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
                                    Private Key - ⚠️ Keep Secret
                                  </label>
                                  <div className="flex gap-2">
                                    <input
                                      type={showPrivateKey ? "text" : "password"}
                                      value={agentKeys.privateKey}
                                      readOnly
                                      className="flex-1 px-3 py-2 bg-white dark:bg-gray-800 border border-red-200 dark:border-red-800 rounded text-xs font-mono"
                                    />
                                    <button
                                      type="button"
                                      onClick={() => setShowPrivateKey(!showPrivateKey)}
                                      className="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 rounded transition-colors"
                                    >
                                      {showPrivateKey ? (
                                        <EyeOff className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                      ) : (
                                        <Eye className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                      )}
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => copyToClipboard(agentKeys.privateKey, "private_key")}
                                      className="px-3 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 rounded transition-colors"
                                    >
                                      {copiedField === "private_key" ? (
                                        <CheckCircle className="h-4 w-4 text-green-600" />
                                      ) : (
                                        <Copy className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                                      )}
                                    </button>
                                  </div>
                                </div>
                              </div>
                            )}
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

              {/* Skip/Close Option - Show for manual method only */}
              {integrationMethod === "manual" && (
                <div className="text-center">
                  <button
                    onClick={handleSkipSDK}
                    className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                  >
                    Done (you can access credentials later from agent details)
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
                    value={formData.displayName}
                    onChange={(e) =>
                      setFormData({ ...formData, displayName: e.target.value })
                    }
                    placeholder="e.g., Claude AI Assistant"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.displayName
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.displayName && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.displayName}
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
                      value={formData.agentType}
                      onChange={(e) =>
                        setFormData({
                          ...formData,
                          agentType: e.target.value as AgentType,
                        })
                      }
                      className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                      disabled={loading || success}
                    >
                      {AGENT_TYPE_CATEGORIES.map((category) => (
                        <optgroup key={category} label={category}>
                          {AGENT_TYPE_OPTIONS.filter(o => o.category === category).map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </optgroup>
                      ))}
                    </select>
                    <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {AGENT_TYPE_OPTIONS.find(o => o.value === formData.agentType)?.description}
                    </p>
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

                {/* Repository URL */}
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Repository URL
                  </label>
                  <input
                    ref={repositoryUrlRef}
                    type="url"
                    value={formData.repositoryUrl}
                    onChange={(e) =>
                      setFormData({
                        ...formData,
                        repositoryUrl: e.target.value,
                      })
                    }
                    placeholder="https://github.com/yourusername/your-agent"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.repositoryUrl
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.repositoryUrl && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.repositoryUrl}
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
                    value={formData.documentationUrl}
                    onChange={(e) =>
                      setFormData({
                        ...formData,
                        documentationUrl: e.target.value,
                      })
                    }
                    placeholder="https://docs.example.com/agents/your-agent"
                    className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${errors.documentationUrl
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                      }`}
                    disabled={loading || success}
                  />
                  {errors.documentationUrl && (
                    <p className="mt-1 text-xs text-red-500">
                      {errors.documentationUrl}
                    </p>
                  )}
                </div>
              </div>

              {/* Capabilities */}
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Capabilities <span className="text-red-500">*</span>
                  </label>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                    Select at least one capability this agent has. These define what
                    actions the agent can perform. Format: <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded">namespace:action</code>
                  </p>
                  {errors.capabilities && (
                    <p className="text-xs text-red-500 mb-2">{errors.capabilities}</p>
                  )}
                </div>

                {/* Loading state */}
                {loadingCapabilities ? (
                  <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 py-4">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading capabilities...
                  </div>
                ) : (
                  <>
                    {/* Core capabilities grid */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                      {mergedCapabilities.map((capability) => (
                        <label
                          key={capability.type}
                          className="flex items-start gap-2 p-2 rounded border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer"
                        >
                          <input
                            type="checkbox"
                            checked={formData.capabilities.includes(capability.type)}
                            onChange={() => toggleCapability(capability.type)}
                            className="mt-1 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                            disabled={loading || success}
                          />
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                                {capability.name}
                              </span>
                              <span className={`text-xs px-1.5 py-0.5 rounded ${RISK_LEVEL_COLORS[capability.riskLevel]}`}>
                                {capability.riskLevel}
                              </span>
                            </div>
                            <span className="text-xs text-gray-500 dark:text-gray-400 font-mono">
                              {capability.type}
                            </span>
                          </div>
                        </label>
                      ))}
                    </div>

                    {/* Custom capability input */}
                    <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        Add Custom Capability
                      </label>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">
                        Define custom capabilities for your organization. Reserved namespaces: {reservedNamespaces.join(", ")}
                      </p>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={customCapability}
                          onChange={(e) => {
                            setCustomCapability(e.target.value);
                            if (customCapabilityError) {
                              setCustomCapabilityError(validateCustomCapability(e.target.value));
                            }
                          }}
                          onKeyPress={(e) =>
                            e.key === "Enter" && (e.preventDefault(), addCustomCapability())
                          }
                          placeholder="e.g., payment:process or email:send"
                          className={`flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 font-mono text-sm ${
                            customCapabilityError ? "border-red-500" : "border-gray-200 dark:border-gray-700"
                          }`}
                          disabled={loading || success}
                        />
                        <button
                          type="button"
                          onClick={addCustomCapability}
                          disabled={!customCapability.trim() || loading || success}
                          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                        >
                          <Plus className="h-4 w-4" />
                          Add
                        </button>
                      </div>
                      {customCapabilityError && (
                        <p className="mt-1 text-xs text-red-500">{customCapabilityError}</p>
                      )}
                    </div>
                  </>
                )}
              </div>

              {/* MCP Servers Communication */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  MCP Servers (Talks To)
                </label>
                <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
                  Select MCP servers this agent communicates with. Choose from registered servers or discovered MCPs.
                </p>

                {/* Add MCP Server Input with Autocomplete */}
                <div className="relative">
                  <div className="flex gap-2 mb-3">
                    <div className="relative flex-1">
                      <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                        <Search className="h-4 w-4 text-gray-400" />
                      </div>
                      <input
                        ref={mcpInputRef}
                        type="text"
                        value={newMcpServer}
                        onChange={(e) => {
                          setNewMcpServer(e.target.value);
                          setShowMcpDropdown(true);
                        }}
                        onFocus={() => setShowMcpDropdown(true)}
                        onKeyPress={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            addMcpServer();
                          }
                        }}
                        placeholder="Search or type MCP server name..."
                        className="w-full pl-10 pr-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                        disabled={loading || success}
                      />
                    </div>
                    <button
                      type="button"
                      onClick={() => addMcpServer()}
                      disabled={!newMcpServer.trim() || loading || success}
                      className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                    >
                      <Plus className="h-4 w-4" />
                      Add
                    </button>
                  </div>

                  {/* Autocomplete Dropdown */}
                  {showMcpDropdown && !loading && !success && (
                    <div
                      ref={mcpDropdownRef}
                      className="absolute z-50 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg max-h-60 overflow-y-auto"
                      style={{ top: "calc(100% - 12px)" }}
                    >
                      {loadingMcpSuggestions ? (
                        <div className="p-3 text-center text-gray-500 dark:text-gray-400">
                          <Loader2 className="h-4 w-4 animate-spin mx-auto mb-1" />
                          <span className="text-sm">Loading MCP servers...</span>
                        </div>
                      ) : filteredMcpSuggestions.length > 0 ? (
                        <>
                          {/* Registered MCPs Section */}
                          {filteredMcpSuggestions.filter(s => s.isRegistered).length > 0 && (
                            <>
                              <div className="px-3 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                                Registered MCP Servers
                              </div>
                              {filteredMcpSuggestions.filter(s => s.isRegistered).map((suggestion) => (
                                <button
                                  key={suggestion.id}
                                  type="button"
                                  onClick={() => addMcpServer(suggestion.name)}
                                  className="w-full px-3 py-2 text-left hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-3 border-b border-gray-100 dark:border-gray-700 last:border-b-0"
                                >
                                  <Server className="h-4 w-4 text-green-600 dark:text-green-400 flex-shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                      <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                                        {suggestion.name}
                                      </span>
                                      <span className={`text-xs px-1.5 py-0.5 rounded ${
                                        suggestion.status === "verified"
                                          ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
                                          : suggestion.status === "active"
                                          ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                                          : "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400"
                                      }`}>
                                        {suggestion.status}
                                      </span>
                                    </div>
                                    {suggestion.url && (
                                      <span className="text-xs text-gray-500 dark:text-gray-400 truncate block">
                                        {suggestion.url}
                                      </span>
                                    )}
                                  </div>
                                </button>
                              ))}
                            </>
                          )}

                          {/* Discovered MCPs Section */}
                          {filteredMcpSuggestions.filter(s => s.isDiscovered).length > 0 && (
                            <>
                              <div className="px-3 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                                Discovered (Not Registered)
                              </div>
                              {filteredMcpSuggestions.filter(s => s.isDiscovered).map((suggestion) => (
                                <button
                                  key={suggestion.id}
                                  type="button"
                                  onClick={() => addMcpServer(suggestion.name)}
                                  className="w-full px-3 py-2 text-left hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-3 border-b border-gray-100 dark:border-gray-700 last:border-b-0"
                                >
                                  <Server className="h-4 w-4 text-yellow-600 dark:text-yellow-400 flex-shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                      <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                                        {suggestion.name}
                                      </span>
                                      <span className="text-xs px-1.5 py-0.5 rounded bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400">
                                        discovered
                                      </span>
                                    </div>
                                    {suggestion.url && (
                                      <span className="text-xs text-gray-500 dark:text-gray-400 truncate block">
                                        {suggestion.url}
                                      </span>
                                    )}
                                  </div>
                                </button>
                              ))}
                            </>
                          )}
                        </>
                      ) : (
                        <div className="p-3 text-center text-gray-500 dark:text-gray-400">
                          <span className="text-sm">
                            {mcpSuggestions.length === 0
                              ? "No MCP servers found. Type a name to add manually."
                              : "No matching MCP servers. Press Enter to add as custom."}
                          </span>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Selected MCP Servers List */}
                {formData.talksTo.length > 0 && (
                  <div className="space-y-2 mt-3">
                    <div className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                      Selected MCP Servers ({formData.talksTo.length})
                    </div>
                    {formData.talksTo.map((server) => {
                      const suggestion = mcpSuggestions.find(s => s.name === server);
                      return (
                        <div
                          key={server}
                          className="flex items-center justify-between p-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded"
                        >
                          <div className="flex items-center gap-2">
                            <Server className={`h-4 w-4 ${
                              suggestion?.isRegistered
                                ? "text-green-600 dark:text-green-400"
                                : suggestion?.isDiscovered
                                ? "text-yellow-600 dark:text-yellow-400"
                                : "text-gray-400"
                            }`} />
                            <span className="text-sm text-gray-700 dark:text-gray-300">
                              {server}
                            </span>
                            {suggestion?.isRegistered && (
                              <span className="text-xs px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400">
                                registered
                              </span>
                            )}
                            {suggestion?.isDiscovered && (
                              <span className="text-xs px-1.5 py-0.5 rounded bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400">
                                discovered
                              </span>
                            )}
                          </div>
                          <button
                            type="button"
                            onClick={() => removeMcpServer(server)}
                            disabled={loading || success}
                            className="text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </div>
                      );
                    })}
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
