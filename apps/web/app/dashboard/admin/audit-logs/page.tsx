'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Search, Download, Filter, ChevronLeft, ChevronRight } from 'lucide-react'
import { api } from '@/lib/api'

interface AuditLog {
  id: string
  user_id: string
  user_email: string
  action: string
  resource_type: string
  resource_id: string
  ip_address: string
  user_agent: string
  metadata?: Record<string, any>
  timestamp: string
}

const actionColors: Record<string, string> = {
  create: 'bg-green-100 text-green-800',
  update: 'bg-blue-100 text-blue-800',
  delete: 'bg-red-100 text-red-800',
  verify: 'bg-purple-100 text-purple-800',
  revoke: 'bg-orange-100 text-orange-800',
  login: 'bg-gray-100 text-gray-800',
  logout: 'bg-gray-100 text-gray-800',
}

export default function AuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [actionFilter, setActionFilter] = useState<string>('all')
  const [resourceFilter, setResourceFilter] = useState<string>('all')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [showExportMenu, setShowExportMenu] = useState(false)
  const limit = 20

  useEffect(() => {
    fetchLogs()
  }, [page])

  const fetchLogs = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getAuditLogs(limit, (page - 1) * limit)
      setLogs(data)
      // In real implementation, get total count from API
      setTotalPages(Math.ceil(100 / limit)) // Mock total of 100 logs
    } catch (err) {
      console.error('Failed to fetch audit logs:', err)
      setError(err instanceof Error ? err.message : 'An unknown error occurred')
      // No mock data fallback - show error instead
    } finally {
      setLoading(false)
    }
  }

  const exportLogs = async (format: 'json' | 'csv' = 'json') => {
    try {
      let content: string;
      let mimeType: string;
      let extension: string;

      if (format === 'csv') {
        // Convert to CSV
        const headers = ['ID', 'User Email', 'Action', 'Resource Type', 'Resource ID', 'IP Address', 'Timestamp'];
        const csvRows = [
          headers.join(','),
          ...filteredLogs.map(log => [
            log.id,
            log.user_email,
            log.action,
            log.resource_type,
            log.resource_id,
            log.ip_address,
            new Date(log.timestamp).toISOString()
          ].map(field => `"${field}"`).join(','))
        ];
        content = csvRows.join('\n');
        mimeType = 'text/csv';
        extension = 'csv';
      } else {
        // Export as JSON
        content = JSON.stringify(filteredLogs, null, 2);
        mimeType = 'application/json';
        extension = 'json';
      }

      const blob = new Blob([content], { type: mimeType });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit-logs-${new Date().toISOString().split('T')[0]}.${extension}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Failed to export logs:', error);
    }
  }

  const filteredLogs = logs.filter(log => {
    const matchesSearch =
      log.user_email?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      log.action.toLowerCase().includes(searchQuery.toLowerCase()) ||
      log.resource_type.toLowerCase().includes(searchQuery.toLowerCase())

    const matchesAction = actionFilter === 'all' || log.action === actionFilter
    const matchesResource = resourceFilter === 'all' || log.resource_type === resourceFilter

    return matchesSearch && matchesAction && matchesResource
  })

  const actions = Array.from(new Set(logs.map(l => l.action)))
  const resourceTypes = Array.from(new Set(logs.map(l => l.resource_type)))

  if (loading && logs.length === 0) {
    return <div className="flex items-center justify-center h-96">Loading audit logs...</div>
  }

  if (error) {
    const is403 = error.includes('403')
    return (
      <div className="flex flex-col items-center justify-center h-96 space-y-4 max-w-md mx-auto px-4">
        <div className="text-center space-y-3">
          <h2 className="text-2xl font-bold text-amber-600">
            {is403 ? 'Access Restricted' : 'Failed to Load Audit Logs'}
          </h2>
          {is403 ? (
            <>
              <p className="text-base text-gray-600 dark:text-gray-400">
                Audit logs are only available to <strong>Admin</strong> and <strong>Manager</strong> roles.
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-500">
                To view system audit trails and compliance reports, please contact your organization administrator to upgrade your account permissions.
              </p>
            </>
          ) : (
            <p className="text-muted-foreground">{error}</p>
          )}
        </div>
        {!is403 && <Button onClick={() => fetchLogs()}>Retry</Button>}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Audit Logs</h1>
          <p className="text-muted-foreground mt-1">
            Complete history of all platform actions
          </p>
        </div>
        <div className="relative">
          <Button onClick={() => setShowExportMenu(!showExportMenu)}>
            <Download className="mr-2 h-4 w-4" />
            Export Logs
          </Button>
          {showExportMenu && (
            <div className="absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 z-10">
              <button
                onClick={() => {
                  exportLogs('json');
                  setShowExportMenu(false);
                }}
                className="w-full text-left px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 first:rounded-t-lg transition-colors"
              >
                Export as JSON
              </button>
              <button
                onClick={() => {
                  exportLogs('csv');
                  setShowExportMenu(false);
                }}
                className="w-full text-left px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 last:rounded-b-lg transition-colors"
              >
                Export as CSV
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Logs</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{logs.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Today</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {logs.filter(l =>
                new Date(l.timestamp).toDateString() === new Date().toDateString()
              ).length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Unique Users</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {new Set(logs.map(l => l.user_id)).size}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Actions/Hour</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {Math.round(logs.length / 24)}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Search and Filter</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-4">
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search by user, action, or resource..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
              />
            </div>
            <Select value={actionFilter} onValueChange={setActionFilter}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="Filter by action" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Actions</SelectItem>
                {actions.map(action => (
                  <SelectItem key={action} value={action}>
                    {action}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={resourceFilter} onValueChange={setResourceFilter}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="Filter by resource" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Resources</SelectItem>
                {resourceTypes.map(type => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {(searchQuery || actionFilter !== 'all' || resourceFilter !== 'all') && (
            <div className="flex items-center gap-2">
              <Badge variant="secondary">
                {filteredLogs.length} of {logs.length} logs
              </Badge>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setSearchQuery('')
                  setActionFilter('all')
                  setResourceFilter('all')
                }}
              >
                Clear filters
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Logs Table */}
      <Card>
        <CardHeader>
          <CardTitle>Audit Trail</CardTitle>
          <CardDescription>
            Detailed record of all system actions
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {filteredLogs.map((log) => (
              <div
                key={log.id}
                className="p-4 border rounded-lg hover:bg-accent/50 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 space-y-1">
                    <div className="flex items-center gap-2">
                      <Badge className={actionColors[log.action] || 'bg-gray-100 text-gray-800'}>
                        {log.action}
                      </Badge>
                      <span className="text-sm font-medium">{log.resource_type}</span>
                      <span className="text-xs text-muted-foreground">
                        ID: {log.resource_id.substring(0, 8)}...
                      </span>
                    </div>

                    <p className="text-sm text-muted-foreground">
                      by <span className="font-medium">{log.user_email}</span>
                    </p>

                    <div className="flex items-center gap-4 text-xs text-muted-foreground">
                      <span>IP: {log.ip_address}</span>
                      <span>•</span>
                      <span className="truncate max-w-[300px]">
                        {log.user_agent}
                      </span>
                    </div>

                    {log.metadata && Object.keys(log.metadata).length > 0 && (
                      <details className="mt-2">
                        <summary className="text-xs text-muted-foreground cursor-pointer hover:text-foreground">
                          View metadata
                        </summary>
                        <pre className="mt-2 p-2 bg-muted rounded text-xs overflow-auto">
                          {JSON.stringify(log.metadata, null, 2)}
                        </pre>
                      </details>
                    )}
                  </div>

                  <div className="text-right">
                    <p className="text-sm font-medium">
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {new Date(log.timestamp).toLocaleDateString()}
                    </p>
                  </div>
                </div>
              </div>
            ))}

            {filteredLogs.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                No audit logs found matching your criteria
              </div>
            )}
          </div>

          {/* Pagination */}
          {filteredLogs.length > 0 && (
            <div className="flex items-center justify-between mt-6 pt-6 border-t">
              <p className="text-sm text-muted-foreground">
                Page {page} of {totalPages}
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page === 1}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                >
                  Next
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
