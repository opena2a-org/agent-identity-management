"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Bot,
  Shield,
  AlertTriangle,
  ExternalLink,
  Edit,
  Trash2,
  CheckCircle,
  Loader2,
  Download,
  KeyRound,
  Tag,
  Ban,
  Play,
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
import { AutoDetectButton } from "@/components/agents/auto-detect-button";
import { MCPServerSelector } from "@/components/agents/mcp-server-selector";
import { MCPServerList } from "@/components/agents/mcp-server-list";
import { AgentCapabilities } from "@/components/agents/agent-capabilities";
import { api } from "@/lib/api";
import { RegisterAgentModal } from "@/components/modals/register-agent-modal";
import { ViolationsTab } from "@/components/agent/violations-tab";
import { KeyVaultTab } from "@/components/agent/key-vault-tab";
import { TrustScoreBreakdown } from "@/components/agent/trust-score-breakdown";
import { AgentTagsTab } from "@/components/agent/tags-tab";
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
import { Skeleton } from "@/components/ui/skeleton";
import { AuthGuard } from "@/components/auth-guard";

interface Agent {
  id: string;
  name: string;
  display_name: string;
  description: string;
  agent_type: string;
  status: string;
  version: string;
  trust_score: number;
  talks_to?: string[];
  capabilities?: string[]; // Basic capability tags from SDK detection
  created_at: string;
  updated_at: string;
  organization_id: string;
}

interface MCPServer {
  id: string;
  name: string;
  url?: string;
  description?: string;
  command?: string;
  args?: string[];
  status?: string;
  verification_status?: string;
  isActive?: boolean;
  trustScore?: number;
  last_verified_at?: string;
  created_at: string;
  capabilities?: string[];
}

