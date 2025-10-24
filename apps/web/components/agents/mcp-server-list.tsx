'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Trash2, AlertTriangle, Loader2, ExternalLink, CheckCircle2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api'

interface MCPServerDetails {
  name: string
  id: string
  capabilities?: string[]
  url?: string
}

interface MCPServerListProps {
  agentId: string
  mcpServers: string[] // Array of MCP server names (for backward compatibility)
  serverDetails?: MCPServerDetails[] // Optional array of full server details with capabilities
  serverNameToId?: Map<string, string> // Optional mapping from server name to ID for navigation
  onUpdate?: () => void
  showBulkActions?: boolean
}

export function MCPServerList({
  agentId,
  mcpServers,
  serverDetails,
  serverNameToId,
  onUpdate,
  showBulkActions = true,
}: MCPServerListProps) {
  // Create a map from server name to details for easy lookup
  const serverDetailsMap = new Map<string, MCPServerDetails>()
  serverDetails?.forEach((server) => {
    serverDetailsMap.set(server.name, server)
  })
  const router = useRouter()
  const [selectedServers, setSelectedServers] = useState<Set<string>>(new Set())
  const [isRemoving, setIsRemoving] = useState(false)
  const [serverToRemove, setServerToRemove] = useState<string | null>(null)
  const [showBulkRemoveDialog, setShowBulkRemoveDialog] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleToggleServer = (serverName: string) => {
    const newSelected = new Set(selectedServers)
    if (newSelected.has(serverName)) {
      newSelected.delete(serverName)
    } else {
      newSelected.add(serverName)
    }
    setSelectedServers(newSelected)
  }

  const handleSelectAll = () => {
    if (selectedServers.size === mcpServers.length) {
      setSelectedServers(new Set())
    } else {
      setSelectedServers(new Set(mcpServers))
    }
  }

  const handleRemoveSingle = async (serverName: string) => {
    setIsRemoving(true)
    setError(null)

    try {
      await api.removeMCPServerFromAgent(agentId, serverName)
      setServerToRemove(null)

      if (onUpdate) {
        onUpdate()
      }
    } catch (err: any) {
      console.error('Failed to remove MCP server:', err)
      setError(err.message || 'Failed to remove MCP server')
    } finally {
      setIsRemoving(false)
    }
  }

  const handleBulkRemove = async () => {
    if (selectedServers.size === 0) return

    setIsRemoving(true)
    setError(null)

    try {
      // Remove servers one by one since bulk endpoint was removed
      for (const serverId of Array.from(selectedServers)) {
        await api.removeMCPServerFromAgent(agentId, serverId)
      }

      setSelectedServers(new Set())
      setShowBulkRemoveDialog(false)

      if (onUpdate) {
        onUpdate()
      }
    } catch (err: any) {
      console.error('Failed to remove MCP servers:', err)
      setError(err.message || 'Failed to remove MCP servers')
    } finally {
      setIsRemoving(false)
    }
  }

  const handleServerClick = (serverName: string) => {
    const serverId = serverNameToId?.get(serverName)
    if (serverId) {
      router.push(`/dashboard/mcp/${serverId}`)
    }
  }

  if (mcpServers.length === 0) {
    return (
      <div className="text-center py-12 px-4">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-muted mb-4">
          <ExternalLink className="h-8 w-8 text-muted-foreground" />
        </div>
        <h3 className="text-lg font-semibold mb-2">No MCP Servers Connected</h3>
        <p className="text-sm text-muted-foreground max-w-md mx-auto mb-6">
          This agent is not currently connected to any MCP servers. Use the buttons above to add
          MCP servers manually or auto-detect from Claude Desktop config.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Bulk Actions Bar */}
      {showBulkActions && (
        <div className="flex items-center justify-between p-3 rounded-lg bg-muted/50">
          <div className="flex items-center gap-3">
            <Checkbox
              checked={selectedServers.size === mcpServers.length && mcpServers.length > 0}
              onCheckedChange={handleSelectAll}
              aria-label="Select all servers"
            />
            <span className="text-sm text-muted-foreground">
              {selectedServers.size > 0
                ? `${selectedServers.size} server${selectedServers.size > 1 ? 's' : ''} selected`
                : `${mcpServers.length} server${mcpServers.length > 1 ? 's' : ''} total`}
            </span>
          </div>

          {selectedServers.size > 0 && (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setShowBulkRemoveDialog(true)}
              disabled={isRemoving}
            >
              <Trash2 className="h-4 w-4 mr-2" />
              Remove {selectedServers.size > 1 ? `(${selectedServers.size})` : ''}
            </Button>
          )}
        </div>
      )}

      {/* Error Display */}
      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-destructive/10 text-destructive">
          <AlertTriangle className="h-5 w-5 mt-0.5 flex-shrink-0" />
          <div className="text-sm">{error}</div>
        </div>
      )}

      {/* MCP Server Cards */}
      <div className="grid gap-3">
        {mcpServers.map((serverName) => (
          <div
            key={serverName}
            className={`p-4 rounded-lg border transition-all ${
              selectedServers.has(serverName)
                ? 'bg-primary/5 border-primary'
                : 'bg-card hover:bg-accent/5'
            }`}
          >
            <div className="flex items-start gap-3">
              {/* Checkbox */}
              {showBulkActions && (
                <div className="flex h-5 items-center pt-0.5">
                  <Checkbox
                    checked={selectedServers.has(serverName)}
                    onCheckedChange={() => handleToggleServer(serverName)}
                    aria-label={`Select ${serverName}`}
                  />
                </div>
              )}

              {/* Server Info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-2 mb-2">
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <CheckCircle2 className="h-4 w-4 text-green-600 flex-shrink-0" />
                    <h4
                      className={`font-semibold text-sm truncate ${
                        serverNameToId?.has(serverName)
                          ? 'cursor-pointer hover:underline'
                          : ''
                      }`}
                      onClick={() => handleServerClick(serverName)}
                    >
                      {serverName}
                    </h4>
                    {serverNameToId?.has(serverName) && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleServerClick(serverName)}
                        className="h-6 px-2 text-xs"
                      >
                        <ExternalLink className="h-3 w-3" />
                      </Button>
                    )}
                  </div>
                  <Badge variant="secondary" className="text-xs whitespace-nowrap">
                    Connected
                  </Badge>
                </div>

                <p className="text-xs text-muted-foreground mb-2">
                  This agent can communicate with the{' '}
                  <span
                    className={`font-medium ${
                      serverNameToId?.has(serverName)
                        ? 'cursor-pointer hover:underline'
                        : ''
                    }`}
                    onClick={() => handleServerClick(serverName)}
                  >
                    {serverName}
                  </span>{' '}
                  MCP server and access its tools and resources.
                </p>

                {/* Show capabilities if available */}
                {serverDetailsMap.get(serverName)?.capabilities &&
                 serverDetailsMap.get(serverName)!.capabilities!.length > 0 && (
                  <div className="flex gap-1 flex-wrap mt-2">
                    {serverDetailsMap.get(serverName)!.capabilities!.map((cap) => (
                      <Badge key={cap} variant="outline" className="text-xs">
                        {cap}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>

              {/* Remove Button */}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setServerToRemove(serverName)}
                disabled={isRemoving}
                className="flex-shrink-0 text-destructive hover:text-destructive hover:bg-destructive/10"
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          </div>
        ))}
      </div>

      {/* Single Remove Confirmation Dialog */}
      <AlertDialog open={!!serverToRemove} onOpenChange={() => setServerToRemove(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove MCP Server</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to remove <strong>{serverToRemove}</strong> from this agent's
              talks_to list? The agent will no longer be able to communicate with this MCP server.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isRemoving}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => serverToRemove && handleRemoveSingle(serverToRemove)}
              disabled={isRemoving}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isRemoving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Removing...
                </>
              ) : (
                <>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Remove
                </>
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Bulk Remove Confirmation Dialog */}
      <AlertDialog open={showBulkRemoveDialog} onOpenChange={setShowBulkRemoveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove Multiple MCP Servers</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to remove <strong>{selectedServers.size} MCP server
              {selectedServers.size > 1 ? 's' : ''}</strong> from this agent's talks_to list? The
              agent will no longer be able to communicate with these servers.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isRemoving}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleBulkRemove}
              disabled={isRemoving}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isRemoving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Removing...
                </>
              ) : (
                <>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Remove {selectedServers.size > 1 ? `(${selectedServers.size})` : ''}
                </>
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
