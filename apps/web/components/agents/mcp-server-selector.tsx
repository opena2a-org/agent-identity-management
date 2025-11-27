'use client'

import { useState, useEffect, useMemo } from 'react'
import { Check, ChevronsUpDown, Loader2, Plus, Search, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { api } from '@/lib/api'

interface MCPServer {
  id: string
  name: string
  url: string
  status:
    | "active"
    | "inactive"
    | "pending"
    | "verified"
    | "suspended"
    | "revoked"
  isVerified?: boolean
  description?: string
  lastVerifiedAt?: string
  createdAt: string
}

interface MCPServerSelectorProps {
  agentId: string
  currentMCPServers: string[] // Array of MCP server names currently mapped
  onSelectionComplete?: () => void
  variant?: 'default' | 'outline' | 'ghost'
  size?: 'default' | 'sm' | 'lg'
}

export function MCPServerSelector({
  agentId,
  currentMCPServers,
  onSelectionComplete,
  variant = 'default',
  size = 'default',
}: MCPServerSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [mcpServers, setMCPServers] = useState<MCPServer[]>([])
  const [selectedServers, setSelectedServers] = useState<Set<string>>(new Set())
  const [searchQuery, setSearchQuery] = useState('')
  const [error, setError] = useState<string | null>(null)

  // Filter MCP servers based on search query
  const filteredServers = useMemo(() => {
    if (!searchQuery) return mcpServers

    const query = searchQuery.toLowerCase()
    return mcpServers.filter(
      (server) =>
        server.name.toLowerCase().includes(query) ||
        server.description?.toLowerCase().includes(query) ||
        server.url.toLowerCase().includes(query)
    )
  }, [mcpServers, searchQuery])

  // Separate into already mapped and available servers
  const { mappedServers, availableServers } = useMemo(() => {
    const mapped = filteredServers.filter((server) =>
      currentMCPServers.includes(server.name)
    )
    const available = filteredServers.filter(
      (server) => !currentMCPServers.includes(server.name)
    )
    return { mappedServers: mapped, availableServers: available }
  }, [filteredServers, currentMCPServers])

  // Fetch MCP servers when dialog opens
  useEffect(() => {
    if (isOpen) {
      fetchMCPServers()
    }
  }, [isOpen])

  const fetchMCPServers = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const response = await api.listMCPServers(100, 0) // limit, offset
      setMCPServers(response.mcpServers || [])
    } catch (err: any) {
      console.error('Failed to fetch MCP servers:', err)
      setError('Failed to load MCP servers')
    } finally {
      setIsLoading(false)
    }
  }

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
    const newSelected = new Set<string>()
    availableServers.forEach((server) => {
      newSelected.add(server.name)
    })
    setSelectedServers(newSelected)
  }

  const handleClearSelection = () => {
    setSelectedServers(new Set())
  }

  const handleAddServers = async () => {
    if (selectedServers.size === 0) {
      setError('Please select at least one MCP server')
      return
    }

    setIsSaving(true)
    setError(null)

    try {
      await api.addMCPServersToAgent(agentId, {
        mcpServerIds: Array.from(selectedServers),
      })

      // Close dialog and notify parent
      setIsOpen(false)
      setSelectedServers(new Set())
      setSearchQuery('')

      if (onSelectionComplete) {
        onSelectionComplete()
      }
    } catch (err: any) {
      console.error('Failed to add MCP servers:', err)
      setError(err.message || 'Failed to add MCP servers to agent')
    } finally {
      setIsSaving(false)
    }
  }

  const handleClose = () => {
    setIsOpen(false)
    setSelectedServers(new Set())
    setSearchQuery('')
    setError(null)
  }

  return (
    <>
      <Button
        variant={variant}
        size={size}
        onClick={() => setIsOpen(true)}
        className="gap-2"
      >
        <Plus className="h-4 w-4" />
        Add MCP Servers
      </Button>

      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Plus className="h-5 w-5" />
              Add MCP Servers to Agent
            </DialogTitle>
            <DialogDescription>
              Select MCP servers to add to this agent's talks_to list.
            </DialogDescription>
          </DialogHeader>

          <div className="flex-1 space-y-4 py-4 overflow-hidden flex flex-col">
            {/* Search Bar */}
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search MCP servers..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-9"
                  disabled={isLoading}
                />
              </div>
              {selectedServers.size > 0 && (
                <Badge variant="secondary" className="px-3 py-1">
                  {selectedServers.size} selected
                </Badge>
              )}
            </div>

            {/* Selection Actions */}
            {availableServers.length > 0 && (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleSelectAll}
                  disabled={isLoading}
                >
                  Select All Available
                </Button>
                {selectedServers.size > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleClearSelection}
                    disabled={isLoading}
                  >
                    <X className="h-4 w-4 mr-1" />
                    Clear Selection
                  </Button>
                )}
              </div>
            )}

            {/* Error Display */}
            {error && (
              <div className="p-3 rounded-lg bg-destructive/10 text-destructive text-sm">
                {error}
              </div>
            )}

            {/* Loading State */}
            {isLoading && (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            )}

            {/* Server Lists */}
            {!isLoading && (
              <ScrollArea className="flex-1 pr-4">
                <div className="space-y-4">
                  {/* Currently Mapped Servers */}
                  {mappedServers.length > 0 && (
                    <div>
                      <h4 className="text-sm font-semibold mb-2 text-muted-foreground">
                        Already Mapped ({mappedServers.length})
                      </h4>
                      <div className="space-y-2">
                        {mappedServers.map((server) => (
                          <div
                            key={server.id}
                            className="p-3 rounded-lg border bg-muted/50 opacity-60"
                          >
                            <div className="flex items-start gap-3">
                              <div className="flex h-5 items-center">
                                <Check className="h-4 w-4 text-muted-foreground" />
                              </div>
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 mb-1">
                                  <h5 className="font-semibold text-sm truncate">
                                    {server.name}
                                  </h5>
                                  <Badge variant="outline" className="text-xs">
                                    Mapped
                                  </Badge>
                                </div>
                                <p className="text-xs text-muted-foreground line-clamp-2">
                                  {server.description || 'No description'}
                                </p>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Available Servers */}
                  {availableServers.length > 0 && (
                    <div>
                      <h4 className="text-sm font-semibold mb-2">
                        Available Servers ({availableServers.length})
                      </h4>
                      <div className="space-y-2">
                        {availableServers.map((server) => {
                          const isSelected = selectedServers.has(server.name)
                          return (
                            <div
                              key={server.id}
                              onClick={() => handleToggleServer(server.name)}
                              className={`p-3 rounded-lg border cursor-pointer transition-all hover:border-primary ${
                                isSelected ? 'bg-primary/5 border-primary' : 'bg-card'
                              }`}
                            >
                              <div className="flex items-start gap-3">
                                <div className="flex h-5 items-center">
                                  <Checkbox
                                    checked={isSelected}
                                    onCheckedChange={() => handleToggleServer(server.name)}
                                  />
                                </div>
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center gap-2 mb-1">
                                    <h5 className="font-semibold text-sm truncate">
                                      {server.name}
                                    </h5>
                                    {server.status === 'active' ? (
                                      <Badge
                                        variant="secondary"
                                        className="text-xs bg-green-500/10 text-green-600"
                                      >
                                        Active
                                      </Badge>
                                    ) : (
                                      <Badge variant="secondary" className="text-xs">
                                        {server.status.charAt(0).toUpperCase() + server.status.slice(1)}
                                      </Badge>
                                    )}
                                  </div>
                                  <p className="text-xs text-muted-foreground line-clamp-2 mb-2">
                                    {server.description || 'No description'}
                                  </p>
                                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                                    <span>
                                      <span className="font-medium">URL:</span>{' '}
                                      <span className="truncate">{server.url}</span>
                                    </span>
                                    <span>
                                      <span className="font-medium">Status:</span>{' '}
                                      {server.status}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  )}

                  {/* No Servers Found */}
                  {!isLoading &&
                    availableServers.length === 0 &&
                    mappedServers.length === 0 && (
                      <div className="text-center py-8 text-muted-foreground">
                        <p className="text-sm">
                          {searchQuery
                            ? 'No MCP servers match your search'
                            : 'No MCP servers available'}
                        </p>
                      </div>
                    )}
                </div>
              </ScrollArea>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={handleClose} disabled={isSaving}>
              Cancel
            </Button>
            <Button
              onClick={handleAddServers}
              disabled={isSaving || selectedServers.size === 0}
            >
              {isSaving ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Adding...
                </>
              ) : (
                <>
                  <Plus className="mr-2 h-4 w-4" />
                  Add {selectedServers.size > 0 && `(${selectedServers.size})`}
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
