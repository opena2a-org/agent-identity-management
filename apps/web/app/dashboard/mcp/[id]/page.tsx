"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Server,
  Shield,
  AlertTriangle,
  ExternalLink,
  Globe,
  Edit,
  Trash2,
  CheckCircle,
  Loader2,
  Tag,
  Activity,
  Bot,
  User,
  XCircle,
  Ban,
  Code2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { api, AttestationWithAgentDetails } from "@/lib/api";
import { RegisterMCPModal } from "@/components/modals/register-mcp-modal";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { MCPServerDetailSkeleton } from "@/components/ui/content-loaders";
import { AuthGuard } from "@/components/auth-guard";
import { MCPTagsTab } from "@/components/mcp/tags-tab";
import { ConsensusProgress } from "@/components/mcp/consensus-progress";
import { ManualAttestationForm } from "@/components/mcp/manual-attestation-form";
import { MCPSetupGuide } from "@/components/mcp/mcp-setup-guide";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";

interface MCPServer {
  id: string;
  name: string;
  url: string;
  description?: string;
  status: "active" | "inactive" | "verified" | "pending";
  publicKey?: string;
  keyType?: string;
  lastVerifiedAt?: string;
  createdAt: string;
  updatedAt?: string;
  trustScore?: number;
  capabilityCount?: number;
  organizationId: string;
  capabilities?: string[]; // Array of capability type strings like ["tools", "prompts", "resources"]

  // ✅ NEW: Agent Attestation fields
  verificationMethod?: string; // "agent_attestation", "api_key", or "manual"
  attestationCount?: number;
  confidenceScore?: number; // 0-100
  lastAttestedAt?: string;
}

