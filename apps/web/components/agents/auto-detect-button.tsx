'use client'

import { useState } from 'react'
import { Loader2, Scan, CheckCircle2, XCircle, AlertTriangle } from 'lucide-react'
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
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { api } from '@/lib/api'

interface AutoDetectButtonProps {
  agentId: string
  onDetectionComplete?: () => void
  variant?: 'default' | 'outline' | 'ghost'
  size?: 'default' | 'sm' | 'lg'
}

interface DetectionResult {
  detected_servers: Array<{
    name: string
    command: string
    args: string[]
    env?: Record<string, string>
    confidence: number
    source: string
    metadata?: Record<string, any>
  }>
  registered_count: number
  mapped_count: number
  total_talks_to: number
  dry_run: boolean
  errors_encountered?: string[]
}

// Helper to get default config path based on platform
function getClaudeDesktopConfigPath(): string {
  if (typeof window === 'undefined') return ''

  const platform = navigator.platform.toLowerCase()

  if (platform.includes('mac')) {
    return '~/Library/Application Support/Claude/claude_desktop_config.json'
  } else if (platform.includes('win')) {
    return '%APPDATA%/Claude/claude_desktop_config.json'
  } else {
    // Linux
    return '~/.config/Claude/claude_desktop_config.json'
  }
}

