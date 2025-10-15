"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Bot,
  Shield,
  AlertTriangle,
  ExternalLink,
  Network,
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
import { AgentMCPGraph } from "@/components/agents/agent-mcp-graph";
import { DetectionStatus } from "@/components/agents/detection-status";
import { SDKSetupGuide } from "@/components/agents/sdk-setup-guide";
import { AgentCapabilities } from "@/components/agents/agent-capabilities";
import { api } from "@/lib/api";

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
      } catch (err: any) {
        console.error("Failed to fetch agent data:", err);
        setError(err.message || "Failed to load agent details");
      } finally {
        setIsLoading(false);
      }
    }

    fetchData();
  }, [agentId, refreshKey]);

  const handleRefresh = () => {
    setRefreshKey((prev) => prev + 1);
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

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <Bot className="h-12 w-12 mx-auto text-muted-foreground mb-4 animate-pulse" />
          <p className="text-muted-foreground">Loading agent details...</p>
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
              {agent.talks_to?.length ?? 0}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Connected MCP server
              {(agent.talks_to?.length ?? 0) !== 1 ? "s" : ""}
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
            Connections
          </TabsTrigger>
          <TabsTrigger value="capabilities">
            <Shield className="h-4 w-4 mr-2" />
            Capabilities
          </TabsTrigger>
          <TabsTrigger value="graph">
            <Network className="h-4 w-4 mr-2" />
            Graph View
          </TabsTrigger>
          <TabsTrigger value="detection">
            <Bot className="h-4 w-4 mr-2" />
            Detection
          </TabsTrigger>
          <TabsTrigger value="sdk">
            <Shield className="h-4 w-4 mr-2" />
            SDK Setup
          </TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>

        <TabsContent value="connections" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>MCP Server Connections</CardTitle>
              <CardDescription>
                Manage which MCP servers this agent can communicate with. Use
                the buttons above to auto-detect from Claude Desktop config or
                manually add servers.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <MCPServerList
                agentId={agent.id}
                mcpServers={agent.talks_to ?? []}
                serverNameToId={serverNameToId}
                onUpdate={handleRefresh}
                showBulkActions={true}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="capabilities">
          <AgentCapabilities
            agentId={agent.id}
            agentCapabilities={agent.capabilities}
          />
        </TabsContent>

        <TabsContent value="graph">
          <AgentMCPGraph
            agents={allAgents.map((a) => ({
              id: a.id,
              name: a.name,
              type: a.agent_type,
              isVerified: a.status === "verified",
              trustScore: (a.trust_score ?? 0) * 100,
              talksTo: a.talks_to ?? [],
            }))}
            mcpServers={allMCPServers.map((m) => ({
              id: m.id,
              name: m.name,
              isActive: m.isActive ?? m.status === "active",
              trustScore: m.trustScore ?? 0,
            }))}
            highlightAgentId={agent.id}
          />
        </TabsContent>

        <TabsContent value="detection">
          <DetectionStatus agentId={agent.id} />
        </TabsContent>

        <TabsContent value="sdk">
          <SDKSetupGuide
            agentId={agent.id}
            agentName={agent.name}
            agentType={agent.agent_type}
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
    </div>
  );
}
