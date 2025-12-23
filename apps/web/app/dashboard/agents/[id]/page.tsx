"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
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
  KeyRound,
  Tag,
  Ban,
  Play,
  TrendingDown,
  TrendingUp,
  Bell,
  Activity,
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
import { api, Agent } from "@/lib/api";
import { RegisterAgentModal } from "@/components/modals/register-agent-modal";
import { ViolationsTab } from "@/components/agent/violations-tab";
import { KeyVaultTab } from "@/components/agent/key-vault-tab";
import { APIKeysTab } from "@/components/agent/api-keys-tab";
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

interface MCPServer {
  id: string;
  name: string;
  url?: string;
  description?: string;
  command?: string;
  args?: string[];
  status?: string;
  verificationStatus?: string;
  isActive?: boolean;
  trustScore?: number;
  lastVerifiedAt?: string;
  createdAt: string;
  capabilities?: string[];
}

export default function AgentDetailsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [agentId, setAgentId] = useState<string | null>(null);

  // URL-based tab persistence
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'connections');
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
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [showSuspendConfirm, setShowSuspendConfirm] = useState(false);
  const [events, setEvents] = useState<any[]>([]);
  const [agentActivity, setAgentActivity] = useState<any[]>([]);
  const [detectedMCPs, setDetectedMCPs] = useState<any[]>([]);
  const [agentAlerts, setAgentAlerts] = useState<any[]>([]);
  const [trustScoreHistory, setTrustScoreHistory] = useState<any[]>([]);
  const [activityPage, setActivityPage] = useState(1);
  const ACTIVITY_PAGE_SIZE = 20;

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

        // Fetch fresh trust score and update agent data for consistency
        try {
          const trustBreakdown = await api.getTrustScoreBreakdown(agentId!);
          if (trustBreakdown?.overall !== undefined) {
            agentData.trustScore = trustBreakdown.overall;
          }
        } catch (e) {
          // non-fatal - use stored trust score
        }

        setAgent(agentData);

        // Fetch all agents (for graph visualization)
        const agentsResponse = await api.listAgents();
        setAllAgents(agentsResponse.agents || []);

        // Fetch all MCP servers (for graph visualization)
        const mcpServersResponse = await api.listMCPServers(100, 0);
        setAllMCPServers(mcpServersResponse.mcpServers || []);

        // Fetch verification events (for trust score chart)
        try {
          const ev = await api.getRecentVerificationEvents(60);
          setEvents(ev.events?.filter((e: any) => e.agentId === agentId) || []);
        } catch (e) {
          // non-fatal
        }

        // Fetch agent activity (actions the agent has performed)
        try {
          const activityResponse = await api.getSingleAgentActivity(agentId!);
          setAgentActivity(activityResponse.activities || []);
        } catch (e) {
          // Activity fetch is non-fatal - agent page still works without it
        }

        // Fetch detection status (for detected MCP servers)
        try {
          const detectionStatus = await api.getDetectionStatus(agentId!);
          setDetectedMCPs(detectionStatus.detectedMCPs || []);
        } catch (e) {
          // non-fatal
        }

        // Fetch agent-specific alerts (security events, trust score drops, etc.)
        try {
          const alertsResponse = await api.getAgentAlerts(agentId!, 50, 0);
          setAgentAlerts(alertsResponse.alerts || []);
        } catch (e) {
          // non-fatal - alerts are supplementary
        }

        // Fetch trust score history (for timeline)
        try {
          const historyResponse = await api.getAgentTrustScoreHistory(agentId!);
          setTrustScoreHistory(historyResponse.history || []);
        } catch (e) {
          // non-fatal
        }

        // Fetch agent's connected MCP servers (via attestation)
        // This gets the actual MCP servers the agent is connected to from agent_mcp_connections table
        try {
          const agentMCPsResponse = await api.getAgentMCPServers(agentId!);
          if (agentMCPsResponse.mcpServers && agentMCPsResponse.mcpServers.length > 0) {
            // Merge these servers into allMCPServers (they may not be in the org-wide list yet)
            const mcpServerIds = agentMCPsResponse.mcpServers.map(s => s.id);
            setAllMCPServers(prev => {
              const existingIds = new Set(prev.map(s => s.id));
              // Convert ConnectedMCPServer to MCPServer format
              const newServers = agentMCPsResponse.mcpServers!
                .filter(s => !existingIds.has(s.id))
                .map(s => ({
                  ...s,
                  created_at: s.createdAt || new Date().toISOString(), // Add created_at if missing
                } as MCPServer));
              return [...prev, ...newServers];
            });

            // Update agent's talks_to to include these server IDs
            // This fixes the data consistency issue where talks_to is out of sync
            if (agentData) {
              agentData.talksTo = mcpServerIds;
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

  // Handle tab change with URL update
  const handleTabChange = (tab: string) => {
    setActiveTab(tab);
    const params = new URLSearchParams(searchParams.toString());
    params.set('tab', tab);
    router.push(`?${params.toString()}`, { scroll: false });
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
    if (!agent?.talksTo || allMCPServers.length === 0) return [];

    // Filter MCP servers that this agent talks to
    return allMCPServers
      .filter((server) => agent.talksTo?.includes(server.id))
      .map((server) => server.name);
  }, [agent?.talksTo, allMCPServers]);

  // Get connected MCP server details with capabilities
  const connectedMCPServerDetails = useMemo(() => {
    if (!agent?.talksTo || allMCPServers.length === 0) return [];

    return allMCPServers.filter((server) => agent.talksTo?.includes(server.id));
  }, [agent?.talksTo, allMCPServers]);

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
                <Badge variant="outline">{agent.agentType}</Badge>
                {isActive ? (
                  <Badge className="bg-green-500/10 text-green-600">
                    Active
                  </Badge>
                ) : (
                  <Badge variant="secondary">Inactive</Badge>
                )}
                <Badge
                  className={getTrustColor((agent.trustScore ?? 0) * 100)}
                >
                  Trust: {((agent.trustScore ?? 0) * 100).toFixed(1)}%
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
              currentMCPServers={agent.talksTo ?? []}
              onSelectionComplete={handleRefresh}
              variant="outline"
            />
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
              {connectedMCPServers.length + detectedMCPs.length}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {connectedMCPServers.length > 0 && detectedMCPs.length > 0
                ? `${connectedMCPServers.length} connected, ${detectedMCPs.length} detected`
                : connectedMCPServers.length > 0
                ? `Connected MCP server${connectedMCPServers.length !== 1 ? "s" : ""}`
                : detectedMCPs.length > 0
                ? `Auto-detected server${detectedMCPs.length !== 1 ? "s" : ""}`
                : "No MCP servers"}
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
              className={`text-2xl font-bold ${getTrustColor((agent.trustScore ?? 0) * 100).split(" ")[0]}`}
            >
              {((agent.trustScore ?? 0) * 100).toFixed(1)}%
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {(agent.trustScore ?? 0) * 100 >= 80
                ? "High trust"
                : (agent.trustScore ?? 0) * 100 >= 60
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
      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-4">
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
            Identity & Signing
          </TabsTrigger>
          <TabsTrigger value="api-keys">
            <KeyRound className="h-4 w-4 mr-2" />
            API Keys
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
                        <div className="flex items-center justify-between gap-4 mb-2">
                          <div className="flex-1 min-w-0">
                            <h4 className="font-semibold text-sm truncate">{detection.name}</h4>
                          </div>
                          <div className="flex items-center gap-2 flex-shrink-0">
                            <Badge variant="secondary" className="text-xs whitespace-nowrap">
                              {Math.round(detection.confidenceScore)}% confidence
                            </Badge>
                            {detection.detectedBy && detection.detectedBy.length > 0 && (
                              <Badge variant="outline" className="text-xs whitespace-nowrap min-w-[90px] justify-center">
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

        <TabsContent value="api-keys">
          <APIKeysTab agentId={agent.id} />
        </TabsContent>

        <TabsContent value="tags">
          <AgentTagsTab agentId={agent.id} />
        </TabsContent>

        <TabsContent value="activity">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Activity className="h-5 w-5" />
                Activity Timeline
              </CardTitle>
              <CardDescription>
                Unified view of trust score changes, security alerts, verifications, and agent actions
              </CardDescription>
            </CardHeader>
            <CardContent>
              {/* Unified Timeline */}
              {(() => {
                // Combine all events into a unified timeline
                const timelineEvents: Array<{
                  id: string;
                  type: 'alert' | 'trust_change' | 'verification' | 'action';
                  timestamp: Date;
                  title: string;
                  description: string;
                  severity?: string;
                  icon: 'alert' | 'trust_up' | 'trust_down' | 'verification' | 'action';
                  badge?: { text: string; variant: 'default' | 'secondary' | 'outline' | 'destructive' };
                  metadata?: Record<string, any>;
                }> = [];

                // Add alerts
                agentAlerts.forEach((alert: any) => {
                  timelineEvents.push({
                    id: `alert-${alert.id}`,
                    type: 'alert',
                    timestamp: new Date(alert.createdAt),
                    title: alert.title,
                    description: alert.description,
                    severity: alert.severity,
                    icon: 'alert',
                    badge: {
                      text: alert.severity?.toUpperCase() || 'ALERT',
                      variant: alert.severity === 'critical' || alert.severity === 'high' ? 'destructive' :
                               alert.severity === 'warning' ? 'secondary' : 'outline'
                    },
                    metadata: alert.metadata
                  });
                });

                // Add trust score changes
                trustScoreHistory.forEach((change: any, idx: number) => {
                  const prevScore = idx < trustScoreHistory.length - 1
                    ? trustScoreHistory[idx + 1]?.trustScore
                    : null;
                  const currentScore = change.trustScore;
                  const isIncrease = prevScore !== null && currentScore > prevScore;
                  const isDecrease = prevScore !== null && currentScore < prevScore;
                  const scoreDiff = prevScore !== null ? ((currentScore - prevScore) * 100).toFixed(1) : null;

                  timelineEvents.push({
                    id: `trust-${change.timestamp}-${idx}`,
                    type: 'trust_change',
                    timestamp: new Date(change.timestamp),
                    title: `Trust Score ${isIncrease ? 'Increased' : isDecrease ? 'Decreased' : 'Updated'}`,
                    description: scoreDiff
                      ? `${isDecrease ? '' : '+'}${scoreDiff}% → Now at ${(currentScore * 100).toFixed(1)}%${change.reason ? `: ${change.reason}` : ''}`
                      : `Trust score is ${(currentScore * 100).toFixed(1)}%${change.reason ? `: ${change.reason}` : ''}`,
                    icon: isDecrease ? 'trust_down' : isIncrease ? 'trust_up' : 'trust_up',
                    badge: {
                      text: `${(currentScore * 100).toFixed(0)}%`,
                      variant: currentScore >= 0.7 ? 'default' : currentScore >= 0.4 ? 'secondary' : 'destructive'
                    }
                  });
                });

                // Add verification events
                events.forEach((ev: any) => {
                  timelineEvents.push({
                    id: `verification-${ev.id}`,
                    type: 'verification',
                    timestamp: new Date(ev.startedAt),
                    title: `Verification: ${ev.verificationType?.replace(/_/g, ' ') || 'Check'}`,
                    description: `Status: ${ev.status}`,
                    icon: 'verification',
                    badge: {
                      text: ev.status,
                      variant: ev.status === 'passed' || ev.status === 'success' ? 'default' :
                               ev.status === 'failed' ? 'destructive' : 'secondary'
                    }
                  });
                });

                // Add agent actions
                agentActivity.forEach((activity: any) => {
                  const meta = activity.metadata || {};
                  const riskLevel = meta.riskLevel || 'unknown';
                  const resource = meta.resource || activity.resourceType || 'unknown';
                  const wasApproved = meta.autoApproved;
                  const trustScore = meta.trustScore ? `${(meta.trustScore * 100).toFixed(0)}%` : null;

                  // Build a detailed description
                  let description = `Resource: ${resource}`;
                  if (riskLevel !== 'unknown') {
                    description += ` • Risk: ${riskLevel}`;
                  }
                  if (trustScore) {
                    description += ` • Trust: ${trustScore}`;
                  }
                  if (wasApproved !== undefined) {
                    description += wasApproved ? ' • Auto-approved' : ' • Denied';
                  }

                  timelineEvents.push({
                    id: `action-${activity.id}`,
                    type: 'action',
                    timestamp: new Date(activity.timestamp),
                    title: activity.action?.replace(/_/g, ' ').replace(/:/g, ':') || 'Action',
                    description,
                    icon: 'action',
                    badge: {
                      text: riskLevel,
                      variant: riskLevel === 'high' || riskLevel === 'critical' ? 'destructive' :
                               riskLevel === 'medium' ? 'secondary' : 'outline'
                    },
                    metadata: meta
                  });
                });

                // Sort by timestamp (newest first)
                timelineEvents.sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());

                const hasEvents = timelineEvents.length > 0;

                if (!hasEvents) {
                  return (
                    <div className="text-center py-12">
                      <Activity className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                      <p className="text-muted-foreground">No activity recorded for this agent yet.</p>
                      <p className="text-sm text-muted-foreground mt-2">
                        Activities will appear here when the agent performs actions or security events occur.
                      </p>
                    </div>
                  );
                }

                return (
                  <div className="space-y-4">
                    {/* Summary Cards */}
                    <div className="grid grid-cols-4 gap-4 mb-6">
                      <div className="p-3 rounded-lg border bg-card">
                        <div className="flex items-center gap-2 text-muted-foreground text-xs mb-1">
                          <Bell className="h-3 w-3" />
                          Alerts
                        </div>
                        <div className="text-2xl font-bold">{agentAlerts.length}</div>
                      </div>
                      <div className="p-3 rounded-lg border bg-card">
                        <div className="flex items-center gap-2 text-muted-foreground text-xs mb-1">
                          <TrendingDown className="h-3 w-3" />
                          Trust Changes
                        </div>
                        <div className="text-2xl font-bold">{trustScoreHistory.length}</div>
                      </div>
                      <div className="p-3 rounded-lg border bg-card">
                        <div className="flex items-center gap-2 text-muted-foreground text-xs mb-1">
                          <Shield className="h-3 w-3" />
                          Verifications
                        </div>
                        <div className="text-2xl font-bold">{events.length}</div>
                      </div>
                      <div className="p-3 rounded-lg border bg-card">
                        <div className="flex items-center gap-2 text-muted-foreground text-xs mb-1">
                          <Bot className="h-3 w-3" />
                          Performed Actions
                        </div>
                        <div className="text-2xl font-bold">{agentActivity.length}</div>
                      </div>
                    </div>

                    {/* Timeline */}
                    <div className="relative">
                      <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-border" />
                      <div className="space-y-4">
                        {timelineEvents.slice(0, activityPage * ACTIVITY_PAGE_SIZE).map((event) => (
                          <div key={event.id} className="relative pl-10">
                            {/* Timeline dot */}
                            <div className={`absolute left-2.5 w-3 h-3 rounded-full border-2 bg-background ${
                              event.type === 'alert' ? 'border-red-500' :
                              event.icon === 'trust_down' ? 'border-orange-500' :
                              event.icon === 'trust_up' ? 'border-green-500' :
                              event.type === 'verification' ? 'border-blue-500' :
                              'border-gray-400'
                            }`} />

                            {/* Event card */}
                            <div className={`p-3 rounded-lg border ${
                              event.type === 'alert' ? 'bg-red-50 dark:bg-red-950/20 border-red-200 dark:border-red-900' :
                              event.icon === 'trust_down' ? 'bg-orange-50 dark:bg-orange-950/20 border-orange-200 dark:border-orange-900' :
                              event.icon === 'trust_up' ? 'bg-green-50 dark:bg-green-950/20 border-green-200 dark:border-green-900' :
                              'bg-card'
                            }`}>
                              <div className="flex items-start justify-between gap-4">
                                <div className="flex items-start gap-3">
                                  {/* Icon */}
                                  <div className={`mt-0.5 ${
                                    event.type === 'alert' ? 'text-red-500' :
                                    event.icon === 'trust_down' ? 'text-orange-500' :
                                    event.icon === 'trust_up' ? 'text-green-500' :
                                    event.type === 'verification' ? 'text-blue-500' :
                                    'text-gray-500'
                                  }`}>
                                    {event.type === 'alert' && <AlertTriangle className="h-4 w-4" />}
                                    {event.icon === 'trust_down' && <TrendingDown className="h-4 w-4" />}
                                    {event.icon === 'trust_up' && event.type === 'trust_change' && <TrendingUp className="h-4 w-4" />}
                                    {event.type === 'verification' && <Shield className="h-4 w-4" />}
                                    {event.type === 'action' && <Bot className="h-4 w-4" />}
                                  </div>
                                  <div>
                                    <div className="font-medium text-sm">{event.title}</div>
                                    <div className="text-sm text-muted-foreground">{event.description}</div>
                                    <div className="text-xs text-muted-foreground mt-1">
                                      {event.timestamp.toLocaleString()}
                                    </div>
                                  </div>
                                </div>
                                {event.badge && (
                                  <Badge variant={event.badge.variant} className="text-xs whitespace-nowrap">
                                    {event.badge.text}
                                  </Badge>
                                )}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    {timelineEvents.length > 0 && (
                      <div className="flex items-center justify-between pt-4 border-t">
                        <div className="text-sm text-muted-foreground">
                          Showing {Math.min(activityPage * ACTIVITY_PAGE_SIZE, timelineEvents.length)} of {timelineEvents.length} events
                        </div>
                        <div className="flex items-center gap-2">
                          {activityPage > 1 && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setActivityPage(1)}
                            >
                              Show Less
                            </Button>
                          )}
                          {activityPage * ACTIVITY_PAGE_SIZE < timelineEvents.length && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setActivityPage(activityPage + 1)}
                            >
                              Load More ({Math.min(ACTIVITY_PAGE_SIZE, timelineEvents.length - activityPage * ACTIVITY_PAGE_SIZE)} more)
                            </Button>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                );
              })()}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="trust">
          <TrustScoreBreakdown
            agentId={agent.id}
            userRole={userRole}
            onTrustScoreUpdate={(newScore) => {
              // Update the agent's displayed trust score for consistency
              setAgent(prev => prev ? { ...prev, trustScore: newScore } : null);
            }}
          />
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
                  <span className="col-span-2 text-sm">{agent.agentType}</span>
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
                      className={getTrustColor((agent.trustScore ?? 0) * 100)}
                    >
                      {((agent.trustScore ?? 0) * 100).toFixed(1)}%
                    </Badge>
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Created:
                  </span>
                  <span className="col-span-2 text-sm">
                    {new Date(agent.createdAt).toLocaleString()}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Created By:
                  </span>
                  <span className="col-span-2 text-sm">
                    {agent.createdByName || agent.createdByEmail ? (
                      <div className="flex flex-col gap-1">
                        {agent.createdByName && (
                          <span className="font-medium">{agent.createdByName}</span>
                        )}
                        {agent.createdByEmail && (
                          <span className="text-muted-foreground">{agent.createdByEmail}</span>
                        )}
                        {agent.createdBySdkTokenId && (
                          <Button
                            variant="link"
                            size="sm"
                            className="p-0 h-auto text-xs text-primary justify-start"
                            onClick={() => router.push(`/dashboard/sdk-tokens?highlight=${agent.createdBySdkTokenId}`)}
                          >
                            <KeyRound className="h-3 w-3 mr-1" />
                            View SDK Token
                          </Button>
                        )}
                        {agent.createdByApiKeyId && (
                          <Button
                            variant="link"
                            size="sm"
                            className="p-0 h-auto text-xs text-orange-600 justify-start"
                            onClick={() => router.push(`/dashboard/api-keys?highlight=${agent.createdByApiKeyId}`)}
                          >
                            <KeyRound className="h-3 w-3 mr-1" />
                            View API Key
                          </Button>
                        )}
                      </div>
                    ) : (
                      <span className="text-muted-foreground">System</span>
                    )}
                  </span>
                </div>
                {agent.updatedByName && (
                  <>
                    <Separator />
                    <div className="grid grid-cols-3 items-center gap-4">
                      <span className="text-sm font-medium text-muted-foreground">
                        Last Updated By:
                      </span>
                      <span className="col-span-2 text-sm">
                        <div className="flex flex-col gap-1">
                          {agent.updatedByName && (
                            <span className="font-medium">{agent.updatedByName}</span>
                          )}
                          {agent.updatedByEmail && (
                            <span className="text-muted-foreground">{agent.updatedByEmail}</span>
                          )}
                        </div>
                      </span>
                    </div>
                  </>
                )}
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Last Updated:
                  </span>
                  <span className="col-span-2 text-sm">
                    {new Date(agent.updatedAt).toLocaleString()}
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">
                    Organization ID:
                  </span>
                  <span className="col-span-2 text-sm font-mono">
                    {agent.organizationId}
                  </span>
                </div>
                {agent.metadata && Object.keys(agent.metadata).length > 0 && (
                  <>
                    <Separator />
                    <div className="grid grid-cols-3 items-start gap-4">
                      <span className="text-sm font-medium text-muted-foreground">
                        Metadata:
                      </span>
                      <div className="col-span-2">
                        <div className="flex flex-wrap gap-2">
                          {Object.entries(agent.metadata).map(([key, value]) => (
                            <Badge key={key} variant="outline" className="text-xs">
                              <span className="font-medium">{key}:</span>
                              <span className="ml-1">{typeof value === 'object' ? JSON.stringify(value) : String(value)}</span>
                            </Badge>
                          ))}
                        </div>
                      </div>
                    </div>
                  </>
                )}
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
    </div>
    </AuthGuard>
  );
}
