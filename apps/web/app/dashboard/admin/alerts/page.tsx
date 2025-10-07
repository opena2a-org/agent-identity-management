'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertTriangle,
  Info,
  ShieldAlert,
  CheckCircle2,
  Clock,
  Key,
  TrendingDown
} from 'lucide-react'
import { api } from '@/lib/api'

interface Alert {
  id: string
  alert_type: string
  severity: 'info' | 'warning' | 'critical'
  title: string
  description: string
  resource_type: string
  resource_id: string
  is_acknowledged: boolean
  acknowledged_by?: string
  acknowledged_at?: string
  created_at: string
}

const severityConfig = {
  info: {
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Info,
  },
  warning: {
    color: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    icon: AlertTriangle,
  },
  critical: {
    color: 'bg-red-100 text-red-800 border-red-200',
    icon: ShieldAlert,
  },
}

const alertTypeIcons: Record<string, any> = {
  certificate_expiring: Clock,
  api_key_expiring: Key,
  trust_score_low: TrendingDown,
  agent_offline: AlertTriangle,
  security_breach: ShieldAlert,
  unusual_activity: Info,
}

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)
  const [severityFilter, setSeverityFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('unacknowledged')

  useEffect(() => {
    fetchAlerts()
  }, [])

  const fetchAlerts = async () => {
    try {
      const data = await api.getAlerts(100, 0)
      setAlerts(data)
    } catch (error) {
      console.error('Failed to fetch alerts:', error)
    } finally {
      setLoading(false)
    }
  }

  const acknowledgeAlert = async (alertId: string) => {
    try {
      await api.acknowledgeAlert(alertId)
      // Update local state
      setAlerts(alerts.map(a =>
        a.id === alertId
          ? { ...a, is_acknowledged: true, acknowledged_at: new Date().toISOString() }
          : a
      ))
    } catch (error) {
      console.error('Failed to acknowledge alert:', error)
      alert('Failed to acknowledge alert')
    }
  }

  const acknowledgeAll = async () => {
    try {
      const unacknowledged = filteredAlerts.filter(a => !a.is_acknowledged)
      await Promise.all(unacknowledged.map(a => api.acknowledgeAlert(a.id)))
      setAlerts(alerts.map(a =>
        !a.is_acknowledged
          ? { ...a, is_acknowledged: true, acknowledged_at: new Date().toISOString() }
          : a
      ))
    } catch (error) {
      console.error('Failed to acknowledge all alerts:', error)
      alert('Failed to acknowledge all alerts')
    }
  }

  const filteredAlerts = alerts.filter(alert => {
    const matchesSeverity = severityFilter === 'all' || alert.severity === severityFilter
    const matchesStatus =
      statusFilter === 'all' ||
      (statusFilter === 'acknowledged' && alert.is_acknowledged) ||
      (statusFilter === 'unacknowledged' && !alert.is_acknowledged)

    return matchesSeverity && matchesStatus
  })

  const stats = {
    total: alerts.length,
    critical: alerts.filter(a => a.severity === 'critical' && !a.is_acknowledged).length,
    warning: alerts.filter(a => a.severity === 'warning' && !a.is_acknowledged).length,
    info: alerts.filter(a => a.severity === 'info' && !a.is_acknowledged).length,
    unacknowledged: alerts.filter(a => !a.is_acknowledged).length,
  }

  if (loading) {
    return <div className="flex items-center justify-center h-96">Loading alerts...</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Security Alerts</h1>
          <p className="text-muted-foreground mt-1">
            Proactive monitoring and notifications
          </p>
        </div>
        {stats.unacknowledged > 0 && (
          <Button onClick={acknowledgeAll}>
            <CheckCircle2 className="mr-2 h-4 w-4" />
            Acknowledge All ({stats.unacknowledged})
          </Button>
        )}
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Alerts</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.total}</div>
          </CardContent>
        </Card>
        <Card className="border-red-200 bg-red-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-red-800">
              Critical
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-900">{stats.critical}</div>
          </CardContent>
        </Card>
        <Card className="border-yellow-200 bg-yellow-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-yellow-800">
              Warning
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-900">{stats.warning}</div>
          </CardContent>
        </Card>
        <Card className="border-blue-200 bg-blue-50">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-blue-800">
              Info
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-900">{stats.info}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Filter Alerts</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4">
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Alerts</SelectItem>
              <SelectItem value="unacknowledged">Unacknowledged</SelectItem>
              <SelectItem value="acknowledged">Acknowledged</SelectItem>
            </SelectContent>
          </Select>

          <Select value={severityFilter} onValueChange={setSeverityFilter}>
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by severity" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Severities</SelectItem>
              <SelectItem value="critical">Critical</SelectItem>
              <SelectItem value="warning">Warning</SelectItem>
              <SelectItem value="info">Info</SelectItem>
            </SelectContent>
          </Select>

          {(severityFilter !== 'all' || statusFilter !== 'unacknowledged') && (
            <Button
              variant="ghost"
              onClick={() => {
                setSeverityFilter('all')
                setStatusFilter('unacknowledged')
              }}
            >
              Clear filters
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Alerts List */}
      <Card>
        <CardHeader>
          <CardTitle>Active Alerts ({filteredAlerts.length})</CardTitle>
          <CardDescription>
            Security and operational notifications requiring attention
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {filteredAlerts.map((alert) => {
              const config = severityConfig[alert.severity]
              const Icon = config.icon
              const TypeIcon = alertTypeIcons[alert.alert_type] || AlertTriangle

              return (
                <div
                  key={alert.id}
                  className={`p-4 border-2 rounded-lg ${
                    alert.is_acknowledged ? 'opacity-60 bg-muted/30' : config.color
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-3 flex-1">
                      <Icon className="h-5 w-5 mt-0.5" />

                      <div className="flex-1 space-y-2">
                        <div className="flex items-start justify-between">
                          <div>
                            <div className="flex items-center gap-2">
                              <h3 className="font-semibold">{alert.title}</h3>
                              <Badge variant="outline" className="text-xs">
                                <TypeIcon className="h-3 w-3 mr-1" />
                                {alert.alert_type.replace(/_/g, ' ')}
                              </Badge>
                            </div>
                            <p className="text-sm mt-1">{alert.description}</p>
                          </div>
                        </div>

                        <div className="flex items-center gap-4 text-xs">
                          <span>
                            {alert.resource_type}: {alert.resource_id.substring(0, 8)}...
                          </span>
                          <span>•</span>
                          <span>
                            {new Date(alert.created_at).toLocaleString()}
                          </span>
                        </div>

                        {alert.is_acknowledged && (
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <CheckCircle2 className="h-3 w-3" />
                            <span>
                              Acknowledged {alert.acknowledged_at &&
                                new Date(alert.acknowledged_at).toLocaleString()}
                            </span>
                          </div>
                        )}
                      </div>

                      {!alert.is_acknowledged && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => acknowledgeAlert(alert.id)}
                        >
                          <CheckCircle2 className="h-4 w-4 mr-2" />
                          Acknowledge
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}

            {filteredAlerts.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <CheckCircle2 className="h-12 w-12 mx-auto mb-4 text-green-600" />
                <p className="text-lg font-medium">No alerts to display</p>
                <p className="text-sm">
                  {statusFilter === 'unacknowledged'
                    ? 'All alerts have been acknowledged'
                    : 'No alerts match your filter criteria'}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