export default function AgentDetailsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const router = useRouter();
  const [agentId, setAgentId] = useState<string | null>(null);
  const [agent, setAgent] = useState<Agent | null>(null);
  const [allAgents, setAllAgents] = useState<Agent[]>([]);
  const [allMCPServers, setAllMCPServers] = useState<MCPServer[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [userRole, setUserRole] = useState<
    "admin" | "manager" | "member" | "viewer"
  >("viewer");
  const [verifying, setVerifying] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [suspending, setSuspending] = useState(false);
  const [reactivating, setReactivating] = useState(false);
  const [rotatingCreds, setRotatingCreds] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showSuspendConfirm, setShowSuspendConfirm] = useState(false);
  const [showRotateCredsConfirm, setShowRotateCredsConfirm] = useState(false);
  const [events, setEvents] = useState<any[]>([]);
  const [detectedMCPs, setDetectedMCPs] = useState<any[]>([]);

  // Extract agent ID from params Promise
  useEffect(() => {
    params.then(({ id }) => setAgentId(id));
  }, [params]);

  // Fetch agent data
  useEffect(() => {
    if (!agentId) return;

    async function fetchData() {
      setIsLoading(true);
      setError(null);

      try {
        // Fetch current agent
        const agentData = await api.getAgent(agentId!);
        setAgent(agentData);

        // Fetch all agents (for graph visualization)
        const agentsResponse = await api.listAgents();
        setAllAgents(agentsResponse.agents || []);

        // Fetch all MCP servers (for graph visualization)
        const mcpServersResponse = await api.listMCPServers(100, 0);
        setAllMCPServers(mcpServersResponse.mcp_servers || []);

        // Fetch verification events (for trust score chart)
        try {
          const ev = await api.getRecentVerificationEvents(60);
          setEvents(ev.events?.filter((e: any) => e.agentId === agentId) || []);
        } catch (e) {
          // non-fatal
        }

        // Fetch detection status (for detected MCP servers)
        try {
          const detectionStatus = await api.getDetectionStatus(agentId!);
          setDetectedMCPs(detectionStatus.detectedMCPs || []);
        } catch (e) {
          // non-fatal
        }

        // Fetch agent's connected MCP servers (via attestation)
        // This gets the actual MCP servers the agent is connected to from agent_mcp_connections table
        try {
          const agentMCPsResponse = await api.getAgentMCPServers(agentId!);
          if (agentMCPsResponse.mcp_servers && agentMCPsResponse.mcp_servers.length > 0) {
            // Merge these servers into allMCPServers (they may not be in the org-wide list yet)
            const mcpServerIds = agentMCPsResponse.mcp_servers.map(s => s.id);
            setAllMCPServers(prev => {
              const existingIds = new Set(prev.map(s => s.id));
              // Convert ConnectedMCPServer to MCPServer format
              const newServers = agentMCPsResponse.mcp_servers!
                .filter(s => !existingIds.has(s.id))
                .map(s => ({
                  ...s,
                  created_at: s.created_at || new Date().toISOString(), // Add created_at if missing
                } as MCPServer));
              return [...prev, ...newServers];
            });

            // Update agent's talks_to to include these server IDs
            // This fixes the data consistency issue where talks_to is out of sync
            if (agentData) {
              agentData.talks_to = mcpServerIds;
              setAgent({ ...agentData });
            }
          }
        } catch (e) {
          console.error("Failed to fetch agent MCP servers:", e);
          // non-fatal
        }
      } catch (err: any) {
        console.error("Failed to fetch agent data:", err);
        setError(err.message || "Failed to load agent details");
      } finally {
        setIsLoading(false);
      }
    }

    fetchData();
  }, [agentId, refreshKey]);

  // Extract user role from token for permissions
  useEffect(() => {
    const token = api.getToken?.();
    if (!token) return;
    try {
      const payload = JSON.parse(atob(token.split(".")[1]));
      const role = (payload.role as any) || "viewer";
      setUserRole(role);
    } catch {}
  }, []);

  const handleRefresh = () => {
    setRefreshKey((prev) => prev + 1);
  };

  const canEdit = ["admin", "manager", "member"].includes(userRole);
  const canManage = ["admin", "manager"].includes(userRole);

  const handleVerify = async () => {
    if (!agentId) return;
    setVerifying(true);
    try {
      await api.verifyAgent(agentId);
      handleRefresh();
    } catch (e: any) {
      alert(e?.message || "Verification failed");
    } finally {
      setVerifying(false);
    }
  };

  const handleDelete = async () => {
    if (!agentId) return;
    setDeleting(true);
    try {
      await api.deleteAgent(agentId);
      router.push("/dashboard/agents");
    } catch (e: any) {
      alert(e?.message || "Delete failed");
    } finally {
      setDeleting(false);
      setShowDeleteConfirm(false);
    }
  };

  const handleSuspend = async () => {
    if (!agentId) return;
    setSuspending(true);
    try {
      await api.suspendAgent(agentId);
      alert("Agent suspended successfully");
      handleRefresh();
    } catch (e: any) {
      alert(e?.message || "Suspend failed");
    } finally {
      setSuspending(false);
      setShowSuspendConfirm(false);
    }
  };

  const handleReactivate = async () => {
    if (!agentId) return;
    setReactivating(true);
    try {
      await api.reactivateAgent(agentId);
      alert("Agent reactivated successfully");
      handleRefresh();
    } catch (e: any) {
      alert(e?.message || "Reactivate failed");
    } finally {
      setReactivating(false);
    }
  };

  const handleRotateCredentials = async () => {
    if (!agentId) return;
    setRotatingCreds(true);
    try {
      const result = await api.rotateAgentCredentials(agentId);
      alert(`Credentials rotated successfully!\n\nNew API Key:\n${result.api_key}\n\nPlease save this key - you won't be able to see it again.`);
      handleRefresh();
    } catch (e: any) {
      alert(e?.message || "Credential rotation failed");
    } finally {
      setRotatingCreds(false);
      setShowRotateCredsConfirm(false);
    }
  };

  // Get trust score color
  const getTrustColor = (score: number): string => {
    if (score >= 80) return "text-green-600 bg-green-500/10";
    if (score >= 60) return "text-yellow-600 bg-yellow-500/10";
    return "text-red-600 bg-red-500/10";
  };

  // Check if agent is verified
  const isVerified = agent?.status === "verified";

  // Check if agent is active
  const isActive = agent?.status !== "suspended" && agent?.status !== "revoked";

  // Create mapping from MCP server name to ID for clickable navigation
  const serverNameToId = new Map<string, string>();
  allMCPServers.forEach((server) => {
    serverNameToId.set(server.name, server.id);
  });

  // Get connected MCP server names and details
  const connectedMCPServers = useMemo(() => {
    if (!agent?.talks_to || allMCPServers.length === 0) return [];

    // Filter MCP servers that this agent talks to
    return allMCPServers
      .filter((server) => agent.talks_to?.includes(server.id))
      .map((server) => server.name);
  }, [agent?.talks_to, allMCPServers]);

  // Get connected MCP server details with capabilities
  const connectedMCPServerDetails = useMemo(() => {
    if (!agent?.talks_to || allMCPServers.length === 0) return [];

    return allMCPServers.filter((server) => agent.talks_to?.includes(server.id));
  }, [agent?.talks_to, allMCPServers]);

  // Loading state
  if (isLoading) {
    return (
      <div className="space-y-6">
        {/* Header skeleton */}
        <div>
          <Skeleton className="h-8 w-40 mb-4" />
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-4">
              <Skeleton className="h-16 w-16 rounded-xl" />
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <Skeleton className="h-8 w-64" />
                  <Skeleton className="h-6 w-6 rounded-full" />
                </div>
                <Skeleton className="h-4 w-80 mb-2" />
                <div className="flex items-center gap-2 flex-wrap">
                  <Skeleton className="h-6 w-20 rounded-full" />
                  <Skeleton className="h-6 w-16 rounded-full" />
                  <Skeleton className="h-6 w-28 rounded-full" />
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Skeleton className="h-9 w-24" />
              <Skeleton className="h-9 w-24" />
              <Skeleton className="h-9 w-24" />
            </div>
          </div>
        </div>

        <Separator />

        {/* Info cards skeleton */}
        <div className="grid gap-4 md:grid-cols-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="p-4 border rounded-lg">
              <Skeleton className="h-4 w-32 mb-3" />
              <Skeleton className="h-7 w-16" />
            </div>
          ))}
        </div>

        {/* Tabs skeleton */}
        <div className="space-y-4">
          <div className="flex gap-2">
            <Skeleton className="h-9 w-32" />
            <Skeleton className="h-9 w-40" />
            <Skeleton className="h-9 w-28" />
          </div>
          <div className="p-4 border rounded-lg space-y-3">
            {[...Array(4)].map((_, i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (error || !agent) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Card className="max-w-md">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-5 w-5" />
              Error Loading Agent
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-muted-foreground mb-4">
              {error ||
                "Agent not found or you do not have permission to view it."}
            </p>
            <Button
              variant="outline"
              onClick={() => router.push("/dashboard/agents")}
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Agents
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
          onClick={() => router.push("/dashboard/agents")}
          className="mb-4"
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Agents
        </Button>

        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="flex h-16 w-16 items-center justify-center rounded-xl bg-primary/10">
              <Bot className="h-8 w-8 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-2 mb-1">
                <h1 className="text-3xl font-bold">{agent.name}</h1>
                {isVerified && (
                  <span title="Verified">
                    <Shield className="h-6 w-6 text-green-600" />
                  </span>
                )}
              </div>
              <p className="text-muted-foreground mb-2">{agent.description}</p>
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="outline">{agent.agent_type}</Badge>
                {isActive ? (
                  <Badge className="bg-green-500/10 text-green-600">
                    Active
                  </Badge>
                ) : (
                  <Badge variant="secondary">Inactive</Badge>
                )}
                <Badge
                  className={getTrustColor((agent.trust_score ?? 0) * 100)}
                >
                  Trust: {((agent.trust_score ?? 0) * 100).toFixed(1)}%
                </Badge>
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2 flex-wrap">
            <AutoDetectButton
              agentId={agent.id}
              onDetectionComplete={handleRefresh}
              variant="default"
            />
            <MCPServerSelector
              agentId={agent.id}
              currentMCPServers={agent.talks_to ?? []}
              onSelectionComplete={handleRefresh}
              variant="outline"
            />
            <Button
              variant="outline"
              onClick={() => router.push('/dashboard/sdk')}
            >
              <Download className="h-4 w-4 mr-1" /> Download SDK
            </Button>
            <Button
              variant="outline"
              onClick={() => router.push(`/dashboard/sdk-tokens`)}
            >
              <KeyRound className="h-4 w-4 mr-1" /> Get Credentials
            </Button>
            {canEdit && (
              <Button variant="outline" onClick={() => setShowEditModal(true)}>
                <Edit className="h-4 w-4 mr-1" /> Edit
              </Button>
            )}
            {canManage && (
              <Button
                onClick={handleVerify}
                disabled={verifying || isVerified}
                className="bg-green-600 hover:bg-green-700"
              >
                {verifying ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                    Verifying...
                  </>
                ) : (
                  <>
                    <CheckCircle className="h-4 w-4 mr-1" />{" "}
                    {isVerified ? "Verified" : "Verify Agent"}
                  </>
                )}
              </Button>
            )}
            {canManage && agent.status !== "suspended" && (
              <Button
                variant="outline"
                onClick={() => setShowSuspendConfirm(true)}
                disabled={suspending}
                className="border-orange-500 text-orange-600 hover:bg-orange-50"
              >
                {suspending ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                    Suspending...
                  </>
                ) : (
                  <>
                    <Ban className="h-4 w-4 mr-1" /> Suspend
                  </>
                )}
              </Button>
            )}
            {canManage && agent.status === "suspended" && (
              <Button
                variant="outline"
                onClick={handleReactivate}
                disabled={reactivating}
                className="border-green-500 text-green-600 hover:bg-green-50"
              >
                {reactivating ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                    Reactivating...
                  </>
                ) : (
                  <>
                    <Play className="h-4 w-4 mr-1" /> Reactivate
                  </>
                )}
              </Button>
            )}
            {canEdit && (
              <Button
                variant="outline"
                onClick={() => setShowRotateCredsConfirm(true)}
                disabled={rotatingCreds}
                className="border-blue-500 text-blue-600 hover:bg-blue-50"
              >
                {rotatingCreds ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                    Rotating...
                  </>
                ) : (
                  <>
                    <KeyRound className="h-4 w-4 mr-1" /> Rotate Credentials
                  </>
                )}
              </Button>
            )}
            {canManage && (
              <Button
                variant="destructive"
                onClick={() => setShowDeleteConfirm(true)}
                disabled={deleting}
              >
                {deleting ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-1 animate-spin" />{" "}
                    Deleting...
                  </>
                ) : (
                  <>
                    <Trash2 className="h-4 w-4 mr-1" /> Delete
                  </>
                )}
              </Button>
            )}
          </div>
        </div>
      </div>

      <Separator />

      {/* Agent Info Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              MCP Connections
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {connectedMCPServers.length}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Connected MCP server
              {connectedMCPServers.length !== 1 ? "s" : ""}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Trust Score
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div
              className={`text-2xl font-bold ${getTrustColor((agent.trust_score ?? 0) * 100).split(" ")[0]}`}
            >
              {((agent.trust_score ?? 0) * 100).toFixed(1)}%
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {(agent.trust_score ?? 0) * 100 >= 80
                ? "High trust"
                : (agent.trust_score ?? 0) * 100 >= 60
                  ? "Medium trust"
                  : "Low trust"}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Verification Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {isVerified ? (
                <Shield className="h-8 w-8 text-green-600" />
              ) : (
                <AlertTriangle className="h-8 w-8 text-yellow-600" />
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {isVerified ? "Verified agent" : "Pending verification"}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="connections" className="space-y-4">
        <TabsList>
          <TabsTrigger value="connections">
            <ExternalLink className="h-4 w-4 mr-2" />
            MCPs
          </TabsTrigger>
          <TabsTrigger value="capabilities">
            <Shield className="h-4 w-4 mr-2" />
            Capabilities
          </TabsTrigger>
          <TabsTrigger value="violations">
            <AlertTriangle className="h-4 w-4 mr-2" />
            Violations
          </TabsTrigger>
          <TabsTrigger value="key-vault">
            <KeyRound className="h-4 w-4 mr-2" />
            Key Vault
          </TabsTrigger>
          <TabsTrigger value="tags">
            <Tag className="h-4 w-4 mr-2" />
            Tags
          </TabsTrigger>
          <TabsTrigger value="activity">Recent Activity</TabsTrigger>
          <TabsTrigger value="trust">
            <Shield className="h-4 w-4 mr-2" />
            Trust Score
          </TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>

        <TabsContent value="connections" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>MCP Server Connections</CardTitle>
              <CardDescription>
                Manage which MCP servers this agent can communicate with. Shows both manually connected servers and auto-detected servers from SDK runtime.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Manually Connected Servers */}
              {connectedMCPServers.length > 0 && (
                <div>
                  <h3 className="text-sm font-semibold mb-3 text-muted-foreground">Manually Connected</h3>
                  <MCPServerList
                    agentId={agent.id}
                    mcpServers={connectedMCPServers}
                    serverDetails={connectedMCPServerDetails.map(server => ({
                      name: server.name,
                      id: server.id,
                      capabilities: server.capabilities,
                      url: server.url
                    }))}
                    serverNameToId={serverNameToId}
                    onUpdate={handleRefresh}
                    showBulkActions={true}
                  />
                </div>
              )}

              {/* Auto-Detected Servers */}
              {detectedMCPs.length > 0 && (
                <div>
                  <h3 className="text-sm font-semibold mb-3 text-muted-foreground">Auto-Detected by SDK</h3>
                  <div className="grid gap-3">
                    {detectedMCPs.map((detection: any) => (
                      <div key={detection.name} className="p-4 rounded-lg border bg-card">
                        <div className="flex items-start justify-between gap-2 mb-2">
                          <div className="flex items-center gap-2">
                            <h4 className="font-semibold text-sm">{detection.name}</h4>
                          </div>
                          <div className="flex gap-2">
                            <Badge variant="secondary" className="text-xs">
                              {Math.round(detection.confidenceScore)}% confidence
                            </Badge>
                            {detection.detectedBy && detection.detectedBy.length > 0 && (
                              <Badge variant="outline" className="text-xs">
                                {detection.detectedBy[0].replace(/_/g, ' ')}
                              </Badge>
                            )}
                          </div>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Detected via SDK runtime monitoring
                          {detection.lastSeen && ` • Last seen ${new Date(detection.lastSeen).toLocaleString()}`}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Empty State */}
              {connectedMCPServers.length === 0 && detectedMCPs.length === 0 && (
                <div className="text-center py-12 px-4">
                  <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-muted mb-4">
                    <ExternalLink className="h-8 w-8 text-muted-foreground" />
                  </div>
                  <h3 className="text-lg font-semibold mb-2">No MCP Servers Detected</h3>
                  <p className="text-sm text-muted-foreground max-w-md mx-auto mb-6">
                    This agent has no MCP servers connected or detected. Use the buttons above to add servers manually or install the AIM SDK to enable auto-detection.
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="capabilities">
          <AgentCapabilities
            agentId={agent.id}
            agentCapabilities={agent.capabilities}
          />
        </TabsContent>

        <TabsContent value="violations">
          <ViolationsTab agentId={agent.id} />
        </TabsContent>

        <TabsContent value="key-vault">
          <KeyVaultTab agentId={agent.id} />
        </TabsContent>

        <TabsContent value="tags">
          <AgentTagsTab agentId={agent.id} />
        </TabsContent>

        <TabsContent value="activity">
          <Card>
            <CardHeader>
              <CardTitle>Recent Activity</CardTitle>
              <CardDescription>
                Latest verification events and actions
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                  <thead className="bg-gray-50 dark:bg-gray-800">
                    <tr>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">
                        When
                      </th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">
                        Type
                      </th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">
                        Status
                      </th>
                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">
                        Confidence
                      </th>
                    </tr>
                  </thead>
                  <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                    {events.slice(0, 10).map((ev) => (
                      <tr key={ev.id}>
                        <td className="px-4 py-2 text-sm">
                          {new Date(ev.startedAt).toLocaleString()}
                        </td>
                        <td className="px-4 py-2 text-sm">
                          {ev.verificationType}
                        </td>
                        <td className="px-4 py-2 text-sm">{ev.status}</td>
                        <td className="px-4 py-2 text-sm">
                          {(ev.confidence * 100).toFixed(1)}%
                        </td>
                      </tr>
                    ))}
                    {events.length === 0 && (
                      <tr>
                        <td
                          colSpan={4}
                          className="px-4 py-6 text-center text-sm text-muted-foreground"
                        >
                          No recent activity
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="trust">
          <TrustScoreBreakdown agentId={agent.id} userRole={userRole} />
        </TabsContent>

        <TabsContent value="details" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Agent Details</CardTitle>
              <CardDescription>
                Detailed information about this agent
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4">
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Agent ID:
                  </span>
                  <span className="col-span-2 text-sm font-mono">
                    {agent.id}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Name:
                  </span>
                  <span className="col-span-2 text-sm">{agent.name}</span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Type:
                  </span>
                  <span className="col-span-2 text-sm">{agent.agent_type}</span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Description:
                  </span>
                  <span className="col-span-2 text-sm">
                    {agent.description}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Status:
                  </span>
                  <span className="col-span-2 text-sm">
                    {isActive ? (
                      <Badge className="bg-green-500/10 text-green-600">
                        Active
                      </Badge>
                    ) : (
                      <Badge variant="secondary">Inactive</Badge>
                    )}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Verified:
                  </span>
                  <span className="col-span-2 text-sm">
                    {isVerified ? (
                      <Badge className="bg-green-500/10 text-green-600">
                        Verified
                      </Badge>
                    ) : (
                      <Badge variant="secondary">Unverified</Badge>
                    )}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Protocols Agent Uses:
                  </span>
                  <span className="col-span-2 text-sm">
                    <Badge variant="secondary">MCP</Badge>
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Trust Score:
                  </span>
                  <span className="col-span-2 text-sm">
                    <Badge
                      className={getTrustColor((agent.trust_score ?? 0) * 100)}
                    >
                      {((agent.trust_score ?? 0) * 100).toFixed(1)}%
                    </Badge>
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Created:
                  </span>
                  <span className="col-span-2 text-sm">
                    {new Date(agent.created_at).toLocaleString()}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Last Updated:
                  </span>
                  <span className="col-span-2 text-sm">
                    {new Date(agent.updated_at).toLocaleString()}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Organization ID:
                  </span>
                  <span className="col-span-2 text-sm font-mono">
                    {agent.organization_id}
                  </span>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Edit Modal */}
      <RegisterAgentModal
        isOpen={showEditModal}
        onClose={() => setShowEditModal(false)}
        onSuccess={() => {
          setShowEditModal(false);
          handleRefresh();
        }}
        editMode={true}
        initialData={agent as any}
      />

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Agent</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. This will permanently delete the
              agent "{agent.name}" and remove associated data.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-red-600 hover:bg-red-700"
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Suspend Confirmation Dialog */}
      <AlertDialog open={showSuspendConfirm} onOpenChange={setShowSuspendConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Suspend Agent</AlertDialogTitle>
            <AlertDialogDescription>
              This will temporarily suspend the agent "{agent.name}". The agent
              will be unable to authenticate or perform actions until
              reactivated. You can reactivate this agent at any time.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleSuspend}
              className="bg-orange-600 hover:bg-orange-700"
            >
              {suspending ? "Suspending..." : "Suspend"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Rotate Credentials Confirmation Dialog */}
      <AlertDialog open={showRotateCredsConfirm} onOpenChange={setShowRotateCredsConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Rotate Agent Credentials</AlertDialogTitle>
            <AlertDialogDescription>
              This will generate new credentials for "{agent.name}". The old API key will be immediately revoked and can no longer be used.
              <br /><br />
              <strong>Warning:</strong> Any applications using the old key will lose access until updated with the new key.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRotateCredentials}
              className="bg-blue-600 hover:bg-blue-700"
            >
              {rotatingCreds ? "Rotating..." : "Rotate Credentials"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
    </AuthGuard>
  );
}
