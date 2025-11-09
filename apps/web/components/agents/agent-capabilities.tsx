'use client'

import { useState, useEffect } from 'react'
import {
  Shield,
  AlertTriangle,
  AlertCircle,
  CheckCircle,
  XCircle,
  Folder,
  Database,
  Globe,
  Code,
  Key,
  Chrome,
  Cpu,
  Brain,
  FileText,
  TrendingUp,
  TrendingDown,
  Minus,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { api } from '@/lib/api'

interface AgentCapabilitiesProps {
  agentId: string
  agentCapabilities?: string[]  // Basic capability tags from agent
}

interface ProgrammingEnvironment {
  language: string
  version: string
  runtime: string
  platform: string
  arch: string
  frameworks?: string[]
  packageManagers?: string[]
}

interface AIModelUsage {
  provider: string
  models: string[]
  detectionType: string
}

interface FileSystemCapability {
  read: boolean
  write: boolean
  delete: boolean
  execute: boolean
  pathsAccessed?: string[]
  detectionMethod: string
}

interface DatabaseCapability {
  postgresql: boolean
  mongodb: boolean
  mysql: boolean
  sqlite: boolean
  redis: boolean
  operations?: string[]
  detectionMethod: string
}

interface NetworkCapability {
  http: boolean
  https: boolean
  websocket: boolean
  tcp: boolean
  udp: boolean
  externalApis?: string[]
  detectionMethod: string
}

interface CodeExecutionCapability {
  eval: boolean
  exec: boolean
  shellCommands: boolean
  childProcesses: boolean
  vmExecution: boolean
  detectionMethod: string
}

interface CredentialAccessCapability {
  envVars: boolean
  configFiles: boolean
  keyring: boolean
  credentialFiles?: string[]
  detectionMethod: string
}

interface BrowserAutomationCapability {
  puppeteer: boolean
  playwright: boolean
  selenium: boolean
  detectionMethod: string
}

interface SecurityAlert {
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  capability: string
  message: string
  recommendation: string
  trustScoreImpact: number
}

interface RiskAssessment {
  overallRiskScore: number
  riskLevel: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'
  trustScoreImpact: number
  alerts: SecurityAlert[]
}

interface AgentCapabilityReport {
  detectedAt: string
  environment: ProgrammingEnvironment
  aiModels: AIModelUsage[]
  capabilities: {
    fileSystem?: FileSystemCapability
    database?: DatabaseCapability
    network?: NetworkCapability
    codeExecution?: CodeExecutionCapability
    credentialAccess?: CredentialAccessCapability
    browserAutomation?: BrowserAutomationCapability
  }
  riskAssessment: RiskAssessment
}

export function AgentCapabilities({ agentId, agentCapabilities }: AgentCapabilitiesProps) {
  const [capabilityReport, setCapabilityReport] = useState<AgentCapabilityReport | null>(null)
  const [fetchedCapabilities, setFetchedCapabilities] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function fetchCapabilities() {
      setIsLoading(true)
      setError(null)

      try {
        // First, fetch basic capabilities for badges
        const capabilities = await api.getAgentCapabilities(agentId, false)

        console.log('Agent capabilities:', capabilities)

        // Extract capability types from the response
        if (Array.isArray(capabilities)) {
          const capabilityTypes = capabilities.map((cap: any) => cap.capabilityType)
          setFetchedCapabilities(capabilityTypes)
        }

        // Try to fetch latest capability report
        try {
          const report = await api.getLatestCapabilityReport(agentId)
          console.log('Latest capability report:', report)
          setCapabilityReport(report)
        } catch (reportErr: any) {
          // If no report exists yet (404 or "no capability reports found"), that's fine - just show basic view
          if (
            reportErr.message?.includes('404') ||
            reportErr.message?.includes('not found') ||
            reportErr.message?.includes('no capability reports')
          ) {
            console.log('No capability report found yet')
            setCapabilityReport(null)
          } else {
            throw reportErr
          }
        }
      } catch (err: any) {
        console.error('Failed to fetch capabilities:', err)
        setError(err.message || 'Failed to load capability report')
      } finally {
        setIsLoading(false)
      }
    }

    fetchCapabilities()
  }, [agentId])

  // Get risk level color
  const getRiskColor = (level: string): string => {
    switch (level) {
      case 'LOW':
        return 'text-green-600 bg-green-500/10'
      case 'MEDIUM':
        return 'text-yellow-600 bg-yellow-500/10'
      case 'HIGH':
        return 'text-orange-600 bg-orange-500/10'
      case 'CRITICAL':
        return 'text-red-600 bg-red-500/10'
      default:
        return 'text-gray-600 bg-gray-500/10'
    }
  }

  // Get severity icon
  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return <XCircle className="h-5 w-5 text-red-600" />
      case 'HIGH':
        return <AlertTriangle className="h-5 w-5 text-orange-600" />
      case 'MEDIUM':
        return <AlertCircle className="h-5 w-5 text-yellow-600" />
      case 'LOW':
        return <CheckCircle className="h-5 w-5 text-blue-600" />
      default:
        return <AlertCircle className="h-5 w-5 text-gray-600" />
    }
  }

  // Loading state
  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center py-12">
          <div className="text-center">
            <Shield className="h-12 w-12 mx-auto text-muted-foreground mb-4 animate-pulse" />
            <p className="text-muted-foreground">Loading capability report...</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  // No capability report state - but show basic capabilities if available
  if (!capabilityReport) {
    // Merge provided capabilities with fetched capabilities
    const allCapabilities = [...(agentCapabilities || []), ...fetchedCapabilities]
    const uniqueCapabilities = Array.from(new Set(allCapabilities))

    return (
      <div className="space-y-4">
        {/* Show basic capabilities if available */}
        {uniqueCapabilities.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Detected Capabilities</CardTitle>
              <CardDescription>
                Basic capabilities detected by the AIM SDK. For detailed capability analysis, ensure
                the agent is using the latest SDK version with full capability detection enabled.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {uniqueCapabilities.map((capability, idx) => (
                  <Badge
                    key={idx}
                    variant="secondary"
                    className="px-3 py-1 text-sm font-medium"
                  >
                    {capability.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())}
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Show alert only if NO capabilities detected */}
        {uniqueCapabilities.length === 0 && (
          <Alert>
            <Brain className="h-4 w-4" />
            <AlertTitle>No Capabilities Detected</AlertTitle>
            <AlertDescription>
              This agent hasn't reported its capabilities yet. Install the AIM SDK in your agent application to enable automatic capability detection and risk assessment.
            </AlertDescription>
          </Alert>
        )}
      </div>
    )
  }

  const { environment, aiModels, capabilities, riskAssessment } = capabilityReport

  return (
    <div className="space-y-6">
      {/* Risk Assessment Overview */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card className={getRiskColor(riskAssessment.riskLevel)}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium">Risk Level</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{riskAssessment.riskLevel}</div>
            <Progress
              value={riskAssessment.overallRiskScore}
              className="mt-2"
              aria-label={`Risk score: ${riskAssessment.overallRiskScore}%`}
            />
            <p className="text-xs mt-1">{riskAssessment.overallRiskScore}/100</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium">Trust Impact</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {riskAssessment.trustScoreImpact >= 0 ? (
                <TrendingUp className="h-5 w-5 text-green-600" />
              ) : riskAssessment.trustScoreImpact > -10 ? (
                <Minus className="h-5 w-5 text-yellow-600" />
              ) : (
                <TrendingDown className="h-5 w-5 text-red-600" />
              )}
              <div className="text-2xl font-bold">
                {riskAssessment.trustScoreImpact > 0 ? '+' : ''}
                {riskAssessment.trustScoreImpact}
              </div>
            </div>
            <p className="text-xs text-muted-foreground mt-1">Points to trust score</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium">Security Alerts</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{riskAssessment.alerts.length}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {riskAssessment.alerts.filter((a) => a.severity === 'CRITICAL' || a.severity === 'HIGH').length}{' '}
              critical/high
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium">Last Detected</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-sm font-medium">{new Date(capabilityReport.detectedAt).toLocaleDateString()}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {new Date(capabilityReport.detectedAt).toLocaleTimeString()}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Security Alerts */}
      {riskAssessment.alerts.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Security Alerts</CardTitle>
            <CardDescription>Review and address security concerns detected in this agent</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {riskAssessment.alerts.map((alert, idx) => (
              <Alert key={idx} variant={alert.severity === 'CRITICAL' || alert.severity === 'HIGH' ? 'destructive' : 'default'}>
                {getSeverityIcon(alert.severity)}
                <AlertTitle className="flex items-center gap-2">
                  {alert.severity} - {alert.capability}
                  <Badge variant="outline" className="ml-auto">
                    {alert.trustScoreImpact} trust points
                  </Badge>
                </AlertTitle>
                <AlertDescription>
                  <div className="mt-2">{alert.message}</div>
                  <div className="mt-2 text-sm font-medium">Recommendation:</div>
                  <div className="text-sm">{alert.recommendation}</div>
                </AlertDescription>
              </Alert>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Detected Capabilities */}
      <Card>
        <CardHeader>
          <CardTitle>Detected Capabilities</CardTitle>
          <CardDescription>Capabilities automatically detected from agent code and runtime</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {capabilities.fileSystem && (
            <CapabilitySection
              icon={<Folder className="h-5 w-5 text-blue-600" />}
              title="File System"
              capabilities={capabilities.fileSystem}
              fields={[
                { label: 'Read', value: capabilities.fileSystem.read },
                { label: 'Write', value: capabilities.fileSystem.write },
                { label: 'Delete', value: capabilities.fileSystem.delete },
                { label: 'Execute', value: capabilities.fileSystem.execute },
              ]}
            />
          )}

          {capabilities.database && (
            <CapabilitySection
              icon={<Database className="h-5 w-5 text-purple-600" />}
              title="Database"
              capabilities={capabilities.database}
              fields={[
                { label: 'PostgreSQL', value: capabilities.database.postgresql },
                { label: 'MongoDB', value: capabilities.database.mongodb },
                { label: 'MySQL', value: capabilities.database.mysql },
                { label: 'SQLite', value: capabilities.database.sqlite },
                { label: 'Redis', value: capabilities.database.redis },
              ]}
            />
          )}

          {capabilities.network && (
            <CapabilitySection
              icon={<Globe className="h-5 w-5 text-green-600" />}
              title="Network"
              capabilities={capabilities.network}
              fields={[
                { label: 'HTTP', value: capabilities.network.http },
                { label: 'HTTPS', value: capabilities.network.https },
                { label: 'WebSocket', value: capabilities.network.websocket },
                { label: 'TCP', value: capabilities.network.tcp },
                { label: 'UDP', value: capabilities.network.udp },
              ]}
            />
          )}

          {capabilities.codeExecution && (
            <CapabilitySection
              icon={<Code className="h-5 w-5 text-orange-600" />}
              title="Code Execution"
              capabilities={capabilities.codeExecution}
              fields={[
                { label: 'eval()', value: capabilities.codeExecution.eval },
                { label: 'exec()', value: capabilities.codeExecution.exec },
                { label: 'Shell Commands', value: capabilities.codeExecution.shellCommands },
                { label: 'Child Processes', value: capabilities.codeExecution.childProcesses },
                { label: 'VM Execution', value: capabilities.codeExecution.vmExecution },
              ]}
            />
          )}

          {capabilities.credentialAccess && (
            <CapabilitySection
              icon={<Key className="h-5 w-5 text-red-600" />}
              title="Credential Access"
              capabilities={capabilities.credentialAccess}
              fields={[
                { label: 'Environment Variables', value: capabilities.credentialAccess.envVars },
                { label: 'Config Files', value: capabilities.credentialAccess.configFiles },
                { label: 'Keyring', value: capabilities.credentialAccess.keyring },
              ]}
            />
          )}

          {capabilities.browserAutomation && (
            <CapabilitySection
              icon={<Chrome className="h-5 w-5 text-indigo-600" />}
              title="Browser Automation"
              capabilities={capabilities.browserAutomation}
              fields={[
                { label: 'Puppeteer', value: capabilities.browserAutomation.puppeteer },
                { label: 'Playwright', value: capabilities.browserAutomation.playwright },
                { label: 'Selenium', value: capabilities.browserAutomation.selenium },
              ]}
            />
          )}
        </CardContent>
      </Card>

      {/* Environment Details */}
      <Card>
        <CardHeader>
          <CardTitle>Environment Details</CardTitle>
          <CardDescription>Runtime environment information</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <div className="text-sm font-medium text-muted-foreground mb-1">Language</div>
              <div className="text-sm">{environment.language} {environment.version}</div>
            </div>
            <div>
              <div className="text-sm font-medium text-muted-foreground mb-1">Runtime</div>
              <div className="text-sm">{environment.runtime}</div>
            </div>
            <div>
              <div className="text-sm font-medium text-muted-foreground mb-1">Platform</div>
              <div className="text-sm">{environment.platform} ({environment.arch})</div>
            </div>
            {environment.frameworks && environment.frameworks.length > 0 && (
              <div>
                <div className="text-sm font-medium text-muted-foreground mb-1">Frameworks</div>
                <div className="flex gap-1 flex-wrap">
                  {environment.frameworks.map((fw, idx) => (
                    <Badge key={idx} variant="secondary">
                      {fw}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* AI Model Usage */}
      {aiModels.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>AI Model Usage</CardTitle>
            <CardDescription>Detected AI models and providers</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {aiModels.map((model, idx) => (
                <div key={idx} className="flex items-start gap-3">
                  <Brain className="h-5 w-5 text-purple-600 flex-shrink-0 mt-0.5" />
                  <div className="flex-1">
                    <div className="font-medium">{model.provider}</div>
                    <div className="text-sm text-muted-foreground">
                      Models: {model.models.join(', ')}
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">
                      Detected via: {model.detectionType}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

interface CapabilitySectionProps {
  icon: React.ReactNode
  title: string
  capabilities: any
  fields: { label: string; value: boolean }[]
}

function CapabilitySection({ icon, title, capabilities, fields }: CapabilitySectionProps) {
  return (
    <div>
      <div className="flex items-center gap-2 mb-3">
        {icon}
        <h3 className="font-semibold">{title}</h3>
        <Badge variant="outline" className="ml-auto text-xs">
          {capabilities.detectionMethod}
        </Badge>
      </div>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-2 ml-7">
        {fields.map((field, idx) => (
          <div key={idx} className="flex items-center gap-2">
            {field.value ? (
              <CheckCircle className="h-4 w-4 text-green-600" />
            ) : (
              <XCircle className="h-4 w-4 text-gray-400" />
            )}
            <span className={`text-sm ${field.value ? '' : 'text-muted-foreground'}`}>{field.label}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
