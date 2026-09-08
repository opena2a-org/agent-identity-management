"use client";

import { useState, useEffect } from "react";
import {
  X,
  Shield,
  Calendar,
  CheckCircle,
  Clock,
  Edit,
  Trash2,
  Key,
  Package,
  Code,
  Download,
  Copy,
  Eye,
  EyeOff,
  ExternalLink,
  Loader2,
} from "lucide-react";
import { Agent, Tag, AgentCapability, api } from "@/lib/api";
import { TagSelector } from "../ui/tag-selector";

// Agent type display configuration
const AGENT_TYPE_LABELS: Record<string, { label: string; color: string }> = {
  // LLM Providers
  claude: { label: "Claude", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  gpt: { label: "GPT", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  gemini: { label: "Gemini", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  llama: { label: "Llama", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  mistral: { label: "Mistral", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  cohere: { label: "Cohere", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Frameworks
  langchain: { label: "LangChain", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  llamaindex: { label: "LlamaIndex", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  langgraph: { label: "LangGraph", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  crewai: { label: "CrewAI", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  autogen: { label: "AutoGen", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  semantic_kernel: { label: "Semantic Kernel", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  haystack: { label: "Haystack", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Copilots & Assistants
  copilot: { label: "Copilot", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  assistant: { label: "Assistant", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  chatbot: { label: "Chatbot", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Autonomous Agents
  autogpt: { label: "AutoGPT", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  babyagi: { label: "BabyAGI", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Other
  custom: { label: "Custom", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Legacy
  ai_agent: { label: "AI Agent", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
};

// Get display info for an agent type
function getAgentTypeDisplay(agentType: string | undefined): { label: string; color: string } {
  if (!agentType) return AGENT_TYPE_LABELS.custom;
  return AGENT_TYPE_LABELS[agentType] || { label: agentType, color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" };
}

interface AgentDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  agent: Agent | null;
  onEdit?: (agent: Agent) => void;
  onDelete?: (agent: Agent) => void;
}

export function AgentDetailModal({
  isOpen,
  onClose,
  agent,
  onEdit,
  onDelete,
}: AgentDetailModalProps) {
  const [agentTags, setAgentTags] = useState<Tag[]>([]);
  const [availableTags, setAvailableTags] = useState<Tag[]>([]);
  const [suggestedTags, setSuggestedTags] = useState<Tag[]>([]);
  const [loadingTags, setLoadingTags] = useState(false);
  const [capabilities, setCapabilities] = useState<AgentCapability[]>([]);
  const [loadingCapabilities, setLoadingCapabilities] = useState(false);

  // Dual-path download state
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
  const [downloadingSDK, setDownloadingSDK] = useState(false);
  const [showCredentialsSection, setShowCredentialsSection] = useState(false);
  const [initialTags, setInitialTags] = useState<Tag[]>([]);

  useEffect(() => {
    if (isOpen && agent) {
      loadTags();
      loadCapabilities();
    }
  }, [isOpen, agent]);

  const loadTags = async () => {
    if (!agent) return;
    setLoadingTags(true);
    try {
      const [currentTags, allTags, suggestions] = await Promise.all([
        api.getAgentTags(agent.id),
        api.listTags(),
        api.suggestTagsForAgent(agent.id),
      ]);
      setAgentTags(currentTags || []);
      setInitialTags(currentTags || []); // Store initial state
      setAvailableTags(allTags || []);
      setSuggestedTags(suggestions || []);
    } catch (error) {
      console.error("Failed to load tags:", error);
    } finally {
      setLoadingTags(false);
    }
  };

  const loadCapabilities = async () => {
    if (!agent) return;
    setLoadingCapabilities(true);
    try {
      const caps = await api.getAgentCapabilities(agent.id, true);
      setCapabilities(caps || []);
    } catch (error) {
      console.error("Failed to load capabilities:", error);
    } finally {
      setLoadingCapabilities(false);
    }
  };

  const handleTagsChange = async (newTags: Tag[]) => {
    if (!agent) return;

    const addedTags = newTags.filter(
      (t) => !agentTags.some((at) => at.id === t.id)
    );
    const removedTags = agentTags.filter(
      (t) => !newTags.some((nt) => nt.id === t.id)
    );

    try {
      // Add new tags
      if (addedTags.length > 0) {
        await api.addTagsToAgent(
          agent.id,
          addedTags.map((t) => t.id)
        );
      }

      // Remove tags
      for (const tag of removedTags) {
        await api.removeTagFromAgent(agent.id, tag.id);
      }

      setAgentTags(newTags);
    } catch (error) {
      console.error("Failed to update tags:", error);
    }
  };

  const handleDownloadSDK = async (language: "python" | "node" | "go") => {
    if (!agent) return;

    setDownloadingSDK(true);
    try {
      const token = api.getToken();

      // Get runtime-detected API URL from api client's baseURL
      const apiBaseURL = (api as any).baseURL;

      const response = await fetch(
        `${apiBaseURL}/api/v1/agents/${agent.id}/sdk?language=${language}`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!response.ok) {
        throw new Error("Failed to download SDK");
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${agent?.name || "agent"}-${language}-sdk.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err) {
      console.error("Failed to download SDK:", err);
      alert(
        "Failed to download SDK. Please try again or use Manual Integration."
      );
    } finally {
      setDownloadingSDK(false);
    }
  };

  const fetchAgentKeys = async () => {
    if (!agent) return;

    setLoadingKeys(true);
    try {
      const token = api.getToken();

      // Get runtime-detected API URL from api client's baseURL
      const apiBaseURL = (api as any).baseURL;

      const response = await fetch(
        `${apiBaseURL}/api/v1/agents/${agent.id}/credentials`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(
          `Failed to fetch agent keys: ${response.status} ${errorText}`
        );
      }

      const data = await response.json();
      setAgentKeys({
        publicKey: data?.publicKey || "",
        privateKey: data?.privateKey || "",
      });
    } catch (err) {
      console.error("Failed to fetch agent keys:", err);
      alert(
        "Failed to fetch agent credentials. Please try again or contact support."
      );
      setIntegrationMethod(null); // Reset to main selection
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

  // Check if tags have been modified
  const hasUnsavedChanges = () => {
    if (initialTags.length !== agentTags.length) return true;
    const initialIds = initialTags.map((t) => t.id).sort();
    const currentIds = agentTags.map((t) => t.id).sort();
    return JSON.stringify(initialIds) !== JSON.stringify(currentIds);
  };

  // Handle click on overlay (outside modal)
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    // Only close if clicking the overlay itself, not its children
    if (e.target === e.currentTarget) {
      if (hasUnsavedChanges()) {
        if (
          confirm(
            "You have unsaved tag changes. Are you sure you want to close without saving?"
          )
        ) {
          onClose();
        }
      } else {
        onClose();
      }
    }
  };

  if (!isOpen || !agent) return null;

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString("en-US", {
      month: "long",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "verified":
        return "border border-success-border bg-success-fill text-success-text";
      case "pending":
        return "border border-warning-border bg-warning-fill text-warning-text";
      case "suspended":
      case "revoked":
        return "border border-danger-border bg-danger-fill text-danger-text";
      default:
        return "border border-glass-inset-border bg-glass-inset-gray text-ink-body";
    }
  };

  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return "text-success-text";
    if (score >= 60) return "text-warning-text";
    return "text-danger-text";
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(29,29,31,0.45)] backdrop-blur-sm"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="overlay-surface max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-divider">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-logo rounded-inset flex items-center justify-center">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-ink">
                {agent.displayName}
              </h2>
              <p className="text-sm text-ink-secondary">
                {agent.name}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-ink-tertiary hover:text-ink transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-6">
          {/* Status and Trust Score */}
          <div className="flex items-center gap-4">
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                Status
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-pill text-sm font-medium capitalize ${getStatusColor(agent.status)}`}
              >
                {agent.status}
              </span>
            </div>
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                Trust score
              </span>
              <span
                className={`text-2xl font-bold ${getTrustScoreColor(agent.trustScore)}`}
              >
                {agent.trustScore <= 1
                  ? Math.round(agent.trustScore * 100)
                  : Math.round(agent.trustScore)}
                %
              </span>
            </div>
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                Type
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-pill text-sm font-medium ${getAgentTypeDisplay(agent.agentType).color}`}
              >
                {getAgentTypeDisplay(agent.agentType).label}
              </span>
            </div>
          </div>

          {/* Description */}
          {agent.description && (
            <div>
              <h3 className="text-sm font-medium text-ink mb-2">
                Description
              </h3>
              <p className="text-sm text-ink-body">
                {agent.description}
              </p>
            </div>
          )}

          {/* Tags */}
          <div>
            <h3 className="text-sm font-medium text-ink mb-3">
              Tags
            </h3>
            {loadingTags ? (
              <div className="text-sm text-ink-secondary">
                Loading tags...
              </div>
            ) : (
              <TagSelector
                selectedTags={agentTags}
                availableTags={availableTags}
                suggestedTags={suggestedTags}
                onTagsChange={handleTagsChange}
              />
            )}
          </div>

          {/* Capabilities */}
          <div>
            <h3 className="text-sm font-medium text-ink mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {loadingCapabilities ? (
              <div className="text-sm text-ink-secondary">
                Loading capabilities...
              </div>
            ) : capabilities && capabilities.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {capabilities.map((capability) => (
                  <div
                    key={capability.id}
                    className="inline-flex items-center gap-2 px-3 py-2 bg-brand-soft border border-stroke rounded-inset-sm"
                  >
                    <CheckCircle className="h-4 w-4 text-brand-text flex-shrink-0" />
                    <div>
                      <p className="text-sm font-medium text-ink">
                        {capability.capabilityType}
                      </p>
                      {capability.capabilityScope &&
                        Object.keys(capability.capabilityScope).length > 0 && (
                          <p className="text-xs text-brand-text">
                            {Object.entries(capability.capabilityScope)
                              .map(([key, value]) => `${key}: ${value}`)
                              .join(", ")}
                          </p>
                        )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-ink-secondary italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Talks To (MCP Servers) */}
          <div>
            <h3 className="text-sm font-medium text-ink mb-3 flex items-center gap-2">
              <svg
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                />
              </svg>
              Talks to (MCP servers)
            </h3>
            {agent.talksTo && agent.talksTo.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {agent.talksTo.map((mcpServer, index) => (
                  <div
                    key={index}
                    className="px-3 py-2 border border-glass-inset-border bg-glass-inset-gray text-ink-body rounded-inset-sm text-sm font-medium"
                  >
                    {mcpServer}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-ink-secondary italic">
                No MCP servers configured
              </div>
            )}
          </div>

          {/* Details Grid */}
          <div className="grid grid-cols-2 gap-6">
            <div>
              <h3 className="text-sm font-medium text-ink mb-2">
                Version
              </h3>
              <p className="text-sm text-ink font-mono">
                {agent.version}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink mb-2">
                Organization ID
              </h3>
              <p className="text-sm text-ink font-mono">
                {agent.organizationId}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Created
              </h3>
              <p className="text-sm text-ink">
                {formatDate(agent.createdAt)}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last updated
              </h3>
              <p className="text-sm text-ink">
                {formatDate(agent.updatedAt)}
              </p>
            </div>
          </div>

          {/* Audit History */}
          <div>
            <h3 className="text-sm font-medium text-ink mb-3">
              Recent activity
            </h3>
            <div className="space-y-2">
              <div className="flex items-center gap-3 p-3 glass-inset">
                <CheckCircle className="h-4 w-4 text-success-text" />
                <div className="flex-1">
                  <p className="text-sm text-ink">
                    Agent registered
                  </p>
                  <p className="text-xs text-ink-secondary">
                    {formatDate(agent.createdAt)}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-3 p-3 glass-inset">
                <CheckCircle className="h-4 w-4 text-brand-text" />
                <div className="flex-1">
                  <p className="text-sm text-ink">
                    Agent updated
                  </p>
                  <p className="text-xs text-ink-secondary">
                    {formatDate(agent.updatedAt)}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-divider">
          {onDelete && (
            <button
              onClick={() => onDelete(agent)}
              className="px-4 py-2 text-sm font-medium text-danger-text hover:bg-danger-fill rounded-pill transition-colors flex items-center gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(agent)}
              className="px-4 py-2 text-sm font-medium rounded-pill bg-brand text-white shadow-glow hover:bg-brand-hover transition-colors flex items-center gap-2"
            >
              <Edit className="h-4 w-4" />
              Edit agent
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