export function AutoDetectButton({
  agentId,
  onDetectionComplete,
  variant = 'default',
  size = 'default',
}: AutoDetectButtonProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [configPath, setConfigPath] = useState(getClaudeDesktopConfigPath())
  const [autoRegister, setAutoRegister] = useState(true)
  const [dryRun, setDryRun] = useState(false)
  const [result, setResult] = useState<DetectionResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleDetect = async () => {
    if (!configPath.trim()) {
      setError('Config path is required')
      return
    }

    setIsLoading(true)
    setError(null)
    setResult(null)

    try {
      const detectionResult = await api.detectAndMapMCPServers(agentId, {
        config_path: configPath,
        auto_register: autoRegister,
        dry_run: dryRun,
      })

      setResult(detectionResult)

      // If not a dry run and detection was successful, call onDetectionComplete
      if (!dryRun && detectionResult.detected_servers.length > 0 && onDetectionComplete) {
        onDetectionComplete()
      }
    } catch (err: any) {
      console.error('Auto-detection failed:', err)
      setError(err.message || 'Failed to auto-detect MCP servers. Please check the config path.')
    } finally {
      setIsLoading(false)
    }
  }

  const handleClose = () => {
    setIsOpen(false)
    setResult(null)
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
        <Scan className="h-4 w-4" />
        Auto-Detect MCPs
      </Button>

      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Scan className="h-5 w-5" />
              Auto-Detect MCP Servers
            </DialogTitle>
            <DialogDescription>
              Automatically detect MCP servers from your Claude Desktop configuration file.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            {/* Config Path Input */}
            <div className="space-y-2">
              <Label htmlFor="config-path">Claude Desktop Config Path</Label>
              <Input
                id="config-path"
                value={configPath}
                onChange={(e) => setConfigPath(e.target.value)}
                placeholder="~/Library/Application Support/Claude/claude_desktop_config.json"
                disabled={isLoading}
              />
              <p className="text-xs text-muted-foreground">
                Default path has been auto-detected for your platform.
              </p>
            </div>

            {/* Options */}
            <div className="space-y-3 pt-2">
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="auto-register"
                  checked={autoRegister}
                  onCheckedChange={(checked) => setAutoRegister(checked as boolean)}
                  disabled={isLoading}
                />
                <Label
                  htmlFor="auto-register"
                  className="text-sm font-normal cursor-pointer"
                >
                  Auto-register new MCP servers
                </Label>
              </div>

              <div className="flex items-center space-x-2">
                <Checkbox
                  id="dry-run"
                  checked={dryRun}
                  onCheckedChange={(checked) => setDryRun(checked as boolean)}
                  disabled={isLoading}
                />
                <Label
                  htmlFor="dry-run"
                  className="text-sm font-normal cursor-pointer"
                >
                  Dry run (preview without applying changes)
                </Label>
              </div>
            </div>

            {/* Error Display */}
            {error && (
              <div className="flex items-start gap-2 p-3 rounded-lg bg-destructive/10 text-destructive">
                <XCircle className="h-5 w-5 mt-0.5 flex-shrink-0" />
                <div className="text-sm">{error}</div>
              </div>
            )}

            {/* Results Display */}
            {result && (
              <div className="space-y-4 pt-2">
                {/* Summary Stats */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  <div className="p-3 rounded-lg bg-muted">
                    <div className="text-2xl font-bold text-foreground">
                      {result.detected_servers.length}
                    </div>
                    <div className="text-xs text-muted-foreground">Detected</div>
                  </div>
                  <div className="p-3 rounded-lg bg-muted">
                    <div className="text-2xl font-bold text-green-600">
                      {result.registered_count}
                    </div>
                    <div className="text-xs text-muted-foreground">Registered</div>
                  </div>
                  <div className="p-3 rounded-lg bg-muted">
                    <div className="text-2xl font-bold text-blue-600">
                      {result.mapped_count}
                    </div>
                    <div className="text-xs text-muted-foreground">Mapped</div>
                  </div>
                  <div className="p-3 rounded-lg bg-muted">
                    <div className="text-2xl font-bold text-purple-600">
                      {result.total_talks_to}
                    </div>
                    <div className="text-xs text-muted-foreground">Total MCPs</div>
                  </div>
                </div>

                {/* Dry Run Notice */}
                {result.dry_run && (
                  <div className="flex items-start gap-2 p-3 rounded-lg bg-blue-500/10 text-blue-600">
                    <AlertTriangle className="h-5 w-5 mt-0.5 flex-shrink-0" />
                    <div className="text-sm">
                      <strong>Dry Run Mode:</strong> No changes were applied. Uncheck "Dry run" and
                      detect again to apply changes.
                    </div>
                  </div>
                )}

                {/* Success Message */}
                {!result.dry_run && result.detected_servers.length > 0 && (
                  <div className="flex items-start gap-2 p-3 rounded-lg bg-green-500/10 text-green-600">
                    <CheckCircle2 className="h-5 w-5 mt-0.5 flex-shrink-0" />
                    <div className="text-sm">
                      Successfully detected and mapped {result.mapped_count} MCP server(s) to this
                      agent!
                    </div>
                  </div>
                )}

                {/* Detected Servers List */}
                {result.detected_servers.length > 0 && (
                  <div className="space-y-2">
                    <h4 className="text-sm font-semibold">Detected MCP Servers:</h4>
                    <div className="space-y-2">
                      {result.detected_servers.map((server, index) => (
                        <div
                          key={index}
                          className="p-3 rounded-lg border bg-card hover:bg-accent/5 transition-colors"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2 mb-1">
                                <h5 className="font-semibold text-sm truncate">{server.name}</h5>
                                <Badge variant="secondary" className="text-xs">
                                  {server.confidence}% confidence
                                </Badge>
                              </div>
                              <div className="text-xs text-muted-foreground space-y-1">
                                <div>
                                  <span className="font-medium">Command:</span> {server.command}
                                </div>
                                {server.args && server.args.length > 0 && (
                                  <div>
                                    <span className="font-medium">Args:</span>{' '}
                                    {server.args.join(' ')}
                                  </div>
                                )}
                                <div>
                                  <span className="font-medium">Source:</span> {server.source}
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Errors */}
                {result.errors_encountered && result.errors_encountered.length > 0 && (
                  <div className="space-y-2">
                    <h4 className="text-sm font-semibold text-orange-600">Warnings:</h4>
                    <div className="space-y-1">
                      {result.errors_encountered.map((err, index) => (
                        <div
                          key={index}
                          className="text-xs text-orange-600 p-2 rounded bg-orange-500/10"
                        >
                          {err}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* No Servers Found */}
                {result.detected_servers.length === 0 && (
                  <div className="flex items-start gap-2 p-3 rounded-lg bg-muted">
                    <AlertTriangle className="h-5 w-5 mt-0.5 flex-shrink-0" />
                    <div className="text-sm text-muted-foreground">
                      No MCP servers found in the configuration file. Make sure the config path is
                      correct and that you have MCP servers configured in Claude Desktop.
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          <DialogFooter>
            {result && !result.dry_run ? (
              <Button onClick={handleClose}>Done</Button>
            ) : (
              <>
                <Button variant="outline" onClick={handleClose} disabled={isLoading}>
                  Cancel
                </Button>
                <Button onClick={handleDetect} disabled={isLoading}>
                  {isLoading ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Detecting...
                    </>
                  ) : (
                    <>
                      <Scan className="mr-2 h-4 w-4" />
                      {dryRun ? 'Preview Detection' : 'Detect & Map'}
                    </>
                  )}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