// Detailed capability information from mcp_server_capabilities table
interface Capability {
  id: string;
  mcpServerId: string;
  name: string;
  type: "tool" | "resource" | "prompt";
  description: string;
  schema: any;
  detectedAt: string;
  lastVerifiedAt?: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

interface Agent {
  id: string;
  name: string;
  displayName: string;
  agentType: string;
  status?: string;
}

export default function MCPServerDetailsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const router = useRouter();
  const [serverId, setServerId] = useState<string | null>(null);
  const [server, setServer] = useState<MCPServer | null>(null);
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [connectedAgents, setConnectedAgents] = useState<Agent[]>([]);
  const [agentsError, setAgentsError] = useState<string | null>(null);
  const [attestations, setAttestations] = useState<AttestationWithAgentDetails[]>([]);
  const [auditLogs, setAuditLogs] = useState<any[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [userRole, setUserRole] = useState<
    "admin" | "manager" | "member" | "viewer"
  >("viewer");
  const [verifying, setVerifying] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Attestation revocation state
  const [showRevokeDialog, setShowRevokeDialog] = useState(false);
  const [selectedAttestation, setSelectedAttestation] = useState<AttestationWithAgentDetails | null>(null);
  const [revokeReason, setRevokeReason] = useState<string>("");
  const [revoking, setRevoking] = useState(false);

  // Extract server ID from params Promise
  useEffect(() => {
    params.then(({ id }) => setServerId(id));
  }, [params]);

  // Extract user role from JWT token
  useEffect(() => {
    const token = api.getToken?.();
    if (!token) return;
    try {
      const payload = JSON.parse(atob(token.split(".")[1]));
      const role = (payload.role as any) || "viewer";
      setUserRole(role);
    } catch {}
  }, []);

  // Fetch server data
  useEffect(() => {
    if (!serverId) return;

    async function fetchData() {
      setIsLoading(true);
      setError(null);

      try {
        // Fetch MCP server details
        const serverData = await api.getMCPServer(serverId!);
        setServer(serverData);

        // Fetch detailed capabilities from the dedicated endpoint
        try {
          const capabilitiesData = await api.getMCPServerCapabilities(serverId!);
          setCapabilities(capabilitiesData.capabilities || []);
        } catch (err) {
          console.error("Failed to fetch capabilities:", err);
          setCapabilities([]);
        }

        // Fetch connected agents
        try {
          setAgentsError(null);
          const agentsData = await api.getMCPServerAgents(serverId!);
          let agents = agentsData.agents || [];
          // Robust client fallback: also match by talks_to entries containing id/name/url
          if (
            (!agents || agents.length === 0) &&
            (serverData?.name || serverData?.url)
          ) {
            try {
              const allAgentsResp = await api.listAgents();
              const allAgents = allAgentsResp.agents || [];
              const candidates = new Set<string>([
                String(serverId),
                String(serverData?.name || ""),
                String(serverData?.url || ""),
              ]);
              const lc = new Set<string>(
                Array.from(candidates).map((s) => s.toLowerCase())
              );
              const matches = (talks: any): boolean => {
                if (!Array.isArray(talks)) return false;
                return talks.some((entry) => {
                  if (typeof entry === "string") {
                    const v = entry.toLowerCase();
                    return (
                      lc.has(v) || Array.from(lc).some((c) => v.includes(c))
                    );
                  }
                  if (entry && typeof entry === "object") {
                    const idStr = (entry.id || entry.server_id || "")
                      .toString()
                      .toLowerCase();
                    const nameStr = (entry.name || entry.server_name || "")
                      .toString()
                      .toLowerCase();
                    const urlStr = (entry.url || entry.endpoint || "")
                      .toString()
                      .toLowerCase();
                    if (idStr && lc.has(idStr)) return true;
                    if (
                      nameStr &&
                      (lc.has(nameStr) ||
                        Array.from(lc).some((c) => nameStr.includes(c)))
                    )
                      return true;
                    if (
                      urlStr &&
                      Array.from(lc).some((c) => urlStr.includes(c))
                    )
                      return true;
                  }
                  return false;
                });
              };
              agents = allAgents.filter((a: any) => matches(a.talks_to));
            } catch (e) {
              console.error("Fallback agent listing failed:", e);
            }
          }
          setConnectedAgents(agents);
        } catch (err: any) {
          console.error("Failed to fetch connected agents:", err);
          setAgentsError(err?.message || "Failed to load connected agents");
        }

        // Fetch attestations
        try {
          const data = await api.getMCPAttestations(serverId!);
          setAttestations(data.attestations || []);
        } catch (err) {
          console.error("Failed to fetch attestations:", err);
        }

        // Fetch audit logs
        try {
          const auditData = await api.getMCPServerAuditLogs(serverId!, 50, 0);
          setAuditLogs(auditData.logs || []);
        } catch (err) {
          console.error("Failed to fetch audit logs:", err);
        }
      } catch (err: any) {
        console.error("Failed to fetch MCP server data:", err);
        setError(err.message || "Failed to load MCP server details");
      } finally {
        setIsLoading(false);
      }
    }

    fetchData();
  }, [serverId, refreshKey]);

  const handleRefresh = () => {
    setRefreshKey((prev) => prev + 1);
  };

  const canEdit = ["admin", "manager", "member"].includes(userRole);
  const canManage = ["admin", "manager"].includes(userRole);

  const handleVerify = async () => {
    if (!serverId) return;
    setVerifying(true);
    try {
      await api.verifyMCPServer(serverId);
      handleRefresh();
    } catch (e: any) {
      alert(e?.message || "Verification failed");
    } finally {
      setVerifying(false);
    }
  };

  const handleDelete = async () => {
    if (!serverId) return;
    try {
      await api.deleteMCPServer(serverId);
      router.push("/dashboard/mcp");
    } catch (e: any) {
      alert(e?.message || "Delete failed");
    } finally {
      setShowDeleteConfirm(false);
    }
  };

  const handleRevokeAttestation = async () => {
    if (!selectedAttestation || !revokeReason) return;
    setRevoking(true);
    try {
      await api.revokeAttestation(selectedAttestation.id, revokeReason);
      toast.success("Attestation revoked successfully");
      handleRefresh();
    } catch (e: any) {
      toast.error(e?.message || "Failed to revoke attestation");
    } finally {
      setRevoking(false);
      setShowRevokeDialog(false);
      setSelectedAttestation(null);
      setRevokeReason("");
    }
  };

  const openRevokeDialog = (attestation: AttestationWithAgentDetails) => {
    setSelectedAttestation(attestation);
    setRevokeReason("");
    setShowRevokeDialog(true);
  };

  // Get trust score color
  const getTrustColor = (score: number): string => {
    if (score >= 80) return "text-green-600 bg-green-500/10";
    if (score >= 60) return "text-yellow-600 bg-yellow-500/10";
    return "text-red-600 bg-red-500/10";
  };

  // Get confidence score color (for agent attestation)
  const getConfidenceColor = (score: number): string => {
    if (score >= 80) return "text-green-600 bg-green-500/10";
    if (score >= 60) return "text-yellow-600 bg-yellow-500/10";
    if (score >= 40) return "text-orange-600 bg-orange-500/10";
    return "text-red-600 bg-red-500/10";
  };

  // Get status color
  const getStatusColor = (status: string): string => {
    switch (status) {
      case "active":
      case "verified":
        return "bg-green-500/10 text-green-600";
      case "pending":
        return "bg-yellow-500/10 text-yellow-600";
      case "inactive":
        return "bg-gray-500/10 text-gray-600";
      default:
        return "bg-gray-500/10 text-gray-600";
    }
  };

  // Check if server is verified (strict)
  const isVerified = server?.status === "verified";

  // Loading state
  if (isLoading) {
    return <MCPServerDetailSkeleton />;
  }

  // Error state
  if (error || !server) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Card className="max-w-md">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-5 w-5" />
              Error Loading MCP Server
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-muted-foreground mb-4">
              {error ||
                "MCP server not found or you do not have permission to view it."}
            </p>
            <Button
              variant="outline"
              onClick={() => router.push("/dashboard/mcp")}
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to MCP Servers
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => router.push("/dashboard/mcp")}
            className="mb-4"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to MCP Servers
          </Button>

          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-4">
              <div className="flex h-16 w-16 items-center justify-center rounded-xl bg-purple-500/10">
                <Server className="h-8 w-8 text-purple-600" />
              </div>
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <h1 className="text-3xl font-bold">{server.name}</h1>
                  {isVerified && (
                    <span title="Verified">
                      <Shield className="h-6 w-6 text-green-600" />
                    </span>
                  )}
                </div>
                {server.description && (
                  <p className="text-muted-foreground mb-2">
                    {server.description}
                  </p>
                )}
                <div className="flex items-center gap-2 flex-wrap">
                  <Badge variant="outline" className="flex items-center gap-1">
                    <Globe className="h-3 w-3" />
                    <a
                      href={server.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:underline"
                    >
                      {server.url}
                    </a>
                  </Badge>
                  <Badge className={getStatusColor(server.status)}>
                    {server.status.charAt(0).toUpperCase() +
                      server.status.slice(1)}
                  </Badge>
                  <Badge
                    className={
                      server.verificationMethod === "agent_attestation"
                        ? getConfidenceColor(server.confidenceScore ?? 0)
                        : getTrustColor(server.trustScore ?? 0)
                    }
                  >
                    {server.verificationMethod === "agent_attestation"
                      ? `Confidence: ${(server.confidenceScore ?? 0).toFixed(1)}%`
                      : `Trust: ${(server.trustScore ?? 0).toFixed(1)}%`}
                  </Badge>
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {canManage && !isVerified && (
                <Button
                  onClick={handleVerify}
                  disabled={verifying}
                  className="bg-green-600 hover:bg-green-700"
                >
                  {verifying ? (
                    <>
                      <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                      Verifying...
                    </>
                  ) : (
                    <>
                      <CheckCircle className="h-4 w-4 mr-1" /> Verify
                    </>
                  )}
                </Button>
              )}
              {canEdit && (
                <Button
                  variant="outline"
                  onClick={() => setShowEditModal(true)}
                >
                  <Edit className="h-4 w-4 mr-1" /> Edit
                </Button>
              )}
              {canManage && (
                <Button
                  variant="destructive"
                  onClick={() => setShowDeleteConfirm(true)}
                >
                  <Trash2 className="h-4 w-4 mr-1" /> Delete
                </Button>
              )}
            </div>
          </div>
        </div>

        <Separator />

        {/* HERO: Confidence Score Card */}
        {server.verificationMethod === "agent_attestation" ? (
          <Card className="border-blue-200 dark:border-blue-700">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-500/10">
                    <Shield className="h-6 w-6 text-blue-600 dark:text-blue-400" />
                  </div>
                  <div>
                    <CardTitle className="text-lg font-semibold">
                      Confidence Score
                    </CardTitle>
                    <CardDescription className="text-sm">
                      Verified by {server.attestationCount || 0} agent attestation
                      {server.attestationCount !== 1 ? "s" : ""}
                    </CardDescription>
                  </div>
                </div>
                <div className="text-right">
                  <div
                    className={`text-4xl font-bold ${
                      getConfidenceColor(server.confidenceScore ?? 0).split(" ")[0]
                    }`}
                  >
                    {(server.confidenceScore ?? 0).toFixed(1)}%
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {(server.attestationCount ?? 0) === 0
                      ? "No attestations yet"
                      : (server.confidenceScore ?? 0) >= 80
                        ? "High confidence"
                        : (server.confidenceScore ?? 0) >= 60
                          ? "Medium confidence"
                          : (server.confidenceScore ?? 0) >= 40
                            ? "Low confidence"
                            : "Needs more attestations"}
                  </p>
                </div>
              </div>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="flex items-center gap-4 text-sm text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <CheckCircle className="h-4 w-4" />
                  <span>
                    {server.attestationCount || 0} attestation
                    {server.attestationCount !== 1 ? "s" : ""}
                  </span>
                </div>
                {server.lastAttestedAt && (
                  <>
                    <span>•</span>
                    <span>
                      Last verified:{" "}
                      {new Date(server.lastAttestedAt).toLocaleDateString()}
                    </span>
                  </>
                )}
                {canManage && !isVerified && (
                  <>
                    <span className="ml-auto" />
                    <Button
                      size="sm"
                      onClick={handleVerify}
                      disabled={verifying}
                      variant="outline"
                    >
                      {verifying ? (
                        <>
                          <Loader2 className="h-3 w-3 mr-1.5 animate-spin" />
                          Verifying...
                        </>
                      ) : (
                        <>
                          <CheckCircle className="h-3 w-3 mr-1.5" />
                          Request Verification
                        </>
                      )}
                    </Button>
                  </>
                )}
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-lg font-semibold">
                    Trust Score
                  </CardTitle>
                  <CardDescription className="text-sm">
                    Traditional verification method
                  </CardDescription>
                </div>
                <div className="text-right">
                  <div
                    className={`text-4xl font-bold ${
                      getTrustColor(server.trustScore ?? 0).split(" ")[0]
                    }`}
                  >
                    {(server.trustScore ?? 0).toFixed(1)}%
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {(server.trustScore ?? 0) >= 80
                      ? "High trust"
                      : (server.trustScore ?? 0) >= 60
                        ? "Medium trust"
                        : "Low trust"}
                  </p>
                </div>
              </div>
            </CardHeader>
          </Card>
        )}

        {/* Consensus Progress - Multi-Agent Verification */}
        {server.verificationMethod === "agent_attestation" && (
          <ConsensusProgress
            mcpServerId={server.id}
            currentAgents={server.attestationCount || 0}
            requiredAgents={3}
            currentOwners={0}
            requiredOwners={2}
            confidenceScore={server.confidenceScore || 0}
            requiredScore={60}
            status={server.status}
            isVerified={isVerified}
          />
        )}

        {/* Secondary Metrics */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Connected Agents
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{connectedAgents.length}</div>
              <p className="text-xs text-muted-foreground mt-1">
                Agent{connectedAgents.length !== 1 ? "s" : ""} using this server
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Capabilities
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{capabilities.length}</div>
              <p className="text-xs text-muted-foreground mt-1">
                Tool{capabilities.length !== 1 ? "s" : ""} and resource
                {capabilities.length !== 1 ? "s" : ""}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Attestations
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{attestations.length}</div>
              <p className="text-xs text-muted-foreground mt-1">
                Cryptographic verification{attestations.length !== 1 ? "s" : ""}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Status
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Badge className={getStatusColor(server.status)}>
                {server.status.charAt(0).toUpperCase() + server.status.slice(1)}
              </Badge>
              <p className="text-xs text-muted-foreground mt-2">
                {server.lastVerifiedAt
                  ? `Verified ${new Date(server.lastVerifiedAt).toLocaleDateString()}`
                  : "Not yet verified"}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Tabs */}
        <Tabs defaultValue="details" className="space-y-4">
          <TabsList>
            <TabsTrigger value="details">Details</TabsTrigger>
            <TabsTrigger value="capabilities">
              <ExternalLink className="h-4 w-4 mr-2" />
              Capabilities
            </TabsTrigger>
            <TabsTrigger value="agents">Connected Agents</TabsTrigger>
            <TabsTrigger value="activity">
              <Activity className="h-4 w-4 mr-2" />
              Attestations
            </TabsTrigger>
            <TabsTrigger value="tags">
              <Tag className="h-4 w-4 mr-2" />
              Tags
            </TabsTrigger>
            <TabsTrigger value="audit">
              <Activity className="h-4 w-4 mr-2" />
              Audit Trail
            </TabsTrigger>
            <TabsTrigger value="integration">
              <Code2 className="h-4 w-4 mr-2" />
              Integration
            </TabsTrigger>
          </TabsList>

          <TabsContent value="capabilities" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>MCP Server Capabilities</CardTitle>
                <CardDescription>
                  Capability types supported by this MCP server
                </CardDescription>
              </CardHeader>
              <CardContent>
                {capabilities.length === 0 ? (
                  <div className="text-center py-8 space-y-4">
                    <p className="text-muted-foreground">
                      No capabilities detected yet
                    </p>
                    <div className="max-w-md mx-auto space-y-3">
                      <div className="bg-muted/50 rounded-md p-3 text-left">
                        <p className="font-medium text-sm mb-1">Option 1: Auto-detect via SDK</p>
                        <p className="text-xs text-muted-foreground">
                          When an agent attests to this MCP server, capabilities are automatically detected.
                        </p>
                      </div>
                      <div className="bg-muted/50 rounded-md p-3 text-left">
                        <p className="font-medium text-sm mb-1">Option 2: Manual verification</p>
                        <p className="text-xs text-muted-foreground">
                          Click "Verify" above to query the MCP server directly.
                        </p>
                      </div>
                    </div>
                    <p className="text-xs text-muted-foreground mt-2">
                      See the <a href="https://opena2a.org/docs/integration/python" className="underline hover:text-foreground">SDK documentation</a> for more details.
                    </p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {/* Group by type */}
                    {["tool", "resource", "prompt"].map((type) => {
                      const typeCaps = capabilities.filter((c) => c.type === type);
                      if (typeCaps.length === 0) return null;

                      return (
                        <div key={type} className="space-y-2">
                          <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 capitalize flex items-center gap-2">
                            <CheckCircle className="h-4 w-4" />
                            {type}s ({typeCaps.length})
                          </h4>
                          <div className="grid gap-2">
                            {typeCaps.map((capability) => (
                              <div
                                key={capability.id}
                                className="p-3 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md"
                              >
                                <div className="flex items-start justify-between gap-2">
                                  <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                                      {capability.name}
                                    </p>
                                    {capability.description && (
                                      <p className="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
                                        {capability.description}
                                      </p>
                                    )}
                                  </div>
                                  <Badge
                                    variant="outline"
                                    className="flex-shrink-0 text-xs capitalize"
                                  >
                                    {capability.type}
                                  </Badge>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="agents" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Connected Agents</CardTitle>
                <CardDescription>
                  Agents that can communicate with this MCP server
                </CardDescription>
              </CardHeader>
              <CardContent>
                {agentsError ? (
                  <div className="text-center py-8">
                    <AlertTriangle className="h-8 w-8 text-destructive mx-auto mb-2" />
                    <p className="text-destructive font-medium">
                      Failed to load connected agents
                    </p>
                    <p className="text-sm text-muted-foreground mt-1">
                      {agentsError}
                    </p>
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-4"
                      onClick={handleRefresh}
                    >
                      Try Again
                    </Button>
                  </div>
                ) : connectedAgents.length === 0 ? (
                  <div className="text-center py-8">
                    <Bot className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
                    <p className="text-muted-foreground">
                      No agents connected yet
                    </p>
                    <p className="text-sm text-muted-foreground mt-1">
                      Agents that attest to this MCP server will appear here
                    </p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {connectedAgents.map((agent) => (
                      <div
                        key={agent.id}
                        className="flex items-center gap-3 p-3 border rounded-lg hover:bg-accent/50 transition-colors cursor-pointer"
                        onClick={() =>
                          router.push(`/dashboard/agents/${agent.id}`)
                        }
                      >
                        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-500/10">
                          <Bot className="h-5 w-5 text-blue-600" />
                        </div>
                        <div className="flex-1">
                          <h4 className="font-medium">
                            {agent.displayName || agent.name}
                          </h4>
                          <p className="text-sm text-muted-foreground">
                            {agent.agentType}
                          </p>
                        </div>
                        {agent.status && (
                          <Badge
                            variant={
                              agent.status === "active"
                                ? "default"
                                : agent.status === "inactive"
                                  ? "secondary"
                                  : "outline"
                            }
                          >
                            {agent.status}
                          </Badge>
                        )}
                        <ExternalLink className="h-4 w-4 text-muted-foreground" />
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="activity" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Attestations</CardTitle>
                <CardDescription>
                  All attestations for this MCP server from agents and users
                </CardDescription>
              </CardHeader>
              <CardContent>
                {attestations.length === 0 ? (
                  <div className="py-6">
                    {/* Header */}
                    <div className="text-center mb-6">
                      <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-blue-100 dark:bg-blue-900/30 mb-3">
                        <Shield className="h-6 w-6 text-blue-600 dark:text-blue-400" />
                      </div>
                      <h3 className="text-lg font-semibold text-foreground">
                        No attestations yet
                      </h3>
                      <p className="text-sm text-muted-foreground mt-1 max-w-sm mx-auto">
                        Build trust by having agents verify this MCP server
                      </p>
                    </div>

                    {/* Options Grid */}
                    <div className="grid md:grid-cols-2 gap-4 max-w-2xl mx-auto">
                      {/* Option 1: SDK */}
                      <div className="relative group">
                        <div className="absolute -inset-0.5 bg-gradient-to-r from-blue-500 to-purple-500 rounded-lg opacity-20 group-hover:opacity-30 transition" />
                        <div className="relative bg-card border rounded-lg p-4 h-full">
                          <div className="flex items-center gap-2 mb-2">
                            <div className="flex items-center justify-center w-8 h-8 rounded-md bg-blue-100 dark:bg-blue-900/30">
                              <Bot className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                            </div>
                            <div>
                              <span className="text-xs font-medium text-blue-600 dark:text-blue-400">Recommended</span>
                              <h4 className="font-semibold text-sm">Agent Attestation</h4>
                            </div>
                          </div>
                          <p className="text-xs text-muted-foreground mb-3">
                            Agents automatically create cryptographically-signed attestations via the SDK.
                          </p>
                          <div className="bg-blue-50 dark:bg-blue-950/50 border border-blue-200 dark:border-blue-800 rounded-md p-2 overflow-x-auto">
                            <code className="text-xs text-blue-700 dark:text-blue-300 whitespace-nowrap font-mono">
                              agent.attest_mcp("{server.name}")
                            </code>
                          </div>
                        </div>
                      </div>

                      {/* Option 2: Manual */}
                      <div className="relative group">
                        <div className="absolute -inset-0.5 bg-gradient-to-r from-gray-400 to-gray-500 rounded-lg opacity-10 group-hover:opacity-20 transition" />
                        <div className="relative bg-card border rounded-lg p-4 h-full">
                          <div className="flex items-center gap-2 mb-2">
                            <div className="flex items-center justify-center w-8 h-8 rounded-md bg-gray-100 dark:bg-gray-800">
                              <User className="h-4 w-4 text-gray-600 dark:text-gray-400" />
                            </div>
                            <div>
                              <span className="text-xs font-medium text-muted-foreground">Alternative</span>
                              <h4 className="font-semibold text-sm">Manual Attestation</h4>
                            </div>
                          </div>
                          <p className="text-xs text-muted-foreground mb-3">
                            {canManage
                              ? "Verify this server manually using the attestation form below."
                              : "Contact an admin or manager to manually verify this server."}
                          </p>
                          {canManage ? (
                            <div className="flex items-center text-xs text-muted-foreground">
                              <CheckCircle className="h-3.5 w-3.5 mr-1.5 text-green-500" />
                              <span>You have permission to attest</span>
                            </div>
                          ) : (
                            <div className="flex items-center text-xs text-muted-foreground">
                              <AlertTriangle className="h-3.5 w-3.5 mr-1.5 text-amber-500" />
                              <span>Requires admin/manager role</span>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Footer link */}
                    <p className="text-xs text-center text-muted-foreground mt-5">
                      Learn more in the{" "}
                      <a
                        href="https://opena2a.org/docs/integration/python"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        SDK documentation
                      </a>
                    </p>
                  </div>
                ) : (
                  <div className="space-y-4">
                    {attestations.map((attestation) => (
                      <div
                        key={attestation.id}
                        className="border rounded-lg p-4 space-y-3"
                      >
                        {/* Header */}
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex items-center gap-3">
                            <Bot className="h-5 w-5 text-blue-600" />
                            <div>
                              <p className="font-medium">
                                {attestation.agentName}
                              </p>
                              <p className="text-sm text-muted-foreground">
                                Trust Score: {(attestation.agentTrustScore * 100).toFixed(0)}%
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            {attestation.isValid ? (
                              <Badge variant="default" className="bg-green-600">
                                <CheckCircle className="h-3 w-3 mr-1" />
                                Valid
                              </Badge>
                            ) : (
                              <Badge variant="destructive">Expired</Badge>
                            )}
                            {canManage && attestation.isValid && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 text-red-600 hover:text-red-700 hover:bg-red-50"
                                onClick={() => openRevokeDialog(attestation)}
                              >
                                <Ban className="h-3.5 w-3.5 mr-1" />
                                Revoke
                              </Button>
                            )}
                          </div>
                        </div>

                        {/* Details */}
                        <div className="grid grid-cols-2 gap-4 text-sm">
                          <div>
                            <span className="text-muted-foreground">Verified:</span>
                            <span className="ml-2">
                              {new Date(attestation.verifiedAt).toLocaleString()}
                            </span>
                          </div>
                          {attestation.connectionLatencyMs > 0 && (
                            <div>
                              <span className="text-muted-foreground">Latency:</span>
                              <span className="ml-2">{attestation.connectionLatencyMs}ms</span>
                            </div>
                          )}
                        </div>

                        {/* Status Badges */}
                        <div className="flex flex-wrap gap-2">
                          {attestation.healthCheckPassed && (
                            <Badge variant="outline" className="text-xs">
                              <CheckCircle className="h-3 w-3 mr-1" />
                              Health Check Passed
                            </Badge>
                          )}
                        </div>

                        {/* Capabilities */}
                        {attestation.capabilitiesConfirmed && attestation.capabilitiesConfirmed.length > 0 && (
                          <div>
                            <p className="text-sm text-muted-foreground mb-2">
                              Capabilities Verified ({attestation.capabilitiesConfirmed.length}):
                            </p>
                            <div className="flex flex-wrap gap-1">
                              {attestation.capabilitiesConfirmed.map((cap, idx) => (
                                <Badge key={idx} variant="secondary" className="text-xs">
                                  {cap}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Manual Attestation Form */}
            <ManualAttestationForm
              mcpServerId={server.id}
              mcpServerName={server.name}
              mcpServerUrl={server.url}
              capabilities={capabilities.map(c => c.name)}
              onAttestationSubmitted={handleRefresh}
              userRole={userRole}
            />
          </TabsContent>

          <TabsContent value="tags">
            <MCPTagsTab mcpServerId={server.id} />
          </TabsContent>

          <TabsContent value="audit" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Audit Trail</CardTitle>
                <CardDescription>
                  History of all operations performed on this MCP server
                </CardDescription>
              </CardHeader>
              <CardContent>
                {auditLogs.length === 0 ? (
                  <div className="text-center py-8">
                    <Activity className="h-12 w-12 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
                    <p className="text-muted-foreground">
                      No audit logs recorded yet
                    </p>
                    <p className="text-xs text-muted-foreground mt-2">
                      Operations on this MCP server will appear here
                    </p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {auditLogs.map((log, index) => {
                      const action = log.action || 'unknown';
                      const actionLabel = action.replace(/_/g, ' ').replace(/\b\w/g, (l: string) => l.toUpperCase());
                      const metadata = log.metadata || {};

                      return (
                        <div
                          key={log.id || index}
                          className="flex items-start gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700"
                        >
                          <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-900/30 rounded-full flex items-center justify-center">
                            <Activity className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center justify-between gap-2">
                              <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                                {actionLabel}
                              </p>
                              <span className="text-xs text-gray-500 dark:text-gray-400">
                                {log.timestamp ? new Date(log.timestamp).toLocaleString() : 'Unknown'}
                              </span>
                            </div>

                            {/* Show agent/user who performed the action */}
                            <div className="mt-1 text-sm">
                              {log.agentId ? (
                                <span className="font-medium text-purple-600 dark:text-purple-400">
                                  Agent: {log.agentName || (log.metadata as Record<string, unknown>)?.agentName as string || log.agentId.slice(0, 8)}
                                </span>
                              ) : log.userId ? (
                                <span className="font-medium text-blue-600 dark:text-blue-400">
                                  User: {log.userName || (log.metadata as Record<string, unknown>)?.userName as string || log.userId.slice(0, 8)}
                                </span>
                              ) : (
                                <span className="text-gray-500 dark:text-gray-400">System</span>
                              )}
                            </div>

                            {/* Show relevant metadata */}
                            {metadata.serverName && (
                              <p className="mt-1 text-xs text-gray-600 dark:text-gray-300">
                                {metadata.serverName}
                              </p>
                            )}

                            {metadata.confidence_score && (
                              <p className="mt-1 text-xs text-gray-600 dark:text-gray-300">
                                Confidence: {metadata.confidence_score.toFixed(1)}%
                              </p>
                            )}

                            {metadata.trustScore && (
                              <p className="mt-1 text-xs text-gray-600 dark:text-gray-300">
                                Trust score: {metadata.trustScore.toFixed(1)}%
                              </p>
                            )}

                            {metadata.reason && (
                              <p className="mt-1 text-xs text-gray-600 dark:text-gray-300">
                                Reason: {metadata.reason}
                              </p>
                            )}

                            {log.ipAddress && (
                              <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                                IP: {log.ipAddress}
                              </p>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="details" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>MCP Server Details</CardTitle>
                <CardDescription>
                  Detailed information about this MCP server
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4">
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      Server ID:
                    </span>
                    <span className="col-span-2 text-sm font-mono">
                      {server.id}
                    </span>
                  </div>
                  <Separator />
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      Name:
                    </span>
                    <span className="col-span-2 text-sm">{server.name}</span>
                  </div>
                  <Separator />
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      URL:
                    </span>
                    <a
                      href={server.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="col-span-2 text-sm text-blue-600 hover:underline"
                    >
                      {server.url}
                    </a>
                  </div>
                  <Separator />
                  {server.description && (
                    <>
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Description:
                        </span>
                        <span className="col-span-2 text-sm">
                          {server.description}
                        </span>
                      </div>
                      <Separator />
                    </>
                  )}
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      Status:
                    </span>
                    <span className="col-span-2 text-sm">
                      <Badge className={getStatusColor(server.status)}>
                        {server.status.charAt(0).toUpperCase() +
                          server.status.slice(1)}
                      </Badge>
                    </span>
                  </div>
                  <Separator />
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      {server.verificationMethod === "agent_attestation"
                        ? "Confidence Score:"
                        : "Trust Score:"}
                    </span>
                    <span className="col-span-2 text-sm">
                      <Badge
                        className={
                          server.verificationMethod === "agent_attestation"
                            ? getConfidenceColor(server.confidenceScore ?? 0)
                            : getTrustColor(server.trustScore ?? 0)
                        }
                      >
                        {server.verificationMethod === "agent_attestation"
                          ? (server.confidenceScore ?? 0).toFixed(1)
                          : (server.trustScore ?? 0).toFixed(1)}%
                      </Badge>
                    </span>
                  </div>
                  <Separator />

                  {/* ✅ NEW: Attestation Info Card */}
                  {server.verificationMethod === "agent_attestation" && (
                    <>
                      <div className="col-span-3 bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                        <div className="flex items-start gap-3">
                          <Shield className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                          <div className="flex-1">
                            <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-2">
                              Verified by Agent Attestations
                            </h4>
                            <div className="grid grid-cols-2 gap-3 text-sm">
                              <div>
                                <span className="text-blue-700 dark:text-blue-300 font-medium">
                                  {server.attestationCount || 0}
                                </span>
                                <span className="text-blue-600 dark:text-blue-400 ml-1">
                                  {server.attestationCount === 1
                                    ? "attestation"
                                    : "attestations"}
                                </span>
                              </div>
                              {server.lastAttestedAt && (
                                <div>
                                  <span className="text-blue-700 dark:text-blue-300 font-medium">
                                    Last attested:
                                  </span>
                                  <span className="text-blue-600 dark:text-blue-400 ml-1">
                                    {new Date(
                                      server.lastAttestedAt
                                    ).toLocaleDateString()}
                                  </span>
                                </div>
                              )}
                            </div>
                            <p className="text-xs text-blue-600 dark:text-blue-400 mt-2">
                              This MCP server's identity is cryptographically
                              verified by {server.attestationCount || 0} verified
                              agent{server.attestationCount !== 1 ? "s" : ""} with
                              Ed25519 signatures.
                            </p>
                          </div>
                        </div>
                      </div>
                      <Separator />
                    </>
                  )}
                  {server.keyType && (
                    <>
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Key Type:
                        </span>
                        <span className="col-span-2 text-sm">
                          {server.keyType}
                        </span>
                      </div>
                      <Separator />
                    </>
                  )}
                  {server.lastVerifiedAt && (
                    <>
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Last Verified:
                        </span>
                        <span className="col-span-2 text-sm">
                          {new Date(server.lastVerifiedAt).toLocaleString()}
                        </span>
                      </div>
                      <Separator />
                    </>
                  )}
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      Created:
                    </span>
                    <span className="col-span-2 text-sm">
                      {new Date(server.createdAt).toLocaleString()}
                    </span>
                  </div>
                  {server.updatedAt && (
                    <>
                      <Separator />
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Last Updated:
                        </span>
                        <span className="col-span-2 text-sm">
                          {new Date(server.updatedAt).toLocaleString()}
                        </span>
                      </div>
                    </>
                  )}
                  <Separator />
                  <div className="grid grid-cols-3 items-center gap-4">
                    <span className="text-sm font-medium text-muted-foreground">
                      Organization ID:
                    </span>
                    <span className="col-span-2 text-sm font-mono">
                      {server.organizationId}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="integration" className="space-y-4">
            <MCPSetupGuide
              mcpServerId={server.id}
              mcpServerName={server.name}
              mcpServerUrl={server.url}
            />
          </TabsContent>
        </Tabs>

        {/* Edit Modal */}
        <RegisterMCPModal
          isOpen={showEditModal}
          onClose={() => setShowEditModal(false)}
          onSuccess={() => {
            setShowEditModal(false);
            handleRefresh();
          }}
          editMode={true}
          initialData={server as any}
        />

        {/* Delete Confirmation */}
        <AlertDialog
          open={showDeleteConfirm}
          onOpenChange={setShowDeleteConfirm}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete MCP Server</AlertDialogTitle>
              <AlertDialogDescription>
                This action cannot be undone. This will permanently delete the
                server "{server?.name}".
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDelete}
                className="bg-red-600 hover:bg-red-700"
              >
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        {/* Attestation Revocation Dialog */}
        <AlertDialog
          open={showRevokeDialog}
          onOpenChange={(open) => {
            setShowRevokeDialog(open);
            if (!open) {
              setSelectedAttestation(null);
              setRevokeReason("");
            }
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <Ban className="h-5 w-5 text-red-600" />
                Revoke Attestation
              </AlertDialogTitle>
              <AlertDialogDescription asChild>
                <div className="space-y-4">
                  <p>
                    You are about to revoke the attestation from{" "}
                    <span className="font-semibold">
                      {selectedAttestation?.agentName}
                    </span>
                    . This action will reduce the MCP server's confidence score.
                  </p>

                  <div className="space-y-2">
                    <label className="text-sm font-medium text-foreground">
                      Reason for revocation <span className="text-red-500">*</span>
                    </label>
                    <Select
                      value={revokeReason}
                      onValueChange={setRevokeReason}
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Select a reason..." />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="compromised">
                          <div className="flex items-center gap-2">
                            <XCircle className="h-4 w-4 text-red-600" />
                            <span>Compromised - Security concern detected</span>
                          </div>
                        </SelectItem>
                        <SelectItem value="outdated">
                          <div className="flex items-center gap-2">
                            <Activity className="h-4 w-4 text-yellow-600" />
                            <span>Outdated - Server changed significantly</span>
                          </div>
                        </SelectItem>
                        <SelectItem value="false_positive">
                          <div className="flex items-center gap-2">
                            <AlertTriangle className="h-4 w-4 text-orange-600" />
                            <span>False Positive - Incorrect attestation</span>
                          </div>
                        </SelectItem>
                        <SelectItem value="policy_violation">
                          <div className="flex items-center gap-2">
                            <Shield className="h-4 w-4 text-purple-600" />
                            <span>Policy Violation - Does not meet requirements</span>
                          </div>
                        </SelectItem>
                        <SelectItem value="other">
                          <div className="flex items-center gap-2">
                            <Edit className="h-4 w-4 text-gray-600" />
                            <span>Other</span>
                          </div>
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  {selectedAttestation && (
                    <div className="bg-muted rounded-lg p-3 space-y-1 text-sm">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Agent:</span>
                        <span className="font-medium">{selectedAttestation.agentName}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Agent Trust:</span>
                        <span className="font-medium">
                          {(selectedAttestation.agentTrustScore * 100).toFixed(0)}%
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Attested:</span>
                        <span className="font-medium">
                          {new Date(selectedAttestation.verifiedAt).toLocaleDateString()}
                        </span>
                      </div>
                    </div>
                  )}
                </div>
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={revoking}>Cancel</AlertDialogCancel>
              <Button
                variant="destructive"
                onClick={handleRevokeAttestation}
                disabled={!revokeReason || revoking}
              >
                {revoking ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Revoking...
                  </>
                ) : (
                  <>
                    <Ban className="h-4 w-4 mr-2" />
                    Revoke Attestation
                  </>
                )}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </AuthGuard>
  );
}
