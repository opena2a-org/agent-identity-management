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
  KeyRound,
  Activity,
  Bot,
  User,
} from "lucide-react";
import { formatDateTime } from "@/lib/date-utils";
import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";

interface MCPCapability {
  id: string;
  mcpServerId: string;
  name: string;
  type: "tool" | "resource" | "prompt";
  description: string;
  schema: any;
  detectedAt: string;
  lastVerifiedAt?: string;
  isActive: boolean;
}

interface Attestation {
  id: string;
  agentId: string;
  agentName: string;
  agentTrustScore: number;
  verifiedAt: string;
  expiresAt: string;
  capabilitiesConfirmed: string[];
  connectionLatencyMs: number;
  healthCheckPassed: boolean;
  isValid: boolean;
  // New metadata fields
  attestationType: string; // "sdk" or "manual"
  attestedBy: string; // Agent name or User name
  attesterType: string; // "agent" or "user"
  signatureVerified: boolean;
  sdkVersion?: string;
  connectionSuccessful: boolean;
  agentOwnerName?: string; // Name of user who owns the agent (for SDK attestations)
  agentOwnerId?: string; // ID of user who owns the agent (for SDK attestations)
}

interface AuditLog {
  id: string;
  organizationId: string;
  userId?: string;
  agentId?: string;
  action: string;
  resourceType: string;
  resourceId: string;
  ipAddress: string;
  userAgent: string;
  metadata: Record<string, any>;
  timestamp: string;
  // Populated via JOIN queries (from backend)
  agentName?: string;
  userName?: string;
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
  publicKey?: string;
  keyType?: string;
  lastVerifiedAt?: string;
  createdAt: string;
  trustScore?: number;
  capabilityCount?: number;
  capabilities?: MCPCapability[]; // List of capabilities this MCP provides
  talksTo?: string[]; // List of agents that communicate with this MCP

  // ✅ NEW: Agent Attestation fields
  verificationMethod?: string; // "agent_attestation", "api_key", or "manual"
  attestationCount?: number; // Number of agent attestations
  confidenceScore?: number; // Calculated confidence (0-100) based on attestations
  lastAttestedAt?: string; // Most recent attestation timestamp

  // ✅ Audit trail fields
  createdBy?: string;           // User UUID who created this server
  createdByName?: string;       // Name of the creator
  createdByEmail?: string;      // Email of the creator
  createdBySdkTokenId?: string; // SDK token used to create this server
  createdByApiKeyId?: string;   // API key used to create this server
  updatedBy?: string;           // User UUID who last updated this server
  updatedByName?: string;       // Name of the updater
  updatedByEmail?: string;      // Email of the updater
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
  const router = useRouter();
  const [attestations, setAttestations] = useState<Attestation[]>([]);
  const [showAttestations, setShowAttestations] = useState(false);
  const [loadingAttestations, setLoadingAttestations] = useState(false);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [loadingAuditLogs, setLoadingAuditLogs] = useState(false);

  // Fetch detailed attestations when modal opens and MCP has attestations
  useEffect(() => {
    if (isOpen && mcp && mcp.verificationMethod === "agent_attestation" && mcp.attestationCount && mcp.attestationCount > 0) {
      fetchAttestations();
    }
  }, [isOpen, mcp]);

  // Fetch audit logs when modal opens
  useEffect(() => {
    if (isOpen && mcp) {
      fetchAuditLogs();
    }
  }, [isOpen, mcp]);

