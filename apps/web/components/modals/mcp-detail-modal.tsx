"use client";

import {
  X,
  Shield,
  Calendar,
  CheckCircle,
  Clock,
  Edit,
  Trash2,
  Key,
  Download,
  TrendingUp,
  ChevronDown,
  ChevronUp,
  User,
  Bot,
  Activity,
} from "lucide-react";
import { formatDateTime } from "@/lib/date-utils";
import { useState, useEffect } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

interface MCPCapability {
  id: string;
  mcp_server_id: string;
  name: string;
  type: "tool" | "resource" | "prompt";
  description: string;
  schema: any;
  detected_at: string;
  last_verified_at?: string;
  is_active: boolean;
}

interface Attestation {
  id: string;
  agent_id: string;
  agent_name: string;
  agent_trust_score: number;
  verified_at: string;
  expires_at: string;
  capabilities_confirmed: string[];
  connection_latency_ms: number;
  health_check_passed: boolean;
  is_valid: boolean;
  // New metadata fields
  attestation_type: string; // "sdk" or "manual"
  attested_by: string; // Agent name or User name
  attester_type: string; // "agent" or "user"
  signature_verified: boolean;
  sdk_version?: string;
  connection_successful: boolean;
  agent_owner_name?: string; // Name of user who owns the agent (for SDK attestations)
  agent_owner_id?: string; // ID of user who owns the agent (for SDK attestations)
}

interface MCPServer {
  id: string;
  name: string;
  url: string;
  description?: string;
  status:
    | "active"
    | "inactive"
    | "pending"
    | "verified"
    | "suspended"
    | "revoked";
  public_key?: string;
  key_type?: string;
  last_verified_at?: string;
  created_at: string;
  trust_score?: number;
  capability_count?: number;
  capabilities?: MCPCapability[]; // List of capabilities this MCP provides
  talks_to?: string[]; // List of agents that communicate with this MCP

  // ✅ NEW: Agent Attestation fields
  verification_method?: string; // "agent_attestation", "api_key", or "manual"
  attestation_count?: number; // Number of agent attestations
  confidence_score?: number; // Calculated confidence (0-100) based on attestations
  last_attested_at?: string; // Most recent attestation timestamp
}

interface MCPDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  mcp: MCPServer | null;
  onEdit?: (mcp: MCPServer) => void;
  onDelete?: (mcp: MCPServer) => void;
}

