'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { api, type DetectionStatusResponse, type DetectedMCPSummary } from '@/lib/api'
import {
  DetectionMethodBadge,
  DetectionMethodsBadges,
  ConfidenceBadge
} from './detection-method-badge'
import {
  CheckCircle2,
  XCircle,
  Loader2,
  Clock,
  Download,
  AlertCircle,
  Activity
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'

interface DetectionStatusProps {
  agentId: string
}

export function DetectionStatus({ agentId }: DetectionStatusProps) {
  const [status, setStatus] = useState<DetectionStatusResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    loadDetectionStatus()
  }, [agentId])

  const loadDetectionStatus = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getDetectionStatus(agentId)
      setStatus(data)
    } catch (err: any) {
      setError(err.message || 'Failed to load detection status')
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Detection Status
          </CardTitle>
          <CardDescription>
            SDK integration and MCP auto-detection status
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Detection Status
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  if (!status) return null

  return (
    <div className="space-y-6">
      {/* Protocol Detection */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Protocol Detection
          </CardTitle>
          <CardDescription>
            Communication protocol used by this agent (auto-detected by SDK)
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Detected Protocol</p>
              <p className="text-2xl font-semibold mt-1">
                {status.protocol || 'MCP'}
              </p>
            </div>
            <Badge variant="secondary" className="text-xs">
              {status.protocol ? 'SDK Detected' : 'Default'}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground mt-4">
            Protocol is automatically detected by the AIM SDK based on runtime context.
            Users can also explicitly declare protocol in secure() wrapper.
          </p>
        </CardContent>
      </Card>

      {/* SDK Installation Status */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="h-5 w-5" />
              SDK Integration Status
            </div>
            {status.sdkInstalled ? (
              <Badge variant="success" className="gap-1">
                <CheckCircle2 className="h-3 w-3" />
                Installed
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1">
                <XCircle className="h-3 w-3" />
                Not Installed
              </Badge>
            )}
          </CardTitle>
          <CardDescription>
            Auto-detection capabilities via AIM SDK
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {status.sdkInstalled ? (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm text-muted-foreground">SDK Version</p>
                  <p className="text-sm font-mono font-medium">
                    {status.sdkVersion || 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Auto-Detection</p>
                  <p className="text-sm font-medium">
                    {status.autoDetectEnabled ? (
                      <span className="text-green-600 dark:text-green-400">Enabled</span>
                    ) : (
                      <span className="text-gray-600 dark:text-gray-400">Disabled</span>
                    )}
                  </p>
                </div>
              </div>
              {status.lastReportedAt && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Clock className="h-4 w-4" />
                  <span>
                    Last reported:{' '}
                    {formatDistanceToNow(new Date(status.lastReportedAt), {
                      addSuffix: true,
                    })}
                  </span>
                </div>
              )}
            </>
          ) : (
            <Alert>
              <Download className="h-4 w-4" />
              <AlertDescription>
                Install the AIM SDK to enable automatic MCP server detection.
                The SDK will monitor your agent's runtime and report detected MCP servers.
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      {/* Detected MCPs */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <div>Detected MCP Servers</div>
            <Badge variant="secondary">
              {status.detectedMCPs.length} detected
            </Badge>
          </CardTitle>
          <CardDescription>
            MCP servers identified through various detection methods
          </CardDescription>
        </CardHeader>
        <CardContent>
          {status.detectedMCPs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-8 text-center">
              <AlertCircle className="h-12 w-12 text-muted-foreground opacity-50 mb-4" />
              <p className="text-sm text-muted-foreground">
                No MCP servers detected yet
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Install the SDK or manually register MCP servers
              </p>
            </div>
          ) : (
            <DetectedMCPsTable detections={status.detectedMCPs} />
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function DetectedMCPsTable({ detections }: { detections: DetectedMCPSummary[] }) {
  // Helper to check if timestamp is valid
  const isValidTimestamp = (timestamp: string | null | undefined): boolean => {
    if (!timestamp) return false
    const date = new Date(timestamp)
    return !isNaN(date.getTime()) && date.getFullYear() > 1970
  }

  // Helper to check if server was manually added
  const isManualServer = (detection: DetectedMCPSummary): boolean => {
    return detection.detectedBy.includes('manual' as any)
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b">
            <th className="text-left py-3 px-2 text-sm font-medium text-muted-foreground">
              MCP Server
            </th>
            <th className="text-left py-3 px-2 text-sm font-medium text-muted-foreground">
              Confidence
            </th>
            <th className="text-left py-3 px-2 text-sm font-medium text-muted-foreground">
              Detection Methods
            </th>
            <th className="text-left py-3 px-2 text-sm font-medium text-muted-foreground">
              First / Last Seen
            </th>
          </tr>
        </thead>
        <tbody>
          {detections.map((detection) => {
            const isManual = isManualServer(detection)
            const hasValidFirstDetected = isValidTimestamp(detection.firstDetected)
            const hasValidLastSeen = isValidTimestamp(detection.lastSeen)

            return (
              <tr key={detection.name} className="border-b last:border-0">
                <td className="py-3 px-2">
                  <p className="text-sm font-mono font-medium">{detection.name}</p>
                </td>
                <td className="py-3 px-2">
                  {isManual ? (
                    <Badge variant="secondary" className="gap-1">
                      <span className="text-xs">Manual</span>
                    </Badge>
                  ) : (
                    <ConfidenceBadge score={detection.confidenceScore} />
                  )}
                </td>
                <td className="py-3 px-2">
                  <DetectionMethodsBadges methods={detection.detectedBy} maxDisplay={2} />
                </td>
                <td className="py-3 px-2">
                  {isManual ? (
                    <div className="text-xs text-muted-foreground">
                      <span className="font-medium">Manually added</span>
                    </div>
                  ) : (
                    <div className="text-xs text-muted-foreground space-y-1">
                      <div className="flex items-center gap-1">
                        <span>First:</span>
                        <span className="font-medium">
                          {hasValidFirstDetected
                            ? formatDistanceToNow(new Date(detection.firstDetected), {
                                addSuffix: true,
                              })
                            : 'N/A'}
                        </span>
                      </div>
                      <div className="flex items-center gap-1">
                        <span>Last:</span>
                        <span className="font-medium">
                          {hasValidLastSeen
                            ? formatDistanceToNow(new Date(detection.lastSeen), {
                                addSuffix: true,
                              })
                            : 'N/A'}
                        </span>
                      </div>
                    </div>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
