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
import { api } from "@/lib/api";
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

interface MCPServer {
  id: string;
  name: string;
  url: string;
  description?: string;
  status: "active" | "inactive" | "verified" | "pending";
  public_key?: string;
  key_type?: string;
  last_verified_at?: string;
  created_at: string;
  updated_at?: string;
  trust_score?: number;
  capability_count?: number;
  organization_id: string;
  capabilities?: string[]; // Array of capability type strings like ["tools", "prompts", "resources"]

  // ✅ NEW: Agent Attestation fields
  verification_method?: string; // "agent_attestation", "api_key", or "manual"
  attestation_count?: number;
  confidence_score?: number; // 0-100
  last_attested_at?: string;
}

// Detailed capability information from mcp_server_capabilities table
interface Capability {
  id: string;
  mcp_server_id: string;
  name: string;
  type: "tool" | "resource" | "prompt";
  description: string;
  schema: any;
  detected_at: string;
  last_verified_at?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

interface Agent {
  id: string;
  name: string;
  display_name: string;
  agent_type: string;
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
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [userRole, setUserRole] = useState<
    "admin" | "manager" | "member" | "viewer"
  >("viewer");
  const [verifying, setVerifying] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

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
        } catch (err) {
          console.error("Failed to fetch connected agents:", err);
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
                      server.verification_method === "agent_attestation"
                        ? getConfidenceColor(server.confidence_score ?? 0)
                        : getTrustColor(server.trust_score ?? 0)
                    }
                  >
                    {server.verification_method === "agent_attestation"
                      ? `Confidence: ${(server.confidence_score ?? 0).toFixed(1)}%`
                      : `Trust: ${(server.trust_score ?? 0).toFixed(1)}%`}
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
        {server.verification_method === "agent_attestation" ? (
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
                      Verified by {server.attestation_count || 0} agent attestation
                      {server.attestation_count !== 1 ? "s" : ""}
                    </CardDescription>
                  </div>
                </div>
                <div className="text-right">
                  <div
                    className={`text-4xl font-bold ${
                      getConfidenceColor(server.confidence_score ?? 0).split(" ")[0]
                    }`}
                  >
                    {(server.confidence_score ?? 0).toFixed(1)}%
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {(server.confidence_score ?? 0) >= 80
                      ? "High confidence"
                      : (server.confidence_score ?? 0) >= 60
                        ? "Medium confidence"
                        : (server.confidence_score ?? 0) >= 40
                          ? "Low confidence"
                          : "No attestations"}
                  </p>
                </div>
              </div>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="flex items-center gap-4 text-sm text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <CheckCircle className="h-4 w-4" />
                  <span>
                    {server.attestation_count || 0} attestation
                    {server.attestation_count !== 1 ? "s" : ""}
                  </span>
                </div>
                {server.last_attested_at && (
                  <>
                    <span>•</span>
                    <span>
                      Last verified:{" "}
                      {new Date(server.last_attested_at).toLocaleDateString()}
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
                      getTrustColor(server.trust_score ?? 0).split(" ")[0]
                    }`}
                  >
                    {(server.trust_score ?? 0).toFixed(1)}%
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {(server.trust_score ?? 0) >= 80
                      ? "High trust"
                      : (server.trust_score ?? 0) >= 60
                        ? "Medium trust"
                        : "Low trust"}
                  </p>
                </div>
              </div>
            </CardHeader>
          </Card>
        )}

        {/* Secondary Metrics */}
        <div className="grid gap-4 md:grid-cols-3">
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
                Status
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Badge className={getStatusColor(server.status)}>
                {server.status.charAt(0).toUpperCase() + server.status.slice(1)}
              </Badge>
              <p className="text-xs text-muted-foreground mt-2">
                {server.last_verified_at
                  ? `Verified ${new Date(server.last_verified_at).toLocaleDateString()}`
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
            <TabsTrigger value="tags">
              <Tag className="h-4 w-4 mr-2" />
              Tags
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
                  <div className="text-center py-8">
                    <p className="text-muted-foreground">
                      No capabilities detected yet
                    </p>
                    <p className="text-xs text-muted-foreground mt-2">
                      Click "Verify" to automatically detect capabilities from the MCP server
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
                {connectedAgents.length === 0 ? (
                  <div className="text-center py-8">
                    <p className="text-muted-foreground">
                      No agents connected yet
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
                          <Server className="h-5 w-5 text-blue-600" />
                        </div>
                        <div className="flex-1">
                          <h4 className="font-medium">
                            {agent.display_name || agent.name}
                          </h4>
                          <p className="text-sm text-muted-foreground">
                            {agent.agent_type}
                          </p>
                        </div>
                        <ExternalLink className="h-4 w-4 text-muted-foreground" />
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="tags">
            <MCPTagsTab mcpServerId={server.id} />
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
                      {server.verification_method === "agent_attestation"
                        ? "Confidence Score:"
                        : "Trust Score:"}
                    </span>
                    <span className="col-span-2 text-sm">
                      <Badge
                        className={
                          server.verification_method === "agent_attestation"
                            ? getConfidenceColor(server.confidence_score ?? 0)
                            : getTrustColor(server.trust_score ?? 0)
                        }
                      >
                        {server.verification_method === "agent_attestation"
                          ? (server.confidence_score ?? 0).toFixed(1)
                          : (server.trust_score ?? 0).toFixed(1)}%
                      </Badge>
                    </span>
                  </div>
                  <Separator />

                  {/* ✅ NEW: Attestation Info Card */}
                  {server.verification_method === "agent_attestation" && (
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
                                  {server.attestation_count || 0}
                                </span>
                                <span className="text-blue-600 dark:text-blue-400 ml-1">
                                  {server.attestation_count === 1
                                    ? "attestation"
                                    : "attestations"}
                                </span>
                              </div>
                              {server.last_attested_at && (
                                <div>
                                  <span className="text-blue-700 dark:text-blue-300 font-medium">
                                    Last attested:
                                  </span>
                                  <span className="text-blue-600 dark:text-blue-400 ml-1">
                                    {new Date(
                                      server.last_attested_at
                                    ).toLocaleDateString()}
                                  </span>
                                </div>
                              )}
                            </div>
                            <p className="text-xs text-blue-600 dark:text-blue-400 mt-2">
                              This MCP server's identity is cryptographically
                              verified by {server.attestation_count || 0} verified
                              agent{server.attestation_count !== 1 ? "s" : ""} with
                              Ed25519 signatures.
                            </p>
                          </div>
                        </div>
                      </div>
                      <Separator />
                    </>
                  )}
                  {server.key_type && (
                    <>
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Key Type:
                        </span>
                        <span className="col-span-2 text-sm">
                          {server.key_type}
                        </span>
                      </div>
                      <Separator />
                    </>
                  )}
                  {server.last_verified_at && (
                    <>
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Last Verified:
                        </span>
                        <span className="col-span-2 text-sm">
                          {new Date(server.last_verified_at).toLocaleString()}
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
                      {new Date(server.created_at).toLocaleString()}
                    </span>
                  </div>
                  {server.updated_at && (
                    <>
                      <Separator />
                      <div className="grid grid-cols-3 items-center gap-4">
                        <span className="text-sm font-medium text-muted-foreground">
                          Last Updated:
                        </span>
                        <span className="col-span-2 text-sm">
                          {new Date(server.updated_at).toLocaleString()}
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
                      {server.organization_id}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>
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
      </div>
    </AuthGuard>
  );
}
