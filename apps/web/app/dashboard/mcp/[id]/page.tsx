'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Server, Shield, AlertTriangle, ExternalLink, Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { api } from '@/lib/api'

interface MCPServer {
  id: string
  name: string
  url: string
  description?: string
  status: 'active' | 'inactive' | 'verified' | 'pending'
  public_key?: string
  key_type?: string
  last_verified_at?: string
  created_at: string
  updated_at?: string
  trust_score?: number
  capability_count?: number
  organization_id: string
}

interface Capability {
  id: string
  mcp_server_id: string
  name: string
  type: 'tool' | 'resource' | 'prompt'
  description: string
  schema: any
  detected_at: string
  last_verified_at?: string
  is_active: boolean
}

interface Agent {
  id: string
  name: string
  display_name: string
  agent_type: string
}

export default function MCPServerDetailsPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter()
  const [serverId, setServerId] = useState<string | null>(null)
  const [server, setServer] = useState<MCPServer | null>(null)
  const [capabilities, setCapabilities] = useState<Capability[]>([])
  const [connectedAgents, setConnectedAgents] = useState<Agent[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)

  // Extract server ID from params Promise
  useEffect(() => {
    params.then(({ id }) => setServerId(id))
  }, [params])

  // Fetch server data
  useEffect(() => {
    if (!serverId) return

    async function fetchData() {
      setIsLoading(true)
      setError(null)

      try {
        // Fetch MCP server details
        const serverData = await api.getMCPServer(serverId!)
        setServer(serverData)

        // Fetch capabilities
        try {
          const capabilitiesData = await api.getMCPServerCapabilities(serverId!)
          setCapabilities(capabilitiesData.capabilities || [])
        } catch (err) {
          console.error('Failed to fetch capabilities:', err)
        }

        // Fetch connected agents
        try {
          const agentsData = await api.getMCPServerAgents(serverId!)
          setConnectedAgents(agentsData.agents || [])
        } catch (err) {
          console.error('Failed to fetch connected agents:', err)
        }
      } catch (err: any) {
        console.error('Failed to fetch MCP server data:', err)
        setError(err.message || 'Failed to load MCP server details')
      } finally {
        setIsLoading(false)
      }
    }

    fetchData()
  }, [serverId, refreshKey])

  const handleRefresh = () => {
    setRefreshKey((prev) => prev + 1)
  }

  // Get trust score color
  const getTrustColor = (score: number): string => {
    if (score >= 80) return 'text-green-600 bg-green-500/10'
    if (score >= 60) return 'text-yellow-600 bg-yellow-500/10'
    return 'text-red-600 bg-red-500/10'
  }

  // Get status color
  const getStatusColor = (status: string): string => {
    switch (status) {
      case 'active':
      case 'verified':
        return 'bg-green-500/10 text-green-600'
      case 'pending':
        return 'bg-yellow-500/10 text-yellow-600'
      case 'inactive':
        return 'bg-gray-500/10 text-gray-600'
      default:
        return 'bg-gray-500/10 text-gray-600'
    }
  }

  // Check if server is verified
  const isVerified = server?.status === 'verified' || server?.status === 'active'

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <Server className="h-12 w-12 mx-auto text-muted-foreground mb-4 animate-pulse" />
          <p className="text-muted-foreground">Loading MCP server details...</p>
        </div>
      </div>
    )
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
              {error || 'MCP server not found or you do not have permission to view it.'}
            </p>
            <Button variant="outline" onClick={() => router.push('/dashboard/mcp')}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to MCP Servers
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => router.push('/dashboard/mcp')}
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
                <p className="text-muted-foreground mb-2">{server.description}</p>
              )}
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="outline" className="flex items-center gap-1">
                  <Globe className="h-3 w-3" />
                  <a href={server.url} target="_blank" rel="noopener noreferrer" className="hover:underline">
                    {server.url}
                  </a>
                </Badge>
                <Badge className={getStatusColor(server.status)}>
                  {server.status.charAt(0).toUpperCase() + server.status.slice(1)}
                </Badge>
                <Badge className={getTrustColor((server.trust_score ?? 0) * 100)}>
                  Trust: {((server.trust_score ?? 0) * 100).toFixed(1)}%
                </Badge>
              </div>
            </div>
          </div>
        </div>
      </div>

      <Separator />

      {/* Info Cards */}
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
              Agent{connectedAgents.length !== 1 ? 's' : ''} using this server
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
              Tool{capabilities.length !== 1 ? 's' : ''} and resource{capabilities.length !== 1 ? 's' : ''}
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
            <div className={`text-2xl font-bold ${getTrustColor((server.trust_score ?? 0) * 100).split(' ')[0]}`}>
              {((server.trust_score ?? 0) * 100).toFixed(1)}%
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {((server.trust_score ?? 0) * 100) >= 80
                ? 'High trust'
                : ((server.trust_score ?? 0) * 100) >= 60
                ? 'Medium trust'
                : 'Low trust'}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="capabilities" className="space-y-4">
        <TabsList>
          <TabsTrigger value="capabilities">
            <ExternalLink className="h-4 w-4 mr-2" />
            Capabilities
          </TabsTrigger>
          <TabsTrigger value="agents">
            Connected Agents
          </TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>

        <TabsContent value="capabilities" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>MCP Server Capabilities</CardTitle>
              <CardDescription>
                Tools, resources, and prompts provided by this MCP server
              </CardDescription>
            </CardHeader>
            <CardContent>
              {capabilities.length === 0 ? (
                <div className="text-center py-8">
                  <p className="text-muted-foreground">No capabilities detected yet</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {capabilities.map((capability) => (
                    <div
                      key={capability.id}
                      className="flex items-start gap-3 p-3 border rounded-lg hover:bg-accent/50 transition-colors"
                    >
                      <Badge variant="outline" className="mt-1">
                        {capability.type}
                      </Badge>
                      <div className="flex-1">
                        <h4 className="font-medium">{capability.name}</h4>
                        <p className="text-sm text-muted-foreground">{capability.description}</p>
                        <p className="text-xs text-muted-foreground mt-1">
                          Detected: {new Date(capability.detected_at).toLocaleString()}
                        </p>
                      </div>
                      <Badge variant={capability.is_active ? 'default' : 'secondary'}>
                        {capability.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </div>
                  ))}
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
                  <p className="text-muted-foreground">No agents connected yet</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {connectedAgents.map((agent) => (
                    <div
                      key={agent.id}
                      className="flex items-center gap-3 p-3 border rounded-lg hover:bg-accent/50 transition-colors cursor-pointer"
                      onClick={() => router.push(`/dashboard/agents/${agent.id}`)}
                    >
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-500/10">
                        <Server className="h-5 w-5 text-blue-600" />
                      </div>
                      <div className="flex-1">
                        <h4 className="font-medium">{agent.display_name || agent.name}</h4>
                        <p className="text-sm text-muted-foreground">{agent.agent_type}</p>
                      </div>
                      <ExternalLink className="h-4 w-4 text-muted-foreground" />
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="details" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>MCP Server Details</CardTitle>
              <CardDescription>Detailed information about this MCP server</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4">
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">Server ID:</span>
                  <span className="col-span-2 text-sm font-mono">{server.id}</span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">Name:</span>
                  <span className="col-span-2 text-sm">{server.name}</span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">URL:</span>
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
                      <span className="text-sm font-medium text-muted-foreground">Description:</span>
                      <span className="col-span-2 text-sm">{server.description}</span>
                    </div>
                    <Separator />
                  </>
                )}
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">Status:</span>
                  <span className="col-span-2 text-sm">
                    <Badge className={getStatusColor(server.status)}>
                      {server.status.charAt(0).toUpperCase() + server.status.slice(1)}
                    </Badge>
                  </span>
                </div>
                <Separator />
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">Trust Score:</span>
                  <span className="col-span-2 text-sm">
                    <Badge className={getTrustColor((server.trust_score ?? 0) * 100)}>
                      {((server.trust_score ?? 0) * 100).toFixed(1)}%
                    </Badge>
                  </span>
                </div>
                <Separator />
                {server.key_type && (
                  <>
                    <div className="grid grid-cols-3 items-center gap-4">
                      <span className="text-sm font-medium text-muted-foreground">Key Type:</span>
                      <span className="col-span-2 text-sm">{server.key_type}</span>
                    </div>
                    <Separator />
                  </>
                )}
                {server.last_verified_at && (
                  <>
                    <div className="grid grid-cols-3 items-center gap-4">
                      <span className="text-sm font-medium text-muted-foreground">Last Verified:</span>
                      <span className="col-span-2 text-sm">
                        {new Date(server.last_verified_at).toLocaleString()}
                      </span>
                    </div>
                    <Separator />
                  </>
                )}
                <div className="grid grid-cols-3 items-center gap-4">
                  <span className="text-sm font-medium text-muted-foreground">Created:</span>
                  <span className="col-span-2 text-sm">
                    {new Date(server.created_at).toLocaleString()}
                  </span>
                </div>
                {server.updated_at && (
                  <>
                    <Separator />
                    <div className="grid grid-cols-3 items-center gap-4">
                      <span className="text-sm font-medium text-muted-foreground">Last Updated:</span>
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
                  <span className="col-span-2 text-sm font-mono">{server.organization_id}</span>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