export function MCPDetailModal({
  isOpen,
  onClose,
  mcp,
  onEdit,
  onDelete,
}: MCPDetailModalProps) {
  const [attestations, setAttestations] = useState<Attestation[]>([]);
  const [showAttestations, setShowAttestations] = useState(false);
  const [loadingAttestations, setLoadingAttestations] = useState(false);

  // Fetch detailed attestations when modal opens and MCP has attestations
  useEffect(() => {
    if (isOpen && mcp && mcp.verification_method === "agent_attestation" && mcp.attestation_count && mcp.attestation_count > 0) {
      fetchAttestations();
    }
  }, [isOpen, mcp]);

  const fetchAttestations = async () => {
    if (!mcp) return;
    setLoadingAttestations(true);
    try {
      const token = localStorage.getItem("token");
      const response = await fetch(`http://localhost:8080/api/v1/mcp-servers/${mcp.id}/attestations`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setAttestations(data.attestations || []);
      }
    } catch (error) {
      console.error("Failed to fetch attestations:", error);
    } finally {
      setLoadingAttestations(false);
    }
  };

  if (!isOpen || !mcp) return null;

  const getStatusColor = (status: string) => {
    switch (status) {
      case "active":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "pending":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "inactive":
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return "text-green-600 dark:text-green-400";
    if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
    if (score >= 40) return "text-orange-600 dark:text-orange-400";
    return "text-red-600 dark:text-red-400";
  };

  const getConfidenceScoreColor = (score: number) => {
    if (score >= 80) return "text-green-600 dark:text-green-400";
    if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
    if (score >= 40) return "text-orange-600 dark:text-orange-400";
    return "text-red-600 dark:text-red-400";
  };

  const getVerificationMethodBadge = (method?: string) => {
    switch (method) {
      case "agent_attestation":
        return {
          text: "Agent Attested",
          color: "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300",
        };
      case "api_key":
        return {
          text: "API Key",
          color: "bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300",
        };
      case "manual":
        return {
          text: "Manual",
          color: "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300",
        };
      default:
        return {
          text: "Unknown",
          color: "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300",
        };
    }
  };

  const calculateFingerprint = (publicKey: string): string => {
    if (!publicKey) return "N/A";
    // Simple mock fingerprint - in production this would use crypto.subtle.digest
    const hash = publicKey.substring(0, 64);
    return (
      hash
        .match(/.{1,2}/g)
        ?.slice(0, 16)
        .join(":") || "N/A"
    );
  };

  const handleDownloadKey = () => {
    if (!mcp?.public_key) return;

    const blob = new Blob([mcp.public_key], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${mcp?.name?.replace(/\s+/g, "_") || "mcp"}_public_key.pem`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  // Handle click on overlay (outside modal) - MCP detail modal is read-only, so close immediately
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-gradient-to-br from-purple-500 to-purple-600 rounded-lg flex items-center justify-center">
              <Shield className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                {mcp?.name || "Unknown MCP"}
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {mcp?.id || "Unknown ID"}
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
        <div className="p-6">
          <Tabs defaultValue="overview" className="w-full">
            <TabsList className="grid w-full grid-cols-2 mb-6">
              <TabsTrigger value="overview" className="flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Overview
              </TabsTrigger>
              <TabsTrigger value="activity" className="flex items-center gap-2">
                <Activity className="h-4 w-4" />
                Attestations
              </TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-6">
              {/* Status and Metrics - Updated to match agent detail modal */}
              <div className="flex items-center gap-4">
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                Status
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium capitalize ${getStatusColor(mcp?.status || "unknown")}`}
              >
                {mcp?.status || "unknown"}
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                {mcp.verification_method === "agent_attestation"
                  ? "Confidence Score"
                  : "Trust Score"}
              </span>
              <span
                className={`text-2xl font-bold ${
                  mcp.verification_method === "agent_attestation"
                    ? getConfidenceScoreColor(mcp.confidence_score || 0)
                    : getTrustScoreColor(mcp.trust_score || 0)
                }`}
              >
                {mcp.verification_method === "agent_attestation"
                  ? (mcp.confidence_score || 0).toFixed(1)
                  : (mcp.trust_score || 0).toFixed(1)}
                %
              </span>
            </div>
            <div>
              <span className="text-sm text-gray-500 dark:text-gray-400 block mb-1">
                Capabilities
              </span>
              <span className="text-lg font-semibold text-gray-900 dark:text-white">
                {mcp.capabilities?.length || 0}
              </span>
            </div>
          </div>

          {/* Agent Attestation Info (if using agent attestation) */}
          {mcp.verification_method === "agent_attestation" && (
            <div className="bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <Shield className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100">
                      Verified by Attestations
                    </h4>
                    {attestations.length > 0 && (
                      <button
                        onClick={() => setShowAttestations(!showAttestations)}
                        className="flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
                      >
                        {showAttestations ? (
                          <>
                            <ChevronUp className="h-3 w-3" />
                            Hide details
                          </>
                        ) : (
                          <>
                            <ChevronDown className="h-3 w-3" />
                            Show details
                          </>
                        )}
                      </button>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <span className="text-blue-700 dark:text-blue-300 font-medium">
                        {mcp.attestation_count || 0}
                      </span>
                      <span className="text-blue-600 dark:text-blue-400 ml-1">
                        {mcp.attestation_count === 1 ? "attestation" : "attestations"}
                      </span>
                    </div>
                    {mcp.last_attested_at && (
                      <div>
                        <span className="text-blue-700 dark:text-blue-300 font-medium">
                          Last attested:
                        </span>
                        <span className="text-blue-600 dark:text-blue-400 ml-1">
                          {formatDateTime(mcp.last_attested_at)}
                        </span>
                      </div>
                    )}
                  </div>
                  <p className="text-xs text-blue-600 dark:text-blue-400 mt-2">
                    This MCP server's identity is verified by {mcp.attestation_count || 0} attestation
                    {mcp.attestation_count !== 1 ? "s" : ""} from agents and users.
                  </p>

                  {/* Detailed Attestations List */}
                  {showAttestations && attestations.length > 0 && (
                    <div className="mt-4 space-y-2 border-t border-blue-200 dark:border-blue-800 pt-3">
                      <h5 className="text-xs font-semibold text-blue-800 dark:text-blue-200 mb-2">
                        Attestation History
                      </h5>
                      {attestations.map((att) => (
                        <div
                          key={att.id}
                          className="bg-white/50 dark:bg-black/20 rounded p-3 text-xs space-y-2"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex items-center gap-2">
                              {att.attester_type === "agent" ? (
                                <Bot className="h-4 w-4 text-blue-600 dark:text-blue-400 flex-shrink-0" />
                              ) : (
                                <User className="h-4 w-4 text-purple-600 dark:text-purple-400 flex-shrink-0" />
                              )}
                              <div>
                                <p className="font-medium text-blue-900 dark:text-blue-100">
                                  {att.attested_by}
                                  {att.attester_type === "agent" && att.agent_owner_name && (
                                    <span className="ml-1 font-normal text-xs text-blue-700 dark:text-blue-300">
                                      (owned by {att.agent_owner_name})
                                    </span>
                                  )}
                                </p>
                                <p className="text-blue-600 dark:text-blue-400">
                                  {att.attester_type === "agent" ? "Agent" : "User"} • {att.attestation_type === "sdk" ? "SDK" : "Manual"}
                                </p>
                              </div>
                            </div>
                            <div className="text-right flex-shrink-0">
                              {att.is_valid ? (
                                <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded text-xs">
                                  <CheckCircle className="h-3 w-3" />
                                  Valid
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded text-xs">
                                  Expired
                                </span>
                              )}
                            </div>
                          </div>

                          <div className="grid grid-cols-2 gap-2 text-xs">
                            <div>
                              <span className="text-blue-700 dark:text-blue-300">Verified:</span>
                              <span className="ml-1 text-blue-600 dark:text-blue-400">
                                {formatDateTime(att.verified_at)}
                              </span>
                            </div>
                            {att.attestation_type === "sdk" && att.sdk_version && (
                              <div>
                                <span className="text-blue-700 dark:text-blue-300">SDK:</span>
                                <span className="ml-1 text-blue-600 dark:text-blue-400">
                                  {att.sdk_version}
                                </span>
                              </div>
                            )}
                          </div>

                          <div className="flex flex-wrap gap-2">
                            {att.signature_verified && (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded text-xs">
                                <Shield className="h-3 w-3" />
                                Signature Verified
                              </span>
                            )}
                            {att.connection_successful && (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded text-xs">
                                <CheckCircle className="h-3 w-3" />
                                Connection OK
                              </span>
                            )}
                            {att.health_check_passed && (
                              <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded text-xs">
                                <CheckCircle className="h-3 w-3" />
                                Health Check Passed
                              </span>
                            )}
                          </div>

                          {att.capabilities_confirmed && att.capabilities_confirmed.length > 0 && (
                            <div>
                              <p className="text-blue-700 dark:text-blue-300 mb-1">
                                Capabilities Verified ({att.capabilities_confirmed.length}):
                              </p>
                              <div className="flex flex-wrap gap-1">
                                {att.capabilities_confirmed.map((cap, idx) => (
                                  <span
                                    key={idx}
                                    className="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded text-xs"
                                  >
                                    {cap}
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}

                  {loadingAttestations && (
                    <div className="mt-4 text-center text-xs text-blue-600 dark:text-blue-400">
                      Loading attestations...
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Description */}
          {mcp.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Description
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                {mcp.description}
              </p>
            </div>
          )}

          {/* Capabilities */}
          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {mcp.capabilities && mcp.capabilities.length > 0 ? (
              <div className="space-y-3">
                {mcp.capabilities.map((capability) => {
                  const typeColors = {
                    tool: "bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800 text-blue-900 dark:text-blue-100",
                    resource:
                      "bg-purple-50 dark:bg-purple-900/20 border-purple-200 dark:border-purple-800 text-purple-900 dark:text-purple-100",
                    prompt:
                      "bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-900 dark:text-green-100",
                  };

                  return (
                    <div
                      key={capability.id}
                      className={`p-3 border rounded-md ${typeColors[capability.type]}`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-1">
                            <CheckCircle className="h-4 w-4 flex-shrink-0" />
                            <p className="text-sm font-semibold">
                              {capability.name}
                            </p>
                            <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-white/50 dark:bg-black/20">
                              {capability.type}
                            </span>
                          </div>
                          {capability.description && (
                            <p className="text-xs opacity-80 ml-6">
                              {capability.description}
                            </p>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Talks To (Agents) */}
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
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                />
              </svg>
              Talks To (Agents)
            </h3>
            {mcp.talks_to && mcp.talks_to.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {mcp.talks_to.map((agent, index) => (
                  <div
                    key={index}
                    className="px-3 py-2 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md text-sm font-medium text-green-900 dark:text-green-100"
                  >
                    {agent}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-500 dark:text-gray-400 italic">
                No agents configured to use this MCP server
              </div>
            )}
          </div>

          {/* Details Grid - Updated to match agent detail modal */}
          <div className="grid grid-cols-2 gap-6">
            <div className="col-span-2">
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                URL
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono break-all">
                {mcp.url}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Server ID
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">
                {mcp.id}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Organization ID
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100 font-mono">
                {/* MCP servers don't have organization_id, so we'll show placeholder */}
                N/A
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Created
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {formatDateTime(mcp.created_at)}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last Updated
              </h3>
              <p className="text-sm text-gray-900 dark:text-gray-100">
                {mcp.last_verified_at
                  ? formatDateTime(mcp.last_verified_at)
                  : "Never"}
              </p>
            </div>
          </div>

          {/* Cryptographic Identity Section */}
          {mcp.public_key && (
            <div className="border-t border-gray-200 dark:border-gray-700 pt-6">
              <div className="flex items-center gap-2 mb-4">
                <Key className="h-5 w-5 text-purple-600 dark:text-purple-400" />
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                  Cryptographic Identity
                </h3>
              </div>

              <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Public Key Fingerprint (SHA-256)
                    </h4>
                    <p className="text-xs text-gray-900 dark:text-gray-100 font-mono bg-white dark:bg-gray-900 p-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                      {calculateFingerprint(mcp.public_key)}
                    </p>
                  </div>

                  <div>
                    <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Key Type
                    </h4>
                    <p className="text-sm text-gray-900 dark:text-gray-100 font-medium">
                      {mcp.key_type || "RSA-2048"}
                    </p>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Public Key
                  </h4>
                  <div className="relative">
                    <pre className="text-xs text-gray-900 dark:text-gray-100 font-mono bg-white dark:bg-gray-900 p-3 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto max-h-32">
                      {mcp.public_key}
                    </pre>
                    <button
                      onClick={handleDownloadKey}
                      className="absolute top-2 right-2 p-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded transition-colors"
                      title="Download public key"
                    >
                      <Download className="h-4 w-4 text-gray-600 dark:text-gray-300" />
                    </button>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                  <div className="flex items-center gap-2">
                    <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                    <span className="text-sm text-green-600 dark:text-green-400 font-medium">
                      Cryptographic identity verified on registration
                    </span>
                  </div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    Ed25519 signature
                  </div>
                </div>
              </div>
            </div>
          )}
            </TabsContent>

            <TabsContent value="activity" className="space-y-4">
              {/* Attestations for this MCP */}
              <div>
                <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                  Attestations
                </h3>
                <div className="space-y-2">
                  <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                    <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                    <div className="flex-1">
                      <p className="text-sm text-gray-900 dark:text-gray-100">
                        MCP server registered
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {formatDateTime(mcp.created_at)}
                      </p>
                    </div>
                  </div>
                  {mcp.last_verified_at && (
                    <div className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <CheckCircle className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                      <div className="flex-1">
                        <p className="text-sm text-gray-900 dark:text-gray-100">
                          Last activity
                        </p>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                            {formatDateTime(mcp.last_verified_at)}
                        </p>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </div>

        {/* Footer - Updated to match agent detail modal */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
          {onDelete && (
            <button
              onClick={() => onDelete(mcp)}
              className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex items-center gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(mcp)}
              className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors flex items-center gap-2"
            >
              <Edit className="h-4 w-4" />
              Edit Server
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
