'use client'

import { useMemo } from 'react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Bot, Network, ArrowRight, Shield } from 'lucide-react'

interface Agent {
  id: string
  name: string
  type: string
  isVerified: boolean
  trustScore: number
  talksTo: string[]
}

interface MCPServer {
  id: string
  name: string
  isActive: boolean
  trustScore: number
}

interface AgentMCPGraphProps {
  agents: Agent[]
  mcpServers: MCPServer[]
  highlightAgentId?: string
}

export function AgentMCPGraph({ agents, mcpServers, highlightAgentId }: AgentMCPGraphProps) {
  // Build relationship map
  const relationships = useMemo(() => {
    const map: Array<{
      agent: Agent
      connectedServers: MCPServer[]
    }> = []

    agents.forEach((agent) => {
      const connectedServers = agent.talksTo
        .map((serverName) => mcpServers.find((s) => s.name === serverName))
        .filter((s): s is MCPServer => s !== undefined)

      map.push({ agent, connectedServers })
    })

    return map
  }, [agents, mcpServers])

  // Calculate statistics
  const stats = useMemo(() => {
    const totalAgents = agents.length
    const totalMCPServers = mcpServers.length
    const totalConnections = agents.reduce((sum, agent) => sum + agent.talksTo.length, 0)
    const avgConnectionsPerAgent =
      totalAgents > 0 ? (totalConnections / totalAgents).toFixed(1) : '0'

    return {
      totalAgents,
      totalMCPServers,
      totalConnections,
      avgConnectionsPerAgent,
    }
  }, [agents, mcpServers])

  // Get trust color
  const getTrustColor = (score: number): string => {
    if (score >= 80) return 'text-green-600 bg-green-500/10'
    if (score >= 60) return 'text-yellow-600 bg-yellow-500/10'
    return 'text-red-600 bg-red-500/10'
  }

  if (relationships.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Agent-MCP Relationship Graph
          </CardTitle>
          <CardDescription>
            Visual representation of agent and MCP server relationships
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12">
            <Network className="h-16 w-16 mx-auto text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No Relationships Yet</h3>
            <p className="text-sm text-muted-foreground max-w-md mx-auto">
              Connect agents to MCP servers to see the relationship graph here.
            </p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Network className="h-5 w-5" />
          Agent-MCP Relationship Graph
        </CardTitle>
        <CardDescription>
          Visual representation of {stats.totalAgents} agent{stats.totalAgents !== 1 ? 's' : ''}{' '}
          and {stats.totalMCPServers} MCP server{stats.totalMCPServers !== 1 ? 's' : ''} with{' '}
          {stats.totalConnections} connection{stats.totalConnections !== 1 ? 's' : ''}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {/* Statistics */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div className="p-3 rounded-lg bg-muted">
            <div className="text-2xl font-bold text-foreground">{stats.totalAgents}</div>
            <div className="text-xs text-muted-foreground">Agents</div>
          </div>
          <div className="p-3 rounded-lg bg-muted">
            <div className="text-2xl font-bold text-foreground">{stats.totalMCPServers}</div>
            <div className="text-xs text-muted-foreground">MCP Servers</div>
          </div>
          <div className="p-3 rounded-lg bg-muted">
            <div className="text-2xl font-bold text-blue-600">{stats.totalConnections}</div>
            <div className="text-xs text-muted-foreground">Connections</div>
          </div>
          <div className="p-3 rounded-lg bg-muted">
            <div className="text-2xl font-bold text-purple-600">
              {stats.avgConnectionsPerAgent}
            </div>
            <div className="text-xs text-muted-foreground">Avg per Agent</div>
          </div>
        </div>

        {/* Relationship Graph */}
        <div className="space-y-6">
          {relationships.map(({ agent, connectedServers }) => {
            const isHighlighted = highlightAgentId === agent.id
            const hasConnections = connectedServers.length > 0

            return (
              <div
                key={agent.id}
                className={`p-4 rounded-lg border transition-all ${
                  isHighlighted
                    ? 'bg-primary/5 border-primary shadow-lg'
                    : 'bg-card hover:bg-accent/5'
                }`}
              >
                {/* Agent Node */}
                <div className="flex items-start gap-4 mb-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                    <Bot className="h-6 w-6 text-primary" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h4 className="font-semibold truncate">{agent.name}</h4>
                      {agent.isVerified && (
                        <span title="Verified">
                          <Shield className="h-4 w-4 text-green-600" />
                        </span>
                      )}
                      <Badge variant="outline" className="text-xs">
                        {agent.type}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-3 text-xs text-muted-foreground">
                      <span>
                        <span className="font-medium">Trust Score:</span>{' '}
                        <span className={getTrustColor(agent.trustScore).split(' ')[0]}>
                          {agent.trustScore.toFixed(1)}%
                        </span>
                      </span>
                      <span>
                        <span className="font-medium">Connections:</span> {connectedServers.length}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Connections */}
                {hasConnections ? (
                  <div className="pl-16 space-y-2">
                    {connectedServers.map((server, idx) => (
                      <div
                        key={`${agent.id}-${server.id}`}
                        className="flex items-center gap-3 group"
                      >
                        {/* Connection Line */}
                        <div className="flex items-center gap-2 text-muted-foreground">
                          <div className="w-8 border-t-2 border-dashed border-muted-foreground/30 group-hover:border-primary transition-colors" />
                          <ArrowRight className="h-4 w-4 group-hover:text-primary transition-colors" />
                        </div>

                        {/* MCP Server Node */}
                        <div className="flex-1 flex items-center gap-3 p-3 rounded-lg bg-muted/50 group-hover:bg-accent/50 transition-colors">
                          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-background">
                            <Network className="h-4 w-4 text-foreground" />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-0.5">
                              <span className="font-medium text-sm truncate">{server.name}</span>
                              {server.isActive ? (
                                <Badge
                                  variant="secondary"
                                  className="text-xs bg-green-500/10 text-green-600"
                                >
                                  Active
                                </Badge>
                              ) : (
                                <Badge variant="secondary" className="text-xs opacity-50">
                                  Inactive
                                </Badge>
                              )}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              Trust Score:{' '}
                              <span className={getTrustColor(server.trustScore).split(' ')[0]}>
                                {server.trustScore.toFixed(1)}%
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="pl-16 text-sm text-muted-foreground italic">
                    No MCP servers connected
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Legend */}
        <div className="mt-6 pt-6 border-t">
          <h4 className="text-sm font-semibold mb-3">Trust Score Legend</h4>
          <div className="flex flex-wrap gap-4 text-xs">
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-green-500" />
              <span className="text-muted-foreground">High Trust (≥80%)</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-yellow-500" />
              <span className="text-muted-foreground">Medium Trust (60-79%)</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 rounded-full bg-red-500" />
              <span className="text-muted-foreground">Low Trust (&lt;60%)</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