  const fetchAttestations = async () => {
    if (!mcp) return;
    setLoadingAttestations(true);
    try {
      const token = localStorage.getItem("token");
      // Dynamic API URL based on environment
      const apiUrl = process.env.NEXT_PUBLIC_API_URL ||
        (typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
          ? `${window.location.protocol}//${window.location.hostname}:8080`
          : `${window.location.protocol}//${window.location.host}`);
      const response = await fetch(`${apiUrl}/api/v1/mcp-servers/${mcp.id}/attestations`, {
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

  const fetchAuditLogs = async () => {
    if (!mcp) return;
    setLoadingAuditLogs(true);
    try {
      const token = localStorage.getItem("token");
      // Dynamic API URL based on environment
      const apiUrl = process.env.NEXT_PUBLIC_API_URL ||
        (typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
          ? `${window.location.protocol}//${window.location.hostname}:8080`
          : `${window.location.protocol}//${window.location.host}`);
      const response = await fetch(`${apiUrl}/api/v1/mcp-servers/${mcp.id}/audit-logs?limit=50`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setAuditLogs(data.logs || []);
      }
    } catch (error) {
      console.error("Failed to fetch audit logs:", error);
    } finally {
      setLoadingAuditLogs(false);
    }
  };

  // Helper to get action display info
  const getActionDisplayInfo = (action: string) => {
    const actionMap: Record<string, { label: string; color: string }> = {
      // Standard action names from backend
      "create": { label: "Created", color: "text-success-text" },
      "update": { label: "Updated", color: "text-brand-text" },
      "delete": { label: "Deleted", color: "text-danger-text" },
      "verify": { label: "Verified", color: "text-success-text" },
      "attest": { label: "Attestation recorded", color: "text-brand-text" },
      "view": { label: "Viewed", color: "text-ink-secondary" },
      // Legacy/alternative action names for compatibility
      "mcp.created": { label: "MCP created", color: "text-success-text" },
      "mcp.updated": { label: "MCP updated", color: "text-brand-text" },
      "mcp.deleted": { label: "MCP deleted", color: "text-danger-text" },
      "mcp.attestation.created": { label: "Attestation created", color: "text-brand-text" },
      "mcp.attestation.verified": { label: "Attestation verified", color: "text-brand-text" },
      "mcp.capability.detected": { label: "Capability detected", color: "text-brand-text" },
      "mcp.capability.created": { label: "Capability added", color: "text-brand-text" },
      "mcp.connection.created": { label: "Agent connected", color: "text-brand-text" },
      "mcp.verified": { label: "MCP verified", color: "text-success-text" },
    };
    return actionMap[action] || { label: action, color: "text-ink-secondary" };
  };

  if (!isOpen || !mcp) return null;

  const getStatusColor = (status: string) => {
    switch (status) {
      case "active":
        return "border border-success-border bg-success-fill text-success-text";
      case "pending":
        return "border border-warning-border bg-warning-fill text-warning-text";
      case "inactive":
        return "border border-stroke bg-glass-inset-gray text-ink-body";
      default:
        return "border border-stroke bg-glass-inset-gray text-ink-body";
    }
  };

  // Three tiers: 80 and above, 60-79, below 60.
  const getTrustScoreColor = (score: number) => {
    if (score >= 80) return "text-success-text";
    if (score >= 60) return "text-warning-text";
    return "text-danger-text";
  };

  const getConfidenceScoreColor = (score: number) => {
    if (score >= 80) return "text-success-text";
    if (score >= 60) return "text-warning-text";
    return "text-danger-text";
  };

  const getVerificationMethodBadge = (method?: string) => {
    switch (method) {
      case "agent_attestation":
        return {
          text: "Agent attested",
          color: "border border-stroke bg-brand-soft text-brand-text",
        };
      case "api_key":
        return {
          text: "API key",
          color: "border border-stroke bg-brand-soft text-brand-text",
        };
      case "manual":
        return {
          text: "Manual",
          color: "border border-stroke bg-glass-inset-gray text-ink-body",
        };
      default:
        return {
          text: "Unknown",
          color: "border border-stroke bg-glass-inset-gray text-ink-body",
        };
    }
  };

  // The first 64 characters of the encoded key, grouped in pairs. Not a digest.
  const publicKeyPrefix = (publicKey: string): string => {
    if (!publicKey) return "N/A";
    const hash = publicKey.substring(0, 64);
    return (
      hash
        .match(/.{1,2}/g)
        ?.slice(0, 16)
        .join(":") || "N/A"
    );
  };

  const handleDownloadKey = () => {
    if (!mcp?.publicKey) return;

    const blob = new Blob([mcp.publicKey], { type: "text/plain" });
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
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(29,29,31,0.45)] backdrop-blur-sm"
      onClick={handleOverlayClick}
    >
      <div className="overlay-surface max-w-4xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-divider">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-logo rounded-inset flex items-center justify-center">
              <Shield className="h-6 w-6 text-ink-inverse" />
            </div>
            <div>
              <h2 className="text-xl font-semibold text-ink">
                {mcp?.name || "Unknown MCP"}
              </h2>
              <p className="text-sm text-ink-secondary">
                {mcp?.id || "Unknown ID"}
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
        <div className="p-6">
          <Tabs defaultValue="overview" className="w-full">
            <TabsList className="grid w-full grid-cols-2 mb-6">
              <TabsTrigger value="overview" className="flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Overview
              </TabsTrigger>
              <TabsTrigger value="activity" className="flex items-center gap-2">
                <Activity className="h-4 w-4" />
                Audit trail
              </TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-6">
              {/* Status and Metrics - Updated to match agent detail modal */}
              <div className="flex items-center gap-4">
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                Status
              </span>
              <span
                className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium capitalize ${getStatusColor(mcp?.status || "unknown")}`}
              >
                {mcp?.status || "unknown"}
              </span>
            </div>
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                {mcp.verificationMethod === "agent_attestation"
                  ? "Confidence score"
                  : "Trust score"}
              </span>
              <span
                className={`text-2xl font-bold ${
                  mcp.verificationMethod === "agent_attestation"
                    ? getConfidenceScoreColor(mcp.confidenceScore || 0)
                    : getTrustScoreColor(mcp.trustScore || 0)
                }`}
              >
                {mcp.verificationMethod === "agent_attestation"
                  ? Math.round(mcp.confidenceScore || 0)
                  : Math.round(mcp.trustScore || 0)}
                %
              </span>
            </div>
            <div>
              <span className="text-sm text-ink-secondary block mb-1">
                Capabilities
              </span>
              <span className="text-lg font-semibold text-ink">
                {mcp.capabilities?.length || 0}
              </span>
            </div>
          </div>

          {/* Agent Attestation Info (if using agent attestation) */}
          {mcp.verificationMethod === "agent_attestation" && (
            <div className="rounded-card border border-stroke bg-brand-soft p-4">
              <div className="flex items-start gap-3">
                <Shield className="h-5 w-5 text-brand-text mt-0.5" />
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-sm font-semibold text-ink">
                      Verified by attestations
                    </h4>
                    {attestations.length > 0 && (
                      <button
                        onClick={() => setShowAttestations(!showAttestations)}
                        className="flex items-center gap-1 text-xs font-semibold text-brand-text hover:underline"
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
                      <span className="text-ink font-medium">
                        {mcp.attestationCount || 0}
                      </span>
                      <span className="text-ink-secondary ml-1">
                        {mcp.attestationCount === 1 ? "attestation" : "attestations"}
                      </span>
                    </div>
                    {mcp.lastAttestedAt && (
                      <div>
                        <span className="text-ink font-medium">
                          Last attested:
                        </span>
                        <span className="text-ink-secondary ml-1">
                          {formatDateTime(mcp.lastAttestedAt)}
                        </span>
                      </div>
                    )}
                  </div>
                  <p className="text-xs text-ink-body mt-2">
                    This MCP server's identity is verified by {mcp.attestationCount || 0} attestation
                    {mcp.attestationCount !== 1 ? "s" : ""} from agents and users.
                  </p>

                  {/* Detailed Attestations List */}
                  {showAttestations && attestations.length > 0 && (
                    <div className="mt-4 space-y-2 border-t border-divider pt-3">
                      <h5 className="text-xs font-semibold text-ink mb-2">
                        Attestation history
                      </h5>
                      {attestations.map((att) => (
                        <div
                          key={att.id}
                          className="glass-inset p-3 text-xs space-y-2"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex items-center gap-2">
                              {att.attesterType === "agent" ? (
                                <Bot className="h-4 w-4 text-brand-text flex-shrink-0" />
                              ) : (
                                <User className="h-4 w-4 text-ink-secondary flex-shrink-0" />
                              )}
                              <div>
                                <p className="font-medium text-ink">
                                  {att.attestedBy}
                                  {att.attesterType === "agent" && att.agentOwnerName && (
                                    <span className="ml-1 font-normal text-xs text-ink-secondary">
                                      (owned by {att.agentOwnerName})
                                    </span>
                                  )}
                                </p>
                                <p className="text-ink-secondary">
                                  {att.attesterType === "agent" ? "Agent" : "User"} • {att.attestationType === "sdk" ? "SDK" : "Manual"}
                                </p>
                              </div>
                            </div>
                            <div className="text-right flex-shrink-0">
                              {att.isValid ? (
                                <span className="inline-flex items-center gap-1 rounded-pill border border-success-border bg-success-fill px-2 py-0.5 text-xs text-success-text">
                                  <CheckCircle className="h-3 w-3" />
                                  Valid
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-1 rounded-pill border border-danger-border bg-danger-fill px-2 py-0.5 text-xs text-danger-text">
                                  Expired
                                </span>
                              )}
                            </div>
                          </div>

                          <div className="grid grid-cols-2 gap-2 text-xs">
                            <div>
                              <span className="text-ink-body">Verified:</span>
                              <span className="ml-1 text-ink-secondary">
                                {formatDateTime(att.verifiedAt)}
                              </span>
                            </div>
                            {att.attestationType === "sdk" && att.sdkVersion && (
                              <div>
                                <span className="text-ink-body">SDK:</span>
                                <span className="ml-1 text-ink-secondary">
                                  {att.sdkVersion}
                                </span>
                              </div>
                            )}
                          </div>

                          <div className="flex flex-wrap gap-2">
                            {att.signatureVerified && (
                              <span className="inline-flex items-center gap-1 rounded-pill border border-stroke bg-brand-soft px-2 py-0.5 text-xs text-brand-text">
                                <Shield className="h-3 w-3" />
                                Signature verified
                              </span>
                            )}
                            {att.connectionSuccessful && (
                              <span className="inline-flex items-center gap-1 rounded-pill border border-success-border bg-success-fill px-2 py-0.5 text-xs text-success-text">
                                <CheckCircle className="h-3 w-3" />
                                Connection OK
                              </span>
                            )}
                            {att.healthCheckPassed && (
                              <span className="inline-flex items-center gap-1 rounded-pill border border-success-border bg-success-fill px-2 py-0.5 text-xs text-success-text">
                                <CheckCircle className="h-3 w-3" />
                                Health check passed
                              </span>
                            )}
                          </div>

                          {att.capabilitiesConfirmed && att.capabilitiesConfirmed.length > 0 && (
                            <div>
                              <p className="text-ink-body mb-1">
                                Capabilities verified ({att.capabilitiesConfirmed.length}):
                              </p>
                              <div className="flex flex-wrap gap-1">
                                {att.capabilitiesConfirmed.map((cap, idx) => (
                                  <span
                                    key={idx}
                                    className="rounded-pill bg-glass-inset-gray px-2 py-0.5 text-xs text-ink-body"
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
                    <div className="mt-4 text-center text-xs text-ink-secondary">
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
              <h3 className="text-sm font-medium text-ink-body mb-2">
                Description
              </h3>
              <p className="text-sm text-ink-secondary">
                {mcp.description}
              </p>
            </div>
          )}

          {/* Capabilities */}
          <div>
            <h3 className="text-sm font-medium text-ink-body mb-3 flex items-center gap-2">
              <Key className="h-4 w-4" />
              Capabilities
            </h3>
            {mcp.capabilities && mcp.capabilities.length > 0 ? (
              <div className="space-y-3">
                {mcp.capabilities.map((capability) => {
                  const typeColors = {
                    tool: "bg-brand-soft border-stroke text-ink",
                    resource:
                      "bg-glass-inset-gray border-stroke text-ink",
                    prompt:
                      "bg-success-fill border-success-border text-ink",
                  };

                  return (
                    <div
                      key={capability.id}
                      className={`p-3 border rounded-inset ${typeColors[capability.type]}`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-1">
                            <CheckCircle className="h-4 w-4 flex-shrink-0" />
                            <p className="text-sm font-semibold">
                              {capability.name}
                            </p>
                            <span className="px-2 py-0.5 text-xs font-medium rounded-pill bg-glass-inset-gray">
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
              <div className="text-sm text-ink-tertiary italic">
                No capabilities registered
              </div>
            )}
          </div>

          {/* Talks To (Agents) */}
          <div>
            <h3 className="text-sm font-medium text-ink-body mb-3 flex items-center gap-2">
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
              Talks to (agents)
            </h3>
            {mcp.talksTo && mcp.talksTo.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {mcp.talksTo.map((agent, index) => (
                  <div
                    key={index}
                    className="px-3 py-2 bg-success-fill border border-success-border rounded-inset text-sm font-medium text-success-text"
                  >
                    {agent}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-ink-tertiary italic">
                No agents configured to use this MCP server
              </div>
            )}
          </div>

          {/* Details Grid - Updated to match agent detail modal */}
          <div className="grid grid-cols-2 gap-6">
            <div className="col-span-2">
              <h3 className="text-sm font-medium text-ink-body mb-2">
                URL
              </h3>
              <p className="text-sm text-ink font-mono break-all">
                {mcp.url}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink-body mb-2">
                Server ID
              </h3>
              <p className="text-sm text-ink font-mono">
                {mcp.id}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink-body mb-2">
                Organization ID
              </h3>
              <p className="text-sm text-ink font-mono">
                {/* MCP servers don't have organization_id, so we'll show placeholder */}
                N/A
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink-body mb-2 flex items-center gap-2">
                <Calendar className="h-4 w-4" />
                Created
              </h3>
              <p className="text-sm text-ink">
                {formatDateTime(mcp.createdAt)}
              </p>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink-body mb-2 flex items-center gap-2">
                <User className="h-4 w-4" />
                Created by
              </h3>
              <div className="text-sm text-ink">
                {mcp.createdByName || mcp.createdByEmail ? (
                  <div className="flex flex-col gap-1">
                    {mcp.createdByName && (
                      <span className="font-medium">{mcp.createdByName}</span>
                    )}
                    {mcp.createdByEmail && (
                      <span className="text-ink-secondary">{mcp.createdByEmail}</span>
                    )}
                    {mcp.createdBySdkTokenId && (
                      <Button
                        variant="link"
                        size="sm"
                        className="p-0 h-auto text-xs justify-start"
                        onClick={() => {
                          onClose();
                          router.push(`/dashboard/sdk-tokens?highlight=${mcp.createdBySdkTokenId}`);
                        }}
                      >
                        <KeyRound className="h-3 w-3 mr-1" />
                        View SDK token
                      </Button>
                    )}
                    {mcp.createdByApiKeyId && (
                      <Button
                        variant="link"
                        size="sm"
                        className="p-0 h-auto text-xs justify-start text-warning-text"
                        onClick={() => {
                          onClose();
                          router.push(`/dashboard/api-keys?highlight=${mcp.createdByApiKeyId}`);
                        }}
                      >
                        <KeyRound className="h-3 w-3 mr-1" />
                        View API key
                      </Button>
                    )}
                  </div>
                ) : (
                  <span className="text-ink-secondary">System</span>
                )}
              </div>
            </div>

            <div>
              <h3 className="text-sm font-medium text-ink-body mb-2 flex items-center gap-2">
                <Clock className="h-4 w-4" />
                Last updated
              </h3>
              <p className="text-sm text-ink">
                {mcp.lastVerifiedAt
                  ? formatDateTime(mcp.lastVerifiedAt)
                  : "Never"}
              </p>
            </div>
          </div>

          {/* Cryptographic Identity Section */}
          {mcp.publicKey && (
            <div className="border-t border-divider pt-6">
              <div className="flex items-center gap-2 mb-4">
                <Key className="h-5 w-5 text-brand-text" />
                <h3 className="text-lg font-semibold text-ink">
                  Cryptographic identity
                </h3>
              </div>

              <div className="rounded-card bg-glass-inset-gray p-4 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <h4 className="text-sm font-medium text-ink-body mb-2">
                      Public key (prefix)
                    </h4>
                    <p className="text-xs text-ink font-mono bg-glass-inset p-2 rounded-inset-sm border border-stroke break-all">
                      {publicKeyPrefix(mcp.publicKey)}
                    </p>
                  </div>

                  <div>
                    <h4 className="text-sm font-medium text-ink-body mb-2">
                      Key type
                    </h4>
                    <p className="text-sm text-ink font-medium">
                      {mcp.keyType || "RSA-2048"}
                    </p>
                  </div>
                </div>

                <div>
                  <h4 className="text-sm font-medium text-ink-body mb-2">
                    Public key
                  </h4>
                  <div className="relative">
                    <pre className="text-xs text-ink font-mono bg-glass-inset p-3 rounded-inset-sm border border-stroke overflow-x-auto max-h-32">
                      {mcp.publicKey}
                    </pre>
                    <button
                      onClick={handleDownloadKey}
                      className="absolute top-2 right-2 p-1.5 bg-glass-inset-gray hover:bg-glass-inset rounded-inset-sm transition-colors"
                      title="Download public key"
                    >
                      <Download className="h-4 w-4 text-ink-body" />
                    </button>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2 border-t border-divider">
                  <div className="flex items-center gap-2">
                    <CheckCircle className="h-4 w-4 text-success-text" />
                    <span className="text-sm text-success-text font-medium">
                      Cryptographic identity verified on registration
                    </span>
                  </div>
                  <div className="text-xs text-ink-tertiary">
                    Ed25519 signature
                  </div>
                </div>
              </div>
            </div>
          )}
            </TabsContent>

            <TabsContent value="activity" className="space-y-4">
              {/* Audit Trail for this MCP */}
              <div>
                <h3 className="text-sm font-medium text-ink-body mb-3">
                  Audit trail
                </h3>

                {loadingAuditLogs ? (
                  <div className="text-center py-8 text-ink-secondary">
                    Loading audit logs...
                  </div>
                ) : auditLogs.length === 0 ? (
                  <div className="text-center py-8 text-ink-secondary">
                    <p>No audit events recorded yet</p>
                  </div>
                ) : (
                  <div className="space-y-2 max-h-96 overflow-y-auto">
                    {auditLogs.map((log) => {
                      const actionInfo = getActionDisplayInfo(log.action);
                      return (
                        <div
                          key={log.id}
                          className="flex items-start gap-3 p-3 glass-inset"
                        >
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center justify-between gap-2">
                              <p className={`text-sm font-medium ${actionInfo.color}`}>
                                {actionInfo.label}
                              </p>
                              <p className="text-xs text-ink-tertiary flex-shrink-0">
                                {formatDateTime(log.timestamp)}
                              </p>
                            </div>
                            <div className="mt-1 text-sm">
                              {log.agentId ? (
                                <span className="font-medium text-brand-text">
                                  Agent: {log.agentName || log.metadata?.agentName || log.agentId.slice(0, 8)}
                                </span>
                              ) : log.userId ? (
                                <span className="font-medium text-ink">
                                  User: {log.userName || log.metadata?.userName || log.userId.slice(0, 8)}
                                </span>
                              ) : (
                                <span className="text-ink-tertiary">System</span>
                              )}
                            </div>
                            {log.metadata && Object.keys(log.metadata).length > 0 && (
                              <div className="mt-2 text-xs text-ink-secondary bg-glass-inset rounded-inset-sm p-2">
                                {log.metadata.auth_method && (
                                  <p>Auth: {log.metadata.auth_method === "ed25519" ? "Ed25519 signature" : log.metadata.auth_method}</p>
                                )}
                                {(log.metadata.capabilities_found || log.metadata.capabilitiesFound) && (
                                  <p>Capabilities: {(log.metadata.capabilities_found || log.metadata.capabilitiesFound).length} detected</p>
                                )}
                                {log.metadata.confidence_score !== undefined && (
                                  <p>Confidence: {Math.round(log.metadata.confidence_score)}%</p>
                                )}
                                {log.metadata.attestation_count !== undefined && (
                                  <p>Total attestations: {log.metadata.attestation_count}</p>
                                )}
                                {log.ipAddress && log.ipAddress !== "" && (
                                  <p>IP: {log.ipAddress}</p>
                                )}
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </TabsContent>
          </Tabs>
        </div>

        {/* Footer - Updated to match agent detail modal */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-divider">
          {onDelete && (
            <button
              onClick={() => onDelete(mcp)}
              className="px-4 py-2 text-sm font-medium text-danger-text hover:bg-danger-fill rounded-pill transition-colors flex items-center gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(mcp)}
              className="px-4 py-2 text-sm font-medium text-ink-inverse bg-brand shadow-glow hover:bg-brand-hover rounded-pill transition-colors flex items-center gap-2"
            >
              <Edit className="h-4 w-4" />
              Edit server
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
