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
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "pending":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "suspended":
      case "revoked":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return "text-green-600 dark:text-green-400";
    if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
    return "text-red-600 dark:text-red-400";
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      style={{ margin: 0 }}
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                {agent.displayName}
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {agent.name}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 space-y-6">
          {/* Status and Trust Score */}
          <div className="flex items-center gap-4">
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                Status
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium capitalize ${getStatusColor(agent.status)}`}
              >
                {agent.status}
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                Trust Score
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
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                Type
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
                  agent.agentType === "ai_agent"
                    ? "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300"
                    : "bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300"
                }`}
              >
                {agent.agentType === "ai_agent" ? "AI Agent" : "MCP Server"}
              </span>
            </div>
          </div>

          {/* Description */}
          {agent.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Description
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                {agent.description}
              </p>
            </div>
          )}

          {/* Tags */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              Tags
            </h3>
            {loadingTags ? (
              <div className="text-sm text-gray-500 dark:text-gray-400">
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
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {loadingCapabilities ? (
              <div className="text-sm text-gray-500 dark:text-gray-400">
                Loading capabilities...
              </div>
            ) : capabilities && capabilities.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {capabilities.map((capability) => (
                  <div
                    key={capability.id}
                    className="inline-flex items-center gap-2 px-3 py-2 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md"
                  >
                    <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400 flex-shrink-0" />
                    <div>
                      <p className="text-sm font-medium text-blue-900 dark:text-blue-100">
                        {capability.capabilityType}
                      </p>
                      {capability.capabilityScope &&
                        Object.keys(capability.capabilityScope).length > 0 && (
                          <p className="text-xs text-blue-600 dark:text-blue-400">
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
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Talks To (MCP Servers) */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
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
              Talks To (MCP Servers)
            </h3>
            {agent.talksTo && agent.talksTo.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {agent.talksTo.map((mcpServer, index) => (
                  <div
                    key={index}
                    className="px-3 py-2 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-md text-sm font-medium text-purple-900 dark:text-purple-100"
                  >
                    {mcpServer}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No MCP servers configured
              </div>
            )}
          </div>

          {/* Download SDK / View Credentials */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 flex items-center gap-2">
                <Download className="h-4 w-4" />
                Download SDK / View Credentials
              </h3>
              <button
                onClick={() =>
                  setShowCredentialsSection(!showCredentialsSection)
                }
                className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
                title={showCredentialsSection ? "Hide details" : "Show details"}
              >
                {showCredentialsSection ? (
                  <EyeOff className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                ) : (
                  <Eye className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                )}
              </button>
            </div>

            {showCredentialsSection && !integrationMethod && (
              <div className="space-y-3">
                <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                  Access your agent's SDK or view credentials for manual
                  integration
                </p>

                {/* SDK Integration Option */}
                <button
                  onClick={() => setIntegrationMethod("sdk")}
                  className="w-full p-4 border-2 border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20 rounded-lg hover:border-blue-300 dark:hover:border-blue-700 transition-colors text-left"
                >
                  <div className="flex items-start gap-3">
                    <Package className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
                    <div className="flex-1">
                      <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                        📦 Download Python SDK (Recommended)
                      </h4>
                      <p className="text-xs text-blue-800 dark:text-blue-200">
                        Download production-ready <strong>Python SDK</strong> with
                        cryptographic keys and automatic verification. 100% test coverage.
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
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1">
                        🔧 View Credentials (Manual Integration)
                      </h4>
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

            {/* SDK Download UI */}
            {showCredentialsSection && integrationMethod === "sdk" && (
              <div className="p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg space-y-4">
                <div className="flex items-start gap-3">
                  <Package className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                  <div className="flex-1">
                    <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-1">
                      📦 Download Python SDK
                    </h4>
                    <p className="text-xs text-blue-800 dark:text-blue-200 mb-3">
                      Production-ready Python SDK with Ed25519 cryptographic signing, OAuth integration,
                      MCP auto-detection, and 100% test coverage.
                    </p>

                    <div className="space-y-3">
                      <button
                        onClick={() => handleDownloadSDK("python")}
                        disabled={downloadingSDK}
                        className="w-full px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 shadow-md"
                      >
                        {downloadingSDK ? (
                          <>
                            <Loader2 className="h-4 w-4 animate-spin" />
                            Downloading...
                          </>
                        ) : (
                          <>
                            <Download className="h-4 w-4" />
                            Download Python SDK
                          </>
                        )}
                      </button>

                      <div className="bg-white dark:bg-gray-900 border border-blue-200 dark:border-blue-800 rounded p-3">
                        <p className="text-xs text-blue-800 dark:text-blue-200 mb-2">
                          <strong>Future Releases:</strong>
                        </p>
                        <p className="text-xs text-blue-700 dark:text-blue-300">
                          Go and JavaScript/TypeScript SDKs are planned for Q1-Q2 2026.
                          The Python SDK provides complete functionality today.
                        </p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Change Method */}
                <div className="text-center pt-3 border-t border-blue-200 dark:border-blue-800">
                  <button
                    onClick={() => setIntegrationMethod(null)}
                    className="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 underline"
                  >
                    ← Choose a different option
                  </button>
                </div>
              </div>
            )}

            {/* Manual Integration Credentials Display */}
            {showCredentialsSection && integrationMethod === "manual" && (
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
                        {/* Agent ID with Copy Button */}
                        <div>
                          <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Agent ID
                          </label>
                          <div className="flex gap-2">
                            <input
                              type="text"
                              value={agent.id}
                              readOnly
                              className="flex-1 px-3 py-2 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded text-xs font-mono"
                            />
                            <button
                              onClick={() =>
                                copyToClipboard(agent.id, "agent_id")
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

                        {/* Public Key with Copy Button */}
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

                        {/* Private Key with Reveal/Hide and Copy */}
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
                        Failed to load credentials. Please try again.
                      </p>
                    )}
                  </div>
                </div>

                {/* Change Method */}
                <div className="text-center pt-3 border-t border-gray-200 dark:border-gray-700">
                  <button
                    onClick={() => setIntegrationMethod(null)}
                    className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                  >
                    ← Choose a different option
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Details Grid */}
          <div className="grid grid-cols-2 gap-6">
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Version
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">
                {agent.version}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Organization ID
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">
                {agent.organizationId}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Created
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {formatDate(agent.createdAt)}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last Updated
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {formatDate(agent.updatedAt)}
              </p>
            </div>
          </div>

          {/* Audit History */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              Recent Activity
            </h3>
            <div className="space-y-2">
              <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                <div className="flex-1">
                  <p className="text-sm text-gray-900 dark:text-gray-100">
                    Agent registered
                  </p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    {formatDate(agent.createdAt)}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <div className="flex-1">
                  <p className="text-sm text-gray-900 dark:text-gray-100">
                    Agent updated
                  </p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    {formatDate(agent.updatedAt)}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
          {onDelete && (
            <button
              onClick={() => onDelete(agent)}
              className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex items-center gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(agent)}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors flex items-center gap-2"
            >
              <Edit className="h-4 w-4" />
              Edit Agent
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
